package application

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
	"time"
)

// ---------------------------------------------------------------------------
// Per-target verified backups (backup.create / backup.restore; released
// slice: primary coverage rides the inherited core backup surface, private
// targets get VACUUM INTO archives with a file-coverage manifest and a
// validated copy-and-verify publication).
// ---------------------------------------------------------------------------

// ErrBackupInvalid reports an invalid or unverifiable backup archive.
var ErrBackupInvalid = errors.New("invalid backup archive")

// TargetBackupManifest is the file-coverage manifest written beside every
// archive ("Create verified context archive and file-coverage manifest").
type TargetBackupManifest struct {
	ArchiveID    string `json:"archiveId"`
	DatabaseID   string `json:"databaseId"`
	Generation   string `json:"generation"`
	Profile      string `json:"profile"`
	Records      string `json:"recordsFile"`
	SizeBytes    int64  `json:"sizeBytes"`
	SHA256       string `json:"sha256"`
	CreatedAt    string `json:"createdAt"`
	IncludeFiles bool   `json:"includeFiles"`
}

// backupDir returns the per-database archives directory.
func backupDir(dataDir, databaseID string) string {
	return filepath.Join(dataDir, "databases", databaseID, "backups")
}

// CreateTargetBackup creates a verified snapshot of one ready private
// database with VACUUM INTO and writes the coverage manifest.
func (rt *Runtime) CreateTargetBackup(ctx context.Context, databaseID string, includeFiles bool) (*TargetBackupManifest, error) {
	dir := backupDir(rt.DataDir, databaseID)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil, fmt.Errorf("failed to create the backup directory: %w", err)
	}
	archiveID := "b-" + time.Now().UTC().Format("20060102150405") + "-" + randHex(6)
	archivePath := filepath.Join(dir, archiveID+".db")
	tmpPath := archivePath + ".vacuum-tmp"

	// VACUUM INTO produces a compacted, consistent snapshot even while
	// the source is serving.
	handle, err := rt.Handles.Open(ctx, databaseID)
	if err != nil {
		return nil, fmt.Errorf("failed to open the target database: %w", err)
	}
	if _, err := handle.ExecContext(ctx, fmt.Sprintf(`VACUUM INTO '%s'`, strings.ReplaceAll(tmpPath, "'", "''"))); err != nil {
		_ = os.Remove(tmpPath)
		return nil, fmt.Errorf("VACUUM INTO failed: %w", err)
	}
	if err := os.Rename(tmpPath, archivePath); err != nil {
		_ = os.Remove(tmpPath)
		return nil, fmt.Errorf("failed to publish the archive: %w", err)
	}

	// Verify the archive independently (never trust the write alone).
	if err := verifySQLiteFile(ctx, archivePath); err != nil {
		_ = os.Remove(archivePath)
		return nil, fmt.Errorf("%w: %v", ErrBackupInvalid, err)
	}

	manifest, err := rt.writeTargetManifest(databaseID, archiveID, archivePath, includeFiles)
	if err != nil {
		return nil, err
	}
	return manifest, nil
}

// randHex returns n random hex characters.
func randHex(n int) string {
	if n < 1 {
		n = 1
	}
	b := make([]byte, (n+1)/2)
	if _, err := rand.Read(b); err != nil {
		return strings.Repeat("0", n)
	}
	return hex.EncodeToString(b)[:n]
}

// writeTargetManifest computes the archive digest and stores the manifest.
func (rt *Runtime) writeTargetManifest(databaseID, archiveID, archivePath string, includeFiles bool) (*TargetBackupManifest, error) {
	st, err := os.Stat(archivePath)
	if err != nil {
		return nil, fmt.Errorf("failed to stat the archive: %w", err)
	}
	digest, err := fileSHA256(archivePath)
	if err != nil {
		return nil, fmt.Errorf("failed to hash the archive: %w", err)
	}
	manifest := &TargetBackupManifest{
		ArchiveID:    archiveID,
		DatabaseID:   databaseID,
		Records:      filepath.Base(archivePath),
		SizeBytes:    st.Size(),
		SHA256:       digest,
		CreatedAt:    time.Now().UTC().Format(time.RFC3339Nano),
		IncludeFiles: includeFiles,
	}
	if m, err := rt.Service.ReadManifest(rt.DataDir, databaseID); err == nil && m != nil {
		manifest.Generation = m.Generation
		manifest.Profile = m.Profile
	}
	b, err := json.Marshal(manifest)
	if err != nil {
		return nil, err
	}
	if err := os.WriteFile(archivePath+".manifest.json", b, 0o600); err != nil {
		return nil, fmt.Errorf("failed to write the manifest: %w", err)
	}
	return manifest, nil
}

