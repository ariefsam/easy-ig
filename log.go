package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
)

func Log(data interface{}) {
	log.Println(data)
	go func() {

		i, ok := data.(error)
		if ok {
			d := struct {
				Message string
				Type    string
			}{
				Message: i.Error(),
				Type:    "error",
			}
			data = d
		}
		requestBody, _ := json.Marshal(data)
		resp, err := http.Post("http://logs-01.loggly.com/inputs/393493cf-9263-4052-8d19-f31784c10883/tag/http/", "application/json", bytes.NewBuffer(requestBody))
		if err != nil {
			fmt.Println(err)
			return
		}
		body, _ := ioutil.ReadAll(resp.Body)
		log.Println(string(body))
	}()
}
