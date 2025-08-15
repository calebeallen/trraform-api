package payment

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"time"
	"trraformapi/internal/api"
	"trraformapi/pkg/config"
	plotutils "trraformapi/pkg/plot_utils"
	"trraformapi/pkg/schemas"
	"trraformapi/pkg/utils"

	"github.com/stripe/stripe-go/v82"
	"github.com/stripe/stripe-go/v82/webhook"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

func (h *Handler) StripeWebhook(w http.ResponseWriter, r *http.Request) {

	defer r.Body.Close()
	ctx, cancel := context.WithTimeout(context.Background(), config.API_TIMEOUT)
	defer cancel()
	resParams := &api.ResParams{W: w, R: r}

	payload, err := io.ReadAll(r.Body)
	if err != nil {
		h.Bad(resParams, err)
		return
	}

	event, err := webhook.ConstructEvent(payload, r.Header.Get("Stripe-Signature"), config.ENV.STRIPE_WEBHOOK_SECRET)
	if err != nil {
		resParams.Code = http.StatusUnauthorized
		resParams.Err = err
		h.Res(resParams)
		return
	}

	switch event.Type {

	// handle plot purchase success
	case stripe.EventTypeCheckoutSessionCompleted:
		var checkoutSession stripe.CheckoutSession
		if err = json.Unmarshal(event.Data.Raw, &checkoutSession); err != nil {
			h.Bad(resParams, err)
			return
		}
		if checkoutSession.Mode == stripe.CheckoutSessionModePayment {
			if err := checkoutCompleted(h, ctx, &checkoutSession); err != nil {
				h.Err(resParams, err)
				return
			}
		}

	// handle plot purchase failed
	case stripe.EventTypeCheckoutSessionExpired:
		var checkoutSession stripe.CheckoutSession
		if err := json.Unmarshal(event.Data.Raw, &checkoutSession); err != nil {
			h.Bad(resParams, err)
			return
		}
		if checkoutSession.Mode == stripe.CheckoutSessionModePayment {
			if err := checkoutCanceled(h, &checkoutSession); err != nil {
				resParams.Code = http.StatusInternalServerError
				resParams.Err = err
				h.Res(resParams)
				return
			}
		}

	// handle subscription creation/cycle
	case stripe.EventTypeInvoicePaid:
		var invoice stripe.Invoice
		if err := json.Unmarshal(event.Data.Raw, &invoice); err != nil {
			h.Bad(resParams, err)
			return
		}
		switch invoice.BillingReason {

		case stripe.InvoiceBillingReasonSubscriptionCreate:
			if err := createSubscription(h, ctx, &invoice); err != nil {
				h.Err(resParams, err)
				return
			}

		case stripe.InvoiceBillingReasonSubscriptionCycle:
			if err := renewSubscription(h, ctx, &invoice); err != nil {
				h.Err(resParams, err)
				return
			}

		}

	// handle subscription cancellation
	case stripe.EventTypeCustomerSubscriptionDeleted:
		var sub stripe.Subscription
		if err := json.Unmarshal(event.Data.Raw, &sub); err != nil {
			h.Bad(resParams, err)
			return
		}
		if err := cancelSubscription(h, ctx, &sub); err != nil {
			h.Err(resParams, err)
			return
		}

	}

	resParams.Code = http.StatusOK
	h.Res(resParams)

}

func checkoutCompleted(h *Handler, ctx context.Context, checkoutSession *stripe.CheckoutSession) error {

	// extract uid and cart session id
	uidStr, ok := checkoutSession.Metadata["uid"]
	if !ok {
		return errors.New("uid field missing from payment intent metadata")
	}
	uid, err := bson.ObjectIDFromHex(uidStr)
	if err != nil {
		return err
	}
	lockOwner, ok := checkoutSession.Metadata["lo"]
	if !ok {
		return errors.New("sid field missing from payment intent metadata")
	}

	// extract purchased plot ids
	var plotIds []*plotutils.PlotId
	var plotIdStrs []string
	for i := range config.MAX_CART_SIZE {
		plotIdStr, ok := checkoutSession.Metadata[fmt.Sprintf("%d", i)]
		if !ok {
			break
		}
		plotIdStrs = append(plotIdStrs, plotIdStr)
		plotId, err := plotutils.PlotIdFromHexString(plotIdStr)
		if err != nil {
			return err
		}
		plotIds = append(plotIds, plotId)
	}

	// update and get user
	var user schemas.User
	if err := h.MongoDB.Collection("users").FindOneAndUpdate(ctx, bson.M{
		"_id": uid,
	}, bson.M{
		"$addToSet": bson.M{
			"plotIds": bson.M{
				"$each": plotIdStrs,
			},
			"purchasedIds": bson.M{
				"$each": plotIdStrs,
			},
		},
	}, options.FindOneAndUpdate().SetReturnDocument(options.After)).Decode(&user); err != nil {
		return err
	}

	// create plot entries in mongo
	plotDocs := make([]schemas.Plot, len(plotIds))
	now := time.Now().UTC()
	for i := range plotIds {
		doc := &plotDocs[i]
		doc.PlotId = plotIdStrs[i]
		doc.Ctime = now
	}
	if _, err := h.MongoDB.Collection("plots").InsertMany(ctx, plotDocs); err != nil {
		return err
	}

	// create default plot data
	for _, plotId := range plotIds {
		if err := plotutils.SetDefaultPlot(h.RedisCli, h.R2Cli, ctx, plotId, &user); err != nil {
			return err
		}
	}

	_, err = plotutils.UnlockPlots(h.RedisCli, lockOwner)
	if err != nil {
		return err
	}

	return nil

}

