package main

import (
	"github.com/getsentry/sentry-go"
)

func init() {
	sentry.Init(sentry.ClientOptions{
		Dsn: config.Sentry.DSN,
	})

}

func SentryCaptureException(err error) {
	if err != nil {
		sentry.CaptureException(err)
	}
}
