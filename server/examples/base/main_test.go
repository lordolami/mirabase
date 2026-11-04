package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDefaultPublicDirForArchiveExtractedUnderTemp(t *testing.T) {
	root := t.TempDir()
	previousArgs := os.Args
	os.Args = []string{filepath.Join(root, "mirabase.exe")}
	t.Cleanup(func() { os.Args = previousArgs })
	// The development-run fallback remains available without bundled assets.
	if got := defaultPublicDir(); got != "./mb_public" {
		t.Fatalf("development fallback: got %q", got)
	}
	publicDir := filepath.Join(root, "mb_public")
	if err := os.Mkdir(publicDir, 0700); err != nil {
		t.Fatal(err)
	}
	if got := defaultPublicDir(); got != publicDir {
		t.Fatalf("packaged assets under temp: got %q, want %q", got, publicDir)
	}
}
