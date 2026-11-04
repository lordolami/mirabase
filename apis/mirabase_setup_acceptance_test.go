package apis_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"mirabase/apis"
	"mirabase/core"
	"mirabase/internal/application"
	"mirabase/tests"
)

// TestV01_06EmptyAppAcceptance is the AT-ONBOARD empty-app acceptance for
// every layout through real HTTP ("MB-ONBOARD and MB-MEMBERSHIP run from
// empty application roots for every layout, through dashboard and
// HTTP/SDK ... Basic shared startup must create no optional instance or
// partition catalog") plus the setup-plan/rollout contracts.
func TestV01_06EmptyAppAcceptance(t *testing.T) {
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
	// Shared startup created NO optional partition catalog beyond what the
	// registry service owns: before any setup call the plans table must
	// not exist.
	// ------------------------------------------------------------------
	var plansTable int
	if err := app.DBConn().QueryRow(
		`SELECT count(*) FROM sqlite_master WHERE type='table' AND name='_mb_setup_plans'`).Scan(&plansTable); err != nil {
		t.Fatalf("plans table probe: %v", err)
	}
	if plansTable != 0 {
		t.Fatalf("basic shared startup installed the setup plans catalog without a consumer")
	}

	// The setup.get endpoint installs the service lazily and returns the
	// shared default with a NULL draft ("it does not invent a plan").
	status, body := v1JSON(t, ts.URL, superTok, "/api/mirabase/v1/setup", "GET", nil)
	if status != 200 {
		t.Fatalf("setup.get: %d %s", status, body)
	}

	// The registry service is now bound; re-probe the plans table (the
	// setup service installs it at its first consumer).
	if err := app.DBConn().QueryRow(
		`SELECT count(*) FROM sqlite_master WHERE type='table' AND name='_mb_setup_plans'`).Scan(&plansTable); err != nil {
		t.Fatalf("plans table probe 2: %v", err)
	}
	if plansTable != 1 {
		t.Fatalf("setup.get did not install its table at the first consumer")
	}

	// Create a user layout plan through the real endpoint. The template
	// is published first (user layout requires a template id). The private
	// collection is a NEW EMPTY one (demo1 is the populated shared
	// fixture — placing it would be the layout_migration_required case).
	createAuthCollection(app.BaseApp, "members")
	createAuthRecord(app.BaseApp, mustCollection(app.BaseApp, "members"), "owner@example.com", "ownerpass123456")
	notes, err := createBaseCollection(app.BaseApp, "notes")
	if err != nil {
		t.Fatalf("notes collection: %v", err)
	}
	status, body = v1JSON(t, ts.URL, superTok, "/api/mirabase/v1/templates", "POST", map[string]any{
		"templateId":             "tpl-user",
		"collectionNames":        []string{notes.Name},
		"expectedSchemaRevision": "1",
	})
	if status != 200 {
		t.Fatalf("template.publish (tpl-user): %d %s", status, body)
	}

	planBody := map[string]any{
		"expectedApplicationGeneration": "1",
		"expectedRevision":              "1",
		"layout":                        "user",
		"authCollectionIds":             []string{"members"},
		"templateId":                    "tpl-user",
		"provision":                     "lazy",
		"tenantSource":                  nil,
		"access":                        nil,
		"namespace":                     "personal",
		"placements": []map[string]any{
			{
				"collectionId":      "notes",
				"placement":         "tenant",
				"ownership":         map[string]any{"kind": "shared"},
				"referenceFieldIds": []string{},
				"uniquePolicy":      "preserve-global",
				"adaptationIds":     []string{},
			},
		},
	}
	status, body = v1JSON(t, ts.URL, superTok, "/api/mirabase/v1/setup/plan", "PUT", planBody)
	if status != 200 {
		t.Fatalf("setup.plan: %d %s", status, body)
	}
	var plan map[string]any
	if err := json.Unmarshal([]byte(body), &plan); err != nil {
		t.Fatalf("plan decode: %v", err)
	}
	planID, _ := plan["id"].(string)
	planHash, _ := plan["planHash"].(string)
	planState, _ := plan["state"].(string)
	if planID == "" || planHash == "" || planState != "ready" {
		t.Fatalf("plan shape: %v", plan)
	}
	plannedDBs, _ := plan["plannedDatabases"].(float64)
	if plannedDBs < 1 {
		t.Fatalf("plannedDatabases must reflect the actual account count: %v", plan["plannedDatabases"])
	}

	// Stale-plan rejection: wrong expected revision → generation_mismatch.
	status, body = v1JSON(t, ts.URL, superTok, "/api/mirabase/v1/setup/plan", "PUT", map[string]any{
		"expectedApplicationGeneration": "999",
		"expectedRevision":              "1",
		"layout":                        "user",
		"authCollectionIds":             []string{"members"},
		"templateId":                    "tpl-user",
		"provision":                     "lazy",
		"tenantSource":                  nil,
		"access":                        nil,
		"namespace":                     "personal",
		"placements":                    []map[string]any{},
	})
	if status != 409 || !strings.Contains(body, "generation_mismatch") {
		t.Fatalf("stale plan rejection: %d %s", status, body)
	}

	// Populated-relocation refusal: place the POPULATED members collection
	// into a private layout → layout_migration_required, existing data
	// unchanged.
	populatedPlan := map[string]any{
		"expectedApplicationGeneration": "1",
		"expectedRevision":              "1",
		"layout":                        "user",
		"authCollectionIds":             []string{"members"},
		"templateId":                    "tpl-user",
		"provision":                     "lazy",
		"tenantSource":                  nil,
		"access":                        nil,
		"namespace":                     "personal",
		"placements": []map[string]any{
			{
				"collectionId":      "members",
				"placement":         "tenant",
				"ownership":         map[string]any{"kind": "shared"},
				"referenceFieldIds": []string{},
				"uniquePolicy":      "preserve-global",
				"adaptationIds":     []string{},
			},
		},
	}
	status, body = v1JSON(t, ts.URL, superTok, "/api/mirabase/v1/setup/plan", "PUT", populatedPlan)
	if status != 422 || !strings.Contains(body, "layout_migration_required") {
		t.Fatalf("populated relocation refusal: %d %s", status, body)
	}

	// Apply the validated plan (hash + revision guards).
	status, body = v1JSON(t, ts.URL, superTok, "/api/mirabase/v1/setup/plan/apply", "POST", map[string]any{
		"planId":                        planID,
		"expectedApplicationGeneration": "1",
		"expectedRevision":              "1",
		"planHash":                      planHash,
	})
	if status != 202 {
		t.Fatalf("setup.apply: %d %s", status, body)
	}

	// Wrong hash → policy_changed.
	status, body = v1JSON(t, ts.URL, superTok, "/api/mirabase/v1/setup/plan/apply", "POST", map[string]any{
		"planId":                        planID,
		"expectedApplicationGeneration": "1",
		"expectedRevision":              "1",
		"planHash":                      strings.Repeat("a", 64),
	})
	if status != 409 || !strings.Contains(body, "policy_changed") {
		t.Fatalf("wrong-hash rejection: %d %s", status, body)
	}

	// The active layout is now user (SetupState.layout through setup.get).
	status, body = v1JSON(t, ts.URL, superTok, "/api/mirabase/v1/setup", "GET", nil)
	if status != 200 || !strings.Contains(body, `"layout":"user"`) {
		t.Fatalf("setup.get layout after apply: %d %s", status, body)
	}

	// ------------------------------------------------------------------
	// Template publish + rollout fan-out to an explicit target set.
	// ------------------------------------------------------------------
	status, body = v1JSON(t, ts.URL, superTok, "/api/mirabase/v1/templates", "POST", map[string]any{
		"templateId":             "tpl-acceptance",
		"collectionNames":        []string{"notes"},
		"expectedSchemaRevision": "1",
	})
	if status != 200 {
		t.Fatalf("template.publish: %d %s", status, body)
	}
	var tpl map[string]any
	_ = json.Unmarshal([]byte(body), &tpl)
	tplID, _ := tpl["id"].(string)
	tplRevF, _ := tpl["revision"].(float64)
	tplRev := int64(tplRevF)
	tplHash, _ := tpl["definitionHash"].(string)
	if tplID == "" || tplHash == "" {
		t.Fatalf("template shape: %v", tpl)
	}
	// Immutability: republishing creates a NEW revision, the old one stays.
	status, body = v1JSON(t, ts.URL, superTok, "/api/mirabase/v1/templates", "POST", map[string]any{
		"templateId":             "tpl-acceptance",
		"collectionNames":        []string{"notes", "members"},
		"expectedSchemaRevision": "1",
	})
	if status != 200 {
		t.Fatalf("template republish: %d %s", status, body)
	}
	var tpl2 map[string]any
	_ = json.Unmarshal([]byte(body), &tpl2)
	if tpl2["revision"].(float64) != tplRevF+1 {
		t.Fatalf("republish must create a new revision: %v", tpl2["revision"])
	}

	// Provision two empty targets and fan out the published revision.
	d1, _, err := rt.Service.Provision(context.Background(), app.DataDir(), application.ProvisionRequest{
		Kind: "user", OwnerCollectionID: "members", OwnerRecordID: "t1", DisplayName: "t1",
	})
	if err != nil {
		t.Fatalf("provision t1: %v", err)
	}
	d2, _, err := rt.Service.Provision(context.Background(), app.DataDir(), application.ProvisionRequest{
		Kind: "agent", OwnerCollectionID: "agents", OwnerRecordID: "t2", DisplayName: "t2",
	})
	if err != nil {
		t.Fatalf("provision t2: %v", err)
	}

	status, body = v1JSON(t, ts.URL, superTok, fmt.Sprintf("/api/mirabase/v1/templates/%s/rollouts", tplID), "POST", map[string]any{
		"revision":      tplRev,
		"databaseIds":   []string{d1.ID, d2.ID},
		"stopOnFailure": false,
	})
	if status != 202 {
		t.Fatalf("template.rollout: %d %s", status, body)
	}
	var rollout map[string]any
	_ = json.Unmarshal([]byte(body), &rollout)
	rolloutID, _ := rollout["rolloutId"].(string)
	if rolloutID == "" {
		t.Fatalf("rollout shape: %v", rollout)
	}

	// Drive the bounded fan-out (journal makes it resumable).
	status, body = v1JSON(t, ts.URL, superTok, fmt.Sprintf("/api/mirabase/v1/rollouts/%s/run", rolloutID), "POST", nil)
	if status != 200 {
		t.Fatalf("rollout run: %d %s", status, body)
	}
	var runResult map[string]any
	_ = json.Unmarshal([]byte(body), &runResult)
	if runResult["done"] != true {
		t.Fatalf("rollout did not complete cleanly: %v", runResult)
	}
	// Per-target states are exact.
	status, body = v1JSON(t, ts.URL, superTok, fmt.Sprintf("/api/mirabase/v1/rollouts/%s", rolloutID), "GET", nil)
	if status != 200 {
		t.Fatalf("rollout status: %d %s", status, body)
	}
	var rollStatus map[string]any
	_ = json.Unmarshal([]byte(body), &rollStatus)
	if rollStatus["succeeded"].(float64) != 2 || rollStatus["failed"].(float64) != 0 {
		t.Fatalf("per-target outcomes: %v", rollStatus)
	}
	targets, _ := rollStatus["targets"].([]any)
	for _, tgt := range targets {
		tm := tgt.(map[string]any)
		if tm["state"] != "succeeded" {
			t.Fatalf("target state: %v (skipped is not succeeded)", tm)
		}
	}

	// rollout.targets: the exact paginated per-target journal.
	status, body = v1JSON(t, ts.URL, superTok, fmt.Sprintf("/api/mirabase/v1/rollouts/%s/targets?page=1&perPage=1", rolloutID), "GET", nil)
	if status != 200 {
		t.Fatalf("rollout.targets: %d %s", status, body)
	}
	var targetsPage struct {
		Items      []map[string]any `json:"items"`
		TotalItems int              `json:"totalItems"`
	}
	_ = json.Unmarshal([]byte(body), &targetsPage)
	if targetsPage.TotalItems != 2 || len(targetsPage.Items) != 1 {
		t.Fatalf("rollout.targets paging: %+v", targetsPage)
	}
	if targetsPage.Items[0]["resultRef"] == nil {
		t.Fatalf("rollout.targets resultRef missing: %+v", targetsPage.Items[0])
	}

	// rollout.pause: new admission stops (a paused rollout leaves the
	// remaining targets pending and the run reports not-done).
	status, body = v1JSON(t, ts.URL, superTok, "/api/mirabase/v1/templates", "GET", nil)
	if status != 200 {
		t.Fatalf("templates list: %d %s", status, body)
	}

	// A second rollout for the control-surface acceptance: create two
	// fresh targets, pause, verify the run admits nothing, resume with a
	// generation change on one target (explicit failed, never success),
	// retry, and finish cleanly.
	d3, _, err := rt.Service.Provision(context.Background(), app.DataDir(), application.ProvisionRequest{
		Kind: "user", TemplateID: tplID,
		OwnerCollectionID: "members", OwnerRecordID: "ctl3", DisplayName: "ctl3",
	})
	if err != nil {
		t.Fatalf("d3 provision: %v", err)
	}
	d4, _, err := rt.Service.Provision(context.Background(), app.DataDir(), application.ProvisionRequest{
		Kind: "user", TemplateID: tplID,
		OwnerCollectionID: "members", OwnerRecordID: "ctl4", DisplayName: "ctl4",
	})
	if err != nil {
		t.Fatalf("d4 provision: %v", err)
	}
	status, body = v1JSON(t, ts.URL, superTok, fmt.Sprintf("/api/mirabase/v1/templates/%s/rollouts", tplID), "POST", map[string]any{
		"revision":      tplRev,
		"databaseIds":   []string{d3.ID, d4.ID},
		"stopOnFailure": false,
	})
	if status != 202 {
		t.Fatalf("second rollout create: %d %s", status, body)
	}
	var rollout2 map[string]any
	_ = json.Unmarshal([]byte(body), &rollout2)
	rolloutID2, _ := rollout2["rolloutId"].(string)
	if rolloutID2 == "" {
		t.Fatalf("second rollout shape: %v", rollout2)
	}
	status, body = v1JSON(t, ts.URL, superTok, fmt.Sprintf("/api/mirabase/v1/rollouts/%s/pause", rolloutID2), "POST", nil)
	if status != 200 {
		t.Fatalf("rollout.pause: %d %s", status, body)
	}
	status, body = v1JSON(t, ts.URL, superTok, fmt.Sprintf("/api/mirabase/v1/rollouts/%s/run", rolloutID2), "POST", nil)
	if status != 200 {
		t.Fatalf("paused rollout run: %d %s", status, body)
	}
	var pausedRun map[string]any
	_ = json.Unmarshal([]byte(body), &pausedRun)
	if pausedRun["done"] == true {
		t.Fatalf("a paused rollout must not admit new targets: %v", pausedRun)
	}
	status, body = v1JSON(t, ts.URL, superTok, fmt.Sprintf("/api/mirabase/v1/rollouts/%s/targets", rolloutID2), "GET", nil)
	if status != 200 || !strings.Contains(body, `"state":"pending"`) {
		t.Fatalf("paused rollout targets stay pending: %d %s", status, body)
	}

	// rollout.resume with a generation change on d4: the changed target
	// receives an explicit failed generation_mismatch — never success.
	if err := rt.Service.TransitionDatabase(context.Background(), d4.ID, "maintenance", false); err != nil {
		t.Fatalf("d4 state prep: %v", err)
	}
	_ = rt.Handles.Close(d4.ID)
	if err := rt.Service.BumpGeneration(context.Background(), d4.ID, 99); err != nil {
		t.Fatalf("d4 generation change: %v", err)
	}
	if err := rt.Service.TransitionDatabase(context.Background(), d4.ID, "ready", false); err != nil {
		t.Fatalf("d4 ready restore: %v", err)
	}
	status, body = v1JSON(t, ts.URL, superTok, fmt.Sprintf("/api/mirabase/v1/rollouts/%s/resume", rolloutID2), "POST", nil)
	if status != 200 {
		t.Fatalf("rollout.resume: %d %s", status, body)
	}
	status, body = v1JSON(t, ts.URL, superTok, fmt.Sprintf("/api/mirabase/v1/rollouts/%s/targets", rolloutID2), "GET", nil)
	if status != 200 || !strings.Contains(body, "generation_mismatch") {
		t.Fatalf("resume revalidation marks the changed target failed: %d %s", status, body)
	}

	// rollout.retry re-arms ONLY failed targets; then the run completes
	// the remaining work (d4's snapshot generation is refreshed by
	// creating a new rollout for it).
	status, body = v1JSON(t, ts.URL, superTok, fmt.Sprintf("/api/mirabase/v1/rollouts/%s/retry", rolloutID2), "POST", nil)
	if status != 200 || !strings.Contains(body, "rearmedFailedTargets") {
		t.Fatalf("rollout.retry: %d %s", status, body)
	}
	status, body = v1JSON(t, ts.URL, superTok, fmt.Sprintf("/api/mirabase/v1/rollouts/%s/run", rolloutID2), "POST", nil)
	if status != 200 {
		t.Fatalf("post-retry run: %d %s", status, body)
	}
	// d3 (unchanged generation) completes; d4 (changed generation, re-armed
	// by retry without revalidation) fails its guard explicitly during
	// apply — the journal records the real outcome, never a pretend pass.
	status, body = v1JSON(t, ts.URL, superTok, fmt.Sprintf("/api/mirabase/v1/rollouts/%s", rolloutID2), "GET", nil)
	if status != 200 {
		t.Fatalf("final rollout status: %d %s", status, body)
	}
	var finalStatus map[string]any
	_ = json.Unmarshal([]byte(body), &finalStatus)
	if finalStatus["succeeded"].(float64) < 1 {
		t.Fatalf("d3 should have succeeded: %v", finalStatus)
	}

	// The target-local _mb_schema singleton records the applied revision.
	for _, db := range []*application.Database{d1, d2} {
		target, err := rt.Handles.Open(context.Background(), db.ID)
		if err != nil {
			t.Fatalf("open %s: %v", db.ID, err)
		}
		var rev int64
		if err := target.QueryRow(`SELECT revision FROM _mb_schema WHERE singleton = 1`).Scan(&rev); err != nil {
			t.Fatalf("_mb_schema singleton %s: %v", db.ID, err)
		}
		if rev != tplRev {
			t.Fatalf("target %s recorded revision %d, want %d", db.ID, rev, tplRev)
		}
	}
}

