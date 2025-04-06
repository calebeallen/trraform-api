package plot

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"
	"trraformapi/utils"
)

func ClaimWithCredit(w http.ResponseWriter, r *http.Request) {

	ctx := r.Context()

	authToken, err := utils.ValidateAuthToken(r)
	if err != nil {
		utils.MakeAPIResponse(w, r, http.StatusForbidden, nil, "Invalid token", true)
	}
	uid, _ := authToken.GetUidObjectId()

	var requestData struct {
		PlotId string `json:plotId`
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

	// validate plot id
	plotId, err := utils.PlotIdFromHexString(requestData.PlotId)
	if err := utils.Validate.Struct(&requestData); err != nil {
		utils.MakeAPIResponse(w, r, http.StatusBadRequest, nil, "Invalid plot id", true)
		return
	}

	// lock plot
	key := fmt.Sprintf("lock:%s", requestData.PlotId)
	lockAquired, err := utils.RedisCli.SetNX(ctx, key, "", time.Minute*30).Result()
	if err != nil {
		log.Println(err)
		utils.LogErrorDiscord("ClaimWithCredit", err, &requestData)
		utils.MakeAPIResponse(w, r, http.StatusInternalServerError, nil, "Internal server error", true)
		return
	}

	// if already locked, return
	if !lockAquired {
		utils.MakeAPIResponse(w, r, http.StatusForbidden, nil, "Plot locked", true)
		return
	}

	//check

}
