package instagram

import "encoding/json"

type mediaCandidateV1 struct {
	URL    string `json:"url"`
	Width  int    `json:"width"`
	Height int    `json:"height"`
}

type mediaVersionV1 struct {
	URL string `json:"url"`
}

type mediaItemV1 struct {
	ID                   string  `json:"id"`
	Code                 string  `json:"code"`
	TakenAt              int     `json:"taken_at"`
	MediaType            int     `json:"media_type"`
	LikeCount            int     `json:"like_count"`
	CommentCount         int     `json:"comment_count"`
	PlayCount            *int    `json:"play_count"`
	ViewCount            *int    `json:"view_count"`
	AccessibilityCaption string  `json:"accessibility_caption"`
	Caption              *struct {
		Text string `json:"text"`
	} `json:"caption"`
	ImageVersions2 struct {
		Candidates []mediaCandidateV1 `json:"candidates"`
	} `json:"image_versions2"`
	VideoVersions []mediaVersionV1 `json:"video_versions"`
	User          struct {
		Pk            string `json:"pk"`
		Username      string `json:"username"`
		ProfilePicURL string `json:"profile_pic_url"`
	} `json:"user"`
	CarouselMedia []mediaItemV1 `json:"carousel_media"`
}

type postWebInfoResponseV1 struct {
	Data struct {
		WebInfo struct {
			Items []mediaItemV1 `json:"items"`
		} `json:"xdt_api__v1__media__shortcode__web_info"`
	} `json:"data"`
}

func (item mediaItemV1) videoViewCount() int {
	if item.PlayCount != nil {
		return *item.PlayCount
	}
	if item.ViewCount != nil {
		return *item.ViewCount
	}
	return 0
}

func (item mediaItemV1) toCarouselPost() CarouselPost {
	var cp CarouselPost
	cp.ID = item.ID
	cp.ShortCode = item.Code
	cp.AccessibilityCaption = item.AccessibilityCaption
	cp.IsVideo = item.MediaType == 2
	cp.VideoViewCount = item.videoViewCount()
	if len(item.ImageVersions2.Candidates) > 0 {
		candidate := item.ImageVersions2.Candidates[0]
		cp.DisplayUrl = candidate.URL
		cp.Dimensions.Width = candidate.Width
		cp.Dimensions.Height = candidate.Height
	}
	return cp
}

// ParsePostV1 parses the response of the PolarisPostRootQuery GraphQL query
// (doc_id 27128499623469141), which Instagram serves in v1/iPhone API shape
// under data.xdt_api__v1__media__shortcode__web_info.items[0].
func ParsePostV1(raw string, shortcode string) (post InstagramPost) {
	post.Shortcode = shortcode

	var resp postWebInfoResponseV1
	json.Unmarshal([]byte(raw), &resp)

	if len(resp.Data.WebInfo.Items) == 0 {
		return
	}

	item := resp.Data.WebInfo.Items[0]

	post.ID = item.ID
	if item.Code != "" {
		post.Shortcode = item.Code
	}
	post.TimestampTaken = item.TakenAt
	if item.Caption != nil {
		post.Caption = item.Caption.Text
	}
	post.Comment = item.CommentCount
	post.Like = item.LikeCount
	post.Username = item.User.Username
	post.UserID = item.User.Pk
	post.ProfilePicURL = item.User.ProfilePicURL
	post.IsVideo = item.MediaType == 2
	post.VideoView = item.videoViewCount()

	if len(item.ImageVersions2.Candidates) > 0 {
		post.DisplayURL = item.ImageVersions2.Candidates[0].URL
	}
	if len(item.VideoVersions) > 0 {
		post.VideoURL = item.VideoVersions[0].URL
	}

	if item.MediaType == 8 && len(item.CarouselMedia) > 0 {
		post.IsCarousel = true
		for _, child := range item.CarouselMedia {
			post.CarouselPosts = append(post.CarouselPosts, child.toCarouselPost())
			post.VideoView += child.videoViewCount()
		}
	}

	return
}