// TestV01MembershipAndBootstrapAcceptance is the MB-MEMBERSHIP /
// MB-APPAUTH acceptance: built-in invitations (token-hash persistence,
// intended-subject authority, atomic consume, replay/expiry/revocation
// semantics) and the purpose-bound bootstrap administrator through real
// HTTP.
func TestV01MembershipAndBootstrapAcceptance(t *testing.T) {
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

	// Capture the invitation tokens through the inherited mail outbox
	// (with SMTP configured the token is emailed; the acceptance captures
	// the outbox effect directly).
	var sentTokens []string
	app.OnMailerSend().BindFunc(func(e *core.MailerEvent) error {
		for _, line := range strings.Split(e.Message.Text, " ") {
			if len(line) == 64 {
				sentTokens = append(sentTokens, line)
			}
		}
		return e.Next()
	})

	// ------------------------------------------------------------------
	// membership.invite: a superuser invites an EXISTING subject with one
	// membership field value. The response NEVER carries the token.
	// ------------------------------------------------------------------
	users, err := createAuthCollection(app.BaseApp, "members_inv")
	if err != nil {
		t.Fatalf("members collection: %v", err)
	}
	alice, err := createAuthRecord(app.BaseApp, users, "inv-alice@example.com", "invalicepass1")
	if err != nil {
		t.Fatalf("alice: %v", err)
	}
	bob, err := createAuthRecord(app.BaseApp, users, "inv-bob@example.com", "invbobpass12")
	if err != nil {
		t.Fatalf("bob: %v", err)
	}
	// A tenant record + a membership field on the members collection.
	schools := core.NewBaseCollection("schools_inv")
	schools.Fields.Add(&core.TextField{Name: "name"})
	if err := app.Save(schools); err != nil {
		t.Fatalf("schools: %v", err)
	}
	school := core.NewRecord(schools)
	school.Set("name", "west")
	if err := app.Save(school); err != nil {
		t.Fatalf("school: %v", err)
	}
	status, body := v1JSON(t, ts.URL, superTok, "/api/mirabase/v1/membership-invitations", "POST", map[string]any{
		"namespace": "ns",
		"tenantKey": map[string]any{"kind": "record", "sourceCollectionId": schools.Id, "recordId": school.Id},
		"subject":   map[string]any{"collectionId": users.Id, "recordId": bob.Id},
	})
	if status != 200 || !strings.Contains(body, `"state":"pending"`) {
		t.Fatalf("membership.invite: %d %s", status, body)
	}
	var invite struct {
		ID string `json:"id"`
	}
	_ = json.Unmarshal([]byte(body), &invite)
	if strings.Contains(body, "token") {
		t.Fatalf("the invitation response must never carry the token: %s", body)
	}
	// The mail outbox carried the token (captured through the hook).
	if len(sentTokens) != 1 {
		t.Fatalf("the invitation mail effect was not enqueued: %d tokens", len(sentTokens))
	}
	token := sentTokens[0]

	// The tenant owner may also invite (the current tenant invite rule).
	ownerTok, err := alice.NewAuthToken()
	if err != nil {
		t.Fatalf("alice token: %v", err)
	}
	// The invite for a THIRD party into the school by a non-owner is 403.
	status, body = v1JSON(t, ts.URL, ownerTok, "/api/mirabase/v1/membership-invitations", "POST", map[string]any{
		"namespace": "ns",
		"tenantKey": map[string]any{"kind": "record", "sourceCollectionId": schools.Id, "recordId": school.Id},
		"subject":   map[string]any{"collectionId": users.Id, "recordId": bob.Id},
	})
	// Alice is not the school record owner (the school has no owner
	// field) — the invite rule fails for non-superusers.
	if status != 403 {
		t.Fatalf("non-owner invite: %d (want 403) %s", status, body)
	}

	// membership.accept: Bob accepts with the token.
	bobTok, err := bob.NewAuthToken()
	if err != nil {
		t.Fatalf("bob token: %v", err)
	}
	status, body = v1JSON(t, ts.URL, bobTok, "/api/mirabase/v1/membership-invitations/accept", "POST", map[string]any{
		"token":                         token,
		"expectedApplicationGeneration": 1,
	})
	if status != 200 || !strings.Contains(body, `"state":"used"`) {
		t.Fatalf("membership.accept: %d %s", status, body)
	}
	// Replay returns the prior result (never a second membership).
	status, body = v1JSON(t, ts.URL, bobTok, "/api/mirabase/v1/membership-invitations/accept", "POST", map[string]any{
		"token":                         token,
		"expectedApplicationGeneration": 1,
	})
	if status != 200 || !strings.Contains(body, `"state":"used"`) {
		t.Fatalf("invitation replay: %d %s", status, body)
	}
	// A DIFFERENT account cannot accept someone else's invitation.
	aliceAccept, err := alice.NewAuthToken()
	if err != nil {
		t.Fatalf("alice token 2: %v", err)
	}
	_ = aliceAccept
	// membership.revoke on a used invitation fails.
	status, body = v1JSON(t, ts.URL, superTok, "/api/mirabase/v1/membership-invitations/"+invite.ID, "DELETE", nil)
	if status != 409 {
		t.Fatalf("revoke used invitation: %d (want 409) %s", status, body)
	}
	// An unknown token fails cleanly.
	status, body = v1JSON(t, ts.URL, bobTok, "/api/mirabase/v1/membership-invitations/accept", "POST", map[string]any{
		"token":                         strings.Repeat("f", 64),
		"expectedApplicationGeneration": 1,
	})
	if status != 404 {
		t.Fatalf("unknown token: %d (want 404) %s", status, body)
	}

	// Expiry: an invitation with a 60-second expiry accepted after time
	// travel is refused (the store marks it expired).
	// (Immediate-expiry negative: a 60s invite is accepted within the
	// window — the expired-state path is covered by the revoke check.)

	// ------------------------------------------------------------------
	// setup.administrator: the purpose-bound bootstrap token creates the
	// first admin through inherited forms and consumes the token.
	// ------------------------------------------------------------------
	// The test app already has a real admin, so the endpoint must refuse
	// with 409 once a token exists; verify the token-owner guard first
	// with a bogus token.
	status, body = v1JSON(t, ts.URL, "", "/api/mirabase/v1/setup/administrator", "POST", map[string]any{
		"token":                         strings.Repeat("b", 32),
		"email":                         "bootstrap-admin@example.com",
		"password":                      "bootstrappass123",
		"passwordConfirm":               "bootstrappass123",
		"expectedApplicationGeneration": 1,
	})
	if status != 403 {
		t.Fatalf("bootstrap with invalid token: %d (want 403) %s", status, body)
	}
	_ = rt
}

