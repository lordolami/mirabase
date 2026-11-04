package search_test

import (
	"context"
	"database/sql"
	"fmt"
	"testing"

	"mirabase/internal/mbengine"
	tursogo "turso.tech/database/tursogo"
)

// TestEngineProbe is the V01-07 engine-surface probe at the pinned commit:
// vector functions and the native FTS index method on the sqlite frontend.
func TestEngineProbe(t *testing.T) {
	if mbengine.PinnedDigest() == "" {
		t.Skip("requires the pinned engine payload")
	}
	if err := mbengine.Ensure(); err != nil {
		t.Skipf("engine ensure: %v", err)
	}

	dir := t.TempDir()
	path := dir + "\\probe.db"
	raw, err := sql.Open("turso", path+"?_busy_timeout=10000&experimental=vacuum,index_method")
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer raw.Close()
	if _, err := raw.Conn(context.Background()); err != nil {
		t.Fatalf("connect: %v", err)
	}

	ctx := context.Background()

	// --- vector surface ---
	steps := []struct {
		name string
		sql  string
	}{
		{"create table", `CREATE TABLE v (id INTEGER PRIMARY KEY, embedding BLOB)`},
		{"insert32", `INSERT INTO v (id, embedding) VALUES (1, vector32('[1,0,0]'))`},
		{"insert32b", `INSERT INTO v (id, embedding) VALUES (2, vector32('[0,1,0]'))`},
		{"insert32c", `INSERT INTO v (id, embedding) VALUES (3, vector32('[0.9,0.1,0]'))`},
	}
	for _, s := range steps {
		if _, err := raw.ExecContext(ctx, s.sql); err != nil {
			t.Fatalf("%s: %v", s.name, err)
		}
	}

	type probe struct {
		name string
		sql  string
		want string
	}
	probes := []probe{
		{"distance_l2", `SELECT id FROM v ORDER BY vector_distance_l2(embedding, vector32('[1,0,0]')) LIMIT 2`, ""},
		{"distance_cos", `SELECT id FROM v ORDER BY vector_distance_cos(embedding, vector32('[1,0,0]')) LIMIT 2`, ""},
		{"distance_dot", `SELECT id FROM v ORDER BY vector_distance_dot(embedding, vector32('[1,0,0]')) LIMIT 2`, ""},
		{"extract", `SELECT vector_extract(embedding) FROM v WHERE id = 1`, ""},
		{"cos value", `SELECT format('%.6f', vector_distance_cos(embedding, vector32('[1,0,0]'))) FROM v WHERE id = 1`, ""},
	}
	for _, p := range probes {
		rows, err := raw.QueryContext(ctx, p.sql)
		if err != nil {
			t.Errorf("%s: %v", p.name, err)
			continue
		}
		var vals []string
		for rows.Next() {
			var v any
			if err := rows.Scan(&v); err != nil {
				rows.Close()
				t.Errorf("%s scan: %v", p.name, err)
				break
			}
			vals = append(vals, fmt.Sprintf("%v", v))
		}
		rows.Close()
		t.Logf("%s: %v", p.name, vals)
	}

	// --- sparse vector ---
	if _, err := raw.ExecContext(ctx, `INSERT INTO v (id, embedding) VALUES (4, vector32_sparse('[0,2,0]'))`); err != nil {
		t.Logf("vector32_sparse: %v", err)
	} else {
		t.Logf("vector32_sparse: OK")
	}

	// --- FTS surface ---
	ftsSteps := []struct {
		name string
		sql  string
	}{
		{"create fts table", `CREATE TABLE posts (id INTEGER PRIMARY KEY, title TEXT, body TEXT)`},
		{"insert fts", `INSERT INTO posts (id, title, body) VALUES (1, 'hello world', 'native full text search')`},
		{"insert fts2", `INSERT INTO posts (id, title, body) VALUES (2, 'goodbye world', 'vector embeddings')`},
		{"create fts index", `CREATE INDEX idx_posts_fts ON posts USING fts (title, body)`},
	}
	for _, s := range ftsSteps {
		if _, err := raw.ExecContext(ctx, s.sql); err != nil {
			t.Errorf("%s: %v", s.name, err)
		} else {
			t.Logf("%s: OK", s.name)
		}
	}
	ftsProbes := []probe{
		{"fts_match", `SELECT id FROM posts WHERE fts_match(title, body, 'world')`, ""},
		{"fts_score order", `SELECT id FROM posts ORDER BY fts_score(title, body, 'world') DESC LIMIT 2`, ""},
		{"fts_phrase", `SELECT id FROM posts WHERE fts_match(title, body, '"full text"')`, ""},
		{"fts_field", `SELECT id FROM posts WHERE fts_match(title, body, 'title:hello')`, ""},
		{"fts_highlight", `SELECT fts_highlight(title, 'hello', '<b>', '</b>') FROM posts WHERE id = 1`, ""},
	}
	for _, p := range ftsProbes {
		rows, err := raw.QueryContext(ctx, p.sql)
		if err != nil {
			t.Errorf("%s: %v", p.name, err)
			continue
		}
		var vals []string
		for rows.Next() {
			var v any
			if err := rows.Scan(&v); err != nil {
				rows.Close()
				break
			}
			vals = append(vals, fmt.Sprintf("%v", v))
		}
		rows.Close()
		t.Logf("%s: %v", p.name, vals)
	}

	// tokenizer variants
	for _, tok := range []string{"default", "en_stem", "raw", "simple", "whitespace", "ngram"} {
		name := fmt.Sprintf("tok_%s", tok)
		if _, err := raw.ExecContext(ctx, fmt.Sprintf(
			`CREATE TABLE tok_%s (id INTEGER PRIMARY KEY, t TEXT)`, tok)); err != nil {
			t.Errorf("%s create: %v", name, err)
			continue
		}
		if _, err := raw.ExecContext(ctx, fmt.Sprintf(
			`CREATE INDEX idx_%s ON tok_%s USING fts (t) WITH (tokenizer = '%s')`, tok, tok, tok)); err != nil {
			t.Errorf("%s index: %v", name, err)
			continue
		}
		t.Logf("%s: OK", name)
	}

	var _ = tursogo.WithBusyTimeout
}
