package turso_libs

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestLoadTursoLibraryVerifiesAndExtracts runs the full loader flow against
// the real embedded payload: unset pin -> refuse; malformed pin -> refuse;
// valid pin -> extract to the private cache, verify, load.
func TestLoadTursoLibraryVerifiesAndExtracts(t *testing.T) {
	t.Setenv("MIRABASE_NATIVE_CACHE_DIR", filepath.Join(t.TempDir(), "cache"))

	// Compute the real digest of the embedded payload for this platform.
	payload, err := payloadBytes()
	if err != nil {
		if strings.Contains(err.Error(), "no payload embedded") {
			t.Skip("no payload embedded for this platform")
		}
		t.Fatalf("payloadBytes: %v", err)
	}
	sum := sha256.Sum256(payload)
	digest := hex.EncodeToString(sum[:])

	savedDigest := expectedDigest
	defer func() {
		expectedDigest = savedDigest
		releaseLoaded()
	}()

	// Unset pin is fatal.
	expectedDigest = ""
	if _, err := LoadTursoLibrary(LoadTursoLibraryConfig{}); err != ErrPayloadDigest {
		t.Fatalf("unset pin: want ErrPayloadDigest, got %v", err)
	}

	// Malformed pin is fatal.
	expectedDigest = "deadbeef"
	if _, err := LoadTursoLibrary(LoadTursoLibraryConfig{}); err != ErrPayloadDigest {
		t.Fatalf("malformed pin: want ErrPayloadDigest, got %v", err)
	}

	// Mismatched pin is fatal (tamper negative): the embedded bytes do not
	// match the pin.
	expectedDigest = "1111111111111111111111111111111111111111111111111111111111111111"
	if _, err := LoadTursoLibrary(LoadTursoLibraryConfig{}); err == nil {
		t.Fatalf("mismatched pin: want an error, got nil")
	} else if !strings.Contains(err.Error(), "digest mismatch") {
		t.Fatalf("mismatched pin: want digest mismatch, got %v", err)
	}

	// Correct pin: extraction + verification + load.
	expectedDigest = digest
	path, err := loadForTest()
	if err != nil {
		t.Fatalf("LoadTursoLibrary: %v", err)
	}
	if path == "" {
		t.Fatalf("LoadTursoLibrary returned an empty path")
	}
	if !strings.Contains(path, "mirabase") && !strings.Contains(path, LibraryFileName()) {
		t.Fatalf("unexpected cache path %q", path)
	}
	if LoadedLibraryPath() != path {
		t.Fatalf("LoadedLibraryPath not recorded")
	}

	// The extracted file hashes to the pin.
	onDisk, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read extracted payload: %v", err)
	}
	got := sha256.Sum256(onDisk)
	if hex.EncodeToString(got[:]) != digest {
		t.Fatalf("extracted payload digest mismatch")
	}

	// Release the loaded handle before tampering the cache file (a loaded
	// DLL is locked on Windows).
	releaseLoaded()

	// A tampered cache copy cannot authorize the payload: the loader
	// re-verifies and self-heals from the embedded (re-verified) bytes.
	tampered := append([]byte(nil), onDisk...)
	tampered[len(tampered)/2] ^= 0xFF
	if err := os.WriteFile(path, tampered, 0o600); err != nil {
		t.Fatalf("tamper cache: %v", err)
	}
	path2, err := loadForTest()
	if err != nil {
		t.Fatalf("reload after cache tamper: %v", err)
	}
	repaired, err := os.ReadFile(path2)
	if err != nil {
		t.Fatalf("read repaired payload: %v", err)
	}
	got2 := sha256.Sum256(repaired)
	if hex.EncodeToString(got2[:]) != digest {
		t.Fatalf("cache tamper was not healed from the embedded authority")
	}

	// Unsupported strategies are rejected (no search-path loading).
	for _, strategy := range []LibraryLoadStrategy{SystemLibraryLoadStrategy, MixedLibraryLoadStrategy, "banana"} {
		if _, err := LoadTursoLibrary(LoadTursoLibraryConfig{LoadStrategy: strategy}); err == nil {
			t.Fatalf("strategy %q: want an error, got nil", strategy)
		}
	}
}

// loadForTest calls LoadTursoLibrary, records the handle for cleanup, and
// returns the loaded path.
func loadForTest() (string, error) {
	before := LoadedLibraryPath()
	handle, err := LoadTursoLibrary(LoadTursoLibraryConfig{})
	if err != nil {
		return before, err
	}
	loadedHandles = append(loadedHandles, handle)
	return LoadedLibraryPath(), nil
}

var loadedHandles []uintptr

// releaseLoaded frees handles opened during the test so Windows can delete
// the payload files again.
func releaseLoaded() {
	for _, h := range loadedHandles {
		_ = freeLibrary(h)
	}
	loadedHandles = nil
}
