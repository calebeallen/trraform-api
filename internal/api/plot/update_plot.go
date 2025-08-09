package plot

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"trraformapi/internal/api"
	"trraformapi/pkg/config"
	plotutils "trraformapi/pkg/plot_utils"
	"trraformapi/pkg/schemas"
	"trraformapi/pkg/utils"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
)

func (h *Handler) UpdatePlot(w http.ResponseWriter, r *http.Request) {

	defer r.Body.Close()
	ctx := r.Context()
	uid := ctx.Value("uid").(bson.ObjectID)
	resParams := &api.ResParams{W: w, R: r}

	var reqData struct {
		PlotId      string `json:"plotId" validate:"required,plotid"`
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
	if err := h.Validate.Struct(&plotData); err != nil {
		resParams.Code = http.StatusBadRequest
		resParams.Err = err
		h.Res(resParams)
		return
	}

	// check that plot is within build size constraints for subscription status
	// link and large build size only allowed for subscribed users
	buildSize := buildData[1]
	if buildSize < config.MIN_BUILD_SIZE || buildSize > config.LRG_BUILD_SIZE || (!user.Subscription.IsActive && (plotData.Link != "" || plotData.LinkTitle != "" || buildSize > config.STD_BUILD_SIZE)) {
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
	metadata := map[string]string{"verified": strconv.FormatBool(user.Subscription.IsActive)}
	if err := utils.PutObjectR2(h.R2Cli, ctx, config.CF_PLOT_BUCKET, plotIdStr+".dat", bytes.NewReader(plotDataBytes), "application/octet-stream", metadata); err != nil {
		resParams.Code = http.StatusInternalServerError
		resParams.Err = err
		h.Res(resParams)
		return
	}

	if err := plotutils.FlagPlotForUpdate(h.RedisCli, ctx, plotId); err != nil {
		resParams.Code = http.StatusInternalServerError
		resParams.Err = err
		h.Res(resParams)
		return
	}

	resParams.Code = http.StatusOK
	h.Res(resParams)

}
