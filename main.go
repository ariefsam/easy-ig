package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gorilla/mux"

	"gitlab.com/ariefhidayatulloh/easy-ig/reqlog"
)

var router *mux.Router
var start int64

func main() {
	log.Default().SetFlags(log.LstdFlags | log.Lshortfile)
	start = time.Now().Unix()
	log.SetFlags(log.LstdFlags | log.Lshortfile)
	log.SetOutput(os.Stdout)
	config = LoadConfig()
	// Config.String masks the proxy password and the RapidAPI secret. Printing
	// the struct directly wrote both into the log on every start.
	log.Printf("%s", config)

	router = mux.NewRouter()

	// The API lives on its own subrouter so the RapidAPI secret guard applies
	// only to it. Registered on the parent, that middleware would also demand
	// the header on the dashboard, which no browser sends.
	api := router.NewRoute().Subrouter()
	api.Path("/username").HandlerFunc(UsernameHandler)
	api.Path("/username-with-base64-image").HandlerFunc(UsernameWithBase64ImageHandler)
	api.Path("/get-post").HandlerFunc(GetPostByShortcodeHandler)
	api.Path("/get-post-with-base64-image").HandlerFunc(GetPostByShortcodeHandler)

	store, dash, shutdownReqLog := setupRequestLog(api)
	defer shutdownReqLog()

	// Where the dashboard is served decides whether it is exposed.
	//
	// The API port is public, so mounting the dashboard on it would leave the
	// logs reachable at http://<host>:<port>/logs over plain HTTP, bypassing
	// any TLS front end. With DASHBOARD_ADDR set (bind to 127.0.0.1) the
	// dashboard is only reachable through whatever proxies to it.
	var dashSrv *http.Server
	if dash != nil {
		if addr := config.ReqLog.DashboardAddr; addr != "" {
			dashSrv = &http.Server{
				Handler:      dash.Handler(),
				Addr:         addr,
				ReadTimeout:  30 * time.Second,
				WriteTimeout: 30 * time.Second,
			}
			go func() {
				log.Printf("reqlog: dashboard listening on %s (path %s)", addr, dash.Prefix())
				if err := dashSrv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
					log.Printf("reqlog: dashboard listener: %v", err)
				}
			}()
		} else {
			router.PathPrefix(dash.Prefix()).Handler(dash.Handler())
			log.Printf("reqlog: dashboard on the API port at %s — set DASHBOARD_ADDR "+
				"to 127.0.0.1:PORT to keep it off the public port", dash.Prefix())
		}
	}

	// Added after the request logger so the logger sits outside it, and a
	// rejected request (401, missing secret) is still recorded.
	if config.RapidApi.ProxySecret != "" {
		api.Use(rapidApiMiddleware)
	}

	srv := &http.Server{
		Handler: router,
		Addr:    ":" + config.Port,
		// Good practice: enforce timeouts for servers you create!
		WriteTimeout: 50 * time.Second,
		ReadTimeout:  50 * time.Second,
	}

	// Shut down on signal rather than dying mid-write, so the log queue
	// drains and SQLite closes cleanly instead of leaving a stale WAL.
	idle := make(chan struct{})
	go func() {
		sig := make(chan os.Signal, 1)
		signal.Notify(sig, os.Interrupt, syscall.SIGTERM)
		<-sig
		log.Println("shutting down")

		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
		defer cancel()
		if err := srv.Shutdown(ctx); err != nil {
			log.Printf("graceful shutdown: %v", err)
		}
		if dashSrv != nil {
			if err := dashSrv.Shutdown(ctx); err != nil {
				log.Printf("graceful shutdown (dashboard): %v", err)
			}
		}
		if store != nil {
			if w, d := store.WriterStats(); d > 0 {
				log.Printf("reqlog: %d written, %d dropped this run", w, d)
			}
		}
		close(idle)
	}()

	log.Printf("listening on :%s", config.Port)
	if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatal(err)
	}
	<-idle
}

// setupRequestLog wires request logging and its dashboard when enabled, and
// returns a cleanup func that drains the queue.
//
// Failures here are logged and skipped rather than fatal — with one
// exception. A misconfigured dashboard is fatal, because the alternative is
// a service that looks healthy while request logs sit behind broken or
// absent authentication.
func setupRequestLog(api *mux.Router) (*reqlog.Store, *reqlog.Dashboard, func()) {
	noop := func() {}
	if !config.ReqLog.Enabled {
		log.Println("reqlog: disabled (set REQLOG_ENABLED=true to turn on)")
		return nil, nil, noop
	}

	store, err := reqlog.Open(reqlog.Config{
		Path:          config.ReqLog.DBPath,
		RetentionDays: config.ReqLog.RetentionDays,
		Capture:       reqlog.ParseCaptureMode(config.ReqLog.Capture),
		MaxBodyBytes:  config.ReqLog.MaxBodyBytes,
		QueueSize:     config.ReqLog.QueueSize,
	})
	if err != nil {
		log.Printf("reqlog: disabled — %v", err)
		return nil, nil, noop
	}

	api.Use(store.MiddlewareFunc())

	auth, err := reqlog.NewAuth(reqlog.AuthConfig{
		User:          config.ReqLog.DashboardUser,
		PasswordHash:  config.ReqLog.DashboardPassHash,
		SessionSecret: config.ReqLog.SessionSecret,
	})
	if err != nil {
		_ = store.Close()
		log.Fatalf("reqlog dashboard: %v", err)
	}

	dash, err := reqlog.NewDashboard(store, auth, config.ReqLog.DashboardPrefix)
	if err != nil {
		_ = store.Close()
		log.Fatalf("reqlog dashboard: %v", err)
	}

	// Started only now that the dashboard is known good. RunRetention prunes
	// immediately, so starting it earlier raced Close() on the fatal path
	// above and logged "database is closed".
	ctx, cancelRetention := context.WithCancel(context.Background())
	go store.RunRetention(ctx, 24*time.Hour)
	log.Printf("reqlog: recording to %s (retention %d day(s), capture %q)",
		config.ReqLog.DBPath, store.RetentionDays(), config.ReqLog.Capture)

	return store, dash, func() {
		cancelRetention()
		if err := store.Close(); err != nil {
			log.Printf("reqlog: close: %v", err)
		}
	}
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
