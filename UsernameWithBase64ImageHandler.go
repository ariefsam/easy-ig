package main

import (
	"crypto/tls"
	"log"
	"net/http"
	"net/url"
	"os"

	"gitlab.com/ariefhidayatulloh/easy-ig/instagram"
)

func UsernameWithBase64ImageHandler(w http.ResponseWriter, r *http.Request) {

	var data Instagram
	data.Username = _GET(r, "username")
	if data.Username == "" {
		JSONView(w, r, nil, http.StatusBadRequest)
		return
	}
	if data.Username != "" {
		if data.Username == "explore" {
			JSONView(w, r, map[string]string{"client_error": "Username not exist or deleted. Your RapidAPI quota still reduced."}, 200)
			return
		}
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

		var profile instagram.Profile
		var try, maxTry, statusCode int
		var isRestricted bool
		var err error

		if config.LocalProxy != "" && _GET(r, "no_proxy") != "1" {
			localClient := &http.Client{}
			log.Println("using local client ", config.LocalProxy)
			profile, statusCode, isRestricted, err = instagram.GetProfileByLocalProxy(config.LocalProxy, data.Username, localClient)
			if profile.IsExist == "no" {
				JSONView(w, r, map[string]string{"client_error": "Username not exist or deleted. Your RapidAPI quota still reduced.", "is_exist": "no"}, 200)
				return
			}
		}

		if config.LocalProxy != "" && _GET(r, "no_proxy") == "1" {
			log.Println("local proxy set, but no proxy choose")
		}

		maxTry = 15
		if config.Proxy == "" {
			maxTry = 2
		}
		for profile.Username == "" && statusCode != 404 && !isRestricted {

			log.Println("Trying using proxy ", try)

			if try > maxTry {
				break
			}

			if os.Getenv("SCRAPERAPI") != "" {
				profile, statusCode, isRestricted, err = instagram.GetProfileByScraperAPI(data.Username)
			} else {
				profile, statusCode, isRestricted, err = instagram.GetProfile(data.Username, myClient)
			}
			if err != nil {
				log.Println(err)
			}
			try++

		}

		if statusCode == 404 {
			JSONView(w, r, map[string]string{"client_error": "Username not exist or deleted. Your RapidAPI quota still reduced.", "is_exist": "no"}, 200)
			return
		}

		if isRestricted {
			JSONView(w, r, map[string]string{"client_error": "Profile restricted for 18+, Our API is public app, so we cannot read restricted profile without login. Your RapidAPI quota still reduced."}, 200)
			return
		}

		if profile.Username == "" {
			JSONView(w, r, map[string]string{"error": "We were sorry, our request blocked by Instagram. Your RapidAPI quota or overage will not be reduced. Please try again, we will try another IP Address."}, http.StatusBadGateway)
			return
		}
		log.Println(data.Username, profile.Follower)
		JSONView(w, r, profile, 200)
		return
	}
	JSONView(w, r, "", 200)
}
