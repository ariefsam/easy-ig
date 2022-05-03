package private_test

import (
	"log"
	"testing"

	"github.com/stretchr/testify/assert"
	"gitlab.com/ariefhidayatulloh/easy-ig/private"
)

func init() {
	log.SetFlags(log.LstdFlags | log.Lshortfile)
}
func TestGetProfile(t *testing.T) {
	profile, err := private.GetProfile("maroon5")
	assert.NoError(t, err)
	assert.NotEmpty(t, profile.ID)
	assert.NotEmpty(t, profile.FullName)
	assert.Equal(t, "196859368", profile.ID)
	assert.NotEmpty(t, profile.Biography)
	assert.NotEmpty(t, profile.Username)
	assert.False(t, profile.IsPrivate)
	assert.NotEmpty(t, profile.ExternalURL)
	assert.NotEmpty(t, profile.ProfilePicUrl)
	assert.NotEmpty(t, profile.Following)
	assert.NotEmpty(t, profile.Follower)
	assert.NotEmpty(t, profile.Like)
	assert.NotEmpty(t, profile.Comment)
	assert.NotEmpty(t, profile.VideoView)
	assert.NotEmpty(t, profile.TotalPost)

	assert.NotEmpty(t, profile.AverageLike)
	assert.NotEmpty(t, profile.AverageComment)
	assert.NotEmpty(t, profile.AverageVideoView)

	assert.NotEmpty(t, profile.LastPost)

	// js := ariefjson.Marshal(profile)
	// fmt.Println(js)

}
