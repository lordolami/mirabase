// Package pbimport implements the shared-layout PocketBase adoption
// (pocketbase-tenant-import.md, narrowed by v0.1-build.md: shared layout,
// one default application, targetMode new-application|replace-application,
// placements all shared with preserve-global uniqueness).
//
// The pipeline: session → chunked hash-verified source upload → analyze
// (private copy recovery + read-only inspection) → plan → preview (actual
// counts/hashes) → stage (rows/files copied into an unpublished target
// with stable IDs) → validate (count/hash reconciliation cases) → guarded
// cutover (durable journal, maintenance/drain/rename/verify/active
// marker). Side effects are suppressed during import
// ("sideEffects":"suppressed-during-import"); the original source is
// retained; a rollback can never discard accepted post-cutover writes.
package pbimport

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// Session states (the 18-value ImportPhase; v0.1 reachable set noted).
const (
	StateAwaitingSource   = "awaiting-source"
	StateSourceReady      = "source-ready"
	StateAnalyzing        = "analyzing"
	StateDraft            = "draft"
	StatePreviewing       = "previewing"
	StatePreviewed        = "previewed"
	StateStaging          = "staging"
	StateStaged           = "staged"
	StateValidating       = "validating"
	StateReadyForCutover  = "ready-for-cutover"
	StateCuttingOver      = "cutting-over"
	StateActive           = "active"
	StateFailed           = "failed"
	StateCancelled        = "cancelled"
	StateRollingBack      = "rolling-back"
	StateRolledBack       = "rolled-back"
	StateRecoveryExported = "recovery-exported"
	StateRetired          = "retired"
)

// Defaults (pocketbase-tenant-import.md).
const (
	DefaultMaxSourceBytes = 20 << 30 // 20 GiB
	DefaultChunkSize      = 8 << 20  // 8 MiB upload chunks
	MaxActiveSessions     = 2
	CopyBatchRows         = 1000
	CopyBatchBytes        = 4 << 20
)

// Normative `_mb_import_sessions` DDL (storage-format-reference.md).
const sessionsDDL = `CREATE TABLE IF NOT EXISTS _mb_import_sessions (
 id TEXT PRIMARY KEY,
 actor_ref_json TEXT NOT NULL CHECK(json_valid(actor_ref_json)),
 state TEXT NOT NULL CHECK(state IN ('awaiting-source','source-ready','analyzing','draft','previewing','previewed','staging','staged','validating','ready-for-cutover','cutting-over','active','failed','cancelled','rolling-back','rolled-back','recovery-exported','retired')),
 revision INTEGER NOT NULL CHECK(revision > 0),
 source_kind TEXT NOT NULL CHECK(source_kind IN ('pocketbase-backup','pocketbase-directory')),
 target_mode TEXT NOT NULL CHECK(target_mode IN ('new-application','replace-application','append-application')),
 source_bytes INTEGER NOT NULL CHECK(source_bytes >= 0),
 source_hash TEXT,
 normalized_snapshot_hash TEXT,
 plan_hash TEXT,
 preview_hash TEXT,
 validation_hash TEXT,
 source_ref TEXT,
 plan_ref TEXT,
 staging_ref TEXT,
 recovery_ref TEXT,
 expected_primary_generation INTEGER NOT NULL CHECK(expected_primary_generation > 0),
 current_job_id TEXT,
 last_error_code TEXT,
 created TEXT NOT NULL,
 updated TEXT NOT NULL,
 expires_at TEXT
)`

const sessionsIndexDDL = `CREATE INDEX IF NOT EXISTS _mb_import_sessions_state
 ON _mb_import_sessions(state,updated)`

// Errors (the pbimport common error set).
var (
	ErrSourceDataRequired = errors.New("source_data_required")
	ErrSourceChanged      = errors.New("source_changed")
	ErrSourceIncomplete   = errors.New("source_incomplete")
	ErrPlanStale          = errors.New("plan_stale")
	ErrUnresolvedIssues   = errors.New("unresolved_import_issues")
	ErrReportStale        = errors.New("report_stale")
	ErrGenerationMismatch = errors.New("generation_mismatch")
	ErrNotFound           = errors.New("not_found")
	ErrInvalidRequest     = errors.New("invalid_request")
	ErrConflict           = errors.New("conflict")
)

