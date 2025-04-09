package utils

import (
	"encoding/binary"
	"fmt"
	"net/http"
	"regexp"

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

func init() {

	Validate.RegisterValidation("username", func(fl validator.FieldLevel) bool {
		username := fl.Field().String()
		re := regexp.MustCompile(`^[a-zA-Z0-9._]{3,32}$`)
		return re.MatchString(username)
	})

	Validate.RegisterValidation("password", func(fl validator.FieldLevel) bool {
		password := fl.Field().String()
		re := regexp.MustCompile(`^[A-Za-z0-9~` + "`" + `!@#$%^&*()_\-+={[}\]|\\:;"'<,>.?/]{8,128}$`)
		return re.MatchString(password)
	})

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

func BytesToUint16Arr(data []byte) ([]uint16, error) {

	if len(data)%2 != 0 {
		return nil, fmt.Errorf("in BytesToUint16Arr:\nbytes length must be even")
	}

	u16 := make([]uint16, len(data)/2)

	for i := range u16 {
		u16[i] = binary.LittleEndian.Uint16(data[2*i : 2*i+2])
	}

	return u16, nil

}

func Uint16ArrToBytes(u16 []uint16) []byte {

	data := make([]byte, len(u16)*2)

	for i, val := range u16 {
		binary.LittleEndian.PutUint16(data[2*i:2*i+2], val)
	}

	return data

}
