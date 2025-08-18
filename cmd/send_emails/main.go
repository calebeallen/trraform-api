package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"html/template"
	"log"
	"math"
	"path/filepath"
	"runtime"
	"time"
	"trraformapi/pkg/config"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/ses"
	"github.com/aws/aws-sdk-go-v2/service/ses/types"
	"github.com/redis/go-redis/v9"
)

const (
	DEFAULT_RATE_LIMIT       = 13
	DAY_LIMIT_WARN_THRESHOLD = 2000
	SENDER                   = "no-reply@trraform.com"
	SUBJECT                  = "Verification Code | Trraform"
)

var emailTemplate *template.Template

func init() {
	var err error
	_, file, _, _ := runtime.Caller(0) // path to current .go file
	dir := filepath.Dir(file)
	path := filepath.Join(dir, "template.html")
	emailTemplate, err = template.ParseFiles(path)
	if err != nil {
		panic(err)
	}
}

type FailedEmail struct {
	Email string `json:"email"`
	Error string `json:"err"`
}

func sendEmail(ctx context.Context, redisCli *redis.Client, sesCli *ses.Client, email string) *FailedEmail {

	// fetch verification code for email
	code, err := redisCli.Get(ctx, "vercode:"+email).Result()
	if err != nil {
		return &FailedEmail{
			Email: email,
			Error: err.Error(),
		}
	}

	// render email body
	data := &struct {
		Code string
	}{Code: code}
	var buf bytes.Buffer
	if err := emailTemplate.Execute(&buf, data); err != nil {
		return &FailedEmail{
			Email: email,
			Error: err.Error(),
		}
	}
	emailBody := aws.String(buf.String())

	// prep email
	emailInput := &ses.SendEmailInput{
		Source: aws.String(SENDER),
		Destination: &types.Destination{
			ToAddresses: []string{email},
		},
		Message: &types.Message{
			Subject: &types.Content{
				Data: aws.String(SUBJECT),
			},
			Body: &types.Body{
				Html: &types.Content{
					Data: emailBody,
				},
			},
		},
	}

	_, err = sesCli.SendEmail(ctx, emailInput)
	if err != nil {
		return &FailedEmail{
			Email: email,
			Error: err.Error(),
		}
	}

	return nil

}

func main() {

	ctx := context.Background()

	// init redis
	redisCli := redis.NewClient(&redis.Options{
		Addr:     "redis-16216.c15.us-east-1-4.ec2.redns.redis-cloud.com:16216",
		Username: "default",
		Password: config.ENV.REDIS_PASSWORD,
		DB:       0,
	})

	// init aws ses
	sesCfg, err := awsconfig.LoadDefaultConfig(ctx, awsconfig.WithRegion("us-east-1"))
	if err != nil {
		panic(err)
	}
	sesCli := ses.NewFromConfig(sesCfg)

	lastQuotaCheck := time.Time{}
	rateLimit := DEFAULT_RATE_LIMIT

	fmt.Println("Starting email dispatcher")

	// main loop
	for {
		// check for daily quota usage
		if time.Since(lastQuotaCheck) > time.Minute*10 {
			quota, err := sesCli.GetSendQuota(ctx, &ses.GetSendQuotaInput{})
			if err != nil {
				log.Printf("GetSendQuota error: %v", err)
				time.Sleep(time.Second * 10)
			} else {
				rateLimit = max(DEFAULT_RATE_LIMIT, int(math.Floor(quota.MaxSendRate)))
				dailyRemaining := int(quota.Max24HourSend - quota.SentLast24Hours)
				if dailyRemaining < DAY_LIMIT_WARN_THRESHOLD {
					log.Printf("Only %d emails remaining for 24 hours! Backing off...", dailyRemaining)
					time.Sleep(time.Minute * 5)
				}
			}
			lastQuotaCheck = time.Now()
		}

		// check for queued emails
		recipients, err := redisCli.RPopCount(ctx, "vemailq", rateLimit).Result()
		if err == redis.Nil {
			time.Sleep(time.Second * 10) // no email right now, sleep
			continue
		} else if err != nil {
			log.Printf("Redis RPopCount error: %v", err)
			time.Sleep(time.Minute) // brief backoff
			continue
		}

		start := time.Now()

		// send emails
		for _, email := range recipients {
			failed := sendEmail(ctx, redisCli, sesCli, email)
			if failed != nil {
				data, _ := json.Marshal(failed)
				log.Printf("Email failed %s", data)

				// keep track of failed email
				if err := redisCli.RPush(ctx, "failed_emails", data).Err(); err != nil {
					log.Printf("Couldn't push to failed email list %s, %v", email, err)
				}
			}
		}

		// avoid ses rate limit
		elapsed := time.Since(start)
		if remaining := time.Second - elapsed; remaining > 0 {
			time.Sleep(remaining)
		}

	}

}
