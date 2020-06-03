package main

import (
	"log"

	"github.com/getsentry/sentry-go"
)

func init() {
	sentry.Init(sentry.ClientOptions{
		Dsn: config.Sentry.DSN,
	})

}

func SentryCaptureException(err error) {
	if err != nil {
		log.Println(err)
		sentry.CaptureException(err)
	}
}
