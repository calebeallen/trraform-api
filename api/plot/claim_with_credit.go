package plot

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"trraformapi/utils"
	plotutils "trraformapi/utils/plot_utils"
	"trraformapi/utils/schemas"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

func ClaimWithCredit(w http.ResponseWriter, r *http.Request) {

	ctx := r.Context()

	authToken, err := utils.ValidateAuthToken(r)
	if err != nil {
		utils.MakeAPIResponse(w, r, http.StatusForbidden, nil, "Invalid token", true)
		return
	}
	uid, _ := authToken.GetUidObjectId()

	var requestData struct {
		PlotId string `json:"plotId" validate:"required"`
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
	uidString := uid.Hex()

	// lock plot to prevent duplicate claims
	lockAquired, err := plotutils.LockPlot(ctx, plotIdStr, uidString)
	if err != nil {
		if !errors.Is(err, context.Canceled) {
			utils.LogErrorDiscord("ClaimWithCredit", err, &requestData)
		}
		utils.MakeAPIResponse(w, r, http.StatusInternalServerError, nil, "Internal server error", true)
		return
	}
	defer plotutils.UnlockPlot(plotIdStr, uidString)

	// if already locked, return
	if !lockAquired {
		utils.MakeAPIResponse(w, r, http.StatusForbidden, nil, "Plot locked", true)
		return
	}

	// verify that plot isn't claimed
	if utils.HasObjectR2("plots", plotIdStr+".dat", ctx) {
		utils.MakeAPIResponse(w, r, http.StatusForbidden, nil, "Plot already claimed", true)
		return
	}

	usersCollection := utils.MongoDB.Collection("users")

	// add plot id to user's list, decrement plotCredits
	// this requires that the user has more than 0 credits left
	res := usersCollection.FindOneAndUpdate(ctx,
		bson.M{
			"_id":         uid,
			"plotCredits": bson.M{"$gt": 0},
		},
		bson.M{
			"$inc":  bson.M{"plotCredits": -1},
			"$push": bson.M{"plotIds": plotIdStr},
		},
		options.FindOneAndUpdate().SetReturnDocument(options.After),
	)
	var user schemas.User
	err = res.Decode(&user)
	if errors.Is(err, mongo.ErrNoDocuments) {
		utils.MakeAPIResponse(w, r, http.StatusForbidden, nil, "User has 0 credits available", true)
		return
	} else if err != nil {
		if !errors.Is(err, context.Canceled) {
			utils.LogErrorDiscord("ClaimWithCredit", err, &requestData)
		}
		utils.MakeAPIResponse(w, r, http.StatusInternalServerError, nil, "Internal server error", true)
		return
	}

	// create plot with default data
	err = plotutils.SetDefaultPlot(ctx, plotId, &user)
	if err != nil {
		if !errors.Is(err, context.Canceled) {
			utils.LogErrorDiscord("ClaimWithCredit", err, &requestData)
		}
		utils.MakeAPIResponse(w, r, http.StatusInternalServerError, nil, "Internal server error", true)
		return
	}

	// remove plotId from available plots
	depth := plotId.Depth()
	err = utils.RedisCli.SRem(ctx, fmt.Sprintf("openplots:%d", depth), plotIdStr).Err()
	if err != nil {
		if !errors.Is(err, context.Canceled) {
			utils.LogErrorDiscord("ClaimWithCredit", err, &requestData)
		}
		utils.MakeAPIResponse(w, r, http.StatusInternalServerError, nil, "Internal server error", true)
		return
	}

	// add plot's children (if it has any) to available plots
	if depth < utils.MaxDepth {

		childIds := make([]any, utils.SubplotCount)
		for i := 0; i < utils.SubplotCount; i++ {
			childId := plotutils.CreateSubplotId(plotId, uint64(i+1))
			childIds[i] = childId.ToString()
		}

		err = utils.RedisCli.SAdd(ctx, fmt.Sprintf("openplots:%d", depth+1), childIds...).Err()
		if err != nil {
			if !errors.Is(err, context.Canceled) {
				utils.LogErrorDiscord("ClaimWithCredit", err, &requestData)
			}
			utils.MakeAPIResponse(w, r, http.StatusInternalServerError, nil, "Internal server error", true)
			return
		}

	}

	utils.MakeAPIResponse(w, r, http.StatusOK, nil, "Success", false)

}
