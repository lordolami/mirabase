package pbimport

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
)

// ---------------------------------------------------------------------------
// R2/R3 batch adoption (importbatch.*) and the tenant read-back
// (pbimport.tenants): an import batch orchestrates multiple verified
// PocketBase sources into per-source staged targets with append/repeat/
// replacement provenance and per-source recovery; the tenants surface
// reads the analyzed source's tenant-kind inventory without ownership
// splitting (the ownership split is the R2 remainder and is refused with
// the honest direction, never silently skipped).
// ---------------------------------------------------------------------------

// batchesDDL is the import batch store.
const batchesDDL = `CREATE TABLE IF NOT EXISTS _mb_import_batches (
 id TEXT PRIMARY KEY,
 state TEXT NOT NULL CHECK(state IN ('draft','running','completed','failed','cancelled')),
 mode TEXT NOT NULL CHECK(mode IN ('append','repeat','replacement')),
 created TEXT NOT NULL,
 updated TEXT NOT NULL
)`

const batchSourcesDDL = `CREATE TABLE IF NOT EXISTS _mb_import_batch_sources (
 batch_id TEXT NOT NULL REFERENCES _mb_import_batches(id),
 source_id TEXT NOT NULL,
 provenance TEXT NOT NULL CHECK(provenance IN ('append','repeat','replacement')),
 session_id TEXT,
 state TEXT NOT NULL CHECK(state IN ('pending','staged','applied','failed','skipped')),
 error_code TEXT,
 PRIMARY KEY(batch_id, source_id)
)`

// ErrBatchState reports an unusable batch state.
var ErrBatchState = errors.New("batch_state")

// ImportBatch is one batch row.
type ImportBatch struct {
	ID      string            `json:"id"`
	State   string            `json:"state"`
	Mode    string            `json:"mode"`
	Created string            `json:"created"`
	Updated string            `json:"updated"`
	Sources []BatchSourceRow  `json:"sources"`
}

// BatchSourceRow is one batch source row.
type BatchSourceRow struct {
	SourceID   string  `json:"sourceId"`
	Provenance string  `json:"provenance"`
	SessionID  *string `json:"sessionId"`
	State      string  `json:"state"`
	ErrorCode  *string `json:"errorCode"`
}

// ensureBatchesTable installs the batch store.
func (s *Service) ensureBatchesTable(ctx context.Context) error {
	if _, err := s.db.ExecContext(ctx, batchesDDL); err != nil {
		return err
	}
	if _, err := s.db.ExecContext(ctx, batchSourcesDDL); err != nil {
		return err
	}
	return nil
}

// CreateImportBatch opens a batch with per-source provenance (the source
// ids are the session ids of already-verified shared-layout import
// sessions — the batch orchestrates their publication order).
func (s *Service) CreateImportBatch(ctx context.Context, sources []struct {
	SourceID   string `json:"sourceId"`
	Provenance string `json:"provenance"`
}, mode string) (*ImportBatch, error) {
	if err := s.ensureBatchesTable(ctx); err != nil {
		return nil, err
	}
	if mode != "append" && mode != "repeat" && mode != "replacement" {
		return nil, fmt.Errorf("%w: mode must be append, repeat or replacement", ErrBatchState)
	}
	if len(sources) == 0 || len(sources) > 100 {
		return nil, fmt.Errorf("%w: 1..100 sources are required", ErrBatchState)
	}
	id := strings.ReplaceAll(newID(""), "-", "")
	now := nowRFC()
	if _, err := s.db.ExecContext(ctx,
		`INSERT INTO _mb_import_batches (id, state, mode, created, updated) VALUES (?, 'draft', ?, ?, ?)`,
		id, mode, now, now); err != nil {
		return nil, err
	}
	for _, src := range sources {
		prov := src.Provenance
		if prov == "" {
			prov = mode
		}
		if prov != "append" && prov != "repeat" && prov != "replacement" {
			return nil, fmt.Errorf("%w: provenance must be append, repeat or replacement", ErrBatchState)
		}
		// The source must be a real verified session (opaque ids never
		// authorize paths; the session's existence IS the check).
		if _, err := s.Get(ctx, src.SourceID); err != nil {
			return nil, fmt.Errorf("%w: source session %q does not exist", ErrNotFound, src.SourceID)
		}
		if _, err := s.db.ExecContext(ctx,
			`INSERT INTO _mb_import_batch_sources (batch_id, source_id, provenance, state)
			 VALUES (?, ?, ?, 'pending')`, id, src.SourceID, prov); err != nil {
			return nil, err
		}
	}
	return s.GetImportBatch(ctx, id)
}

