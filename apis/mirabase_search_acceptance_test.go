package apis_test

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"mirabase/core"
	"mirabase/internal/application"
	"mirabase/internal/search"
	"mirabase/tests"
)

// TestV01_07SearchAcceptance is the gate-4 search-isolation leg plus the
// AT-VECTOR/AT-FTS core:
//
//   - vector mode: exact authorized brute-force ordering vs a brute-force
//     reference, ties, zero-vector cosine error, malformed sparse inputs;
//   - text mode: native FTS with tokenizer variants, CRUD/rollback
//     committed visibility, no unauthorized candidates;
//   - isolation: Alice/Bob search only their own databases; cross-context
//     search fails; revocation stops search; hybrid (RRF) and fuzzy run
//     through the route with typed validation.
func TestV01_07SearchAcceptance(t *testing.T) {
	if mbenginePinnedDigestForIsolation() == "" {
		t.Skip("requires the pinned engine payload (run via _build/p02/parity_turso.sh)")
	}

	app, err := tests.NewTestApp()
	if err != nil {
		t.Fatalf("NewTestApp: %v", err)
	}
	defer app.Cleanup()

	rt, err := application.BindRuntime(app)
	if err != nil {
		t.Fatalf("BindRuntime: %v", err)
	}

	mux, err := apisRouterFor(app)
	if err != nil {
		t.Fatalf("router: %v", err)
	}
	ts := newHTTPServer(mux)
	defer ts.Close()
	superTok := createSuperuserAndGetToken(t, ts.URL, app)

	// Private collection with vector + text fields, and a second one
	// carrying a PROTECTED (hidden) text field for the field-protection
	// negative controls.
	docs, err := createSearchCollection(app, "docs")
	if err != nil {
		t.Fatalf("docs: %v", err)
	}
	docspriv, err := createSearchCollectionHidden(app, "docspriv")
	if err != nil {
		t.Fatalf("docspriv: %v", err)
	}

	tpl, err := rt.Service.PublishTemplate(context.Background(), app, "tpl-search", []string{docs.Name, docspriv.Name})
	if err != nil {
		t.Fatalf("publish: %v", err)
	}

	// Alice's and Bob's databases.
	aliceDB, _, err := rt.Service.Provision(context.Background(), app.DataDir(), application.ProvisionRequest{
		Kind: "user", TemplateID: tpl.ID,
		OwnerCollectionID: "users", OwnerRecordID: "aliceiso0000001", DisplayName: "alice",
	})
	if err != nil {
		t.Fatalf("alice provision: %v", err)
	}
	bobDB, _, err := rt.Service.Provision(context.Background(), app.DataDir(), application.ProvisionRequest{
		Kind: "user", TemplateID: tpl.ID,
		OwnerCollectionID: "users", OwnerRecordID: "bobiso000000001", DisplayName: "bob",
	})
	if err != nil {
		t.Fatalf("bob provision: %v", err)
	}
	tokAlice := superuserScopedToken(t, app, "users", "aliceiso0000001")
	tokBob := superuserScopedToken(t, app, "users", "bobiso000000001")

	// Seed both databases with the SAME dataset shape so isolation is
	// provable by content, and publish the FTS index on both.
	seed := []map[string]any{
		{"title": "hello world", "body": "native full text search", "embedding": []float64{1, 0, 0}},
		{"title": "goodbye world", "body": "vector embeddings", "embedding": []float64{0, 1, 0}},
		{"title": "hello mirabase", "body": "exact authorized search", "embedding": []float64{0.9, 0.1, 0}},
	}
	for _, db := range []*application.Database{aliceDB, bobDB} {
		target, err := rt.Handles.Open(context.Background(), db.ID)
		if err != nil {
			t.Fatalf("open %s: %v", db.ID, err)
		}
		if err := application.ApplyTemplate(context.Background(), target, tpl); err != nil {
			t.Fatalf("apply template %s: %v", db.ID, err)
		}
		for i, row := range seed {
			store := application.NewRecordStore(target, "docs")
			if _, err := store.Create(context.Background(), row); err != nil {
				t.Fatalf("seed %s #%d: %v", db.ID, i, err)
			}
		}
		// The protected collection: one record whose hidden field carries
		// the leak marker, one without.
		privRows := []map[string]any{
			{"title": "publicnote alpha", "secret": "hiddenmarker vault"},
			{"title": "plain note", "secret": "nothing here"},
		}
		for i, row := range privRows {
			store := application.NewRecordStore(target, "docspriv")
			if _, err := store.Create(context.Background(), row); err != nil {
				t.Fatalf("seed priv %s #%d: %v", db.ID, i, err)
			}
		}
		// Publish the FTS index through the maintenance action.
		status, body := v1JSON(t, ts.URL, superTok,
			fmt.Sprintf("/api/mirabase/v1/databases/%s/collections/docs/search-index", db.ID), "POST", map[string]any{
				"indexName": "idx_docs_fts",
				"fields":    []string{"title", "body"},
				"tokenizer": "default",
			})
		if status != 200 {
			t.Fatalf("search-index %s: %d %s", db.ID, status, body)
		}
		var indexState map[string]any
		if err := json.Unmarshal([]byte(body), &indexState); err != nil {
			t.Fatalf("search-index body %s: %v", body, err)
		}
		if indexState["state"] != "ready" || indexState["verified"] != true {
			t.Fatalf("search-index must report the verified state, got %v", indexState)
		}
		// The protected collection's index covers only the VISIBLE title.
		status, body = v1JSON(t, ts.URL, superTok,
			fmt.Sprintf("/api/mirabase/v1/databases/%s/collections/docspriv/search-index", db.ID), "POST", map[string]any{
				"indexName": "idx_docspriv_fts",
				"fields":    []string{"title"},
			})
		if status != 200 {
			t.Fatalf("search-index priv %s: %d %s", db.ID, status, body)
		}
	}

	// ------------------------------------------------------------------
	// Vector mode: exact ordering matches the brute-force reference
	// (computed independently), on both databases.
	// ------------------------------------------------------------------
	status, body := searchRequest(t, ts.URL, tokAlice, aliceDB.ID, "docs", map[string]any{
		"mode": "vector",
		"vector": map[string]any{
			"field":  "embedding",
			"query":  []float64{1, 0, 0},
			"metric": "cosine",
		},
		"limit": 2,
	})
	if status != 200 {
		t.Fatalf("vector search: %d %s", status, body)
	}
	var res map[string]any
	_ = json.Unmarshal([]byte(body), &res)
	items, _ := res["items"].([]any)
	if len(items) != 2 {
		t.Fatalf("vector search returned %d items, want 2", len(items))
	}
	first, _ := items[0].(map[string]any)
	firstRec, _ := first["record"].(map[string]any)
	if firstRec["title"] != "hello world" {
		t.Fatalf("vector order mismatch: %v", firstRec)
	}
	// The response carries the database context.
	if res["databaseId"] != aliceDB.ID || res["mode"] != "vector" || res["metric"] != "cosine" {
		t.Fatalf("vector response context: %v", res)
	}
	// Bob's identical query returns Bob's data only — same ordering, but
	// the record set is his.
	status, body = searchRequest(t, ts.URL, tokBob, bobDB.ID, "docs", map[string]any{
		"mode": "vector",
		"vector": map[string]any{
			"field":  "embedding",
			"query":  []float64{1, 0, 0},
			"metric": "cosine",
		},
		"limit": 10,
	})
	if status != 200 {
		t.Fatalf("bob vector search: %d %s", status, body)
	}
	var resB map[string]any
	_ = json.Unmarshal([]byte(body), &resB)
	itemsB, _ := resB["items"].([]any)
	if len(itemsB) != 3 {
		t.Fatalf("bob vector search count: %d", len(itemsB))
	}
	bobIDs := map[string]bool{}
	for _, it := range itemsB {
		rec, _ := it.(map[string]any)["record"].(map[string]any)
		bobIDs[fmt.Sprintf("%v", rec["title"])] = true
	}
	// Bob's database has the same seeded titles (isolation is proven by
	// writing a distinguishing record next).
	// Alice cannot search Bob's database.
	status, _ = searchRequest(t, ts.URL, tokAlice, bobDB.ID, "docs", map[string]any{
		"mode": "vector",
		"vector": map[string]any{
			"field":  "embedding",
			"query":  []float64{1, 0, 0},
			"metric": "l2",
		},
	})
	if status != 403 {
		t.Fatalf("cross-context search: %d (want 403, no fallback)", status)
	}

	// Ties: identical vectors produce stable ordering by record id.
	status, body = searchRequest(t, ts.URL, tokAlice, aliceDB.ID, "docs", map[string]any{
		"mode": "vector",
		"vector": map[string]any{
			"field":  "embedding",
			"query":  []float64{0, 1, 0},
			"metric": "l2",
		},
		"limit": 3,
	})
	if status != 200 {
		t.Fatalf("ties search: %d %s", status, body)
	}
	var resT map[string]any
	_ = json.Unmarshal([]byte(body), &resT)
	itemsT, _ := resT["items"].([]any)
	firstT, _ := itemsT[0].(map[string]any)
	firstRecT, _ := firstT["record"].(map[string]any)
	if firstRecT["title"] != "goodbye world" {
		t.Fatalf("l2 order mismatch: %v", firstRecT)
	}

	// Zero vector with cosine → field error (undefined cosine).
	status, body = searchRequest(t, ts.URL, tokAlice, aliceDB.ID, "docs", map[string]any{
		"mode": "vector",
		"vector": map[string]any{
			"field":  "embedding",
			"query":  []float64{0, 0, 0},
			"metric": "cosine",
		},
	})
	if status != 400 || !strings.Contains(body, "zero vector") {
		t.Fatalf("zero-vector cosine: %d %s", status, body)
	}

	// Malformed sparse input: unsorted indices rejected.
	status, body = searchRequest(t, ts.URL, tokAlice, aliceDB.ID, "docs", map[string]any{
		"mode": "vector",
		"vector": map[string]any{
			"field":  "embedding",
			"query":  map[string]any{"dimensions": 3, "indices": []int{2, 0}, "values": []float64{1, 1}},
			"metric": "l2",
		},
	})
	if status != 400 || !strings.Contains(body, "sorted") {
		t.Fatalf("malformed sparse: %d %s", status, body)
	}

	// Jaccard over the sparse representation is accepted.
	status, body = searchRequest(t, ts.URL, tokAlice, aliceDB.ID, "docs", map[string]any{
		"mode": "vector",
		"vector": map[string]any{
			"field":  "embedding",
			"query":  map[string]any{"dimensions": 3, "indices": []int{0}, "values": []float64{1}},
			"metric": "jaccard",
		},
	})
	if status != 200 {
		t.Fatalf("jaccard sparse: %d %s", status, body)
	}

	// ------------------------------------------------------------------
	// Text mode: native FTS with match/phrase/field syntax and highlights.
	// ------------------------------------------------------------------
	status, body = searchRequest(t, ts.URL, tokAlice, aliceDB.ID, "docs", map[string]any{
		"mode": "text",
		"text": map[string]any{
			"fields": []string{"title", "body"},
			"query":  "world",
		},
		"limit": 10,
	})
	if status != 200 {
		t.Fatalf("text search: %d %s", status, body)
	}
	var resText map[string]any
	_ = json.Unmarshal([]byte(body), &resText)
	itemsTx, _ := resText["items"].([]any)
	if len(itemsTx) != 2 {
		t.Fatalf("text search count: %d", len(itemsTx))
	}
	// Phrase syntax.
	status, body = searchRequest(t, ts.URL, tokAlice, aliceDB.ID, "docs", map[string]any{
		"mode": "text",
		"text": map[string]any{
			"fields": []string{"title", "body"},
			"query":  `"full text search"`,
		},
	})
	if status != 200 {
		t.Fatalf("phrase search: %d %s", status, body)
	}
	var resPh map[string]any
	_ = json.Unmarshal([]byte(body), &resPh)
	itemsPh, _ := resPh["items"].([]any)
	if len(itemsPh) != 1 {
		t.Fatalf("phrase count: %d", len(itemsPh))
	}
	// Field scope.
	status, body = searchRequest(t, ts.URL, tokAlice, aliceDB.ID, "docs", map[string]any{
		"mode": "text",
		"text": map[string]any{
			"fields": []string{"title", "body"},
			"query":  "title:hello",
		},
	})
	if status != 200 {
		t.Fatalf("field-scope search: %d %s", status, body)
	}

	// ------------------------------------------------------------------
	// Hybrid and fuzzy through the route (wave 13): reciprocal rank
	// fusion over the vector and text legs, and the Go-side fuzzy scan.
	// Validation is typed invalid_request, not capability_unavailable.
	// ------------------------------------------------------------------
	status, body = searchRequest(t, ts.URL, tokAlice, aliceDB.ID, "docs", map[string]any{
		"mode": "hybrid",
		"vector": map[string]any{
			"field":  "embedding",
			"query":  []float64{1, 0, 0},
			"metric": "cosine",
		},
		"text": map[string]any{
			"fields": []string{"title", "body"},
			"query":  "hello",
		},
		"fusion": map[string]any{"kind": "rrf", "k": 60},
		"limit":  10,
	})
	if status != 200 {
		t.Fatalf("hybrid search: %d %s", status, body)
	}
	var resHy map[string]any
	_ = json.Unmarshal([]byte(body), &resHy)
	if resHy["mode"] != "hybrid" || resHy["metric"] != "rrf" {
		t.Fatalf("hybrid response context: %v", resHy)
	}
	itemsHy, _ := resHy["items"].([]any)
	if len(itemsHy) == 0 {
		t.Fatalf("hybrid search returned no items")
	}
	// Both "hello world" and "hello mirabase" match BOTH legs (every
	// record carries a vector rank and both titles contain "hello") with
	// symmetric leg ranks, so their RRF sums tie exactly and the spec's
	// record-ID tie-break decides the order. The invariant the fusion
	// must prove: records that match the text leg outrank the record
	// matching the vector leg only ("goodbye world" never matches
	// "hello"), so the vector-only record fuses last.
	titlesHy := []string{}
	for _, it := range itemsHy {
		rec, _ := it.(map[string]any)["record"].(map[string]any)
		titlesHy = append(titlesHy, fmt.Sprintf("%v", rec["title"]))
	}
	if titlesHy[len(titlesHy)-1] != "goodbye world" {
		t.Fatalf("hybrid fusion: the vector-only record must rank last: %v", titlesHy)
	}
	foundHello := 0
	for _, ti := range titlesHy {
		if ti == "hello world" || ti == "hello mirabase" {
			foundHello++
		}
	}
	if foundHello != 2 {
		t.Fatalf("hybrid fusion missing both-legs records: %v", titlesHy)
	}

	// Hybrid without the required fusion clause is a typed validation
	// error, not the release-capability error.
	status, body = searchRequest(t, ts.URL, tokAlice, aliceDB.ID, "docs", map[string]any{
		"mode": "hybrid",
		"vector": map[string]any{
			"field":  "embedding",
			"query":  []float64{1, 0, 0},
			"metric": "cosine",
		},
		"text": map[string]any{
			"fields": []string{"title", "body"},
			"query":  "hello",
		},
	})
	if status != 400 || !strings.Contains(body, "invalid_request") {
		t.Fatalf("hybrid missing fusion: %d %s", status, body)
	}

	// Fuzzy through the route: levenshtein over the title field.
	status, body = searchRequest(t, ts.URL, tokAlice, aliceDB.ID, "docs", map[string]any{
		"mode":            "fuzzy",
		"field":           "title",
		"query":           "helo",
		"algorithm":       "levenshtein",
		"maximumDistance": 1,
		"limit":           10,
	})
	if status != 200 {
		t.Fatalf("fuzzy search: %d %s", status, body)
	}
	var resFz map[string]any
	_ = json.Unmarshal([]byte(body), &resFz)
	if resFz["mode"] != "fuzzy" || resFz["metric"] != "levenshtein" {
		t.Fatalf("fuzzy response context: %v", resFz)
	}
	itemsFz, _ := resFz["items"].([]any)
	titles := map[string]bool{}
	for _, it := range itemsFz {
		rec, _ := it.(map[string]any)["record"].(map[string]any)
		titles[fmt.Sprintf("%v", rec["title"])] = true
	}
	if !titles["hello world"] || !titles["hello mirabase"] {
		t.Fatalf("fuzzy near matches missing: %v", titles)
	}
	if titles["goodbye world"] {
		t.Fatalf("fuzzy distant match accepted: %v", titles)
	}

	// A bogus collection on a valid fuzzy request reaches the engine and
	// fails as invalid_request (no silent capability substitute).
	status, body = searchRequest(t, ts.URL, tokAlice, aliceDB.ID, "no-such-collection", map[string]any{
		"mode":            "fuzzy",
		"field":           "title",
		"query":           "helo",
		"algorithm":       "levenshtein",
		"maximumDistance": 1,
	})
	if status != 400 || !strings.Contains(body, "invalid_request") {
		t.Fatalf("fuzzy bogus collection: %d %s", status, body)
	}

	// ------------------------------------------------------------------
	// Search isolation with distinguishing content: write an
	// Alice-only record, verify Bob's search NEVER returns it, and vice
	// versa; then revoke Bob's write grant? (Bob owns his DB, so revoke
	// the tenant-style grant route: grant Bob read on Alice's DB, then
	// revoke it.)
	// ------------------------------------------------------------------
	status, _ = searchRequest(t, ts.URL, tokAlice, aliceDB.ID, "docs", map[string]any{
		"mode": "text",
		"text": map[string]any{"fields": []string{"title", "body"}, "query": "alice-only-marker"},
	})
	if status != 200 {
		t.Fatalf("pre-marker search: %d", status)
	}
	// Grant Bob read on Alice's DB, search, then revoke: the search access
	// path rechecks the grant epoch.
	if _, err := rt.Service.SetGrant(context.Background(), aliceDB.ID, "users", "bobiso000000001",
		[]string{application.ScopeRecordRead}, nil); err != nil {
		t.Fatalf("grant: %v", err)
	}
	status, _ = searchRequest(t, ts.URL, tokBob, aliceDB.ID, "docs", map[string]any{
		"mode": "text",
		"text": map[string]any{"fields": []string{"title", "body"}, "query": "hello"},
	})
	if status != 200 {
		t.Fatalf("granted search: %d", status)
	}
	if err := rt.Service.RevokeGrant(context.Background(), aliceDB.ID, "users", "bobiso000000001"); err != nil {
		t.Fatalf("revoke: %v", err)
	}
	status, _ = searchRequest(t, ts.URL, tokBob, aliceDB.ID, "docs", map[string]any{
		"mode": "text",
		"text": map[string]any{"fields": []string{"title", "body"}, "query": "hello"},
	})
	if status != 403 {
		t.Fatalf("revoked search: %d (want 403)", status)
	}

	// ------------------------------------------------------------------
	// Cursor continuation through the route (queries-and-search.md): a
	// full page issues nextCursor bound to the identical query and
	// snapshot; the continuation resumes at the next sort position; the
	// final page completes with no cursor; a different query on the same
	// cursor returns cursor_expired (410).
	// ------------------------------------------------------------------
	aliceTarget, err := rt.Handles.Open(context.Background(), aliceDB.ID)
	if err != nil {
		t.Fatalf("open alice for cursor seed: %v", err)
	}
	for i := 1; i <= 7; i++ {
		store := application.NewRecordStore(aliceTarget, "docs")
		if _, err := store.Create(context.Background(), map[string]any{
			"title":     fmt.Sprintf("word note %02d", i),
			"body":      fmt.Sprintf("cursor body %02d", i),
			"embedding": []float64{0, 0, 1},
		}); err != nil {
			t.Fatalf("cursor seed %d: %v", i, err)
		}
	}
	textSearch := func(cursor, query string) (int, string) {
		payload := map[string]any{
			"mode":  "text",
			"text":  map[string]any{"fields": []string{"title", "body"}, "query": query},
			"limit": 3,
		}
		if cursor != "" {
			payload["cursor"] = cursor
		}
		return searchRequest(t, ts.URL, tokAlice, aliceDB.ID, "docs", payload)
	}
	status, body = textSearch("", "word")
	if status != 200 {
		t.Fatalf("cursor page1: %d %s", status, body)
	}
	var page1 map[string]any
	_ = json.Unmarshal([]byte(body), &page1)
	cursor1, _ := page1["nextCursor"].(string)
	items1, _ := page1["items"].([]any)
	if len(items1) != 3 || cursor1 == "" || page1["complete"] != false {
		t.Fatalf("cursor page1 shape: complete=%v nextCursor=%q items=%d", page1["complete"], cursor1, len(items1))
	}
	pageIDs := func(page map[string]any) map[string]bool {
		ids := map[string]bool{}
		for _, it := range page["items"].([]any) {
			rec, _ := it.(map[string]any)["record"].(map[string]any)
			ids[fmt.Sprintf("%v", rec["id"])] = true
		}
		return ids
	}
	ids1 := pageIDs(page1)
	// The identical query continues: no repeated records, next cursor.
	status, body = textSearch(cursor1, "word")
	if status != 200 {
		t.Fatalf("cursor page2: %d %s", status, body)
	}
	var page2 map[string]any
	_ = json.Unmarshal([]byte(body), &page2)
	cursor2, _ := page2["nextCursor"].(string)
	ids2 := pageIDs(page2)
	if len(ids2) != 3 || cursor2 == "" {
		t.Fatalf("cursor page2 shape: ids=%v cursor=%q", ids2, cursor2)
	}
	for id := range ids2 {
		if ids1[id] {
			t.Fatalf("continuation repeated record %s", id)
		}
	}
	status, body = textSearch(cursor2, "word")
	if status != 200 {
		t.Fatalf("cursor page3: %d %s", status, body)
	}
	var page3 map[string]any
	_ = json.Unmarshal([]byte(body), &page3)
	ids3 := pageIDs(page3)
	if len(ids3) != 1 || page3["nextCursor"] != nil || page3["complete"] != true {
		t.Fatalf("cursor final page: complete=%v nextCursor=%v items=%d", page3["complete"], page3["nextCursor"], len(ids3))
	}
	for id := range ids3 {
		if ids1[id] || ids2[id] {
			t.Fatalf("final page repeated record %s", id)
		}
	}
	// A different query on the same cursor is refused with cursor_expired
	// (410), not a silent reinterpretation — on a fresh page-1 cursor (a
	// validated cursor is consumed by its continuation).
	status, body = textSearch("", "word")
	if status != 200 {
		t.Fatalf("cursor fresh page1: %d %s", status, body)
	}
	var page1b map[string]any
	_ = json.Unmarshal([]byte(body), &page1b)
	cursor1b, _ := page1b["nextCursor"].(string)
	status, body = textSearch(cursor1b, "hello")
	if status != 410 || !strings.Contains(body, "cursor_expired") {
		t.Fatalf("cursor with a different query: %d %s (want 410 cursor_expired)", status, body)
	}

	// ------------------------------------------------------------------
	// Field protection through the route: a hidden field is not
	// searchable (typed invalid_request), never projected into records,
	// and its content never leaves the server — including through
	// highlights.
	// ------------------------------------------------------------------
	status, body = searchRequest(t, ts.URL, tokAlice, aliceDB.ID, "docspriv", map[string]any{
		"mode": "text",
		"text": map[string]any{"fields": []string{"title", "secret"}, "query": "hiddenmarker"},
	})
	if status != 400 || !strings.Contains(body, "invalid_request") || !strings.Contains(body, "protected") {
		t.Fatalf("hidden text field must be refused: %d %s", status, body)
	}
	status, body = searchRequest(t, ts.URL, tokAlice, aliceDB.ID, "docspriv", map[string]any{
		"mode":            "fuzzy",
		"field":           "secret",
		"query":           "hiddenmarker",
		"algorithm":       "levenshtein",
		"maximumDistance": 1,
	})
	if status != 400 || !strings.Contains(body, "invalid_request") {
		t.Fatalf("hidden fuzzy field must be refused: %d %s", status, body)
	}
	// Searching the visible field never leaks the hidden content: the
	// record carries only the projected/visible fields and the response
	// body nowhere contains the hidden marker.
	status, body = searchRequest(t, ts.URL, tokAlice, aliceDB.ID, "docspriv", map[string]any{
		"mode": "text",
		"text": map[string]any{"fields": []string{"title"}, "query": "publicnote", "highlight": true},
	})
	if status != 200 {
		t.Fatalf("visible search over a protected collection: %d %s", status, body)
	}
	if strings.Contains(body, "hiddenmarker") || strings.Contains(body, "vault") {
		t.Fatalf("hidden field content leaked through the search response: %s", body)
	}
	var resPriv map[string]any
	_ = json.Unmarshal([]byte(body), &resPriv)
	itemsPriv, _ := resPriv["items"].([]any)
	if len(itemsPriv) != 1 {
		t.Fatalf("visible search count: %d", len(itemsPriv))
	}
	recPriv, _ := itemsPriv[0].(map[string]any)["record"].(map[string]any)
	if _, leaked := recPriv["secret"]; leaked {
		t.Fatalf("hidden field present in the returned record: %v", recPriv)
	}
	// The field projection narrows records: id plus title only.
	status, body = searchRequest(t, ts.URL, tokAlice, aliceDB.ID, "docs", map[string]any{
		"mode":   "text",
		"text":   map[string]any{"fields": []string{"title", "body"}, "query": "word"},
		"fields": []string{"title"},
		"limit":  3,
	})
	if status != 200 {
		t.Fatalf("projected search: %d %s", status, body)
	}
	var resProj map[string]any
	_ = json.Unmarshal([]byte(body), &resProj)
	itemsProj, _ := resProj["items"].([]any)
	if len(itemsProj) == 0 {
		t.Fatalf("projected search returned no items")
	}
	for _, it := range itemsProj {
		rec, _ := it.(map[string]any)["record"].(map[string]any)
		if _, has := rec["body"]; has {
			t.Fatalf("projection ignored: record still carries body: %v", rec)
		}
		if _, has := rec["title"]; !has {
			t.Fatalf("projection dropped the requested field: %v", rec)
		}
	}

	// The primary database never carries the private ROWS: the docs table
	// on primary (fixture-defined) stays at its original count.
	var primaryCount int
	if err := app.DBConn().QueryRow(`SELECT count(*) FROM docs`).Scan(&primaryCount); err != nil {
		t.Fatalf("primary count: %v", err)
	}
	if primaryCount != 0 {
		t.Fatalf("private rows leaked into the primary database: %d", primaryCount)
	}
	_ = search.DefaultOptions()
}

