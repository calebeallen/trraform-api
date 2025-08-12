package payment

import (
	"net/http"
	"trraformapi/internal/api"
	"trraformapi/pkg/config"
	"trraformapi/pkg/schemas"

	"github.com/stripe/stripe-go/v82"
	"github.com/stripe/stripe-go/v82/checkout/session"
	"github.com/stripe/stripe-go/v82/customer"
	"go.mongodb.org/mongo-driver/v2/bson"
)

func (h *Handler) CreateSubscriptionSession(w http.ResponseWriter, r *http.Request) {

	defer r.Body.Close()
	ctx := r.Context()
	uid := ctx.Value("uid").(bson.ObjectID)
	uidStr := uid.Hex()
	resParams := &api.ResParams{W: w, R: r}

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
			"stripeCustomer": cus.ID,
		}); err != nil {
			resParams.Code = http.StatusInternalServerError
			resParams.Err = err
			h.Res(resParams)
			return
		}
	} else {
		stripeCustomerId = user.StripeCustomer
	}

	// metadata
	metadata := map[string]string{
		"uid": uidStr,
	}

	// create stripe session
	checkoutParams := &stripe.CheckoutSessionParams{
		Mode:       stripe.String(string(stripe.CheckoutSessionModeSubscription)),
		SuccessURL: stripe.String("https://yourapp.com/checkout/success?session_id={CHECKOUT_SESSION_ID}"),
		CancelURL:  stripe.String("https://yourapp.com/checkout/cancel"),
		Customer:   stripe.String(stripeCustomerId),

		// tax
		BillingAddressCollection: stripe.String("required"),
		AutomaticTax: &stripe.CheckoutSessionAutomaticTaxParams{
			Enabled: stripe.Bool(true),
		},
		CustomerUpdate: &stripe.CheckoutSessionCustomerUpdateParams{
			Address: stripe.String("auto"),
		},

		LineItems: []*stripe.CheckoutSessionLineItemParams{
			{
				Price:    stripe.String(config.PRICE_ID_SUBSCRIPTION),
				Quantity: stripe.Int64(1),
			},
		},
		Metadata: metadata,
	}

	checkoutSession, err := session.New(checkoutParams)
	if err != nil {
		resParams.Code = http.StatusInternalServerError
		resParams.Err = err
		h.Res(resParams)
		return
	}

	resParams.ResData = &struct {
		CheckoutSession string `json:"checkoutSession"`
	}{CheckoutSession: checkoutSession.ID}
	resParams.Code = http.StatusOK
	h.Res(resParams)

}
