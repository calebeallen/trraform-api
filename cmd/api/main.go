package main

import (
	"context"
	"net/http"
	"os"
	"regexp"
	"time"
	"trraformapi/internal/api"
	"trraformapi/internal/api/auth"
	"trraformapi/internal/api/leaderboard"
	"trraformapi/internal/api/plot"
	"trraformapi/internal/api/user"
	"trraformapi/pkg/config"
	plotutils "trraformapi/pkg/plot_utils"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/ses"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
	"github.com/go-playground/validator/v10"
	"github.com/redis/go-redis/v9"
	"github.com/stripe/stripe-go/v82"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
	"go.mongodb.org/mongo-driver/v2/mongo/readpref"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

func main() {

	ctx := context.Background()
	h := &api.Handler{}

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

	h.Validate.RegisterValidation("plotid", plotutils.PlotIdValidator)
	h.Validate.RegisterValidation("maxgraphemes", plotutils.MaxGraphemesValidator)
	h.Validate.RegisterValidation("builddata", plotutils.BuildDataValidator)

	h.HttpCli = &http.Client{
		Timeout: 30 * time.Second,
	}

	// init mongo
	mongoServerAPI := options.ServerAPI(options.ServerAPIVersion1)
	mongoOpts := options.Client().ApplyURI("mongodb+srv://caleballen:" + config.ENV.MONGO_PASSWORD + "@trraform.cenuh0o.mongodb.net/?retryWrites=true&w=majority&appName=Trraform").SetServerAPIOptions(mongoServerAPI)
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
	h.MongoDB = mongoCli.Database(config.MONGO_DB)

	// init redis
	h.RedisCli = redis.NewClient(&redis.Options{
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
	h.AWSSESCli = ses.NewFromConfig(sesCfg)

	// init s3
	cred := credentials.NewStaticCredentialsProvider(
		config.ENV.CF_R2_ACCESS_KEY,
		config.ENV.CF_R2_SECRET_KEY,
		"",
	)
	h.R2Cli = s3.New(s3.Options{
		Credentials:  cred,
		BaseEndpoint: aws.String(os.Getenv("CF_R2_API_ENDPOINT")),
		UsePathStyle: true,
		Region:       "auto",
	})

	// init stripe
	h.StripeCli = stripe.NewClient(config.ENV.STRIPE_SECRET_KEY)

	router := chi.NewRouter()

	// Middleware
	router.Use(cors.Handler(cors.Options{
		AllowedOrigins: []string{config.ORIGIN},
		AllowedMethods: []string{"GET", "POST", "OPTIONS"},
		AllowedHeaders: []string{"Content-Type", "Authorization"},
	}))
	router.Use(middleware.Recoverer)
	router.Use(middleware.RequestSize(1 << 20))

	authH := &auth.Handler{Handler: h}
	userH := &user.Handler{Handler: h}
	plotH := &plot.Handler{Handler: h}
	leaderboardH := &leaderboard.Handler{Handler: h}
	// paymentsH := &payments.Handler{Handler: deps}

	// auth endpoints (add captcha)
	router.Post("/auth/create-account", authH.CreateAccount)
	router.Post("/auth/password-login", authH.PasswordLogin)
	router.Post("/auth/google-login", authH.GoogleLogin)
	router.Post("/auth/send-verification-code", authH.SendVerificationCode)
	router.Post("/auth/verify-email", authH.VerifyEmail)
	router.Post("/auth/reset-password", authH.ResetPassword)

	// user endpoints
	router.Get("/user", userH.GetUserData)
	router.Post("/user/change-username", h.AuthMiddleware(userH.ChangeUsername))

	// plot endpoints
	router.Post("/plot/claim-with-credit", h.AuthMiddleware(plotH.ClaimWithCredit))
	router.Post("/plot/update", h.AuthMiddleware(plotH.UpdatePlot))

	// leaderboard endpoints
	router.Get("/leaderboard", leaderboardH.GetLeaderboard)
	router.Post("/leaderboard/vote", leaderboardH.Vote)

	// payment endpoints
	// router.Get("/payment/intent/details", payment.GetPaymentIntentDetails)
	// router.Post("/payment/intent", payment.CreatePaymentIntent)
	// router.Post("/payment/subscription/create", payment.CreateSubscription)
	// router.Post("/payment/subscription/update", payment.UpdateSubscription)
	// router.Post("/payment/stripe-webhook", payment.StripeWebhook)

	logger.Info("Server running on port 8080")
	http.ListenAndServe(":8080", router)

}
