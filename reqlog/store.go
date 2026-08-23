// Package reqlog records one row per HTTP request into SQLite, and serves a
// small dashboard over that data.
//
// SQLite rather than daily files because the two things this exists for —
// a sliding retention window and status/latency statistics — are a DELETE
// and a GROUP BY here, versus parsing and rotating files by hand.
//
// Response bodies are stored gzip-compressed in a BLOB column rather than
// the whole log being an archive: metadata stays queryable while the bulky
// part still compresses. See CaptureMode for why bodies are off by default
// for successful responses.
package reqlog

import (
	"bytes"
	"compress/gzip"
	"context"
	"database/sql"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"sync/atomic"
	"time"

	// Embedded zone database: LoadLocation("Asia/Jakarta") then works even on
	// a host without /usr/share/zoneinfo, which a scratch container lacks.
	_ "time/tzdata"

	_ "modernc.org/sqlite" // pure-Go driver: no CGO, so cross-compiling stays trivial
)

// CaptureMode decides when a response body is kept.
//
// The default is CaptureErrors, deliberately. Two endpoints
// (/username-with-base64-image, /get-post-with-base64-image) embed
// base64-encoded images, so a single successful response can run to
// megabytes — and gzip barely helps, because the underlying JPEG/PNG is
// already compressed. Those responses also carry personal data of the
// Instagram accounts being looked up, which is worth storing only when
// there is a reason to.
type CaptureMode string

// Unlimited, as MaxBodyBytes, stores bodies whole with no cap.
const Unlimited = -1

// DefaultMaxBodyBytes is the cap applied when none is configured.
const DefaultMaxBodyBytes = 32 << 10

const (
	CaptureNone   CaptureMode = "none"   // never store bodies
	CaptureErrors CaptureMode = "errors" // store only when status >= 400
	CaptureAll    CaptureMode = "all"    // store every body, still size-capped
)

func ParseCaptureMode(s string) CaptureMode {
	switch CaptureMode(s) {
	case CaptureNone, CaptureErrors, CaptureAll:
		return CaptureMode(s)
	case "":
		return CaptureErrors
	default:
		log.Printf("reqlog: unknown capture mode %q, falling back to %q", s, CaptureErrors)
		return CaptureErrors
	}
}

// Entry is one recorded request.
type Entry struct {
	ID         int64
	At         time.Time
	Method     string
	Path       string
	Query      string
	Status     int
	Duration   time.Duration
	ReqBytes   int64
	RespBytes  int64 // uncompressed size, even when the stored body is truncated
	IP         string
	UserAgent  string
	Err        string
	Body       []byte // uncompressed; nil when not captured
	BodyStored bool
	Truncated  bool

	// Request side. ReqHeaders is a redacted rendering; ReqBody is the
	// uncompressed request body, nil when absent or not captured.
	ReqHeaders    string
	ReqBody       []byte
	ReqBodyStored bool
	ReqTruncated  bool
}

// Config configures a Store.
type Config struct {
	// Path to the SQLite file. Parent directories are created.
	Path string
	// RetentionDays is the initial sliding window. Zero disables pruning.
	// The dashboard can change this; the stored value then wins on restart.
	RetentionDays int
	// Capture decides when response bodies are kept.
	Capture CaptureMode
	// MaxBodyBytes caps a stored body. Larger bodies are truncated and
	// flagged; RespBytes still reports the true size.
	//
	// Zero means "use the default". Use Unlimited to store bodies whole —
	// note that removes the only bound on how large a single row can get,
	// and /username-with-base64-image can return megabytes.
	MaxBodyBytes int
	// QueueSize bounds the async write buffer.
	QueueSize int
	// TimeZone names the zone used to display timestamps and to decide day
	// boundaries in the per-day statistics. Defaults to Asia/Jakarta.
	//
	// This is a presentation choice only — rows are always stored as UTC
	// milliseconds, so changing it reinterprets history rather than
	// migrating it.
	TimeZone string
}

