//go:build no_default_driver

package core

// TestDBConnect returns the connector used by the test harnesses when the
// no_default_driver build is active: the Turso-native provider, so the full
// inherited suite runs on the pinned engine (V01-03 parity harness). With
// the default build the seam returns nil and the inherited connector
// applies exactly as upstream.
func TestDBConnect() DBConnectFunc { return TursoDBConnect }
