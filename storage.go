package main

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"strings"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3/s3manager"
)

var endpoint string
var region string
var bucket string
var sess *session.Session

func init() {
	endpoint = config.Space.Endpoint
	region = config.Space.Region
	bucket = config.Space.Bucket
	// os.Setenv("AWS_ACCESS_KEY_ID", "QXFURGCA4RFCTEOOVGYW")
	// os.Setenv("AWS_SECRET_ACCESS_KEY", "69cd95ffa3ce3b8f2b0f13a9b1f2da84141754570c7470555f00f2c71b6f69cf")
	sess = session.Must(session.NewSession(&aws.Config{
		Endpoint: &endpoint,
		Region:   &region,
	}))
}

func Put(data string, path string) error {
	uploader := s3manager.NewUploader(sess)

	result, err := uploader.Upload(&s3manager.UploadInput{
		Bucket:      aws.String(bucket),
		Key:         aws.String(path),
		Body:        strings.NewReader(data),
		ContentType: aws.String("json"),
		ACL:         aws.String("public-read"),
	})
	if err != nil {
		return fmt.Errorf("failed to upload file, %v", err)
	}
	fmt.Printf("file uploaded to, %s\n", aws.StringValue(&result.Location))
	return nil
}

func UploadFile(localPath string, remotePath string) error {

	sess := session.Must(session.NewSession(&aws.Config{
		Endpoint: &endpoint,
		Region:   &region,
	}))

	// Create an uploader with the session and default options
	uploader := s3manager.NewUploader(sess)

	filename := localPath
	f, err := os.Open(filename)
	if err != nil {
		return fmt.Errorf("failed to open file %q, %v", filename, err)
	}

	myBucket := bucket
	myString := remotePath
	// Upload the file to S3.
	result, err := uploader.Upload(&s3manager.UploadInput{
		Bucket: aws.String(myBucket),
		Key:    aws.String(myString),
		Body:   f,
		ACL:    aws.String("public-read"),
	})
	if err != nil {
		return fmt.Errorf("failed to upload file, %v", err)
	}
	fmt.Printf("file uploaded to, %s\n", aws.StringValue(&result.Location))
	return nil
}

func UploadByte(data []byte, remotePath string) (location string, err error) {

	sess := session.Must(session.NewSession(&aws.Config{
		Endpoint: &endpoint,
		Region:   &region,
	}))

	uploader := s3manager.NewUploader(sess)

	myBucket := bucket
	myString := remotePath

	result, err := uploader.Upload(&s3manager.UploadInput{
		Bucket:      aws.String(myBucket),
		Key:         aws.String(myString),
		Body:        bytes.NewReader(data),
		ACL:         aws.String("public-read"),
		ContentType: aws.String(http.DetectContentType(data)),
	})
	if err != nil {
		return
	}
	location = aws.StringValue(&result.Location)
	return
}

func UploadImageFromURL(url, path string) (location string, err error) {
	response, err := http.Get(url)
	if err != nil {
		return
	}

	if response.StatusCode != 200 {
		err = fmt.Errorf("Image not found")
		return
	}
	body, err := ioutil.ReadAll(response.Body)
	if err != nil {
		return
	}

	response.Body.Close()

	location, err = UploadByte(body, path)

	return
}
