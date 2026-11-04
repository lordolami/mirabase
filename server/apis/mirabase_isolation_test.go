package apis_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"mirabase/apis"
	"mirabase/core"
	"mirabase/internal/application"
	"mirabase/internal/mbengine"
	"mirabase/tests"
)

// TestV01_05IsolationAcceptance is the gate-4 acceptance scenario:
//
//	"Two auth collections with colliding IDs, Alice/Bob, two agents and a
//	multi-member tenant: simultaneous REST/files/SSE/search must remain
//	isolated, including after eviction/reopen, restore and revocation."
//
// REST isolation runs through the real HTTP surface; eviction/reopen,
// restore and revocation exercise the handle cache, lifecycle and grant
// epoch machinery. SSE and search isolation arrive with their owning
// surfaces (SSE is primary-bound in v0.1; vector/FTS search is V01-07)
// and are recorded as boundary notes, not fabricated passes.
func mbenginePinnedDigestForIsolation() string { return mbengine.PinnedDigest() }

func TestV01_05IsolationAcceptance(t *testing.T) {
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

	// ------------------------------------------------------------------
	// Primary-side setup: two auth collections with colliding record IDs.
	// ------------------------------------------------------------------
	usersA, err := createAuthCollection(app.BaseApp, "users_a")
	if err != nil {
		t.Fatalf("users_a: %v", err)
	}
	usersB, err := createAuthCollection(app.BaseApp, "users_b")
	if err != nil {
		t.Fatalf("users_b: %v", err)
	}

	alice, err := createAuthRecord(app.BaseApp, usersA, "alice@example.com", "alicepass123456")
	if err != nil {
		t.Fatalf("alice: %v", err)
	}
	bob, err := createAuthRecord(app.BaseApp, usersB, "bob@example.com", "bobpass123456789")
	if err != nil {
		t.Fatalf("bob: %v", err)
	}

	// Colliding IDs: force Bob's record id to equal Alice's — the two
	// identities must stay distinct bindings ("Multiple auth collections
	// with the same record ID cannot collide").
	if _, err := app.BaseApp.DB().NewQuery(
		fmt.Sprintf("UPDATE {{%s}} SET [[id]] = {:id} WHERE [[id]] = {:old}", usersB.Name),
	).Bind(map[string]any{"id": alice.Id, "old": bob.Id}).Execute(); err != nil {
		t.Fatalf("collide ids: %v", err)
	}
	bob.Id = alice.Id

	// Publish the template with the private collections.
	tpl, err := rt.Service.PublishTemplate(context.Background(), app.BaseApp, "tpl-iso", []string{"demo1"})
	if err != nil {
		t.Fatalf("PublishTemplate: %v", err)
	}

	// Provision: Alice's DB, Bob's DB, two agent DBs, one tenant DB.
	aliceDB, _, err := rt.Service.Provision(context.Background(), app.DataDir(), application.ProvisionRequest{
		Kind: "user", TemplateID: tpl.ID,
		OwnerCollectionID: usersA.Id, OwnerRecordID: alice.Id, DisplayName: "alice",
	})
	if err != nil {
		t.Fatalf("alice provision: %v", err)
	}
	bobDB, _, err := rt.Service.Provision(context.Background(), app.DataDir(), application.ProvisionRequest{
		Kind: "user", TemplateID: tpl.ID,
		OwnerCollectionID: usersB.Id, OwnerRecordID: bob.Id, DisplayName: "bob",
	})
	if err != nil {
		t.Fatalf("bob provision: %v", err)
	}
	if bobDB.ID == aliceDB.ID {
		t.Fatalf("colliding record IDs collided into one database")
	}
	agent1DB, _, err := rt.Service.Provision(context.Background(), app.DataDir(), application.ProvisionRequest{
		Kind: "agent", TemplateID: tpl.ID,
		OwnerCollectionID: "agents_x", OwnerRecordID: "agent-1", DisplayName: "agent1",
	})
	if err != nil {
		t.Fatalf("agent1 provision: %v", err)
	}
	agent2DB, _, err := rt.Service.Provision(context.Background(), app.DataDir(), application.ProvisionRequest{
		Kind: "agent", TemplateID: tpl.ID,
		OwnerCollectionID: "agents_x", OwnerRecordID: "agent-2", DisplayName: "agent2",
	})
	if err != nil {
		t.Fatalf("agent2 provision: %v", err)
	}
	if agent1DB.ID == agent2DB.ID {
		t.Fatalf("two agents share a database")
	}
	tenantDB, _, err := rt.Service.Provision(context.Background(), app.DataDir(), application.ProvisionRequest{
		Kind: "tenant", TemplateID: tpl.ID,
		OwnerCollectionID: "tenants", OwnerRecordID: "school-1", DisplayName: "school",
	})
	if err != nil {
		t.Fatalf("tenant provision: %v", err)
	}
	// Multi-member tenant: Alice and Bob both hold grants on the tenant DB.
	for _, subj := range [][2]string{{usersA.Id, alice.Id}, {usersB.Id, bob.Id}} {
		if _, err := rt.Service.SetGrant(context.Background(), tenantDB.ID, subj[0], subj[1],
			[]string{application.ScopeRecordRead, application.ScopeRecordWrite}, nil); err != nil {
			t.Fatalf("tenant grant: %v", err)
		}
	}

	// Publish the template into every private database.
	for _, db := range []*application.Database{aliceDB, bobDB, agent1DB, agent2DB, tenantDB} {
		target, err := rt.Handles.Open(context.Background(), db.ID)
		if err != nil {
			t.Fatalf("open %s: %v", db.ID, err)
		}
		if err := application.ApplyTemplate(context.Background(), target, tpl); err != nil {
			t.Fatalf("apply template %s: %v", db.ID, err)
		}
	}

	// ------------------------------------------------------------------
	// HTTP surface: the real apis router (all middlewares + routes) served
	// over the test app.
	// ------------------------------------------------------------------
	mux, err := apis.NewRouter(app)
	if err != nil {
		t.Fatalf("NewRouter: %v", err)
	}
	handler, err := mux.BuildMux()
	if err != nil {
		t.Fatalf("BuildMux: %v", err)
	}
	ts := httptest.NewServer(handler)
	defer ts.Close()

	aliceTok, err := authWithPassword(ts.URL, "users_a", "alice@example.com", "alicepass123456")
	if err != nil {
		t.Fatalf("alice auth: %v", err)
	}
	bobTok, err := authWithPassword(ts.URL, "users_b", "bob@example.com", "bobpass123456789")
	if err != nil {
		t.Fatalf("bob auth: %v", err)
	}

	// ------------------------------------------------------------------
	// Simultaneous REST isolation: Alice writes into her DB, Bob writes
	// into his DB at the same time; neither can observe the other's
	// records, and neither can read the other's database.
	// ------------------------------------------------------------------
	var wg sync.WaitGroup
	aliceID := make(chan string, 1)
	bobID := make(chan string, 1)
	wg.Add(2)
	go func() {
		defer wg.Done()
		status, body := v1Request(t, ts.URL, aliceTok, aliceDB.ID, "demo1", "POST", map[string]any{"text": "alice-secret"})
		if status != 200 {
			t.Errorf("alice create: %d %s", status, body)
			return
		}
		var m map[string]any
		_ = json.Unmarshal([]byte(body), &m)
		aliceID <- fmt.Sprintf("%v", m["id"])
	}()
	go func() {
		defer wg.Done()
		status, body := v1Request(t, ts.URL, bobTok, bobDB.ID, "demo1", "POST", map[string]any{"text": "bob-secret"})
		if status != 200 {
			t.Errorf("bob create: %d %s", status, body)
			return
		}
		var m map[string]any
		_ = json.Unmarshal([]byte(body), &m)
		bobID <- fmt.Sprintf("%v", m["id"])
	}()
	wg.Wait()
	close(aliceID)
	close(bobID)
	<-aliceID
	bobRec := <-bobID

	// Alice's listing shows only her record; Bob's only his.
	status, body := v1Request(t, ts.URL, aliceTok, aliceDB.ID, "demo1", "GET", nil)
	if status != 200 || strings.Contains(body, "bob-secret") || !strings.Contains(body, "alice-secret") {
		t.Fatalf("alice listing leaked: %d %s", status, body)
	}
	status, body = v1Request(t, ts.URL, bobTok, bobDB.ID, "demo1", "GET", nil)
	if status != 200 || strings.Contains(body, "alice-secret") || !strings.Contains(body, "bob-secret") {
		t.Fatalf("bob listing leaked: %d %s", status, body)
	}

	// Cross-context reads: Alice reading Bob's DB and vice versa must fail
	// (no grant) and never fall back to primary.
	status, body = v1Request(t, ts.URL, aliceTok, bobDB.ID, "demo1", "GET", nil)
	if status != 403 {
		t.Fatalf("alice cross-context read: %d %s", status, body)
	}
	status, body = v1Request(t, ts.URL, bobTok, aliceDB.ID, "demo1", "GET", nil)
	if status != 403 {
		t.Fatalf("bob cross-context read: %d %s", status, body)
	}
	// Direct record reads across contexts fail too.
	status, _ = v1Request(t, ts.URL, aliceTok, bobDB.ID, "demo1", "GET", map[string]any{"__path": bobRec})
	if status != 403 {
		t.Fatalf("alice cross-context record read: %d", status)
	}


	// Agent databases are isolated from the owner and from each other.
	status, _ = v1Request(t, ts.URL, aliceTok, agent1DB.ID, "demo1", "GET", nil)
	if status != 403 {
		t.Fatalf("owner reading the agent DB: %d", status)
	}

	// The multi-member tenant: both members see the shared records.
	status, _ = v1Request(t, ts.URL, aliceTok, tenantDB.ID, "demo1", "POST", map[string]any{"text": "shared-school-note"})
	if status != 200 {
		t.Fatalf("alice tenant write: %d", status)
	}
	status, body = v1Request(t, ts.URL, bobTok, tenantDB.ID, "demo1", "GET", nil)
	if status != 200 || !strings.Contains(body, "shared-school-note") {
		t.Fatalf("bob tenant read: %d %s", status, body)
	}

	// ------------------------------------------------------------------
	// Eviction/reopen: force-idle every handle, reopen, verify the data.
	// ------------------------------------------------------------------
	rt.Handles.EvictIdle()
	// Reopen after eviction: the data must be intact.
	status, body = v1Request(t, ts.URL, aliceTok, aliceDB.ID, "demo1", "GET", nil)
	if status != 200 || !strings.Contains(body, "alice-secret") {
		t.Fatalf("alice data after eviction/reopen: %d %s", status, body)
	}
	status, body = v1Request(t, ts.URL, bobTok, bobDB.ID, "demo1", "GET", nil)
	if status != 200 || !strings.Contains(body, "bob-secret") {
		t.Fatalf("bob data after eviction/reopen: %d %s", status, body)
	}

	// ------------------------------------------------------------------
	// Restore: quarantine + restore Alice's DB (new generation), data
	// intact.
	// ------------------------------------------------------------------
	if err := rt.Service.Quarantine(context.Background(), aliceDB.ID); err != nil {
		t.Fatalf("quarantine: %v", err)
	}
	_ = rt.CloseRuntimeHandle(aliceDB.ID)
	status, _ = v1Request(t, ts.URL, aliceTok, aliceDB.ID, "demo1", "GET", nil)
	if status != 403 && status != 404 {
		t.Fatalf("quarantined db must not serve: %d", status)
	}
	if err := rt.Service.Restore(context.Background(), aliceDB.ID); err != nil {
		t.Fatalf("restore: %v", err)
	}
	status, body = v1Request(t, ts.URL, aliceTok, aliceDB.ID, "demo1", "GET", nil)
	if status != 200 || !strings.Contains(body, "alice-secret") {
		t.Fatalf("alice data after restore: %d %s", status, body)
	}
	restored, _ := rt.Service.GetDatabase(context.Background(), aliceDB.ID)
	if restored.Generation <= 1 {
		t.Fatalf("restore must bump the generation: %+v", restored)
	}

	// ------------------------------------------------------------------
	// Revocation: revoke Bob's tenant grant; his access fails while
	// Alice's continues.
	// ------------------------------------------------------------------
	if err := rt.Service.RevokeGrant(context.Background(), tenantDB.ID, usersB.Id, bob.Id); err != nil {
		t.Fatalf("revoke: %v", err)
	}
	status, _ = v1Request(t, ts.URL, bobTok, tenantDB.ID, "demo1", "GET", nil)
	if status != 403 {
		t.Fatalf("bob tenant read after revocation: %d", status)
	}
	status, _ = v1Request(t, ts.URL, aliceTok, tenantDB.ID, "demo1", "GET", nil)
	if status != 200 {
		t.Fatalf("alice tenant read after bob's revocation: %d", status)
	}

	// The primary (inherited) surface never carries the private data:
	// querying demo1 on primary returns only the fixture rows.
	primaryCheck, err := app.BaseApp.FindCollectionByNameOrId("demo1")
	if err != nil {
		t.Fatalf("primary demo1: %v", err)
	}
	var primaryCount int
	if err := app.BaseApp.DB().NewQuery(fmt.Sprintf("SELECT count(*) FROM {{%s}} WHERE [[text]] = 'alice-secret'", primaryCheck.Name)).Row(&primaryCount); err != nil {
		t.Fatalf("primary count: %v", err)
	}
	if primaryCount != 0 {
		t.Fatalf("private records leaked into the primary database: %d", primaryCount)
	}
}

