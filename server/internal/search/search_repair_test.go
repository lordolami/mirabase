package search_test

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"mirabase/internal/search"
)

// TestCursorLeaseBindingAndExpiry proves the cursor lease contract at the
// store level (queries-and-search.md): the lease binds plan hash,
// schema/generation/policy epoch and snapshot head, the default TTL is 60
// seconds (configurable), expiry returns cursor_expired, the global
// maximum bounds active leases, IDs are unique even for rapid same-offset
// issues, and a validated cursor is consumed by its continuation.
func TestCursorLeaseBindingAndExpiry(t *testing.T) {
	// Binding: a mismatched plan/generation/schema/epoch refuses with
	// cursor_expired (never a silent continue against another context).
	// Each mismatch check uses its own lease (a failed validate consumes
	// it).
	for _, c := range []struct {
		name            string
		plan            string
		gen, rev, epoch int64
	}{
		{"wrong plan hash", "plan-b", 1, 1, 1},
		{"wrong generation", "plan-a", 2, 1, 1},
		{"wrong schema revision", "plan-a", 1, 2, 1},
		{"wrong policy epoch", "plan-a", 1, 1, 2},
	} {
		bad := search.NewCursorLeases()
		badID, ierr := bad.Issue("plan-a", 1, 1, 1, 20, 7)
		if ierr != nil {
			t.Fatalf("issue: %v", ierr)
		}
		if _, _, verr := bad.Validate(badID, c.plan, c.gen, c.rev, c.epoch); !errors.Is(verr, search.ErrCursorExpired) {
			t.Fatalf("%s: want cursor_expired, got %v", c.name, verr)
		}
	}
	leases := search.NewCursorLeases()
	id, err := leases.Issue("plan-a", 1, 1, 1, 20, 7)
	if err != nil {
		t.Fatalf("issue: %v", err)
	}
	offset, head, err := leases.Validate(id, "plan-a", 1, 1, 1)
	if err != nil || offset != 20 || head != 7 {
		t.Fatalf("valid continuation: offset=%d head=%d err=%v", offset, head, err)
	}
	// The validated cursor is consumed: reuse (double-submit) fails
	// explicitly instead of re-serving the page from a stale lease.
	if _, _, err := leases.Validate(id, "plan-a", 1, 1, 1); !errors.Is(err, search.ErrCursorExpired) {
		t.Fatalf("reused cursor must be consumed: want cursor_expired, got %v", err)
	}

	// TTL expiry: an expired lease returns cursor_expired.
	fast := search.NewCursorLeasesWith(50*time.Millisecond, 16)
	if _, err := fast.Issue("plan", 1, 1, 1, 0, 0); err != nil {
		t.Fatalf("fast issue: %v", err)
	}
	time.Sleep(80 * time.Millisecond)
	if _, _, err := fast.Validate("cur-1-0", "plan", 1, 1, 1); !errors.Is(err, search.ErrCursorExpired) {
		t.Fatalf("expired lease: want cursor_expired, got %v", err)
	}

	// Global maximum: at most `max` active leases; the excess issue is
	// refused as budget exhaustion (the bounded shared resource).
	tiny := search.NewCursorLeasesWith(time.Minute, 2)
	for i := 0; i < 2; i++ {
		if _, err := tiny.Issue(fmt.Sprintf("plan-%d", i), 1, 1, 1, i, 0); err != nil {
			t.Fatalf("issue %d: %v", i, err)
		}
	}
	if _, err := tiny.Issue("plan-x", 1, 1, 1, 9, 0); !errors.Is(err, search.ErrBudgetExhausted) {
		t.Fatalf("lease over the maximum: want resource_exhausted, got %v", err)
	}

	// IDs are unique even for rapid issues at the same position (a
	// collision would silently overwrite another client's lease).
	rapid := search.NewCursorLeases()
	seen := map[string]bool{}
	for i := 0; i < 8; i++ {
		id, err := rapid.Issue("plan", 1, 1, 1, 0, 0)
		if err != nil {
			t.Fatalf("rapid issue %d: %v", i, err)
		}
		if seen[id] {
			t.Fatalf("duplicate lease id %q", id)
		}
		seen[id] = true
	}
}

