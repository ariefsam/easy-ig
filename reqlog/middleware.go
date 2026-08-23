package reqlog

import (
	"bytes"
	"io"
	"net"
	"net/http"
	"sort"
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

		// Request side, captured before the handler consumes the body.
		reqHeaders := ""
		var reqBody []byte
		reqTruncated := false
		if s.cfg.Capture != CaptureNone {
			reqHeaders = renderHeaders(r.Header)
			reqBody, reqTruncated = drainBody(r, s.cfg.MaxBodyBytes)
		}

		// Handlers can attach internal detail via reqlog.Note, so a response
		// body of {"error":"system error"} still leaves a diagnosable entry.
		r, notes := withNotes(r)

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
			Err:       notes.String(),
		}
		if keep && rec.buf != nil {
			e.Body = rec.buf
			e.BodyStored = true
			e.Truncated = rec.truncated
		}
		if keep {
			e.ReqHeaders = reqHeaders
			if len(reqBody) > 0 {
				e.ReqBody = reqBody
				e.ReqBodyStored = true
				e.ReqTruncated = reqTruncated
			}
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

	if rc.capture {
		switch {
		case rc.limit == Unlimited:
			rc.buf = append(rc.buf, b...)
		case rc.limit > 0:
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

// sensitiveHeaders are never recorded. X-RapidAPI-Proxy-Secret is the shared
// secret guarding the API, so storing it would put a working credential in
// the log the dashboard displays.
var sensitiveHeaders = map[string]bool{
	"x-rapidapi-proxy-secret": true,
	"authorization":           true,
	"cookie":                  true,
	"set-cookie":              true,
	"proxy-authorization":     true,
	"x-api-key":               true,
}

// renderHeaders formats request headers one per line, masking the values of
// anything credential-bearing while keeping the key visible — knowing a
// header was present is often the point.
func renderHeaders(h http.Header) string {
	if len(h) == 0 {
		return ""
	}
	keys := make([]string, 0, len(h))
	for k := range h {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	var b strings.Builder
	for _, k := range keys {
		if sensitiveHeaders[strings.ToLower(k)] {
			b.WriteString(k + ": REDACTED\n")
			continue
		}
		for _, v := range h.Values(k) {
			b.WriteString(k + ": " + v + "\n")
		}
	}
	return strings.TrimRight(b.String(), "\n")
}

// drainBody reads up to limit bytes of the request body and puts it back, so
// the handler still sees a complete body.
//
// Reading is capped: an oversized upload would otherwise be buffered whole
// just to log a fraction of it. Whatever is left unread is streamed straight
// through to the handler rather than discarded.
func drainBody(r *http.Request, limit int) (body []byte, truncated bool) {
	if r.Body == nil || r.Body == http.NoBody || limit == 0 {
		return nil, false
	}
	// GET/HEAD/DELETE conventionally carry no body; skip the allocation.
	switch r.Method {
	case http.MethodGet, http.MethodHead, http.MethodDelete, http.MethodOptions:
		return nil, false
	}

	if limit == Unlimited {
		all, err := io.ReadAll(r.Body)
		if err != nil {
			// Hand back what was read; the handler sees the same stream end.
			r.Body = readCloser{bytes.NewReader(all), r.Body}
			return all, false
		}
		r.Body = readCloser{bytes.NewReader(all), r.Body}
		return all, false
	}

	buf := make([]byte, limit)
	n, err := io.ReadFull(r.Body, buf)
	buf = buf[:n]

	if err == io.EOF || err == io.ErrUnexpectedEOF {
		// Whole body fit within the cap.
		r.Body = readCloser{io.NopCloser(bytes.NewReader(buf)), r.Body}
		return buf, false
	}
	if err != nil {
		// Read failed; hand back what we have and let the handler see the rest.
		r.Body = readCloser{io.MultiReader(bytes.NewReader(buf), r.Body), r.Body}
		return buf, false
	}
	// Filled the cap exactly — there may be more, so the handler gets the
	// captured prefix followed by the untouched remainder.
	r.Body = readCloser{io.MultiReader(bytes.NewReader(buf), r.Body), r.Body}
	return buf, true
}

// readCloser pairs a replacement reader with the original body's Close, so
// the underlying connection is still released.
type readCloser struct {
	io.Reader
	orig io.Closer
}

func (rc readCloser) Close() error { return rc.orig.Close() }
