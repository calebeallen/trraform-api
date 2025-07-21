package auth

import (
	"encoding/json"
	"errors"
	"net/http"
	"trraformapi/api"
	"trraformapi/utils"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
)

func (h *Handler) SendVerificationCode(w http.ResponseWriter, r *http.Request) {

	ctx := r.Context()
	resParams := &api.ResParams{W: w, R: r}

	var reqData struct {
		Uid string `json:"uid" validate:"required"`
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
	defer r.Body.Close()
	resParams.ReqData = reqData

	if err := utils.Validate.Struct(&reqData); err != nil {
		resParams.Code = http.StatusBadRequest
		resParams.Err = err
		h.Res(resParams)
		return
	}

	// validate uid
	uid, err := bson.ObjectIDFromHex(reqData.Uid)
	if err != nil {
		resParams.Code = http.StatusBadRequest
		resParams.Err = err
		h.Res(resParams)
		return
	}

	// check that account with uid exists
	usersCollection := utils.MongoDB.Collection("users")
	if err := usersCollection.FindOne(ctx, bson.M{"_id": uid}).Err(); err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			resParams.Code = http.StatusNotFound
		} else {
			resParams.Code = http.StatusInternalServerError
		}
		resParams.Err = err
		h.Res(resParams)
		return
	}

	// create new verification code for uid
	if _, err := utils.NewVerificationCode(ctx, reqData.Uid); err != nil {
		if err == utils.ErrUnusedVerificationCode { // a valid code exist under uid
			resParams.Code = http.StatusTooManyRequests
		} else {
			resParams.Code = http.StatusInternalServerError
		}
		resParams.Err = err
		h.Res(resParams)
		return
	}

	// queue email
	if err := utils.RedisCli.LPush(ctx, "emailq", reqData.Uid).Err(); err != nil {
		resParams.Code = http.StatusInternalServerError
		resParams.Err = err
		h.Res(resParams)
		return
	}

	resParams.Code = http.StatusOK
	h.Res(resParams)

}
