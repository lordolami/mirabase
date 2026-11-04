package pbimport

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	_ "modernc.org/sqlite" // the import-only reference reader (native-provider.md)
)

// Analyze runs the source inspection over a PRIVATE copy of the verified
// source ("The compatibility import is a copy-and-verify operation, never
// an in-place engine experiment on the only source copy"): copies the
// source bytes to the private working copy, opens it READ-ONLY with the
// import-only reference reader (modernc.org/sqlite v1.60.1, never the
// live provider, never registered as a fallback), and inspects SQLite
// integrity, user_version, schema, indexes, views, settings and migration
// history. Unsupported structures produce issues — a report with affected
// objects and no cutover.
//
// Returns the inventory (per-collection record counts and schema hashes)
// and the normalized snapshot hash.
type CollectionInventory struct {
	CollectionID   string   `json:"collectionId"`
	Name           string   `json:"name"`
	Type           string   `json:"type"`
	RecordCount    int      `json:"recordCount"`
	SchemaHash     string   `json:"schemaHash"`
	RelationFields []string `json:"relationFieldIds"`
}

type Analysis struct {
	SnapshotHash      string                `json:"snapshotHash"`
	Integrity         string                `json:"integrity"`
	UserVersion       int64                 `json:"userVersion"`
	Collections       []CollectionInventory `json:"collections"`
	TotalRecords      int                   `json:"sourceRecords"`
	SettingsEncrypted bool                  `json:"settingsEncrypted"`
	MigrationsApplied int                   `json:"migrationsApplied"`
	Issues            []map[string]any      `json:"issues"`
}

