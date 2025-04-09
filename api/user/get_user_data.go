package user

import (
	"context"
	"errors"
	"net/http"
	"trraformapi/utils"
	"trraformapi/utils/schemas"

	"go.mongodb.org/mongo-driver/v2/bson"
)

func GetUserData(w http.ResponseWriter, r *http.Request) {

	ctx := r.Context()

	var responseData struct {
		Token       string            `json:"token"`
		Username    string            `json:"username"`
		Subscribed  bool              `json:"subscribed"`
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
	if err != nil {
		if !errors.Is(err, context.Canceled) {
			utils.LogErrorDiscord("GetUserData", err, token)
		}
		utils.MakeAPIResponse(w, r, http.StatusInternalServerError, nil, "Internal server error", true)
		return
	}

	responseData.Username = user.Username
	responseData.Subscribed = user.Subscribed
	responseData.PlotCredits = user.PlotCredits
	responseData.PlotIds = user.PlotIds
	responseData.Offenses = user.Offenses

	utils.MakeAPIResponse(w, r, http.StatusOK, &responseData, "Success", false)

}
