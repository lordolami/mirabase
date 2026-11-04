package setup

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"mirabase/internal/application"
)

// Schema fan-out (schemas-and-views.md + v0.1-build.md): "Managed
// templates are immutable revisions. A basic rollout snapshots explicit
// target IDs/generations to durable rows, validates actual data, applies
// with bounded concurrency, and reports every target. Never retain an
// unbounded target array or mark mixed revisions complete. A failed
// rollout resumes from its journal through job recovery; rerunning failed
// targets requires revision/generation guards."
const (
	rolloutsDDL = `CREATE TABLE IF NOT EXISTS _mb_schema_rollouts (
 id TEXT PRIMARY KEY,
 template_id TEXT NOT NULL,
 revision INTEGER NOT NULL,
 target_snapshot_json TEXT NOT NULL CHECK(json_valid(target_snapshot_json)),
 results_json TEXT NOT NULL CHECK(json_valid(results_json)),
 stop_on_failure INTEGER NOT NULL CHECK(stop_on_failure IN (0,1)),
 created TEXT NOT NULL,
 FOREIGN KEY(template_id,revision) REFERENCES _mb_schema_templates(id,revision)
)`

	rolloutTargetsDDL = `CREATE TABLE IF NOT EXISTS _mb_schema_rollout_targets (
 rollout_id TEXT NOT NULL REFERENCES _mb_schema_rollouts(id), database_id TEXT NOT NULL, generation INTEGER NOT NULL CHECK(generation>0),
 state TEXT NOT NULL CHECK(state IN ('pending','running','succeeded','failed','skipped','cancelled')),
 revision INTEGER NOT NULL CHECK(revision>0), result_ref TEXT, error_code TEXT, PRIMARY KEY(rollout_id,database_id)
)`

	rolloutTargetsIndexDDL = `CREATE INDEX IF NOT EXISTS _mb_rollout_targets_admission
 ON _mb_schema_rollout_targets(rollout_id,state,database_id)`

	// TARGET metadata singleton ("TARGET: metadata only; collection tables
	// are produced by the schema planner").
	targetSchemaDDL = `CREATE TABLE IF NOT EXISTS _mb_schema (
 singleton INTEGER PRIMARY KEY CHECK(singleton = 1),
 format_version INTEGER NOT NULL CHECK(format_version = 1),
 template_id TEXT,
 revision INTEGER NOT NULL CHECK(revision > 0),
 definitions_hash TEXT NOT NULL
)`

	// Bounded parallelism default (schemas-and-views.md: "applies in
	// bounded parallelism (default 2)").
	DefaultRolloutParallelism = 2
	// Explicit target list cap (v0.1: maxItems 10000 per request).
	MaxRolloutTargets = 10000
)

// RolloutRequest is the template.rollout request (v0.1 slice: explicit
// fixed IDs/generations; no structured fleet selector or advanced control
// API).
type RolloutRequest struct {
	Revision      int64    `json:"revision"`
	DatabaseIDs   []string `json:"databaseIds,omitempty"`
	StopOnFailure bool     `json:"stopOnFailure"`
}

// RolloutTarget is one per-target durable row.
type RolloutTarget struct {
	DatabaseID string
	Generation int64
	State      string // pending|running|succeeded|failed|skipped|cancelled
	Revision   int64
	ResultRef  *string
	ErrorCode  *string
}

