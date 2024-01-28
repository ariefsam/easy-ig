package apify

import (
	"bytes"
	"encoding/json"
	"io"
	"log"
	"net/http"
	"os"
	"time"
)

type apifyResponse struct {
	InputUrl string `json:"inputUrl"`
	Error    string `json:"error"`
	Username string `json:"username"`
	Url      string `json:"url"`

	ID                   string            `json:"id"`
	FullName             string            `json:"fullName"`
	Biography            string            `json:"biography"`
	ExternalUrl          string            `json:"externalUrl"`
	ExternalUrlShimmed   string            `json:"externalUrlShimmed"`
	FollowersCount       int               `json:"followersCount"`
	FollowsCount         int               `json:"followsCount"`
	HasChannel           bool              `json:"hasChannel"`
	HighlightReelCount   int               `json:"highlightReelCount"`
	IsBusinessAccount    bool              `json:"isBusinessAccount"`
	JoinedRecently       bool              `json:"joinedRecently"`
	BusinessCategoryName string            `json:"businessCategoryName"`
	Private              bool              `json:"private"`
	Verified             bool              `json:"verified"`
	ProfilePicUrl        string            `json:"profilePicUrl"`
	ProfilePicUrlHD      string            `json:"profilePicUrlHD"`
	IgtvVideoCount       int               `json:"igtvVideoCount"`
	RelatedProfiles      []RelatedProfiles `json:"relatedProfiles"`
	LatestIgtvVideos     []IgtvVideos      `json:"latestIgtvVideos"`
	PostsCount           int               `json:"postsCount"`
	LatestPosts          []Post            `json:"latestPosts"`
}

type RelatedProfiles struct {
	ID            string `json:"id"`
	FullName      string `json:"fullName"`
	IsPrivate     bool   `json:"isPrivate"`
	IsVerified    bool   `json:"isVerified"`
	ProfilePicUrl string `json:"profilePicUrl"`
	Username      string `json:"username"`
}

type IgtvVideos struct {
	Type             string  `json:"type"`
	ShortCode        string  `json:"shortCode"`
	Title            string  `json:"title"`
	Caption          string  `json:"caption"`
	CommentsCount    int     `json:"commentsCount"`
	CommentsDisabled bool    `json:"commentsDisabled"`
	DimensionsHeight int     `json:"dimensionsHeight"`
	DimensionsWidth  int     `json:"dimensionsWidth"`
	DisplayUrl       string  `json:"displayUrl"`
	LikesCount       int     `json:"likesCount"`
	VideoDuration    float64 `json:"videoDuration"`
	VideoViewCount   int     `json:"videoViewCount"`
}

type Post struct {
	ID               string       `json:"id"`
	Type             string       `json:"type"`
	ShortCode        string       `json:"shortCode"`
	Caption          string       `json:"caption"`
	Hashtags         []string     `json:"hashtags"`
	Mentions         []string     `json:"mentions"`
	URL              string       `json:"url"`
	CommentsCount    int          `json:"commentsCount"`
	DimensionsHeight int          `json:"dimensionsHeight"`
	DimensionsWidth  int          `json:"dimensionsWidth"`
	DisplayUrl       string       `json:"displayUrl"`
	Images           []string     `json:"images"`
	Alt              string       `json:"alt"`
	LikesCount       int          `json:"likesCount"`
	VideoViewCount   int          `json:"videoViewCount"`
	VideoURL         string       `json:"videoUrl"`
	Timestamp        time.Time    `json:"timestamp"`
	ChildPosts       []Post       `json:"childPosts"`
	LocationName     string       `json:"locationName"`
	LocationId       string       `json:"locationId"`
	OwnerUsername    string       `json:"ownerUsername"`
	OwnerId          string       `json:"ownerId"`
	ProductType      string       `json:"productType"`
	TaggedUser       []TaggedUser `json:"taggedUser"`
	IsPinned         bool         `json:"isPinned"`
}

type TaggedUser struct {
	FullName      string `json:"fullName"`
	ID            string `json:"id"`
	IsVerified    bool   `json:"isVerified"`
	ProfilePicUrl string `json:"profilePicUrl"`
	Username      string `json:"username"`
}

func execute(username []string) (data []byte, err error) {
	token := os.Getenv("APIFY_KEY")
	urlString := "https://api.apify.com/v2/acts/shu8hvrXbJbY3Eb9W/run-sync-get-dataset-items?token=" + token +
		"&memory=256"
		/*
			{
				"directUrls": [
				  "https://www.instagram.com/humansofny/"
				],
				"resultsType": "details",
				"resultsLimit": 200,
				"addParentData": false,
				"searchType": "hashtag",
				"searchLimit": 1
			  }
		*/
	payload := struct {
		DirectUrls    []string `json:"directUrls"`
		ResultsType   string   `json:"resultsType"`
		ResultsLimit  int      `json:"resultsLimit"`
		AddParentData bool     `json:"addParentData"`
		SearchType    string   `json:"searchType"`
		SearchLimit   int      `json:"searchLimit"`
	}{
		ResultsType:   "details",
		ResultsLimit:  200,
		AddParentData: false,
		SearchLimit:   1,
		SearchType:    "user",
	}

	for _, u := range username {
		payload.DirectUrls = append(payload.DirectUrls, "https://www.instagram.com/"+u+"/")
	}

	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		log.Println(err)
		return
	}

	body := bytes.NewBuffer(payloadBytes)

	req, err := http.NewRequest("POST", urlString, body)
	if err != nil {
		log.Println(err)
		return
	}

	req.Header.Set("Content-Type", "application/json")
	client := http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		log.Println(err)
		return
	}

	defer resp.Body.Close()
	responseBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Println(err)
		return
	}

	log.Println(string(responseBytes))
	data = responseBytes

	return
}
