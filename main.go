package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"regexp"
	"trraformapi/api"
	"trraformapi/api/auth"
	cronjobs "trraformapi/api/cron_jobs"
	"trraformapi/api/leaderboard"
	"trraformapi/api/payment"
	"trraformapi/api/plot"
	"trraformapi/api/user"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/ses"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
	"github.com/go-playground/validator/v10"
	"github.com/joho/godotenv"
	"github.com/redis/go-redis/v9"
	"github.com/stripe/stripe-go/v82"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
	"go.mongodb.org/mongo-driver/v2/mongo/readpref"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
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
	h := api.Handler{}

	// init logger
	logger, err := zap.NewDevelopment(
		zap.AddCaller(),
		zap.AddStacktrace(zapcore.ErrorLevel),
	)

	if err != nil {
		panic(err)
	}
	logger.Info("Server starting...")
	defer logger.Sync()
	h.Logger = logger

	// init validator
	h.Validate = validator.New()
	h.Validate.RegisterValidation("username", func(fl validator.FieldLevel) bool {
		username := fl.Field().String()
		re := regexp.MustCompile(`^[a-zA-Z0-9._]{3,32}$`)
		return re.MatchString(username)
	})

	h.Validate.RegisterValidation("password", func(fl validator.FieldLevel) bool {
		password := fl.Field().String()
		re := regexp.MustCompile(`^[A-Za-z0-9~` + "`" + `!@#$%^&*()_\-+={[}\]|\\:;"'<,>.?/]{8,128}$`)
		return re.MatchString(password)
	})

	// init mongo
	mongoServerAPI := options.ServerAPI(options.ServerAPIVersion1)
	mongoOpts := options.Client().ApplyURI("mongodb+srv://caleballen:" + os.Getenv("MONGO_PASSWORD") + "@trraform.cenuh0o.mongodb.net/?retryWrites=true&w=majority&appName=Trraform").SetServerAPIOptions(mongoServerAPI)
	mongoCli, err := mongo.Connect(mongoOpts)
	if err != nil {
		panic(err)
	}
	defer func() {
		if err = mongoCli.Disconnect(ctx); err != nil {
			panic(err)
		}
	}()
	if err := mongoCli.Ping(ctx, readpref.Primary()); err != nil {
		panic(err)
	}
	h.MongoDB = mongoCli.Database("Trraform")

	// init redis
	h.RedisClient = redis.NewClient(&redis.Options{
		Addr:     "redis-16216.c15.us-east-1-4.ec2.redns.redis-cloud.com:16216",
		Username: "default",
		Password: os.Getenv("REDIS_PASSWORD"),
		DB:       0,
	})

	// init aws ses
	sesCfg, err := config.LoadDefaultConfig(ctx, config.WithRegion("us-east-1"))
	if err != nil {
		panic(err)
	}
	h.AWSSESClient = ses.NewFromConfig(sesCfg)

	// init s3
	cred := credentials.NewStaticCredentialsProvider(
		os.Getenv("CF_R2_ACCESS_KEY"),
		os.Getenv("CF_R2_SECRET_KEY"),
		"",
	)
	h.R2Client = s3.New(s3.Options{
		Credentials:  cred,
		BaseEndpoint: aws.String(os.Getenv("CF_R2_API_ENDPOINT")),
		UsePathStyle: true,
		Region:       "auto",
	})

	// init stripe
	stripe.Key = os.Getenv("STRIPE_SECRET_KEY")

	router := chi.NewRouter()

	// Middleware
	router.Use(cors.Handler(cors.Options{
		AllowedOrigins: []string{"http://localhost:5173", "https://trraform.com"},
		AllowedMethods: []string{"GET", "POST", "OPTIONS"},
		AllowedHeaders: []string{"Content-Type", "Authorization"},
	}))
	router.Use(middleware.Recoverer)
	router.Use(middleware.RequestSize(1 << 20))

	authH := &auth.Handler{Handler: &h}
	// paymentsH := &payments.Handler{Handler: deps}

	// auth endpoints (add captcha)
	router.Post("/auth/create-account", h.CreateAccount)
	router.Post("/auth/password-login", auth.PasswordLogin)
	router.Post("/auth/google-login", auth.GoogleLogin)
	router.Post("/auth/verify-email", auth.VerifyEmail)
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

	logger.Info("Server running on port 8080")
	http.ListenAndServe(":8080", router)

}
