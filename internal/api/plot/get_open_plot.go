package plot

import (
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"trraformapi/internal/api"
	"trraformapi/pkg/config"

	"github.com/redis/go-redis/v9"
)

func (h *Handler) GetOpenPlot(w http.ResponseWriter, r *http.Request) {

	ctx := r.Context()
	resParams := &api.ResParams{W: w, R: r}

	var responseData struct {
		PlotId string `json:"plotId"`
	}

	depthParam := r.URL.Query().Get("depth")
	depth, err := strconv.ParseUint(depthParam, 10, 64)
	if err != nil || depth > uint64(config.VAR.MAX_DEPTH) {
		resParams.ResData = &struct {
			InvalidDepth bool `json:"invalidDepth"`
		}{InvalidDepth: true}
		resParams.Code = http.StatusBadRequest
		resParams.Err = err
		h.Res(resParams)
		return
	}

	key := fmt.Sprintf("openplots:%d", depth)
	responseData.PlotId, err = h.RedisCli.SRandMember(ctx, key).Result()
	if errors.Is(err, redis.Nil) {
		resParams.ResData = &struct {
			NoneAvailable bool `json:"noneAvailable"`
		}{NoneAvailable: true}
		resParams.Code = http.StatusBadRequest
		resParams.Err = err
		h.Res(resParams)
		return
	} else if err != nil {
		resParams.Code = http.StatusInternalServerError
		resParams.Err = err
		h.Res(resParams)
		return
	}

	resParams.Code = http.StatusOK
	h.Res(resParams)

}
