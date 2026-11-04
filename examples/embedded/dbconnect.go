//go:build !no_default_driver

package main

import (
	"mirabase/core"
)

// dbConnect returns the database connection function for this build
// variant. The default build leaves it nil so the inherited
// core.DefaultDBConnect applies.
func dbConnect() core.DBConnectFunc { return nil }

// engineEnsure is a no-op for the default (reference) build.
func engineEnsure() error { return nil }
