package instagram

import (
	"encoding/json"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"time"
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
	IGTV     []IGTV          `json:"igtv",omitempty`
	IsExist  string          `json:"is_exist"`
}

type ProfileWithBase64Image struct {
	ID                    string `json:"id"`
	FullName              string `json:"full_name"`
	Biography             string `json:"biography"`
	Username              string `json:"username"`
	IsPrivate             bool   `json:"is_private"`
	ExternalURL           string `json:"external_url"`
	IsVerified            bool   `json:"is_verified"`
	ProfilePicUrl         string `json:"profile_pic_url"`
	ProfilePicBase64Image string `json:"profile_pic_base64_image"`
	StoredProfilePicUrl   string `json:"stored_profile_pic_url"`
	Following             int    `json:"following"`
	Follower              int    `json:"follower"`
	Like                  int    `json:"like"`
	Comment               int    `json:"comment"`
	VideoView             int    `json:"video_view"`
	TotalPost             int    `json:"total_post"`
	AverageLike           int    `json:"average_like"`
	AverageComment        int    `json:"average_comment"`
	AverageVideoView      int    `json:"average_video_view"`

	LastPost []InstagramPostWithBase64Image `json:"last_post"`
	IGTV     []IGTV                         `json:"igtv",omitempty`
	IsExist  string                         `json:"is_exist"`
}

type IGTV struct {
	ID             string `json:"id"`
	Shortcode      string `json:"shortcode"`
	DisplayURL     string `json:"display_url"`
	VideoURL       string `json:"video_url"`
	VideoViewCount int    `json:"video_view_count"`
	Dimensions     struct {
		Height int `json:"height"`
		Width  int `json:"width"`
	} `json:"dimensions"`
	Comment          int     `json:"count"`
	Like             int     `json:"count"`
	TakenAtTimestamp int     `json:"taken_at_timestamp"`
	VideoDuration    float64 `json:"video_duration"`
	Caption          string  `json:"caption"`
}

type RawPost struct {
	TakenAt int    `json:"taken_at"`
	PK      int    `json:"pk"`
	Code    string `json:"code"`
	User    struct {
		Username string `json:"username"`
	} `json:"user"`
	Caption struct {
		Text string `json:"text"`
	} `json:"caption"`
}

type InstagramPostWithBase64Image struct {
	ID                    string         `json:"id"`
	TimestampTaken        int            `json:"timestamp_taken"`
	Shortcode             string         `json:"shortcode"`
	Caption               string         `json:"caption"`
	DisplayURL            string         `json:"display_url"`
	DisplayURLBase64Image string         `json:"display_url_base64_image"`
	Comment               int            `json:"comment"`
	Like                  int            `json:"like"`
	VideoView             int            `json:"video_view"`
	VideoURL              string         `json:"video_url"`
	Username              string         `json:"username"`
	UserID                string         `json:"user_id"`
	ProfilePicURL         string         `json:"profile_pic_url"`
	LastUpdate            string         `json:"last_update"`
	IsVideo               bool           `json:"is_video"`
	StoredDisplayURL      string         `json:"store_display_url"`
	AICategory            []string       `json:"ai_category"`
	IsCarousel            bool           `json:"is_carousel"`
	CarouselPosts         []CarouselPost `json:"carousel_posts"`
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

func GetProfile(username string, myClient *http.Client, start int64) (profile Profile, statusCode int, isRestricted bool, err error) {
	address := "https://www.instagram.com/" + username + "/?__a=1"
	resp, err := myClient.Get(address)
	if err != nil {
		return
	}

	statusCode = resp.StatusCode

	log.Println(resp.StatusCode)
	if statusCode != 200 {
		return
	}

	end := time.Now().Unix()
	if end-start > 50 {
		return
	}

	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return
	}
	log.Println(string(body))

	profile = ParseProfile(string(body))
	isRestricted = IsRestricted(string(body))

	return
}

func GetProfileByScrapeDo(username string, start int64) (profile Profile, statusCode int, isRestricted bool, err error) {
	log.Println("Get profile by scrape do")
	address := "http://api.scrape.do/?token=aa6119eab8424ca5b38c404b2cd1ebed5090de0e2d5&url=https://www.instagram.com/" + username + "/?__a=1"
	myClient := &http.Client{
		Timeout: time.Second * 45,
	}
	resp, err := myClient.Get(address)
	if err != nil {
		return
	}

	statusCode = resp.StatusCode

	log.Println(resp.StatusCode)
	if statusCode != 200 {
		return
	}

	end := time.Now().Unix()
	if end-start > 50 {
		return
	}

	log.Println(end-start, " second")

	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return
	}
	log.Println(string(body))

	profile = ParseProfile(string(body))
	isRestricted = IsRestricted(string(body))

	return
}

func GetProfileByLocalProxy(localProxy string, username string, myClient *http.Client) (profile Profile, statusCode int, isRestricted bool, err error) {
	client := http.Client{
		Timeout: 5 * time.Second,
	}
	url := localProxy + "username?username=" + username
	resp, err := client.Get(url)
	if err != nil {
		log.Println(err)
		return
	}
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	json.Unmarshal(body, &profile)
	return
}

func GetProfileByScraperAPI(username string) (profile Profile, statusCode int, isRestricted bool, err error) {
	myClient := &http.Client{}
	address := os.Getenv("SCRAPERAPI") + username + "?__a=1"
	resp, err := myClient.Get(address)
	if err != nil {
		return
	}

	statusCode = resp.StatusCode

	log.Println(resp.StatusCode)

	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return
	}
	log.Println(string(body))

	profile = ParseProfile(string(body))
	isRestricted = IsRestricted(string(body))

	return
}
