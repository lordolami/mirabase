package dbexec

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
)

// The committed-mutation boundary (v0.1-build.md):
//
//	Target-local `_mb_mutation_boundary` contains one row:
//	`id INTEGER PRIMARY KEY CHECK(id=1)`, `sequence INTEGER NOT NULL
//	CHECK(sequence>=0)`.
//
// The coordinator increments it once in each committed target mutation
// transaction, including administrative SQL and schema changes; internal
// receipt-only bookkeeping is excluded. The table is installed lazily at
// the first consumer ("Install application/target tables only at their
// first released consumer"). Integrity tests must demonstrate raw/trusted
// SQL cannot commit around this guard: writes that bypass the coordinator
// leave the boundary unchanged, which journal publication and rollback
// checks treat as evidence of an un-coordinated mutation.
const (
	mutationBoundaryTable = `_mb_mutation_boundary`
	mutationBoundaryDDL   = `CREATE TABLE IF NOT EXISTS ` + mutationBoundaryTable + ` (
		id INTEGER PRIMARY KEY CHECK(id=1),
		sequence INTEGER NOT NULL CHECK(sequence>=0)
	)`
)

// advanceMutationBoundary installs the boundary table if missing and
// increments the sequence within the caller's transaction. The increment
// is part of the same committed transaction as the mutation it marks; the
// install insert IS the first advance (sequence 1).
func advanceMutationBoundary(tx *sql.Tx) error {
	if _, err := tx.Exec(mutationBoundaryDDL); err != nil {
		return fmt.Errorf("failed to install %s: %w", mutationBoundaryTable, err)
	}
	if _, err := tx.Exec(`INSERT INTO ` + mutationBoundaryTable + ` (id, sequence) VALUES (1, 1)
		ON CONFLICT(id) DO UPDATE SET sequence = sequence + 1`); err != nil {
		return fmt.Errorf("failed to advance %s: %w", mutationBoundaryTable, err)
	}
	return nil
}

// MutationBoundarySequence reads the current boundary sequence (0 when the
// table is not installed).
func MutationBoundarySequence(ctx context.Context, db *sql.DB) (int64, error) {
	var seq int64
	err := db.QueryRowContext(ctx, `SELECT sequence FROM `+mutationBoundaryTable+` WHERE id = 1`).Scan(&seq)
	switch {
	case errors.Is(err, sql.ErrNoRows):
		return 0, nil
	case err != nil && isMissingTableErr(err):
		return 0, nil
	case err != nil:
		return 0, err
	}
	return seq, nil
}

// isMissingTableErr recognizes "no such table" style failures across
// providers.
func isMissingTableErr(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "no such table") || strings.Contains(msg, "does not exist")
}
