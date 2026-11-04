package apis

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
	"testing"
	"time"

	"mirabase/core"
	"mirabase/internal/application"
	"mirabase/internal/mbengine"
	"mirabase/tools/types"
)

// TestV01_11bRecordJournalUpdateDeleteAcceptance is the change-journal
// acceptance leg for UPDATE/DELETE commits (integration-contracts.md D-14:
// "Durable changes remain the interface for every transaction" and "On a
// dependent commit, reevaluate within a consistent read transaction";
// AT-LIVE's required deletion checks):
//
//   - a record PATCH through the real v1 wire path appends ONE
//     _mb_changes/_mb_change_rows frame (update, before+after images) that
//     commits inside the same transaction as the data;
//   - a record DELETE appends the tombstone frame (delete, before image
//     only, after NULL);
//   - the live-query subscription Step (observed through the existing
//     live.stream SSE harness) advances on the update and the delete
//     commit: the keyed delta's toSequence equals the journal head each
//     commit produced;
//   - the sync push mutations (recordsync.go SyncPush update/delete) report
//     the NEW journal head as their mutation version — never the unchanged
//     head the mutation found — and a rejected mutation journals nothing.
//
// The journal rows are asserted directly on the target database (the same
// tables livequery.readJournalHead and the sync pull cursor read), so the
// proofs bind the durable interface, not a mock.
func TestV01_11bRecordJournalUpdateDeleteAcceptance(t *testing.T) {
	if mbengine.PinnedDigest() == "" {
		t.Skip("requires the pinned engine payload (run via _build/p02/parity_turso.sh)")
	}
	ctx := context.Background()
	app := liveTestApp(t)
	defer func() {
		app.ClearBootstrap()
		_ = os.RemoveAll(app.DataDir())
	}()

	rt, err := application.BindRuntime(app)
	if err != nil {
		t.Fatalf("BindRuntime: %v", err)
	}

	// The live surface plus the record surface: the mutations run through
	// the real v1 PATCH/DELETE handlers (apis/mirabase_records.go), which
	// must journal their commits the same way the create path does.
	ts, closeRouter := liveAcceptanceRouter(t, app)
	defer closeRouter()

	coll := core.NewBaseCollection("metrics")
	coll.Fields.Add(
		&core.TextField{Name: "name"},
		&core.TextField{Name: "note"},
	)
	// The list rule is the actor's narrowing rule the live evaluator
	// injects (all seeded rows satisfy it).
	coll.ListRule = types.Pointer("note != 'secret'")
	if err := app.Save(coll); err != nil {
		t.Fatalf("metrics collection: %v", err)
	}
	tpl, err := rt.Service.PublishTemplate(ctx, app, "tpl-live-journal", []string{coll.Name})
	if err != nil {
		t.Fatalf("publish template: %v", err)
	}
	aliceDB, _, err := rt.Service.Provision(ctx, app.DataDir(), application.ProvisionRequest{
		Kind: "user", TemplateID: tpl.ID,
		OwnerCollectionID: "users", OwnerRecordID: "alicejrnl000001", DisplayName: "alice",
	})
	if err != nil {
		t.Fatalf("alice provision: %v", err)
	}
	target, err := rt.Handles.Open(ctx, aliceDB.ID)
	if err != nil {
		t.Fatalf("open alice target: %v", err)
	}
	if err := application.ApplyTemplate(ctx, target, tpl); err != nil {
		t.Fatalf("apply template: %v", err)
	}
	store := application.NewRecordStore(target, coll.Name)
	seeded := map[string]string{}
	for _, row := range []map[string]any{
		{"name": "alpha", "note": "first"},
		{"name": "beta", "note": "second"},
	} {
		stored, err := store.Create(ctx, row)
		if err != nil {
			t.Fatalf("seed %v: %v", row, err)
		}
		seeded[stored["name"].(string)] = stored["id"].(string)
	}
	alphaID := seeded["alpha"]
	betaID := seeded["beta"]
	tokAlice := liveRecordToken(t, app, "users", "alicejrnl000001")
	dbRow, err := rt.Service.GetDatabase(ctx, aliceDB.ID)
	if err != nil {
		t.Fatalf("db row: %v", err)
	}

	// The journal starts EMPTY (the seed used the uncaptured Create, the
	// exact shape the snapshot sequence assertion below depends on).
	if head := journalHead(t, target); head != 0 {
		t.Fatalf("the journal must start empty before any mutation, got head %d", head)
	}

	// The published plan (superuser registry lifecycle). The result key is
	// the record id — the identity the injected rule correlation binds.
	plan, err := rt.Service.CreateLivePlan(ctx, app, &application.LivePlanWire{
		ID: "metricsJournal", SQL: `SELECT id, name, note FROM metrics ORDER BY name`,
		ResultKey: "id", MaxRows: 100, MaxBytes: 1048576,
		Dependencies: []string{coll.Name},
	})
	if err != nil {
		t.Fatalf("create plan: %v", err)
	}
	if _, err := rt.PublishLivePlan(ctx, plan.ID, plan.Revision); err != nil {
		t.Fatalf("publish plan: %v", err)
	}

	// Register the subscription and open the stream (the harness's Step
	// loop): the snapshot must arrive at the empty-journal boundary 0.
	status, body := liveJSON(t, ts.URL, tokAlice, fmt.Sprintf(
		"/api/mirabase/v1/databases/%s/live-queries", aliceDB.ID), "POST", map[string]any{
		"queryId":        "metricsJournal",
		"parameters":     []any{},
		"schemaRevision": dbRow.SchemaRevision,
	})
	if status != 200 {
		t.Fatalf("register: %d %s", status, body)
	}
	var reg map[string]any
	if err := json.Unmarshal([]byte(body), &reg); err != nil {
		t.Fatalf("register decode: %v", err)
	}
	subID, _ := reg["id"].(string)
	cursor, _ := reg["cursor"].(string)
	if subID == "" || cursor == "" {
		t.Fatalf("register id/cursor empty: %v", reg)
	}
	resp, cancelStream := liveOpenStream(t, ts.URL, tokAlice, aliceDB.ID, "metricsJournal", cursor, 40*time.Second)
	defer cancelStream()
	if resp.StatusCode != 200 {
		b, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		t.Fatalf("stream status: %d %s", resp.StatusCode, b)
	}
	frames := liveReadFrames(t, resp.Body, func(f liveFrame) bool { return f.Kind == "snapshot" }, 15*time.Second)
	if len(frames) == 0 || frames[len(frames)-1].Kind != "snapshot" {
		t.Fatalf("no snapshot frame delivered: %v", frames)
	}
	if seq, _ := frames[len(frames)-1].Data["sequence"].(float64); seq != 0 {
		t.Fatalf("snapshot sequence must be the empty-journal head 0: %v", frames[len(frames)-1].Data)
	}

	// ------------------------------------------------------------------
	// 1. UPDATE: the real wire PATCH appends one update frame (before +
	//    after images, collection-bound) and the subscription Step
	//    advances to the new journal head.
	// ------------------------------------------------------------------
	status, body = liveJSON(t, ts.URL, tokAlice, fmt.Sprintf(
		"/api/mirabase/v1/db/%s/collections/metrics/records/%s", aliceDB.ID, alphaID), "PATCH",
		map[string]any{"note": "first-edited"})
	if status != 200 {
		t.Fatalf("record patch: %d %s", status, body)
	}
	head := journalHead(t, target)
	if head != 1 {
		t.Fatalf("the record PATCH must append exactly one journal row (head 0→1), got head %d", head)
	}
	updateRows := journalRowsFor(t, target, head)
	if len(updateRows) != 1 {
		t.Fatalf("the update commit must carry exactly one change row: %+v", updateRows)
	}
	assertChangeRow(t, updateRows[0], changeExpectation{
		table: "metrics", collection: "metrics", recordID: alphaID, action: "update",
		before: map[string]any{"name": "alpha", "note": "first"},
		after:  map[string]any{"name": "alpha", "note": "first-edited"},
	})
	frames = liveReadFrames(t, resp.Body, func(f liveFrame) bool {
		if f.Kind != "update" {
			return false
		}
		for _, op := range livePatchOps(f) {
			if op["key"] == alphaID && op["op"] == "update" {
				return true
			}
		}
		return false
	}, 15*time.Second)
	if len(frames) == 0 {
		t.Fatalf("no delta frame for the UPDATE commit: the subscription Step did not re-evaluate")
	}
	delta := frames[len(frames)-1]
	ops := livePatchOps(delta)
	if len(ops) != 1 || ops[0]["op"] != "update" || ops[0]["key"] != alphaID {
		t.Fatalf("update delta patch: %v", delta.Raw)
	}
	if rowDelta, _ := ops[0]["row"].(map[string]any); rowDelta["note"] != "first-edited" {
		t.Fatalf("update delta row payload: %v", ops[0])
	}
	if from, _ := delta.Data["fromSequence"].(float64); from != 0 {
		t.Fatalf("update delta fromSequence: %v", delta.Data["fromSequence"])
	}
	if to, _ := delta.Data["toSequence"].(float64); int64(to) != head {
		t.Fatalf("update delta toSequence %v must equal the commit's journal head %d", delta.Data["toSequence"], head)
	}

	// ------------------------------------------------------------------
	// 2. DELETE: the real wire DELETE appends the tombstone frame
	//    (before image only) and the Step re-evaluates the deletion
	//    (AT-LIVE's deletion check).
	// ------------------------------------------------------------------
	status, body = liveJSON(t, ts.URL, tokAlice, fmt.Sprintf(
		"/api/mirabase/v1/db/%s/collections/metrics/records/%s", aliceDB.ID, alphaID), "DELETE", nil)
	if status != 204 {
		t.Fatalf("record delete: %d %s", status, body)
	}
	head = journalHead(t, target)
	if head != 2 {
		t.Fatalf("the record DELETE must append exactly one journal row (head 1→2), got head %d", head)
	}
	deleteRows := journalRowsFor(t, target, head)
	if len(deleteRows) != 1 {
		t.Fatalf("the delete commit must carry exactly one change row: %+v", deleteRows)
	}
	// The tombstone's before image carries the state the UPDATE produced:
	// the deletion journals what existed at its commit, not a stale row.
	assertChangeRow(t, deleteRows[0], changeExpectation{
		table: "metrics", collection: "metrics", recordID: alphaID, action: "delete",
		before: map[string]any{"name": "alpha", "note": "first-edited"},
		after:  nil,
	})
	frames = liveReadFrames(t, resp.Body, func(f liveFrame) bool {
		for _, op := range livePatchOps(f) {
			if op["key"] == alphaID && op["op"] == "delete" {
				return true
			}
		}
		return false
	}, 15*time.Second)
	if len(frames) == 0 {
		t.Fatalf("no delta frame for the DELETE commit: the subscription Step did not re-evaluate the deletion")
	}
	delta = frames[len(frames)-1]
	ops = livePatchOps(delta)
	// The keyed ordered patch applies atomically: deletes first — here the
	// alpha tombstone — and the following rows' position change folds into
	// an update carrying the row at its new position (the wire kinds are
	// only snapshot/insert/update/delete/reset/error, so a mixed patch
	// carries the "update" kind; the patch itself is the contract).
	if len(ops) != 2 || ops[0]["op"] != "delete" || ops[0]["key"] != alphaID {
		t.Fatalf("delete delta patch must open with the alpha delete: %v", delta.Raw)
	}
	if ops[1]["op"] != "update" || ops[1]["key"] != betaID {
		t.Fatalf("delete delta patch must fold beta's position change into an update: %v", delta.Raw)
	}
	if from, _ := delta.Data["fromSequence"].(float64); from != 1 {
		t.Fatalf("delete delta fromSequence: %v", delta.Data["fromSequence"])
	}
	if to, _ := delta.Data["toSequence"].(float64); int64(to) != head {
		t.Fatalf("delete delta toSequence %v must equal the commit's journal head %d", delta.Data["toSequence"], head)
	}
	cancelStream()

	// ------------------------------------------------------------------
	// 3. Sync push mutations report the NEW journal head as the mutation
	//    version (the repair the finding demands): the update's version
	//    is the frame its commit produced, the delete's is the next one —
	//    never the unchanged head found before the mutation.
	// ------------------------------------------------------------------
	third, err := store.Create(ctx, map[string]any{"name": "charlie", "note": "to-delete"})
	if err != nil {
		t.Fatalf("seed charlie: %v", err)
	}
	preHead := journalHead(t, target)
	push, err := rt.SyncPush(ctx, target, aliceDB.ID, "journal-test-instance", dbRow.SchemaRevision,
		[]application.SyncPushGroup{{
			GroupID: "g1",
			Mutations: []map[string]any{
				{
					"mutationId": "m1", "collection": "metrics", "recordId": betaID,
					"action": "update", "baseVersion": float64(0),
					"data": map[string]any{"note": "second-edited"},
				},
				{
					"mutationId": "m2", "collection": "metrics", "recordId": third["id"].(string),
					"action": "delete", "baseVersion": float64(0),
				},
			},
		}})
	if err != nil {
		t.Fatalf("sync push: %v", err)
	}
	groups, _ := push["groups"].([]map[string]any)
	if len(groups) != 1 {
		t.Fatalf("sync push groups: %v", push)
	}
	group := groups[0]
	if group["status"] != "committed" {
		t.Fatalf("sync push group status: %v (%v)", group["status"], group)
	}
	mutations, _ := group["mutations"].([]map[string]any)
	if len(mutations) != 2 {
		t.Fatalf("sync push mutations: %v", group)
	}
	m1, m2 := mutations[0], mutations[1]
	if v, _ := m1["version"].(int64); v != preHead+1 {
		t.Fatalf("the sync update mutation must report the NEW journal head %d as its version, got %v", preHead+1, m1["version"])
	}
	if v, _ := m2["version"].(int64); v != preHead+2 {
		t.Fatalf("the sync delete mutation must report the NEW journal head %d as its version, got %v", preHead+2, m2["version"])
	}
	if head := journalHead(t, target); head != preHead+2 {
		t.Fatalf("the sync mutations must journal exactly one frame each (head %d→%d), got head %d", preHead, preHead+2, head)
	}

	// Negative control: a rejected mutation journals nothing (atomic
	// capture — the frame commits with the data it describes or not at
	// all).
	failed, err := rt.SyncPush(ctx, target, aliceDB.ID, "journal-test-instance", dbRow.SchemaRevision,
		[]application.SyncPushGroup{{
			GroupID: "g2",
			Mutations: []map[string]any{
				{
					"mutationId": "m3", "collection": "metrics", "recordId": "missingrecord0001",
					"action": "update", "baseVersion": float64(0),
					"data": map[string]any{"note": "never"},
				},
			},
		}})
	if err != nil {
		t.Fatalf("sync push rejected group: %v", err)
	}
	fgroups, _ := failed["groups"].([]map[string]any)
	if len(fgroups) != 1 {
		t.Fatalf("rejected sync push groups: %v", failed)
	}
	fgroup := fgroups[0]
	if fgroup["status"] != "rejected" {
		t.Fatalf("the missing-record mutation must be rejected: %v", fgroup)
	}
	if head := journalHead(t, target); head != preHead+2 {
		t.Fatalf("a rejected sync mutation must not journal (head must stay %d), got %d", preHead+2, head)
	}
}

