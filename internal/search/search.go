// Package search implements the MiraBase search operation
// (queries-and-search.md): exact authorized vector search, native
// Tantivy-backed FTS text search, reciprocal-rank-fusion hybrid search
// and Go-side fuzzy search on the pinned Turso engine, with rule
// filtering, field protection, budgets and cursors.
package search

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"sort"
	"strings"
	"time"
	"unicode/utf8"
)

// Budget defaults (configuration-reference.md search block).
const (
	DefaultDeadlineMs      = 5000
	DefaultCandidateBudget = 10000
	DefaultLimit           = 20
	DefaultMaxLimit        = 100
	DefaultCursorLeaseSec  = 60
	DefaultMaxCursorLeases = 16
)

// Metrics and their engine functions (pinned engine, function.rs).
var metricFunctions = map[string]string{
	"cosine":       "vector_distance_cos",
	"l2":           "vector_distance_l2",
	"negative_dot": "vector_distance_dot",
}

// jaccard applies only to the qualified representation (sparse).
var jaccardMetric = "jaccard"

// Errors.
var (
	ErrCapabilityUnavailable = errors.New("capability_unavailable")
	ErrCursorExpired         = errors.New("cursor_expired")
	ErrBudgetExhausted       = errors.New("resource_exhausted")
	ErrInvalidRequest        = errors.New("invalid_request")
	ErrNotIndexed            = errors.New("schema_mismatch")
)

// VectorClause is the vector mode clause (additionalProperties false, all
// required).
type VectorClause struct {
	Field  string          `json:"field"`
	Query  json.RawMessage `json:"query"`
	Metric string          `json:"metric"`
}

// TextClause is the text mode clause.
type TextClause struct {
	Fields    []string     `json:"fields"`
	Query     string       `json:"query"`
	Syntax    string       `json:"syntax,omitempty"` // simple|advanced
	Weights   []TextWeight `json:"weights,omitempty"`
	Highlight bool         `json:"highlight,omitempty"`
	Tokenizer string       `json:"-"`
	IndexName string       `json:"-"`
}
type TextWeight struct {
	Field  string  `json:"field"`
	Weight float64 `json:"weight"`
}

// FusionClause is the hybrid fusion clause (wire-schema-reference.md,
// SearchRequest hybrid variant): kind rrf only, k 1..1000.
type FusionClause struct {
	Kind string `json:"kind"`
	K    int    `json:"k"`
}

// FuzzyClause is the fuzzy mode clause. On the wire the fuzzy variant
// carries field/query/algorithm/threshold at the top level of the
// SearchRequest object; Request.UnmarshalJSON lifts them into this clause
// (queries-and-search.md: "fuzzy: field, query, algorithm,
// maximumDistance or minimumSimilarity, limit and filter").
type FuzzyClause struct {
	Field             string   `json:"field"`
	Query             string   `json:"query"`
	Algorithm         string   `json:"algorithm"`
	MaximumDistance   *int     `json:"maximumDistance,omitempty"`
	MinimumSimilarity *float64 `json:"minimumSimilarity,omitempty"`
}

// Request is the SearchRequest (discriminated by mode).
type Request struct {
	Mode   string        `json:"mode"`
	Vector *VectorClause `json:"vector,omitempty"`
	Text   *TextClause   `json:"text,omitempty"`
	Fusion *FusionClause `json:"fusion,omitempty"`
	Fuzzy  *FuzzyClause  `json:"fuzzy,omitempty"`
	Limit  int           `json:"limit,omitempty"`
	Filter string        `json:"filter,omitempty"`
	Fields []string      `json:"fields,omitempty"`
	Cursor string        `json:"cursor,omitempty"`

	// Top-level fuzzy wire fields; lifted into Fuzzy by UnmarshalJSON.
	Field             string   `json:"field,omitempty"`
	Query             string   `json:"query,omitempty"`
	Algorithm         string   `json:"algorithm,omitempty"`
	MaximumDistance   *int     `json:"maximumDistance,omitempty"`
	MinimumSimilarity *float64 `json:"minimumSimilarity,omitempty"`
}

// UnmarshalJSON decodes the wire SearchRequest and lifts the top-level
// fuzzy fields into the Fuzzy clause (the fuzzy wire variant does not
// nest them). A nested "fuzzy" object is also accepted for direct Go
// callers. Every variant declares additionalProperties:false (wire
// schema reference, SearchRequest vector/text/hybrid/fuzzy variants and
// the clause defs): unknown request properties are REFUSED at the top
// level AND inside the clauses and weight items — a silently ignored key
// can mask a client bug into a differently-interpreted query.
func (r *Request) UnmarshalJSON(data []byte) error {
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	var mode string
	if v, ok := raw["mode"]; ok {
		if err := json.Unmarshal(v, &mode); err != nil {
			return fmt.Errorf("%w: mode must be a string", ErrInvalidRequest)
		}
	}
	allowed := map[string]bool{
		"mode": true, "limit": true, "filter": true, "cursor": true,
	}
	switch mode {
	case "vector":
		allowed["vector"] = true
		allowed["fields"] = true
	case "text":
		allowed["text"] = true
		allowed["fields"] = true
	case "hybrid":
		allowed["vector"] = true
		allowed["text"] = true
		allowed["fusion"] = true
		allowed["fields"] = true
	case "fuzzy":
		allowed["field"] = true
		allowed["query"] = true
		allowed["algorithm"] = true
		allowed["maximumDistance"] = true
		allowed["minimumSimilarity"] = true
		// the nested fuzzy clause form (direct Go callers; the wire lifts
		// the fuzzy fields to the top level)
		allowed["fuzzy"] = true
	default:
		return fmt.Errorf("%w: unknown mode %q", ErrInvalidRequest, mode)
	}
	for k := range raw {
		if !allowed[k] {
			return fmt.Errorf("%w: unknown request property %q", ErrInvalidRequest, k)
		}
	}
	// The wire variants bound limit at 1..100 and the projection at 100
	// fields; an explicit out-of-range limit is a client bug, not a
	// default.
	if v, ok := raw["limit"]; ok {
		var limit int
		if err := json.Unmarshal(v, &limit); err != nil {
			return fmt.Errorf("%w: limit must be an integer", ErrInvalidRequest)
		}
		if limit < 1 {
			return fmt.Errorf("%w: limit must be at least 1", ErrInvalidRequest)
		}
	}
	if v, ok := raw["fields"]; ok {
		var fields []string
		if err := json.Unmarshal(v, &fields); err != nil {
			return fmt.Errorf("%w: fields must be an array of strings", ErrInvalidRequest)
		}
		if len(fields) > 100 {
			return fmt.Errorf("%w: too many projected fields (max 100)", ErrInvalidRequest)
		}
	}
	// Clause-level additionalProperties:false (wire-schema-reference.md
	// VectorClause/TextClause/FusionClause/fuzzy variant).
	if err := strictClauseKeys(raw["vector"], "vector", "field", "query", "metric"); err != nil {
		return err
	}
	if err := strictClauseKeys(raw["text"], "text", "fields", "query", "syntax", "weights", "highlight"); err != nil {
		return err
	}
	if err := strictClauseKeys(raw["fusion"], "fusion", "kind", "k"); err != nil {
		return err
	}
	if err := strictClauseKeys(raw["fuzzy"], "fuzzy", "field", "query", "algorithm", "maximumDistance", "minimumSimilarity"); err != nil {
		return err
	}
	// Weight items are {field, weight} objects with no extra properties.
	if v, ok := raw["text"]; ok {
		var t struct {
			Weights []json.RawMessage `json:"weights"`
		}
		if err := json.Unmarshal(v, &t); err != nil {
			return fmt.Errorf("%w: text clause must be an object", ErrInvalidRequest)
		}
		for i, w := range t.Weights {
			if err := strictClauseKeys(w, "weights", "field", "weight"); err != nil {
				return fmt.Errorf("%w: weight item %d: %s", ErrInvalidRequest, i, strings.TrimPrefix(err.Error(), "invalid_request: "))
			}
		}
	}
	type alias Request
	var a alias
	if err := json.Unmarshal(data, &a); err != nil {
		return err
	}
	*r = Request(a)
	if r.Mode == "fuzzy" && r.Fuzzy == nil && r.Field != "" {
		r.Fuzzy = &FuzzyClause{
			Field:             r.Field,
			Query:             r.Query,
			Algorithm:         r.Algorithm,
			MaximumDistance:   r.MaximumDistance,
			MinimumSimilarity: r.MinimumSimilarity,
		}
	}
	return nil
}

// strictClauseKeys rejects unknown properties inside one clause object
// (the clause wire schemas declare additionalProperties:false).
func strictClauseKeys(raw json.RawMessage, clause string, allowed ...string) error {
	if len(raw) == 0 {
		return nil
	}
	var m map[string]json.RawMessage
	if err := json.Unmarshal(raw, &m); err != nil {
		return fmt.Errorf("%w: the %s clause must be an object", ErrInvalidRequest, clause)
	}
	permitted := map[string]bool{}
	for _, k := range allowed {
		permitted[k] = true
	}
	for k := range m {
		if !permitted[k] {
			return fmt.Errorf("%w: unknown %s property %q", ErrInvalidRequest, clause, k)
		}
	}
	return nil
}

// Item is one SearchItem.
type Item struct {
	Record     map[string]any `json:"record"`
	Score      Score          `json:"score"`
	Highlights []Highlight    `json:"highlights"`
}

type Score struct {
	Kind  string  `json:"kind"`
	Value float64 `json:"value"`
}

