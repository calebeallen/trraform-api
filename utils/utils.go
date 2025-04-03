package utils

import (
	"net/http"

	"github.com/aws/aws-sdk-go-v2/service/ses"
	"github.com/go-chi/render"
	"github.com/go-playground/validator/v10"
	"github.com/redis/go-redis/v9"
	"go.mongodb.org/mongo-driver/v2/mongo"
)

var RedisCli *redis.Client
var MongoCli *mongo.Client
var MongoDB *mongo.Database
var AwsSESCli *ses.Client

var Validate = validator.New()

type APIResponse struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    any    `json:"data"`
	Error   bool   `json:"error"`
}

func MakeAPIResponse(w http.ResponseWriter, r *http.Request, code int, data any, message string, err bool) {

	res := APIResponse{
		Code:    code,
		Message: message,
		Data:    data,
		Error:   err,
	}

	render.Status(r, code)
	render.JSON(w, r, res)

}
