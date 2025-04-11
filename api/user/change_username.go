package user

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"trraformapi/utils"
	"trraformapi/utils/schemas"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
)

func ChangeUsername(w http.ResponseWriter, r *http.Request) {

	ctx := r.Context()

	//validate token
	token, err := utils.ValidateAuthToken(r)
	if err != nil {
		utils.MakeAPIResponse(w, r, http.StatusForbidden, nil, "Invalid token", true)
		return
	}
	uid, _ := token.GetUidObjectId()

	var requestData struct {
		NewUsername string `json:"newUsername" validate:"required,username"`
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

	// check that username doesn't exist
	err = usersCollection.FindOne(ctx, bson.M{"username": requestData.NewUsername}).Decode(&schemas.User{})
	if !errors.Is(err, mongo.ErrNoDocuments) {

		if err != nil {
			if !errors.Is(err, context.Canceled) {
				utils.LogErrorDiscord("ChangeUsername", err, &requestData)
			}
			utils.MakeAPIResponse(w, r, http.StatusInternalServerError, nil, "Internal server error", true)
			return
		}

		responseData := struct {
			UsernameTaken bool `json:"usernameTaken"`
		}{UsernameTaken: true}
		utils.MakeAPIResponse(w, r, http.StatusConflict, &responseData, "Username already taken", true)
		return

	}

	// set new username
	_, err = usersCollection.UpdateOne(
		ctx,
		bson.M{"_id": uid},
		bson.M{"$set": bson.M{"username": requestData.NewUsername}},
	)
	if err != nil {
		if !errors.Is(err, context.Canceled) {
			utils.LogErrorDiscord("ChangeUsername", err, &requestData)
		}
		utils.MakeAPIResponse(w, r, http.StatusInternalServerError, nil, "Internal server error", true)
		return
	}

	utils.MakeAPIResponse(w, r, http.StatusOK, nil, "Success", false)

}
