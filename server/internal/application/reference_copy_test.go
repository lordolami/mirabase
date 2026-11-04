package application_test

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"mirabase/core"
	"mirabase/internal/application"
	"mirabase/internal/mbengine"
	"mirabase/tests"
)

// requireEngineForReference skips when the pinned payload is not compiled
// in (the parity harness passes the digest pin), mirroring requireEngine
// in application_test.go.
func requireEngineForReference(t *testing.T) {
	t.Helper()
	if mbengine.PinnedDigest() == "" {
		t.Skip("requires the pinned engine payload (run via _build/p02/parity_turso.sh)")
	}
	if err := mbengine.Ensure(); err != nil {
		t.Skipf("engine ensure failed: %v", err)
	}
}

// newReferenceFixture boots one application (the reference SOURCE), binds
// its runtime, provisions one kind=user TARGET and defines the projection
// policy (publishing binds the source CDC intent capture, so subsequent
// source writes advance the journal head the refresh gates on).
func newReferenceFixture(t *testing.T, policyID string, lagSeconds int) (app *core.BaseApp, rt *application.Runtime, targetID string, sourceColl *core.Collection, policy *application.ReferencePolicyWire) {
	t.Helper()
	ctx := context.Background()
	ta, err := tests.NewTestApp()
	if err != nil {
		t.Fatalf("NewTestApp: %v", err)
	}
	t.Cleanup(ta.Cleanup)
	app = ta.BaseApp

	rt, err = application.BindRuntime(app)
	if err != nil {
		t.Fatalf("BindRuntime: %v", err)
	}

	sourceColl = core.NewBaseCollection("ref_src_rows")
	sourceColl.Fields.Add(&core.TextField{Name: "label"})
	if err := app.Save(sourceColl); err != nil {
		t.Fatalf("source collection: %v", err)
	}

	target, _, err := rt.Service.Provision(ctx, app.DataDir(), application.ProvisionRequest{
		Kind: "user", TemplateID: "",
		OwnerCollectionID: "members", OwnerRecordID: "ref-acc-user",
		DisplayName: "reference target",
	})
	if err != nil {
		t.Fatalf("provision target: %v", err)
	}

	policy = &application.ReferencePolicyWire{
		ID:                 policyID,
		Revision:           1,
		SourceCollectionID: sourceColl.Id,
		FieldIDs:           []string{"label"},
		Filter:             "",
		Targets:            map[string]any{"kinds": []any{"user"}},
		DeleteBehavior:     "remove-copy",
		MaximumLagSeconds:  lagSeconds,
	}
	if _, err := rt.Service.DefineReferencePolicy(ctx, app, policy); err != nil {
		t.Fatalf("define policy: %v", err)
	}
	return app, rt, target.ID, sourceColl, policy
}

// sourceJournalHead reads the SOURCE change-journal head (the value the
// refresh must capture inside its source read transaction).
func sourceJournalHead(t *testing.T, app *core.BaseApp) int64 {
	t.Helper()
	var head int64
	if err := app.DBConn().QueryRow(`SELECT COALESCE(MAX(commit_seq), 0) FROM _mb_changes`).Scan(&head); err != nil {
		t.Fatalf("source journal head: %v", err)
	}
	return head
}

// mirrorSnapshot reads the target mirror (id → payload) and its recorded
// checkpoint row.
func mirrorSnapshot(t *testing.T, rt *application.Runtime, targetID, policyID string) (map[string]string, int64, int64, string) {
	t.Helper()
	ctx := context.Background()
	handle, err := rt.Handles.Open(ctx, targetID)
	if err != nil {
		t.Fatalf("target open: %v", err)
	}
	rows, err := handle.QueryContext(ctx,
		fmt.Sprintf(`SELECT id, payload_json FROM %s`, fmt.Sprintf("%q", "_mb_ref_copy_"+policyID)))
	if err != nil {
		t.Fatalf("mirror probe: %v", err)
	}
	payloads := map[string]string{}
	for rows.Next() {
		var id, payload string
		if err := rows.Scan(&id, &payload); err != nil {
			rows.Close()
			t.Fatalf("mirror scan: %v", err)
		}
		payloads[id] = payload
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		t.Fatalf("mirror rows: %v", err)
	}
	var lastSeq, revision int64
	var state string
	if err := handle.QueryRowContext(ctx,
		fmt.Sprintf(`SELECT last_source_seq, policy_revision, state FROM %s WHERE singleton = 1`,
			fmt.Sprintf("%q", "_mb_ref_state_"+policyID)),
	).Scan(&lastSeq, &revision, &state); err != nil {
		t.Fatalf("state probe: %v", err)
	}
	return payloads, lastSeq, revision, state
}

