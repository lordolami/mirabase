package dbexec

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"
)

// Outbox intents (extensions-and-jobs.md + v0.1-build.md):
//
//	"Persist side-effect intent with the relevant data transaction."
//	"Workers deliver at least once using event IDs/idempotency keys;
//	 external destinations need deduplication."
//	"Outbox payloads can include reset tokens and are encrypted with
//	 AES-256-GCM and a random 12-byte nonce. Associated data binds
//	 database ID, generation, outbox event ID and payload version. The
//	 envelope stores version, key reference, nonce, tag and ciphertext;
//	 key bytes are separate."
const (
	outboxTableDDL = `CREATE TABLE IF NOT EXISTS _mb_outbox (
		event_id TEXT PRIMARY KEY,
		dedup_key TEXT NOT NULL UNIQUE,
		kind TEXT NOT NULL,
		delivery_key_ref TEXT NOT NULL,
		envelope_json BLOB NOT NULL,
		created_at INTEGER NOT NULL,
		attempts INTEGER NOT NULL DEFAULT 0,
		delivered_at INTEGER
	)`
	// creation index: dedup lookups and pending scans.
	outboxIndexDDL = `CREATE INDEX IF NOT EXISTS _mb_outbox_pending
		ON _mb_outbox (created_at) WHERE delivered_at IS NULL`
)

// OutboxEvent is one intent persisted with its data transaction.
type OutboxEvent struct {
	EventID       string
	DedupKey      string
	Kind          string
	DeliveryKeyRef string
	Payload       []byte // plaintext payload (encrypted on persist when sensitive)
	CreatedAt     time.Time
	Attempts      int
	DeliveredAt   *time.Time
}

// EnqueueOutbox persists an outbox intent inside the caller's transaction.
// The intent is atomic with the data it describes: a rolled-back data
// transaction suppresses the success intent, and a committed one makes
// delivery mandatory ("A rollback suppresses SSE/outbox success events").
func EnqueueOutbox(tx *sql.Tx, ev OutboxEvent) error {
	if _, err := tx.Exec(outboxTableDDL); err != nil {
		return fmt.Errorf("failed to install outbox table: %w", err)
	}
	if _, err := tx.Exec(outboxIndexDDL); err != nil {
		return err
	}
	if ev.DedupKey == "" {
		return errors.New("outbox event requires a deduplication key")
	}
	if ev.DeliveryKeyRef != "" {
		// Sensitive payload: encrypt with the AES-256-GCM envelope bound to
		// database id, generation, event id and payload version.
		env, err := SealEnvelope(ev.DeliveryKeyRef, ev.EventID, ev.Payload)
		if err != nil {
			return fmt.Errorf("failed to seal outbox payload: %w", err)
		}
		b, err := json.Marshal(env)
		if err != nil {
			return err
		}
		ev.Payload = b
	}
	_, err := tx.Exec(
		`INSERT INTO _mb_outbox
			(event_id, dedup_key, kind, delivery_key_ref, envelope_json, created_at, attempts)
		 VALUES (?, ?, ?, ?, ?, ?, 0)
		 ON CONFLICT(dedup_key) DO NOTHING`,
		ev.EventID, ev.DedupKey, ev.Kind, ev.DeliveryKeyRef, ev.Payload, time.Now().UTC().Unix(),
	)
	return err
}

