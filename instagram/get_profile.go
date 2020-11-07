package instagram

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
)

type Profile struct {
	ID                  string `json:"id"`
	FullName            string `json:"full_name"`
	Biography           string `json:"biography"`
	Username            string `json:"username"`
	IsPrivate           bool   `json:"is_private"`
	ExternalURL         string `json:"external_url"`
	IsVerified          bool   `json:"is_verified"`
	ProfilePicUrl       string `json:"profile_pic_url"`
	StoredProfilePicUrl string `json:"stored_profile_pic_url"`
	Following           int    `json:"following"`
	Follower            int    `json:"follower"`
	Like                int    `json:"like"`
	Comment             int    `json:"comment"`
	VideoView           int    `json:"video_view"`
	TotalPost           int    `json:"total_post"`
	AverageLike         int    `json:"average_like"`
	AverageComment      int    `json:"average_comment"`
	AverageVideoView    int    `json:"average_video_view"`

	LastPost []InstagramPost `json:"last_post"`
}

type InstagramPost struct {
	ID               string   `json:"id"`
	TimestampTaken   int      `json:"timestamp_taken"`
	Shortcode        string   `json:"shortcode"`
	Caption          string   `json:"caption"`
	DisplayURL       string   `json:"display_url"`
	Comment          int      `json:"comment"`
	Like             int      `json:"like"`
	VideoView        int      `json:"video_view"`
	VideoURL         string   `json:"video_url"`
	Username         string   `json:"username"`
	UserID           string   `json:"user_id"`
	ProfilePicURL    string   `json:"profile_pic_url"`
	LastUpdate       string   `json:"last_update"`
	IsVideo          bool     `json:"is_video"`
	StoredDisplayURL string   `json:"store_display_url"`
	AICategory       []string `json:"ai_category"`
}

func GetProfile(username string, myClient *http.Client) (profile Profile, statusCode int, isRestricted bool, err error) {
	address := "https://instagram.com/" + username + "?__a=1"
	resp, err := myClient.Get(address)
	if err != nil {
		return
	}

	statusCode = resp.StatusCode

	fmt.Println(resp.StatusCode)
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return
	}

	profile = ParseProfile(string(body))
	isRestricted = IsRestricted(string(body))

	return
}

func GetProfileByLocalProxy(localProxy string, username string, myClient *http.Client) (profile Profile, statusCode int, isRestricted bool, err error) {
	url := localProxy + "username?username=" + username
	resp, err := http.Get(url)
	if err != nil {
		log.Println(err)
		return
	}
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	json.Unmarshal(body, &profile)
	return
}
