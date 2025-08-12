package config

import (
	"log"
	"os"

	"github.com/joho/godotenv"
)

const (
	CF_ZONE_ID       = "64097c6d2cf0e0810ca05cdf8d4d1273"
	CF_ACCOUNT_ID    = "1534f5e1cce37d41a018df4c9716751e"
	CF_PLOT_BUCKET   = "plots-dev"
	GOOGLE_CLIENT_ID = "505214281747-g26m4g2lv692ff819neq6pbus4q6f36f.apps.googleusercontent.com"
	ORIGIN           = "http://localhost:5173"
	MONGO_DB         = "TrraformDev"

	MAX_COLOR_IDX   = 30649
	DEP0_PLOT_COUNT = 34998
	SUBPLOT_COUNT   = 24
	MAX_DEPTH       = 2
	CHUNK_SIZE      = 6
	STD_BUILD_SIZE  = 48
	LRG_BUILD_SIZE  = 72
	MIN_BUILD_SIZE  = 6

	USER_PLOT_LIMIT       = 100
	PRICE_ID_SUBSCRIPTION = ""
)

var PRICE_ID_DEPTH = []string{
	"price_depth_1",
	"price_depth_2",
	"price_depth_3",
}

type EnvVars struct {
	CF_TURNSTILE_SECRET_KEY string
	CF_R2_ACCESS_KEY        string
	CF_R2_SECRET_KEY        string
	CF_API_TOKEN            string
	AWS_ACCESS_KEY_ID       string
	AWS_SECRET_ACCESS_KEY   string
	MONGO_PASSWORD          string
	REDIS_PASSWORD          string
	JWT_SECRET              string
	STRIPE_SECRET_KEY       string
	STRIPE_WEBHOOK_SECRET   string
}

var ENV *EnvVars

func init() {

	prod := os.Getenv("ENV") == "prod"

	if !prod {
		err := godotenv.Load()
		if err != nil {
			log.Fatal("Error loading .env file")
			return
		}
	}

	ENV = &EnvVars{
		CF_TURNSTILE_SECRET_KEY: os.Getenv("CF_TURNSTILE_SECRET_KEY"),
		CF_R2_ACCESS_KEY:        os.Getenv("CF_R2_ACCESS_KEY"),
		CF_R2_SECRET_KEY:        os.Getenv("CF_R2_SECRET_KEY"),
		CF_API_TOKEN:            os.Getenv("CF_API_TOKEN"),
		AWS_ACCESS_KEY_ID:       os.Getenv("AWS_ACCESS_KEY_ID"),
		AWS_SECRET_ACCESS_KEY:   os.Getenv("AWS_SECRET_ACCESS_KEY"),
		MONGO_PASSWORD:          os.Getenv("MONGO_PASSWORD"),
		REDIS_PASSWORD:          os.Getenv("REDIS_PASSWORD"),
		JWT_SECRET:              os.Getenv("JWT_SECRET"),
		STRIPE_SECRET_KEY:       os.Getenv("STRIPE_SECRET_KEY"),
		STRIPE_WEBHOOK_SECRET:   os.Getenv("STRIPE_WEBHOOK_SECRET"),
	}

}
