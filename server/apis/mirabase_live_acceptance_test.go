package apis

import (
	"bufio"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"mirabase/core"
	"mirabase/internal/application"
	"mirabase/internal/mbengine"
	"mirabase/tools/router"
	"mirabase/tools/types"
)

// TestV01_11LiveQueryAcceptance is the D-14 live-query acceptance leg
// (live.register / live.stream / live.cancel over the real HTTP surface):
//
//   - register returns the LiveQuery wire shape and its stream delivers a
//     real SQL snapshot at the captured journal boundary;
//   - the actor's CURRENT collection rule injects into the evaluation:
//     a row the rule excludes (note = 'secret') is missing from the
//     ordinary actor's snapshot and present in the superuser's — a
//     privileged SQL plan is not an unfiltered record subscription;
//   - a committed data change produces a keyed ordered delta
//     (fromSequence/toSequence with the record's result key);
//   - cancel stops delivery (idempotent {id, closed:true}; the open stream
//     terminates and a later change is never delivered);
//   - a wrong actor is rejected (another record's token cannot register
//     the database nor stream/step with someone else's cursor);
//   - repeat registration returns the SAME subscription;
//   - two parameter sets of one plan are TWO subscriptions: each streams
//     its own snapshot and each cancels independently (by parameter set or
//     id), while an identifier-less cancel of the ambiguous plan is
//     refused;
//   - a non-deterministic plan (random(), strftime('now')) is refused at
//     creation and a mistyped parameter is refused BEFORE subscription
//     allocation (the durable subscription count never grows);
//   - a plan whose dependency collection grants no list rule is forbidden
//     for an ordinary actor (fail closed, before allocation).
func TestV01_11LiveQueryAcceptance(t *testing.T) {
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

	// The live surface plus the record surface (the data change is written
	// through the real v1 record path, which captures the durable change
	// journal rows the subscription depends on).
	ts, closeRouter := liveAcceptanceRouter(t, app)
	defer closeRouter()

	// Collection, template and Alice's private database. The list rule is
	// the actor's narrowing rule the evaluator injects: every row whose
	// note is 'secret' must never reach an ordinary actor's stream.
	coll := core.NewBaseCollection("metrics")
	coll.Fields.Add(
		&core.TextField{Name: "name"},
		&core.TextField{Name: "note"},
		&core.TextField{Name: "code"},
	)
	coll.Fields.GetByName("code").SetHidden(true)
	coll.ListRule = types.Pointer("note != 'secret'")
	if err := app.Save(coll); err != nil {
		t.Fatalf("metrics collection: %v", err)
	}
	tpl, err := rt.Service.PublishTemplate(ctx, app, "tpl-live", []string{coll.Name})
	if err != nil {
		t.Fatalf("publish template: %v", err)
	}
	aliceDB, _, err := rt.Service.Provision(ctx, app.DataDir(), application.ProvisionRequest{
		Kind: "user", TemplateID: tpl.ID,
		OwnerCollectionID: "users", OwnerRecordID: "aliceliv0000001", DisplayName: "alice",
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
		// The row the actor's rule excludes: it exists in the plan's raw
		// result but must never appear in the injected evaluation.
		{"name": "vault-1", "note": "secret"},
	} {
		stored, err := store.Create(ctx, row)
		if err != nil {
			t.Fatalf("seed %v: %v", row, err)
		}
		seeded[stored["name"].(string)] = stored["id"].(string)
	}
	tokAlice := liveRecordToken(t, app, "users", "aliceliv0000001")
	tokBob := liveRecordToken(t, app, "users", "boblive00000011")
	// The durable subscription rows bind the actor's COLLECTION ID (the
	// store key the register request carries).
	usersColl, err := app.FindCollectionByNameOrId("users")
	if err != nil {
		t.Fatalf("users collection: %v", err)
	}
	actorColID := usersColl.Id
	// The superuser streams the plan unfiltered (the plan is the
	// superuser's own privileged surface).
	tokSuper := liveSuperuserToken(t, app)
	dbRow, err := rt.Service.GetDatabase(ctx, aliceDB.ID)
	if err != nil {
		t.Fatalf("db row: %v", err)
	}

	// The published plan (superuser registry lifecycle). The plan's SQL
	// selects the record id: the result key binds the record identity the
	// injected rule correlation narrows.
	plan, err := rt.Service.CreateLivePlan(ctx, app, &application.LivePlanWire{
		ID: "metricsByName", SQL: `SELECT id, name, note FROM metrics ORDER BY name`,
		ResultKey: "id", MaxRows: 100, MaxBytes: 1048576,
		Dependencies: []string{coll.Name},
	})
	if err != nil {
		t.Fatalf("create plan: %v", err)
	}
	if _, err := rt.PublishLivePlan(ctx, plan.ID, plan.Revision); err != nil {
		t.Fatalf("publish plan: %v", err)
	}

	registerBody := func() map[string]any {
		return map[string]any{
			"queryId":        "metricsByName",
			"parameters":     []any{},
			"schemaRevision": dbRow.SchemaRevision,
		}
	}

	// ------------------------------------------------------------------
	// 1. Register: the LiveQuery wire shape.
	// ------------------------------------------------------------------
	status, body := liveJSON(t, ts.URL, tokAlice, fmt.Sprintf(
		"/api/mirabase/v1/databases/%s/live-queries", aliceDB.ID), "POST", registerBody())
	if status != 200 {
		t.Fatalf("register: %d %s", status, body)
	}
	var reg1 map[string]any
	if err := json.Unmarshal([]byte(body), &reg1); err != nil {
		t.Fatalf("register decode: %v", err)
	}
	for _, key := range []string{"id", "databaseId", "queryId", "schemaRevision", "policyEpoch", "cursor", "streamUrl", "expiresAt"} {
		if _, ok := reg1[key]; !ok {
			t.Fatalf("register response misses %q: %v", key, reg1)
		}
	}
	subID, _ := reg1["id"].(string)
	cursor, _ := reg1["cursor"].(string)
	if subID == "" || cursor == "" {
		t.Fatalf("register id/cursor empty: %v", reg1)
	}
	if reg1["databaseId"] != aliceDB.ID || reg1["queryId"] != "metricsByName" {
		t.Fatalf("register context binding: %v", reg1)
	}
	if want := fmt.Sprintf("/api/mirabase/v1/databases/%s/live-queries/metricsByName/stream", aliceDB.ID); reg1["streamUrl"] != want {
		t.Fatalf("streamUrl: %v", reg1["streamUrl"])
	}

	// ------------------------------------------------------------------
	// 5. Repeat registration returns the SAME subscription.
	// ------------------------------------------------------------------
	status, body = liveJSON(t, ts.URL, tokAlice, fmt.Sprintf(
		"/api/mirabase/v1/databases/%s/live-queries", aliceDB.ID), "POST", registerBody())
	if status != 200 {
		t.Fatalf("repeat register: %d %s", status, body)
	}
	var reg2 map[string]any
	_ = json.Unmarshal([]byte(body), &reg2)
	if reg2["id"] != subID {
		t.Fatalf("repeat registration returned a different subscription: %v vs %v", reg2["id"], subID)
	}

	// ------------------------------------------------------------------
	// 2. The stream delivers the snapshot at the captured boundary, with
	//    the actor's rule injected: the 'secret' row is never in an
	//    ordinary actor's result.
	// ------------------------------------------------------------------
	resp, cancelStream := liveOpenStream(t, ts.URL, tokAlice, aliceDB.ID, "metricsByName", cursor, 30*time.Second)
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
	snap := frames[len(frames)-1].Data
	rows, _ := snap["rows"].([]any)
	if len(rows) != 2 {
		t.Fatalf("snapshot rows (the 'secret' row must be excluded by the injected rule): %v", snap["rows"])
	}
	for _, r := range rows {
		first, _ := r.(map[string]any)
		if first["note"] == "secret" {
			t.Fatalf("the injected rule did not exclude the secret row: %v", rows)
		}
	}
	first, _ := rows[0].(map[string]any)
	if first["name"] != "alpha" {
		t.Fatalf("snapshot order: %v", rows)
	}
	if seq, _ := snap["sequence"].(float64); seq != 0 {
		t.Fatalf("snapshot sequence (journal head at register): %v", snap["sequence"])
	}

	// Superuser control: the same plan streams the FULL result (the plan
	// is the superuser's own privileged surface) — the narrowing is the
	// ordinary actor's injected rule, not a missing row.
	status, body = liveJSON(t, ts.URL, tokSuper, fmt.Sprintf(
		"/api/mirabase/v1/databases/%s/live-queries", aliceDB.ID), "POST", registerBody())
	if status != 200 {
		t.Fatalf("superuser register: %d %s", status, body)
	}
	var regSuper map[string]any
	_ = json.Unmarshal([]byte(body), &regSuper)
	superCursor, _ := regSuper["cursor"].(string)
	if superCursor == "" {
		t.Fatalf("superuser register cursor: %v", regSuper)
	}
	suResp, suCancel := liveOpenStream(t, ts.URL, tokSuper, aliceDB.ID, "metricsByName", superCursor, 15*time.Second)
	defer suCancel()
	if suResp.StatusCode != 200 {
		b, _ := io.ReadAll(suResp.Body)
		t.Fatalf("superuser stream status: %d %s", suResp.StatusCode, b)
	}
	suFrames := liveReadFrames(t, suResp.Body, func(f liveFrame) bool { return f.Kind == "snapshot" }, 15*time.Second)
	if len(suFrames) == 0 {
		t.Fatalf("no superuser snapshot frame")
	}
	suRows, _ := suFrames[len(suFrames)-1].Data["rows"].([]any)
	if len(suRows) != 3 {
		t.Fatalf("superuser snapshot rows (unfiltered plan result): %v", suRows)
	}

	// ------------------------------------------------------------------
	// 3. A committed data change produces a keyed delta (keyed by the
	//    record id).
	// ------------------------------------------------------------------
	status, body = liveJSON(t, ts.URL, tokAlice, fmt.Sprintf(
		"/api/mirabase/v1/db/%s/collections/metrics/records", aliceDB.ID), "POST",
		map[string]any{"name": "charlie", "note": "third"})
	if status != 200 {
		t.Fatalf("record create: %d %s", status, body)
	}
	var created map[string]any
	_ = json.Unmarshal([]byte(body), &created)
	charlieID, _ := created["id"].(string)
	if charlieID == "" {
		t.Fatalf("record create response carries no id: %s", body)
	}
	frames = liveReadFrames(t, resp.Body, func(f liveFrame) bool {
		if f.Kind != "insert" && f.Kind != "update" {
			return false
		}
		for _, op := range livePatchOps(f) {
			if op["key"] == charlieID && op["op"] == "insert" {
				return true
			}
		}
		return false
	}, 15*time.Second)
	if len(frames) == 0 {
		t.Fatalf("no delta frame for the committed change")
	}
	delta := frames[len(frames)-1]
	if delta.Kind != "insert" {
		t.Fatalf("delta kind: %s (%v)", delta.Kind, delta.Raw)
	}
	ops := livePatchOps(delta)
	if len(ops) != 1 || ops[0]["op"] != "insert" || ops[0]["key"] != charlieID {
		t.Fatalf("keyed patch: %v", delta.Raw)
	}
	from, _ := delta.Data["fromSequence"].(float64)
	if from != 0 {
		t.Fatalf("fromSequence: %v", delta.Data["fromSequence"])
	}
	if to, _ := delta.Data["toSequence"].(float64); to <= from {
		t.Fatalf("toSequence did not advance: %v", delta.Data["toSequence"])
	}
	rowDelta, _ := ops[0]["row"].(map[string]any)
	if rowDelta["note"] != "third" {
		t.Fatalf("delta row payload: %v", ops[0])
	}

	// ------------------------------------------------------------------
	// 4. Cancel stops delivery: idempotent {id, closed:true}; the open
	//    stream terminates; a change committed after termination is never
	//    delivered.
	// ------------------------------------------------------------------
	cancelPath := fmt.Sprintf("/api/mirabase/v1/databases/%s/live-queries/metricsByName", aliceDB.ID)
	status, body = liveJSON(t, ts.URL, tokAlice, cancelPath, "DELETE", nil)
	if status != 200 {
		t.Fatalf("cancel: %d %s", status, body)
	}
	var cancelled map[string]any
	_ = json.Unmarshal([]byte(body), &cancelled)
	if cancelled["id"] != subID || cancelled["closed"] != true {
		t.Fatalf("cancel response: %v", cancelled)
	}
	// Idempotent repeat cancel.
	status, body = liveJSON(t, ts.URL, tokAlice, cancelPath, "DELETE", nil)
	if status != 200 {
		t.Fatalf("repeat cancel: %d %s", status, body)
	}
	var cancelled2 map[string]any
	_ = json.Unmarshal([]byte(body), &cancelled2)
	if cancelled2["id"] != subID || cancelled2["closed"] != true {
		t.Fatalf("repeat cancel response: %v", cancelled2)
	}
	// The open stream terminates with the cancellation error frame.
	frames = liveReadFrames(t, resp.Body, func(f liveFrame) bool { return f.Kind == "error" }, 15*time.Second)
	if len(frames) == 0 || frames[len(frames)-1].Kind != "error" {
		t.Fatalf("the stream did not terminate after cancel: %v", frames)
	}
	lastErr := frames[len(frames)-1]
	if code, _ := lastErr.Data["code"].(string); code != "subscription_cancelled" {
		t.Fatalf("termination frame: %v", lastErr.Raw)
	}
	// Drain to EOF: the stream is over.
	_ = liveReadFrames(t, resp.Body, nil, 3*time.Second)
	// A change committed AFTER termination is never delivered.
	status, body = liveJSON(t, ts.URL, tokAlice, fmt.Sprintf(
		"/api/mirabase/v1/db/%s/collections/metrics/records", aliceDB.ID), "POST",
		map[string]any{"name": "dave", "note": "after-cancel"})
	if status != 200 {
		t.Fatalf("post-cancel record create: %d %s", status, body)
	}
	postFrames := liveReadFrames(t, resp.Body, nil, 2*time.Second)
	for _, f := range postFrames {
		for _, op := range livePatchOps(f) {
			if op["key"] == "dave" {
				t.Fatalf("cancelled subscription delivered %v", f.Raw)
			}
		}
	}

	// ------------------------------------------------------------------
	// 6. A wrong actor is rejected: another record's token cannot register
	//    the database, cannot cancel someone's query, and cannot step with
	//    someone else's cursor.
	// ------------------------------------------------------------------
	status, body = liveJSON(t, ts.URL, tokBob, fmt.Sprintf(
		"/api/mirabase/v1/databases/%s/live-queries", aliceDB.ID), "POST", registerBody())
	if status != 403 {
		t.Fatalf("bob register on alice's database: %d %s (want 403)", status, body)
	}
	status, body = liveJSON(t, ts.URL, tokBob, cancelPath, "DELETE", nil)
	if status != 403 {
		t.Fatalf("bob cancel of alice's query: %d %s (want 403)", status, body)
	}
	// Bob streams Alice's subscription with Alice's cursor.
	bobResp, bobCancel := liveOpenStream(t, ts.URL, tokBob, aliceDB.ID, "metricsByName", cursor, 10*time.Second)
	defer bobCancel()
	if bobResp.StatusCode != 403 {
		b, _ := io.ReadAll(bobResp.Body)
		t.Fatalf("bob stream with alice's cursor: %d %s (want 403)", bobResp.StatusCode, b)
	}
	bobResp.Body.Close()

	// A malformed cursor is refused before the stream opens.
	badResp, badCancel := liveOpenStream(t, ts.URL, tokAlice, aliceDB.ID, "metricsByName", "not-a-cursor", 10*time.Second)
	defer badCancel()
	if badResp.StatusCode != 400 {
		b, _ := io.ReadAll(badResp.Body)
		t.Fatalf("malformed cursor: %d %s (want 400)", badResp.StatusCode, b)
	}
	badResp.Body.Close()

	// Cancel is terminal: a later registration is a NEW subscription.
	status, body = liveJSON(t, ts.URL, tokAlice, fmt.Sprintf(
		"/api/mirabase/v1/databases/%s/live-queries", aliceDB.ID), "POST", registerBody())
	if status != 200 {
		t.Fatalf("post-cancel register: %d %s", status, body)
	}
	var reg3 map[string]any
	_ = json.Unmarshal([]byte(body), &reg3)
	if reg3["id"] == subID {
		t.Fatalf("a cancelled subscription was revived: %v", reg3)
	}

	// ------------------------------------------------------------------
	// 7. Two parameter sets of one plan are TWO independent
	//    subscriptions: each streams its own snapshot and each cancels
	//    independently; the identifier-less cancel of the ambiguous plan
	//    is refused ("the latest row" is never an identity).
	// ------------------------------------------------------------------
	byNotePlan, err := rt.Service.CreateLivePlan(ctx, app, &application.LivePlanWire{
		ID: "metricsByNote", SQL: `SELECT id, name, note FROM metrics WHERE note = ?`,
		ResultKey: "id", MaxRows: 100, MaxBytes: 1048576,
		Parameters:   []map[string]any{{"name": "note", "type": "text"}},
		Dependencies: []string{coll.Name},
	})
	if err != nil {
		t.Fatalf("create byNote plan: %v", err)
	}
	if _, err := rt.PublishLivePlan(ctx, byNotePlan.ID, byNotePlan.Revision); err != nil {
		t.Fatalf("publish byNote plan: %v", err)
	}
	registerWith := func(params ...any) (string, string, int, string) {
		st, bd := liveJSON(t, ts.URL, tokAlice, fmt.Sprintf(
			"/api/mirabase/v1/databases/%s/live-queries", aliceDB.ID), "POST", map[string]any{
			"queryId":        "metricsByNote",
			"parameters":     params,
			"schemaRevision": dbRow.SchemaRevision,
		})
		var reg map[string]any
		_ = json.Unmarshal([]byte(bd), &reg)
		id, _ := reg["id"].(string)
		cur, _ := reg["cursor"].(string)
		return id, cur, st, bd
	}
	sub1, cursor1, st, bd := registerWith("first")
	if st != 200 || sub1 == "" || cursor1 == "" {
		t.Fatalf("register [first]: %d %s", st, bd)
	}
	sub2, cursor2, st, bd := registerWith("second")
	if st != 200 || sub2 == "" || cursor2 == "" {
		t.Fatalf("register [second]: %d %s", st, bd)
	}
	if sub1 == sub2 {
		t.Fatalf("distinct parameter sets must be distinct subscriptions: %v", sub1)
	}
	// Re-registering the SAME parameter set reissues the same subscription.
	sub1Again, _, st, _ := registerWith("first")
	if st != 200 || sub1Again != sub1 {
		t.Fatalf("re-register [first] returned %q (want the same subscription %q)", sub1Again, sub1)
	}

	streamSnapRows := func(cur string) []any {
		r, c := liveOpenStream(t, ts.URL, tokAlice, aliceDB.ID, "metricsByNote", cur, 20*time.Second)
		defer c()
		if r.StatusCode != 200 {
			b, _ := io.ReadAll(r.Body)
			t.Fatalf("stream [byNote] status: %d %s", r.StatusCode, b)
		}
		fs := liveReadFrames(t, r.Body, func(f liveFrame) bool { return f.Kind == "snapshot" }, 15*time.Second)
		if len(fs) == 0 {
			t.Fatalf("no snapshot for a parameterized subscription")
		}
		rv, _ := fs[len(fs)-1].Data["rows"].([]any)
		return rv
	}
	rows1 := streamSnapRows(cursor1)
	if len(rows1) != 1 {
		t.Fatalf("subscription [first] snapshot rows: %v", rows1)
	}
	if r0, _ := rows1[0].(map[string]any); r0["name"] != "alpha" {
		t.Fatalf("subscription [first] must stream only the 'first' row: %v", rows1)
	}
	rows2 := streamSnapRows(cursor2)
	if len(rows2) != 1 {
		t.Fatalf("subscription [second] snapshot rows: %v", rows2)
	}
	if r0, _ := rows2[0].(map[string]any); r0["name"] != "beta" {
		t.Fatalf("subscription [second] must stream only the 'second' row: %v", rows2)
	}

	// Both streams open: cancelling the FIRST by its parameter set must
	// terminate only the first stream and leave the second streaming.
	resp1, cancel1 := liveOpenStream(t, ts.URL, tokAlice, aliceDB.ID, "metricsByNote", cursor1, 30*time.Second)
	defer cancel1()
	resp2, cancel2 := liveOpenStream(t, ts.URL, tokAlice, aliceDB.ID, "metricsByNote", cursor2, 30*time.Second)
	defer cancel2()
	if resp1.StatusCode != 200 || resp2.StatusCode != 200 {
		t.Fatalf("parameterized streams: %d %d", resp1.StatusCode, resp2.StatusCode)
	}
	// Drain the snapshot frames both streams deliver first.
	_ = liveReadFrames(t, resp1.Body, func(f liveFrame) bool { return f.Kind == "snapshot" }, 15*time.Second)
	_ = liveReadFrames(t, resp2.Body, func(f liveFrame) bool { return f.Kind == "snapshot" }, 15*time.Second)

	status, body = liveJSON(t, ts.URL, tokAlice, fmt.Sprintf(
		"/api/mirabase/v1/databases/%s/live-queries/metricsByNote", aliceDB.ID), "DELETE",
		map[string]any{"parameters": []any{"first"}})
	if status != 200 {
		t.Fatalf("cancel by parameters [first]: %d %s", status, body)
	}
	var cancelledFirst map[string]any
	_ = json.Unmarshal([]byte(body), &cancelledFirst)
	if cancelledFirst["id"] != sub1 || cancelledFirst["closed"] != true {
		t.Fatalf("cancel by parameters closed the wrong subscription: %v (want %v)", cancelledFirst, sub1)
	}
	if st := liveSubState(t, target, sub1); st != "cancelled" {
		t.Fatalf("subscription [first] durable state after cancel: %q", st)
	}
	if st := liveSubState(t, target, sub2); st != "active" {
		t.Fatalf("subscription [second] must remain active: %q", st)
	}
	// The first stream terminates; the second stays open (keepalives only,
	// never an error frame).
	f1 := liveReadFrames(t, resp1.Body, func(f liveFrame) bool { return f.Kind == "error" }, 15*time.Second)
	if len(f1) == 0 {
		t.Fatalf("the cancelled subscription's stream did not terminate")
	}
	if code, _ := f1[len(f1)-1].Data["code"].(string); code != "subscription_cancelled" {
		t.Fatalf("first stream termination frame: %v", f1[len(f1)-1].Raw)
	}
	quiet := liveReadFrames(t, resp2.Body, nil, 2*time.Second)
	for _, f := range quiet {
		if f.Kind == "error" || f.Kind == "reset" {
			t.Fatalf("the second subscription was disturbed by the first cancel: %v", f.Raw)
		}
	}
	// The second subscription cancels independently by its own parameters.
	status, body = liveJSON(t, ts.URL, tokAlice, fmt.Sprintf(
		"/api/mirabase/v1/databases/%s/live-queries/metricsByNote", aliceDB.ID), "DELETE",
		map[string]any{"parameters": []any{"second"}})
	if status != 200 {
		t.Fatalf("cancel by parameters [second]: %d %s", status, body)
	}
	var cancelledSecond map[string]any
	_ = json.Unmarshal([]byte(body), &cancelledSecond)
	if cancelledSecond["id"] != sub2 || cancelledSecond["closed"] != true {
		t.Fatalf("cancel by parameters [second] closed the wrong subscription: %v (want %v)", cancelledSecond, sub2)
	}
	if st := liveSubState(t, target, sub2); st != "cancelled" {
		t.Fatalf("subscription [second] durable state after its cancel: %q", st)
	}

	// Ambiguity: two ACTIVE subscriptions of the same plan with different
	// parameter sets cannot be cancelled without an identifier.
	sub3, _, st, _ := registerWith("first")
	if st != 200 || sub3 == "" {
		t.Fatalf("register [first] again: %d", st)
	}
	sub4, _, st, _ := registerWith("second")
	if st != 200 || sub4 == "" {
		t.Fatalf("register [second] again: %d", st)
	}
	status, body = liveJSON(t, ts.URL, tokAlice, fmt.Sprintf(
		"/api/mirabase/v1/databases/%s/live-queries/metricsByNote", aliceDB.ID), "DELETE", nil)
	if status != 400 {
		t.Fatalf("ambiguous identifier-less cancel: %d %s (want 400)", status, body)
	}
	if !strings.Contains(body, "different parameters") {
		t.Fatalf("ambiguous cancel message: %s", body)
	}
	// With identifiers each still cancels exactly its own subscription.
	status, body = liveJSON(t, ts.URL, tokAlice, fmt.Sprintf(
		"/api/mirabase/v1/databases/%s/live-queries/metricsByNote", aliceDB.ID), "DELETE",
		map[string]any{"parameters": []any{"first"}})
	if status != 200 {
		t.Fatalf("cancel [first] by parameters after ambiguity: %d %s", status, body)
	}
	var c3 map[string]any
	_ = json.Unmarshal([]byte(body), &c3)
	if c3["id"] != sub3 || c3["closed"] != true {
		t.Fatalf("cancel [first] after ambiguity: %v (want %v)", c3, sub3)
	}
	status, body = liveJSON(t, ts.URL, tokAlice, fmt.Sprintf(
		"/api/mirabase/v1/databases/%s/live-queries/metricsByNote", aliceDB.ID), "DELETE",
		map[string]any{"id": sub4})
	if status != 200 {
		t.Fatalf("cancel [second] by id after ambiguity: %d %s", status, body)
	}
	var c4 map[string]any
	_ = json.Unmarshal([]byte(body), &c4)
	if c4["id"] != sub4 || c4["closed"] != true {
		t.Fatalf("cancel [second] by id after ambiguity: %v (want %v)", c4, sub4)
	}

	// ------------------------------------------------------------------
	// 8. A mistyped parameter is refused BEFORE subscription allocation:
	//    the durable subscription count for the actor+plan never grows.
	// ------------------------------------------------------------------
	before := liveSubCount(t, target, aliceDB.ID, "metricsByNote", actorColID, "aliceliv0000001")
	st, bd = liveJSON(t, ts.URL, tokAlice, fmt.Sprintf(
		"/api/mirabase/v1/databases/%s/live-queries", aliceDB.ID), "POST", map[string]any{
		"queryId":        "metricsByNote",
		"parameters":     []any{42}, // declared "text": an integer value
		"schemaRevision": dbRow.SchemaRevision,
	})
	if st != 400 {
		t.Fatalf("mistyped parameter: %d %s (want 400)", st, bd)
	}
	if !strings.Contains(bd, "invalid_request") {
		t.Fatalf("mistyped parameter code: %s", bd)
	}
	after := liveSubCount(t, target, aliceDB.ID, "metricsByNote", actorColID, "aliceliv0000001")
	if after != before {
		t.Fatalf("the refused register allocated a subscription: %d → %d", before, after)
	}

	// ------------------------------------------------------------------
	// 9. A non-deterministic plan is refused at creation: random() and
	//    the current-time forms never publish (their keyed diff would
	//    flap per evaluation).
	// ------------------------------------------------------------------
	_, ndErr := rt.Service.CreateLivePlan(ctx, app, &application.LivePlanWire{
		ID: "metricsRandom", SQL: `SELECT id, random() AS seed FROM metrics`,
		ResultKey: "id", MaxRows: 100, MaxBytes: 1048576,
		Dependencies: []string{coll.Name},
	})
	if ndErr == nil || !strings.Contains(ndErr.Error(), "deterministic") {
		t.Fatalf("random() plan must be refused as non-deterministic: %v", ndErr)
	}
	_, ndErr = rt.Service.CreateLivePlan(ctx, app, &application.LivePlanWire{
		ID: "metricsNow", SQL: `SELECT id, strftime('%s','now') AS ts FROM metrics`,
		ResultKey: "id", MaxRows: 100, MaxBytes: 1048576,
		Dependencies: []string{coll.Name},
	})
	if ndErr == nil || !strings.Contains(ndErr.Error(), "deterministic") {
		t.Fatalf("strftime('now') plan must be refused as non-deterministic: %v", ndErr)
	}

	// A plan selecting a hidden field of its dependency collection is
	// refused at creation (the field policy).
	_, hfErr := rt.Service.CreateLivePlan(ctx, app, &application.LivePlanWire{
		ID: "metricsCode", SQL: `SELECT id, code FROM metrics`,
		ResultKey: "id", MaxRows: 100, MaxBytes: 1048576,
		Dependencies: []string{coll.Name},
	})
	if hfErr == nil || !strings.Contains(hfErr.Error(), "hidden") {
		t.Fatalf("hidden-field plan must be refused: %v", hfErr)
	}

	// ------------------------------------------------------------------
	// 10. A plan whose dependency collection grants no list rule is
	//     forbidden for an ordinary actor — fail closed BEFORE allocation
	//     (a privileged SQL plan cannot be exposed as an ordinary record
	//     subscription).
	// ------------------------------------------------------------------
	vault := core.NewBaseCollection("vault")
	vault.Fields.Add(&core.TextField{Name: "label"})
	// ListRule stays nil: superuser-only under the ordinary record rules.
	if err := app.Save(vault); err != nil {
		t.Fatalf("vault collection: %v", err)
	}
	vaultPlan, err := rt.Service.CreateLivePlan(ctx, app, &application.LivePlanWire{
		ID: "vaultAll", SQL: `SELECT id, label FROM vault`,
		ResultKey: "id", MaxRows: 100, MaxBytes: 1048576,
		Dependencies: []string{vault.Name},
	})
	if err != nil {
		t.Fatalf("create vault plan: %v", err)
	}
	if _, err := rt.PublishLivePlan(ctx, vaultPlan.ID, vaultPlan.Revision); err != nil {
		t.Fatalf("publish vault plan: %v", err)
	}
	before = liveSubCount(t, target, aliceDB.ID, "vaultAll", actorColID, "aliceliv0000001")
	st, bd = liveJSON(t, ts.URL, tokAlice, fmt.Sprintf(
		"/api/mirabase/v1/databases/%s/live-queries", aliceDB.ID), "POST", map[string]any{
		"queryId":        "vaultAll",
		"parameters":     []any{},
		"schemaRevision": dbRow.SchemaRevision,
	})
	if st != 403 {
		t.Fatalf("nil-list-rule plan for an ordinary actor: %d %s (want 403)", st, bd)
	}
	after = liveSubCount(t, target, aliceDB.ID, "vaultAll", actorColID, "aliceliv0000001")
	if after != before || after != 0 {
		t.Fatalf("the forbidden register allocated a subscription: %d → %d", before, after)
	}
}

// ---------------------------------------------------------------------------
// Acceptance helpers.
// ---------------------------------------------------------------------------

// liveTestApp builds the test application from the fixture data dir with
// the same recipe as tests.NewTestApp (the mirabase/tests package imports
// apis, so a package-apis test cannot import it).
func liveTestApp(t *testing.T) *core.BaseApp {
	t.Helper()
	_, thisFile, _, _ := runtime.Caller(0)
	fixture := filepath.Join(filepath.Dir(thisFile), "..", "tests", "data")
	tempDir, err := os.MkdirTemp("", "live_test_*")
	if err != nil {
		t.Fatalf("temp dir: %v", err)
	}
	if err := os.CopyFS(tempDir, os.DirFS(fixture)); err != nil {
		_ = os.RemoveAll(tempDir)
		t.Fatalf("fixture clone: %v", err)
	}
	app := core.NewBaseApp(core.BaseAppConfig{
		DataDir:       tempDir,
		EncryptionEnv: "pb_test_env",
		DBConnect:     core.TestDBConnect(),
	})
	if err := app.Bootstrap(); err != nil {
		_ = os.RemoveAll(tempDir)
		t.Fatalf("bootstrap: %v", err)
	}
	if _, err := app.DB().NewQuery("Select 1").Execute(); err != nil {
		_ = os.RemoveAll(tempDir)
		t.Fatalf("db: %v", err)
	}
	if err := app.RunAllMigrations(); err != nil {
		_ = os.RemoveAll(tempDir)
		t.Fatalf("migrations: %v", err)
	}
	app.Settings().Logs.MaxDays = 0
	return app
}

// liveAcceptanceRouter builds the test router with the live surface (the
// wiring step binds bindMirabaseLiveApi into the shared surface later).
func liveAcceptanceRouter(t *testing.T, app *core.BaseApp) (*httptest.Server, func()) {
	t.Helper()
	pbRouter := router.NewRouter(func(w http.ResponseWriter, r *http.Request) (*core.RequestEvent, router.EventCleanupFunc) {
		event := new(core.RequestEvent)
		event.Response = w
		event.Request = r
		event.App = app
		return event, nil
	})
	pbRouter.Bind(loadAuthToken())
	apiGroup := pbRouter.Group("/api")
	v1 := apiGroup.Group("/mirabase/v1")
	bindMirabaseLiveApi(app, v1)
	bindMirabaseRecordsApi(app, v1)
	mux, err := pbRouter.BuildMux()
	if err != nil {
		t.Fatalf("build mux: %v", err)
	}
	ts := httptest.NewServer(mux)
	return ts, ts.Close
}

// liveRecordToken mints a record auth token for the fixture identity
// (creating the record when missing).
func liveRecordToken(t *testing.T, app *core.BaseApp, collectionName, recordID string) string {
	t.Helper()
	coll, err := app.FindCollectionByNameOrId(collectionName)
	if err != nil {
		t.Fatalf("collection %s: %v", collectionName, err)
	}
	record, err := app.FindRecordById(coll.Name, recordID)
	if err != nil {
		record = core.NewRecord(coll)
		record.Set("email", recordID+"@live.example.com")
		record.Set("password", "livepass12345678")
		record.Id = recordID
		if err := app.Save(record); err != nil {
			t.Fatalf("record %s: %v", recordID, err)
		}
	}
	token, err := record.NewAuthToken()
	if err != nil {
		t.Fatalf("token %s: %v", recordID, err)
	}
	return token
}

// liveSuperuserToken mints a superuser auth token (the superuser's live
// registration is the unfiltered control for the injected ordinary
// evaluation).
func liveSuperuserToken(t *testing.T, app *core.BaseApp) string {
	t.Helper()
	superusers, err := app.FindCollectionByNameOrId(core.CollectionNameSuperusers)
	if err != nil {
		t.Fatalf("superusers collection: %v", err)
	}
	record, err := app.FindAuthRecordByEmail(core.CollectionNameSuperusers, "live-admin@example.com")
	if err != nil {
		record = core.NewRecord(superusers)
		record.Set("email", "live-admin@example.com")
		record.Set("password", "liveadminpass123")
		if err := app.Save(record); err != nil {
			t.Fatalf("superuser save: %v", err)
		}
	}
	token, err := record.NewAuthToken()
	if err != nil {
		t.Fatalf("superuser token: %v", err)
	}
	return token
}

// liveSubState reads one subscription's durable state directly from the
// target's subscription store.
func liveSubState(t *testing.T, target *sql.DB, id string) string {
	t.Helper()
	var state string
	if err := target.QueryRow(
		`SELECT state FROM _mb_live_subscriptions WHERE id = ?`, id).Scan(&state); err != nil {
		t.Fatalf("subscription row %s: %v", id, err)
	}
	return state
}

// liveSubCount counts the actor's durable subscriptions of one plan (any
// state) — the pre/post-allocation witness of a refused register.
func liveSubCount(t *testing.T, target *sql.DB, databaseID, planID, actorCol, actorRec string) int {
	t.Helper()
	var n int
	if err := target.QueryRow(
		`SELECT COUNT(*) FROM _mb_live_subscriptions
		 WHERE database_id = ? AND plan_id = ? AND actor_collection_id = ? AND actor_record_id = ?`,
		databaseID, planID, actorCol, actorRec).Scan(&n); err != nil {
		t.Fatalf("subscription count: %v", err)
	}
	return n
}

// liveJSON performs a JSON request against the v1 surface.
func liveJSON(t *testing.T, baseURL, token, path, method string, payload any) (int, string) {
	t.Helper()
	var reader io.Reader
	if payload != nil {
		b, _ := json.Marshal(payload)
		reader = strings.NewReader(string(b))
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
		t.Fatalf("do %s %s: %v", method, path, err)
	}
	defer resp.Body.Close()
	var buf strings.Builder
	_, _ = io.Copy(&buf, resp.Body)
	return resp.StatusCode, buf.String()
}

// liveFrame is one parsed SSE frame.
type liveFrame struct {
	Kind string
	Data map[string]any
	Raw  string
}

// liveOpenStream opens the SSE stream with a bounded context.
func liveOpenStream(t *testing.T, baseURL, token, databaseID, queryID, cursor string, timeout time.Duration) (*http.Response, context.CancelFunc) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	req, err := http.NewRequestWithContext(ctx, "GET",
		fmt.Sprintf("%s/api/mirabase/v1/databases/%s/live-queries/%s/stream?cursor=%s", baseURL, databaseID, queryID, cursor), nil)
	if err != nil {
		cancel()
		t.Fatalf("stream request: %v", err)
	}
	req.Header.Set("Authorization", token)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		cancel()
		t.Fatalf("stream open: %v", err)
	}
	return resp, cancel
}

