package payment

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"trraformapi/utils"
	"trraformapi/utils/schemas"

	"github.com/stripe/stripe-go/v82"
	"github.com/stripe/stripe-go/v82/subscription"
	"go.mongodb.org/mongo-driver/bson"
)

func UpdateSubscription(w http.ResponseWriter, r *http.Request) {

	// validate request
	ctx := r.Context()

	authToken, err := utils.ValidateAuthToken(r)
	if err != nil {
		utils.MakeAPIResponse(w, r, http.StatusForbidden, nil, "Invalid token", true)
		return
	}
	uid, _ := authToken.GetUidObjectId()

	var requestData struct {
		Update string `json:"update"`
	}

	if err := json.NewDecoder(r.Body).Decode(&requestData); err != nil {
		utils.MakeAPIResponse(w, r, http.StatusBadRequest, nil, "Invalid request body", true)
		return
	}
	defer r.Body.Close()

	var cancel bool
	if requestData.Update == "cancel" {
		cancel = true
	} else if requestData.Update == "renew" {
		cancel = false
	} else {
		utils.MakeAPIResponse(w, r, http.StatusBadRequest, nil, "Unknown update method", true)
		return
	}

	usersCollection := utils.MongoDB.Collection("users")

	// get user data
	var user schemas.User
	err = usersCollection.FindOne(ctx, bson.M{"_id": uid}).Decode(&user)
	if err != nil {
		if !errors.Is(err, context.Canceled) {
			utils.LogErrorDiscord("CancelSubscription", err, nil)
		}
		utils.MakeAPIResponse(w, r, http.StatusInternalServerError, nil, "Internal server error", true)
		return
	}

	params := &stripe.SubscriptionParams{CancelAtPeriodEnd: stripe.Bool(cancel)}
	_, err = subscription.Update(user.Subscription.SubscriptionId, params)
	if err != nil {
		if !errors.Is(err, context.Canceled) {
			utils.LogErrorDiscord("CancelSubscription", err, nil)
		}
		utils.MakeAPIResponse(w, r, http.StatusInternalServerError, nil, "Internal server error", true)
		return
	}

	utils.MakeAPIResponse(w, r, http.StatusOK, nil, "Success", false)

}