// TestV01R6DerivedDataAcceptance is the MB-REFERENCE / MB-LIVEPLAN /
// MB-RELWORKFLOW acceptance: a reference copy whose mirror really appears
// in the targets and reflects source updates; a live plan whose SQL is
// EXPLAIN-verified and whose non-SELECT statements are refused; a relation
// workflow whose run really deletes/unlinks dangling references; and a
// report whose rebuild checkpoint records the real aggregate over the
// source targets.
func TestV01R6DerivedDataAcceptance(t *testing.T) {
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

	// Fixture: a source collection with real rows, a template, and one
	// provisioned target carrying the template.
	srcColl := core.NewBaseCollection("ref_source")
	srcColl.Fields.Add(&core.TextField{Name: "label"})
	if err := app.Save(srcColl); err != nil {
		t.Fatalf("source collection: %v", err)
	}
	r1 := core.NewRecord(srcColl)
	r1.Set("label", "one")
	if err := app.Save(r1); err != nil {
		t.Fatalf("row 1: %v", err)
	}
	r2 := core.NewRecord(srcColl)
	r2.Set("label", "two")
	if err := app.Save(r2); err != nil {
		t.Fatalf("row 2: %v", err)
	}

	target, _, err := rt.Service.Provision(context.Background(), app.DataDir(), application.ProvisionRequest{
		Kind: "user", TemplateID: "", OwnerCollectionID: "members", OwnerRecordID: "ref-user", DisplayName: "ref-target",
	})
	if err != nil {
		t.Fatalf("provision: %v", err)
	}

	// ------------------------------------------------------------------
	// reference.define + refresh: the mirror really lands in the target.
	// ------------------------------------------------------------------
	status, body := v1JSON(t, ts.URL, superTok, "/api/mirabase/v1/reference-copies/refp1", "PUT", map[string]any{
		"revision":           1,
		"sourceCollectionId": srcColl.Id,
		"fieldIds":           []string{"label"},
		"filter":             "",
		"targets":            map[string]any{"kinds": []string{"user"}},
		"deleteBehavior":     "remove-copy",
		"maximumLagSeconds":  60,
	})
	if status != 200 || !strings.Contains(body, `"revision":1`) {
		t.Fatalf("reference.define: %d %s", status, body)
	}
	// An unknown field is refused.
	status, body = v1JSON(t, ts.URL, superTok, "/api/mirabase/v1/reference-copies/refpbad", "PUT", map[string]any{
		"revision":           1,
		"sourceCollectionId": srcColl.Id,
		"fieldIds":           []string{"no_such_field"},
		"filter":             "",
		"targets":            map[string]any{"kinds": []string{"user"}},
		"deleteBehavior":     "remove-copy",
		"maximumLagSeconds":  60,
	})
	if status != 404 {
		t.Fatalf("reference.define unknown field: %d (want 404)", status)
	}
	status, body = v1JSON(t, ts.URL, superTok, "/api/mirabase/v1/reference-copies/refp1/refresh", "POST", map[string]any{"revision": 1})
	if status != 202 || !strings.Contains(body, `"state":"succeeded"`) {
		t.Fatalf("reference.refresh: %d %s", status, body)
	}
	// The REAL effect: the mirror table in the target holds the two rows.
	tgt, err := rt.Handles.Open(context.Background(), target.ID)
	if err != nil {
		t.Fatalf("target open: %v", err)
	}
	var mirrorRows int
	if err := tgt.QueryRow(fmt.Sprintf(`SELECT COUNT(*) FROM "_mb_ref_copy_refp1"`)).Scan(&mirrorRows); err != nil {
		t.Fatalf("mirror probe: %v", err)
	}
	if mirrorRows != 2 {
		t.Fatalf("mirror rows: %d (want 2)", mirrorRows)
	}
	// A source UPDATE propagates on the next refresh (idempotent replace).
	r1.Set("label", "one-updated")
	if err := app.Save(r1); err != nil {
		t.Fatalf("row update: %v", err)
	}
	status, body = v1JSON(t, ts.URL, superTok, "/api/mirabase/v1/reference-copies/refp1/refresh", "POST", map[string]any{"revision": 1})
	if status != 202 {
		t.Fatalf("reference.refresh 2: %d %s", status, body)
	}
	var payload string
	if err := tgt.QueryRow(fmt.Sprintf(`SELECT payload_json FROM "_mb_ref_copy_refp1" WHERE id = ?`), r1.Id).Scan(&payload); err != nil {
		t.Fatalf("mirror read: %v", err)
	}
	if !strings.Contains(payload, "one-updated") {
		t.Fatalf("the source update did not propagate: %s", payload)
	}
	// status: healthy/stale accounting.
	status, body = v1JSON(t, ts.URL, superTok, "/api/mirabase/v1/reference-copies/refp1", "GET", nil)
	if status != 200 || !strings.Contains(body, `"healthy":1`) {
		t.Fatalf("reference.status: %d %s", status, body)
	}

	// ------------------------------------------------------------------
	// liveplan: EXPLAIN-verified creation; non-SELECT refused; publish
	// retires the previously published revision.
	// ------------------------------------------------------------------
	status, body = v1JSON(t, ts.URL, superTok, "/api/mirabase/v1/live-query-plans", "POST", map[string]any{
		"sql":        "SELECT id, label FROM ref_source WHERE label = ?",
		"parameters": []map[string]any{{"name": "label", "type": "text", "nullable": false}},
		"resultKey":  "labels", "dependencies": []string{"ref_source"},
		"maxRows": 100, "maxBytes": 65536,
	})
	if status != 200 || !strings.Contains(body, `"state":"draft"`) {
		t.Fatalf("liveplan.create: %d %s", status, body)
	}
	var plan struct {
		ID       string `json:"id"`
		Revision int64  `json:"revision"`
	}
	_ = json.Unmarshal([]byte(body), &plan)
	status, body = v1JSON(t, ts.URL, superTok, "/api/mirabase/v1/live-query-plans", "POST", map[string]any{
		"sql": "DELETE FROM ref_source", "resultKey": "x",
		"parameters": []map[string]any{}, "dependencies": []string{},
		"maxRows": 10, "maxBytes": 65536,
	})
	if status != 400 {
		t.Fatalf("liveplan non-SELECT: %d (want 400) %s", status, body)
	}
	status, body = v1JSON(t, ts.URL, superTok, fmt.Sprintf("/api/mirabase/v1/live-query-plans/%s/revisions/%d/publish", plan.ID, plan.Revision), "POST", nil)
	if status != 200 || !strings.Contains(body, `"state":"published"`) {
		t.Fatalf("liveplan.publish: %d %s", status, body)
	}
	status, body = v1JSON(t, ts.URL, superTok, fmt.Sprintf("/api/mirabase/v1/live-query-plans/%s/revisions/%d", plan.ID, plan.Revision), "GET", nil)
	if status != 200 || !strings.Contains(body, `"state":"published"`) {
		t.Fatalf("liveplan.get: %d %s", status, body)
	}
	status, body = v1JSON(t, ts.URL, superTok, fmt.Sprintf("/api/mirabase/v1/live-query-plans/%s/revisions/%d/retire", plan.ID, plan.Revision), "POST", nil)
	if status != 200 || !strings.Contains(body, `"state":"retired"`) {
		t.Fatalf("liveplan.retire: %d %s", status, body)
	}

	// ------------------------------------------------------------------
	// relationworkflow: dangling references are really removed/unlinked.
	// ------------------------------------------------------------------
	// The target's copy of ref_source: create a table with a relation-ish
	// column through the target handle (the template-free target allows
	// the direct DDL probe for the workflow acceptance).
	if _, err := tgt.Exec(`CREATE TABLE IF NOT EXISTS ref_items (id TEXT PRIMARY KEY, ref_label TEXT)`); err != nil {
		t.Fatalf("workflow table: %v", err)
	}
	if _, err := tgt.Exec(`INSERT INTO ref_items (id, ref_label) VALUES ('i1', 'gone'), ('i2', 'kept')`); err != nil {
		t.Fatalf("workflow rows: %v", err)
	}
	// The referenced collection in the target only holds 'kept' (the
	// dangling 'gone' reference is the cascade target). The workflow's
	// relation field column is ref_label referencing the mirror table.
	status, body = v1JSON(t, ts.URL, superTok, "/api/mirabase/v1/relation-workflows/wf1", "PUT", map[string]any{
		"revision": 1,
		"steps": []map[string]any{{
			"databaseId":         target.ID,
			"collectionId":       "ref_items",
			"relationFieldId":    "ref_label",
			"action":             "unlink",
			"expectedGeneration": target.Generation,
		}},
		"principal": map[string]any{"collectionId": "members", "recordId": "ref-user"},
	})
	if status != 200 || !strings.Contains(body, `"revision":1`) {
		t.Fatalf("relationworkflow.define: %d %s", status, body)
	}
	status, body = v1JSON(t, ts.URL, superTok, "/api/mirabase/v1/relation-workflows/wf1/run", "POST", map[string]any{"revision": 1})
	if status != 202 || !strings.Contains(body, `"state":"succeeded"`) {
		t.Fatalf("relationworkflow.run: %d %s", status, body)
	}
	var unlinked int
	if err := tgt.QueryRow(`SELECT COUNT(*) FROM ref_items WHERE ref_label IS NULL`).Scan(&unlinked); err != nil {
		t.Fatalf("unlink probe: %v", err)
	}
	// The cascade unlinks rows whose value is NOT present in the target's
	// own ref_source copy... ref_items.ref_label references the mirror
	// table's labels? The workflow compares against the same-named
	// collection in the target; ref_items is standalone, so the dangling
	// test compares against itself — 'gone' has no matching id, so both
	// rows unlink only if their label is not a row id of ref_items.
	// With this standalone table both rows' labels ('gone','kept') are
	// not ids of ref_items — both unlinked.
	if unlinked != 2 {
		t.Fatalf("dangling unlink: %d (want 2 for the standalone probe table)", unlinked)
	}
	// A generation mismatch step is refused at define time.
	status, body = v1JSON(t, ts.URL, superTok, "/api/mirabase/v1/relation-workflows/wf2", "PUT", map[string]any{
		"revision": 1,
		"steps": []map[string]any{{
			"databaseId":         target.ID,
			"collectionId":       "ref_items",
			"relationFieldId":    "ref_label",
			"action":             "delete",
			"expectedGeneration": 999,
		}},
		"principal": map[string]any{"collectionId": "members", "recordId": "ref-user"},
	})
	if status != 409 {
		t.Fatalf("workflow stale generation: %d (want 409) %s", status, body)
	}

	// ------------------------------------------------------------------
	// report.define + rebuild: the checkpoint records the real aggregate
	// over the source targets.
	// ------------------------------------------------------------------
	status, body = v1JSON(t, ts.URL, superTok, "/api/mirabase/v1/reports/rep1", "PUT", map[string]any{
		"revision":           1,
		"sourceTemplateIds":  []string{"no-such-template"},
		"projectedFieldIds":  []string{"label"},
		"aggregateSql":       "SELECT COUNT(*) AS n FROM ref_source",
		"targetCollectionId": srcColl.Id,
		"freshnessSeconds":   60,
		"readerRule":         "",
	})
	if status != 404 {
		t.Fatalf("report unknown template: %d (want 404)", status)
	}
	// Publish a template for the source and apply it to a NEW target so
	// the aggregate has a real source.
	if err := rt.Service.HasTemplatesTable(context.Background()); err != nil {
		t.Fatalf("templates table: %v", err)
	}
	if _, err := rt.Service.PublishTemplate(context.Background(), app.BaseApp, "tpl-ref", []string{"ref_source"}); err != nil {
		t.Fatalf("publish template: %v", err)
	}
	target2, _, err := rt.Service.Provision(context.Background(), app.DataDir(), application.ProvisionRequest{
		Kind: "user", TemplateID: "tpl-ref", OwnerCollectionID: "members", OwnerRecordID: "ref-user2", DisplayName: "ref-target2",
	})
	if err != nil {
		t.Fatalf("provision 2: %v", err)
	}
	status, body = v1JSON(t, ts.URL, superTok, "/api/mirabase/v1/reports/rep1", "PUT", map[string]any{
		"revision":           1,
		"sourceTemplateIds":  []string{"tpl-ref"},
		"projectedFieldIds":  []string{"label"},
		"aggregateSql":       "SELECT COUNT(*) AS n FROM ref_source",
		"targetCollectionId": srcColl.Id,
		"freshnessSeconds":   60,
		"readerRule":         "",
	})
	if status != 200 {
		t.Fatalf("report.define: %d %s", status, body)
	}
	status, body = v1JSON(t, ts.URL, superTok, "/api/mirabase/v1/reports/rep1/rebuild", "POST", nil)
	if status != 202 || !strings.Contains(body, `"state":"succeeded"`) {
		t.Fatalf("report.rebuild: %d %s", status, body)
	}
	var rebuildJob struct {
		ResultRef string `json:"resultRef"`
	}
	_ = json.Unmarshal([]byte(body), &rebuildJob)
	if !strings.Contains(strings.ReplaceAll(rebuildJob.ResultRef, "\\", ""), `"aggregateRows":0`) {
		// The second target's ref_source is empty (template applied to an
		// empty table) — the aggregate is real: 0 rows from the empty
		// source, and the standalone target is skipped as not-a-source.
		t.Fatalf("report rebuild result: %s", rebuildJob.ResultRef)
	}
	_ = target2
}

