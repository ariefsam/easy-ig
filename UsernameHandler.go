package main

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/allegro/bigcache/v3"
	"gitlab.com/ariefhidayatulloh/easy-ig/apify"
	"gitlab.com/ariefhidayatulloh/easy-ig/instagram"
	"gitlab.com/ariefhidayatulloh/easy-ig/webprofile"
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

var privateLastHit int64

func getIgProfile(r *http.Request, username string) (profile instagram.Profile, clientError map[string]string, systemError error) {
	start := time.Now().Unix()
	if username == "explore" {
		clientError = map[string]string{"client_error": "Username not exist or deleted. Your RapidAPI quota still reduced.", "is_exist": "no"}
		return
	}
	myClient := &http.Client{}
	if config.Proxy != "" {
		proxyURL, _ := url.Parse(config.Proxy)
		myClient = &http.Client{
			Transport: &http.Transport{
				Proxy:                 http.ProxyURL(proxyURL),
				TLSClientConfig:       &tls.Config{InsecureSkipVerify: true},
				ResponseHeaderTimeout: time.Second * 100,
			},
			Timeout: time.Second * 100,
		}

		log.Println("Using proxy: ", proxyURL)
	}

	var try, maxTry int
	var statusCode int
	var isRestricted bool
	var err error

	if config.LocalProxy != "" && _GET(r, "no_proxy") != "1" {
		localClient := &http.Client{}
		log.Println("using local client ", config.LocalProxy)
		profile, statusCode, isRestricted, err = instagram.GetProfileByLocalProxy(config.LocalProxy, username, localClient)
		if profile.IsExist == "no" {
			clientError = map[string]string{"client_error": "Username not exist or deleted. Your RapidAPI quota still reduced.", "is_exist": "no"}
			return
		}
		if profile.Username != "" {
			return
		}
	}

	if config.LocalProxy != "" && _GET(r, "no_proxy") == "1" {
		log.Println("local proxy set, but no proxy choose")
	}

	// if profile.Username == "" && statusCode != 404 && !isRestricted {
	// 	profile, statusCode, isRestricted, err = instagram.GetProfileByScrapeDo(username, start)
	// }

	maxTry = 1
	if config.Proxy == "" {
		maxTry = 1
	}

	for profile.Username == "" && statusCode != 404 && !isRestricted {

		log.Println("Trying using proxy ", try, "username", username)

		if os.Getenv("SCRAPERAPI") != "" {
			profile, statusCode, isRestricted, err = instagram.GetProfileByScraperAPI(username)
		} else {
			profile, statusCode, isRestricted, err = instagram.GetProfile(username, myClient, start)
		}
		if err != nil {
			log.Println(err)
		}
		try++
		if try >= maxTry {
			break
		}

	}

	// if profile.Username == "" && statusCode != 404 && !isRestricted && time.Now().Unix()-privateLastHit > 30 {
	// 	log.Println("Using private API to get Profile ", username)
	// 	privateLastHit = time.Now().Unix()
	// 	profile, err = private.GetProfile(username)
	// 	if err != nil {
	// 		log.Println(err, "error get profile")
	// 	}
	// 	if profile.IsExist == "no" {
	// 		clientError = map[string]string{"client_error": "Username not exist or deleted. Your RapidAPI quota still reduced.", "is_exist": "no"}
	// 		statusCode = 404
	// 		return
	// 	}

	// }

	// http://api.scrape.do/?token=aa6119eab8424ca5b38c404b2cd1ebed5090de0e2d5&url=https://www.instagram.com/maroon5/?__a=1
	// log.Println(err)

	if statusCode == 404 {
		clientError = map[string]string{"client_error": "Username not exist or deleted. Your RapidAPI quota still reduced.", "is_exist": "no"}
		return
	}

	if isRestricted {
		clientError = map[string]string{"client_error": "Profile restricted for 18+, Our API is public app, so we cannot read restricted profile without login. Your RapidAPI quota still reduced."}
		return
	}

	if profile.Username == "" {
		// clientError = map[string]string{"client_error": "Username not exist or deleted. Your RapidAPI quota still reduced.", "is_exist": "no"}
		time.Sleep(30 * time.Second)
		systemError = errors.New("We were sorry, our request blocked by Instagram. Your RapidAPI quota or overage will not be reduced. Please try again, we will try another IP Address.")
		return
	}

	log.Println(statusCode, username, profile.Follower, ". Profile id:", profile.ID)
	return
}

