package instagram

import (
	"encoding/json"
)

func ParseProfile(html string) (profile Profile) {
	var page ProfilePage
	json.Unmarshal([]byte(html), &page)

	data := page.Graphql.User
	profile.ID = data.Id
	profile.Username = data.Username
	profile.FullName = data.FullName
	profile.IsPrivate = data.IsPrivate
	profile.ProfilePicUrl = data.ProfilePicUrl
	profile.Follower = data.EdgeFollowedBy.Count
	profile.Following = data.EdgeFollow.Count
	profile.Biography = data.Biography
	profile.TotalPost = data.EdgeOwnderToTimelineMedia.Count
	profile.ExternalURL = data.ExternalURL
	profile.IsVerified = data.IsVerified

	var last_post []InstagramPost
	var account_comment, account_like, account_video_view, totalVideo, tot int

	for _, edge := range data.EdgeOwnderToTimelineMedia.Edges {
		var node PostNode
		var post InstagramPost
		node = edge.Node

		post = node.ToAccountPost()

		account_comment = account_comment + post.Comment
		account_like = account_like + post.Like
		account_video_view = account_video_view + post.VideoView
		tot++
		if edge.Node.IsVideo {
			totalVideo++
		}
		last_post = append(last_post, post)
	}

	if tot > 0 {
		profile.LastPost = last_post

		profile.Comment = account_comment
		profile.Like = account_like
		profile.VideoView = account_video_view
		profile.AverageLike = account_like / tot
		profile.AverageComment = account_comment / tot
		if totalVideo > 0 {
			profile.AverageVideoView = account_video_view / totalVideo
		} else {
			profile.AverageVideoView = 0
		}

	}

	profile.IGTV = []IGTV{}

	for _, v := range page.Graphql.User.EdgeFelixVideoTimeline.Edges {
		caption := ""
		if len(v.Node.EdgeMediaToCaption.Edges) > 0 {
			caption = v.Node.EdgeMediaToCaption.Edges[0].Node.Text
		}
		profile.IGTV = append(profile.IGTV, IGTV{
			ID:               v.Node.ID,
			Comment:          v.Node.EdgeMediaToComment.Count,
			Like:             v.Node.EdgeLikedBy.Count,
			Shortcode:        v.Node.Shortcode,
			DisplayURL:       v.Node.DisplayURL,
			VideoURL:         v.Node.VideoURL,
			VideoViewCount:   v.Node.VideoViewCount,
			Dimensions:       v.Node.Dimensions,
			TakenAtTimestamp: v.Node.TakenAtTimestamp,
			VideoDuration:    v.Node.VideoDuration,
			Caption:          caption,
		})
	}

	return
}

type ProfilePage struct {
	Graphql GraphQL `json:"graphql"`
}

type GraphQL struct {
	User RawUser `json:"user"`
}

type RawUser struct {
	Id             string `json:"id"`
	FullName       string `json:"full_name"`
	Username       string `json:"username"`
	Biography      string `json:"biography"`
	ExternalURL    string `json:"external_url"`
	EdgeFollowedBy struct {
		Count int `json:"count"`
	} `json:"edge_followed_by"`
	EdgeFollow struct {
		Count int `json:"count"`
	} `json:"edge_follow"`
	IsPrivate                 bool   `json:"is_private"`
	IsVerified                bool   `json:"is_verified"`
	ProfilePicUrl             string `json:"profile_pic_url"`
	EdgeOwnderToTimelineMedia struct {
		Count int
		Edges []struct {
			Node PostNode `json:"node"`
		} `json:"edges"`
	} `json:"edge_owner_to_timeline_media"`
	EdgeFelixVideoTimeline EdgeFelixVideoTimeline `json:"edge_felix_video_timeline,omitempty"`
	LastUpdateStatus       string
}

/*
"edge_media_to_caption": {
                "edges": [
                  {
                    "node": {
                      "text": "Today is World Mental Health Day and we support kids' mental health and organizations like @YourMomCares and RxWell that are doing innovative, groundbreaking work in transforming how young people receive the mental health support they need.\nWe are donating to help continue the further development of behavioral health care that was not previously available for adolescents. Text maroon5 to 44-321 to help support this cause as well."
                    }
                  }
                ]
              },
*/
type EdgeFelixVideoTimeline struct {
	Count int `json:"count"`
	Edges []struct {
		Node struct {
			ID             string `json:"id"`
			Shortcode      string `json:"shortcode"`
			DisplayURL     string `json:"display_url"`
			VideoURL       string `json:"video_url"`
			VideoViewCount int    `json:"video_view_count"`
			Dimensions     struct {
				Height int `json:"height"`
				Width  int `json:"width"`
			} `json:"dimensions"`
			EdgeMediaToComment struct {
				Count int `json:"count"`
			} `json:"edge_media_to_comment"`
			EdgeLikedBy struct {
				Count int `json:"count"`
			} `json:"edge_liked_by"`
			EdgeMediaToCaption struct {
				Edges []struct {
					Node struct {
						Text string `json:"text"`
					} `json:"node"`
				} `json:"edges"`
			} `json:"edge_media_to_caption"`
			TakenAtTimestamp int     `json:"taken_at_timestamp"`
			VideoDuration    float64 `json:"video_duration"`
		} `json:"node"`
	} `json:"edges"`
}

type PostPage struct {
	GraphQL struct {
		ShortCodeMedia PostNode `json:"shortcode_media"`
	} `json:"graphql"`
}

// func (node *PostNode) ToAccountPost() InstagramPost {
// 	var post InstagramPost
// 	post.TimestampTaken = node.TakenAtTimestamp
// 	post.UserID = node.Owner.Id
// 	if len(node.EdgeMediaToCaption.Edges) > 0 {
// 		post.Caption = node.EdgeMediaToCaption.Edges[0].Node.Text
// 	}
// 	post.ID = node.Id
// 	post.DisplayURL = node.DisplayUrl
// 	post.Comment = node.EdgeMediaToComment.Count
// 	if post.Comment == 0 {
// 		post.Comment = node.EdgeMediaPreviewComment.Count
// 	}
// 	post.Like = node.EdgeLikedBy.Count
// 	if post.Like == 0 {
// 		post.Like = node.EdgeMediaPreviewLike.Count
// 	}
// 	post.VideoView = node.VideoViewCount
// 	post.VideoURL = node.VideoURL
// 	post.Shortcode = node.ShortCode
// 	post.DisplayURL = node.DisplayUrl
// 	post.IsVideo = node.IsVideo
// 	post.Username = node.Owner.Username
// 	post.UserID = node.Owner.Id
// 	post.ProfilePicURL = node.Owner.ProfilePicURL
// 	t := time.Now()
// 	post.LastUpdate = t.Format(time.RFC3339)
// 	return post
// }