// MB-REFERENCE acceptance: refresh copies rows stamped with the REAL
// source change-journal head; duplicate execution is a safe skip; source
// updates propagate; source deletions remove the copy; an older event
// never overwrites a newer sequence; a policy edit invalidates the old
// work and rebuilds; a schema mismatch quarantines the target with a
// typed recorded failure; status reports healthy/stale/quarantined with
// sequences and failures; and a read that requires a healthy copy
// surfaces the typed reference_stale error.
func TestReferenceCopyRefreshAcceptance(t *testing.T) {
	requireEngineForReference(t)
	ctx := context.Background()
	app, rt, targetID, sourceColl, policy := newReferenceFixture(t, "refacc", 60)

	// Two source rows: each create is a captured change, so the journal
	// head advances before the first refresh.
	r1 := core.NewRecord(sourceColl)
	r1.Set("label", "one")
	if err := app.Save(r1); err != nil {
		t.Fatalf("row 1: %v", err)
	}
	r2 := core.NewRecord(sourceColl)
	r2.Set("label", "two")
	if err := app.Save(r2); err != nil {
		t.Fatalf("row 2: %v", err)
	}

	// ------------------------------------------------------------------
	// 1) Refresh copies the rows stamped with the REAL source sequence
	// (the journal head captured inside the source read transaction; the
	// old hardcoded source_seq=1 is gone).
	// ------------------------------------------------------------------
	res, err := rt.RefreshReferencePolicy(ctx, app, policy)
	if err != nil {
		t.Fatalf("refresh: %v", err)
	}
	if res["healthy"] != 1 || res["failed"] != 0 {
		t.Fatalf("refresh result: %v", res)
	}
	head := sourceJournalHead(t, app)
	if head < 2 {
		t.Fatalf("the source journal must have advanced on the source writes: %d", head)
	}
	payloads, lastSeq, revision, state := mirrorSnapshot(t, rt, targetID, "refacc")
	if len(payloads) != 2 {
		t.Fatalf("mirror rows: %d (want 2): %v", len(payloads), payloads)
	}
	if !strings.Contains(payloads[r1.Id], "one") || !strings.Contains(payloads[r2.Id], "two") {
		t.Fatalf("mirror payloads: %v", payloads)
	}
	if lastSeq != head {
		t.Fatalf("checkpoint sequence %d != the real source journal head %d", lastSeq, head)
	}
	if revision != 1 || state != "healthy" {
		t.Fatalf("checkpoint must record the policy revision and state: rev=%d state=%s", revision, state)
	}
	var mirrorSeq int64
	tgt, err := rt.Handles.Open(ctx, targetID)
	if err != nil {
		t.Fatalf("target open: %v", err)
	}
	if err := tgt.QueryRowContext(ctx,
		`SELECT source_seq FROM "_mb_ref_copy_refacc" WHERE id = ?`, r1.Id).Scan(&mirrorSeq); err != nil {
		t.Fatalf("mirror sequence probe: %v", err)
	}
	if mirrorSeq != head {
		t.Fatalf("copy row source_seq %d != the batch's real source head %d", mirrorSeq, head)
	}

	// Status: the target is healthy with its sequence reported.
	st, err := rt.GetReferenceStatus(ctx, policy)
	if err != nil {
		t.Fatalf("status: %v", err)
	}
	if st.Healthy != 1 || st.Stale != 0 || st.Quarantined != 0 || st.Failed != 0 || len(st.TargetStates) != 1 {
		t.Fatalf("status: %+v", st)
	}
	if st.TargetStates[0].DatabaseID != targetID || st.TargetStates[0].LastSourceSeq != head {
		t.Fatalf("status target entry: %+v", st.TargetStates[0])
	}

	// The copy reads back through the healthy-copy read path.
	copyRes, err := rt.ReadReferenceCopy(ctx, policy, targetID, r1.Id)
	if err != nil {
		t.Fatalf("read copy: %v", err)
	}
	if copyRes["lastSourceSeq"].(int64) != head {
		t.Fatalf("copy read diagnostics: %v", copyRes)
	}

	// ------------------------------------------------------------------
	// 2) Duplicate execution is safe: with no source change the head
	// equals the applied sequence and the batch is skipped (nothing
	// rewrites the mirror or regresses the checkpoint).
	// ------------------------------------------------------------------
	res, err = rt.RefreshReferencePolicy(ctx, app, policy)
	if err != nil {
		t.Fatalf("duplicate refresh: %v", err)
	}
	if res["healthy"] != 0 || res["skipped"] != 1 {
		t.Fatalf("duplicate refresh must skip: %v", res)
	}
	_, lastSeq, revision, state = mirrorSnapshot(t, rt, targetID, "refacc")
	if lastSeq != head || revision != 1 || state != "healthy" {
		t.Fatalf("duplicate refresh changed the checkpoint: seq=%d rev=%d state=%s", lastSeq, revision, state)
	}

	// ------------------------------------------------------------------
	// 3) A source update advances the head and propagates on the next
	// refresh.
	// ------------------------------------------------------------------
	r1.Set("label", "one-updated")
	if err := app.Save(r1); err != nil {
		t.Fatalf("row update: %v", err)
	}
	headAfterUpdate := sourceJournalHead(t, app)
	if headAfterUpdate <= head {
		t.Fatalf("the update must advance the source journal: %d -> %d", head, headAfterUpdate)
	}
	res, err = rt.RefreshReferencePolicy(ctx, app, policy)
	if err != nil {
		t.Fatalf("refresh after update: %v", err)
	}
	if res["healthy"] != 1 || res["skipped"] != 0 {
		t.Fatalf("the newer batch must apply: %v", res)
	}
	var updatedPayload string
	if err := tgt.QueryRowContext(ctx,
		`SELECT payload_json FROM "_mb_ref_copy_refacc" WHERE id = ?`, r1.Id).Scan(&updatedPayload); err != nil {
		t.Fatalf("mirror read: %v", err)
	}
	if !strings.Contains(updatedPayload, "one-updated") {
		t.Fatalf("the source update did not propagate: %s", updatedPayload)
	}
	_, lastSeq, _, _ = mirrorSnapshot(t, rt, targetID, "refacc")
	if lastSeq != headAfterUpdate {
		t.Fatalf("checkpoint must record the newest applied head %d, got %d", headAfterUpdate, lastSeq)
	}

	// ------------------------------------------------------------------
	// 4) A source deletion removes the copy (deleteBehavior remove-copy).
	// ------------------------------------------------------------------
	if err := app.Delete(r2); err != nil {
		t.Fatalf("source delete: %v", err)
	}
	headAfterDelete := sourceJournalHead(t, app)
	if headAfterDelete <= headAfterUpdate {
		t.Fatalf("the deletion must advance the source journal: %d -> %d", headAfterUpdate, headAfterDelete)
	}
	res, err = rt.RefreshReferencePolicy(ctx, app, policy)
	if err != nil {
		t.Fatalf("refresh after delete: %v", err)
	}
	if res["healthy"] != 1 {
		t.Fatalf("refresh after delete: %v", res)
	}
	payloads, _, _, _ = mirrorSnapshot(t, rt, targetID, "refacc")
	if len(payloads) != 1 {
		t.Fatalf("the deleted source row's copy must be removed: %v", payloads)
	}
	if _, ok := payloads[r2.Id]; ok {
		t.Fatalf("the copy of the deleted source row survived: %v", payloads)
	}
	if _, ok := payloads[r1.Id]; !ok {
		t.Fatalf("the surviving copy disappeared: %v", payloads)
	}

	// ------------------------------------------------------------------
	// 5) An older event can never overwrite a newer sequence: the
	// checkpoint is moved past every reachable head (simulating a newer
	// batch applied by another worker), then an older source change is
	// captured. The stale batch must skip and the copy must keep the
	// pre-change value.
	// ------------------------------------------------------------------
	currentHead := sourceJournalHead(t, app)
	if _, err := tgt.ExecContext(ctx,
		`UPDATE "_mb_ref_state_refacc" SET last_source_seq = ? WHERE singleton = 1`, currentHead+100); err != nil {
		t.Fatalf("checkpoint bump: %v", err)
	}
	r1.Set("label", "one-poisoned")
	if err := app.Save(r1); err != nil {
		t.Fatalf("poison write: %v", err)
	}
	res, err = rt.RefreshReferencePolicy(ctx, app, policy)
	if err != nil {
		t.Fatalf("older-event refresh: %v", err)
	}
	if res["healthy"] != 0 || res["skipped"] != 1 {
		t.Fatalf("the older event must not overwrite the newer sequence: %v", res)
	}
	var poisonedPayload string
	if err := tgt.QueryRowContext(ctx,
		`SELECT payload_json FROM "_mb_ref_copy_refacc" WHERE id = ?`, r1.Id).Scan(&poisonedPayload); err != nil {
		t.Fatalf("mirror read: %v", err)
	}
	if strings.Contains(poisonedPayload, "one-poisoned") {
		t.Fatalf("the older event overwrote the newer copy: %s", poisonedPayload)
	}
	_, lastSeq, _, _ = mirrorSnapshot(t, rt, targetID, "refacc")
	if lastSeq != currentHead+100 {
		t.Fatalf("the checkpoint must not regress: %d", lastSeq)
	}

	// ------------------------------------------------------------------
	// 6) A source schema mismatch quarantines the target with the typed
	// recorded failure; the quarantine survives refresh attempts.
	// ------------------------------------------------------------------
	if _, err := app.DBConn().ExecContext(ctx, `ALTER TABLE ref_src_rows DROP COLUMN label`); err != nil {
		t.Fatalf("drop column: %v", err)
	}
	res, err = rt.RefreshReferencePolicy(ctx, app, policy)
	if !errors.Is(err, application.ErrReferenceSchemaIncompatible) {
		t.Fatalf("the schema mismatch must surface the typed failure, got %v (res %v)", err, res)
	}
	if res["quarantined"] != 1 {
		t.Fatalf("the mismatch must quarantine the target: %v", res)
	}
	_, _, _, state = mirrorSnapshot(t, rt, targetID, "refacc")
	if state != "quarantined" {
		t.Fatalf("the target state must be quarantined, got %q", state)
	}
	st, err = rt.GetReferenceStatus(ctx, policy)
	if err != nil {
		t.Fatalf("status after quarantine: %v", err)
	}
	if st.Quarantined != 1 || st.Healthy != 0 || len(st.TargetStates) != 1 {
		t.Fatalf("status must report the quarantine: %+v", st)
	}
	if !strings.Contains(st.TargetStates[0].Failure, "reference_schema_incompatible") {
		t.Fatalf("the recorded failure must be typed: %+v", st.TargetStates[0])
	}
	// A refresh attempt while quarantined stays quarantined (the failure
	// is recorded until the policy is edited).
	res, err = rt.RefreshReferencePolicy(ctx, app, policy)
	if !errors.Is(err, application.ErrReferenceSchemaIncompatible) {
		t.Fatalf("the quarantined refresh must keep surfacing the typed failure, got %v", err)
	}
	if res["quarantined"] != 1 {
		t.Fatalf("the quarantine must survive: %v", res)
	}
	// A read that requires a healthy copy surfaces reference_stale.
	if _, err := rt.ReadReferenceCopy(ctx, policy, targetID, ""); !errors.Is(err, application.ErrReferenceCopyStale) {
		t.Fatalf("a quarantined copy read must surface reference_stale, got %v", err)
	}

	// ------------------------------------------------------------------
	// 7) A policy edit invalidates the old refresh work: revision 2
	// rebuilds the projection and clears the quarantine (the source
	// schema is repaired first).
	// ------------------------------------------------------------------
	if _, err := app.DBConn().ExecContext(ctx, `ALTER TABLE ref_src_rows ADD COLUMN label TEXT`); err != nil {
		t.Fatalf("re-add column: %v", err)
	}
	policy.Revision = 2
	if _, err := rt.Service.DefineReferencePolicy(ctx, app, policy); err != nil {
		t.Fatalf("policy edit: %v", err)
	}
	res, err = rt.RefreshReferencePolicy(ctx, app, policy)
	if err != nil {
		t.Fatalf("refresh after policy edit: %v", err)
	}
	if res["healthy"] != 1 || res["quarantined"] != 0 {
		t.Fatalf("the policy edit must rebuild and clear the quarantine: %v", res)
	}
	_, _, revision, state = mirrorSnapshot(t, rt, targetID, "refacc")
	if revision != 2 || state != "healthy" {
		t.Fatalf("the rebuild must record the new revision and health: rev=%d state=%s", revision, state)
	}

	// ------------------------------------------------------------------
	// 8) Staleness is detected against MaximumLagSeconds: backdate the
	// apply time past the declared lag and both the status and the
	// healthy-copy read report it.
	// ------------------------------------------------------------------
	staleAt := time.Now().UTC().Add(-2 * time.Hour).Format(time.RFC3339Nano)
	if _, err := tgt.ExecContext(ctx,
		`UPDATE "_mb_ref_state_refacc" SET updated_at = ? WHERE singleton = 1`, staleAt); err != nil {
		t.Fatalf("backdate: %v", err)
	}
	st, err = rt.GetReferenceStatus(ctx, policy)
	if err != nil {
		t.Fatalf("status after backdate: %v", err)
	}
	if st.Stale != 1 || st.Healthy != 0 || len(st.TargetStates) != 1 {
		t.Fatalf("the stale target must be reported: %+v", st)
	}
	if st.TargetStates[0].State != "stale" || st.TargetStates[0].UpdatedAt != staleAt {
		t.Fatalf("the stale entry must carry the apply diagnostics: %+v", st.TargetStates[0])
	}
	if _, err := rt.ReadReferenceCopy(ctx, policy, targetID, r1.Id); !errors.Is(err, application.ErrReferenceCopyStale) {
		t.Fatalf("a stale copy read must surface reference_stale, got %v", err)
	}
	// A real source change lets the worker apply again; the fresh apply
	// clears the staleness.
	r1.Set("label", "after-lag")
	if err := app.Save(r1); err != nil {
		t.Fatalf("lag write: %v", err)
	}
	res, err = rt.RefreshReferencePolicy(ctx, app, policy)
	if err != nil {
		t.Fatalf("refresh after staleness: %v", err)
	}
	if res["healthy"] != 1 {
		t.Fatalf("the fresh apply must restore health: %v", res)
	}
	st, err = rt.GetReferenceStatus(ctx, policy)
	if err != nil {
		t.Fatalf("status after fresh apply: %v", err)
	}
	if st.Healthy != 1 || st.Stale != 0 {
		t.Fatalf("the fresh apply must be healthy again: %+v", st)
	}
	// And the healthy copy reads back its projected row.
	copyRes, err = rt.ReadReferenceCopy(ctx, policy, targetID, r1.Id)
	if err != nil {
		t.Fatalf("healthy copy read: %v", err)
	}
	rowsOut := copyRes["rows"].([]map[string]any)
	if len(rowsOut) != 1 || rowsOut[0]["id"].(string) != r1.Id {
		t.Fatalf("healthy copy rows: %v", copyRes)
	}

	// ------------------------------------------------------------------
	// 9) A SAME-REVISION policy edit also invalidates the old refresh
	// work: the definition epoch advances, so the refresh REBUILDS the
	// projection (it does not skip under an equal revision with an
	// unchanged source sequence) and the edited selection is applied.
	// ------------------------------------------------------------------
	prevEpoch := policy.DefinitionEpoch
	if prevEpoch < 1 {
		t.Fatalf("the policy edits must have advanced the definition epoch: %d", prevEpoch)
	}
	policy.Revision = 2 // unchanged revision: only the selection changes
	policy.Filter = "label = 'never-matches'"
	if _, err := rt.Service.DefineReferencePolicy(ctx, app, policy); err != nil {
		t.Fatalf("same-revision policy edit: %v", err)
	}
	if policy.DefinitionEpoch != prevEpoch+1 {
		t.Fatalf("the same-revision edit must bump the definition epoch: %d -> %d", prevEpoch, policy.DefinitionEpoch)
	}
	res, err = rt.RefreshReferencePolicy(ctx, app, policy)
	if err != nil {
		t.Fatalf("refresh after same-revision edit: %v", err)
	}
	if res["healthy"] != 1 || res["skipped"] != 0 {
		t.Fatalf("the same-revision edit must rebuild, not skip: %v", res)
	}
	payloads, _, revision, state = mirrorSnapshot(t, rt, targetID, "refacc")
	if len(payloads) != 0 {
		t.Fatalf("the edited selection must have rebuilt (and emptied) the projection: %v", payloads)
	}
	if revision != 2 || state != "healthy" {
		t.Fatalf("the rebuild must keep the revision and health: rev=%d state=%s", revision, state)
	}
	// The healthy read serves the REBUILT (empty) projection, never the
	// pre-edit rows.
	copyRes, err = rt.ReadReferenceCopy(ctx, policy, targetID, "")
	if err != nil {
		t.Fatalf("read after the same-revision rebuild: %v", err)
	}
	if got := len(copyRes["rows"].([]map[string]any)); got != 0 {
		t.Fatalf("the pre-edit copy row must be gone: %d rows", got)
	}
}

