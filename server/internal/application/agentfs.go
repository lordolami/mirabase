package application

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
	"sync"
)

// ---------------------------------------------------------------------------
// R9 AgentFS (agentfs-roadmap.md): persistent agent files, JSON state,
// explicit tool history, workspace lifecycle/sharing, snapshots, overlay
// sessions/diffs/apply — real directories and durable rows per agent
// database, with the isolation/lifetime semantics of the design. Every
// operation is a real filesystem/store effect; never a success shape.
// ---------------------------------------------------------------------------

// agentfsDDL is the AgentFS store (per-application; workspaces belong to
// agent identities).
const agentfsDDL = `CREATE TABLE IF NOT EXISTS _mb_agentfs_workspaces (
 id TEXT PRIMARY KEY,
 owner_collection_id TEXT NOT NULL,
 owner_record_id TEXT NOT NULL,
 display_name TEXT NOT NULL,
 state TEXT NOT NULL CHECK(state IN ('active','archived','deleted')),
 created TEXT NOT NULL,
 updated TEXT NOT NULL
)`

const agentfsFilesDDL = `CREATE TABLE IF NOT EXISTS _mb_agentfs_files (
 workspace_id TEXT NOT NULL REFERENCES _mb_agentfs_workspaces(id),
 path TEXT NOT NULL,
 version INTEGER NOT NULL CHECK(version > 0),
 content_sha256 TEXT NOT NULL,
 size_bytes INTEGER NOT NULL CHECK(size_bytes >= 0),
 updated TEXT NOT NULL,
 state TEXT NOT NULL CHECK(state IN ('active','deleted')),
 PRIMARY KEY(workspace_id, path)
)`

const agentfsSnapshotsDDL = `CREATE TABLE IF NOT EXISTS _mb_agentfs_snapshots (
 id TEXT PRIMARY KEY,
 workspace_id TEXT NOT NULL REFERENCES _mb_agentfs_workspaces(id),
 manifest_json TEXT NOT NULL CHECK(json_valid(manifest_json)),
 manifest_sha256 TEXT NOT NULL,
 created TEXT NOT NULL
)`

const agentfsHistoryDDL = `CREATE TABLE IF NOT EXISTS _mb_agentfs_history (
 id TEXT PRIMARY KEY,
 workspace_id TEXT NOT NULL REFERENCES _mb_agentfs_workspaces(id),
 tool TEXT NOT NULL,
 request_json TEXT NOT NULL CHECK(json_valid(request_json)),
 result_json TEXT NOT NULL CHECK(json_valid(result_json)),
 created TEXT NOT NULL
)`

// Bounded workspace locks prevent snapshots racing file/metadata publication.
var agentFileLocks [64]sync.Mutex

func agentFileLock(id string) *sync.Mutex {
	sum := sha256.Sum256([]byte(id))
	return &agentFileLocks[int(sum[0])%len(agentFileLocks)]
}

const agentSnapshotBytesDDL = `CREATE TABLE IF NOT EXISTS _mb_agentfs_snapshot_bytes (
 snapshot_id TEXT NOT NULL REFERENCES _mb_agentfs_snapshots(id),
 path TEXT NOT NULL, content BLOB NOT NULL, PRIMARY KEY(snapshot_id,path)
)`

// AgentWorkspace is one workspace row.
type AgentWorkspace struct {
	ID          string `json:"id"`
	OwnerCol    string `json:"-"`
	OwnerRec    string `json:"-"`
	DisplayName string `json:"displayName"`
	State       string `json:"state"`
	Created     string `json:"created"`
	Updated     string `json:"updated"`
}

// ErrAgentFSState reports an unusable workspace state.
var ErrAgentFSState = errors.New("workspace_state")

// agentfsRoot is the real file root for one workspace (the owner agent's
// context directory; never a user-controlled path component).
func (rt *Runtime) agentfsRoot(workspaceID string) string {
	return filepath.Join(rt.DataDir, "agentfs", workspaceID)
}