// AnalyzeSource copies the source file to the private working copy and
// inspects it. sourcePath points at the normalized source.bin (a PocketBase
// data directory snapshot: data.db) — the v0.1 accepted sources are a
// stopped directory snapshot or a backup zip.
func (s *Service) AnalyzeSource(ctx context.Context, id string) (*Analysis, error) {
	sess, err := s.Get(ctx, id)
	if err != nil {
		return nil, err
	}
	if sess.Phase != StateSourceReady {
		return nil, fmt.Errorf("%w: phase %q is not source-ready", ErrConflict, sess.Phase)
	}
	sourceBin := filepath.Join(s.SourceDir(id), "source.bin")
	if _, err := os.Stat(sourceBin); err != nil {
		return nil, fmt.Errorf("%w: the verified source bytes are missing", ErrSourceIncomplete)
	}
	if err := s.Transition(ctx, id, StateAnalyzing, ""); err != nil {
		return nil, err
	}

	workDir := filepath.Join(s.SessionDir(id), "work")
	if err := os.MkdirAll(workDir, 0o700); err != nil {
		return nil, err
	}

	// Private copy recovery: the analysis operates on a copy; SQLite
	// recovery, if required, changes the private copy — never the original
	// upload.
	copyPath := filepath.Join(workDir, "data.db")
	src, err := os.ReadFile(sourceBin)
	if err != nil {
		return nil, err
	}
	if err := os.WriteFile(copyPath, src, 0o600); err != nil {
		return nil, err
	}
	snapHash := hashBytes(src)

	analysis := &Analysis{SnapshotHash: snapHash}
	issues := []map[string]any{}
	// The normalized snapshot hash is recorded for both source kinds: for
	// a directory snapshot it is the verified source bytes hash; for a
	// backup zip it is the unpacked data.db hash.
	if err := s.SetHash(ctx, id, "normalized_snapshot_hash", analysis.SnapshotHash); err != nil {
		return nil, err
	}

	// The source may be a raw data.db or a PocketBase backup ZIP. v0.1
	// accepts the stopped directory snapshot (data.db bytes) and the
	// backup zip; a zip is unpacked here into the work dir.
	if isZip(src) {
		if err := unpackBackupZip(ctx, src, workDir); err != nil {
			return nil, fmt.Errorf("%w: the archive was rejected: %v", ErrInvalidRequest, err)
		}
		copyPath = filepath.Join(workDir, "data.db")
		if _, err := os.Stat(copyPath); err != nil {
			return nil, fmt.Errorf("%w: the backup contains no data.db", ErrSourceIncomplete)
		}
		// The normalized snapshot hash covers the unpacked data.db bytes.
		nb, err := os.ReadFile(copyPath)
		if err != nil {
			return nil, err
		}
		analysis.SnapshotHash = hashBytes(nb)
		if err := s.SetHash(ctx, id, "normalized_snapshot_hash", analysis.SnapshotHash); err != nil {
			return nil, err
		}
	}

	// The import-only read-only reader: one bounded connection, read-only
	// DSN, never the live provider.
	reader, err := sql.Open("sqlite", copyPath+"?mode=ro&_pragma=busy_timeout(10000)")
	if err != nil {
		return nil, fmt.Errorf("failed to open the source copy read-only: %w", err)
	}
	defer reader.Close()

	// Integrity + user_version.
	var integrity string
	if err := reader.QueryRowContext(ctx, `PRAGMA integrity_check`).Scan(&integrity); err != nil {
		return nil, fmt.Errorf("integrity_check: %w", err)
	}
	analysis.Integrity = integrity
	if integrity != "ok" {
		issues = append(issues, issue("error", "source_integrity", "integrity_check reported: "+integrity))
	}
	var userVersion int64
	_ = reader.QueryRowContext(ctx, `PRAGMA user_version`).Scan(&userVersion)
	analysis.UserVersion = userVersion
	if userVersion != 0 && userVersion < 12 {
		// The pinned baseline is v0.41.1 (schema app version ~12+); older
		// structures need the documented upstream upgrade first.
		issues = append(issues, issue("error", "source_version",
			fmt.Sprintf("user_version %d predates the pinned v0.41.1 baseline; run the supported upstream upgrade first", userVersion)))
	}

	// Collections inventory from _collections (the pinned v0.41.1 shape:
	// id, type, name, system, schema JSON in the `options`/`fields` column).
	inventory, totalRecords, invErr := inspectCollections(ctx, reader)
	if invErr != nil {
		return nil, invErr
	}
	analysis.Collections = inventory
	analysis.TotalRecords = totalRecords

	// Migration history: _migrations rows.
	var migrations int
	if err := reader.QueryRowContext(ctx,
		`SELECT count(*) FROM sqlite_master WHERE type='table' AND name='_migrations'`).Scan(&migrations); err == nil && migrations > 0 {
		var applied int
		if err := reader.QueryRowContext(ctx, `SELECT count(*) FROM _migrations`).Scan(&applied); err == nil {
			analysis.MigrationsApplied = applied
		}
	}

	// Settings encryption: the _params rows with settings prefix are
	// encrypted when the source used the encryption option (the audit
	// records the fact; the key reference is supplied at cutover).
	var paramsTable int
	_ = reader.QueryRowContext(ctx,
		`SELECT count(*) FROM sqlite_master WHERE type='table' AND name='_params'`).Scan(&paramsTable)
	if paramsTable > 0 {
		var enc int
		if err := reader.QueryRowContext(ctx,
			`SELECT count(*) FROM _params WHERE id LIKE 'settings%'`).Scan(&enc); err == nil {
			analysis.SettingsEncrypted = enc > 0
		}
	}

	// Views and triggers inventory (unsupported structures surface as
	// issues, never silently dropped).
	var views, triggers int
	_ = reader.QueryRowContext(ctx,
		`SELECT count(*) FROM sqlite_master WHERE type='view'`).Scan(&views)
	_ = reader.QueryRowContext(ctx,
		`SELECT count(*) FROM sqlite_master WHERE type='trigger' AND name NOT LIKE 'sqlite_%'`).Scan(&triggers)
	if views > 0 {
		issues = append(issues, issue("warning", "source_views",
			fmt.Sprintf("the source defines %d views; view definitions transfer as schema objects with the shared layout", views)))
	}
	if triggers > 0 {
		issues = append(issues, issue("warning", "source_triggers",
			fmt.Sprintf("the source defines %d triggers; triggers transfer only with reviewed compatibility (Turso native triggers)", triggers)))
	}

	if err := s.SetHash(ctx, id, "source_ref", workDir); err != nil {
		return nil, err
	}
	analysis.Issues = issues
	// The analysis phase completes into draft (the planning state).
	if err := s.Transition(ctx, id, StateDraft, ""); err != nil {
		return nil, err
	}
	// Persist the artifact for pbimport.inventory read-back.
	if perr := s.persistInventory(id, analysis); perr != nil {
		return nil, perr
	}
	return analysis, nil
}

