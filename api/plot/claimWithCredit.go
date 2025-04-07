package plot

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"
	"trraformapi/utils"
	plotutils "trraformapi/utils/plotUtils"
	"trraformapi/utils/schemas"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
)

func ClaimWithCredit(w http.ResponseWriter, r *http.Request) {

	ctx := r.Context()

	authToken, err := utils.ValidateAuthToken(r)
	if err != nil {
		utils.MakeAPIResponse(w, r, http.StatusForbidden, nil, "Invalid token", true)
	}
	uid, _ := authToken.GetUidObjectId()

	var requestData struct {
		PlotId string `json:"plotId"`
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
	if err != nil || !plotId.Verify() {
		utils.MakeAPIResponse(w, r, http.StatusBadRequest, nil, "Invalid request body", true)
		return
	}
	plotIdStr := plotId.ToString()

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

	plotsCollection := utils.MongoDB.Collection("plots")
	usersCollection := utils.MongoDB.Collection("users")

	// verify that plot doesn't exist
	err = plotsCollection.FindOne(ctx, bson.M{"plotId": plotIdStr}).Decode(&schemas.Plot{})
	if err == nil || err != mongo.ErrNoDocuments {
		utils.MakeAPIResponse(w, r, http.StatusForbidden, nil, "Plot already claimed", true)
		return
	}

	// get user data
	var user schemas.User
	err = usersCollection.FindOne(ctx, bson.M{"_id": uid}).Decode(&user)
	if err != nil {
		log.Println(err)
		utils.LogErrorDiscord("ClaimWithCredit", err, &requestData)
		utils.MakeAPIResponse(w, r, http.StatusInternalServerError, nil, "Internal server error", true)
		return
	}

	//if user has no plot credits, return
	if user.PlotCredits <= 0 {
		utils.MakeAPIResponse(w, r, http.StatusForbidden, nil, "User has 0 plot credits", true)
		return
	}

	// create plot with default data
	err = plotutils.SetDefaultPlotData(ctx, plotId, &user)
	if err != nil {
		log.Println(err)
		utils.LogErrorDiscord("ClaimWithCredit", err, &requestData)
		utils.MakeAPIResponse(w, r, http.StatusInternalServerError, nil, "Internal server error", true)
		return
	}

	// append plotId to user's list of plotId, use plot credit
	_, err = usersCollection.UpdateOne(ctx, bson.M{"_id": uid}, bson.M{
		"$push": bson.M{
			"plotIds": plotIdStr,
		},
		"$inc": bson.M{
			"plotCredits": -1,
		},
	})
	if err != nil {
		log.Println(err)
		utils.LogErrorDiscord("ClaimWithCredit", err, &requestData)
		utils.MakeAPIResponse(w, r, http.StatusInternalServerError, nil, "Internal server error", true)
		return
	}

	// clear lock
	_, err = utils.RedisCli.Del(ctx, key).Result()
	if err != nil {
		log.Println(err)
		utils.LogErrorDiscord("ClaimWithCredit", err, &requestData)
		utils.MakeAPIResponse(w, r, http.StatusInternalServerError, nil, "Internal server error", true)
		return
	}

	utils.MakeAPIResponse(w, r, http.StatusOK, nil, "Success", false)

}