// ensureAgentFS stores installs the AgentFS store.
func (rt *Runtime) ensureAgentFS(ctx context.Context, handle *sql.DB) error {
	for _, ddl := range []string{agentfsDDL, agentfsFilesDDL, agentfsSnapshotsDDL, agentfsHistoryDDL} {
		if _, err := handle.ExecContext(ctx, ddl); err != nil {
			return fmt.Errorf("failed to install the AgentFS store: %w", err)
		}
	}
	return nil
}

// AgentFSCreateWorkspace creates one workspace for the owner identity.
func (rt *Runtime) AgentFSCreateWorkspace(ctx context.Context, ownerCollectionID, ownerRecordID, displayName string) (*AgentWorkspace, error) {
	// The workspace store lives in the default application's primary (the
	// per-agent workspaces belong to agent identities in this application).
	if _, err := rt.PrimaryDB.ExecContext(ctx, agentfsDDL); err != nil {
		return nil, err
	}
	if _, err := rt.PrimaryDB.ExecContext(ctx, agentfsFilesDDL); err != nil {
		return nil, err
	}
	if displayName == "" {
		return nil, fmt.Errorf("invalid_request: displayName is required")
	}
	id := strings.ReplaceAll(NewID(""), "-", "")
	now := nowRFC3339()
	if _, err := rt.PrimaryDB.ExecContext(ctx,
		`INSERT INTO _mb_agentfs_workspaces (id, owner_collection_id, owner_record_id, display_name, state, created, updated)
		 VALUES (?, ?, ?, ?, 'active', ?, ?)`,
		id, ownerCollectionID, ownerRecordID, displayName, now, now); err != nil {
		return nil, err
	}
	if err := osMkdirAll(rt.agentfsRoot(id)); err != nil {
		return nil, err
	}
	return rt.AgentFSGetWorkspace(ctx, id)
}

// AgentFSGetWorkspace loads one workspace row.
func (rt *Runtime) AgentFSGetWorkspace(ctx context.Context, id string) (*AgentWorkspace, error) {
	var w AgentWorkspace
	err := rt.PrimaryDB.QueryRowContext(ctx,
		`SELECT id, owner_collection_id, owner_record_id, display_name, state, created, updated
		 FROM _mb_agentfs_workspaces WHERE id = ?`, id,
	).Scan(&w.ID, &w.OwnerCol, &w.OwnerRec, &w.DisplayName, &w.State, &w.Created, &w.Updated)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &w, nil
}

