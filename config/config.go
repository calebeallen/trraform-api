package config

import (
	"log"
	"os"

	"github.com/joho/godotenv"
)

type env struct {
	CF_TURNSTILE_SECRET_KEY string
	CF_R2_ACCESS_KEY        string
	CF_R2_SECRET_KEY        string
	CF_ZONE_ID              string
	CF_API_TOKEN            string
	CF_ACCOUNT_ID           string
	GOOGLE_CLIENT_ID        string
	AWS_ACCESS_KEY_ID       string
	AWS_SECRET_ACCESS_KEY   string
	MONGO_PASSWORD          string
	REDIS_PASSWORD          string
	JWT_SECRET              string
	STRIPE_SECRET_KEY       string
	STRIPE_WEBHOOK_SECRET   string
}

type general struct {
	ORIGIN          string
	MAX_COLOR_IDX   int
	DEP0_PLOT_COUNT int
	SUBPLOT_COUNT   int
	MAX_DEPTH       int
	CHUNK_SIZE      int
	STD_BUILD_SIZE  int
	LRG_BUILD_SIZE  int
	MIN_BUILD_SIZE  int
}

var ENV *env
var GEN *general

func init() {

	prod := os.Getenv("ENV") == "prod"

	if !prod {
		err := godotenv.Load()
		if err != nil {
			log.Fatal("Error loading .env file")
			return
		}
	}

	ENV = &env{
		CF_TURNSTILE_SECRET_KEY: os.Getenv("CF_TURNSTILE_SECRET_KEY"),
		CF_R2_ACCESS_KEY:        os.Getenv("CF_R2_ACCESS_KEY"),
		CF_R2_SECRET_KEY:        os.Getenv("CF_R2_SECRET_KEY"),
		CF_ZONE_ID:              os.Getenv("CF_ZONE_ID"),
		CF_API_TOKEN:            os.Getenv("CF_API_TOKEN"),
		CF_ACCOUNT_ID:           os.Getenv("CF_ACCOUNT_ID"),
		GOOGLE_CLIENT_ID:        os.Getenv("GOOGLE_CLIENT_ID"),
		AWS_ACCESS_KEY_ID:       os.Getenv("AWS_ACCESS_KEY_ID"),
		AWS_SECRET_ACCESS_KEY:   os.Getenv("AWS_SECRET_ACCESS_KEY"),
		MONGO_PASSWORD:          os.Getenv("MONGO_PASSWORD"),
		REDIS_PASSWORD:          os.Getenv("REDIS_PASSWORD"),
		JWT_SECRET:              os.Getenv("JWT_SECRET"),
		STRIPE_SECRET_KEY:       os.Getenv("STRIPE_SECRET_KEY"),
		STRIPE_WEBHOOK_SECRET:   os.Getenv("STRIPE_WEBHOOK_SECRET"),
	}

	var origin string
	if prod {
		origin = "https://trraform.com"
	} else {
		origin = "http://localhost:5137"
	}

	GEN = &general{
		ORIGIN:          origin,
		MAX_COLOR_IDX:   30649,
		DEP0_PLOT_COUNT: 34998,
		SUBPLOT_COUNT:   24,
		MAX_DEPTH:       2,
		CHUNK_SIZE:      6,
		STD_BUILD_SIZE:  48,
		LRG_BUILD_SIZE:  72,
		MIN_BUILD_SIZE:  6,
	}

}
