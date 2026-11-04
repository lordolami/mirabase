//go:build js && wasm

package core

import "errors"

// MiraBase provenance note: upstream pull 7116
func execve(argv0 string, argv []string, envv []string) error {
	return errors.ErrUnsupported
}
