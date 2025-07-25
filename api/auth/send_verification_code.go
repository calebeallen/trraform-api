package auth

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"trraformapi/api"
	"trraformapi/utils"
	"trraformapi/utils/schemas"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
)

func (h *Handler) SendVerificationCode(w http.ResponseWriter, r *http.Request) {

	defer r.Body.Close()
	ctx := r.Context()
	resParams := &api.ResParams{W: w, R: r}

	var reqData struct {
		Email string `json:"email" validate:"required,email"`
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
	reqData.Email = strings.TrimSpace(strings.ToLower(reqData.Email))

	if err := h.Validate.Struct(&reqData); err != nil {
		resParams.Code = http.StatusBadRequest
		resParams.Err = err
		h.Res(resParams)
		return
	}

	// check that account exists
	var user schemas.User
	err := h.MongoDB.Collection("users").FindOne(ctx, bson.M{"email": reqData.Email}).Decode(&user)
	if errors.Is(err, mongo.ErrNoDocuments) || (err == nil && user.PassHash == "") {
		resParams.ResData = &struct {
			CredentialError bool `json:"credentialError"`
		}{CredentialError: true}
		resParams.Err = err
		resParams.Code = http.StatusForbidden
		h.Res(resParams)
		return
	} else if err != nil {
		resParams.Err = err
		resParams.Code = http.StatusInternalServerError
		h.Res(resParams)
		return
	}

	// create new verification code for email
	if _, err := utils.NewVerificationCode(h.RedisCli, ctx, reqData.Email); err != nil {
		if err == utils.ErrUnusedVerificationCode {
			resParams.Code = http.StatusTooManyRequests
		} else {
			resParams.Code = http.StatusInternalServerError
		}
		resParams.Err = err
		h.Res(resParams)
		return
	}

	// queue email
	if err := h.RedisCli.LPush(ctx, "vemailq", reqData.Email).Err(); err != nil {
		resParams.Code = http.StatusInternalServerError
		resParams.Err = err
		h.Res(resParams)
		return
	}

	resParams.Code = http.StatusOK
	h.Res(resParams)

}