func checkoutCanceled(h *Handler, checkoutSession *stripe.CheckoutSession) error {

	// cart session id
	lockOwner, ok := checkoutSession.Metadata["lo"]
	if !ok {
		return errors.New("sid field missing from payment intent metadata")
	}

	_, err := plotutils.UnlockPlots(h.RedisCli, lockOwner)
	if err != nil {
		return err
	}

	return nil

}

func createSubscription(h *Handler, ctx context.Context, invoice *stripe.Invoice) error {

	uidStr, ok := invoice.Parent.SubscriptionDetails.Metadata["uid"]
	if !ok {
		return errors.New("uid field missing from payment intent metadata")
	}
	uid, err := bson.ObjectIDFromHex(uidStr)
	if err != nil {
		return err
	}

	var user schemas.User
	if err := h.MongoDB.Collection("users").FindOneAndUpdate(ctx, bson.M{
		"_id": uid,
	}, bson.M{
		"$set": bson.M{
			"subscription.isActive":       true,
			"subscription.subscriptionId": invoice.Parent.SubscriptionDetails.Subscription.ID,
		},
	}, options.FindOneAndUpdate().SetReturnDocument(options.After)).Decode(&user); err != nil {
		return err
	}

	// set metadata verified true for all plots
	for _, plotId := range user.PlotIds {
		metadata := map[string]string{
			"owner":    user.Username,
			"verified": strconv.FormatBool(true),
		}
		if err := utils.UpdateMetadataR2(h.R2Cli, ctx, config.CF_PLOT_BUCKET, plotId+".dat", "application/octet-stream", metadata); err != nil {
			return err
		}
	}

	return renewSubscription(h, ctx, invoice)

}

func renewSubscription(h *Handler, ctx context.Context, invoice *stripe.Invoice) error {

	uidStr, ok := invoice.Parent.SubscriptionDetails.Metadata["uid"]
	if !ok {
		return errors.New("uid field missing from payment intent metadata")
	}
	uid, err := bson.ObjectIDFromHex(uidStr)
	if err != nil {
		return err
	}

	// give bonus plot credit for first 6 payment cycles
	bonusPlotThreshold := fmt.Sprintf("subscription.invoices.%d", config.SUBSCRIPTION_BONUS_PLOTS-1)
	res, err := h.MongoDB.Collection("users").UpdateOne(ctx,
		bson.M{
			"_id": uid,
			"subscription.invoices": bson.M{
				"$nin": invoice.ID,
			},
			bonusPlotThreshold: bson.M{
				"$exists": false,
			},
		},
		bson.M{
			"$inc": bson.M{
				"plotCredits": 1,
			},
			"$push": bson.M{
				"subscription.invoices": invoice.ID,
			},
		},
	)
	if err != nil {
		return err
	}
	if res.MatchedCount > 0 { // success
		return nil
	}

	// increment recurred count
	_, err = h.MongoDB.Collection("users").UpdateOne(ctx,
		bson.M{
			"_id": uid,
			"subscription.invoices": bson.M{
				"$nin": invoice.ID,
			},
		},
		bson.M{
			"$push": bson.M{
				"subscription.invoices": invoice.ID,
			},
		},
	)

	return err

}

func cancelSubscription(h *Handler, ctx context.Context, subscription *stripe.Subscription) error {

	uidStr, ok := subscription.Metadata["uid"]
	if !ok {
		return errors.New("uid field missing from payment intent metadata")
	}
	uid, err := bson.ObjectIDFromHex(uidStr)
	if err != nil {
		return err
	}

	var user schemas.User
	if err := h.MongoDB.Collection("users").FindOneAndUpdate(ctx, bson.M{
		"_id": uid,
	}, bson.M{
		"$set": bson.M{
			"subscription.isActive": false,
		},
	}, options.FindOneAndUpdate().SetReturnDocument(options.After)).Decode(user); err != nil {
		return err
	}

	// set metadata verified false for all plots
	for _, plotId := range user.PlotIds {
		metadata := map[string]string{
			"owner":    user.Username,
			"verified": strconv.FormatBool(false),
		}
		if err := utils.UpdateMetadataR2(h.R2Cli, ctx, config.CF_PLOT_BUCKET, plotId+".dat", "application/octet-stream", metadata); err != nil {
			return err
		}
	}

	return nil

}
