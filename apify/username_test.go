package apify_test

import (
	"encoding/json"
	"log"
	"testing"
	"time"

	"github.com/alitto/pond"
	"github.com/joho/godotenv"
	"github.com/stretchr/testify/assert"
	"gitlab.com/ariefhidayatulloh/easy-ig/apify"
)

func TestUsername(t *testing.T) {

	log.Default().SetFlags(log.LstdFlags | log.Lshortfile)

	godotenv.Load()
	pool := pond.New(5, 1000)
	pool.Submit(func() {
		username := "maroon5"
		profile, err := apify.Username(username)
		assert.NoError(t, err)
		assert.Equal(t, username, profile.Username)
		temp, _ := json.MarshalIndent(profile, "", "  ")
		log.Println(string(temp))
	})

	pool.Submit(func() {
		username := "agungkdev"
		profile, err := apify.Username(username)
		assert.NoError(t, err)
		assert.Equal(t, username, profile.Username)
		temp, _ := json.MarshalIndent(profile, "", "  ")
		log.Println(string(temp))
	})

	pool.Submit(func() {
		username := "khannedy"
		profile, err := apify.Username(username)
		assert.NoError(t, err)
		assert.Equal(t, username, profile.Username)
		temp, _ := json.MarshalIndent(profile, "", "  ")
		log.Println(string(temp))
	})

	pool.Submit(func() {
		username := "hfgoldpuzzle"
		profile, err := apify.Username(username)
		assert.NoError(t, err)
		assert.Equal(t, username, profile.Username)
		temp, _ := json.MarshalIndent(profile, "", "  ")
		log.Println(string(temp))
	})

	pool.Submit(func() {
		username := "thenotfounduser"
		profile, err := apify.Username(username)
		assert.NoError(t, err)
		assert.Equal(t, username, profile.Username)
		temp, _ := json.MarshalIndent(profile, "", "  ")
		log.Println(string(temp))
	})

	pool.StopAndWait()
	time.Sleep(2 * time.Second)
}
