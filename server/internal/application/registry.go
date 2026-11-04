// Package application owns the immutable application context, registry and
// routing, the operator boundary, artifacts and lifecycle (build-start.md
// source ownership table). v0.1 keeps one application with ID "default";
// additional applications are roadmap. The registry DDL is the normative
// design DDL from storage-format-reference.md, installed only when
// partition/lifecycle features are enabled ("Install application/target
// tables only at their first released consumer").
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
	"time"
)

// ApplicationID is the immutable v0.1 application context ID.
const ApplicationID = "default"

// Reserved identifiers.
const (
	// PrimaryDatabaseID is the stable reserved identifier of the resolver.
	PrimaryDatabaseID = "primary"

	// Layout values (namespace-scoped storage selection).
	LayoutShared = "shared"
	LayoutUser   = "user"
	LayoutTenant = "tenant"
	LayoutAgent  = "agent"
)

// Database kinds and states (storage-format-reference.md CHECK lists).
const (
	KindPrimary = "primary"
	KindManual  = "manual"
	KindTenant  = "tenant"
	KindUser    = "user"
	KindAgent   = "agent"

	StateRequested    = "requested"
	StateProvisioning = "provisioning"
	StateReady        = "ready"
	StateFailed       = "failed"
	StateMaintenance  = "maintenance"
	StateMigrating    = "migrating"
	StateRestoring    = "restoring"
	StateDeleting     = "deleting"
	StateTombstoned   = "tombstoned"
	StatePurged       = "purged"
	StateQuarantined  = "quarantined"

	ProfileWAL  = "wal"
	ProfileMVCC = "mvcc"
)

// Grant scopes. v0.1 grants accept record/file/database-inspect scopes
// only (v0.1-build.md); changes.read, record.sync and privileged SQL or
// native-replica credentials are not granted in v0.1.
const (
	ScopeRecordRead      = "record.read"
	ScopeRecordWrite     = "record.write"
	ScopeFilesRead       = "files.read"
	ScopeFilesWrite      = "files.write"
	ScopeDatabaseInspect = "database.inspect"
)

// v0.1GrantScopes is the accepted v0.1 scope set.
var v01GrantScopes = map[string]bool{
	ScopeRecordRead:      true,
	ScopeRecordWrite:     true,
	ScopeFilesRead:       true,
	ScopeFilesWrite:      true,
	ScopeDatabaseInspect: true,
}

// Registry DDL — the normative definitions from storage-format-reference.md,
// quoted column for column. "Permission enforcement, immutable template
// publication, owner uniqueness policy, path validation and request-hash
// conflict checks are service invariants in addition to DDL."
const (
	databasesDDL = `CREATE TABLE IF NOT EXISTS _mb_databases (
 id TEXT PRIMARY KEY,
 owner_collection_id TEXT,
 owner_record_id TEXT,
 kind TEXT NOT NULL CHECK(kind IN ('primary','manual','tenant','user','agent')),
 state TEXT NOT NULL CHECK(state IN ('requested','provisioning','ready','failed','maintenance','migrating','restoring','deleting','tombstoned','purged','quarantined')),
 generation INTEGER NOT NULL CHECK(generation > 0),
 profile TEXT NOT NULL CHECK(profile IN ('wal','mvcc')),
 template_id TEXT,
 schema_revision INTEGER NOT NULL CHECK(schema_revision > 0),
 policy_epoch INTEGER NOT NULL CHECK(policy_epoch > 0),
 display_name TEXT NOT NULL,
 created TEXT NOT NULL,
 updated TEXT NOT NULL,
 last_error_code TEXT,
 CHECK((owner_collection_id IS NULL) = (owner_record_id IS NULL))
)`

	databasesOwnerIndexDDL = `CREATE INDEX IF NOT EXISTS _mb_databases_owner
 ON _mb_databases(owner_collection_id,owner_record_id,kind)`
	databasesStateIndexDDL = `CREATE INDEX IF NOT EXISTS _mb_databases_state
 ON _mb_databases(state,id)`

	grantsDDL = `CREATE TABLE IF NOT EXISTS _mb_database_grants (
 id TEXT PRIMARY KEY,
 database_id TEXT NOT NULL REFERENCES _mb_databases(id),
 subject_collection_id TEXT NOT NULL,
 subject_record_id TEXT NOT NULL,
 scopes_json TEXT NOT NULL CHECK(json_valid(scopes_json)),
 expires_at TEXT,
 revoked_at TEXT,
 epoch INTEGER NOT NULL CHECK(epoch > 0),
 UNIQUE(database_id,subject_collection_id,subject_record_id)
)`

	agentsDDL = `CREATE TABLE IF NOT EXISTS _mb_agent_bindings (
 id TEXT PRIMARY KEY,
 auth_collection_id TEXT NOT NULL,
 auth_record_id TEXT NOT NULL,
 owner_collection_id TEXT NOT NULL,
 owner_record_id TEXT NOT NULL,
 registration_state TEXT NOT NULL CHECK(registration_state IN ('active','revoked')),
 created TEXT NOT NULL,
 UNIQUE(auth_collection_id,auth_record_id)
)`
	agentsOwnerIndexDDL = `CREATE INDEX IF NOT EXISTS _mb_agents_owner
 ON _mb_agent_bindings(owner_collection_id,owner_record_id)`
)

