package mbengine

import "io"

// copyInto is a small helper so pin.go does not import io directly.
func copyInto(dst io.Writer, src io.Reader) (int64, error) {
	return io.Copy(dst, src)
}