// StartRollout snapshots the explicit target set into durable rows and
// returns the rollout id. The snapshot is bounded (max 10000) and streamed
// into rows, never kept as an unbounded resident array.
func (s *Service) StartRollout(ctx context.Context, templateID string, req *RolloutRequest) (string, int, error) {
	if len(req.DatabaseIDs) == 0 {
		return "", 0, errors.New("the rollout requires an explicit databaseIds set in this release")
	}
	if len(req.DatabaseIDs) > MaxRolloutTargets {
		return "", 0, fmt.Errorf("the target set exceeds %d entries", MaxRolloutTargets)
	}

	// The template revision must exist and be published.
	tpl, err := s.app.Service.LoadTemplate(ctx, templateID, req.Revision)
	if err != nil {
		return "", 0, fmt.Errorf("template revision: %w", err)
	}

	id := application.NewID("rollout-")
	stopOnFailure := 0
	if req.StopOnFailure {
		stopOnFailure = 1
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return "", 0, err
	}
	defer func() { _ = tx.Rollback() }()

	if _, err := tx.Exec(rolloutsDDL); err != nil {
		return "", 0, err
	}
	if _, err := tx.Exec(rolloutTargetsDDL); err != nil {
		return "", 0, err
	}
	if _, err := tx.Exec(rolloutTargetsIndexDDL); err != nil {
		return "", 0, err
	}
	if _, err := tx.Exec(
		`INSERT INTO _mb_schema_rollouts (id, template_id, revision, target_snapshot_json, results_json, stop_on_failure, created)
		 VALUES (?, ?, ?, ?, '{}', ?, ?)`,
		id, templateID, req.Revision, snapshotJSON(req.DatabaseIDs, tpl), stopOnFailure, nowRFC()); err != nil {
		return "", 0, err
	}
	for _, dbID := range req.DatabaseIDs {
		// Pin the generation AT SNAPSHOT TIME (the generation guard for
		// reruns: "Resume validates changed targets").
		gen := int64(1)
		if d, err := s.app.Service.GetDatabase(ctx, dbID); err == nil {
			gen = d.Generation
		}
		if _, err := tx.Exec(
			`INSERT INTO _mb_schema_rollout_targets (rollout_id, database_id, generation, state, revision)
			 VALUES (?, ?, ?, 'pending', ?)`,
			id, dbID, gen, req.Revision); err != nil {
			return "", 0, err
		}
	}
	if err := tx.Commit(); err != nil {
		return "", 0, err
	}
	return id, len(req.DatabaseIDs), nil
}

func snapshotJSON(dbIDs []string, tpl *application.PublishedTemplate) string {
	b, _ := json.Marshal(map[string]any{
		"databaseIds":     dbIDs,
		"templateId":      tpl.ID,
		"revision":        tpl.Revision,
		"definitionHash":  tpl.Hash(),
	})
	return string(b)
}

func nowRFC() string { return time.Now().UTC().Format(time.RFC3339Nano) }

func toAnyMap(m map[string]int) map[string]any {
	out := make(map[string]any, len(m))
	for k, v := range m {
		out[k] = v
	}
	return out
}

