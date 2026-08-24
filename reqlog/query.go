package reqlog

import (
	"database/sql"
	"fmt"
	"os"
	"strings"
	"time"
)

// Filter narrows a listing. Zero values mean "no constraint".
type Filter struct {
	From       time.Time
	To         time.Time
	Status     int    // exact status, e.g. 404
	StatusFrom int    // inclusive lower bound, e.g. 400 for "all errors"
	StatusTo   int    // inclusive upper bound
	Path       string // exact match
	Caller     string // exact match on the RapidAPI subscriber
	Search     string // substring across path, query and error
	Limit      int
	Offset     int
}

func (f Filter) where() (string, []any) {
	var cond []string
	var args []any

	if !f.From.IsZero() {
		cond = append(cond, "ts >= ?")
		args = append(args, f.From.UnixMilli())
	}
	if !f.To.IsZero() {
		cond = append(cond, "ts <= ?")
		args = append(args, f.To.UnixMilli())
	}
	if f.Status > 0 {
		cond = append(cond, "status = ?")
		args = append(args, f.Status)
	}
	if f.StatusFrom > 0 {
		cond = append(cond, "status >= ?")
		args = append(args, f.StatusFrom)
	}
	if f.StatusTo > 0 {
		cond = append(cond, "status <= ?")
		args = append(args, f.StatusTo)
	}
	if f.Path != "" {
		cond = append(cond, "path = ?")
		args = append(args, f.Path)
	}
	if f.Caller != "" {
		cond = append(cond, "caller = ?")
		args = append(args, f.Caller)
	}
	if f.Search != "" {
		// Parameterised LIKE — the term is a value, never concatenated into SQL.
		cond = append(cond, "(path LIKE ? OR query LIKE ? OR err LIKE ?)")
		like := "%" + f.Search + "%"
		args = append(args, like, like, like)
	}
	if len(cond) == 0 {
		return "", nil
	}
	return " WHERE " + strings.Join(cond, " AND "), args
}

// List returns entries newest-first. Bodies are not loaded — use Get for that,
// so a listing never pulls megabytes of BLOB it will not show.
func (s *Store) List(f Filter) ([]Entry, error) {
	if f.Limit <= 0 || f.Limit > 500 {
		f.Limit = 100
	}
	where, args := f.where()
	q := `SELECT id, ts, method, path, query, status, dur_ms, req_bytes,
	             resp_bytes, ip, user_agent, err, truncated,
	             body_gz IS NOT NULL, caller, plan
	      FROM requests` + where + ` ORDER BY ts DESC, id DESC LIMIT ? OFFSET ?`
	args = append(args, f.Limit, f.Offset)

	rows, err := s.db.Query(q, args...)
	if err != nil {
		return nil, fmt.Errorf("reqlog: list: %w", err)
	}
	defer rows.Close()

	var out []Entry
	for rows.Next() {
		var e Entry
		var ts, durMS int64
		if err := rows.Scan(&e.ID, &ts, &e.Method, &e.Path, &e.Query, &e.Status,
			&durMS, &e.ReqBytes, &e.RespBytes, &e.IP, &e.UserAgent, &e.Err,
			&e.Truncated, &e.BodyStored, &e.Caller, &e.Plan); err != nil {
			return nil, err
		}
		e.At = time.UnixMilli(ts)
		e.Duration = time.Duration(durMS) * time.Millisecond
		out = append(out, e)
	}
	return out, rows.Err()
}

// Count returns how many entries match, for pagination.
func (s *Store) Count(f Filter) (int64, error) {
	where, args := f.where()
	var n int64
	err := s.db.QueryRow(`SELECT COUNT(*) FROM requests`+where, args...).Scan(&n)
	if err != nil {
		return 0, fmt.Errorf("reqlog: count: %w", err)
	}
	return n, nil
}