func (c *Config) applyDefaults() {
	if c.Path == "" {
		c.Path = "logs/reqlog.db"
	}
	if c.RetentionDays == 0 {
		c.RetentionDays = 14
	}
	if c.Capture == "" {
		c.Capture = CaptureErrors
	}
	// Zero is "unset" rather than "unlimited" so the zero-value Config keeps
	// a safe bound; Unlimited has to be asked for by name.
	if c.MaxBodyBytes == 0 {
		c.MaxBodyBytes = DefaultMaxBodyBytes
	}
	if c.QueueSize <= 0 {
		c.QueueSize = 1024
	}
	if c.TimeZone == "" {
		c.TimeZone = "Asia/Jakarta"
	}
}

// Store writes entries to SQLite from a single background goroutine.
//
// Writes are asynchronous and lossy under pressure: if the queue is full,
// the entry is dropped and a counter incremented. Request logging must
// never slow down or fail the request it is describing — a dropped log
// line is strictly better than a request that blocks behind disk I/O.
type Store struct {
	db  *sql.DB
	cfg Config
	loc *time.Location

	queue   chan *Entry
	done    chan struct{}
	dropped atomic.Int64
	written atomic.Int64
}

// Open opens (creating if needed) the database and starts the writer.
func Open(cfg Config) (*Store, error) {
	cfg.applyDefaults()

	if dir := filepath.Dir(cfg.Path); dir != "" && dir != "." {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return nil, fmt.Errorf("reqlog: create %s: %w", dir, err)
		}
	}

	// WAL keeps the dashboard's reads from blocking the writer. busy_timeout
	// covers the brief moments the two still contend.
	dsn := cfg.Path + "?_pragma=journal_mode(WAL)&_pragma=busy_timeout(5000)&_pragma=synchronous(NORMAL)"
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("reqlog: open %s: %w", cfg.Path, err)
	}
	// One writer goroutine, but the dashboard reads concurrently.
	db.SetMaxOpenConns(4)

	loc, err := time.LoadLocation(cfg.TimeZone)
	if err != nil {
		// Not fatal: fall back to UTC and say so, rather than refusing to
		// start over a display setting.
		log.Printf("reqlog: time zone %q could not be loaded (%v) — using UTC", cfg.TimeZone, err)
		loc = time.UTC
	}

	s := &Store{
		db:    db,
		cfg:   cfg,
		loc:   loc,
		queue: make(chan *Entry, cfg.QueueSize),
		done:  make(chan struct{}),
	}
	if err := s.migrate(); err != nil {
		db.Close()
		return nil, err
	}
	// A retention value set through the dashboard outlives the .env default.
	if v, ok := s.settingInt("retention_days"); ok {
		s.cfg.RetentionDays = v
	} else {
		_ = s.SetRetentionDays(cfg.RetentionDays)
	}

	go s.writeLoop()
	return s, nil
}

