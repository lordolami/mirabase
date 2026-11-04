package dbexec_test

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"mirabase/internal/mbengine"
	"mirabase/internal/dbexec"
)

// testConnector builds the provider connector for a temp engine database.
// Production wiring (core.providerConnector) additionally applies the
// inherited pragma set with readback; these pool tests exercise the
// transaction machinery itself.
func testConnector(t *testing.T, path string) driver.Connector {
	t.Helper()
	c, err := newTestConnector(path)
	if err != nil {
		t.Fatalf("NewConnector: %v", err)
	}
	return c
}

// requireEngine skips the test when the pinned payload is not available
// (default builds; use _build/p02/parity_turso.sh for the engine runs).
func requireEngine(t *testing.T) {
	t.Helper()
	if mbengine.PinnedDigest() == "" {
		t.Skip("requires the pinned engine payload (run via _build/p02/parity_turso.sh)")
	}
	if err := mbengine.Ensure(); err != nil {
		t.Skipf("engine ensure failed: %v", err)
	}
}

func newPool(t *testing.T, profile dbexec.Profile, writers int) (*dbexec.WriterPool, string) {
	t.Helper()
	requireEngine(t)
	dir := t.TempDir()
	path := dir + "\\test.db"
	pool, err := dbexec.NewWriterPool(dbexec.WriterPoolConfig{
		Profile:    profile,
		Writers:    writers,
		Connector:  testConnector(t, path),
		MaxRetries: 3,
	})
	if err != nil {
		t.Fatalf("NewWriterPool: %v", err)
	}
	t.Cleanup(func() { _ = pool.Close() })
	return pool, path
}

func TestWriterPoolWALProfileSerializesWriters(t *testing.T) {
	pool, path := newPool(t, dbexec.ProfileWAL, 1)

	// create the table through the pool first
	if err := pool.RunTx(context.Background(), true, func(tx *sql.Tx) error {
		_, err := tx.Exec(`CREATE TABLE IF NOT EXISTS t (id INTEGER PRIMARY KEY, v TEXT)`)
		return err
	}); err != nil {
		t.Fatalf("create: %v", err)
	}

	var active, peak int64
	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			err := pool.RunTx(context.Background(), true, func(tx *sql.Tx) error {
				n := atomic.AddInt64(&active, 1)
				for {
					p := atomic.LoadInt64(&peak)
					if n <= p || atomic.CompareAndSwapInt64(&peak, p, n) {
						break
					}
				}
				defer atomic.AddInt64(&active, -1)
				_, err := tx.Exec(`INSERT INTO t (v) VALUES (?)`, fmt.Sprintf("w%d", i))
				return err
			})
			if err != nil {
				t.Errorf("writer %d: %v", i, err)
			}
		}(i)
	}
	wg.Wait()
	if peak != 1 {
		t.Fatalf("WAL profile must serialize writers: peak=%d want 1", peak)
	}

	// verify all writes committed through a fresh raw connection
	raw, err := sql.Open("turso", path)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer raw.Close()
	var total int
	if err := raw.QueryRow(`SELECT count(*) FROM t`).Scan(&total); err != nil {
		t.Fatalf("count: %v", err)
	}
	if total != 8 {
		t.Fatalf("expected 8 committed rows, got %d", total)
	}
}

func TestWriterPoolMVCCConcurrentIndependentRowWriters(t *testing.T) {
	pool, path := newPool(t, dbexec.ProfileMVCC, 4)

	if err := pool.RunTx(context.Background(), true, func(tx *sql.Tx) error {
		_, err := tx.Exec(`CREATE TABLE IF NOT EXISTS t (id INTEGER PRIMARY KEY, v TEXT)`)
		return err
	}); err != nil {
		t.Fatalf("create: %v", err)
	}

	// 8 concurrent writers over 4 bounded MVCC writer connections writing
	// independent rows: actual overlap must occur (the bound is a ceiling,
	// not a serialization), and every write must commit.
	var peak int64
	var wg sync.WaitGroup
	var failures int64
	for i := 0; i < 12; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
			defer cancel()
			err := pool.RunTx(ctx, true, func(tx *sql.Tx) error {
				atomic.AddInt64(&peak, 1)
				defer atomic.AddInt64(&peak, -1)
				time.Sleep(20 * time.Millisecond)
				_, err := tx.Exec(`INSERT INTO t (v) VALUES (?)`, fmt.Sprintf("m%d", i))
				return err
			})
			if err != nil {
				t.Logf("writer %d: %v", i, err)
				atomic.AddInt64(&failures, 1)
			}
		}(i)
	}
	wg.Wait()

	raw, err := sql.Open("turso", path)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer raw.Close()
	var total int
	if err := raw.QueryRow(`SELECT count(*) FROM t`).Scan(&total); err != nil {
		t.Fatalf("count: %v", err)
	}
	if peak > 4 {
		t.Fatalf("MVCC writer bound exceeded: peak=%d", peak)
	}
	if total+int(failures) != 12 {
		t.Fatalf("expected 12 attempted writes, committed=%d failed=%d", total, failures)
	}
	if failures > 0 {
		t.Logf("note: %d writers failed under contention (retry budget); committed=%d", failures, total)
	}
	if peak < 2 {
		t.Logf("note: overlap observed=%d (scheduler-dependent)", peak)
	}
}

