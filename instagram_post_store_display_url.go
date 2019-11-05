package main

import (
	"github.com/getsentry/sentry-go"
)

func (i *InstagramPost) StoreDisplayURL() {
	if i.ID != "" {
		var path string
		path = "/insta/post-image/" + i.ID
		_, err := UploadImageFromURL(i.DisplayURL, path)
		if err != nil {
			sentry.CaptureException(err)
		} else {
			i.StoredDisplayURL = "https://abcd.sgp1.cdn.digitaloceanspaces.com" + path
		}
	}
}
