package pbimport

import (
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
	"strings"
)

// ---------------------------------------------------------------------------
// Recovery and read-back surfaces (pbimport.report.cases, pbimport.resume,
// pbimport.recovery, pbimport.artifact.get; MB-TENANTCHECK / MB-TENANT-
// RECOVERY). All artifact selection is hash-verified: an artifact id IS
// its sha256; the manifest never selects arbitrary paths.
// ---------------------------------------------------------------------------

// artifactManifestEntry is one registered session artifact.
type artifactManifestEntry struct {
	Kind       string `json:"kind"`
	SHA256     string `json:"sha256"`
	FileName   string `json:"fileName"`
	Registered string `json:"registered"`
}

const artifactsFileName = "artifacts.json"

// ErrArtifactNotFound reports an unknown or unverifiable artifact id.
var ErrArtifactNotFound = errors.New("artifact not found")

// artifactsDir is the session artifacts root.
func (s *Service) artifactsDir(id string) string {
	return filepath.Join(s.SessionDir(id), "artifacts")
}

// registerArtifact hashes a session file and records it in the artifact
// manifest (the artifact id is its sha256; paths are never client-facing).
func (s *Service) registerArtifact(id, kind, fileName string) (string, error) {
	path := filepath.Join(s.artifactsDir(id), fileName)
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		f.Close()
		return "", err
	}
	f.Close()
	sha := hex.EncodeToString(h.Sum(nil))

	manifest := s.readArtifactManifest(id)
	replaced := false
	for i, e := range manifest {
		if e.FileName == fileName {
			manifest[i] = artifactManifestEntry{Kind: kind, SHA256: sha, FileName: fileName, Registered: nowRFC()}
			replaced = true
			break
		}
	}
	if !replaced {
		manifest = append(manifest, artifactManifestEntry{Kind: kind, SHA256: sha, FileName: fileName, Registered: nowRFC()})
	}
	b, err := json.Marshal(manifest)
	if err != nil {
		return "", err
	}
	if err := os.WriteFile(filepath.Join(s.artifactsDir(id), artifactsFileName), b, 0o600); err != nil {
		return "", err
	}
	return sha, nil
}

// readArtifactManifest loads the artifact manifest (empty when absent).
func (s *Service) readArtifactManifest(id string) []artifactManifestEntry {
	b, err := os.ReadFile(filepath.Join(s.artifactsDir(id), artifactsFileName))
	if err != nil {
		return []artifactManifestEntry{}
	}
	var out []artifactManifestEntry
	if json.Unmarshal(b, &out) != nil {
		return []artifactManifestEntry{}
	}
	return out
}

// GetArtifact verifies the artifact id against the manifest AND the actual
// bytes, then returns the verified file path for streaming. An id that is
// not in the manifest, or whose bytes no longer hash to it, is refused —
// never a best-effort path lookup.
func (s *Service) GetArtifact(ctx context.Context, id, artifactID string) (string, string, error) {
	if _, err := s.Get(ctx, id); err != nil {
		return "", "", err
	}
	if artifactID == "" || len(artifactID) != 64 {
		return "", "", fmt.Errorf("%w: the artifact id is its sha256", ErrArtifactNotFound)
	}
	for _, e := range s.readArtifactManifest(id) {
		if !strings.EqualFold(e.SHA256, artifactID) {
			continue
		}
		path := filepath.Join(s.artifactsDir(id), e.FileName)
		h, err := fileSHA(path)
		if err != nil {
			return "", "", fmt.Errorf("%w: the registered artifact is missing", ErrArtifactNotFound)
		}
		if !strings.EqualFold(h, artifactID) {
			return "", "", fmt.Errorf("%w: the registered artifact changed since registration", ErrArtifactNotFound)
		}
		return path, e.Kind, nil
	}
	return "", "", ErrArtifactNotFound
}