// Get returns one entry with its body decompressed.
func (s *Store) Get(id int64) (*Entry, error) {
	var e Entry
	var ts, durMS int64
	var bodyGz, reqBodyGz []byte

	err := s.db.QueryRow(`
SELECT id, ts, method, path, query, status, dur_ms, req_bytes, resp_bytes,
       ip, user_agent, err, truncated, body_gz,
       req_headers, req_body_gz, req_truncated, caller, plan
FROM requests WHERE id = ?`, id).
		Scan(&e.ID, &ts, &e.Method, &e.Path, &e.Query, &e.Status, &durMS,
			&e.ReqBytes, &e.RespBytes, &e.IP, &e.UserAgent, &e.Err,
			&e.Truncated, &bodyGz,
			&e.ReqHeaders, &reqBodyGz, &e.ReqTruncated, &e.Caller, &e.Plan)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("reqlog: get %d: %w", id, err)
	}

	e.At = time.UnixMilli(ts)
	e.Duration = time.Duration(durMS) * time.Millisecond
	if len(bodyGz) > 0 {
		body, err := gunzipBytes(bodyGz)
		if err != nil {
			// A corrupt blob should not hide the rest of the row.
			e.Err = strings.TrimSpace(e.Err + " [stored body could not be decompressed: " + err.Error() + "]")
		} else {
			e.Body = body
			e.BodyStored = true
		}
	}
	if len(reqBodyGz) > 0 {
		body, err := gunzipBytes(reqBodyGz)
		if err != nil {
			e.Err = strings.TrimSpace(e.Err + " [stored request body could not be decompressed: " + err.Error() + "]")
		} else {
			e.ReqBody = body
			e.ReqBodyStored = true
		}
	}
	return &e, nil
}

// ---- statistics ---------------------------------------------------------

// StatusCount is one bar of the status-code breakdown.
type StatusCount struct {
	Status int
	Count  int64
}

// PathStat summarises traffic to one route.
type PathStat struct {
	Path     string
	Count    int64
	AvgMS    float64
	MaxMS    int64
	ErrCount int64
}

// CallerStat summarises one API consumer's usage.
type CallerStat struct {
	Caller   string
	Plan     string
	Count    int64
	ErrCount int64
	AvgMS    float64
	LastSeen time.Time
}

// DayCount is one point of the requests-per-day series.
type DayCount struct {
	Day    string // YYYY-MM-DD, UTC
	Count  int64
	Errors int64
}

// Summary is the dashboard's headline block.
type Summary struct {
	Total       int64
	Errors      int64
	ErrorRate   float64 // percent
	AvgMS       float64
	P95MS       int64
	OldestEntry time.Time
	NewestEntry time.Time
	DBBytes     int64
}

// Summary computes headline figures for the filtered range.
func (s *Store) Summary(f Filter) (Summary, error) {
	where, args := f.where()
	var out Summary
	var avg sql.NullFloat64
	var oldest, newest sql.NullInt64

	err := s.db.QueryRow(`
SELECT COUNT(*),
       COALESCE(SUM(CASE WHEN status >= 400 THEN 1 ELSE 0 END), 0),
       AVG(dur_ms), MIN(ts), MAX(ts)
FROM requests`+where, args...).Scan(&out.Total, &out.Errors, &avg, &oldest, &newest)
	if err != nil {
		return out, fmt.Errorf("reqlog: summary: %w", err)
	}
	if avg.Valid {
		out.AvgMS = avg.Float64
	}
	if oldest.Valid {
		out.OldestEntry = time.UnixMilli(oldest.Int64)
	}
	if newest.Valid {
		out.NewestEntry = time.UnixMilli(newest.Int64)
	}
	if out.Total > 0 {
		out.ErrorRate = float64(out.Errors) / float64(out.Total) * 100
	}

	// p95 by offset rather than a window function: works on any SQLite build
	// and the index on dur_ms is not needed at these row counts.
	if out.Total > 0 {
		off := int64(float64(out.Total) * 0.95)
		if off >= out.Total {
			off = out.Total - 1
		}
		p95Args := append(append([]any{}, args...), off)
		var p95 sql.NullInt64
		if err := s.db.QueryRow(`SELECT dur_ms FROM requests`+where+
			` ORDER BY dur_ms ASC LIMIT 1 OFFSET ?`, p95Args...).Scan(&p95); err == nil && p95.Valid {
			out.P95MS = p95.Int64
		}
	}

	out.DBBytes = s.fileSize()
	return out, nil
}

// StatusBreakdown counts entries per status code, most frequent first.
func (s *Store) StatusBreakdown(f Filter) ([]StatusCount, error) {
	where, args := f.where()
	rows, err := s.db.Query(`SELECT status, COUNT(*) c FROM requests`+where+
		` GROUP BY status ORDER BY c DESC`, args...)
	if err != nil {
		return nil, fmt.Errorf("reqlog: status breakdown: %w", err)
	}
	defer rows.Close()

	var out []StatusCount
	for rows.Next() {
		var sc StatusCount
		if err := rows.Scan(&sc.Status, &sc.Count); err != nil {
			return nil, err
		}
		out = append(out, sc)
	}
	return out, rows.Err()
}

