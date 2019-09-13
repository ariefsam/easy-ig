package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
)

func JSONView(w http.ResponseWriter, r *http.Request, data interface{}) {
	log.Println(data)
	view, _ := json.Marshal(data)
	fmt.Fprintf(w, "%s", string(view))
}