// waitForMirrorRows polls until the target mirror holds exactly want rows
// (the worker applies asynchronously; a missing mirror or a transient
// lock keeps polling until the deadline).
func waitForMirrorRows(t *testing.T, rt *application.Runtime, targetID, policyID string, want int, timeout time.Duration) {
	t.Helper()
	ctx := context.Background()
	deadline := time.Now().Add(timeout)
	for {
		handle, err := rt.Handles.Open(ctx, targetID)
		if err == nil {
			var n int
			qerr := handle.QueryRowContext(ctx,
				fmt.Sprintf(`SELECT COUNT(*) FROM %s`, fmt.Sprintf("%q", "_mb_ref_copy_"+policyID))).Scan(&n)
			if qerr == nil && n == want {
				return
			}
		}
		if time.Now().After(deadline) {
			t.Fatalf("the mirror never reached %d rows on %s: %v", want, targetID, err)
		}
		time.Sleep(20 * time.Millisecond)
	}
}

// MB-REFERENCE worker acceptance (implementation-closure.md: "The source
// CDC commit schedules a durable refresh intent. A shared worker reads a
// consistent authorized source snapshot and upserts/deletes the exact
// read-only projection in bounded target transactions"): the shared
// worker consumes the durable source journal intents WITHOUT any manual
// refresh call; a same-revision policy edit (a definition-epoch bump)
// invalidates the old refresh work and the worker rebuilds the
// projection; and a stopped worker stops refreshing.
func TestReferenceCopyWorkerRefreshesWithoutManualCall(t *testing.T) {
	requireEngineForReference(t)
	ctx := context.Background()
	app, rt, targetID, sourceColl, policy := newReferenceFixture(t, "refwork", 60)

	// Start the shared worker; NOTHING in this test calls
	// RefreshReferencePolicy manually.
	w := application.StartReferenceWorker(rt, app, 20*time.Millisecond, nil)
	t.Cleanup(w.Stop)

	r1 := core.NewRecord(sourceColl)
	r1.Set("label", "auto")
	if err := app.Save(r1); err != nil {
		t.Fatalf("row 1: %v", err)
	}
	r2 := core.NewRecord(sourceColl)
	r2.Set("label", "gone")
	if err := app.Save(r2); err != nil {
		t.Fatalf("row 2: %v", err)
	}
	waitForMirrorRows(t, rt, targetID, "refwork", 2, 6*time.Second)

	// The worker-applied copy reads back through the healthy path with
	// the real source/target sequences.
	copyRes, err := rt.ReadReferenceCopy(ctx, policy, targetID, r1.Id)
	if err != nil {
		t.Fatalf("worker-applied copy read: %v", err)
	}
	if copyRes["lastSourceSeq"].(int64) != sourceJournalHead(t, app) {
		t.Fatalf("worker copy diagnostics: %v", copyRes)
	}

	// A SAME-REVISION policy edit invalidates the old refresh work: the
	// definition epoch advances and the worker rebuilds the projection
	// (no revision bump, no manual refresh).
	prevEpoch := policy.DefinitionEpoch
	policy.Filter = "label != 'gone'"
	if _, err := rt.Service.DefineReferencePolicy(ctx, app, policy); err != nil {
		t.Fatalf("same-revision policy edit: %v", err)
	}
	if policy.DefinitionEpoch != prevEpoch+1 {
		t.Fatalf("the same-revision edit must bump the definition epoch: %d -> %d", prevEpoch, policy.DefinitionEpoch)
	}
	waitForMirrorRows(t, rt, targetID, "refwork", 1, 6*time.Second)
	payloads, _, _, state := mirrorSnapshot(t, rt, targetID, "refwork")
	if _, ok := payloads[r2.Id]; ok {
		t.Fatalf("the edited selection must have removed the stale copy: %v", payloads)
	}
	if _, ok := payloads[r1.Id]; !ok {
		t.Fatalf("the surviving copy disappeared: %v", payloads)
	}
	if state != "healthy" {
		t.Fatalf("the rebuilt copy must be healthy: %s", state)
	}

	// A stopped worker stops refreshing: a further source write is NOT
	// copied (clean lifecycle stop, and the exit signal is observable).
	w.Stop()
	select {
	case <-w.Done():
	default:
		t.Fatalf("the worker must have exited after Stop")
	}
	r3 := core.NewRecord(sourceColl)
	r3.Set("label", "post-stop")
	if err := app.Save(r3); err != nil {
		t.Fatalf("row 3: %v", err)
	}
	time.Sleep(300 * time.Millisecond) // several tick intervals
	payloads, _, _, _ = mirrorSnapshot(t, rt, targetID, "refwork")
	if _, ok := payloads[r3.Id]; ok {
		t.Fatalf("the stopped worker must not refresh: %v", payloads)
	}
}

