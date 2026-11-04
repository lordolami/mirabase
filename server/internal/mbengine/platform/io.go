package turso_libs

import "io"

// ioReadAll reads a payload file fully into memory.
func ioReadAll(f io.Reader) ([]byte, error) {
	return io.ReadAll(f)
}
