package main

import (
	"encoding/json"
	"io/ioutil"
	"log"
	"net/http"
)

type Instagram struct {
	ID               string          `json:"id"`
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
	IScore           float64         `json:"iscore"`
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
	log.Println("request username handle")
	var data Instagram
	data.Username = _GET(r, "username")
	log.Println(data.Username)
	if data.Username != "" {
		http.Get("http://ig.adpl.bz/update-ig?username=" + data.Username)
		url := "https://adf.sgp1.digitaloceanspaces.com/ig/account/username/" + data.Username
		log.Println(url)
		resp, err := http.Get(url)
		if err != nil {
			// handle error
			log.Println(err)
		}
		defer resp.Body.Close()
		body, err := ioutil.ReadAll(resp.Body)
		json.Unmarshal(body, &data)
		log.Println(string(body))
	}
	JSONView(w, r, data)
}
