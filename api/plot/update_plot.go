package plot

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"os"
	"trraformapi/utils"
	plotutils "trraformapi/utils/plot_utils"
	"trraformapi/utils/schemas"

	"slices"

	"go.mongodb.org/mongo-driver/bson"
)

func UpdatePlot(w http.ResponseWriter, r *http.Request) {

	ctx := r.Context()

	authToken, err := utils.ValidateAuthToken(r)
	if err != nil {
		utils.MakeAPIResponse(w, r, http.StatusForbidden, nil, "Invalid token", true)
		return
	}
	uid, _ := authToken.GetUidObjectId()

	var requestData struct {
		PlotId      string `json:"plotId" validate:"required"`
		Name        string `json:"name"`
		Description string `json:"description"`
		Link        string `json:"link"`
		LinkTitle   string `json:"linkTitle"`
		BuildData   string `json:"buildData" validate:"required,base64"`
		ImageData   string `json:"imageData"`
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
	plotId, err := plotutils.PlotIdFromHexString(requestData.PlotId)
	if err != nil || !plotId.Validate() {
		utils.MakeAPIResponse(w, r, http.StatusBadRequest, nil, "Invalid request body", true)
		return
	}
	plotIdStr := plotId.ToString()

	// decode base64 buildData
	buildDataBytes, _ := base64.StdEncoding.DecodeString(requestData.BuildData)
	buildData, err := utils.BytesToUint16Arr(buildDataBytes)
	if err != nil {
		utils.MakeAPIResponse(w, r, http.StatusBadRequest, nil, "Invalid request body", true)
		return
	}

	usersCollection := utils.MongoDB.Collection("users")

	// get user data
	var user schemas.User
	err = usersCollection.FindOne(ctx, bson.M{"_id": uid}).Decode(&user)
	if err != nil {
		if !errors.Is(err, context.Canceled) {
			utils.LogErrorDiscord("UpdatePlot", err, &requestData)
		}
		utils.MakeAPIResponse(w, r, http.StatusInternalServerError, nil, "Internal server error", true)
		return
	}

	// check that user owns plot
	if !slices.Contains(user.PlotIds, plotIdStr) {
		utils.MakeAPIResponse(w, r, http.StatusForbidden, nil, "User does not own this plot", true)
		return
	}

	// create plot data
	plotData := plotutils.PlotData{
		Name:        requestData.Name,
		Description: requestData.Description,
		Link:        requestData.Link,
		LinkTitle:   requestData.LinkTitle,
		Verified:    user.Subscribed,
		Owner:       user.Username,
		BuildData:   buildData,
	}

	// validate plot data
	if err := utils.Validate.Struct(&plotData); err != nil {
		utils.MakeAPIResponse(w, r, http.StatusBadRequest, nil, "Invalid plot data", true)
		return
	}

	// check that plot is within build size constraints for subscription status
	// link and large build size only allowed for subscribed users
	buildSize := buildData[1]
	if buildSize > utils.BuildSizeLarge || (!user.Subscribed && (plotData.Link != "" || plotData.LinkTitle != "" || buildSize > utils.BuildSizeStd)) {
		utils.MakeAPIResponse(w, r, http.StatusForbidden, nil, "Plot data has unauthorized attributes", true)
		return
	}

	// encode plot data
	plotDataBytes, err := plotData.Encode()
	if err != nil {
		if !errors.Is(err, context.Canceled) {
			utils.LogErrorDiscord("UpdatePlot", err, &requestData)
		}
		utils.MakeAPIResponse(w, r, http.StatusInternalServerError, nil, "Internal server error", true)
		return
	}

	var imageData io.Reader
	if requestData.ImageData == "" {

		imageBytes, err := os.ReadFile("static/default_img.png")
		if err != nil {
			if !errors.Is(err, context.Canceled) {
				utils.LogErrorDiscord("UpdatePlot", err, &requestData)
			}
			utils.MakeAPIResponse(w, r, http.StatusInternalServerError, nil, "Internal server error", true)
			return
		}
		imageData = bytes.NewReader(imageBytes)

	} else {

		// decode base64 imageData
		imageDataBytes, _ := base64.StdEncoding.DecodeString(requestData.ImageData)

		// create image data
		imageData, err = plotutils.CreateBuildImage(imageDataBytes)
		if err != nil {
			utils.MakeAPIResponse(w, r, http.StatusBadRequest, "", "Invalid build image data", true)
			return
		}

	}

	// upload plot data
	err = utils.PutObjectR2("plots", plotIdStr+".dat", bytes.NewReader(plotDataBytes), "application/octet-stream", ctx)
	if err != nil {
		if !errors.Is(err, context.Canceled) {
			utils.LogErrorDiscord("UpdatePlot", err, &requestData)
		}
		utils.MakeAPIResponse(w, r, http.StatusInternalServerError, nil, "Internal server error", true)
		return
	}

	// upload plot image
	err = utils.PutObjectR2("images", plotIdStr+".png", imageData, "image/png", ctx)
	if err != nil {
		if !errors.Is(err, context.Canceled) {
			utils.LogErrorDiscord("UpdatePlot", err, &requestData)
		}
		utils.MakeAPIResponse(w, r, http.StatusInternalServerError, nil, "Internal server error", true)
		return
	}

	//TODO: flag chunk for update

	utils.MakeAPIResponse(w, r, http.StatusOK, nil, "Success", false)

}
