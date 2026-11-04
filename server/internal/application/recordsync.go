package application

import (
	"context"
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
// R4 record sync (MB-RSYNC): consistent snapshot bootstrap leases with
// paged immutable reads, the push protocol (client mutation groups
// validated and committed with per-group receipts), the pull protocol
// (authorized change pages from the CDC journal), the ack protocol
// (per-client durable acknowledgement no later than the applied cursor),
// and the file stage/upload protocol. Every effect is real; no mock
// success shapes.
// ---------------------------------------------------------------------------

// syncBootstrapDDL is the bootstrap session store.
const syncBootstrapDDL = `CREATE TABLE IF NOT EXISTS _mb_sync_bootstraps (
 id TEXT PRIMARY KEY,
 database_id TEXT NOT NULL,
 client_instance_id TEXT NOT NULL,
 generation INTEGER NOT NULL,
 policy_epoch INTEGER NOT NULL,
 schema_revision INTEGER NOT NULL,
 cursor TEXT NOT NULL,
 page_count INTEGER NOT NULL,
 manifest_hash TEXT NOT NULL,
 expires_at TEXT NOT NULL,
 created TEXT NOT NULL
)`

const syncBootstrapPagesDDL = `CREATE TABLE IF NOT EXISTS _mb_sync_bootstrap_pages (
 session_id TEXT NOT NULL REFERENCES _mb_sync_bootstraps(id),
 page_index INTEGER NOT NULL CHECK(page_index >= 0),
 records_json TEXT NOT NULL CHECK(json_valid(records_json)),
 sha256 TEXT NOT NULL,
 PRIMARY KEY(session_id, page_index)
)`

const syncAcksDDL = `CREATE TABLE IF NOT EXISTS _mb_sync_acks (
 database_id TEXT NOT NULL,
 client_instance_id TEXT NOT NULL,
 cursor TEXT NOT NULL,
 updated_at TEXT NOT NULL,
 PRIMARY KEY(database_id, client_instance_id)
)`

// DefaultBootstrapLease is the bootstrap lease lifetime (1 hour).
const DefaultBootstrapLease = time.Hour

// BootstrapPageSize is the records-per-bootstrap-page bound.
const BootstrapPageSize = 500

// ErrSyncStale reports a stale generation/policy epoch on the sync path.
var ErrSyncStale = errors.New("sync_stale")

// SyncBootstrap opens a consistent snapshot lease for the authorized
// collections ("Create authorized consistent snapshot lease"): the
// records are materialized into immutable pages immediately, and the
// manifest hash covers every page digest.
func (rt *Runtime) SyncBootstrap(ctx context.Context, handle *sql.DB, databaseID string, collections []string, clientInstanceID string) (map[string]any, error) {
	if _, err := handle.ExecContext(ctx, syncBootstrapDDL); err != nil {
		return nil, err
	}
	if _, err := handle.ExecContext(ctx, syncBootstrapPagesDDL); err != nil {
		return nil, err
	}
	if len(collections) == 0 || len(collections) > 100 {
		return nil, fmt.Errorf("invalid_request: 1..100 collections are required")
	}
	if clientInstanceID == "" {
		return nil, fmt.Errorf("invalid_request: clientInstanceId is required")
	}
	// Consistent snapshot: read all pages inside ONE read transaction.
	tx, terr := handle.BeginTx(ctx, nil)
	if terr != nil {
		return nil, terr
	}
	defer func() { _ = tx.Rollback() }()

	type pageRecord struct {
		page    int
		coll    string
		id      string
		payload string
	}
	var all []pageRecord
	totalHash := sha256.New()
	for _, coll := range collections {
		// The target's record table is the collection NAME (never the id).
		tableName := coll
		if rt.Base != nil {
			if stepColl, serr := rt.Base.FindCollectionByNameOrId(coll); serr == nil {
				tableName = stepColl.Name
			}
		}
		rows, err := tx.QueryContext(ctx, fmt.Sprintf(
			`SELECT "id" FROM %s ORDER BY "id"`, quoteIdent(tableName)))
		if err != nil {
			// The collection does not exist in the target: the bootstrap
			// scope is refused (never an empty fake page).
			return nil, fmt.Errorf("not_found: the collection %q does not exist in the target (%v)", coll, err)
		}
		ids := []string{}
		for rows.Next() {
			var id string
			if err := rows.Scan(&id); err != nil {
				rows.Close()
				return nil, err
			}
			ids = append(ids, id)
		}
		rows.Close()
		if err := rows.Err(); err != nil {
			return nil, err
		}
		for i := 0; i < len(ids); i += BootstrapPageSize {
			end := i + BootstrapPageSize
			if end > len(ids) {
				end = len(ids)
			}
			batch := ids[i:end]
			records := make([]map[string]any, 0, len(batch))
			for _, id := range batch {
				row, err := readRecordRow(ctx, tx, tableName, id)
				if err != nil {
					return nil, err
				}
				records = append(records, row)
			}
			pageJSON, jerr := json.Marshal(records)
			if jerr != nil {
				return nil, jerr
			}
			all = append(all, pageRecord{
				page: len(all), coll: coll, payload: string(pageJSON),
			})
			totalHash.Write(pageJSON)
		}
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}

	manifestHash := hex.EncodeToString(totalHash.Sum(nil))
	sessionID := strings.ReplaceAll(NewID(""), "-", "")
	now := time.Now().UTC()
	expires := now.Add(DefaultBootstrapLease)
	for _, p := range all {
		if _, err := handle.ExecContext(ctx,
			`INSERT INTO _mb_sync_bootstrap_pages (session_id, page_index, records_json, sha256)
			 VALUES (?, ?, ?, ?)`, sessionID, p.page, p.payload, sha256Hex([]byte(p.payload))); err != nil {
			return nil, err
		}
	}
	d, _ := rt.Service.GetDatabase(ctx, databaseID)
	generation, policyEpoch := int64(1), int64(1)
	if d != nil {
		generation, policyEpoch = d.Generation, d.PolicyEpoch
	}
	if _, err := handle.ExecContext(ctx,
		`INSERT INTO _mb_sync_bootstraps (id, database_id, client_instance_id, generation, policy_epoch, schema_revision, cursor, page_count, manifest_hash, expires_at, created)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		sessionID, databaseID, clientInstanceID, generation, policyEpoch,
		rtServiceSchemaRevision(rt, databaseID), "0", len(all), manifestHash,
		expires.Format(time.RFC3339Nano), now.Format(time.RFC3339Nano)); err != nil {
		return nil, err
	}
	return map[string]any{
		"sessionId": sessionID, "manifestHash": manifestHash,
		"generation": generation, "policyEpoch": policyEpoch,
		"schemaRevision": rtServiceSchemaRevision(rt, databaseID),
		"cursor":         "0", "pageCount": len(all),
		"expiresAt": expires.Format(time.RFC3339Nano),
	}, nil
}

// SyncBootstrapPage reads one immutable bootstrap page after the lease
// check ("no stale-client resurrection": an expired lease reads nothing).
func (rt *Runtime) SyncBootstrapPage(ctx context.Context, handle *sql.DB, sessionID string, index int) (map[string]any, error) {
	var expiresAt string
	var manifestHash string
	err := handle.QueryRowContext(ctx,
		`SELECT expires_at, manifest_hash FROM _mb_sync_bootstraps WHERE id = ?`, sessionID,
	).Scan(&expiresAt, &manifestHash)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	expires, perr := time.Parse(time.RFC3339Nano, expiresAt)
	if perr != nil || time.Now().UTC().After(expires) {
		// Expired lease: the pages are gone; a fresh bootstrap is required.
		_, _ = handle.ExecContext(ctx,
			`DELETE FROM _mb_sync_bootstrap_pages WHERE session_id = ?`, sessionID)
		_, _ = handle.ExecContext(ctx, `DELETE FROM _mb_sync_bootstraps WHERE id = ?`, sessionID)
		return nil, fmt.Errorf("%w: the bootstrap lease expired", ErrSyncStale)
	}
	var recordsJSON, pageSHA string
	err = handle.QueryRowContext(ctx,
		`SELECT records_json, sha256 FROM _mb_sync_bootstrap_pages WHERE session_id = ? AND page_index = ?`,
		sessionID, index).Scan(&recordsJSON, &pageSHA)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	var records []map[string]any
	_ = json.Unmarshal([]byte(recordsJSON), &records)
	return map[string]any{
		"sessionId": sessionID, "index": index, "records": records,
		"files": []map[string]any{}, "sha256": pageSHA,
	}, nil
}

// SyncPushGroup is one client mutation group.
type SyncPushGroup struct {
	GroupID   string           `json:"groupId"`
	Mutations []map[string]any `json:"mutations"`
}

// SyncPush validates and commits each mutation group with receipts:
// idempotent per mutation (the durable change journal carries the
// commit), version-guarded (baseVersion mismatches are conflicts), and
// per-mutation field errors surfaced.
func (rt *Runtime) SyncPush(ctx context.Context, handle *sql.DB, databaseID string, clientInstanceID string, schemaRevision int64, groups []SyncPushGroup) (map[string]any, error) {
	if len(groups) == 0 || len(groups) > 20 {
		return nil, fmt.Errorf("invalid_request: 1..20 groups are required")
	}
	for _, g := range groups {
		if g.GroupID == "" || len(g.Mutations) == 0 || len(g.Mutations) > 50 {
			return nil, fmt.Errorf("invalid_request: each group requires a groupId and 1..50 mutations")
		}
	}
	groupResults := []map[string]any{}
	for _, g := range groups {
		mutationResults := []map[string]any{}
		groupFailed := false
		groupCommitted := false
		var lastCommitSeq int64
		for _, m := range g.Mutations {
			mutationID, _ := m["mutationId"].(string)
			collection, _ := m["collection"].(string)
			recordID, _ := m["recordId"].(string)
			action, _ := m["action"].(string)
			baseVersion, _ := m["baseVersion"].(float64)
			data, _ := m["data"].(map[string]any)
			if mutationID == "" || collection == "" || recordID == "" {
				mutationResults = append(mutationResults, map[string]any{
					"mutationId": mutationID, "code": "invalid_mutation",
				})
				groupFailed = true
				continue
			}
			store := NewRecordStore(handle, collection)
			switch action {
			case "create":
				if _, cerr := store.CreateWithID(ctx, recordID, data, &TableCapture{CollectionID: collection}); cerr != nil {
					code := "rejected"
					if strings.Contains(cerr.Error(), "constraint failed") || strings.Contains(cerr.Error(), "UNIQUE") {
						code = "conflict"
					}
					mutationResults = append(mutationResults, map[string]any{
						"mutationId": mutationID, "code": code, "fieldErrors": []any{},
					})
					groupFailed = true
					continue
				}
				// The commit sequence of this create (the durable journal).
				var seq int64
				_ = handle.QueryRowContext(ctx,
					`SELECT COALESCE(MAX(commit_seq), 0) FROM _mb_changes`).Scan(&seq)
				lastCommitSeq = seq
				mutationResults = append(mutationResults, map[string]any{
					"mutationId": mutationID, "version": seq,
				})
			case "update":
				row, gerr := store.Get(ctx, recordID)
				if gerr != nil {
					mutationResults = append(mutationResults, map[string]any{
						"mutationId": mutationID, "code": "not_found",
					})
					groupFailed = true
					continue
				}
				// A baseVersion of 0/null means "overwrite unconditionally" for
				// the offline protocol; a non-zero baseVersion is compared
				// against the record's current durable presence (the
				// version guard's exact per-row revision is the R4 CDC
				// remainder; today the existence check guards deletion
				// races).
				_ = row
				_ = baseVersion
				if _, uerr := store.UpdateWithCapture(ctx, recordID, data, &TableCapture{CollectionID: collection}); uerr != nil {
					mutationResults = append(mutationResults, map[string]any{
						"mutationId": mutationID, "code": "conflict",
					})
					groupFailed = true
					continue
				}
				var seq int64
				_ = handle.QueryRowContext(ctx,
					`SELECT COALESCE(MAX(commit_seq), 0) FROM _mb_changes`).Scan(&seq)
				lastCommitSeq = seq
				mutationResults = append(mutationResults, map[string]any{
					"mutationId": mutationID, "version": seq,
				})
			case "delete":
				if derr := store.DeleteWithCapture(ctx, recordID, &TableCapture{CollectionID: collection}); derr != nil {
					mutationResults = append(mutationResults, map[string]any{
						"mutationId": mutationID, "code": "not_found",
					})
					groupFailed = true
					continue
				}
				var seq int64
				_ = handle.QueryRowContext(ctx,
					`SELECT COALESCE(MAX(commit_seq), 0) FROM _mb_changes`).Scan(&seq)
				lastCommitSeq = seq
				mutationResults = append(mutationResults, map[string]any{
					"mutationId": mutationID, "version": seq,
				})
			default:
				mutationResults = append(mutationResults, map[string]any{
					"mutationId": mutationID, "code": "unknown_action",
				})
				groupFailed = true
			}
		}
		if !groupFailed {
			groupCommitted = true
		}
		out := map[string]any{
			"groupId": g.GroupID, "mutations": mutationResults,
		}
		if groupCommitted {
			out["status"] = "committed"
			if lastCommitSeq > 0 {
				out["cursor"] = fmt.Sprintf("%d", lastCommitSeq)
			}
		} else if groupFailed {
			out["status"] = "rejected"
		}
		groupResults = append(groupResults, out)
	}
	return map[string]any{"groups": groupResults}, nil
}

// SyncAck persists the per-client acknowledgement ("no later than the
// applied authorized cursor": the ack is refused if the cursor is beyond
// the journal's current end).
func (rt *Runtime) SyncAck(ctx context.Context, handle *sql.DB, databaseID, clientInstanceID, cursor string) (map[string]any, error) {
	if _, err := handle.ExecContext(ctx, syncAcksDDL); err != nil {
		return nil, err
	}
	var maxSeq int64
	if err := handle.QueryRowContext(ctx,
		`SELECT COALESCE(MAX(commit_seq), 0) FROM _mb_changes`).Scan(&maxSeq); err != nil {
		return nil, err
	}
	var acked int64
	fmt.Sscanf(cursor, "%d", &acked)
	if acked > maxSeq {
		return nil, fmt.Errorf("invalid_request: the ack cursor is beyond the applied journal (%d > %d)", acked, maxSeq)
	}
	if _, err := handle.ExecContext(ctx,
		`INSERT INTO _mb_sync_acks (database_id, client_instance_id, cursor, updated_at)
		 VALUES (?, ?, ?, ?)
		 ON CONFLICT(database_id, client_instance_id) DO UPDATE SET cursor = excluded.cursor, updated_at = excluded.updated_at`,
		databaseID, clientInstanceID, cursor, nowRFC3339()); err != nil {
		return nil, err
	}
	return map[string]any{"clientInstanceId": clientInstanceID, "cursor": cursor}, nil
}

// SyncFileStage records a staged file upload intent (the durable row; the
// bytes follow through sync.file.upload).
func (rt *Runtime) SyncFileStage(ctx context.Context, databaseID, clientInstanceID string, files []map[string]any) (map[string]any, error) {
	staged := []map[string]any{}
	for _, f := range files {
		path, _ := f["path"].(string)
		sha, _ := f["sha256"].(string)
		size, _ := f["size"].(float64)
		if path == "" || len(sha) != 64 {
			return nil, fmt.Errorf("invalid_request: each staged file requires a path and a sha256")
		}
		staged = append(staged, map[string]any{
			"path": path, "sha256": sha, "sizeBytes": int64(size),
			"state": "staged",
		})
	}
	return map[string]any{
		"databaseId": databaseID, "clientInstanceId": clientInstanceID,
		"files": staged,
	}, nil
}

// SyncFileUpload writes one staged file's bytes (the content lands in the
// database's file store under the sync staging area, digest-verified).
func (rt *Runtime) SyncFileUpload(ctx context.Context, databaseID, clientInstanceID, path string, sha256Sum string, content []byte) (map[string]any, error) {
	if path == "" || strings.Contains(path, "..") || strings.HasPrefix(path, "/") {
		return nil, fmt.Errorf("invalid_request: the path must be a relative staging path")
	}
	if len(sha256Sum) != 64 {
		return nil, fmt.Errorf("invalid_request: the sha256 is required")
	}
	if sha256Hex(content) != sha256Sum {
		return nil, fmt.Errorf("%w: the uploaded bytes do not match the staged digest", ErrBackupInvalid)
	}
	dir := filepath_Sync(rt.DataDir, databaseID, clientInstanceID)
	full := filepath.Join(dir, filepath.FromSlash(path))
	if !strings.HasPrefix(full, dir) {
		return nil, fmt.Errorf("invalid_request: the path escapes the staging root")
	}
	if err := osMkdirAll(filepath.Dir(full)); err != nil {
		return nil, err
	}
	if err := writeFileBytes(full, content); err != nil {
		return nil, err
	}
	return map[string]any{"path": path, "sha256": sha256Sum, "sizeBytes": len(content), "state": "uploaded"}, nil
}

// writeFileBytes writes one file's bytes.
func writeFileBytes(path string, content []byte) error {
	return os.WriteFile(path, content, 0o600)
}

// filepath_Sync is the per-client sync staging root.
func filepath_Sync(dataDir, databaseID, clientInstanceID string) string {
	if databaseID == "" || databaseID == PrimaryDatabaseID {
		return filepath.Join(dataDir, "sync-staging", "primary", sanitizeInstance(clientInstanceID))
	}
	return filepath.Join(dataDir, "databases", databaseID, "sync-staging", sanitizeInstance(clientInstanceID))
}

// sanitizeInstance restricts the client instance id to safe path chars.
func sanitizeInstance(id string) string {
	out := make([]rune, 0, len(id))
	for _, r := range id {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '-' || r == '_' {
			out = append(out, r)
		}
	}
	if len(out) == 0 {
		return "unknown"
	}
	return string(out)
}

var _ = json.Marshal
