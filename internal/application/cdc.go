package application

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
)

// ---------------------------------------------------------------------------
// R4 durable changes (MB-CDC): transaction-grouped CDC capture with a
// durable per-database commit journal, cursor pull over complete authorized
// commit frames, and tombstone retention. The capture rides the record
// interceptor path (every record write appends its change rows inside the
// SAME transaction); pull reads whole authorized frames with cursor
// retention and redaction (data of unauthorized collections is reduced to
// evict markers, never raw rows).
// ---------------------------------------------------------------------------

// DefaultChangeRetention is the change journal retention (7 days).
const DefaultChangeRetention = 7 * 24 * time.Hour

// changesJournalDDL is the durable per-database change journal.
const changesJournalDDL = `CREATE TABLE IF NOT EXISTS _mb_changes (
 commit_seq INTEGER PRIMARY KEY,
 transaction_id TEXT NOT NULL,
 generation INTEGER NOT NULL,
 schema_revision INTEGER NOT NULL,
 source TEXT NOT NULL CHECK(source IN ('record','admin-sql','pg','native-sync','migration')),
 timestamp TEXT NOT NULL
)`

const changesRowsDDL = `CREATE TABLE IF NOT EXISTS _mb_change_rows (
 commit_seq INTEGER NOT NULL REFERENCES _mb_changes(commit_seq),
 ordinal INTEGER NOT NULL,
 table_name TEXT NOT NULL,
 collection_id TEXT,
 record_id TEXT NOT NULL,
 action TEXT NOT NULL CHECK(action IN ('create','update','delete','schema','evict')),
 before_json BLOB,
 after_json BLOB,
 PRIMARY KEY(commit_seq, ordinal)
)`

// ChangeCapture is one captured record change.
type ChangeCapture struct {
	Table         string
	CollectionID  *string
	RecordID      string
	Action        string // create|update|delete|schema|evict
	Before        map[string]any
	After         map[string]any
}

// CaptureRunner is the minimal write surface CaptureChangesOnRunner needs
// (satisfied by *sql.Tx and *sql.Conn — the coordinator's transaction and
// per-session engine connection).
type CaptureRunner interface {
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
}

// CaptureChanges appends one transaction group's changes into the durable
// journal inside the CALLER'S transaction (atomic capture: the change rows
// commit with the data they describe).
func CaptureChanges(ctx context.Context, tx *sql.Tx, source string, captures []ChangeCapture) (int64, error) {
	return CaptureChangesOnRunner(ctx, tx, source, captures)
}