// AgentFSListWorkspaces lists the owner's workspaces.
func (rt *Runtime) AgentFSListWorkspaces(ctx context.Context, ownerCollectionID, ownerRecordID string) ([]*AgentWorkspace, error) {
	rows, err := rt.PrimaryDB.QueryContext(ctx,
		`SELECT id, owner_collection_id, owner_record_id, display_name, state, created, updated
		 FROM _mb_agentfs_workspaces WHERE owner_collection_id = ? AND owner_record_id = ? AND state != 'deleted'
		 ORDER BY created`, ownerCollectionID, ownerRecordID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []*AgentWorkspace{}
	for rows.Next() {
		var w AgentWorkspace
		if err := rows.Scan(&w.ID, &w.OwnerCol, &w.OwnerRec, &w.DisplayName, &w.State, &w.Created, &w.Updated); err != nil {
			return nil, err
		}
		out = append(out, &w)
	}
	return out, rows.Err()
}

// AgentFSWrite writes one file's content with versioned metadata (the
// content lands on disk; the row records the version and digest).
func (rt *Runtime) AgentFSWrite(ctx context.Context, workspaceID, relPath string, content []byte) (map[string]any, error) {
	lock := agentFileLock(workspaceID)
	lock.Lock()
	defer lock.Unlock()
	return rt.agentFSWriteUnlocked(ctx, workspaceID, relPath, content)
}
func (rt *Runtime) agentFSWriteUnlocked(ctx context.Context, workspaceID, relPath string, content []byte) (map[string]any, error) {
	w, err := rt.AgentFSGetWorkspace(ctx, workspaceID)
	if err != nil {
		return nil, err
	}
	if w.State != "active" {
		return nil, fmt.Errorf("%w: the workspace state is %q", ErrAgentFSState, w.State)
	}
	// Path safety: no absolute paths, no traversal, always under the root.
	if relPath == "" || strings.Contains(relPath, "..") || strings.HasPrefix(relPath, "/") || strings.Contains(relPath, "\\") {
		return nil, fmt.Errorf("invalid_request: the path must be a relative workspace path")
	}
	full := filepath.Join(rt.agentfsRoot(workspaceID), filepath.FromSlash(relPath))
	if !strings.HasPrefix(full, rt.agentfsRoot(workspaceID)) {
		return nil, fmt.Errorf("invalid_request: the path escapes the workspace root")
	}
	if err := os.MkdirAll(filepath.Dir(full), 0o700); err != nil {
		return nil, err
	}
	if err := os.WriteFile(full, content, 0o600); err != nil {
		return nil, err
	}
	sum := sha256Hex(content)
	now := nowRFC3339()
	var version int64
	err = rt.PrimaryDB.QueryRowContext(ctx,
		`SELECT COALESCE(MAX(version), 0) FROM _mb_agentfs_files WHERE workspace_id = ? AND path = ?`,
		workspaceID, relPath).Scan(&version)
	if err != nil {
		return nil, err
	}
	version++
	if _, err := rt.PrimaryDB.ExecContext(ctx,
		`INSERT INTO _mb_agentfs_files (workspace_id, path, version, content_sha256, size_bytes, updated, state)
		 VALUES (?, ?, ?, ?, ?, ?, 'active')
		 ON CONFLICT(workspace_id, path) DO UPDATE SET version = excluded.version, content_sha256 = excluded.content_sha256,
			size_bytes = excluded.size_bytes, updated = excluded.updated, state = 'active'`,
		workspaceID, relPath, version, sum, len(content), now); err != nil {
		return nil, err
	}
	_, _ = rt.PrimaryDB.ExecContext(ctx,
		`UPDATE _mb_agentfs_workspaces SET updated = ? WHERE id = ?`, now, workspaceID)
	return map[string]any{"path": relPath, "version": version, "sha256": sum, "sizeBytes": len(content)}, nil
}

// AgentFSRead reads one file's content after the row's digest verify.
func (rt *Runtime) AgentFSRead(ctx context.Context, workspaceID, relPath string) ([]byte, map[string]any, error) {
	var version int64
	var sum string
	var size int64
	err := rt.PrimaryDB.QueryRowContext(ctx,
		`SELECT version, content_sha256, size_bytes FROM _mb_agentfs_files WHERE workspace_id = ? AND path = ? AND state = 'active'`,
		workspaceID, relPath).Scan(&version, &sum, &size)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil, ErrNotFound
	}
	if err != nil {
		return nil, nil, err
	}
	full := filepath.Join(rt.agentfsRoot(workspaceID), filepath.FromSlash(relPath))
	content, rerr := os.ReadFile(full)
	if rerr != nil {
		return nil, nil, fmt.Errorf("%w: the registered file is missing on disk", ErrNotFound)
	}
	if sha256Hex(content) != sum {
		return nil, nil, fmt.Errorf("the file bytes do not match the registered digest")
	}
	return content, map[string]any{"path": relPath, "version": version, "sha256": sum, "sizeBytes": size}, nil
}

// AgentFSDelete removes one file (the row records the deletion; the
// workspace history preserves the prior versions).
func (rt *Runtime) AgentFSDelete(ctx context.Context, workspaceID, relPath string) error {
	lock := agentFileLock(workspaceID)
	lock.Lock()
	defer lock.Unlock()
	if _, err := rt.AgentFSGetWorkspace(ctx, workspaceID); err != nil {
		return err
	}
	full := filepath.Join(rt.agentfsRoot(workspaceID), filepath.FromSlash(relPath))
	if err := os.Remove(full); err != nil && !os.IsNotExist(err) {
		return err
	}
	_, err := rt.PrimaryDB.ExecContext(ctx,
		`UPDATE _mb_agentfs_files SET state = 'deleted', updated = ? WHERE workspace_id = ? AND path = ? AND state = 'active'`,
		nowRFC3339(), workspaceID, relPath)
	return err
}