// liveReadFrames reads SSE frames until until(frame) matches, the body
// ends or the deadline passes. Comment keepalive lines are skipped.
func liveReadFrames(t *testing.T, body io.Reader, until func(f liveFrame) bool, wait time.Duration) []liveFrame {
	t.Helper()
	deadline := time.Now().Add(wait)
	br := bufio.NewReader(body)
	var frames []liveFrame
	var kind, data string
	flush := func() {
		if kind == "" && data == "" {
			return
		}
		f := liveFrame{Kind: kind, Raw: data}
		if data != "" {
			_ = json.Unmarshal([]byte(data), &f.Data)
		}
		frames = append(frames, f)
		kind, data = "", ""
	}
	for time.Now().Before(deadline) {
		line, err := br.ReadString('\n')
		if err != nil {
			flush()
			return frames
		}
		line = strings.TrimRight(line, "\r\n")
		switch {
		case line == "":
			flush()
			if until != nil && len(frames) > 0 && until(frames[len(frames)-1]) {
				return frames
			}
		case strings.HasPrefix(line, ":"):
			// keepalive comment
		case strings.HasPrefix(line, "event: "):
			kind = strings.TrimPrefix(line, "event: ")
		case strings.HasPrefix(line, "data: "):
			data += strings.TrimPrefix(line, "data: ")
		}
	}
	flush()
	return frames
}

// livePatchOps extracts the delta frame's ordered patch operations.
func livePatchOps(f liveFrame) []map[string]any {
	raw, _ := f.Data["patch"].([]any)
	ops := make([]map[string]any, 0, len(raw))
	for _, o := range raw {
		if m, ok := o.(map[string]any); ok {
			ops = append(ops, m)
		}
	}
	return ops
}
