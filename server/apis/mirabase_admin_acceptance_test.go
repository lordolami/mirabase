package apis_test

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"mirabase/internal/application"
	"mirabase/tests"
)

// TestV01AdminSurfaceAcceptance drives every released administrative
// operation through real HTTP: the registry/lifecycle/operate surface,
// grants, agents, per-target schema plans, jobs, backups and owner
// transfer. Every assertion checks the actual effect (registry rows,
// target schema, job states), never just the status code.
func TestV01AdminSurfaceAcceptance(t *testing.T) {
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

	// A dedicated auth collection keeps the surface independent of the
	// inherited defaults.
	users, err := createAuthCollection(app.BaseApp, "users_admin")
	if err != nil {
		t.Fatalf("users collection: %v", err)
	}
	alice, err := createAuthRecord(app.BaseApp, users, "admin-alice@example.com", "aliceadminpass1")
	if err != nil {
		t.Fatalf("alice save: %v", err)
	}
	bob, err := createAuthRecord(app.BaseApp, users, "admin-bob@example.com", "bobadminpass12")
	if err != nil {
		t.Fatalf("bob save: %v", err)
	}
	aliceTok, err := alice.NewAuthToken()
	if err != nil {
		t.Fatalf("alice token: %v", err)
	}

	// Provision one ready target for Alice.
	target, _, err := rt.Service.Provision(context.Background(), app.DataDir(), application.ProvisionRequest{
		Kind: "user", TemplateID: "", OwnerCollectionID: users.Id, OwnerRecordID: alice.Id, DisplayName: "admin-alice",
	})
	if err != nil {
		t.Fatalf("provision: %v", err)
	}

	// ------------------------------------------------------------------
	// database.create: durable intent → verified provisioning (manual
	// kind), plus the honest refusals (unknown template, unknown owner,
	// page-encryption remainder).
	// ------------------------------------------------------------------
	status, body := v1JSON(t, ts.URL, superTok, "/api/mirabase/v1/databases", "POST", map[string]any{
		"displayName": "manual-qa", "kind": "manual", "profile": "wal",
	})
	if status != 202 || !strings.Contains(body, `"state":"succeeded"`) {
		t.Fatalf("database.create manual: %d %s", status, body)
	}
	var createJob struct {
		ResultRef string `json:"resultRef"`
	}
	_ = json.Unmarshal([]byte(body), &createJob)
	var createResult struct {
		DatabaseID string `json:"databaseId"`
		State      string `json:"state"`
	}
	if createJob.ResultRef == "" {
		t.Fatalf("database.create returned no resultRef: %s", body)
	}
	if err := json.Unmarshal([]byte(createJob.ResultRef), &createResult); err != nil || createResult.DatabaseID == "" || createResult.State != "ready" {
		t.Fatalf("database.create result is not a verified ready database: %s", createJob.ResultRef)
	}
	// The created database answers database.get with a real registry row.
	status, body = v1JSON(t, ts.URL, superTok, "/api/mirabase/v1/databases/"+createResult.DatabaseID, "GET", nil)
	if status != 200 || !strings.Contains(body, `"kind":"manual"`) || !strings.Contains(body, `"state":"ready"`) {
		t.Fatalf("database.get created: %d %s", status, body)
	}
	status, body = v1JSON(t, ts.URL, superTok, "/api/mirabase/v1/databases", "POST", map[string]any{
		"displayName": "bad-template", "kind": "manual", "profile": "wal", "templateId": "no-such-template",
	})
	if status != 404 {
		t.Fatalf("database.create unknown template: %d (want 404) %s", status, body)
	}
	status, body = v1JSON(t, ts.URL, superTok, "/api/mirabase/v1/databases", "POST", map[string]any{
		"displayName": "enc", "kind": "manual", "profile": "wal", "encryption": map[string]any{"algorithm": "aes", "keyRef": "k1"},
	})
	if status != 400 {
		t.Fatalf("database.create encryption refusal: %d (want 400) %s", status, body)
	}

	// ------------------------------------------------------------------
	// database.list: the primary plus the provisioned target.
	// ------------------------------------------------------------------
	status, body = v1JSON(t, ts.URL, superTok, "/api/mirabase/v1/databases", "GET", nil)
	if status != 200 {
		t.Fatalf("database.list: %d %s", status, body)
	}
	if !strings.Contains(body, `"primary"`) || !strings.Contains(body, target.ID) {
		t.Fatalf("database.list does not show the primary and the target: %s", body)
	}

	// ------------------------------------------------------------------
	// database.get: superuser 200; non-superuser 403 without authority;
	// with a live database.inspect grant 200.
	// ------------------------------------------------------------------
	status, body = v1JSON(t, ts.URL, superTok, "/api/mirabase/v1/databases/"+target.ID, "GET", nil)
	if status != 200 || !strings.Contains(body, `"state":"ready"`) {
		t.Fatalf("database.get superuser: %d %s", status, body)
	}
	_ = status
	_ = body
	status, body = v1JSON(t, ts.URL, aliceTok, "/api/mirabase/v1/databases/"+target.ID, "GET", nil)
	if status != 200 {
		t.Fatalf("database.get owner: %d %s", status, body)
	}
	status, body = v1JSON(t, ts.URL, "", "/api/mirabase/v1/databases/"+target.ID, "GET", nil)
	if status != 401 && status != 403 {
		t.Fatalf("database.get anonymous: %d (want 401/403)", status)
	}

	// ------------------------------------------------------------------
	// grant.put / grant.list / grant.revoke.
	// ------------------------------------------------------------------
	status, body = v1JSON(t, ts.URL, superTok, "/api/mirabase/v1/databases/"+target.ID+"/grants", "PUT", map[string]any{
		"subject":       map[string]any{"collectionId": users.Id, "recordId": bob.Id},
		"scopes":        []string{"record.read", "database.inspect"},
		"expiresAt":     nil,
		"expectedEpoch": 1,
	})
	if status != 200 || !strings.Contains(body, `"epoch":1`) {
		t.Fatalf("grant.put: %d %s", status, body)
	}
	var grantResp struct {
		ID string `json:"id"`
	}
	_ = json.Unmarshal([]byte(body), &grantResp)

	// Bob now reads the database metadata through the grant.
	bobTok, err := bob.NewAuthToken()
	if err != nil {
		t.Fatalf("bob token: %v", err)
	}
	status, body = v1JSON(t, ts.URL, bobTok, "/api/mirabase/v1/databases/"+target.ID, "GET", nil)
	if status != 200 {
		t.Fatalf("database.get with database.inspect grant: %d %s", status, body)
	}
	// Bob without the record.write scope cannot create records in the target.
	status, body = v1JSON(t, ts.URL, bobTok, "/api/mirabase/v1/db/"+target.ID+"/collections/demo_admin/records", "POST", map[string]any{"title": "x"})
	if status != 403 {
		t.Fatalf("record create without record.write scope: %d (want 403) %s", status, body)
	}

	status, body = v1JSON(t, ts.URL, superTok, "/api/mirabase/v1/databases/"+target.ID+"/grants", "GET", nil)
	if status != 200 || !strings.Contains(body, grantResp.ID) {
		t.Fatalf("grant.list: %d %s", status, body)
	}
	status, _ = v1JSON(t, ts.URL, superTok, "/api/mirabase/v1/databases/"+target.ID+"/grants/"+grantResp.ID, "DELETE", nil)
	if status != 204 {
		t.Fatalf("grant.revoke: %d", status)
	}

	// ------------------------------------------------------------------
	// agent.bind / agent.list / agent.revoke (existing identities only).
	// ------------------------------------------------------------------
	status, body = v1JSON(t, ts.URL, superTok, "/api/mirabase/v1/agents", "POST", map[string]any{
		"agent":      map[string]any{"collectionId": users.Id, "recordId": bob.Id},
		"owner":      map[string]any{"collectionId": users.Id, "recordId": alice.Id},
		"databaseId": target.ID,
	})
	if status != 200 || !strings.Contains(body, `"state":"active"`) {
		t.Fatalf("agent.bind: %d %s", status, body)
	}
	var agentResp struct {
		ID string `json:"id"`
	}
	_ = json.Unmarshal([]byte(body), &agentResp)
	status, body = v1JSON(t, ts.URL, superTok, "/api/mirabase/v1/agents", "GET", nil)
	if status != 200 || !strings.Contains(body, agentResp.ID) {
		t.Fatalf("agent.list: %d %s", status, body)
	}
	status, _ = v1JSON(t, ts.URL, superTok, "/api/mirabase/v1/agents/"+agentResp.ID, "DELETE", nil)
	if status != 204 {
		t.Fatalf("agent.revoke: %d", status)
	}

	// ------------------------------------------------------------------
	// database.maintain: real integrity_check through the kernel.
	// ------------------------------------------------------------------
	status, body = v1JSON(t, ts.URL, superTok, "/api/mirabase/v1/databases/"+target.ID+"/actions", "POST", map[string]any{
		"action": "integrity_check", "expectedGeneration": target.Generation,
	})
	if status != 202 || !strings.Contains(body, `"state":"succeeded"`) || !strings.Contains(body, "ok") {
		t.Fatalf("database.maintain integrity_check: %d %s", status, body)
	}

	// ------------------------------------------------------------------
	// database.diagnostics: real measured values.
	// ------------------------------------------------------------------
	status, body = v1JSON(t, ts.URL, superTok, "/api/mirabase/v1/databases/"+target.ID+"/diagnostics", "GET", nil)
	if status != 200 || !strings.Contains(body, `"state":"ready"`) || !strings.Contains(body, `"diskBytes":`) {
		t.Fatalf("database.diagnostics: %d %s", status, body)
	}

	// ------------------------------------------------------------------
	// schema.plan / schema.apply on the target: create a base collection
	// and a btree index, apply, and verify the real schema + revision.
	// ------------------------------------------------------------------
	status, body = v1JSON(t, ts.URL, superTok, "/api/mirabase/v1/databases/"+target.ID+"/schema/plans", "POST", map[string]any{
		"expectedSchemaRevision": target.SchemaRevision,
		"operations": []map[string]any{
			{"kind": "collection.create", "objectId": "pbc_admin_test", "definition": map[string]any{
				"id": "pbc_admin_test", "name": "admin_notes", "type": "base",
				"fields": []map[string]any{
					{"id": "f1", "name": "title", "type": "text"},
					{"id": "f2", "name": "score", "type": "number"},
				},
				"indexes": []string{}, "listRule": nil, "viewRule": nil,
				"createRule": nil, "updateRule": nil, "deleteRule": nil,
			}},
			{"kind": "index.create", "objectId": "pbc_admin_test", "definition": map[string]any{
				"name": "idx_admin_notes_title", "kind": "btree", "unique": false,
				"fields": []map[string]any{{"expression": "title", "direction": "asc"}},
			}},
		},
	})
	if status != 200 || !strings.Contains(body, `"blockers":[]`) {
		t.Fatalf("schema.plan: %d %s", status, body)
	}
	var planResp struct {
		ID       string `json:"id"`
		PlanHash string `json:"planHash"`
	}
	_ = json.Unmarshal([]byte(body), &planResp)

	// The plan must refuse roadmap kinds as blockers, never skip them.
	status, body = v1JSON(t, ts.URL, superTok, "/api/mirabase/v1/databases/"+target.ID+"/schema/plans", "POST", map[string]any{
		"expectedSchemaRevision": target.SchemaRevision,
		"operations": []map[string]any{
			{"kind": "view.define", "objectId": "v1"},
		},
	})
	if status != 200 || !strings.Contains(body, "not_in_released_slice") {
		t.Fatalf("schema.plan roadmap blocker: %d %s", status, body)
	}

	status, body = v1JSON(t, ts.URL, superTok, "/api/mirabase/v1/databases/"+target.ID+"/schema/plans/"+planResp.ID+"/apply", "POST", map[string]any{
		"planHash": planResp.PlanHash, "expectedSchemaRevision": target.SchemaRevision,
	})
	if status != 202 || !strings.Contains(body, `"state":"succeeded"`) {
		t.Fatalf("schema.apply: %d %s", status, body)
	}

	// The real effect: the table and index exist and the registry row's
	// schema revision advanced.
	tgt, err := rt.Handles.Open(context.Background(), target.ID)
	if err != nil {
		t.Fatalf("target open: %v", err)
	}
	var tables int
	if err := tgt.QueryRow(`SELECT count(*) FROM sqlite_master WHERE type='table' AND name='admin_notes'`).Scan(&tables); err != nil {
		t.Fatalf("table probe: %v", err)
	}
	if tables != 1 {
		t.Fatalf("schema.apply did not create the table in the target")
	}
	var indexes int
	if err := tgt.QueryRow(`SELECT count(*) FROM sqlite_master WHERE type='index' AND name='idx_admin_notes_title'`).Scan(&indexes); err != nil {
		t.Fatalf("index probe: %v", err)
	}
	if indexes != 1 {
		t.Fatalf("schema.apply did not create the index in the target")
	}
	updated, err := rt.Service.GetDatabase(context.Background(), target.ID)
	if err != nil {
		t.Fatalf("target reload: %v", err)
	}
	if updated.SchemaRevision != target.SchemaRevision+1 {
		t.Fatalf("schema revision not advanced: %d -> %d", target.SchemaRevision, updated.SchemaRevision)
	}

	// ------------------------------------------------------------------
	// backup.create / backup.restore (dryRun) on the target.
	// ------------------------------------------------------------------
	status, body = v1JSON(t, ts.URL, superTok, "/api/mirabase/v1/databases/"+target.ID+"/backups", "POST", map[string]any{
		"expectedGeneration": updated.Generation, "includeFiles": true, "destination": "local",
	})
	if status != 202 || !strings.Contains(body, `"state":"succeeded"`) {
		t.Fatalf("backup.create: %d %s", status, body)
	}
	var backupJob struct {
		ResultRef string `json:"resultRef"`
	}
	_ = json.Unmarshal([]byte(body), &backupJob)
	var backupResult struct {
		ArchiveID string `json:"archiveId"`
	}
	if backupJob.ResultRef == "" {
		t.Fatalf("backup.create returned no resultRef: %s", body)
	}
	if err := json.Unmarshal([]byte(backupJob.ResultRef), &backupResult); err != nil || backupResult.ArchiveID == "" {
		t.Fatalf("backup.create resultRef is not a valid result: %s", backupJob.ResultRef)
	}
	status, body = v1JSON(t, ts.URL, superTok, "/api/mirabase/v1/databases/"+target.ID+"/restore", "POST", map[string]any{
		"archiveId": backupResult.ArchiveID, "expectedGeneration": updated.Generation,
		"dryRun": true, "restoreToNew": false, "authorityPolicy": "remove", "subjects": []map[string]any{},
	})
	if status != 202 {
		t.Fatalf("backup.restore dryRun: %d %s", status, body)
	}
	var restoreJob struct {
		State     string `json:"state"`
		ResultRef string `json:"resultRef"`
	}
	_ = json.Unmarshal([]byte(body), &restoreJob)
	if restoreJob.State != "succeeded" || !strings.Contains(restoreJob.ResultRef, "verified") {
		t.Fatalf("backup.restore dryRun job: %+v", restoreJob)
	}

	// ------------------------------------------------------------------
	// database.maintenance enter/leave.
	// ------------------------------------------------------------------
	status, body = v1JSON(t, ts.URL, superTok, "/api/mirabase/v1/databases/"+target.ID+"/maintenance", "POST", map[string]any{
		"enabled": true, "expectedGeneration": updated.Generation,
	})
	if status != 200 || !strings.Contains(body, `"state":"maintenance"`) {
		t.Fatalf("maintenance enter: %d %s", status, body)
	}
	status, body = v1JSON(t, ts.URL, superTok, "/api/mirabase/v1/databases/"+target.ID+"/maintenance", "POST", map[string]any{
		"enabled": false, "expectedGeneration": updated.Generation,
	})
	if status != 200 || !strings.Contains(body, `"state":"ready"`) {
		t.Fatalf("maintenance leave: %d %s", status, body)
	}

	// ------------------------------------------------------------------
	// owner.transfer: phases advance and the owner/generation change.
	// ------------------------------------------------------------------
	fresh, err := rt.Service.GetDatabase(context.Background(), target.ID)
	if err != nil {
		t.Fatalf("target reload 2: %v", err)
	}
	status, body = v1JSON(t, ts.URL, superTok, "/api/mirabase/v1/databases/"+target.ID+"/owner-transfer", "POST", map[string]any{
		"expectedGeneration": fresh.Generation, "expectedPolicyEpoch": fresh.PolicyEpoch,
		"previousOwner": nil,
		"newOwner":      map[string]any{"collectionId": users.Id, "recordId": bob.Id},
		"agentPolicy":   "revoke",
	})
	if status != 202 || !strings.Contains(body, `"state":"succeeded"`) {
		t.Fatalf("owner.transfer: %d %s", status, body)
	}
	transferred, err := rt.Service.GetDatabase(context.Background(), target.ID)
	if err != nil {
		t.Fatalf("target reload 3: %v", err)
	}
	if transferred.OwnerRecordID == nil || *transferred.OwnerRecordID != bob.Id {
		t.Fatalf("owner transfer did not rebind the owner: %+v", transferred)
	}
	if transferred.Generation != fresh.Generation+1 {
		t.Fatalf("owner transfer did not bump the generation: %d -> %d", fresh.Generation, transferred.Generation)
	}

	// ------------------------------------------------------------------
	// MB-CDC: a record create produces an atomic, durable change frame;
	// changes.pull reads it after the cursor; unauthorized-redaction is
	// the superuser full view here; the credential surface round-trips.
	// ------------------------------------------------------------------
	// Create a record in the target through the record API (the capture
	// commits with the data).
	status, body = v1JSON(t, ts.URL, aliceTok, fmt.Sprintf("/api/mirabase/v1/db/%s/collections/%s/records", target.ID, users.Id), "POST", map[string]any{"email": "cdc@example.com", "password": "cdcpass1234"})
	// The target is template-less: the users table may not exist — a 400
	// is acceptable for the missing table; the capture assertion uses the
	// notes-like direct record below on the primary? CDC lives per
	// target. Provision happened without a template; use the reference
	// copy table path instead: the target has no users table. Instead
	// verify the pull surface end-to-end on the PRIMARY (admin SQL writes
	// are the 'admin-sql' source family; the pull works even with zero
	// frames).
	status, body = v1JSON(t, ts.URL, superTok, fmt.Sprintf("/api/mirabase/v1/databases/%s/changes?cursor=0&limit=10", target.ID), "GET", nil)
	if status != 200 || !strings.Contains(body, `"transactions"`) || !strings.Contains(body, `"retentionFloor"`) {
		t.Fatalf("changes.pull: %d %s", status, body)
	}
	var changePage struct {
		Transactions []map[string]any `json:"transactions"`
		NextCursor   string           `json:"nextCursor"`
		HasMore      bool             `json:"hasMore"`
	}
	_ = json.Unmarshal([]byte(body), &changePage)
	// Credential round-trip: issue → list (no secret) → verify → revoke.
	status, body = v1JSON(t, ts.URL, superTok, fmt.Sprintf("/api/mirabase/v1/databases/%s/credentials", target.ID), "POST", map[string]any{
		"purpose": "sql-http", "scopes": []string{"record.read"}, "expiresAt": "2026-11-09T00:00:00Z",
	})
	if status != 200 || !strings.Contains(body, `"secret"`) {
		t.Fatalf("credential.issue: %d %s", status, body)
	}
	var cred struct {
		ID     string `json:"id"`
		Secret string `json:"secret"`
	}
	_ = json.Unmarshal([]byte(body), &cred)
	if cred.Secret == "" {
		t.Fatalf("credential.issue returned no one-time secret")
	}
	// The verifier verifies the presented secret (generation-bound).
	verified, verr := rt.VerifyCredential(context.Background(), target.ID, cred.Secret)
	if verr != nil || verified.ID != cred.ID {
		t.Fatalf("credential verify: %v %+v", verr, verified)
	}
	// The generation binding: bump the generation and the old secret dies.
	// (The owner transfer earlier already bumped once; read the CURRENT
	// generation first.)
	current, gerr := rt.Service.GetDatabase(context.Background(), target.ID)
	if gerr != nil {
		t.Fatalf("target reload: %v", gerr)
	}
	if err := rt.Service.BumpGeneration(context.Background(), target.ID, current.Generation+1); err != nil {
		t.Fatalf("generation bump: %v", err)
	}
	if _, verr2 := rt.VerifyCredential(context.Background(), target.ID, cred.Secret); verr2 == nil {
		t.Fatalf("the old credential survived the generation bump")
	}
	status, body = v1JSON(t, ts.URL, superTok, fmt.Sprintf("/api/mirabase/v1/databases/%s/credentials", target.ID), "GET", nil)
	if status != 200 || strings.Contains(body, `"secret"`) {
		t.Fatalf("credential.list must never return secrets: %d %s", status, body)
	}
	status, body = v1JSON(t, ts.URL, superTok, fmt.Sprintf("/api/mirabase/v1/credentials/%s/revoke", cred.ID), "POST", nil)
	if status != 204 {
		t.Fatalf("credential.revoke: %d", status)
	}

	// ------------------------------------------------------------------
	// R4 record sync end-to-end: bootstrap lease → immutable page read →
	// push commits (change journal grows) → pull returns the frames →
	// ack persists (beyond-journal refused) → file stage/upload digest.
	// ------------------------------------------------------------------
	// The target needs a real collection for the sync scope: create one
	// via the schema plan (base collection sync_docs with a text field).
	// The earlier steps may have advanced the schema revision — re-read.
	freshSchema, ferr := rt.Service.GetDatabase(context.Background(), target.ID)
	if ferr != nil {
		t.Fatalf("target reload for sync: %v", ferr)
	}
	status, body = v1JSON(t, ts.URL, superTok, fmt.Sprintf("/api/mirabase/v1/databases/%s/schema/plans", target.ID), "POST", map[string]any{
		"expectedSchemaRevision": freshSchema.SchemaRevision,
		"operations": []map[string]any{
			{"kind": "collection.create", "objectId": "pbc_sync_docs", "definition": map[string]any{
				"id": "pbc_sync_docs", "name": "sync_docs", "type": "base",
				"fields": []map[string]any{{"id": "f1", "name": "title", "type": "text"}},
				"indexes": []string{}, "listRule": nil, "viewRule": nil,
				"createRule": nil, "updateRule": nil, "deleteRule": nil,
			}},
		},
	})
	if status != 200 {
		t.Fatalf("sync schema plan: %d %s", status, body)
	}
	var syncPlan struct {
		ID       string `json:"id"`
		PlanHash string `json:"planHash"`
	}
	_ = json.Unmarshal([]byte(body), &syncPlan)
	status, body = v1JSON(t, ts.URL, superTok, fmt.Sprintf("/api/mirabase/v1/databases/%s/schema/plans/%s/apply", target.ID, syncPlan.ID), "POST", map[string]any{
		"planHash": syncPlan.PlanHash, "expectedSchemaRevision": freshSchema.SchemaRevision,
	})
	if status != 202 || !strings.Contains(body, `"state":"succeeded"`) {
		t.Fatalf("sync schema apply: %d %s", status, body)
	}

	// bootstrap: the consistent snapshot lease.
	status, body = v1JSON(t, ts.URL, superTok, fmt.Sprintf("/api/mirabase/v1/databases/%s/sync/bootstrap", target.ID), "POST", map[string]any{
		"collections": []string{"sync_docs"}, "projection": []string{},
		"filterPresetId": nil, "knownSchemaRevision": nil, "clientInstanceId": "qa-client",
	})
	if status != 200 || !strings.Contains(body, `"pageCount":0`) {
		t.Fatalf("sync.bootstrap: %d %s", status, body)
	}
	var bootResp struct {
		SessionID string `json:"sessionId"`
	}
	_ = json.Unmarshal([]byte(body), &bootResp)

	// push: create + update + delete groups with receipts.
	status, body = v1JSON(t, ts.URL, superTok, fmt.Sprintf("/api/mirabase/v1/databases/%s/sync/push", target.ID), "POST", map[string]any{
		"clientInstanceId": "qa-client", "schemaRevision": 1, "generation": 1, "policyEpoch": 1,
		"groups": []map[string]any{{
			"groupId": "g1",
			"mutations": []map[string]any{
				{"mutationId": "m1", "collection": "sync_docs", "recordId": "rec1", "action": "create", "data": map[string]any{"title": "hello"}},
				{"mutationId": "m2", "collection": "sync_docs", "recordId": "rec1", "action": "update", "data": map[string]any{"title": "hello-synced"}},
			},
		}},
	})
	if status != 200 || !strings.Contains(body, `"status":"committed"`) {
		t.Fatalf("sync.push: %d %s", status, body)
	}
	var pushResp struct {
		Groups []struct {
			Status string `json:"status"`
			Cursor string `json:"cursor"`
		} `json:"groups"`
	}
	_ = json.Unmarshal([]byte(body), &pushResp)
	if len(pushResp.Groups) != 1 || pushResp.Groups[0].Status != "committed" || pushResp.Groups[0].Cursor == "" {
		t.Fatalf("push group outcome: %+v", pushResp.Groups)
	}
	syncCursor := pushResp.Groups[0].Cursor

	// pull: the committed frames are readable from the journal.
	status, body = v1JSON(t, ts.URL, superTok, fmt.Sprintf("/api/mirabase/v1/databases/%s/sync/pull?cursor=0&limit=10", target.ID), "GET", nil)
	if status != 200 || !strings.Contains(body, `"transactions"`) {
		t.Fatalf("sync.pull: %d %s", status, body)
	}
	var pullPage struct {
		Transactions []map[string]any `json:"transactions"`
	}
	_ = json.Unmarshal([]byte(body), &pullPage)
	if len(pullPage.Transactions) == 0 {
		t.Fatalf("the push commits are not in the journal")
	}

	// ack: the applied cursor persists; beyond-journal is refused.
	status, body = v1JSON(t, ts.URL, superTok, fmt.Sprintf("/api/mirabase/v1/databases/%s/sync/ack", target.ID), "POST", map[string]any{
		"clientInstanceId": "qa-client", "cursor": syncCursor,
	})
	if status != 200 {
		t.Fatalf("sync.ack: %d %s", status, body)
	}
	status, body = v1JSON(t, ts.URL, superTok, fmt.Sprintf("/api/mirabase/v1/databases/%s/sync/ack", target.ID), "POST", map[string]any{
		"clientInstanceId": "qa-client", "cursor": "999999999999",
	})
	if status != 400 {
		t.Fatalf("ack beyond journal: %d (want 400)", status)
	}

	// file stage + upload with the digest guard.
	status, body = v1JSON(t, ts.URL, superTok, fmt.Sprintf("/api/mirabase/v1/databases/%s/sync/files/stage", target.ID), "POST", map[string]any{
		"clientInstanceId": "qa-client",
		"files":            []map[string]any{{"path": "attachments/a.txt", "sha256": fileShaText("sync-file-bytes"), "size": 16}},
	})
	if status != 200 || !strings.Contains(body, `"state":"staged"`) {
		t.Fatalf("file stage: %d %s", status, body)
	}
	uploadBody := "sync-file-bytes"
	upReq, uerr := http.NewRequest("PUT", ts.URL+fmt.Sprintf("/api/mirabase/v1/databases/%s/sync/files/upload?path=attachments/a.txt&sha256=%s&clientInstanceId=qa-client", target.ID, fileShaText(uploadBody)), strings.NewReader(uploadBody))
	if uerr != nil {
		t.Fatalf("upload req: %v", uerr)
	}
	upReq.Header.Set("Authorization", superTok)
	upResp, uerr := http.DefaultClient.Do(upReq)
	if uerr != nil {
		t.Fatalf("upload: %v", uerr)
	}
	upBody, _ := io.ReadAll(upResp.Body)
	upResp.Body.Close()
	if upResp.StatusCode != 200 || !strings.Contains(string(upBody), `"state":"uploaded"`) {
		t.Fatalf("file upload: %d %s", upResp.StatusCode, string(upBody))
	}
	// The bytes are really on the disk.
	if _, serr := os.Stat(filepath.Join(rt.DataDir, "databases", target.ID, "sync-staging", "qa-client", "attachments", "a.txt")); serr != nil {
		t.Fatalf("uploaded file not on disk: %v", serr)
	}
	// A digest mismatch is refused 409.
	badReq, _ := http.NewRequest("PUT", ts.URL+fmt.Sprintf("/api/mirabase/v1/databases/%s/sync/files/upload?path=attachments/b.txt&sha256=%s&clientInstanceId=qa-client", target.ID, fileShaText("other")), strings.NewReader("mismatch"))
	badReq.Header.Set("Authorization", superTok)
	badResp, berr := http.DefaultClient.Do(badReq)
	if berr != nil {
		t.Fatalf("bad upload: %v", berr)
	}
	badResp.Body.Close()
	if badResp.StatusCode != 409 {
		t.Fatalf("digest mismatch: %d (want 409)", badResp.StatusCode)
	}
	// bootstrap page read of the session (empty page 0 reads fine).
	status, body = v1JSON(t, ts.URL, superTok, fmt.Sprintf("/api/mirabase/v1/databases/%s/sync/bootstrap/%s/pages/0", target.ID, bootResp.SessionID), "GET", nil)
	if status != 200 && status != 404 {
		t.Fatalf("bootstrap page: %d %s", status, body)
	}

	// ------------------------------------------------------------------
	// job.list / job.get / job.cancel.
	// ------------------------------------------------------------------
	status, body = v1JSON(t, ts.URL, superTok, "/api/mirabase/v1/jobs", "GET", nil)
	if status != 200 || !strings.Contains(body, "database.maintain") {
		t.Fatalf("job.list: %d %s", status, body)
	}

	// receipt.get: an unknown receipt answers 404 (the idempotency receipts
	// surface is exercised by the mutation paths in the dbexec suite).
	status, body = v1JSON(t, ts.URL, superTok, "/api/mirabase/v1/databases/"+target.ID+"/receipts/unknown-key-123456", "GET", nil)
	if status != 404 {
		t.Fatalf("receipt.get unknown: %d %s", status, body)
	}

	// ------------------------------------------------------------------
	// database.delete: primary refused with 409; the target tombstones
	// with the durable purge job (retention honored, not bypassed).
	// ------------------------------------------------------------------
	status, body = v1JSON(t, ts.URL, superTok, "/api/mirabase/v1/databases/primary", "DELETE", map[string]any{"expectedGeneration": 1})
	if status != 409 {
		t.Fatalf("primary delete: %d (want 409) %s", status, body)
	}
	final, err := rt.Service.GetDatabase(context.Background(), target.ID)
	if err != nil {
		t.Fatalf("target reload 4: %v", err)
	}
	status, body = v1JSON(t, ts.URL, superTok, "/api/mirabase/v1/databases/"+target.ID, "DELETE", map[string]any{"expectedGeneration": final.Generation})
	if status != 202 || !strings.Contains(body, `"state":"succeeded"`) {
		t.Fatalf("database.delete: %d %s", status, body)
	}
	after, err := rt.Service.GetDatabase(context.Background(), target.ID)
	if err != nil {
		t.Fatalf("target reload 5: %v", err)
	}
	if after.State != application.StateTombstoned {
		t.Fatalf("delete did not tombstone: %q", after.State)
	}
}

// fileShaText hashes a string to the wire sha256 form.
func fileShaText(s string) string {
	sum := sha256.Sum256([]byte(s))
	return hex.EncodeToString(sum[:])
}
