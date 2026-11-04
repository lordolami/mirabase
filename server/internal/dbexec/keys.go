package dbexec

import (
	"crypto/rand"
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

// DefaultDeliveryKeyRef is the default local delivery key reference:
// owner-private mb_data/keys, generated atomically with platform-
// appropriate ACLs ("Default local delivery keys are generated atomically
// into owner-private mb_data/keys ... no cloud account or manual key setup
// is required").
const DefaultDeliveryKeyRef = "local/mb_data/keys/delivery.key"

// ErrDeliveryKey reports a missing or unreadable delivery key.
var ErrDeliveryKey = errors.New("delivery key unavailable")

// LoadOrCreateDeliveryKey resolves (or atomically creates) the 32-byte
// AES-256 key for a reference. "local/..." references resolve under the
// process data directory's keys folder; rotation retains old keys so
// pending jobs stay readable.
func LoadOrCreateDeliveryKey(keyRef string) ([]byte, error) {
	path, err := keyRefPath(keyRef)
	if err != nil {
		return nil, err
	}
	if b, err := os.ReadFile(path); err == nil && len(b) == 32 {
		return b, nil
	} else if err != nil && !errors.Is(err, os.ErrNotExist) {
		return nil, fmt.Errorf("%w: %v", ErrDeliveryKey, err)
	} else if err == nil && len(b) != 32 {
		return nil, fmt.Errorf("%w: key at %q is not a 32-byte key", ErrDeliveryKey, path)
	}

	// Atomic creation: temp file in the same directory, restrictive
	// permissions, rename into place.
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrDeliveryKey, err)
	}
	key := make([]byte, 32)
	if _, err := rand.Read(key); err != nil {
		return nil, err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, key, 0o600); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrDeliveryKey, err)
	}
	_ = os.Chmod(tmp, 0o600)
	if err := os.Rename(tmp, path); err != nil {
		_ = os.Remove(tmp)
		return nil, fmt.Errorf("%w: %v", ErrDeliveryKey, err)
	}
	return key, nil
}

// keyRefPath maps "local/<name>" key references to the filesystem path.
// MIRABASE_KEYS_DIR overrides the keys directory (tests and operator
// relocation); the default resolves under the process data directory's
// keys folder. Non-local references are rejected: operators wire a secret
// provider for those; there is no silent fallback to the filesystem.
func keyRefPath(keyRef string) (string, error) {
	const localPrefix = "local/"
	if len(keyRef) <= len(localPrefix) || keyRef[:len(localPrefix)] != localPrefix {
		return "", fmt.Errorf("%w: unsupported key reference %q (operators wire a secret provider for non-local keys)", ErrDeliveryKey, keyRef)
	}
	name := keyRef[len(localPrefix):]
	if dir := os.Getenv("MIRABASE_KEYS_DIR"); dir != "" {
		return filepath.Join(dir, filepath.Base(name)), nil
	}
	return name, nil
}

// LocalKeyRef builds a local key reference for the given key file name
// under the keys directory.
func LocalKeyRef(name string) string {
	return "local/" + name
}
