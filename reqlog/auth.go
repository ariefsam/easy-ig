package reqlog

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"golang.org/x/crypto/bcrypt"
)

const (
	sessionCookie = "reqlog_session"
	sessionMaxAge = 12 * time.Hour
	maxLoginTries = 8
	loginLockout  = 15 * time.Minute
)

// AuthConfig holds the dashboard credentials, read from the environment.
type AuthConfig struct {
	// User is the single dashboard account name.
	User string
	// PasswordHash is a bcrypt hash. Generate one with cmd/dashboard-hash.
	PasswordHash string
	// SessionSecret signs session cookies. When empty a random secret is
	// generated at startup, which is secure but logs everyone out on
	// restart — set it explicitly to avoid that.
	SessionSecret string
}

// Auth guards the dashboard.
type Auth struct {
	cfg    AuthConfig
	secret []byte

	mu       sync.Mutex
	attempts map[string]*attemptRecord
}

type attemptRecord struct {
	count int
	until time.Time
}

// NewAuth validates the configuration and prepares the guard.
//
// Returns an error rather than degrading when credentials are missing: a
// dashboard that silently accepts everyone is worse than one that refuses
// to start, since this exposes request logs and error bodies.
func NewAuth(cfg AuthConfig) (*Auth, error) {
	if cfg.User == "" || cfg.PasswordHash == "" {
		return nil, fmt.Errorf(
			"dashboard credentials missing: set DASHBOARD_USER and DASHBOARD_PASSWORD_HASH " +
				"(generate a hash with: go run ./cmd/dashboard-hash)")
	}
	if !strings.HasPrefix(cfg.PasswordHash, "$2") {
		return nil, fmt.Errorf(
			"DASHBOARD_PASSWORD_HASH does not look like a bcrypt hash (expected a $2a$/$2b$ prefix). " +
				"If you did paste a bcrypt hash, wrap it in SINGLE QUOTES in .env — " +
				"unquoted values undergo $VAR expansion, which eats the dollar signs in the hash. " +
				"Generate a correctly-quoted line with: go run ./cmd/dashboard-hash")
	}

	secret := []byte(cfg.SessionSecret)
	if len(secret) == 0 {
		secret = make([]byte, 32)
		if _, err := rand.Read(secret); err != nil {
			return nil, fmt.Errorf("generate session secret: %w", err)
		}
		log.Println("reqlog: DASHBOARD_SESSION_SECRET unset — using a random secret; " +
			"dashboard sessions will not survive a restart")
	}

	return &Auth{cfg: cfg, secret: secret, attempts: map[string]*attemptRecord{}}, nil
}

// Check verifies a username/password pair.
//
// Both branches run bcrypt so a wrong username costs the same as a wrong
// password — otherwise response timing reveals which usernames exist.
func (a *Auth) Check(user, password string) bool {
	userOK := subtle.ConstantTimeCompare([]byte(user), []byte(a.cfg.User)) == 1
	err := bcrypt.CompareHashAndPassword([]byte(a.cfg.PasswordHash), []byte(password))
	return userOK && err == nil
}

// ---- brute-force throttling --------------------------------------------

// Throttled reports whether this IP is currently locked out.
func (a *Auth) Throttled(ip string) (bool, time.Duration) {
	a.mu.Lock()
	defer a.mu.Unlock()
	rec, ok := a.attempts[ip]
	if !ok {
		return false, 0
	}
	if time.Now().After(rec.until) {
		delete(a.attempts, ip)
		return false, 0
	}
	if rec.count >= maxLoginTries {
		return true, time.Until(rec.until)
	}
	return false, 0
}

// NoteFailure records a failed attempt and extends the lockout window.
func (a *Auth) NoteFailure(ip string) {
	a.mu.Lock()
	defer a.mu.Unlock()
	rec, ok := a.attempts[ip]
	if !ok || time.Now().After(rec.until) {
		rec = &attemptRecord{}
		a.attempts[ip] = rec
	}
	rec.count++
	rec.until = time.Now().Add(loginLockout)
}

// NoteSuccess clears the counter for an IP.
func (a *Auth) NoteSuccess(ip string) {
	a.mu.Lock()
	defer a.mu.Unlock()
	delete(a.attempts, ip)
}

// ---- sessions -----------------------------------------------------------

// issue builds a signed cookie value: user|expiry|hmac. Stateless, so a
// restart with a stable secret keeps people logged in and there is no
// server-side session table to prune.
func (a *Auth) issue(user string, now time.Time) string {
	exp := now.Add(sessionMaxAge).Unix()
	payload := user + "|" + strconv.FormatInt(exp, 10)
	return payload + "|" + a.sign(payload)
}

func (a *Auth) sign(payload string) string {
	m := hmac.New(sha256.New, a.secret)
	m.Write([]byte(payload))
	return hex.EncodeToString(m.Sum(nil))
}

// validate returns the username when the cookie is authentic and unexpired.
func (a *Auth) validate(raw string, now time.Time) (string, bool) {
	parts := strings.Split(raw, "|")
	if len(parts) != 3 {
		return "", false
	}
	user, expRaw, sig := parts[0], parts[1], parts[2]

	if !hmac.Equal([]byte(sig), []byte(a.sign(user+"|"+expRaw))) {
		return "", false
	}
	exp, err := strconv.ParseInt(expRaw, 10, 64)
	if err != nil || now.Unix() >= exp {
		return "", false
	}
	// The signature covers the name, but a renamed account should not keep
	// old cookies working.
	if subtle.ConstantTimeCompare([]byte(user), []byte(a.cfg.User)) != 1 {
		return "", false
	}
	return user, true
}

// SetSession writes the session cookie.
func (a *Auth) SetSession(w http.ResponseWriter, r *http.Request, user string) {
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookie,
		Value:    a.issue(user, time.Now()),
		Path:     "/",
		HttpOnly: true,
		Secure:   isTLS(r),
		SameSite: http.SameSiteLaxMode,
		MaxAge:   int(sessionMaxAge.Seconds()),
	})
}

// ClearSession expires the session cookie.
func (a *Auth) ClearSession(w http.ResponseWriter, r *http.Request) {
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookie,
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		Secure:   isTLS(r),
		SameSite: http.SameSiteLaxMode,
		MaxAge:   -1,
	})
}

// LoggedIn reports whether the request carries a valid session.
func (a *Auth) LoggedIn(r *http.Request) bool {
	c, err := r.Cookie(sessionCookie)
	if err != nil {
		return false
	}
	_, ok := a.validate(c.Value, time.Now())
	return ok
}

// Require redirects unauthenticated requests to the login page.
func (a *Auth) Require(loginPath string, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !a.LoggedIn(r) {
			http.Redirect(w, r, loginPath+"?next="+urlEscape(r.URL.RequestURI()), http.StatusSeeOther)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// isTLS reports whether the original client connection was HTTPS, honouring
// the forwarding header so the Secure flag is still set behind a TLS proxy.
func isTLS(r *http.Request) bool {
	if r.TLS != nil {
		return true
	}
	return strings.EqualFold(r.Header.Get("X-Forwarded-Proto"), "https")
}

func urlEscape(s string) string {
	return base64.RawURLEncoding.EncodeToString([]byte(s))
}

func urlUnescape(s string) string {
	b, err := base64.RawURLEncoding.DecodeString(s)
	if err != nil {
		return ""
	}
	return string(b)
}

// HashPassword produces a bcrypt hash for DASHBOARD_PASSWORD_HASH.
func HashPassword(plain string) (string, error) {
	b, err := bcrypt.GenerateFromPassword([]byte(plain), bcrypt.DefaultCost)
	return string(b), err
}
