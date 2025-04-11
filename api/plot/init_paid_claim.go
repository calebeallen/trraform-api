package plot

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"time"
	"trraformapi/utils"
	plotutils "trraformapi/utils/plot_utils"
	"trraformapi/utils/schemas"

	"github.com/stripe/stripe-go/v82"
	"github.com/stripe/stripe-go/v82/paymentintent"
	"go.mongodb.org/mongo-driver/bson"
)

func InitPaidClaim(w http.ResponseWriter, r *http.Request) {

	// validate request
	ctx := r.Context()

	authToken, err := utils.ValidateAuthToken(r)
	if err != nil {
		utils.MakeAPIResponse(w, r, http.StatusForbidden, nil, "Invalid token", true)
		return
	}
	uid, _ := authToken.GetUidObjectId()

	var requestData struct {
		PlotId string `json:"plotId" validate:"required,plotid"`
	}

	var responseData struct {
		ClientSecret string    `json:"clientSecret"`
		Expiration   time.Time `json:"expiration"`
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

	plotId, _ := plotutils.PlotIdFromHexString(requestData.PlotId)
	plotIdStr := plotId.ToString()
	uidString := uid.Hex()

	usersCollection := utils.MongoDB.Collection("users")

	// get user data
	var user schemas.User
	err = usersCollection.FindOne(ctx, bson.M{"_id": uid}).Decode(&user)
	if err != nil {

	}

	/* 	calculate lock expiration time BEFORE actually setting the
	lock so that the front end has a value < the actual lock exipration.
	Set 10 mins in the future. Actual lock last for 15 mins. 5 min buffer
	for the time it could take for stripe to process the transaction.
	*/
	responseData.Expiration = time.Now().UTC().Add(time.Minute * 10)

	// lock plot to prevent duplicate claims
	lockAquired, err := plotutils.LockPlot(ctx, plotIdStr, uidString)
	if err != nil {
		if !errors.Is(err, context.Canceled) {
			utils.LogErrorDiscord("InitPaidClaim", err, &requestData)
		}
		utils.MakeAPIResponse(w, r, http.StatusInternalServerError, nil, "Internal server error", true)
		return
	}

	// if already locked, return
	if !lockAquired {
		utils.MakeAPIResponse(w, r, http.StatusForbidden, nil, "Plot locked", true)
		return
	}

	// verify that plot isn't claimed
	if utils.HasObjectR2("plots", plotIdStr+".dat", ctx) {
		utils.MakeAPIResponse(w, r, http.StatusForbidden, nil, "Plot already claimed", true)
		plotutils.UnlockPlot(plotIdStr, uidString)
		return
	}

	// create payment intent
	params := stripe.PaymentIntentParams{
		Customer:         stripe.String(user.StripeCustomer),
		Amount:           stripe.Int64(2000),
		Currency:         stripe.String("usd"),
		SetupFutureUsage: stripe.String("off_session"), //save card
	}
	params.AddMetadata("uid", uidString)
	params.AddMetadata("plotId", plotIdStr)
	intent, err := paymentintent.New(&params)
	if err != nil {
		utils.LogErrorDiscord("InitPaidClaim", err, &requestData)
		utils.MakeAPIResponse(w, r, http.StatusInternalServerError, nil, "Internal server error", true)
		plotutils.UnlockPlot(plotIdStr, uidString)
		return
	}

	// return client secret
	responseData.ClientSecret = intent.ClientSecret
	utils.MakeAPIResponse(w, r, http.StatusOK, &responseData, "Success", false)

}
