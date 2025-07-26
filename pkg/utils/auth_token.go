package utils

import (
	"errors"
	"net/http"
	"strings"
	"time"
	"trraformapi/pkg/config"

	"github.com/golang-jwt/jwt/v5"
	"go.mongodb.org/mongo-driver/v2/bson"
)

type AuthToken struct {
	Uid string `json:"uid"`
	jwt.RegisteredClaims
}

func CreateNewAuthToken(uid bson.ObjectID) *AuthToken {

	token := AuthToken{Uid: uid.Hex()}
	token.refreshToken()
	return &token

}

func ValidateAuthToken(r *http.Request) (*AuthToken, error) {

	header := r.Header.Get("Authorization")
	if header == "" {
		return nil, errors.New("missing token")
	}

	parts := strings.Split(header, " ")
	if len(parts) != 2 {
		return nil, errors.New("invalid token format")
	}
	token_raw := parts[1]

	// validate token
	var authToken AuthToken
	token, err := jwt.ParseWithClaims(token_raw, &authToken, func(token *jwt.Token) (any, error) {
		return []byte(config.VAR.JWT_SECRET), nil
	})
	if err != nil {
		return nil, err
	}
	if !token.Valid {
		return nil, errors.New("invalid token")
	}

	// error if expired
	if time.Now().UTC().After(authToken.ExpiresAt.Time) {
		return nil, errors.New("token expired")
	}

	return &authToken, nil

}

func (authToken *AuthToken) Sign() (string, error) {

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, authToken)
	key := []byte(config.VAR.JWT_SECRET)
	signed, err := token.SignedString(key)
	if err != nil {
		return "", err
	}

	return "Bearer " + signed, nil

}

func (authToken *AuthToken) GetUidObjectId() (bson.ObjectID, error) {
	return bson.ObjectIDFromHex(authToken.Uid)
}

func (authToken *AuthToken) Refresh() {

	//if expiring in < 3 month refresh token
	timeTillExpire := authToken.ExpiresAt.Sub(time.Now().UTC())
	if timeTillExpire <= time.Hour*24*7*4*3 {
		authToken.refreshToken()
	}

}

func (authToken *AuthToken) refreshToken() {

	now := time.Now().UTC()
	authToken.RegisteredClaims = jwt.RegisteredClaims{
		ExpiresAt: jwt.NewNumericDate(now.AddDate(0, 6, 0)), //6 months
		IssuedAt:  jwt.NewNumericDate(now),
		Issuer:    "trraformapi",
	}

}
