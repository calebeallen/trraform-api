package auth

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"trraformapi/api"
	"trraformapi/utils"

	"github.com/redis/go-redis/v9"
	"go.mongodb.org/mongo-driver/v2/bson"
	"golang.org/x/crypto/bcrypt"
)

func (h *Handler) ResetPassword(w http.ResponseWriter, r *http.Request) {

	defer r.Body.Close()
	ctx := r.Context()
	resParams := &api.ResParams{W: w, R: r}

	// validate request data
	var reqData struct {
		Token       string `json:"token" validate:"required"`
		Email       string `json:"email" validate:"required,email"`
		NewPassword string `json:"newPassword" validate:"required,password"`
	}

	// validate request body
	if err := json.NewDecoder(r.Body).Decode(&reqData); err != nil {
		utils.MakeAPIResponse(w, r, http.StatusBadRequest, nil, "Invalid request body", true)
		return
	}
	password := strings.TrimSpace(reqData.NewPassword)
	reqData.Password = ""
	if err := utils.Validate.Struct(&reqData); err != nil {
		utils.MakeAPIResponse(w, r, http.StatusBadRequest, nil, "Invalid request body", true)
		return
	}

	// hash password and clear it from request data so it is not sent to error logs
	passHash, err := bcrypt.GenerateFromPassword([]byte(reqData.NewPassword), bcrypt.DefaultCost)
	reqData.NewPassword = ""
	if err != nil {
		utils.LogErrorDiscord("ResetPassword", err, &reqData)
		utils.MakeAPIResponse(w, r, http.StatusInternalServerError, nil, "Internal server error", true)
		return
	}

	redisKey := "email:reset:token:" + reqData.Email

	// validate token by first checking if it exists in redis
	redisToken, err := utils.RedisCli.Get(ctx, redisKey).Result()
	if errors.Is(err, redis.Nil) { // if token not found (expired)
		utils.MakeAPIResponse(w, r, http.StatusForbidden, nil, "Expired token", true)
		return
	} else if err != nil {
		if !errors.Is(err, context.Canceled) {
			utils.LogErrorDiscord("ResetPassword", err, &reqData)
		}
		utils.MakeAPIResponse(w, r, http.StatusInternalServerError, nil, "Internal server error", true)
		return
	}

	// validate that token in redis is the same as one sent
	if redisToken != reqData.Token {
		utils.MakeAPIResponse(w, r, http.StatusForbidden, nil, "Invalid token", true)
		return
	}

	usersCollection := utils.MongoDB.Collection("users")

	// reset password
	_, err = usersCollection.UpdateOne(ctx, bson.M{
		"email": reqData.Email,
	}, bson.M{
		"$set": bson.M{
			"passHash": string(passHash),
		},
	})
	if err != nil {
		if !errors.Is(err, context.Canceled) {
			utils.LogErrorDiscord("ResetPassword", err, &reqData)
		}
		utils.MakeAPIResponse(w, r, http.StatusInternalServerError, nil, "Internal server error", true)
		return
	}

	// delete token
	_, err = utils.RedisCli.Del(ctx, redisKey).Result()
	if err != nil {
		if !errors.Is(err, context.Canceled) {
			utils.LogErrorDiscord("ResetPassword", err, &reqData)
		}
		utils.MakeAPIResponse(w, r, http.StatusInternalServerError, nil, "Internal server error", true)
		return
	}

	utils.MakeAPIResponse(w, r, http.StatusOK, nil, "Success", false)

}
