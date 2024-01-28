package instagram

import (
	"encoding/json"
)

type UserPage struct {
	EntryData struct {
		ProfilePage []struct {
			Graphql GraphQL `json:"graphql"`
		} `json:"ProfilePage"`
	} `json:"entry_data"`
}

type UserPageA struct {
	Graphql GraphQL `json:"graphql"`
}

type User struct {
	Id             string `json:"id"`
	FullName       string `json:"full_name"`
	Username       string `json:"username"`
	Biography      string `json:"biography"`
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
	LastUpdateStatus string
}

type PostNode struct {
	TypeName           string `json:"__typename"`
	Id                 string `json:"id"`
	ShortCode          string `json:"shortcode"`
	EdgeMediaToComment struct {
		Count int `json:"count"`
	} `json:"edge_media_to_comment"`
	EdgeMediaPreviewComment struct {
		Count int `json:"count"`
	} `json:"edge_media_preview_comment"`
	EdgeMediaToCaption struct {
		Edges []struct {
			Node struct {
				Text string `json:"text"`
			} `json:"node"`
		} `json:"edges"`
	} `json:"edge_media_to_caption"`
	DisplayUrl       string `json:"display_url"`
	TakenAtTimestamp int    `json:"taken_at_timestamp"`
	EdgeLikedBy      struct {
		Count int `json:"count"`
	} `json:"edge_liked_by"`
	EdgeMediaPreviewLike struct {
		Count int `json:"count"`
	} `json:"edge_media_preview_like"`
	Owner struct {
		Id            string `json:"id"`
		Username      string `json:"username"`
		ProfilePicURL string `json:"profile_pic_url"`
	} `json:"owner"`
	IsVideo               bool   `json:"is_video"`
	VideoURL              string `json:"video_url"`
	VideoViewCount        int    `json:"video_view_count"`
	EdgeSidecarToChildren struct {
		Edges []struct {
			Node struct {
				ID         string `json:"id"`
				ShortCode  string `json:"shortcode"`
				Dimensions struct {
					Height int `json:"height"`
					Width  int `json:"width"`
				} `json:"dimensions"`
				DisplayUrl            string `json:"display_url"`
				AccessibilityCaption  string `json:"accessibility_caption"`
				IsVideo               bool   `json:"is_video"`
				VideoViewCount        int    `json:"video_view_count"`
				EdgeMediaToTaggedUser struct {
					Edges []struct {
						Node struct {
							User struct {
								Id            string `json:"id"`
								FullName      string `json:"full_name"`
								Username      string `json:"username"`
								IsVerified    bool   `json:"is_verified"`
								ProfilePicUrl string `json:"profile_pic_url"`
							} `json:"user"`
							X float64 `json:"x"`
							Y float64 `json:"y"`
						} `json:"node"`
					} `json:"edges"`
				} `json:"edge_media_to_tagged_user"`
			} `json:"node"`
		} `json:"Edges"`
	} `json:"edge_sidecar_to_children"`
}

func (node *PostNode) ToAccountPost() InstagramPost {
	var post InstagramPost
	post.TimestampTaken = node.TakenAtTimestamp
	post.UserID = node.Owner.Id
	if len(node.EdgeMediaToCaption.Edges) > 0 {
		post.Caption = node.EdgeMediaToCaption.Edges[0].Node.Text
	}
	post.ID = node.Id
	post.DisplayURL = node.DisplayUrl
	post.Comment = node.EdgeMediaToComment.Count
	if post.Comment == 0 {
		post.Comment = node.EdgeMediaPreviewComment.Count
	}
	post.Like = node.EdgeMediaPreviewLike.Count
	if post.Like == 0 {
		post.Like = node.EdgeLikedBy.Count
	}
	post.VideoView = node.VideoViewCount
	post.VideoURL = node.VideoURL
	post.Shortcode = node.ShortCode
	post.DisplayURL = node.DisplayUrl
	post.IsVideo = node.IsVideo
	post.Username = node.Owner.Username
	post.UserID = node.Owner.Id
	// t := time.Now()
	// post.LastUpdate = t.Format(time.RFC3339)
	if len(node.EdgeSidecarToChildren.Edges) > 0 {
		post.IsCarousel = true
		for _, val := range node.EdgeSidecarToChildren.Edges {
			var cp CarouselPost
			cp.AccessibilityCaption = val.Node.AccessibilityCaption
			cp.Dimensions = val.Node.Dimensions
			cp.DisplayUrl = val.Node.DisplayUrl
			cp.ID = val.Node.ID
			cp.IsVideo = val.Node.IsVideo
			cp.VideoViewCount = val.Node.VideoViewCount
			post.VideoView += cp.VideoViewCount
			cp.ShortCode = val.Node.ShortCode
			for _, usr := range val.Node.EdgeMediaToTaggedUser.Edges {
				var tu TaggedUser
				tu.Id = usr.Node.User.Id
				tu.FullName = usr.Node.User.FullName
				tu.Username = usr.Node.User.Username
				tu.IsVerified = usr.Node.User.IsVerified
				tu.ProfilePicUrl = usr.Node.User.ProfilePicUrl
				cp.TaggedUser = append(cp.TaggedUser, tu)
			}

			post.CarouselPosts = append(post.CarouselPosts, cp)
		}
	}
	return post
}

func ParsePost(raw string) (post InstagramPost) {
	rawPost := PostPage{}
	json.Unmarshal([]byte(raw), &rawPost)
	post = rawPost.GraphQL.ShortCodeMedia.ToAccountPost()
	return
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