// Database is one `_mb_databases` row.
type Database struct {
	ID             string
	OwnerCollectionID *string
	OwnerRecordID     *string
	Kind           string
	State          string
	Generation     int64
	Profile        string
	TemplateID     *string
	SchemaRevision int64
	PolicyEpoch    int64
	DisplayName    string
	Created        string
	Updated        string
	LastErrorCode  *string
}

// Service owns the registry and lifecycle for the default application.
type Service struct {
	db *sql.DB
}

// NewService installs the registry tables when partition/lifecycle
// features are enabled and returns the service.
func NewService(db *sql.DB) (*Service, error) {
	stmts := []string{
		databasesDDL, databasesOwnerIndexDDL, databasesStateIndexDDL,
		grantsDDL, agentsDDL, agentsOwnerIndexDDL,
	}
	for _, ddl := range stmts {
		if _, err := db.Exec(ddl); err != nil {
			return nil, fmt.Errorf("failed to install the database registry: %w", err)
		}
	}
	return &Service{db: db}, nil
}

// ErrNotFound reports a missing registry row.
var ErrNotFound = errors.New("registry row not found")

// nowRFC3339 returns the UTC RFC3339 timestamp used in registry rows
// ("Timestamps are UTC RFC3339").
func nowRFC3339() string { return time.Now().UTC().Format(time.RFC3339Nano) }

// newDatabaseID generates a 32 lowercase hex database id ("Database IDs
// are 32 lowercase hexadecimal characters generated server-side. No
// display name, email, auth ID or request path becomes a filesystem
// component.").
func newDatabaseID() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

// scanDatabase scans one `_mb_databases` row.
func scanDatabase(row *sql.Row) (*Database, error) {
	var d Database
	var ownerCol, ownerRec, templateID, lastErr sql.NullString
	err := row.Scan(&d.ID, &ownerCol, &ownerRec, &d.Kind, &d.State, &d.Generation,
		&d.Profile, &templateID, &d.SchemaRevision, &d.PolicyEpoch, &d.DisplayName,
		&d.Created, &d.Updated, &lastErr)
	if err != nil {
		return nil, err
	}
	if ownerCol.Valid {
		d.OwnerCollectionID = &ownerCol.String
		d.OwnerRecordID = &ownerRec.String
	}
	if templateID.Valid {
		d.TemplateID = &templateID.String
	}
	if lastErr.Valid {
		d.LastErrorCode = &lastErr.String
	}
	return &d, nil
}

const databaseColumns = `id, owner_collection_id, owner_record_id, kind, state,
 generation, profile, template_id, schema_revision, policy_epoch,
 display_name, created, updated, last_error_code`

// GetDatabase loads one database row.
func (s *Service) GetDatabase(ctx context.Context, id string) (*Database, error) {
	row := s.db.QueryRowContext(ctx,
		`SELECT `+databaseColumns+` FROM _mb_databases WHERE id = ?`, id)
	d, err := scanDatabase(row)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	return d, err
}

