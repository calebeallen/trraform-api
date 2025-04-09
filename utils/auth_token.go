package utils

import (
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"go.mongodb.org/mongo-driver/v2/bson"
)

type AuthToken struct {
	Uid string `json:"uid"`
	jwt.RegisteredClaims
}

func CreateNewAuthToken(uid bson.ObjectID) *AuthToken {

	token := AuthToken{Uid: uid.Hex()}
	token.refresh()
	return &token

}

func ValidateAuthToken(r *http.Request) (*AuthToken, error) {

	header := r.Header.Get("Authorization")
	if header == "" {
		return nil, fmt.Errorf("in ParseAuthToken: missing token")
	}

	parts := strings.Split(header, " ")
	if len(parts) != 2 {
		return nil, fmt.Errorf("in ParseAuthToken: invalid token format")
	}
	token := parts[1]

	// validate token
	var authToken AuthToken
	_, err := jwt.ParseWithClaims(token, &authToken, func(token *jwt.Token) (any, error) {
		return []byte(os.Getenv("JWT_SECRET")), nil
	})
	if err != nil {
		return nil, fmt.Errorf("in ParseAuthToken:\n%w", err)
	}

	// error if expired
	if time.Now().UTC().After(authToken.ExpiresAt.Time) {
		return nil, fmt.Errorf("in ParseAuthToken: token expired")
	}

	return &authToken, nil

}

func (authToken *AuthToken) Sign() (string, error) {

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, authToken)
	key := []byte(os.Getenv("JWT_SECRET"))
	signed, err := token.SignedString(key)
	if err != nil {
		return "", fmt.Errorf("in writeToHeader:\n%w", err)
	}

	return "Bearer " + signed, nil

}

func (authToken *AuthToken) GetUidObjectId() (*bson.ObjectID, error) {

	objId, err := bson.ObjectIDFromHex(authToken.Uid)
	if err != nil {
		return nil, fmt.Errorf("in UidObjectID:\n%w", err)
	}
	return &objId, nil

}

func (authToken *AuthToken) Refresh() {

	//if expiring in < 1 month refresh token
	timeTillExpire := authToken.ExpiresAt.Sub(time.Now().UTC())
	if timeTillExpire <= time.Hour*24*7*4 {
		authToken.refresh()
	}

}

func (authToken *AuthToken) refresh() {

	now := time.Now().UTC()
	authToken.RegisteredClaims = jwt.RegisteredClaims{
		ExpiresAt: jwt.NewNumericDate(now.AddDate(0, 6, 0)), //6 months
		IssuedAt:  jwt.NewNumericDate(now),
		Issuer:    "trraformapi",
	}

}
