package pbimport

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"mirabase/internal/mbengine"
	_ "turso.tech/database/tursogo" // registers the "turso" driver
)

// requireEngine skips when the pinned engine payload is not compiled in
// (plain builds; the parity harness passes the digest pin), mirroring
// requireEngine in internal/application and internal/dbexec. The "turso"
// driver cannot open any database without the pinned payload: its first
// Open resolves the link-time digest pin and panics on an unset pin.
func requireEngine(t *testing.T) {
	t.Helper()
	if mbengine.PinnedDigest() == "" {
		t.Skip("requires the pinned engine payload (run via _build/p02/parity_turso.sh)")
	}
	if err := mbengine.Ensure(); err != nil {
		t.Skipf("engine ensure failed: %v", err)
	}
}

func TestParseRemoteManifest(t *testing.T) {
	raw := []byte(`{"entries":[
		{"key":"posts/abc/file.txt","bucket":"pb-src","size":5,"sha256":"` + strings.Repeat("a", 64) + `","collectionId":"pbc1","recordId":"r1","filename":"file.txt"},
		{"key":"posts/def/img.png","bucket":"pb-src","size":0,"sha256":"` + strings.Repeat("b", 64) + `","collectionId":"pbc1","recordId":"r2","filename":"img.png"}
	]}`)
	m, err := ParseRemoteManifest(raw)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if len(m.Entries) != 2 {
		t.Fatalf("entries: %d", len(m.Entries))
	}
	// Duplicated key rejected.
	dup := []byte(`{"entries":[
		{"key":"a","bucket":"b","size":0,"sha256":"` + strings.Repeat("a", 64) + `"},
		{"key":"a","bucket":"b","size":0,"sha256":"` + strings.Repeat("b", 64) + `"}
	]}`)
	if _, err := ParseRemoteManifest(dup); err == nil {
		t.Fatalf("duplicate key accepted")
	}
	// Bad sha rejected.
	bad := []byte(`{"entries":[{"key":"a","bucket":"b","size":0,"sha256":"xyz"}]}`)
	if _, err := ParseRemoteManifest(bad); err == nil {
		t.Fatalf("invalid sha accepted")
	}
}

// TestVerifyRemoteCoverage exercises the adapter end to end: a manifest
// with an accompanying content export verifies the present object and
// flags the missing one (never a deceptively complete import).
func TestVerifyRemoteCoverage(t *testing.T) {
	requireEngine(t)
	dir := t.TempDir()
	db, err := sql.Open("turso", filepath.Join(dir, "primary.db")+"?_busy_timeout=10000")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	svc, err := NewService(db, dir)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Create(context.Background(), &CreateRequest{
		SourceKind: "pocketbase-directory", SourceBytes: 1,
		SourceSha256: strings.Repeat("0", 64), TargetMode: "new-application",
		ExpectedPrimaryGeneration: "1",
	}); err != nil {
		t.Fatalf("create: %v", err)
	}
	// The declared sessions cap allows one more; find the created session id.
	var id string
	if err := db.QueryRow(`SELECT id FROM _mb_import_sessions LIMIT 1`).Scan(&id); err != nil {
		t.Fatal(err)
	}

	// The manifest: two objects, one present in the export, one missing.
	content := "hello"
	h := sha256.Sum256([]byte(content))
	manifest := []byte(`{"entries":[
		{"key":"posts/r1/file.txt","bucket":"pb-src","size":5,"sha256":"` + hex.EncodeToString(h[:]) + `","collectionId":"pbc1","recordId":"r1","filename":"file.txt"},
		{"key":"posts/r2/absent.bin","bucket":"pb-src","size":100,"sha256":"` + strings.Repeat("f", 64) + `","collectionId":"pbc1","recordId":"r2","filename":"absent.bin"}
	]}`)
	// The manifest lives in the session's work dir (the analyze location).
	work := filepath.Join(svc.SessionDir(id), "work")
	if err := os.MkdirAll(work, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(work, "remote-manifest.json"), manifest, 0o600); err != nil {
		t.Fatal(err)
	}

	// No export yet: every declared object is missing — not a complete
	// import.
	cov, err := svc.VerifyRemoteCoverage(context.Background(), id)
	if err != nil {
		t.Fatalf("coverage (no export): %v", err)
	}
	if cov.Declared != 2 || cov.Missing != 2 || cov.Verified != 0 {
		t.Fatalf("coverage without export: %+v", cov)
	}

	// Provide the content export with ONE of the two objects.
	exportDir := filepath.Join(work, "remote-export")
	if err := os.MkdirAll(filepath.Join(exportDir, "posts", "r1"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(exportDir, "posts", "r1", "file.txt"), []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	cov, err = svc.VerifyRemoteCoverage(context.Background(), id)
	if err != nil {
		t.Fatalf("coverage (with export): %v", err)
	}
	if cov.Verified != 1 || cov.Missing != 1 {
		t.Fatalf("coverage with export: verified=%d missing=%d", cov.Verified, cov.Missing)
	}
	if len(cov.MissingKeys) != 1 || cov.MissingKeys[0] != "posts/r2/absent.bin" {
		t.Fatalf("missing keys: %v", cov.MissingKeys)
	}
	// The required bytes count covers BOTH declared objects (the missing
	// one still counts as required).
	if cov.RequiredBytes != 105 {
		t.Fatalf("requiredBytes: %d", cov.RequiredBytes)
	}

	// Tampered content (right size, wrong hash) is missing, not verified.
	if err := os.WriteFile(filepath.Join(exportDir, "posts", "r1", "file.txt"), []byte("hellO"), 0o600); err != nil {
		t.Fatal(err)
	}
	cov, err = svc.VerifyRemoteCoverage(context.Background(), id)
	if err != nil {
		t.Fatal(err)
	}
	if cov.Verified != 0 || cov.Missing != 2 {
		t.Fatalf("tampered content not flagged: %+v", cov)
	}
}
