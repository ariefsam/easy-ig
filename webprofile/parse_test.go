package webprofile

import (
	"encoding/json"
	"log"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestParse(t *testing.T) {
	resp, err := os.ReadFile("./resp.json")
	assert.NoError(t, err)
	output, err := Parse(resp)
	assert.NoError(t, err)
	assert.Equal(t, "196859368", output.Id)
	temp, _ := json.MarshalIndent(output, "", "  ")
	log.Println(string(temp))
}