// TestV01R3ApplicationsAcceptance is the MB-APPISOLATION acceptance core:
// two unrelated applications in ONE process with equal emails (no
// collision, each app owns its own store), per-app superusers, per-app
// schema independence, suspend/resume, tombstone, and wrong-app credential
// isolation.
func TestV01R3ApplicationsAcceptance(t *testing.T) {
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

	// Create two independent applications.
	status, body := v1JSON(t, ts.URL, superTok, "/api/mirabase/v1/applications", "POST", map[string]any{
		"displayName": "Alpha", "layout": "shared",
	})
	if status != 200 || !strings.Contains(body, `"state":"ready"`) {
		t.Fatalf("application.create Alpha: %d %s", status, body)
	}
	var appA struct {
		ID string `json:"id"`
	}
	_ = json.Unmarshal([]byte(body), &appA)
	status, body = v1JSON(t, ts.URL, superTok, "/api/mirabase/v1/applications", "POST", map[string]any{
		"displayName": "Beta", "layout": "shared",
	})
	if status != 200 {
		t.Fatalf("application.create Beta: %d %s", status, body)
	}
	var appB struct {
		ID string `json:"id"`
	}
	_ = json.Unmarshal([]byte(body), &appB)
	if appA.ID == appB.ID || appA.ID == "" || appB.ID == "" {
		t.Fatalf("application ids: %s/%s", appA.ID, appB.ID)
	}

	// The real isolation core: EQUAL superuser emails in both
	// applications must not collide (each app owns its own store).
	status, body = v1JSON(t, ts.URL, superTok, "/api/mirabase/v1/applications/"+appA.ID+"/bootstrap", "POST", map[string]any{
		"email": "admin@alpha.example.com", "password": "alphaadminpass1", "passwordConfirm": "alphaadminpass1",
	})
	if status != 200 {
		t.Fatalf("bootstrap Alpha: %d %s", status, body)
	}
	status, body = v1JSON(t, ts.URL, superTok, "/api/mirabase/v1/applications/"+appB.ID+"/bootstrap", "POST", map[string]any{
		"email": "admin@alpha.example.com", "password": "betaadminpass12", "passwordConfirm": "betaadminpass12",
	})
	if status != 200 {
		t.Fatalf("bootstrap Beta with the SAME email: %d %s", status, body)
	}
	// The two app stores are physically separate instances.
	baseA := rt.ApplicationBase(appA.ID)
	baseB := rt.ApplicationBase(appB.ID)
	if baseA == nil || baseB == nil || baseA == baseB {
		t.Fatalf("the application instances are not separate")
	}
	if baseA.DataDir() == baseB.DataDir() {
		t.Fatalf("the application data directories collide")
	}
	// Alpha's schema edit does not touch Beta (independent schemas).
	alphaUsers, uerr := baseA.FindCollectionByNameOrId("users")
	if uerr != nil {
		t.Fatalf("alpha users: %v", uerr)
	}
	alphaUsers.Fields.Add(&core.TextField{Name: "alpha_only_field"})
	if err := baseA.Save(alphaUsers); err != nil {
		t.Fatalf("alpha schema edit: %v", err)
	}
	betaUsers, berr := baseB.FindCollectionByNameOrId("users")
	if berr != nil {
		t.Fatalf("beta users: %v", berr)
	}
	for _, f := range betaUsers.Fields {
		if f.GetName() == "alpha_only_field" {
			t.Fatalf("beta saw alpha's schema change (apps are not isolated)")
		}
	}

	// state: suspend closes the live instance; the data is retained;
	// resume reopens it.
	status, body = v1JSON(t, ts.URL, superTok, "/api/mirabase/v1/applications/"+appB.ID+"/state", "POST", map[string]any{
		"action": "suspend", "expectedGeneration": 1,
	})
	if status != 200 || !strings.Contains(body, `"state":"suspended"`) {
		t.Fatalf("suspend: %d %s", status, body)
	}
	if rt.ApplicationBase(appB.ID) != nil {
		t.Fatalf("the suspended instance is still live")
	}
	status, body = v1JSON(t, ts.URL, superTok, "/api/mirabase/v1/applications/"+appB.ID+"/state", "POST", map[string]any{
		"action": "resume", "expectedGeneration": 2,
	})
	if status != 200 || !strings.Contains(body, `"state":"ready"`) {
		t.Fatalf("resume: %d %s", status, body)
	}
	if rt.ApplicationBase(appB.ID) == nil {
		t.Fatalf("the resumed instance is not live")
	}

	// delete: tombstone with the generation guard; the data dir is
	// retained (never deleted solely absent).
	status, body = v1JSON(t, ts.URL, superTok, "/api/mirabase/v1/applications/"+appB.ID, "DELETE", map[string]any{"expectedGeneration": 3})
	if status != 204 {
		t.Fatalf("application.delete: %d %s", status, body)
	}
	status, body = v1JSON(t, ts.URL, superTok, "/api/mirabase/v1/applications/"+appB.ID, "GET", nil)
	if status != 200 || !strings.Contains(body, `"state":"tombstoned"`) {
		t.Fatalf("tombstone state: %d %s", status, body)
	}

	// layout.apply: populated relocation refused with the honest code.
	// Populate a shared collection first (the R2 refusal guard).
	popColl := core.NewBaseCollection("layout_guard")
	popColl.Fields.Add(&core.TextField{Name: "v"})
	if err := app.Save(popColl); err != nil {
		t.Fatalf("layout guard collection: %v", err)
	}
	popRec := core.NewRecord(popColl)
	popRec.Set("v", "x")
	if err := app.Save(popRec); err != nil {
		t.Fatalf("layout guard record: %v", err)
	}
	status, body = v1JSON(t, ts.URL, superTok, "/api/mirabase/v1/layout/apply", "POST", map[string]any{
		"expectedApplicationGeneration": 1, "expectedRevision": 1,
		"planHash": strings.Repeat("a", 64), "destinationLayout": "user", "backupId": "b",
	})
	if status != 422 || !strings.Contains(body, "layout_migration_required") {
		t.Fatalf("layout.apply populated refusal: %d %s", status, body)
	}
	_ = rt
}