// TestCursorContinuationPages proves the end-to-end cursor contract on the
// pinned engine (text mode): a full page issues a nextCursor bound to the
// identical query and snapshot, the continuation resumes at the next sort
// position without repeating or skipping records, the final page completes
// with no cursor, a different query on the same cursor expires it, and an
// advanced snapshot expires it ("do not silently continue against a
// different snapshot").
func TestCursorContinuationPages(t *testing.T) {
	db := openEngineDB(t)
	ctx := context.Background()

	steps := []string{
		`CREATE TABLE cur_pages (id INTEGER PRIMARY KEY, title TEXT, body TEXT)`,
		`CREATE INDEX idx_cur_pages ON cur_pages USING fts (title)`,
	}
	for i := 1; i <= 12; i++ {
		steps = append(steps, fmt.Sprintf(
			`INSERT INTO cur_pages (id, title, body) VALUES (%d, 'word %02d alpha', 'note body %02d')`, i, i, i))
	}
	for _, s := range steps {
		if _, err := db.ExecContext(ctx, s); err != nil {
			t.Fatalf("seed: %v", err)
		}
	}

	req := func(cursor string, query string) *search.Request {
		return &search.Request{
			Mode:  "text",
			Text:  &search.TextClause{Fields: []string{"title"}, Query: query},
			Limit: 5, Cursor: cursor,
		}
	}

	// Page 1: five ordered items plus a continuation cursor.
	res1, err := search.Search(ctx, db, "cur_pages", req("", "word"), search.DefaultOptions(), "db", 1, 1, 1)
	if err != nil {
		t.Fatalf("page1: %v", err)
	}
	if len(res1.Items) != 5 {
		t.Fatalf("page1 items: %d, want 5", len(res1.Items))
	}
	if res1.NextCursor == nil || *res1.NextCursor == "" {
		t.Fatalf("page1 must issue a continuation cursor: %+v", res1.NextCursor)
	}
	if res1.Complete {
		t.Fatalf("page1 of 3 must be incomplete")
	}
	// bm25 ties are ordered by ascending record id (stable ties use
	// record ID), so the pages partition the ids deterministically.
	pageIDs := func(res *search.Result) []string {
		ids := []string{}
		for _, it := range res.Items {
			ids = append(ids, fmt.Sprintf("%v", it.Record["id"]))
		}
		return ids
	}
	ids1 := pageIDs(res1)
	for _, want := range []string{"1", "2", "3", "4", "5"} {
		if !contains(ids1, want) {
			t.Fatalf("page1 ids %v missing %s (ties must order by record id)", ids1, want)
		}
	}

	// Page 2: the continuation resumes at the next sort position.
	res2, err := search.Search(ctx, db, "cur_pages", req(*res1.NextCursor, "word"), search.DefaultOptions(), "db", 1, 1, 1)
	if err != nil {
		t.Fatalf("page2: %v", err)
	}
	if len(res2.Items) != 5 || res2.NextCursor == nil {
		t.Fatalf("page2 shape: %d items, cursor %v", len(res2.Items), res2.NextCursor)
	}
	ids2 := pageIDs(res2)
	for _, got := range ids2 {
		if contains(ids1, got) {
			t.Fatalf("page2 repeated page1 record %s (ids1 %v ids2 %v)", got, ids1, ids2)
		}
	}
	for _, want := range []string{"6", "7", "8", "9", "10"} {
		if !contains(ids2, want) {
			t.Fatalf("page2 ids %v missing %s", ids2, want)
		}
	}

	// Page 3: the last page completes and stops issuing cursors.
	res3, err := search.Search(ctx, db, "cur_pages", req(*res2.NextCursor, "word"), search.DefaultOptions(), "db", 1, 1, 1)
	if err != nil {
		t.Fatalf("page3: %v", err)
	}
	if len(res3.Items) != 2 {
		t.Fatalf("page3 items: %d, want 2", len(res3.Items))
	}
	if res3.NextCursor != nil {
		t.Fatalf("final page must not issue a cursor: %v", *res3.NextCursor)
	}
	if !res3.Complete {
		t.Fatalf("final page must be complete")
	}

	// A different query on the same cursor is refused: the cursor binds
	// the identical query (cursor_expired, not a silent reinterpretation).
	if _, err := search.Search(ctx, db, "cur_pages", req(*res1.NextCursor, "alpha"), search.DefaultOptions(), "db", 1, 1, 1); !errors.Is(err, search.ErrCursorExpired) {
		t.Fatalf("continuation with a different query: want cursor_expired, got %v", err)
	}

	// A different context (policy epoch) is refused the same way.
	if _, err := search.Search(ctx, db, "cur_pages", req(*res1.NextCursor, "word"), search.DefaultOptions(), "db", 1, 1, 2); !errors.Is(err, search.ErrCursorExpired) {
		t.Fatalf("continuation with a different policy epoch: want cursor_expired, got %v", err)
	}

	// An advanced snapshot (a committed write) expires the cursor: the
	// continuation refuses to read across a different snapshot boundary.
	var head int64
	if err := db.QueryRowContext(ctx, `SELECT COALESCE(MAX(commit_seq), 0) FROM _mb_changes`).Scan(&head); err != nil {
		t.Fatalf("read head: %v", err)
	}
	if _, err := db.ExecContext(ctx, `INSERT INTO _mb_changes (commit_seq, transaction_id, generation, schema_revision, source, timestamp)
		VALUES (?, 'tx-expiry', 1, 1, 'record', '2026-01-01T00:00:00Z')`, head+1); err != nil {
		t.Fatalf("advance snapshot: %v", err)
	}
	if _, err := search.Search(ctx, db, "cur_pages", req(*res1.NextCursor, "word"), search.DefaultOptions(), "db", 1, 1, 1); !errors.Is(err, search.ErrCursorExpired) {
		t.Fatalf("continuation after a snapshot advance: want cursor_expired, got %v", err)
	}
}

