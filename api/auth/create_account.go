package auth

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"
	"trraformapi/utils"
	"trraformapi/utils/schemas"

	"go.mongodb.org/mongo-driver/v2/bson"
	"golang.org/x/crypto/bcrypt"
)

func CreateAccount(w http.ResponseWriter, r *http.Request) {

	ctx := r.Context()

	var requestData struct {
		Username string `json:"username" validate:"required,username"`
		Email    string `json:"email" validate:"required,email"`
		Password string `json:"password" validate:"required,password"`
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

	// hash password and clear it from request data to sanitize logs
	passHash, err := bcrypt.GenerateFromPassword([]byte(requestData.Password), bcrypt.DefaultCost)
	requestData.Password = ""
	if err != nil {
		utils.LogErrorDiscord("CreateAccount", err, &requestData)
		utils.MakeAPIResponse(w, r, http.StatusInternalServerError, nil, "Internal server error", true)
		return
	}

	// avoid matching issues
	requestData.Email = strings.ToLower(requestData.Email)

	usersCollection := utils.MongoDB.Collection("users")

	// check if username or email already exists
	cursor, err := usersCollection.Find(ctx, bson.M{
		"$or": bson.A{
			bson.M{"username": requestData.Username},
			bson.M{"email": requestData.Email},
		},
	})
	if err != nil {
		if !errors.Is(err, context.Canceled) {
			utils.LogErrorDiscord("CreateAccount", err, &requestData)
		}
		utils.MakeAPIResponse(w, r, http.StatusInternalServerError, nil, "Internal server error", true)
		return
	}
	defer cursor.Close(ctx)

	var users []*schemas.User
	if err := cursor.All(ctx, &users); err != nil {
		if !errors.Is(err, context.Canceled) {
			utils.LogErrorDiscord("CreateAccount", err, &requestData)
		}
		utils.MakeAPIResponse(w, r, http.StatusInternalServerError, nil, "Internal server error", true)
		return
	}

	// if username and/or email taken return
	if len(users) != 0 {

		var exist struct {
			Username bool `json:"usernameExist"`
			Email    bool `json:"emailExist"`
		}

		for i := range users {

			user := users[i]
			if user.Username == requestData.Username {
				exist.Username = true
			}
			if user.Email == requestData.Email {
				exist.Email = true
			}

		}

		utils.MakeAPIResponse(w, r, http.StatusConflict, exist, "Credentials already exist", true)
		return

	}

	// create default entry in mongo
	newUser := &schemas.User{
		Ctime:       time.Now().UTC(),
		Username:    requestData.Username,
		Email:       requestData.Email,
		PassHash:    string(passHash),
		PlotCredits: 2,
		PlotIds:     []string{},
		Offenses:    []schemas.Offense{},
	}

	if _, err := usersCollection.InsertOne(ctx, newUser); err != nil {
		if !errors.Is(err, context.Canceled) {
			utils.LogErrorDiscord("CreateAccount", err, &requestData)
		}
		utils.MakeAPIResponse(w, r, http.StatusInternalServerError, nil, "Internal server error", true)
		return
	}

	utils.MakeAPIResponse(w, r, http.StatusCreated, nil, "Success", false)

}
