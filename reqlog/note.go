package reqlog

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"sync"
)

// noteKey is the context key carrying the per-request error collector.
type noteKey struct{}

// notes accumulates internal detail for one request.
//
// Handlers frequently return a deliberately vague body — "system error" —
// so as not to leak internals to API consumers. That leaves the log entry
// equally vague. Notes bridge the gap: the client still gets the generic
// message, while the dashboard records what actually went wrong.
type notes struct {
	mu   sync.Mutex
	msgs []string
}

func (n *notes) add(s string) {
	s = strings.TrimSpace(s)
	if s == "" {
		return
	}
	n.mu.Lock()
	defer n.mu.Unlock()
	// Bounded: a handler in a retry loop could otherwise append without limit.
	if len(n.msgs) >= 20 {
		return
	}
	n.msgs = append(n.msgs, s)
}

func (n *notes) String() string {
	n.mu.Lock()
	defer n.mu.Unlock()
	return strings.Join(n.msgs, " | ")
}

// withNotes attaches a collector to the request context.
func withNotes(r *http.Request) (*http.Request, *notes) {
	n := &notes{}
	return r.WithContext(context.WithValue(r.Context(), noteKey{}, n)), n
}

// Note records internal error detail against the current request, to appear
// on the dashboard entry for it.
//
// Safe to call unconditionally: when request logging is disabled, or the
// request never passed through Middleware, this is a no-op. That means call
// sites need no guard.
//
// Whatever is passed here is stored and shown on the dashboard — pass the
// error, not the credentials or payload that caused it.
func Note(r *http.Request, err error) {
	if r == nil || err == nil {
		return
	}
	if n, ok := r.Context().Value(noteKey{}).(*notes); ok {
		n.add(err.Error())
	}
}

// Notef records a formatted message against the current request.
func Notef(r *http.Request, format string, args ...any) {
	if r == nil {
		return
	}
	if n, ok := r.Context().Value(noteKey{}).(*notes); ok {
		n.add(fmt.Sprintf(format, args...))
	}
}
