package cronjobs

import (
	"encoding/json"
	"errors"
	"net/http"
	"trraformapi/utils"
	"trraformapi/utils/schemas"

	"github.com/redis/go-redis/v9"
)

func RefreshLeaderboard(w http.ResponseWriter, r *http.Request) {

	ctx := r.Context()

	// get the current top 10 plots
	top, err := utils.RedisCli.ZRevRangeWithScores(r.Context(), "leaderboard:votes", 0, 9).Result()
	if err != nil {
		utils.LogErrorDiscord("RefreshLeaderboard", err, nil)
		utils.MakeAPIResponse(w, r, http.StatusInternalServerError, nil, "Internal server error", true)
		return
	}

	// create leaderboard
	newLeaderboard := make([]*schemas.LeaderboardEntry, len(top))
	for i, z := range top {

		newLeaderboard[i] = &schemas.LeaderboardEntry{
			PlotId: z.Member.(string),
			Votes:  z.Score,
			Dir:    1,
		}

	}

	// get the last leaderboard
	lastlb, err := utils.RedisCli.Get(ctx, "leaderboard:top").Result()

	// if there was a last leaderboard, compute directions for new leaderboard
	if err == nil {

		var lastLeaderboard []*schemas.LeaderboardEntry
		err = json.Unmarshal([]byte(lastlb), &lastLeaderboard)
		if err != nil {
			utils.LogErrorDiscord("RefreshLeaderboard", err, nil)
			utils.MakeAPIResponse(w, r, http.StatusInternalServerError, nil, "Internal server error", true)
			return
		}

		for i := range newLeaderboard {
			for j := range lastLeaderboard {

				if newLeaderboard[i].PlotId == lastLeaderboard[j].PlotId {

					diff := j - i

					if diff >= 0 {
						newLeaderboard[i].Dir = 1
					} else if diff < 0 {
						newLeaderboard[i].Dir = -1
					}

					break

				}

			}
		}

	} else if !errors.Is(err, redis.Nil) {
		utils.LogErrorDiscord("RefreshLeaderboard", err, nil)
		utils.MakeAPIResponse(w, r, http.StatusInternalServerError, nil, "Internal server error", true)
		return
	}

	data, err := json.Marshal(&newLeaderboard)
	if err != nil {
		utils.LogErrorDiscord("RefreshLeaderboard", err, nil)
		utils.MakeAPIResponse(w, r, http.StatusInternalServerError, nil, "Internal server error", true)
		return
	}

	// set new leaderboard
	err = utils.RedisCli.Set(ctx, "leaderboard:top", data, 0).Err()
	if err != nil {
		utils.LogErrorDiscord("RefreshLeaderboard", err, nil)
		utils.MakeAPIResponse(w, r, http.StatusInternalServerError, nil, "Internal server error", true)
		return
	}

	utils.MakeAPIResponse(w, r, http.StatusOK, nil, "Success", false)

}
