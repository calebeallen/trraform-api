package user

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"
	"trraformapi/internal/api"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
)

func (h *Handler) ChangeUsername(w http.ResponseWriter, r *http.Request) {

	defer r.Body.Close()
	ctx := r.Context()
	uid := ctx.Value("uid").(bson.ObjectID)
	resParams := &api.ResParams{W: w, R: r}

	var reqData struct {
		NewUsername string `json:"newUsername" validate:"required,username"`
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

	// normalize
	reqData.NewUsername = strings.TrimSpace(reqData.NewUsername)

	if err := h.Validate.Struct(&reqData); err != nil {
		resParams.Code = http.StatusBadRequest
		resParams.Err = err
		h.Res(resParams)
		return
	}

	// check that username doesn't exist
	usersCollection := h.MongoDB.Collection("users")
	err := usersCollection.FindOne(ctx, bson.M{"username": reqData.NewUsername}).Err()
	if err == nil {
		resParams.ResData = &struct {
			UsernameConflict bool `json:"usernameConflict"`
		}{UsernameConflict: true}
		resParams.Code = http.StatusConflict
		h.Res(resParams)
		return
	} else if !errors.Is(err, mongo.ErrNoDocuments) {
		resParams.Code = http.StatusInternalServerError
		resParams.Err = err
		h.Res(resParams)
		return
	}

	// set new username, only allow change every 14 days
	cutoff := time.Now().UTC().Add(-14 * 24 * time.Hour)
	res, err := usersCollection.UpdateOne(ctx,
		bson.M{
			"_id":            uid,
			"unameChangedAt": bson.M{"$lt": cutoff},
		},
		bson.M{
			"$set":         bson.M{"username": reqData.NewUsername},
			"$currentDate": bson.M{"unameChangedAt": true},
		},
	)
	if err != nil {
		resParams.Code = http.StatusInternalServerError
		resParams.Err = err
		h.Res(resParams)
		return
	}

	if res.MatchedCount == 0 {
		resParams.ResData = &struct {
			RateLimited bool `json:"rateLimited"`
		}{RateLimited: true}
		resParams.Code = http.StatusTooManyRequests
		h.Res(resParams)
		return
	}

	resParams.Code = http.StatusOK
	h.Res(resParams)

}
