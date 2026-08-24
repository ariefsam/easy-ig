package reqlog

import (
	"database/sql"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// newTestStore opens a Store on a throwaway file. A real file rather than
// :memory: because the retention path VACUUMs and Summary reports the file
// size — both meaningless in memory.
func newTestStore(t *testing.T, mutate ...func(*Config)) *Store {
	t.Helper()
	cfg := Config{Path: t.TempDir() + "/reqlog.db"}
	for _, m := range mutate {
		m(&cfg)
	}
	s, err := Open(cfg)
	require.NoError(t, err)
	t.Cleanup(func() { _ = s.Close() })
	return s
}

// recordSync queues entries and waits for the writer to drain them, so
// assertions do not race the background goroutine.
func recordSync(t *testing.T, s *Store, entries ...*Entry) {
	t.Helper()
	before, _ := s.WriterStats()
	for _, e := range entries {
		s.Record(e)
	}
	want := before + int64(len(entries))
	deadline := time.Now().Add(3 * time.Second)
	for {
		if got, _ := s.WriterStats(); got >= want {
			return
		}
		if time.Now().After(deadline) {
			got, dropped := s.WriterStats()
			t.Fatalf("writer did not drain: written=%d want=%d dropped=%d", got, want, dropped)
		}
		time.Sleep(2 * time.Millisecond)
	}
}

// waitWritten blocks until the async writer has persisted n entries.
func waitWritten(t *testing.T, s *Store, n int64) {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for {
		if got, _ := s.WriterStats(); got >= n {
			return
		}
		if time.Now().After(deadline) {
			got, dropped := s.WriterStats()
			t.Fatalf("writer did not reach %d: written=%d dropped=%d", n, got, dropped)
		}
		time.Sleep(2 * time.Millisecond)
	}
}

func entryAt(at time.Time, status int, path string) *Entry {
	return &Entry{At: at, Method: "GET", Path: path, Status: status,
		Duration: 100 * time.Millisecond, RespBytes: 10}
}

func TestStore_RoundTrip(t *testing.T) {
	s := newTestStore(t)
	now := time.Now()

	recordSync(t, s, &Entry{
		At: now, Method: "GET", Path: "/username", Query: "username=someone",
		Status: 200, Duration: 250 * time.Millisecond, RespBytes: 1234,
		IP: "10.0.0.1", UserAgent: "test-agent",
	})

	got, err := s.List(Filter{})
	require.NoError(t, err)
	require.Len(t, got, 1)

	e := got[0]
	assert.Equal(t, "GET", e.Method)
	assert.Equal(t, "/username", e.Path)
	assert.Equal(t, "username=someone", e.Query)
	assert.Equal(t, 200, e.Status)
	assert.Equal(t, int64(1234), e.RespBytes)
	assert.Equal(t, "10.0.0.1", e.IP)
	assert.Equal(t, 250*time.Millisecond, e.Duration)
	// Millisecond precision is all the schema stores.
	assert.WithinDuration(t, now, e.At, time.Second)
}

// The body is stored gzip-compressed and must survive the round trip
// byte-for-byte — this is the compression the feature is built on.
func TestStore_BodyCompressionRoundTrip(t *testing.T) {
	s := newTestStore(t)

	// Repetitive JSON so compression genuinely engages.
	body := []byte(`{"errors":["` + strings.Repeat("something went wrong. ", 500) + `"]}`)

	recordSync(t, s, &Entry{
		At: time.Now(), Method: "GET", Path: "/username", Status: 500,
		Body: body, BodyStored: true, RespBytes: int64(len(body)),
	})

	list, err := s.List(Filter{})
	require.NoError(t, err)
	require.Len(t, list, 1)
	assert.True(t, list[0].BodyStored, "listing should flag that a body exists")
	assert.Nil(t, list[0].Body, "listing must not load the blob")

	full, err := s.Get(list[0].ID)
	require.NoError(t, err)
	require.NotNil(t, full)
	assert.Equal(t, body, full.Body, "body must decompress to the exact original")
}

func TestStore_CompressionActuallyShrinksStorage(t *testing.T) {
	body := []byte(strings.Repeat(`{"status":"error","detail":"upstream timeout"}`, 400))
	gz, err := gzipBytes(body)
	require.NoError(t, err)

	assert.Less(t, len(gz), len(body)/4,
		"repetitive JSON should compress to well under a quarter: got %d from %d",
		len(gz), len(body))

	back, err := gunzipBytes(gz)
	require.NoError(t, err)
	assert.Equal(t, body, back)
}

// ---- sliding window -----------------------------------------------------

func TestStore_Prune_SlidingWindow(t *testing.T) {
	s := newTestStore(t, func(c *Config) { c.RetentionDays = 7 })
	now := time.Date(2026, 8, 23, 12, 0, 0, 0, time.UTC)

	recordSync(t, s,
		entryAt(now.AddDate(0, 0, -30), 200, "/old"), // outside
		entryAt(now.AddDate(0, 0, -8), 200, "/old"),  // outside (just)
		entryAt(now.AddDate(0, 0, -6), 200, "/keep"), // inside
		entryAt(now.AddDate(0, 0, -1), 500, "/keep"), // inside
		entryAt(now, 200, "/keep"),                   // inside
	)

	removed, err := s.Prune(now)
	require.NoError(t, err)
	assert.Equal(t, int64(2), removed, "only entries older than the window go")

	left, err := s.List(Filter{})
	require.NoError(t, err)
	require.Len(t, left, 3)
	for _, e := range left {
		assert.Equal(t, "/keep", e.Path, "a pruned entry survived")
	}
}

func TestStore_Prune_ZeroRetentionKeepsEverything(t *testing.T) {
	s := newTestStore(t, func(c *Config) { c.RetentionDays = 7 })
	require.NoError(t, s.SetRetentionDays(0)) // 0 = disabled

	now := time.Now()
	recordSync(t, s, entryAt(now.AddDate(-2, 0, 0), 200, "/ancient"))

	removed, err := s.Prune(now)
	require.NoError(t, err)
	assert.Zero(t, removed, "retention 0 must disable pruning, not prune everything")

	left, err := s.List(Filter{})
	require.NoError(t, err)
	assert.Len(t, left, 1)
}

// The window is changed from the dashboard, so it has to outlive a restart.
func TestStore_RetentionDays_PersistsAcrossReopen(t *testing.T) {
	dir := t.TempDir()
	path := dir + "/reqlog.db"

	s1, err := Open(Config{Path: path, RetentionDays: 14})
	require.NoError(t, err)
	require.NoError(t, s1.SetRetentionDays(3))
	require.NoError(t, s1.Close())

	// Reopen with a different .env default — the stored value must win.
	s2, err := Open(Config{Path: path, RetentionDays: 30})
	require.NoError(t, err)
	defer s2.Close()

	assert.Equal(t, 3, s2.RetentionDays(),
		"a dashboard-set window must beat the .env default on restart")
}

func TestStore_SetRetentionDays_RejectsNegative(t *testing.T) {
	s := newTestStore(t)
	assert.Error(t, s.SetRetentionDays(-1))
}

// ---- filtering & stats --------------------------------------------------

func TestStore_Filters(t *testing.T) {
	s := newTestStore(t)
	now := time.Now()
	recordSync(t, s,
		entryAt(now.Add(-3*time.Hour), 200, "/username"),
		entryAt(now.Add(-2*time.Hour), 404, "/username"),
		entryAt(now.Add(-1*time.Hour), 500, "/get-post"),
		entryAt(now, 200, "/get-post"),
	)

	t.Run("exact status", func(t *testing.T) {
		got, err := s.List(Filter{Status: 404})
		require.NoError(t, err)
		require.Len(t, got, 1)
		assert.Equal(t, 404, got[0].Status)
	})

	t.Run("errors only via range", func(t *testing.T) {
		got, err := s.List(Filter{StatusFrom: 400})
		require.NoError(t, err)
		assert.Len(t, got, 2)
	})

	t.Run("by path", func(t *testing.T) {
		got, err := s.List(Filter{Path: "/get-post"})
		require.NoError(t, err)
		assert.Len(t, got, 2)
	})

	t.Run("time range", func(t *testing.T) {
		got, err := s.List(Filter{From: now.Add(-90 * time.Minute)})
		require.NoError(t, err)
		assert.Len(t, got, 2)
	})

	t.Run("newest first", func(t *testing.T) {
		got, err := s.List(Filter{})
		require.NoError(t, err)
		require.Len(t, got, 4)
		for i := 1; i < len(got); i++ {
			assert.False(t, got[i].At.After(got[i-1].At), "listing must be newest-first")
		}
	})

	t.Run("count matches filter", func(t *testing.T) {
		n, err := s.Count(Filter{StatusFrom: 400})
		require.NoError(t, err)
		assert.Equal(t, int64(2), n)
	})
}

// A search term with SQL metacharacters must be treated as data.
func TestStore_Search_IsParameterised(t *testing.T) {
	s := newTestStore(t)
	recordSync(t, s, entryAt(time.Now(), 200, "/username"))

	got, err := s.List(Filter{Search: "'; DROP TABLE requests; --"})
	require.NoError(t, err, "a quoted search term must not break the query")
	assert.Empty(t, got)

	// The table must still be there.
	n, err := s.Count(Filter{})
	require.NoError(t, err)
	assert.Equal(t, int64(1), n)
}

func TestStore_Summary(t *testing.T) {
	s := newTestStore(t)
	now := time.Now()
	for i := 0; i < 9; i++ {
		recordSync(t, s, &Entry{At: now, Method: "GET", Path: "/username",
			Status: 200, Duration: time.Duration(i*10) * time.Millisecond})
	}
	recordSync(t, s, &Entry{At: now, Method: "GET", Path: "/username",
		Status: 500, Duration: 900 * time.Millisecond})

	sum, err := s.Summary(Filter{})
	require.NoError(t, err)
	assert.Equal(t, int64(10), sum.Total)
	assert.Equal(t, int64(1), sum.Errors)
	assert.InDelta(t, 10.0, sum.ErrorRate, 0.01)
	assert.Greater(t, sum.DBBytes, int64(0), "file size should be reported")
}

func TestStore_StatusBreakdownAndTopPaths(t *testing.T) {
	s := newTestStore(t)
	now := time.Now()
	recordSync(t, s,
		entryAt(now, 200, "/username"), entryAt(now, 200, "/username"),
		entryAt(now, 404, "/username"), entryAt(now, 200, "/get-post"),
	)

	br, err := s.StatusBreakdown(Filter{})
	require.NoError(t, err)
	require.Len(t, br, 2)
	assert.Equal(t, 200, br[0].Status, "most frequent status first")
	assert.Equal(t, int64(3), br[0].Count)

	paths, err := s.TopPaths(Filter{}, 10)
	require.NoError(t, err)
	require.Len(t, paths, 2)
	assert.Equal(t, "/username", paths[0].Path, "busiest path first")
	assert.Equal(t, int64(3), paths[0].Count)
	assert.Equal(t, int64(1), paths[0].ErrCount)
}

func TestStore_PerDay(t *testing.T) {
	s := newTestStore(t)
	base := time.Date(2026, 8, 20, 10, 0, 0, 0, time.UTC)
	recordSync(t, s,
		entryAt(base, 200, "/a"),
		entryAt(base.Add(time.Hour), 500, "/a"),
		entryAt(base.AddDate(0, 0, 2), 200, "/a"),
	)

	days, err := s.PerDay(Filter{})
	require.NoError(t, err)
	require.Len(t, days, 2, "two distinct days")
	assert.Equal(t, "2026-08-20", days[0].Day, "oldest first")
	assert.Equal(t, int64(2), days[0].Count)
	assert.Equal(t, int64(1), days[0].Errors)
}

func TestStore_Get_MissingReturnsNil(t *testing.T) {
	s := newTestStore(t)
	e, err := s.Get(4242)
	require.NoError(t, err, "a missing id is not an error")
	assert.Nil(t, e)
}

// ---- middleware ---------------------------------------------------------

func serve(t *testing.T, s *Store, h http.Handler, r *http.Request) *httptest.ResponseRecorder {
	t.Helper()
	rec := httptest.NewRecorder()
	s.Middleware(h).ServeHTTP(rec, r)
	return rec
}

func TestMiddleware_RecordsMetadata(t *testing.T) {
	s := newTestStore(t)
	h := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(201)
		_, _ = w.Write([]byte(`{"ok":true}`))
	})

	req := httptest.NewRequest("GET", "/username?username=someone", nil)
	req.Header.Set("User-Agent", "probe/1.0")
	req.Header.Set("X-Forwarded-For", "203.0.113.9, 10.0.0.1")
	rec := serve(t, s, h, req)

	assert.Equal(t, 201, rec.Code, "the client must see the handler's response untouched")
	assert.Equal(t, `{"ok":true}`, rec.Body.String())

	// Drain the async writer.
	deadline := time.Now().Add(3 * time.Second)
	for {
		if n, _ := s.WriterStats(); n >= 1 {
			break
		}
		require.False(t, time.Now().After(deadline), "entry never written")
		time.Sleep(2 * time.Millisecond)
	}

	got, err := s.List(Filter{})
	require.NoError(t, err)
	require.Len(t, got, 1)
	e := got[0]
	assert.Equal(t, 201, e.Status)
	assert.Equal(t, "/username", e.Path)
	assert.Equal(t, int64(11), e.RespBytes)
	assert.Equal(t, "probe/1.0", e.UserAgent)
	assert.Equal(t, "203.0.113.9", e.IP, "the client IP, not the proxy's")
	// Sub-millisecond handlers legitimately round to 0 — the schema stores
	// milliseconds, which is the right granularity for an API whose real
	// calls go out through a proxy. TestMiddleware_RecordsDuration covers
	// that the timing is actually measured.
	assert.GreaterOrEqual(t, e.Duration, time.Duration(0))
}

