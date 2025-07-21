package auth

import (
	"encoding/json"
	"net/http"
	"trraformapi/api"
	"trraformapi/utils"

	"go.mongodb.org/mongo-driver/v2/bson"
)

func (h *Handler) VerifyEmail(w http.ResponseWriter, r *http.Request) {

	ctx := r.Context()
	resParams := &api.ResParams{W: w, R: r}

	// validate request data
	var reqData struct {
		Uid              string `json:"uid" validate:"required"`
		VerificationCode string `json:"verificationCode" validate:"required"`
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

	// check verification code
	ok, err := utils.ValidateVerificationCode(ctx, reqData.Uid, reqData.VerificationCode)
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

	// set account to verified
	usersCollection := utils.MongoDB.Collection("users")
	_, err = usersCollection.UpdateOne(ctx, bson.M{
		"_id": uid,
	}, bson.M{
		"$set": bson.M{
			"emailVerified": true,
		},
	})
	if err != nil {
		resParams.Code = http.StatusInternalServerError
		resParams.Err = err
		h.Res(resParams)
		return
	}

	//issue token
	authToken := utils.CreateNewAuthToken(uid)
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