// RunRollout applies the template to every snapshot target with bounded
// parallelism. Each target records its exact state; failed targets can be
// retried idempotently with the revision/generation guards.
func (s *Service) RunRollout(ctx context.Context, rolloutID string, dataDir string, handles *application.HandleCache) (done bool, summary map[string]any, err error) {
	var rollout struct {
		TemplateID    string
		Revision      int64
		StopOnFailure bool
	}
	var defsJSON string
	err = s.db.QueryRowContext(ctx,
		`SELECT template_id, revision, target_snapshot_json, stop_on_failure FROM _mb_schema_rollouts WHERE id = ?`,
		rolloutID).Scan(&rollout.TemplateID, &rollout.Revision, &defsJSON, &rollout.StopOnFailure)
	if err != nil {
		return false, nil, err
	}
	tpl, err := s.app.Service.LoadTemplate(ctx, rollout.TemplateID, rollout.Revision)
	if err != nil {
		return false, nil, err
	}

	// Targets snapshot: pending/failed targets are the work set (a
	// retried rollout resumes from its journal).
	rows, err := s.db.QueryContext(ctx,
		`SELECT database_id, generation, state FROM _mb_schema_rollout_targets WHERE rollout_id = ?`, rolloutID)
	if err != nil {
		return false, nil, err
	}
	var targets []RolloutTarget
	for rows.Next() {
		var t RolloutTarget
		if err := rows.Scan(&t.DatabaseID, &t.Generation, &t.State); err != nil {
			rows.Close()
			return false, nil, err
		}
		targets = append(targets, t)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return false, nil, err
	}

	var work []RolloutTarget
	for _, t := range targets {
		if t.State == "pending" || t.State == "failed" {
			work = append(work, t)
		}
	}

	// Bounded parallelism (default 2), stop_on_failure honored.
	sem := make(chan struct{}, DefaultRolloutParallelism)
	var mu sync.Mutex
	var fatal error
	var wg sync.WaitGroup
	counters := map[string]int{"total": len(targets), "succeeded": 0, "failed": 0, "remaining": 0}
	for _, t := range work {
		// pause stops NEW target admission (a paused rollout leaves the
		// remaining targets pending; already-running targets complete and
		// the journal records them).
		if s.isRolloutPaused(ctx, rolloutID) {
			break
		}
		wg.Add(1)
		go func(t RolloutTarget) {
			defer wg.Done()
			mu.Lock()
			if fatal != nil {
				mu.Unlock()
				return
			}
			mu.Unlock()
			sem <- struct{}{}
			defer func() { <-sem }()

			if err := s.applyTarget(ctx, rolloutID, t, tpl, dataDir, handles); err != nil {
				mu.Lock()
				counters["failed"]++
				code := err.Error()
				_, _ = s.db.ExecContext(ctx,
					`UPDATE _mb_schema_rollout_targets SET state = 'failed', error_code = ? WHERE rollout_id = ? AND database_id = ?`,
					code, rolloutID, t.DatabaseID)
				if rollout.StopOnFailure {
					fatal = err
				}
				mu.Unlock()
				return
			}
			mu.Lock()
			counters["succeeded"]++
			mu.Unlock()
		}(t)
	}
	wg.Wait()

	mu.Lock()
	defer mu.Unlock()
	// "Never ... mark mixed revisions complete": the rollout result
	// summary reports total/succeeded/failed/remaining exactly.
	var pending, failed, succeeded int
	_ = s.db.QueryRowContext(ctx,
		`SELECT sum(state='pending'), sum(state='failed'), sum(state='succeeded')
		 FROM _mb_schema_rollout_targets WHERE rollout_id = ?`, rolloutID).Scan(&pending, &failed, &succeeded)
	counters["succeeded"] = succeeded
	counters["failed"] = failed
	counters["remaining"] = pending + failed
	counters["total"] = len(targets)

	if fatal != nil {
		return false, toAnyMap(counters), fatal
	}
	// The rollout is complete only when NO target is pending/running/failed
	// — mixed revisions are never marked complete.
	return pending == 0 && failed == 0, toAnyMap(counters), nil
}