// ListByOwner returns the databases bound to an owner (auth collection +
// record pair), newest first.
func (s *Service) ListByOwner(ctx context.Context, ownerCollectionID, ownerRecordID string) ([]*Database, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT `+databaseColumns+` FROM _mb_databases
		 WHERE owner_collection_id = ? AND owner_record_id = ?
		 ORDER BY created DESC`, ownerCollectionID, ownerRecordID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []*Database
	for rows.Next() {
		var d Database
		var ownerCol, ownerRec, templateID, lastErr sql.NullString
		if err := rows.Scan(&d.ID, &ownerCol, &ownerRec, &d.Kind, &d.State, &d.Generation,
			&d.Profile, &templateID, &d.SchemaRevision, &d.PolicyEpoch, &d.DisplayName,
			&d.Created, &d.Updated, &lastErr); err != nil {
			return nil, err
		}
		if ownerCol.Valid {
			d.OwnerCollectionID = &ownerCol.String
			d.OwnerRecordID = &ownerRec.String
		}
		if templateID.Valid {
			d.TemplateID = &templateID.String
		}
		if lastErr.Valid {
			d.LastErrorCode = &lastErr.String
		}
		out = append(out, &d)
	}
	return out, rows.Err()
}

// RequestDatabase registers a requested database row (the provisioning
// intent). Creation is idempotent per provisioning key (owner pair +
// template): the registry's owner index plus the caller's lookup guard
// converges concurrent first requests on one binding.
func (s *Service) RequestDatabase(ctx context.Context, kind, templateID, displayName string, ownerCollectionID, ownerRecordID *string) (*Database, error) {
	now := nowRFC3339()
	d := &Database{
		ID:                newDatabaseID(),
		OwnerCollectionID: ownerCollectionID,
		OwnerRecordID:     ownerRecordID,
		Kind:              kind,
		State:             StateRequested,
		Generation:        1,
		Profile:           ProfileWAL,
		SchemaRevision:    1,
		PolicyEpoch:       1,
		DisplayName:       displayName,
		Created:           now,
		Updated:           now,
	}
	if templateID != "" {
		t := templateID
		d.TemplateID = &t
	}
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO _mb_databases (id, owner_collection_id, owner_record_id, kind, state,
		 generation, profile, template_id, schema_revision, policy_epoch,
		 display_name, created, updated, last_error_code)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, NULL)`,
		d.ID, d.OwnerCollectionID, d.OwnerRecordID, d.Kind, d.State,
		d.Generation, d.Profile, d.TemplateID, d.SchemaRevision, d.PolicyEpoch,
		d.DisplayName, d.Created, d.Updated)
	if err != nil {
		return nil, err
	}
	return d, nil
}

