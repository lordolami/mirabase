package search_test

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"path/filepath"
	"testing"

	"mirabase/internal/mbengine"
	"mirabase/internal/search"
	_ "turso.tech/database/tursogo"
)

// TestHybridFuzzyWireLiftAndValidation proves the wire shape of the new
// modes (wire-schema-reference.md SearchRequest hybrid/fuzzy variants) and
// the typed validation: hybrid requires vector+text+fusion (kind rrf only,
// k 1..1000); fuzzy requires field/query/algorithm and exactly one of
// maximumDistance (0..1000) or minimumSimilarity (0..1), with
// similarity-declaring algorithms (jaro_winkler, soundex) rejecting
// maximumDistance. No engine connection is needed: validation happens
// before query allocation, so a never-connected handle is the negative
// control (an accidental SQL touch would surface as a driver error, not a
// typed invalid_request).
func TestHybridFuzzyWireLiftAndValidation(t *testing.T) {
	path := filepath.Join(t.TempDir(), "validation.db")
	db, err := sql.Open("turso", path)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer db.Close()

	run := func(req search.Request) error {
		_, err := search.Search(context.Background(), db, "nope", &req, search.DefaultOptions(), "db", 1, 1, 1)
		return err
	}

	// The fuzzy wire variant carries its clause at the top level; the
	// decode lifts it into the Fuzzy clause.
	var lifted search.Request
	if err := json.Unmarshal([]byte(`{
		"mode": "fuzzy",
		"field": "name",
		"query": "kitten",
		"algorithm": "levenshtein",
		"maximumDistance": 2
	}`), &lifted); err != nil {
		t.Fatalf("decode fuzzy: %v", err)
	}
	if lifted.Fuzzy == nil || lifted.Fuzzy.Field != "name" || lifted.Fuzzy.Query != "kitten" ||
		lifted.Fuzzy.Algorithm != "levenshtein" || lifted.Fuzzy.MaximumDistance == nil || *lifted.Fuzzy.MaximumDistance != 2 {
		t.Fatalf("fuzzy wire lift: %+v", lifted.Fuzzy)
	}

	// The hybrid wire variant nests the fusion clause.
	var hybridReq search.Request
	if err := json.Unmarshal([]byte(`{
		"mode": "hybrid",
		"vector": {"field": "embedding", "query": [1,0,0], "metric": "cosine"},
		"text": {"fields": ["title"], "query": "alpha"},
		"fusion": {"kind": "rrf", "k": 60}
	}`), &hybridReq); err != nil {
		t.Fatalf("decode hybrid: %v", err)
	}
	if hybridReq.Fusion == nil || hybridReq.Fusion.Kind != "rrf" || hybridReq.Fusion.K != 60 {
		t.Fatalf("hybrid fusion decode: %+v", hybridReq.Fusion)
	}

	cases := []struct {
		name string
		req  search.Request
	}{
		{"hybrid without fusion", search.Request{Mode: "hybrid",
			Vector: &search.VectorClause{Field: "embedding", Query: json.RawMessage(`[1,0,0]`), Metric: "cosine"},
			Text:   &search.TextClause{Fields: []string{"title"}, Query: "alpha"}}},
		{"hybrid with unknown fusion kind", search.Request{Mode: "hybrid",
			Vector: &search.VectorClause{Field: "embedding", Query: json.RawMessage(`[1,0,0]`), Metric: "cosine"},
			Text:   &search.TextClause{Fields: []string{"title"}, Query: "alpha"},
			Fusion: &search.FusionClause{Kind: "linear", K: 60}}},
		{"hybrid with fusion k below range", search.Request{Mode: "hybrid",
			Vector: &search.VectorClause{Field: "embedding", Query: json.RawMessage(`[1,0,0]`), Metric: "cosine"},
			Text:   &search.TextClause{Fields: []string{"title"}, Query: "alpha"},
			Fusion: &search.FusionClause{Kind: "rrf", K: 0}}},
		{"hybrid with fusion k above range", search.Request{Mode: "hybrid",
			Vector: &search.VectorClause{Field: "embedding", Query: json.RawMessage(`[1,0,0]`), Metric: "cosine"},
			Text:   &search.TextClause{Fields: []string{"title"}, Query: "alpha"},
			Fusion: &search.FusionClause{Kind: "rrf", K: 1001}}},
		{"fuzzy without clause", search.Request{Mode: "fuzzy"}},
		{"fuzzy without threshold", search.Request{Mode: "fuzzy",
			Fuzzy: &search.FuzzyClause{Field: "name", Query: "kitten", Algorithm: "levenshtein"}}},
		{"fuzzy with both thresholds", search.Request{Mode: "fuzzy",
			Fuzzy: &search.FuzzyClause{Field: "name", Query: "kitten", Algorithm: "levenshtein",
				MaximumDistance: intPtr(2), MinimumSimilarity: floatPtr(0.5)}}},
		{"fuzzy with unknown algorithm", search.Request{Mode: "fuzzy",
			Fuzzy: &search.FuzzyClause{Field: "name", Query: "kitten", Algorithm: "levenshtein2", MaximumDistance: intPtr(2)}}},
		{"fuzzy with maximumDistance above range", search.Request{Mode: "fuzzy",
			Fuzzy: &search.FuzzyClause{Field: "name", Query: "kitten", Algorithm: "levenshtein", MaximumDistance: intPtr(1001)}}},
		{"fuzzy with minimumSimilarity above range", search.Request{Mode: "fuzzy",
			Fuzzy: &search.FuzzyClause{Field: "name", Query: "kitten", Algorithm: "levenshtein", MinimumSimilarity: floatPtr(1.5)}}},
		{"fuzzy similarity algorithm with maximumDistance", search.Request{Mode: "fuzzy",
			Fuzzy: &search.FuzzyClause{Field: "name", Query: "robert", Algorithm: "jaro_winkler", MaximumDistance: intPtr(1)}}},
		{"fuzzy empty query", search.Request{Mode: "fuzzy",
			Fuzzy: &search.FuzzyClause{Field: "name", Query: "", Algorithm: "levenshtein", MaximumDistance: intPtr(2)}}},
	}
	for _, c := range cases {
		err := run(c.req)
		if !errors.Is(err, search.ErrInvalidRequest) {
			t.Errorf("%s: want invalid_request, got %v", c.name, err)
		}
	}

	// Valid clauses pass validation (the collection is unknown, so the
	// failure point moves past validation to the engine query). This
	// negative control needs a working engine connection; without the
	// pinned payload the driver cannot connect at all, so it is skipped.
	if mbengine.PinnedDigest() != "" && mbengine.Ensure() == nil {
		err = run(search.Request{Mode: "fuzzy",
			Fuzzy: &search.FuzzyClause{Field: "name", Query: "kitten", Algorithm: "levenshtein", MaximumDistance: intPtr(2)}})
		if err == nil || errors.Is(err, search.ErrInvalidRequest) {
			t.Errorf("valid fuzzy clause: want past-validation engine error, got %v", err)
		}
	} else {
		t.Log("valid-clause negative control skipped: pinned engine payload not available")
	}
}

