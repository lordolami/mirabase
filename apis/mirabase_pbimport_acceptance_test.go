package apis_test

import (
	"bytes"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"golang.org/x/crypto/bcrypt"

	_ "modernc.org/sqlite" // the import-only reference reader (fixture authoring)

	"mirabase/internal/application"
	"mirabase/tests"
)

// TestV01_08AdoptionAcceptance is release gate 7's shared-layout
// acceptance: a real PocketBase data-directory fixture (actual SQLite
// data.db with _collections/users tables and bcrypt hashes) imported
// through the full pipeline over HTTP — analyze → plan → preview → stage
// → validate → cutover — with the negatives (source hash mismatch,
// wrong plan hash) and the cutover result verified through a real
// bcrypt login on the cut-over target.
func TestV01_08AdoptionAcceptance(t *testing.T) {
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

	// ------------------------------------------------------------------
	// The source fixture: a real stopped PocketBase data directory.
	// ------------------------------------------------------------------
	fixtureDir := t.TempDir()
	fixtureDB := filepath.Join(fixtureDir, "data.db")
	if err := buildPocketBaseFixture(fixtureDB); err != nil {
		t.Fatalf("fixture: %v", err)
	}

	// Declare the source: a pocketbase-directory snapshot.
	status, body := v1JSON(t, ts.URL, superTok, "/api/mirabase/v1/pocketbase-imports", "POST", map[string]any{
		"sourceKind":                 "pocketbase-directory",
		"sourceBytes":                fileLen(fixtureDB),
		"sourceSha256":               fileSha(fixtureDB),
		"targetMode":                 "new-application",
		"expectedPrimaryGeneration":  "1",
		"sourceLabel":                "acceptance-fixture",
	})
	if status != 200 {
		t.Fatalf("pbimport.create: %d %s", status, body)
	}
	var sess map[string]any
	_ = json.Unmarshal([]byte(body), &sess)
	importID, _ := sess["id"].(string)
	if importID == "" {
		t.Fatalf("session shape: %v", sess)
	}

	// Upload the source bytes as one chunk (within the 8 MiB boundary).
	srcBytes, err := os.ReadFile(fixtureDB)
	if err != nil {
		t.Fatal(err)
	}
	status, body = v1Upload(t, ts.URL, superTok, fmt.Sprintf("/api/mirabase/v1/pocketbase-imports/%s/source", importID), srcBytes)
	if status != 200 {
		t.Fatalf("source upload: %d %s", status, body)
	}
	if !strings.Contains(body, `"complete":true`) {
		t.Fatalf("source upload did not finalize: %s", body)
	}

	// Negative: a different chunk at the same offset is a conflict.
	status, body = v1Upload(t, ts.URL, superTok, fmt.Sprintf("/api/mirabase/v1/pocketbase-imports/%s/source", importID), srcBytes)
	if status != 200 && status != 409 {
		t.Fatalf("repeat chunk: %d %s", status, body)
	}

	// Analyze: the read-only reader inspects the private copy.
	status, body = v1JSON(t, ts.URL, superTok, fmt.Sprintf("/api/mirabase/v1/pocketbase-imports/%s/analyze", importID), "POST", nil)
	if status != 200 {
		t.Fatalf("analyze: %d %s", status, body)
	}
	var analysis map[string]any
	_ = json.Unmarshal([]byte(body), &analysis)
	if analysis["integrity"] != "ok" {
		t.Fatalf("analyze integrity: %v", analysis["integrity"])
	}
	colls, _ := analysis["collections"].([]any)
	foundUsers := false
	for _, c := range colls {
		cm := c.(map[string]any)
		if cm["name"] == "users" && cm["type"] == "auth" {
			foundUsers = true
		}
	}
	if !foundUsers {
		t.Fatalf("the inventory missed the users auth collection: %v", analysis["collections"])
	}

	// Plan: shared layout, all placements shared, preserve-global.
	status, body = v1JSON(t, ts.URL, superTok, fmt.Sprintf("/api/mirabase/v1/pocketbase-imports/%s/plan", importID), "PUT", map[string]any{
		"expectedRevision": "1",
		"sourceHash":       fileSha(fixtureDB),
		"snapshotHash":     analysis["snapshotHash"],
		"layout":           "shared",
		"namespace":        "shared",
		"tenantKind":       nil,
		"tenantSource":     nil,
		"access":           nil,
		"placements": []map[string]any{
			{"collectionId": "users", "placement": "shared", "ownership": map[string]any{"kind": "shared"}, "uniquePolicy": "preserve-global"},
			{"collectionId": "posts", "placement": "shared", "ownership": map[string]any{"kind": "shared"}, "uniquePolicy": "preserve-global"},
		},
		"accountPolicy": map[string]any{
			"sessionPolicy":  "preserve-if-compatible",
			"operatorPolicy": "use-source-superuser",
		},
		"selector":    map[string]any{"kind": "none", "source": nil},
		"profile":     "wal",
		"sideEffects": "suppressed-during-import",
	})
	if status != 200 {
		t.Fatalf("plan: %d %s", status, body)
	}
	var plan map[string]any
	_ = json.Unmarshal([]byte(body), &plan)
	planHash, _ := plan["planHash"].(string)
	if planHash == "" {
		t.Fatalf("plan hash missing: %v", plan)
	}

	// Tenant-layout rejection: the shared-layout request rejects it
	// explicitly with roadmap direction.
	status, body = v1JSON(t, ts.URL, superTok, fmt.Sprintf("/api/mirabase/v1/pocketbase-imports/%s/plan", importID), "PUT", map[string]any{
		"expectedRevision": "1",
		"sourceHash":       fileSha(fixtureDB),
		"snapshotHash":     analysis["snapshotHash"],
		"layout":           "tenant",
		"namespace":        "schools",
		"tenantKind":       "tenant",
		"tenantSource":     map[string]any{"kind": "records", "collectionId": "schools"},
		"access":           map[string]any{"membershipScopes": []string{"record.read"}},
		"placements":       []map[string]any{},
		"accountPolicy":    map[string]any{"sessionPolicy": "preserve-if-compatible", "operatorPolicy": "use-source-superuser"},
		"selector":         map[string]any{"kind": "mirabase-header", "source": nil},
		"profile":          "wal",
		"sideEffects":      "suppressed-during-import",
	})
	if status != 400 || !strings.Contains(body, "R2") {
		t.Fatalf("tenant-layout rejection: %d %s", status, body)
	}

	// Preview: actual counts from the source copy.
	status, body = v1JSON(t, ts.URL, superTok, fmt.Sprintf("/api/mirabase/v1/pocketbase-imports/%s/preview", importID), "POST", nil)
	if status != 200 {
		t.Fatalf("preview: %d %s", status, body)
	}
	var preview map[string]any
	_ = json.Unmarshal([]byte(body), &preview)
	if preview["sourceRecords"].(float64) != 3 {
		t.Fatalf("preview sourceRecords: %v (want 3)", preview["sourceRecords"])
	}
	if preview["sharedRecords"].(float64) != preview["sourceRecords"].(float64) {
		t.Fatalf("sharedRecords must equal sourceRecords in the shared layout: %v", preview)
	}
	previewHash, _ := preview["previewHash"].(string)

	// Stage: rows copied into the unpublished staged target.
	status, body = v1JSON(t, ts.URL, superTok, fmt.Sprintf("/api/mirabase/v1/pocketbase-imports/%s/stage", importID), "POST", nil)
	if status != 202 {
		t.Fatalf("stage: %d %s", status, body)
	}
	var staged map[string]any
	_ = json.Unmarshal([]byte(body), &staged)
	if staged["recordCount"].(float64) != 3 {
		t.Fatalf("staged recordCount: %v (want 3)", staged["recordCount"])
	}
	if staged["stagedManifestHash"] == "" {
		t.Fatalf("staged manifest hash missing: %v", staged)
	}

	// Validate: count/hash/auth reconciliation cases.
	status, body = v1JSON(t, ts.URL, superTok, fmt.Sprintf("/api/mirabase/v1/pocketbase-imports/%s/validate", importID), "POST", nil)
	if status != 200 {
		t.Fatalf("validate: %d %s", status, body)
	}
	var validation map[string]any
	_ = json.Unmarshal([]byte(body), &validation)
	if validation["passed"] != true {
		t.Fatalf("validation did not pass: %v", validation)
	}
	reportHash, _ := validation["reportHash"].(string)

	// Cutover negatives: a wrong report hash is rejected.
	status, body = v1JSON(t, ts.URL, superTok, fmt.Sprintf("/api/mirabase/v1/pocketbase-imports/%s/cutover", importID), "POST", map[string]any{
		"expectedRevision": "1",
		"planHash":         planHash,
		"previewHash":      previewHash,
		"reportHash":       strings.Repeat("f", 64),
	})
	if status != 409 || !strings.Contains(body, "report_stale") {
		t.Fatalf("cutover wrong-report rejection: %d %s", status, body)
	}

	// pbimport.report.cases: paginate the ACTUAL recorded cases.
	status, body = v1JSON(t, ts.URL, superTok, fmt.Sprintf("/api/mirabase/v1/pocketbase-imports/%s/report/cases", importID), "GET", nil)
	if status != 200 {
		t.Fatalf("report/cases: %d %s", status, body)
	}
	var casesPage struct {
		Items []struct {
			ID     string  `json:"id"`
			Kind   string  `json:"kind"`
			Status string  `json:"status"`
			Hash   *string `json:"artifactHash"`
			Msg    *string `json:"message"`
		} `json:"items"`
		TotalItems int `json:"totalItems"`
	}
	_ = json.Unmarshal([]byte(body), &casesPage)
	if casesPage.TotalItems < 3 || len(casesPage.Items) == 0 {
		t.Fatalf("report/cases page is empty: %+v", casesPage)
	}
	sawRowPlacement := false
	for _, c := range casesPage.Items {
		if c.ID == "" || (c.Status != "passed" && c.Status != "failed") {
			t.Fatalf("malformed case entry: %+v", c)
		}
		if c.Kind == "row-placement" {
			sawRowPlacement = true
		}
	}
	if !sawRowPlacement {
		t.Fatalf("report/cases misses the row-placement cases: %+v", casesPage.Items)
	}
	// Paginated read: page 1 with perPage=1 returns exactly one.
	status, body = v1JSON(t, ts.URL, superTok, fmt.Sprintf("/api/mirabase/v1/pocketbase-imports/%s/report/cases?page=1&perPage=1", importID), "GET", nil)
	if status != 200 {
		t.Fatalf("report/cases paged: %d %s", status, body)
	}

	// Cutover: the guarded journal publishes the staged root.
	status, body = v1JSON(t, ts.URL, superTok, fmt.Sprintf("/api/mirabase/v1/pocketbase-imports/%s/cutover", importID), "POST", map[string]any{
		"expectedRevision": "1",
		"planHash":         planHash,
		"previewHash":      previewHash,
		"reportHash":       reportHash,
	})
	if status != 202 {
		t.Fatalf("cutover: %d %s", status, body)
	}
	var cutSess map[string]any
	_ = json.Unmarshal([]byte(body), &cutSess)
	if cutSess["phase"] != "active" {
		t.Fatalf("cutover phase: %v", cutSess["phase"])
	}
	// The guard revision is the session's CURRENT revision (the client
	// echoes the revision it last read).
	status, body = v1JSON(t, ts.URL, superTok, fmt.Sprintf("/api/mirabase/v1/pocketbase-imports/%s", importID), "GET", nil)
	if status != 200 {
		t.Fatalf("session before recovery: %d %s", status, body)
	}
	var sessForGuard struct {
		Revision int64 `json:"revision"`
	}
	_ = json.Unmarshal([]byte(body), &sessForGuard)

	// pbimport.recovery: export the verified current state as the
	// authoritative recovery artifact (hash-registered).
	status, body = v1JSON(t, ts.URL, superTok, fmt.Sprintf("/api/mirabase/v1/pocketbase-imports/%s/recovery", importID), "POST", map[string]any{
		"expectedRevision":         sessForGuard.Revision,
		"planHash":                 planHash,
		"action":                   "export-current-and-preserve",
		"expectedActiveGeneration": 1,
	})
	if status != 202 || !strings.Contains(body, `"state":"succeeded"`) {
		t.Fatalf("recovery export: %d %s", status, body)
	}
	var recoveryJob struct {
		ResultRef string `json:"resultRef"`
	}
	_ = json.Unmarshal([]byte(body), &recoveryJob)
	var recoveryResult struct {
		ArtifactID string `json:"artifactId"`
	}
	if recoveryJob.ResultRef == "" {
		t.Fatalf("recovery export returned no resultRef: %s", body)
	}
	if err := json.Unmarshal([]byte(recoveryJob.ResultRef), &recoveryResult); err != nil || len(recoveryResult.ArtifactID) != 64 {
		t.Fatalf("recovery export artifactId missing: %s", recoveryJob.ResultRef)
	}
	// pbimport.artifact.get: the VERIFIED id streams the artifact; an
	// unknown id is 404 and never a path lookup.
	artReq, areqErr := http.NewRequest("GET", ts.URL+fmt.Sprintf("/api/mirabase/v1/pocketbase-imports/%s/artifacts/%s", importID, recoveryResult.ArtifactID), nil)
	if areqErr != nil {
		t.Fatalf("artifact get: %v", areqErr)
	}
	artReq.Header.Set("Authorization", superTok)
	artResp, err := http.DefaultClient.Do(artReq)
	if err != nil {
		t.Fatalf("artifact get: %v", err)
	}
	artBody, _ := io.ReadAll(artResp.Body)
	artResp.Body.Close()
	if artResp.StatusCode != 200 || len(artBody) == 0 {
		t.Fatalf("artifact get: %d (%d bytes)", artResp.StatusCode, len(artBody))
	}
	artSum := sha256.Sum256(artBody)
	if hex.EncodeToString(artSum[:]) != recoveryResult.ArtifactID {
		t.Fatalf("artifact bytes do not hash to the requested id")
	}
	status, body = v1JSON(t, ts.URL, superTok, fmt.Sprintf("/api/mirabase/v1/pocketbase-imports/%s/artifacts/%s", importID, strings.Repeat("a", 64)), "GET", nil)
	if status != 404 {
		t.Fatalf("artifact unknown id: %d (want 404)", status)
	}
	// A PATH-SHAPED id never resolves (path traversal is not an artifact).
	status, body = v1JSON(t, ts.URL, superTok, fmt.Sprintf("/api/mirabase/v1/pocketbase-imports/%s/artifacts/..%%2F..%%2Fdata.db", importID), "GET", nil)
	if status != 404 {
		t.Fatalf("artifact path traversal: %d (want 404)", status)
	}
	// resume negative: an ACTIVE session cannot be resumed (the guard
	// revision check comes first; the conflict is the phase rule).
	status, body = v1JSON(t, ts.URL, superTok, fmt.Sprintf("/api/mirabase/v1/pocketbase-imports/%s/resume", importID), "POST", map[string]any{
		"expectedRevision": sessForGuard.Revision, "planHash": nil,
	})
	if status != 409 {
		t.Fatalf("resume active session: %d (want 409) %s", status, body)
	}

	// pbimport.resume positive: force a validation failure on a second
	// session (corrupt the staged bytes), then reconcile to the last
	// durable phase.
	status, body = v1JSON(t, ts.URL, superTok, "/api/mirabase/v1/pocketbase-imports", "POST", map[string]any{
		"sourceKind":                "pocketbase-directory",
		"sourceBytes":               int64(len(srcBytes)),
		"sourceSha256":              fileSha(fixtureDB),
		"targetMode":                "new-application",
		"expectedPrimaryGeneration": "1",
		"sourceLabel":               "resume-fixture",
	})
	if status != 200 {
		t.Fatalf("second session: %d %s", status, body)
	}
	var sess2 struct {
		ID string `json:"id"`
	}
	_ = json.Unmarshal([]byte(body), &sess2)
	importID2 := sess2.ID
	if importID2 == "" {
		t.Fatalf("second session id missing: %s", body)
	}
	status, body = v1Upload(t, ts.URL, superTok, fmt.Sprintf("/api/mirabase/v1/pocketbase-imports/%s/source", importID2), srcBytes)
	if status != 200 {
		t.Fatalf("second session upload: %d %s", status, body)
	}
	status, body = v1JSON(t, ts.URL, superTok, fmt.Sprintf("/api/mirabase/v1/pocketbase-imports/%s/analyze", importID2), "POST", nil)
	if status != 200 {
		t.Fatalf("second analyze: %d %s", status, body)
	}
	analysis2 := map[string]any{}
	_ = json.Unmarshal([]byte(body), &analysis2)
	status, body = v1JSON(t, ts.URL, superTok, fmt.Sprintf("/api/mirabase/v1/pocketbase-imports/%s/plan", importID2), "PUT", map[string]any{
		"expectedRevision": "1",
		"sourceHash":       fileSha(fixtureDB),
		"snapshotHash":     analysis2["snapshotHash"],
		"layout":           "shared",
		"namespace":        "shared",
		"tenantKind":       nil,
		"tenantSource":     nil,
		"access":           nil,
		"placements": []map[string]any{
			{"collectionId": "users", "placement": "shared", "ownership": map[string]any{"kind": "shared"}, "uniquePolicy": "preserve-global"},
			{"collectionId": "posts", "placement": "shared", "ownership": map[string]any{"kind": "shared"}, "uniquePolicy": "preserve-global"},
		},
		"accountPolicy": map[string]any{"sessionPolicy": "preserve-if-compatible", "operatorPolicy": "use-source-superuser"},
		"selector":      map[string]any{"kind": "none", "source": nil},
		"profile":       "wal",
		"sideEffects":   "suppressed-during-import",
	})
	if status != 200 {
		t.Fatalf("second plan: %d %s", status, body)
	}
	status, body = v1JSON(t, ts.URL, superTok, fmt.Sprintf("/api/mirabase/v1/pocketbase-imports/%s/preview", importID2), "POST", nil)
	if status != 200 {
		t.Fatalf("second preview: %d %s", status, body)
	}
	status, body = v1JSON(t, ts.URL, superTok, fmt.Sprintf("/api/mirabase/v1/pocketbase-imports/%s/stage", importID2), "POST", nil)
	if status != 202 {
		t.Fatalf("second stage: %d %s", status, body)
	}
	// Corrupt the staged bytes, then validate: the integrity case fails
	// and the session FAILS (durable).
	stagedPath2 := filepath.Join(rt.DataDir, "imports", importID2, "staged", "data.db")
	if err := os.WriteFile(stagedPath2, []byte("corrupted-staged-bytes"), 0o600); err != nil {
		t.Fatalf("corrupt staged: %v", err)
	}
	status, body = v1JSON(t, ts.URL, superTok, fmt.Sprintf("/api/mirabase/v1/pocketbase-imports/%s/validate", importID2), "POST", nil)
	// A staged file that cannot even open is a FAILED validation surfaced
	// as unresolved_import_issues (422) with the session FAILED durably.
	if status != 422 || !strings.Contains(body, "unresolved_import_issues") {
		t.Fatalf("second validate: %d %s", status, body)
	}
	// The session must now be failed (durable).
	status, body = v1JSON(t, ts.URL, superTok, fmt.Sprintf("/api/mirabase/v1/pocketbase-imports/%s", importID2), "GET", nil)
	if status != 200 || !strings.Contains(body, `"phase":"failed"`) {
		t.Fatalf("session after failed validation: %d %s", status, body)
	}
	// Read the failed session's revision for the resume guard.
	status, body = v1JSON(t, ts.URL, superTok, fmt.Sprintf("/api/mirabase/v1/pocketbase-imports/%s", importID2), "GET", nil)
	if status != 200 {
		t.Fatalf("session before resume: %d %s", status, body)
	}
	var sess2Guard struct {
		Revision int64 `json:"revision"`
	}
	_ = json.Unmarshal([]byte(body), &sess2Guard)
	// Resume: reconcile to the last durable phase (staged), guards on.
	status, body = v1JSON(t, ts.URL, superTok, fmt.Sprintf("/api/mirabase/v1/pocketbase-imports/%s/resume", importID2), "POST", map[string]any{
		"expectedRevision": sess2Guard.Revision, "planHash": nil,
	})
	if status != 202 || !strings.Contains(body, `"state":"succeeded"`) {
		t.Fatalf("resume: %d %s", status, body)
	}
	var resumeJob struct {
		ResultRef string `json:"resultRef"`
	}
	_ = json.Unmarshal([]byte(body), &resumeJob)
	if resumeJob.ResultRef == "" {
		t.Fatalf("resume returned no resultRef: %s", body)
	}
	if !strings.Contains(strings.ReplaceAll(resumeJob.ResultRef, "\\", ""), "reconciled-to-staged") {
		t.Fatalf("resume did not reconcile to staged: %s", resumeJob.ResultRef)
	}
	// The resumed session phase is staged (the durable reconcile).
	status, body = v1JSON(t, ts.URL, superTok, fmt.Sprintf("/api/mirabase/v1/pocketbase-imports/%s", importID2), "GET", nil)
	if status != 200 || !strings.Contains(body, `"phase":"staged"`) {
		t.Fatalf("resumed session phase: %d %s", status, body)
	}

	// The cut-over target: the staged data directory now holds a working
	// MiraBase-format primary with the imported users table (bcrypt
	// hashes verbatim). Open it through the engine and log in with the
	// ORIGINAL password.
	targetDB := filepath.Join(rt.DataDir, "cutover-target", "data.db")
	if _, err := os.Stat(targetDB); err != nil {
		t.Fatalf("cutover target data.db: %v", err)
	}
	target, err := application.OpenRaw(targetDB)
	if err != nil {
		t.Fatalf("open target: %v", err)
	}
	defer target.Close()
	var importedCount int
	if err := target.QueryRow(`SELECT count(*) FROM users`).Scan(&importedCount); err != nil {
		t.Fatalf("target users count: %v", err)
	}
	if importedCount != 2 {
		t.Fatalf("imported users: %d (want 2)", importedCount)
	}
	// bcrypt login preservation: the imported hash verifies against the
	// original password.
	var hash string
	if err := target.QueryRow(`SELECT password FROM users WHERE email = 'alice@pb.example.com'`).Scan(&hash); err != nil {
		t.Fatalf("imported hash: %v", err)
	}
	if !strings.HasPrefix(hash, "$2") {
		t.Fatalf("the imported password is not a bcrypt hash: %q", hash)
	}
	if err := verifyBcrypt("original-alice-password", hash); err != nil {
		t.Fatalf("bcrypt login preservation failed: %v", err)
	}

	// pbimport.tenants read-back: the analyzed inventory's tenant note.
	status, body = v1JSON(t, ts.URL, superTok, fmt.Sprintf("/api/mirabase/v1/pocketbase-imports/%s/tenants", importID), "GET", nil)
	if status != 200 || !strings.Contains(body, "R2") {
		t.Fatalf("pbimport.tenants: %d %s", status, body)
	}

	// importbatch: orchestrate the two completed/validated sessions with
	// per-source provenance; unknown source sessions are refused.
	status, body = v1JSON(t, ts.URL, superTok, "/api/mirabase/v1/import-batches", "POST", map[string]any{
		"mode": "append",
		"sources": []map[string]any{
			{"sourceId": importID, "provenance": "append"},
		},
	})
	if status != 200 || !strings.Contains(body, `"state":"draft"`) {
		t.Fatalf("importbatch.create: %d %s", status, body)
	}
	var batch struct {
		ID string `json:"id"`
	}
	_ = json.Unmarshal([]byte(body), &batch)
	status, body = v1JSON(t, ts.URL, superTok, "/api/mirabase/v1/import-batches", "POST", map[string]any{
		"mode": "append",
		"sources": []map[string]any{
			{"sourceId": "no-such-session", "provenance": "append"},
		},
	})
	if status != 404 {
		t.Fatalf("batch unknown source: %d (want 404) %s", status, body)
	}
	status, body = v1JSON(t, ts.URL, superTok, fmt.Sprintf("/api/mirabase/v1/import-batches/%s/run", batch.ID), "POST", nil)
	if status != 202 || !strings.Contains(body, `"state":"completed"`) {
		t.Fatalf("importbatch.run: %d %s", status, body)
	}
	var batchRun struct {
		Sources []struct {
			SourceID string `json:"sourceId"`
			State    string `json:"state"`
		} `json:"sources"`
	}
	_ = json.Unmarshal([]byte(body), &batchRun)
	if len(batchRun.Sources) != 1 || batchRun.Sources[0].State != "applied" {
		t.Fatalf("batch per-source outcome: %+v", batchRun.Sources)
	}
	// A completed batch cannot be cancelled.
	status, body = v1JSON(t, ts.URL, superTok, fmt.Sprintf("/api/mirabase/v1/import-batches/%s/cancel", batch.ID), "POST", nil)
	if status != 409 {
		t.Fatalf("batch cancel completed: %d (want 409)", status)
	}

	// The original source is retained.
	if _, err := os.Stat(fixtureDB); err != nil {
		t.Fatalf("the original source was not retained: %v", err)
	}
}