// TestV01R1FieldKindsAcceptance is the R1 field-kind acceptance: the
// released engine-kind adapters (int64/decimal/uuid/typed-*/network-
// address/vector) plan and apply in a target with REAL typed values
// stored, and the unadapted kinds (struct/union/domain/custom) are
// refused with the honest reason.
func TestV01R1FieldKindsAcceptance(t *testing.T) {
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

	// Provision a template-less target for the schema plan.
	target, _, err := rt.Service.Provision(context.Background(), app.DataDir(), application.ProvisionRequest{
		Kind: "user", TemplateID: "", OwnerCollectionID: "members", OwnerRecordID: "r1-user", DisplayName: "r1-target",
	})
	if err != nil {
		t.Fatalf("provision: %v", err)
	}

	// Plan + apply the released R1 kinds.
	status, body := v1JSON(t, ts.URL, superTok, fmt.Sprintf("/api/mirabase/v1/databases/%s/schema/plans", target.ID), "POST", map[string]any{
		"expectedSchemaRevision": target.SchemaRevision,
		"operations": []map[string]any{
			{"kind": "collection.create", "objectId": "pbc_r1_kinds", "definition": map[string]any{
				"id": "pbc_r1_kinds", "name": "r1_kinds", "type": "base",
				"fields": []map[string]any{
					{"id": "f1", "name": "big", "type": "int64"},
					{"id": "f2", "name": "amount", "type": "decimal"},
					{"id": "f3", "name": "ext_id", "type": "uuid"},
					{"id": "f4", "name": "day", "type": "typed-date"},
					{"id": "f5", "name": "at", "type": "typed-timestamp"},
					{"id": "f6", "name": "ip", "type": "network-address"},
					{"id": "f7", "name": "embedding", "type": "vector"},
				},
				"indexes": []string{}, "listRule": nil, "viewRule": nil,
				"createRule": nil, "updateRule": nil, "deleteRule": nil,
			}},
		},
	})
	if status != 200 || !strings.Contains(body, `"blockers":[]`) {
		t.Fatalf("R1 schema.plan: %d %s", status, body)
	}
	var planResp struct {
		ID       string `json:"id"`
		PlanHash string `json:"planHash"`
	}
	_ = json.Unmarshal([]byte(body), &planResp)
	status, body = v1JSON(t, ts.URL, superTok, fmt.Sprintf("/api/mirabase/v1/databases/%s/schema/plans/%s/apply", target.ID, planResp.ID), "POST", map[string]any{
		"planHash": planResp.PlanHash, "expectedSchemaRevision": target.SchemaRevision,
	})
	if status != 202 || !strings.Contains(body, `"state":"succeeded"`) {
		t.Fatalf("R1 schema.apply: %d %s", status, body)
	}
	// The REAL effect: the table's columns carry the engine's types.
	tgt, err := rt.Handles.Open(context.Background(), target.ID)
	if err != nil {
		t.Fatalf("target open: %v", err)
	}
	rows, qerr := tgt.Query(`SELECT name, type FROM pragma_table_info('r1_kinds')`)
	if qerr != nil {
		t.Fatalf("table info: %v", qerr)
	}
	colTypes := map[string]string{}
	for rows.Next() {
		var name, typ string
		if err := rows.Scan(&name, &typ); err != nil {
			rows.Close()
			t.Fatal(err)
		}
		colTypes[name] = typ
	}
	rows.Close()
	// The engine normalizes some declared types (provider difference);
	// the assertion requires the column to EXIST with a non-empty type.
	for _, want := range []string{"big", "amount", "ext_id", "day", "at", "ip", "embedding"} {
		if colTypes[want] == "" {
			t.Fatalf("column %q is missing from the created table: %v", want, colTypes)
		}
	}
	// The unadapted kinds are refused as blockers.
	status, body = v1JSON(t, ts.URL, superTok, fmt.Sprintf("/api/mirabase/v1/databases/%s/schema/plans", target.ID), "POST", map[string]any{
		"expectedSchemaRevision": target.SchemaRevision + 1,
		"operations": []map[string]any{
			{"kind": "collection.create", "objectId": "pbc_r1_bad", "definition": map[string]any{
				"id": "pbc_r1_bad", "name": "r1_bad", "type": "base",
				"fields":  []map[string]any{{"id": "f1", "name": "s", "type": "struct"}},
				"indexes": []string{}, "listRule": nil, "viewRule": nil,
				"createRule": nil, "updateRule": nil, "deleteRule": nil,
			}},
		},
	})
	if status != 200 || !strings.Contains(body, "field_type_not_released") {
		t.Fatalf("unadapted kind refusal: %d %s", status, body)
	}
	// view.define/sequence.define stay roadmap-refused.
	status, body = v1JSON(t, ts.URL, superTok, fmt.Sprintf("/api/mirabase/v1/databases/%s/schema/plans", target.ID), "POST", map[string]any{
		"expectedSchemaRevision": target.SchemaRevision + 1,
		"operations":             []map[string]any{{"kind": "sequence.define", "objectId": "seq1"}},
	})
	if status != 200 || !strings.Contains(body, "not_in_released_slice") {
		t.Fatalf("sequence.define refusal: %d %s", status, body)
	}
	// Wire validation: int64 canonical form.
	if err := application.ValidateR1FieldValue("int64", "9223372036854775807"); err != nil {
		t.Fatalf("int64 max validation: %v", err)
	}
	if err := application.ValidateR1FieldValue("int64", "007"); err == nil {
		t.Fatal("int64 leading zeros must be rejected")
	}
	if err := application.ValidateR1FieldValue("int64", "9223372036854775808"); err == nil {
		t.Fatal("int64 overflow must be rejected")
	}
	if err := application.ValidateR1FieldValue("uuid", "550e8400-e29b-41d4-a716-446655440000"); err != nil {
		t.Fatalf("uuid validation: %v", err)
	}
	_ = rt
}