// CaptureChangesOnRunner appends the change frame over any write runner
// already inside the caller's transaction scope (the record interceptor's
// *sql.Tx, the PG gateway's per-statement transaction or its open
// transaction-block connection). The caller owns commit/rollback.
func CaptureChangesOnRunner(ctx context.Context, r CaptureRunner, source string, captures []ChangeCapture) (int64, error) {
	if _, err := r.ExecContext(ctx, changesJournalDDL); err != nil {
		return 0, err
	}
	if _, err := r.ExecContext(ctx, changesRowsDDL); err != nil {
		return 0, err
	}
	if len(captures) == 0 {
		return 0, nil
	}
	var commitSeq int64
	if err := r.QueryRowContext(ctx,
		`INSERT INTO _mb_changes (transaction_id, generation, schema_revision, source, timestamp)
		 VALUES (?, 1, 1, ?, ?) RETURNING commit_seq`,
		randomHexID("txn-"), source, time.Now().UTC().Format(time.RFC3339Nano)).Scan(&commitSeq); err != nil {
		return 0, err
	}
	for i, c := range captures {
		var beforeJSON, afterJSON []byte
		if c.Before != nil {
			b, err := json.Marshal(c.Before)
			if err != nil {
				return 0, err
			}
			beforeJSON = b
		}
		if c.After != nil {
			b, err := json.Marshal(c.After)
			if err != nil {
				return 0, err
			}
			afterJSON = b
		}
		if _, err := r.ExecContext(ctx,
			`INSERT INTO _mb_change_rows (commit_seq, ordinal, table_name, collection_id, record_id, action, before_json, after_json)
			 VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
			commitSeq, i, c.Table, c.CollectionID, c.RecordID, c.Action, beforeJSON, afterJSON); err != nil {
			return 0, err
		}
	}
	return commitSeq, nil
}

// ChangeCursor is the opaque pull cursor (commit sequence).
type ChangeCursor struct {
	CommitSeq int64 `json:"commitSeq"`
}

// ChangeTransactionWire is the wire ChangeTransaction.
type ChangeTransactionWire struct {
	DatabaseID     string          `json:"databaseId"`
	Generation     int64           `json:"generation"`
	TransactionID  string          `json:"transactionId"`
	CommitSequence int64           `json:"commitSequence"`
	SchemaRevision int64           `json:"schemaRevision"`
	Timestamp      string          `json:"timestamp"`
	Source         string          `json:"source"`
	Changes        []map[string]any `json:"changes"`
}

// PullChanges reads whole authorized commit frames after the cursor:
// complete transactions only (a partially authorized frame is reduced:
// unauthorized collections' rows become evict markers — the data never
// crosses the wire).
func (rt *Runtime) PullChanges(ctx context.Context, handle *sql.DB, databaseID string, afterCommitSeq int64, limit int, authorizedTables map[string]bool) (map[string]any, error) {
	if _, err := handle.ExecContext(ctx, changesJournalDDL); err != nil {
		return nil, err
	}
	if _, err := handle.ExecContext(ctx, changesRowsDDL); err != nil {
		return nil, err
	}
	if limit < 1 || limit > 1000 {
		limit = 50
	}
	rows, err := handle.QueryContext(ctx,
		`SELECT commit_seq, transaction_id, generation, schema_revision, source, timestamp
		 FROM _mb_changes WHERE commit_seq > ? ORDER BY commit_seq LIMIT ?`,
		afterCommitSeq, limit)
	if err != nil {
		return nil, err
	}
	type frameRow struct {
		seq            int64
		txnID          string
		generation     int64
		schemaRevision int64
		source         string
		timestamp      string
	}
	var frames []frameRow
	for rows.Next() {
		var f frameRow
		if err := rows.Scan(&f.seq, &f.txnID, &f.generation, &f.schemaRevision, &f.source, &f.timestamp); err != nil {
			rows.Close()
			return nil, err
		}
		frames = append(frames, f)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, err
	}

	transactions := []map[string]any{}
	lastSeq := afterCommitSeq
	for _, f := range frames {
		rrows, rerr := handle.QueryContext(ctx,
			`SELECT ordinal, table_name, collection_id, record_id, action, before_json, after_json
			 FROM _mb_change_rows WHERE commit_seq = ? ORDER BY ordinal`, f.seq)
		if rerr != nil {
			return nil, rerr
		}
		changes := []map[string]any{}
		complete := true
		for rrows.Next() {
			var ordinal int
			var table, recordID, action string
			var collectionID sql.NullString
			var beforeJSON, afterJSON []byte
			if err := rrows.Scan(&ordinal, &table, &collectionID, &recordID, &action, &beforeJSON, &afterJSON); err != nil {
				rrows.Close()
				return nil, err
			}
			if !authorizedTables[table] {
				// Redaction: unauthorized rows become evict markers (the
				// frame stays complete; the data never crosses the wire).
				changes = append(changes, map[string]any{
					"ordinal": ordinal, "table": table, "recordId": recordID,
					"action": "evict",
				})
				continue
			}
			entry := map[string]any{
				"ordinal": ordinal, "table": table, "recordId": recordID, "action": action,
			}
			if collectionID.Valid {
				entry["collectionId"] = collectionID.String
			} else {
				entry["collectionId"] = nil
			}
			if len(beforeJSON) > 0 {
				var b map[string]any
				_ = json.Unmarshal(beforeJSON, &b)
				entry["before"] = b
			} else {
				entry["before"] = nil
			}
			if len(afterJSON) > 0 {
				var a map[string]any
				_ = json.Unmarshal(afterJSON, &a)
				entry["after"] = a
			} else {
				entry["after"] = nil
			}
			changes = append(changes, entry)
		}
		rrows.Close()
		if err := rrows.Err(); err != nil {
			return nil, err
		}
		if !complete {
			continue
		}
		transactions = append(transactions, map[string]any{
			"databaseId":     databaseID,
			"generation":     f.generation,
			"transactionId":  f.txnID,
			"commitSequence": f.seq,
			"schemaRevision": f.schemaRevision,
			"timestamp":      f.timestamp,
			"source":         f.source,
			"changes":        changes,
		})
		lastSeq = f.seq
	}
	// Retention floor: the oldest surviving commit sequence.
	var floor int64
	if err := handle.QueryRowContext(ctx,
		`SELECT COALESCE(MIN(commit_seq), 0) FROM _mb_changes`).Scan(&floor); err != nil {
		return nil, err
	}
	// Tombstone/journal expiry: expired rows are pruned on pull (bounded).
	cutoff := time.Now().UTC().Add(-DefaultChangeRetention).Unix()
	_, _ = handle.ExecContext(ctx,
		`DELETE FROM _mb_change_rows WHERE commit_seq IN (SELECT commit_seq FROM _mb_changes WHERE timestamp < ?)`,
		time.Unix(cutoff, 0).UTC().Format(time.RFC3339Nano))
	_, _ = handle.ExecContext(ctx, `DELETE FROM _mb_changes WHERE timestamp < ?`,
		time.Unix(cutoff, 0).UTC().Format(time.RFC3339Nano))

	nextCursor := fmt.Sprintf("%d", lastSeq)
	hasMore := len(frames) == limit
	return map[string]any{
		"transactions":   transactions,
		"nextCursor":     nextCursor,
		"hasMore":        hasMore,
		"schemaRevision": rtServiceSchemaRevision(rt, databaseID),
		"retentionFloor": fmt.Sprintf("%d", floor),
	}, nil
}

// rtServiceSchemaRevision reads the database's current schema revision.
func rtServiceSchemaRevision(rt *Runtime, databaseID string) int64 {
	if databaseID == "" || databaseID == PrimaryDatabaseID {
		return 1
	}
	d, err := rt.Service.GetDatabase(context.Background(), databaseID)
	if err != nil {
		return 1
	}
	return d.SchemaRevision
}

// ---------------------------------------------------------------------------
// R4 scoped credentials (MB-APPAUTH remainder): restricted whole-database
// SQL/replication credentials with one-time secret display, verifier-only
// storage, generation binding and expiry.
// ---------------------------------------------------------------------------

// DefaultCredentialLifetime is the default maximum lifetime (one hour).
const DefaultCredentialLifetime = time.Hour

// credentialsDDL is the credential store (verifiers only; secrets are
// never stored or logged).
const credentialsDDL = `CREATE TABLE IF NOT EXISTS _mb_credentials (
 id TEXT PRIMARY KEY,
 database_id TEXT NOT NULL,
 purpose TEXT NOT NULL CHECK(purpose IN ('pg','native-sync','sql-http')),
 scopes_json TEXT NOT NULL CHECK(json_valid(scopes_json)),
 verifier TEXT NOT NULL,
 expires_at TEXT NOT NULL,
 revoked_at TEXT,
 generation INTEGER NOT NULL CHECK(generation > 0),
 policy_epoch INTEGER NOT NULL CHECK(policy_epoch > 0),
 created TEXT NOT NULL
)`

// CredentialWire is the credential wire shape (secret returned exactly
// once).
type CredentialWire struct {
	ID         string   `json:"id"`
	DatabaseID string   `json:"databaseId"`
	Purpose    string   `json:"-"`
	Scopes     []string `json:"scopes"`
	ExpiresAt  string   `json:"expiresAt"`
	Secret     string   `json:"secret"`
}

// ErrCredentialRevoked reports a revoked/expired credential.
var ErrCredentialRevoked = errors.New("credential_unusable")

// IssueCredential issues one-time secret; retains verifier and audit only.
func (rt *Runtime) IssueCredential(ctx context.Context, databaseID, purpose string, scopes []string, expiresAt time.Time) (*CredentialWire, error) {
	if _, err := rt.PrimaryDB.ExecContext(ctx, credentialsDDL); err != nil {
		return nil, err
	}
	if purpose != "pg" && purpose != "native-sync" && purpose != "sql-http" {
		return nil, fmt.Errorf("invalid_request: purpose must be pg, native-sync or sql-http")
	}
	if len(scopes) == 0 || len(scopes) > 10 {
		return nil, fmt.Errorf("invalid_request: 1..10 scopes are required")
	}
	d := (*Database)(nil)
	var generation, policyEpoch int64 = 1, 1
	if databaseID != "" && databaseID != PrimaryDatabaseID {
		var err error
		d, err = rt.Service.GetDatabase(ctx, databaseID)
		if err != nil {
			return nil, err
		}
		generation = d.Generation
		policyEpoch = d.PolicyEpoch
	}
	// One-time secret: 32 random bytes; only its verifier is stored.
	secret := make([]byte, 32)
	if _, err := rand.Read(secret); err != nil {
		return nil, err
	}
	secretText := hex.EncodeToString(secret)
	verifier := verifierOf(secretText)
	id := strings.ReplaceAll(NewID(""), "-", "")
	now := time.Now().UTC()
	if _, err := rt.PrimaryDB.ExecContext(ctx,
		`INSERT INTO _mb_credentials (id, database_id, purpose, scopes_json, verifier, expires_at, generation, policy_epoch, created)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		id, databaseID, purpose, mustJSON(scopes), verifier,
		expiresAt.UTC().Format(time.RFC3339Nano), generation, policyEpoch, now.Format(time.RFC3339Nano)); err != nil {
		return nil, err
	}
	// PG credentials additionally persist SCRAM-SHA-256 verifier material
	// (postgresql-access.md: "Implement a verified pgwire authentication
	// gateway/adapter with SCRAM-SHA-256 credential verification"). The
	// exchange never carries the plaintext secret, so the keys must be
	// derived at issuance time while the one-time secret is still
	// available; only the RFC 5802 stored/server keys are retained.
	if purpose == "pg" {
		if err := storePGSCRAM(ctx, rt.PrimaryDB, id, secretText); err != nil {
			// Never leave a pg credential without its verifier material:
			// the credential row is removed so a half-issued credential
			// cannot be repaired later into an authenticatable state.
			_, _ = rt.PrimaryDB.ExecContext(ctx,
				`DELETE FROM _mb_credentials WHERE id = ?`, id)
			return nil, err
		}
	}
	return &CredentialWire{
		ID: id, DatabaseID: databaseID, Purpose: purpose, Scopes: scopes,
		ExpiresAt: expiresAt.UTC().Format(time.RFC3339Nano), Secret: secretText,
	}, nil
}

// ListCredentials returns the credential metadata WITHOUT secrets.
func (rt *Runtime) ListCredentials(ctx context.Context, databaseID string) ([]map[string]any, error) {
	if _, err := rt.PrimaryDB.ExecContext(ctx, credentialsDDL); err != nil {
		return nil, err
	}
	rows, err := rt.PrimaryDB.QueryContext(ctx,
		`SELECT id, database_id, purpose, scopes_json, expires_at, revoked_at, generation, policy_epoch
		 FROM _mb_credentials WHERE database_id = ? AND revoked_at IS NULL AND expires_at > ?
		 ORDER BY created`, databaseID, time.Now().UTC().Format(time.RFC3339Nano))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []map[string]any{}
	for rows.Next() {
		var id, purpose, scopesJSON, expiresAt string
		var revokedAt sql.NullString
		var generation, policyEpoch int64
		if err := rows.Scan(&id, &databaseID, &purpose, &scopesJSON, &expiresAt, &revokedAt, &generation, &policyEpoch); err != nil {
			return nil, err
		}
		var scopes []string
		_ = json.Unmarshal([]byte(scopesJSON), &scopes)
		out = append(out, map[string]any{
			"id": id, "databaseId": databaseID, "purpose": purpose, "scopes": scopes,
			"expiresAt": expiresAt, "generation": generation, "policyEpoch": policyEpoch,
		})
	}
	return out, rows.Err()
}

// RevokeCredential revokes one credential.
func (rt *Runtime) RevokeCredential(ctx context.Context, credentialID string) error {
	res, err := rt.PrimaryDB.ExecContext(ctx,
		`UPDATE _mb_credentials SET revoked_at = ? WHERE id = ? AND revoked_at IS NULL`,
		time.Now().UTC().Format(time.RFC3339Nano), credentialID)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	return nil
}

// VerifyCredential validates a presented secret against the stored
// verifier and the database's CURRENT generation ("Restore/replacement
// cannot leave an old SCRAM credential usable merely because its ID and
// password verifier survived the archive").
func (rt *Runtime) VerifyCredential(ctx context.Context, databaseID, secret string) (*CredentialRecord, error) {
	var rec CredentialRecord
	var scopesJSON string
	var revokedAt sql.NullString
	err := rt.PrimaryDB.QueryRowContext(ctx,
		`SELECT id, database_id, purpose, scopes_json, expires_at, revoked_at, generation, policy_epoch
		 FROM _mb_credentials WHERE verifier = ?`, verifierOf(secret),
	).Scan(&rec.ID, &rec.DatabaseID, &rec.Purpose, &scopesJSON, &rec.ExpiresAt, &revokedAt, &rec.Generation, &rec.PolicyEpoch)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	_ = json.Unmarshal([]byte(scopesJSON), &rec.Scopes)
	if revokedAt.Valid {
		return nil, ErrCredentialRevoked
	}
	expires, perr := time.Parse(time.RFC3339Nano, rec.ExpiresAt)
	if perr != nil || time.Now().UTC().After(expires) {
		return nil, ErrCredentialRevoked
	}
	if rec.DatabaseID != "" && rec.DatabaseID != PrimaryDatabaseID {
		d, derr := rt.Service.GetDatabase(ctx, rec.DatabaseID)
		if derr != nil {
			return nil, ErrCredentialRevoked
		}
		// Generation binding: the credential is unusable after a restore
		// or generation bump.
		if d.Generation != rec.Generation {
			return nil, ErrCredentialRevoked
		}
	}
	return &rec, nil
}

// CredentialRecord is one credential row (never carries the secret).
type CredentialRecord struct {
	ID          string
	DatabaseID  string
	Purpose     string
	Scopes      []string
	ExpiresAt   string
	Generation  int64
	PolicyEpoch int64
}

// verifierOf derives the stored verifier from the presented secret.
func verifierOf(secret string) string {
	h := sha256.Sum256([]byte("mirabase-credential:" + secret))
	return hex.EncodeToString(h[:])
}

// ---------------------------------------------------------------------------
// PG SCRAM verifier material (postgresql-access.md): pg-purpose credentials
// authenticate through a real SCRAM-SHA-256 exchange (RFC 5802 / RFC 7677),
// which requires server-side stored keys instead of the verifier column
// (the exchange never presents the plaintext secret, so verifierOf cannot
// be used on the wire). The keys are derived from the one-time secret at
// issuance; the plaintext secret itself is never stored.
// ---------------------------------------------------------------------------

// SCRAMIterations is the PBKDF2 iteration count of the pg SCRAM verifiers
// (PostgreSQL's default scram_iterations).
const SCRAMIterations = 4096

// pgScramDDL is the SCRAM verifier store for pg-purpose credentials,
// keyed by credential id.
const pgScramDDL = `CREATE TABLE IF NOT EXISTS _mb_pg_scram (
 credential_id TEXT PRIMARY KEY,
 salt_hex TEXT NOT NULL,
 iterations INTEGER NOT NULL CHECK(iterations >= 4096),
 stored_key_hex TEXT NOT NULL,
 server_key_hex TEXT NOT NULL,
 created TEXT NOT NULL
)`

// PGSCRAMMaterial is the RFC 5802 verifier material of one pg credential.
type PGSCRAMMaterial struct {
	Salt       []byte
	Iterations int
	StoredKey  []byte
	ServerKey  []byte
}

// storePGSCRAM derives and persists the SCRAM keys of one pg credential.
func storePGSCRAM(ctx context.Context, db *sql.DB, credentialID, secret string) error {
	if _, err := db.ExecContext(ctx, pgScramDDL); err != nil {
		return fmt.Errorf("failed to install the pg scram store: %w", err)
	}
	salt := make([]byte, 16)
	if _, err := rand.Read(salt); err != nil {
		return err
	}
	storedKey, serverKey := scramKeys(secret, salt, SCRAMIterations)
	if _, err := db.ExecContext(ctx,
		`INSERT INTO _mb_pg_scram (credential_id, salt_hex, iterations, stored_key_hex, server_key_hex, created)
		 VALUES (?, ?, ?, ?, ?, ?)`,
		credentialID, hex.EncodeToString(salt), SCRAMIterations,
		hex.EncodeToString(storedKey), hex.EncodeToString(serverKey),
		time.Now().UTC().Format(time.RFC3339Nano)); err != nil {
		return err
	}
	return nil
}

// GetPGSCRAM loads the SCRAM verifier material of one credential.
func (rt *Runtime) GetPGSCRAM(ctx context.Context, credentialID string) (*PGSCRAMMaterial, error) {
	if _, err := rt.PrimaryDB.ExecContext(ctx, pgScramDDL); err != nil {
		return nil, fmt.Errorf("failed to install the pg scram store: %w", err)
	}
	var saltHex, storedHex, serverHex string
	var iterations int64
	err := rt.PrimaryDB.QueryRowContext(ctx,
		`SELECT salt_hex, iterations, stored_key_hex, server_key_hex
		 FROM _mb_pg_scram WHERE credential_id = ?`, credentialID,
	).Scan(&saltHex, &iterations, &storedHex, &serverHex)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	m := &PGSCRAMMaterial{Iterations: int(iterations)}
	if m.Salt, err = hex.DecodeString(saltHex); err != nil {
		return nil, err
	}
	if m.StoredKey, err = hex.DecodeString(storedHex); err != nil {
		return nil, err
	}
	if m.ServerKey, err = hex.DecodeString(serverHex); err != nil {
		return nil, err
	}
	return m, nil
}

// CheckCredentialUsable re-validates a credential by id against the
// CURRENT stored state (revocation and expiry). Callers that also need the
// generation/policy binding compare the returned record against the live
// database row themselves (the binding is checked at connection and at
// every transaction boundary — postgresql-access.md). This is the bounded
// revocation check used by PG sessions; revocation/expiry of the row is
// the durable signal the exchange-derived keys alone cannot carry.
func (rt *Runtime) CheckCredentialUsable(ctx context.Context, credentialID string) (*CredentialRecord, error) {
	if _, err := rt.PrimaryDB.ExecContext(ctx, credentialsDDL); err != nil {
		return nil, err
	}
	var rec CredentialRecord
	var scopesJSON string
	var revokedAt sql.NullString
	err := rt.PrimaryDB.QueryRowContext(ctx,
		`SELECT id, database_id, purpose, scopes_json, expires_at, revoked_at, generation, policy_epoch
		 FROM _mb_credentials WHERE id = ?`, credentialID,
	).Scan(&rec.ID, &rec.DatabaseID, &rec.Purpose, &scopesJSON, &rec.ExpiresAt, &revokedAt, &rec.Generation, &rec.PolicyEpoch)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	_ = json.Unmarshal([]byte(scopesJSON), &rec.Scopes)
	if revokedAt.Valid {
		return nil, ErrCredentialRevoked
	}
	expires, perr := time.Parse(time.RFC3339Nano, rec.ExpiresAt)
	if perr != nil || time.Now().UTC().After(expires) {
		return nil, ErrCredentialRevoked
	}
	return &rec, nil
}

// scramKeys derives the RFC 5802 keys from a one-time secret:
// SaltedPassword = Hi(secret, salt, i); ClientKey = HMAC(SaltedPassword,
// "Client Key"); StoredKey = H(ClientKey); ServerKey = HMAC(SaltedPassword,
// "Server Key").
func scramKeys(secret string, salt []byte, iterations int) (storedKey, serverKey []byte) {
	salted := pbkdf2SHA256([]byte(secret), salt, iterations, sha256.Size)
	clientKey := hmacSHA256(salted, []byte("Client Key"))
	stored := sha256.Sum256(clientKey)
	server := hmacSHA256(salted, []byte("Server Key"))
	return stored[:], server
}

func hmacSHA256(key, data []byte) []byte {
	h := hmac.New(sha256.New, key)
	h.Write(data)
	return h.Sum(nil)
}

// pbkdf2SHA256 is PBKDF2 with HMAC-SHA256 (RFC 2898) — a small local
// implementation so the credential path stays on the standard library.
func pbkdf2SHA256(password, salt []byte, iterations, keyLen int) []byte {
	prf := hmac.New(sha256.New, password)
	hashLen := prf.Size()
	numBlocks := (keyLen + hashLen - 1) / hashLen
	var counter [4]byte
	dk := make([]byte, 0, numBlocks*hashLen)
	U := make([]byte, hashLen)
	for block := 1; block <= numBlocks; block++ {
		prf.Reset()
		prf.Write(salt)
		counter[0] = byte(block >> 24)
		counter[1] = byte(block >> 16)
		counter[2] = byte(block >> 8)
		counter[3] = byte(block)
		prf.Write(counter[:])
		dk = prf.Sum(dk)
		T := dk[len(dk)-hashLen:]
		copy(U, T)
		for n := 2; n <= iterations; n++ {
			prf.Reset()
			prf.Write(U)
			U = U[:0]
			U = prf.Sum(U)
			for i := range T {
				T[i] ^= U[i]
			}
		}
	}
	return dk[:keyLen]
}

// mustJSON marshals or panics-free fallback ("[]").
func mustJSON(v any) string {
	b, err := json.Marshal(v)
	if err != nil {
		return "[]"
	}
	return string(b)
}

// randomHexID generates a prefixed random identifier.
func randomHexID(prefix string) string {
	b := make([]byte, 8)
	_, _ = rand.Read(b)
	return prefix + hex.EncodeToString(b)
}
