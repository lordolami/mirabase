//go:build linux && amd64

package turso_libs

import "embed"

// The build script copies the qualified mirabase_engine.so into this
// directory before compiling. The embedded bytes are the payload of
// record; the adjacent on-disk checksum file (if any) is never an
// authority.
//
//go:embed payload/linux_amd64/mirabase_engine.so
var payloadFS embed.FS