// RequestDatabaseDeterministic registers the requested database with an id
// derived from the provisioning key (sha256 of kind+template+owner pair,
// 32 lowercase hex — still server-generated and never a filesystem
// component of a display name/email). Concurrent requests with the same
// binding key converge on ONE row via INSERT OR IGNORE, removing the
// adoption race entirely.
func (s *Service) RequestDatabaseDeterministic(ctx context.Context, kind, templateID, displayName string, ownerCollectionID, ownerRecordID *string) (*Database, error) {
	binding := kind + "|" + templateID + "|" + derefOrEmpty(ownerCollectionID) + "|" + derefOrEmpty(ownerRecordID)
	h := sha256.Sum256([]byte(binding))
	id := hex.EncodeToString(h[:])[:32]
	now := time.Now().UTC().Format(time.RFC3339Nano)
	d := &Database{
		ID:                id,
		OwnerCollectionID: ownerCollectionID,
		OwnerRecordID:     ownerRecordID,
		Kind:              kind,
		State:             StateRequested,
		Generation:        1,
		Profile:           ProfileWAL,
		SchemaRevision:    1,
		PolicyEpoch:       1,
		DisplayName:       displayName,
		Created:           now,
		Updated:           now,
	}
	if templateID != "" {
		t := templateID
		d.TemplateID = &t
	}
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO _mb_databases (id, owner_collection_id, owner_record_id, kind, state,
		 generation, profile, template_id, schema_revision, policy_epoch,
		 display_name, created, updated, last_error_code)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, NULL)
		 ON CONFLICT(id) DO NOTHING`,
		d.ID, d.OwnerCollectionID, d.OwnerRecordID, d.Kind, d.State,
		d.Generation, d.Profile, d.TemplateID, d.SchemaRevision, d.PolicyEpoch,
		d.DisplayName, d.Created, d.Updated)
	if err != nil {
		return nil, err
	}
	// Re-read: a concurrent request may have created the row first.
	fresh, err := s.GetDatabase(ctx, id)
	if err != nil {
		return nil, err
	}
	return fresh, nil
}

// TransitionDatabase moves a database to a new state with generation
// semantics: restoring/migrating take a new generation; tombstoned and
// purged keep the generation recorded at delete time.
func (s *Service) TransitionDatabase(ctx context.Context, id, state string, bumpGeneration bool) error {
	if bumpGeneration {
		_, err := s.db.ExecContext(ctx,
			`UPDATE _mb_databases SET state = ?, generation = generation + 1, updated = ?, last_error_code = NULL WHERE id = ?`,
			state, nowRFC3339(), id)
		return err
	}
	_, err := s.db.ExecContext(ctx,
		`UPDATE _mb_databases SET state = ?, updated = ?, last_error_code = NULL WHERE id = ?`,
		state, nowRFC3339(), id)
	return err
}

// FailDatabase records a failed transition with its error code.
func (s *Service) FailDatabase(ctx context.Context, id, errorCode string) error {
	_, err := s.db.ExecContext(ctx,
		`UPDATE _mb_databases SET state = ?, last_error_code = ?, updated = ? WHERE id = ?`,
		StateFailed, errorCode, nowRFC3339(), id)
	return err
}

// ---------------------------------------------------------------------------
// Grants
// ---------------------------------------------------------------------------

// Grant is one `_mb_database_grants` row.
type Grant struct {
	ID                 string
	DatabaseID         string
	SubjectCollectionID string
	SubjectRecordID    string
	Scopes             []string
	ExpiresAt          *string
	RevokedAt          *string
	Epoch              int64
}

// SetGrant upserts a grant for a subject; the scope set is validated
// against the v0.1 scope set and sorted ("sorted scopes" in the registry
// row contract).
func (s *Service) SetGrant(ctx context.Context, databaseID, subjectCollectionID, subjectRecordID string, scopes []string, expiresAt *string) (*Grant, error) {
	if len(scopes) == 0 {
		return nil, errors.New("a grant requires at least one scope")
	}
	for _, sc := range scopes {
		if !v01GrantScopes[sc] {
			return nil, fmt.Errorf("scope %q is not grantable in this release (record/file/database-inspect scopes only)", sc)
		}
	}
	sorted := append([]string(nil), scopes...)
	for i := 1; i < len(sorted); i++ {
		for j := i; j > 0 && sorted[j] < sorted[j-1]; j-- {
			sorted[j], sorted[j-1] = sorted[j-1], sorted[j]
		}
	}
	scopesJSON, err := json.Marshal(sorted)
	if err != nil {
		return nil, err
	}
	id := newDatabaseID()
	_, err = s.db.ExecContext(ctx,
		`INSERT INTO _mb_database_grants
			(id, database_id, subject_collection_id, subject_record_id, scopes_json, expires_at, revoked_at, epoch)
		 VALUES (?, ?, ?, ?, ?, ?, NULL, 1)
		 ON CONFLICT(database_id, subject_collection_id, subject_record_id) DO UPDATE SET
			scopes_json = excluded.scopes_json,
			expires_at = excluded.expires_at,
			revoked_at = NULL,
			epoch = epoch + 1`,
		id, databaseID, subjectCollectionID, subjectRecordID, string(scopesJSON), expiresAt)
	if err != nil {
		return nil, err
	}
	return s.GetGrant(ctx, databaseID, subjectCollectionID, subjectRecordID)
}

// GetGrant loads one grant row.
func (s *Service) GetGrant(ctx context.Context, databaseID, subjectCollectionID, subjectRecordID string) (*Grant, error) {
	row := s.db.QueryRowContext(ctx,
		`SELECT id, database_id, subject_collection_id, subject_record_id, scopes_json, expires_at, revoked_at, epoch
		 FROM _mb_database_grants WHERE database_id = ? AND subject_collection_id = ? AND subject_record_id = ?`,
		databaseID, subjectCollectionID, subjectRecordID)
	var g Grant
	var scopesJSON string
	var expiresAt, revokedAt sql.NullString
	err := row.Scan(&g.ID, &g.DatabaseID, &g.SubjectCollectionID, &g.SubjectRecordID, &scopesJSON, &expiresAt, &revokedAt, &g.Epoch)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	if err := json.Unmarshal([]byte(scopesJSON), &g.Scopes); err != nil {
		return nil, err
	}
	if expiresAt.Valid {
		g.ExpiresAt = &expiresAt.String
	}
	if revokedAt.Valid {
		g.RevokedAt = &revokedAt.String
	}
	return &g, nil
}

// HasScope reports whether a grant is live (not revoked, not expired) and
// carries the scope.
func (g *Grant) HasScope(scope string) bool {
	if g == nil || g.RevokedAt != nil {
		return false
	}
	if g.ExpiresAt != nil {
		if t, err := time.Parse(time.RFC3339, *g.ExpiresAt); err == nil && time.Now().UTC().After(t) {
			return false
		}
	}
	for _, s := range g.Scopes {
		if s == scope {
			return true
		}
	}
	return false
}

// RevokeGrant advances the policy epoch and stamps revoked_at ("Revocation
// increments policyEpoch; requests recheck the epoch at durable operation
// boundaries").
func (s *Service) RevokeGrant(ctx context.Context, databaseID, subjectCollectionID, subjectRecordID string) error {
	res, err := s.db.ExecContext(ctx,
		`UPDATE _mb_database_grants
		 SET revoked_at = ?, epoch = epoch + 1
		 WHERE database_id = ? AND subject_collection_id = ? AND subject_record_id = ?`,
		nowRFC3339(), databaseID, subjectCollectionID, subjectRecordID)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	return nil
}

// ---------------------------------------------------------------------------
// Agent bindings
// ---------------------------------------------------------------------------

// BindAgent registers an agent identity with a protected binding to its
// owner ("Agents are ordinary identities in an application auth collection
// with a protected binding to an owner"). The agent receives only its own
// scopes and context bindings and cannot modify the binding via normal
// record update.
func (s *Service) BindAgent(ctx context.Context, agentCollectionID, agentRecordID, ownerCollectionID, ownerRecordID string) (string, error) {
	id := newDatabaseID()
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO _mb_agent_bindings
			(id, auth_collection_id, auth_record_id, owner_collection_id, owner_record_id, registration_state, created)
		 VALUES (?, ?, ?, ?, ?, 'active', ?)
		 ON CONFLICT(auth_collection_id, auth_record_id) DO UPDATE SET
			owner_collection_id = excluded.owner_collection_id,
			owner_record_id = excluded.owner_record_id,
			registration_state = 'active'`,
		id, agentCollectionID, agentRecordID, ownerCollectionID, ownerRecordID, nowRFC3339())
	if err != nil {
		return "", err
	}
	return id, nil
}

