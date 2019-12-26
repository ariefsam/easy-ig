package main

import "log"

func (i *Instagram) Save() {

}

func (i *Instagram) SavePost() {

}

func (i *Instagram) CheckVision() {
	posts := i.LastPost
	for k, v := range posts {
		log.Println("Checking posting ", v.Shortcode)
		if !v.CheckStoredDisplayURL() {
			log.Println("Storing image")
			v.StoreDisplayURL()
			i.LastPost[k].StoredDisplayURL = v.GetStorageURL()
		} else {
			log.Println("Image already in storage.")
			i.LastPost[k].StoredDisplayURL = v.GetStorageURL()
			v.StoredDisplayURL = v.GetStorageURL()
		}
		var visions []string
		visions, err := Vision(v.StoredDisplayURL)
		if err != nil {

		}
		log.Println(k, "posting ", v.ID, " ", visions)
	}
}
