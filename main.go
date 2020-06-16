package main

import (
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"time"

	"github.com/gorilla/mux"
)

var router *mux.Router

func main() {
	myClient := &http.Client{}
	proxyUrl, _ := url.Parse("http://lum-customer-hl_52c756b8-zone-zone1:cjajlzx3q8wk@zproxy.luminati.io:22225")
	myClient = &http.Client{Transport: &http.Transport{Proxy: http.ProxyURL(proxyUrl)}}

	x, err := myClient.Get("https://api.myip.com/")
	if err != nil {
		log.Println(err)
	} else {
		body, _ := ioutil.ReadAll(x.Body)
		log.Println(string(body))
	}
	router = mux.NewRouter()

	router.Path("/username").HandlerFunc(UsernameHandler)
	router.Path("/post").HandlerFunc(PostHandler)
	router.Path("/get-post").HandlerFunc(UpdatePostHandler)
	router.Use(rapidApiMiddleware)

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
