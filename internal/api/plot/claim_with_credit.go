package plot

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"time"
	"trraformapi/internal/api"
	"trraformapi/pkg/config"
	plotutils "trraformapi/pkg/plot_utils"
	"trraformapi/pkg/schemas"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
	"go.mongodb.org/mongo-driver/v2/mongo/readconcern"
	"go.mongodb.org/mongo-driver/v2/mongo/writeconcern"
)

func (h *Handler) ClaimWithCredit(w http.ResponseWriter, r *http.Request) {

	defer r.Body.Close()
	ctx := r.Context()
	uid := ctx.Value("uid").(bson.ObjectID)
	resParams := &api.ResParams{W: w, R: r}

	var reqData struct {
		PlotId string `json:"plotId" validate:"required,plotid"`
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

	plotId, _ := plotutils.PlotIdFromHexString(reqData.PlotId)
	plotIdStr := plotId.ToString()
	uidString := uid.Hex()

	// lock plot to prevent duplicate claims
	failedIds, err := plotutils.LockPlots(h.RedisCli, ctx, []string{plotIdStr}, uidString)
	if err != nil {
		resParams.Code = http.StatusInternalServerError
		resParams.Err = err
		h.Res(resParams)
		return
	}
	if len(failedIds) > 0 {
		resParams.ResData = &struct {
			Conflict bool `json:"conflict"`
		}{Conflict: true}
		resParams.Code = http.StatusConflict
		h.Res(resParams)
		return
	}
	defer plotutils.UnlockPlots(h.RedisCli, []string{plotIdStr}, uidString)

	// create transaction session
	txSession, err := h.MongoDB.Client().StartSession()
	if err != nil {
		resParams.Code = http.StatusInternalServerError
		resParams.Err = err
		h.Res(resParams)
		return
	}
	defer txSession.EndSession(ctx)
	txOpts := options.Transaction().SetReadConcern(readconcern.Snapshot()).SetWriteConcern(writeconcern.Majority())

	var updatedUser schemas.User

	_, err = txSession.WithTransaction(ctx, func(txCtx context.Context) (interface{}, error) {

		// handle plot credits
		err := h.MongoDB.Collection("users").FindOneAndUpdate(txCtx,
			bson.M{
				"_id":         uid,
				"plotCredits": bson.M{"$gt": 0},
			},
			bson.M{
				"$inc":      bson.M{"plotCredits": -1},
				"$addToSet": bson.M{"plotIds": plotIdStr},
			},
			options.FindOneAndUpdate().SetReturnDocument(options.After),
		).Decode(&updatedUser)
		if err != nil {
			return nil, err
		}

		// claim plot
		now := time.Now().UTC()
		plotEntry := schemas.Plot{
			PlotId:    plotIdStr,
			Ctime:     now,
			Owner:     uid,
			Purchased: false,
			ClaimedAt: now,
			Default:   true,
		}
		if _, err := h.MongoDB.Collection("plots").InsertOne(txCtx, &plotEntry); err != nil {
			return nil, err
		}

		return nil, nil

	}, txOpts)

	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) || mongo.IsDuplicateKeyError(err) {
			resParams.ResData = &struct {
				Conflict bool `json:"conflict"`
			}{Conflict: true}
			resParams.Code = http.StatusConflict
		} else {
			resParams.Code = http.StatusInternalServerError
		}
		resParams.Err = err
		h.Res(resParams)
		return
	}

	// flag for update
	if err := plotutils.FlagPlotForUpdate(ctx, plotId); err != nil {
		resParams.Code = http.StatusInternalServerError
		resParams.Err = err
		h.Res(resParams)
		return
	}

	// remove plotId from available plots
	depth := plotId.Depth()
	err = h.RedisCli.SRem(ctx, fmt.Sprintf("openplots:%d", depth), plotIdStr).Err()
	if err != nil {
		resParams.Code = http.StatusInternalServerError
		resParams.Err = err
		h.Res(resParams)
		return
	}

	// add plot's children (if it has any) to available plots
	if depth < config.VAR.MAX_DEPTH {

		childIds := make([]any, config.VAR.SUBPLOT_COUNT)
		for i := 0; i < config.VAR.SUBPLOT_COUNT; i++ {
			childId := plotutils.CreateSubplotId(plotId, uint64(i+1))
			childIds[i] = childId.ToString()
		}

		err = h.RedisCli.SAdd(ctx, fmt.Sprintf("openplots:%d", depth+1), childIds...).Err()
		if err != nil {
			resParams.Code = http.StatusInternalServerError
			resParams.Err = err
			h.Res(resParams)
			return
		}

	}

	resParams.Code = http.StatusOK
	h.Res(resParams)

}
