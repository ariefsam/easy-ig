package reqlog

import (
	"embed"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"
)

//go:embed templates/*.html
var templateFS embed.FS

// Dashboard serves the request-log UI.
type Dashboard struct {
	store  *Store
	auth   *Auth
	prefix string
	tmpl   *template.Template
}

// NewDashboard builds the UI. prefix is the mount path, e.g. "/logs".
func NewDashboard(store *Store, auth *Auth, prefix string) (*Dashboard, error) {
	prefix = "/" + strings.Trim(prefix, "/")

	loc := store.Location()
	zoneName, _ := time.Now().In(loc).Zone()

	funcs := template.FuncMap{
		// Explicit zone rather than t.Local(): the server runs in UTC, so
		// Local() rendered UTC and every timestamp read seven hours early.
		"fmtTime":  func(t time.Time) string { return t.In(loc).Format("2006-01-02 15:04:05") },
		"zone":     func() string { return zoneName },
		"fmtBytes": humanBytes,
		"fmtMS":    func(d time.Duration) string { return fmt.Sprintf("%d ms", d.Milliseconds()) },
		"pct":      func(f float64) string { return fmt.Sprintf("%.1f%%", f) },
		"round1":   func(f float64) string { return fmt.Sprintf("%.1f", f) },
		"statusClass": func(code int) string {
			switch {
			case code >= 500:
				return "s5"
			case code >= 400:
				return "s4"
			case code >= 300:
				return "s3"
			default:
				return "s2"
			}
		},
		// barWidth scales a count to a percentage of the largest value, so
		// the CSS bars need no inline arithmetic.
		"barWidth": func(n, max int64) string {
			if max <= 0 {
				return "0"
			}
			return fmt.Sprintf("%.1f", float64(n)/float64(max)*100)
		},
		"prefix": func() string { return prefix },
		"add":    func(a, b int) int { return a + b },
		// dict builds a map inline so a page can pass several values into
		// the shared header template.
		"dict": func(kv ...any) (map[string]any, error) {
			if len(kv)%2 != 0 {
				return nil, fmt.Errorf("dict needs an even number of arguments, got %d", len(kv))
			}
			m := make(map[string]any, len(kv)/2)
			for i := 0; i < len(kv); i += 2 {
				k, ok := kv[i].(string)
				if !ok {
					return nil, fmt.Errorf("dict key %d is not a string", i)
				}
				m[k] = kv[i+1]
			}
			return m, nil
		},
		// qv reads one query parameter. Templates must not use index on
		// url.Values directly: a missing key is an empty slice and index 0
		// on it aborts rendering.
		"qv": func(v url.Values, key string) string { return v.Get(key) },
	}

	tmpl, err := template.New("").Funcs(funcs).ParseFS(templateFS, "templates/*.html")
	if err != nil {
		return nil, fmt.Errorf("reqlog: parse templates: %w", err)
	}
	return &Dashboard{store: store, auth: auth, prefix: prefix, tmpl: tmpl}, nil
}

// Handler returns the dashboard as a single http.Handler owning its own
// routing, so it can be mounted under any router (gorilla/mux, ServeMux, …)
// without the caller restating the route table.
//
// Mount it away from any router carrying the RapidAPI secret middleware, or
// the UI would demand an API header no browser sends.
func (d *Dashboard) Handler() http.Handler {
	m := http.NewServeMux()
	d.Register(m)
	return m
}

// Prefix is the path the dashboard is mounted at.
func (d *Dashboard) Prefix() string { return d.prefix }

// Register mounts the dashboard routes on a ServeMux.
func (d *Dashboard) Register(mux *http.ServeMux) {
	p := d.prefix
	mux.HandleFunc(p+"/login", d.handleLogin)
	mux.HandleFunc(p+"/logout", d.handleLogout)
	mux.Handle(p+"/stats", d.auth.Require(p+"/login", http.HandlerFunc(d.handleStats)))
	mux.Handle(p+"/settings", d.auth.Require(p+"/login", http.HandlerFunc(d.handleSettings)))
	mux.Handle(p+"/entry", d.auth.Require(p+"/login", http.HandlerFunc(d.handleEntry)))
	mux.Handle(p+"/", d.auth.Require(p+"/login", http.HandlerFunc(d.handleList)))
	mux.Handle(p, http.RedirectHandler(p+"/", http.StatusMovedPermanently))
}

