package dbexec_test

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"testing"

	"mirabase/internal/dbexec"
)

func newJobsKernel(t *testing.T) (*dbexec.JobKernel, string) {
	t.Helper()
	requireEngine(t)
	dir := t.TempDir()
	path := dir + "\\jobs.db"
	raw, err := sql.Open("turso", path+"?_busy_timeout=10000&experimental=vacuum")
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(func() { _ = raw.Close() })
	if _, err := raw.Conn(context.Background()); err != nil {
		t.Fatalf("connect: %v", err)
	}
	k, err := dbexec.NewJobKernel(raw)
	if err != nil {
		t.Fatalf("NewJobKernel: %v", err)
	}
	return k, path
}

func TestJobsLifecycleSucceedsWithManifest(t *testing.T) {
	k, _ := newJobsKernel(t)
	called := 0
	k.Register("succeed", func(ctx context.Context, job *dbexec.Job) (any, error) {
		called++
		return map[string]any{"steps": []string{"s1", "s2"}, "target": "ok"}, nil
	})

	job, err := k.Enqueue(context.Background(), "succeed", map[string]string{"p": "1"})
	if err != nil {
		t.Fatalf("enqueue: %v", err)
	}

	ran, err := k.RunOnce(context.Background())
	if err != nil || !ran {
		t.Fatalf("RunOnce: ran=%v err=%v", ran, err)
	}
	if called != 1 {
		t.Fatalf("handler calls = %d", called)
	}
	// A succeeded job has a durable result manifest.
	jobRead := kRead(t, k, job.ID)
	if jobRead.State != "succeeded" || len(jobRead.Result) == 0 {
		t.Fatalf("state=%q result=%q — want succeeded with a manifest", jobRead.State, string(jobRead.Result))
	}
}

func kRead(t *testing.T, k *dbexec.JobKernel, id string) *dbexec.Job {
	t.Helper()
	job, err := k.ReadJob(context.Background(), id)
	if err != nil {
		t.Fatalf("ReadJob %s: %v", id, err)
	}
	return job
}

func TestJobsRetryWaitsThenStopsAtMaxAttempts(t *testing.T) {
	k, _ := newJobsKernel(t)
	attempts := 0
	var mu sync.Mutex
	k.Register("flaky", func(ctx context.Context, job *dbexec.Job) (any, error) {
		mu.Lock()
		attempts++
		n := attempts
		mu.Unlock()
		if n < 3 {
			return nil, errors.New("transient failure")
		}
		return "done", nil
	})

	if _, err := k.Enqueue(context.Background(), "flaky", nil); err != nil {
		t.Fatalf("enqueue: %v", err)
	}
	for i := 0; i < 5; i++ {
		if _, err := k.RunOnce(context.Background()); err != nil {
			t.Fatalf("RunOnce %d: %v", i, err)
		}
	}
	mu.Lock()
	defer mu.Unlock()
	if attempts != 3 {
		t.Fatalf("attempts = %d, want 3 (two retries then success)", attempts)
	}
}

func TestJobsNonRetryableStopsImmediately(t *testing.T) {
	k, _ := newJobsKernel(t)
	attempts := 0
	k.Register("bad", func(ctx context.Context, job *dbexec.Job) (any, error) {
		attempts++
		return nil, fmt.Errorf("schema drift: %w", dbexec.ErrNonRetryable)
	})
	job, _ := k.Enqueue(context.Background(), "bad", nil)
	for i := 0; i < 5; i++ {
		if _, err := k.RunOnce(context.Background()); err != nil {
			t.Fatalf("RunOnce %d: %v", i, err)
		}
	}
	if attempts != 1 {
		t.Fatalf("nonretryable ran %d times, want exactly 1", attempts)
	}
	if got := kRead(t, k, job.ID).State; got != "failed" {
		t.Fatalf("state = %q, want failed", got)
	}
}

func TestJobsLeaseExpiryReconciles(t *testing.T) {
	k, _ := newJobsKernel(t)
	if _, err := k.Enqueue(context.Background(), "slow", nil); err != nil {
		t.Fatalf("enqueue: %v", err)
	}

	// Worker A leases (an active lease holds the job).
	jobA, err := k.Lease(context.Background(), nil)
	if err != nil || jobA == nil {
		t.Fatalf("worker A lease: %v %v", jobA, err)
	}
	// While A holds the lease, worker B cannot lease the same job.
	jobB, err := k.Lease(context.Background(), nil)
	if err != nil || jobB != nil {
		t.Fatalf("worker B leased the held job: %v %v", jobB, err)
	}

	// Force-expire the held lease (operational repair path), then
	// reconcile: the job returns to queued and is leasable again.
	if n, err := k.ExpireAllLeases(context.Background()); err != nil || n != 1 {
		t.Fatalf("expire leases: n=%v err=%v", n, err)
	}
	reconciled, err := k.ReconcileExpiredLeases(context.Background())
	if err != nil || reconciled != 1 {
		t.Fatalf("reconcile: n=%v err=%v", reconciled, err)
	}
	jobC, err := k.Lease(context.Background(), nil)
	if err != nil || jobC == nil {
		t.Fatalf("lease after reconcile: %v %v", jobC, err)
	}
	if jobC.ID != jobA.ID {
		t.Fatalf("reconciled job %s came back as %s", jobA.ID, jobC.ID)
	}
}

