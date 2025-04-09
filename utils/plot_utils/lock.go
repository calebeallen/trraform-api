package plotutils

import (
	"context"
	"fmt"
	"time"
	"trraformapi/utils"

	"github.com/redis/go-redis/v9"
)

func LockPlot(ctx context.Context, plotId string, lockOwner string) (bool, error) {

	// lock plot with 30 min deadlock prevention
	key := fmt.Sprintf("lock:%s", plotId)

	lockAquired, err := utils.RedisCli.SetNX(ctx, key, lockOwner, time.Minute*30).Result()
	if err != nil {
		return false, fmt.Errorf("in LockPlot:\n%w", err)
	}

	return lockAquired, nil

}

func UnlockPlot(plotId string, lockOwner string) {

	ctx := context.Background()
	key := fmt.Sprintf("lock:%s", plotId)

	var unlockScript = redis.NewScript(`
	if redis.call("get", KEYS[1]) == ARGV[1] then
		return redis.call("del", KEYS[1])
	else
		return 0
	end`)

	_, err := unlockScript.Run(ctx, utils.RedisCli, []string{key}, lockOwner).Result()
	if err != nil {
		args := struct {
			PlotId    string
			LockOwner string
		}{
			PlotId:    plotId,
			LockOwner: lockOwner,
		}
		utils.LogErrorDiscord("UnlockPlot", err, &args)
	}

}