// CreateRequest is the ImportCreate wire shape (v0.1: targetMode
// new-application|replace-application only; no batch/reservation fields).
type CreateRequest struct {
	SourceKind                string `json:"sourceKind"`
	SourceBytes               int64  `json:"sourceBytes"`
	SourceSha256              string `json:"sourceSha256"`
	TargetMode                string `json:"targetMode"`
	ExpectedPrimaryGeneration string `json:"expectedPrimaryGeneration"`
	SourceLabel               string `json:"sourceLabel"`
}

// Session is the ImportSession wire shape.
type Session struct {
	ID                        string  `json:"id"`
	Revision                  int64   `json:"revision"`
	Phase                     string  `json:"phase"`
	SourceKind                string  `json:"sourceKind"`
	SourceHash                *string `json:"sourceHash"`
	SnapshotHash              *string `json:"snapshotHash"`
	PlanHash                  *string `json:"planHash"`
	PreviewHash               *string `json:"previewHash"`
	ReportHash                *string `json:"reportHash"`
	TargetMode                string  `json:"targetMode"`
	ExpectedPrimaryGeneration string  `json:"expectedPrimaryGeneration"`
	CurrentJobID              *string `json:"currentJobId"`
	LastErrorCode             *string `json:"lastErrorCode"`
	Created                   string  `json:"created"`
	Updated                   string  `json:"updated"`
}

// Service owns the import sessions and the pipeline.
type Service struct {
	db       *sql.DB
	dataDir  string
	mu       sync.Mutex
	sessions map[string]*sessionRuntime
}

// sessionRuntime is the per-session staging state (disk-backed ledger;
// never an unbounded resident map).
type sessionRuntime struct {
	mu       sync.Mutex
	dir      string
	received int64
	chunks   map[int64]string // offset -> sha256 of stored chunk
}

// NewService installs the sessions table (at its first consumer) and
// returns the service rooted at the data directory.
func NewService(db *sql.DB, dataDir string) (*Service, error) {
	if _, err := db.Exec(sessionsDDL); err != nil {
		return nil, fmt.Errorf("failed to install import sessions: %w", err)
	}
	if _, err := db.Exec(sessionsIndexDDL); err != nil {
		return nil, err
	}
	return &Service{db: db, dataDir: dataDir, sessions: map[string]*sessionRuntime{}}, nil
}

func nowRFC() string { return time.Now().UTC().Format(time.RFC3339Nano) }

func newID(prefix string) string {
	b := make([]byte, 8)
	_, _ = rand.Read(b)
	return prefix + hex.EncodeToString(b)
}

// activeSessionCount enforces the two-active-sessions default.
func (s *Service) activeSessionCount(ctx context.Context) (int, error) {
	var n int
	err := s.db.QueryRowContext(ctx,
		`SELECT count(*) FROM _mb_import_sessions
		 WHERE state NOT IN ('cancelled','rolled-back','recovery-exported','retired','failed')`).Scan(&n)
	return n, err
}