// TestCursorVectorContinuation proves the same continuation contract for
// exact vector mode, including deterministic tie ordering by record id
// (identical vectors must partition into stable pages).
func TestCursorVectorContinuation(t *testing.T) {
	db := openEngineDB(t)
	ctx := context.Background()
	if _, err := db.ExecContext(ctx,
		`CREATE TABLE cur_vec (id INTEGER PRIMARY KEY, title TEXT, embedding TEXT)`); err != nil {
		t.Fatalf("create: %v", err)
	}
	for i := 1; i <= 12; i++ {
		if _, err := db.ExecContext(ctx,
			fmt.Sprintf(`INSERT INTO cur_vec (id, title, embedding) VALUES (%d, 'doc %02d', '[1,0,0]')`, i, i)); err != nil {
			t.Fatalf("seed %d: %v", i, err)
		}
	}

	req := func(cursor string) *search.Request {
		return &search.Request{
			Mode:   "vector",
			Vector: &search.VectorClause{Field: "embedding", Query: json.RawMessage(`[1,0,0]`), Metric: "cosine"},
			Limit:  5, Cursor: cursor,
		}
	}
	res1, err := search.Search(ctx, db, "cur_vec", req(""), search.DefaultOptions(), "db", 1, 1, 1)
	if err != nil {
		t.Fatalf("page1: %v", err)
	}
	if len(res1.Items) != 5 || res1.NextCursor == nil {
		t.Fatalf("page1 shape: %d items cursor %v", len(res1.Items), res1.NextCursor)
	}
	if got := fmt.Sprintf("%v", res1.Items[0].Record["id"]); got != "1" {
		t.Fatalf("vector ties must order by ascending record id, first=%s", got)
	}
	res2, err := search.Search(ctx, db, "cur_vec", req(*res1.NextCursor), search.DefaultOptions(), "db", 1, 1, 1)
	if err != nil {
		t.Fatalf("page2: %v", err)
	}
	if len(res2.Items) != 5 || res2.NextCursor == nil {
		t.Fatalf("page2 shape: %d items cursor %v", len(res2.Items), res2.NextCursor)
	}
	if got := fmt.Sprintf("%v", res2.Items[0].Record["id"]); got != "6" {
		t.Fatalf("vector continuation must resume at the next sort position, first=%s", got)
	}
	res3, err := search.Search(ctx, db, "cur_vec", req(*res2.NextCursor), search.DefaultOptions(), "db", 1, 1, 1)
	if err != nil {
		t.Fatalf("page3: %v", err)
	}
	if len(res3.Items) != 2 || res3.NextCursor != nil || !res3.Complete {
		t.Fatalf("page3 shape: %d items cursor %v complete %v", len(res3.Items), res3.NextCursor, res3.Complete)
	}
}