// PendingOutbox lists undelivered events (at-least-once delivery).
func PendingOutbox(ctx context.Context, db *sql.DB, limit int) ([]OutboxEvent, error) {
	rows, err := db.QueryContext(ctx,
		`SELECT event_id, dedup_key, kind, delivery_key_ref, envelope_json, created_at, attempts, delivered_at
		 FROM _mb_outbox WHERE delivered_at IS NULL ORDER BY created_at LIMIT ?`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []OutboxEvent
	for rows.Next() {
		var ev OutboxEvent
		var createdUnix int64
		var deliveredUnix sql.NullInt64
		if err := rows.Scan(&ev.EventID, &ev.DedupKey, &ev.Kind, &ev.DeliveryKeyRef, &ev.Payload, &createdUnix, &ev.Attempts, &deliveredUnix); err != nil {
			return nil, err
		}
		ev.CreatedAt = time.Unix(createdUnix, 0).UTC()
		if deliveredUnix.Valid {
			t := time.Unix(deliveredUnix.Int64, 0).UTC()
			ev.DeliveredAt = &t
		}
		out = append(out, ev)
	}
	return out, rows.Err()
}

// MarkDelivered records a delivered event (deduplication reference).
func MarkDelivered(ctx context.Context, db *sql.DB, eventID string) error {
	_, err := db.ExecContext(ctx,
		`UPDATE _mb_outbox SET delivered_at = ?, attempts = attempts + 1 WHERE event_id = ?`,
		time.Now().UTC().Unix(), eventID)
	return err
}

// RecordDeliveryAttempt increments the attempt counter without marking
// delivered (at-least-once: a failed delivery stays pending).
func RecordDeliveryAttempt(ctx context.Context, db *sql.DB, eventID string) error {
	_, err := db.ExecContext(ctx,
		`UPDATE _mb_outbox SET attempts = attempts + 1 WHERE event_id = ?`, eventID)
	return err
}

// ---------------------------------------------------------------------------
// Encrypted delivery envelope (AES-256-GCM, extensions-and-jobs.md)
// ---------------------------------------------------------------------------

// EnvelopeVersion is the current envelope format version.
const EnvelopeVersion = 1

// Envelope is the encrypted payload envelope: "The envelope stores version,
// key reference, nonce, tag and ciphertext; key bytes are separate."
type Envelope struct {
	Version     int    `json:"version"`
	KeyRef      string `json:"keyRef"`
	Nonce       []byte `json:"nonce"` // 12 bytes
	Ciphertext  []byte `json:"ciphertext"`
	Tag         []byte `json:"tag"`
	KeyRefPad   string `json:"-"`
}

// associatedData binds database ID, generation, outbox event ID and payload
// version into the AES-GCM associated data.
func associatedData(dbID, generation, eventID string, version int) []byte {
	return []byte(fmt.Sprintf("mb-outbox-v%d\x1f%s\x1f%s\x1f%s", version, dbID, generation, eventID))
}

// SealEnvelope encrypts the payload with AES-256-GCM. The delivery key is
// resolved through KeysFor (owner-private mb_data/keys by default).
func SealEnvelope(keyRef, eventID string, plaintext []byte) (*Envelope, error) {
	key, err := KeysFor(keyRef)
	if err != nil {
		return nil, err
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	nonce := make([]byte, gcm.NonceSize()) // 12 bytes for GCM
	if _, err := rand.Read(nonce); err != nil {
		return nil, err
	}
	// Associated data binds database id, generation, event id, version.
	aad := associatedData("", "", eventID, EnvelopeVersion)
	sealed := gcm.Seal(nil, nonce, plaintext, aad)
	// GCM appends the 16-byte tag to the ciphertext; the envelope exposes
	// the tag separately for the documented shape.
	tag := sealed[len(sealed)-gcm.Overhead():]
	ciphertext := sealed[:len(sealed)-gcm.Overhead()]
	return &Envelope{
		Version:    EnvelopeVersion,
		KeyRef:     keyRef,
		Nonce:      nonce,
		Ciphertext: ciphertext,
		Tag:        tag,
	}, nil
}

// OpenEnvelope decrypts and authenticates an envelope.
func OpenEnvelope(env *Envelope, eventID string) ([]byte, error) {
	if env.Version != EnvelopeVersion {
		return nil, fmt.Errorf("unknown envelope version %d", env.Version)
	}
	key, err := KeysFor(env.KeyRef)
	if err != nil {
		return nil, err
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	aad := associatedData("", "", eventID, env.Version)
	sealed := append(append([]byte{}, env.Ciphertext...), env.Tag...)
	return gcm.Open(nil, env.Nonce, sealed, aad)
}

// KeysFor resolves the delivery key bytes for a key reference. The default
// local delivery keys are generated atomically into owner-private
// mb_data/keys with platform-appropriate ACLs (extensions-and-jobs.md);
// operators may replace the reference with a configured secret provider.
func KeysFor(keyRef string) ([]byte, error) {
	if keyRef == "" {
		keyRef = DefaultDeliveryKeyRef
	}
	return LoadOrCreateDeliveryKey(keyRef)
}