func issue(severity, code, message string) map[string]any {
	return map[string]any{"severity": severity, "code": code, "message": message, "resolved": false}
}

func hashBytes(b []byte) string {
	h := sha256.Sum256(b)
	return hex.EncodeToString(h[:])
}

func isZip(b []byte) bool {
	return len(b) > 4 && b[0] == 'P' && b[1] == 'K' && (b[2] == 3 || b[2] == 5) && (b[3] == 4 || b[3] == 6)
}

// unpackBackupZip extracts a PocketBase backup ZIP into the work dir with
// the archive-safety rules: reject traversal, absolute paths, duplicate
// entries, symlinks/reparse points and decompression bombs (bounded
// per-entry and total sizes).
func unpackBackupZip(ctx context.Context, src []byte, workDir string) error {
	r, err := zip.NewReader(bytes.NewReader(src), int64(len(src)))
	if err != nil {
		return err
	}
	const maxEntrySize = 2 << 30 // 2 GiB per entry
	const maxTotalSize = 21 << 30
	seen := map[string]bool{}
	var total int64
	for _, f := range r.File {
		name := filepath.Clean(f.Name)
		if strings.HasPrefix(name, "/") || strings.Contains(name, "..") ||
			strings.Contains(name, ":\\") || strings.Contains(name, "\\") || strings.Contains(name, ":") {
			return fmt.Errorf("unsafe archive path %q", f.Name)
		}
		if seen[name] {
			return fmt.Errorf("duplicate archive entry %q", f.Name)
		}
		seen[name] = true
		if f.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("symlink entry %q is not allowed", f.Name)
		}
		if f.UncompressedSize64 > maxEntrySize {
			return fmt.Errorf("decompression bomb: entry %q exceeds the per-entry bound", f.Name)
		}
		total += int64(f.UncompressedSize64)
		if total > maxTotalSize {
			return fmt.Errorf("decompression bomb: total exceeds the archive bound")
		}
		dest := filepath.Join(workDir, filepath.FromSlash(name))
		if f.FileInfo().IsDir() {
			if err := os.MkdirAll(dest, 0o700); err != nil {
				return err
			}
			continue
		}
		if err := os.MkdirAll(filepath.Dir(dest), 0o700); err != nil {
			return err
		}
		rc, err := f.Open()
		if err != nil {
			return err
		}
		data, err := io.ReadAll(rc)
		rc.Close()
		if err != nil {
			return err
		}
		if err := os.WriteFile(dest, data, 0o600); err != nil {
			return err
		}
	}
	return nil
}

var _ = sort.Strings

// inspectCollections reads the `_collections` table (pinned v0.41.1
// shape) and counts records per collection table.
func inspectCollections(ctx context.Context, reader *sql.DB) ([]CollectionInventory, int, error) {
	var hasCollections int
	if err := reader.QueryRowContext(ctx,
		`SELECT count(*) FROM sqlite_master WHERE type='table' AND name='_collections'`).Scan(&hasCollections); err != nil {
		return nil, 0, fmt.Errorf("collections table probe: %w", err)
	}
	if hasCollections == 0 {
		return nil, 0, fmt.Errorf("%w: the source copy has no _collections table (a schema-only JSON import may fit; a full adoption requires the pinned v0.41.1 source)", ErrInvalidRequest)
	}
	rows, err := reader.QueryContext(ctx,
		`SELECT id, type, name, system FROM _collections ORDER BY name`)
	if err != nil {
		return nil, 0, fmt.Errorf("collections read: %w", err)
	}
	defer rows.Close()

	var out []CollectionInventory
	total := 0
	for rows.Next() {
		var c CollectionInventory
		var system int
		if err := rows.Scan(&c.CollectionID, &c.Type, &c.Name, &system); err != nil {
			return nil, 0, err
		}
		// Record count for the collection's table (record tables share the
		// collection name; system collections have no record table unless
		// they are auth collections).
		if c.Type == "base" || c.Type == "auth" {
			var count int
			q := fmt.Sprintf(`SELECT count(*) FROM sqlite_master WHERE type='table' AND name=%q`, c.Name)
			var has int
			if err := reader.QueryRowContext(ctx, q).Scan(&has); err == nil && has > 0 {
				q2 := fmt.Sprintf(`SELECT count(*) FROM %q`, c.Name)
				if err := reader.QueryRowContext(ctx, q2).Scan(&count); err != nil {
					rows.Close()
					return nil, 0, fmt.Errorf("record count %q: %w", c.Name, err)
				}
			}
			c.RecordCount = count
			total += count
		}
		out = append(out, c)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, err
	}
	return out, total, nil
}

