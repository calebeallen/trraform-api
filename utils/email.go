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
)

type EmailStatus struct {
	Sent     bool    `json:"sent"`
	Cooldown float64 `json:"cooldown"`
}

func SendVerificationEmail(ctx context.Context, to string) (*EmailStatus, error) {

	// send email with token
	token := uuid.New().String()
	encodedToken := url.QueryEscape(token)
	encodedEmail := url.QueryEscape(to)
	verifyUrl := fmt.Sprintf("%s/auth/verify-email?token=%s&email=%s", Origin, encodedToken, encodedEmail)
	html := fmt.Sprintf(`
		<!DOCTYPE html>
		<html lang="en">
		<head>
			<meta charset="UTF-8" />
		</head>
		<body style="margin: 0; padding: 0; background-color: #18181b; font-family: sans-serif;">
			<table width="100%%" cellspacing="0" cellpadding="0">
				<tr>
					<td align="center" style="padding: 40px 20px;">
						<table width="600" style="background-color: #27272a; border-radius: 8px;">
							<tr>
								<td valign="top" style="padding: 32px; width: 60%%; padding-right: 20px;">
									<div style="color: #FFF; font-size: 24px; font-weight: bold;">Welcome!</div>
									<div style="padding-top: 12px; font-size: 16px; color: #FFF;">
										Thanks for joining Trraform! Verify your email to start building.
									</div>
									<div style="padding-top: 24px;">
										<a href="%s" style="
											background-color: #007BFF;
											color: #FFFFFF;
											text-decoration: none;
											padding: 12px 24px;
											border-radius: 6px;
											display: inline-block;
											font-weight: bold;
										">
											Verify Email
										</a>
									</div>
									<div style="font-size: 12px; color: #FFF; padding-top: 24px;">© 2025 Trraform</div>
								</td>
								<td valign="top" style="text-align: center;">
									<img src="%s/email_img.png"
										alt="Image"
										width="200"
										style="background: transparent;">
								</td>
							</tr>
						</table>
					</td>
				</tr>
			</table>
		</body>
		</html>`, verifyUrl, Origin)

	return sendEmail(ctx, "verify", to, token, "Verify your email", html)

}

func SendResetPasswordEmail(ctx context.Context, to string) (*EmailStatus, error) {

	token := uuid.New().String()
	encodedToken := url.QueryEscape(token)
	encodedEmail := url.QueryEscape(to)
	url := fmt.Sprintf("%s/auth/reset-password?token=%s&email=%s", Origin, encodedToken, encodedEmail)
	html := fmt.Sprintf(`
		<!DOCTYPE html>
		<html lang="en">
		<head>
			<meta charset="UTF-8" />
		</head>
		<body style="margin: 0; padding: 0; background-color: #18181b; font-family: sans-serif;">
			<table width="100%%" cellspacing="0" cellpadding="0">
				<tr>
					<td align="center" style="padding: 40px 20px;">
						<table width="600" style="background-color: #27272a; border-radius: 8px;">
							<tr>
								<td valign="top" style="padding: 32px; width: 60%%; padding-right: 20px;">
									<div style="color: #FFF; font-size: 24px; font-weight: bold;">Reset Password</div>
									<div style="padding-top: 12px; font-size: 16px; color: #FFF;">
										Click the button below to reset your password.
									</div>
									<div style="padding-top: 24px;">
										<a href="%s" style="
											background-color: #007BFF;
											color: #FFFFFF;
											text-decoration: none;
											padding: 12px 24px;
											border-radius: 6px;
											display: inline-block;
											font-weight: bold;
										">
											Reset password
										</a>
									</div>
									<div style="font-size: 12px; color: #FFF; padding-top: 24px;">© 2025 Trraform</div>
								</td>
								<td valign="top" style="text-align: center;">
									<img src="%s/email_img.png"
										alt="Image"
										width="200"
										style="background: transparent;">
								</td>
							</tr>
						</table>
					</td>
				</tr>
			</table>
		</body>
		</html>`, url, Origin)

	return sendEmail(ctx, "reset", to, token, "Reset password", html)

}

func sendEmail(ctx context.Context, redisSet string, to string, token string, subject string, html string) (*EmailStatus, error) {

	cooldownKey := fmt.Sprintf("email:%s:cooldown:%s", redisSet, to) // cooldown only used in this function. For rate limiting
	tokenKey := fmt.Sprintf("email:%s:token:%s", redisSet, to)

	// check if cooldown has passed
	ttl, err := RedisCli.TTL(ctx, cooldownKey).Result()
	if err != nil {
		return nil, fmt.Errorf("in sendEmail:\n%w", err)
	}
	if ttl > 0 {
		return &EmailStatus{
			Sent:     false,
			Cooldown: ttl.Seconds(),
		}, nil
	}

	// set cooldown
	cooldown := time.Minute * 2
	_, err = RedisCli.Set(ctx, cooldownKey, "1", cooldown).Result()
	if err != nil {
		return nil, fmt.Errorf("in sendEmail:\n%w", err)
	}

	// set token
	_, err = RedisCli.Set(ctx, tokenKey, token, time.Hour*2).Result()
	if err != nil {
		return nil, fmt.Errorf("in sendEmail:\n%w", err)
	}

	// prep email
	emailInput := &ses.SendEmailInput{
		Source: aws.String("no-reply@trraform.com"), // your verified sender
		Destination: &types.Destination{
			ToAddresses: []string{to},
		},
		Message: &types.Message{
			Subject: &types.Content{
				Data: aws.String(subject),
			},
			Body: &types.Body{
				Html: &types.Content{Data: aws.String(html)},
			},
		},
	}

	// Send the email
	_, err = AwsSESCli.SendEmail(ctx, emailInput)
	if err != nil {
		log := struct {
			Email string `json:"email"`
		}{Email: to}
		LogErrorDiscord("in sendEmail:\n%w", err, log)
		return nil, fmt.Errorf("in sendEmail:\n%w", err)
	}

	status := EmailStatus{
		Sent:     true,
		Cooldown: cooldown.Seconds(),
	}

	return &status, nil

}
