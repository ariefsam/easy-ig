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
	Proxy string
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
	if data.Port == "" {
		log.Println("Empty PORT, using default: 8211")
		data.Port = "8211"
	}
	data.RapidApi.ProxySecret = os.Getenv("RAPIDAPI_PROXY_SECRET")

	data.Proxy = os.Getenv("PROXY")
	if data.Proxy == "" {
		data.Proxy = "http://lum-customer-ronaldsihom-zone-zone3-country-us:zfbvdqv0nsj4@zproxy.lum-superproxy.io:22225"
	}

	return data
}
