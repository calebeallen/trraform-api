package api

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"runtime"
	"trraformapi/pkg/utils"

	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/ses"
	"github.com/go-chi/render"
	"github.com/go-playground/validator/v10"
	"github.com/redis/go-redis/v9"
	"github.com/stripe/stripe-go/v82"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.uber.org/zap"
)

type Handler struct {
	Logger    *zap.Logger
	Validate  *validator.Validate
	HttpCli   *http.Client
	MongoDB   *mongo.Database
	RedisCli  *redis.Client
	AWSSESCli *ses.Client
	R2Cli     *s3.Client
	StripeCli *stripe.Client
}

type ResParams struct {
	W       http.ResponseWriter
	R       *http.Request
	Code    int
	Err     error
	ReqData any // for logs
	ResData any
}

func (h *Handler) AuthMiddleware(f http.HandlerFunc) http.HandlerFunc {

	return func(w http.ResponseWriter, r *http.Request) {
		resParams := &ResParams{W: w, R: r}
		authToken, err := utils.ValidateAuthToken(r)
		if err != nil {
			resParams.Err = err
			resParams.Code = http.StatusUnauthorized
			h.Res(resParams)
			return
		}
		uid, err := authToken.GetUidObjectId()
		if err != nil {
			resParams.Err = err
			resParams.Code = http.StatusInternalServerError
			h.Res(resParams)
			return
		}
		ctx := context.WithValue(r.Context(), "uid", uid)
		f(w, r.WithContext(ctx))
	}

}

func (h *Handler) Res(params *ResParams) {

	if params.Err != nil && errors.Is(params.Err, context.Canceled) {
		return
	}

	pc, file, line, ok := runtime.Caller(1)
	var caller string
	if !ok {
		caller = "unknown"
	}
	fn := runtime.FuncForPC(pc)
	caller = fmt.Sprintf("%s:%d (%s)", file, line, fn.Name())

	// handle logging
	if params.Code >= 500 {
		h.Logger.Error("Error at "+caller,
			zap.Error(params.Err),
			zap.Any("request_data", params.ReqData),
		)
	} else if params.Code >= 400 {
		h.Logger.Warn("Warning at "+caller,
			zap.Error(params.Err),
			zap.Any("request_data", params.ReqData),
		)
	}

	render.Status(params.R, params.Code)
	render.JSON(params.W, params.R, params.ResData)

}
