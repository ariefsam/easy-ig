package instagram

import (
	"encoding/json"
	"io/ioutil"
	"testing"

	a "github.com/stretchr/testify/assert"
)

func Test_ParseProfileV2(t *testing.T) {
	type args struct {
		html string
	}
	type scenario struct {
		name        string
		args        args
		wantProfile Profile
	}
	tests := []scenario{}
	raw_page, err := ioutil.ReadFile("page_v2.txt")
	if err != nil {
		panic(err)
	}
	profile_page := string(raw_page)

	raw_want, err := ioutil.ReadFile("page_parse_v2.txt")
	if err != nil {
		panic(err)
	}
	var want Profile
	json.Unmarshal(raw_want, &want)
	tests = append(tests, scenario{
		"Test Parsing",
		args{profile_page},
		want,
	})
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotProfile := ParseProfile(tt.args.html)
			//x, _ := json.Marshal(gotProfile)
			//a.JSONEq(t, string(raw_want), string(x))
			a.NotEmpty(t, gotProfile.IGTV)
			if len(gotProfile.IGTV) > 0 {
				a.Equal(t, 210, gotProfile.IGTV[0].Comment)
				caption := "Is #tbt still a thing?? A look behind the scenes at the making of \u201cBeautiful Mistakes\u201d!"
				a.Equal(t, caption, gotProfile.IGTV[0].Caption)
			}
		})
	}
}
