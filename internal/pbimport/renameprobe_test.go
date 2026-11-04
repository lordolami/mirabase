package pbimport

import (
	"database/sql"
	"os"
	"path/filepath"
	"testing"

	_ "turso.tech/database/tursogo" // registers the "turso" driver
)

// TestStagedDirEngineLock documents the Windows platform reality the
// cutover protocol adapts to: the pinned engine holds a native directory
// lock on an opened database root that survives sql.DB.Close — a
// directory rename after close is denied indefinitely. The cutover
// protocol therefore publishes with copy+verify and RETAINS the staged
// dir as the session's recovery copy (durability unchanged: journal,
// verified bytes at the target, active marker, old root and source
// retained). This test verifies the copy+retain alternative produces a
// working published target.
func TestStagedDirEngineLock(t *testing.T) {
	requireEngine(t)
	dir := t.TempDir()
	staged := filepath.Join(dir, "staged")
	os.MkdirAll(staged, 0o700)
	db, err := sql.Open("turso", staged+`\data.db?_busy_timeout=10000&experimental=vacuum,index_method`)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`CREATE TABLE x (id INTEGER PRIMARY KEY)`); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`PRAGMA wal_checkpoint(TRUNCATE)`); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Conn(t.Context()); err != nil {
		t.Fatal(err)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	// The platform reality: rename denied after close.
	target := filepath.Join(dir, "target")
	if err := os.Rename(staged, target); err == nil {
		t.Log("note: rename unexpectedly allowed on this host; the copy+verify path still applies")
		return
	}
	// The adaptation: copy the staged content and verify the published
	// bytes.
	if err := copyTree(staged, target); err != nil {
		t.Fatalf("copy: %v", err)
	}
	m1, err := manifestOf(staged)
	if err != nil {
		t.Fatal(err)
	}
	m2, err := manifestOf(target)
	if err != nil {
		t.Fatal(err)
	}
	if m1 != m2 {
		t.Fatalf("published bytes do not match the staged manifest")
	}
	// The published target opens through the engine.
	check, err := sql.Open("turso", target+`\data.db?_busy_timeout=10000`)
	if err != nil {
		t.Fatal(err)
	}
	defer check.Close()
	var n int
	if err := check.QueryRow(`SELECT count(*) FROM x`).Scan(&n); err != nil {
		t.Fatalf("published target query: %v", err)
	}
}
