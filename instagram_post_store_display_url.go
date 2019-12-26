package main

import (
	"net/http"

	"github.com/getsentry/sentry-go"
)

func (i *InstagramPost) StoreDisplayURL() {
	if i.ID != "" {
		path := i.GetStoragePath()
		_, err := UploadImageFromURL(i.DisplayURL, path)
		if err != nil {
			sentry.CaptureException(err)
		} else {
			i.StoredDisplayURL = i.GetStorageURL()
		}
	}
}

func (i *InstagramPost) GetStoragePath() (path string) {
	path = "/insta/post-image/" + i.Shortcode
	return
}

func (i *InstagramPost) GetStorageURL() (url string) {
	url = "https://abcd.sgp1.digitaloceanspaces.com" + i.GetStoragePath()
	return
}

func (i *InstagramPost) CheckStoredDisplayURL() (exist bool) {
	if i.ID != "" {
		url := i.GetStorageURL()
		resp, _ := http.Get(url)
		if resp.StatusCode == 200 || resp.StatusCode == 304 {
			exist = true
		}
	}
	return
}
