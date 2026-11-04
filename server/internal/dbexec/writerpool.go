// Package dbexec owns native execution, pooling, transactions, policy
// guards and capture. This file implements the bounded transaction writer
// pool: the qualified BEGIN CONCURRENT path (mvcc profile) with safe
// retries, uncertain-commit classification and the committed-mutation
// boundary advancement (transactions-and-concurrency.md,
// integration-contracts.md, v0.1-build.md).
package dbexec

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"fmt"
	"math/rand"
	"strings"
	"time"
)

// Profile selects the transaction profile (transactions-and-concurrency.md:
// "The default profile is wal. ... mvcc is opt-in per database ... A request
// cannot choose its own transaction profile.").
type Profile string

const (
	// ProfileWAL preserves serialized application writes and native WAL
	// recovery (the inherited default).
	ProfileWAL Profile = "wal"
	// ProfileMVCC enables the qualified BEGIN CONCURRENT path.
	ProfileMVCC Profile = "mvcc"
)

// Writer pool defaults (transactions-and-concurrency.md): "Optional
// database handles default to 4 read connections and one WAL writer or 4
// MVCC writers, constrained by a global default 256 connection budget."
const (
	DefaultMVCCWritersPerHandle = 4
	DefaultWALWritersPerHandle  = 1
)

// TxFunc is the transaction body. It must be replay-safe to be retried:
// "Automatic retries are allowed only before any unsafe external hook
// effect, or for a registered replay-safe transaction body with durable
// idempotency" (transactions-and-concurrency.md). The pool only retries
// bodies explicitly marked as pure.
type TxFunc func(tx *sql.Tx) error

// WriterPoolConfig configures one pool for one database handle.
type WriterPoolConfig struct {
	Profile Profile
	// Writers is the writer bound: 4 MVCC writers or one WAL writer by
	// default. Pool maximums are ceilings, not preallocated promises.
	Writers int
	// Connector opens connections through the provider (pragmas applied,
	// MVCC BEGIN CONCURRENT selection via context tagging).
	Connector driver.Connector
	// MaxRetries bounds automatic retries for pure transaction bodies
	// ("Internal pure transactions may retry up to 3 attempts with bounded
	// jitter under the request deadline").
	MaxRetries int
}

// WriterPool is a bounded transaction-writer pool for one database handle.
// The serialized writer path is preserved by constructing a WAL-profile
// pool (one writer); framework callers keep the inherited NonconcurrentDB
// semantics.
type WriterPool struct {
	profile   Profile
	db        *sql.DB
	maxRetry  int
	writeDML  bool
}

// NewWriterPool opens the pool with the configured writer bound.
func NewWriterPool(cfg WriterPoolConfig) (*WriterPool, error) {
	if cfg.Profile != ProfileWAL && cfg.Profile != ProfileMVCC {
		return nil, fmt.Errorf("unknown transaction profile %q", cfg.Profile)
	}
	writers := cfg.Writers
	if writers <= 0 {
		if cfg.Profile == ProfileMVCC {
			writers = DefaultMVCCWritersPerHandle
		} else {
			writers = DefaultWALWritersPerHandle
		}
	}
	if cfg.MaxRetries <= 0 {
		cfg.MaxRetries = 3
	}
	pool := &WriterPool{
		profile:  cfg.Profile,
		maxRetry: cfg.MaxRetries,
	}
	pool.db = sql.OpenDB(cfg.Connector)
	// Ceilings, not preallocated promises (implementation-closure.md).
	pool.db.SetMaxOpenConns(writers)
	pool.db.SetMaxIdleConns(writers)
	pool.db.SetConnMaxLifetime(0)
	return pool, nil
}

// Close closes the pool and its connections.
func (p *WriterPool) Close() error { return p.db.Close() }

// Profile returns the pool's transaction profile.
func (p *WriterPool) Profile() Profile { return p.profile }

