package webprofile

import (
	"crypto/tls"
	"errors"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"gitlab.com/ariefhidayatulloh/easy-ig/instagram"
)

func GetWebProfile(username string) (profile instagram.Profile, statusCode int, isRestricted bool, err error) {
	proxy := os.Getenv("PROXY")
	myClient := &http.Client{}
	if proxy != "" {
		proxyURL, _ := url.Parse(proxy)
		myClient = &http.Client{
			Transport: &http.Transport{
				Proxy:           http.ProxyURL(proxyURL),
				TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
			},
		}

		log.Println("Using proxy: ", proxyURL)
	}

	myClient.Timeout = 60 * time.Second

	// i.instagram.com (mobile subdomain) avoids the aggressive rate-throttling
	// that www.instagram.com now applies to this REST endpoint.
	address := "https://i.instagram.com/api/v1/users/web_profile_info/?username=" + username
	log.Println("Getting web profile info for", username)
	log.Println(address)

	req, err := http.NewRequest("GET", address, nil)
	if err != nil {
		log.Println(err)
		return
	}
	req.Header.Set("x-ig-app-id", "936619743392459")
	req.Header.Set("User-Agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36")
	req.Header.Set("Accept", "*/*")
	req.Header.Set("Accept-Language", "en-US,en;q=0.9")
	req.Header.Set("Referer", "https://www.instagram.com/")
	req.Header.Set("Origin", "https://www.instagram.com")
	req.Header.Set("Sec-Fetch-Site", "same-origin")
	req.Header.Set("Sec-Fetch-Mode", "cors")
	req.Header.Set("Sec-Fetch-Dest", "empty")
	req.Header.Set("X-Requested-With", "XMLHttpRequest")

	resp, err := myClient.Do(req)
	if err != nil {
		if strings.Contains(err.Error(), "server gave HTTP response to HTTPS client") {
			err = nil
			statusCode = 404
			return
		}
		log.Println(err)
		return
	}

	statusCode = resp.StatusCode
	if statusCode == 404 {
		log.Println(username, "status code 404")
		return
	}

	if statusCode == 400 {
		log.Println(username, "status code 400")
		err = errors.New("bad server response")
		return
	}

	respBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Println(err)
		return
	}

	log.Println(address, "status code", statusCode, ". response", string(respBytes))

	if statusCode == 502 {
		log.Println("502 Bad Gateway, return 404")
		statusCode = 404
		return
	}

	rawUser, err := Parse(respBytes)
	if err != nil {
		log.Println(err)
		log.Println(string(respBytes))
		return
	}

	IgProfile := instagram.ParseRawUser(rawUser)

	profile = IgProfile

	if profile.Username == "" {
		isRestricted = true
	}

	return
}