func (s *Store) migrate() error {
	const schema = `
CREATE TABLE IF NOT EXISTS requests (
	id          INTEGER PRIMARY KEY AUTOINCREMENT,
	ts          INTEGER NOT NULL,   -- unix millis
	method      TEXT    NOT NULL,
	path        TEXT    NOT NULL,
	query       TEXT    NOT NULL DEFAULT '',
	status      INTEGER NOT NULL,
	dur_ms      INTEGER NOT NULL,
	req_bytes   INTEGER NOT NULL DEFAULT 0,
	resp_bytes  INTEGER NOT NULL DEFAULT 0,
	ip          TEXT    NOT NULL DEFAULT '',
	user_agent  TEXT    NOT NULL DEFAULT '',
	err         TEXT    NOT NULL DEFAULT '',
	truncated   INTEGER NOT NULL DEFAULT 0,
	body_gz     BLOB,               -- NULL when not captured
	req_headers TEXT    NOT NULL DEFAULT '',
	req_body_gz BLOB,               -- NULL when absent or not captured
	req_truncated INTEGER NOT NULL DEFAULT 0
);
-- ts leads every dashboard query (recent-first listing, range filters, and
-- the retention DELETE), so it carries the index.
CREATE INDEX IF NOT EXISTS idx_requests_ts     ON requests(ts DESC);
CREATE INDEX IF NOT EXISTS idx_requests_status ON requests(status, ts DESC);
CREATE INDEX IF NOT EXISTS idx_requests_path   ON requests(path, ts DESC);

CREATE TABLE IF NOT EXISTS settings (
	key   TEXT PRIMARY KEY,
	value TEXT NOT NULL
);`
	if _, err := s.db.Exec(schema); err != nil {
		return fmt.Errorf("reqlog: migrate: %w", err)
	}

	// Columns added after the first release. SQLite has no
	// "ADD COLUMN IF NOT EXISTS", so check first — a database created by an
	// older build is upgraded in place rather than rebuilt.
	existing, err := s.columns("requests")
	if err != nil {
		return err
	}
	for _, add := range []struct{ name, ddl string }{
		{"req_headers", `ALTER TABLE requests ADD COLUMN req_headers TEXT NOT NULL DEFAULT ''`},
		{"req_body_gz", `ALTER TABLE requests ADD COLUMN req_body_gz BLOB`},
		{"req_truncated", `ALTER TABLE requests ADD COLUMN req_truncated INTEGER NOT NULL DEFAULT 0`},
	} {
		if existing[add.name] {
			continue
		}
		if _, err := s.db.Exec(add.ddl); err != nil {
			return fmt.Errorf("reqlog: add column %s: %w", add.name, err)
		}
	}
	return nil
}

func (s *Store) columns(table string) (map[string]bool, error) {
	rows, err := s.db.Query(`SELECT name FROM pragma_table_info(?)`, table)
	if err != nil {
		return nil, fmt.Errorf("reqlog: inspect %s: %w", table, err)
	}
	defer rows.Close()
	out := map[string]bool{}
	for rows.Next() {
		var n string
		if err := rows.Scan(&n); err != nil {
			return nil, err
		}
		out[n] = true
	}
	return out, rows.Err()
}

// Record queues an entry. Never blocks; drops when the queue is full.
func (s *Store) Record(e *Entry) {
	select {
	case s.queue <- e:
	default:
		s.dropped.Add(1)
	}
}

// Stats about the writer itself, surfaced on the dashboard so a saturated
// queue is visible rather than silently losing rows.
func (s *Store) WriterStats() (written, dropped int64) {
	return s.written.Load(), s.dropped.Load()
}

func (s *Store) writeLoop() {
	defer close(s.done)
	for e := range s.queue {
		if err := s.insert(e); err != nil {
			// Logging the failure to log is the end of the line — just say so
			// once per occurrence and carry on.
			log.Printf("reqlog: insert: %v", err)
			continue
		}
		s.written.Add(1)
	}
}

func (s *Store) insert(e *Entry) error {
	var bodyGz any
	if e.BodyStored && len(e.Body) > 0 {
		gz, err := gzipBytes(e.Body)
		if err != nil {
			return fmt.Errorf("gzip body: %w", err)
		}
		bodyGz = gz
	}
	var reqBodyGz any
	if e.ReqBodyStored && len(e.ReqBody) > 0 {
		gz, err := gzipBytes(e.ReqBody)
		if err != nil {
			return fmt.Errorf("gzip request body: %w", err)
		}
		reqBodyGz = gz
	}

	_, err := s.db.Exec(`
INSERT INTO requests (ts, method, path, query, status, dur_ms, req_bytes,
                      resp_bytes, ip, user_agent, err, truncated, body_gz,
                      req_headers, req_body_gz, req_truncated)
VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)`,
		e.At.UnixMilli(), e.Method, e.Path, e.Query, e.Status,
		e.Duration.Milliseconds(), e.ReqBytes, e.RespBytes,
		e.IP, e.UserAgent, e.Err, boolToInt(e.Truncated), bodyGz,
		e.ReqHeaders, reqBodyGz, boolToInt(e.ReqTruncated))
	return err
}

