package user

import (
	"net/http"
	"trraformapi/internal/api"
	"trraformapi/pkg/schemas"
	"trraformapi/pkg/utils"

	"go.mongodb.org/mongo-driver/v2/bson"
)

func (h *Handler) GetUserData(w http.ResponseWriter, r *http.Request) {

	resParams := &api.ResParams{W: w, R: r}
	ctx := r.Context()

	authToken, err := utils.ValidateAuthToken(r)
	if err != nil {
		resParams.Err = err
		resParams.Code = http.StatusUnauthorized
		h.Res(resParams)
		return
	}

	uid, err := authToken.GetUidObjectId()
	if err != nil {
		resParams.Err = err
		resParams.Code = http.StatusInternalServerError
		h.Res(resParams)
		return
	}

	// refresh token if expiring soon
	authToken.Refresh()
	token, err := authToken.Sign()
	if err != nil {
		resParams.Err = err
		resParams.Code = http.StatusInternalServerError
		h.Res(resParams)
		return
	}

	// get user data
	var user schemas.User
	if err := h.MongoDB.Collection("users").FindOne(ctx, bson.M{"_id": uid}).Decode(&user); err != nil {
		resParams.Err = err
		resParams.Code = http.StatusInternalServerError
		h.Res(resParams)
		return
	}

	resParams.ResData = &struct {
		Token       string            `json:"token"`
		Username    string            `json:"username"`
		SubActive   bool              `json:"subActive"`
		HasFreePlot bool              `json:"hasFreePlot"`
		PlotCredits int               `json:"plotCredits"`
		PlotIds     []string          `json:"plotIds"`
		Offenses    []schemas.Offense `json:"offenses"`
	}{
		Token:       token,
		Username:    user.Username,
		SubActive:   user.Subscription.IsActive,
		HasFreePlot: user.FreePlot == "",
		PlotCredits: user.PlotCredits,
		PlotIds:     user.PlotIds,
		Offenses:    user.Offenses,
	}
	resParams.Code = http.StatusOK
	h.Res(resParams)

}
