package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"trraformapi/api/auth"
	cronjobs "trraformapi/api/cron_jobs"
	"trraformapi/api/leaderboard"
	"trraformapi/api/payment"
	"trraformapi/api/plot"
	"trraformapi/api/user"
	"trraformapi/utils"

	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/ses"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
	"github.com/joho/godotenv"
	"github.com/redis/go-redis/v9"
	"github.com/stripe/stripe-go/v82"
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

	// init stripe
	stripe.Key = os.Getenv("STRIPE_SECRET_KEY")

	// Middleware
	router.Use(cors.Handler(cors.Options{
		AllowedOrigins: []string{"http://localhost:5173", "https://trraform.com"},
		AllowedMethods: []string{"GET", "POST", "OPTIONS"},
		AllowedHeaders: []string{"Content-Type", "Authorization"},
	}))
	router.Use(middleware.Recoverer)

	// auth endpoints (add captcha)
	router.Post("/auth/create-account", auth.CreateAccount)
	router.Post("/auth/password-login", auth.PasswordLogIn)
	router.Post("/auth/google-login", auth.GoogleLogIn)
	router.Post("/auth/send-verification-email", auth.SendVerificationEmail)
	router.Post("/auth/verify-email", auth.VerifyEmail)
	router.Post("/auth/send-password-reset-email", auth.SendPasswordResetEmail)
	router.Post("/auth/reset-password", auth.ResetPassword)

	// user endpoints
	router.Get("/user", user.GetUserData)
	router.Post("/user/change-username", user.ChangeUsername)

	// plot endpoints
	router.Post("/plot/claim-with-credit", plot.ClaimWithCredit)
	router.Post("/plot/update", plot.UpdatePlot)
	router.Get("/plot/open", plot.GetOpenPlot)

	// leaderboard endpoints
	router.Get("/leaderboard", leaderboard.GetLeaderboard)
	router.Post("/leaderboard/vote", leaderboard.Vote)

	// cron endpoints
	router.Post("/cron-jobs/refresh-leaderboard", cronjobs.RefreshLeaderboard)

	// payment endpoints
	router.Get("/payment/intent/details", payment.GetPaymentIntentDetails)
	router.Post("/payment/intent", payment.CreatePaymentIntent)
	router.Post("/payment/subscription/create", payment.CreateSubscription)
	router.Post("/payment/subscription/update", payment.UpdateSubscription)
	router.Post("/payment/stripe-webhook", payment.StripeWebhook)

	fmt.Println("Server starting")
	http.ListenAndServe(":8080", router)

}