// TestUnknownPropertiesRejected proves additionalProperties:false across
// every SearchRequest variant AND inside the clauses (wire-schema
// reference): an unknown request property is a typed invalid_request, not
// a silently ignored key that can mask a client bug into a
// differently-interpreted query.
func TestUnknownPropertiesRejected(t *testing.T) {
	cases := []struct {
		name string
		body string
	}{
		{"unknown top-level key in vector mode", `{"mode":"vector","vector":{"field":"v","query":[1,0],"metric":"cosine"},"sortBy":"x"}`},
		{"vector clause key in text mode", `{"mode":"text","text":{"fields":["t"],"query":"x"},"vector":{"field":"v","query":[1],"metric":"l2"}}`},
		{"unknown top-level key in text mode", `{"mode":"text","text":{"fields":["t"],"query":"x"},"tokenizer":"raw"}`},
		{"unknown key inside the vector clause", `{"mode":"vector","vector":{"field":"v","query":[1],"metric":"l2","dimensions":3}}`},
		{"unknown key inside the text clause", `{"mode":"text","text":{"fields":["t"],"query":"x","indexName":"i"}}`},
		{"unknown key inside the fusion clause", `{"mode":"hybrid","vector":{"field":"v","query":[1],"metric":"l2"},"text":{"fields":["t"],"query":"x"},"fusion":{"kind":"rrf","k":60,"normalize":true}}`},
		{"unknown key inside a weight item", `{"mode":"text","text":{"fields":["t"],"query":"x","weights":[{"field":"t","weight":1,"boost":2}]}}`},
		{"fields projection in the fuzzy variant", `{"mode":"fuzzy","field":"name","query":"kit","algorithm":"levenshtein","maximumDistance":1,"fields":["name"]}`},
		{"nested fuzzy clause outside fuzzy mode", `{"mode":"vector","vector":{"field":"v","query":[1],"metric":"l2"},"fuzzy":{"field":"n","query":"x","algorithm":"osa","maximumDistance":1}}`},
		{"limit below the wire minimum", `{"mode":"text","text":{"fields":["t"],"query":"x"},"limit":0}`},
		{"negative limit", `{"mode":"text","text":{"fields":["t"],"query":"x"},"limit":-3}`},
		{"projection above the wire maximum", fmt.Sprintf(`{"mode":"vector","vector":{"field":"v","query":[1],"metric":"l2"},"fields":[%s]}`, strings.TrimSuffix(strings.Repeat(`"f",`, 101), ","))},
		{"missing mode", `{"text":{"fields":["t"],"query":"x"}}`},
	}
	for _, c := range cases {
		var req search.Request
		err := json.Unmarshal([]byte(c.body), &req)
		if !errors.Is(err, search.ErrInvalidRequest) {
			t.Errorf("%s: want invalid_request, got %v", c.name, err)
		}
	}

	// The valid variants still decode: each mode's exact property set,
	// including the shared cursor/fields/filter/limit properties.
	valid := []string{
		`{"mode":"vector","vector":{"field":"v","query":[1,0],"metric":"cosine"},"limit":10,"filter":"a = 'b'","fields":["a"],"cursor":"cur-1-5"}`,
		`{"mode":"text","text":{"fields":["t"],"query":"x","syntax":"advanced","highlight":true,"weights":[{"field":"t","weight":1.5}]}}`,
		`{"mode":"hybrid","vector":{"field":"v","query":[1],"metric":"l2"},"text":{"fields":["t"],"query":"x"},"fusion":{"kind":"rrf","k":60},"fields":["a"]}`,
		`{"mode":"fuzzy","field":"name","query":"kitten","algorithm":"levenshtein","maximumDistance":2}`,
		`{"mode":"fuzzy","fuzzy":{"field":"name","query":"kitten","algorithm":"osa","maximumDistance":1}}`,
	}
	for _, body := range valid {
		var req search.Request
		if err := json.Unmarshal([]byte(body), &req); err != nil {
			t.Errorf("valid variant must decode: %s -> %v", body, err)
		}
	}
}

