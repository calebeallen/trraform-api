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
	"golang.org/x/crypto/bcrypt"
)

func (h *Handler) PasswordLogin(w http.ResponseWriter, r *http.Request) {

	defer r.Body.Close()
	ctx := r.Context()
	resParams := &api.ResParams{W: w, R: r}

	var reqData struct {
		Email    string `json:"email" validate:"required,email"`
		Password string `json:"password" validate:"required,password"`
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
	password := strings.TrimSpace(reqData.Password)
	reqData.Password = ""

	if err := h.Validate.Struct(&reqData); err != nil {
		resParams.Code = http.StatusBadRequest
		resParams.Err = err
		h.Res(resParams)
		return
	}

	// find user
	var user schemas.User
	err := h.MongoDB.Collection("users").FindOne(ctx, bson.M{"email": reqData.Email}).Decode(&user)
	if errors.Is(err, mongo.ErrNoDocuments) || (err == nil && user.PassHash == "") { //doesn't exist, return
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

	// check password
	if err := bcrypt.CompareHashAndPassword([]byte(user.PassHash), []byte(password)); err != nil {
		resParams.ResData = &struct {
			CredentialError bool `json:"credentialError"`
		}{CredentialError: true}
		resParams.Err = err
		resParams.Code = http.StatusForbidden
		h.Res(resParams)
		return
	}

	// check email verification
	if !user.EmailVerified {
		resParams.ResData = &struct {
			NeedsVerification bool `json:"needsVerification"`
		}{NeedsVerification: true}
		resParams.Code = http.StatusForbidden
		h.Res(resParams)
		return
	}

	// issue jwt
	authToken := utils.CreateNewAuthToken(user.Id)
	authTokenStr, err := authToken.Sign()
	if err != nil {
		resParams.Err = err
		resParams.Code = http.StatusInternalServerError
		h.Res(resParams)
		return
	}

	resParams.ResData = &struct {
		Token string `json:"token"`
	}{Token: authTokenStr}
	resParams.Code = http.StatusOK
	h.Res(resParams)

}