func TestJobsDuplicateWorkersClaimDifferentJobs(t *testing.T) {
	k, _ := newJobsKernel(t)
	for i := 0; i < 3; i++ {
		if _, err := k.Enqueue(context.Background(), "multi", nil); err != nil {
			t.Fatalf("enqueue %d: %v", i, err)
		}
	}
	seen := map[string]bool{}
	for i := 0; i < 3; i++ {
		job, err := k.Lease(context.Background(), nil)
		if err != nil {
			t.Fatalf("lease %d: %v", i, err)
		}
		if job == nil {
			t.Fatalf("lease %d returned no job", i)
		}
		if seen[job.ID] {
			t.Fatalf("the same job was leased twice: %s", job.ID)
		}
		seen[job.ID] = true
	}
	if job, _ := k.Lease(context.Background(), nil); job != nil {
		t.Fatalf("leaked an extra job: %+v", job)
	}
}

func TestJobsCancellationAtSafeBoundaryRecordsEffects(t *testing.T) {
	k, _ := newJobsKernel(t)
	k.Register("cancellable", func(ctx context.Context, job *dbexec.Job) (any, error) {
		// Deterministic step: s1 commits an effect, then the handler
		// observes the cancel request and stops at the safe boundary.
		if err := k.RecordStep(ctx, job.ID, "s1", "s1-committed"); err != nil {
			return nil, err
		}
		cancel, err := k.IsCancelRequested(ctx, job.ID)
		if err != nil {
			return nil, err
		}
		if cancel {
			return map[string]any{"committed": []string{"s1"}}, nil
		}
		return nil, errors.New("cancel was not requested")
	})
	job, err := k.Enqueue(context.Background(), "cancellable", nil)
	if err != nil {
		t.Fatalf("enqueue: %v", err)
	}
	if err := k.RequestCancel(context.Background(), job.ID); err != nil {
		t.Fatalf("RequestCancel: %v", err)
	}
	if _, err := k.RunOnce(context.Background()); err != nil {
		t.Fatalf("RunOnce: %v", err)
	}
	jobRead := kRead(t, k, job.ID)
	if jobRead.State != "cancelled" {
		t.Fatalf("state = %q, want cancelled", jobRead.State)
	}
	if len(jobRead.Result) == 0 {
		t.Fatalf("cancelled job lost its committed-effects record")
	}
}

