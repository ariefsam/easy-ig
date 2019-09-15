package main

import (
	"encoding/json"
	"io/ioutil"
	"log"
	"net/http"
)

type Instagram struct {
	InstagramID      string          `json:"instagram_id"`
	FullName         string          `json:"full_name"`
	Biography        string          `json:"biography"`
	Username         string          `json:"username"`
	IsPrivate        bool            `json:"is_private"`
	IsVerified       bool            `json:"is_verified"`
	ProfilePicUrl    string          `json:"profile_pic_url"`
	Following        int             `json:"following"`
	Follower         int             `json:"follower"`
	Like             int             `json:"like"`
	Comment          int             `json:"comment"`
	VideoView        int             `json:"video_view"`
	TotalPost        int             `json:"total_post"`
	AverageLike      int             `json:"average_like"`
	AverageComment   int             `json:"average_comment"`
	AverageVideoView int             `json:"average_video_view"`
	LastUpdate       string          `json:"last_update"`
	LastUpdateStatus string          `json:"last_update_status"`
	LastPost         []InstagramPost `json:"last_post"`
}

type InstagramPost struct {
	ID             string `json:"id"`
	TimestampTaken int    `json:"timestamp_taken"`
	Shortcode      string `json:"shortcode"`
	Caption        string `json:"caption"`
	DisplayURL     string `json:"display_url"`
	Comment        int    `json:"comment"`
	Like           int    `json:"like"`
	VideoView      int    `json:"video_view"`
	Username       string `json:"username"`
	UserID         string `json:"user_id"`
	LastUpdate     string `json:"last_update"`
	IsVideo        bool   `json:"is_video"`
}

func UsernameHandler(w http.ResponseWriter, r *http.Request) {

	var data Instagram
	data.Username = _GET(r, "username")
	if data.Username != "" {
		resp, err := http.Get("https://ig.adpl.bz/update-ig?username=" + data.Username)
		if err != nil {
			Log(err)
		}

		url := "https://adf.sgp1.digitaloceanspaces.com/ig/account/username/" + data.Username
		Log(url)
		resp, err = http.Get(url)
		if err != nil {
			Log(err)
		}
		defer resp.Body.Close()
		body, err := ioutil.ReadAll(resp.Body)
		json.Unmarshal(body, &data)
		var dataId struct {
			ID string `json:"id"`
		}
		json.Unmarshal(body, &data)
		json.Unmarshal(body, &dataId)
		data.InstagramID = dataId.ID
		log.Println(string(body))
	}

	d := struct {
		Type    string
		Request struct {
			Username string
		}
		Response Instagram
	}{
		Type:     "ig-api-username",
		Response: data,
	}
	d.Request.Username = data.Username
	Log(d)
	JSONView(w, r, data)
}

func PostHandler(w http.ResponseWriter, r *http.Request) {

	var data []InstagramPost
	igID := _GET(r, "instagram_id")
	if igID != "" {

		url := "https://adf.sgp1.digitaloceanspaces.com/ig/account/post/" + igID
		Log(url)
		resp, err := http.Get(url)
		if err != nil {
			Log(err)
		}
		defer resp.Body.Close()
		body, err := ioutil.ReadAll(resp.Body)
		json.Unmarshal(body, &data)
		log.Println(string(body))
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
	Log(d)
	JSONView(w, r, data)
}
