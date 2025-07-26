package leaderboard

import (
	"encoding/json"
	"net/http"
	"trraformapi/internal/api"
	plotutils "trraformapi/pkg/plot_utils"
)

func (h *Handler) Vote(w http.ResponseWriter, r *http.Request) {

	defer r.Body.Close()
	ctx := r.Context()
	resParams := &api.ResParams{W: w, R: r}

	var reqData struct {
		PlotId string `json:"plotId" validate:"required,plotid"`
	}

	// validate request body
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := json.NewDecoder(r.Body).Decode(&reqData); err != nil {
		resParams.Code = http.StatusBadRequest
		resParams.Err = err
		h.Res(resParams)
		return
	}
	resParams.ReqData = reqData

	if err := h.Validate.Struct(&reqData); err != nil {
		resParams.Code = http.StatusBadRequest
		resParams.Err = err
		h.Res(resParams)
		return
	}

	plotId, _ := plotutils.PlotIdFromHexString(reqData.PlotId)

	err := h.RedisCli.ZIncrBy(ctx, "leaderboard:votes", 1, plotId.ToString()).Err()
	if err != nil {
		resParams.Code = http.StatusInternalServerError
		resParams.Err = err
		h.Res(resParams)
		return
	}

	resParams.Code = http.StatusOK
	h.Res(resParams)

}
