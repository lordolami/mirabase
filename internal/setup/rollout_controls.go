package setup

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"mirabase/internal/application"
)

// ---------------------------------------------------------------------------
// MB-ROLLOUT remainder (implementation-closure.md): pause stops new target
// admission and lets running targets complete; resume requires the
// recorded selection/revision and revalidates each target's CURRENT
// generation against the snapshot (changed/deleted targets receive an
// explicit failed result with generation_mismatch — never success);
// retry re-arms only failed targets (succeeded targets are never migrated
// twice); targets pages the exact per-target journal.
// ---------------------------------------------------------------------------

// rolloutControlsDDL is the rollout control state (separate table: the
// normative rollouts DDL is unchanged).
const rolloutControlsDDL = `CREATE TABLE IF NOT EXISTS _mb_schema_rollout_controls (
 rollout_id TEXT PRIMARY KEY REFERENCES _mb_schema_rollouts(id),
 paused INTEGER NOT NULL CHECK(paused IN (0,1)),
 paused_at TEXT,
 resumed_at TEXT
)`

// ErrRolloutNotFound reports a missing rollout row.
var ErrRolloutNotFound = errors.New("not_found")

// PauseRollout stops new target admission (running targets complete;
// the durable journal is untouched).
func (s *Service) PauseRollout(ctx context.Context, rolloutID string) (map[string]any, error) {
	if _, err := s.db.ExecContext(ctx, rolloutControlsDDL); err != nil {
		return nil, err
	}
	var exists int
	if err := s.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM _mb_schema_rollouts WHERE id = ?`, rolloutID).Scan(&exists); err != nil {
		return nil, err
	}
	if exists == 0 {
		return nil, ErrRolloutNotFound
	}
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO _mb_schema_rollout_controls (rollout_id, paused, paused_at) VALUES (?, 1, ?)
		 ON CONFLICT(rollout_id) DO UPDATE SET paused = 1, paused_at = ?`,
		rolloutID, nowRFC(), nowRFC())
	if err != nil {
		return nil, err
	}
	return s.RolloutStatus(ctx, rolloutID)
}

// isRolloutPaused reports the admission gate.
func (s *Service) isRolloutPaused(ctx context.Context, rolloutID string) bool {
	var paused int
	err := s.db.QueryRowContext(ctx,
		`SELECT paused FROM _mb_schema_rollout_controls WHERE rollout_id = ?`, rolloutID).Scan(&paused)
	return err == nil && paused == 1
}

// ResumeRollout clears the pause and revalidates every not-yet-done
// target's CURRENT generation against the recorded snapshot: a target
// whose live generation changed (or that no longer exists) receives an
// explicit failed generation_mismatch result — resume cannot invent a
// pass after the world moved.
func (s *Service) ResumeRollout(ctx context.Context, rolloutID string, dataDir string, handles *application.HandleCache) (map[string]any, error) {
	var revision int64
	err := s.db.QueryRowContext(ctx,
		`SELECT revision FROM _mb_schema_rollouts WHERE id = ?`, rolloutID).Scan(&revision)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrRolloutNotFound
	}
	if err != nil {
		return nil, err
	}
	if _, err := s.db.ExecContext(ctx, rolloutControlsDDL); err != nil {
		return nil, err
	}
	// Revalidate each pending target's current generation.
	rows, err := s.db.QueryContext(ctx,
		`SELECT database_id, generation FROM _mb_schema_rollout_targets WHERE rollout_id = ? AND state IN ('pending','failed')`, rolloutID)
	if err != nil {
		return nil, err
	}
	type pendingRow struct {
		dbID string
		gen  int64
	}
	var pending []pendingRow
	for rows.Next() {
		var r pendingRow
		if err := rows.Scan(&r.dbID, &r.gen); err != nil {
			rows.Close()
			return nil, err
		}
		pending = append(pending, r)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, err
	}
	for _, r := range pending {
		d, derr := s.app.Service.GetDatabase(ctx, r.dbID)
		if derr != nil {
			_, _ = s.db.ExecContext(ctx,
				`UPDATE _mb_schema_rollout_targets SET state = 'failed', error_code = ? WHERE rollout_id = ? AND database_id = ?`,
				"generation_mismatch: the target no longer exists in the registry", rolloutID, r.dbID)
			continue
		}
		if d.Generation != r.gen {
			_, _ = s.db.ExecContext(ctx,
				`UPDATE _mb_schema_rollout_targets SET state = 'failed', error_code = ? WHERE rollout_id = ? AND database_id = ?`,
				fmt.Sprintf("generation_mismatch: snapshot generation %d vs live %d", r.gen, d.Generation), rolloutID, r.dbID)
		}
	}
	_, uerr := s.db.ExecContext(ctx,
		`INSERT INTO _mb_schema_rollout_controls (rollout_id, paused, paused_at, resumed_at) VALUES (?, 0, NULL, ?)
		 ON CONFLICT(rollout_id) DO UPDATE SET paused = 0, paused_at = NULL, resumed_at = ?`,
		rolloutID, nowRFC(), nowRFC())
	if uerr != nil {
		return nil, uerr
	}
	return s.RolloutStatus(ctx, rolloutID)
}