// TestHybridOutranksSingleLeg proves the RRF fusion: the record that
// matches both the vector and the text leg outranks records matching a
// single leg, the metric is reported as rrf, and the fusion combines
// ranks (score >= 1/(k+1) + 1/(k+2) for a double match, while any
// single-leg record is bounded by 1/(k+1)).
func TestHybridOutranksSingleLeg(t *testing.T) {
	db := openEngineDB(t)
	ctx := context.Background()

	steps := []string{
		`CREATE TABLE docs (id INTEGER PRIMARY KEY, title TEXT, body TEXT, embedding TEXT)`,
		`INSERT INTO docs (id, title, body, embedding) VALUES (1, 'alpha', 'alpha zebra', '[1,0,0]')`,
		`INSERT INTO docs (id, title, body, embedding) VALUES (2, 'beta', 'unrelated words', '[0.94,0.34,0]')`,
		`INSERT INTO docs (id, title, body, embedding) VALUES (3, 'gamma', 'alpha monitor', '[0,1,0]')`,
		`CREATE INDEX idx_docs_fts ON docs USING fts (title, body)`,
	}
	for _, s := range steps {
		if _, err := db.ExecContext(ctx, s); err != nil {
			t.Fatalf("seed: %v", err)
		}
	}

	req := &search.Request{
		Mode:   "hybrid",
		Vector: &search.VectorClause{Field: "embedding", Query: json.RawMessage(`[1,0,0]`), Metric: "cosine"},
		Text:   &search.TextClause{Fields: []string{"title", "body"}, Query: "alpha"},
		Fusion: &search.FusionClause{Kind: "rrf", K: 60},
		Limit:  10,
	}
	res, err := search.Search(ctx, db, "docs", req, search.DefaultOptions(), "db", 1, 1, 1)
	if err != nil {
		t.Fatalf("hybrid search: %v", err)
	}
	if res.Mode != "hybrid" || res.Metric == nil || *res.Metric != "rrf" {
		t.Fatalf("hybrid response context: mode=%s metric=%v", res.Mode, res.Metric)
	}
	if !res.Complete || res.ScannedCount != 3 {
		t.Fatalf("hybrid completeness: complete=%v scanned=%d", res.Complete, res.ScannedCount)
	}
	if len(res.Items) != 3 {
		t.Fatalf("hybrid items: %d", len(res.Items))
	}
	first := res.Items[0]
	if id := fmt.Sprintf("%v", first.Record["id"]); id != "1" {
		t.Fatalf("record matching both legs must outrank single-leg records: first=%v", first.Record)
	}
	// Record 1 matches both legs (vector rank 1; text rank 1 or 2): its
	// fused score is 1/(60+r1) + 1/(60+r2) with r1 = 1, r2 <= 2. Record 2
	// never matches the text leg, so it carries exactly its vector
	// contribution 1/62 (rank 2) — the single-leg record. Record 3 carries
	// two weaker contributions (vector rank 3 + text rank 2).
	if first.Score.Kind != "rrf" {
		t.Fatalf("fused score kind: %v", first.Score.Kind)
	}
	both := 1.0/61.0 + 1.0/62.0
	if first.Score.Value < both-1e-9 {
		t.Fatalf("both-legs score %v below the two-contribution minimum %v", first.Score.Value, both)
	}
	byID := map[string]search.Item{}
	for _, it := range res.Items {
		byID[fmt.Sprintf("%v", it.Record["id"])] = it
	}
	single := byID["2"].Score.Value
	if diff := single - 1.0/62.0; diff > 1e-9 || diff < -1e-9 {
		t.Fatalf("single-leg record fused at %v, want exactly its vector contribution 1/62", single)
	}
	// The full fused order: both-legs record first, then the weak both-legs
	// record, then the single-leg record last.
	wantOrder := []string{"1", "3", "2"}
	for i, want := range wantOrder {
		if got := fmt.Sprintf("%v", res.Items[i].Record["id"]); got != want {
			t.Fatalf("fused order: position %d = id %s, want %s (items %v)", i, got, want, res.Items)
		}
	}
}

