package reqlog

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	testUser = "admin"
	testPass = "correct-horse-battery"
)

func newTestDashboard(t *testing.T) (*Dashboard, *Store, *http.ServeMux) {
	t.Helper()
	store := newTestStore(t)

	hash, err := HashPassword(testPass)
	require.NoError(t, err)

	auth, err := NewAuth(AuthConfig{
		User: testUser, PasswordHash: hash,
		SessionSecret: "test-secret-stable-across-this-test",
	})
	require.NoError(t, err)

	d, err := NewDashboard(store, auth, "/logs")
	require.NoError(t, err)

	mux := http.NewServeMux()
	d.Register(mux)
	return d, store, mux
}

// login performs a real login and returns the session cookie.
func login(t *testing.T, mux *http.ServeMux) *http.Cookie {
	t.Helper()
	form := url.Values{"username": {testUser}, "password": {testPass}}
	req := httptest.NewRequest("POST", "/logs/login", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	require.Equal(t, http.StatusSeeOther, rec.Code, "valid credentials should redirect")
	for _, c := range rec.Result().Cookies() {
		if c.Name == sessionCookie && c.Value != "" {
			return c
		}
	}
	t.Fatal("no session cookie issued")
	return nil
}

func get(t *testing.T, mux *http.ServeMux, path string, c *http.Cookie) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest("GET", path, nil)
	if c != nil {
		req.AddCookie(c)
	}
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	return rec
}

// ---- auth ---------------------------------------------------------------

func TestNewAuth_RejectsMissingOrNonBcryptCredentials(t *testing.T) {
	_, err := NewAuth(AuthConfig{})
	require.Error(t, err, "must refuse to start without credentials")
	assert.Contains(t, err.Error(), "DASHBOARD_USER")

	_, err = NewAuth(AuthConfig{User: "admin", PasswordHash: "plaintext-oops"})
	require.Error(t, err, "a plaintext password must be rejected, not silently accepted")
	assert.Contains(t, err.Error(), "bcrypt")
}

func TestAuth_Check(t *testing.T) {
	hash, err := HashPassword(testPass)
	require.NoError(t, err)
	a, err := NewAuth(AuthConfig{User: testUser, PasswordHash: hash, SessionSecret: "s"})
	require.NoError(t, err)

	assert.True(t, a.Check(testUser, testPass))
	assert.False(t, a.Check(testUser, "wrong"), "wrong password")
	assert.False(t, a.Check("someone", testPass), "wrong username")
	assert.False(t, a.Check("", ""), "empty credentials")
}

func TestDashboard_RequiresLogin(t *testing.T) {
	_, _, mux := newTestDashboard(t)
	for _, p := range []string{"/logs/", "/logs/stats", "/logs/settings", "/logs/entry?id=1"} {
		rec := get(t, mux, p, nil)
		assert.Equal(t, http.StatusSeeOther, rec.Code, "%s must redirect when signed out", p)
		assert.Contains(t, rec.Header().Get("Location"), "/logs/login", "%s", p)
	}
}

func TestDashboard_LoginFlow(t *testing.T) {
	_, _, mux := newTestDashboard(t)

	t.Run("login page renders", func(t *testing.T) {
		rec := get(t, mux, "/logs/login", nil)
		assert.Equal(t, 200, rec.Code)
		assert.Contains(t, rec.Body.String(), "Sign in")
	})

	t.Run("bad password is rejected without revealing which field", func(t *testing.T) {
		form := url.Values{"username": {testUser}, "password": {"nope"}}
		req := httptest.NewRequest("POST", "/logs/login", strings.NewReader(form.Encode()))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusUnauthorized, rec.Code)
		body := rec.Body.String()
		assert.Contains(t, body, "Incorrect username or password")
		assert.NotContains(t, strings.ToLower(body), "no such user")
	})

	t.Run("valid login grants access", func(t *testing.T) {
		c := login(t, mux)
		rec := get(t, mux, "/logs/", c)
		assert.Equal(t, 200, rec.Code)
	})
}

