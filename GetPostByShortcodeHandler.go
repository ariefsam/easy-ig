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
	shortcode := _GET(r, "shortcode")

	if len(shortcode) < 5 {
		return
	}
	data, err := getDataPost(shortcode)
	if err != nil {
		log.Println(err)
		return
	}

	JSONView(w, r, data, http.StatusOK)
}

func getDataPost(shortcode string) (data instagram.InstagramPost, err error) {
	// urlPost := "https://www.instagram.com/p/" + shortcode + "?__a=1&__d=dis"
	urlPost := "https://www.instagram.com/graphql/query"

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

	// resp, err := myClient.Get(urlPost)
	resp, err := myClient.PostForm(urlPost, url.Values{
		"variables": []string{"{\"shortcode\":\"" + shortcode + "\"}"},
		"doc_id":    []string{"8845758582119845"},
	})
	if err != nil {
		log.Println(err)
	}
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		log.Println(err)
	}

	log.Println(string(body))
	data = instagram.ParsePost(string(body))
	return
}

func GetPostByShortcodeBase64Handler(w http.ResponseWriter, r *http.Request) {
	shortcode := _GET(r, "shortcode")

	if len(shortcode) < 5 {
		return
	}
	data, err := getDataPost(shortcode)
	if err != nil {
		log.Println(err)
		return
	}
	dataPost := instagram.InstagramPostWithBase64Image{}
	dataPost.InstagramPost = data
	dataPost.DisplayURLBase64Image, _ = getBase64Image(data.DisplayURL)

	JSONView(w, r, dataPost, http.StatusOK)
}
