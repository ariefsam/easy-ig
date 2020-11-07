package main

import (
	"log"
	"net/http"
	"net/url"

	"gitlab.com/ariefhidayatulloh/easy-ig/instagram"
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
	ID               string         `json:"id"`
	TimestampTaken   int            `json:"timestamp_taken"`
	Shortcode        string         `json:"shortcode"`
	Caption          string         `json:"caption"`
	DisplayURL       string         `json:"display_url"`
	Comment          int            `json:"comment"`
	Like             int            `json:"like"`
	VideoView        int            `json:"video_view"`
	VideoURL         string         `json:"video_url"`
	Username         string         `json:"username"`
	UserID           string         `json:"user_id"`
	ProfilePicURL    string         `json:"profile_pic_url"`
	LastUpdate       string         `json:"last_update"`
	IsVideo          bool           `json:"is_video"`
	StoredDisplayURL string         `json:"store_display_url"`
	AICategory       []string       `json:"ai_category"`
	IsCarousel       bool           `json:"is_carousel"`
	CarouselPosts    []CarouselPost `json:"carousel_posts"`
}

type CarouselPost struct {
	ID         string `json:"id"`
	ShortCode  string `json:"shortcode"`
	Dimensions struct {
		Height int `json:"height"`
		Width  int `json:"width"`
	} `json:"dimensions"`
	DisplayUrl           string       `json:"display_url"`
	AccessibilityCaption string       `json:"accessibility_caption"`
	IsVideo              bool         `json:"is_video"`
	VideoViewCount       int          `json:"video_view_count"`
	TaggedUser           []TaggedUser `json:"tagged_user"`
}

type TaggedUser struct {
	Id            string `json:"id"`
	FullName      string `json:"full_name"`
	Username      string `json:"username"`
	IsVerified    bool   `json:"is_verified"`
	ProfilePicUrl string `json:"profile_pic_url"`
}

func UsernameHandler(w http.ResponseWriter, r *http.Request) {

	var data Instagram
	data.Username = _GET(r, "username")
	if data.Username == "" {
		JSONView(w, r, nil, http.StatusBadRequest)
		return
	}
	if data.Username != "" {
		if data.Username == "explore" {
			JSONView(w, r, map[string]string{"client_error": "Username not exist or deleted. Your RapidAPI quota still reduced."}, 200)
			return
		}
		myClient := &http.Client{}
		proxyURL, _ := url.Parse(config.Proxy)
		myClient = &http.Client{Transport: &http.Transport{Proxy: http.ProxyURL(proxyURL)}}

		profile, statusCode, isRestricted, err := instagram.GetProfile(data.Username, myClient)

		if err != nil {
			log.Println(err)
		}
		var try int

		for profile.Username == "" && statusCode != 404 && !isRestricted {

			if try > 15 {
				break
			}
			profile, statusCode, isRestricted, _ = instagram.GetProfile(data.Username, myClient)
			try++

		}

		if statusCode == 404 {
			JSONView(w, r, map[string]string{"client_error": "Username not exist or deleted. Your RapidAPI quota still reduced."}, 200)
			return
		}

		if isRestricted {
			JSONView(w, r, map[string]string{"client_error": "Profile restricted for 18+, Our API is public app, so we cannot read restricted profile without login. Your RapidAPI quota still reduced."}, 200)
			return
		}

		if profile.Username == "" {
			JSONView(w, r, map[string]string{"error": "We were sorry, our request blocked by Instagram. Your RapidAPI quota or overage will not be reduced. Please try again, we will try another IP Address."}, http.StatusBadGateway)
			return
		}
		log.Println(data.Username, profile.Follower)
		JSONView(w, r, profile, 200)
		return
	}
	JSONView(w, r, "", 200)
}