// MB-REFERENCE definition acceptance (implementation-closure.md: "allowed
// nonsecret field IDs" and "Application auth/secrets are forbidden
// projections"): hidden fields and auth-secret fields are refused at
// DEFINITION time (a non-hidden auth secret included), a refused
// definition is never stored, and a plain nonsecret auth field is
// projectable.
func TestReferenceCopyDefinitionRejectsSecretAndHiddenFields(t *testing.T) {
	requireEngineForReference(t)
	ctx := context.Background()
	app, rt, _, _, _ := newReferenceFixture(t, "refsec", 60)

	// A fresh auth collection: password/tokenKey are the hidden system
	// secrets; "secret" is a NON-hidden field whose name is an auth
	// secret (the name-based refusal must fire independently of the
	// hidden flag).
	authColl := core.NewCollection(core.CollectionTypeAuth, "ref_auth_clients")
	authColl.Fields.Add(&core.TextField{Name: "secret"})
	if err := app.Save(authColl); err != nil {
		t.Fatalf("auth collection: %v", err)
	}

	for _, tc := range []struct {
		name     string
		fieldIDs []string
	}{
		{"hidden password", []string{"password"}},
		{"hidden tokenKey", []string{"tokenKey"}},
		{"non-hidden auth secret", []string{"secret"}},
	} {
		p := &application.ReferencePolicyWire{
			ID:                 "refsec_" + strings.NewReplacer(" ", "_", "-", "_").Replace(tc.name),
			Revision:           1,
			SourceCollectionID: authColl.Id,
			FieldIDs:           tc.fieldIDs,
			Filter:             "",
			Targets:            map[string]any{"kinds": []any{"user"}},
			DeleteBehavior:     "remove-copy",
			MaximumLagSeconds:  60,
		}
		_, err := rt.Service.DefineReferencePolicy(ctx, app, p)
		if err == nil {
			t.Fatalf("the %s must be refused at definition time", tc.name)
		}
		if !strings.Contains(err.Error(), "cannot be projected into a reference copy") {
			t.Fatalf("the %s refusal must be a projection refusal: %v", tc.name, err)
		}
		if _, gerr := rt.Service.GetReferencePolicy(ctx, p.ID); !errors.Is(gerr, application.ErrNotFound) {
			t.Fatalf("the refused definition must not be stored: %v", gerr)
		}
	}

	// A plain nonsecret auth field is projectable.
	ok := &application.ReferencePolicyWire{
		ID:                 "refsec_plain",
		Revision:           1,
		SourceCollectionID: authColl.Id,
		FieldIDs:           []string{"email"},
		Filter:             "",
		Targets:            map[string]any{"kinds": []any{"user"}},
		DeleteBehavior:     "remove-copy",
		MaximumLagSeconds:  60,
	}
	if _, err := rt.Service.DefineReferencePolicy(ctx, app, ok); err != nil {
		t.Fatalf("a nonsecret auth field must be projectable: %v", err)
	}
}

