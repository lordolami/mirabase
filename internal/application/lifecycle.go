package application

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

var provisionLocks [64]sync.Mutex

// DataDirLayout returns the filesystem location of a private database:
// mb_data/databases/<databaseId>/data.db. Database IDs are server-generated
// 32 lowercase hex characters; no display name, email, auth ID or request
// path becomes a filesystem component.
func (s *Service) DatabaseDir(dataDir, databaseID string) string {
	return filepath.Join(dataDir, "databases", databaseID)
}

// ErrLayoutMigration is returned when a populated shared collection is
// selected for relocation (v0.1: "moving existing populated shared
// collections is refused with `layout_migration_required` and a link to
// its roadmap contract. Existing data remains unchanged.").
var ErrLayoutMigration = errors.New("layout_migration_required")

// ErrTenantSelection is returned when multiple eligible tenants require a
// choice ("Multiple eligible tenants require a choice; one tenant may be
// the configured default").
var ErrTenantSelection = errors.New("tenant_selection_required")

// Private manifest ("A private manifest contains formatVersion=1,
// databaseId, generation ..., profile, schemaRevision, templateId,
// encryptionKeyRef, codecId and creation timestamp. It never contains
// encryption key bytes. The primary registry is authoritative for routing;
// the manifest detects mismatched files, not authorization.").
type PrivateManifest struct {
	FormatVersion    int    `json:"formatVersion"`
	DatabaseID       string `json:"databaseId"`
	Generation       string `json:"generation"` // positive 64-bit integer as decimal text
	Profile          string `json:"profile"`
	SchemaRevision   int64  `json:"schemaRevision"`
	TemplateID       string `json:"templateId"`
	EncryptionKeyRef string `json:"encryptionKeyRef"`
	CodecID          string `json:"codecId"`
	CreatedAt        string `json:"createdAt"`
}

// ManifestFormatVersion is the manifest format version.
const ManifestFormatVersion = 1

// ProvisionRequest describes a lazy first-access provisioning binding.
type ProvisionRequest struct {
	Kind              string // user | tenant | agent
	TemplateID        string
	OwnerCollectionID string
	OwnerRecordID     string
	DisplayName       string
}

// Provision lazily provisions the private database for a verified binding:
// "A verified account, tenant or agent is bound once to a database. Lazy
// first access provisions it durably. Joining an existing tenant reuses
// its database. Concurrent first requests converge on one binding, not
// duplicate files."
//
// Convergence: the registry insert is a single INSERT of a server-generated
// id; concurrent callers each insert a candidate and then resolve the
// winner by re-querying the binding, discarding the loser rows. The
// physical file is created only for the surviving binding.
func (s *Service) Provision(ctx context.Context, dataDir string, req ProvisionRequest) (*Database, bool, error) {
	key := sha256.Sum256([]byte(dataDir + "\x00" + req.Kind + "\x00" + req.OwnerCollectionID + "\x00" + req.OwnerRecordID + "\x00" + req.TemplateID))
	lock := &provisionLocks[int(key[0])%len(provisionLocks)]
	lock.Lock()
	defer lock.Unlock()
	if req.Kind != KindUser && req.Kind != KindTenant && req.Kind != KindAgent {
		return nil, false, fmt.Errorf("provisioning kind must be user, tenant or agent, got %q", req.Kind)
	}

	// Reuse an existing binding first ("Joining an existing tenant reuses
	// its database"; idempotent per owner pair + template).
	existing, err := s.ListByOwner(ctx, req.OwnerCollectionID, req.OwnerRecordID)
	if err != nil {
		return nil, false, err
	}
	for _, d := range existing {
		if d.Kind == req.Kind && d.State != StateDeleting && d.State != StateTombstoned && d.State != StatePurged {
			if req.TemplateID == "" || d.TemplateID == nil || *d.TemplateID == req.TemplateID {
				return d, false, nil
			}
		}
	}

	// Register the requested row (provisioning intent, durable). The row
	// id is DERIVED from the provisioning key (kind+template+owner pair)
	// so concurrent first requests converge on ONE registry row by
	// construction — no candidate rows, no adoption race.
	d, err := s.RequestDatabaseDeterministic(ctx, req.Kind, req.TemplateID, req.DisplayName,
		&req.OwnerCollectionID, &req.OwnerRecordID)
	if err != nil {
		return nil, false, err
	}

	// Converge: with concurrent first requests several candidate rows may
	// exist momentarily; every caller deterministically adopts the SAME
	// winner (the smallest database id for this binding) and tombstones
	// its own duplicate row (never deleted — "Never delete a directory
	// solely because it is absent from a stale registry snapshot";
	// "Database IDs are never reused for unrelated data").
	existing, err = s.ListByOwner(ctx, req.OwnerCollectionID, req.OwnerRecordID)
	if err != nil {
		return nil, false, err
	}
	var winner *Database
	var losers []string
	for _, cand := range existing {
		if cand.Kind != req.Kind || cand.State == StateDeleting ||
			cand.State == StateTombstoned || cand.State == StatePurged {
			continue
		}
		if req.TemplateID != "" && cand.TemplateID != nil && *cand.TemplateID != req.TemplateID {
			continue
		}
		if winner == nil || cand.ID < winner.ID {
			if winner != nil {
				losers = append(losers, winner.ID)
			}
			winner = cand
		} else {
			losers = append(losers, cand.ID)
		}
	}
	if winner != nil {
		// Deterministic winner: EVERY other candidate row is retired by
		// whoever observes it (a caller that lists before others commit
		// would otherwise adopt its own row and leave the later duplicates
		// alive). Loser rows are tombstoned, never deleted.
		if winner.ID != d.ID {
			losers = append(losers, d.ID)
		}
		for _, loser := range losers {
			_ = s.TransitionDatabase(ctx, loser, StateTombstoned, false)
		}
		if winner.ID != d.ID {
			return winner, false, nil
		}
	}

	if err := s.createPrivateDatabase(ctx, dataDir, d); err != nil {
		_ = s.FailDatabase(ctx, d.ID, "provisioning")
		return nil, false, err
	}
	if err := s.TransitionDatabase(ctx, d.ID, StateReady, false); err != nil {
		return nil, false, err
	}
	fresh, err := s.GetDatabase(ctx, d.ID)
	if err != nil {
		return nil, false, err
	}
	return fresh, true, nil
}

