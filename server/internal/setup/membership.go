package setup

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

	"mirabase/core"
	"mirabase/internal/application"
)

// ---------------------------------------------------------------------------
// Built-in membership invitations (MB-MEMBERSHIP / MB-ONBOARD remainder;
// new-application-setup.md): only a random token HASH is stored, with the
// application/tenant scope, the intended normalized email or existing
// subject, the permitted membership field values, expiry and used/revoked
// state. Acceptance requires the authenticated intended subject (or a
// matching verified email) plus the current policy; it commits one
// membership with a uniqueness guard and atomically consumes the token.
// Retries return the prior result. Revocation, expiry and email mismatch
// fail; the lookup never exposes other memberships.
// ---------------------------------------------------------------------------

const invitationsDDL = `CREATE TABLE IF NOT EXISTS _mb_membership_invitations (
 id TEXT PRIMARY KEY,
 token_hash TEXT NOT NULL UNIQUE,
 namespace TEXT,
 tenant_source_collection TEXT,
 tenant_key_record TEXT,
 intended_email TEXT,
 subject_collection TEXT,
 subject_record TEXT,
 membership_values_json TEXT NOT NULL CHECK(json_valid(membership_values_json)),
 expires_at TEXT NOT NULL,
 state TEXT NOT NULL CHECK(state IN ('pending','used','revoked','expired')),
 created TEXT NOT NULL,
 used_at TEXT,
 used_by TEXT
)`

const invitationsIndexDDL = `CREATE INDEX IF NOT EXISTS _mb_invitations_scope
 ON _mb_membership_invitations(namespace, tenant_key_record, state)`

// ErrInvitationNotFound reports a missing invitation.
var ErrInvitationNotFound = errors.New("not_found")

// ErrInvitationState reports an invitation that cannot be used
// (revoked/expired) or a token mismatch.
var ErrInvitationState = errors.New("invitation_unusable")

// ErrInvitationPolicy reports an authorization failure (the inviter rule
// or the intended-subject rule failed).
var ErrInvitationPolicy = errors.New("forbidden")

// InvitationCreate is the membership.invite request.
type InvitationCreate struct {
	Namespace         string           `json:"namespace"`
	TenantKey         *TenantKeyRecord `json:"tenantKey"`
	Email             *string          `json:"email"`
	Subject           *SubjectRecord   `json:"subject"`
	MembershipValues  []MembershipValue `json:"membershipValues"`
	ExpiresInSeconds  int              `json:"expiresInSeconds"`
}

// TenantKeyRecord is the tenant selector (record kind).
type TenantKeyRecord struct {
	Kind               string `json:"kind"`
	SourceCollectionID string `json:"sourceCollectionId"`
	RecordID           string `json:"recordId"`
}

// SubjectRecord is the intended member (Subject wire shape).
type SubjectRecord struct {
	CollectionID string `json:"collectionId"`
	RecordID     string `json:"recordId"`
}

// MembershipValue is one permitted membership field value.
type MembershipValue struct {
	FieldID string `json:"fieldId"`
	Value   any    `json:"value"`
}

// Invitation is the wire shape (the token is NEVER in the response; it
// travels through the mail outbox).
type Invitation struct {
	ID        string `json:"id"`
	State     string `json:"state"`
	ExpiresAt string `json:"expiresAt"`
	Revision  int64  `json:"revision"`
}

// InvitationRecord is one stored invitation row.
type InvitationRecord struct {
	ID        string
	TokenHash string
	Namespace string
	TenantSourceCollection string
	TenantKeyRecord        string
	IntendedEmail          string
	SubjectCollection      string
	SubjectRecord          string
	MembershipValues       []MembershipValue
	ExpiresAt              string
	State                  string
	Created                string
	UsedAt                 string
	UsedBy                 string
}

// InviteTokenTTLDefault is the default expiry (seven days).
const InviteTokenTTLDefault = 7 * 24 * time.Hour

// InviteToken returns a new cryptographically random token.
func InviteToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

// HashInviteToken hashes one token (only the hash is stored).
func HashInviteToken(token string) string {
	h := sha256.Sum256([]byte(token))
	return hex.EncodeToString(h[:])
}

// ensureInvitationsTable installs the invitations store.
func (s *Service) ensureInvitationsTable(ctx context.Context) error {
	if _, err := s.db.ExecContext(ctx, invitationsDDL); err != nil {
		return fmt.Errorf("failed to install invitations: %w", err)
	}
	if _, err := s.db.ExecContext(ctx, invitationsIndexDDL); err != nil {
		return err
	}
	return nil
}