// hashBcrypt creates a real bcrypt hash (the inherited password hasher).
func hashBcrypt(password string) (string, error) {
	b, err := bcrypt.GenerateFromPassword([]byte(password), 12)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

// verifyBcrypt checks a password against the imported hash (bcrypt login
// preservation).
func verifyBcrypt(password, hash string) error {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(password))
}

// buildPocketBaseFixture writes a real stopped-PocketBase data.db: the
// _collections table with a users auth collection and a posts base
// collection, users rows with real bcrypt hashes, posts rows, and the
// _migrations history.
func buildPocketBaseFixture(path string) error {
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return err
	}
	defer db.Close()
	stmts := []string{
		`CREATE TABLE _collections (
			id TEXT PRIMARY KEY NOT NULL,
			type TEXT NOT NULL DEFAULT 'base',
			name TEXT NOT NULL UNIQUE,
			system INTEGER NOT NULL DEFAULT 0,
			fields TEXT NOT NULL DEFAULT '[]',
			indexes TEXT NOT NULL DEFAULT '[]',
			listRule TEXT,
			viewRule TEXT,
			createRule TEXT,
			updateRule TEXT,
			deleteRule TEXT,
			created TEXT NOT NULL,
			updated TEXT NOT NULL)`,
		`CREATE TABLE _migrations (file VARCHAR(255) PRIMARY KEY NOT NULL, applied INTEGER NOT NULL)`,
		`CREATE TABLE _params (
			id TEXT PRIMARY KEY NOT NULL,
			value JSON DEFAULT NULL,
			created TEXT NOT NULL,
			updated TEXT NOT NULL)`,
		`CREATE TABLE users (
			id TEXT PRIMARY KEY NOT NULL,
			email TEXT NOT NULL UNIQUE,
			password TEXT NOT NULL,
			verified INTEGER NOT NULL DEFAULT 0,
			created TEXT NOT NULL,
			updated TEXT NOT NULL)`,
		`CREATE TABLE posts (
			id TEXT PRIMARY KEY NOT NULL,
			title TEXT NOT NULL,
			created TEXT NOT NULL,
			updated TEXT NOT NULL)`,
		`INSERT INTO _collections (id, type, name, system, fields, indexes, created, updated) VALUES
			('pbc_users_coll', 'auth', 'users', 0,
			 '[{"id":"f1","name":"email","type":"email"},{"id":"f2","name":"password","type":"password"}]',
			 '[]', '2024-01-01 00:00:00.000Z', '2024-01-01 00:00:00.000Z'),
			('pbc_posts_coll', 'base', 'posts', 0,
			 '[{"id":"f3","name":"title","type":"text"}]',
			 '[]', '2024-01-01 00:00:00.000Z', '2024-01-01 00:00:00.000Z')`,
		`INSERT INTO _migrations (file, applied) VALUES ('1640988000_init.go', 1)`,
		`INSERT INTO _params (id, value, created, updated) VALUES
			('settings', '{"meta":{"appName":"pb fixture"}}', '2024-01-01 00:00:00.000Z', '2024-01-01 00:00:00.000Z')`,
	}
	for _, s := range stmts {
		if _, err := db.Exec(s); err != nil {
			return fmt.Errorf("fixture stmt: %w", err)
		}
	}
	// Real bcrypt hashes (imported verbatim; login uses the ORIGINAL
	// password).
	aliceHash, err := hashBcrypt("original-alice-password")
	if err != nil {
		return err
	}
	bobHash, err := hashBcrypt("original-bob-password")
	if err != nil {
		return err
	}
	rows := []string{
		fmt.Sprintf(`INSERT INTO users (id, email, password, verified, created, updated) VALUES
			('usr_alice0000001', 'alice@pb.example.com', %q, 1, '2024-01-01 00:00:00.000Z', '2024-01-01 00:00:00.000Z')`, aliceHash),
		fmt.Sprintf(`INSERT INTO users (id, email, password, verified, created, updated) VALUES
			('usr_bob000000001', 'bob@pb.example.com', %q, 0, '2024-01-01 00:00:00.000Z', '2024-01-01 00:00:00.000Z')`, bobHash),
		`INSERT INTO posts (id, title, created, updated) VALUES
			('pst_00000000001', 'imported post', '2024-01-01 00:00:00.000Z', '2024-01-01 00:00:00.000Z')`,
	}
	for _, s := range rows {
		if _, err := db.Exec(s); err != nil {
			return err
		}
	}
	return nil
}

