// +build integration

package instagram

import (
	"net/http"
	"testing"
)

func Test_GetProfile(t *testing.T) {
	type args struct {
		username string
	}
	type scenario struct {
		name string
		args args
	}
	tests := []scenario{
		scenario{
			"Test user arief_hidayatulloh",
			args{"arief_hidayatulloh"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotProfile, _, _, err := GetProfile(tt.args.username, &http.Client{})
			ok(t, err)
			if gotProfile.Username != tt.args.username {
				t.Errorf("Bad response from Instagram %v %s", gotProfile, tt.name)
			}
		})
	}
}
