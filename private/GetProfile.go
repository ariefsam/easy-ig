package private

import (
	"fmt"
	"log"

	"gitlab.com/ariefhidayatulloh/easy-ig/ariefjson"
	"gitlab.com/ariefhidayatulloh/easy-ig/instagram"
)

func GetProfile(username string) (profile instagram.Profile, err error) {
	ig, err := SelectAccount()
	if err != nil {
		return
	}

	// log.Println(ariefjson.MarshalIndent(ig))

	// module := "newsfeed"
	if ig.Profiles == nil {
		return
	}

	userInfo, err := ig.Profiles.ByName(username)
	if err != nil {
		if err.Error() == "UserInfoResponse: Challenge required." {
			log.Println("got here")
		}
		return
	}

	// log.Println(sInfo.)
	profile.FullName = userInfo.FullName
	profile.ID = fmt.Sprint(userInfo.ID)
	profile.Biography = userInfo.Biography
	profile.Username = userInfo.Username
	profile.IsPrivate = userInfo.IsPrivate
	profile.ExternalURL = userInfo.ExternalURL
	profile.ProfilePicUrl = userInfo.ProfilePicURL
	profile.Following = userInfo.FollowingCount
	profile.Follower = userInfo.FollowerCount
	profile.TotalPost = userInfo.MediaCount
	// louserInfo.User.LatestReelMedia

	feed := userInfo.Feed() // ig.P .Timeline. .GetUserFeed(userInfo.User.Pk, nil)
	feed.Next()
	// log.Println(ariefjson.MarshalIndent(userInfo))
	// log.Println(ariefjson.MarshalIndent(feed))
	if feed.Items == nil {
		return
	}

	var totalLastPost, totalLike, totalComment, totalVideoView int

	for _, v := range feed.Items {
		// if totalLastPost == 12 {
		// 	continue
		// }
		totalLastPost++
		rowMedia := instagram.InstagramPost{}
		rowMedia.ID = fmt.Sprint(v.ID)
		rowMedia.Shortcode = v.Code
		rowMedia.Like = v.Likes
		totalLike += rowMedia.Like

		rowMedia.VideoView = int(v.ViewCount)
		totalVideoView += rowMedia.VideoView

		rowMedia.Comment = v.CommentCount
		totalComment += rowMedia.Comment

		rowMedia.Caption = v.Caption.Text
		rowMedia.Username = v.User.Username
		rowMedia.UserID = fmt.Sprint(v.User.ID)
		rowMedia.ProfilePicURL = v.User.ProfilePicURL
		// if v.ImageVersions2 != nil {
		// 	rowMedia.DisplayURL = v.ImageVersions2.Candidates[0].Url
		// }
		rowMedia.DisplayURL = v.Images.GetBest()
		// v.Mediat
		fmt.Println(ariefjson.MarshalIndent(v))
		// rowMedia.DisplayURL = v.CoverMedia.FullImageVersion.Url
		// xx:=v.Media.Image

		// mediaInfo, _ := ig.Media.GetInfo(v.Pk)
		// for _, med := range mediaInfo.Items {
		// 	rowMedia.Comment += med.CommentCount
		// 	if med.ImageVersions2 != nil {
		// 		if len(med.ImageVersions2.Candidates) > 0 {
		// 			rowMedia.DisplayURL = med.ImageVersions2.Candidates[0].Url
		// 		}
		// 	}

		switch v.MediaType {
		case 2:
			rowMedia.IsVideo = true

			if len(v.Videos) > 0 {
				rowMedia.VideoURL = v.Videos[0].URL
			}
			break
		case 8:
			rowMedia.IsCarousel = true
			rowCarousel := instagram.CarouselPost{}
			for _, carousel := range v.CarouselMedia {
				rowCarousel.ID = fmt.Sprint(carousel.Pk)
				rowCarousel.AccessibilityCaption = ""
				if len(carousel.Images.Versions) > 0 {
					rowCarousel.DisplayUrl = carousel.Images.Versions[0].URL
					rowCarousel.Dimensions.Width = carousel.Images.Versions[0].Width
					rowCarousel.Dimensions.Height = carousel.Images.Versions[0].Height
					// if len(carousel.ImageVersions2.Candidates) > 0 {
					// 	rowCarousel.DisplayUrl = carousel.ImageVersions2.Candidates[0].Url
					// 	rowCarousel.Dimensions.Height = carousel.ImageVersions2.Candidates[0].Height
					// 	rowCarousel.Dimensions.Width = carousel.ImageVersions2.Candidates[0].Width
					// }
				}
				rowCarousel.ShortCode = carousel.ID

			}
			rowMedia.CarouselPosts = append(rowMedia.CarouselPosts, rowCarousel)
		}
		// }

		// totalComment += rowMedia.Comment
		// if rowMedia.Shortcode == "Cb0MToFrrWN" {
		// 	fmt.Println(ariefjson.Marshal(mediaInfo))
		// }

		// rowMedia.IsVideo=v.MediaType

		profile.LastPost = append(profile.LastPost, rowMedia)

	}
	if totalLastPost > 0 {
		profile.AverageLike = totalLike / totalLastPost
		profile.AverageComment = totalComment / totalLastPost
		profile.AverageVideoView = totalVideoView / totalLastPost
		profile.Like = totalLike
		profile.Comment = totalComment
		profile.VideoView = totalVideoView
	}
	log.Println("total Last post", totalLastPost)

	return
}
