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

var ErrUnusedVerificationCode = errors.New("attempted to create a verification code when an unused valid code already exists")
var ErrVerificationCodeNotFound = errors.New("verification code not found")

var validateScript = redis.NewScript(`
	if redis.call("GET", KEYS[1]) == ARGV[1] then
		redis.call("DEL", KEYS[1])
		return 1
	end
	return 0
`)

func NewVerificationCode(redisCli *redis.Client, ctx context.Context, email string) (string, error) {

	// if a code already exist
	key := "vercode:" + email
	n, err := rand.Int(rand.Reader, big.NewInt(1_000_000))
	if err != nil {
		return "", err
	}
	code := fmt.Sprintf("%06d", uint32(n.Uint64()))

	// set new code if not already set
	ok, err := redisCli.SetNX(ctx, key, code, 30*time.Minute).Result()
	if err != nil {
		return "", err
	}
	if !ok {
		return "", ErrUnusedVerificationCode
	}

	return code, nil

}

func GetVerificationCode(redisCli *redis.Client, ctx context.Context, email string) (string, error) {

	key := "vercode:" + email
	code, err := redisCli.Get(ctx, key).Result()

	if err == redis.Nil {
		return "", ErrVerificationCodeNotFound
	} else if err != nil {
		return "", err
	}

	return code, nil

}

func ValidateVerificationCode(redisCli *redis.Client, ctx context.Context, email string, code string) (bool, error) {
	key := "vercode:" + email

	res, err := validateScript.Run(ctx, redisCli, []string{key}, code).Int()
	if err != nil {
		return false, err
	}
	if res == 1 {
		return true, nil
	}

	return false, nil
}