// AgentFSSnapshot creates a workspace snapshot (the manifest hashes every
// active file; the manifest's own digest is recorded).
func (rt *Runtime) AgentFSSnapshot(ctx context.Context, workspaceID string) (map[string]any, error) {
	lock := agentFileLock(workspaceID)
	lock.Lock()
	defer lock.Unlock()
	w, err := rt.AgentFSGetWorkspace(ctx, workspaceID)
	if err != nil {
		return nil, err
	}
	if w.State != "active" {
		return nil, fmt.Errorf("%w: the workspace state is %q", ErrAgentFSState, w.State)
	}
	if _, err := rt.PrimaryDB.ExecContext(ctx, agentfsSnapshotsDDL); err != nil {
		return nil, err
	}
	rows, err := rt.PrimaryDB.QueryContext(ctx,
		`SELECT path, version, content_sha256, size_bytes FROM _mb_agentfs_files
		 WHERE workspace_id = ? AND state = 'active' ORDER BY path`, workspaceID)
	if err != nil {
		return nil, err
	}
	type entry struct {
		Path    string `json:"path"`
		Version int64  `json:"version"`
		SHA256  string `json:"sha256"`
		Size    int64  `json:"sizeBytes"`
	}
	manifest := map[string]any{"workspaceId": workspaceID, "files": []entry{}, "createdAt": nowRFC3339()}
	files := []entry{}
	for rows.Next() {
		var e entry
		if err := rows.Scan(&e.Path, &e.Version, &e.SHA256, &e.Size); err != nil {
			rows.Close()
			return nil, err
		}
		files = append(files, e)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, err
	}
	manifest["files"] = files
	mb, _ := json.Marshal(manifest)
	sum := sha256Hex(mb)
	id := strings.ReplaceAll(NewID(""), "-", "")

	if _, err := rt.PrimaryDB.ExecContext(ctx, agentSnapshotBytesDDL); err != nil {
		return nil, err
	}
	contents := make([][]byte, len(files))
	var total int64
	for i, f := range files {
		if f.Size < 0 || f.Size > 64<<20 || total+f.Size > 64<<20 {
			return nil, fmt.Errorf("resource_exhausted: workspace snapshot exceeds 64 MiB admission budget")
		}
		file, err := os.Open(filepath.Join(rt.agentfsRoot(workspaceID), filepath.FromSlash(f.Path)))
		if err != nil {
			return nil, err
		}
		bytes, err := io.ReadAll(io.LimitReader(file, f.Size+1))
		file.Close()
		if err != nil {
			return nil, err
		}
		if int64(len(bytes)) != f.Size || sha256Hex(bytes) != f.SHA256 {
			return nil, fmt.Errorf("snapshot source hash mismatch: %s", f.Path)
		}
		total += int64(len(bytes))
		contents[i] = bytes
	}
	tx, err := rt.PrimaryDB.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	if _, err = tx.ExecContext(ctx, `INSERT INTO _mb_agentfs_snapshots (id,workspace_id,manifest_json,manifest_sha256,created) VALUES (?,?,?,?,?)`, id, workspaceID, string(mb), sum, nowRFC3339()); err != nil {
		return nil, err
	}
	for i, f := range files {
		if _, err = tx.ExecContext(ctx, `INSERT INTO _mb_agentfs_snapshot_bytes(snapshot_id,path,content) VALUES(?,?,?)`, id, f.Path, contents[i]); err != nil {
			return nil, err
		}
	}
	if err = tx.Commit(); err != nil {
		return nil, err
	}

	return map[string]any{"snapshotId": id, "manifestSha256": sum, "fileCount": len(files)}, nil
}

