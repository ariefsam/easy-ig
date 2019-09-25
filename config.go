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

	return data
}
