package utils

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"math/big"
	"time"

	"github.com/redis/go-redis/v9"
)

var ErrUnusedVerificationCode = errors.New("unused valid code")
var ErrVerificationCodeNotFound = errors.New("verification code not found")

var validateScript = redis.NewScript(`
	if redis.call("GET", KEYS[1]) == ARGV[1] then
		redis.call("DEL", KEYS[1])
		return 1
	end
	return 0
`)

func NewVerificationCode(ctx context.Context, uid string) (string, error) {

	// if a code already exist
	key := "vercode:" + uid
	n, err := rand.Int(rand.Reader, big.NewInt(1_000_000))
	if err != nil {
		return "", err
	}
	code := fmt.Sprintf("%06d", uint32(n.Uint64()))

	// set new code if not already set
	ok, err := RedisCli.SetNX(ctx, key, code, 30*time.Minute).Result()
	if err != nil {
		return "", fmt.Errorf("in SetVerificationCode (0):\n%w", err)
	}
	if !ok {
		return "", ErrUnusedVerificationCode
	}

	return code, nil

}

func GetVerificationCodeByUid(ctx context.Context, uid string) (string, error) {

	key := "vercode:" + uid
	code, err := RedisCli.Get(ctx, key).Result()

	if err == redis.Nil {
		return "", ErrVerificationCodeNotFound
	} else if err != nil {
		return "", fmt.Errorf("in GetVerificationCodeByUid (0):\n%w", err)
	}

	return code, nil

}

func ValidateVerificationCode(ctx context.Context, uid string, code string) (bool, error) {

	key := "vercode:" + uid

	res, err := validateScript.Run(ctx, RedisCli, []string{key}, code).Int()
	if err != nil {
		return false, fmt.Errorf("in ValidateVerificationCode (0):\n%w", err)
	}
	if res == 1 {
		return true, nil
	}

	return false, nil

}
