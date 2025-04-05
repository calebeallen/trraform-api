package auth

import (
	"encoding/json"
	"log"
	"net/http"
	"trraformapi/utils"

	"github.com/redis/go-redis/v9"
	"go.mongodb.org/mongo-driver/v2/bson"
	"golang.org/x/crypto/bcrypt"
)

func ResetPassword(w http.ResponseWriter, r *http.Request) {

	ctx := r.Context()

	// validate request data
	var requestData struct {
		Token       string `json:"token" validate:"required"`
		Email       string `json:"email" validate:"required,email"`
		NewPassword string `json:"newPassword" validate:"required,password"`
	}

	// validate request body
	if err := json.NewDecoder(r.Body).Decode(&requestData); err != nil {
		utils.MakeAPIResponse(w, r, http.StatusBadRequest, nil, "Invalid request body", true)
		return
	}
	defer r.Body.Close()
	if err := utils.Validate.Struct(&requestData); err != nil {
		utils.MakeAPIResponse(w, r, http.StatusBadRequest, nil, "Invalid request body", true)
		return
	}

	// hash password and clear it from request data so it is not sent to error logs
	passHash, err := bcrypt.GenerateFromPassword([]byte(requestData.NewPassword), bcrypt.DefaultCost)
	requestData.NewPassword = ""
	if err != nil {
		log.Println(err)
		utils.LogErrorDiscord("ResetPassword", err, &requestData)
		utils.MakeAPIResponse(w, r, http.StatusInternalServerError, nil, "Internal server error", true)
		return
	}

	redisKey := "email:reset:token:" + requestData.Email

	// validate token by first checking if it exists in redis
	redisToken, err := utils.RedisCli.Get(ctx, redisKey).Result()
	if err == redis.Nil { // if token not found (expired)
		utils.MakeAPIResponse(w, r, http.StatusForbidden, nil, "Expired token", true)
		return
	} else if err != nil {
		log.Println(err)
		utils.LogErrorDiscord("ResetPassword", err, &requestData)
		utils.MakeAPIResponse(w, r, http.StatusInternalServerError, nil, "Internal server error", true)
		return
	}

	// validate that token in redis is the same as one sent
	if redisToken != requestData.Token {
		utils.MakeAPIResponse(w, r, http.StatusForbidden, nil, "Invalid token", true)
		return
	}

	// reset password
	usersCollection := utils.MongoDB.Collection("users")
	usersCollection.UpdateOne(ctx, bson.M{
		"email": requestData.Email,
	}, bson.M{
		"$set": bson.M{
			"passHash": string(passHash),
		},
	})

	// delete token
	_, err = utils.RedisCli.Del(ctx, redisKey).Result()
	if err != nil {
		log.Println(err)
		utils.LogErrorDiscord("ResetPassword", err, &requestData)
		utils.MakeAPIResponse(w, r, http.StatusInternalServerError, nil, "Internal server error", true)
		return
	}

	utils.MakeAPIResponse(w, r, http.StatusOK, nil, "Success", false)

}
