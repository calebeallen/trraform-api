package leaderboard

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"trraformapi/utils"
	"trraformapi/utils/schemas"

	"github.com/redis/go-redis/v9"
)

func GetLeaderboard(w http.ResponseWriter, r *http.Request) {

	lb, err := utils.RedisCli.Get(r.Context(), "leaderboard:top").Result()

	var leaderboard []*schemas.LeaderboardEntry

	if err == nil {

		err = json.Unmarshal([]byte(lb), &leaderboard)
		if err != nil {
			utils.LogErrorDiscord("GetLeaderboard", err, nil)
			utils.MakeAPIResponse(w, r, http.StatusInternalServerError, nil, "Internal server error", true)
			return
		}

	} else if errors.Is(err, redis.Nil) {

		leaderboard = make([]*schemas.LeaderboardEntry, 0)

	} else {

		if !errors.Is(err, context.Canceled) {
			utils.LogErrorDiscord("GetLeaderboard", err, nil)
		}
		utils.MakeAPIResponse(w, r, http.StatusInternalServerError, nil, "Internal server error", true)
		return

	}

	utils.MakeAPIResponse(w, r, http.StatusOK, leaderboard, "Success", false)

}
