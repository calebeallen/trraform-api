package user

import (
	"context"
	"errors"
	"net/http"
	"trraformapi/utils"
	"trraformapi/utils/schemas"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
)

func GetUserData(w http.ResponseWriter, r *http.Request) {

	ctx := r.Context()

	var responseData struct {
		Token       string            `json:"token"`
		Username    string            `json:"username"`
		SubActive   bool              `json:"subActive"`
		SubCanceled bool              `json:"subCanceled"`
		PlotCredits int               `json:"plotCredits"`
		PlotIds     []string          `json:"plotIds"`
		Offenses    []schemas.Offense `json:"offenses"`
	}

	//validate token
	token, err := utils.ValidateAuthToken(r)
	if err != nil {
		utils.MakeAPIResponse(w, r, http.StatusForbidden, nil, "Invalid token", true)
		return
	}

	//refresh token if expiring soon
	token.Refresh()
	responseData.Token, _ = token.Sign()

	uid, err := token.GetUidObjectId()
	if err != nil {
		utils.LogErrorDiscord("GetUserData", err, token)
		utils.MakeAPIResponse(w, r, http.StatusInternalServerError, nil, "Internal server error", true)
		return
	}

	usersCollection := utils.MongoDB.Collection("users")

	// get user data
	query := usersCollection.FindOne(ctx, bson.M{"_id": uid})
	var user schemas.User
	err = query.Decode(&user)
	if errors.Is(err, mongo.ErrNoDocuments) {
		utils.MakeAPIResponse(w, r, http.StatusNotFound, nil, "User not found", true)
		return
	} else if err != nil {
		if !errors.Is(err, context.Canceled) {
			utils.LogErrorDiscord("GetUserData", err, token)
		}
		utils.MakeAPIResponse(w, r, http.StatusInternalServerError, nil, "Internal server error", true)
		return
	}

	responseData.Username = user.Username
	responseData.SubActive = user.Subscription.IsActive
	responseData.SubCanceled = user.Subscription.IsCanceled
	responseData.PlotCredits = user.PlotCredits
	responseData.PlotIds = user.PlotIds
	responseData.Offenses = user.Offenses

	utils.MakeAPIResponse(w, r, http.StatusOK, &responseData, "Success", false)

}