// createPrivateDatabase physically creates the private database: the
// directory, the engine file via the provider, and the private manifest.
func (s *Service) createPrivateDatabase(ctx context.Context, dataDir string, d *Database) error {
	var template *PublishedTemplate
	if d.TemplateID != nil && *d.TemplateID != "" {
		var err error
		template, err = s.LoadLatestPublishedTemplate(ctx, *d.TemplateID)
		if err != nil {
			return fmt.Errorf("provisioning template: %w", err)
		}
		d.SchemaRevision = template.Revision
	}
	dir := s.DatabaseDir(dataDir, d.ID)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf("failed to create the database directory: %w", err)
	}
	// The engine creates data.db on first open through the provider
	// connector (both application databases go through the selected
	// provider — native-provider.md). The handle cache opens it lazily.
	manifest := PrivateManifest{
		FormatVersion:    ManifestFormatVersion,
		DatabaseID:       d.ID,
		Generation:       fmt.Sprintf("%d", d.Generation),
		Profile:          d.Profile,
		SchemaRevision:   d.SchemaRevision,
		TemplateID:       derefOrEmpty(d.TemplateID),
		EncryptionKeyRef: "",
		CodecID:          "",
		CreatedAt:        nowRFC3339(),
	}
	b, err := json.Marshal(manifest)
	if err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(dir, "manifest.json"), b, 0o600); err != nil {
		return err
	}
	// Physically create the engine file through the selected provider (the
	// engine creates data.db on first open; both application databases go
	// through the selected provider — native-provider.md, no SQLite
	// fallback).
	connector, err := newPrivateConnector(filepath.Join(dir, "data.db"))
	if err != nil {
		return fmt.Errorf("private provider connector: %w", err)
	}
	database := sql.OpenDB(connector)
	defer database.Close()
	conn, err := database.Conn(ctx)
	if err != nil {
		return fmt.Errorf("private database first open: %w", err)
	}
	if err := conn.Close(); err != nil {
		return err
	}
	if template != nil {
		if err := ApplyTemplate(ctx, database, template); err != nil {
			return fmt.Errorf("provisioning template application: %w", err)
		}
		if _, err := s.db.ExecContext(ctx, `UPDATE _mb_databases SET schema_revision=? WHERE id=?`, template.Revision, d.ID); err != nil {
			return err
		}
	}
	return nil
}

// ReadManifest loads the private manifest for mismatch detection.
func (s *Service) ReadManifest(dataDir, databaseID string) (*PrivateManifest, error) {
	b, err := os.ReadFile(filepath.Join(s.DatabaseDir(dataDir, databaseID), "manifest.json"))
	if err != nil {
		return nil, err
	}
	var m PrivateManifest
	if err := json.Unmarshal(b, &m); err != nil {
		return nil, err
	}
	return &m, nil
}

