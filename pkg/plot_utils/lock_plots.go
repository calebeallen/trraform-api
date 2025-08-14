package plotutils

import (
	"context"
	"time"
	"trraformapi/pkg/config"

	"github.com/redis/go-redis/v9"
)

var lockScript = redis.NewScript(`
local owner  = ARGV[1]
local ttl_ms = tonumber(ARGV[2])
local setkey = "claimset:" .. owner

-- 1) collect conflicts (keys owned by someone else)
local offenders = {}
for _, pid in ipairs(KEYS) do
	local k = "claimlock:" .. pid
	local cur = redis.call("GET", k)
	if cur and cur ~= owner then
		offenders[#offenders+1] = pid
	end
end
if #offenders > 0 then
	return offenders   -- list of failed locks
end

-- 2) acquire/refresh locks for this owner
for _, pid in ipairs(KEYS) do
	local k = "claimlock:" .. pid
	redis.call("SET", k, owner, "PX", ttl_ms)
end

-- 3) add plotIds to owner's set and refresh TTL (incremental semantics)
if #KEYS > 0 then
  	redis.call("SADD", setkey, unpack(KEYS))
end
redis.call("PEXPIRE", setkey, ttl_ms)

return {} -- success => empty list
`)

var unlockScript = redis.NewScript(`
local owner  = ARGV[1]
local setkey = "claimset:" .. owner

local pids = redis.call("SMEMBERS", setkey)
local n = 0
for _, pid in ipairs(pids) do
	local k = "claimlock:" .. pid
	if redis.call("GET", k) == owner then
		redis.call("DEL", k)
		n = n + 1
	end
end
redis.call("DEL", setkey)
return n
`)

func LockPlots(redisCli *redis.Client, ctx context.Context, plotIds []string, owner string) ([]string, error) {
	ttl := config.CHECKOUT_SESSION_DURATION * 2
	res, err := lockScript.Run(ctx, redisCli, plotIds, owner, ttl.Milliseconds()).Result()
	if err != nil {
		return nil, err
	}

	raw := res.([]any)
	failed := make([]string, len(raw))
	for i, v := range raw {
		failed[i] = v.(string)
	}
	return failed, nil
}

func UnlockPlots(redisCli *redis.Client, owner string) (int64, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	res, err := unlockScript.Run(ctx, redisCli, []string{}, owner).Result()
	if err != nil {
		return 0, err
	}
	return res.(int64), nil
}