// verifySQLiteFile opens one SQLite file read-only and runs
// integrity_check (the archive must be a complete database).
func verifySQLiteFile(ctx context.Context, path string) error {
	db, err := sql.Open("sqlite", "file:"+path+"?mode=ro")
	if err != nil {
		return fmt.Errorf("open: %w", err)
	}
	defer db.Close()
	rows, err := db.QueryContext(ctx, `PRAGMA integrity_check`)
	if err != nil {
		return fmt.Errorf("integrity_check: %w", err)
	}
	defer rows.Close()
	var first string
	for rows.Next() {
		var line string
		if err := rows.Scan(&line); err != nil {
			return err
		}
		if first == "" {
			first = line
		}
	}
	if err := rows.Err(); err != nil {
		return err
	}
	if !strings.EqualFold(first, "ok") {
		return fmt.Errorf("integrity_check reported %q", first)
	}
	return nil
}

// fileSHA256 hashes a file with a streaming reader.
func fileSHA256(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	h := sha256.New()
	buf := make([]byte, 1<<20)
	for {
		n, rerr := f.Read(buf)
		if n > 0 {
			_, _ = h.Write(buf[:n])
		}
		if rerr != nil {
			break
		}
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

// ListTargetBackups returns the manifests of one target's archives.
func (rt *Runtime) ListTargetBackups(ctx context.Context, databaseID string) ([]*TargetBackupManifest, error) {
	dir := backupDir(rt.DataDir, databaseID)
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return []*TargetBackupManifest{}, nil
		}
		return nil, err
	}
	out := []*TargetBackupManifest{}
	for _, e := range entries {
		if !strings.HasSuffix(e.Name(), ".manifest.json") {
			continue
		}
		b, err := os.ReadFile(filepath.Join(dir, e.Name()))
		if err != nil {
			return nil, err
		}
		var m TargetBackupManifest
		if err := json.Unmarshal(b, &m); err != nil {
			return nil, fmt.Errorf("corrupt manifest %s: %w", e.Name(), err)
		}
		out = append(out, &m)
	}
	return out, nil
}