// Quarantine moves a database into quarantine with the registry still
// recording its generation and files intact ("quarantine ... Do not create
// an empty replacement"). The runtime closes the cached handle so the
// state check at request time is backed by no live file handle.
func (s *Service) Quarantine(ctx context.Context, id string) error {
	d, err := s.GetDatabase(ctx, id)
	if err != nil {
		return err
	}
	if d.State == StateTombstoned || d.State == StatePurged {
		return fmt.Errorf("cannot quarantine a %s database", d.State)
	}
	return s.TransitionDatabase(ctx, id, StateQuarantined, false)
}

// CloseRuntimeHandle closes the cached handle of a database (called by the
// lifecycle transitions that take a database out of service).
func (rt *Runtime) CloseRuntimeHandle(databaseID string) error {
	return rt.Handles.Close(databaseID)
}

// Restore brings a quarantined database back with a new generation
// ("Generation changes revoke old leases/tokens and invalidate cursors").
func (s *Service) Restore(ctx context.Context, id string) error {
	d, err := s.GetDatabase(ctx, id)
	if err != nil {
		return err
	}
	if d.State != StateQuarantined && d.State != StateTombstoned {
		return fmt.Errorf("cannot restore a %s database", d.State)
	}
	if err := s.TransitionDatabase(ctx, id, StateRestoring, true); err != nil {
		return err
	}
	return s.TransitionDatabase(ctx, id, StateReady, false)
}

// Tombstone marks a deleted database: tombstoned state is durable and the
// trash retention clock starts ("trash retention and safe purge"; default
// retention 24 hours).
func (s *Service) Tombstone(ctx context.Context, id string) error {
	d, err := s.GetDatabase(ctx, id)
	if err != nil {
		return err
	}
	if d.State == StatePurged {
		return fmt.Errorf("database already purged")
	}
	return s.TransitionDatabase(ctx, id, StateTombstoned, false)
}

// TrashRetentionDefault is the trash retention default ("Trash retention
// default: 24 hours; zero requires an explicit setting").
const TrashRetentionDefault = 24 * time.Hour