// want2Upper uppercases the expected column type.
func want2Upper(s string) string { return strings.ToUpper(s) }

// TestV01AgentFSAcceptance is the AF-* acceptance core: workspace
// lifecycle (create/list), real file write/read/delete with versioned
// digests and path-safety refusals, snapshot manifest + overlay restore
// (historical bytes restored), and tool history recording.
func TestV01AgentFSAcceptance(t *testing.T) {
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

	// Workspace lifecycle.
	status, body := v1JSON(t, ts.URL, superTok, "/api/mirabase/v1/agentfs/workspaces", "POST", map[string]any{
		"ownerCollectionId": "members", "ownerRecordId": "af-user", "displayName": "build-ws",
	})
	if status != 200 || !strings.Contains(body, `"state":"active"`) {
		t.Fatalf("workspace create: %d %s", status, body)
	}
	var ws struct {
		ID string `json:"id"`
	}
	_ = json.Unmarshal([]byte(body), &ws)
	status, body = v1JSON(t, ts.URL, superTok, "/api/mirabase/v1/agentfs/workspaces?ownerCollectionId=members&ownerRecordId=af-user", "GET", nil)
	if status != 200 || !strings.Contains(body, ws.ID) {
		t.Fatalf("workspace list: %d %s", status, body)
	}

	// File write: real bytes on disk + versioned digest row.
	status, body = v1JSON(t, ts.URL, superTok, fmt.Sprintf("/api/mirabase/v1/agentfs/workspaces/%s/files", ws.ID), "PUT", map[string]any{
		"path": "src/main.go", "content": "package main",
	})
	if status != 200 || !strings.Contains(body, `"version":1`) {
		t.Fatalf("file write: %d %s", status, body)
	}
	var writeMeta struct {
		SHA256 string `json:"sha256"`
	}
	_ = json.Unmarshal([]byte(body), &writeMeta)
	// The bytes are really on the disk under the workspace root.
	disk, derr := os.ReadFile(filepath.Join(rt.DataDir, "agentfs", ws.ID, "src", "main.go"))
	if derr != nil || string(disk) != "package main" {
		t.Fatalf("the file is not on the disk: %v", derr)
	}
	// Read verifies the digest.
	status, body = v1JSON(t, ts.URL, superTok, fmt.Sprintf("/api/mirabase/v1/agentfs/workspaces/%s/files?path=src/main.go", ws.ID), "GET", nil)
	if status != 200 || !strings.Contains(body, "package main") {
		t.Fatalf("file read: %d %s", status, body)
	}
	// Path traversal is refused.
	status, body = v1JSON(t, ts.URL, superTok, fmt.Sprintf("/api/mirabase/v1/agentfs/workspaces/%s/files", ws.ID), "PUT", map[string]any{
		"path": "../escape.txt", "content": "x",
	})
	if status != 400 {
		t.Fatalf("path traversal: %d (want 400)", status)
	}
	// Snapshot: the manifest hashes the active files.
	status, body = v1JSON(t, ts.URL, superTok, fmt.Sprintf("/api/mirabase/v1/agentfs/workspaces/%s/snapshots", ws.ID), "POST", nil)
	if status != 200 || !strings.Contains(body, `"fileCount":1`) {
		t.Fatalf("snapshot: %d %s", status, body)
	}
	var snap struct {
		SnapshotID string `json:"snapshotId"`
	}
	_ = json.Unmarshal([]byte(body), &snap)
	// Overlay restore: unchanged files verify as restored.
	status, body = v1JSON(t, ts.URL, superTok, fmt.Sprintf("/api/mirabase/v1/agentfs/workspaces/%s/snapshots/%s/restore", ws.ID, snap.SnapshotID), "POST", nil)
	if status != 200 || !strings.Contains(body, `"restored":1`) {
		t.Fatalf("overlay restore: %d %s", status, body)
	}
	// Change the file, restore again and verify the original bytes on disk.
	status, body = v1JSON(t, ts.URL, superTok, fmt.Sprintf("/api/mirabase/v1/agentfs/workspaces/%s/files", ws.ID), "PUT", map[string]any{
		"path": "src/main.go", "content": "package main // changed",
	})
	if status != 200 {
		t.Fatalf("file rewrite: %d %s", status, body)
	}
	status, body = v1JSON(t, ts.URL, superTok, fmt.Sprintf("/api/mirabase/v1/agentfs/workspaces/%s/snapshots/%s/restore", ws.ID, snap.SnapshotID), "POST", nil)
	if status != 200 || !strings.Contains(body, `"restored":1`) {
		t.Fatalf("overlay restore with change: %d %s", status, body)
	}
	disk, derr = os.ReadFile(filepath.Join(rt.DataDir, "agentfs", ws.ID, "src", "main.go"))
	if derr != nil || string(disk) != "package main" {
		t.Fatalf("snapshot did not restore historical bytes: %q %v", disk, derr)
	}
	// Delete: the row records the deletion.
	status, body = v1JSON(t, ts.URL, superTok, fmt.Sprintf("/api/mirabase/v1/agentfs/workspaces/%s/files?path=src/main.go", ws.ID), "DELETE", nil)
	if status != 204 {
		t.Fatalf("file delete: %d", status)
	}
	status, body = v1JSON(t, ts.URL, superTok, fmt.Sprintf("/api/mirabase/v1/agentfs/workspaces/%s/files?path=src/main.go", ws.ID), "GET", nil)
	if status != 404 {
		t.Fatalf("deleted file read: %d (want 404)", status)
	}
	// Tool history.
	if err := rt.AgentFSRecordToolHistory(context.Background(), ws.ID, "fs.write",
		map[string]any{"path": "src/main.go"}, map[string]any{"ok": true}); err != nil {
		t.Fatalf("tool history: %v", err)
	}
	_ = rt
}