func TestAuth_SessionCookieIsHardened(t *testing.T) {
	_, _, mux := newTestDashboard(t)
	c := login(t, mux)
	assert.True(t, c.HttpOnly, "session cookie must be HttpOnly")
	assert.Equal(t, http.SameSiteLaxMode, c.SameSite)
	assert.Positive(t, c.MaxAge)
}

func TestAuth_TamperedCookieIsRejected(t *testing.T) {
	_, _, mux := newTestDashboard(t)
	c := login(t, mux)

	// Flip the username while keeping the original signature.
	parts := strings.Split(c.Value, "|")
	require.Len(t, parts, 3)
	forged := &http.Cookie{Name: sessionCookie, Value: "attacker|" + parts[1] + "|" + parts[2]}

	rec := get(t, mux, "/logs/", forged)
	assert.Equal(t, http.StatusSeeOther, rec.Code, "a forged cookie must not authenticate")
}

func TestAuth_ExpiredCookieIsRejected(t *testing.T) {
	hash, err := HashPassword(testPass)
	require.NoError(t, err)
	a, err := NewAuth(AuthConfig{User: testUser, PasswordHash: hash, SessionSecret: "s"})
	require.NoError(t, err)

	// Issued far enough in the past that it has lapsed.
	stale := a.issue(testUser, time.Now().Add(-2*sessionMaxAge))
	_, ok := a.validate(stale, time.Now())
	assert.False(t, ok, "an expired session must not validate")

	fresh := a.issue(testUser, time.Now())
	_, ok = a.validate(fresh, time.Now())
	assert.True(t, ok)
}

func TestAuth_SessionSecretIsolatesInstances(t *testing.T) {
	hash, _ := HashPassword(testPass)
	a1, _ := NewAuth(AuthConfig{User: testUser, PasswordHash: hash, SessionSecret: "secret-one"})
	a2, _ := NewAuth(AuthConfig{User: testUser, PasswordHash: hash, SessionSecret: "secret-two"})

	token := a1.issue(testUser, time.Now())
	_, ok := a2.validate(token, time.Now())
	assert.False(t, ok, "a cookie signed with another secret must not validate")
}

func TestAuth_ThrottlesRepeatedFailures(t *testing.T) {
	hash, _ := HashPassword(testPass)
	a, err := NewAuth(AuthConfig{User: testUser, PasswordHash: hash, SessionSecret: "s"})
	require.NoError(t, err)

	const ip = "203.0.113.7"
	locked, _ := a.Throttled(ip)
	require.False(t, locked, "clean IP starts unlocked")

	for i := 0; i < maxLoginTries; i++ {
		a.NoteFailure(ip)
	}
	locked, wait := a.Throttled(ip)
	assert.True(t, locked, "repeated failures must lock the IP out")
	assert.Positive(t, wait)

	// A different IP is unaffected.
	other, _ := a.Throttled("198.51.100.4")
	assert.False(t, other)

	a.NoteSuccess(ip)
	locked, _ = a.Throttled(ip)
	assert.False(t, locked, "a successful login clears the counter")
}

func TestDashboard_Logout(t *testing.T) {
	_, _, mux := newTestDashboard(t)
	c := login(t, mux)

	rec := get(t, mux, "/logs/logout", c)
	assert.Equal(t, http.StatusSeeOther, rec.Code)

	var cleared bool
	for _, ck := range rec.Result().Cookies() {
		if ck.Name == sessionCookie && ck.MaxAge < 0 {
			cleared = true
		}
	}
	assert.True(t, cleared, "logout must expire the session cookie")
}

// ---- pages --------------------------------------------------------------

