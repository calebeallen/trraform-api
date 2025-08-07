package plot

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"trraformapi/internal/api"
	"trraformapi/utils"
	plotutils "trraformapi/utils/plot_utils"
	"trraformapi/utils/schemas"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
)

func (h *Handler) UpdatePlot(w http.ResponseWriter, r *http.Request) {

	defer r.Body.Close()
	ctx := r.Context()
	uid := ctx.Value("uid").(bson.ObjectID)
	resParams := &api.ResParams{W: w, R: r}

	var reqData struct {
		PlotId      string `json:"plotId" validate:"required"`
		Name        string `json:"name"`
		Description string `json:"description"`
		Link        string `json:"link"`
		LinkTitle   string `json:"linkTitle"`
		BuildData   string `json:"buildData" validate:"required,base64"`
	}

	// validate request body
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&reqData); err != nil {
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

	// validate plot id
	plotId, err := plotutils.PlotIdFromHexString(reqData.PlotId)
	if err != nil || !plotId.Validate() {
		resParams.Code = http.StatusBadRequest
		resParams.Err = err
		h.Res(resParams)
		return
	}
	plotIdStr := plotId.ToString()

	// decode base64 buildData
	buildDataBytes, _ := base64.StdEncoding.DecodeString(reqData.BuildData)
	buildData, err := utils.BytesToUint16Arr(buildDataBytes)
	if err != nil {
		resParams.Code = http.StatusBadRequest
		resParams.Err = err
		h.Res(resParams)
		return
	}

	// get user data, check that user owns plot
	var user schemas.User
	if err := h.MongoDB.Collection("users").FindOne(ctx, bson.M{
		"_id":     uid,
		"plotIds": plotIdStr,
	}).Decode(&user); err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			resParams.Code = http.StatusUnauthorized
		} else {
			resParams.Code = http.StatusInternalServerError
		}
		resParams.Err = err
		h.Res(resParams)
		return
	}

	// create plot data (don't set verified status here)
	plotData := plotutils.PlotData{
		Name:        reqData.Name,
		Description: reqData.Description,
		Link:        reqData.Link,
		LinkTitle:   reqData.LinkTitle,
		Owner:       user.Username,
		BuildData:   buildData,
	}

	// validate plot data
	if err := utils.Validate.Struct(&plotData); err != nil {
		resParams.Code = http.StatusBadRequest
		resParams.Err = err
		h.Res(resParams)
		return
	}

	// check that plot is within build size constraints for subscription status
	// link and large build size only allowed for subscribed users
	buildSize := buildData[1]
	if buildSize < utils.MinBuildSize || buildSize > utils.BuildSizeLarge || (!user.Subscription.IsActive && (plotData.Link != "" || plotData.LinkTitle != "" || buildSize > utils.BuildSizeStd)) {
		resParams.Code = http.StatusUnauthorized
		resParams.Err = err
		h.Res(resParams)
		return
	}

	// encode plot data
	plotDataBytes, err := plotData.Encode()
	if err != nil {
		resParams.Code = http.StatusInternalServerError
		resParams.Err = err
		h.Res(resParams)
		return
	}

	// upload plot data
	err = utils.PutObjectR2(ctx, "plots", plotIdStr+".dat", bytes.NewReader(plotDataBytes), "application/octet-stream")
	if err != nil {
		if !errors.Is(err, context.Canceled) {
			utils.LogErrorDiscord("UpdatePlot", err, &reqData)
		}
		log.Printf("Error uploading plot data:\n%v", err)
		utils.MakeAPIResponse(w, r, http.StatusInternalServerError, nil, "Internal server error", true)
		return
	}

	// flag chunk for update
	err = plotutils.FlagPlotForUpdate(ctx, plotId)
	if err != nil {
		if !errors.Is(err, context.Canceled) {
			utils.LogErrorDiscord("UpdatePlot", err, &reqData)
		}
		log.Printf("Error flagging chunk for update:\n%v", err)
		utils.MakeAPIResponse(w, r, http.StatusInternalServerError, nil, "Internal server error", true)
		return
	}

	utils.MakeAPIResponse(w, r, http.StatusOK, nil, "Success", false)

}
