package apify

import (
	"encoding/json"
	"log"
	"os"
	"testing"

	"github.com/joho/godotenv"
	"github.com/stretchr/testify/assert"
)

func TestTransform(t *testing.T) {
	log.Default().SetFlags(log.LstdFlags | log.Lshortfile)

	godotenv.Load()
	input, _ := os.ReadFile("apify_resp.json")
	resp, _, err := transform(input)
	assert.NoError(t, err)
	assert.NotEmpty(t, resp)

	x, err := json.MarshalIndent(resp, "", "  ")
	assert.NoError(t, err)
	log.Println(string(x))

	os.WriteFile("apify_resp_transformed.json", x, 0644)
}