// TopPaths summarises the busiest routes.
func (s *Store) TopPaths(f Filter, limit int) ([]PathStat, error) {
	if limit <= 0 {
		limit = 10
	}
	where, args := f.where()
	args = append(args, limit)
	rows, err := s.db.Query(`
SELECT path, COUNT(*) c, AVG(dur_ms), MAX(dur_ms),
       COALESCE(SUM(CASE WHEN status >= 400 THEN 1 ELSE 0 END), 0)
FROM requests`+where+` GROUP BY path ORDER BY c DESC LIMIT ?`, args...)
	if err != nil {
		return nil, fmt.Errorf("reqlog: top paths: %w", err)
	}
	defer rows.Close()

	var out []PathStat
	for rows.Next() {
		var p PathStat
		var avg sql.NullFloat64
		var max sql.NullInt64
		if err := rows.Scan(&p.Path, &p.Count, &avg, &max, &p.ErrCount); err != nil {
			return nil, err
		}
		p.AvgMS, p.MaxMS = avg.Float64, max.Int64
		out = append(out, p)
	}
	return out, rows.Err()
}

// PerDay returns the requests-per-day series, oldest first.
//
// Days are bucketed in the configured zone, not UTC. Grouping by UTC put a
// 06:00 Jakarta request on the previous day, which made the series wrong for
// anyone reading it locally.
//
// SQLite has no zone database, so the current offset is applied as a fixed
// shift. Exact for Jakarta, which has no daylight saving; for a zone that
// does, days either side of a transition could be off by an hour.
func (s *Store) PerDay(f Filter) ([]DayCount, error) {
	where, args := f.where()
	_, offset := time.Now().In(s.loc).Zone()
	shift := fmt.Sprintf("%+d seconds", offset)

	rows, err := s.db.Query(`
SELECT date(ts/1000, 'unixepoch', ?) d, COUNT(*),
       COALESCE(SUM(CASE WHEN status >= 400 THEN 1 ELSE 0 END), 0)
FROM requests`+where+` GROUP BY d ORDER BY d ASC`, append([]any{shift}, args...)...)
	if err != nil {
		return nil, fmt.Errorf("reqlog: per day: %w", err)
	}
	defer rows.Close()

	var out []DayCount
	for rows.Next() {
		var d DayCount
		if err := rows.Scan(&d.Day, &d.Count, &d.Errors); err != nil {
			return nil, err
		}
		out = append(out, d)
	}
	return out, rows.Err()
}

// DistinctPaths lists recorded routes, for the filter dropdown.
func (s *Store) DistinctPaths() ([]string, error) {
	rows, err := s.db.Query(`SELECT DISTINCT path FROM requests ORDER BY path`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var p string
		if err := rows.Scan(&p); err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

func (s *Store) fileSize() int64 {
	fi, err := os.Stat(s.cfg.Path)
	if err != nil {
		return 0
	}
	return fi.Size()
}

// TopCallers ranks API consumers by request count.
//
// Rows without a caller are grouped under "(unknown)" rather than dropped —
// a direct hit that bypassed the RapidAPI gateway is worth seeing, not
// hiding.
func (s *Store) TopCallers(f Filter, limit int) ([]CallerStat, error) {
	if limit <= 0 {
		limit = 20
	}
	where, args := f.where()
	args = append(args, limit)
	rows, err := s.db.Query(`
SELECT CASE WHEN caller = '' THEN '(unknown)' ELSE caller END AS c,
       COALESCE(MAX(plan), '') ,
       COUNT(*) n,
       COALESCE(SUM(CASE WHEN status >= 400 THEN 1 ELSE 0 END), 0),
       AVG(dur_ms),
       MAX(ts)
FROM requests`+where+` GROUP BY c ORDER BY n DESC LIMIT ?`, args...)
	if err != nil {
		return nil, fmt.Errorf("reqlog: top callers: %w", err)
	}
	defer rows.Close()

	var out []CallerStat
	for rows.Next() {
		var cs CallerStat
		var avg sql.NullFloat64
		var last sql.NullInt64
		if err := rows.Scan(&cs.Caller, &cs.Plan, &cs.Count, &cs.ErrCount, &avg, &last); err != nil {
			return nil, err
		}
		cs.AvgMS = avg.Float64
		if last.Valid {
			cs.LastSeen = time.UnixMilli(last.Int64)
		}
		out = append(out, cs)
	}
	return out, rows.Err()
}

// DistinctCallers lists known consumers, for the filter dropdown.
func (s *Store) DistinctCallers() ([]string, error) {
	rows, err := s.db.Query(
		`SELECT DISTINCT caller FROM requests WHERE caller != '' ORDER BY caller`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var c string
		if err := rows.Scan(&c); err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}
