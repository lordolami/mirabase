//go:build !windows

package turso_libs

import (
	"fmt"

	"github.com/ebitengine/purego"
)

// loadLibrary opens the payload at the given absolute verified path via
// purego's dlopen wrapper (no CGO).
func loadLibrary(path string) (uintptr, error) {
	handle, err := purego.Dlopen(path, purego.RTLD_NOW|purego.RTLD_GLOBAL)
	if err != nil {
		return 0, fmt.Errorf("dlopen failed: %w", err)
	}
	return handle, nil
}

// freeLibrary releases a library handle opened by loadLibrary (refcounted).
// purego does not expose dlclose in v0.9; the process lifetime owns the
// library on posix targets, so this is a no-op there.
func freeLibrary(handle uintptr) error {
	_ = handle
	return nil
}