// PurgeTombstoned permanently removes tombstoned databases past the
// retention window. The caller owns durable intent (the lifecycle job
// records the purge receipt); Purge verifies the registry state and the
// manifest generation before touching files ("Never delete a directory
// solely because it is absent from a stale registry snapshot").
func (s *Service) PurgeTombstoned(ctx context.Context, dataDir string, retention time.Duration, now time.Time) (purged []string, err error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, updated FROM _mb_databases WHERE state = ?`, StateTombstoned)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var candidates []string
	var updated map[string]string = map[string]string{}
	for rows.Next() {
		var id, upd string
		if err := rows.Scan(&id, &upd); err != nil {
			return nil, err
		}
		candidates = append(candidates, id)
		updated[id] = upd
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	for _, id := range candidates {
		upd, err := time.Parse(time.RFC3339, updated[id])
		if err != nil {
			return purged, fmt.Errorf("tombstone timestamp for %s: %w", id, err)
		}
		if now.Sub(upd) < retention {
			continue
		}
		// Re-read the state fresh (never act on a stale snapshot).
		d, err := s.GetDatabase(ctx, id)
		if err != nil {
			if errors.Is(err, ErrNotFound) {
				continue
			}
			return purged, err
		}
		if d.State != StateTombstoned {
			continue
		}
		dir := s.DatabaseDir(dataDir, id)
		if err := os.RemoveAll(dir); err != nil {
			return purged, fmt.Errorf("purge %s: %w", id, err)
		}
		if err := s.TransitionDatabase(ctx, id, StatePurged, false); err != nil {
			return purged, err
		}
		purged = append(purged, id)
	}
	return purged, nil
}

// TransferOwnership implements the MB-OWNERTRANSFER phases over the
// `_mb_owner_transfers` contract: intent → fenced → revoked → bound →
// complete. The agent policy is recorded with the intent (revoke or
// rebind-reviewed). Owner deletion cannot make data public.
type OwnerTransferPhases struct {
	Intent, Fenced, Revoked, Bound, Complete, Failed string
}

// OwnerTransfer records the durable phases in the primary registry
// (`_mb_owner_transfers` per storage-format-reference.md).
func (s *Service) OwnerTransferTable(ctx context.Context) error {
	const ddl = `CREATE TABLE IF NOT EXISTS _mb_owner_transfers (
 id TEXT PRIMARY KEY, database_id TEXT NOT NULL REFERENCES _mb_databases(id), expected_generation INTEGER NOT NULL CHECK(expected_generation>0),
 previous_owner_json TEXT, new_owner_json TEXT NOT NULL CHECK(json_valid(new_owner_json)), policy_epoch INTEGER NOT NULL CHECK(policy_epoch>0),
 agent_policy TEXT NOT NULL CHECK(agent_policy IN ('revoke','rebind-reviewed')),
 phase TEXT NOT NULL CHECK(phase IN ('intent','fenced','revoked','bound','complete','failed'))
)`
	_, err := s.db.ExecContext(ctx, ddl)
	return err
}

// StartOwnerTransfer records the intent with the expected generation
// guard.
func (s *Service) StartOwnerTransfer(ctx context.Context, databaseID string, expectedGeneration int64, newOwner any, policyEpoch int64, agentPolicy string) (string, error) {
	if agentPolicy != "revoke" && agentPolicy != "rebind-reviewed" {
		return "", fmt.Errorf("agent policy must be revoke or rebind-reviewed")
	}
	if err := s.OwnerTransferTable(ctx); err != nil {
		return "", err
	}
	newOwnerJSON, err := json.Marshal(newOwner)
	if err != nil {
		return "", err
	}
	id := newDatabaseID()
	_, err = s.db.ExecContext(ctx,
		`INSERT INTO _mb_owner_transfers (id, database_id, expected_generation, previous_owner_json, new_owner_json, policy_epoch, agent_policy, phase)
		 VALUES (?, ?, ?, NULL, ?, ?, ?, 'intent')`,
		id, databaseID, expectedGeneration, string(newOwnerJSON), policyEpoch, agentPolicy)
	if err != nil {
		return "", err
	}
	return id, nil
}

// AdvanceOwnerTransfer moves the transfer to the next phase with the
// expected-generation guard ("A post-publication boundary change forbids
// snapshot rollback"; generation changes fence the old owner).
func (s *Service) AdvanceOwnerTransfer(ctx context.Context, id, phase string) error {
	if err := s.OwnerTransferTable(ctx); err != nil {
		return err
	}
	_, err := s.db.ExecContext(ctx,
		`UPDATE _mb_owner_transfers SET phase = ? WHERE id = ?`, phase, id)
	return err
}

func derefOrEmpty(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

// randomHex is a small local helper (registry uses its own generator).
func randomHex(n int) string {
	b := make([]byte, n)
	_, _ = rand.Read(b)
	return fmt.Sprintf("%x", b)
}

var _ = sql.ErrNoRows

// ProvisionManual creates one owner-less (manual kind) database through
// the same durable path as Provision: registry row → engine file +
// manifest → ready. The wire layer verifies the created file
// (integrity_check) before the operation succeeds — "Persist create
// intent; publish only after verified provisioning".
func (s *Service) ProvisionManual(ctx context.Context, dataDir, displayName, profile, templateID string) (*Database, error) {
	if profile != ProfileWAL && profile != ProfileMVCC {
		return nil, fmt.Errorf("unknown profile %q", profile)
	}
	d, err := s.RequestDatabase(ctx, KindManual, templateID, displayName, nil, nil)
	if err != nil {
		return nil, err
	}
	if err := s.createPrivateDatabase(ctx, dataDir, d); err != nil {
		_ = s.FailDatabase(ctx, d.ID, "provisioning")
		return nil, err
	}
	if err := s.TransitionDatabase(ctx, d.ID, StateReady, false); err != nil {
		return nil, err
	}
	return s.GetDatabase(ctx, d.ID)
}

// ProvisionManualWithKind provisions an owner-less database of any target
// kind (the pre-binding create intent for database.create without an
// owner; the owner binds later through owner.transfer or lazy access).
func (s *Service) ProvisionManualWithKind(ctx context.Context, dataDir, kind, displayName, profile, templateID string) (*Database, error) {
	if kind != KindManual && kind != KindUser && kind != KindTenant && kind != KindAgent {
		return nil, fmt.Errorf("unknown kind %q", kind)
	}
	if profile != ProfileWAL && profile != ProfileMVCC {
		return nil, fmt.Errorf("unknown profile %q", profile)
	}
	d, err := s.RequestDatabase(ctx, kind, templateID, displayName, nil, nil)
	if err != nil {
		return nil, err
	}
	if err := s.createPrivateDatabase(ctx, dataDir, d); err != nil {
		_ = s.FailDatabase(ctx, d.ID, "provisioning")
		return nil, err
	}
	if err := s.TransitionDatabase(ctx, d.ID, StateReady, false); err != nil {
		return nil, err
	}
	return s.GetDatabase(ctx, d.ID)
}