// Close drains the queue and closes the database.
func (s *Store) Close() error {
	close(s.queue)
	<-s.done
	return s.db.Close()
}

// ---- retention ----------------------------------------------------------

// MaxBodyBytes is the active per-body storage cap, or Unlimited.
func (s *Store) MaxBodyBytes() int { return s.cfg.MaxBodyBytes }

// Location is the zone timestamps are displayed in and days are bucketed by.
func (s *Store) Location() *time.Location { return s.loc }

// RetentionDays is the active sliding window in days.
func (s *Store) RetentionDays() int { return s.cfg.RetentionDays }

// SetRetentionDays persists a new window. Zero disables pruning.
func (s *Store) SetRetentionDays(days int) error {
	if days < 0 {
		return fmt.Errorf("retention days cannot be negative, got %d", days)
	}
	if _, err := s.db.Exec(
		`INSERT INTO settings (key, value) VALUES ('retention_days', ?)
		 ON CONFLICT(key) DO UPDATE SET value = excluded.value`,
		strconv.Itoa(days)); err != nil {
		return err
	}
	s.cfg.RetentionDays = days
	return nil
}

func (s *Store) settingInt(key string) (int, bool) {
	var raw string
	if err := s.db.QueryRow(`SELECT value FROM settings WHERE key = ?`, key).Scan(&raw); err != nil {
		return 0, false
	}
	v, err := strconv.Atoi(raw)
	if err != nil {
		return 0, false
	}
	return v, true
}

// Prune deletes rows outside the retention window and reports how many
// went. A zero window keeps everything.
func (s *Store) Prune(now time.Time) (int64, error) {
	if s.cfg.RetentionDays <= 0 {
		return 0, nil
	}
	cutoff := now.AddDate(0, 0, -s.cfg.RetentionDays).UnixMilli()
	res, err := s.db.Exec(`DELETE FROM requests WHERE ts < ?`, cutoff)
	if err != nil {
		return 0, fmt.Errorf("reqlog: prune: %w", err)
	}
	n, _ := res.RowsAffected()
	return n, nil
}

// RunRetention prunes now and then daily until ctx is cancelled.
//
// SQLite does not return freed pages to the filesystem on DELETE, so a
// VACUUM follows a prune that actually removed rows — otherwise the file
// only ever grows, which defeats the point of a retention window.
func (s *Store) RunRetention(ctx context.Context, every time.Duration) {
	if every <= 0 {
		every = 24 * time.Hour
	}
	prune := func() {
		n, err := s.Prune(time.Now())
		if err != nil {
			log.Printf("reqlog: retention: %v", err)
			return
		}
		if n > 0 {
			log.Printf("reqlog: retention removed %d row(s) older than %d day(s)", n, s.cfg.RetentionDays)
			if _, err := s.db.Exec(`VACUUM`); err != nil {
				log.Printf("reqlog: vacuum: %v", err)
			}
		}
	}
	prune()

	t := time.NewTicker(every)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			prune()
		}
	}
}

// ---- helpers ------------------------------------------------------------

func gzipBytes(b []byte) ([]byte, error) {
	var buf bytes.Buffer
	zw := gzip.NewWriter(&buf)
	if _, err := zw.Write(b); err != nil {
		return nil, err
	}
	if err := zw.Close(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func gunzipBytes(b []byte) ([]byte, error) {
	zr, err := gzip.NewReader(bytes.NewReader(b))
	if err != nil {
		return nil, err
	}
	defer zr.Close()
	return io.ReadAll(zr)
}

func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}