// TestFuzzyAlgorithms proves every algorithm accepts a known near match
// and rejects a distant one on the pinned engine, with the distance-based
// algorithms honoring maximumDistance and the similarity-based ones
// (jaro_winkler, soundex) minimumSimilarity.
func TestFuzzyAlgorithms(t *testing.T) {
	db := openEngineDB(t)
	ctx := context.Background()

	type row struct {
		id   int
		name string
	}
	cases := []struct {
		algorithm string
		threshold search.FuzzyClause
		rows      []row
		accepted  []string
		rejected  []string
	}{
		{
			algorithm: "levenshtein",
			threshold: search.FuzzyClause{Field: "name", Query: "kitten", Algorithm: "levenshtein", MaximumDistance: intPtr(1)},
			rows:      []row{{1, "kitten"}, {2, "sitten"}, {3, "elephant"}, {4, "mitten"}},
			accepted:  []string{"kitten", "sitten", "mitten"},
			rejected:  []string{"elephant"},
		},
		{
			algorithm: "damerau",
			// Unrestricted Damerau-Levenshtein: ca -> ac -> abc is
			// distance 2, which plain OSA would score 3.
			threshold: search.FuzzyClause{Field: "name", Query: "ca", Algorithm: "damerau", MaximumDistance: intPtr(2)},
			rows:      []row{{1, "ca"}, {2, "abc"}, {3, "xyz"}},
			accepted:  []string{"ca", "abc"},
			rejected:  []string{"xyz"},
		},
		{
			algorithm: "osa",
			threshold: search.FuzzyClause{Field: "name", Query: "ac", Algorithm: "osa", MaximumDistance: intPtr(1)},
			rows:      []row{{1, "ca"}, {2, "xyz"}},
			accepted:  []string{"ca"},
			rejected:  []string{"xyz"},
		},
		{
			algorithm: "hamming",
			// Equal-length strings only: "sitting" differs in length and
			// must be rejected even though its edit distance is 3.
			threshold: search.FuzzyClause{Field: "name", Query: "kitten", Algorithm: "hamming", MaximumDistance: intPtr(1)},
			rows:      []row{{1, "kitten"}, {2, "sitten"}, {3, "system"}, {4, "sitting"}},
			accepted:  []string{"kitten", "sitten"},
			rejected:  []string{"system", "sitting"},
		},
		{
			algorithm: "jaro_winkler",
			threshold: search.FuzzyClause{Field: "name", Query: "martha", Algorithm: "jaro_winkler", MinimumSimilarity: floatPtr(0.85)},
			rows:      []row{{1, "martha"}, {2, "marhta"}, {3, "sandra"}},
			accepted:  []string{"martha", "marhta"},
			rejected:  []string{"sandra"},
		},
		{
			algorithm: "soundex",
			threshold: search.FuzzyClause{Field: "name", Query: "robert", Algorithm: "soundex", MinimumSimilarity: floatPtr(0.5)},
			rows:      []row{{1, "robert"}, {2, "rupert"}, {3, "smith"}},
			accepted:  []string{"robert", "rupert"},
			rejected:  []string{"smith"},
		},
	}
	for _, c := range cases {
		t.Run(c.algorithm, func(t *testing.T) {
			table := "fz_" + c.algorithm
			if _, err := db.ExecContext(ctx,
				fmt.Sprintf(`CREATE TABLE %s (id INTEGER PRIMARY KEY, name TEXT)`, table)); err != nil {
				t.Fatalf("create: %v", err)
			}
			for _, r := range c.rows {
				if _, err := db.ExecContext(ctx,
					fmt.Sprintf(`INSERT INTO %s (id, name) VALUES (?, ?)`, table), r.id, r.name); err != nil {
					t.Fatalf("seed %s: %v", r.name, err)
				}
			}
			clause := c.threshold
			req := &search.Request{Mode: "fuzzy", Fuzzy: &clause, Limit: 10}
			res, err := search.Search(ctx, db, table, req, search.DefaultOptions(), "db", 1, 1, 1)
			if err != nil {
				t.Fatalf("%s search: %v", c.algorithm, err)
			}
			if res.Mode != "fuzzy" || res.Metric == nil || *res.Metric != c.algorithm {
				t.Fatalf("%s response context: mode=%s metric=%v", c.algorithm, res.Mode, res.Metric)
			}
			got := map[string]float64{}
			for _, it := range res.Items {
				got[fmt.Sprintf("%v", it.Record["name"])] = it.Score.Value
				if it.Score.Kind != "distance:"+c.algorithm {
					t.Fatalf("%s score kind: %v", c.algorithm, it.Score.Kind)
				}
			}
			for _, want := range c.accepted {
				if _, ok := got[want]; !ok {
					t.Errorf("%s: near match %q not accepted (items: %v)", c.algorithm, want, got)
				}
			}
			for _, miss := range c.rejected {
				if _, ok := got[miss]; ok {
					t.Errorf("%s: distant match %q accepted with distance %v", c.algorithm, miss, got[miss])
				}
			}
			if !res.Complete {
				t.Errorf("%s: small collection must be complete", c.algorithm)
			}
		})
	}
}

