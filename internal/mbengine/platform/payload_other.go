//go:build !(windows && amd64) && !(linux && amd64) && !(linux && amd64)

package turso_libs

import (
	"errors"
	"io/fs"
)

// notFoundFS is the payload source for platforms whose qualified payload is
// not embedded yet (the remaining five targets are qualified in the
// packaging gate). LoadTursoLibrary reports "not embedded" for them.
var payloadFS fs.FS = notFoundFS{}

var errNoPayload = errors.New("no payload embedded for this platform")

type notFoundFS struct{}

func (notFoundFS) Open(name string) (fs.File, error) {
	return nil, &fs.PathError{Op: "open", Path: name, Err: errNoPayload}
}
