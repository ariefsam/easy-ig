package main

import (
	"log"
	"net/http"
	"time"

	"github.com/gorilla/mux"
)

var router *mux.Router

func main() {
	//os.Setenv("GOOGLE_APPLICATION_CREDENTIALS", "Easy Insta-29794461bb65.json")
	detectLandmarks()
	router = mux.NewRouter()

	router.Path("/username").HandlerFunc(UsernameHandler)
	router.Path("/post").HandlerFunc(PostHandler)
	router.Path("/get-post").HandlerFunc(GetPostHandler)
	router.Use(rapidApiMiddleware)

	//loggedRouter := handlers.LoggingHandler(os.Stdout, r)
	srv := &http.Server{
		Handler: router,
		Addr:    ":" + config.Port,
		// Good practice: enforce timeouts for servers you create!
		WriteTimeout: 15 * time.Second,
		ReadTimeout:  15 * time.Second,
	}

	log.Fatal(srv.ListenAndServe())

}

func rapidApiMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		secret_key := r.Header.Get("X-RapidAPI-Proxy-Secret")
		if secret_key != config.RapidApi.ProxySecret {
			var d struct {
				Errors []string
			}
			d.Errors = append(d.Errors, "Unauth")
			JSONView(w, r, d, http.StatusUnauthorized)
			return
		} else {
			next.ServeHTTP(w, r)
		}
	})
}

func _GET(r *http.Request, key string) (val string) {

	keys, ok := r.URL.Query()[key]

	if !ok || len(keys[0]) < 1 {
		return
	}

	val = keys[0]
	return
}
