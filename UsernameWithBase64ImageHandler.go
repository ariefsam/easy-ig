package main

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"

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

	imgRes, err := http.Get(p.ProfilePicUrl)
	if err != nil {
		log.Println(err)
		return
	}
	imgByte, err := ioutil.ReadAll(imgRes.Body)
	if err != nil {
		log.Println(err)
		return
	}
	mimeType := http.DetectContentType(imgByte)

	base64Image := base64.StdEncoding.EncodeToString(imgByte)
	base64Image = fmt.Sprintf("data:%s;base64,%s", mimeType, base64Image)
	p.ProfilePicBase64Image = base64Image
	return p, errClient, errSystem
}

func UsernameWithBase64ImageHandler(w http.ResponseWriter, r *http.Request) {

	var data Instagram
	data.Username = _GET(r, "username")
	if data.Username == "" {
		JSONView(w, r, nil, http.StatusBadRequest)
		return
	}

	profile, errClient, errSystem := getIgProfileWithBase64Image(r, data.Username)
	if errClient != nil {
		JSONView(w, r, errClient, 200)
		return
	}
	if errSystem != nil {
		JSONView(w, r, map[string]string{"error": errSystem.Error()}, http.StatusBadGateway)
	}
	JSONView(w, r, profile, 200)
}
