package payment

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"
	"trraformapi/utils"
	plotutils "trraformapi/utils/plot_utils"
	"trraformapi/utils/schemas"

	"github.com/stripe/stripe-go/v82"
	"github.com/stripe/stripe-go/v82/webhook"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
	"golang.org/x/sync/errgroup"
)

func StripeWebhook(w http.ResponseWriter, r *http.Request) {

	ctx := context.Background()

	const MaxBodyBytes = int64(65536)
	r.Body = http.MaxBytesReader(w, r.Body, MaxBodyBytes)
	payload, err := io.ReadAll(r.Body)

	if err != nil {
		utils.LogErrorDiscord("StripeWebhook", err, nil)
		w.WriteHeader(http.StatusServiceUnavailable)
		return
	}

	event, err := webhook.ConstructEvent(payload, r.Header.Get("Stripe-Signature"), os.Getenv("STRIPE_WEBHOOK_SECRET"))

	if err != nil {
		utils.LogErrorDiscord("StripeWebhook", err, nil)
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	// Unmarshal the event data into an appropriate struct depending on its Type
	switch event.Type {
	case "payment_intent.succeeded":

		var paymentIntent stripe.PaymentIntent
		if err := json.Unmarshal(event.Data.Raw, &paymentIntent); err != nil {
			utils.LogErrorDiscord("StripeWebhook", err, nil)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		if err := handlePaymentSucceeded(ctx, &paymentIntent); err != nil {
			utils.LogErrorDiscord("StripeWebhook", err, nil)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

	case "payment_intent.payment_failed":

		var paymentIntent stripe.PaymentIntent
		if err := json.Unmarshal(event.Data.Raw, &paymentIntent); err != nil {
			utils.LogErrorDiscord("StripeWebhook", err, nil)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		if err := handlePaymentFailed(&paymentIntent); err != nil {
			utils.LogErrorDiscord("StripeWebhook", err, nil)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

	case "customer.subscription.updated": //handle activate subscription, handle cancellation at end of month flag

		var subscription stripe.Subscription
		if err := json.Unmarshal(event.Data.Raw, &subscription); err != nil {
			utils.LogErrorDiscord("StripeWebhook", err, nil)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		err := handleUpdateSubscription(ctx, &subscription) // will return if isActive is false
		if err != nil {
			utils.LogErrorDiscord("StripeWebhook", err, nil)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

	case "invoice.paid": // listen for monthly renewal

		var invoice stripe.Invoice
		if err := json.Unmarshal(event.Data.Raw, &invoice); err != nil {
			utils.LogErrorDiscord("StripeWebhook", err, nil)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		if err := handleRenewSubscription(ctx, &invoice); err != nil {
			utils.LogErrorDiscord("StripeWebhook", err, nil)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

	case "customer.subscription.deleted": // listen for deletion cancellation

		var subscription stripe.Subscription
		if err := json.Unmarshal(event.Data.Raw, &subscription); err != nil {
			utils.LogErrorDiscord("StripeWebhook", err, nil)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		err := handleDeleteSubscription(ctx, &subscription)
		if err != nil {
			utils.LogErrorDiscord("StripeWebhook", err, nil)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

	default:
		utils.LogErrorDiscord("StripeWebhook", errors.New("unhandled event type"), nil)
	}

	w.WriteHeader(http.StatusOK)

}

/* payment intent events */

func handlePaymentSucceeded(ctx context.Context, paymentIntent *stripe.PaymentIntent) error {

	t, ok := paymentIntent.Metadata["type"]
	if !ok || t != "plot-purchase" {
		return nil //as of now, there are no other payment events that need handling
	}

	uidString := paymentIntent.Metadata["uid"]

	// get list of plotIds
	plotIds := make([]string, 0)
	for i := 0; ; i++ {
		key := fmt.Sprintf("i%d", i)
		if val, ok := paymentIntent.Metadata[key]; ok {
			plotIds = append(plotIds, val)
		} else {
			break
		}
	}

	// check & refresh lock
	_, lockFailed, err := plotutils.LockMany(ctx, plotIds, uidString)
	defer plotutils.UnlockMany(plotIds, uidString)

	// lock acquisition should never fail here, but just in case, return and handle manually
	if err != nil || len(lockFailed) != 0 {
		return fmt.Errorf("in handlePaymentSucceeded, SEVERE ERROR lock acquisition failed:\n%w", err)
	}

	// verify that all plots do not already exist

	g, ctxw := errgroup.WithContext(ctx)
	for _, _plotId := range plotIds {

		plotId := _plotId
		g.Go(func() error {

			exists, err := utils.HasObjectR2(ctxw, "plots", plotId+".dat")
			if err != nil {
				return fmt.Errorf("in handlePaymentSucceeded, SEVERE ERROR checking for plot:\n%w", err)
			}

			if exists {
				return fmt.Errorf("in handlePaymentSucceeded, SEVERE ERROR plot already exists:\n%w", err)
			}

			return nil

		})

	}
	// if one or more already exists, return
	if err := g.Wait(); err != nil {
		return err
	}

	usersCollection := utils.MongoDB.Collection("users")

	// verify that none of the plotIds being inserted already exist
	// if this operation is successful, all of the plot ids will be added.
	// uid, _ := bson.ObjectIDFromHex(uidString)
	var user schemas.User
	res := usersCollection.FindOneAndUpdate(ctx,
		bson.M{
			"stripeCustomer": paymentIntent.Customer.ID,
			"plotIds": bson.M{
				"$not": bson.M{
					"$elemMatch": bson.M{
						"$in": plotIds,
					},
				},
			},
		},
		bson.M{
			"$push": bson.M{
				"plotIds": bson.M{
					"$each": plotIds,
				},
			},
		},
		options.FindOneAndUpdate().SetReturnDocument(options.After),
	)
	err = res.Decode(&user)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return fmt.Errorf("in handlePaymentSucceeded, SEVERE ERROR plot ids already in user's list:\n%w", err)
	} else if err != nil {
		return fmt.Errorf("in handlePaymentSucceeded, SEVERE ERROR mongo error:\n%w", err)
	}

	// create default plots
	g, ctxw = errgroup.WithContext(ctx)
	for _, _plotId := range plotIds {

		plotId, _ := plotutils.PlotIdFromHexString(_plotId)
		g.Go(func() error {

			// create plot with default data
			err = plotutils.SetDefaultPlot(ctxw, plotId, &user)
			if err != nil {
				return fmt.Errorf("in handlePaymentSucceeded, error setting default plot data:\n%w", err)
			}

			// remove plotId from available plots
			depth := plotId.Depth()
			err = utils.RedisCli.SRem(ctxw, fmt.Sprintf("openplots:%d", depth), plotId.ToString()).Err()
			if err != nil {
				return fmt.Errorf("in handlePaymentSucceeded, error removing available plot id:\n%w", err)
			}

			// add plot's children (if it has any) to available plots
			if depth < utils.MaxDepth {

				childIds := make([]any, utils.SubplotCount)
				for i := range utils.SubplotCount {
					childId := plotutils.CreateSubplotId(plotId, uint64(i+1))
					childIds[i] = childId.ToString()
				}

				err = utils.RedisCli.SAdd(ctxw, fmt.Sprintf("openplots:%d", depth+1), childIds...).Err()
				if err != nil {
					return fmt.Errorf("in handlePaymentSucceeded, error adding child plot ids to available:\n%w", err)
				}

			}

			return nil

		})

	}

	if err := g.Wait(); err != nil {
		return err
	}

	return nil

}

func handlePaymentFailed(paymentIntent *stripe.PaymentIntent) error {

	t, ok := paymentIntent.Metadata["type"]
	if !ok || t != "plot-purchase" {
		return nil
	}
	uid := paymentIntent.Metadata["uid"]

	// get list of plotIds
	plotIds := make([]string, 0)
	for i := 0; ; i++ {
		key := fmt.Sprintf("i%d", i)
		if val, ok := paymentIntent.Metadata[key]; ok {
			plotIds = append(plotIds, val)
		} else {
			break
		}
	}

	// clear locks
	return plotutils.UnlockMany(plotIds, uid)

}

/* subscription events */

func handleUpdateSubscription(ctx context.Context, sub *stripe.Subscription) error {

	// as of now, ignore update events that are not status active
	if sub.Status != "active" {
		return nil
	}

	usersCollection := utils.MongoDB.Collection("users")

	// update user in mongo
	var user schemas.User
	err := usersCollection.FindOneAndUpdate(ctx,
		bson.M{
			"stripeCustomer": sub.Customer.ID,
		},
		bson.M{
			"$set": bson.M{
				"subscription.isActive":       true,
				"subscription.isCanceled":     sub.CancelAtPeriodEnd,
				"subscription.productId":      "prod_S7k4E96oClYk9l",
				"subscription.subscriptionId": sub.ID,
			},
		},
		options.FindOneAndUpdate().SetReturnDocument(options.Before),
	).Decode(&user)
	if err != nil {
		return fmt.Errorf("in handleSubscribe, SEVERE ERROR updating user by customer id failed:\n%w", err)
	}

	// if user's subscription was already active (i.e. the update was for cancellation or renewal),
	// no need to flag plots for verified tag update
	if user.Subscription.IsActive || len(user.PlotIds) == 0 {
		return nil
	}

	// flag user's plots for update so that verification badges show
	g, ctxw := errgroup.WithContext(ctx)
	for _, id := range user.PlotIds {

		plotId, _ := plotutils.PlotIdFromHexString(id)
		g.Go(func() error {

			// update verified metadata
			err := utils.UpdateMetadataR2(ctxw, "plots", plotId.ToString()+".dat", map[string]string{"verified": strconv.FormatBool(true)})
			if err != nil {
				return fmt.Errorf("in handleSubscribe, error updating plot verified metadata:\n%w", err)
			}

			// flag for update
			err = plotutils.FlagPlotForUpdate(ctxw, plotId)
			if err != nil {
				return fmt.Errorf("in handleSubscribe, error flagging plot for update:\n%w", err)
			}

			return nil

		})

	}

	if err := g.Wait(); err != nil {
		return err
	}

	return nil

}

func handleRenewSubscription(ctx context.Context, invoice *stripe.Invoice) error {

	usersCollection := utils.MongoDB.Collection("users")

	// give a bonus plot credit if user is within their first 6 payment cycles
	_, err := usersCollection.UpdateOne(
		ctx,
		bson.M{
			"stripeCustomer":             invoice.Customer.ID,
			"subscription.recurredCount": bson.M{"$lt": 6},
		},
		bson.M{
			"$inc": bson.M{
				"plotCredits": 1, // bonus credit
			},
		},
	)
	if err != nil {
		return fmt.Errorf("failed to award bonus plot credit: %w", err)
	}

	// increment the recurredCount by 1 for every renewal
	_, err = usersCollection.UpdateOne(
		ctx,
		bson.M{
			"stripeCustomer": invoice.Customer.ID,
		},
		bson.M{
			"$inc": bson.M{
				"subscription.recurredCount": 1,
			},
		},
	)
	if err != nil {
		return fmt.Errorf("failed to increment subscription.recurredCount: %w", err)
	}

	return nil

}

func handleDeleteSubscription(ctx context.Context, sub *stripe.Subscription) error {

	usersCollection := utils.MongoDB.Collection("users")

	// update user in mongo
	var user schemas.User
	err := usersCollection.FindOneAndUpdate(ctx, bson.M{
		"stripeCustomer": sub.Customer.ID,
	}, bson.M{
		"$set": bson.M{
			"subscription.isActive":       false,
			"subscription.isCanceled":     true,
			"subscription.productId":      "",
			"subscription.subscriptionId": "",
		},
	}).Decode(&user)
	if err != nil {
		return fmt.Errorf("in handleDeleteSubscription, SEVERE ERROR updating user by customer id failed:\n%w", err)
	}

	if len(user.PlotIds) == 0 {
		return nil
	}

	// flag user's plots for update so that subscriber benefits are removed
	g, ctxw := errgroup.WithContext(ctx)
	for _, id := range user.PlotIds {

		plotId, _ := plotutils.PlotIdFromHexString(id)
		g.Go(func() error {

			// update verified metadata
			err := utils.UpdateMetadataR2(ctxw, "plots", plotId.ToString()+".dat", map[string]string{"verified": strconv.FormatBool(false)})
			if err != nil {
				return fmt.Errorf("in handleDeleteSubscription, error updating plot verified metadata:\n%w", err)
			}

			// flag for update
			err = plotutils.FlagPlotForUpdate(ctxw, plotId)
			if err != nil {
				return fmt.Errorf("in handleDeleteSubscription, error flagging plot for update:\n%w", err)
			}

			return nil

		})

	}

	if err := g.Wait(); err != nil {
		return err
	}

	return nil

}
