package main

import (
	"crypto/tls"
	"errors"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"

	"gitlab.com/ariefhidayatulloh/easy-ig/instagram"
)

const igUserAgent = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36"

// postDocID is Instagram's PolarisPostRootQuery persisted GraphQL query,
// which replaced the deprecated xdt_shortcode_media doc_id in June 2026.
const postDocID = "27128499623469141"

var csrfTokenRegexp = regexp.MustCompile(`"csrf_token":"([^"]+)"`)

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

func newIgHTTPClient() *http.Client {
	myClient := &http.Client{}
	if config.Proxy != "" {
		proxyURL, _ := url.Parse(config.Proxy)
		myClient = &http.Client{
			Transport: &http.Transport{
				Proxy:                 http.ProxyURL(proxyURL),
				TLSClientConfig:       &tls.Config{InsecureSkipVerify: true},
				ResponseHeaderTimeout: time.Second * 100,
			},
			Timeout: time.Second * 100,
		}

		log.Println("Using proxy: ", proxyURL)
	}
	return myClient
}

// getCSRFToken fetches the Instagram homepage to obtain a csrf_token, which
// the PolarisPostRootQuery endpoint requires as a header (a cookie alone is
// rejected with 403).
func getCSRFToken(client *http.Client) (csrf string, err error) {
	req, err := http.NewRequest(http.MethodGet, "https://www.instagram.com/", nil)
	if err != nil {
		return
	}
	req.Header.Set("User-Agent", igUserAgent)

	resp, err := client.Do(req)
	if err != nil {
		return
	}
	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return
	}

	match := csrfTokenRegexp.FindSubmatch(body)
	if match == nil {
		err = errors.New("csrf token not found in instagram homepage")
		return
	}
	csrf = string(match[1])
	return
}

func getDataPost(shortcode string) (data instagram.InstagramPost, err error) {
	myClient := newIgHTTPClient()

	csrf, err := getCSRFToken(myClient)
	if err != nil {
		log.Println(err)
		return
	}

	form := url.Values{
		"variables": []string{`{"shortcode":"` + shortcode + `","__relay_internal__pv__PolarisAIGMMediaWebLabelEnabledrelayprovider":false}`},
		"doc_id":    []string{postDocID},
	}

	req, err := http.NewRequest(http.MethodPost, "https://www.instagram.com/graphql/query", strings.NewReader(form.Encode()))
	if err != nil {
		log.Println(err)
		return
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("User-Agent", igUserAgent)
	req.Header.Set("X-IG-App-ID", "936619743392459")
	req.Header.Set("X-CSRFToken", csrf)
	req.Header.Set("Cookie", "csrftoken="+csrf)

	resp, err := myClient.Do(req)
	if err != nil {
		log.Println(err)
		return
	}
	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		log.Println(err)
		return
	}

	log.Println(string(body))
	data = instagram.ParsePostV1(string(body), shortcode)
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
