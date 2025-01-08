package webprofile

import (
	"crypto/tls"
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

	address := "https://www.instagram.com/api/v1/users/web_profile_info/?username=" + username
	log.Println("Getting web profile info for", username)
	log.Println(address)
	resp, err := myClient.Get(address)
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
