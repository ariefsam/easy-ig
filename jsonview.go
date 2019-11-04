package main

import (
	"encoding/json"
	"net/http"
)

func JSONView(w http.ResponseWriter, r *http.Request, data interface{}, statuscode int) {
	//log.Println(data)
	view, _ := json.Marshal(data)
	w.Header().Set("Content-Type", "application/json")
	w.Write(view)
}