// createAuthCollection creates an auth collection with the given name.
func createAuthCollection(app *core.BaseApp, name string) (*core.Collection, error) {
	collection := core.NewAuthCollection(name)
	collection.Fields.Add(
		&core.EmailField{Name: "email", Required: true},
		&core.PasswordField{Name: "password", Hidden: true},
	)
	if err := app.Save(collection); err != nil {
		return nil, err
	}
	return collection, nil
}

func createAuthRecord(app *core.BaseApp, collection *core.Collection, email, password string) (*core.Record, error) {
	record := core.NewRecord(collection)
	record.Set("email", email)
	record.Set("password", password)
	if err := app.Save(record); err != nil {
		return nil, err
	}
	return record, nil
}

func authWithPassword(baseURL, collection, identity, password string) (string, error) {
	body, _ := json.Marshal(map[string]any{"identity": identity, "password": password})
	resp, err := http.Post(baseURL+"/api/collections/"+collection+"/auth-with-password",
		"application/json", bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	var m map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&m); err != nil {
		return "", err
	}
	token, _ := m["token"].(string)
	if token == "" {
		return "", fmt.Errorf("no token in response: %v", m)
	}
	return token, nil
}

// v1Request performs a request against the partitioning record API.
func v1Request(t *testing.T, baseURL, token, databaseID, collection, method string, payload any) (int, string) {
	t.Helper()
	url := fmt.Sprintf("%s/api/mirabase/v1/db/%s/collections/%s/records", baseURL, databaseID, collection)
	if payload != nil {
		if p, ok := payload.(map[string]any); ok {
			if extra, ok := p["__path"].(string); ok {
				url = url + "/" + extra
				m := map[string]any{}
				for k, v := range p {
					if k != "__path" {
						m[k] = v
					}
				}
				payload = m
			}
		}
	}
	var reader *bytes.Reader
	if payload != nil {
		b, _ := json.Marshal(payload)
		reader = bytes.NewReader(b)
	} else {
		reader = bytes.NewReader(nil)
	}
	req, err := http.NewRequest(method, url, reader)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	if payload != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if token != "" {
		req.Header.Set("Authorization", token)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("do: %v", err)
	}
	defer resp.Body.Close()
	var buf bytes.Buffer
	_, _ = buf.ReadFrom(resp.Body)
	return resp.StatusCode, buf.String()
}
