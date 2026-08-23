package main

import (
	"log"
	"os"
	"strconv"
	"strings"

	"github.com/joho/godotenv"
)

type Config struct {
	Port     string
	RapidApi struct {
		ProxySecret string
	}
	Proxy      string
	LocalProxy string

	// ReqLog configures request logging and its dashboard. Logging is off
	// unless REQLOG_ENABLED is true, so an existing deployment is unchanged
	// until it opts in.
	ReqLog struct {
		Enabled       bool
		DBPath        string
		RetentionDays int
		Capture       string
		MaxBodyBytes  int
		QueueSize     int

		// DashboardAddr, when set, serves the dashboard on its own
		// listener instead of the public API port. Bind it to
		// 127.0.0.1 and front it with TLS.
		DashboardAddr     string
		DashboardPrefix   string
		DashboardUser     string
		DashboardPassHash string
		SessionSecret     string
	}
}

var config Config

func LoadConfig() (data Config) {
	err := godotenv.Load()
	if err != nil {
		log.Println("Error loading .env file")
	}

	data.Port = os.Getenv("PORT")
	if data.Port == "" {
		log.Println("Empty PORT, using default: 8211")
		data.Port = "8211"
	}
	data.RapidApi.ProxySecret = os.Getenv("RAPIDAPI_PROXY_SECRET")

	data.Proxy = os.Getenv("PROXY")
	data.LocalProxy = os.Getenv("LOCAL_PROXY")

	data.ReqLog.Enabled = envBool("REQLOG_ENABLED", false)
	data.ReqLog.DBPath = envStr("REQLOG_DB", "logs/reqlog.db")
	data.ReqLog.RetentionDays = envInt("REQLOG_RETENTION_DAYS", 14)
	data.ReqLog.Capture = envStr("REQLOG_CAPTURE", "errors")
	data.ReqLog.MaxBodyBytes = envInt("REQLOG_MAX_BODY_BYTES", 32*1024)
	data.ReqLog.QueueSize = envInt("REQLOG_QUEUE_SIZE", 1024)

	data.ReqLog.DashboardAddr = os.Getenv("DASHBOARD_ADDR")
	data.ReqLog.DashboardPrefix = envStr("DASHBOARD_PREFIX", "/logs")
	data.ReqLog.DashboardUser = os.Getenv("DASHBOARD_USER")
	data.ReqLog.DashboardPassHash = os.Getenv("DASHBOARD_PASSWORD_HASH")
	data.ReqLog.SessionSecret = os.Getenv("DASHBOARD_SESSION_SECRET")

	return data
}

// String returns the config with secrets masked.
//
// It exists because main used to log the struct verbatim, which wrote the
// proxy password and the RapidAPI secret into the journal on every start.
func (c Config) String() string {
	return "Config{" +
		"Port:" + c.Port +
		" RapidApi.ProxySecret:" + mask(c.RapidApi.ProxySecret) +
		" Proxy:" + maskProxyURL(c.Proxy) +
		" LocalProxy:" + c.LocalProxy +
		" ReqLog.Enabled:" + strconv.FormatBool(c.ReqLog.Enabled) +
		" ReqLog.DBPath:" + c.ReqLog.DBPath +
		" ReqLog.RetentionDays:" + strconv.Itoa(c.ReqLog.RetentionDays) +
		" ReqLog.Capture:" + c.ReqLog.Capture +
		" DashboardAddr:" + orUnset(c.ReqLog.DashboardAddr) +
		" DashboardUser:" + c.ReqLog.DashboardUser +
		" DashboardPasswordHash:" + mask(c.ReqLog.DashboardPassHash) +
		" DashboardSessionSecret:" + mask(c.ReqLog.SessionSecret) +
		"}"
}

func orUnset(s string) string {
	if s == "" {
		return "<same port as API>"
	}
	return s
}

func mask(s string) string {
	if s == "" {
		return "<unset>"
	}
	return "<set>"
}

// maskProxyURL keeps the host and port — useful for spotting a wrong port —
// while removing the credentials.
func maskProxyURL(raw string) string {
	if raw == "" {
		return "<unset>"
	}
	at := strings.IndexByte(raw, '@')
	if at < 0 {
		return raw // no credentials embedded
	}
	scheme := ""
	if i := strings.Index(raw, "://"); i >= 0 {
		scheme = raw[:i+3]
	}
	return scheme + "***:***@" + raw[at+1:]
}

func envStr(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func envInt(key string, def int) int {
	raw := os.Getenv(key)
	if raw == "" {
		return def
	}
	v, err := strconv.Atoi(raw)
	if err != nil {
		log.Printf("%s=%q is not a number, using default %d", key, raw, def)
		return def
	}
	return v
}

func envBool(key string, def bool) bool {
	raw := os.Getenv(key)
	if raw == "" {
		return def
	}
	v, err := strconv.ParseBool(raw)
	if err != nil {
		log.Printf("%s=%q is not a boolean, using default %v", key, raw, def)
		return def
	}
	return v
}