// Every page is rendered for real: template errors surface only at execution
// time, so parsing alone would prove nothing.
func TestDashboard_PagesRender(t *testing.T) {
	_, store, mux := newTestDashboard(t)
	c := login(t, mux)

	now := time.Now()
	recordSync(t, store,
		&Entry{At: now.Add(-2 * time.Hour), Method: "GET", Path: "/username",
			Query: "username=someone", Status: 200, Duration: 120 * time.Millisecond, RespBytes: 900},
		&Entry{At: now.Add(-time.Hour), Method: "GET", Path: "/get-post",
			Status: 502, Duration: 2 * time.Second, RespBytes: 42,
			Body: []byte(`{"errors":["upstream failed"]}`), BodyStored: true},
	)

	for _, tc := range []struct{ name, path, want string }{
		{"list", "/logs/", "/username"},
		{"list filtered by errors", "/logs/?status=errors", "/get-post"},
		{"list with search", "/logs/?q=username", "/username"},
		{"stats", "/logs/stats", "Status codes"},
		{"stats windowed", "/logs/stats?days=7", "Requests per day"},
		{"settings", "/logs/settings", "Retention"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			rec := get(t, mux, tc.path, c)
			assert.Equal(t, 200, rec.Code)
			assert.Contains(t, rec.Body.String(), tc.want)
			// A template that fails mid-execution yields a truncated page.
			assert.Contains(t, rec.Body.String(), "</html>", "page should render to completion")
		})
	}
}

func TestDashboard_EntryPage_ShowsDecompressedBody(t *testing.T) {
	_, store, mux := newTestDashboard(t)
	c := login(t, mux)

	marker := "upstream exploded in a distinctive way"
	recordSync(t, store, &Entry{
		At: time.Now(), Method: "GET", Path: "/get-post", Status: 500,
		Body: []byte(`{"errors":["` + marker + `"]}`), BodyStored: true,
	})
	list, err := store.List(Filter{})
	require.NoError(t, err)
	require.Len(t, list, 1)

	rec := get(t, mux, "/logs/entry?id="+strconv.FormatInt(list[0].ID, 10), c)
	assert.Equal(t, 200, rec.Code)
	assert.Contains(t, rec.Body.String(), marker,
		"the gzip'd body must be decompressed for display")
}

func TestDashboard_EntryPage_MissingID(t *testing.T) {
	_, _, mux := newTestDashboard(t)
	c := login(t, mux)

	assert.Equal(t, http.StatusNotFound, get(t, mux, "/logs/entry?id=99999", c).Code)
	assert.Equal(t, http.StatusBadRequest, get(t, mux, "/logs/entry?id=abc", c).Code)
}

// Changing the window from the UI must both persist and prune immediately.
func TestDashboard_SettingsUpdatesRetentionAndPrunes(t *testing.T) {
	_, store, mux := newTestDashboard(t)
	c := login(t, mux)

	now := time.Now()
	recordSync(t, store,
		entryAt(now.AddDate(0, 0, -40), 200, "/old"),
		entryAt(now, 200, "/new"),
	)

	form := url.Values{"retention_days": {"7"}}
	req := httptest.NewRequest("POST", "/logs/settings", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.AddCookie(c)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	assert.Equal(t, 200, rec.Code)
	assert.Equal(t, 7, store.RetentionDays(), "new window must be persisted")

	left, err := store.List(Filter{})
	require.NoError(t, err)
	require.Len(t, left, 1, "saving should prune immediately")
	assert.Equal(t, "/new", left[0].Path)
}

func TestDashboard_SettingsRejectsBadInput(t *testing.T) {
	_, store, mux := newTestDashboard(t)
	c := login(t, mux)
	before := store.RetentionDays()

	for _, bad := range []string{"-5", "not-a-number", "99999"} {
		form := url.Values{"retention_days": {bad}}
		req := httptest.NewRequest("POST", "/logs/settings", strings.NewReader(form.Encode()))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		req.AddCookie(c)
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, req)

		assert.Equal(t, 200, rec.Code, "input %q", bad)
		assert.Equal(t, before, store.RetentionDays(), "invalid input %q must not change the window", bad)
	}
}

func TestDashboard_SetsNoStoreAndNoIndex(t *testing.T) {
	_, _, mux := newTestDashboard(t)
	c := login(t, mux)
	rec := get(t, mux, "/logs/", c)

	assert.Equal(t, "no-store", rec.Header().Get("Cache-Control"),
		"logged request data must not be cached")
	assert.Contains(t, rec.Header().Get("X-Robots-Tag"), "noindex")
}
