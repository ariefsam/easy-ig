package main

import (
	"log"
	"net/http"
	"os"
	"time"

	"github.com/gorilla/mux"
)

var router *mux.Router
var start int64

func main() {
	start = time.Now().Unix()
	log.SetFlags(log.LstdFlags | log.Lshortfile)
	log.SetOutput(os.Stdout)
	config = LoadConfig()
	log.Printf("%+v", config)
	router = mux.NewRouter()

	router.Path("/username").HandlerFunc(UsernameHandler)
	router.Path("/username-with-base64-image").HandlerFunc(UsernameWithBase64ImageHandler)
	router.Path("/get-post").HandlerFunc(GetPostByShortcodeHandler)
	router.Path("/get-post-with-base64-image").HandlerFunc(GetPostByShortcodeHandler)
	if config.RapidApi.ProxySecret != "" {
		router.Use(rapidApiMiddleware)
	}

	//loggedRouter := handlers.LoggingHandler(os.Stdout, r)
	srv := &http.Server{
		Handler: router,
		Addr:    ":" + config.Port,
		// Good practice: enforce timeouts for servers you create!
		WriteTimeout: 50 * time.Second,
		ReadTimeout:  50 * time.Second,
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
