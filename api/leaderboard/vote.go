package leaderboard

import (
	"encoding/json"
	"net/http"
	"trraformapi/utils"
	plotutils "trraformapi/utils/plot_utils"
)

func Vote(w http.ResponseWriter, r *http.Request) {

	ctx := r.Context()

	var requestData struct {
		PlotId string `json:"plotId" validate:"required,plotid"`
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

	plotId, _ := plotutils.PlotIdFromHexString(requestData.PlotId)

	err := utils.RedisCli.ZIncrBy(ctx, "leaderboard:votes", 1, plotId.ToString()).Err()
	if err != nil {
		utils.MakeAPIResponse(w, r, http.StatusInternalServerError, nil, "Internal server error", true)
		return
	}

	utils.MakeAPIResponse(w, r, http.StatusOK, nil, "Success", false)

}
