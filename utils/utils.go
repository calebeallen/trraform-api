package utils

import (
	"crypto/rand"
	"encoding/binary"
	"fmt"
	"math/big"
	"net/http"

	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/ses"
	petname "github.com/dustinkirkland/golang-petname"
	"github.com/go-chi/render"
	"github.com/go-playground/validator/v10"
	"github.com/redis/go-redis/v9"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.uber.org/zap"
)

type APIResponse struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    any    `json:"data"`
	Error   bool   `json:"error"`
}

func NewUsername() string {
	petname.NonDeterministicMode()
	n, _ := rand.Int(rand.Reader, big.NewInt(1000))
	return fmt.Sprintf("%s-%03d", petname.Generate(2, "-"), n.Int64())
}

type Handler struct {
	logger       *zap.Logger
	validate     *validator.Validate
	mongoDB      *mongo.Database
	redisClient  *redis.Client
	awsSESClient *ses.Client
	r2Client     *s3.Client
}

type ResParams struct {
	w        http.ResponseWriter
	r        *http.Request
	endpoint string
	code     int
	msg      string
	err      error
	errmsg   string
	reqData  any // 4 logs
	resData  any
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
