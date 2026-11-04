package dbexec

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"time"
)

// Idempotency receipts (transactions-and-concurrency.md):
//
//	"New mutation operations accept Idempotency-Key (16..128 printable
//	ASCII, no whitespace). Scope it to actor, database generation and
//	operation. Hash the canonical request, including staged file hashes.
//	Same key/same hash returns the committed result; same key/different
//	hash returns idempotency_conflict. Persist receipt in the target
//	transaction. Default receipt retention is 7 days and is advertised to
//	clients; retries after expiry have no exactly-once guarantee."
const (
	DefaultReceiptRetention = 7 * 24 * time.Hour

	receiptsTableDDL = `CREATE TABLE IF NOT EXISTS _mb_idempotency_receipts (
		scope TEXT NOT NULL,
		key TEXT NOT NULL,
		request_hash TEXT NOT NULL,
		outcome TEXT NOT NULL,
		result_json BLOB,
		created_at INTEGER NOT NULL,
		expires_at INTEGER NOT NULL,
		PRIMARY KEY (scope, key)
	)`
)

// ErrIdempotencyConflict is returned for same key / different request hash.
var ErrIdempotencyConflict = errors.New("idempotency_conflict")

// ErrIdempotencyKey is an invalid Idempotency-Key header.
var ErrIdempotencyKey = errors.New("invalid Idempotency-Key")

// ValidateIdempotencyKey enforces "16..128 printable ASCII, no whitespace".
func ValidateIdempotencyKey(key string) error {
	if n := len(key); n < 16 || n > 128 {
		return fmt.Errorf("%w: length must be 16..128", ErrIdempotencyKey)
	}
	for _, r := range key {
		if r < 0x21 || r > 0x7E {
			return fmt.Errorf("%w: only printable ASCII without whitespace", ErrIdempotencyKey)
		}
	}
	return nil
}

// CanonicalRequestHash hashes the canonical request payload (including any
// staged file hashes — the caller serializes them into the payload).
func CanonicalRequestHash(payload []byte) string {
	h := sha256.Sum256(payload)
	return hex.EncodeToString(h[:])
}

// Receipt is one durable idempotency receipt row.
type Receipt struct {
	Scope       string
	Key         string
	RequestHash string
	Outcome     string
	ResultJSON  []byte
	CreatedAt   time.Time
	ExpiresAt   time.Time
}

// ConsumeReceipt looks up a prior receipt inside the caller's transaction.
// Results:
//   - (receipt, nil): same key + same request hash — the caller returns the
//     committed result ("Same key/same hash returns the committed result").
//   - (nil, ErrIdempotencyConflict): same key + different hash.
//   - (nil, nil): no receipt — proceed and persist one in this transaction.
//   - expired receipts are treated as absent ("retries after expiry have
//     no exactly-once guarantee") and deleted.
func ConsumeReceipt(tx *sql.Tx, scope, key, requestHash string) (*Receipt, error) {
	if _, err := tx.Exec(receiptsTableDDL); err != nil {
		return nil, fmt.Errorf("failed to install receipts table: %w", err)
	}
	var r Receipt
	var createdUnix, expiresUnix int64
	err := tx.QueryRow(
		`SELECT request_hash, outcome, result_json, created_at, expires_at
		 FROM _mb_idempotency_receipts WHERE scope = ? AND key = ?`,
		scope, key,
	).Scan(&r.RequestHash, &r.Outcome, &r.ResultJSON, &createdUnix, &expiresUnix)
	switch {
	case errors.Is(err, sql.ErrNoRows):
		return nil, nil
	case err != nil:
		return nil, err
	}
	r.CreatedAt = time.Unix(createdUnix, 0).UTC()
	r.ExpiresAt = time.Unix(expiresUnix, 0).UTC()
	if time.Now().After(r.ExpiresAt) {
		if _, err := tx.Exec(
			`DELETE FROM _mb_idempotency_receipts WHERE scope = ? AND key = ?`, scope, key,
		); err != nil {
			return nil, err
		}
		return nil, nil
	}
	if r.RequestHash != requestHash {
		return nil, ErrIdempotencyConflict
	}
	r.Scope, r.Key = scope, key
	return &r, nil
}