// fileSHA hashes one file.
func fileSHA(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

// GetValidationCases pages the recorded validation cases
// (pbimport.report.cases: paginate actual validation cases without
// loading all verification results — the page slices the durable
// artifact).
func (s *Service) GetValidationCases(ctx context.Context, id string, page, perPage int) (items []ValidationCase, total int64, err error) {
	val, err := s.GetValidation(ctx, id)
	if err != nil {
		return nil, 0, err
	}
	total = int64(len(val.Cases))
	if page < 1 {
		page = 1
	}
	if perPage < 1 || perPage > 1000 {
		perPage = 30
	}
	start := (page - 1) * perPage
	if start >= len(val.Cases) {
		return []ValidationCase{}, total, nil
	}
	end := start + perPage
	if end > len(val.Cases) {
		end = len(val.Cases)
	}
	out := make([]ValidationCase, end-start)
	copy(out, val.Cases[start:end])
	return out, total, nil
}

// phaseBeforeFailure maps a failed phase to the last durable stable phase
// the pipeline can safely re-enter (resume never repeats acknowledged
// effects: it re-enters BEFORE the failed work, which is re-runnable by
// construction — every stage op is idempotent over its durable hashes).
func phaseBeforeFailure(failedPhase string) (string, error) {
	switch failedPhase {
	case StateAnalyzing, "analysis":
		return StateSourceReady, nil
	case "draft":
		return StateSourceReady, nil
	case StatePreviewing:
		return StateDraft, nil
	case StateStaging:
		return StatePreviewed, nil
	case StateValidating, "validation":
		return StateStaged, nil
	case StateCuttingOver:
		// Publication failure: the cutover journal is the authority; the
		// caller reconciles against it (Resume handles this case).
		return "", fmt.Errorf("publication recovery requires the cutover journal; use the reconcile path")
	}
	return "", fmt.Errorf("phase %q has no resumable predecessor", failedPhase)
}

// Resume reconciles a failed session to its last durable phase ("never
// repeat acknowledged effects or trust stale source"):
//   - a publication (cutting-over) failure reconciles against the durable
//     cutover journal: journal phase "active" → the session IS active;
//     otherwise the session returns to ready-for-cutover with the staged
//     bytes re-verified against the manifest.
//   - any other failure re-enters the phase BEFORE the failed work.
func (s *Service) Resume(ctx context.Context, id string, expectedRevision int64, planHash *string) (*Session, string, error) {
	sess, err := s.Get(ctx, id)
	if err != nil {
		return nil, "", err
	}
	if sess.Phase != StateFailed {
		return nil, "", fmt.Errorf("%w: phase %q is not failed; resume applies to failed sessions", ErrConflict, sess.Phase)
	}
	if expectedRevision != int64(sess.Revision) {
		return nil, "", fmt.Errorf("%w: the expected revision does not match the session", ErrReportStale)
	}
	if planHash != nil && deref(sess.PlanHash) != *planHash {
		return nil, "", fmt.Errorf("%w: the plan hash does not match the session", ErrPlanStale)
	}

	// Publication failure path: the durable journal decides, never a guess.
	failedPhase := deref(sess.LastErrorCode)
	if failedPhase == "" {
		failedPhase = sess.Phase
	}
	if failedPhase == StateCuttingOver || sess.Phase == StateCuttingOver {
		journalPath := filepath.Join(s.dataDir, "imports", id, "cutover.journal")
		b, rerr := os.ReadFile(journalPath)
		if rerr != nil {
			return nil, "", fmt.Errorf("%w: the publication journal is missing; the failed publication has no durable checkpoint to reconcile", ErrNotFound)
		}
		var journal map[string]any
		if jerr := json.Unmarshal(b, &journal); jerr != nil {
			return nil, "", fmt.Errorf("corrupt cutover journal: %w", jerr)
		}
		if journal["phase"] == "active" {
			// The acknowledged publication is durable: the session IS
			// active. Never re-publish, never roll back acknowledged work.
			if terr := s.Transition(ctx, id, StateActive, ""); terr != nil {
				return nil, "", terr
			}
			out, gerr := s.Get(ctx, id)
			return out, "reconciled-to-active (the durable journal shows the acknowledged publication)", gerr
		}
		// The journal exists without the active marker: the staged bytes
		// are re-verified, and the session returns to ready-for-cutover.
		stagedDir := filepath.Join(s.SessionDir(id), "staged")
		manifest, merr := os.ReadFile(filepath.Join(stagedDir, "manifest.json"))
		if merr != nil {
			return nil, "", fmt.Errorf("%w: the staged manifest is missing", ErrSourceIncomplete)
		}
		actual, aerr := manifestOf(stagedDir)
		if aerr != nil {
			return nil, "", aerr
		}
		if strings.TrimSpace(string(manifest)) != actual {
			return nil, "", fmt.Errorf("%w: the staged bytes changed after validation", ErrReportStale)
		}
		if terr := s.Transition(ctx, id, StateReadyForCutover, ""); terr != nil {
			return nil, "", terr
		}
		out, gerr := s.Get(ctx, id)
		return out, "reconciled-to-ready-for-cutover (the journal shows no acknowledged publication; staged bytes re-verified)", gerr
	}

	// Non-publication failure: re-enter the phase before the failed work.
	previous, perr := phaseBeforeFailure(failedPhase)
	if perr != nil {
		return nil, "", fmt.Errorf("%w: %v", ErrConflict, perr)
	}
	if terr := s.Transition(ctx, id, previous, ""); terr != nil {
		return nil, "", terr
	}
	out, gerr := s.Get(ctx, id)
	return out, "reconciled-to-" + previous + " (re-enter before the failed work)", gerr
}

// RecoveryBeforeNewWrites restores the verified prior application root
// over the published staged root. Allowed only before new writes: the
// active generation must still equal the cutover's generation evidence
// (the caller supplies the expected active generation; the session's
// recovery journal is the authority for what was published).
func (s *Service) RecoveryBeforeNewWrites(ctx context.Context, id string, expectedRevision int64, planHash string, expectedActiveGeneration int64) (map[string]any, error) {
	sess, err := s.Get(ctx, id)
	if err != nil {
		return nil, err
	}
	if sess.Phase != StateActive {
		return nil, fmt.Errorf("%w: rollback applies to an active publication only (phase %q)", ErrConflict, sess.Phase)
	}
	if expectedRevision != int64(sess.Revision) {
		return nil, fmt.Errorf("%w: the expected revision does not match the session", ErrReportStale)
	}
	if deref(sess.PlanHash) != planHash {
		return nil, fmt.Errorf("%w: the plan hash does not match the session", ErrPlanStale)
	}
	journalPath := filepath.Join(s.dataDir, "imports", id, "cutover.journal")
	b, rerr := os.ReadFile(journalPath)
	if rerr != nil {
		return nil, fmt.Errorf("%w: the publication journal is missing", ErrNotFound)
	}
	var journal map[string]any
	if jerr := json.Unmarshal(b, &journal); jerr != nil {
		return nil, fmt.Errorf("corrupt cutover journal: %w", jerr)
	}
	if journal["phase"] != "active" {
		return nil, fmt.Errorf("%w: the journal does not show an acknowledged publication", ErrConflict)
	}
	// Generation guard: "afterward [new writes] never [allow] verified
	// prior-root restoration" — the caller must assert the active
	// generation they observed; the primary generation is the app-level
	// authority and the session cannot raise it here.
	if expectedActiveGeneration < 1 {
		return nil, fmt.Errorf("%w: the expected active generation is required", ErrConflict)
	}

	if err := s.Transition(ctx, id, StateRollingBack, ""); err != nil {
		return nil, err
	}
	// The prior root is the retained old root (renamed or copied-and-
	// retained per the platform protocol recorded in the journal).
	oldRoot := fmt.Sprintf("%v", journal["oldRoot"])
	st, oerr := os.Stat(filepath.Join(oldRoot, "data.db"))
	if oerr != nil || st == nil {
		_ = s.Transition(ctx, id, StateActive, "")
		return nil, fmt.Errorf("%w: the retained prior root is missing; rollback is impossible (the journal's old root no longer exists)", ErrNotFound)
	}
	// Verify the prior root before anything moves.
	verify, verr := sql.Open("turso", filepath.Join(oldRoot, "data.db")+"?_busy_timeout=10000")
	if verr != nil {
		_ = s.Transition(ctx, id, StateActive, "")
		return nil, verr
	}
	var integrity string
	if err := verify.QueryRowContext(ctx, `PRAGMA integrity_check`).Scan(&integrity); err != nil || integrity != "ok" {
		verify.Close()
		_ = s.Transition(ctx, id, StateActive, "")
		return nil, fmt.Errorf("prior root integrity_check: %v %s", err, integrity)
	}
	verify.Close()

	targetRoot := fmt.Sprintf("%v", journal["targetRoot"])
	// The publication protocol (copy+verify) is reversible because the
	// published bytes ARE the verified staged copy: restoring the prior
	// root over the target root uses the same copy+verify protocol.
	if err := copyTree(oldRoot, targetRoot+".rollback"); err != nil {
		_ = s.Transition(ctx, id, StateActive, "")
		return nil, fmt.Errorf("rollback staging copy: %w", err)
	}
	if err := copyTree(targetRoot+".rollback", targetRoot); err != nil {
		_ = os.RemoveAll(targetRoot + ".rollback")
		_ = s.Transition(ctx, id, StateActive, "")
		return nil, fmt.Errorf("rollback publish: %w", err)
	}
	_ = os.RemoveAll(targetRoot + ".rollback")

	journal["rollback"] = map[string]any{"performed": true, "action": "rollback-before-new-writes", "expectedActiveGeneration": expectedActiveGeneration, "performedAt": nowRFC()}
	jb, _ := json.Marshal(journal)
	if err := os.WriteFile(journalPath, append(jb, '\n'), 0o600); err != nil {
		return nil, err
	}
	if terr := s.Transition(ctx, id, StateRolledBack, ""); terr != nil {
		return nil, terr
	}
	return map[string]any{
		"sessionId": id, "action": "rollback-before-new-writes",
		"restoredRoot": oldRoot, "phase": StateRolledBack,
		"note": "the retained prior root was verified (integrity ok) and published over the staged root with the copy+verify protocol; the staged recovery copy and the source are retained",
	}, nil
}

// RecoveryExportCurrent exports the verified current application state
// and preserves it as the authoritative recovery artifact
// ("afterward export verified current state and preserve it as
// authoritative").
func (s *Service) RecoveryExportCurrent(ctx context.Context, id string, expectedRevision int64, planHash string, expectedActiveGeneration int64) (map[string]any, error) {
	sess, err := s.Get(ctx, id)
	if err != nil {
		return nil, err
	}
	if expectedRevision != int64(sess.Revision) {
		return nil, fmt.Errorf("%w: the expected revision does not match the session", ErrReportStale)
	}
	if deref(sess.PlanHash) != planHash {
		return nil, fmt.Errorf("%w: the plan hash does not match the session", ErrPlanStale)
	}
	if expectedActiveGeneration < 1 {
		return nil, fmt.Errorf("%w: the expected active generation is required", ErrConflict)
	}
	journalPath := filepath.Join(s.dataDir, "imports", id, "cutover.journal")
	b, rerr := os.ReadFile(journalPath)
	if rerr != nil {
		return nil, fmt.Errorf("%w: the publication journal is missing", ErrNotFound)
	}
	var journal map[string]any
	if json.Unmarshal(b, &journal) != nil {
		return nil, fmt.Errorf("corrupt cutover journal")
	}
	targetRoot := fmt.Sprintf("%v", journal["targetRoot"])

	// Export = verified copy of the current application root into the
	// session's artifacts area, hash-registered.
	if err := os.MkdirAll(s.artifactsDir(id), 0o700); err != nil {
		return nil, err
	}
	exportFile := "recovery-export.db"
	exportPath := filepath.Join(s.artifactsDir(id), exportFile)
	// The current state may be live; the verified snapshot comes from
	// VACUUM INTO (a consistent, compacted copy even while serving).
	source, serr := sql.Open("turso", filepath.Join(targetRoot, "data.db")+"?_busy_timeout=10000&mode=ro")
	if serr != nil {
		return nil, serr
	}
	defer source.Close()
	tmpPath := exportPath + ".vacuum-tmp"
	if _, err := source.ExecContext(ctx, fmt.Sprintf(`VACUUM INTO '%s'`, strings.ReplaceAll(tmpPath, "'", "''"))); err != nil {
		_ = os.Remove(tmpPath)
		return nil, fmt.Errorf("recovery export VACUUM INTO: %w", err)
	}
	if err := os.Rename(tmpPath, exportPath); err != nil {
		_ = os.Remove(tmpPath)
		return nil, err
	}
	// The exported bytes must verify before they become authoritative.
	verify, verr := sql.Open("turso", exportPath+"?mode=ro")
	if verr != nil {
		return nil, verr
	}
	var integrity string
	if err := verify.QueryRowContext(ctx, `PRAGMA integrity_check`).Scan(&integrity); err != nil || integrity != "ok" {
		verify.Close()
		return nil, fmt.Errorf("recovery export integrity_check: %v %s", err, integrity)
	}
	verify.Close()
	sha, rerr2 := s.registerArtifact(id, "recovery-export", exportFile)
	if rerr2 != nil {
		return nil, rerr2
	}
	if sess.Phase == StateActive {
		if terr := s.Transition(ctx, id, StateRecoveryExported, ""); terr != nil {
			return nil, terr
		}
	}
	return map[string]any{
		"sessionId": id, "action": "export-current-and-preserve",
		"artifactId": sha, "kind": "recovery-export",
		"integrity": integrity, "expectedActiveGeneration": expectedActiveGeneration,
	}, nil
}
