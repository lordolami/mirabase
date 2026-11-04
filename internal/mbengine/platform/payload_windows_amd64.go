//go:build windows && amd64

package turso_libs

import "embed"

// The build script copies the qualified mirabase_engine.dll into this
// directory before compiling. The embedded bytes are the payload of record;
// the adjacent on-disk checksum file (if any) is never an authority.
//
//go:embed payload/windows_amd64/mirabase_engine.dll
var payloadFS embed.FS