// TestWeightedTextRanking proves per-request text weights are APPLIED in
// the ranking (queries-and-search.md text clause): the score of a record
// is the weight sum of its per-field engine scores, the ordering follows
// the weights (flipping the weights flips the order of two otherwise
// identical single-field matches), and weight validation is typed.
func TestWeightedTextRanking(t *testing.T) {
	db := openEngineDB(t)
	ctx := context.Background()
	steps := []string{
		`CREATE TABLE wtx (id INTEGER PRIMARY KEY, title TEXT, body TEXT)`,
		`CREATE INDEX idx_wtx ON wtx USING fts (title, body)`,
		`INSERT INTO wtx (id, title, body) VALUES (1, 'alpha', 'zzz')`,
		`INSERT INTO wtx (id, title, body) VALUES (2, 'zzz', 'alpha')`,
	}
	for _, s := range steps {
		if _, err := db.ExecContext(ctx, s); err != nil {
			t.Fatalf("seed: %v", err)
		}
	}

	run := func(titleWeight, bodyWeight float64) *search.Result {
		t.Helper()
		req := &search.Request{
			Mode: "text",
			Text: &search.TextClause{
				Fields:  []string{"title", "body"},
				Query:   "alpha",
				Weights: []search.TextWeight{{Field: "title", Weight: titleWeight}, {Field: "body", Weight: bodyWeight}},
			},
			Limit: 10,
		}
		res, err := search.Search(ctx, db, "wtx", req, search.DefaultOptions(), "db", 1, 1, 1)
		if err != nil {
			t.Fatalf("weighted search tw=%v bw=%v: %v", titleWeight, bodyWeight, err)
		}
		return res
	}

	// Title-weighted: the title match outranks the body match.
	res := run(3, 1)
	if got := fmt.Sprintf("%v", res.Items[0].Record["id"]); got != "1" {
		t.Fatalf("title-weighted order: first=%v (items %v)", got, res.Items)
	}
	if !res.Complete {
		t.Fatalf("weighted search over a small domain must be complete")
	}

	// Body-weighted: the same two records reverse.
	res = run(1, 3)
	if got := fmt.Sprintf("%v", res.Items[0].Record["id"]); got != "2" {
		t.Fatalf("body-weighted order: first=%v (items %v)", got, res.Items)
	}

	// The weighted score IS the weight sum of the per-field engine scores:
	// doc 1 matches on the title only, so its weighted score (3,1) is
	// 3*titleScore — computed here directly against the engine.
	var sTitle float64
	if err := db.QueryRowContext(ctx,
		`SELECT fts_score(title, 'alpha') FROM wtx WHERE fts_match(title, 'alpha') AND id = 1`).Scan(&sTitle); err != nil {
		t.Fatalf("direct title score: %v", err)
	}
	res = run(3, 1)
	want := 3 * sTitle
	if got := res.Items[0].Score.Value; diff(got, want) > 1e-6 {
		t.Fatalf("weighted score %v does not equal 3*titleScore = %v", got, want)
	}

	// Weight validation is typed invalid_request.
	bad := []func(*search.TextClause){
		func(c *search.TextClause) { c.Weights = []search.TextWeight{{Field: "ghost", Weight: 1}} },
		func(c *search.TextClause) { c.Weights = []search.TextWeight{{Field: "title", Weight: 0}} },
		func(c *search.TextClause) { c.Weights = []search.TextWeight{{Field: "title", Weight: -1}} },
	}
	for i, mutate := range bad {
		req := &search.Request{
			Mode:  "text",
			Text:  &search.TextClause{Fields: []string{"title", "body"}, Query: "alpha"},
			Limit: 10,
		}
		mutate(req.Text)
		_, err := search.Search(ctx, db, "wtx", req, search.DefaultOptions(), "db", 1, 1, 1)
		if !errors.Is(err, search.ErrInvalidRequest) {
			t.Errorf("bad weights case %d: want invalid_request, got %v", i, err)
		}
	}
}

func diff(a, b float64) float64 {
	if a > b {
		return a - b
	}
	return b - a
}

func contains(list []string, want string) bool {
	for _, x := range list {
		if x == want {
			return true
		}
	}
	return false
}

