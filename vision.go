package main

import (
	"bytes"
	"context"
	"fmt"
	"log"

	vision "cloud.google.com/go/vision/apiv1"
)

func VisionTest() {
	img_url := "https://scontent-sin2-2.cdninstagram.com/vp/dec85a97aa9fb7eb469ca932f319d8f1/5E4C6F89/t51.2885-15/e35/p1080x1080/49289806_2222648331090389_5010698129034418584_n.jpg?_nc_ht=scontent-sin2-2.cdninstagram.com&_nc_cat=105"
	img_byte, err := GetImageFromURL(img_url)
	if err != nil {
		log.Println(err)
		return
	}
	ctx := context.Background()

	// Creates a client.
	client, err := vision.NewImageAnnotatorClient(ctx)
	if err != nil {
		log.Fatalf("Failed to create client: %v", err)
	}
	defer client.Close()

	image, err := vision.NewImageFromReader(bytes.NewBuffer(img_byte))
	if err != nil {
		log.Fatalf("Failed to create image: %v", err)
	}

	labels, err := client.DetectLabels(ctx, image, nil, 10)
	if err != nil {
		log.Fatalf("Failed to detect labels: %v", err)
	}

	fmt.Println("Labels:")
	for _, label := range labels {
		fmt.Println(label.Description)
	}

	//la, err := client.DetectImageProperties(ctx, image, nil, 10)
}

func detectLandmarks() (err error) {
	img_url := "https://scontent-sin2-2.cdninstagram.com/vp/42cce9cee9e6b1d8f42c2e021b03d9e1/5E3F7135/t51.2885-15/sh0.08/e35/p750x750/69521236_2454631951284805_2295791645817830241_n.jpg?_nc_ht=scontent-sin2-2.cdninstagram.com&_nc_cat=101"
	img_byte, err := GetImageFromURL(img_url)
	if err != nil {
		log.Println(err)
		return
	}
	log.Println("Detect")
	ctx := context.Background()

	client, err := vision.NewImageAnnotatorClient(ctx)
	if err != nil {
		log.Println(err)
		return err
	}

	image, err := vision.NewImageFromReader(bytes.NewBuffer(img_byte))
	if err != nil {
		log.Println(err)
		return err
	}
	annotations, err := client.DetectLandmarks(ctx, image, nil, 10)
	if err != nil {
		log.Println(err)
		return err
	}
	log.Println("testing")
	if len(annotations) == 0 {
		fmt.Println("No landmarks found.")
	} else {
		fmt.Println("Landmarks:")
		for _, annotation := range annotations {
			fmt.Println(annotation.Description)
		}
	}

	return nil
}
