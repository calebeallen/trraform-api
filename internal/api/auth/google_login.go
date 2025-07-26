package auth

import (
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"strings"
	"time"
	"trraformapi/internal/api"
	"trraformapi/pkg/schemas"
	"trraformapi/pkg/utils"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"google.golang.org/api/idtoken"
)

func (h *Handler) GoogleLogin(w http.ResponseWriter, r *http.Request) {

	defer r.Body.Close()
	ctx := r.Context()
	resParams := &api.ResParams{W: w, R: r}

	var reqData struct {
		Token string `json:"token" validate:"required"` //google token
	}

	// validate request body
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&reqData); err != nil {
		resParams.Code = http.StatusBadRequest
		resParams.Err = err
		h.Res(resParams)
		return
	}
	resParams.ReqData = reqData

	if err := h.Validate.Struct(&reqData); err != nil {
		resParams.Code = http.StatusBadRequest
		resParams.Err = err
		h.Res(resParams)
		return
	}

	// validate google token
	googleToken, err := idtoken.Validate(ctx, reqData.Token, os.Getenv("GOOGLE_CLIENT_ID"))
	if err != nil {
		resParams.Code = http.StatusForbidden
		resParams.Err = err
		h.Res(resParams)
		return
	}
	googleId := googleToken.Claims["sub"].(string)

	// find user
	usersCollection := h.MongoDB.Collection("users")
	var user schemas.User
	err = usersCollection.FindOne(ctx, bson.M{
		"googleId": googleId,
	}).Decode(&user)

	// create new user if none found
	if errors.Is(err, mongo.ErrNoDocuments) {

		// email must be provided
		email, ok := googleToken.Claims["email"].(string)
		if !ok || email == "" {
			resParams.ResData = &struct {
				EmailMissing bool `json:"emailMissing"`
			}{EmailMissing: true}
			resParams.Err = err
			h.Res(resParams)
			return
		}

		user = schemas.User{
			Ctime:        time.Now().UTC(),
			Username:     utils.NewUsername(),
			GoogleId:     googleId,
			Email:        strings.ToLower(email),
			PlotCredits:  1,
			PlotIds:      []string{},
			PurchasedIds: []string{},
			Offenses:     []schemas.Offense{},
		}

		if _, err := usersCollection.InsertOne(ctx, user); err != nil {
			if mongo.IsDuplicateKeyError(err) {
				resParams.Code = http.StatusConflict
			} else {
				resParams.Code = http.StatusInternalServerError
			}
			resParams.Err = err
			h.Res(resParams)
			return
		}

	} else if err != nil {
		resParams.Code = http.StatusInternalServerError
		resParams.Err = err
		h.Res(resParams)
		return
	}

	// create jwt
	authToken := utils.CreateNewAuthToken(user.Id)
	authTokenStr, err := authToken.Sign()
	if err != nil {
		resParams.Code = http.StatusInternalServerError
		resParams.Err = err
		h.Res(resParams)
		return
	}

	resParams.ResData = &struct {
		Token string `json:"token"`
	}{Token: authTokenStr}
	resParams.Code = http.StatusOK
	h.Res(resParams)

}