// Create validates the ImportCreate and opens the session in
// awaiting-source ("no-session" → awaiting-source).
func (s *Service) Create(ctx context.Context, req *CreateRequest) (*Session, error) {
	switch req.SourceKind {
	case "pocketbase-backup", "pocketbase-directory":
	default:
		return nil, fmt.Errorf("%w: sourceKind must be pocketbase-backup or pocketbase-directory", ErrInvalidRequest)
	}
	switch req.TargetMode {
	case "new-application", "replace-application":
		// v0.1: append-application is rejected with roadmap direction.
	case "append-application":
		return nil, errors.New("append-application is not part of this release (independent application batches are the R3 consolidation roadmap)")
	default:
		return nil, fmt.Errorf("%w: unknown targetMode %q", ErrInvalidRequest, req.TargetMode)
	}
	if req.SourceBytes < 1 {
		return nil, fmt.Errorf("%w: sourceBytes must be >= 1", ErrInvalidRequest)
	}
	if req.SourceBytes > DefaultMaxSourceBytes {
		return nil, fmt.Errorf("%w: source exceeds the 20 GiB limit", ErrInvalidRequest)
	}
	if len(req.SourceSha256) != 64 || !isHex(req.SourceSha256) {
		return nil, fmt.Errorf("%w: sourceSha256 must be a 64-char hex hash", ErrInvalidRequest)
	}
	n, err := s.activeSessionCount(ctx)
	if err != nil {
		return nil, err
	}
	if n >= MaxActiveSessions {
		return nil, fmt.Errorf("%w: %d active sessions (default limit %d)", ErrConflict, n, MaxActiveSessions)
	}

	gen := int64(1)
	if _, err := fmt.Sscanf(req.ExpectedPrimaryGeneration, "%d", &gen); err != nil {
		return nil, fmt.Errorf("%w: expectedPrimaryGeneration must be a revision string", ErrInvalidRequest)
	}
	id := newID("imp-")
	now := nowRFC()
	actorRef, _ := json.Marshal(map[string]any{"applicationId": "default"})
	_, err = s.db.ExecContext(ctx,
		`INSERT INTO _mb_import_sessions
			(id, actor_ref_json, state, revision, source_kind, target_mode, source_bytes,
			 source_hash, expected_primary_generation, created, updated)
		 VALUES (?, ?, 'awaiting-source', 1, ?, ?, ?, ?, ?, ?, ?)`,
		id, string(actorRef), req.SourceKind, req.TargetMode, req.SourceBytes,
		req.SourceSha256, gen, now, now)
	if err != nil {
		return nil, err
	}
	// Owner-private session directory (uploads stage here).
	dir := filepath.Join(s.dataDir, "imports", id)
	if err := os.MkdirAll(filepath.Join(dir, "source"), 0o700); err != nil {
		return nil, err
	}
	s.mu.Lock()
	s.sessions[id] = &sessionRuntime{dir: dir, chunks: map[int64]string{}}
	s.mu.Unlock()
	return s.Get(ctx, id)
}

// Get loads one session.
func (s *Service) Get(ctx context.Context, id string) (*Session, error) {
	row := s.db.QueryRowContext(ctx,
		`SELECT id, revision, state, source_kind, source_hash, normalized_snapshot_hash,
		 plan_hash, preview_hash, validation_hash, target_mode,
		 expected_primary_generation, current_job_id, last_error_code, created, updated
		 FROM _mb_import_sessions WHERE id = ?`, id)
	var sess Session
	var sourceHash, snapHash, planHash, previewHash, validationHash, jobID, lastErr sql.NullString
	err := row.Scan(&sess.ID, &sess.Revision, &sess.Phase, &sess.SourceKind, &sourceHash,
		&snapHash, &planHash, &previewHash, &validationHash, &sess.TargetMode,
		&sess.ExpectedPrimaryGeneration, &jobID, &lastErr, &sess.Created, &sess.Updated)
	// expected_primary_generation is numeric; the wire wants a decimal string.
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	// Re-read the generation as text via a second scan shape.
	_ = validationHash
	copyNull(&sess.SourceHash, sourceHash)
	copyNull(&sess.SnapshotHash, snapHash)
	copyNull(&sess.PlanHash, planHash)
	copyNull(&sess.PreviewHash, previewHash)
	copyNull(&sess.ReportHash, validationHash)
	copyNull(&sess.CurrentJobID, jobID)
	copyNull(&sess.LastErrorCode, lastErr)
	// Patch ExpectedPrimaryGeneration via a direct query.
	var genText string
	if e := s.db.QueryRowContext(ctx,
		`SELECT CAST(expected_primary_generation AS TEXT) FROM _mb_import_sessions WHERE id = ?`,
		id).Scan(&genText); e == nil {
		sess.ExpectedPrimaryGeneration = genText
	}
	return &sess, nil
}

