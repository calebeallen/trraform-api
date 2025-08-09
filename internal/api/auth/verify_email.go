package auth

import (
	"encoding/json"
	"net/http"
	"strings"
	"trraformapi/internal/api"
	"trraformapi/pkg/schemas"
	"trraformapi/pkg/utils"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

func (h *Handler) VerifyEmail(w http.ResponseWriter, r *http.Request) {

	defer r.Body.Close()
	ctx := r.Context()
	resParams := &api.ResParams{W: w, R: r}

	// validate request data
	var reqData struct {
		Email     string `json:"email" validate:"required,email"`
		VerifCode string `json:"verifCode" validate:"required"`
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

	// TODO check verification code then delete it!
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
		resParams.Code = http.StatusUnauthorized
		resParams.Err = err
		h.Res(resParams)
		return
	}

	// set account to verified
	var updatedUser schemas.User
	err = h.MongoDB.Collection("users").FindOneAndUpdate(ctx,
		bson.M{"email": reqData.Email},
		bson.M{"$set": bson.M{"emailVerified": true}},
		options.FindOneAndUpdate().SetReturnDocument(options.After),
	).Decode(&updatedUser)
	if err != nil {
		resParams.Code = http.StatusInternalServerError
		resParams.Err = err
		h.Res(resParams)
		return
	}

	// create token
	authToken := utils.CreateNewAuthToken(updatedUser.Id)
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