// TestFuzzyNormalizedSimilarity proves the normalized minimumSimilarity
// path for a distance algorithm: sim = 1 - dist/maxLen, so
// minimumSimilarity 0.9 over maxLen 6 allows distance <= 1.
func TestFuzzyNormalizedSimilarity(t *testing.T) {
	db := openEngineDB(t)
	ctx := context.Background()
	steps := []string{
		`CREATE TABLE fz_norm (id INTEGER PRIMARY KEY, name TEXT)`,
		`INSERT INTO fz_norm (id, name) VALUES (1, 'kitten')`,
		`INSERT INTO fz_norm (id, name) VALUES (2, 'sitten')`,
		`INSERT INTO fz_norm (id, name) VALUES (3, 'elephant')`,
	}
	for _, s := range steps {
		if _, err := db.ExecContext(ctx, s); err != nil {
			t.Fatalf("seed: %v", err)
		}
	}
	req := &search.Request{Mode: "fuzzy",
		Fuzzy: &search.FuzzyClause{Field: "name", Query: "kitten", Algorithm: "levenshtein", MinimumSimilarity: floatPtr(0.9)},
		Limit: 10}
	res, err := search.Search(ctx, db, "fz_norm", req, search.DefaultOptions(), "db", 1, 1, 1)
	if err != nil {
		t.Fatalf("search: %v", err)
	}
	got := map[string]bool{}
	for _, it := range res.Items {
		got[fmt.Sprintf("%v", it.Record["name"])] = true
	}
	if !got["kitten"] || !got["sitten"] {
		t.Fatalf("normalized similarity: kitten/sitten must be accepted, got %v", got)
	}
	if got["elephant"] {
		t.Fatalf("normalized similarity: elephant must be rejected")
	}
}

