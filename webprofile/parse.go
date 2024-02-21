package webprofile

import (
	"encoding/json"
	"log"
	"strings"

	"gitlab.com/ariefhidayatulloh/easy-ig/instagram"
)

func Parse(input []byte) (output instagram.RawUser, err error) {
	data := struct {
		Data struct {
			User instagram.RawUser `json:"user"`
		} `json:"data"`
	}{}
	err = json.Unmarshal([]byte(input), &data)
	if err != nil {
		log.Println(err)
		if strings.Contains(err.Error(), "unexpected end of JSON input") {
			log.Println(string(input))
		}
		return
	}
	output = data.Data.User
	return
}