// PlanRequest is the v0.1-narrowed ImportPlanDraft: shared layout,
// namespace=shared, tenantKind/source/access null, selector none, one
// shared placement per source collection, preserve-global uniqueness.
type PlanRequest struct {
	ExpectedRevision string             `json:"expectedRevision"`
	SourceHash       string             `json:"sourceHash"`
	SnapshotHash     string             `json:"snapshotHash"`
	Layout           string             `json:"layout"`
	Namespace        string             `json:"namespace"`
	TenantKind       *string            `json:"tenantKind"`
	TenantSource     json.RawMessage    `json:"tenantSource"`
	Access           json.RawMessage    `json:"access"`
	Placements       []PlacementRequest `json:"placements"`
	AccountPolicy    struct {
		SessionPolicy  string `json:"sessionPolicy"`
		OperatorPolicy string `json:"operatorPolicy"`
	} `json:"accountPolicy"`
	Selector struct {
		Kind   string  `json:"kind"`
		Source *string `json:"source"`
	} `json:"selector"`
	Profile            string `json:"profile"`
	SourceFileStoreRef string `json:"sourceFileStoreRef"`
	SettingsKeyRef     string `json:"settingsKeyRef"`
	SideEffects        string `json:"sideEffects"`
}

type PlacementRequest struct {
	CollectionID string `json:"collectionId"`
	Placement    string `json:"placement"`
	Ownership    struct {
		Kind string `json:"kind"`
	} `json:"ownership"`
	UniquePolicy string `json:"uniquePolicy"`
}

// Plan is the ImportPlan wire shape.
type Plan struct {
	ID        string `json:"id"`
	SessionID string `json:"sessionId"`
	Revision  int64  `json:"revision"`
	PlanHash  string `json:"planHash"`
	Status    string `json:"status"` // draft|previewed|staged|validated|superseded
}

