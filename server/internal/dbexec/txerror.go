package dbexec

import (
	"context"
	"errors"
	"strings"

	tursogo "turso.tech/database/tursogo"
)

// TxErrClass normalizes transaction failures into the separate classes
// required by transactions-and-concurrency.md: "Normalize busy,
// snapshot/write conflicts, deadlock-like conditions, constraint failures,
// cancellation and storage faults separately."
type TxErrClass int

const (
	// TxErrUnknown is any unclassified failure; never retried.
	TxErrUnknown TxErrClass = iota
	// TxErrBusy is a lock-busy condition; retryable.
	TxErrBusy
	// TxErrConflict is a snapshot/write conflict (BEGIN CONCURRENT loser);
	// retryable.
	TxErrConflict
	// TxErrConstraint is a unique/FK/check failure; never retried.
	TxErrConstraint
	// TxErrCancelled is context cancellation or interrupt; never retried
	// ("Cancellation never means rollback by assertion" — the caller
	// decides; the outcome is not claimed to be rolled back).
	TxErrCancelled
	// TxErrStorage is a storage fault (readonly, full, corrupt, not-a-db);
	// never retried.
	TxErrStorage
	// TxErrOutcomeUnknown is a commit whose outcome cannot be determined
	// (connection drop during commit). Clients reconcile through
	// idempotency receipt/status after reconnect (transactions-
	// concurrency.md).
	TxErrOutcomeUnknown
)

// ErrOutcomeUnknown marks an uncertain commit outcome.
var ErrOutcomeUnknown = errors.New("outcome_unknown")

// Retryable reports whether the class may be retried. Constraint and
// permission/schema errors are not retried; busy and conflict are.
func (c TxErrClass) Retryable() bool {
	return c == TxErrBusy || c == TxErrConflict
}

// ClassifyTxError maps a transaction failure onto the normalized classes.
func ClassifyTxError(err error) TxErrClass {
	if err == nil {
		return TxErrUnknown
	}
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return TxErrCancelled
	}
	if errors.Is(err, ErrOutcomeUnknown) {
		return TxErrOutcomeUnknown
	}
	// Driver sentinel classes (purego FFI surface).
	switch {
	case errors.Is(err, tursogo.ErrTursoConstraint):
		return TxErrConstraint
	case errors.Is(err, tursogo.ErrTursoBusy):
		// A busy-snapshot conflict surfaces as Busy from the engine's
		// transaction machinery; distinguish by message for the conflict
		// class (BEGIN CONCURRENT losers).
		msg := err.Error()
		if strings.Contains(msg, "snapshot") || strings.Contains(msg, "conflict") {
			return TxErrConflict
		}
		return TxErrBusy
	case errors.Is(err, tursogo.ErrTursoInterrupt):
		return TxErrCancelled
	case errors.Is(err, tursogo.ErrTursoReadOnly),
		errors.Is(err, tursogo.ErrTursoDatabaseFull),
		errors.Is(err, tursogo.ErrTursoNotADb),
		errors.Is(err, tursogo.ErrTursoCorrupt):
		return TxErrStorage
	}
	msg := strings.ToLower(err.Error())
	switch {
	case strings.Contains(msg, "snapshot"), strings.Contains(msg, "begin concurrent"):
		return TxErrConflict
	case strings.Contains(msg, "interrupt"), strings.Contains(msg, "cancel"):
		return TxErrCancelled
	case strings.Contains(msg, "constraint"):
		return TxErrConstraint
	}
	return TxErrUnknown
}
