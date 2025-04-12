package plotutils

import (
	"context"
	"errors"
	"fmt"
	"time"
	"trraformapi/utils"

	"github.com/redis/go-redis/v9"
)

const LockDuration = 10 * time.Minute

func LockMany(ctx context.Context, plotIds []string, lockOwner string) ([]string, []string, error) {

	acquired := []string{}

	// 1) First pipeline: attempt to SetNX on each key
	pipeline := utils.RedisCli.Pipeline()

	// try to aquire lock for all plotids
	setNXCmds := make(map[string]*redis.BoolCmd, len(plotIds))
	for _, plotId := range plotIds {
		key := fmt.Sprintf("claimlock:%s", plotId)
		setNXCmds[plotId] = pipeline.SetNX(ctx, key, lockOwner, LockDuration)
	}
	if _, err := pipeline.Exec(ctx); err != nil {
		return nil, nil, fmt.Errorf("LockMany: pipeline exec for SetNX failed: %w", err)
	}

	// for each lock that wasn't acquired, mark it to be checked if it is already owned by lockOwner
	needCheck := make([]string, 0, len(plotIds))
	for plotId, cmd := range setNXCmds {
		locked, err := cmd.Result()
		if err != nil {
			return nil, nil, fmt.Errorf("LockMany: reading SetNX result for %q: %w", plotId, err)
		}
		if locked {
			acquired = append(acquired, plotId)
		} else {
			needCheck = append(needCheck, plotId)
		}
	}

	// If all locks acquired, return
	if len(needCheck) == 0 {
		return acquired, []string{}, nil
	}

	// check if lockOwner already owns the lock
	pipeline = utils.RedisCli.Pipeline()
	getCmds := make(map[string]*redis.StringCmd, len(needCheck))
	for _, plotId := range needCheck {
		key := fmt.Sprintf("claimlock:%s", plotId)
		getCmds[plotId] = pipeline.Get(ctx, key)
	}
	if _, err := pipeline.Exec(ctx); err != nil {
		return nil, nil, fmt.Errorf("LockMany: pipeline exec for GET failed: %w", err)
	}

	// separate the locks that need refresh
	var toRefresh []string
	var failed []string

	for _, plotId := range needCheck {

		owner, err := getCmds[plotId].Result()
		if errors.Is(err, redis.Nil) {
			failed = append(failed, plotId)
			continue
		} else if err != nil {
			return nil, nil, fmt.Errorf("LockMany: GET error for %q: %w", plotId, err)
		}

		if owner == lockOwner {
			toRefresh = append(toRefresh, plotId)
		} else {
			failed = append(failed, plotId)
		}

	}

	// ff none need a refresh, return
	if len(toRefresh) == 0 {
		return acquired, failed, nil
	}

	// refresh the TTL for keys that lockOwner owns
	pipeline = utils.RedisCli.Pipeline()
	expireCmds := make(map[string]*redis.BoolCmd, len(toRefresh))
	for _, plotId := range toRefresh {
		key := fmt.Sprintf("claimlock:%s", plotId)
		expireCmds[plotId] = pipeline.Expire(ctx, key, LockDuration)
	}

	if _, err := pipeline.Exec(ctx); err != nil {
		return nil, nil, fmt.Errorf("LockMany: pipeline exec for EXPIRE failed: %w", err)
	}

	for _, plotId := range toRefresh {

		ok, err := expireCmds[plotId].Result()
		if err != nil {
			return nil, nil, fmt.Errorf("LockMany: reading EXPIRE result for %q: %w", plotId, err)
		}

		if ok {
			acquired = append(acquired, plotId)
		} else {
			// if for some reason the refresh failed
			failed = append(failed, plotId)
		}

	}

	return acquired, failed, nil

}

func UnlockMany(plotIds []string, lockOwner string) error {

	ctx := context.Background()

	// Build the Redis keys we need to process
	keys := make([]string, len(plotIds))
	for i, plotID := range plotIds {
		keys[i] = fmt.Sprintf("claimlock:%s", plotID)
	}

	// Lua script: For each key in KEYS, if it’s owned by ARGV[1], then DEL it.
	// We also accumulate how many keys we actually deleted (optional).
	unlockManyScript := redis.NewScript(`
        local count = 0
        for _, key in ipairs(KEYS) do
            if redis.call("GET", key) == ARGV[1] then
                redis.call("DEL", key)
                count = count + 1
            end
        end
        return count
    `)

	// Run the script in one round trip
	err := unlockManyScript.Run(ctx, utils.RedisCli, keys, lockOwner).Err()
	if err != nil {
		// Handle or log the error as you wish
		utils.LogErrorDiscord("UnlockMany", err, struct {
			PlotIds   []string
			LockOwner string
		}{
			PlotIds:   plotIds,
			LockOwner: lockOwner,
		})
		return err
	}

	return nil

}

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

	// if not acquired, check if lockOwner is the one who holds the lock. If so, refresh the lock.
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

	// otherwise, lock acquired by someone else.
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
