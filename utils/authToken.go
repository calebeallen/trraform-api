package utils

import (
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

type AuthToken struct {
	Uid string `json:"uid"`
	jwt.RegisteredClaims
}

func ValidateAndRefreshAuthToken(w http.ResponseWriter, r *http.Request) (*AuthToken, error) {

	cookie, err := r.Cookie("auth_token")
	if err != nil {
		return nil, fmt.Errorf("in ValidateAndRefreshAuthToken: %w", err)
	}

	tokenStr := cookie.Value

	// validate token
	var authToken AuthToken
	_, err = jwt.ParseWithClaims(tokenStr, &authToken, func(token *jwt.Token) (any, error) {
		return os.Getenv("JWT_SECRET"), nil
	})
	if err != nil {
		return nil, fmt.Errorf("in ParseAuthToken:\n%w", err)
	}

	//return if expired
	timeLeft := authToken.ExpiresAt.Sub(time.Now().UTC())
	if timeLeft <= 0 {
		return nil, fmt.Errorf("in ParseAuthToken:\n%w", err)
	}

	// refresh 1 month before expire
	if timeLeft <= time.Hour*24*7*4 {
		authToken.Refresh()
		authToken.SetCookie(w)
	}

	return &authToken, nil

}

func CreateNewAuthToken(uid primitive.ObjectID) *AuthToken {

	token := AuthToken{Uid: uid.Hex()}
	token.Refresh()
	return &token

}

func (authToken *AuthToken) Refresh() {

	now := time.Now().UTC()
	authToken.RegisteredClaims = jwt.RegisteredClaims{
		ExpiresAt: jwt.NewNumericDate(now.Add(time.Hour * 24 * 7 * 4 * 6)), //6 months
		IssuedAt:  jwt.NewNumericDate(now),
		Issuer:    "trraformapi",
	}

}

func (authToken *AuthToken) SetCookie(w http.ResponseWriter) error {

	tokenStr, err := authToken.sign()
	if err != nil {
		return fmt.Errorf("in writeToHeader:\n%w", err)
	}

	http.SetCookie(w, &http.Cookie{
		Name:     "auth_token",
		Value:    tokenStr,
		Expires:  time.Now().UTC().AddDate(1, 0, 0),
		Path:     "/",
		HttpOnly: true,
		Secure:   true,
		SameSite: http.SameSiteLaxMode,
	})

	return nil

}

func (authToken *AuthToken) UidObjectID() (*primitive.ObjectID, error) {

	objId, err := primitive.ObjectIDFromHex(authToken.Uid)
	if err != nil {
		return nil, fmt.Errorf("in UidObjectID:\n%w", err)
	}
	return &objId, nil

}

func (authToken *AuthToken) sign() (string, error) {

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, authToken)
	return token.SignedString(os.Getenv("JWT_SECRET"))

}
