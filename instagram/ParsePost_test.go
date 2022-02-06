package instagram

import (
	"encoding/json"
	"io/ioutil"
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_ParsePost(t *testing.T) {
	type args struct {
		html string
	}
	type scenario struct {
		name     string
		args     args
		wantPost InstagramPost
	}
	tests := []scenario{}
	raw_page, err := ioutil.ReadFile("post_carousel_page.txt")
	if err != nil {
		panic(err)
	}
	page := string(raw_page)

	raw_want, err := ioutil.ReadFile("post_carousel_result.txt")
	if err != nil {
		panic(err)
	}
	var wantPost InstagramPost
	json.Unmarshal(raw_want, &wantPost)
	tests = append(tests, scenario{
		"Test Parsing",
		args{page},
		wantPost,
	})
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotProfile := ParsePost(tt.args.html)
			// equals(t, tt.wantPost.Username, gotProfile.Username)a
			// assert.Equal(t, tt.wantPost.ID, gotProfile.ID)
			assert.Equal(t, tt.wantPost, gotProfile)
			// log.Println(gotProfile)
		})
	}
}
