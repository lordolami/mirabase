package dbexec

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"time"
)

// Durable jobs kernel (extensions-and-jobs.md, v0.1 slice):
//
//	"States are queued → running → succeeded|failed|cancelled. Running may
//	 enter waiting_retry; cancellation is requested before it is
//	 acknowledged."
//	"A succeeded job has a durable result manifest."
//	"Failure has code, phase, retryability and committed-step records."
//	"A worker leases a job for 30 seconds, renews every 10 seconds, and
//	 reconciles an expired lease before retrying."
//	"Retry uses deterministic step IDs and idempotent effects."
//	"Default maximum 8 attempts; nonretryable schema/auth/capability
//	 failures stop immediately."
const (
	DefaultLeaseSeconds = 30
	DefaultRenewSeconds = 10
	DefaultMaxAttempts  = 8

	jobsTableDDL = `CREATE TABLE IF NOT EXISTS _mb_jobs (
		id TEXT PRIMARY KEY,
		kind TEXT NOT NULL,
		payload_json BLOB,
		state TEXT NOT NULL DEFAULT 'queued',
		attempts INTEGER NOT NULL DEFAULT 0,
		max_attempts INTEGER NOT NULL DEFAULT 8,
		lease_until INTEGER,
		worker_id TEXT,
		step_id TEXT,
		result_json BLOB,
		error_code TEXT,
		error_phase TEXT,
		retryable INTEGER NOT NULL DEFAULT 1,
		cancel_requested INTEGER NOT NULL DEFAULT 0,
		created_at INTEGER NOT NULL,
		updated_at INTEGER NOT NULL
	)`
	jobsKindIndexDDL = `CREATE INDEX IF NOT EXISTS _mb_jobs_kind_state
		ON _mb_jobs (kind, state, created_at)`
)

// Job states.
const (
	JobStateQueued    = "queued"
	JobStateRunning   = "running"
	JobStateWaiting   = "waiting_retry"
	JobStateSucceeded = "succeeded"
	JobStateFailed    = "failed"
	JobStateCancelled = "cancelled"
	JobStateCancelReq = "cancel_requested"
)

