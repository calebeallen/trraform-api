package plotutils

import (
	"context"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

var lockScript = redis.NewScript(`
	local owner  = ARGV[1]
	local ttl_ms = tonumber(ARGV[2])

	-- gather all conflicts
	local offenders = {}
	for _, key in ipairs(KEYS) do
		local cur = redis.call("GET", key)
		if cur and cur ~= owner then
			table.insert(offenders, key)
		end
	end

	-- if any conflicts, abort
	if #offenders > 0 then
		return {0, offenders}
	end

	-- lock
	for _, key in ipairs(KEYS) do
		redis.call("SET", key, owner, "PX", ttl_ms)
	end

	return {1, #KEYS}
`)

var unlockScript = redis.NewScript(`
	local owner = ARGV[1]

	-- validate owner owns all plots
	for _, key in ipairs(KEYS) do
		local cur = redis.call("GET", key)
		if cur and cur ~= owner then
			return 0
		end
	end

	-- unlock
	for _, key in ipairs(KEYS) do
		redis.call("DEL", key)
	end

	return 1
`)

func LockPlots(redisCli *redis.Client, ctx context.Context, plotIds []string, owner string) ([]string, error) {
	keys := make([]string, len(plotIds))
	for i, id := range plotIds {
		keys[i] = "claimlock:" + id
	}

	ttl := time.Hour
	res, err := lockScript.Run(ctx, redisCli, keys, owner, ttl.Milliseconds()).Result()
	if err != nil {
		return nil, err
	}
	out := res.([]any)
	ok := out[0].(int64)
	if ok == 0 {

		raw := out[1].([]any)
		failed := make([]string, len(raw))
		for i, v := range raw {
			failed[i] = v.(string)
		}

		return failed, nil

	}
	return nil, nil
}

func UnlockPlots(redisCli *redis.Client, plotIds []string, owner string) (bool, error) {
	ctx := context.Background()

	keys := make([]string, len(plotIds))
	for i, id := range plotIds {
		keys[i] = "claimlock:" + id
	}

	res, err := unlockScript.Run(ctx, redisCli, keys, owner).Result()
	if err != nil {
		return false, err
	}
	okVal, ok := res.(int64)
	if !ok {
		return false, fmt.Errorf("unexpected lua reply: %#v", res)
	}
	if okVal == 0 {
		return false, nil
	}
	return true, nil
}
