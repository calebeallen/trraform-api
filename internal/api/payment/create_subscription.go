package payment

import (
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"time"
	"trraformapi/internal/api"
	"trraformapi/pkg/config"
	"trraformapi/pkg/schemas"

	"github.com/stripe/stripe-go/v82"
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

	// block dual subs
	if user.Subscription.IsActive {
		resParams.ResData = &struct {
			ActiveSub bool `json:"activeSub"`
		}{ActiveSub: true}
		resParams.Code = http.StatusForbidden
		h.Res(resParams)
		return
	}

	// create stripe customer if needed
	var stripeCustomerId string
	if user.StripeCustomer == "" {
		cus, err := h.StripeCli.V1Customers.Create(ctx, &stripe.CustomerCreateParams{
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

	// create idempotency key
	base := uidStr + ":sub:" + config.PRICE_ID_SUBSCRIPTION
	sum := sha256.Sum256([]byte(base))
	idemKey := hex.EncodeToString(sum[:])

	// metadata
	metadata := map[string]string{
		"uid": uidStr,
	}

	// create stripe session
	subscriptionParams := &stripe.CheckoutSessionCreateParams{
		Mode:              stripe.String(string(stripe.CheckoutSessionModeSubscription)),
		SuccessURL:        stripe.String("https://yourapp.com/checkout/success?session_id={CHECKOUT_SESSION_ID}"),
		CancelURL:         stripe.String("https://yourapp.com/checkout/cancel"),
		Customer:          stripe.String(stripeCustomerId),
		ClientReferenceID: stripe.String(uidStr),
		ExpiresAt:         stripe.Int64(time.Now().Add(config.CHECKOUT_SESSION_DURATION).Unix()),

		// tax
		BillingAddressCollection: stripe.String("auto"),
		AutomaticTax: &stripe.CheckoutSessionCreateAutomaticTaxParams{
			Enabled: stripe.Bool(true),
		},
		CustomerUpdate: &stripe.CheckoutSessionCreateCustomerUpdateParams{
			Address: stripe.String("auto"),
		},

		LineItems: []*stripe.CheckoutSessionCreateLineItemParams{
			{
				Price:    stripe.String(config.PRICE_ID_SUBSCRIPTION),
				Quantity: stripe.Int64(1),
			},
		},

		// metadata
		Metadata: metadata,
		SubscriptionData: &stripe.CheckoutSessionCreateSubscriptionDataParams{
			Metadata: metadata,
		},
		PaymentIntentData: &stripe.CheckoutSessionCreatePaymentIntentDataParams{
			Metadata: metadata,
		},
	}
	subscriptionParams.SetIdempotencyKey(idemKey)

	subscriptionSession, err := h.StripeCli.V1CheckoutSessions.Create(ctx, subscriptionParams)
	if err != nil {
		resParams.Code = http.StatusInternalServerError
		resParams.Err = err
		h.Res(resParams)
		return
	}

	resParams.ResData = &struct {
		StripeSession string `json:"stripeSession"`
	}{StripeSession: subscriptionSession.ID}
	resParams.Code = http.StatusOK
	h.Res(resParams)

}