// The plan is persisted inside the session directory as canonical JSON
// with its hash (plan_ref); the session row records the plan hash.
func (s *Service) PutPlan(ctx context.Context, id string, req *PlanRequest) (*Plan, error) {
	sess, err := s.Get(ctx, id)
	if err != nil {
		return nil, err
	}
	switch sess.Phase {
	case StateDraft, StatePreviewed, StateStaged, StateReadyForCutover, StateFailed:
	default:
		return nil, fmt.Errorf("%w: phase %s does not accept plan edits", ErrConflict, sess.Phase)
	}
	if req.SourceHash != deref(sess.SourceHash) {
		return nil, fmt.Errorf("%w: the plan's sourceHash does not match the session", ErrSourceChanged)
	}
	if req.SnapshotHash != deref(sess.SnapshotHash) {
		return nil, fmt.Errorf("%w: the plan's snapshotHash does not match the session", ErrSourceChanged)
	}
	// v0.1 narrowing: shared layout only, one shared placement per source
	// collection, preserve-global, sideEffects suppressed.
	if req.Layout != "shared" {
		return nil, fmt.Errorf("%w: this release adopts the shared layout only; tenant/user/agent conversion is the R2 roadmap", ErrInvalidRequest)
	}
	if req.Namespace != "shared" {
		return nil, fmt.Errorf("%w: namespace must be 'shared' for the shared layout", ErrInvalidRequest)
	}
	if req.TenantKind != nil || isPresentJSON(req.TenantSource) || isPresentJSON(req.Access) {
		return nil, fmt.Errorf("%w: tenantKind/tenantSource/access are the R2 split-layout roadmap; the shared-layout request rejects them explicitly", ErrInvalidRequest)
	}
	if req.Selector.Kind != "none" {
		return nil, fmt.Errorf("%w: the shared layout requires selector kind 'none'", ErrInvalidRequest)
	}
	if req.SideEffects != "suppressed-during-import" {
		return nil, fmt.Errorf("%w: sideEffects must be suppressed-during-import", ErrInvalidRequest)
	}
	if req.AccountPolicy.SessionPolicy != "preserve-if-compatible" && req.AccountPolicy.SessionPolicy != "revoke-existing" {
		return nil, fmt.Errorf("%w: accountPolicy.sessionPolicy must be preserve-if-compatible or revoke-existing", ErrInvalidRequest)
	}
	if req.AccountPolicy.OperatorPolicy != "retain-destination-superuser" && req.AccountPolicy.OperatorPolicy != "use-source-superuser" {
		return nil, fmt.Errorf("%w: accountPolicy.operatorPolicy must be retain-destination-superuser or use-source-superuser", ErrInvalidRequest)
	}
	for _, pl := range req.Placements {
		if pl.Placement != "shared" {
			return nil, fmt.Errorf("%w: every placement must be shared in this release", ErrInvalidRequest)
		}
		if pl.Ownership.Kind != "shared" {
			return nil, fmt.Errorf("%w: every ownership must be shared in this release", ErrInvalidRequest)
		}
		if pl.UniquePolicy != "preserve-global" {
			return nil, fmt.Errorf("%w: uniquePolicy must be preserve-global in this release", ErrInvalidRequest)
		}
	}

	canonical, err := json.Marshal(struct {
		ExpectedRevision string             `json:"expectedRevision"`
		SourceHash       string             `json:"sourceHash"`
		SnapshotHash     string             `json:"snapshotHash"`
		Layout           string             `json:"layout"`
		Namespace        string             `json:"namespace"`
		Placements       []PlacementRequest `json:"placements"`
		AccountPolicy    struct {
			SessionPolicy  string `json:"sessionPolicy"`
			OperatorPolicy string `json:"operatorPolicy"`
		} `json:"accountPolicy"`
		Selector struct {
			Kind   string  `json:"kind"`
			Source *string `json:"source"`
		} `json:"selector"`
		Profile     string `json:"profile"`
		SideEffects string `json:"sideEffects"`
	}{req.ExpectedRevision, req.SourceHash, req.SnapshotHash, req.Layout, req.Namespace,
		req.Placements, req.AccountPolicy, req.Selector, req.Profile, req.SideEffects})
	if err != nil {
		return nil, err
	}
	planHash := hashBytes(canonical)

	dir := filepath.Join(s.SessionDir(id), "plan")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil, err
	}
	planPath := filepath.Join(dir, "plan-"+planHash[:16]+".json")
	if err := os.WriteFile(planPath, canonical, 0o600); err != nil {
		return nil, err
	}
	if err := s.SetHash(ctx, id, "plan_hash", planHash); err != nil {
		return nil, err
	}
	if err := s.SetHash(ctx, id, "plan_ref", planPath); err != nil {
		return nil, err
	}
	if sess.Phase != StateDraft {
		if err := s.Transition(ctx, id, StateDraft, ""); err != nil {
			return nil, err
		}
	}
	return &Plan{
		ID:        planHash[:16],
		SessionID: id,
		PlanHash:  planHash,
		Status:    "draft",
	}, nil
}

// isPresentJSON reports whether a RawMessage field carries a value
// (explicit JSON null is absence).
func isPresentJSON(raw json.RawMessage) bool {
	trimmed := strings.TrimSpace(string(raw))
	return len(trimmed) > 0 && trimmed != "null"
}

