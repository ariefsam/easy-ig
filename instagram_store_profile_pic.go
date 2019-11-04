package main

import (
	"github.com/getsentry/sentry-go"
)

func (i *Instagram) StoreProfilePic() {
	if i.ID != "" {
		var path string
		path = "/insta/user/pic/" + i.ID
		_, err := UploadImageFromURL(i.ProfilePicUrl, path)
		if err != nil {
			sentry.CaptureException(err)
		} else {
			i.StoredProfilePicUrl = "https://abcd.sgp1.cdn.digitaloceanspaces.com" + path
		}
	}
}