func copyNull(dst **string, src sql.NullString) {
	if src.Valid {
		v := src.String
		*dst = &v
	}
}

func isHex(s string) bool {
	_, err := hex.DecodeString(s)
	return err == nil
}

// Transition moves the session to a new state (failures preserve the
// failed phase's checkpoints — the state is set to failed with the error
// code recorded).
func (s *Service) Transition(ctx context.Context, id, state, errorCode string) error {
	if errorCode != "" {
		_, err := s.db.ExecContext(ctx,
			`UPDATE _mb_import_sessions SET state = ?, last_error_code = ?, revision = revision + 1, updated = ? WHERE id = ?`,
			state, errorCode, nowRFC(), id)
		return err
	}
	_, err := s.db.ExecContext(ctx,
		`UPDATE _mb_import_sessions SET state = ?, revision = revision + 1, updated = ? WHERE id = ?`,
		state, nowRFC(), id)
	return err
}

// SetHash records a pipeline hash column (source/normalized/plan/preview/
// validation per the session row).
func (s *Service) SetHash(ctx context.Context, id, column, value string) error {
	allowed := map[string]bool{
		"source_hash": true, "normalized_snapshot_hash": true, "plan_hash": true,
		"preview_hash": true, "validation_hash": true, "source_ref": true,
		"plan_ref": true, "staging_ref": true, "recovery_ref": true,
		"current_job_id": true,
	}
	if !allowed[column] {
		return fmt.Errorf("%w: unknown session column %q", ErrInvalidRequest, column)
	}
	_, err := s.db.ExecContext(ctx,
		fmt.Sprintf(`UPDATE _mb_import_sessions SET %s = ?, revision = revision + 1, updated = ? WHERE id = ?`, column),
		value, nowRFC(), id)
	return err
}

// SessionDir returns the owner-private session directory.
func (s *Service) SessionDir(id string) string {
	return filepath.Join(s.dataDir, "imports", id)
}

// SourceDir is where the uploaded/staged source bytes live.
func (s *Service) SourceDir(id string) string {
	return filepath.Join(s.SessionDir(id), "source")
}

// UploadChunk stores one resumable chunk: "Stream resumable uploads in
// chunks of at most 8 MiB into an owner-private session directory.
// Validate Content-Range, chunk boundaries and SHA-256. Repeated
// identical chunks are safe; a different chunk at the same offset is a
// conflict."
func (s *Service) UploadChunk(ctx context.Context, id string, offset int64, data []byte, chunkSha string) (received int64, complete bool, err error) {
	if len(data) > DefaultChunkSize {
		return 0, false, fmt.Errorf("%w: chunk exceeds the 8 MiB boundary", ErrInvalidRequest)
	}
	sess, err := s.Get(ctx, id)
	if err != nil {
		return 0, false, err
	}
	if sess.Phase != StateAwaitingSource && sess.Phase != StateSourceReady {
		return 0, false, fmt.Errorf("%w: phase %s does not accept source uploads", ErrConflict, sess.Phase)
	}
	sum := sha256.Sum256(data)
	got := hex.EncodeToString(sum[:])
	if got != chunkSha {
		return 0, false, fmt.Errorf("%w: chunk hash mismatch at offset %d", ErrSourceChanged, offset)
	}
	s.mu.Lock()
	rt := s.sessions[id]
	if rt == nil {
		rt = &sessionRuntime{dir: s.SessionDir(id), chunks: map[int64]string{}}
		s.sessions[id] = rt
	}
	s.mu.Unlock()

	rt.mu.Lock()
	defer rt.mu.Unlock()
	if prev, ok := rt.chunks[offset]; ok {
		if prev != got {
			return 0, false, fmt.Errorf("%w: a different chunk was already stored at offset %d", ErrConflict, offset)
		}
		// Repeated identical chunk: safe.
	} else {
		path := filepath.Join(s.SourceDir(id), fmt.Sprintf("chunk-%016d", offset))
		if err := os.WriteFile(path, data, 0o600); err != nil {
			return 0, false, err
		}
		rt.chunks[offset] = got
		rt.received += int64(len(data))
	}

	sess2, sessErr := s.Get(ctx, id)
	if sessErr != nil {
		return 0, false, sessErr
	}
	complete = rt.received >= sess2ExpectedBytes(sess2)
	if complete {
		// Only an exact length/hash match finalizes the source.
		totalHash, err2 := s.finalizeSource(ctx, id)
		if err2 != nil {
			return rt.received, false, err2
		}
		if err2 := s.SetHash(ctx, id, "source_hash", totalHash); err2 != nil {
			return rt.received, false, err2
		}
		if sess2.Phase == StateAwaitingSource {
			if err2 := s.Transition(ctx, id, StateSourceReady, ""); err2 != nil {
				return rt.received, false, err2
			}
		}
	}
	return rt.received, complete, nil
}