// GetAgent loads an agent binding.
func (s *Service) GetAgent(ctx context.Context, agentCollectionID, agentRecordID string) (*AgentBinding, error) {
	row := s.db.QueryRowContext(ctx,
		`SELECT id, auth_collection_id, auth_record_id, owner_collection_id, owner_record_id, registration_state, created
		 FROM _mb_agent_bindings WHERE auth_collection_id = ? AND auth_record_id = ?`,
		agentCollectionID, agentRecordID)
	var a AgentBinding
	err := row.Scan(&a.ID, &a.AuthCollectionID, &a.AuthRecordID, &a.OwnerCollectionID, &a.OwnerRecordID, &a.RegistrationState, &a.Created)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &a, nil
}

// AgentBinding is one `_mb_agent_bindings` row.
type AgentBinding struct {
	ID                string
	AuthCollectionID  string
	AuthRecordID      string
	OwnerCollectionID string
	OwnerRecordID     string
	RegistrationState string
	Created           string
}

// RevokeAgent advances the binding to revoked ("Deleting an agent affects
// only its grants/bindings and declared agent database policy, not its
// owner's database").
func (s *Service) RevokeAgent(ctx context.Context, agentCollectionID, agentRecordID string) error {
	res, err := s.db.ExecContext(ctx,
		`UPDATE _mb_agent_bindings SET registration_state = 'revoked' WHERE auth_collection_id = ? AND auth_record_id = ?`,
		agentCollectionID, agentRecordID)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	return nil
}

// NewID generates a prefixed server-side identifier.
func NewID(prefix string) string {
	b := make([]byte, 8)
	_, _ = rand.Read(b)
	return prefix + hex.EncodeToString(b)
}