func (d *Dashboard) render(w http.ResponseWriter, name string, data any) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	// The dashboard shows logged request data; never let it be cached or indexed.
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("X-Robots-Tag", "noindex, nofollow")
	if err := d.tmpl.ExecuteTemplate(w, name, data); err != nil {
		log.Printf("reqlog: render %s: %v", name, err)
	}
}

// ---- login --------------------------------------------------------------

func (d *Dashboard) handleLogin(w http.ResponseWriter, r *http.Request) {
	next := urlUnescape(r.URL.Query().Get("next"))
	if next == "" || !strings.HasPrefix(next, d.prefix) {
		next = d.prefix + "/"
	}

	if r.Method == http.MethodGet {
		if d.auth.LoggedIn(r) {
			http.Redirect(w, r, next, http.StatusSeeOther)
			return
		}
		d.render(w, "login.html", map[string]any{"Prefix": d.prefix, "Next": r.URL.Query().Get("next")})
		return
	}

	ip := clientIP(r)
	if locked, wait := d.auth.Throttled(ip); locked {
		w.WriteHeader(http.StatusTooManyRequests)
		d.render(w, "login.html", map[string]any{
			"Prefix": d.prefix,
			"Error":  fmt.Sprintf("Too many attempts. Try again in %d minute(s).", int(wait.Minutes())+1),
		})
		return
	}

	if err := r.ParseForm(); err != nil {
		http.Error(w, "bad form", http.StatusBadRequest)
		return
	}
	user, pass := r.FormValue("username"), r.FormValue("password")

	if !d.auth.Check(user, pass) {
		d.auth.NoteFailure(ip)
		w.WriteHeader(http.StatusUnauthorized)
		// One message for both cases — saying which was wrong would confirm
		// whether a username exists.
		d.render(w, "login.html", map[string]any{
			"Prefix": d.prefix,
			"Error":  "Incorrect username or password.",
			"Next":   r.URL.Query().Get("next"),
		})
		return
	}

	d.auth.NoteSuccess(ip)
	d.auth.SetSession(w, r, user)
	http.Redirect(w, r, next, http.StatusSeeOther)
}

func (d *Dashboard) handleLogout(w http.ResponseWriter, r *http.Request) {
	d.auth.ClearSession(w, r)
	http.Redirect(w, r, d.prefix+"/login", http.StatusSeeOther)
}

// ---- listing ------------------------------------------------------------

func (d *Dashboard) handleList(w http.ResponseWriter, r *http.Request) {
	f, page, perPage := filterFromQuery(r)

	entries, err := d.store.List(f)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	total, err := d.store.Count(f)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	paths, _ := d.store.DistinctPaths()
	written, dropped := d.store.WriterStats()

	totalPages := int((total + int64(perPage) - 1) / int64(perPage))
	d.render(w, "list.html", map[string]any{
		"Prefix":     d.prefix,
		"Entries":    entries,
		"Total":      total,
		"Page":       page,
		"TotalPages": totalPages,
		"HasPrev":    page > 1,
		"HasNext":    page < totalPages,
		"Query":      r.URL.Query(),
		"Paths":      paths,
		"Dropped":    dropped,
		"Written":    written,
		"Retention":  d.store.RetentionDays(),
	})
}

func (d *Dashboard) handleEntry(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.URL.Query().Get("id"), 10, 64)
	if err != nil {
		http.Error(w, "bad id", http.StatusBadRequest)
		return
	}
	e, err := d.store.Get(id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if e == nil {
		http.NotFound(w, r)
		return
	}

	// Bodies are arbitrary bytes; only render as text when it really is text,
	// otherwise say so rather than emitting control characters into HTML.
	body, isText := "", false
	if e.BodyStored {
		if utf8.Valid(e.Body) {
			body, isText = string(e.Body), true
		}
	}
	reqBody, reqIsText := "", false
	if e.ReqBodyStored {
		if utf8.Valid(e.ReqBody) {
			reqBody, reqIsText = string(e.ReqBody), true
		}
	}

	d.render(w, "entry.html", map[string]any{
		"Prefix":    d.prefix,
		"E":         e,
		"Body":      body,
		"IsText":    isText,
		"ReqBody":   reqBody,
		"ReqIsText": reqIsText,
	})
}

