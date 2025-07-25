package auth

import (
	"encoding/json"
	"net/http"
	"strings"
	"trraformapi/api"
	"trraformapi/utils"

	"go.mongodb.org/mongo-driver/v2/bson"
	"golang.org/x/crypto/bcrypt"
)

func (h *Handler) ResetPassword(w http.ResponseWriter, r *http.Request) {

	defer r.Body.Close()
	ctx := r.Context()
	resParams := &api.ResParams{W: w, R: r}

	// validate request data
	var reqData struct {
		Email       string `json:"email" validate:"required,email"`
		NewPassword string `json:"newPassword" validate:"required,password"`
		VerifCode   string `json:"verifCode" validate:"required"`
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
	password := strings.TrimSpace(reqData.NewPassword)
	reqData.NewPassword = ""

	if err := h.Validate.Struct(&reqData); err != nil {
		resParams.Code = http.StatusBadRequest
		resParams.Err = err
		h.Res(resParams)
		return
	}

	// check verification code
	ok, err := utils.ValidateVerificationCode(h.RedisCli, ctx, reqData.Email, reqData.VerifCode)
	if err != nil {
		resParams.Code = http.StatusInternalServerError
		resParams.Err = err
		h.Res(resParams)
		return
	}
	if !ok {
		resParams.ResData = &struct {
			InvalidCode bool `json:"invalidCode"`
		}{InvalidCode: true}
		resParams.Code = http.StatusInternalServerError
		resParams.Err = err
		h.Res(resParams)
		return
	}

	// hash password
	passHash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		resParams.Code = http.StatusInternalServerError
		resParams.Err = err
		h.Res(resParams)
		return
	}

	// reset password
	_, err = h.MongoDB.Collection("users").UpdateOne(ctx, bson.M{
		"uid": reqData.Email,
	}, bson.M{
		"$set": bson.M{
			"passHash": string(passHash),
		},
	})
	if err != nil {
		resParams.Code = http.StatusInternalServerError
		resParams.Err = err
		h.Res(resParams)
		return
	}

	resParams.Code = http.StatusOK
	h.Res(resParams)

}
