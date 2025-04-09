package auth

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"trraformapi/utils"
	"trraformapi/utils/schemas"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
)

func SendVerificationEmail(w http.ResponseWriter, r *http.Request) {

	ctx := r.Context()

	var requestData struct {
		Email string `json:"email" validate:"required,email"`
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

	// avoid matching conflict
	requestData.Email = strings.ToLower(requestData.Email)

	usersCollection := utils.MongoDB.Collection("users")

	// check that a user with this email exists
	var user schemas.User
	res := usersCollection.FindOne(ctx, bson.M{"email": requestData.Email})
	err := res.Decode(&user)
	if errors.Is(err, mongo.ErrNoDocuments) { //doesn't exist, return
		utils.MakeAPIResponse(w, r, http.StatusNotFound, nil, "User does not exist", true)
		return
	} else if err != nil {
		if !errors.Is(err, context.Canceled) {
			utils.LogErrorDiscord("SendVerificationEmail", err, &requestData)
		}
		utils.MakeAPIResponse(w, r, http.StatusInternalServerError, nil, "Internal server error", true)
		return
	}

	// return if user is already verified
	if user.EmailVerified {
		utils.MakeAPIResponse(w, r, http.StatusBadRequest, nil, "User already verified", true)
		return
	}

	// resend email
	emailStatus, err := utils.SendVerificationEmail(ctx, requestData.Email)
	if err != nil {
		if !errors.Is(err, context.Canceled) {
			utils.LogErrorDiscord("SendVerificationEmail", err, &requestData)
		}
		utils.MakeAPIResponse(w, r, http.StatusInternalServerError, nil, "Internal server error", true)
		return
	}

	// respond with email status
	utils.MakeAPIResponse(w, r, http.StatusOK, emailStatus, "Success", false)

}