// ---- stats --------------------------------------------------------------

func (d *Dashboard) handleStats(w http.ResponseWriter, r *http.Request) {
	f, _, _ := filterFromQuery(r)
	f.Limit, f.Offset = 0, 0

	summary, err := d.store.Summary(f)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	statuses, _ := d.store.StatusBreakdown(f)
	paths, _ := d.store.TopPaths(f, 10)
	days, _ := d.store.PerDay(f)

	var maxStatus, maxDay int64
	for _, s := range statuses {
		maxStatus = max(maxStatus, s.Count)
	}
	for _, dd := range days {
		maxDay = max(maxDay, dd.Count)
	}

	d.render(w, "stats.html", map[string]any{
		"Prefix":    d.prefix,
		"Summary":   summary,
		"Statuses":  statuses,
		"MaxStatus": maxStatus,
		"Paths":     paths,
		"Days":      days,
		"MaxDay":    maxDay,
		"Query":     r.URL.Query(),
	})
}

// ---- settings -----------------------------------------------------------

func (d *Dashboard) handleSettings(w http.ResponseWriter, r *http.Request) {
	data := map[string]any{
		"Prefix":    d.prefix,
		"Retention": d.store.RetentionDays(),
		"Capture":   string(d.store.cfg.Capture),
		"MaxBody":   int64(d.store.cfg.MaxBodyBytes), // fmtBytes is int64-typed; templates do not coerce
		"DBPath":    d.store.cfg.Path,
	}

	if r.Method == http.MethodPost {
		days, err := strconv.Atoi(strings.TrimSpace(r.FormValue("retention_days")))
		switch {
		case err != nil:
			data["Error"] = "Retention must be a whole number of days."
		case days < 0:
			data["Error"] = "Retention cannot be negative."
		case days > 3650:
			data["Error"] = "Retention above 3650 days is almost certainly a mistake."
		default:
			if err := d.store.SetRetentionDays(days); err != nil {
				data["Error"] = err.Error()
			} else {
				removed, _ := d.store.Prune(time.Now())
				data["Retention"] = days
				if days == 0 {
					data["Notice"] = "Retention disabled — entries are kept indefinitely."
				} else {
					data["Notice"] = fmt.Sprintf(
						"Window set to %d day(s). %d entr%s removed.",
						days, removed, plural(removed))
				}
			}
		}
	}

	sum, _ := d.store.Summary(Filter{})
	data["Summary"] = sum
	d.render(w, "settings.html", data)
}

// ---- helpers ------------------------------------------------------------

func filterFromQuery(r *http.Request) (Filter, int, int) {
	q := r.URL.Query()
	f := Filter{
		Path:   q.Get("path"),
		Search: strings.TrimSpace(q.Get("q")),
	}

	switch q.Get("status") {
	case "":
	case "errors":
		f.StatusFrom = 400
	case "2xx":
		f.StatusFrom, f.StatusTo = 200, 299
	case "4xx":
		f.StatusFrom, f.StatusTo = 400, 499
	case "5xx":
		f.StatusFrom, f.StatusTo = 500, 599
	default:
		if code, err := strconv.Atoi(q.Get("status")); err == nil {
			f.Status = code
		}
	}

	if days, err := strconv.Atoi(q.Get("days")); err == nil && days > 0 {
		f.From = time.Now().AddDate(0, 0, -days)
	}

	perPage := 50
	page := 1
	if p, err := strconv.Atoi(q.Get("page")); err == nil && p > 1 {
		page = p
	}
	f.Limit = perPage
	f.Offset = (page - 1) * perPage
	return f, page, perPage
}

func humanBytes(n int64) string {
	const unit = 1024
	if n < unit {
		return fmt.Sprintf("%d B", n)
	}
	div, exp := int64(unit), 0
	for m := n / unit; m >= unit; m /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(n)/float64(div), "KMGTPE"[exp])
}

func plural(n int64) string {
	if n == 1 {
		return "y"
	}
	return "ies"
}