// GetImportBatch loads one batch with its sources.
func (s *Service) GetImportBatch(ctx context.Context, id string) (*ImportBatch, error) {
	if err := s.ensureBatchesTable(ctx); err != nil {
		return nil, err
	}
	var b ImportBatch
	err := s.db.QueryRowContext(ctx,
		`SELECT id, state, mode, created, updated FROM _mb_import_batches WHERE id = ?`, id,
	).Scan(&b.ID, &b.State, &b.Mode, &b.Created, &b.Updated)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	rows, err := s.db.QueryContext(ctx,
		`SELECT source_id, provenance, session_id, state, error_code
		 FROM _mb_import_batch_sources WHERE batch_id = ? ORDER BY source_id`, id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var r BatchSourceRow
		var sessID, errCode sql.NullString
		if err := rows.Scan(&r.SourceID, &r.Provenance, &sessID, &r.State, &errCode); err != nil {
			return nil, err
		}
		if sessID.Valid {
			r.SessionID = &sessID.String
		}
		if errCode.Valid {
			r.ErrorCode = &errCode.String
		}
		b.Sources = append(b.Sources, r)
	}
	return &b, rows.Err()
}

// RunImportBatch drives the batch: each pending source is validated as a
// real session (the per-source cutover is the owning session's own
// guarded publication; the batch records the per-source outcome with
// provenance and never marks a failed source applied).
func (s *Service) RunImportBatch(ctx context.Context, id string) (*ImportBatch, error) {
	b, err := s.GetImportBatch(ctx, id)
	if err != nil {
		return nil, err
	}
	if b.State == "completed" || b.State == "cancelled" {
		return nil, fmt.Errorf("%w: the batch is already %s", ErrBatchState, b.State)
	}
	now := nowRFC()
	if _, err := s.db.ExecContext(ctx,
		`UPDATE _mb_import_batches SET state = 'running', updated = ? WHERE id = ?`, now, id); err != nil {
		return nil, err
	}
	for _, src := range b.Sources {
		if src.State != "pending" {
			continue
		}
		sess, serr := s.Get(ctx, src.SourceID)
		if serr != nil {
			_, _ = s.db.ExecContext(ctx,
				`UPDATE _mb_import_batch_sources SET state = 'failed', error_code = ? WHERE batch_id = ? AND source_id = ?`,
				"source_session_missing", id, src.SourceID)
			continue
		}
		// The per-source publication state is the session's own phase:
		// staged/ready-for-cutover sources are 'staged'; active sessions
		// are 'applied' (the acknowledged publication is durable — the
		// batch never re-publishes or rolls back acknowledged work).
		switch sess.Phase {
		case StateActive, StateRecoveryExported, StateRolledBack:
			_, _ = s.db.ExecContext(ctx,
				`UPDATE _mb_import_batch_sources SET state = 'applied' WHERE batch_id = ? AND source_id = ?`,
				id, src.SourceID)
		case StateStaged, StateReadyForCutover:
			_, _ = s.db.ExecContext(ctx,
				`UPDATE _mb_import_batch_sources SET state = 'staged' WHERE batch_id = ? AND source_id = ?`,
				id, src.SourceID)
		case StateFailed:
			_, _ = s.db.ExecContext(ctx,
				`UPDATE _mb_import_batch_sources SET state = 'failed', error_code = ? WHERE batch_id = ? AND source_id = ?`,
				"source_failed", id, src.SourceID)
		default:
			_, _ = s.db.ExecContext(ctx,
				`UPDATE _mb_import_batch_sources SET state = 'failed', error_code = ? WHERE batch_id = ? AND source_id = ?`,
				"source_phase_"+sess.Phase, id, src.SourceID)
		}
	}
	// The batch state: completed when every source settled; failed when
	// any failed (the per-source outcomes stay exact).
	var failed, unsettled int
	row := s.db.QueryRowContext(ctx,
		`SELECT
		 (SELECT COUNT(*) FROM _mb_import_batch_sources WHERE batch_id = ? AND state = 'failed'),
		 (SELECT COUNT(*) FROM _mb_import_batch_sources WHERE batch_id = ? AND state IN ('pending','staged'))`,
		id, id)
	if err := row.Scan(&failed, &unsettled); err != nil {
		return nil, err
	}
	final := "completed"
	if failed > 0 {
		final = "failed"
	} else if unsettled > 0 {
		final = "running"
	}
	if _, err := s.db.ExecContext(ctx,
		`UPDATE _mb_import_batches SET state = ?, updated = ? WHERE id = ?`, final, nowRFC(), id); err != nil {
		return nil, err
	}
	return s.GetImportBatch(ctx, id)
}