// ListDatabases returns one page of the registry (database.list: an
// indexed paginated registry read), id ascending.
func (s *Service) ListDatabases(ctx context.Context, page, perPage int) (items []*Database, total int64, err error) {
	if page < 1 {
		page = 1
	}
	if perPage < 1 || perPage > 1000 {
		perPage = 30
	}
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM _mb_databases`).Scan(&total); err != nil {
		return nil, 0, err
	}
	rows, err := s.db.QueryContext(ctx,
		`SELECT `+databaseColumns+` FROM _mb_databases ORDER BY id LIMIT ? OFFSET ?`,
		perPage, (page-1)*perPage)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	for rows.Next() {
		var d Database
		var ownerCol, ownerRec, templateID, lastErr sql.NullString
		if err := rows.Scan(&d.ID, &ownerCol, &ownerRec, &d.Kind, &d.State, &d.Generation,
			&d.Profile, &templateID, &d.SchemaRevision, &d.PolicyEpoch, &d.DisplayName,
			&d.Created, &d.Updated, &lastErr); err != nil {
			return nil, 0, err
		}
		if ownerCol.Valid {
			d.OwnerCollectionID = &ownerCol.String
			d.OwnerRecordID = &ownerRec.String
		}
		if templateID.Valid {
			d.TemplateID = &templateID.String
		}
		if lastErr.Valid {
			d.LastErrorCode = &lastErr.String
		}
		items = append(items, &d)
	}
	return items, total, rows.Err()
}

// ListGrants returns the live (non-revoked) grants of one database
// (grant.list).
func (s *Service) ListGrants(ctx context.Context, databaseID string) ([]*Grant, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, database_id, subject_collection_id, subject_record_id, scopes_json, expires_at, revoked_at, epoch
		 FROM _mb_database_grants WHERE database_id = ? AND revoked_at IS NULL ORDER BY id`,
		databaseID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []*Grant{}
	for rows.Next() {
		var g Grant
		var scopesJSON string
		var expiresAt, revokedAt sql.NullString
		if err := rows.Scan(&g.ID, &g.DatabaseID, &g.SubjectCollectionID, &g.SubjectRecordID, &scopesJSON, &expiresAt, &revokedAt, &g.Epoch); err != nil {
			return nil, err
		}
		if err := json.Unmarshal([]byte(scopesJSON), &g.Scopes); err != nil {
			return nil, err
		}
		if expiresAt.Valid {
			g.ExpiresAt = &expiresAt.String
		}
		if revokedAt.Valid {
			g.RevokedAt = &revokedAt.String
		}
		out = append(out, &g)
	}
	return out, rows.Err()
}

// ListAgents returns the active agent bindings (agent.list).
func (s *Service) ListAgents(ctx context.Context) ([]*AgentBinding, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, auth_collection_id, auth_record_id, owner_collection_id, owner_record_id, registration_state, created
		 FROM _mb_agent_bindings WHERE registration_state = 'active' ORDER BY created, id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []*AgentBinding{}
	for rows.Next() {
		var a AgentBinding
		if err := rows.Scan(&a.ID, &a.AuthCollectionID, &a.AuthRecordID, &a.OwnerCollectionID, &a.OwnerRecordID, &a.RegistrationState, &a.Created); err != nil {
			return nil, err
		}
		out = append(out, &a)
	}
	return out, rows.Err()
}

// SetProfile records the durable profile of one database row (wired by
// database.profile; the manifest is updated by the caller after the drain).
func (s *Service) SetProfile(ctx context.Context, id, profile string) error {
	if profile != ProfileWAL && profile != ProfileMVCC {
		return fmt.Errorf("unknown profile %q", profile)
	}
	res, err := s.db.ExecContext(ctx,
		`UPDATE _mb_databases SET profile = ?, updated = ? WHERE id = ?`,
		profile, nowRFC3339(), id)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	return nil
}

// SetSchemaRevision records the applied per-target schema revision.
func (s *Service) SetSchemaRevision(ctx context.Context, id string, revision int64) error {
	if revision < 1 {
		return fmt.Errorf("schema revision must be positive")
	}
	res, err := s.db.ExecContext(ctx,
		`UPDATE _mb_databases SET schema_revision = ?, updated = ? WHERE id = ?`,
		revision, nowRFC3339(), id)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	return nil
}

// SetOwner rebinds one database's owner (owner.transfer publication).
func (s *Service) SetOwner(ctx context.Context, id string, owner any) error {
	m, ok := owner.(map[string]any)
	if !ok {
		return fmt.Errorf("the owner subject is not a valid Subject object")
	}
	collectionID, _ := m["collectionId"].(string)
	recordID, _ := m["recordId"].(string)
	if collectionID == "" || recordID == "" {
		return fmt.Errorf("the owner subject requires collectionId and recordId")
	}
	res, err := s.db.ExecContext(ctx,
		`UPDATE _mb_databases SET owner_collection_id = ?, owner_record_id = ?, updated = ? WHERE id = ?`,
		collectionID, recordID, nowRFC3339(), id)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	return nil
}
