package leaderboard

import (
	"encoding/json"
	"errors"
	"net/http"
	"trraformapi/internal/api"
	"trraformapi/pkg/schemas"

	"github.com/redis/go-redis/v9"
)

func (h *Handler) GetLeaderboard(w http.ResponseWriter, r *http.Request) {

	resParams := &api.ResParams{W: w, R: r}

	lb, err := h.RedisCli.Get(r.Context(), "leaderboard:top").Result()
	var leaderboard []*schemas.LeaderboardEntry
	if errors.Is(err, redis.Nil) { //no leaderboard yet
		resParams.ResData = leaderboard
		resParams.Code = http.StatusOK
		h.Res(resParams)
		return
	} else if err != nil {
		resParams.Code = http.StatusInternalServerError
		resParams.Err = err
		h.Res(resParams)
		return
	}

	err = json.Unmarshal([]byte(lb), &leaderboard)
	if err != nil {
		resParams.Code = http.StatusInternalServerError
		resParams.Err = err
		h.Res(resParams)
		return
	}

	h.Res(resParams)
	return

}
