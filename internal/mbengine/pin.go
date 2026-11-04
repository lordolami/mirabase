// Package mbengine owns the native engine payload trust boundary:
// link-time digest pinning, verified extraction, loading, ABI and capability
// probes — all fatal-before-application-files per docs/mirabase/
// native-provider.md.
package mbengine

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"sync"

	turso_libs "github.com/tursodatabase/turso-go-platform-libs"
)

// pinnedLibraryDigest is set at link time with
//
//	-ldflags "-X mirabase/internal/mbengine.pinnedLibraryDigest=<sha256>"
//
// and is the sole authority for the native payload. An unset or malformed
// pin is fatal (native-provider.md). The loader module receives the same
// value via its own -X flag; Verify cross-checks the two at startup so a
// build with mismatched flags cannot pass.
var pinnedLibraryDigest = ""

// ErrDigestPin reports an unset or malformed link-time digest pin.
var ErrDigestPin = errors.New("native payload digest pin is unset or malformed")

// ErrDigestCrossCheck reports a mismatch between the two link-time digest
// flags (mbengine pin vs loader pin), which indicates a build misuse.
var ErrDigestCrossCheck = errors.New("native payload digest flags disagree between mbengine and the loader")

// ErrDigestMismatch reports a digest mismatch of the loaded library against
// the pinned value.
var ErrDigestMismatch = errors.New("native payload digest mismatch")

// EngineRevision is the pinned upstream engine commit this payload was
// produced from (build-start.md; source-reference.md TURSO rows).
const EngineRevision = "b0e1b8fde8eff66ef9843fdd1139fa3305cb16dd"

func isDigest(s string) bool {
	if len(s) != 64 {
		return false
	}
	_, err := hex.DecodeString(s)
	return err == nil
}

// PinnedDigest returns the link-time pinned payload digest.
func PinnedDigest() string { return pinnedLibraryDigest }

var verifyOnce sync.Once
var verifyErr error

// Verify checks, exactly once, that:
//  1. the mbengine digest pin is set and well-formed (fatal otherwise),
//  2. the loader's own link-time digest flag equals the pin,
//  3. the loaded library file (absolute path recorded by the loader) hashes
//     to the pinned value — an independent re-verification of the exact
//     bytes on disk with the mbengine-side flag value.
func Verify() error {
	verifyOnce.Do(func() {
		if !isDigest(pinnedLibraryDigest) {
			verifyErr = ErrDigestPin
			return
		}
		if pinnedLibraryDigest != turso_libs.ExpectedDigest() {
			verifyErr = fmt.Errorf("%w: mbengine=%s loader=%s", ErrDigestCrossCheck, pinnedLibraryDigest, turso_libs.ExpectedDigest())
			return
		}
		path := turso_libs.LoadedLibraryPath()
		if path == "" {
			verifyErr = errors.New("no library has been loaded yet; call Ensure before Verify")
			return
		}
		st, err := os.Lstat(path)
		if err != nil {
			verifyErr = fmt.Errorf("loaded library stat failed: %w", err)
			return
		}
		if !st.Mode().IsRegular() {
			verifyErr = fmt.Errorf("%w: loaded path %q is not a regular file", ErrDigestMismatch, path)
			return
		}
		f, err := os.Open(path)
		if err != nil {
			verifyErr = fmt.Errorf("loaded library open failed: %w", err)
			return
		}
		defer f.Close()
		h := sha256.New()
		if _, err := copyInto(h, f); err != nil {
			verifyErr = fmt.Errorf("loaded library hash failed: %w", err)
			return
		}
		got := hex.EncodeToString(h.Sum(nil))
		if got != pinnedLibraryDigest {
			verifyErr = fmt.Errorf("%w: expected=%s got=%s path=%s", ErrDigestMismatch, pinnedLibraryDigest, got, path)
			return
		}
	})
	return verifyErr
}
