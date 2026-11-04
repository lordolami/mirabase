package livequery

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"time"
)

// ---------------------------------------------------------------------------
// Cursor token: opaque authenticated data binding subscription, actor,
// context/generation, query hash and sequence (integration-contracts.md
// D-14). The HMAC secret is durable per target database (_mb_live_meta) so
// cursors issued before a process restart still authenticate against the
// persisted subscription/replay state.
// ---------------------------------------------------------------------------

// cursorClaims are the authenticated bindings of a live cursor token.
type cursorClaims struct {
	Sub        string `json:"s"`
	Database   string `json:"d"`
	Actor      string `json:"a"`
	Plan       string `json:"p"`
	Revision   int64  `json:"r"`
	Generation int64  `json:"g"`
	SchemaRev  int64  `json:"c"`
	Policy     int64  `json:"e"`
	Seq        int64  `json:"q"`
	QueryHash  string `json:"h"`
	// Snap marks a registration cursor: the first step delivers the full
	// snapshot the registration captured (the client has no local view
	// yet; the snapshot supersedes any missed delta range).
	Snap bool `json:"n,omitempty"`
}

// actorOf renders the canonical actor binding.
func actorOf(collectionID, recordID string) string {
	return collectionID + "/" + recordID
}

// encodeCursor signs the claims into "payload.mac" base64url.
func encodeCursor(secret []byte, c *cursorClaims) string {
	payload, err := json.Marshal(c)
	if err != nil {
		// The claims are plain scalars; marshaling cannot fail.
		panic(fmt.Sprintf("livequery: cursor claims: %v", err))
	}
	mac := hmac.New(sha256.New, secret)
	mac.Write(payload)
	return base64.RawURLEncoding.EncodeToString(payload) + "." +
		base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
}

// decodeCursor verifies the token against the secret and returns the
// claims. Any structural, encoding or MAC failure is ErrInvalidCursor.
func decodeCursor(secret []byte, token string) (*cursorClaims, error) {
	payload, mac, ok := cutCursor(token)
	if !ok {
		return nil, fmt.Errorf("%w: malformed token", ErrInvalidCursor)
	}
	mac2 := hmac.New(sha256.New, secret)
	mac2.Write(payload)
	if !hmac.Equal(mac, mac2.Sum(nil)) {
		return nil, fmt.Errorf("%w: cursor signature does not authenticate", ErrInvalidCursor)
	}
	var c cursorClaims
	if err := json.Unmarshal(payload, &c); err != nil {
		return nil, fmt.Errorf("%w: unreadable cursor payload", ErrInvalidCursor)
	}
	if c.Sub == "" || c.Actor == "" || c.Plan == "" || c.QueryHash == "" {
		return nil, fmt.Errorf("%w: cursor misses its bindings", ErrInvalidCursor)
	}
	return &c, nil
}

// cutCursor splits "payload.mac".
func cutCursor(token string) (payload, mac []byte, ok bool) {
	for i := 0; i < len(token); i++ {
		if token[i] == '.' {
			p, err1 := base64.RawURLEncoding.DecodeString(token[:i])
			m, err2 := base64.RawURLEncoding.DecodeString(token[i+1:])
			if err1 != nil || err2 != nil || len(m) != sha256.Size {
				return nil, nil, false
			}
			return p, m, true
		}
	}
	return nil, nil, false
}

// metaDDL stores the per-target cursor signing secret (never a user
// secret; it authenticates this database's live cursors only).
const metaDDL = `CREATE TABLE IF NOT EXISTS _mb_live_meta (
 key TEXT PRIMARY KEY,
 value TEXT NOT NULL
)`

// loadOrCreateCursorSecret loads (or creates once) the per-target cursor
// signing secret.
func loadOrCreateCursorSecret(ctx context.Context, handle *sql.DB) ([]byte, error) {
	if _, err := handle.ExecContext(ctx, metaDDL); err != nil {
		return nil, err
	}
	var value string
	err := handle.QueryRowContext(ctx,
		`SELECT value FROM _mb_live_meta WHERE key = 'cursor_secret'`).Scan(&value)
	if err == nil {
		b, derr := hex.DecodeString(value)
		if derr == nil && len(b) == 32 {
			return b, nil
		}
		// A corrupt secret row is re-keyed below (all live cursors
		// re-register on their next step: the MAC fails → reset).
	} else if err != sql.ErrNoRows {
		return nil, err
	}
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return nil, err
	}
	encoded := hex.EncodeToString(b)
	if _, err := handle.ExecContext(ctx,
		`INSERT INTO _mb_live_meta (key, value) VALUES ('cursor_secret', ?)
		 ON CONFLICT(key) DO UPDATE SET value = excluded.value`, encoded); err != nil {
		return nil, err
	}
	return b, nil
}

// cursorSecretCached caches the per-handle secret for the process
// lifetime (the durable row survives; re-reading per step is wasted I/O).
func (s *Service) cursorSecretCached(ctx context.Context, handle *sql.DB) ([]byte, error) {
	key := handleKey(handle)
	s.mu.Lock()
	cached, ok := s.secrets[key]
	s.mu.Unlock()
	if ok {
		return cached, nil
	}
	secret, err := loadOrCreateCursorSecret(ctx, handle)
	if err != nil {
		return nil, err
	}
	s.mu.Lock()
	s.secrets[key] = secret
	s.mu.Unlock()
	return secret, nil
}

// handleKey identifies an open handle for caching (the shared *sql.DB
// pointer identity).
func handleKey(handle *sql.DB) string {
	return fmt.Sprintf("%p", handle)
}

// nowRFC3339 is the UTC RFC3339 timestamp used in durable rows.
func nowRFC3339() string { return time.Now().UTC().Format(time.RFC3339Nano) }