var cfg = bigcache.Config{
	// number of shards (must be a power of 2)
	Shards: 1024,

	// time after which entry can be evicted
	LifeWindow: 10 * time.Minute,

	// Interval between removing expired entries (clean up).
	// If set to <= 0 then no action is performed.
	// Setting to < 1 second is counterproductive — bigcache has a one second resolution.
	CleanWindow: 5 * time.Minute,

	// rps * lifeWindow, used only in initial memory allocation
	MaxEntriesInWindow: 1000 * 10 * 60,

	// max entry size in bytes, used only in initial memory allocation
	MaxEntrySize: 500,

	// prints information about additional memory allocation
	Verbose: true,

	// cache will not allocate more memory than this limit, value in MB
	// if value is reached then the oldest entries can be overridden for the new ones
	// 0 value means no size limit
	HardMaxCacheSize: 900,

	// callback fired when the oldest entry is removed because of its expiration time or no space left
	// for the new entry, or because delete was called. A bitmask representing the reason will be returned.
	// Default value is nil which means no callback and it prevents from unwrapping the oldest entry.
	OnRemove: nil,

	// OnRemoveWithReason is a callback fired when the oldest entry is removed because of its expiration time or no space left
	// for the new entry, or because delete was called. A constant representing the reason will be passed through.
	// Default value is nil which means no callback and it prevents from unwrapping the oldest entry.
	// Ignored if OnRemove is specified.
	OnRemoveWithReason: nil,
}

// cache, initErr := bigcache.New(context.Background(), config)
var c, _ = bigcache.New(context.Background(), cfg)

type ResponseUsername struct {
	Profile      instagram.Profile `json:"profile"`
	StatusCode   int               `json:"status_code"`
	IsRestricted bool              `json:"is_restricted"`
}

func isAlphanumeric(input string) bool {
	for _, char := range input {
		if !(char >= 'a' && char <= 'z') && !(char >= 'A' && char <= 'Z') && !(char >= '0' && char <= '9') && char != '_' && char != '.' {
			return false
		}
	}
	return true
}

func GetWebProfile(username string) (profile instagram.Profile, statusCode int, isRestricted bool, err error) {
	log.Println("Checcking username: ", username)
	if len(username) == 0 || strings.ContainsAny(username, "@/ ") || !isAlphanumeric(username) {
		statusCode = 404
		return
	}

	defer func() {
		if errs := recover(); errs != nil {
			log.Println("panic occurred:", errs)
			err = errors.New("service interrupted")
			return
		}
	}()
	resp := ResponseUsername{}
	if v, ok := c.Get(username); ok == nil {
		log.Println(string(v))
		err = json.Unmarshal(v, &resp)
		if err == nil {

			profile = resp.Profile
			statusCode = resp.StatusCode

			log.Println("from cache, status", statusCode, ". username:", username, ". Name", profile.FullName, ". Follower", profile.Follower)
			return
		}

		log.Println(err)

	}

	profile, statusCode, isRestricted, err = webprofile.GetWebProfile(username)
	if err != nil {
		log.Println(err)
		return
	}

	resp.Profile = profile
	resp.StatusCode = statusCode
	resp.IsRestricted = isRestricted
	respByte, err := json.Marshal(resp)
	if err != nil {
		log.Println(err)
		return
	}
	if statusCode == 200 {

		err = c.Set(username, respByte)
		if err != nil {
			log.Println(err)
		}
	}
	if statusCode == 404 {
		err = c.Set(username, respByte)
		if err != nil {
			log.Println(err)
		}
	}
	return

}

func UsernameHandler(w http.ResponseWriter, r *http.Request) {

	username := _GET(r, "username")
	if username == "" {
		JSONView(w, r, map[string]string{"client_error": "Username not exist or deleted. Your RapidAPI quota still reduced.", "is_exist": "no"}, 200)
		return
	}

	headers, _ := json.MarshalIndent(r.Header["X-Rapidapi-User"], "", " ")
	log.Println("headers", string(headers))

	profile, statusCode, isRestricted, err := GetWebProfile(username)
	if err != nil {
		log.Println(err)
		log.Println("system error")
		JSONView(w, r, map[string]string{"error": "system error"}, http.StatusInternalServerError)
		return
	}

	if isRestricted {
		log.Println("restricted profile")
		JSONView(w, r, map[string]string{"client_error": "Profile restricted for 18+, Our API is public app, so we cannot read restricted profile without login. Your RapidAPI quota still reduced."}, 200)
		return
	}

	if statusCode == 404 {
		JSONView(w, r, map[string]string{"client_error": "Username not exist or deleted. Your RapidAPI quota still reduced.", "is_exist": "no"}, 200)
		return
	}
	log.Println("from webprofile", statusCode, username, profile.Follower, profile.Biography)
	JSONView(w, r, profile, 200)
	return

}

func UsernameHandlerApify(w http.ResponseWriter, r *http.Request) {

	var data Instagram
	data.Username = _GET(r, "username")
	if data.Username == "" {
		JSONView(w, r, nil, http.StatusBadRequest)
		return
	}

	// profile, errClient, errSystem := getIgProfile(r, data.Username)
	// if errClient != nil {
	// 	JSONView(w, r, errClient, 200)
	// 	return
	// }
	// if errSystem != nil {
	// 	JSONView(w, r, map[string]string{"error": errSystem.Error()}, http.StatusGatewayTimeout)
	// 	return
	// }

	profile, err := apify.Username(data.Username)
	if err != nil {
		log.Println(err)
		JSONView(w, r, map[string]string{"error": "system error"}, http.StatusInternalServerError)
	}

	if profile.IsExist == "no" {
		JSONView(w, r, map[string]string{"client_error": "Username not exist or deleted. Your RapidAPI quota still reduced.", "is_exist": "no"}, 200)
		return
	}

	JSONView(w, r, profile, 200)

	return

}
