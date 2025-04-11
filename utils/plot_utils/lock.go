package plotutils

import (
	"context"
	"errors"
	"fmt"
	"time"
	"trraformapi/utils"

	"github.com/redis/go-redis/v9"
)

const LockDuration = 15 * time.Minute

func LockPlot(ctx context.Context, plotId string, lockOwner string) (bool, error) {

	key := fmt.Sprintf("claimlock:%s", plotId)

	// try to acquire lock
	lockAcquired, err := utils.RedisCli.SetNX(ctx, key, lockOwner, LockDuration).Result()
	if err != nil {
		return false, fmt.Errorf("in LockPlot (SetNX): %w", err)
	}

	if lockAcquired {
		return true, nil
	}

	// if not aquired, check if lockOwner is the one who holds the lock. If so, refresh the lock.
	currentOwner, err := utils.RedisCli.Get(ctx, key).Result()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return false, nil
		}
		return false, fmt.Errorf("in LockPlot: %w", err)
	}

	if currentOwner == lockOwner {
		// Refresh TTL since the same owner is extending it
		ok, err := utils.RedisCli.Expire(ctx, key, LockDuration).Result()
		if err != nil {
			return false, fmt.Errorf("in LockPlot: %w", err)
		}
		return ok, nil

	}

	// otherwise, lock aquired by someone else.
	return false, nil

}

func UnlockPlot(plotId string, lockOwner string) {

	ctx := context.Background()
	key := fmt.Sprintf("claimlock:%s", plotId)

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
