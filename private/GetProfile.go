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

	module := "newsfeed"
	userInfo, err := ig.People.GetInfoByName("maroon5", &module)
	if err != nil {
		if err.Error() == "UserInfoResponse: User Not Found" {
			profile.IsExist = "no"
		}
		return
	}
	profile.FullName = userInfo.User.FullName
	profile.ID = fmt.Sprint(userInfo.User.Pk)
	profile.Biography = userInfo.User.Biography
	profile.Username = userInfo.User.Username
	profile.IsPrivate = userInfo.User.IsPrivate
	profile.ExternalURL = userInfo.User.ExternalUrl
	profile.ProfilePicUrl = userInfo.User.ProfilePicUrl
	profile.Following = userInfo.User.FollowingCount
	profile.Follower = userInfo.User.FollowerCount
	profile.TotalPost = userInfo.User.MediaCount
	// louserInfo.User.LatestReelMedia

	feed, err := ig.Timeline.GetUserFeed(userInfo.User.Pk, nil)
	if err != nil {
		log.Println(err)
		return
	}
	if feed.Items == nil {
		return
	}

	var totalLastPost, totalLike, totalComment, totalVideoView int

	for _, v := range *feed.Items {
		if totalLastPost == 12 {
			continue
		}
		totalLastPost++
		rowMedia := instagram.InstagramPost{}
		rowMedia.ID = fmt.Sprint(v.Id)
		rowMedia.Shortcode = v.Code
		rowMedia.Like = v.LikeCount
		totalLike += rowMedia.Like

		rowMedia.VideoView = int(v.ViewCount)
		totalVideoView += rowMedia.VideoView

		rowMedia.Caption = v.Caption.Text
		rowMedia.Username = v.User.Username
		rowMedia.UserID = fmt.Sprint(v.User.Pk)
		rowMedia.ProfilePicURL = v.User.ProfilePicUrl

		mediaInfo, _ := ig.Media.GetInfo(v.Pk)
		for _, med := range mediaInfo.Items {
			rowMedia.Comment += med.CommentCount
			if med.ImageVersions2 != nil {
				if len(med.ImageVersions2.Candidates) > 0 {
					rowMedia.DisplayURL = med.ImageVersions2.Candidates[0].Url
				}
			}

			switch med.MediaType {
			case 2:
				rowMedia.IsVideo = true
				if med.VideoVersions != nil {
					versions := *med.VideoVersions
					rowMedia.VideoURL = versions[0].Url
				}
				break
			case 8:
				rowMedia.IsCarousel = true
				rowCarousel := instagram.CarouselPost{}
				for _, carousel := range med.CarouselMedia {
					rowCarousel.ID = fmt.Sprint(carousel.Pk)
					rowCarousel.AccessibilityCaption = ""
					if carousel.ImageVersions2 != nil {
						if len(carousel.ImageVersions2.Candidates) > 0 {
							rowCarousel.DisplayUrl = carousel.ImageVersions2.Candidates[0].Url
							rowCarousel.Dimensions.Height = carousel.ImageVersions2.Candidates[0].Height
							rowCarousel.Dimensions.Width = carousel.ImageVersions2.Candidates[0].Width
						}
					}
					rowCarousel.ShortCode = carousel.Id

				}
				rowMedia.CarouselPosts = append(rowMedia.CarouselPosts, rowCarousel)
			}
		}

		totalComment += rowMedia.Comment
		if rowMedia.Shortcode == "Cb0MToFrrWN" {
			fmt.Println(ariefjson.Marshal(mediaInfo))
		}

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
