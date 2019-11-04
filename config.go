package main

import (
	"log"
	"os"

	"github.com/joho/godotenv"
)

type Config struct {
	Port     string
	RapidApi struct {
		ProxySecret string
	}
	Space struct {
		Bucket   string
		Endpoint string
		Region   string
	}
	Sentry struct {
		DSN string
	}
}

var config Config

func init() {
	config = LoadConfig()
}

func LoadConfig() (data Config) {
	err := godotenv.Load()
	if err != nil {
		log.Println("Error loading .env file")
	}

	data.Port = os.Getenv("PORT")
	data.RapidApi.ProxySecret = os.Getenv("RAPIDAPI_PROXY_SECRET")
	data.Sentry.DSN = os.Getenv("SENTRY_DSN")

	data.Space.Endpoint = os.Getenv("AWS_ENDPOINT")
	data.Space.Region = os.Getenv("AWS_REGION")
	data.Space.Bucket = os.Getenv("AWS_BUCKET")
	// fmt.Println(os.Getenv("AWS_ACCESS_KEY_ID"))
	// fmt.Println(os.Getenv("AWS_SECRET_ACCESS_KEY"))
	// os.Setenv("AWS_ACCESS_KEY_ID", "xxx")
	// os.Setenv("AWS_SECRET_ACCESS_KEY", "xxx")

	return data
}