// ---------------------------------------------------------------------------
// Journal observation helpers (direct reads of the target's durable change
// journal — the same tables livequery.readJournalHead and the sync pull
// cursor consume).
// ---------------------------------------------------------------------------

// journalHead reads the target's durable change-journal commit head. A
// target that never recorded a change has no journal tables yet: its head
// is 0 (the same mirror semantics livequery.readJournalHead applies).
func journalHead(t *testing.T, target *sql.DB) int64 {
	t.Helper()
	var head int64
	err := target.QueryRow(`SELECT COALESCE(MAX(commit_seq), 0) FROM _mb_changes`).Scan(&head)
	if err != nil {
		if strings.Contains(err.Error(), "no such table") {
			return 0
		}
		t.Fatalf("journal head: %v", err)
	}
	return head
}

// journalRow is one captured change row of a commit frame.
type journalRow struct {
	CommitSeq    int64
	TableName    string
	CollectionID sql.NullString
	RecordID     string
	Action       string
	Before       []byte
	After        []byte
}

// journalRowsFor reads one commit frame's change rows in ordinal order.
func journalRowsFor(t *testing.T, target *sql.DB, commitSeq int64) []journalRow {
	t.Helper()
	rows, err := target.Query(
		`SELECT commit_seq, table_name, collection_id, record_id, action, before_json, after_json
		 FROM _mb_change_rows WHERE commit_seq = ? ORDER BY ordinal`, commitSeq)
	if err != nil {
		t.Fatalf("journal rows: %v", err)
	}
	defer rows.Close()
	var out []journalRow
	for rows.Next() {
		var r journalRow
		if err := rows.Scan(&r.CommitSeq, &r.TableName, &r.CollectionID, &r.RecordID, &r.Action, &r.Before, &r.After); err != nil {
			t.Fatalf("journal row scan: %v", err)
		}
		out = append(out, r)
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("journal rows: %v", err)
	}
	return out
}

