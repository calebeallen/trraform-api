package auth

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"strings"
	"time"
	"trraformapi/utils"
	"trraformapi/utils/schemas"

	"github.com/stripe/stripe-go/v82"
	"github.com/stripe/stripe-go/v82/customer"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"google.golang.org/api/idtoken"
)

func GoogleLogIn(w http.ResponseWriter, r *http.Request) {

	ctx := r.Context()

	var requestData struct {
		Token string `json:"token" validate:"required"` //google token
	}

	var responseData struct {
		Token string `json:"token"`
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
	email = strings.ToLower(email)
	username := strings.Split(email, "@")[0]
	if len(username) > 32 {
		username = username[:32]
	}

	usersCollection := utils.MongoDB.Collection("users")

	// find user
	// if user used their email to create an account with a password, then logged in with that gmail via google, retrieve the existing account.
	var user schemas.User
	err = usersCollection.FindOne(ctx, bson.M{
		"$or": bson.M{
			"googleId": googleId,
			"email":    email,
		},
	}).Decode(&user)

	// create new user if not found
	if errors.Is(err, mongo.ErrNoDocuments) {

		// create new stripe customer
		params := stripe.CustomerParams{
			Email: stripe.String(email),
		}
		stripeCustomer, err := customer.New(&params)
		if err != nil {
			utils.LogErrorDiscord("GoogleLogIn", err, &requestData)
			utils.MakeAPIResponse(w, r, http.StatusInternalServerError, nil, "Internal server error", true)
			return
		}

		user = schemas.User{
			Id:             bson.NewObjectID(),
			Ctime:          time.Now().UTC(),
			Username:       username,
			GoogleId:       googleId,
			Email:          email,
			StripeCustomer: stripeCustomer.ID,
			PlotCredits:    2,
			PlotIds:        []string{},
			Offenses:       []schemas.Offense{},
		}

		if _, err := usersCollection.InsertOne(ctx, &user); err != nil {
			if !errors.Is(err, context.Canceled) {
				utils.LogErrorDiscord("GoogleLogIn", err, &requestData)
			}
			utils.MakeAPIResponse(w, r, http.StatusInternalServerError, nil, "Internal server error", true)
			return
		}

	} else if err != nil {
		if !errors.Is(err, context.Canceled) {
			utils.LogErrorDiscord("GoogleLogIn", err, &requestData)
		}
		utils.MakeAPIResponse(w, r, http.StatusInternalServerError, nil, "Internal server error", true)
		return
	}

	// issue jwt
	authToken := utils.CreateNewAuthToken(user.Id)
	authTokenStr, err := authToken.Sign()
	if err != nil {
		utils.LogErrorDiscord("GoogleLogIn", err, &requestData)
		utils.MakeAPIResponse(w, r, http.StatusInternalServerError, nil, "Internal server error", true)
		return
	}
	responseData.Token = authTokenStr

	utils.MakeAPIResponse(w, r, http.StatusOK, &responseData, "Success", false)

}
