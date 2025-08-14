package payment

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"
	"trraformapi/internal/api"
	"trraformapi/pkg/config"
	plotutils "trraformapi/pkg/plot_utils"
	"trraformapi/pkg/schemas"

	"github.com/stripe/stripe-go/v82"
	"github.com/stripe/stripe-go/v82/webhook"
	"go.mongodb.org/mongo-driver/v2/bson"
)

func (h *Handler) StripeWebhook(w http.ResponseWriter, r *http.Request) {

	defer r.Body.Close()
	ctx := context.Background()
	resParams := &api.ResParams{W: w, R: r}

	payload, err := io.ReadAll(r.Body)
	if err != nil {
		resParams.Code = http.StatusBadRequest
		h.Res(resParams)
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
		var cs stripe.CheckoutSession
		if err = json.Unmarshal(event.Data.Raw, &cs); err != nil {
			resParams.Code = http.StatusBadRequest
			resParams.Err = err
			h.Res(resParams)
			return
		}
		t, ok := cs.Metadata["type"]
		if !ok {
			resParams.Code = http.StatusInternalServerError
			resParams.Err = errors.New("type field missing from payment intent metadata")
			h.Res(resParams)
			return
		}
		if t == config.CHECK_OUT_TYPE_PLOT_PURCHASE {
			if err := checkoutCompleted(h, ctx, &cs); err != nil {
				resParams.Code = http.StatusInternalServerError
				resParams.Err = err
				h.Res(resParams)
				return
			}
		}

	// handle plot purchase failed
	case stripe.EventTypeCheckoutSessionExpired:
		var cs stripe.CheckoutSession
		if err := json.Unmarshal(event.Data.Raw, &cs); err != nil {
			resParams.Code = http.StatusBadRequest
			resParams.Err = err
			h.Res(resParams)
			return
		}
		t, ok := cs.Metadata["type"]
		if !ok {
			resParams.Code = http.StatusInternalServerError
			resParams.Err = errors.New("type field missing from invoice metadata")
			h.Res(resParams)
			return
		}
		if t == config.CHECK_OUT_TYPE_PLOT_PURCHASE {
			if err := checkoutCanceled(h, &cs); err != nil {
				resParams.Code = http.StatusInternalServerError
				resParams.Err = err
				h.Res(resParams)
				return
			}
		}

	// handle subscription creation/renewal
	case stripe.EventTypeInvoicePaid:
		var inv stripe.Invoice
		if err := json.Unmarshal(event.Data.Raw, &inv); err != nil {
			resParams.Code = http.StatusBadRequest
			resParams.Err = err
			h.Res(resParams)
			return
		}

		t, ok := inv.Parent.SubscriptionDetails.Metadata["type"]
		if !ok {
			resParams.Code = http.StatusInternalServerError
			resParams.Err = errors.New("type field missing from invoice metadata")
			h.Res(resParams)
			return
		}

		if t == config.CHECK_OUT_TYPE_SUBSCRIPTION {
			var err error = nil
			switch inv.BillingReason {
			case stripe.InvoiceBillingReasonSubscriptionCreate:
				err = createSubscription(h, ctx, &inv)
			case stripe.InvoiceBillingReasonSubscriptionCycle:
				err = renewSubscription(h, ctx, inv.Parent.SubscriptionDetails.Metadata)
			}
			if err != nil {
				resParams.Code = http.StatusInternalServerError
				resParams.Err = err
				h.Res(resParams)
				return
			}
		}

	// handle subscription cancellation
	case stripe.EventTypeCustomerSubscriptionDeleted:
		var si stripe.Subscription
		if err := json.Unmarshal(event.Data.Raw, &si); err != nil {
			resParams.Code = http.StatusBadRequest
			resParams.Err = err
			h.Res(resParams)
			return
		}
		if err := cancelSubscription(h, ctx, si.Metadata); err != nil {
			resParams.Code = http.StatusInternalServerError
			resParams.Err = err
			h.Res(resParams)
			return
		}

	}

	resParams.Code = http.StatusOK
	h.Res(resParams)

}

