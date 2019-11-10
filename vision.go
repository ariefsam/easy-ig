package main

import (
	"bytes"
	"context"
	"fmt"

	"github.com/getsentry/sentry-go"

	vision "cloud.google.com/go/vision/apiv1"
)

func Vision(image_url string) (keywords []string, err error) {
	img_byte, err := GetImageFromURL(image_url)
	if err != nil {
		SentryCaptureException(err)
		return
	}
	ctx := context.Background()

	// Creates a client.
	client, err := vision.NewImageAnnotatorClient(ctx)
	if err != nil {
		sentry.CaptureException(err)
		return
	}
	defer client.Close()

	image, err := vision.NewImageFromReader(bytes.NewBuffer(img_byte))
	if err != nil {
		sentry.CaptureException(err)
		return
	}

	labels, err := client.DetectLabels(ctx, image, nil, 10)
	if err != nil {
		sentry.CaptureException(err)
		return
	}

	for _, label := range labels {
		fmt.Println(label.Description)
		keywords = append(keywords, label.Description)
	}
	return
}