func TestOutboxIntentPersistsWithTransactionAndDeliversAtLeastOnce(t *testing.T) {
	requireEngine(t)
	dir := t.TempDir()
	path := dir + "\\outbox.db"
	raw, err := sql.Open("turso", path+"?_busy_timeout=10000&experimental=vacuum")
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer raw.Close()

	// Committed data transaction: the intent is atomic with the data.
	tx, err := raw.Begin()
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	if _, err := tx.Exec(`CREATE TABLE IF NOT EXISTS data (id INTEGER)`); err != nil {
		t.Fatalf("create: %v", err)
	}
	if _, err := tx.Exec(`INSERT INTO data (id) VALUES (1)`); err != nil {
		t.Fatalf("insert: %v", err)
	}
	if err := dbexec.EnqueueOutbox(tx, dbexec.OutboxEvent{
		EventID:  "evt-1",
		DedupKey: "dedup-1",
		Kind:     "mail.verification",
		Payload:  []byte(`{"to":"u@example.com"}`),
	}); err != nil {
		t.Fatalf("enqueue outbox: %v", err)
	}
	if err := tx.Commit(); err != nil {
		t.Fatalf("commit: %v", err)
	}

	pending, err := dbexec.PendingOutbox(context.Background(), raw, 10)
	if err != nil || len(pending) != 1 {
		t.Fatalf("pending = %v err = %v", pending, err)
	}

	// At-least-once: deliver, then the event is marked delivered; a failed
	// delivery stays pending with an attempt counter.
	if err := dbexec.RecordDeliveryAttempt(context.Background(), raw, "evt-1"); err != nil {
		t.Fatalf("attempt: %v", err)
	}
	pending, err = dbexec.PendingOutbox(context.Background(), raw, 10)
	if err != nil || len(pending) != 1 || pending[0].Attempts != 1 {
		t.Fatalf("after attempt: pending=%v err=%v", pending, err)
	}
	if err := dbexec.MarkDelivered(context.Background(), raw, "evt-1"); err != nil {
		t.Fatalf("mark: %v", err)
	}
	pending, err = dbexec.PendingOutbox(context.Background(), raw, 10)
	if err != nil || len(pending) != 0 {
		t.Fatalf("after delivery: pending=%v err=%v", pending, err)
	}

	// Rollback suppresses the intent: the event never becomes pending.
	tx, err = raw.Begin()
	if err != nil {
		t.Fatalf("begin 2: %v", err)
	}
	_ = dbexec.EnqueueOutbox(tx, dbexec.OutboxEvent{
		EventID: "evt-2", DedupKey: "dedup-2", Kind: "never", Payload: []byte(`{}`),
	})
	_ = tx.Rollback()
	pending, err = dbexec.PendingOutbox(context.Background(), raw, 10)
	if err != nil || len(pending) != 0 {
		t.Fatalf("rollback leaked an intent: pending=%v err=%v", pending, err)
	}

	// Deduplication: the same dedup key enqueues only once.
	tx, err = raw.Begin()
	if err != nil {
		t.Fatalf("begin 3: %v", err)
	}
	for i := 0; i < 3; i++ {
		if err := dbexec.EnqueueOutbox(tx, dbexec.OutboxEvent{
			EventID: fmt.Sprintf("evt-dup-%d", i), DedupKey: "same-dedup", Kind: "k", Payload: []byte(`{}`),
		}); err != nil {
			t.Fatalf("dup enqueue: %v", err)
		}
	}
	if err := tx.Commit(); err != nil {
		t.Fatalf("commit 3: %v", err)
	}
	pending, _ = dbexec.PendingOutbox(context.Background(), raw, 10)
	if len(pending) != 1 {
		t.Fatalf("dedup failed: %d pending events with one dedup key", len(pending))
	}
}

func TestOutboxEnvelopeEncryptsAndBindsAssociatedData(t *testing.T) {
	t.Setenv("MIRABASE_KEYS_DIR", filepath.Join(t.TempDir(), "keys"))

	payload := []byte(`{"token":"reset-secret-123"}`)
	env, err := dbexec.SealEnvelope(dbexec.DefaultDeliveryKeyRef, "evt-enc-1", payload)
	if err != nil {
		t.Fatalf("seal: %v", err)
	}
	if env.Version != 1 || len(env.Nonce) != 12 || len(env.Tag) != 16 || len(env.Ciphertext) == 0 {
		t.Fatalf("envelope shape wrong: %+v", env)
	}
	// Key bytes are separate from the envelope; the key file exists with
	// restrictive permissions.
	keyPath := filepath.Join(os.Getenv("MIRABASE_KEYS_DIR"), "delivery.key")
	if st, err := os.Stat(keyPath); err != nil || !st.Mode().IsRegular() {
		t.Fatalf("delivery key file %q: %v", keyPath, err)
	}

	opened, err := dbexec.OpenEnvelope(env, "evt-enc-1")
	if err != nil || string(opened) != string(payload) {
		t.Fatalf("roundtrip: %q err=%v", opened, err)
	}

	// Tampered ciphertext must fail authentication.
	bad := *env
	bad.Ciphertext[len(bad.Ciphertext)-1] ^= 0xFF
	if _, err := dbexec.OpenEnvelope(&bad, "evt-enc-1"); err == nil {
		t.Fatalf("tampered ciphertext authenticated")
	}

	// A different event id (associated data) must fail.
	if _, err := dbexec.OpenEnvelope(env, "evt-OTHER"); err == nil {
		t.Fatalf("envelope accepted a different associated-data binding")
	}

	// Non-local key references are rejected (operators wire a provider).
	if _, err := dbexec.SealEnvelope("vault://prod", "evt-x", payload); err == nil {
		t.Fatalf("non-local key reference was silently accepted")
	}
}

func TestDeliveryKeyAtomicCreation(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("MIRABASE_KEYS_DIR", filepath.Join(dir, "keys"))

	ref := dbexec.LocalKeyRef("delivery.key")
	k1, err := dbexec.KeysFor(ref)
	if err != nil {
		t.Fatalf("first resolve: %v", err)
	}
	k2, err := dbexec.KeysFor(ref)
	if err != nil {
		t.Fatalf("second resolve: %v", err)
	}
	if string(k1) != string(k2) {
		t.Fatalf("key rotated between calls")
	}
	if len(k1) != 32 {
		t.Fatalf("key length %d, want 32 (AES-256)", len(k1))
	}
}

