// Package turso_libs is the MiraBase-owned replacement for the upstream
// turso-go-platform-libs loader. It keeps the exact public API consumed by
// the pristine upstream Go driver (package name, LoadTursoLibraryConfig,
// strategy constants, LibraryName) while implementing the MiraBase payload
// trust contract:
//
//   - the native payload is embedded in the executable at build time and its
//     expected sha256 is pinned at link time (github.com/tursodatabase/
//     turso-go-platform-libs.expectedDigest, set by the build from the same
//     value as mirabase/internal/mbengine.pinnedLibraryDigest; the mbengine
//     package cross-checks the two at startup),
//   - extraction goes to a private, versioned cache with restrictive
//     permissions and an atomic rename; no writable search-path lookup,
//   - only the absolute verified path is resolved; symlink/reparse
//     substitution is rejected; loading from the working directory or PATH
//     is not implemented (the "system"/"mixed" strategies are unsupported),
//   - a checksum file next to the library is never an authority; only the
//     link-time digest decides.
package turso_libs

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
)

// LibraryName mirrors the upstream loader's constant.
const LibraryName = "libturso_sync_sdk_kit"

// LibraryFileName returns the on-disk name of the MiraBase payload
// library for the current platform (.dll on Windows, .so elsewhere).
func LibraryFileName() string {
	if runtime.GOOS == "windows" {
		return "mirabase_engine.dll"
	}
	return "mirabase_engine.so"
}

// expectedDigest is set at link time:
//
//	-ldflags "-X github.com/tursodatabase/turso-go-platform-libs.expectedDigest=<sha256>"
//
// An unset or malformed pin is fatal: LoadTursoLibrary refuses to extract
// or load anything.
var expectedDigest = ""

// ExpectedDigest returns the link-time pinned payload digest.
func ExpectedDigest() string { return expectedDigest }

// loadedPath records the absolute verified path of the loaded library for
// independent re-verification by the mbengine package.
var loadedPath string

// LoadedLibraryPath returns the absolute path of the loaded payload, if any.
func LoadedLibraryPath() string { return loadedPath }

type LibraryLoadStrategy string

const (
	// EmbeddedLibraryLoadStrategy (the default): extract the embedded
	// payload to the private versioned cache after verifying its digest and
	// load from the resulting absolute path only.
	EmbeddedLibraryLoadStrategy LibraryLoadStrategy = "embedded"
	// SystemLibraryLoadStrategy is intentionally unsupported: MiraBase never
	// loads a native library from PATH / LD_LIBRARY_PATH / DYLD_LIBRARY_PATH
	// or the working directory.
	SystemLibraryLoadStrategy LibraryLoadStrategy = "system"
	// MixedLibraryLoadStrategy is intentionally unsupported for the same
	// reason.
	MixedLibraryLoadStrategy LibraryLoadStrategy = "mixed"
)

// LoadTursoLibraryConfig keeps the upstream field name and semantics that
// the pristine driver relies on. Only the embedded strategy is implemented.
type LoadTursoLibraryConfig struct {
	// LoadStrategy accepts "embedded" or "" (default). "system" and "mixed"
	// are rejected with an explicit error (no search-path loading).
	LoadStrategy LibraryLoadStrategy
}

// ErrPayloadDigest reports an unset or malformed link-time digest pin.
var ErrPayloadDigest = errors.New("native payload digest pin is unset or malformed")

// ErrTampered reports a digest mismatch between the pinned value and the
// actual payload bytes (embedded or extracted cache copy).
var ErrTampered = errors.New("native payload digest mismatch")

// ErrUnsupportedStrategy reports the intentionally unimplemented strategies.
var ErrUnsupportedStrategy = errors.New("library load strategy is not supported by the MiraBase loader (no search-path loading)")

// LoadTursoLibrary verifies, extracts (when needed) and loads the embedded
// native payload, returning the OS library handle.
func LoadTursoLibrary(config LoadTursoLibraryConfig) (handle uintptr, err error) {
	switch config.LoadStrategy {
	case SystemLibraryLoadStrategy, MixedLibraryLoadStrategy:
		return 0, ErrUnsupportedStrategy
	case EmbeddedLibraryLoadStrategy, "":
		// fall through to the embedded flow below
	default:
		return 0, fmt.Errorf("%w: unknown load strategy %q", ErrUnsupportedStrategy, config.LoadStrategy)
	}

	digest, err := parseDigest(expectedDigest)
	if err != nil {
		return 0, err
	}

	path, err := embeddedLibraryTryCreate(digest)
	if err != nil {
		return 0, fmt.Errorf("failed to prepare the native payload: %w", err)
	}
	if path == "" {
		return 0, errors.New("native payload is not embedded for this platform in the executable")
	}

	handle, err = loadLibrary(path)
	if err != nil {
		return 0, fmt.Errorf("failed to load library from path %v: %w", path, err)
	}
	loadedPath = path
	return handle, nil
}