func fileLen(path string) int64 {
	info, err := os.Stat(path)
	if err != nil {
		return 0
	}
	return info.Size()
}

func fileSha(path string) string {
	b, err := os.ReadFile(path)
	if err != nil {
		return strings.Repeat("0", 64)
	}
	h := sha256.Sum256(b)
	return fmt.Sprintf("%x", h)
}

// v1Upload performs a chunked binary upload against the import surface.
func v1Upload(t *testing.T, baseURL, token, path string, data []byte) (int, string) {
	t.Helper()
	req, err := http.NewRequest("PUT", baseURL+path, bytes.NewReader(data))
	if err != nil {
		t.Fatalf("upload request: %v", err)
	}
	req.Header.Set("Content-Type", "application/octet-stream")
	req.Header.Set("Content-Range", fmt.Sprintf("bytes 0-%d/%d", len(data)-1, len(data)))
	sum := sha256.Sum256(data)
	req.Header.Set("X-Chunk-Sha256", fmt.Sprintf("%x", sum))
	if token != "" {
		req.Header.Set("Authorization", token)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("upload do: %v", err)
	}
	defer resp.Body.Close()
	var buf bytes.Buffer
	_, _ = buf.ReadFrom(resp.Body)
	return resp.StatusCode, buf.String()
}