// ErrOutcomeUnknown reports a commit whose result cannot be determined;
// clients reconcile through idempotency receipts (transactions-
// concurrency.md: "If a connection drops during Commit, return
// outcome_unknown when the outcome cannot be determined.").
func (p *WriterPool) RunTx(ctx context.Context, pure bool, fn TxFunc) error {
	var lastErr error
	for attempt := 0; attempt <= p.maxRetry; attempt++ {
		if attempt > 0 {
			// Bounded jitter under the request deadline.
			jitter := time.Duration(rand.Int63n(int64(20 * time.Millisecond)))
			select {
			case <-time.After(jitter):
			case <-ctx.Done():
				return ctx.Err()
			}
		}
		err := p.runOnce(ctx, fn)
		if err == nil {
			return nil
		}
		class := ClassifyTxError(err)
		if class == TxErrOutcomeUnknown {
			// Commit outcome cannot be determined: never retry on the same
			// connection; the caller reconciles through receipts.
			return fmt.Errorf("%w: %v", ErrOutcomeUnknown, err)
		}
		if !pure || !class.Retryable() || ctx.Err() != nil {
			return err
		}
		lastErr = err
	}
	if lastErr != nil {
		return lastErr
	}
	return errors.New("transaction failed")
}

// runOnce runs one attempt: acquire a writer connection, begin the
// profile's transaction, run the body, advance the committed-mutation
// boundary in the same transaction, then commit.
func (p *WriterPool) runOnce(ctx context.Context, fn TxFunc) error {
	conn, err := p.db.Conn(ctx)
	if err != nil {
		return err
	}
	defer func() {
		// Pool hygiene: no connection may leak an unfinished transaction
		// back into the pool (native-provider.md). Rollback on return if
		// the tx is still live; database/sql discards broken conns.
		_ = conn.Close()
	}()

	opts := &sql.TxOptions{}
	beginCtx := ctx
	if p.profile == ProfileMVCC {
		// The connector selects BEGIN CONCURRENT for the MVCC writer
		// profile on this pinned connection (integration-contracts.md).
		beginCtx = WithMvccWriter(ctx)
	}
	tx, err := conn.BeginTx(beginCtx, opts)
	if err != nil {
		return err
	}

	committed := false
	defer func() {
		if !committed {
			_ = tx.Rollback()
		}
	}()

	if err := fn(tx); err != nil {
		return err
	}

	// Advance the committed-mutation boundary inside the same transaction
	// (v0.1-build.md: the coordinator increments it once in each committed
	// target mutation transaction). Receipt-only bookkeeping transactions
	// run through RunBookkeepingTx and do not advance the boundary.
	if err := advanceMutationBoundary(tx); err != nil {
		return err
	}

	if err := commitWithOutcomeClassification(beginCtx, tx); err != nil {
		return err
	}
	committed = true
	return nil
}

// RunBookkeepingTx runs an internal receipt-only transaction: it does not
// advance the committed-mutation boundary (v0.1-build.md: "internal
// receipt-only bookkeeping is excluded").
func (p *WriterPool) RunBookkeepingTx(ctx context.Context, fn TxFunc) error {
	conn, err := p.db.Conn(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = conn.Close() }()

	beginCtx := ctx
	if p.profile == ProfileMVCC {
		beginCtx = WithMvccWriter(ctx)
	}
	tx, err := conn.BeginTx(beginCtx, &sql.TxOptions{})
	if err != nil {
		return err
	}
	committed := false
	defer func() {
		if !committed {
			_ = tx.Rollback()
		}
	}()
	if err := fn(tx); err != nil {
		return err
	}
	if err := commitWithOutcomeClassification(beginCtx, tx); err != nil {
		return err
	}
	committed = true
	return nil
}

// commitWithOutcomeClassification performs the commit and classifies a
// failed commit: a broken/dropped connection during commit is
// outcome_unknown ("the outcome cannot be determined"); clean engine
// refusals keep their classified class.
func commitWithOutcomeClassification(ctx context.Context, tx *sql.Tx) error {
	err := tx.Commit()
	if err == nil {
		return nil
	}
	if isBrokenConnCommit(err, ctx) {
		return fmt.Errorf("%w: commit outcome could not be determined: %v", ErrOutcomeUnknown, err)
	}
	return err
}

func isBrokenConnCommit(err error, ctx context.Context) bool {
	if ctx.Err() != nil {
		// Cancellation never means rollback by assertion — classify as
		// uncertain so the caller reconciles through receipts instead of
		// claiming a rollback.
		return true
	}
	if errors.Is(err, driver.ErrBadConn) || errors.Is(err, sql.ErrConnDone) {
		return true
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "bad connection") || strings.Contains(msg, "connection")
}