// MB-REFERENCE record-selection acceptance (implementation-closure.md:
// "record selection AST"; queries-and-search.md: "Never interpolate client
// field names, filter strings ... into executable SQL"): the policy
// selection is compiled through the parsed filter machinery into a
// parameterized predicate — an injected raw SQL expression is refused at
// definition time and never stored, while a VALID selection compiles and
// projects exactly the matching rows.
func TestReferenceCopyFilterASTRejectsRawSQL(t *testing.T) {
	requireEngineForReference(t)
	ctx := context.Background()
	app, rt, targetID, sourceColl, _ := newReferenceFixture(t, "refast", 60)

	r1 := core.NewRecord(sourceColl)
	r1.Set("label", "one")
	if err := app.Save(r1); err != nil {
		t.Fatalf("row 1: %v", err)
	}
	r2 := core.NewRecord(sourceColl)
	r2.Set("label", "two")
	if err := app.Save(r2); err != nil {
		t.Fatalf("row 2: %v", err)
	}

	injected := []string{
		`id = 'x' OR (SELECT 1 FROM _mb_changes) = 1`, // a raw subquery riding in a boolean group
		`label = 'x'; DROP TABLE ref_src_rows`,        // a statement riding in after the selection
		`WHERE label = 'x'`,                           // a raw SQL WHERE prefix
	}
	for i, expr := range injected {
		p := &application.ReferencePolicyWire{
			ID:                 fmt.Sprintf("refast_inj%d", i),
			Revision:           1,
			SourceCollectionID: sourceColl.Id,
			FieldIDs:           []string{"label"},
			Filter:             expr,
			Targets:            map[string]any{"kinds": []any{"user"}},
			DeleteBehavior:     "remove-copy",
			MaximumLagSeconds:  60,
		}
		if _, err := rt.Service.DefineReferencePolicy(ctx, app, p); err == nil {
			t.Fatalf("the injected expression must be refused at definition time: %q", expr)
		}
		if _, gerr := rt.Service.GetReferencePolicy(ctx, p.ID); !errors.Is(gerr, application.ErrNotFound) {
			t.Fatalf("the refused expression must not be stored: %v", gerr)
		}
	}

	// A VALID selection compiles and projects exactly the matching rows
	// (the AST path must not over-refuse).
	valid := &application.ReferencePolicyWire{
		ID:                 "refast_valid",
		Revision:           1,
		SourceCollectionID: sourceColl.Id,
		FieldIDs:           []string{"label"},
		Filter:             "label = 'one'",
		Targets:            map[string]any{"kinds": []any{"user"}},
		DeleteBehavior:     "remove-copy",
		MaximumLagSeconds:  60,
	}
	if _, err := rt.Service.DefineReferencePolicy(ctx, app, valid); err != nil {
		t.Fatalf("a valid filter must compile: %v", err)
	}
	res, err := rt.RefreshReferencePolicy(ctx, app, valid)
	if err != nil {
		t.Fatalf("refresh with a valid selection: %v", err)
	}
	if res["healthy"] != 1 {
		t.Fatalf("refresh with a valid selection: %v", res)
	}
	payloads, _, _, _ := mirrorSnapshot(t, rt, targetID, "refast_valid")
	if len(payloads) != 1 {
		t.Fatalf("the selection must project only the matching rows: %v", payloads)
	}
	if _, ok := payloads[r1.Id]; !ok {
		t.Fatalf("the matching copy is missing: %v", payloads)
	}
	if _, ok := payloads[r2.Id]; ok {
		t.Fatalf("the non-matching row must not be projected: %v", payloads)
	}
}