// TestFuzzyDeterministicOrdering proves the ordering contract: ascending
// distance, ties broken by ascending record ID.
func TestFuzzyDeterministicOrdering(t *testing.T) {
	db := openEngineDB(t)
	ctx := context.Background()
	steps := []string{
		`CREATE TABLE fz_order (id INTEGER PRIMARY KEY, name TEXT)`,
		`INSERT INTO fz_order (id, name) VALUES (5, 'sitten')`,
		`INSERT INTO fz_order (id, name) VALUES (2, 'kitten')`,
		`INSERT INTO fz_order (id, name) VALUES (9, 'sittin')`,
		`INSERT INTO fz_order (id, name) VALUES (4, 'bitten')`,
	}
	for _, s := range steps {
		if _, err := db.ExecContext(ctx, s); err != nil {
			t.Fatalf("seed: %v", err)
		}
	}
	req := &search.Request{Mode: "fuzzy",
		Fuzzy: &search.FuzzyClause{Field: "name", Query: "kitten", Algorithm: "levenshtein", MaximumDistance: intPtr(2)},
		Limit: 10}
	res, err := search.Search(ctx, db, "fz_order", req, search.DefaultOptions(), "db", 1, 1, 1)
	if err != nil {
		t.Fatalf("search: %v", err)
	}
	wantIDs := []string{"2", "4", "5", "9"} // distance 0 first, then distance-1 ties by ascending id
	for i, want := range wantIDs {
		if got := fmt.Sprintf("%v", res.Items[i].Record["id"]); got != want {
			t.Fatalf("ordering: position %d = id %s, want %s (items %v)", i, got, want, res.Items)
		}
	}
}

// TestFuzzyCandidateBudgetBoundsScan proves the candidate budget bounds
// the Go-side scan: with 25 records and a budget of 10 the scan stops at
// 10 rows and the result is explicitly incomplete, never a deceptively
// complete list; raising the budget above the record count completes.
func TestFuzzyCandidateBudgetBoundsScan(t *testing.T) {
	db := openEngineDB(t)
	ctx := context.Background()
	if _, err := db.ExecContext(ctx, `CREATE TABLE fz_budget (id INTEGER PRIMARY KEY, name TEXT)`); err != nil {
		t.Fatalf("create: %v", err)
	}
	for i := 1; i <= 25; i++ {
		if _, err := db.ExecContext(ctx, `INSERT INTO fz_budget (id, name) VALUES (?, ?)`, i, fmt.Sprintf("word%02d", i)); err != nil {
			t.Fatalf("seed %d: %v", i, err)
		}
	}

	run := func(budget int) *search.Result {
		t.Helper()
		opts := search.DefaultOptions()
		opts.CandidateBudget = budget
		req := &search.Request{Mode: "fuzzy",
			Fuzzy: &search.FuzzyClause{Field: "name", Query: "word01", Algorithm: "levenshtein", MaximumDistance: intPtr(1)},
			Limit: 20}
		res, err := search.Search(ctx, db, "fz_budget", req, opts, "db", 1, 1, 1)
		if err != nil {
			t.Fatalf("search budget %d: %v", budget, err)
		}
		return res
	}

	res := run(10)
	if res.ScannedCount != 10 {
		t.Fatalf("budget must bound the scan at 10 rows, scanned %d", res.ScannedCount)
	}
	if res.Complete {
		t.Fatalf("budget exhaustion must set complete=false, got a complete result")
	}
	if len(res.Items) > 20 {
		t.Fatalf("items exceed the limit: %d", len(res.Items))
	}

	res = run(30)
	if !res.Complete {
		t.Fatalf("budget 30 over 25 records must complete")
	}
	if res.ScannedCount != 25 {
		t.Fatalf("full scan expected 25 rows, scanned %d", res.ScannedCount)
	}
}

// openEngineDB opens a scratch database on the pinned engine payload.
func openEngineDB(t *testing.T) *sql.DB {
	t.Helper()
	if mbengine.PinnedDigest() == "" {
		t.Skip("requires the pinned engine payload")
	}
	if err := mbengine.Ensure(); err != nil {
		t.Skipf("engine ensure: %v", err)
	}
	path := filepath.Join(t.TempDir(), "hybrid_fuzzy.db")
	db, err := sql.Open("turso", path+"?_busy_timeout=10000&experimental=vacuum,index_method")
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	if _, err := db.Conn(context.Background()); err != nil {
		t.Fatalf("connect: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	return db
}

func intPtr(v int) *int           { return &v }
func floatPtr(v float64) *float64 { return &v }
