package reqlog

import (
	"net"
	"net/http"
	"strings"
	"time"
)

// Middleware records one entry per request.
//
// It is deliberately defensive about the request path it wraps: capture is
// bounded, the write is queued rather than performed inline, and nothing
// here can fail the underlying handler.
func (s *Store) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()

		rec := &recorder{
			ResponseWriter: w,
			status:         http.StatusOK, // Go's default when WriteHeader is never called
			capture:        s.cfg.Capture == CaptureAll,
			limit:          s.cfg.MaxBodyBytes,
		}

		next.ServeHTTP(rec, r)

		// For CaptureErrors the decision can only be made after the handler
		// has run, so buffering starts optimistically for every request and
		// the buffer is discarded here when it turns out not to be wanted.
		// With CaptureNone nothing is buffered at all.
		keep := false
		switch s.cfg.Capture {
		case CaptureAll:
			keep = true
		case CaptureErrors:
			keep = rec.status >= 400
		}

		e := &Entry{
			At:        start,
			Method:    r.Method,
			Path:      r.URL.Path,
			Query:     redactQuery(r.URL.RawQuery),
			Status:    rec.status,
			Duration:  time.Since(start),
			ReqBytes:  max(r.ContentLength, 0),
			RespBytes: rec.written,
			IP:        clientIP(r),
			UserAgent: r.UserAgent(),
		}
		if keep && rec.buf != nil {
			e.Body = rec.buf
			e.BodyStored = true
			e.Truncated = rec.truncated
		}
		s.Record(e)
	})
}

// MiddlewareFunc adapts Middleware to gorilla/mux's Use.
func (s *Store) MiddlewareFunc() func(http.Handler) http.Handler {
	return s.Middleware
}

// recorder tees the response so status, size and (optionally) body can be
// recorded without altering what the client receives.
type recorder struct {
	http.ResponseWriter
	status      int
	written     int64
	buf         []byte
	capture     bool
	limit       int
	truncated   bool
	wroteHeader bool
}

func (rc *recorder) WriteHeader(code int) {
	if rc.wroteHeader {
		return // mirrors net/http: only the first WriteHeader counts
	}
	rc.status = code
	rc.wroteHeader = true

	// With CaptureErrors we only learn the status here, which is still
	// before any body is written — so start buffering exactly when it turns
	// out to be wanted, and never for successful responses.
	if !rc.capture && code >= 400 {
		rc.capture = true
	}
	rc.ResponseWriter.WriteHeader(code)
}

func (rc *recorder) Write(b []byte) (int, error) {
	n, err := rc.ResponseWriter.Write(b)
	rc.written += int64(n)

	if rc.capture && rc.limit > 0 {
		remaining := rc.limit - len(rc.buf)
		if remaining > 0 {
			take := b[:min(len(b), remaining)]
			rc.buf = append(rc.buf, take...)
			if len(take) < len(b) {
				rc.truncated = true
			}
		} else {
			rc.truncated = true
		}
	}
	return n, err
}

// Flush forwards to the wrapped writer when it supports flushing, so
// wrapping does not silently disable streaming responses.
func (rc *recorder) Flush() {
	if f, ok := rc.ResponseWriter.(http.Flusher); ok {
		f.Flush()
	}
}

// sensitiveParams are query keys whose values are never stored.
var sensitiveParams = map[string]bool{
	"secret":   true,
	"token":    true,
	"password": true,
	"key":      true,
	"apikey":   true,
	"api_key":  true,
}

// redactQuery keeps the query string useful for debugging while masking
// values that would be credentials.
//
// Note the header X-RapidAPI-Proxy-Secret is never recorded at all — this
// middleware stores no request headers beyond User-Agent, by design.
func redactQuery(raw string) string {
	if raw == "" {
		return ""
	}
	parts := strings.Split(raw, "&")
	for i, p := range parts {
		k, _, found := strings.Cut(p, "=")
		if found && sensitiveParams[strings.ToLower(k)] {
			parts[i] = k + "=REDACTED"
		}
	}
	return strings.Join(parts, "&")
}

// clientIP prefers the forwarded address, since the service runs behind a
// proxy and RemoteAddr would otherwise always be the proxy itself.
func clientIP(r *http.Request) string {
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		if first, _, found := strings.Cut(xff, ","); found {
			return strings.TrimSpace(first)
		}
		return strings.TrimSpace(xff)
	}
	if rip := r.Header.Get("X-Real-IP"); rip != "" {
		return strings.TrimSpace(rip)
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}