// RetryRollout re-arms ONLY failed targets (succeeded targets are never
// migrated twice); the recorded error remains visible in the journal.
func (s *Service) RetryRollout(ctx context.Context, rolloutID string) (map[string]any, error) {
	var exists int
	if err := s.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM _mb_schema_rollouts WHERE id = ?`, rolloutID).Scan(&exists); err != nil {
		return nil, err
	}
	if exists == 0 {
		return nil, ErrRolloutNotFound
	}
	res, err := s.db.ExecContext(ctx,
		`UPDATE _mb_schema_rollout_targets SET state = 'pending', result_ref = NULL
		 WHERE rollout_id = ? AND state = 'failed'`, rolloutID)
	if err != nil {
		return nil, err
	}
	rearmed, _ := res.RowsAffected()
	st, err := s.RolloutStatus(ctx, rolloutID)
	if err != nil {
		return nil, err
	}
	st["rearmedFailedTargets"] = rearmed
	return st, nil
}

// ListRolloutTargets pages the exact per-target journal (state, recorded
// result reference and error code).
func (s *Service) ListRolloutTargets(ctx context.Context, rolloutID string, page, perPage int) (items []map[string]any, total int64, err error) {
	var exists int
	if err := s.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM _mb_schema_rollouts WHERE id = ?`, rolloutID).Scan(&exists); err != nil {
		return nil, 0, err
	}
	if exists == 0 {
		return nil, 0, ErrRolloutNotFound
	}
	if page < 1 {
		page = 1
	}
	if perPage < 1 || perPage > 1000 {
		perPage = 30
	}
	if err := s.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM _mb_schema_rollout_targets WHERE rollout_id = ?`, rolloutID).Scan(&total); err != nil {
		return nil, 0, err
	}
	rows, err := s.db.QueryContext(ctx,
		`SELECT database_id, generation, state, revision, result_ref, error_code
		 FROM _mb_schema_rollout_targets WHERE rollout_id = ?
		 ORDER BY database_id LIMIT ? OFFSET ?`,
		rolloutID, perPage, (page-1)*perPage)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	items = []map[string]any{}
	for rows.Next() {
		var dbID, state string
		var gen, rev int64
		var resultRef, errCode sql.NullString
		if err := rows.Scan(&dbID, &gen, &state, &rev, &resultRef, &errCode); err != nil {
			return nil, 0, err
		}
		t := map[string]any{
			"databaseId": dbID, "generation": gen, "state": state, "revision": rev,
		}
		if resultRef.Valid {
			t["resultRef"] = resultRef.String
		} else {
			t["resultRef"] = nil
		}
		if errCode.Valid {
			t["errorCode"] = errCode.String
		} else {
			t["errorCode"] = nil
		}
		items = append(items, t)
	}
	return items, total, rows.Err()
}
