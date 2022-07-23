package main

import (
	"net/http"

	"gitlab.com/ariefhidayatulloh/easy-ig/ariefjson"
)

func JSONView(w http.ResponseWriter, r *http.Request, data interface{}, statuscode int) {
	view := ariefjson.MarshalIndent(data)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statuscode)

	w.Write([]byte(view))
}

func Log(data interface{}) {
	// log.Println(data)
}