// PersistReceipt writes the receipt inside the target transaction, so the
// receipt's existence is atomic with the commit it describes ("Externally
// visible record success requires committed data and applicable local
// change/receipt entries"; "Persist receipt in the target transaction").
func PersistReceipt(tx *sql.Tx, scope, key, requestHash, outcome string, result any, retention time.Duration) error {
	if _, err := tx.Exec(receiptsTableDDL); err != nil {
		return fmt.Errorf("failed to install receipts table: %w", err)
	}
	var resultJSON []byte
	if result != nil {
		b, err := json.Marshal(result)
		if err != nil {
			return fmt.Errorf("failed to encode receipt result: %w", err)
		}
		resultJSON = b
	}
	if retention <= 0 {
		retention = DefaultReceiptRetention
	}
	now := time.Now().UTC()
	_, err := tx.Exec(
		`INSERT INTO _mb_idempotency_receipts
			(scope, key, request_hash, outcome, result_json, created_at, expires_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?)
		 ON CONFLICT(scope, key) DO UPDATE SET
			request_hash = excluded.request_hash,
			outcome = excluded.outcome,
			result_json = excluded.result_json,
			created_at = excluded.created_at,
			expires_at = excluded.expires_at`,
		scope, key, requestHash, outcome, resultJSON, now.Unix(), now.Add(retention).Unix(),
	)
	return err
}

// ReconcileReceipt reads a committed receipt outside a transaction, for
// outcome_unknown reconciliation after reconnect ("Clients reconcile
// through idempotency receipt/status after reconnect").
func ReconcileReceipt(ctx context.Context, db *sql.DB, scope, key, requestHash string) (*Receipt, error) {
	var r Receipt
	var createdUnix, expiresUnix int64
	var resultJSON []byte
	err := db.QueryRowContext(ctx,
		`SELECT request_hash, outcome, result_json, created_at, expires_at
		 FROM _mb_idempotency_receipts WHERE scope = ? AND key = ?`,
		scope, key,
	).Scan(&r.RequestHash, &r.Outcome, &resultJSON, &createdUnix, &expiresUnix)
	switch {
	case errors.Is(err, sql.ErrNoRows):
		return nil, nil
	case err != nil && isMissingTableErr(err):
		// The receipts table is installed lazily inside target
		// transactions; a database that never committed a receipt has no
		// table yet — equivalent to no receipt.
		return nil, nil
	case err != nil:
		return nil, err
	}
	r.ResultJSON = resultJSON
	r.CreatedAt = time.Unix(createdUnix, 0).UTC()
	r.ExpiresAt = time.Unix(expiresUnix, 0).UTC()
	if r.RequestHash != requestHash {
		return nil, ErrIdempotencyConflict
	}
	r.Scope, r.Key = scope, key
	return &r, nil
}

// PurgeExpiredReceipts deletes receipts past their retention window.
func PurgeExpiredReceipts(ctx context.Context, db *sql.DB) (int64, error) {
	res, err := db.ExecContext(ctx,
		`DELETE FROM _mb_idempotency_receipts WHERE expires_at < ?`, time.Now().UTC().Unix())
	if err != nil {
		return 0, err
	}
	return res.RowsAffected()
}

// NewScope generates a random scope component when the caller does not
// carry one (used by tests and internal operations).
func NewScope(prefix string) string {
	b := make([]byte, 8)
	_, _ = rand.Read(b)
	return prefix + hex.EncodeToString(b)
}

// ErrReceiptNotFound reports a missing receipt row.
var ErrReceiptNotFound = errors.New("receipt not found")

// GetReceiptRow loads the newest receipt for one idempotency key
// (receipt.get: resolve an uncertain mutation outcome without re-executing).
// Receipts are target-local; the caller passes the owning database handle.
func GetReceiptRow(ctx context.Context, db *sql.DB, key string) (*Receipt, error) {
	// The receipts table is installed by the mutation paths; the read path
	// must never fail merely because no mutation has run yet.
	if _, err := db.ExecContext(ctx, receiptsTableDDL); err != nil {
		return nil, fmt.Errorf("failed to install the receipts table: %w", err)
	}
	var r Receipt
	var resultJSON []byte
	err := db.QueryRowContext(ctx,
		`SELECT scope, key, request_hash, outcome, result_json, created_at, expires_at
		 FROM _mb_idempotency_receipts WHERE key = ?
		 ORDER BY created_at DESC LIMIT 1`, key,
	).Scan(&r.Scope, &r.Key, &r.RequestHash, &r.Outcome, &resultJSON, &r.CreatedAt, &r.ExpiresAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrReceiptNotFound
	}
	if err != nil {
		return nil, err
	}
	r.ResultJSON = resultJSON
	return &r, nil
}
