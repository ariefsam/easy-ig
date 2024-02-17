package main

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"

	"github.com/patrickmn/go-cache"
	"gitlab.com/ariefhidayatulloh/easy-ig/instagram"
)

func getIgProfileWithBase64Image(r *http.Request, username string) (p instagram.ProfileWithBase64Image, clientError map[string]string, systemError error) {

	profile, errClient, errSystem := getIgProfile(r, username)
	if errClient != nil {
		return
	}

	if errSystem != nil {
		return
	}

	temp, _ := json.Marshal(profile)
	json.Unmarshal(temp, &p)

	p.ProfilePicBase64Image, _ = getBase64Image(p.ProfilePicUrl)

	for k, v := range p.LastPost {
		p.LastPost[k].DisplayURLBase64Image, _ = getBase64Image(v.DisplayURL)
	}

	return p, errClient, errSystem
}

func GetProfileBase64(input instagram.Profile) (p instagram.ProfileWithBase64Image) {
	temp, _ := json.Marshal(input)
	json.Unmarshal(temp, &p)
	p.ProfilePicBase64Image, _ = getBase64Image(p.ProfilePicUrl)

	for k, v := range p.LastPost {
		p.LastPost[k].DisplayURLBase64Image, _ = getBase64Image(v.DisplayURL)
	}
	return
}

func getBase64Image(url string) (base64Image string, err error) {
	imgRes, err := http.Get(url)
	imgByte, err := ioutil.ReadAll(imgRes.Body)
	if err != nil {
		log.Println(err)
		return
	}
	mimeType := http.DetectContentType(imgByte)

	base64Image = base64.StdEncoding.EncodeToString(imgByte)
	base64Image = fmt.Sprintf("data:%s;base64,%s", mimeType, base64Image)
	return
}

func UsernameWithBase64ImageHandler(w http.ResponseWriter, r *http.Request) {

	var data Instagram
	data.Username = _GET(r, "username")
	if data.Username == "" {
		JSONView(w, r, nil, http.StatusBadRequest)
		return
	}

	// profile, errClient, errSystem := getIgProfileWithBase64Image(r, data.Username)
	// if errClient != nil {
	// 	JSONView(w, r, errClient, 200)
	// 	return
	// }
	// if errSystem != nil {
	// 	JSONView(w, r, map[string]string{"error": errSystem.Error()}, http.StatusNotFound)
	// 	return
	// }

	// profile, err := apify.Username(data.Username)
	// if err != nil {
	// 	log.Println(err)
	// 	JSONView(w, r, map[string]string{"error": "system error"}, http.StatusInternalServerError)
	// }

	// if profile.IsExist == "no" {
	// 	JSONView(w, r, map[string]string{"client_error": "Username not exist or deleted. Your RapidAPI quota still reduced.", "is_exist": "no"}, 200)
	// 	return
	// }

	profileWithBase64Image, statusCode, isRestricted, err := GetUsernameWithBase64(data.Username)
	if err != nil {
		log.Println(err)
		log.Println("system error")
		JSONView(w, r, map[string]string{"error": "system error"}, http.StatusInternalServerError)
		return
	}

	if isRestricted {
		log.Println("restricted profile")
		JSONView(w, r, map[string]string{"client_error": "Profile restricted for 18+, Our API is public app, so we cannot read restricted profile without login. Your RapidAPI quota still reduced."}, 200)
		return
	}

	if statusCode == 404 {
		JSONView(w, r, map[string]string{"client_error": "Username not exist or deleted. Your RapidAPI quota still reduced.", "is_exist": "no"}, 200)
		return
	}

	JSONView(w, r, profileWithBase64Image, 200)
}

type WebProfileBase64Response struct {
	ProfileWithBase64Image instagram.ProfileWithBase64Image
	StatusCode             int
	IsRestricted           bool
	Err                    error
}

func GetUsernameWithBase64(username string) (profileWithBase64Image instagram.ProfileWithBase64Image, statusCode int, isRestricted bool, err error) {

	if v, ok := c.Get("usernamebase64:" + username); ok {

		resp, _ := v.(WebProfileBase64Response)
		log.Println("GetUsernameWithBase64 from cache:", username, ". Follower:", resp.ProfileWithBase64Image.Follower)
		return resp.ProfileWithBase64Image, resp.StatusCode, resp.IsRestricted, resp.Err
	}

	resp := WebProfileBase64Response{}

	defer func() {
		resp.ProfileWithBase64Image = profileWithBase64Image
		resp.StatusCode = statusCode
		resp.IsRestricted = isRestricted
		resp.Err = err
		c.Set("usernamebase64:"+username, resp, cache.DefaultExpiration)
	}()

	profile, statusCode, isRestricted, err := GetWebProfile(username)

	if err != nil {
		log.Println(err)
		return
	}

	resp.StatusCode = statusCode
	resp.IsRestricted = isRestricted
	resp.Err = err

	if isRestricted {
		log.Println("restricted profile")
		return
	}

	if statusCode == 404 {
		return
	}

	profileWithBase64Image = GetProfileBase64(profile)
	resp.ProfileWithBase64Image = profileWithBase64Image

	return
}
