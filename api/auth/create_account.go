package auth

// TODO: create unique index

import (
	"encoding/json"
	"net/http"
	"strings"
	"time"
	"trraformapi/api"
	"trraformapi/utils"
	"trraformapi/utils/schemas"

	"go.mongodb.org/mongo-driver/v2/mongo"
	"golang.org/x/crypto/bcrypt"
)

func (h *Handler) CreateAccount(w http.ResponseWriter, r *http.Request) {

	defer r.Body.Close()
	ctx := r.Context()
	resParams := &api.ResParams{W: w, R: r}

	var reqData struct {
		Email    string `json:"email" validate:"required,email"`
		Password string `json:"password" validate:"required,password"`
		CfToken  string `json:"cfToken" validate:"required"`
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

	// hash password
	passHash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		resParams.Code = http.StatusInternalServerError
		resParams.Err = err
		h.Res(resParams)
		return
	}

	// validate cf turnstile token
	err = utils.ValidateTurnstileToken(h.HttpCli, ctx, reqData.CfToken)
	if err != nil {
		resParams.ResData = &struct {
			InvalidCFToken bool `json:"invalidCFToken"`
		}{InvalidCFToken: true}
		resParams.Code = http.StatusBadRequest
		h.Res(resParams)
		return
	}

	// create user entry in mongo
	newUser := &schemas.User{
		Ctime:        time.Now().UTC(),
		Username:     utils.NewUsername(),
		Email:        reqData.Email,
		PassHash:     string(passHash),
		PlotCredits:  1,
		PlotIds:      []string{},
		PurchasedIds: []string{},
		Offenses:     []schemas.Offense{},
	}

	if _, err := h.MongoDB.Collection("users").InsertOne(ctx, newUser); err != nil {
		if mongo.IsDuplicateKeyError(err) {
			resParams.Code = http.StatusConflict
		} else {
			resParams.Code = http.StatusInternalServerError
		}
		resParams.Err = err
		h.Res(resParams)
		return
	}

	resParams.Code = http.StatusOK
	h.Res(resParams)

}