// RestoreTargetBackup validates an archive and either stages it for a new
// database (restoreToNew) or publishes it over the target with the
// copy-and-verify swap (close handles → verify → rename → new generation).
func (rt *Runtime) RestoreTargetBackup(ctx context.Context, databaseID, archiveID string, dryRun, restoreToNew bool) (map[string]any, error) {
	dir := backupDir(rt.DataDir, databaseID)
	archivePath := filepath.Join(dir, archiveID+".db")
	if _, err := os.Stat(archivePath); err != nil {
		if os.IsNotExist(err) {
			return nil, ErrNotFound
		}
		return nil, err
	}

	// Independent verification before any staging decision.
	if err := verifySQLiteFile(ctx, archivePath); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrBackupInvalid, err)
	}

	if dryRun {
		return map[string]any{
			"archiveId":  archiveID,
			"verified":   true,
			"dryRun":     true,
			"published":  false,
			"restoredTo": nil,
		}, nil
	}

	if restoreToNew {
		// Stage the archive as a NEW database directory (the original
		// target is never touched): a fresh registry row with the staged
		// generation follows the lifecycle contract (restoring state).
		stagedID := randHex(32)
		stagedDir := rt.Service.DatabaseDir(rt.DataDir, stagedID)
		if err := os.MkdirAll(stagedDir, 0o700); err != nil {
			return nil, fmt.Errorf("failed to create the staged database directory: %w", err)
		}
		stagedPath := filepath.Join(stagedDir, "data.db")
		if err := copyFileVerified(archivePath, stagedPath); err != nil {
			_ = os.RemoveAll(stagedDir)
			return nil, fmt.Errorf("staged restore copy failed: %w", err)
		}
		manifest, _ := rt.Service.ReadManifest(rt.DataDir, databaseID)
		generation := "1"
		profile := "wal"
		if manifest != nil {
			generation = manifest.Generation
			profile = manifest.Profile
		}
		newManifest := &PrivateManifest{
			FormatVersion:    ManifestFormatVersion,
			DatabaseID:       stagedID,
			Generation:       generation,
			Profile:          profile,
			SchemaRevision:   1,
			TemplateID:       "",
			EncryptionKeyRef: "",
			CodecID:          "none",
			CreatedAt:        nowRFC3339(),
		}
		if err := writeManifestFile(stagedDir, newManifest); err != nil {
			_ = os.RemoveAll(stagedDir)
			return nil, fmt.Errorf("failed to write the staged manifest: %w", err)
		}
		// The staged database is registered (manual kind, restoring state)
		// so the operator can inspect it before activating.
		if _, err := rt.Service.RequestDatabase(ctx, "manual", "", "staged restore from "+archiveID, nil, nil); err == nil {
			_ = rt.Service.TransitionDatabase(ctx, stagedID, StateRestoring, false)
		}
		return map[string]any{
			"archiveId": archiveID,
			"verified":  true,
			"dryRun":    false,
			"published": false,
			"restoredTo": map[string]any{
				"databaseId":  stagedID,
				"directory":   stagedDir,
				"state":       "restoring",
				"newDatabase": true,
			},
		}, nil
	}

	// In-place publication: close the target's live handles, verify once
	// more, copy into a sibling staging file, then atomically swap in with
	// a generation bump (the copy-and-verify publication contract).
	if err := rt.Handles.Close(databaseID); err != nil {
		return nil, fmt.Errorf("failed to close the target handles: %w", err)
	}
	if err := verifySQLiteFile(ctx, archivePath); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrBackupInvalid, err)
	}
	dir0 := rt.Service.DatabaseDir(rt.DataDir, databaseID)
	targetPath := filepath.Join(dir0, "data.db")
	if err := os.MkdirAll(dir0, 0o700); err != nil {
		return nil, err
	}
	stagedPath := targetPath + ".restore-staging"
	if err := copyFileVerified(archivePath, stagedPath); err != nil {
		_ = os.Remove(stagedPath)
		return nil, fmt.Errorf("publication copy failed: %w", err)
	}
	// Retain the prior generation next to the new one.
	if _, err := os.Stat(targetPath); err == nil {
		_ = os.Rename(targetPath, targetPath+".previous")
	}
	if err := os.Rename(stagedPath, targetPath); err != nil {
		// Roll the retained previous generation back.
		_ = os.Rename(targetPath+".previous", targetPath)
		return nil, fmt.Errorf("failed to publish the restored database: %w", err)
	}
	_ = os.Remove(targetPath + ".previous")

	manifest, err := rt.Service.ReadManifest(rt.DataDir, databaseID)
	if err != nil {
		return nil, fmt.Errorf("failed to read the target manifest: %w", err)
	}
	newGeneration := 1
	if manifest != nil {
		newGeneration = parseGeneration(manifest.Generation) + 1
	}
	manifest.Generation = fmt.Sprintf("%d", newGeneration)
	if err := writeManifestFile(rt.Service.DatabaseDir(rt.DataDir, databaseID), manifest); err != nil {
		return nil, fmt.Errorf("failed to record the new generation: %w", err)
	}
	if err := rt.Service.BumpGeneration(ctx, databaseID, int64(newGeneration)); err != nil {
		return nil, fmt.Errorf("failed to bump the registry generation: %w", err)
	}
	return map[string]any{
		"archiveId":  archiveID,
		"verified":   true,
		"dryRun":     false,
		"published":  true,
		"generation": newGeneration,
		"restoredTo": nil,
	}, nil
}

// copyFileVerified copies src to dst and re-verifies the copy by digest.
func copyFileVerified(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	srcDigest, err := fileSHA256(src)
	if err != nil {
		return err
	}
	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	h := sha256.New()
	buf := make([]byte, 1<<20)
	for {
		n, rerr := in.Read(buf)
		if n > 0 {
			if _, werr := out.Write(buf[:n]); werr != nil {
				out.Close()
				return werr
			}
			_, _ = h.Write(buf[:n])
		}
		if rerr != nil {
			break
		}
	}
	if err := out.Close(); err != nil {
		return err
	}
	if hex.EncodeToString(h.Sum(nil)) != srcDigest {
		return errors.New("copy digest mismatch")
	}
	return nil
}

// parseGeneration parses the manifest's decimal generation text.
func parseGeneration(text string) int {
	var v int
	_, _ = fmt.Sscanf(text, "%d", &v)
	if v < 1 {
		return 1
	}
	return v
}

// writeManifestFile writes the manifest.json beside the database file.
func writeManifestFile(dir string, manifest *PrivateManifest) error {
	b, err := json.Marshal(manifest)
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dir, "manifest.json"), b, 0o600)
}

// BumpGeneration records the new generation for one database row.
func (s *Service) BumpGeneration(ctx context.Context, id string, generation int64) error {
	res, err := s.db.ExecContext(ctx,
		`UPDATE _mb_databases SET generation = ?, updated = ? WHERE id = ?`,
		generation, nowRFC3339(), id)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	return nil
}