func sess2ExpectedBytes(sess *Session) int64 {
	var n int64
	_, _ = fmt.Sscanf(sess.ExpectedPrimaryGeneration, "%d", &n) // unused; keep shape
	_ = n
	return sessSourceBytes(sess)
}

// sessSourceBytes reads the declared source size from the session row via
// the wire session (SourceBytes is not on the wire Session; the declared
// total lives in the create request — tracked on the runtime).
func sessSourceBytes(sess *Session) int64 {
	// The session runtime tracks the declared total.
	if v, ok := declaredSources.Load(sess.ID); ok {
		return v.(int64)
	}
	return 0
}

var declaredSources sync.Map // session id -> declared source bytes

// DeclareSource records the declared total for the finalize check.
func (s *Service) DeclareSource(id string, sourceBytes int64) {
	declaredSources.Store(id, sourceBytes)
}

// finalizeSource concatenates the ordered chunks, verifies the total
// length and hash against the declared values, and writes the normalized
// source file.
func (s *Service) finalizeSource(ctx context.Context, id string) (string, error) {
	declared, ok := declaredSources.Load(id)
	if !ok {
		return "", fmt.Errorf("%w: no declared source size", ErrSourceIncomplete)
	}
	total := declared.(int64)
	entries, err := os.ReadDir(s.SourceDir(id))
	if err != nil {
		return "", err
	}
	var names []string
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), "chunk-") {
			names = append(names, e.Name())
		}
	}
	sortStrings(names)
	var out []byte
	h := sha256.New()
	for _, name := range names {
		b, err := os.ReadFile(filepath.Join(s.SourceDir(id), name))
		if err != nil {
			return "", err
		}
		out = append(out, b...)
		h.Write(b)
	}
	if int64(len(out)) != total {
		return "", fmt.Errorf("%w: received %d bytes, declared %d", ErrSourceIncomplete, len(out), total)
	}
	digest := hex.EncodeToString(h.Sum(nil))
	declaredHash, err := s.sessionSourceHash(ctx, id)
	if err != nil {
		return "", err
	}
	if digest != declaredHash {
		return "", fmt.Errorf("%w: source hash mismatch: expected %s got %s", ErrSourceChanged, declaredHash, digest)
	}
	final := filepath.Join(s.SourceDir(id), "source.bin")
	if err := os.WriteFile(final, out, 0o600); err != nil {
		return "", err
	}
	// Remove the chunks (the normalized source file is authoritative).
	for _, name := range names {
		_ = os.Remove(filepath.Join(s.SourceDir(id), name))
	}
	return digest, nil
}

func (s *Service) sessionSourceHash(ctx context.Context, id string) (string, error) {
	var h sql.NullString
	err := s.db.QueryRowContext(ctx, `SELECT source_hash FROM _mb_import_sessions WHERE id = ?`, id).Scan(&h)
	if err != nil {
		return "", err
	}
	if !h.Valid {
		return "", fmt.Errorf("%w: no declared source hash", ErrSourceIncomplete)
	}
	return h.String, nil
}

func sortStrings(s []string) {
	for i := 1; i < len(s); i++ {
		for j := i; j > 0 && s[j] < s[j-1]; j-- {
			s[j], s[j-1] = s[j-1], s[j]
		}
	}
}
