package application_test

import (
	"context"
	"strings"
	"testing"

	"mirabase/internal/application"
	"mirabase/internal/mbengine"
	"mirabase/tests"
)

func TestAgentSnapshotHistoricalBytesAndIntegrity(t *testing.T) {
	if mbengine.PinnedDigest() == "" {
		t.Skip("native payload pin required; qualification runs must report zero skipped tests")
	}
	if err := mbengine.Ensure(); err != nil {
		t.Fatal(err)
	}
	app, err := tests.NewTestApp()
	if err != nil {
		t.Fatal(err)
	}
	defer app.Cleanup()
	rt, err := application.BindRuntime(app.BaseApp)
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	w, err := rt.AgentFSCreateWorkspace(ctx, "users", "snapshot-owner", "snapshot regression")
	if err != nil {
		t.Fatal(err)
	}
	write := func(path, value string) {
		t.Helper()
		if _, err := rt.AgentFSWrite(ctx, w.ID, path, []byte(value)); err != nil {
			t.Fatal(err)
		}
	}
	assertBytes := func(path, expected string) {
		t.Helper()
		value, _, err := rt.AgentFSRead(ctx, w.ID, path)
		if err != nil || string(value) != expected {
			t.Fatalf("%s: got %q, %v; want %q", path, value, err, expected)
		}
	}
	write("first.txt", "original first")
	write("second.txt", "original second")
	snapshot, err := rt.AgentFSSnapshot(ctx, w.ID)
	if err != nil {
		t.Fatal(err)
	}
	id := snapshot["snapshotId"].(string)
	write("first.txt", "overwritten")
	if err := rt.AgentFSDelete(ctx, w.ID, "second.txt"); err != nil {
		t.Fatal(err)
	}
	write("extra.txt", "preserve overlay")
	if _, err := rt.AgentFSRestoreSnapshot(ctx, w.ID, id); err != nil {
		t.Fatal(err)
	}
	assertBytes("first.txt", "original first")
	assertBytes("second.txt", "original second")
	assertBytes("extra.txt", "preserve overlay")

	// A corrupted later entry must be rejected before any earlier file changes.
	write("first.txt", "keep on refusal")
	if _, err := rt.PrimaryDB.ExecContext(ctx, `UPDATE _mb_agentfs_snapshot_bytes SET content=? WHERE snapshot_id=? AND path=?`, []byte("corrupt"), id, "second.txt"); err != nil {
		t.Fatal(err)
	}
	if _, err := rt.AgentFSRestoreSnapshot(ctx, w.ID, id); err == nil || !strings.Contains(err.Error(), "hash mismatch") {
		t.Fatalf("corrupt snapshot not refused: %v", err)
	}
	assertBytes("first.txt", "keep on refusal")
	if _, err := rt.PrimaryDB.ExecContext(ctx, `DELETE FROM _mb_agentfs_snapshot_bytes WHERE snapshot_id=?`, id); err != nil {
		t.Fatal(err)
	}
	if _, err := rt.AgentFSRestoreSnapshot(ctx, w.ID, id); err == nil || !strings.Contains(err.Error(), "bytes unavailable") {
		t.Fatalf("missing historical bytes not refused: %v", err)
	}
	assertBytes("first.txt", "keep on refusal")

	if _, err := rt.AgentFSWrite(ctx, w.ID, "../escape", []byte("bad")); err == nil {
		t.Fatal("path traversal accepted")
	}
	if _, err := rt.PrimaryDB.ExecContext(ctx, `UPDATE _mb_agentfs_files SET size_bytes=? WHERE workspace_id=? AND path=?`, 64<<20+1, w.ID, "first.txt"); err != nil {
		t.Fatal(err)
	}
	if _, err := rt.AgentFSSnapshot(ctx, w.ID); err == nil || !strings.Contains(err.Error(), "resource_exhausted") {
		t.Fatalf("oversized snapshot not refused: %v", err)
	}
}