// TestWeightedBudgetOverflowIncomplete proves the weighted path reports
// exhaustion explicitly: a scan beyond the candidate budget truncates at
// the budget and sets complete=false, never a deceptively complete
// ranking.
func TestWeightedBudgetOverflowIncomplete(t *testing.T) {
	db := openEngineDB(t)
	ctx := context.Background()
	if _, err := db.ExecContext(ctx,
		`CREATE TABLE wtx_b (id INTEGER PRIMARY KEY, title TEXT, body TEXT)`); err != nil {
		t.Fatalf("create: %v", err)
	}
	if _, err := db.ExecContext(ctx,
		`CREATE INDEX idx_wtx_b ON wtx_b USING fts (title)`); err != nil {
		t.Fatalf("index: %v", err)
	}
	for i := 1; i <= 30; i++ {
		if _, err := db.ExecContext(ctx,
			fmt.Sprintf(`INSERT INTO wtx_b (id, title, body) VALUES (%d, 'alpha %02d', 'x')`, i, i)); err != nil {
			t.Fatalf("seed %d: %v", i, err)
		}
	}
	opts := search.DefaultOptions()
	opts.CandidateBudget = 5
	req := &search.Request{
		Mode: "text",
		Text: &search.TextClause{
			Fields:  []string{"title"},
			Query:   "alpha",
			Weights: []search.TextWeight{{Field: "title", Weight: 2}},
		},
		Limit: 20,
	}
	res, err := search.Search(ctx, db, "wtx_b", req, opts, "db", 1, 1, 1)
	if err != nil {
		t.Fatalf("search: %v", err)
	}
	if res.Complete {
		t.Fatalf("budget overflow must be incomplete")
	}
	if res.ScannedCount != 5 {
		t.Fatalf("scanned count must be capped at the budget, got %d", res.ScannedCount)
	}
}

// TestTextFilteredScannedCount proves the text-mode completeness and
// scannedCount cover the FILTERED matching domain the query actually
// applies (queries-and-search.md candidate budget): a narrow filter over a
// large table stays complete when the filtered domain fits the budget,
// while the unfiltered domain over the same table reports exhaustion.
func TestTextFilteredScannedCount(t *testing.T) {
	db := openEngineDB(t)
	ctx := context.Background()
	if _, err := db.ExecContext(ctx,
		`CREATE TABLE ftx (id INTEGER PRIMARY KEY, title TEXT, marker TEXT)`); err != nil {
		t.Fatalf("create: %v", err)
	}
	if _, err := db.ExecContext(ctx,
		`CREATE INDEX idx_ftx ON ftx USING fts (title)`); err != nil {
		t.Fatalf("index: %v", err)
	}
	for i := 1; i <= 30; i++ {
		marker := "no"
		if i <= 5 {
			marker = "yes"
		}
		if _, err := db.ExecContext(ctx,
			fmt.Sprintf(`INSERT INTO ftx (id, title, marker) VALUES (%d, 'word %02d', '%s')`, i, i, marker)); err != nil {
			t.Fatalf("seed %d: %v", i, err)
		}
	}

	opts := search.DefaultOptions()
	opts.CandidateBudget = 10

	// Filtered: the domain is the 5 marker='yes' rows — complete, and
	// scannedCount reports the FILTERED domain, not the table size.
	req := &search.Request{
		Mode:   "text",
		Text:   &search.TextClause{Fields: []string{"title"}, Query: "word"},
		Filter: "marker = 'yes'",
		Limit:  10,
	}
	res, err := search.Search(ctx, db, "ftx", req, opts, "db", 1, 1, 1)
	if err != nil {
		t.Fatalf("filtered search: %v", err)
	}
	if res.ScannedCount != 5 {
		t.Fatalf("scannedCount must cover the filtered domain (5), got %d", res.ScannedCount)
	}
	if !res.Complete || len(res.Items) != 5 {
		t.Fatalf("filtered domain fits the budget: complete=%v items=%d", res.Complete, len(res.Items))
	}

	// Unfiltered: the 30-row domain exceeds the budget of 10 — explicit
	// exhaustion, never a deceptively complete list.
	req.Filter = ""
	res, err = search.Search(ctx, db, "ftx", req, opts, "db", 1, 1, 1)
	if err != nil {
		t.Fatalf("unfiltered search: %v", err)
	}
	if res.ScannedCount != 30 {
		t.Fatalf("unfiltered scannedCount must be the full matching domain (30), got %d", res.ScannedCount)
	}
	if res.Complete {
		t.Fatalf("domain above the budget must be incomplete")
	}
}

