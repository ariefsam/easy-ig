package main

import (
	"encoding/json"
	"io/ioutil"
	"log"
	"net/http"

	"github.com/getsentry/sentry-go"
)

func (i *InstagramPost) StoreDisplayURL() {
	if i.ID != "" {
		path := i.GetImageStoragePath()
		_, err := UploadImageFromURL(i.DisplayURL, path)
		if err != nil {
			sentry.CaptureException(err)
		} else {
			i.StoredDisplayURL = i.GetImageStorageURL()
		}
	}
}

func (i *InstagramPost) GetImageStoragePath() (path string) {
	path = "/insta/post-image/" + i.Shortcode
	return
}

func (i *InstagramPost) GetImageStorageURL() (url string) {
	url = "https://abcd.sgp1.digitaloceanspaces.com" + i.GetImageStoragePath()
	return
}

func (i *InstagramPost) GetStoragePath() (path string) {
	path = "/insta/post/" + i.Shortcode
	return
}

func (i *InstagramPost) GetStorageURL() (url string) {
	url = "https://abcd.sgp1.digitaloceanspaces.com" + i.GetStoragePath()
	return
}

func (i *InstagramPost) CheckStoredDisplayURL() (exist bool) {
	if i.ID != "" {
		url := i.GetImageStorageURL()
		resp, _ := http.Get(url)
		if resp.StatusCode == 200 || resp.StatusCode == 304 {
			exist = true
		}
	}
	return
}

func (i *InstagramPost) CheckStoredVision() (exist bool) {
	if i.ID != "" {
		url := i.GetStorageURL()
		log.Println("Checking " + url)
		resp, _ := http.Get(url)
		if resp.StatusCode == 200 || resp.StatusCode == 304 {
			body, err := ioutil.ReadAll(resp.Body)
			if err == nil {
				var ipost InstagramPost
				err = json.Unmarshal(body, &ipost)
				if ipost.AICategory == nil {
					exist = false
					log.Println("Vision", i.Shortcode, "not exist")
				} else if len(ipost.AICategory) > 0 {
					exist = true
					log.Println("Vision", i.Shortcode, " exist")
					i.AICategory = ipost.AICategory
				}
			}
		} else {
			exist = false
		}
	}
	return
}