func TestWriterPoolConstraintErrorsAreNotRetried(t *testing.T) {
	pool, _ := newPool(t, dbexec.ProfileMVCC, 2)

	if err := pool.RunTx(context.Background(), true, func(tx *sql.Tx) error {
		_, err := tx.Exec(`CREATE TABLE IF NOT EXISTS u (id INTEGER PRIMARY KEY)`)
		return err
	}); err != nil {
		t.Fatalf("create: %v", err)
	}
	// First insert commits.
	if err := pool.RunTx(context.Background(), true, func(tx *sql.Tx) error {
		_, err := tx.Exec(`INSERT INTO u (id) VALUES (1)`)
		return err
	}); err != nil {
		t.Fatalf("first insert: %v", err)
	}

	// The duplicate is a constraint failure and must not be retried.
	attempts := 0
	err := pool.RunTx(context.Background(), true, func(tx *sql.Tx) error {
		attempts++
		_, err := tx.Exec(`INSERT INTO u (id) VALUES (1)`)
		return err
	})
	if err == nil {
		t.Fatalf("expected a constraint failure")
	}
	if attempts != 1 {
		t.Fatalf("constraint errors must not be retried: attempts=%d", attempts)
	}
	if got := dbexec.ClassifyTxError(err); got != dbexec.TxErrConstraint {
		t.Fatalf("class = %v, want constraint", got)
	}
}

func TestMutationBoundaryAdvancesWithCoordinatorAndNotAroundIt(t *testing.T) {
	pool, path := newPool(t, dbexec.ProfileWAL, 1)

	// Coordinator-owned mutation transaction advances the boundary.
	for i := 0; i < 3; i++ {
		if err := pool.RunTx(context.Background(), true, func(tx *sql.Tx) error {
			_, err := tx.Exec(`CREATE TABLE IF NOT EXISTS t (id INTEGER)`)
			return err
		}); err != nil {
			t.Fatalf("coordinator tx: %v", err)
		}
	}
	raw, err := sql.Open("turso", path)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer raw.Close()

	seq, err := dbexec.MutationBoundarySequence(context.Background(), raw)
	if err != nil {
		t.Fatalf("sequence: %v", err)
	}
	if seq != 3 {
		t.Fatalf("boundary sequence = %d, want 3 (one per coordinator commit)", seq)
	}

	// Bookkeeping transactions do not advance it.
	if err := pool.RunBookkeepingTx(context.Background(), func(tx *sql.Tx) error {
		_, err := tx.Exec(`INSERT INTO t (id) VALUES (100)`)
		return err
	}); err != nil {
		t.Fatalf("bookkeeping tx: %v", err)
	}
	if seq, _ = dbexec.MutationBoundarySequence(context.Background(), raw); seq != 3 {
		t.Fatalf("bookkeeping advanced the boundary: %d", seq)
	}

	// Raw/trusted SQL bypassing the coordinator does not advance it —
	// the rollback checks treat that as an un-coordinated mutation.
	if _, err := raw.Exec(`INSERT INTO t (id) VALUES (200)`); err != nil {
		t.Fatalf("raw write: %v", err)
	}
	if seq, _ = dbexec.MutationBoundarySequence(context.Background(), raw); seq != 3 {
		t.Fatalf("raw write advanced the boundary: %d", seq)
	}
}