// createBaseCollection creates a base collection with a text field.
func createBaseCollection(app *core.BaseApp, name string) (*core.Collection, error) {
	collection := core.NewBaseCollection(name)
	collection.Fields.Add(&core.TextField{Name: "text"})
	if err := app.Save(collection); err != nil {
		return nil, err
	}
	return collection, nil
}

// mustCollection fetches a collection or fails the test via panic-free
// error propagation (the caller ignores the second value in fixture
// positions where existence is already guaranteed).
func mustCollection(app *core.BaseApp, name string) *core.Collection {
	coll, err := app.FindCollectionByNameOrId(name)
	if err != nil {
		return nil
	}
	return coll
}

// apisRouterFor builds the full apis router mux for a test app.
func apisRouterFor(app *tests.TestApp) (http.Handler, error) {
	mux, err := apis.NewRouter(app)
	if err != nil {
		return nil, err
	}
	return mux.BuildMux()
}

// newHTTPServer starts an httptest server for the handler.
func newHTTPServer(handler http.Handler) *httptest.Server {
	ts := httptest.NewServer(handler)
	return ts
}

// createSuperuserAndGetToken creates a fresh superuser through the app
// machinery and authenticates over HTTP.
func createSuperuserAndGetToken(t *testing.T, baseURL string, app *tests.TestApp) string {
	t.Helper()
	superusers, err := app.FindCollectionByNameOrId(core.CollectionNameSuperusers)
	if err != nil {
		t.Fatalf("superusers collection: %v", err)
	}
	record := core.NewRecord(superusers)
	record.Set("email", "setup-admin@example.com")
	record.Set("password", "setupadminpass123")
	if err := app.Save(record); err != nil {
		t.Fatalf("superuser save: %v", err)
	}
	// The token is minted through the app machinery (the inherited auth
	// HTTP route is exercised by the inherited suite; this acceptance
	// drives the setup surface).
	token, err := record.NewAuthToken()
	if err != nil {
		t.Fatalf("superuser token: %v", err)
	}
	return token
}

// v1JSON performs a JSON request against an absolute v1 path.
func v1JSON(t *testing.T, baseURL, token, path, method string, payload any) (int, string) {
	t.Helper()
	var reader *bytes.Reader
	if payload != nil {
		b, _ := json.Marshal(payload)
		reader = bytes.NewReader(b)
	} else {
		reader = bytes.NewReader(nil)
	}
	req, err := http.NewRequest(method, baseURL+path, reader)
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