func checkoutCompleted(h *Handler, ctx context.Context, cs *stripe.CheckoutSession) error {

	// extract uid and cart session id
	uidStr, ok := cs.Metadata["uid"]
	if !ok {
		return errors.New("uid field missing from payment intent metadata")
	}
	uid, err := bson.ObjectIDFromHex(uidStr)
	if err != nil {
		return err
	}
	lockOwner, ok := cs.Metadata["lo"]
	if !ok {
		return errors.New("sid field missing from payment intent metadata")
	}

	// extract purchased plot ids
	var plotIds []*plotutils.PlotId
	var plotIdStrs []string
	for i := range config.MAX_CART_SIZE {
		plotIdStr, ok := cs.Metadata[fmt.Sprintf("%d", i)]
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
	}).Decode(&user); err != nil {
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
		if err := plotutils.FlagPlotForUpdate(h.RedisCli, ctx, plotId); err != nil {
			return err
		}
	}

	_, err = plotutils.UnlockPlots(h.RedisCli, lockOwner)
	if err != nil {
		return err
	}

	return nil

}

func checkoutCanceled(h *Handler, cs *stripe.CheckoutSession) error {

	// cart session id
	lockOwner, ok := cs.Metadata["lo"]
	if !ok {
		return errors.New("sid field missing from payment intent metadata")
	}

	_, err := plotutils.UnlockPlots(h.RedisCli, lockOwner)
	if err != nil {
		return err
	}

	return nil

}

func createSubscription(h *Handler, ctx context.Context, inv *stripe.Invoice) error {

	uidStr, ok := inv.Parent.SubscriptionDetails.Metadata["uid"]
	if !ok {
		return errors.New("uid field missing from payment intent metadata")
	}
	uid, err := bson.ObjectIDFromHex(uidStr)
	if err != nil {
		return err
	}

	if _, err := h.MongoDB.Collection("users").UpdateOne(ctx, bson.M{
		"_id": uid,
	}, bson.M{
		"$set": bson.M{
			"subscription.isActive":       true,
			"subscription.subscriptionId": inv.Parent.SubscriptionDetails.Subscription.ID,
		},
	}); err != nil {
		return err
	}

	return renewSubscription(h, ctx, inv.Parent.SubscriptionDetails.Metadata)

}

func renewSubscription(h *Handler, ctx context.Context, data map[string]string) error {

	uidStr, ok := data["uid"]
	if !ok {
		return errors.New("uid field missing from payment intent metadata")
	}
	uid, err := bson.ObjectIDFromHex(uidStr)
	if err != nil {
		return err
	}

	// give bonus plot credit for first 6 payment cycles
	if _, err := h.MongoDB.Collection("users").UpdateOne(ctx, bson.M{
		"_id": uid,
		"subscription.recurredCount": bson.M{
			"$lt": config.SUBSCRIPTION_BONUS_PLOTS,
		},
	}, bson.M{
		"$inc": bson.M{
			"plotCredits": 1,
		},
	}); err != nil {
		return err
	}

	// increment recurred count
	if _, err := h.MongoDB.Collection("users").UpdateOne(ctx, bson.M{
		"_id": uid,
	}, bson.M{
		"$inc": bson.M{
			"subscription.recurredCount": 1,
		},
	}); err != nil {
		return err
	}

	return nil

}

func cancelSubscription(h *Handler, ctx context.Context, data map[string]string) error {

	uidStr, ok := data["uid"]
	if !ok {
		return errors.New("uid field missing from payment intent metadata")
	}
	uid, err := bson.ObjectIDFromHex(uidStr)
	if err != nil {
		return err
	}

	if _, err := h.MongoDB.Collection("users").UpdateOne(ctx, bson.M{
		"_id": uid,
	}, bson.M{
		"$set": bson.M{
			"subscription.isActive": false,
		},
	}); err != nil {
		return err
	}

	return nil

}