// changeExpectation is one change row's expected content.
type changeExpectation struct {
	table      string
	collection string
	recordID   string
	action     string
	before     map[string]any
	after      map[string]any // nil asserts a NULL after image
}

// assertChangeRow asserts one captured change row's identity and images.
func assertChangeRow(t *testing.T, r journalRow, want changeExpectation) {
	t.Helper()
	if r.TableName != want.table || r.RecordID != want.recordID || r.Action != want.action {
		t.Fatalf("change row identity: table %q record %q action %q, want %q %q %q",
			r.TableName, r.RecordID, r.Action, want.table, want.recordID, want.action)
	}
	if !r.CollectionID.Valid || r.CollectionID.String != want.collection {
		t.Fatalf("change row collection_id: %v, want %q", r.CollectionID, want.collection)
	}
	image := func(raw []byte) map[string]any {
		if len(raw) == 0 {
			return nil
		}
		var m map[string]any
		if err := json.Unmarshal(raw, &m); err != nil {
			t.Fatalf("change row image decode: %v (%s)", err, raw)
		}
		return m
	}
	before := image(r.Before)
	for k, v := range want.before {
		if got, ok := before[k].(string); !ok || got != v.(string) {
			t.Fatalf("change row before image %q: %v, want %v (row %s)", k, before[k], v, r.Before)
		}
	}
	after := image(r.After)
	if want.after == nil {
		if len(r.After) != 0 {
			t.Fatalf("change row after image must be NULL for a %s, got %s", want.action, r.After)
		}
	} else {
		for k, v := range want.after {
			if got, ok := after[k].(string); !ok || got != v.(string) {
				t.Fatalf("change row after image %q: %v, want %v (row %s)", k, after[k], v, r.After)
			}
		}
	}
}