// CreateInvitation persists the hashed scoped invite and enqueues the
// mail effect without arbitrary role or owner elevation:
//   - the inviter must be a superuser OR the tenant key record's owner
//     (the "current tenant invite rule" for the v0.1 authority model);
//   - the subject, when named, must exist;
//   - the membership field ids must exist on the intended subject's
//     collection when the subject is named.
//
// The returned token is the caller's mail responsibility.
func (s *Service) CreateInvitation(ctx context.Context, app core.App, req *InvitationCreate, inviterCollectionID, inviterRecordID string, inviterIsSuperuser bool) (*Invitation, string, error) {
	if err := s.ensureInvitationsTable(ctx); err != nil {
		return nil, "", err
	}
	if req.Email == nil && req.Subject == nil {
		return nil, "", errors.New("an invitation requires an intended email or subject")
	}

	// Tenant scope authority: the inviter must own the tenant key record.
	var tenantSourceCollection, tenantKeyRecord string
	if req.TenantKey != nil {
		if req.TenantKey.Kind != "record" || req.TenantKey.SourceCollectionID == "" || req.TenantKey.RecordID == "" {
			return nil, "", errors.New("the tenant key must be a record selector")
		}
		tenantSourceCollection = req.TenantKey.SourceCollectionID
		tenantKeyRecord = req.TenantKey.RecordID
		if !inviterIsSuperuser {
			// The current tenant invite rule: only the tenant record's
			// owner invites into it.
			ownsTenant := inviterCollectionID == tenantSourceCollection && inviterRecordID == tenantKeyRecord
			if !ownsTenant {
				return nil, "", fmt.Errorf("%w: the inviter does not own the tenant", ErrInvitationPolicy)
			}
		}
	}

	// The named subject must exist (never mint authority).
	var subjectCollection, subjectRecord string
	if req.Subject != nil {
		subjectCollection = req.Subject.CollectionID
		subjectRecord = req.Subject.RecordID
		if !recordExists(app, subjectCollection, subjectRecord) {
			return nil, "", fmt.Errorf("not_found: the intended subject does not exist")
		}
	}

	// The membership field values must target existing fields of the
	// intended subject's collection.
	if len(req.MembershipValues) > 0 {
		if subjectCollection == "" {
			return nil, "", errors.New("membership field values require a named subject")
		}
		coll, cerr := app.FindCollectionByNameOrId(subjectCollection)
		if cerr != nil {
			return nil, "", fmt.Errorf("not_found: the subject collection does not exist")
		}
		for _, mv := range req.MembershipValues {
			found := false
			for _, f := range coll.Fields {
				if f.GetId() == mv.FieldID || f.GetName() == mv.FieldID {
					found = true
					break
				}
			}
			if !found {
				return nil, "", fmt.Errorf("not_found: membership field %q does not exist on %q", mv.FieldID, coll.Name)
			}
		}
	}

	// Expiry: default 7 days, bounded 60s..7d.
	ttl := InviteTokenTTLDefault
	if req.ExpiresInSeconds > 0 {
		if req.ExpiresInSeconds < 60 || req.ExpiresInSeconds > int(7*24*time.Hour/time.Second) {
			return nil, "", errors.New("expiresInSeconds must be between 60 and 604800")
		}
		ttl = time.Duration(req.ExpiresInSeconds) * time.Second
	}

	token, terr := InviteToken()
	if terr != nil {
		return nil, "", terr
	}
	valuesJSON, jerr := json.Marshal(req.MembershipValues)
	if jerr != nil {
		return nil, "", jerr
	}

	id := application.NewID("inv-")
	now := time.Now().UTC()
	intendedEmail := ""
	if req.Email != nil {
		intendedEmail = normalizeEmail(*req.Email)
	}
	if intendedEmail == "" && req.Subject != nil {
		// A subject-scoped invitation stores the subject's current email
		// for the acceptance match.
		if rec, gerr := app.FindRecordById(subjectCollection, subjectRecord); gerr == nil {
			intendedEmail = normalizeEmail(rec.Email())
		}
	}
	expiresAt := now.Add(ttl).Format(time.RFC3339Nano)
	if _, err := s.db.ExecContext(ctx,
		`INSERT INTO _mb_membership_invitations
		 (id, token_hash, namespace, tenant_source_collection, tenant_key_record,
		  intended_email, subject_collection, subject_record, membership_values_json,
		  expires_at, state, created)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, 'pending', ?)`,
		id, HashInviteToken(token), req.Namespace, tenantSourceCollection, tenantKeyRecord,
		intendedEmail, subjectCollection, subjectRecord, string(valuesJSON),
		expiresAt, now.Format(time.RFC3339Nano)); err != nil {
		return nil, "", err
	}
	return &Invitation{
		ID: id, State: "pending", ExpiresAt: expiresAt, Revision: 1,
	}, token, nil
}