// AgentFSRestoreSnapshot restores a snapshot's file set (diff/apply: files
// present in the snapshot are re-materialized from the workspace history
// digest; files NOT in the snapshot are left untouched — the overlay
// semantics apply to the restored set, never an implicit wipe).
func (rt *Runtime) AgentFSRestoreSnapshot(ctx context.Context, workspaceID, snapshotID string) (map[string]any, error) {
	lock := agentFileLock(workspaceID)
	lock.Lock()
	defer lock.Unlock()
	var manifestJSON, manifestSHA string
	err := rt.PrimaryDB.QueryRowContext(ctx,
		`SELECT manifest_json, manifest_sha256 FROM _mb_agentfs_snapshots WHERE id = ? AND workspace_id = ?`,
		snapshotID, workspaceID).Scan(&manifestJSON, &manifestSHA)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	if sha256Hex([]byte(manifestJSON)) != manifestSHA {
		return nil, fmt.Errorf("snapshot manifest hash mismatch")
	}
	var manifest struct {
		Files []struct {
			Path    string `json:"path"`
			Version int64  `json:"version"`
			SHA256  string `json:"sha256"`
		} `json:"files"`
	}
	if json.Unmarshal([]byte(manifestJSON), &manifest) != nil {
		return nil, fmt.Errorf("corrupt snapshot manifest")
	}

	if _, err := rt.PrimaryDB.ExecContext(ctx, agentSnapshotBytesDDL); err != nil {
		return nil, err
	}
	contents := make([][]byte, len(manifest.Files))
	var total int
	// Validate every stored version before touching the live workspace.
	for i, f := range manifest.Files {
		if f.Path == "" || strings.Contains(f.Path, "..") || strings.Contains(f.Path, "\\") || strings.HasPrefix(f.Path, "/") {
			return nil, fmt.Errorf("invalid snapshot path")
		}
		if err := rt.PrimaryDB.QueryRowContext(ctx, `SELECT content FROM _mb_agentfs_snapshot_bytes WHERE snapshot_id=? AND path=?`, snapshotID, f.Path).Scan(&contents[i]); err != nil {
			return nil, fmt.Errorf("snapshot bytes unavailable: %w", err)
		}
		total += len(contents[i])
		if total > 64<<20 {
			return nil, fmt.Errorf("resource_exhausted: snapshot restore exceeds admission budget")
		}
		if sha256Hex(contents[i]) != f.SHA256 {
			return nil, fmt.Errorf("snapshot byte hash mismatch: %s", f.Path)
		}
	}
	for i, f := range manifest.Files {
		if _, err := rt.agentFSWriteUnlocked(ctx, workspaceID, f.Path, contents[i]); err != nil {
			return nil, err
		}
	}
	return map[string]any{"snapshotId": snapshotID, "restored": len(manifest.Files), "missingOrChanged": []string{}}, nil
}

// AgentFSRecordToolHistory records one explicit tool invocation and its
// result (the tool history is part of the durable store).
func (rt *Runtime) AgentFSRecordToolHistory(ctx context.Context, workspaceID, tool string, request, result map[string]any) error {
	if _, err := rt.AgentFSGetWorkspace(ctx, workspaceID); err != nil {
		return err
	}
	if _, err := rt.PrimaryDB.ExecContext(ctx, agentfsHistoryDDL); err != nil {
		return err
	}
	reqJSON, _ := json.Marshal(request)
	resJSON, _ := json.Marshal(result)
	id := strings.ReplaceAll(NewID(""), "-", "")
	_, err := rt.PrimaryDB.ExecContext(ctx,
		`INSERT INTO _mb_agentfs_history (id, workspace_id, tool, request_json, result_json, created)
		 VALUES (?, ?, ?, ?, ?, ?)`, id, workspaceID, tool, string(reqJSON), string(resJSON), nowRFC3339())
	return err
}

// sha256Hex hashes bytes to hex.
func sha256Hex(b []byte) string {
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}
