//go:build !no_default_driver

package core

// TestDBConnect returns nil for the default (reference) build so the
// inherited DefaultDBConnect applies unchanged.
func TestDBConnect() DBConnectFunc { return nil }