type Highlight struct {
	Field     string   `json:"field"`
	Fragments []string `json:"fragments"`
}

// Result is the SearchResult (all fields required).
type Result struct {
	DatabaseID     string  `json:"databaseId"`
	Generation     int64   `json:"generation"`
	SchemaRevision int64   `json:"schemaRevision"`
	Items          []Item  `json:"items"`
	Mode           string  `json:"mode"`
	Metric         *string `json:"metric"`
	Complete       bool    `json:"complete"`
	ScannedCount   int     `json:"scannedCount"`
	ElapsedMs      float64 `json:"elapsedMs"`
	NextCursor     *string `json:"nextCursor"`

	// moreHint/snapHead are the internal cursor-issue signals: whether
	// more result pages exist and the snapshot head the result was read
	// at (the lease binds both).
	moreHint bool
	snapHead int64
}

// Options carries the per-call budgets, the shared cursor lease store and
// the authorized policy predicate the API layer injects ("Apply list/view
// rules and field protection before results leave the server" — the
// predicate is compiled by the caller through the parsed rule machinery
// and applied inside every scanned query).
type Options struct {
	DeadlineMs      int
	CandidateBudget int
	DefaultLimit    int
	MaxLimit        int
	// Leases is the shared search-cursor lease store; nil uses the
	// package default.
	Leases *CursorLeases
	// PolicyWhere/PolicyArgs are the authorized rule predicate (a
	// parameterized SQL fragment with ? placeholders) applied to every
	// scanned/counted query.
	PolicyWhere string
	PolicyArgs  []any
}

// DefaultLeases is the process-shared search cursor lease store.
var DefaultLeases = NewCursorLeases()

// DefaultOptions returns the contract defaults.
func DefaultOptions() Options {
	return Options{
		DeadlineMs:      DefaultDeadlineMs,
		CandidateBudget: DefaultCandidateBudget,
		DefaultLimit:    DefaultLimit,
		MaxLimit:        DefaultMaxLimit,
	}
}

// Search executes one authorized search against a target database. The
// caller has already resolved the database context and authorization
// ("Each query resolves one database, published schema revision, actor
// and rules"); the filter parameter is applied as a conservative
// parameterized predicate, and the caller's policy predicate
// (Options.PolicyWhere) plus field protection/projection run before
// results leave the server.
//
// Cursors (queries-and-search.md): a result with more pages issues a
// cursor bound to the plan hash, the context (generation/schema/policy
// epoch), the last sort position and the captured SNAPSHOT (the source
// change-journal head at read time). A continuation validates the lease
// and refuses to silently continue against a different snapshot
// (cursor_expired); the default snapshot lease is 60 seconds and the
// global maximum is 16 active leases.
func Search(ctx context.Context, target *sql.DB, collection string, req *Request, opts Options, dbID string, generation, schemaRevision, policyEpoch int64) (*Result, error) {
	start := time.Now()
	if opts.DefaultLimit <= 0 {
		opts = DefaultOptions()
	}
	limit := req.Limit
	if limit <= 0 {
		limit = opts.DefaultLimit
	}
	if limit > opts.MaxLimit {
		limit = opts.MaxLimit
	}
	deadline := opts.DeadlineMs
	if deadline <= 0 {
		deadline = DefaultDeadlineMs
	}
	budget := opts.CandidateBudget
	if budget <= 0 {
		budget = DefaultCandidateBudget
	}
	dctx, cancel := context.WithTimeout(ctx, time.Duration(deadline)*time.Millisecond)
	defer cancel()

	// Cursor continuation: the lease binds the identical query, context
	// and snapshot; it supplies the sort position to continue from.
	planHash := planHashOf(req)
	offset, boundHead := 0, int64(-1)
	if req.Cursor != "" {
		if opts.Leases == nil {
			opts.Leases = DefaultLeases
		}
		var err error
		offset, boundHead, err = opts.Leases.Validate(req.Cursor, planHash, generation, schemaRevision, policyEpoch)
		if err != nil {
			return nil, err
		}
	}
	issue := func(res *Result) error {
		if res == nil || !res.moreHint {
			return nil
		}
		if opts.Leases == nil {
			opts.Leases = DefaultLeases
		}
		id, ierr := opts.Leases.Issue(planHash, generation, schemaRevision, policyEpoch, offset+limit, res.snapHead)
		if ierr != nil {
			if req.Cursor != "" {
				// A continuation that cannot renew its lease fails
				// explicitly (the bounded shared resource).
				return ierr
			}
			// A fresh search stays valid but not continuable.
			res.Complete = false
			return nil
		}
		res.NextCursor = &id
		return nil
	}

	switch req.Mode {
	case "vector":
		if req.Vector == nil {
			return nil, fmt.Errorf("%w: vector mode requires the vector clause", ErrInvalidRequest)
		}
		res, err := searchVector(dctx, target, collection, req, limit, budget, start, dbID, generation, schemaRevision, offset, boundHead, opts.PolicyWhere, opts.PolicyArgs)
		if err != nil {
			return nil, err
		}
		if err := issue(res); err != nil {
			return nil, err
		}
		return res, nil
	case "text":
		if req.Text == nil {
			return nil, fmt.Errorf("%w: text mode requires the text clause", ErrInvalidRequest)
		}
		res, err := searchText(dctx, target, collection, req, limit, budget, start, dbID, generation, schemaRevision, offset, boundHead, opts.PolicyWhere, opts.PolicyArgs)
		if err != nil {
			return nil, err
		}
		if err := issue(res); err != nil {
			return nil, err
		}
		return res, nil
	case "hybrid":
		res, err := searchHybrid(dctx, target, collection, req, limit, budget, start, dbID, generation, schemaRevision, offset, boundHead, opts.PolicyWhere, opts.PolicyArgs)
		if err != nil {
			return nil, err
		}
		if err := issue(res); err != nil {
			return nil, err
		}
		return res, nil
	case "fuzzy":
		res, err := searchFuzzy(dctx, target, collection, req, limit, budget, start, dbID, generation, schemaRevision, offset, boundHead, opts.PolicyWhere, opts.PolicyArgs)
		if err != nil {
			return nil, err
		}
		if err := issue(res); err != nil {
			return nil, err
		}
		return res, nil
	default:
		return nil, fmt.Errorf("%w: unknown mode %q", ErrInvalidRequest, req.Mode)
	}
}

// planHashOf binds the identical-query identity of a cursor (the request
// without its cursor field, canonically encoded).
func planHashOf(req *Request) string {
	type hashable struct {
		Mode   string        `json:"mode"`
		Vector *VectorClause `json:"vector,omitempty"`
		Text   *TextClause   `json:"text,omitempty"`
		Fusion *FusionClause `json:"fusion,omitempty"`
		Fuzzy  *FuzzyClause  `json:"fuzzy,omitempty"`
		Limit  int           `json:"limit,omitempty"`
		Filter string        `json:"filter,omitempty"`
		Fields []string      `json:"fields,omitempty"`
	}
	h := hashable{
		Mode: req.Mode, Vector: req.Vector, Text: req.Text, Fusion: req.Fusion,
		Fuzzy: req.Fuzzy, Limit: req.Limit, Filter: req.Filter, Fields: req.Fields,
	}
	b, err := json.Marshal(h)
	if err != nil {
		return fmt.Sprintf("%v", h)
	}
	return fmt.Sprintf("%x", sha256.Sum256(b))
}

// snapshotHead reads (and installs, when absent) the target's change
// journal head — the snapshot boundary a cursor binds ("do not silently
// continue against a different snapshot": an advanced head expires the
// cursor).
func snapshotHead(ctx context.Context, target *sql.DB) (int64, error) {
	if _, err := target.ExecContext(ctx, snapshotJournalDDL); err != nil {
		return 0, err
	}
	var head int64
	err := target.QueryRowContext(ctx,
		`SELECT COALESCE(MAX(commit_seq), 0) FROM _mb_changes`).Scan(&head)
	return head, err
}

const snapshotJournalDDL = `CREATE TABLE IF NOT EXISTS _mb_changes (
 commit_seq INTEGER PRIMARY KEY,
 transaction_id TEXT NOT NULL,
 generation INTEGER NOT NULL,
 schema_revision INTEGER NOT NULL,
 source TEXT NOT NULL CHECK(source IN ('record','admin-sql','pg','native-sync','migration')),
 timestamp TEXT NOT NULL
)`

// normalizeQueryVector converts the wire vector (dense JSON array or sparse
// {dimensions, indices, values}) into the native SQL literal used by the
// engine functions. Sparse indices must be sorted/unique/in-range.
func normalizeQueryVector(raw json.RawMessage, metric string) (string, error) {
	var dense []float64
	if err := json.Unmarshal(raw, &dense); err == nil {
		return denseLiteral(dense), nil
	}
	var sparse struct {
		Dimensions int       `json:"dimensions"`
		Indices    []int     `json:"indices"`
		Values     []float64 `json:"values"`
	}
	if err := json.Unmarshal(raw, &sparse); err != nil {
		return "", fmt.Errorf("%w: vector must be a finite number array or a sparse dimensions/indices/values object", ErrInvalidRequest)
	}
	if sparse.Dimensions < 1 || sparse.Dimensions > 4096 {
		return "", fmt.Errorf("%w: sparse dimensions out of range", ErrInvalidRequest)
	}
	if len(sparse.Indices) != len(sparse.Values) {
		return "", fmt.Errorf("%w: sparse indices/values length mismatch", ErrInvalidRequest)
	}
	// Sparse indices must be sorted/unique/in-range.
	prev := -1
	for _, idx := range sparse.Indices {
		if idx < 0 || idx >= sparse.Dimensions {
			return "", fmt.Errorf("%w: sparse index out of range", ErrInvalidRequest)
		}
		if idx <= prev {
			return "", fmt.Errorf("%w: sparse indices must be sorted and unique", ErrInvalidRequest)
		}
		prev = idx
	}
	if metric == jaccardMetric {
		// Jaccard applies only to the qualified representation (sparse).
		// Build the dense representation of the sparse vector for the
		// engine's jaccard implementation.
		full := make([]float64, sparse.Dimensions)
		for i, idx := range sparse.Indices {
			full[idx] = sparse.Values[i]
		}
		return denseLiteral(full), nil
	}
	// Dense fallback for distances: fill zeros.
	full := make([]float64, sparse.Dimensions)
	for i, idx := range sparse.Indices {
		full[idx] = sparse.Values[i]
	}
	return denseLiteral(full), nil
}