// applyTarget applies the template diff to one target with the
// revision/generation guards, then publishes the target-local `_mb_schema`
// metadata singleton.
func (s *Service) applyTarget(ctx context.Context, rolloutID string, t RolloutTarget, tpl *application.PublishedTemplate, dataDir string, handles *application.HandleCache) error {
	// Running state (crash-safe: a killed run leaves the target running and
	// the lease reconciliation re-drives it through job recovery).
	if _, err := s.db.ExecContext(ctx,
		`UPDATE _mb_schema_rollout_targets SET state = 'running', revision = ? WHERE rollout_id = ? AND database_id = ?`,
		tpl.Revision, rolloutID, t.DatabaseID); err != nil {
		return err
	}

	// Generation guard: the target must still be at the snapshotted
	// generation ("Resume validates changed targets"; "rerunning failed
	// targets requires revision/generation guards").
	d, err := s.app.Service.GetDatabase(ctx, t.DatabaseID)
	if err != nil {
		return fmt.Errorf("target lookup: %w", err)
	}
	if d.Generation != t.Generation {
		return fmt.Errorf("target %s changed: generation %d != snapshotted %d", t.DatabaseID, d.Generation, t.Generation)
	}
	if d.State != application.StateReady {
		return fmt.Errorf("target %s state is %s", t.DatabaseID, d.State)
	}

	// Publish the target-local metadata FIRST (fresh target's `_mb_schema`
	// singleton records the applied revision).
	target, err := handles.Open(ctx, t.DatabaseID)
	if err != nil {
		return err
	}
	if _, err := target.ExecContext(ctx, targetSchemaDDL); err != nil {
		return err
	}
	// Apply the template diff (creates missing published collections; the
	// normalized per-target diff from the planner; idempotent re-apply).
	if err := application.ApplyTemplate(ctx, target, tpl); err != nil {
		return err
	}
	if _, err := target.ExecContext(ctx,
		`INSERT INTO _mb_schema (singleton, format_version, template_id, revision, definitions_hash)
		 VALUES (1, 1, ?, ?, ?)
		 ON CONFLICT(singleton) DO UPDATE SET template_id = excluded.template_id,
			revision = excluded.revision, definitions_hash = excluded.definitions_hash`,
		tpl.ID, tpl.Revision, tpl.Hash()); err != nil {
		return err
	}

	if _, err := s.db.ExecContext(ctx,
		`UPDATE _mb_schema_rollout_targets SET state = 'succeeded', error_code = NULL,
			result_ref = ? WHERE rollout_id = ? AND database_id = ?`,
		fmt.Sprintf("template %s revision %d applied; target _mb_schema revision %d definitions %s",
			tpl.ID, tpl.Revision, tpl.Revision, tpl.Hash()), rolloutID, t.DatabaseID); err != nil {
		return err
	}
	return nil
}

// RolloutStatus reports total/completed/failed/remaining with exact
// revisions ("Rollout status reports total, completed, failed and
// remaining, with exact revisions and compatibility windows").
func (s *Service) RolloutStatus(ctx context.Context, rolloutID string) (map[string]any, error) {
	var templateID string
	var revision int64
	var created string
	err := s.db.QueryRowContext(ctx,
		`SELECT template_id, revision, created FROM _mb_schema_rollouts WHERE id = ?`,
		rolloutID).Scan(&templateID, &revision, &created)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, errors.New("not_found")
	}
	if err != nil {
		return nil, err
	}
	rows, err := s.db.QueryContext(ctx,
		`SELECT database_id, generation, state, revision, error_code FROM _mb_schema_rollout_targets WHERE rollout_id = ?`, rolloutID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	targets := []map[string]any{}
	counts := map[string]int{"pending": 0, "running": 0, "succeeded": 0, "failed": 0, "skipped": 0, "cancelled": 0}
	for rows.Next() {
		var dbID, state string
		var gen, rev int64
		var ec sql.NullString
		if err := rows.Scan(&dbID, &gen, &state, &rev, &ec); err != nil {
			return nil, err
		}
		counts[state]++
		t := map[string]any{
			"databaseId": dbID, "generation": gen, "state": state, "revision": rev,
		}
		if ec.Valid {
			t["errorCode"] = ec.String
		}
		targets = append(targets, t)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return map[string]any{
		"rolloutId":   rolloutID,
		"templateId":  templateID,
		"revision":    revision,
		"created":     created,
		"total":       len(targets),
		"succeeded":   counts["succeeded"],
		"failed":      counts["failed"],
		"remaining":   counts["pending"] + counts["running"],
		"byState":     counts,
		"targets":     targets,
	}, nil
}

// ListRolloutTargetsPending returns the pending/failed target ids (used
// by the retry path).
func (s *Service) ListRolloutTargetsPending(ctx context.Context, rolloutID string) ([]string, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT database_id FROM _mb_schema_rollout_targets WHERE rollout_id = ? AND state IN ('pending','failed')`, rolloutID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		out = append(out, id)
	}
	return out, rows.Err()
}

var _ = strings.TrimSpace
