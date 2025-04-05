package auth

import (
	"encoding/json"
	"log"
	"net/http"
	"os"
	"strings"
	"time"
	"trraformapi/utils"
	"trraformapi/utils/schemas"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"google.golang.org/api/idtoken"
)

func GoogleLogIn(w http.ResponseWriter, r *http.Request) {

	ctx := r.Context()

	var requestData struct {
		Token string `json:"token" validate:"required"`
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

	// validate google token
	payload, err := idtoken.Validate(ctx, requestData.Token, os.Getenv("GOOGLE_CLIENT_ID"))
	if err != nil {
		utils.MakeAPIResponse(w, r, http.StatusForbidden, nil, "Invalid token", true)
		return
	}

	// username required to be unique when creating account from front end, but it doesn't really matter.
	googleId := payload.Claims["sub"].(string)
	email := payload.Claims["email"].(string)
	emailVerified := payload.Claims["email_verified"].(bool)
	username := strings.Split(email, "@")[0]
	if len(username) > 32 {
		username = username[:32]
	}

	usersCollection := utils.MongoDB.Collection("users")

	// find user
	var user *schemas.User
	res := usersCollection.FindOne(ctx, bson.M{"googleId": googleId})
	err = res.Decode(&user)

	// create new user if not found
	if err == mongo.ErrNoDocuments {

		user = &schemas.User{
			Ctime:         time.Now().UTC(),
			Username:      username,
			Email:         email,
			EmailVerified: emailVerified,
			PassHash:      "",
			GoogleId:      googleId,
			Subscribed:    false,
			PlotCredits:   2,
			RsxEnd:        nil,
			Banned:        false,
		}

		if _, err := usersCollection.InsertOne(ctx, user); err != nil {
			log.Println(err)
			utils.LogErrorDiscord("GoogleLogIn", err, &requestData)
			utils.MakeAPIResponse(w, r, http.StatusInternalServerError, nil, "Internal server error", true)
			return
		}

	} else if err != nil {
		log.Println(err)
		utils.LogErrorDiscord("GoogleLogIn", err, &requestData)
		utils.MakeAPIResponse(w, r, http.StatusInternalServerError, nil, "Internal server error", true)
		return
	}

	// issue jwt
	authToken := utils.CreateNewAuthToken(user.Id)
	if err := authToken.SetCookie(w); err != nil {
		log.Println(err)
		utils.LogErrorDiscord("GoogleLogIn", err, &requestData)
		utils.MakeAPIResponse(w, r, http.StatusInternalServerError, nil, "Internal server error", true)
		return
	}

	utils.MakeAPIResponse(w, r, http.StatusOK, nil, "Success", false)

}