func parseDigest(d string) (string, error) {
	if len(d) != 64 {
		return "", ErrPayloadDigest
	}
	if _, err := hex.DecodeString(d); err != nil {
		return "", ErrPayloadDigest
	}
	return d, nil
}

// payloadBytes returns the embedded payload for the current platform.
func payloadBytes() ([]byte, error) {
	name := fmt.Sprintf("payload/%s_%s/%s", runtime.GOOS, runtime.GOARCH, LibraryFileName())
	f, err := payloadFS.Open(name)
	if err != nil {
		return nil, fmt.Errorf("payload %s: %w", name, err)
	}
	defer f.Close()
	return ioReadAll(f)
}

// cacheDir returns the private, versioned cache directory for the payload:
// <user-cache>/mirabase/native/<sha8>/ — overridable via
// MIRABASE_NATIVE_CACHE_DIR for operators who redirect caches.
func cacheDir(digest string) (string, error) {
	root := os.Getenv("MIRABASE_NATIVE_CACHE_DIR")
	if root == "" {
		base, err := os.UserCacheDir()
		if err != nil {
			return "", err
		}
		root = filepath.Join(base, "mirabase", "native")
	}
	return filepath.Join(root, digest[:8]), nil
}

// embeddedLibraryTryCreate ensures a verified copy of the embedded payload
// exists in the private cache and returns its absolute path.
func embeddedLibraryTryCreate(digest string) (string, error) {
	payload, err := payloadBytes()
	if err != nil {
		return "", err
	}
	if err := verifyBytes(payload, digest); err != nil {
		return "", err
	}

	dir, err := cacheDir(digest)
	if err != nil {
		return "", err
	}
	// Restrictive permissions on the versioned cache directory (best effort
	// where the OS does not enforce POSIX bits).
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", fmt.Errorf("failed to create payload cache dir %q: %w", dir, err)
	}
	_ = os.Chmod(dir, 0o700)

	final := filepath.Join(dir, LibraryFileName())

	// Cache hit: re-verify the on-disk bytes against the pin every load.
	if st, err := os.Lstat(final); err == nil {
		if !st.Mode().IsRegular() {
			return "", fmt.Errorf("%w: payload cache path %q is not a regular file", ErrTampered, final)
		}
		onDisk, err := os.ReadFile(final)
		if err != nil {
			return "", fmt.Errorf("failed to read cached payload: %w", err)
		}
		if err := verifyBytes(onDisk, digest); err != nil {
			// A corrupted or tampered cache copy is replaced from the
			// embedded (re-verified) bytes below.
			_ = os.Remove(final)
		} else {
			return final, nil
		}
	} else if !os.IsNotExist(err) {
		return "", fmt.Errorf("failed to stat cached payload: %w", err)
	}

	// Atomic write: temp file in the same directory, fsync-free rename.
	tmp := filepath.Join(dir, "."+LibraryFileName()+".tmp")
	if err := os.WriteFile(tmp, payload, 0o600); err != nil {
		return "", fmt.Errorf("failed to write payload temp file: %w", err)
	}
	if err := os.Chmod(tmp, 0o600); err != nil {
		_ = os.Remove(tmp)
		return "", fmt.Errorf("failed to set payload permissions: %w", err)
	}
	if err := os.Rename(tmp, final); err != nil {
		_ = os.Remove(tmp)
		return "", fmt.Errorf("failed to move payload into place: %w", err)
	}
	// Post-rename verification of the exact bytes that will be loaded.
	onDisk, err := os.ReadFile(final)
	if err != nil {
		return "", fmt.Errorf("failed to verify payload after extraction: %w", err)
	}
	if err := verifyBytes(onDisk, digest); err != nil {
		return "", err
	}
	return final, nil
}

func verifyBytes(b []byte, digest string) error {
	sum := sha256.Sum256(b)
	got := hex.EncodeToString(sum[:])
	if got != digest {
		return fmt.Errorf("%w: expected=%s got=%s", ErrTampered, digest, got)
	}
	return nil
}
