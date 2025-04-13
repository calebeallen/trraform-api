package payment

import (
	"context"
	"errors"
	"net/http"
	"trraformapi/utils"
	"trraformapi/utils/schemas"

	"github.com/stripe/stripe-go/v82"
	"github.com/stripe/stripe-go/v82/subscription"
	"go.mongodb.org/mongo-driver/bson"
)

func CreateSubscription(w http.ResponseWriter, r *http.Request) {

	// validate request
	ctx := r.Context()

	authToken, err := utils.ValidateAuthToken(r)
	if err != nil {
		utils.MakeAPIResponse(w, r, http.StatusForbidden, nil, "Invalid token", true)
		return
	}
	uid, _ := authToken.GetUidObjectId()

	var responseData struct {
		Type         string `json:"type"`
		ClientSecret string `json:"clientSecret"`
	}

	usersCollection := utils.MongoDB.Collection("users")

	// get user
	var user schemas.User
	err = usersCollection.FindOne(ctx, bson.M{"_id": uid}).Decode(&user)
	if err != nil {
		if !errors.Is(err, context.Canceled) {
			utils.LogErrorDiscord("CreateSubscription", err, nil)
		}
		utils.MakeAPIResponse(w, r, http.StatusInternalServerError, nil, "Internal server error", true)
		return
	}

	// save user's card
	paymentSettings := &stripe.SubscriptionPaymentSettingsParams{
		SaveDefaultPaymentMethod: stripe.String("on_subscription"),
	}

	// create subscription
	params := &stripe.SubscriptionParams{
		Customer: stripe.String(user.StripeCustomer),
		Items: []*stripe.SubscriptionItemsParams{
			{
				Price: stripe.String("prod_S7k4E96oClYk9l"),
			},
		},
		PaymentSettings: paymentSettings,
		PaymentBehavior: stripe.String("default_incomplete"),
	}
	params.AddExpand("latest_invoice.confirmation_secret")
	params.AddMetadata("type", "sub")
	s, err := subscription.New(params)

	if err != nil {
		utils.LogErrorDiscord("CreateSubscription", err, nil)
		utils.MakeAPIResponse(w, r, http.StatusInternalServerError, nil, "Internal server error", true)
		return
	}

	responseData.ClientSecret = s.LatestInvoice.ConfirmationSecret.ClientSecret
	utils.MakeAPIResponse(w, r, http.StatusOK, &responseData, "Success", false)

}
