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
		PlotId string `json:"plotId" validate:"required,plotid"`
	}

	var responseData struct {
		Conflict bool `json:"conflict"`
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
	plotIdStr := plotId.ToString()
	uidString := uid.Hex()

	// lock plot to prevent duplicate claims
	lockAcquired, err := plotutils.LockPlot(ctx, plotIdStr, uidString)
	if err != nil {
		if !errors.Is(err, context.Canceled) {
			utils.LogErrorDiscord("ClaimWithCredit", err, &requestData)
		}
		utils.MakeAPIResponse(w, r, http.StatusInternalServerError, nil, "Internal server error", true)
		return
	}
	defer plotutils.UnlockPlot(plotIdStr, uidString)

	// if already locked, return
	if !lockAcquired {
		responseData.Conflict = true
		utils.MakeAPIResponse(w, r, http.StatusForbidden, &responseData, "Plot locked", true)
		return
	}

	// verify that plot isn't claimed
	claimed, err := utils.HasObjectR2(ctx, "plots", plotIdStr+".dat")
	if err != nil {
		if !errors.Is(err, context.Canceled) {
			utils.LogErrorDiscord("ClaimWithCredit", err, &requestData)
		}
		utils.MakeAPIResponse(w, r, http.StatusInternalServerError, nil, "Internal server error", true)
		return
	}
	if claimed {
		responseData.Conflict = true
		utils.MakeAPIResponse(w, r, http.StatusForbidden, &responseData, "Plot already claimed", true)
		return
	}

	usersCollection := utils.MongoDB.Collection("users")

	// add plot id to user's list, decrement plotCredits
	// this requires that the user has more than 0 credits left
	// also check that plotId is not already in their list of ids. Could lead to duplicate entry for user.
	res := usersCollection.FindOneAndUpdate(ctx,
		bson.M{
			"_id":         uid,
			"plotCredits": bson.M{"$gt": 0},
			"plotIds":     bson.M{"$ne": plotIdStr},
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
		for i := range utils.SubplotCount {
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

	utils.MakeAPIResponse(w, r, http.StatusOK, responseData, "Success", false)

}