// Default mode keeps error bodies and discards successful ones — the whole
// point being that base64-image responses never reach the database.
func TestMiddleware_CaptureErrors_OnlyStoresErrorBodies(t *testing.T) {
	s := newTestStore(t, func(c *Config) { c.Capture = CaptureErrors })

	ok := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"huge":"base64-image-payload"}`))
	})
	bad := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(502)
		_, _ = w.Write([]byte(`{"errors":["upstream failed"]}`))
	})

	serve(t, s, ok, httptest.NewRequest("GET", "/username", nil))
	serve(t, s, bad, httptest.NewRequest("GET", "/get-post", nil))

	deadline := time.Now().Add(3 * time.Second)
	for {
		if n, _ := s.WriterStats(); n >= 2 {
			break
		}
		require.False(t, time.Now().After(deadline), "entries never written")
		time.Sleep(2 * time.Millisecond)
	}

	list, err := s.List(Filter{})
	require.NoError(t, err)
	require.Len(t, list, 2)

	byPath := map[string]Entry{}
	for _, e := range list {
		byPath[e.Path] = e
	}
	assert.False(t, byPath["/username"].BodyStored, "successful body must not be stored")
	assert.True(t, byPath["/get-post"].BodyStored, "error body must be stored")

	full, err := s.Get(byPath["/get-post"].ID)
	require.NoError(t, err)
	assert.Contains(t, string(full.Body), "upstream failed")
}

func TestMiddleware_CaptureNone_StoresNoBodies(t *testing.T) {
	s := newTestStore(t, func(c *Config) { c.Capture = CaptureNone })
	bad := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(500)
		_, _ = w.Write([]byte(`sensitive detail`))
	})
	serve(t, s, bad, httptest.NewRequest("GET", "/x", nil))

	deadline := time.Now().Add(3 * time.Second)
	for {
		if n, _ := s.WriterStats(); n >= 1 {
			break
		}
		require.False(t, time.Now().After(deadline))
		time.Sleep(2 * time.Millisecond)
	}
	list, err := s.List(Filter{})
	require.NoError(t, err)
	require.Len(t, list, 1)
	assert.False(t, list[0].BodyStored)
}

// A multi-megabyte response must not become a multi-megabyte row.
func TestMiddleware_BodyIsCappedAndFlagged(t *testing.T) {
	const cap = 1024
	s := newTestStore(t, func(c *Config) {
		c.Capture = CaptureAll
		c.MaxBodyBytes = cap
	})

	big := strings.Repeat("A", 50*1024)
	h := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(big))
	})
	rec := serve(t, s, h, httptest.NewRequest("GET", "/big", nil))
	assert.Len(t, rec.Body.String(), len(big), "the client still gets the whole response")

	deadline := time.Now().Add(3 * time.Second)
	for {
		if n, _ := s.WriterStats(); n >= 1 {
			break
		}
		require.False(t, time.Now().After(deadline))
		time.Sleep(2 * time.Millisecond)
	}

	list, err := s.List(Filter{})
	require.NoError(t, err)
	require.Len(t, list, 1)
	assert.True(t, list[0].Truncated, "truncation must be flagged")
	assert.Equal(t, int64(len(big)), list[0].RespBytes, "true size still reported")

	full, err := s.Get(list[0].ID)
	require.NoError(t, err)
	assert.Len(t, full.Body, cap, "stored body must respect the cap")
}

func TestMiddleware_DefaultStatusWhenHandlerNeverSetsIt(t *testing.T) {
	s := newTestStore(t)
	h := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("implicit 200"))
	})
	serve(t, s, h, httptest.NewRequest("GET", "/implicit", nil))

	deadline := time.Now().Add(3 * time.Second)
	for {
		if n, _ := s.WriterStats(); n >= 1 {
			break
		}
		require.False(t, time.Now().After(deadline))
		time.Sleep(2 * time.Millisecond)
	}
	list, err := s.List(Filter{})
	require.NoError(t, err)
	require.Len(t, list, 1)
	assert.Equal(t, 200, list[0].Status)
}

func TestRedactQuery(t *testing.T) {
	for _, tc := range []struct{ in, want string }{
		{"", ""},
		{"username=someone", "username=someone"},
		{"secret=abc123", "secret=REDACTED"},
		{"username=x&token=zzz", "username=x&token=REDACTED"},
		{"API_KEY=zzz", "API_KEY=REDACTED"},
		{"flag", "flag"},
	} {
		assert.Equal(t, tc.want, redactQuery(tc.in), "input %q", tc.in)
	}
}

func TestRecord_DropsRatherThanBlockingWhenQueueFull(t *testing.T) {
	// Queue of 1 with the writer deliberately not keeping up: the point is
	// that Record returns immediately instead of blocking the request.
	s := newTestStore(t, func(c *Config) { c.QueueSize = 1 })

	done := make(chan struct{})
	go func() {
		for i := 0; i < 5000; i++ {
			s.Record(entryAt(time.Now(), 200, fmt.Sprintf("/p%d", i)))
		}
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("Record blocked — logging must never stall the request path")
	}

	written, dropped := s.WriterStats()
	assert.Positive(t, written+dropped)
}

func TestParseCaptureMode(t *testing.T) {
	assert.Equal(t, CaptureErrors, ParseCaptureMode(""), "empty falls back to the safe default")
	assert.Equal(t, CaptureNone, ParseCaptureMode("none"))
	assert.Equal(t, CaptureAll, ParseCaptureMode("all"))
	assert.Equal(t, CaptureErrors, ParseCaptureMode("nonsense"), "unknown falls back, does not panic")
}

// Timing is genuinely measured, not defaulted — verified with a handler slow
// enough to exceed the schema's millisecond granularity.
func TestMiddleware_RecordsDuration(t *testing.T) {
	s := newTestStore(t)
	h := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(15 * time.Millisecond)
		w.WriteHeader(200)
	})
	serve(t, s, h, httptest.NewRequest("GET", "/slow", nil))

	deadline := time.Now().Add(3 * time.Second)
	for {
		if n, _ := s.WriterStats(); n >= 1 {
			break
		}
		require.False(t, time.Now().After(deadline))
		time.Sleep(2 * time.Millisecond)
	}

	list, err := s.List(Filter{})
	require.NoError(t, err)
	require.Len(t, list, 1)
	assert.GreaterOrEqual(t, list[0].Duration, 10*time.Millisecond,
		"a 15ms handler must record a measurable duration")
}

// Close must flush whatever is still queued. This is what makes graceful
// shutdown meaningful: without it, a SIGTERM during a burst silently loses
// the entries still in flight.
func TestStore_Close_DrainsQueuedEntries(t *testing.T) {
	dir := t.TempDir()
	path := dir + "/reqlog.db"

	s, err := Open(Config{Path: path, QueueSize: 512})
	require.NoError(t, err)

	const n = 200
	for i := 0; i < n; i++ {
		s.Record(entryAt(time.Now(), 200, "/drain"))
	}
	// Close immediately, without waiting for the writer to catch up.
	require.NoError(t, s.Close())

	written, dropped := s.WriterStats()
	assert.Equal(t, int64(n), written+dropped,
		"every entry must be either written or explicitly counted as dropped")
	assert.Zero(t, dropped, "a queue of 512 should not drop 200 entries")

	// Reopen and confirm the rows really landed on disk.
	s2, err := Open(Config{Path: path})
	require.NoError(t, err)
	defer s2.Close()

	count, err := s2.Count(Filter{})
	require.NoError(t, err)
	assert.Equal(t, int64(n), count, "queued entries must survive Close")
}

// Reopening must see everything the previous process wrote — the WAL has to
// be readable by a fresh connection, not stranded.
func TestStore_DataSurvivesReopen(t *testing.T) {
	dir := t.TempDir()
	path := dir + "/reqlog.db"

	s1, err := Open(Config{Path: path})
	require.NoError(t, err)
	recordSync(t, s1, &Entry{
		At: time.Now(), Method: "GET", Path: "/persisted", Status: 418,
		Body: []byte(`{"teapot":true}`), BodyStored: true,
	})
	require.NoError(t, s1.Close())

	s2, err := Open(Config{Path: path})
	require.NoError(t, err)
	defer s2.Close()

	list, err := s2.List(Filter{})
	require.NoError(t, err)
	require.Len(t, list, 1)
	assert.Equal(t, "/persisted", list[0].Path)
	assert.Equal(t, 418, list[0].Status)

	full, err := s2.Get(list[0].ID)
	require.NoError(t, err)
	assert.Equal(t, `{"teapot":true}`, string(full.Body), "body must still decompress")
}

// ---- internal error notes ----------------------------------------------

// The whole point: a vague client body still yields a diagnosable entry.
func TestMiddleware_CapturesHandlerNote(t *testing.T) {
	s := newTestStore(t)
	h := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		Notef(r, "GetWebProfile(%s): %v", "someuser", errors.New("proxy timeout after 20s"))
		w.WriteHeader(500)
		_, _ = w.Write([]byte(`{"error":"system error"}`))
	})
	serve(t, s, h, httptest.NewRequest("GET", "/username?username=someuser", nil))
	waitWritten(t, s, 1)

	list, err := s.List(Filter{})
	require.NoError(t, err)
	require.Len(t, list, 1)
	assert.Contains(t, list[0].Err, "proxy timeout after 20s",
		"the internal cause must be recorded")
	assert.Contains(t, list[0].Err, "someuser")

	full, err := s.Get(list[0].ID)
	require.NoError(t, err)
	assert.Contains(t, string(full.Body), "system error",
		"the client body stays generic")
}

func TestNote_AccumulatesAndIsBounded(t *testing.T) {
	s := newTestStore(t)
	h := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		Note(r, errors.New("first failure"))
		Note(r, errors.New("second failure"))
		Note(r, nil)           // ignored
		Notef(r, "  %s  ", "") // blank after trimming, ignored
		for i := 0; i < 50; i++ {
			Notef(r, "retry %d", i)
		}
		w.WriteHeader(502)
	})
	serve(t, s, h, httptest.NewRequest("GET", "/x", nil))
	waitWritten(t, s, 1)

	list, err := s.List(Filter{})
	require.NoError(t, err)
	require.Len(t, list, 1)

	assert.Contains(t, list[0].Err, "first failure")
	assert.Contains(t, list[0].Err, "second failure")
	assert.LessOrEqual(t, strings.Count(list[0].Err, "|"), 20,
		"notes must be bounded so a retry loop cannot grow the row without limit")
}

// Call sites must not need to know whether logging is even enabled.
func TestNote_IsNoOpWithoutMiddleware(t *testing.T) {
	req := httptest.NewRequest("GET", "/x", nil)
	assert.NotPanics(t, func() {
		Note(req, errors.New("nowhere to go"))
		Notef(req, "also %s", "fine")
		Note(nil, errors.New("nil request"))
		Notef(nil, "nil request")
	}, "Note must be safe when the request never passed through Middleware")
}

func TestMiddleware_NoNoteLeavesErrEmpty(t *testing.T) {
	s := newTestStore(t)
	h := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(200) })
	serve(t, s, h, httptest.NewRequest("GET", "/clean", nil))
	waitWritten(t, s, 1)

	list, err := s.List(Filter{})
	require.NoError(t, err)
	require.Len(t, list, 1)
	assert.Empty(t, list[0].Err)
}

// ---- request-side capture ----------------------------------------------

// The API's shared secret must never reach the log the dashboard displays.
func TestMiddleware_RedactsSensitiveHeaders(t *testing.T) {
	s := newTestStore(t, func(c *Config) { c.Capture = CaptureAll })
	h := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(200) })

	req := httptest.NewRequest("GET", "/username?username=x", nil)
	req.Header.Set("X-RapidAPI-Proxy-Secret", "the-actual-shared-secret")
	req.Header.Set("Authorization", "Bearer super-secret-token")
	req.Header.Set("Cookie", "session=abc123")
	req.Header.Set("User-Agent", "probe/2.0")
	req.Header.Set("X-Rapidapi-User", "someuser")
	serve(t, s, h, req)
	waitWritten(t, s, 1)

	list, err := s.List(Filter{})
	require.NoError(t, err)
	full, err := s.Get(list[0].ID)
	require.NoError(t, err)

	for _, leak := range []string{"the-actual-shared-secret", "super-secret-token", "abc123"} {
		assert.NotContains(t, full.ReqHeaders, leak, "credential must not be stored")
	}
	// The key stays visible — knowing the header was sent is the useful part.
	assert.Contains(t, full.ReqHeaders, "X-Rapidapi-Proxy-Secret: REDACTED")
	assert.Contains(t, full.ReqHeaders, "Authorization: REDACTED")
	// Non-sensitive headers are kept in full.
	assert.Contains(t, full.ReqHeaders, "probe/2.0")
	assert.Contains(t, full.ReqHeaders, "someuser")
}

func TestMiddleware_CapturesRequestBody(t *testing.T) {
	s := newTestStore(t, func(c *Config) { c.Capture = CaptureAll })

	var seenByHandler string
	h := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		seenByHandler = string(b)
		w.WriteHeader(200)
	})

	payload := `{"username":"someone","opts":{"images":true}}`
	req := httptest.NewRequest("POST", "/username", strings.NewReader(payload))
	serve(t, s, h, req)
	waitWritten(t, s, 1)

	assert.Equal(t, payload, seenByHandler,
		"the handler must still receive the complete body after capture")

	list, err := s.List(Filter{})
	require.NoError(t, err)
	full, err := s.Get(list[0].ID)
	require.NoError(t, err)
	assert.True(t, full.ReqBodyStored)
	assert.Equal(t, payload, string(full.ReqBody))
	assert.False(t, full.ReqTruncated)
}

// Capture must not truncate what the handler sees, only what is stored.
func TestMiddleware_LargeRequestBodyReachesHandlerIntact(t *testing.T) {
	const cap = 64
	s := newTestStore(t, func(c *Config) {
		c.Capture = CaptureAll
		c.MaxBodyBytes = cap
	})

	payload := strings.Repeat("x", 4096)
	var got string
	h := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		got = string(b)
		w.WriteHeader(200)
	})
	serve(t, s, h, httptest.NewRequest("POST", "/big", strings.NewReader(payload)))
	waitWritten(t, s, 1)

	assert.Equal(t, payload, got, "handler must get every byte despite the storage cap")

	list, err := s.List(Filter{})
	require.NoError(t, err)
	full, err := s.Get(list[0].ID)
	require.NoError(t, err)
	assert.Len(t, full.ReqBody, cap, "stored request body respects the cap")
	assert.True(t, full.ReqTruncated, "and says it was truncated")
}

func TestMiddleware_GetRequestHasNoBodyCapture(t *testing.T) {
	s := newTestStore(t, func(c *Config) { c.Capture = CaptureAll })
	h := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(200) })
	// httptest.NewRequest sets no headers of its own, so set one — a real
	// request always carries at least a User-Agent.
	req := httptest.NewRequest("GET", "/username?username=x", nil)
	req.Header.Set("User-Agent", "probe/3.0")
	serve(t, s, h, req)
	waitWritten(t, s, 1)

	list, err := s.List(Filter{})
	require.NoError(t, err)
	full, err := s.Get(list[0].ID)
	require.NoError(t, err)
	assert.False(t, full.ReqBodyStored, "a GET carries no body to capture")
	assert.Contains(t, full.ReqHeaders, "probe/3.0", "headers are still recorded")
}

func TestMiddleware_CaptureNone_SkipsRequestSideEntirely(t *testing.T) {
	s := newTestStore(t, func(c *Config) { c.Capture = CaptureNone })
	h := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(500) })
	req := httptest.NewRequest("POST", "/x", strings.NewReader(`{"a":1}`))
	req.Header.Set("User-Agent", "probe")
	serve(t, s, h, req)
	waitWritten(t, s, 1)

	list, err := s.List(Filter{})
	require.NoError(t, err)
	full, err := s.Get(list[0].ID)
	require.NoError(t, err)
	assert.Empty(t, full.ReqHeaders, "capture=none stores no headers")
	assert.False(t, full.ReqBodyStored)
}

// A database written by the previous build must gain the new columns rather
// than fail to open.
func TestStore_MigratesOlderSchemaInPlace(t *testing.T) {
	dir := t.TempDir()
	path := dir + "/old.db"

	// Build a table shaped like the first release: no request-side columns.
	raw, err := sql.Open("sqlite", path)
	require.NoError(t, err)
	_, err = raw.Exec(`CREATE TABLE requests (
		id INTEGER PRIMARY KEY AUTOINCREMENT, ts INTEGER NOT NULL,
		method TEXT NOT NULL, path TEXT NOT NULL, query TEXT NOT NULL DEFAULT '',
		status INTEGER NOT NULL, dur_ms INTEGER NOT NULL,
		req_bytes INTEGER NOT NULL DEFAULT 0, resp_bytes INTEGER NOT NULL DEFAULT 0,
		ip TEXT NOT NULL DEFAULT '', user_agent TEXT NOT NULL DEFAULT '',
		err TEXT NOT NULL DEFAULT '', truncated INTEGER NOT NULL DEFAULT 0,
		body_gz BLOB)`)
	require.NoError(t, err)
	_, err = raw.Exec(`INSERT INTO requests (ts, method, path, status, dur_ms)
	                   VALUES (?, 'GET', '/legacy', 200, 5)`, time.Now().UnixMilli())
	require.NoError(t, err)
	require.NoError(t, raw.Close())

	// Opening must migrate, not error, and the old row must survive.
	s, err := Open(Config{Path: path})
	require.NoError(t, err, "an older database must be upgraded in place")
	defer s.Close()

	list, err := s.List(Filter{})
	require.NoError(t, err)
	require.Len(t, list, 1)
	assert.Equal(t, "/legacy", list[0].Path)

	// And the new columns work.
	recordSync(t, s, &Entry{At: time.Now(), Method: "POST", Path: "/new", Status: 200,
		ReqHeaders: "X-Test: yes", ReqBody: []byte(`{"a":1}`), ReqBodyStored: true})
	got, err := s.List(Filter{Path: "/new"})
	require.NoError(t, err)
	require.Len(t, got, 1)
	full, err := s.Get(got[0].ID)
	require.NoError(t, err)
	assert.Equal(t, "X-Test: yes", full.ReqHeaders)
	assert.Equal(t, `{"a":1}`, string(full.ReqBody))
}

// ---- time zone ----------------------------------------------------------

func TestStore_DefaultsToJakarta(t *testing.T) {
	s := newTestStore(t)
	name, offset := time.Now().In(s.Location()).Zone()
	assert.Equal(t, "WIB", name)
	assert.Equal(t, 7*3600, offset, "Asia/Jakarta is UTC+7 year-round")
}

func TestStore_TimeZoneIsConfigurable(t *testing.T) {
	s := newTestStore(t, func(c *Config) { c.TimeZone = "UTC" })
	_, offset := time.Now().In(s.Location()).Zone()
	assert.Zero(t, offset)
}

// An unknown zone name must degrade to UTC, not stop the service.
func TestStore_UnknownTimeZoneFallsBackToUTC(t *testing.T) {
	s := newTestStore(t, func(c *Config) { c.TimeZone = "Mars/Olympus_Mons" })
	require.NotNil(t, s.Location())
	_, offset := time.Now().In(s.Location()).Zone()
	assert.Zero(t, offset, "an unresolvable zone should fall back to UTC")
}

// The bug this fixes: grouping by UTC put an early-morning Jakarta request on
// the previous day.
func TestStore_PerDay_BucketsByConfiguredZone(t *testing.T) {
	jakarta, err := time.LoadLocation("Asia/Jakarta")
	require.NoError(t, err)

	// 06:00 on the 20th in Jakarta is 23:00 on the 19th UTC.
	early := time.Date(2026, 8, 20, 6, 0, 0, 0, jakarta)
	require.Equal(t, 19, early.UTC().Day(), "precondition: this instant is the 19th in UTC")

	t.Run("jakarta groups it on the 20th", func(t *testing.T) {
		s := newTestStore(t, func(c *Config) { c.TimeZone = "Asia/Jakarta" })
		recordSync(t, s, entryAt(early, 200, "/x"))

		days, err := s.PerDay(Filter{})
		require.NoError(t, err)
		require.Len(t, days, 1)
		assert.Equal(t, "2026-08-20", days[0].Day,
			"a 06:00 Jakarta request belongs to the Jakarta day")
	})

	t.Run("utc groups it on the 19th", func(t *testing.T) {
		s := newTestStore(t, func(c *Config) { c.TimeZone = "UTC" })
		recordSync(t, s, entryAt(early, 200, "/x"))

		days, err := s.PerDay(Filter{})
		require.NoError(t, err)
		require.Len(t, days, 1)
		assert.Equal(t, "2026-08-19", days[0].Day,
			"the same instant is the previous day in UTC — which is what made the old behaviour wrong")
	})
}

// Storage stays UTC regardless of display zone, so switching zones
// reinterprets history rather than corrupting it.
func TestStore_StorageIsZoneIndependent(t *testing.T) {
	dir := t.TempDir()
	path := dir + "/tz.db"
	instant := time.Date(2026, 8, 20, 6, 0, 0, 0, time.UTC)

	s1, err := Open(Config{Path: path, TimeZone: "Asia/Jakarta"})
	require.NoError(t, err)
	recordSync(t, s1, entryAt(instant, 200, "/x"))
	first, err := s1.List(Filter{})
	require.NoError(t, err)
	require.NoError(t, s1.Close())

	s2, err := Open(Config{Path: path, TimeZone: "UTC"})
	require.NoError(t, err)
	defer s2.Close()
	second, err := s2.List(Filter{})
	require.NoError(t, err)

	require.Len(t, first, 1)
	require.Len(t, second, 1)
	assert.True(t, first[0].At.Equal(second[0].At),
		"the stored instant must not depend on the display zone")
}

// ---- unlimited body capture --------------------------------------------

func TestConfig_ZeroMaxBodyUsesDefaultNotUnlimited(t *testing.T) {
	// The zero-value Config must stay bounded: an accidental unbounded
	// default would let one response fill the disk.
	s := newTestStore(t)
	assert.Equal(t, DefaultMaxBodyBytes, s.MaxBodyBytes(),
		"an unset cap must fall back to the default, never to unlimited")
}

func TestMiddleware_Unlimited_StoresResponseWhole(t *testing.T) {
	s := newTestStore(t, func(c *Config) {
		c.Capture = CaptureAll
		c.MaxBodyBytes = Unlimited
	})

	// Comfortably past the old 32 KB cap, and past the 63.5 KB that was
	// being truncated in practice.
	big := strings.Repeat("abcdefghij", 20_000) // 200 KB
	h := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(big))
	})
	serve(t, s, h, httptest.NewRequest("GET", "/big", nil))
	waitWritten(t, s, 1)

	list, err := s.List(Filter{})
	require.NoError(t, err)
	require.Len(t, list, 1)
	assert.False(t, list[0].Truncated, "nothing should be flagged truncated")
	assert.Equal(t, int64(len(big)), list[0].RespBytes)

	full, err := s.Get(list[0].ID)
	require.NoError(t, err)
	assert.Equal(t, len(big), len(full.Body), "the whole body must be stored")
	assert.Equal(t, big, string(full.Body), "and survive the gzip round trip byte-for-byte")
}

func TestMiddleware_Unlimited_StoresRequestBodyWhole(t *testing.T) {
	s := newTestStore(t, func(c *Config) {
		c.Capture = CaptureAll
		c.MaxBodyBytes = Unlimited
	})

	payload := strings.Repeat("payload-", 20_000) // 160 KB
	var seen string
	h := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		seen = string(b)
		w.WriteHeader(200)
	})
	serve(t, s, h, httptest.NewRequest("POST", "/big", strings.NewReader(payload)))
	waitWritten(t, s, 1)

	assert.Equal(t, payload, seen, "the handler still receives the whole body")

	list, err := s.List(Filter{})
	require.NoError(t, err)
	full, err := s.Get(list[0].ID)
	require.NoError(t, err)
	assert.False(t, full.ReqTruncated)
	assert.Equal(t, payload, string(full.ReqBody))
}

// Unlimited must not defeat the gzip storage — a large repetitive body should
// still occupy far less on disk than in memory.
func TestUnlimited_StillCompresses(t *testing.T) {
	body := []byte(strings.Repeat(`{"field":"value","n":12345}`, 5000)) // ~135 KB
	gz, err := gzipBytes(body)
	require.NoError(t, err)
	assert.Less(t, len(gz), len(body)/10,
		"repetitive JSON should compress by well over 10x: %d from %d", len(gz), len(body))

	back, err := gunzipBytes(gz)
	require.NoError(t, err)
	assert.Equal(t, body, back)
}

// ---- caller identity ----------------------------------------------------

func TestMiddleware_RecordsRapidAPICaller(t *testing.T) {
	s := newTestStore(t, func(c *Config) { c.Capture = CaptureAll })
	h := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(200) })

	req := httptest.NewRequest("GET", "/username?username=x", nil)
	req.Header.Set("X-RapidAPI-User", "xT9akLm2")
	req.Header.Set("X-RapidAPI-Subscription", "MEGA")
	serve(t, s, h, req)
	waitWritten(t, s, 1)

	list, err := s.List(Filter{})
	require.NoError(t, err)
	require.Len(t, list, 1)
	assert.Equal(t, "xT9akLm2", list[0].Caller)
	assert.Equal(t, "MEGA", list[0].Plan)
}

// The gateway still emits the legacy Mashape names; either must work.
func TestMiddleware_FallsBackToMashapeHeaders(t *testing.T) {
	s := newTestStore(t, func(c *Config) { c.Capture = CaptureAll })
	h := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(200) })

	req := httptest.NewRequest("GET", "/x", nil)
	req.Header.Set("X-Mashape-User", "legacyUser")
	req.Header.Set("X-Mashape-Subscription", "PRO")
	serve(t, s, h, req)
	waitWritten(t, s, 1)

	list, err := s.List(Filter{})
	require.NoError(t, err)
	assert.Equal(t, "legacyUser", list[0].Caller)
	assert.Equal(t, "PRO", list[0].Plan)
}

func TestStore_FilterByCaller(t *testing.T) {
	s := newTestStore(t)
	now := time.Now()
	recordSync(t, s,
		&Entry{At: now, Method: "GET", Path: "/a", Status: 200, Caller: "alice", Plan: "MEGA"},
		&Entry{At: now, Method: "GET", Path: "/b", Status: 200, Caller: "alice", Plan: "MEGA"},
		&Entry{At: now, Method: "GET", Path: "/c", Status: 500, Caller: "bob", Plan: "BASIC"},
		&Entry{At: now, Method: "GET", Path: "/d", Status: 200}, // no caller
	)

	got, err := s.List(Filter{Caller: "alice"})
	require.NoError(t, err)
	assert.Len(t, got, 2)

	n, err := s.Count(Filter{Caller: "bob"})
	require.NoError(t, err)
	assert.Equal(t, int64(1), n)

	names, err := s.DistinctCallers()
	require.NoError(t, err)
	assert.Equal(t, []string{"alice", "bob"}, names, "blank callers are not listed")
}

func TestStore_TopCallers(t *testing.T) {
	s := newTestStore(t)
	now := time.Now()
	recordSync(t, s,
		&Entry{At: now, Method: "GET", Path: "/a", Status: 200, Caller: "heavy", Plan: "MEGA", Duration: 100 * time.Millisecond},
		&Entry{At: now, Method: "GET", Path: "/a", Status: 200, Caller: "heavy", Plan: "MEGA", Duration: 300 * time.Millisecond},
		&Entry{At: now, Method: "GET", Path: "/a", Status: 502, Caller: "heavy", Plan: "MEGA", Duration: 200 * time.Millisecond},
		&Entry{At: now, Method: "GET", Path: "/a", Status: 200, Caller: "light", Plan: "BASIC", Duration: 50 * time.Millisecond},
		&Entry{At: now, Method: "GET", Path: "/a", Status: 200, Duration: 10 * time.Millisecond}, // gateway bypassed
	)

	got, err := s.TopCallers(Filter{}, 10)
	require.NoError(t, err)
	require.Len(t, got, 3)

	assert.Equal(t, "heavy", got[0].Caller, "busiest first")
	assert.Equal(t, int64(3), got[0].Count)
	assert.Equal(t, int64(1), got[0].ErrCount)
	assert.Equal(t, "MEGA", got[0].Plan)
	assert.InDelta(t, 200.0, got[0].AvgMS, 0.1)

	// A direct hit that skipped the gateway must be visible, not dropped.
	var unknown *CallerStat
	for i := range got {
		if got[i].Caller == "(unknown)" {
			unknown = &got[i]
		}
	}
	require.NotNil(t, unknown, "requests without a caller must still be reported")
	assert.Equal(t, int64(1), unknown.Count)
}

// ---- redaction completeness --------------------------------------------

// The gap this closes: the shared secret arrives under two names, and only
// one was on the list, so a working credential was being stored.
func TestIsSensitiveHeader_CoversAliases(t *testing.T) {
	for _, name := range []string{
		"X-RapidAPI-Proxy-Secret",
		"X-Mashape-Proxy-Secret", // the alias that leaked
		"X-RapidAPI-Key",
		"X-Mashape-Key",
		"Authorization",
		"Proxy-Authorization",
		"Cookie",
		"Set-Cookie",
		"X-Api-Key",
		"X-Auth-Token",
		"X-Session-Id",
		"X-Some-Future-Secret-Header",
		"key",
	} {
		assert.True(t, isSensitiveHeader(name), "%s must be redacted", name)
	}

	for _, name := range []string{
		"X-RapidAPI-User",         // an identifier, and the point of the feature
		"X-RapidAPI-Subscription", // plan tier
		"X-RapidAPI-Host",
		"X-RapidAPI-Version",
		"X-RapidAPI-Request-Id",
		"User-Agent",
		"X-Forwarded-For",
		"Content-Type",
		"X-Monkey-Business", // contains "key" but is not a key
	} {
		assert.False(t, isSensitiveHeader(name), "%s must NOT be redacted", name)
	}
}

func TestMiddleware_RedactsBothSecretAliases(t *testing.T) {
	s := newTestStore(t, func(c *Config) { c.Capture = CaptureAll })
	h := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(200) })

	const secret = "f92b54f0-DEAD-BEEF-0000-000000000000"
	req := httptest.NewRequest("GET", "/x", nil)
	req.Header.Set("X-RapidAPI-Proxy-Secret", secret)
	req.Header.Set("X-Mashape-Proxy-Secret", secret)
	req.Header.Set("X-RapidAPI-Key", "subscriber-key-value")
	req.Header.Set("X-RapidAPI-User", "visibleUser")
	serve(t, s, h, req)
	waitWritten(t, s, 1)

	list, err := s.List(Filter{})
	require.NoError(t, err)
	full, err := s.Get(list[0].ID)
	require.NoError(t, err)

	assert.NotContains(t, full.ReqHeaders, secret,
		"the shared secret must not be stored under either alias")
	assert.NotContains(t, full.ReqHeaders, "subscriber-key-value")
	assert.Equal(t, 3, strings.Count(full.ReqHeaders, "REDACTED"),
		"all three credential headers redacted")
	assert.Contains(t, full.ReqHeaders, "visibleUser",
		"the caller identity is not a secret and stays visible")
}