// MB-REFERENCE lag acceptance (implementation-closure.md: "Reads that
// require a healthy copy return reference_stale when its declared lag is
// exceeded; diagnostics show the source/target sequence"): a copy that is
// healthy right after its apply surfaces reference_stale once the
// declared lag is REALLY exceeded (no clock backdating), with the
// target/source sequence diagnostics; status reports the stale target.
func TestReferenceCopyReadPastDeclaredLagSurfacesReferenceStale(t *testing.T) {
	requireEngineForReference(t)
	ctx := context.Background()
	app, rt, targetID, sourceColl, policy := newReferenceFixture(t, "reflag", 2)

	r1 := core.NewRecord(sourceColl)
	r1.Set("label", "laggy")
	if err := app.Save(r1); err != nil {
		t.Fatalf("row: %v", err)
	}
	if _, err := rt.RefreshReferencePolicy(ctx, app, policy); err != nil {
		t.Fatalf("refresh: %v", err)
	}
	// Healthy immediately after the apply (inside the 2s lag).
	copyRes, err := rt.ReadReferenceCopy(ctx, policy, targetID, r1.Id)
	if err != nil {
		t.Fatalf("the fresh copy must read healthy: %v", err)
	}
	if copyRes["lastSourceSeq"].(int64) != sourceJournalHead(t, app) {
		t.Fatalf("healthy diagnostics must carry the sequences: %v", copyRes)
	}
	if _, ok := copyRes["sourceHead"]; !ok {
		t.Fatalf("healthy diagnostics must carry the source head: %v", copyRes)
	}
	// Past the declared lag the same read surfaces reference_stale with
	// the sequence diagnostics.
	time.Sleep(2500 * time.Millisecond)
	_, err = rt.ReadReferenceCopy(ctx, policy, targetID, r1.Id)
	if !errors.Is(err, application.ErrReferenceCopyStale) {
		t.Fatalf("a copy past its declared lag must surface reference_stale, got %v", err)
	}
	if !strings.Contains(err.Error(), "sequence") {
		t.Fatalf("the stale diagnostics must show the sequences: %v", err)
	}
	st, serr := rt.GetReferenceStatus(ctx, policy)
	if serr != nil {
		t.Fatalf("status: %v", serr)
	}
	if st.Stale != 1 || st.Healthy != 0 {
		t.Fatalf("the past-lag target must be reported stale: %+v", st)
	}
}