// TestHighlightFieldAttribution proves highlight fragments are attributed
// to the field that produced them (multi-field text search): a record
// matching only body carries a body highlight, one matching both carries
// two per-field highlight entries in field order, the weighted path keeps
// the same attribution, and fragments carry no sentinel bytes or HTML.
func TestHighlightFieldAttribution(t *testing.T) {
	db := openEngineDB(t)
	ctx := context.Background()
	steps := []string{
		`CREATE TABLE hlx (id INTEGER PRIMARY KEY, title TEXT, body TEXT)`,
		`CREATE INDEX idx_hlx ON hlx USING fts (title, body)`,
		`INSERT INTO hlx (id, title, body) VALUES (1, 'alpha one', 'beta two')`,
		`INSERT INTO hlx (id, title, body) VALUES (2, 'gamma', 'alpha three')`,
		`INSERT INTO hlx (id, title, body) VALUES (3, 'alpha alpha', 'alpha four')`,
	}
	for _, s := range steps {
		if _, err := db.ExecContext(ctx, s); err != nil {
			t.Fatalf("seed: %v", err)
		}
	}

	run := func(weighted bool) map[string][]search.Highlight {
		t.Helper()
		clause := &search.TextClause{
			Fields:    []string{"title", "body"},
			Query:     "alpha",
			Highlight: true,
		}
		if weighted {
			clause.Weights = []search.TextWeight{{Field: "title", Weight: 2}, {Field: "body", Weight: 1}}
		}
		req := &search.Request{Mode: "text", Text: clause, Limit: 10}
		res, err := search.Search(ctx, db, "hlx", req, search.DefaultOptions(), "db", 1, 1, 1)
		if err != nil {
			t.Fatalf("search (weighted=%v): %v", weighted, err)
		}
		byID := map[string][]search.Highlight{}
		for _, it := range res.Items {
			byID[fmt.Sprintf("%v", it.Record["id"])] = it.Highlights
			for _, h := range it.Highlights {
				for _, f := range h.Fragments {
					if strings.ContainsAny(f, "\x1f<>") {
						t.Fatalf("fragment carries sentinels or markup: %q", f)
					}
				}
			}
		}
		return byID
	}

	for _, weighted := range []bool{false, true} {
		byID := run(weighted)
		if hls := byID["1"]; len(hls) != 1 || hls[0].Field != "title" || len(hls[0].Fragments) != 1 || hls[0].Fragments[0] != "alpha" {
			t.Fatalf("weighted=%v doc1 highlights %v, want title:[alpha]", weighted, hls)
		}
		if hls := byID["2"]; len(hls) != 1 || hls[0].Field != "body" || hls[0].Fragments[0] != "alpha" {
			t.Fatalf("weighted=%v doc2 highlights %v, want body:[alpha] (fragments must be attributed to the producing field)", weighted, hls)
		}
		if hls := byID["3"]; len(hls) != 2 {
			t.Fatalf("weighted=%v doc3 highlights %v, want two per-field entries", weighted, hls)
		} else {
			if hls[0].Field != "title" || len(hls[0].Fragments) != 2 {
				t.Fatalf("weighted=%v doc3 title highlights %v", weighted, hls[0])
			}
			if hls[1].Field != "body" || len(hls[1].Fragments) != 1 {
				t.Fatalf("weighted=%v doc3 body highlights %v", weighted, hls[1])
			}
		}
	}
}

