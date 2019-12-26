package main

import (
	"encoding/json"
	"io/ioutil"
	"net/http"

	"github.com/getsentry/sentry-go"
)

func UpdatePostHandler(w http.ResponseWriter, r *http.Request) {

	var data InstagramPost
	shortcode := _GET(r, "shortcode")

	if len(shortcode) < 5 {
		return
	}

	url := "https://ig.adpl.bz/update-post?shortcode=" + shortcode
	resp, err := http.Get(url)
	if err != nil {
		sentry.CaptureException(err)
	}
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	json.Unmarshal(body, &data)

	if data.ID != "" {
		storeImage := _GET(r, "store_image")
		if storeImage == "1" {
			data.StoreDisplayURL()
		}
	}
	d := struct {
		Type    string
		Request struct {
			Shortcode string
		}
		Response InstagramPost
	}{
		Type:     "ig-api-get-post",
		Response: data,
	}
	d.Request.Shortcode = shortcode
	JSONView(w, r, data, http.StatusOK)
}
