package main

import (
	"log"
	"net/http"
	"time"

	"github.com/gorilla/mux"
)

var router *mux.Router

func main() {
	router = mux.NewRouter()

	router.Path("/username").HandlerFunc(UsernameHandler)

	//loggedRouter := handlers.LoggingHandler(os.Stdout, r)
	srv := &http.Server{
		Handler: router,
		Addr:    "127.0.0.1:8101",
		// Good practice: enforce timeouts for servers you create!
		WriteTimeout: 15 * time.Second,
		ReadTimeout:  15 * time.Second,
	}

	log.Fatal(srv.ListenAndServe())
}

func _GET(r *http.Request, key string) (val string) {

	keys, ok := r.URL.Query()[key]

	if !ok || len(keys[0]) < 1 {
		return
	}

	val = keys[0]
	return
}