// TestEnsureFTSIndexVerification proves the FTS index maintenance reports
// a VERIFIED state instead of unconditionally claiming ready
// (queries-and-search.md: "Index creation/rebuild is a durable job with
// old index serving or explicit unavailable status"): a matching stored
// index keeps serving untouched, a same-name index covering anything else
// (different fields, another table, a non-FTS index) is rebuilt, a failed
// rebuild reports unavailable — and text search then fails with
// schema_mismatch until the index is restored.
func TestEnsureFTSIndexVerification(t *testing.T) {
	db := openEngineDB(t)
	ctx := context.Background()
	steps := []string{
		`CREATE TABLE ivx (id INTEGER PRIMARY KEY, title TEXT, body TEXT)`,
		`INSERT INTO ivx (id, title, body) VALUES (1, 'alpha one', 'beta two')`,
	}
	for _, s := range steps {
		if _, err := db.ExecContext(ctx, s); err != nil {
			t.Fatalf("seed: %v", err)
		}
	}

	// a) Fresh create: ready and verified.
	st, err := search.EnsureFTSIndex(ctx, db, "ivx", "idx_ivx", []string{"title"}, "", nil)
	if err != nil || st.State != "ready" || st.Rebuilt || !st.Verified {
		t.Fatalf("fresh create: state=%+v err=%v", st, err)
	}

	// b) Same configuration again: the stored index keeps serving (no
	// rebuild).
	st, err = search.EnsureFTSIndex(ctx, db, "ivx", "idx_ivx", []string{"title"}, "", nil)
	if err != nil || st.State != "ready" || st.Rebuilt {
		t.Fatalf("same config must keep the old index serving: state=%+v err=%v", st, err)
	}

	// c) Wider field set under the same name: rebuild, then ready — and
	// the stored DDL now covers both fields.
	st, err = search.EnsureFTSIndex(ctx, db, "ivx", "idx_ivx", []string{"title", "body"}, "", nil)
	if err != nil || st.State != "ready" || !st.Rebuilt || !st.Verified {
		t.Fatalf("field change must rebuild: state=%+v err=%v", st, err)
	}
	if _, err := search.Search(ctx, db, "ivx", &search.Request{
		Mode: "text",
		Text: &search.TextClause{Fields: []string{"title", "body"}, Query: "alpha"},
	}, search.DefaultOptions(), "db", 1, 1, 1); err != nil {
		t.Fatalf("rebuilt index must serve both fields: %v", err)
	}

	// d) A rebuild that fails (unknown column) reports unavailable — not
	// ready — and the collection is left without a serving index.
	st, err = search.EnsureFTSIndex(ctx, db, "ivx", "idx_ivx", []string{"title", "ghost"}, "", nil)
	if err != nil {
		t.Fatalf("failed rebuild must report its state, not a transport error: %v", err)
	}
	if st.State != "unavailable" || st.Error == "" || !st.Rebuilt {
		t.Fatalf("failed rebuild: state=%+v (want unavailable with error)", st)
	}
	var ddl sql.NullString
	if err := db.QueryRowContext(ctx,
		`SELECT sql FROM sqlite_master WHERE type = 'index' AND name = 'idx_ivx'`).Scan(&ddl); !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("a failed rebuild must not leave the old index stored, got ddl=%v err=%v", ddl, err)
	}
	if _, serr := search.Search(ctx, db, "ivx", &search.Request{
		Mode: "text",
		Text: &search.TextClause{Fields: []string{"title"}, Query: "alpha"},
	}, search.DefaultOptions(), "db", 1, 1, 1); !errors.Is(serr, search.ErrNotIndexed) {
		t.Fatalf("text search over an unavailable index must fail with schema_mismatch, got %v", serr)
	}

	// e) Recovery: recreating the index restores serving.
	st, err = search.EnsureFTSIndex(ctx, db, "ivx", "idx_ivx", []string{"title", "body"}, "", nil)
	if err != nil || st.State != "ready" || !st.Verified {
		t.Fatalf("recovery create: state=%+v err=%v", st, err)
	}

	// f) A same-name NON-FTS index (even on another table) is replaced:
	// the silent IF NOT EXISTS acceptance of a wrong same-name index is
	// gone.
	if _, err := db.ExecContext(ctx, `CREATE TABLE ivx_other (id INTEGER PRIMARY KEY, title TEXT)`); err != nil {
		t.Fatalf("other table: %v", err)
	}
	if _, err := db.ExecContext(ctx, `DROP INDEX idx_ivx`); err != nil {
		t.Fatalf("drop before foreign index: %v", err)
	}
	if _, err := db.ExecContext(ctx, `CREATE INDEX idx_ivx ON ivx_other (title)`); err != nil {
		t.Fatalf("foreign same-name index: %v", err)
	}
	st, err = search.EnsureFTSIndex(ctx, db, "ivx", "idx_ivx", []string{"title"}, "", nil)
	if err != nil || st.State != "ready" || !st.Rebuilt {
		t.Fatalf("foreign same-name index must be rebuilt: state=%+v err=%v", st, err)
	}
	var isFTS bool
	if err := db.QueryRowContext(ctx,
		`SELECT sql LIKE '%USING fts%' FROM sqlite_master WHERE type = 'index' AND name = 'idx_ivx' AND tbl_name = 'ivx'`).Scan(&isFTS); err != nil {
		t.Fatalf("verify rebuilt DDL: %v", err)
	}
	if !isFTS {
		t.Fatalf("the rebuilt index must be the FTS index on ivx")
	}
	if _, serr := search.Search(ctx, db, "ivx", &search.Request{
		Mode: "text",
		Text: &search.TextClause{Fields: []string{"title"}, Query: "alpha"},
	}, search.DefaultOptions(), "db", 1, 1, 1); serr != nil {
		t.Fatalf("replaced index must serve ivx: %v", serr)
	}
}
