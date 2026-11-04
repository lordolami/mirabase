package search

import (
	"database/sql"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

// Cursor leases (queries-and-search.md): "Cursor binds plan hash,
// schema/generation/policy epoch, snapshot lease and last sort position.
// ... Default snapshot lease is 60 seconds and the global maximum is 16
// active search cursor leases. Expiry returns cursor_expired".
type cursorState struct {
	planHash       string
	generation     int64
	schemaRevision int64
	policyEpoch    int64
	offset         int
	head           int64
	expires        time.Time
}

// CursorLeases owns the active cursor leases (global max 16).
type CursorLeases struct {
	mu       sync.Mutex
	leases   map[string]cursorState
	max      int
	leaseTTL time.Duration
	seq      uint64
}

// NewCursorLeases builds the lease store with contract defaults.
func NewCursorLeases() *CursorLeases {
	return NewCursorLeasesWith(
		time.Duration(DefaultCursorLeaseSec)*time.Second,
		DefaultMaxCursorLeases,
	)
}

// NewCursorLeasesWith builds the lease store with explicit bounds
// (configuration-reference.md search block: cursorLeaseSeconds 1..600,
// maxCursorLeases 1..10000). Non-positive values fall back to the
// contract defaults.
func NewCursorLeasesWith(leaseTTL time.Duration, max int) *CursorLeases {
	if leaseTTL <= 0 {
		leaseTTL = time.Duration(DefaultCursorLeaseSec) * time.Second
	}
	if max <= 0 {
		max = DefaultMaxCursorLeases
	}
	return &CursorLeases{
		leases:   map[string]cursorState{},
		max:      max,
		leaseTTL: leaseTTL,
	}
}

// Issue creates a cursor for a search continuation (the snapshot head is
// the change-journal boundary the result was read at). IDs are unique
// process-wide: a monotonic sequence under the lease mutex (a wall-clock
// nanosecond can repeat on coarse timers, and two clients paging at the
// same position must never share a cursor id).
func (c *CursorLeases) Issue(planHash string, generation, schemaRevision, policyEpoch int64, offset int, head int64) (string, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	// Purge expired first.
	now := time.Now()
	for id, l := range c.leases {
		if now.After(l.expires) {
			delete(c.leases, id)
		}
	}
	if len(c.leases) >= c.max {
		return "", ErrBudgetExhausted
	}
	c.seq++
	id := fmt.Sprintf("cur-%d-%d", c.seq, offset)
	c.leases[id] = cursorState{
		planHash:       planHash,
		generation:     generation,
		schemaRevision: schemaRevision,
		policyEpoch:    policyEpoch,
		offset:         offset,
		head:           head,
		expires:        now.Add(c.leaseTTL),
	}
	return id, nil
}

// Validate checks a cursor and returns the bound position and snapshot
// head; expiry or a mismatched query returns cursor_expired ("do not
// silently continue against a different snapshot"). A validated cursor is
// CONSUMED: the continuation immediately issues the next lease, so the
// global pool stays bounded by concurrent continuations rather than by
// every page ever served within the lease window.
func (c *CursorLeases) Validate(id, planHash string, generation, schemaRevision, policyEpoch int64) (int, int64, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	l, ok := c.leases[id]
	if !ok || time.Now().After(l.expires) {
		delete(c.leases, id)
		return 0, 0, ErrCursorExpired
	}
	if l.planHash != planHash || l.generation != generation || l.schemaRevision != schemaRevision || l.policyEpoch != policyEpoch {
		delete(c.leases, id)
		return 0, 0, ErrCursorExpired
	}
	delete(c.leases, id)
	return l.offset, l.head, nil
}

// collectRows scans the vector search rows into items with field
// protection (the caller's authorized field list) and the score object.
func collectRows(rows *sql.Rows, res *Result, limit int, scoreKind, metric string) (bool, error) {
	cols, err := rows.Columns()
	if err != nil {
		return false, err
	}
	scoreIdx := -1
	for i, c := range cols {
		if c == "_mb_score" {
			scoreIdx = i
		}
	}
	more := false
	count := 0
	for rows.Next() {
		if count >= limit {
			more = true
			break
		}
		ptrs := make([]any, len(cols))
		raw := make([]sql.RawBytes, len(cols))
		for i := range ptrs {
			ptrs[i] = &raw[i]
		}
		if err := rows.Scan(ptrs...); err != nil {
			return false, err
		}
		item := Item{
			Record:     map[string]any{},
			Highlights: []Highlight{},
		}
		for i, c := range cols {
			if c == "_mb_score" {
				continue
			}
			if raw[i] == nil {
				item.Record[c] = nil
			} else {
				item.Record[c] = string(raw[i])
			}
		}
		if scoreIdx >= 0 && raw[scoreIdx] != nil {
			var v float64
			if _, err := fmtSscan(string(raw[scoreIdx]), &v); err == nil {
				item.Score = Score{Kind: scoreKind + ":" + metric, Value: v}
			} else {
				item.Score = Score{Kind: scoreKind + ":" + metric, Value: 0}
			}
		}
		res.Items = append(res.Items, item)
		count++
	}
	return more, rows.Err()
}

// collectTextRows scans the FTS rows, adding authorized highlights when
// requested. Highlight fragments are engine-escaped and additionally
// sanitized here ("Highlight fragments are escaped/sanitized data; never
// emit untrusted HTML directly").
//
// The query selects one fts_highlight column per requested field
// (_mb_hl_0, _mb_hl_1, ... aligned with hlFields), so every fragment is
// attributed to the field that produced it (a concatenated multi-column
// highlight cannot be attributed — the engine joins the columns with a
// plain space separator).
func collectTextRows(rows *sql.Rows, res *Result, limit int, clause *TextClause, hlFields []string) (bool, error) {
	allCols, err := rows.Columns()
	if err != nil {
		return false, err
	}
	scoreIdx := -1
	hlIdx := map[int]int{} // highlight column index -> requested field position
	for i, c := range allCols {
		if c == "_mb_score" {
			scoreIdx = i
			continue
		}
		if n, ok := hlAliasIndex(c); ok && n < len(hlFields) {
			hlIdx[i] = n
		}
	}
	more := false
	count := 0
	for rows.Next() {
		if count >= limit {
			more = true
			break
		}
		ptrs := make([]any, len(allCols))
		raw := make([]sql.RawBytes, len(allCols))
		for i := range ptrs {
			ptrs[i] = &raw[i]
		}
		if err := rows.Scan(ptrs...); err != nil {
			return false, err
		}
		item := Item{
			Record:     map[string]any{},
			Highlights: []Highlight{},
		}
		for i, c := range allCols {
			if c == "_mb_score" {
				continue
			}
			if _, isHl := hlIdx[i]; isHl {
				continue
			}
			if raw[i] == nil {
				item.Record[c] = nil
			} else {
				item.Record[c] = string(raw[i])
			}
		}
		if scoreIdx >= 0 && raw[scoreIdx] != nil {
			var v float64
			if _, err := fmtSscan(string(raw[scoreIdx]), &v); err == nil {
				item.Score = Score{Kind: "bm25", Value: v}
			}
		}
		if clause.Highlight {
			// Per-field attribution, in requested-field order: the column
			// _mb_hl_n holds the highlights of hlFields[n] only. Fragments
			// stay sentinel-stripped raw text (JSON-encoded, never HTML).
			for i, n := range hlIdx {
				if raw[i] == nil {
					continue
				}
				frags := splitHighlight(string(raw[i]))
				if len(frags) > 0 {
					item.Highlights = append(item.Highlights, Highlight{
						Field:     hlFields[n],
						Fragments: frags,
					})
				}
			}
			sort.Slice(item.Highlights, func(a, b int) bool {
				return highlightFieldPosition(hlFields, item.Highlights[a].Field) <
					highlightFieldPosition(hlFields, item.Highlights[b].Field)
			})
		}
		res.Items = append(res.Items, item)
		count++
	}
	return more, rows.Err()
}

// hlAlias is the highlight select alias for requested-field position n.
func hlAlias(n int) string { return fmt.Sprintf("_mb_hl_%d", n) }

// hlAliasIndex parses a result column name as a highlight alias.
func hlAliasIndex(name string) (int, bool) {
	const prefix = "_mb_hl_"
	if !strings.HasPrefix(name, prefix) {
		return 0, false
	}
	n, err := strconv.Atoi(name[len(prefix):])
	if err != nil || n < 0 {
		return 0, false
	}
	return n, true
}

func highlightFieldPosition(fields []string, f string) int {
	for i, x := range fields {
		if x == f {
			return i
		}
	}
	return len(fields)
}

func splitHighlight(text string) []string {
	const open = "\x1fB\x1f"
	const close = "\x1f/b\x1f"
	var frags []string
	for {
		i := indexOf(text, open)
		if i < 0 {
			break
		}
		rest := text[i+len(open):]
		j := indexOf(rest, close)
		if j < 0 {
			break
		}
		frags = append(frags, rest[:j])
		text = rest[j+len(close):]
	}
	return frags
}

func indexOf(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}

func fmtSscan(s string, v *float64) (int, error) {
	return fmtSscanImpl(s, v)
}

func fmtSscanImpl(s string, v *float64) (int, error) {
	return fmt.Sscanf(strings.TrimSpace(s), "%g", v)
}
