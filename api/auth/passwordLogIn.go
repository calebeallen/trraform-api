package auth

import (
	"encoding/json"
	"log"
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

	var responseData struct {
		UserExists      bool   `json:"userExists"`
		PasswordCorrect bool   `json:"passwordCorrect"`
		EmailVerified   bool   `json:"emailVerified"`
		Token           string `json:"token"`
	}
	responseData.UserExists = false
	responseData.PasswordCorrect = false
	responseData.EmailVerified = false

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
	if err == mongo.ErrNoDocuments { //doesn't exist, return
		utils.MakeAPIResponse(w, r, http.StatusNotFound, &responseData, "User does not exist", true)
		return
	} else if err != nil {
		log.Println(err)
		requestData.Password = ""
		utils.LogErrorDiscord("PasswordLogIn", err, &requestData)
		utils.MakeAPIResponse(w, r, http.StatusInternalServerError, nil, "Internal server error", true)
		return
	}
	responseData.UserExists = true

	// check password
	hash := []byte(user.PassHash)
	if err := bcrypt.CompareHashAndPassword(hash, []byte(requestData.Password)); err != nil {
		utils.MakeAPIResponse(w, r, http.StatusForbidden, &responseData, "Incorrect password", true)
		return
	}
	responseData.PasswordCorrect = true

	// check email verification
	if !user.EmailVerified {
		utils.MakeAPIResponse(w, r, http.StatusForbidden, &responseData, "Unverified email", true)
		return
	}
	responseData.EmailVerified = true

	// issue jwt
	authToken := utils.CreateNewAuthToken(user.Id)
	authTokenStr, err := authToken.Sign()
	if err != nil {
		log.Println(err)
		utils.LogErrorDiscord("PasswordLogIn", err, &requestData)
		utils.MakeAPIResponse(w, r, http.StatusInternalServerError, nil, "Internal server error", true)
		return
	}
	responseData.Token = authTokenStr

	utils.MakeAPIResponse(w, r, http.StatusOK, &responseData, "Success", false)

}