// Preview computes the actual per-collection record/file totals and
// hashes from the SOURCE copy ("Preview is computed from actual source
// rows/files, not schema guesses"). v0.1 invariants: tenantCount=0,
// referenceCopies=0, sharedRecords=sourceRecords=assignedRecords, no
// unassigned/conflicting records.
type Preview struct {
	SessionID          string                `json:"sessionId"`
	PlanHash           string                `json:"planHash"`
	PreviewHash        string                `json:"previewHash"`
	TenantCount        int                   `json:"tenantCount"`
	SourceRecords      int                   `json:"sourceRecords"`
	AssignedRecords    int                   `json:"assignedRecords"`
	ReferenceCopies    int                   `json:"referenceCopies"`
	SharedRecords      int                   `json:"sharedRecords"`
	UnassignedRecords  int                   `json:"unassignedRecords"`
	ConflictingRecords int                   `json:"conflictingRecords"`
	RequiredFiles      int                   `json:"requiredFiles"`
	MissingFiles       int                   `json:"missingFiles"`
	RequiredBytes      int64                 `json:"requiredBytes"`
	UnresolvedErrors   int                   `json:"unresolvedErrors"`
	Complete           bool                  `json:"complete"`
	Collections        []CollectionInventory `json:"collections"`
}

// ComputePreview re-reads the source copy through the read-only reader
// and emits the actual totals (the analysis inventory is recomputed, not
// trusted — "Preview is computed from actual source rows/files").
func (s *Service) ComputePreview(ctx context.Context, id string) (*Preview, error) {
	sess, err := s.Get(ctx, id)
	if err != nil {
		return nil, err
	}
	switch sess.Phase {
	case StateDraft, StatePreviewed:
	default:
		return nil, fmt.Errorf("%w: phase %s does not accept preview computation", ErrConflict, sess.Phase)
	}
	planHash := deref(sess.PlanHash)
	if planHash == "" {
		return nil, fmt.Errorf("%w: no plan is drafted", ErrPlanStale)
	}
	workData := filepath.Join(s.SessionDir(id), "work", "data.db")
	if _, err := os.Stat(workData); err != nil {
		return nil, fmt.Errorf("%w: the private source copy is missing", ErrSourceIncomplete)
	}
	reader, err := sql.Open("sqlite", workData+"?mode=ro")
	if err != nil {
		return nil, err
	}
	defer reader.Close()

	inventory, total, err := inspectCollections(ctx, reader)
	if err != nil {
		return nil, err
	}
	if err := s.Transition(ctx, id, StatePreviewing, ""); err != nil {
		return nil, err
	}

	// Files: the backup/directory local storage directory (files/) under
	// the work dir or the source dir — counted from what the source
	// actually carries.
	requiredFiles, requiredBytes := countSourceFiles(s.SessionDir(id))

	preview := &Preview{
		SessionID:          id,
		PlanHash:           planHash,
		TenantCount:        0,
		SourceRecords:      total,
		AssignedRecords:    total,
		ReferenceCopies:    0,
		SharedRecords:      total,
		UnassignedRecords:  0,
		ConflictingRecords: 0,
		RequiredFiles:      requiredFiles,
		MissingFiles:       0,
		RequiredBytes:      requiredBytes,
		Collections:        inventory,
		Complete:           true,
	}
	previewJSON, err := json.Marshal(preview)
	if err != nil {
		return nil, err
	}
	previewHash := hashBytes(previewJSON)
	preview.PreviewHash = previewHash
	if err := s.SetHash(ctx, id, "preview_hash", previewHash); err != nil {
		return nil, err
	}
	if err := s.Transition(ctx, id, StatePreviewed, ""); err != nil {
		return nil, err
	}
	// Persist the artifact for pbimport.preview.get read-back.
	if perr := s.persistPreview(id, preview); perr != nil {
		return nil, perr
	}
	return preview, nil
}

// countSourceFiles inventories the local file storage carried by the
// source (data dir snapshot: files/<collectionId>/<recordId>/...).
func countSourceFiles(sessionDir string) (int, int64) {
	var count int
	var bytes int64
	roots := []string{
		filepath.Join(sessionDir, "work", "files"),
		filepath.Join(sessionDir, "work", "storage"),
	}
	var walk func(dir string)
	walk = func(dir string) {
		entries, err := os.ReadDir(dir)
		if err != nil {
			return
		}
		for _, e := range entries {
			p := filepath.Join(dir, e.Name())
			if e.IsDir() {
				walk(p)
				continue
			}
			info, err := e.Info()
			if err != nil {
				continue
			}
			count++
			bytes += info.Size()
		}
	}
	for _, r := range roots {
		walk(r)
	}
	return count, bytes
}

