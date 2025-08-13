package payment

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
	"strings"
	"time"
	"trraformapi/internal/api"
	"trraformapi/pkg/config"
	plotutils "trraformapi/pkg/plot_utils"
	"trraformapi/pkg/schemas"

	"github.com/stripe/stripe-go/v82"
	"github.com/stripe/stripe-go/v82/customer"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

func (h *Handler) CreateCheckoutSession(w http.ResponseWriter, r *http.Request) {

	defer r.Body.Close()
	ctx := r.Context()
	uid := ctx.Value("uid").(bson.ObjectID)
	uidStr := uid.Hex()
	resParams := &api.ResParams{W: w, R: r}

	var reqData struct {
		PlotIds []string `json:"plotIds" validate:"required,min=1,max=40"`
	}

	// validate request body
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&reqData); err != nil {
		resParams.Code = http.StatusBadRequest
		resParams.Err = err
		h.Res(resParams)
		return
	}
	if err := h.Validate.Struct(&reqData); err != nil {
		resParams.Code = http.StatusBadRequest
		resParams.Err = err
		h.Res(resParams)
		return
	}
	resParams.ReqData = reqData

	// get user data
	var user schemas.User
	userColl := h.MongoDB.Collection("users")
	if err := userColl.FindOne(ctx, bson.M{
		"_id": uid,
	}).Decode(&user); err != nil {
		resParams.Code = http.StatusInternalServerError
		resParams.Err = err
		h.Res(resParams)
		return
	}

	// create stripe customer if needed
	var stripeCustomerId string
	if user.StripeCustomer == "" {
		cus, err := customer.New(&stripe.CustomerParams{
			Email: stripe.String(user.Email),
		})
		if err != nil {
			resParams.Code = http.StatusInternalServerError
			resParams.Err = err
			h.Res(resParams)
			return
		}
		stripeCustomerId = cus.ID

		// update user with new stripe customer id
		if _, err := userColl.UpdateOne(ctx, bson.M{
			"_id": uid,
		}, bson.M{
			"$set": bson.M{
				"stripeCustomer": cus.ID,
			},
		}); err != nil {
			resParams.Code = http.StatusInternalServerError
			resParams.Err = err
			h.Res(resParams)
			return
		}
	} else {
		stripeCustomerId = user.StripeCustomer
	}

	// check for user plot limit exceeded
	if len(user.PlotIds)+len(reqData.PlotIds) > config.USER_PLOT_LIMIT {
		resParams.ResData = &struct {
			PlotLimitExceeded bool `json:"plotLimitExceeded"`
		}{PlotLimitExceeded: true}
		resParams.Code = http.StatusBadRequest
		h.Res(resParams)
		return
	}

	// validate plot ids, check for duplicates
	uniq := make(map[string]struct{}, len(reqData.PlotIds))
	plotIdStrs := make([]string, len(reqData.PlotIds)) //normalized plotId strings
	plotIds := make([]*plotutils.PlotId, len(reqData.PlotIds))
	for i := range reqData.PlotIds {

		// validate and normalize plot id
		plotId, err := plotutils.PlotIdFromHexString(reqData.PlotIds[i])
		if err != nil || !plotId.Validate() {
			resParams.ResData = &struct {
				InvalidPlotId bool `json:"invalidPlotId"`
			}{InvalidPlotId: true}
			resParams.Err = err
			resParams.Code = http.StatusBadRequest
			h.Res(resParams)
			return
		}
		plotIds[i] = plotId

		// check for duplicate
		plotIdStr := plotId.ToString()
		plotIdStrs[i] = plotIdStr // use this over passed in plot id string
		if _, isDup := uniq[plotIdStr]; isDup {
			resParams.Code = http.StatusBadRequest
			resParams.ResData = &struct {
				DuplicatePlotId bool `json:"duplicatePlotId"`
			}{DuplicatePlotId: true}
			h.Res(resParams)
			return
		}
		uniq[plotIdStr] = struct{}{}
	}

	// create checkout session id
	sort.Strings(plotIdStrs)
	base := uidStr + ":pay:" + strings.Join(plotIdStrs, ",")
	sum := sha256.Sum256([]byte(base))
	cartId := hex.EncodeToString(sum[:])

	// lock plots
	failed, err := plotutils.LockPlots(h.RedisCli, ctx, plotIdStrs, cartId)
	if err != nil {
		resParams.Code = http.StatusInternalServerError
		resParams.Err = err
		h.Res(resParams)
		return
	}
	if len(failed) > 0 {
		resParams.ResData = &struct {
			Conflicts []string `json:"conflicts"`
		}{Conflicts: failed}
		resParams.Code = http.StatusConflict
		h.Res(resParams)
		return
	}

	// verify that plots are not already claimed
	filter := bson.M{"plotId": bson.M{"$in": plotIdStrs}}
	cursor, err := h.MongoDB.Collection("plots").Find(ctx, filter, options.Find().SetProjection(bson.M{"plotId": 1}))
	if err != nil {
		plotutils.UnlockPlots(h.RedisCli, plotIdStrs, cartId)
		resParams.Code = http.StatusInternalServerError
		resParams.Err = err
		h.Res(resParams)
		return
	}
	defer cursor.Close(ctx)

	// check for conflicts
	var results []struct {
		PlotId string `bson:"plotId"`
	}
	if err := cursor.All(ctx, &results); err != nil {
		plotutils.UnlockPlots(h.RedisCli, plotIdStrs, cartId)
		resParams.Code = http.StatusInternalServerError
		resParams.Err = err
		h.Res(resParams)
		return
	}

	// return conflicts if any
	if len(results) > 0 {
		plotutils.UnlockPlots(h.RedisCli, plotIdStrs, cartId)
		conflicts := make([]string, len(results))
		for i := 0; i < len(results); i++ {
			conflicts[i] = results[i].PlotId
		}
		resParams.ResData = &struct {
			Conflicts []string `json:"conflicts"`
		}{Conflicts: conflicts}
		resParams.Code = http.StatusConflict
		h.Res(resParams)
		return
	}

	// get quantities for each plot depth
	quantities := make([]int64, config.MAX_DEPTH+1)
	for i := range plotIds {
		quantities[plotIds[i].Depth()]++
	}
	// create order
	lineItems := []*stripe.CheckoutSessionCreateLineItemParams{}
	for depth, q := range quantities {
		if q > 0 {
			lineItems = append(lineItems, &stripe.CheckoutSessionCreateLineItemParams{
				Price:    stripe.String(config.PRICE_ID_DEPTH[depth]),
				Quantity: stripe.Int64(q),
			})
		}
	}

	// metadata
	metadata := map[string]string{
		"type": "plot-purchase",
		"uid":  uidStr,
		"sid":  cartId,
	}
	for i, plotId := range plotIdStrs {
		metadata[fmt.Sprintf("%d", i)] = plotId
	}

	// create stripe session
	checkoutParams := &stripe.CheckoutSessionCreateParams{
		Mode:              stripe.String(string(stripe.CheckoutSessionModePayment)),
		SuccessURL:        stripe.String("https://yourapp.com/checkout/success?session_id={CHECKOUT_SESSION_ID}"),
		CancelURL:         stripe.String("https://yourapp.com/checkout/cancel"),
		Customer:          stripe.String(stripeCustomerId),
		ClientReferenceID: stripe.String(uidStr),
		ExpiresAt:         stripe.Int64(time.Now().Add(time.Hour).Unix()),

		// tax
		BillingAddressCollection: stripe.String("auto"),
		AutomaticTax: &stripe.CheckoutSessionCreateAutomaticTaxParams{
			Enabled: stripe.Bool(true),
		},
		CustomerUpdate: &stripe.CheckoutSessionCreateCustomerUpdateParams{
			Address: stripe.String("auto"),
		},

		LineItems: lineItems,
		Metadata:  metadata,
		PaymentIntentData: &stripe.CheckoutSessionCreatePaymentIntentDataParams{
			Metadata:         metadata,
			SetupFutureUsage: stripe.String(string(stripe.PaymentIntentSetupFutureUsageOffSession)),
		},
	}
	checkoutParams.SetIdempotencyKey(cartId)

	checkoutSession, err := h.StripeCli.V1CheckoutSessions.Create(ctx, checkoutParams)
	if err != nil {
		plotutils.UnlockPlots(h.RedisCli, plotIdStrs, cartId)
		resParams.Code = http.StatusInternalServerError
		resParams.Err = err
		h.Res(resParams)
		return
	}

	resParams.ResData = &struct {
		StripeSession string `json:"stripeSession"`
	}{StripeSession: checkoutSession.ID}
	resParams.Code = http.StatusOK
	h.Res(resParams)

}