func denseLiteral(v []float64) string {
	parts := make([]string, len(v))
	for i, f := range v {
		if isInfOrNaN(f) {
			return "" // caller reports invalid_request on empty
		}
		parts[i] = fmt.Sprintf("%v", f)
	}
	return "[" + strings.Join(parts, ",") + "]"
}

func isInfOrNaN(f float64) bool {
	return f != f || f > 1e308 || f < -1e308
}

// searchVector runs exact authorized vector search: brute-force native
// distance ordering ("Vector indexes are not yet supported. All vector
// searches currently use brute-force scanning"), with the caller's filter
// and the authorized policy predicate applied inside the scanned domain so
// no unauthorized candidate set can pre-exist.
func searchVector(ctx context.Context, target *sql.DB, collection string, req *Request, limit, budget int, start time.Time, dbID string, generation, schemaRevision int64, offset int, boundHead int64, policyWhere string, policyArgs []any) (*Result, error) {
	metric := req.Vector.Metric
	fn, ok := metricFunctions[metric]
	if !ok && metric != jaccardMetric {
		return nil, fmt.Errorf("%w: unknown metric %q", ErrInvalidRequest, metric)
	}
	if metric == jaccardMetric {
		fn = "vector_distance_jaccard"
	}
	queryVec, err := normalizeQueryVector(req.Vector.Query, metric)
	if err != nil {
		return nil, err
	}
	if queryVec == "" {
		return nil, fmt.Errorf("%w: vector components must be finite", ErrInvalidRequest)
	}

	// Zero vectors with undefined cosine produce a field error.
	if metric == "cosine" {
		allZero := true
		for _, c := range queryVec {
			if c != '0' && c != '.' && c != ',' && c != '[' && c != ']' && c != '-' {
				allZero = false
				break
			}
		}
		if allZero {
			return nil, fmt.Errorf("%w: zero vector with undefined cosine", ErrInvalidRequest)
		}
	}

	filter, args, err := safeFilter(req.Filter)
	if err != nil {
		return nil, err
	}
	args = append(append([]any{}, args...), policyArgs...)
	where, werr := composeWhere(filter, policyWhere)
	if werr != nil {
		return nil, werr
	}

	// Snapshot binding: a continuation must observe the SAME snapshot
	// (an advanced change-journal head expires the cursor).
	head, herr := snapshotHead(ctx, target)
	if herr != nil {
		return nil, herr
	}
	if boundHead >= 0 && head != boundHead {
		return nil, ErrCursorExpired
	}

	// Candidate budget: exact search scans the authorized domain; the
	// budget caps the scan and exhaustion is reported as incomplete.
	var scanned int
	if err := target.QueryRowContext(ctx,
		fmt.Sprintf(`SELECT count(*) FROM %s%s`, quoteIdent(collection), where), args...).Scan(&scanned); err != nil {
		return nil, err
	}
	complete := scanned <= budget
	fetchLimit := limit + 1
	if !complete {
		fetchLimit = budget
	}
	page := ""
	if offset > 0 {
		page = fmt.Sprintf(" OFFSET %d", offset)
	}

	field := quoteIdent(req.Vector.Field)
	// Stable ties use record ID: equal distances must page deterministically
	// across continuation requests.
	sqlStr := fmt.Sprintf(
		`SELECT *, %s(%s, vector('%s')) AS _mb_score FROM %s%s ORDER BY _mb_score ASC, "id" ASC LIMIT %d%s`,
		fn, field, queryVec, quoteIdent(collection), where, fetchLimit, page)
	rows, err := target.QueryContext(ctx, sqlStr, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	res := &Result{
		DatabaseID:     dbID,
		Generation:     generation,
		SchemaRevision: schemaRevision,
		Items:          []Item{},
		Mode:           "vector",
		Metric:         &metric,
		ScannedCount:   scanned,
		snapHead:       head,
	}
	more, err := collectRows(rows, res, limit, "distance", metric)
	if err != nil {
		return nil, err
	}
	res.moreHint = more
	res.Complete = complete && !more
	res.ElapsedMs = float64(time.Since(start).Milliseconds())
	return res, nil
}

// composeWhere combines the caller filter and the authorized policy
// predicate into one WHERE clause (both parameterized; arguments travel
// in filter-then-policy order).
func composeWhere(filter, policyWhere string) (string, error) {
	var clauses []string
	if filter != "" {
		clauses = append(clauses, "("+filter+")")
	}
	if policyWhere != "" {
		clauses = append(clauses, "("+policyWhere+")")
	}
	if len(clauses) == 0 {
		return "", nil
	}
	return " WHERE " + strings.Join(clauses, " AND "), nil
}

// searchText runs native FTS text search. The collection must have an FTS
// index covering the requested fields (created through the index
// maintenance action); searching without one is a typed schema error
// ("Index creation/rebuild is a durable job").
//
// Completeness and scannedCount cover the FILTERED matching domain the
// query actually applies (count over the filter AND the fts match), so a
// narrow filtered search over a large table is complete when the filtered
// domain fits the budget. Per-request field weights, when present, rank
// by the weight-combined per-field scores.
func searchText(ctx context.Context, target *sql.DB, collection string, req *Request, limit, budget int, start time.Time, dbID string, generation, schemaRevision int64, offset int, boundHead int64, policyWhere string, policyArgs []any) (*Result, error) {
	clause := req.Text
	if len(clause.Fields) == 0 {
		return nil, fmt.Errorf("%w: text mode requires fields", ErrInvalidRequest)
	}
	if len(clause.Fields) > 30 {
		return nil, fmt.Errorf("%w: too many text fields (max 30)", ErrInvalidRequest)
	}
	if clause.Query == "" {
		return nil, fmt.Errorf("%w: empty text query", ErrInvalidRequest)
	}
	switch clause.Syntax {
	case "", "simple", "advanced":
	default:
		return nil, fmt.Errorf("%w: unknown syntax %q", ErrInvalidRequest, clause.Syntax)
	}

	// The FTS index must exist for the collection+fields; discover it.
	if _, err := findFTSIndex(ctx, target, collection, clause.Fields); err != nil {
		return nil, err
	}

	filter, args, err := safeFilter(req.Filter)
	if err != nil {
		return nil, err
	}
	args = append(append([]any{}, args...), policyArgs...)

	// Snapshot binding for a continuation.
	head, herr := snapshotHead(ctx, target)
	if herr != nil {
		return nil, herr
	}
	if boundHead >= 0 && head != boundHead {
		return nil, ErrCursorExpired
	}

	if len(clause.Weights) > 0 {
		return searchTextWeighted(ctx, target, collection, clause, filter, args, limit, budget, start, dbID, generation, schemaRevision, offset, head, policyWhere)
	}

	// The scanned matching domain: the filter and the fts match the query
	// actually applies (a narrow filtered search over a large table stays
	// complete when the filtered domain fits the budget).
	filterClauses := []string{}
	if filter != "" {
		filterClauses = append(filterClauses, "("+filter+")")
	}
	if policyWhere != "" {
		filterClauses = append(filterClauses, "("+policyWhere+")")
	}
	cols := make([]string, 0, len(clause.Fields))
	for _, f := range clause.Fields {
		cols = append(cols, quoteIdent(f))
	}
	colList := strings.Join(cols, ", ")
	queryLiteral := quoteLiteral(clause.Query)
	filterClauses = append(filterClauses, fmt.Sprintf("fts_match(%s, %s)", colList, queryLiteral))
	domainWhere := ""
	if len(filterClauses) > 0 {
		domainWhere = " WHERE " + strings.Join(filterClauses, " AND ")
	}
	var scanned int
	if err := target.QueryRowContext(ctx,
		fmt.Sprintf(`SELECT count(*) FROM %s%s`, quoteIdent(collection), domainWhere), args...).Scan(&scanned); err != nil {
		return nil, err
	}
	complete := scanned <= budget
	fetchLimit := limit + 1
	if !complete {
		fetchLimit = budget
	}
	page := ""
	if offset > 0 {
		page = fmt.Sprintf(" OFFSET %d", offset)
	}

	var selectExtra string
	if clause.Highlight {
		// One highlight column per requested field, aligned with the
		// field order: the pinned engine's fts_highlight takes
		// (text_columns..., before_tag, after_tag, query) and joins
		// multiple text columns with a plain space, which cannot be
		// attributed back — so every field highlights (and is attributed)
		// separately ("items (records plus score and optional authorized
		// highlights)" with per-field fragments). The sentinels are the
		// REAL control bytes the Go-side splitter matches (SQL string
		// literals carry no \x escapes).
		parts := make([]string, 0, len(clause.Fields))
		for i, f := range clause.Fields {
			parts = append(parts, fmt.Sprintf(
				`%s(%s, '%s', '%s', %s) AS %s`,
				"fts_highlight", quoteIdent(f), hlOpenTag, hlCloseTag, queryLiteral, quoteIdent(hlAlias(i))))
		}
		selectExtra = ", " + strings.Join(parts, ", ")
	}
	sqlStr := fmt.Sprintf(
		`SELECT *%s, fts_score(%s, %s) AS _mb_score FROM %s%s ORDER BY _mb_score DESC, "id" ASC LIMIT %d%s`,
		selectExtra, colList, queryLiteral, quoteIdent(collection), domainWhere, fetchLimit, page)
	rows, err := target.QueryContext(ctx, sqlStr, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	metric := "bm25"
	res := &Result{
		DatabaseID:     dbID,
		Generation:     generation,
		SchemaRevision: schemaRevision,
		Items:          []Item{},
		Mode:           "text",
		Metric:         &metric,
		ScannedCount:   scanned,
		snapHead:       head,
	}
	more, err := collectTextRows(rows, res, limit, clause, clause.Fields)
	if err != nil {
		return nil, err
	}
	res.moreHint = more
	res.Complete = complete && !more
	res.ElapsedMs = float64(time.Since(start).Milliseconds())
	return res, nil
}

// searchTextWeighted ranks by per-field weighted scores: each requested
// field runs its own bounded engine query (fts_score restricted to that
// field), the per-record scores combine as the weight sum, and the final
// ordering is the weighted score with record-ID tie-breaks. Weights must
// reference the requested fields (unknown weight fields are
// invalid_request).
func searchTextWeighted(ctx context.Context, target *sql.DB, collection string, clause *TextClause, filter string, args []any, limit, budget int, start time.Time, dbID string, generation, schemaRevision int64, offset int, head int64, policyWhere string) (*Result, error) {
	weights := map[string]float64{}
	for _, w := range clause.Weights {
		if !containsField(clause.Fields, w.Field) {
			return nil, fmt.Errorf("%w: the weight field %q is not among the requested fields", ErrInvalidRequest, w.Field)
		}
		if w.Weight <= 0 {
			return nil, fmt.Errorf("%w: the weight of %q must be positive", ErrInvalidRequest, w.Field)
		}
		weights[w.Field] = w.Weight
	}
	// The wire TextClause bounds weights at 30 items (maxItems).
	if len(clause.Weights) > 30 {
		return nil, fmt.Errorf("%w: too many text weights (max 30)", ErrInvalidRequest)
	}

	var clauses []string
	if filter != "" {
		clauses = append(clauses, "("+filter+")")
	}
	if policyWhere != "" {
		clauses = append(clauses, "("+policyWhere+")")
	}
	baseWhere := ""
	if len(clauses) > 0 {
		baseWhere = " WHERE " + strings.Join(clauses, " AND ")
	}

	// The scanned matching domain (the union of the per-field matches) is
	// the bounded budget for every leg; completeness reflects the domain
	// the query applies.
	weighted := map[string]float64{}
	scanned := 0
	overflowed := false
	for _, f := range clause.Fields {
		w := weights[f]
		if w == 0 {
			continue
		}
		legWhere := baseWhere
		if legWhere == "" {
			legWhere = " WHERE "
		} else {
			legWhere += " AND "
		}
		legWhere += fmt.Sprintf("fts_match(%s, %s)", quoteIdent(f), quoteLiteral(clause.Query))
		legSQL := fmt.Sprintf(
			`SELECT "id", fts_score(%s, %s) AS _mb_s FROM %s%s ORDER BY _mb_s DESC LIMIT %d`,
			quoteIdent(f), quoteLiteral(clause.Query), quoteIdent(collection), legWhere, budget+1)
		rows, err := target.QueryContext(ctx, legSQL, args...)
		if err != nil {
			return nil, err
		}
		for rows.Next() {
			var id string
			var s float64
			if err := rows.Scan(&id, &s); err != nil {
				rows.Close()
				return nil, err
			}
			scanned++
			weighted[id] += w * s
		}
		if err := rows.Err(); err != nil {
			rows.Close()
			return nil, err
		}
		rows.Close()
	}
	if scanned > budget {
		// The per-field scan overflowed the budget: incomplete, never a
		// deceptively complete ranking.
		scanned = budget
		overflowed = true
	}
	// Rank: weighted score descending, record id ascending (stable ties
	// use record ID).
	type ranked struct {
		id    string
		score float64
	}
	all := make([]ranked, 0, len(weighted))
	for id, s := range weighted {
		all = append(all, ranked{id: id, score: s})
	}
	sort.Slice(all, func(i, j int) bool {
		if all[i].score != all[j].score {
			return all[i].score > all[j].score
		}
		return all[i].id < all[j].id
	})
	more := len(all) > offset+limit
	if offset < len(all) {
		all = all[offset:minInt(offset+limit, len(all))]
	} else {
		all = nil
	}
	// Fetch the page's records (parameterized IN list) with optional
	// per-field highlights (fts_highlight takes columns..., before_tag,
	// after_tag, query; one column per field keeps fragments attributed).
	var selectExtra string
	if clause.Highlight {
		parts := make([]string, 0, len(clause.Fields))
		for i, f := range clause.Fields {
			parts = append(parts, fmt.Sprintf(
				`%s(%s, '%s', '%s', %s) AS %s`,
				"fts_highlight", quoteIdent(f), hlOpenTag, hlCloseTag, quoteLiteral(clause.Query), quoteIdent(hlAlias(i))))
		}
		selectExtra = ", " + strings.Join(parts, ", ")
	}
	res := &Result{
		DatabaseID:     dbID,
		Generation:     generation,
		SchemaRevision: schemaRevision,
		Items:          []Item{},
		Mode:           "text",
		Metric:         strPtr("bm25"),
		ScannedCount:   scanned,
		Complete:       !overflowed && !more,
		snapHead:       head,
	}
	if len(all) == 0 {
		res.ElapsedMs = float64(time.Since(start).Milliseconds())
		return res, nil
	}
	ids := make([]string, len(all))
	for i, r := range all {
		ids[i] = r.id
	}
	placeholders := strings.TrimSuffix(strings.Repeat("?, ", len(ids)), ", ")
	fetchSQL := fmt.Sprintf(`SELECT *%s FROM %s WHERE "id" IN (%s)`, selectExtra, quoteIdent(collection), placeholders)
	frows, err := target.QueryContext(ctx, fetchSQL, strArgs(ids)...)
	if err != nil {
		return nil, err
	}
	defer frows.Close()
	byID, err := rowsToItems(frows, clause, ids)
	if err != nil {
		return nil, err
	}
	for _, r := range all {
		it, ok := byID[r.id]
		if !ok {
			continue
		}
		it.Score = Score{Kind: "bm25", Value: r.score}
		res.Items = append(res.Items, it)
	}
	res.moreHint = more
	res.ElapsedMs = float64(time.Since(start).Milliseconds())
	return res, nil
}

func containsField(fields []string, f string) bool {
	for _, x := range fields {
		if x == f {
			return true
		}
	}
	return false
}

func strArgs(ss []string) []any {
	out := make([]any, len(ss))
	for i, s := range ss {
		out[i] = s
	}
	return out
}

// rowsToItems scans one record/highlight query into items keyed by id.
// Highlight columns are the per-field aliases _mb_hl_<n> aligned with
// clause.Fields, so every fragment is attributed to the field that
// produced it.
func rowsToItems(rows *sql.Rows, clause *TextClause, ids []string) (map[string]Item, error) {
	allCols, err := rows.Columns()
	if err != nil {
		return nil, err
	}
	hlIdx := map[int]int{} // result column index -> requested field position
	for i, c := range allCols {
		if n, ok := hlAliasIndex(c); ok && n < len(clause.Fields) {
			hlIdx[i] = n
		}
	}
	out := map[string]Item{}
	for rows.Next() {
		ptrs := make([]any, len(allCols))
		raw := make([]sql.RawBytes, len(allCols))
		for i := range ptrs {
			ptrs[i] = &raw[i]
		}
		if err := rows.Scan(ptrs...); err != nil {
			return nil, err
		}
		item := Item{Record: map[string]any{}, Highlights: []Highlight{}}
		id := ""
		for i, c := range allCols {
			if _, isHl := hlIdx[i]; isHl {
				continue
			}
			if c == "id" {
				id = string(raw[i])
			}
			if raw[i] == nil {
				item.Record[c] = nil
			} else {
				item.Record[c] = string(raw[i])
			}
		}
		if clause.Highlight {
			for i, n := range hlIdx {
				if raw[i] == nil {
					continue
				}
				frags := splitHighlight(string(raw[i]))
				if len(frags) > 0 {
					item.Highlights = append(item.Highlights, Highlight{Field: clause.Fields[n], Fragments: frags})
				}
			}
			sort.Slice(item.Highlights, func(a, b int) bool {
				return highlightFieldPosition(clause.Fields, item.Highlights[a].Field) <
					highlightFieldPosition(clause.Fields, item.Highlights[b].Field)
			})
		}
		out[id] = item
	}
	return out, rows.Err()
}

// searchHybrid runs the hybrid mode (queries-and-search.md): the vector
// and text legs run over an enlarged candidate window — the candidate
// budget itself — and the two ranked lists are fused with reciprocal rank
// fusion, score = sum of 1/(k+rank) over the legs where the record
// appears, k from the required fusion clause (kind rrf only, 1..1000).
// "RRF combines ranks rather than incomparable raw magnitudes"; stable
// ties use record ID and the metric is reported as rrf.
func searchHybrid(ctx context.Context, target *sql.DB, collection string, req *Request, limit, budget int, start time.Time, dbID string, generation, schemaRevision int64, offset int, boundHead int64, policyWhere string, policyArgs []any) (*Result, error) {
	if req.Vector == nil {
		return nil, fmt.Errorf("%w: hybrid mode requires the vector clause", ErrInvalidRequest)
	}
	if req.Text == nil {
		return nil, fmt.Errorf("%w: hybrid mode requires the text clause", ErrInvalidRequest)
	}
	if req.Fusion == nil {
		return nil, fmt.Errorf("%w: hybrid mode requires the fusion clause", ErrInvalidRequest)
	}
	if req.Fusion.Kind != "rrf" {
		return nil, fmt.Errorf("%w: unknown fusion kind %q (only rrf is supported)", ErrInvalidRequest, req.Fusion.Kind)
	}
	if req.Fusion.K < 1 || req.Fusion.K > 1000 {
		return nil, fmt.Errorf("%w: fusion k out of range (1..1000)", ErrInvalidRequest)
	}

	// Snapshot binding: a continuation must observe the same snapshot.
	head, herr := snapshotHead(ctx, target)
	if herr != nil {
		return nil, herr
	}
	if boundHead >= 0 && head != boundHead {
		return nil, ErrCursorExpired
	}

	// Enlarged candidate window: both legs fetch up to the candidate
	// budget. When the authorized domain fits the budget each leg ranks
	// the full domain and the fusion is exact; beyond it the legs truncate
	// at the budget and the result is reported incomplete.
	window := budget
	vectorRes, err := searchVector(ctx, target, collection, req, window, budget, start, dbID, generation, schemaRevision, 0, head, policyWhere, policyArgs)
	if err != nil {
		return nil, err
	}
	textRes, err := searchText(ctx, target, collection, req, window, budget, start, dbID, generation, schemaRevision, 0, head, policyWhere, policyArgs)
	if err != nil {
		return nil, err
	}

	items, more := fuseRRF(vectorRes.Items, textRes.Items, req.Fusion.K, limit+offset)
	// Page the fused order (the fused ranking is recomputed per request;
	// the snapshot head binds the continuation).
	if offset < len(items) {
		items = items[offset:]
	} else {
		items = []Item{}
	}
	pagedMore := len(items) > limit
	if pagedMore {
		items = items[:limit]
	}
	complete := vectorRes.ScannedCount <= budget && textRes.ScannedCount <= budget && !more
	res := &Result{
		DatabaseID:     dbID,
		Generation:     generation,
		SchemaRevision: schemaRevision,
		Items:          items,
		Mode:           "hybrid",
		Metric:         strPtr("rrf"),
		Complete:       complete,
		ScannedCount:   vectorRes.ScannedCount,
		ElapsedMs:      float64(time.Since(start).Milliseconds()),
		snapHead:       head,
		moreHint:       more,
	}
	return res, nil
}

// fuseRRF fuses the two legs' ranked lists (each ordered best-first) with
// reciprocal rank fusion. Rank is 1-based within each leg. Records are
// identified by their record ID ("Stable ties use record ID"); ordering
// is fused score descending, then record ID ascending. The text leg's
// authorized highlights carry over to fused records.
func fuseRRF(vectorItems, textItems []Item, k, limit int) ([]Item, bool) {
	type fused struct {
		key    string
		record map[string]any
		score  float64
		hl     []Highlight
	}
	byKey := map[string]*fused{}
	add := func(items []Item) {
		for rank, it := range items {
			key := recordKey(it.Record)
			f, ok := byKey[key]
			if !ok {
				f = &fused{key: key, record: it.Record, hl: []Highlight{}}
				byKey[key] = f
			}
			f.score += 1.0 / float64(k+rank+1)
			if len(it.Highlights) > 0 && len(f.hl) == 0 {
				f.hl = it.Highlights
			}
		}
	}
	add(vectorItems)
	add(textItems)

	all := make([]*fused, 0, len(byKey))
	for _, f := range byKey {
		all = append(all, f)
	}
	sort.Slice(all, func(i, j int) bool {
		if all[i].score != all[j].score {
			return all[i].score > all[j].score
		}
		return all[i].key < all[j].key
	})

	more := len(all) > limit
	if more {
		all = all[:limit]
	}
	items := make([]Item, 0, len(all))
	for _, f := range all {
		items = append(items, Item{Record: f.record, Score: Score{Kind: "rrf", Value: f.score}, Highlights: f.hl})
	}
	return items, more
}

// recordKey builds the stable fusion/tie-break identity of a record: the
// record ID when present, otherwise the record's canonical JSON (sorted
// keys), which is deterministic.
func recordKey(rec map[string]any) string {
	if v, ok := rec["id"]; ok && v != nil {
		return fmt.Sprintf("%v", v)
	}
	b, err := json.Marshal(rec)
	if err != nil {
		return fmt.Sprintf("%v", rec)
	}
	return string(b)
}

func strPtr(s string) *string { return &s }

// --- fuzzy mode ---------------------------------------------------------

// Fuzzy algorithms (wire-schema-reference.md SearchRequest fuzzy variant;
// queries-and-search.md: "Fuzzy algorithms declare distance versus
// similarity and their allowed thresholds"). Expensive cross-record
// comparisons are prefilters and budget bounded: a length-gap prefilter,
// a row-minimum DP bailout against the threshold-implied distance bound,
// the candidate row budget and the request deadline.
var fuzzyAlgorithms = map[string]bool{
	"levenshtein":  true,
	"damerau":      true,
	"osa":          true,
	"hamming":      true,
	"jaro_winkler": true,
	"soundex":      true,
}

// similarityAlgorithms declare similarity rather than distance: they
// accept minimumSimilarity, not maximumDistance.
var similarityAlgorithms = map[string]bool{
	"jaro_winkler": true,
	"soundex":      true,
}

// hammingLengthMismatch is the distance sentinel for hamming pairs of
// different lengths (hamming is defined for equal-length strings only);
// it exceeds every allowed threshold so such pairs never match, and it is
// finite so the score stays JSON-encodable.
const hammingLengthMismatch = 1e12

// fuzzyMaxCompareLen is the per-string comparison budget in runes
// (queries-and-search.md: "Expensive cross-record comparisons need
// prefilters and budgets"). Values longer than it end the scan with
// complete=false rather than unbounding a quadratic comparison; queries
// longer than it are invalid_request.
const fuzzyMaxCompareLen = 1024

// validateFuzzyClause applies the wire contract to the fuzzy clause.
func validateFuzzyClause(req *Request) (*FuzzyClause, error) {
	c := req.Fuzzy
	if c == nil {
		return nil, fmt.Errorf("%w: fuzzy mode requires field, query and algorithm", ErrInvalidRequest)
	}
	if c.Field == "" {
		return nil, fmt.Errorf("%w: fuzzy mode requires a field", ErrInvalidRequest)
	}
	if c.Query == "" {
		return nil, fmt.Errorf("%w: empty fuzzy query", ErrInvalidRequest)
	}
	if !fuzzyAlgorithms[c.Algorithm] {
		return nil, fmt.Errorf("%w: unknown fuzzy algorithm %q", ErrInvalidRequest, c.Algorithm)
	}
	if utf8.RuneCountInString(c.Query) > fuzzyMaxCompareLen {
		return nil, fmt.Errorf("%w: fuzzy query exceeds the comparison budget (%d runes)", ErrInvalidRequest, fuzzyMaxCompareLen)
	}
	set := 0
	if c.MaximumDistance != nil {
		set++
	}
	if c.MinimumSimilarity != nil {
		set++
	}
	if set == 0 {
		return nil, fmt.Errorf("%w: fuzzy mode requires maximumDistance or minimumSimilarity", ErrInvalidRequest)
	}
	if set == 2 {
		return nil, fmt.Errorf("%w: maximumDistance and minimumSimilarity are mutually exclusive", ErrInvalidRequest)
	}
	if c.MaximumDistance != nil {
		if similarityAlgorithms[c.Algorithm] {
			return nil, fmt.Errorf("%w: algorithm %q declares similarity; use minimumSimilarity", ErrInvalidRequest, c.Algorithm)
		}
		if *c.MaximumDistance < 0 || *c.MaximumDistance > 1000 {
			return nil, fmt.Errorf("%w: maximumDistance out of range (0..1000)", ErrInvalidRequest)
		}
	}
	if c.MinimumSimilarity != nil && (*c.MinimumSimilarity < 0 || *c.MinimumSimilarity > 1) {
		return nil, fmt.Errorf("%w: minimumSimilarity out of range (0..1)", ErrInvalidRequest)
	}
	return c, nil
}

// searchFuzzy runs the fuzzy mode as a Go-side scan over the named field,
// bounded by the candidate budget (rows scanned) and the request deadline,
// with deterministic ordering by distance then record ID. The engine has
// no edit-distance SQL functions; the scan is a plain authorized SELECT
// filtered by safeFilter and the policy predicate, and the comparison runs
// here. Budget or deadline exhaustion reports complete=false, never a
// deceptively complete list.
func searchFuzzy(ctx context.Context, target *sql.DB, collection string, req *Request, limit, budget int, start time.Time, dbID string, generation, schemaRevision int64, offset int, boundHead int64, policyWhere string, policyArgs []any) (*Result, error) {
	clause, err := validateFuzzyClause(req)
	if err != nil {
		return nil, err
	}
	alg := clause.Algorithm
	query := []rune(clause.Query)

	filter, args, err := safeFilter(req.Filter)
	if err != nil {
		return nil, err
	}
	args = append(append([]any{}, args...), policyArgs...)
	where, werr := composeWhere(filter, policyWhere)
	if werr != nil {
		return nil, werr
	}

	// Snapshot binding for a continuation.
	head, herr := snapshotHead(ctx, target)
	if herr != nil {
		return nil, herr
	}
	if boundHead >= 0 && head != boundHead {
		return nil, ErrCursorExpired
	}

	// The authorized domain size decides completeness up front; the scan
	// itself is capped at the candidate budget.
	var total int
	if err := target.QueryRowContext(ctx,
		fmt.Sprintf(`SELECT count(*) FROM %s%s`, quoteIdent(collection), where), args...).Scan(&total); err != nil {
		return nil, err
	}
	scanLimit := budget
	if scanLimit < 1 {
		scanLimit = 1
	}
	rows, err := target.QueryContext(ctx,
		fmt.Sprintf(`SELECT * FROM %s%s LIMIT %d`, quoteIdent(collection), where, scanLimit), args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	cols, err := rows.Columns()
	if err != nil {
		return nil, err
	}
	fieldIdx := -1
	wantField := strings.ToLower(clause.Field)
	for i, c := range cols {
		if strings.ToLower(c) == wantField {
			fieldIdx = i
		}
	}
	if fieldIdx < 0 {
		return nil, fmt.Errorf("%w: unknown fuzzy field %q", ErrInvalidRequest, clause.Field)
	}

	type fuzzyHit struct {
		record map[string]any
		key    string
		dist   float64
	}
	hits := []fuzzyHit{}
	scanned := 0
	oversized := false
	scratch := &fuzzyScratch{da: map[rune]int{}}
	for rows.Next() {
		if ctxErr := ctx.Err(); ctxErr != nil {
			// Deadline fired mid-scan: stop and report incomplete.
			break
		}
		ptrs := make([]any, len(cols))
		raw := make([]sql.RawBytes, len(cols))
		for i := range ptrs {
			ptrs[i] = &raw[i]
		}
		if err := rows.Scan(ptrs...); err != nil {
			return nil, err
		}
		scanned++
		if raw[fieldIdx] == nil {
			continue // NULL never fuzzy-matches
		}
		value := string(raw[fieldIdx])
		if utf8.RuneCountInString(value) > fuzzyMaxCompareLen {
			// Beyond the comparison budget: skipped, and the result is
			// reported incomplete rather than deceptively complete.
			oversized = true
			continue
		}
		record := map[string]any{}
		for i, c := range cols {
			if raw[i] == nil {
				record[c] = nil
			} else {
				record[c] = string(raw[i])
			}
		}
		dist, keep := fuzzyDistance(scratch, alg, clause, query, value)
		if !keep {
			continue
		}
		hits = append(hits, fuzzyHit{record: record, key: recordKey(record), dist: dist})
	}
	scanErr := rows.Err()
	deadlineHit := scanErr != nil && ctx.Err() != nil
	if scanErr != nil && !deadlineHit {
		return nil, scanErr
	}

	sort.Slice(hits, func(i, j int) bool {
		if hits[i].dist != hits[j].dist {
			return hits[i].dist < hits[j].dist
		}
		return hits[i].key < hits[j].key
	})

	// Page the deterministic ranking (a continuation re-scans the same
	// snapshot and skips the already-ranked prefix in Go).
	totalHits := len(hits)
	more := totalHits > offset+limit
	if offset < totalHits {
		hits = hits[offset:minInt(offset+limit, totalHits)]
	} else {
		hits = nil
	}
	res := &Result{
		DatabaseID:     dbID,
		Generation:     generation,
		SchemaRevision: schemaRevision,
		Items:          make([]Item, 0, len(hits)),
		Mode:           "fuzzy",
		Metric:         strPtr(alg),
		Complete:       !deadlineHit && !oversized && scanned >= total && !more,
		ScannedCount:   scanned,
		ElapsedMs:      float64(time.Since(start).Milliseconds()),
		snapHead:       head,
		moreHint:       more,
	}
	for _, h := range hits {
		res.Items = append(res.Items, Item{
			Record:     h.record,
			Score:      Score{Kind: "distance:" + alg, Value: h.dist},
			Highlights: []Highlight{},
		})
	}
	return res, nil
}

// fuzzyDistance compares one record value against the query under the
// clause and reports whether the pair passes the requested threshold.
// Field matching is WORD-WISE (the route contract: a query matches when
// the closest word of the field passes the threshold): distance
// algorithms take the minimum passing distance over the field's words and
// the whole value; similarity algorithms keep the closest word's
// distance-equivalent (1 - similarity, and 0/1 for the binary soundex),
// so ordering stays smaller-is-closer.
func fuzzyDistance(s *fuzzyScratch, alg string, clause *FuzzyClause, query []rune, value string) (float64, bool) {
	candidates := []string{value}
	for _, w := range strings.Fields(value) {
		if w != value {
			candidates = append(candidates, w)
		}
	}
	best := 0.0
	kept := false
	for _, cand := range candidates {
		d, ok := fuzzyDistanceSingle(s, alg, clause, query, cand)
		if !ok {
			continue
		}
		if !kept || d < best {
			best, kept = d, true
		}
	}
	return best, kept
}

// fuzzyDistanceSingle compares one candidate string (the whole value or a
// single word of it) against the query.
func fuzzyDistanceSingle(s *fuzzyScratch, alg string, clause *FuzzyClause, query []rune, value string) (float64, bool) {
	vr := []rune(value)
	switch alg {
	case "jaro_winkler":
		sim := jaroWinkler(query, vr)
		return 1 - sim, sim >= *clause.MinimumSimilarity
	case "soundex":
		if soundex(clause.Query) == soundex(value) {
			return 0, true
		}
		return 1, 0 >= *clause.MinimumSimilarity
	}

	// Distance algorithms. maximumDistance is an explicit bound;
	// minimumSimilarity converts to the normalized bound
	// dist <= ceil((1 - s) * maxLen) through sim = 1 - dist/maxLen. The
	// bound is integral so boundary comparisons stay deterministic.
	intBound := 0
	if clause.MaximumDistance != nil {
		intBound = *clause.MaximumDistance
	} else {
		maxLen := len(query)
		if len(vr) > maxLen {
			maxLen = len(vr)
		}
		intBound = int(math.Ceil((1 - *clause.MinimumSimilarity) * float64(maxLen)))
	}
	// Length-gap prefilter: any edit path costs at least |Δlen|.
	if absLen(len(query), len(vr)) > intBound {
		return 0, false
	}

	var d int
	var ok bool
	switch alg {
	case "levenshtein":
		d, ok = levenshteinBounded(query, vr, intBound)
	case "osa":
		d, ok = osaBounded(query, vr, intBound)
	case "damerau":
		d, ok = s.damerauBounded(query, vr, intBound)
	case "hamming":
		if len(query) != len(vr) {
			return hammingLengthMismatch, false
		}
		d = 0
		for i := range query {
			if query[i] != vr[i] {
				d++
			}
		}
		ok = true
	default:
		return 0, false
	}
	if !ok || d > intBound {
		return 0, false
	}
	return float64(d), true
}

func absLen(a, b int) int {
	if a > b {
		return a - b
	}
	return b - a
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// levenshteinBounded computes the Levenshtein distance, abandoning (and
// reporting !ok) once the row minimum exceeds the bound: every edit path
// accumulates non-negative cost through each row, so the final distance is
// at least the current row minimum.
func levenshteinBounded(a, b []rune, bound int) (int, bool) {
	la, lb := len(a), len(b)
	if la == 0 {
		return lb, lb <= bound
	}
	if lb == 0 {
		return la, la <= bound
	}
	prev := make([]int, lb+1)
	curr := make([]int, lb+1)
	for j := 0; j <= lb; j++ {
		prev[j] = j
	}
	for i := 1; i <= la; i++ {
		curr[0] = i
		rowMin := i
		for j := 1; j <= lb; j++ {
			cost := 1
			if a[i-1] == b[j-1] {
				cost = 0
			}
			v := minInt(minInt(prev[j]+1, curr[j-1]+1), prev[j-1]+cost)
			curr[j] = v
			if v < rowMin {
				rowMin = v
			}
		}
		if rowMin > bound {
			return rowMin, false
		}
		prev, curr = curr, prev
	}
	return prev[lb], prev[lb] <= bound
}

// osaBounded computes the optimal-string-alignment distance (the
// restricted Damerau-Levenshtein: adjacent transpositions, no substring
// edited twice).
func osaBounded(a, b []rune, bound int) (int, bool) {
	la, lb := len(a), len(b)
	d := make([]int, lb+1)
	prev2 := make([]int, lb+1) // row i-2
	prev := make([]int, lb+1)  // row i-1
	for j := 0; j <= lb; j++ {
		prev[j] = j
		prev2[j] = j
	}
	d[0] = 0
	for i := 1; i <= la; i++ {
		d[0] = i
		rowMin := i
		for j := 1; j <= lb; j++ {
			cost := 1
			if a[i-1] == b[j-1] {
				cost = 0
			}
			v := minInt(minInt(prev[j]+1, d[j-1]+1), prev[j-1]+cost)
			if i > 1 && j > 1 && a[i-1] == b[j-2] && a[i-2] == b[j-1] {
				if t := prev2[j-2] + 1; t < v {
					v = t
				}
			}
			d[j] = v
			if v < rowMin {
				rowMin = v
			}
		}
		if rowMin > bound {
			return rowMin, false
		}
		copy(prev2, prev)
		copy(prev, d)
	}
	return prev[lb], prev[lb] <= bound
}

// fuzzyScratch carries the Damerau-Levenshtein offset matrix and
// last-occurrence table between records so the scan does not reallocate
// them per record. The matrix is only grown.
type fuzzyScratch struct {
	rows [][]int
	da   map[rune]int
}

// damerauBounded computes the unrestricted (true) Damerau-Levenshtein
// distance (Lowrance-Wagner) with the last-occurrence table, bounded like
// the other edit distances. Offset row r holds the border row (r = 0),
// C[0][*] (r = 1) or C[r-1][*] (r >= 1) of the conceptual matrix C.
func (s *fuzzyScratch) damerauBounded(a, b []rune, bound int) (int, bool) {
	la, lb := len(a), len(b)
	if la == 0 {
		return lb, lb <= bound
	}
	if lb == 0 {
		return la, la <= bound
	}
	maxdist := la + lb
	rowsWanted := la + 2
	for len(s.rows) < rowsWanted {
		s.rows = append(s.rows, nil)
	}
	for r := 0; r < rowsWanted; r++ {
		if len(s.rows[r]) < lb+2 {
			s.rows[r] = make([]int, lb+2)
		}
		row := s.rows[r][:lb+2]
		for j := range row {
			row[j] = maxdist
		}
		if r == 1 {
			for j := 0; j <= lb; j++ {
				row[j+1] = j
			}
		}
	}
	clear(s.da)
	for i := 1; i <= la; i++ {
		row := s.rows[i+1][:lb+2]
		row[1] = i
		up := s.rows[i][:lb+2] // offset row i: C[i-1][*]
		db := 0
		rowMin := i
		for j := 1; j <= lb; j++ {
			k := s.da[b[j-1]]
			l := db
			cost := 1
			if a[i-1] == b[j-1] {
				cost = 0
				db = j
			}
			v := up[j] + cost // d[i][j] = C[i-1][j-1]
			if w := up[j+1] + 1; w < v {
				v = w
			}
			if w := row[j] + 1; w < v {
				v = w
			}
			// d[k][l]: offset row k, where k is the 1-based row of the
			// last occurrence of b[j] in a[1..i-1] (0 when none).
			if t := s.rows[k][l] + (i - k - 1) + 1 + (j - l - 1); t < v {
				v = t
			}
			row[j+1] = v
			if v < rowMin {
				rowMin = v
			}
		}
		if rowMin > bound {
			return rowMin, false
		}
		s.da[a[i-1]] = i
	}
	return s.rows[la+1][lb+1], s.rows[la+1][lb+1] <= bound
}

// jaroWinkler is the classic Jaro similarity with the Winkler prefix
// boost (applied when jaro exceeds the standard 0.7 boost threshold,
// prefix up to 4 characters).
func jaroWinkler(a, b []rune) float64 {
	la, lb := len(a), len(b)
	if la == 0 && lb == 0 {
		return 1
	}
	if la == 0 || lb == 0 {
		return 0
	}
	window := maxInt(la, lb)/2 - 1
	if window < 0 {
		window = 0
	}
	amatch := make([]bool, la)
	bmatch := make([]bool, lb)
	matches := 0
	for i := 0; i < la; i++ {
		lo := i - window
		if lo < 0 {
			lo = 0
		}
		hi := i + window
		if hi >= lb {
			hi = lb - 1
		}
		for j := lo; j <= hi; j++ {
			if !bmatch[j] && a[i] == b[j] {
				amatch[i] = true
				bmatch[j] = true
				matches++
				break
			}
		}
	}
	if matches == 0 {
		return 0
	}
	half := 0
	k := 0
	for i := 0; i < la; i++ {
		if amatch[i] {
			for !bmatch[k] {
				k++
			}
			if a[i] != b[k] {
				half++
			}
			k++
		}
	}
	m := float64(matches)
	t := float64(half) / 2
	jaro := (m/float64(la) + m/float64(lb) + (m-t)/m) / 3
	if jaro > 0.7 {
		prefix := 0
		for i := 0; i < la && i < lb && i < 4 && a[i] == b[i]; i++ {
			prefix++
		}
		jaro += float64(prefix) * 0.1 * (1 - jaro)
	}
	return jaro
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

// soundex is the classic American Soundex code (letter + three digits):
// first letter kept; b/f/p/v→1, c/g/j/k/q/s/x/z→2, d/t→3, l→4, m/n→5,
// r→6; a/e/i/o/u/y separate identical codes; h/w do not; identical
// adjacent codes coded once; padded or truncated to three digits.
func soundex(s string) string {
	up := []rune(strings.ToUpper(s))
	out := make([]rune, 0, 4)
	prev := byte(0)
	first := true
	for _, r := range up {
		if r < 'A' || r > 'Z' {
			first = false // non-letter acts as a separator
			prev = 0
			continue
		}
		c := soundexCode(r)
		if first {
			out = append(out, r)
			first = false
			prev = c
			continue
		}
		if c == 0 {
			if r != 'H' && r != 'W' {
				prev = 0 // vowels separate identical codes
			}
			continue
		}
		if c != prev {
			out = append(out, rune(c))
			prev = c
		}
	}
	if len(out) == 0 {
		return ""
	}
	for len(out) < 4 {
		out = append(out, '0')
	}
	return string(out[:4])
}

func soundexCode(r rune) byte {
	switch r {
	case 'B', 'F', 'P', 'V':
		return '1'
	case 'C', 'G', 'J', 'K', 'Q', 'S', 'X', 'Z':
		return '2'
	case 'D', 'T':
		return '3'
	case 'L':
		return '4'
	case 'M', 'N':
		return '5'
	case 'R':
		return '6'
	}
	return 0
}

// findFTSIndex locates the FTS index covering the collection and the
// requested fields from sqlite_master's index-method DDL.
func findFTSIndex(ctx context.Context, target *sql.DB, collection string, fields []string) (string, error) {
	rows, err := target.QueryContext(ctx,
		`SELECT name, sql FROM sqlite_master WHERE type = 'index' AND tbl_name = ? AND sql LIKE '%USING fts%'`,
		collection)
	if err != nil {
		return "", err
	}
	defer rows.Close()
	wanted := map[string]bool{}
	for _, f := range fields {
		wanted[strings.ToLower(f)] = true
	}
	for rows.Next() {
		var name, ddl string
		if err := rows.Scan(&name, &ddl); err != nil {
			return "", err
		}
		// Canonical DDL quotes identifiers; strip quotes and match the
		// field names as words.
		plain := strings.Map(func(r rune) rune {
			if r == '"' || r == '`' || r == '[' || r == ']' {
				return -1
			}
			return r
		}, strings.ToLower(ddl))
		covered := 0
		for f := range wanted {
			for _, tok := range strings.Fields(plain) {
				tok = strings.Trim(tok, "(),")
				if tok == f || strings.HasSuffix(tok, ")") && strings.Trim(tok, ")") == f {
					covered++
					break
				}
			}
		}
		if covered == len(wanted) {
			return name, nil
		}
	}
	if err := rows.Err(); err != nil {
		return "", err
	}
	return "", fmt.Errorf("%w: collection %q has no native FTS index covering the requested fields; create one through the index maintenance action", ErrNotIndexed, collection)
}

// FTSIndexState is the verified outcome of the FTS index maintenance
// action ("Index creation/rebuild is a durable job with old index serving
// or explicit unavailable status"). State is "ready" only when the stored
// index was verified to cover the requested fields; a failed rebuild
// reports "unavailable" (and text search then fails with schema_mismatch
// until the index is restored).
type FTSIndexState struct {
	IndexName string   `json:"index"`
	State     string   `json:"state"`
	Fields    []string `json:"fields"`
	Rebuilt   bool     `json:"rebuilt"`
	Verified  bool     `json:"verified"`
	Error     string   `json:"error,omitempty"`
}

// ftsIndexDef is the parsed definition of a stored index DDL.
type ftsIndexDef struct {
	name      string
	table     string
	ddl       string
	fields    []string
	tokenizer string
	weights   string
	isFTS     bool
}

// EnsureFTSIndex performs the FTS index maintenance for a collection's
// fields with the tokenizer/weights configuration, VERIFYING what already
// exists instead of silently accepting it: a same-name index that covers
// exactly the requested fields (and tokenizer/weights) is left serving
// (old index serving, state "ready"); a same-name index covering anything
// else — including a same-name index on another table or a non-FTS index
// — is rebuilt (dropped and recreated); when the rebuild fails the state
// is "unavailable" with the error, never a deceptively ready index.
func EnsureFTSIndex(ctx context.Context, target *sql.DB, collection, indexName string, fields []string, tokenizer string, weights map[string]float64) (*FTSIndexState, error) {
	if len(fields) == 0 {
		return nil, fmt.Errorf("%w: the index requires at least one field", ErrInvalidRequest)
	}
	// Tokenizer whitelist (SUPPORTED_TOKENIZERS).
	switch tokenizer {
	case "", "default", "en_stem", "raw", "simple", "whitespace", "ngram":
	default:
		return nil, fmt.Errorf("%w: unsupported tokenizer %q", ErrInvalidRequest, tokenizer)
	}

	build := func(fields []string) string {
		cols := make([]string, 0, len(fields))
		for _, f := range fields {
			cols = append(cols, quoteIdent(f))
		}
		with := ""
		if tokenizer != "" && tokenizer != "default" {
			with = fmt.Sprintf(` WITH (tokenizer = '%s'`, tokenizer)
			if len(weights) > 0 {
				parts := make([]string, 0, len(weights))
				for f, w := range weights {
					parts = append(parts, fmt.Sprintf("%s=%.1f", f, w))
				}
				sort.Strings(parts)
				with += fmt.Sprintf(", weights = '%s'", strings.Join(parts, ","))
			}
			with += ")"
		}
		return fmt.Sprintf(`CREATE INDEX %s ON %s USING fts (%s)%s`,
			quoteIdent(indexName), quoteIdent(collection), strings.Join(cols, ", "), with)
	}

	// The stored definition, when an index of this name already exists
	// (index names are schema-global: the stored table can even differ).
	existing, derr := readIndexDef(ctx, target, indexName)
	if derr != nil {
		return nil, derr
	}

	if existing != nil && existing.table == strings.ToLower(collection) && existing.isFTS &&
		indexCovers(existing.fields, fields) &&
		effectiveTokenizer(existing.tokenizer) == effectiveTokenizer(tokenizer) &&
		existing.weights == ftsWeightsString(weights) {
		// Old index serving: the stored index covers exactly the requested
		// configuration; report it verified rather than recreating it.
		return &FTSIndexState{
			IndexName: indexName,
			State:     "ready",
			Fields:    fields,
			Verified:  true,
		}, nil
	}

	if existing != nil {
		// Rebuild: the same name must end up covering the requested
		// fields on this collection.
		if _, err := target.ExecContext(ctx, fmt.Sprintf(`DROP INDEX %s`, quoteIdent(indexName))); err != nil {
			return &FTSIndexState{
				IndexName: indexName,
				State:     "unavailable",
				Fields:    fields,
				Error:     fmt.Sprintf("rebuild dropped nothing: %v", err),
			}, nil
		}
	}
	if _, err := target.ExecContext(ctx, build(fields)); err != nil {
		// Explicit unavailable status: the index is NOT serving (a rebuilt
		// index may be gone), so text search will fail with schema_mismatch
		// until the maintenance is redone — never a stale "ready".
		return &FTSIndexState{
			IndexName: indexName,
			State:     "unavailable",
			Fields:    fields,
			Rebuilt:   existing != nil,
			Error:     err.Error(),
		}, nil
	}

	// Verify what was written: the stored DDL must now cover the requested
	// fields before the state is "ready".
	stored, verr := readIndexDef(ctx, target, indexName)
	if verr != nil {
		return &FTSIndexState{
			IndexName: indexName,
			State:     "unavailable",
			Fields:    fields,
			Error:     verr.Error(),
		}, nil
	}
	if stored == nil || stored.table != strings.ToLower(collection) || !stored.isFTS || !indexCovers(stored.fields, fields) {
		return &FTSIndexState{
			IndexName: indexName,
			State:     "unavailable",
			Fields:    fields,
			Error:     "the created index does not cover the requested fields",
		}, nil
	}
	return &FTSIndexState{
		IndexName: indexName,
		State:     "ready",
		Fields:    fields,
		Rebuilt:   existing != nil,
		Verified:  true,
	}, nil
}

// hlOpenTag/hlCloseTag are the highlight sentinels the engine wraps
// matches with and the Go-side splitter strips — real control bytes, since
// SQL string literals carry no \x escapes.
const (
	hlOpenTag  = "\x1fB\x1f"
	hlCloseTag = "\x1f/b\x1f"
)

// effectiveTokenizer normalizes the request tokenizer for comparison
// against a stored DDL (an absent WITH clause means the engine default).
func effectiveTokenizer(tokenizer string) string {
	if tokenizer == "" {
		return "default"
	}
	return tokenizer
}

// ftsWeightsString canonicalizes the requested index weights for
// comparison against a stored DDL (sorted field=weight pairs).
func ftsWeightsString(weights map[string]float64) string {
	if len(weights) == 0 {
		return ""
	}
	parts := make([]string, 0, len(weights))
	for f, w := range weights {
		parts = append(parts, fmt.Sprintf("%s=%.1f", f, w))
	}
	sort.Strings(parts)
	return strings.Join(parts, ",")
}

// readIndexDef loads and parses the stored DDL of one index by name from
// sqlite_master (nil when no such index exists).
func readIndexDef(ctx context.Context, target *sql.DB, indexName string) (*ftsIndexDef, error) {
	var name, tblName string
	var sqlStr sql.NullString
	err := target.QueryRowContext(ctx,
		`SELECT name, tbl_name, sql FROM sqlite_master WHERE type = 'index' AND name = ?`,
		indexName).Scan(&name, &tblName, &sqlStr)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	def := &ftsIndexDef{name: name, table: strings.ToLower(tblName), ddl: sqlStr.String}
	parseIndexDDL(def)
	return def, nil
}

// parseIndexDDL extracts the covered fields, the tokenizer and the
// weights from a stored index DDL: the field list between
// `USING fts (` and its closing parenthesis, and the WITH options after
// it. Quoting/bracketing is stripped the same way findFTSIndex matches.
func parseIndexDDL(def *ftsIndexDef) {
	ddl := strings.ToLower(def.ddl)
	def.isFTS = strings.Contains(ddl, "using fts")
	ftsPos := strings.Index(ddl, "using fts")
	if ftsPos < 0 {
		ftsPos = strings.Index(ddl, "(")
	}
	open := strings.Index(ddl[ftsPos:], "(")
	if open < 0 {
		return
	}
	rest := ddl[ftsPos+open+1:]
	depth := 1
	end := -1
	for i := 0; i < len(rest); i++ {
		switch rest[i] {
		case '(':
			depth++
		case ')':
			depth--
			if depth == 0 {
				end = i
			}
		}
		if end >= 0 {
			break
		}
	}
	if end < 0 {
		return
	}
	colList := rest[:end]
	for _, raw := range strings.Split(colList, ",") {
		f := strings.Map(func(r rune) rune {
			if r == '"' || r == '`' || r == '[' || r == ']' {
				return -1
			}
			return r
		}, strings.TrimSpace(raw))
		if f != "" {
			def.fields = append(def.fields, f)
		}
	}
	// WITH options (tokenizer, weights) after the column list.
	withPart := rest[end+1:]
	if tok := extractDDLOption(withPart, "tokenizer"); tok != "" {
		def.tokenizer = tok
	}
	if w := extractDDLOption(withPart, "weights"); w != "" {
		def.weights = w
	}
}

// extractDDLOption reads one `name = 'value'` option from a WITH clause
// fragment.
func extractDDLOption(withPart, name string) string {
	idx := strings.Index(withPart, name+" = '")
	if idx < 0 {
		idx = strings.Index(withPart, name+"='")
		if idx < 0 {
			return ""
		}
		return withPart[idx+len(name)+2:]
	}
	rest := withPart[idx+len(name)+4:]
	end := strings.Index(rest, "'")
	if end < 0 {
		return ""
	}
	return rest[:end]
}

// indexCovers reports whether the stored field set covers the requested
// field set exactly (order-insensitive, case-insensitive).
func indexCovers(stored, wanted []string) bool {
	if len(stored) != len(wanted) {
		return false
	}
	set := map[string]bool{}
	for _, f := range stored {
		set[strings.ToLower(f)] = true
	}
	for _, f := range wanted {
		if !set[strings.ToLower(f)] {
			return false
		}
	}
	return true
}

// safeFilter translates the caller filter into a conservative
// parameterized predicate. Accepted forms: `field = value`, `field != value`,
// `field > value`, `field < value`, `field >= value`, `field <= value` with
// quoted string or numeric literals. Anything else is invalid_request.
func safeFilter(filter string) (string, []any, error) {
	filter = strings.TrimSpace(filter)
	if filter == "" {
		return "", nil, nil
	}
	ops := []string{">=", "<=", "!=", "=", ">", "<"}
	for _, op := range ops {
		idx := indexOutsideQuotes(filter, op)
		if idx < 0 {
			continue
		}
		field := strings.TrimSpace(filter[:idx])
		value := strings.TrimSpace(filter[idx+len(op):])
		if !isSafeIdent(field) {
			return "", nil, fmt.Errorf("%w: unsafe filter field", ErrInvalidRequest)
		}
		var arg any
		var placeholder = "?"
		if strings.HasPrefix(value, "'") && strings.HasSuffix(value, "'") && len(value) >= 2 {
			arg = strings.ReplaceAll(value[1:len(value)-1], "''", "'")
		} else {
			var num float64
			if _, err := fmt.Sscanf(value, "%g", &num); err != nil {
				return "", nil, fmt.Errorf("%w: filter value must be a quoted string or number", ErrInvalidRequest)
			}
			arg = num
		}
		return quoteIdent(field) + " " + op + " " + placeholder, []any{arg}, nil
	}
	return "", nil, fmt.Errorf("%w: unsupported filter form (field op value)", ErrInvalidRequest)
}

func indexOutsideQuotes(s, sub string) int {
	inQuote := false
	for i := 0; i < len(s); i++ {
		switch s[i] {
		case '\'':
			inQuote = !inQuote
		}
		if !inQuote && strings.HasPrefix(s[i:], sub) {
			return i
		}
	}
	return -1
}

func isSafeIdent(s string) bool {
	if s == "" {
		return false
	}
	for _, r := range s {
		ok := r == '_' || (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9')
		if !ok {
			return false
		}
	}
	return true
}

func quoteIdent(name string) string {
	return `"` + strings.ReplaceAll(name, `"`, `""`) + `"`
}

func quoteLiteral(v string) string {
	return "'" + strings.ReplaceAll(v, "'", "''") + "'"
}