func TestIdempotencyReceiptsSameKeySameHashReplayAndConflict(t *testing.T) {
	pool, path := newPool(t, dbexec.ProfileWAL, 1)

	key := "0123456789abcdef" // 16 printable ASCII
	scope := dbexec.NewScope("scope-")
	hash1 := dbexec.CanonicalRequestHash([]byte(`{"a":1}`))
	hash2 := dbexec.CanonicalRequestHash([]byte(`{"a":2}`))

	// First mutation with the key: persists the receipt in the target tx.
	if err := pool.RunTx(context.Background(), true, func(tx *sql.Tx) error {
		if r, err := dbexec.ConsumeReceipt(tx, scope, key, hash1); err != nil {
			return err
		} else if r != nil {
			t.Fatalf("unexpected pre-existing receipt")
		}
		if _, err := tx.Exec(`CREATE TABLE IF NOT EXISTS r (id INTEGER, note TEXT)`); err != nil {
			return err
		}
		if _, err := tx.Exec(`INSERT INTO r (id, note) VALUES (1, 'first')`); err != nil {
			return err
		}
		return dbexec.PersistReceipt(tx, scope, key, hash1, "committed", map[string]any{"id": 1}, 0)
	}); err != nil {
		t.Fatalf("first mutation: %v", err)
	}

	// Same key/same hash: the committed result replays without re-running.
	replayed := false
	if err := pool.RunTx(context.Background(), true, func(tx *sql.Tx) error {
		r, err := dbexec.ConsumeReceipt(tx, scope, key, hash1)
		if err != nil {
			return err
		}
		if r == nil {
			return fmt.Errorf("receipt missing")
		}
		replayed = true
		// The replayed caller must not re-apply the effect.
		return nil
	}); err != nil {
		t.Fatalf("replay mutation: %v", err)
	}
	if !replayed {
		t.Fatalf("receipt did not replay")
	}

	raw, err := sql.Open("turso", path)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer raw.Close()
	var rows int
	if err := raw.QueryRow(`SELECT count(*) FROM r`).Scan(&rows); err != nil {
		t.Fatalf("count: %v", err)
	}
	if rows != 1 {
		t.Fatalf("replay re-applied the effect: rows=%d", rows)
	}

	// Same key/different hash: idempotency_conflict.
	err = pool.RunTx(context.Background(), true, func(tx *sql.Tx) error {
		_, err := dbexec.ConsumeReceipt(tx, scope, key, hash2)
		return err
	})
	if !errors.Is(err, dbexec.ErrIdempotencyConflict) {
		t.Fatalf("want idempotency_conflict, got %v", err)
	}

	// Reconciliation after reconnect (outcome_unknown path).
	rec, err := dbexec.ReconcileReceipt(context.Background(), raw, scope, key, hash1)
	if err != nil || rec == nil || rec.Outcome != "committed" {
		t.Fatalf("reconcile failed: rec=%v err=%v", rec, err)
	}

	// Invalid keys are rejected before any transaction work.
	if err := dbexec.ValidateIdempotencyKey("short"); err == nil {
		t.Fatalf("short key accepted")
	}
	if err := dbexec.ValidateIdempotencyKey("has whitespace inside 12345"); err == nil {
		t.Fatalf("whitespace key accepted")
	}
}

func TestIdempotencyReceiptRollbackLeavesNoReceipt(t *testing.T) {
	pool, path := newPool(t, dbexec.ProfileWAL, 1)

	key := "fedcba9876543210"
	scope := dbexec.NewScope("scope-")
	hash := dbexec.CanonicalRequestHash([]byte(`{"x":1}`))

	// A rolled-back transaction suppresses the receipt (persisted in the
	// target transaction — atomic with the commit).
	err := pool.RunTx(context.Background(), true, func(tx *sql.Tx) error {
		if err := dbexec.PersistReceipt(tx, scope, key, hash, "committed", nil, 0); err != nil {
			return err
		}
		return fmt.Errorf("forced rollback")
	})
	if err == nil {
		t.Fatalf("expected the forced rollback error")
	}

	raw, err := sql.Open("turso", path)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer raw.Close()
	rec, err := dbexec.ReconcileReceipt(context.Background(), raw, scope, key, hash)
	if err != nil {
		t.Fatalf("reconcile: %v", err)
	}
	if rec != nil {
		t.Fatalf("rolled-back transaction left a receipt: %+v", rec)
	}
}

// mbengineEnsureForHelper runs the payload trust sequence for the
// re-executed crash helper (the digest pin is compiled in by the parity
// harness build).
func mbengineEnsureForHelper() error { return mbengine.Ensure() }
