//go:build no_default_driver

package main

import (
	"mirabase"
	"mirabase/core"
	"mirabase/internal/mbengine"
)

// dbConnect returns the database connection function for this build
// variant: the Turso-native provider connector used for BOTH application
// databases (native-provider.md; no second SQLite engine is used and the
// inherited connector is never a runtime fallback).
func dbConnect() core.DBConnectFunc {
	return core.TursoDBConnect
}

// engineEnsure runs the native payload trust sequence (digest pin,
// extraction, ABI and capability probes) — fatal before application files
// open.
func engineEnsure() error {
	return mbengine.Ensure()
}

var _ = mirabase.New
