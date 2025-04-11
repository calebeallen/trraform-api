package plot

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"trraformapi/utils"

	"github.com/redis/go-redis/v9"
)

func GetOpenPlot(w http.ResponseWriter, r *http.Request) {

	ctx := r.Context()

	_, err := utils.ValidateAuthToken(r)
	if err != nil {
		utils.MakeAPIResponse(w, r, http.StatusForbidden, nil, "Invalid token", true)
		return
	}

	var responseData struct {
		PlotId string `json:"plotId"`
	}

	depthParam := r.URL.Query().Get("depth")
	depth, err := strconv.ParseUint(depthParam, 10, 64)
	if err != nil || depth > utils.MaxDepth {
		utils.MakeAPIResponse(w, r, http.StatusBadRequest, nil, "Invalid depth", true)
		return
	}

	key := fmt.Sprintf("openplots:%d", depth)
	responseData.PlotId, err = utils.RedisCli.SRandMember(ctx, key).Result()
	if errors.Is(err, redis.Nil) {
		utils.MakeAPIResponse(w, r, http.StatusNotFound, nil, "No plots found", true)
		return
	} else if err != nil {
		if !errors.Is(err, context.Canceled) {
			utils.LogErrorDiscord("GetOpenPlot", err, nil)
		}
		utils.MakeAPIResponse(w, r, http.StatusInternalServerError, nil, "Internal server error", true)
		return
	}

	utils.MakeAPIResponse(w, r, http.StatusOK, &responseData, "Success", false)

}