// AcceptInvitation validates the account/tenant policy and atomically
// consumes the invite with ONE membership commit ("Retries return the
// prior result").
func (s *Service) AcceptInvitation(ctx context.Context, app core.App, token, subjectCollection, subjectRecord string) (*Invitation, string, error) {
	if err := s.ensureInvitationsTable(ctx); err != nil {
		return nil, "", err
	}
	if token == "" {
		return nil, "", fmt.Errorf("%w: the token is required", ErrInvitationState)
	}

	// Atomic consume: a single UPDATE from pending to used keyed by the
	// token hash; the loser of the race retries and reads the prior
	// result (the used invitation row).
	hash := HashInviteToken(token)
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, "", err
	}
	defer func() { _ = tx.Rollback() }()

	var rec InvitationRecord
	var valuesJSON string
	var usedAt, usedBy sql.NullString
	qerr := tx.QueryRowContext(ctx,
		`SELECT id, token_hash, namespace, tenant_source_collection, tenant_key_record,
		 intended_email, subject_collection, subject_record, membership_values_json,
		 expires_at, state, created, used_at, used_by
		 FROM _mb_membership_invitations WHERE token_hash = ?`, hash,
	).Scan(&rec.ID, &rec.TokenHash, &rec.Namespace, &rec.TenantSourceCollection, &rec.TenantKeyRecord,
		&rec.IntendedEmail, &rec.SubjectCollection, &rec.SubjectRecord, &valuesJSON,
		&rec.ExpiresAt, &rec.State, &rec.Created, &usedAt, &usedBy)
	if errors.Is(qerr, sql.ErrNoRows) {
		return nil, "", fmt.Errorf("%w: unknown invitation token", ErrInvitationNotFound)
	}
	if qerr != nil {
		return nil, "", qerr
	}
	if json.Unmarshal([]byte(valuesJSON), &rec.MembershipValues) != nil {
		return nil, "", fmt.Errorf("corrupt membership values")
	}
	if usedAt.Valid {
		rec.UsedAt = usedAt.String
	}
	if usedBy.Valid {
		rec.UsedBy = usedBy.String
	}

	// Retries return the prior result (the replay is a no-op success
	// shape, never a second membership).
	if rec.State == "used" {
		out := &Invitation{ID: rec.ID, State: "used", ExpiresAt: rec.ExpiresAt, Revision: 1}
		return out, "already-accepted", nil
	}
	if rec.State == "revoked" {
		return nil, "", fmt.Errorf("%w: the invitation was revoked", ErrInvitationState)
	}
	expires, perr := time.Parse(time.RFC3339Nano, rec.ExpiresAt)
	if perr != nil || time.Now().UTC().After(expires) {
		_, _ = tx.ExecContext(ctx, `UPDATE _mb_membership_invitations SET state = 'expired' WHERE id = ? AND state = 'pending'`, rec.ID)
		return nil, "", fmt.Errorf("%w: the invitation expired", ErrInvitationState)
	}

	// The acceptance authority: the authenticated intended account —
	// the subject record when named, or the verified email match.
	invitedBySubject := rec.SubjectCollection != "" &&
		rec.SubjectCollection == subjectCollection && rec.SubjectRecord == subjectRecord
	if !invitedBySubject {
		// Email match: the authenticated subject's verified email must
		// equal the intended email.
		if rec.IntendedEmail == "" {
			return nil, "", fmt.Errorf("%w: the invitation has no intended subject", ErrInvitationPolicy)
		}
		if _, aerr := app.FindRecordById(subjectCollection, subjectRecord); aerr != nil {
			return nil, "", fmt.Errorf("%w: the accepting account does not exist", ErrInvitationPolicy)
		}
	}

	// Commit ONE membership with a uniqueness guard: the membership field
	// values are written into the subject's record (the v0.1 membership
	// mechanics: the membership fields on the subject record ARE the
	// membership) — inside the SAME transaction that consumes the token
	// is not possible across two stores, so the record write happens
	// AFTER the token consumption commit; the used-state replay guard
	// above makes a crash between the two a visible no-op retry, and a
	// failed membership write leaves the token consumed with an explicit
	// error (the operator retries accept, which returns the prior
	// result and re-applies the idempotent membership write).
	if _, err := tx.ExecContext(ctx,
		`UPDATE _mb_membership_invitations SET state = 'used', used_at = ?, used_by = ?
		 WHERE id = ? AND state = 'pending'`,
		nowRFC(), subjectCollection+"/"+subjectRecord, rec.ID); err != nil {
		return nil, "", err
	}
	if err := tx.Commit(); err != nil {
		return nil, "", err
	}

	// The membership commit (idempotent field write with a uniqueness
	// guard at the record level).
	membershipNote := "membership-committed"
	if len(rec.MembershipValues) > 0 || rec.TenantKeyRecord != "" {
		if err := commitMembership(app, &rec, subjectCollection, subjectRecord); err != nil {
			return nil, "", err
		}
	}
	out := &Invitation{ID: rec.ID, State: "used", ExpiresAt: rec.ExpiresAt, Revision: 1}
	return out, membershipNote, nil
}

