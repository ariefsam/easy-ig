package main

import (
	"crypto/tls"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"

	"gitlab.com/ariefhidayatulloh/easy-ig/instagram"
)

func GetPostByShortcodeHandler(w http.ResponseWriter, r *http.Request) {

	var data instagram.InstagramPost
	shortcode := _GET(r, "shortcode")

	if len(shortcode) < 5 {
		return
	}

	urlPost := "https://www.instagram.com/p/" + shortcode + "?__a=1"

	myClient := &http.Client{}
	if config.Proxy != "" {
		proxyURL, _ := url.Parse(config.Proxy)
		myClient = &http.Client{
			Transport: &http.Transport{
				Proxy:           http.ProxyURL(proxyURL),
				TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
			},
		}

		log.Println("Using proxy: ", proxyURL)
	}

	resp, err := myClient.Get(urlPost)
	if err != nil {
		log.Println(err)
	}
	defer resp.Body.Close()
	body, _ := ioutil.ReadAll(resp.Body)
	data = instagram.ParsePost(string(body))

	JSONView(w, r, data, http.StatusOK)
}