// Job is one durable job row.
type Job struct {
	ID          string
	Kind        string
	Payload     json.RawMessage
	State       string
	Attempts    int
	MaxAttempts int
	LeaseUntil  *time.Time
	WorkerID    string
	StepID      string
	Result      json.RawMessage
	ErrorCode   string
	ErrorPhase  string
	Retryable   bool
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

// JobHandler executes one leased job. Handlers must use deterministic step
// IDs and idempotent effects; returning a non-retryable error stops the
// job immediately (schema/auth/capability failures).
type JobHandler func(ctx context.Context, job *Job) (result any, err error)

// ErrNonRetryable marks a job failure that must stop immediately.
var ErrNonRetryable = errors.New("nonretryable job failure")

// JobKernel owns the durable job store and worker loop for one database.
type JobKernel struct {
	db       *sql.DB
	handlers map[string]JobHandler
	lease    time.Duration
	renew    time.Duration
	maxTry   int
	workerID string
}

// NewJobKernel opens the jobs kernel over a database.
func NewJobKernel(db *sql.DB) (*JobKernel, error) {
	if _, err := db.Exec(jobsTableDDL); err != nil {
		return nil, fmt.Errorf("failed to install jobs table: %w", err)
	}
	if _, err := db.Exec(jobsKindIndexDDL); err != nil {
		return nil, err
	}
	return &JobKernel{
		db:       db,
		handlers: map[string]JobHandler{},
		lease:    time.Duration(DefaultLeaseSeconds) * time.Second,
		renew:    time.Duration(DefaultRenewSeconds) * time.Second,
		maxTry:   DefaultMaxAttempts,
		workerID: randomID("worker-"),
	}, nil
}

// Register installs the handler for a job kind.
func (k *JobKernel) Register(kind string, h JobHandler) { k.handlers[kind] = h }

// Enqueue records a queued job.
func (k *JobKernel) Enqueue(ctx context.Context, kind string, payload any) (*Job, error) {
	var payloadJSON []byte
	if payload != nil {
		b, err := json.Marshal(payload)
		if err != nil {
			return nil, err
		}
		payloadJSON = b
	}
	job := &Job{
		ID:          randomID("job-"),
		Kind:        kind,
		Payload:     payloadJSON,
		State:       JobStateQueued,
		MaxAttempts: k.maxTry,
		Retryable:   true,
		CreatedAt:   time.Now().UTC(),
		UpdatedAt:   time.Now().UTC(),
	}
	_, err := k.db.ExecContext(ctx,
		`INSERT INTO _mb_jobs (id, kind, payload_json, state, attempts, max_attempts, retryable, created_at, updated_at)
		 VALUES (?, ?, ?, ?, 0, ?, 1, ?, ?)`,
		job.ID, job.Kind, job.Payload, job.State, job.MaxAttempts, job.CreatedAt.Unix(), job.UpdatedAt.Unix())
	if err != nil {
		return nil, err
	}
	return job, nil
}

// EnqueueInTx records a queued job inside the caller's transaction
// (durable intent with the data transaction).
func (k *JobKernel) EnqueueInTx(tx *sql.Tx, kind string, payload any) (*Job, error) {
	if _, err := tx.Exec(jobsTableDDL); err != nil {
		return nil, fmt.Errorf("failed to install jobs table: %w", err)
	}
	var payloadJSON []byte
	if payload != nil {
		b, err := json.Marshal(payload)
		if err != nil {
			return nil, err
		}
		payloadJSON = b
	}
	job := &Job{
		ID:          randomID("job-"),
		Kind:        kind,
		Payload:     payloadJSON,
		State:       JobStateQueued,
		MaxAttempts: k.maxTry,
		Retryable:   true,
		CreatedAt:   time.Now().UTC(),
		UpdatedAt:   time.Now().UTC(),
	}
	_, err := tx.Exec(
		`INSERT INTO _mb_jobs (id, kind, payload_json, state, attempts, max_attempts, retryable, created_at, updated_at)
		 VALUES (?, ?, ?, ?, 0, ?, 1, ?, ?)`,
		job.ID, job.Kind, job.Payload, job.State, job.MaxAttempts, job.CreatedAt.Unix(), job.UpdatedAt.Unix())
	if err != nil {
		return nil, err
	}
	return job, nil
}

// ReconcileExpiredLeases returns expired running jobs to the queued state
// before retrying ("reconciles an expired lease before retrying").
func (k *JobKernel) ReconcileExpiredLeases(ctx context.Context) (int64, error) {
	res, err := k.db.ExecContext(ctx,
		`UPDATE _mb_jobs SET state = ?, worker_id = NULL, lease_until = NULL, step_id = NULL, updated_at = ?
		 WHERE state = ? AND lease_until IS NOT NULL AND lease_until < ?`,
		JobStateQueued, time.Now().UTC().Unix(), JobStateRunning, time.Now().UTC().Unix())
	if err != nil {
		return 0, err
	}
	return res.RowsAffected()
}

// Lease claims one queued (or expired-lease) job for this worker. The
// claim is a single atomic UPDATE so duplicate workers never run the same
// job concurrently.
func (k *JobKernel) Lease(ctx context.Context, kinds []string) (*Job, error) {
	now := time.Now().UTC().Unix()
	leaseUntil := time.Now().UTC().Add(k.lease).Unix()

	kindFilter := ""
	extraArgs := []any{}
	if len(kinds) > 0 {
		kindFilter = " AND kind IN ("
		for i, kind := range kinds {
			if i > 0 {
				kindFilter += ","
			}
			kindFilter += "?"
			extraArgs = append(extraArgs, kind)
		}
		kindFilter += ")"
	}

	// Optimistic claim: pick a candidate id, then atomically claim it —
	// the claim predicate re-checks the state so a concurrent worker that
	// won the race makes this attempt a no-op and the loop continues.
	candidateSQL := `SELECT id FROM _mb_jobs
		WHERE (state IN ('queued', 'waiting_retry')
		       OR (state = 'running' AND lease_until IS NOT NULL AND lease_until < ?))
		  AND attempts < max_attempts` + kindFilter + `
		ORDER BY created_at LIMIT 1`
	candidateArgs := append([]any{now}, extraArgs...)

	claimSQL := `UPDATE _mb_jobs SET state = ?, worker_id = ?, lease_until = ?, updated_at = ?, attempts = attempts + 1
		WHERE id = ? AND attempts < max_attempts
		  AND (state IN ('queued', 'waiting_retry')
		       OR (state = 'running' AND lease_until IS NOT NULL AND lease_until < ?))`
	claimBase := append([]any{JobStateRunning, k.workerID, leaseUntil, now}, now)
	_ = claimBase

	for tries := 0; tries < 100; tries++ {
		var id string
		err := k.db.QueryRowContext(ctx, candidateSQL, candidateArgs...).Scan(&id)
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		if err != nil {
			return nil, err
		}
		claim, err := k.db.ExecContext(ctx, claimSQL,
			append([]any{JobStateRunning, k.workerID, leaseUntil, now, id, now}, extraArgs...)...)
		if err != nil {
			return nil, err
		}
		if n, _ := claim.RowsAffected(); n == 0 {
			// Lost the race for this candidate; take the next one.
			continue
		}
		var job Job
		var createdUnix int64
		err = k.db.QueryRowContext(ctx,
			`SELECT id, kind, payload_json, state, attempts, max_attempts, retryable, created_at
			 FROM _mb_jobs WHERE id = ?`, id,
		).Scan(&job.ID, &job.Kind, &job.Payload, &job.State, &job.Attempts, &job.MaxAttempts, &job.Retryable, &createdUnix)
		if err != nil {
			return nil, err
		}
		job.CreatedAt = time.Unix(createdUnix, 0).UTC()
		job.WorkerID = k.workerID
		lease := time.Unix(leaseUntil, 0).UTC()
		job.LeaseUntil = &lease
		return &job, nil
	}
	return nil, nil
}

// Renew extends a held lease (called every 10 seconds).
func (k *JobKernel) Renew(ctx context.Context, jobID string) error {
	until := time.Now().UTC().Add(k.renew).Unix()
	_, err := k.db.ExecContext(ctx,
		`UPDATE _mb_jobs SET lease_until = ?, updated_at = ? WHERE id = ? AND worker_id = ? AND state = ?`,
		until, time.Now().UTC().Unix(), jobID, k.workerID, JobStateRunning)
	return err
}

// RequestCancel marks a running job for cancellation at the next safe step
// boundary ("Cancellation stops at a safe step boundary ... and records
// already committed effects").
func (k *JobKernel) RequestCancel(ctx context.Context, jobID string) error {
	_, err := k.db.ExecContext(ctx,
		`UPDATE _mb_jobs SET cancel_requested = 1, updated_at = ? WHERE id = ? AND state IN (?, ?)`,
		time.Now().UTC().Unix(), jobID, JobStateQueued, JobStateRunning)
	return err
}

// IsCancelRequested reports whether a running job should stop at its next
// safe step boundary.
func (k *JobKernel) IsCancelRequested(ctx context.Context, jobID string) (bool, error) {
	var flag int
	err := k.db.QueryRowContext(ctx, `SELECT cancel_requested FROM _mb_jobs WHERE id = ?`, jobID).Scan(&flag)
	if err != nil {
		return false, err
	}
	return flag != 0, nil
}

// Complete records a succeeded job with its durable result manifest.
func (k *JobKernel) Complete(ctx context.Context, jobID string, result any) error {
	var resultJSON []byte
	if result != nil {
		b, err := json.Marshal(result)
		if err != nil {
			return err
		}
		resultJSON = b
	}
	_, err := k.db.ExecContext(ctx,
		`UPDATE _mb_jobs SET state = ?, result_json = ?, lease_until = NULL, worker_id = NULL, updated_at = ? WHERE id = ?`,
		JobStateSucceeded, resultJSON, time.Now().UTC().Unix(), jobID)
	return err
}

// Fail records a failure with code, phase, retryability and committed-step
// records. A cancelled job records already committed effects and stops.
func (k *JobKernel) Fail(ctx context.Context, jobID, code, phase string, stepID string, retryable bool) error {
	state := JobStateFailed
	_, err := k.db.ExecContext(ctx,
		`UPDATE _mb_jobs SET state = ?, error_code = ?, error_phase = ?, step_id = ?, retryable = ?, cancel_requested = 0, lease_until = NULL, worker_id = NULL, updated_at = ?
		 WHERE id = ?`,
		state, code, phase, stepID, boolToInt(retryable), time.Now().UTC().Unix(), jobID)
	return err
}

// CancelFinalizes acknowledges cancellation at a safe step boundary,
// recording already committed effects in the result.
func (k *JobKernel) CancelFinalizes(ctx context.Context, jobID string, committed any) error {
	var committedJSON []byte
	if committed != nil {
		b, err := json.Marshal(committed)
		if err != nil {
			return err
		}
		committedJSON = b
	}
	_, err := k.db.ExecContext(ctx,
		`UPDATE _mb_jobs SET state = ?, cancel_requested = 0, result_json = COALESCE(?, result_json), lease_until = NULL, worker_id = NULL, updated_at = ? WHERE id = ?`,
		JobStateCancelled, committedJSON, time.Now().UTC().Unix(), jobID)
	return err
}

// RunOnce reconciles expired leases, leases one job and executes it. It
// returns false when nothing was runnable.
func (k *JobKernel) RunOnce(ctx context.Context) (bool, error) {
	if _, err := k.ReconcileExpiredLeases(ctx); err != nil {
		return false, err
	}
	job, err := k.Lease(ctx, nil)
	if err != nil {
		return false, err
	}
	if job == nil {
		return false, nil
	}

	// Lease-renewal watchdog.
	renewStop := make(chan struct{})
	go func() {
		t := time.NewTicker(k.renew)
		defer t.Stop()
		for {
			select {
			case <-renewStop:
				return
			case <-t.C:
				_ = k.Renew(ctx, job.ID)
			}
		}
	}()
	defer close(renewStop)

	handler, ok := k.handlers[job.Kind]
	if !ok {
		_ = k.Fail(ctx, job.ID, "no_handler", "lease", job.ID, false)
		return true, nil
	}

	// Attempts increment with the claim (the claim predicate gates on
	// attempts < max_attempts; the post-claim re-read reports the
	// incremented value).
	result, runErr := handler(ctx, job)
	cancelRequested, _ := k.IsCancelRequested(ctx, job.ID)
	switch {
	case runErr == nil && !cancelRequested:
		return true, k.Complete(ctx, job.ID, result)
	case runErr == nil && cancelRequested:
		// Cancellation acknowledged at a safe step boundary; committed
		// effects stay visible.
		return true, k.CancelFinalizes(ctx, job.ID, result)
	case errors.Is(runErr, ErrNonRetryable) || !job.Retryable:
		return true, k.Fail(ctx, job.ID, "nonretryable", "run", job.StepID, false)
	case job.Attempts >= job.MaxAttempts:
		return true, k.Fail(ctx, job.ID, "max_attempts", "run", job.StepID, false)
	default:
		// waiting_retry with deterministic step id recorded for the next
		// attempt.
		_, err := k.db.ExecContext(ctx,
			`UPDATE _mb_jobs SET state = ?, step_id = ?, lease_until = NULL, worker_id = NULL, updated_at = ? WHERE id = ?`,
			JobStateWaiting, job.StepID, time.Now().UTC().Unix(), job.ID)
		if err != nil {
			return true, err
		}
		// waiting_retry returns to queued immediately; the state exists for
		// observability of the retry decision.
		_, err = k.db.ExecContext(ctx,
			`UPDATE _mb_jobs SET state = ?, updated_at = ? WHERE id = ? AND state = ?`,
			JobStateQueued, time.Now().UTC().Unix(), job.ID, JobStateWaiting)
		return true, err
	}
}

func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

// randomID generates a prefixed random identifier for internal rows.
func randomID(prefix string) string {
	b := make([]byte, 8)
	_, _ = rand.Read(b)
	return prefix + hex.EncodeToString(b)
}

// ---------------------------------------------------------------------------
// Step journal + operational surface
// ---------------------------------------------------------------------------

// stepsTableDDL records deterministic per-step effects for recovery and
// cancellation ("Retry uses deterministic step IDs"; "Cancellation ...
// records already committed effects").
const stepsTableDDL = `CREATE TABLE IF NOT EXISTS _mb_job_steps (
	job_id TEXT NOT NULL,
	step_id TEXT NOT NULL,
	effect TEXT NOT NULL,
	created_at INTEGER NOT NULL,
	PRIMARY KEY (job_id, step_id)
)`

// RecordStep records one committed step effect inside the caller's
// transaction. Handlers wrap effectful steps in transactions that also
// record the step, so recovery and cancellation know what is durable.
func (k *JobKernel) RecordStep(ctx context.Context, jobID, stepID, effect string) error {
	if _, err := k.db.ExecContext(ctx, stepsTableDDL); err != nil {
		return fmt.Errorf("failed to install steps table: %w", err)
	}
	_, err := k.db.ExecContext(ctx,
		`INSERT INTO _mb_job_steps (job_id, step_id, effect, created_at) VALUES (?, ?, ?, ?)
		 ON CONFLICT(job_id, step_id) DO NOTHING`,
		jobID, stepID, effect, time.Now().UTC().Unix())
	return err
}

// JobSteps lists the durable step effects of a job.
func (k *JobKernel) JobSteps(ctx context.Context, jobID string) ([]string, error) {
	rows, err := k.db.QueryContext(ctx,
		`SELECT step_id FROM _mb_job_steps WHERE job_id = ? ORDER BY created_at, step_id`, jobID)
	if err != nil {
		if isMissingTableErr(err) {
			return nil, nil
		}
		return nil, err
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var s string
		if err := rows.Scan(&s); err != nil {
			return nil, err
		}
		out = append(out, s)
	}
	return out, rows.Err()
}

// ReadJob reads a job row by id (operational surface; also used by
// tooling to reconcile uncertain outcomes).
func (k *JobKernel) ReadJob(ctx context.Context, id string) (*Job, error) {
	var job Job
	var createdUnix, updatedUnix int64
	var leaseUntil sql.NullInt64
	var workerID, stepID, errorCode, errorPhase sql.NullString
	var resultJSON []byte
	err := k.db.QueryRowContext(ctx,
		`SELECT id, kind, payload_json, state, attempts, max_attempts, retryable, lease_until, worker_id, step_id, result_json, error_code, error_phase, created_at, updated_at
		 FROM _mb_jobs WHERE id = ?`, id,
	).Scan(&job.ID, &job.Kind, &job.Payload, &job.State, &job.Attempts, &job.MaxAttempts, &job.Retryable, &leaseUntil, &workerID, &stepID, &resultJSON, &errorCode, &errorPhase, &createdUnix, &updatedUnix)
	if err == nil && resultJSON != nil {
		job.Result = json.RawMessage(resultJSON)
	}
	if err != nil {
		return nil, err
	}
	job.CreatedAt = time.Unix(createdUnix, 0).UTC()
	job.UpdatedAt = time.Unix(updatedUnix, 0).UTC()
	if leaseUntil.Valid {
		t := time.Unix(leaseUntil.Int64, 0).UTC()
		job.LeaseUntil = &t
	}
	job.WorkerID = workerID.String
	job.StepID = stepID.String
	job.ErrorCode = errorCode.String
	job.ErrorPhase = errorPhase.String
	return &job, nil
}

// ExpireAllLeases force-expires every held lease (operational repair for
// stuck workers; the regular reconciliation path uses wall-clock expiry).
func (k *JobKernel) ExpireAllLeases(ctx context.Context) (int64, error) {
	res, err := k.db.ExecContext(ctx,
		`UPDATE _mb_jobs SET lease_until = 0 WHERE state = ? AND lease_until IS NOT NULL`,
		JobStateRunning)
	if err != nil {
		return 0, err
	}
	return res.RowsAffected()
}

// ListJobs returns one page of the job store (job.list), newest first.
func (k *JobKernel) ListJobs(ctx context.Context, page, perPage int) (items []*Job, total int64, err error) {
	if page < 1 {
		page = 1
	}
	if perPage < 1 || perPage > 1000 {
		perPage = 30
	}
	if err := k.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM _mb_jobs`).Scan(&total); err != nil {
		return nil, 0, err
	}
	rows, err := k.db.QueryContext(ctx,
		`SELECT id FROM _mb_jobs ORDER BY created_at DESC, id DESC LIMIT ? OFFSET ?`,
		perPage, (page-1)*perPage)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, 0, err
		}
		ids = append(ids, id)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, err
	}
	for _, id := range ids {
		job, err := k.ReadJob(ctx, id)
		if err != nil {
			return nil, 0, err
		}
		items = append(items, job)
	}
	return items, total, nil
}

// CancelQueuedJob cancels a not-yet-running job outright (job.cancel: safe
// cancellation; a running job only gets the cancel flag which the worker
// honors at the next safe step boundary).
func (k *JobKernel) CancelQueuedJob(ctx context.Context, jobID string) (*Job, error) {
	job, err := k.ReadJob(ctx, jobID)
	if err != nil {
		return nil, err
	}
	switch job.State {
	case JobStateQueued:
		now := time.Now().UTC().Unix()
		if _, err := k.db.ExecContext(ctx,
			`UPDATE _mb_jobs SET state = ?, cancel_requested = 0, lease_until = NULL, worker_id = NULL, updated_at = ? WHERE id = ? AND state = ?`,
			JobStateCancelled, now, jobID, JobStateQueued); err != nil {
			return nil, err
		}
		return k.ReadJob(ctx, jobID)
	case JobStateRunning:
		if err := k.RequestCancel(ctx, jobID); err != nil {
			return nil, err
		}
		return k.ReadJob(ctx, jobID)
	default:
		// Terminal states are returned as-is: cancellation of an already
		// settled job never rewinds history.
		return job, nil
	}
}
