package webprofile

import (
	"crypto/tls"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"strings"

	"gitlab.com/ariefhidayatulloh/easy-ig/instagram"
)

func GetWebProfile(username string) (profile instagram.Profile, statusCode int, isRestricted bool, err error) {
	if strings.Contains(username, "@") {
		statusCode = 404
		return
	}

	if strings.Contains(username, "/") {
		statusCode = 404
		return
	}
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

	address := "https://www.instagram.com/api/v1/users/web_profile_info/?username=" + username
	resp, err := myClient.Get(address)
	if err != nil {
		if strings.Contains(err.Error(), "server gave HTTP response to HTTPS client") {
			err = nil
			statusCode = 404
		}
		log.Println(err)
		return
	}

	statusCode = resp.StatusCode
	if statusCode == 404 {
		return
	}

	respBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Println(err)
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

	return
}
