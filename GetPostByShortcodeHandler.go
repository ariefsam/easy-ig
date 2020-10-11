package main

import (
	"encoding/json"
	"io/ioutil"
	"log"
	"net/http"
)

func GetPostByShortcodeHandler(w http.ResponseWriter, r *http.Request) {

	var data InstagramPost
	shortcode := _GET(r, "shortcode")

	if len(shortcode) < 5 {
		return
	}

	url := "https://ig.adpl.bz/update-post?shortcode=" + shortcode
	resp, err := http.Get(url)
	if err != nil {
		log.Println(err)
	}
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	json.Unmarshal(body, &data)

	JSONView(w, r, data, http.StatusOK)
}