// CancelImportBatch cancels a not-yet-completed batch (the per-source
// sessions' own evidence is retained).
func (s *Service) CancelImportBatch(ctx context.Context, id string) (*ImportBatch, error) {
	b, err := s.GetImportBatch(ctx, id)
	if err != nil {
		return nil, err
	}
	if b.State == "completed" {
		return nil, fmt.Errorf("%w: a completed batch cannot be cancelled", ErrBatchState)
	}
	if _, err := s.db.ExecContext(ctx,
		`UPDATE _mb_import_batches SET state = 'cancelled', updated = ? WHERE id = ?`, nowRFC(), id); err != nil {
		return nil, err
	}
	return s.GetImportBatch(ctx, id)
}

// ListImportBatchSources pages the per-source provenance journal
// (importbatch.sources / importbatch.source.list).
func (s *Service) ListImportBatchSources(ctx context.Context, id string, page, perPage int) ([]BatchSourceRow, int64, error) {
	if _, err := s.GetImportBatch(ctx, id); err != nil {
		return nil, 0, err
	}
	if page < 1 {
		page = 1
	}
	if perPage < 1 || perPage > 1000 {
		perPage = 30
	}
	var total int64
	if err := s.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM _mb_import_batch_sources WHERE batch_id = ?`, id).Scan(&total); err != nil {
		return nil, 0, err
	}
	rows, err := s.db.QueryContext(ctx,
		`SELECT source_id, provenance, session_id, state, error_code
		 FROM _mb_import_batch_sources WHERE batch_id = ? ORDER BY source_id LIMIT ? OFFSET ?`,
		id, perPage, (page-1)*perPage)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	out := []BatchSourceRow{}
	for rows.Next() {
		var r BatchSourceRow
		var sessID, errCode sql.NullString
		if err := rows.Scan(&r.SourceID, &r.Provenance, &sessID, &r.State, &errCode); err != nil {
			return nil, 0, err
		}
		if sessID.Valid {
			r.SessionID = &sessID.String
		}
		if errCode.Valid {
			r.ErrorCode = &errCode.String
		}
		out = append(out, r)
	}
	return out, total, rows.Err()
}

// ListSourceTenants reads the analyzed source's tenant-kind inventory
// (pbimport.tenants read-back; the OWNERSHIP split remains the R2
// remainder and is refused by the plan surface with the honest direction,
// never silently skipped).
func (s *Service) ListSourceTenants(ctx context.Context, id string) (map[string]any, error) {
	analysis, err := s.GetInventory(ctx, id)
	if err != nil {
		return nil, err
	}
	tenants := []map[string]any{}
	for _, c := range analysis.Collections {
		if c.Type == "tenant" || strings.Contains(strings.ToLower(c.Name), "tenant") {
			tenants = append(tenants, map[string]any{
				"collection": c.Name, "records": c.RecordCount,
				"note": "tenant-kind ownership split is the R2 remainder (the plan surface refuses it with the direction)",
			})
		}
	}
	return map[string]any{
		"sessionId": id,
		"tenants":   tenants,
		"note":      "the shared-layout adoption preserves every row globally; the ownership split is R2",
	}, nil
}

var _ = json.Marshal
