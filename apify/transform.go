package apify

import (
	"encoding/json"
	"log"

	jsoniter "github.com/json-iterator/go"
	"gitlab.com/ariefhidayatulloh/easy-ig/instagram"
)

func transform(input []byte) (output []instagram.Profile, origin []apifyResponse, err error) {
	// json := jsoniter.ConfigCompatibleWithStandardLibrary
	resp := []apifyResponse{}
	err = jsoniter.Unmarshal(input, &resp)
	if err != nil {
		log.Println(err)
		return
	}

	output = []instagram.Profile{}
	for _, v := range resp {
		output = append(output, v.ToProfile())
	}

	temp, _ := json.MarshalIndent(resp, "", "  ")
	log.Println(string(temp))
	origin = resp
	return
}

func (a apifyResponse) ToProfile() instagram.Profile {
	resp := instagram.Profile{
		ID:               a.ID,
		FullName:         a.FullName,
		Biography:        a.Biography,
		Username:         a.Username,
		ExternalURL:      a.ExternalUrl,
		IsVerified:       a.Verified,
		IsPrivate:        a.Private,
		ProfilePicUrl:    a.ProfilePicUrl,
		Following:        a.FollowsCount,
		Follower:         a.FollowersCount,
		Like:             0,
		Comment:          0,
		VideoView:        0,
		TotalPost:        a.PostsCount,
		AverageLike:      0,
		AverageComment:   0,
		AverageVideoView: 0,
		LastPost:         []instagram.InstagramPost{},
	}

	if a.ID != "" {
		resp.IsExist = "yes"
	} else {
		resp.IsExist = "no"
	}

	for _, v := range a.LatestPosts {
		post := getPost(v)
		resp.Like = resp.Like + post.Like
		resp.Comment = resp.Comment + post.Comment
		resp.VideoView = resp.VideoView + post.VideoView
		resp.LastPost = append(resp.LastPost, post)
	}

	if len(a.LatestPosts) > 0 {
		resp.AverageLike = resp.Like / len(a.LatestPosts)
		resp.AverageComment = resp.Comment / len(a.LatestPosts)
		resp.AverageVideoView = resp.VideoView / len(a.LatestPosts)
	}

	for _, v := range a.LatestIgtvVideos {
		igtv := instagram.IGTV{
			ID:             "",
			Shortcode:      v.ShortCode,
			DisplayURL:     v.DisplayUrl,
			VideoURL:       "",
			VideoViewCount: v.VideoViewCount,
			Comment:        v.CommentsCount,
			Like:           v.LikesCount,
			VideoDuration:  v.VideoDuration,
			Caption:        v.Caption,
		}
		igtv.Dimensions.Height = v.DimensionsHeight
		igtv.Dimensions.Width = v.DimensionsWidth
		resp.IGTV = append(resp.IGTV, igtv)
	}

	return resp
}

func getPost(v Post) (post instagram.InstagramPost) {
	var isVideo bool
	if v.Type == "Video" {
		isVideo = true
	}

	isCarousel, carouselPost := getCarouselPost(v)

	post = instagram.InstagramPost{
		ID:             v.ID,
		TimestampTaken: int(v.Timestamp.Unix()),
		Shortcode:      v.ShortCode,
		Caption:        v.Caption,
		DisplayURL:     v.DisplayUrl,
		Comment:        v.CommentsCount,
		Like:           v.LikesCount,
		VideoView:      v.VideoViewCount,
		VideoURL:       v.VideoURL,
		Username:       v.OwnerUsername,
		UserID:         v.OwnerId,
		ProfilePicURL:  "",
		IsVideo:        isVideo,
		IsCarousel:     isCarousel,
		CarouselPosts:  carouselPost,
	}
	return
}

func getCarouselPost(v Post) (isCarousel bool, carouselPost []instagram.CarouselPost) {
	if v.Type == "Sidecar" {
		isCarousel = true

		for _, val := range v.ChildPosts {
			var isVideoCarousel bool
			if val.Type == "Video" {
				isVideoCarousel = true
			}
			cp := instagram.CarouselPost{
				ID:                   val.ID,
				ShortCode:            val.ShortCode,
				DisplayUrl:           val.DisplayUrl,
				AccessibilityCaption: val.Caption,
				IsVideo:              isVideoCarousel,
				VideoViewCount:       val.VideoViewCount,
				TaggedUser:           []instagram.TaggedUser{},
			}

			cp.Dimensions.Height = val.DimensionsHeight
			cp.Dimensions.Width = val.DimensionsWidth

			for _, usr := range val.TaggedUser {
				var tu instagram.TaggedUser
				tu.Id = usr.ID
				tu.FullName = usr.FullName
				tu.Username = usr.Username
				tu.IsVerified = usr.IsVerified
				tu.ProfilePicUrl = usr.ProfilePicUrl
				cp.TaggedUser = append(cp.TaggedUser, tu)
			}

			carouselPost = append(carouselPost, cp)
		}
	}
	return
}
