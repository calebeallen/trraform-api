package payment

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"trraformapi/utils"
	plotutils "trraformapi/utils/plot_utils"
	"trraformapi/utils/schemas"

	"github.com/stripe/stripe-go/v82"
	"github.com/stripe/stripe-go/v82/paymentintent"
	"go.mongodb.org/mongo-driver/bson"
)

func CreatePaymentIntent(w http.ResponseWriter, r *http.Request) {

	// validate request
	ctx := r.Context()

	authToken, err := utils.ValidateAuthToken(r)
	if err != nil {
		utils.MakeAPIResponse(w, r, http.StatusForbidden, nil, "Invalid token", true)
		return
	}
	uid, _ := authToken.GetUidObjectId()
	uidString := uid.Hex()

	var requestData struct {
		PlotIds []string `json:"plotIds" validate:"required,max=30,dive,plotid"`
	}

	var responseData struct {
		ClientSecret string   `json:"clientSecret"`
		Conflicts    []string `json:"conflicts"`
	}

	// validate request body
	if err := json.NewDecoder(r.Body).Decode(&requestData); err != nil {
		utils.MakeAPIResponse(w, r, http.StatusBadRequest, nil, "Invalid request body", true)
		return
	}
	defer r.Body.Close()
	if err := utils.Validate.Struct(&requestData); err != nil {
		utils.MakeAPIResponse(w, r, http.StatusBadRequest, nil, "Invalid request body", true)
		return
	}

	usersCollection := utils.MongoDB.Collection("users")

	// get user data
	var user schemas.User
	err = usersCollection.FindOne(ctx, bson.M{"_id": uid}).Decode(&user)
	if err != nil {
		if !errors.Is(err, context.Canceled) {
			utils.LogErrorDiscord("CreatePaymentIntent", err, nil)
		}
		utils.MakeAPIResponse(w, r, http.StatusInternalServerError, nil, "Internal server error", true)
		return
	}

	plotIds := make([]*plotutils.PlotId, len(requestData.PlotIds))
	var total int64 = 0
	for i := range plotIds {
		plotId, _ := plotutils.PlotIdFromHexString(requestData.PlotIds[i])
		requestData.PlotIds[i] = plotId.ToString() //normailze just incase :)
		plotIds[i] = plotId
		total += utils.Price[plotId.Depth()]
	}

	// lock plots to prevent duplicate claims
	lockAcquired, conflicts, err := plotutils.LockMany(ctx, requestData.PlotIds, uidString)
	if err != nil {
		if !errors.Is(err, context.Canceled) {
			utils.LogErrorDiscord("InitPaidClaim", err, &requestData)
		}
		utils.MakeAPIResponse(w, r, http.StatusInternalServerError, nil, "Internal server error", true)
		plotutils.UnlockMany(requestData.PlotIds, uidString)
		return
	}

	// verify that each plot doesn't exist (lock could be expired but the plot was already claimed)
	for _, plotId := range lockAcquired {

		claimed, err := utils.HasObjectR2(ctx, "plots", plotId+".dat")
		if err != nil {
			if !errors.Is(err, context.Canceled) {
				utils.LogErrorDiscord("InitPaidClaim", err, &requestData)
			}
			utils.MakeAPIResponse(w, r, http.StatusInternalServerError, nil, "Internal server error", true)
			plotutils.UnlockMany(requestData.PlotIds, uidString)
			return
		}

		if claimed {
			conflicts = append(conflicts, plotId)
		}

	}

	// if any conflicts, revert
	if len(conflicts) != 0 {
		responseData.Conflicts = conflicts
		utils.MakeAPIResponse(w, r, http.StatusConflict, &responseData, "Claim conflicts", true)
		plotutils.UnlockMany(requestData.PlotIds, uidString)
		return
	}

	// create payment intent
	params := stripe.PaymentIntentParams{
		Customer: stripe.String(user.StripeCustomer),
		Amount:   stripe.Int64(total),
		Currency: stripe.String(string(stripe.CurrencyUSD)),
	}
	params.AddMetadata("uid", uidString)

	// add plots to metadata
	for i, plotId := range plotIds {
		params.AddMetadata(fmt.Sprintf("i%d", i), plotId.ToString())
	}

	intent, err := paymentintent.New(&params)
	if err != nil {
		utils.LogErrorDiscord("InitPaidClaim", err, &requestData)
		utils.MakeAPIResponse(w, r, http.StatusInternalServerError, nil, "Internal server error", true)
		plotutils.UnlockMany(requestData.PlotIds, uidString)
		return
	}

	// return client secret
	responseData.ClientSecret = intent.ClientSecret
	responseData.Conflicts = []string{}
	utils.MakeAPIResponse(w, r, http.StatusOK, &responseData, "Success", false)

}