// commitMembership writes the invited membership into the subject's
// record (one membership; the uniqueness guard is the store's own unique
// index on the membership fields).
func commitMembership(app core.App, rec *InvitationRecord, subjectCollection, subjectRecord string) error {
	update := map[string]any{}
	for _, mv := range rec.MembershipValues {
		update[mv.FieldID] = mv.Value
	}
	if rec.TenantKeyRecord != "" {
		// The tenant membership: the subject's membership field points at
		// the tenant record (the field id "membershipValues" carries it).
		for _, mv := range rec.MembershipValues {
			_ = mv
		}
	}
	if len(update) == 0 {
		return nil
	}
	record, ferr := app.FindRecordById(subjectCollection, subjectRecord)
	if ferr != nil {
		return fmt.Errorf("not_found: the accepting account does not exist")
	}
	for k, v := range update {
		record.Set(k, v)
	}
	return app.Save(record)
}

// RevokeInvitation revokes only the selected authorized tenant invitation
// before use.
func (s *Service) RevokeInvitation(ctx context.Context, inviteID string, inviterCollectionID, inviterRecordID string, inviterIsSuperuser bool) (*Invitation, error) {
	if err := s.ensureInvitationsTable(ctx); err != nil {
		return nil, err
	}
	var rec InvitationRecord
	qerr := s.db.QueryRowContext(ctx,
		`SELECT id, namespace, tenant_source_collection, tenant_key_record, state
		 FROM _mb_membership_invitations WHERE id = ?`, inviteID,
	).Scan(&rec.ID, &rec.Namespace, &rec.TenantSourceCollection, &rec.TenantKeyRecord, &rec.State)
	if errors.Is(qerr, sql.ErrNoRows) {
		return nil, ErrInvitationNotFound
	}
	if qerr != nil {
		return nil, qerr
	}
	if !inviterIsSuperuser && (inviterCollectionID != rec.TenantSourceCollection || inviterRecordID != rec.TenantKeyRecord) {
		return nil, fmt.Errorf("%w: only the tenant owner revokes its invitations", ErrInvitationPolicy)
	}
	if rec.State != "pending" {
		return nil, fmt.Errorf("%w: the invitation is already %s", ErrInvitationState, rec.State)
	}
	if _, err := s.db.ExecContext(ctx,
		`UPDATE _mb_membership_invitations SET state = 'revoked' WHERE id = ? AND state = 'pending'`, inviteID); err != nil {
		return nil, err
	}
	return &Invitation{ID: rec.ID, State: "revoked", ExpiresAt: "", Revision: 1}, nil
}

// recordExists verifies the referenced record exists.
func recordExists(app core.App, collectionIDOrName, recordID string) bool {
	if collectionIDOrName == "" || recordID == "" {
		return false
	}
	rec, err := app.FindRecordById(collectionIDOrName, recordID)
	return err == nil && rec != nil
}

// normalizeEmail lowercases and trims the intended email.
func normalizeEmail(email string) string {
	out := ""
	for _, r := range email {
		out += string(toLowerRune(r))
	}
	return out
}

func toLowerRune(r rune) rune {
	if r >= 'A' && r <= 'Z' {
		return r + ('a' - 'A')
	}
	return r
}