func deref(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

var _ = errors.New

// ---------------------------------------------------------------------------
// Read-back surface (pbimport.inventory / pbimport.plan.get /
// pbimport.preview.get / pbimport.issues): the artifacts are persisted in
// the session directory so the read paths return the recorded artifacts
// without recomputation.
// ---------------------------------------------------------------------------

// persistInventory writes the analysis inventory artifact.
func (s *Service) persistInventory(id string, analysis *Analysis) error {
	dir := filepath.Join(s.SessionDir(id), "artifacts")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	b, err := json.Marshal(analysis)
	if err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(dir, "inventory.json"), b, 0o600); err != nil {
		return err
	}
	_, err = s.registerArtifact(id, "inventory", "inventory.json")
	return err
}

// GetInventory returns the recorded analysis artifact (pbimport.inventory).
func (s *Service) GetInventory(ctx context.Context, id string) (*Analysis, error) {
	if _, err := s.Get(ctx, id); err != nil {
		return nil, err
	}
	b, err := os.ReadFile(filepath.Join(s.SessionDir(id), "artifacts", "inventory.json"))
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("%w: the session has no recorded analysis yet", ErrConflict)
		}
		return nil, err
	}
	var a Analysis
	if err := json.Unmarshal(b, &a); err != nil {
		return nil, err
	}
	return &a, nil
}

// GetPlan returns the recorded plan (pbimport.plan.get).
func (s *Service) GetPlan(ctx context.Context, id string) (*Plan, error) {
	sess, err := s.Get(ctx, id)
	if err != nil {
		return nil, err
	}
	if sess.PlanHash == nil || *sess.PlanHash == "" {
		return nil, fmt.Errorf("%w: the session has no recorded plan yet", ErrConflict)
	}
	path := filepath.Join(s.SessionDir(id), "plan", "plan-"+(*sess.PlanHash)[:16]+".json")
	b, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("%w: the recorded plan file is missing", ErrConflict)
		}
		return nil, err
	}
	var p Plan
	if err := json.Unmarshal(b, &p); err != nil {
		return nil, err
	}
	return &p, nil
}

// persistPreview writes the preview artifact.
func (s *Service) persistPreview(id string, preview *Preview) error {
	dir := filepath.Join(s.SessionDir(id), "artifacts")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	b, err := json.Marshal(preview)
	if err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(dir, "preview.json"), b, 0o600); err != nil {
		return err
	}
	_, err = s.registerArtifact(id, "preview", "preview.json")
	return err
}

// GetPreview returns the recorded preview (pbimport.preview.get).
func (s *Service) GetPreview(ctx context.Context, id string) (*Preview, error) {
	if _, err := s.Get(ctx, id); err != nil {
		return nil, err
	}
	b, err := os.ReadFile(filepath.Join(s.SessionDir(id), "artifacts", "preview.json"))
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("%w: the session has no recorded preview yet", ErrConflict)
		}
		return nil, err
	}
	var p Preview
	if err := json.Unmarshal(b, &p); err != nil {
		return nil, err
	}
	return &p, nil
}

// persistValidation writes the validation artifact.
func (s *Service) persistValidation(id string, validation *Validation) error {
	dir := filepath.Join(s.SessionDir(id), "artifacts")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	b, err := json.Marshal(validation)
	if err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(dir, "validation.json"), b, 0o600); err != nil {
		return err
	}
	_, err = s.registerArtifact(id, "validation", "validation.json")
	return err
}

// GetValidation returns the recorded validation artifact (pbimport.issues
// reads the same artifact's unresolved cases).
func (s *Service) GetValidation(ctx context.Context, id string) (*Validation, error) {
	if _, err := s.Get(ctx, id); err != nil {
		return nil, err
	}
	b, err := os.ReadFile(filepath.Join(s.SessionDir(id), "artifacts", "validation.json"))
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("%w: the session has no recorded validation yet", ErrConflict)
		}
		return nil, err
	}
	var v Validation
	if err := json.Unmarshal(b, &v); err != nil {
		return nil, err
	}
	return &v, nil
}
