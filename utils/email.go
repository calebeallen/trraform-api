package utils

import (
	"context"
	"fmt"
	"net/url"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ses"
	"github.com/aws/aws-sdk-go-v2/service/ses/types"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
)

type EmailStatus struct {
	Sent     bool    `json:"sent"`
	Cooldown float64 `json:"cooldown"`
}

func SendVerificationEmail(ctx context.Context, to string) (*EmailStatus, error) {

	cooldownKey := "email:verify:cooldown:" + to // cooldown only used in this function. For rate limiting
	tokenKey := "email:verify:token:" + to

	// check if cooldown has passed
	ttl, err := RedisCli.TTL(ctx, cooldownKey).Result()
	if err != redis.Nil {
		return &EmailStatus{
			Sent:     false,
			Cooldown: ttl.Seconds(),
		}, nil
	} else if err != nil {
		return nil, fmt.Errorf("in SendVerificationEmail:\n%w", err)
	}

	// set cooldown
	cooldown := time.Minute
	_, err = RedisCli.Set(ctx, cooldownKey, "1", cooldown).Result()
	if err != nil {
		return nil, fmt.Errorf("in SendVerificationEmail:\n%w", err)
	}

	// create and store new token (valid for 2 hours)
	token := uuid.New().String()
	_, err = RedisCli.Set(ctx, tokenKey, token, time.Hour*2).Result()
	if err != nil {
		return nil, fmt.Errorf("in SendVerificationEmail:\n%w", err)
	}

	// send email with token
	encodedToken := url.QueryEscape(token)
	encodedEmail := url.QueryEscape(to)
	verifyUrl := fmt.Sprintf("%s/auth/verify-email?token=%s&email=%s", Origin, encodedToken, encodedEmail)
	htmlBody := fmt.Sprintf(`
		<!DOCTYPE html>
		<html>
			<body style="font-family: Arial, sans-serif; padding: 20px;">
				<h2>Welcome to Trraform!</h2>
				<p>Thanks for signing up. Click the link below to verify your email:</p>
				<p><a href="%s">Verify Email</a></p>
			</body>
		</html>
	`, verifyUrl)

	emailInput := &ses.SendEmailInput{
		Source: aws.String("no-reply@trraform.com"), // your verified sender
		Destination: &types.Destination{
			ToAddresses: []string{to},
		},
		Message: &types.Message{
			Subject: &types.Content{
				Data: aws.String("Verify your email"),
			},
			Body: &types.Body{
				Html: &types.Content{Data: aws.String(htmlBody)},
			},
		},
	}

	// Send the email
	_, err = AwsSESCli.SendEmail(ctx, emailInput)
	if err != nil {
		log := struct {
			Email string `json:"email"`
		}{Email: to}
		LogErrorDiscord("SendVerificationEmail", err, log)
		return nil, fmt.Errorf("in SendVerificationEmail:\n%w", err)
	}

	status := EmailStatus{
		Sent:     true,
		Cooldown: cooldown.Minutes(),
	}

	return &status, nil

}
