package auth

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
	"time"
	"trraformapi/utils"
	"trraformapi/utils/schemas"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"google.golang.org/api/idtoken"
)

func GoogleLogIn(w http.ResponseWriter, r *http.Request) {

	ctx := r.Context()

	var requestData struct {
		Token string `json:"token" validate:"required"` //google token
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
			Id:          bson.NewObjectID(),
			Ctime:       time.Now().UTC(),
			Username:    username,
			GoogleId:    googleId,
			PlotCredits: 2,
			PlotIds:     []int64{},
			Offenses:    []schemas.Offense{},
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
	fmt.Println(authToken)
	authTokenStr, err := authToken.Sign()
	if err != nil {
		log.Println(err)
		utils.LogErrorDiscord("GoogleLogIn", err, &requestData)
		utils.MakeAPIResponse(w, r, http.StatusInternalServerError, nil, "Internal server error", true)
		return
	}
	responseData := struct {
		Token string `json:"token"`
	}{Token: authTokenStr}

	utils.MakeAPIResponse(w, r, http.StatusOK, &responseData, "Success", false)

}
