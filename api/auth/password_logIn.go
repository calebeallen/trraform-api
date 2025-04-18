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
	"golang.org/x/crypto/bcrypt"
)

func PasswordLogIn(w http.ResponseWriter, r *http.Request) {

	ctx := r.Context()

	var requestData struct {
		Email    string `json:"email" validate:"required,email"`
		Password string `json:"password" validate:"required,password"`
	}

	type responseData struct {
		CredentialError   bool   `json:"credentialError"`
		NeedsVerification bool   `json:"needsVerification"`
		Token             string `json:"token"`
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

	requestData.Email = strings.ToLower(requestData.Email)

	usersCollection := utils.MongoDB.Collection("users")

	// find user
	var user schemas.User
	err := usersCollection.FindOne(ctx, bson.M{"email": strings.ToLower(requestData.Email)}).Decode(&user)
	if errors.Is(err, mongo.ErrNoDocuments) || user.PassHash == "" { //doesn't exist, return
		resData := responseData{
			CredentialError: true,
		}
		utils.MakeAPIResponse(w, r, http.StatusForbidden, &resData, "Credential error", true)
		return
	} else if err != nil {
		if !errors.Is(err, context.Canceled) {
			requestData.Password = ""
			utils.LogErrorDiscord("PasswordLogIn", err, &requestData)
		}
		utils.MakeAPIResponse(w, r, http.StatusInternalServerError, nil, "Internal server error", true)
		return
	}

	// check password
	hash := []byte(user.PassHash)
	if err := bcrypt.CompareHashAndPassword(hash, []byte(requestData.Password)); err != nil {
		resData := responseData{
			CredentialError: true,
		}
		utils.MakeAPIResponse(w, r, http.StatusForbidden, &resData, "Credential error", true)
		return
	}
	requestData.Password = "" // sanitize logs

	// check email verification
	if !user.EmailVerified {
		resData := responseData{
			NeedsVerification: true,
		}
		utils.MakeAPIResponse(w, r, http.StatusForbidden, &resData, "Unverified email", true)
		return
	}

	// issue jwt
	authToken := utils.CreateNewAuthToken(user.Id)
	authTokenStr, err := authToken.Sign()
	if err != nil {
		utils.LogErrorDiscord("PasswordLogIn", err, &requestData)
		utils.MakeAPIResponse(w, r, http.StatusInternalServerError, nil, "Internal server error", true)
		return
	}

	resData := responseData{
		Token: authTokenStr,
	}
	utils.MakeAPIResponse(w, r, http.StatusOK, &resData, "Success", false)

}
