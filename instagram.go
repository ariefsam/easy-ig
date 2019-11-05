package main

import (
	"encoding/json"
	"io/ioutil"
	"net/http"

	"github.com/getsentry/sentry-go"
)

type Instagram struct {
	ID                  string          `json:"id"`
	FullName            string          `json:"full_name"`
	Biography           string          `json:"biography"`
	Username            string          `json:"username"`
	IsPrivate           bool            `json:"is_private"`
	IsVerified          bool            `json:"is_verified"`
	ProfilePicUrl       string          `json:"profile_pic_url"`
	StoredProfilePicUrl string          `json:"stored_profile_pic_url"`
	Following           int             `json:"following"`
	Follower            int             `json:"follower"`
	Like                int             `json:"like"`
	Comment             int             `json:"comment"`
	VideoView           int             `json:"video_view"`
	TotalPost           int             `json:"total_post"`
	AverageLike         int             `json:"average_like"`
	AverageComment      int             `json:"average_comment"`
	AverageVideoView    int             `json:"average_video_view"`
	LastUpdate          string          `json:"last_update"`
	LastUpdateStatus    string          `json:"last_update_status"`
	LastPost            []InstagramPost `json:"last_post"`
}

type InstagramPost struct {
	ID               string `json:"id"`
	TimestampTaken   int    `json:"timestamp_taken"`
	Shortcode        string `json:"shortcode"`
	Caption          string `json:"caption"`
	DisplayURL       string `json:"display_url"`
	Comment          int    `json:"comment"`
	Like             int    `json:"like"`
	VideoView        int    `json:"video_view"`
	VideoURL         string `json:"video_url"`
	Username         string `json:"username"`
	UserID           string `json:"user_id"`
	ProfilePicURL    string `json:"profile_pic_url"`
	LastUpdate       string `json:"last_update"`
	IsVideo          bool   `json:"is_video"`
	StoredDisplayURL string `json:"store_display_url"`
}

func UsernameHandler(w http.ResponseWriter, r *http.Request) {

	var data Instagram
	data.Username = _GET(r, "username")
	if data.Username == "" {
		JSONView(w, r, nil, http.StatusBadRequest)
		return
	}
	if data.Username != "" {
		resp, err := http.Get("https://ig.adpl.bz/update-ig?username=" + data.Username)
		if err != nil {
			sentry.CaptureException(err)
		}

		url := "https://adf.sgp1.digitaloceanspaces.com/ig/account/username/" + data.Username

		resp, err = http.Get(url)
		if err != nil {
			sentry.CaptureException(err)
		}
		defer resp.Body.Close()
		body, err := ioutil.ReadAll(resp.Body)
		json.Unmarshal(body, &data)

		data.StoreProfilePic()

	}

	JSONView(w, r, data, http.StatusOK)
}

func PostHandler(w http.ResponseWriter, r *http.Request) {

	var data []InstagramPost
	igID := _GET(r, "instagram_id")
	if igID != "" {

		url := "https://adf.sgp1.digitaloceanspaces.com/ig/account/post/" + igID
		resp, err := http.Get(url)
		if err != nil {
			sentry.CaptureException(err)
		}
		defer resp.Body.Close()
		body, err := ioutil.ReadAll(resp.Body)
		json.Unmarshal(body, &data)
	}

	d := struct {
		Type    string
		Request struct {
			ID string
		}
		Response []InstagramPost
	}{
		Type:     "ig-api-username",
		Response: data,
	}
	d.Request.ID = igID
	//Log(d)
	JSONView(w, r, data, http.StatusOK)
}

func GetPostHandler(w http.ResponseWriter, r *http.Request) {

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

func GetImageFromURL(url string) (img []byte, err error) {
	response, err := http.Get(url)
	if err != nil {
		return
	}

	body, err := ioutil.ReadAll(response.Body)
	if err != nil {
		return
	}

	response.Body.Close()
	img = body
	return
}
