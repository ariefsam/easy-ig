package instagram

import (
	"encoding/json"
	"io/ioutil"
	"testing"
)

func Test_ParseProfile(t *testing.T) {
	type args struct {
		html string
	}
	type scenario struct {
		name        string
		args        args
		wantProfile Profile
	}
	tests := []scenario{}
	raw_page, err := ioutil.ReadFile("instagram_profile_page.txt")
	if err != nil {
		panic(err)
	}
	profile_page := string(raw_page)

	raw_want, err := ioutil.ReadFile("instagram_profile_parse_result.txt")
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
			equals(t, tt.wantProfile.Username, gotProfile.Username)
		})
	}
}
