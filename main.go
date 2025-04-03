package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"trraformapi/utils"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/ses"
	"github.com/aws/aws-sdk-go-v2/service/ses/types"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
	"github.com/joho/godotenv"
	"github.com/redis/go-redis/v9"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
	"go.mongodb.org/mongo-driver/v2/mongo/readpref"
)

func init() {

	if os.Getenv("ENV") != "prod" {
		err := godotenv.Load()
		if err != nil {
			log.Fatal("Error loading .env file")
			return
		}
	}

}

func main() {

	ctx := context.Background()

	// init mongo connection
	mongoServerAPI := options.ServerAPI(options.ServerAPIVersion1)
	mongoOpts := options.Client().ApplyURI("mongodb+srv://caleballen:" + os.Getenv("MONGO_PASSWORD") + "@trraform.cenuh0o.mongodb.net/?retryWrites=true&w=majority&appName=Trraform").SetServerAPIOptions(mongoServerAPI)
	mongoCli, err := mongo.Connect(mongoOpts)
	if err != nil {
		panic(err)
	}
	utils.MongoCli = mongoCli
	utils.MongoDB = mongoCli.Database("Trraform")

	defer func() {
		if err = mongoCli.Disconnect(ctx); err != nil {
			panic(err)
		}
	}()

	if err := mongoCli.Ping(ctx, readpref.Primary()); err != nil {
		panic(err)
	}

	// init redis client
	utils.RedisCli = redis.NewClient(&redis.Options{
		Addr:     "redis-16216.c15.us-east-1-4.ec2.redns.redis-cloud.com:16216",
		Username: "default",
		Password: os.Getenv("REDIS_PASSWORD"),
		DB:       0,
	})

	// init aws ses
	cfg, err := config.LoadDefaultConfig(ctx, config.WithRegion("us-east-1"))
	if err != nil {
		panic(err)
	}
	utils.AwsSESCli = ses.NewFromConfig(cfg)

	router := chi.NewRouter()

	// Middleware
	router.Use(cors.Handler(cors.Options{
		AllowedOrigins: []string{"*"},
		AllowedMethods: []string{"GET", "POST", "OPTIONS"},
		AllowedHeaders: []string{"Content-Type", "Authorization"},
	}))
	router.Use(middleware.Recoverer)

	fmt.Println("Server starting")

	testEmail()
	http.ListenAndServe(":8080", router)

}

func testEmail() {

	// Build the email
	input := &ses.SendEmailInput{
		Source: aws.String("no-reply@trraform.com"), // your verified sender
		Destination: &types.Destination{
			ToAddresses: []string{"calebeallen318@gmail.com"}, // recipient
		},
		Message: &types.Message{
			Subject: &types.Content{
				Data: aws.String("Verify your email"),
			},
			Body: &types.Body{
				Html: &types.Content{
					Data: aws.String(`
						<!DOCTYPE html>
<html>
  <body style="margin: 0; padding: 0; background-color: #3f3f46; font-family: Arial, sans-serif;">
    <table align="center" width="100%" cellpadding="0" cellspacing="0" style="padding: 40px 0;">
      <tr>
        <td align="center">
          <table width="100%" cellpadding="0" cellspacing="0" style="max-width: 600px; background-color: #ffffff; border-radius: 6px; padding: 40px; box-shadow: 0 2px 5px rgba(0,0,0,0.05);">
            <tr>
              <td>
                <h2 style="margin-top: 0;">Welcome to Trraform!</h2>
                <p>Check out our docs to help you get things rolling: 
                  <a href="https://trraform.com/docs" style="color: #007bff;">https://trraform.com/docs</a>
                </p>
                <p>If you have questions, get stuck, or want to talk about what you're building, visit our discussion forum: 
                  <a href="https://community.trraform.com" style="color: #007bff;">https://community.trraform.com</a>
                </p>
                <p>Set a password for future logins:</p>

                <!-- CTA Button -->
                <table cellpadding="0" cellspacing="0" width="100%" style="margin-top: 20px;">
                  <tr>
                    <td align="center">
                      <a href="https://trraform.com/set-password?token=abc123"
                         style="background-color: #9d4edd; color: #ffffff; text-decoration: none; padding: 12px 24px; border-radius: 6px; font-weight: bold; display: inline-block;">
                        Set Password
                      </a>
                    </td>
                  </tr>
                </table>

                <p style="margin-top: 40px; color: #999999; font-size: 12px; text-align: center;">
                  — The Trraform Team
                </p>
              </td>
            </tr>
          </table>
        </td>
      </tr>
    </table>
  </body>
</html>

					`),
				},
			},
		},
		// Optional: attach config set for tracking bounces, etc.
		// ConfigurationSetName: aws.String("your-config-set"),
	}

	// Send the email
	res, err := utils.AwsSESCli.SendEmail(context.Background(), input)
	fmt.Println(res)
	if err != nil {
		panic(err)
	}

}
