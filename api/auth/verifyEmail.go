package auth

import (
	"log"
	"net/http"
	"trraformapi/utils"

	"github.com/redis/go-redis/v9"
	"go.mongodb.org/mongo-driver/v2/bson"
)

func VerifyEmail(w http.ResponseWriter, r *http.Request) {

	ctx := r.Context()

	// validate request data
	var requestData struct {
		Token string `json:"token" validate:"required"`
		Email string `json:"email" validate:"required,email"`
	}
	params := r.URL.Query()
	requestData.Token = params.Get("token")
	requestData.Email = params.Get("email")
	if err := utils.Validate.Struct(&requestData); err != nil {
		utils.MakeAPIResponse(w, r, http.StatusBadRequest, nil, "Invalid request body", true)
		return
	}

	redisKey := "email:verify:" + requestData.Email

	// validate token by first checking if it exists in redis
	redisToken, err := utils.RedisCli.Get(ctx, redisKey).Result()
	if err == redis.Nil { // if token not found (expired)
		utils.MakeAPIResponse(w, r, http.StatusForbidden, nil, "Expired token", true)
		return
	} else if err != nil {
		log.Println(err)
		utils.LogErrorDiscord("VerifyEmail", err, &requestData)
		utils.MakeAPIResponse(w, r, http.StatusInternalServerError, nil, "Internal server error", true)
		return
	}

	// validate that token in redis is the same as query param token
	if redisToken != requestData.Token {
		utils.MakeAPIResponse(w, r, http.StatusForbidden, nil, "Invalid token", true)
		return
	}

	// set verification status
	usersCollection := utils.MongoDB.Collection("users")
	usersCollection.UpdateOne(ctx, bson.M{
		"email": requestData.Email,
	}, bson.M{
		"$set": bson.M{
			"emailVerified": true,
		},
	})

	// delete token
	_, err = utils.RedisCli.Del(ctx, redisKey).Result()
	if err != nil {
		log.Println(err)
		utils.LogErrorDiscord("VerifyEmail", err, &requestData)
		utils.MakeAPIResponse(w, r, http.StatusInternalServerError, nil, "Internal server error", true)
		return
	}

	utils.MakeAPIResponse(w, r, http.StatusOK, nil, "Success", false)

}