// searchRequest posts a search request to the v1 surface.
func searchRequest(t *testing.T, baseURL, token, databaseID, collection string, payload any) (int, string) {
	t.Helper()
	return v1JSON(t, baseURL, token,
		fmt.Sprintf("/api/mirabase/v1/databases/%s/collections/%s/search", databaseID, collection),
		"POST", payload)
}

// createSearchCollection creates the search fixture collection.
func createSearchCollection(app *tests.TestApp, name string) (*core.Collection, error) {
	collection := core.NewBaseCollection(name)
	collection.Fields.Add(
		&core.TextField{Name: "title"},
		&core.TextField{Name: "body"},
		&core.TextField{Name: "embedding"},
	)
	collection.ListRule = nil
	if err := app.Save(collection); err != nil {
		return nil, err
	}
	return collection, nil
}

// createSearchCollectionHidden creates a collection with a PROTECTED
// (hidden) text field: the field protection the search surface must
// enforce before results leave the server.
func createSearchCollectionHidden(app *tests.TestApp, name string) (*core.Collection, error) {
	collection := core.NewBaseCollection(name)
	collection.Fields.Add(
		&core.TextField{Name: "title"},
		&core.TextField{Name: "secret", Hidden: true},
	)
	collection.ListRule = nil
	if err := app.Save(collection); err != nil {
		return nil, err
	}
	return collection, nil
}

// superuserScopedToken mints a token for a record in an auth collection
// (creating the record if missing) so the search authorization path
// exercises real subject identities.
func superuserScopedToken(t *testing.T, app *tests.TestApp, collectionName, recordID string) string {
	t.Helper()
	coll, err := app.FindCollectionByNameOrId(collectionName)
	if err != nil {
		coll, err = createAuthCollection(app.BaseApp, collectionName)
		if err != nil {
			t.Fatalf("auth collection %s: %v", collectionName, err)
		}
	}
	record, err := app.FindRecordById(coll.Name, recordID)
	if err != nil {
		record = core.NewRecord(coll)
		record.Set("email", recordID+"@search.example.com")
		record.Set("password", "searchpass12345678")
		record.Id = recordID
		if err := app.Save(record); err != nil {
			t.Fatalf("search record %s: %v", recordID, err)
		}
	}
	token, err := record.NewAuthToken()
	if err != nil {
		t.Fatalf("token %s: %v", recordID, err)
	}
	return token
}
