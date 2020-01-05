package main

import (
	"encoding/json"
	"log"
)

func (i *Instagram) Save() {

}

func (i *Instagram) SavePost() {
	posts := i.LastPost
	for _, v := range posts {
		json, err := json.Marshal(v)
		if err != nil {
			continue
		}
		path := v.GetStoragePath()
		log.Println("Storing ", path)
		UploadByte([]byte(json), path)
	}
}

func (i *Instagram) CheckVision() {
	posts := i.LastPost
	for k, v := range posts {
		log.Println("Checking posting ", v.Shortcode)
		if !v.CheckStoredDisplayURL() {
			log.Println("Storing image")
			v.StoreDisplayURL()
			i.LastPost[k].StoredDisplayURL = v.GetImageStorageURL()
		} else {
			log.Println("Image already in storage.")
			i.LastPost[k].StoredDisplayURL = v.GetImageStorageURL()
			v.StoredDisplayURL = v.GetImageStorageURL()
		}
		if !v.CheckStoredVision() {
			log.Println("Checking vision")
			var visions []string
			visions, err := Vision(v.StoredDisplayURL)
			if err == nil {
				log.Println(k, "posting ", v.ID, " ", visions)
			}
			i.LastPost[k].AICategory = visions
		} else {
			i.LastPost[k].AICategory = v.AICategory
		}

	}
}
