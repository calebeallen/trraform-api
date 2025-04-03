package auth

import (
	"encoding/json"
	"net/http"
	"time"
	"trraformapi/utils"
	"trraformapi/utils/schemas"

	"go.mongodb.org/mongo-driver/v2/bson"
	"golang.org/x/crypto/bcrypt"
)

func SignUp(w http.ResponseWriter, r *http.Request) {

	ctx := r.Context()

	var requestData struct {
		Username string `json:"username" validate:"required,max=48"`
		Email    string `json:"email" validate:"required,email"`
		Password string `json:"password" validate:"required,min=8,max=128"`
	}

	// validate request body
	if err := json.NewDecoder(r.Body).Decode(&requestData); err != nil {
		utils.MakeAPIResponse(w, r, http.StatusBadRequest, nil, "Invalid request body", true)
		return
	}
	defer r.Body.Close()

	if err := utils.Validate.Struct(requestData); err != nil {
		utils.MakeAPIResponse(w, r, http.StatusBadRequest, nil, "Invalid request body", true)
		return
	}

	usersCollection := utils.MongoDB.Collection("users")

	cursor, err := usersCollection.Find(ctx, bson.D{
		{Key: "$or", Value: bson.A{
			bson.D{{Key: "username", Value: ""}},
			bson.D{{Key: "email", Value: ""}},
		}},
	})
	if err != nil {
		utils.MakeAPIResponse(w, r, http.StatusInternalServerError, nil, "Query error", true)
		return
	}
	defer cursor.Close(ctx)

	var users []schemas.User
	if err := cursor.All(ctx, &users); err != nil {
		utils.MakeAPIResponse(w, r, http.StatusInternalServerError, nil, "Query error", true)
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

		utils.MakeAPIResponse(w, r, http.StatusOK, exist, "Credentials already exist", true)
		return

	}

	// hash password
	passHash, err := bcrypt.GenerateFromPassword([]byte(requestData.Password), bcrypt.DefaultCost)
	if err != nil {
		utils.MakeAPIResponse(w, r, http.StatusBadRequest, nil, "Invalid password", true)
		return
	}

	// create default entry in mongo
	newUser := schemas.User{
		Ctime:         time.Now().UTC(),
		Username:      requestData.Username,
		Email:         requestData.Email,
		EmailVerified: false,
		PassHash:      string(passHash),
		GoogleId:      "",
		Subscribed:    false,
		PlotCredits:   2,
		RsxEnd:        time.Time{},
		Banned:        false,
	}

	if _, err := usersCollection.InsertOne(ctx, newUser); err != nil {
		utils.MakeAPIResponse(w, r, http.StatusBadRequest, nil, "Could not create user", true)
		return
	}

	// create token for sending verification email

}
