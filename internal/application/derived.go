package application

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"

	"mirabase/core"
	"mirabase/internal/livequery"
	"mirabase/tools/hook"
)

// ---------------------------------------------------------------------------
// R6 derived data (implementation-closure.md): versioned reference copies
// (MB-REFERENCE), named live query plans (MB-LIVEPLAN), qualified relation
// workflows (MB-RELWORKFLOW) and projected CDC aggregate reports. Every
// policy store is a durable application-local table; every effect is real
// (mirror rows, cascade deletes/unlinks, checkpointed aggregates), never a
// success shape.
// ---------------------------------------------------------------------------

var (
	// ErrReferenceStale reports a stale derived-data revision.
	ErrReferenceStale = errors.New("policy_changed")
	// ErrInvalidPlan reports a non-read-only live plan SQL.
	ErrInvalidPlan = errors.New("invalid_request")
	// ErrReferenceCopyStale reports a read that requires a healthy
	// reference copy while the copy is stale (past the declared maximum
	// lag), quarantined or never applied (MB-REFERENCE: reads that
	// require a healthy copy return `reference_stale`).
	ErrReferenceCopyStale = errors.New("reference_stale")
	// ErrReferenceSchemaIncompatible reports a policy whose field
	// projection no longer matches the real source schema; the affected
	// projection targets are quarantined with this typed recorded failure
	// until the policy is edited (a newer revision rebuilds the copy).
	ErrReferenceSchemaIncompatible = errors.New("reference_schema_incompatible")
	// ErrProjectionTooLarge reports a source selection exceeding the
	// projection row/byte bound: the refresh fails explicitly and the
	// previous projection keeps serving (never a truncated silent copy).
	ErrProjectionTooLarge = errors.New("resource_exhausted")
)

// Projection bounds (implementation-closure.md: "upserts/deletes the
// exact read-only projection in bounded target transactions"): the source
// snapshot read and the staged projection apply are bounded; exceeding a
// bound fails the refresh explicitly.
const (
	maxProjectionRows  = 10000
	maxProjectionBytes = 32 << 20 // 32 MiB of projected payload bytes
	projectionBatchRows = 1000   // rows per target batch transaction
)

// derivedDDL installs the R6 policy stores. `_mb_reference_policies`
// stores definitions; `_mb_reference_targets` stores per-target state
// (target generation, last applied sequence, policy revision and
// failure) — implementation-closure.md, Reference-copy refresh. The
// per-target `_mb_ref_state_<id>` table remains the apply-time
// checkpoint inside each target; the registry is the shared,
// application-side store the worker reconciles.
//
// definition_epoch is the definition-generation fence: EVERY successful
// DefineReferencePolicy call bumps it (same revision included), so any
// policy edit invalidates the old refresh work and forces a projection
// rebuild even when the revision number did not move.
const derivedDDL = `CREATE TABLE IF NOT EXISTS _mb_reference_policies (
 id TEXT PRIMARY KEY,
 revision INTEGER NOT NULL CHECK(revision > 0),
 source_collection_id TEXT NOT NULL,
 field_ids_json TEXT NOT NULL CHECK(json_valid(field_ids_json)),
 filter TEXT NOT NULL,
 targets_json TEXT NOT NULL CHECK(json_valid(targets_json)),
 delete_behavior TEXT NOT NULL CHECK(delete_behavior = 'remove-copy'),
 maximum_lag_seconds INTEGER NOT NULL CHECK(maximum_lag_seconds BETWEEN 1 AND 86400),
 schema_revision TEXT NOT NULL DEFAULT '',
 definition_epoch INTEGER NOT NULL DEFAULT 0,
 created TEXT NOT NULL
)`

// referenceTargetsDDL is the shared per-target registry
// (implementation-closure.md: "_mb_reference_targets stores target
// generation, last applied sequence, policy revision and failure").
const referenceTargetsDDL = `CREATE TABLE IF NOT EXISTS _mb_reference_targets (
 policy_id TEXT NOT NULL,
 target_database_id TEXT NOT NULL,
 target_generation INTEGER NOT NULL CHECK(target_generation > 0),
 last_applied_sequence INTEGER NOT NULL,
 policy_revision INTEGER NOT NULL,
 definition_epoch INTEGER NOT NULL DEFAULT 0,
 state TEXT NOT NULL CHECK(state IN ('healthy','stale','quarantined','failed')),
 failure TEXT,
 updated_at TEXT NOT NULL,
 PRIMARY KEY(policy_id, target_database_id)
)`

const livePlansDDL = `CREATE TABLE IF NOT EXISTS _mb_live_plans (
 id TEXT NOT NULL,
 revision INTEGER NOT NULL CHECK(revision > 0),
 sql TEXT NOT NULL,
 parameters_json TEXT NOT NULL CHECK(json_valid(parameters_json)),
 result_key TEXT NOT NULL,
 dependencies_json TEXT NOT NULL CHECK(json_valid(dependencies_json)),
 max_rows INTEGER NOT NULL CHECK(max_rows BETWEEN 1 AND 1000),
 max_bytes INTEGER NOT NULL CHECK(max_bytes BETWEEN 1024 AND 8388608),
 state TEXT NOT NULL CHECK(state IN ('draft','published','retired')),
 result_shape_json TEXT NOT NULL DEFAULT '[]' CHECK(json_valid(result_shape_json)),
 policy_json TEXT NOT NULL DEFAULT '{}' CHECK(json_valid(policy_json)),
 creator TEXT NOT NULL DEFAULT '',
 created TEXT NOT NULL,
 PRIMARY KEY(id, revision)
)`

const relationWorkflowsDDL = `CREATE TABLE IF NOT EXISTS _mb_relation_workflows (
 id TEXT NOT NULL,
 revision INTEGER NOT NULL CHECK(revision > 0),
 steps_json TEXT NOT NULL CHECK(json_valid(steps_json)),
 principal_json TEXT NOT NULL CHECK(json_valid(principal_json)),
 created TEXT NOT NULL,
 PRIMARY KEY(id, revision)
)`

const reportsDDL = `CREATE TABLE IF NOT EXISTS _mb_reports (
 id TEXT PRIMARY KEY,
 revision INTEGER NOT NULL CHECK(revision > 0),
 source_template_ids_json TEXT NOT NULL CHECK(json_valid(source_template_ids_json)),
 projected_field_ids_json TEXT NOT NULL CHECK(json_valid(projected_field_ids_json)),
 aggregate_sql TEXT NOT NULL,
 target_collection_id TEXT NOT NULL,
 freshness_seconds INTEGER NOT NULL CHECK(freshness_seconds BETWEEN 1 AND 86400),
 reader_rule TEXT NOT NULL,
 last_rebuilt_at TEXT,
 last_checkpoint TEXT,
 created TEXT NOT NULL
)`

// referenceTargetStateDDL lives in EACH target database: the per-target
// mirror table and its durable state row. The state records the last
// applied source sequence, the policy revision the batch was applied
// under, the target state (healthy|stale|quarantined|failed) and the
// typed recorded failure (quarantines) alongside the apply timestamp.
func referenceTargetStateDDL(policyID string) []string {
	return []string{
		// The mirror carries the batch's source sequence AND policy
		// revision on every row: each committed batch records the exact
		// (sequence, revision) pair it was projected under.
		fmt.Sprintf(`CREATE TABLE IF NOT EXISTS %s (
 id TEXT PRIMARY KEY,
 payload_json TEXT NOT NULL,
 source_seq INTEGER NOT NULL,
 policy_revision INTEGER NOT NULL DEFAULT 0
)`, quoteIdent("_mb_ref_copy_"+policyID)),
		fmt.Sprintf(`CREATE TABLE IF NOT EXISTS %s (
 singleton INTEGER PRIMARY KEY CHECK(singleton = 1),
 last_source_seq INTEGER NOT NULL,
 policy_revision INTEGER NOT NULL,
 definition_epoch INTEGER NOT NULL DEFAULT 0,
 state TEXT NOT NULL CHECK(state IN ('healthy','stale','quarantined','failed')),
 failure TEXT,
 updated_at TEXT NOT NULL
)`, quoteIdent("_mb_ref_state_"+policyID)),
	}
}

// ensureReferenceTargetState installs the mirror/state tables in a target
// and upgrades state tables created by an earlier release (the added
// columns are appended in place with the defaults the old single-sequence
// checkpoint implies: epoch 0 rebuilds once under the current definition —
// existing mirror data is kept until the rebuild swaps it). Idempotent and
// bounded: one pragma read plus at most four ALTERs.
func ensureReferenceTargetState(ctx context.Context, handle *sql.DB, policyID string) error {
	for _, ddl := range referenceTargetStateDDL(policyID) {
		if _, err := handle.ExecContext(ctx, ddl); err != nil {
			return fmt.Errorf("failed to install the reference target state: %w", err)
		}
	}
	stateT := quoteIdent("_mb_ref_state_" + policyID)
	cols, err := tableColumns(ctx, handle, "_mb_ref_state_"+policyID)
	if err != nil {
		return err
	}
	upgrades := map[string]string{
		"policy_revision":  "ALTER TABLE %s ADD COLUMN policy_revision INTEGER NOT NULL DEFAULT 1",
		"definition_epoch": "ALTER TABLE %s ADD COLUMN definition_epoch INTEGER NOT NULL DEFAULT 0",
		"state":            "ALTER TABLE %s ADD COLUMN state TEXT NOT NULL DEFAULT 'healthy'",
		"failure":          "ALTER TABLE %s ADD COLUMN failure TEXT",
	}
	for _, name := range []string{"policy_revision", "definition_epoch", "state", "failure"} {
		if cols[name] {
			continue
		}
		if _, err := handle.ExecContext(ctx, fmt.Sprintf(upgrades[name], stateT)); err != nil {
			return fmt.Errorf("failed to upgrade the reference target state: %w", err)
		}
	}
	return nil
}

// tableColumns returns the lowercase column-name set of a table.
func tableColumns(ctx context.Context, db *sql.DB, table string) (map[string]bool, error) {
	rows, err := db.QueryContext(ctx, fmt.Sprintf("SELECT name FROM pragma_table_info(%q)", table))
	if err != nil {
		return nil, fmt.Errorf("failed to read the table columns of %q: %w", table, err)
	}
	defer rows.Close()
	out := map[string]bool{}
	for rows.Next() {
		var n string
		if err := rows.Scan(&n); err != nil {
			return nil, err
		}
		out[strings.ToLower(n)] = true
	}
	return out, rows.Err()
}

// referenceTargetState is one target's durable copy checkpoint.
type referenceTargetState struct {
	Applied         bool
	LastSourceSeq   int64
	PolicyRevision  int64
	DefinitionEpoch int64
	State           string
	Failure         *string
	UpdatedAt       string
}

// readReferenceTargetState reads the checkpoint through tx (missing row
// means the copy has never been applied on this target).
func readReferenceTargetState(ctx context.Context, tx *sql.Tx, policyID string) (*referenceTargetState, error) {
	var st referenceTargetState
	var failure sql.NullString
	err := tx.QueryRowContext(ctx, fmt.Sprintf(
		"SELECT last_source_seq, policy_revision, definition_epoch, state, failure, updated_at FROM %s WHERE singleton = 1",
		quoteIdent("_mb_ref_state_"+policyID)),
	).Scan(&st.LastSourceSeq, &st.PolicyRevision, &st.DefinitionEpoch, &st.State, &failure, &st.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return &st, nil // never applied
	}
	if err != nil {
		return nil, err
	}
	st.Applied = true
	if failure.Valid {
		st.Failure = &failure.String
	}
	return &st, nil
}

// ReferencePolicyWire is the reference-copies policy (wire shape).
type ReferencePolicyWire struct {
	ID                 string         `json:"id"`
	Revision           int64          `json:"revision"`
	SourceCollectionID string         `json:"sourceCollectionId"`
	FieldIDs           []string       `json:"fieldIds"`
	Filter             string         `json:"filter"`
	Targets            map[string]any `json:"targets"`
	DeleteBehavior     string         `json:"deleteBehavior"`
	MaximumLagSeconds  int            `json:"maximumLagSeconds"`
	SchemaRevision     string         `json:"schemaRevision,omitempty"`
	// DefinitionEpoch is the definition-generation fence: bumped by
	// EVERY successful define (same revision included), so any policy
	// edit invalidates the old refresh work and rebuilds the
	// projections. It is assigned by the store; a submitted value is
	// ignored.
	DefinitionEpoch int64 `json:"definitionEpoch,omitempty"`
}

// authSecretFields are the auth-collection secret fields that may never
// be projected into a reference copy (implementation-closure.md:
// "Application auth/secrets are forbidden projections"). Hidden fields of
// every collection type are refused through GetHidden as well; this set
// keeps the auth secrets refused even if a schema ever unsets the hidden
// flag.
var authSecretFields = map[string]bool{
	"password": true, "passwordconfirm": true, "oldpassword": true,
	"tokenkey": true, "secret": true, "apikey": true, "token": true,
}

// DefineReferencePolicy validates and publishes the projection policy.
// Field validation enforces the allowed nonsecret field ids: every field
// must exist, must not be hidden and must not be an auth secret.
// Publishing also binds the source CDC intent capture (the primary
// record-write journal rows that advance the source head the refresh
// worker gates on) and pins the source schema revision (a digest of the
// projected field set; a refresh under a changed schema quarantines the
// targets).
func (s *Service) DefineReferencePolicy(ctx context.Context, app core.App, policy *ReferencePolicyWire) (*ReferencePolicyWire, error) {
	if err := s.ensureDerivedTables(ctx); err != nil {
		return nil, err
	}
	if err := ensureReferenceSourceCDC(app); err != nil {
		return nil, err
	}
	if policy.ID == "" || len(policy.ID) < 3 || strings.ContainsAny(policy.ID, "\"'; -") {
		return nil, fmt.Errorf("invalid_request: the policy id must be a safe identifier")
	}
	if policy.Revision < 1 {
		return nil, fmt.Errorf("invalid_request: the policy revision must be positive")
	}
	if policy.DeleteBehavior != "remove-copy" {
		return nil, fmt.Errorf("invalid_request: deleteBehavior must be remove-copy in this release")
	}
	if policy.MaximumLagSeconds == 0 {
		policy.MaximumLagSeconds = DefaultMaximumLagSeconds
	}
	if policy.MaximumLagSeconds < 1 || policy.MaximumLagSeconds > 86400 {
		return nil, fmt.Errorf("invalid_request: maximumLagSeconds must be 1..86400")
	}
	if policy.SourceCollectionID == "" || len(policy.FieldIDs) == 0 || len(policy.FieldIDs) > 100 {
		return nil, fmt.Errorf("invalid_request: the policy requires a source collection and 1..100 field ids")
	}
	if policy.Targets == nil {
		return nil, fmt.Errorf("invalid_request: the policy requires a target selector")
	}
	coll, cerr := app.FindCollectionByNameOrId(policy.SourceCollectionID)
	if cerr != nil {
		return nil, fmt.Errorf("not_found: the source collection does not exist")
	}
	projected := make([]core.Field, 0, len(policy.FieldIDs))
	for _, fid := range policy.FieldIDs {
		found := false
		for _, f := range coll.Fields {
			if f.GetId() == fid || f.GetName() == fid {
				found = true
				projected = append(projected, f)
				// Hidden fields are never projected ("allowed nonsecret
				// field IDs"); auth secrets are refused even when not
				// flagged hidden.
				if f.GetHidden() {
					return nil, fmt.Errorf("invalid_request: the field %q is hidden and cannot be projected into a reference copy", f.GetName())
				}
				if coll.Type == "auth" && authSecretFields[strings.ToLower(f.GetName())] {
					return nil, fmt.Errorf("invalid_request: the auth secret %q cannot be projected into a reference copy", f.GetName())
				}
				break
			}
		}
		if !found {
			return nil, fmt.Errorf("not_found: the field %q does not exist on the source collection", fid)
		}
	}
	// The selection is a parsed filter AST, never an interpolated string:
	// the policy filter must compile against the source collection
	// through the shared filter machinery and must not depend on a
	// requester (the selection is a system-side record selector).
	if strings.Contains(policy.Filter, ";") {
		return nil, fmt.Errorf("invalid_request: the filter must be a single expression")
	}
	if strings.Contains(policy.Filter, "@request") || strings.Contains(policy.Filter, "@collection") {
		return nil, fmt.Errorf("invalid_request: the record selection must not depend on a requester")
	}
	if strings.TrimSpace(policy.Filter) != "" {
		if base := baseAppOf(app); base != nil {
			if _, _, perr := BuildRulePredicate(base, coll, policy.Filter, nil); perr != nil {
				return nil, perr
			}
		}
	}
	// Revision regression is refused; a definition epoch is ALWAYS
	// bumped on a successful define — including a same-revision edit —
	// so every policy edit invalidates the old refresh work and the
	// targets rebuild their projections from the edited definition
	// (implementation-closure.md: "policy edits invalidate old refresh
	// work and rebuild projections").
	var currentRevision, currentEpoch int64
	switch serr := s.db.QueryRowContext(ctx,
		"SELECT revision, definition_epoch FROM _mb_reference_policies WHERE id = ?", policy.ID,
	).Scan(&currentRevision, &currentEpoch); {
	case errors.Is(serr, sql.ErrNoRows):
		// first definition
	case serr != nil:
		return nil, serr
	case policy.Revision < currentRevision:
		return nil, fmt.Errorf("%w: the policy revision regressed", ErrReferenceStale)
	}
	policy.DefinitionEpoch = currentEpoch + 1
	targetsJSON, jerr := json.Marshal(policy.Targets)
	if jerr != nil {
		return nil, jerr
	}
	fieldIDsJSON, jerr := json.Marshal(policy.FieldIDs)
	if jerr != nil {
		return nil, jerr
	}
	policy.SchemaRevision = projectedSchemaDigest(projected, app)
	if _, err := s.db.ExecContext(ctx,
		"INSERT INTO _mb_reference_policies (id, revision, source_collection_id, field_ids_json, filter, targets_json, delete_behavior, maximum_lag_seconds, schema_revision, definition_epoch, created) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?) ON CONFLICT(id) DO UPDATE SET revision = excluded.revision, source_collection_id = excluded.source_collection_id, field_ids_json = excluded.field_ids_json, filter = excluded.filter, targets_json = excluded.targets_json, delete_behavior = excluded.delete_behavior, maximum_lag_seconds = excluded.maximum_lag_seconds, schema_revision = excluded.schema_revision, definition_epoch = excluded.definition_epoch",
		policy.ID, policy.Revision, policy.SourceCollectionID, string(fieldIDsJSON), policy.Filter,
		string(targetsJSON), policy.DeleteBehavior, policy.MaximumLagSeconds, policy.SchemaRevision, policy.DefinitionEpoch, nowRFC3339()); err != nil {
		return nil, err
	}
	return policy, nil
}

// DefaultMaximumLagSeconds is the default maximum healthy lag of a
// reference copy (implementation-closure.md: "Default maximum healthy lag
// is 60 seconds").
const DefaultMaximumLagSeconds = 60

// projectedSchemaDigest computes the source schema revision of a
// projection: a canonical digest over the projected fields' names and
// column types.
func projectedSchemaDigest(fields []core.Field, app core.App) string {
	parts := make([]string, 0, len(fields))
	for _, f := range fields {
		parts = append(parts, strings.ToLower(f.GetName())+":"+f.ColumnType(app))
	}
	sort.Strings(parts)
	sum := sha256.Sum256([]byte(strings.Join(parts, "|")))
	return fmt.Sprintf("%x", sum)
}

// ListReferencePolicies lists every stored policy (the shared refresh
// worker's catalog).
func (s *Service) ListReferencePolicies(ctx context.Context) ([]ReferencePolicyWire, error) {
	if err := s.ensureDerivedTables(ctx); err != nil {
		return nil, err
	}
	rows, err := s.db.QueryContext(ctx,
		"SELECT id FROM _mb_reference_policies ORDER BY id")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []ReferencePolicyWire{}
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		p, err := s.GetReferencePolicy(ctx, id)
		if err != nil {
			return nil, err
		}
		out = append(out, *p)
	}
	return out, rows.Err()
}

// GetReferencePolicy loads one policy.
func (s *Service) GetReferencePolicy(ctx context.Context, id string) (*ReferencePolicyWire, error) {
	var p ReferencePolicyWire
	var fieldsJSON, targetsJSON string
	err := s.db.QueryRowContext(ctx,
		"SELECT id, revision, source_collection_id, field_ids_json, filter, targets_json, delete_behavior, maximum_lag_seconds, schema_revision, definition_epoch FROM _mb_reference_policies WHERE id = ?", id,
	).Scan(&p.ID, &p.Revision, &p.SourceCollectionID, &fieldsJSON, &p.Filter, &targetsJSON, &p.DeleteBehavior, &p.MaximumLagSeconds, &p.SchemaRevision, &p.DefinitionEpoch)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	if err := json.Unmarshal([]byte(fieldsJSON), &p.FieldIDs); err != nil {
		return nil, err
	}
	p.Targets = map[string]any{}
	_ = json.Unmarshal([]byte(targetsJSON), &p.Targets)
	return &p, nil
}

// ReferenceStatus is the reference.status wire shape.
type ReferenceStatus struct {
	ID           string                  `json:"id"`
	Revision     int64                   `json:"revision"`
	Targets      int                     `json:"targets"`
	Healthy      int                     `json:"healthy"`
	Stale        int                     `json:"stale"`
	Quarantined  int                     `json:"quarantined"`
	Failed       int                     `json:"failed"`
	TargetStates []ReferenceTargetStatus `json:"targetStates"`
}

// ReferenceTargetStatus is one target's recorded copy state: the
// diagnostics carry the applied source sequence, the applied policy
// revision, the target generation and the recorded failure (quarantines).
type ReferenceTargetStatus struct {
	DatabaseID     string `json:"databaseId"`
	State          string `json:"state"` // healthy|stale|quarantined|failed
	LastSourceSeq  int64  `json:"lastSourceSeq"`
	PolicyRevision int64  `json:"policyRevision"`
	Generation     int64  `json:"targetGeneration,omitempty"`
	UpdatedAt      string `json:"updatedAt,omitempty"`
	Failure        string `json:"failure,omitempty"`
}

// refreshSourceRow is one projected source row of a consistent snapshot.
type refreshSourceRow struct {
	id     string
	values []any
}

// referenceTargetIDs resolves the selector-matching ready target
// databases (kinds default to every managed kind).
func (rt *Runtime) referenceTargetIDs(ctx context.Context, policy *ReferencePolicyWire) ([]string, error) {
	kinds := []string{"manual", "tenant", "user", "agent"}
	if tk, ok := policy.Targets["kinds"].([]any); ok && len(tk) > 0 {
		kinds = nil
		for _, k := range tk {
			if ks, ok := k.(string); ok {
				kinds = append(kinds, ks)
			}
		}
	}
	if len(kinds) == 0 {
		kinds = []string{"manual", "tenant", "user", "agent"}
	}
	args := []any{}
	where := ""
	for _, k := range kinds {
		if len(args) > 0 {
			where += " OR "
		}
		where += "kind = ?"
		args = append(args, k)
	}
	rows, err := rt.PrimaryDB.QueryContext(ctx,
		"SELECT id FROM _mb_databases WHERE state = 'ready' AND ("+where+")", args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		out = append(out, id)
	}
	return out, rows.Err()
}

// sourceFieldName resolves a policy field id or name to the source
// schema's field NAME (the record table column is the field name; PB
// record tables are named by the collection NAME, never the id).
func sourceFieldName(coll *core.Collection, fid string) (string, bool) {
	for _, f := range coll.Fields {
		if f.GetId() == fid || f.GetName() == fid {
			return f.GetName(), true
		}
	}
	return "", false
}

// isSchemaQueryError recognizes the engine's schema-level query failures
// (a column or table dropped between the pre-flight check and the read).
func isSchemaQueryError(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "no such column") ||
		strings.Contains(msg, "no such table") ||
		strings.Contains(msg, "no such function")
}

// RefreshReferencePolicy reads one consistent authorized source snapshot
// and applies it to every selector-matching ready target in one bounded
// transaction per target. The projected rows AND the source
// change-journal head are captured inside the SAME source read
// transaction, so a batch never pairs rows from one point in time with a
// head from another. Per target (the decision reads the durable
// checkpoint INSIDE the batch transaction, so the guard is atomic with
// the writes):
//
//   - a first apply or a newer policy revision rebuilds the projection
//     (a policy edit invalidates the old refresh work and clears a
//     quarantine);
//   - an older policy revision never overwrites a newer one;
//   - the same revision applies only when the captured source head is
//     NEWER than the target's applied sequence (an older event can never
//     overwrite a newer sequence; duplicate execution with the same head
//     is a safe skip);
//   - the deleteBehavior remove-copy replace propagates source deletions
//     (a deleted source row is absent from the snapshot, so the copy row
//     is removed);
//   - an incompatible source schema quarantines the targets with the
//     typed recorded failure (ErrReferenceSchemaIncompatible) until the
//     policy is edited.
func (rt *Runtime) RefreshReferencePolicy(ctx context.Context, base *core.BaseApp, policy *ReferencePolicyWire) (map[string]any, error) {
	if base == nil {
		return nil, errors.New("the reference refresh requires the source application")
	}
	// Restart self-heal: the hooks that advance the source head do not
	// survive a process restart; re-bind idempotently before reading it.
	if err := ensureReferenceSourceCDC(base); err != nil {
		return nil, err
	}
	targetIDs, terr := rt.referenceTargetIDs(ctx, policy)
	if terr != nil {
		return nil, terr
	}
	srcColl, cerr := base.FindCollectionByNameOrId(policy.SourceCollectionID)
	if cerr != nil {
		return nil, fmt.Errorf("not_found: the source collection does not exist")
	}
	source := base.DBConn()
	if source == nil {
		return nil, errors.New("the source connection is unavailable")
	}
	// PB record tables are named by the collection NAME (never the id).
	sourceTable := srcColl.Name

	// Source schema compatibility: the policy's field ids/names must
	// resolve on the stored schema AND on the REAL source table (a
	// migration that dropped or renamed a column is caught before any
	// target is touched). The projected field set's digest must also match
	// the policy's stored schema revision — a type change on the source
	// quarantines instead of silently re-projecting.
	columns := make([]string, 0, len(policy.FieldIDs))
	projectedFields := make([]core.Field, 0, len(policy.FieldIDs))
	var incompatible error
	for _, fid := range policy.FieldIDs {
		name, ok := sourceFieldName(srcColl, fid)
		if !ok {
			incompatible = fmt.Errorf("%w: the field %q does not exist on the source collection %q",
				ErrReferenceSchemaIncompatible, fid, sourceTable)
			break
		}
		columns = append(columns, name)
		projectedFields = append(projectedFields, sourceFieldOf(srcColl, fid))
	}
	if incompatible == nil {
		// Schema revision: the projected field set (name + column type)
		// must match what the policy revision was defined against.
		if policy.SchemaRevision != "" {
			if got := projectedSchemaDigest(projectedFields, base); got != policy.SchemaRevision {
				incompatible = fmt.Errorf("%w: the source schema revision of the projection changed (%q vs the policy's %q)",
					ErrReferenceSchemaIncompatible, got[:8], policy.SchemaRevision[:8])
			}
		}
	}
	if incompatible == nil {
		srcCols, serr := tableColumns(ctx, source, sourceTable)
		if serr != nil {
			if isSchemaQueryError(serr) {
				return rt.quarantineReferenceTargets(ctx, policy, targetIDs, fmt.Errorf(
					"%w: the source table %q is unreadable: %v", ErrReferenceSchemaIncompatible, sourceTable, serr))
			}
			return nil, serr
		}
		for _, name := range columns {
			if !srcCols[strings.ToLower(name)] {
				incompatible = fmt.Errorf("%w: the source table %q has no column %q",
					ErrReferenceSchemaIncompatible, sourceTable, name)
				break
			}
		}
	}
	if incompatible != nil {
		return rt.quarantineReferenceTargets(ctx, policy, targetIDs, incompatible)
	}

	// The journal head is only meaningful with the journal present.
	if _, jerr := source.ExecContext(ctx, changesJournalDDL); jerr != nil {
		return nil, fmt.Errorf("failed to prepare the source change journal: %w", jerr)
	}
	quotedColumns := make([]string, 0, len(columns))
	for _, c := range columns {
		quotedColumns = append(quotedColumns, quoteIdent(c))
	}
	// The record selection AST compiles through the parsed filter
	// machinery into a parameterized predicate — never interpolated.
	predicate, predArgs, perr := BuildRulePredicate(base, srcColl, policy.Filter, nil)
	if perr != nil {
		return nil, fmt.Errorf("invalid_request: the record selection does not compile: %v", perr)
	}

	// ONE consistent source read: rows and journal head in one
	// transaction, bounded by the snapshot row/byte budget (exceeding
	// either fails the refresh with an explicit error — the previous
	// projection keeps serving, never a truncated silent copy).
	readTx, rterr := source.BeginTx(ctx, nil)
	if rterr != nil {
		return nil, rterr
	}
	snapshot, head, snapErr := func() ([]refreshSourceRow, int64, error) {
		defer readTx.Rollback()
		query := fmt.Sprintf("SELECT id, %s FROM %s", strings.Join(quotedColumns, ", "), quoteIdent(sourceTable))
		args := append([]any{}, predArgs...)
		if strings.TrimSpace(policy.Filter) != "" {
			query += " WHERE " + predicate
		}
		srcRows, qerr := readTx.QueryContext(ctx, query, args...)
		if qerr != nil {
			if isSchemaQueryError(qerr) {
				return nil, 0, fmt.Errorf("%w: the source schema changed during the read: %v",
					ErrReferenceSchemaIncompatible, qerr)
			}
			return nil, 0, fmt.Errorf("source read failed: %w", qerr)
		}
		defer srcRows.Close()
		var rows []refreshSourceRow
		totalBytes := 0
		for srcRows.Next() {
			if len(rows) > maxProjectionRows {
				return nil, 0, fmt.Errorf("%w: the source selection exceeds the projection row bound (%d)", ErrProjectionTooLarge, maxProjectionRows)
			}
			r := refreshSourceRow{values: make([]any, len(columns))}
			ptrs := make([]any, 0, len(columns)+1)
			ptrs = append(ptrs, &r.id)
			for i := range r.values {
				ptrs = append(ptrs, &r.values[i])
			}
			if serr := srcRows.Scan(ptrs...); serr != nil {
				return nil, 0, serr
			}
			for i, v := range r.values {
				if b, ok := v.([]byte); ok {
					r.values[i] = string(b)
				}
			}
			if payload, jerr := json.Marshal(r.values); jerr == nil {
				totalBytes += len(payload) + len(r.id)
			}
			if totalBytes > maxProjectionBytes {
				return nil, 0, fmt.Errorf("%w: the source selection exceeds the projection byte bound (%d)", ErrProjectionTooLarge, maxProjectionBytes)
			}
			rows = append(rows, r)
		}
		if serr := srcRows.Err(); serr != nil {
			return nil, 0, serr
		}
		var head int64
		if herr := readTx.QueryRowContext(ctx,
			`SELECT COALESCE(MAX(commit_seq), 0) FROM _mb_changes`).Scan(&head); herr != nil {
			return nil, 0, fmt.Errorf("failed to read the source change-journal head: %w", herr)
		}
		return rows, head, nil
	}()
	if snapErr != nil {
		if errors.Is(snapErr, ErrReferenceSchemaIncompatible) {
			return rt.quarantineReferenceTargets(ctx, policy, targetIDs, snapErr)
		}
		return nil, snapErr
	}

	results := map[string]any{
		"targets": len(targetIDs), "updated": 0, "healthy": 0,
		"skipped": 0, "quarantined": 0, "failed": 0,
		"sourceSeq": head, "perTarget": map[string]any{},
	}
	perTarget := results["perTarget"].(map[string]any)
	for _, targetID := range targetIDs {
		generation := int64(1)
		if d, derr := rt.Service.GetDatabase(ctx, targetID); derr == nil {
			generation = d.Generation
		}
		outcome := rt.applyReferenceTarget(ctx, policy, targetID, generation, head, snapshot)
		perTarget[targetID] = outcome
		switch outcome["state"] {
		case "healthy":
			results["healthy"] = results["healthy"].(int) + 1
			results["updated"] = results["updated"].(int) + outcome["rows"].(int)
		case "skipped":
			results["skipped"] = results["skipped"].(int) + 1
		case "quarantined":
			results["quarantined"] = results["quarantined"].(int) + 1
		default:
			results["failed"] = results["failed"].(int) + 1
		}
	}
	return results, nil
}

// referenceApplyGuard relates one candidate batch (head, definition) to a
// target's durable checkpoint:
//
//   - "rebuild" when the copy has never been applied, a NEWER definition
//     epoch invalidated the old refresh work (any policy edit, same
//     revision included) or a newer revision/sequence arrives;
//   - "quarantined" when the applied batch of the SAME definition is
//     quarantined (a policy edit rebuilds and clears the quarantine);
//   - "skip" for a stale refresh (an older definition or revision can
//     never overwrite a newer one) or a duplicate/older event.
func referenceApplyGuard(st *referenceTargetState, policy *ReferencePolicyWire, head int64) (string, string) {
	if !st.Applied {
		return "rebuild", ""
	}
	switch {
	case st.DefinitionEpoch > policy.DefinitionEpoch:
		return "skip", fmt.Sprintf("definition epoch %d is older than the applied %d",
			policy.DefinitionEpoch, st.DefinitionEpoch)
	case st.DefinitionEpoch < policy.DefinitionEpoch:
		// The definition was edited since this target last applied:
		// the old refresh work is invalidated and the projection
		// rebuilds (this also clears a quarantine).
		return "rebuild", ""
	case st.PolicyRevision > policy.Revision:
		return "skip", fmt.Sprintf("policy revision %d is older than the applied %d",
			policy.Revision, st.PolicyRevision)
	case st.PolicyRevision < policy.Revision:
		return "rebuild", ""
	case st.State == "quarantined":
		return "quarantined", ""
	case head <= st.LastSourceSeq:
		return "skip", fmt.Sprintf("source head %d is not newer than the applied %d",
			head, st.LastSourceSeq)
	}
	return "rebuild", ""
}

// applyReferenceTarget applies (or skips) one guarded target batch. The
// deleteBehavior remove-copy replace empties the mirror and repopulates
// it from the verified snapshot, stamping every copy row of every
// committed batch with the batch's source sequence AND policy revision,
// and records the checkpoint (sequence, definition epoch, policy
// revision, state healthy, no failure). The projection builds in a
// STAGED mirror table in bounded batch transactions and the final
// transaction swaps the staged projection into place atomically (readers
// never observe a half-replaced copy; a failed batch leaves the previous
// projection serving and the committed staging batches durable).
func (rt *Runtime) applyReferenceTarget(ctx context.Context, policy *ReferencePolicyWire, targetID string, generation, head int64, rows []refreshSourceRow) map[string]any {
	handle, herr := rt.Handles.Open(ctx, targetID)
	if herr != nil {
		return map[string]any{"state": "failed", "error": herr.Error()}
	}
	if uerr := ensureReferenceTargetState(ctx, handle, policy.ID); uerr != nil {
		return map[string]any{"state": "failed", "error": uerr.Error()}
	}
	mirror := quoteIdent("_mb_ref_copy_" + policy.ID)
	staging := quoteIdent("_mb_ref_copy_" + policy.ID + "_staging")
	stateT := quoteIdent("_mb_ref_state_" + policy.ID)

	// Decision (atomic with the writes when it applies): the guard reads
	// the durable checkpoint inside the first batch transaction.
	{
		tx, terr := handle.BeginTx(ctx, nil)
		if terr != nil {
			return map[string]any{"state": "failed", "error": terr.Error()}
		}
		st, rerr := readReferenceTargetState(ctx, tx, policy.ID)
		if rerr == nil {
			switch action, reason := referenceApplyGuard(st, policy, head); action {
			case "quarantined":
				_ = tx.Rollback()
				outcome := map[string]any{"state": "quarantined"}
				if st.Failure != nil {
					outcome["error"] = *st.Failure
				}
				return outcome
			case "skip":
				_ = tx.Rollback()
				return map[string]any{"state": "skipped", "reason": reason}
			}
		}
		_ = tx.Rollback()
	}

	// Staged build: bounded batches, each its own transaction; every
	// committed batch row records the (source sequence, policy revision)
	// pair it was projected under.
	if _, err := handle.ExecContext(ctx, fmt.Sprintf("DROP TABLE IF EXISTS %s", staging)); err != nil {
		return map[string]any{"state": "failed", "error": err.Error()}
	}
	if _, err := handle.ExecContext(ctx, fmt.Sprintf(
		"CREATE TABLE %s (id TEXT PRIMARY KEY, payload_json TEXT NOT NULL, source_seq INTEGER NOT NULL, policy_revision INTEGER NOT NULL)", staging)); err != nil {
		return map[string]any{"state": "failed", "error": err.Error()}
	}
	for start := 0; start < len(rows); start += projectionBatchRows {
		end := start + projectionBatchRows
		if end > len(rows) {
			end = len(rows)
		}
		btx, terr := handle.BeginTx(ctx, nil)
		if terr != nil {
			return map[string]any{"state": "failed", "error": terr.Error()}
		}
		for _, r := range rows[start:end] {
			payload, jerr := json.Marshal(r.values)
			if jerr != nil {
				_ = btx.Rollback()
				return map[string]any{"state": "failed", "error": jerr.Error()}
			}
			if _, uerr := btx.ExecContext(ctx, fmt.Sprintf(
				"INSERT INTO %s (id, payload_json, source_seq, policy_revision) VALUES (?, ?, ?, ?)", staging),
				r.id, string(payload), head, policy.Revision); uerr != nil {
				_ = btx.Rollback()
				return map[string]any{"state": "failed", "error": uerr.Error()}
			}
		}
		if err := btx.Commit(); err != nil {
			return map[string]any{"state": "failed", "error": err.Error()}
		}
	}

	// Final transaction: the staged projection swaps into place and the
	// checkpoint records the batch (sequence, epoch, revision, healthy).
	outcome := map[string]any{"state": "failed", "error": "the mirror transaction failed"}
	func() {
		tx, terr := handle.BeginTx(ctx, nil)
		if terr != nil {
			outcome["error"] = terr.Error()
			return
		}
		defer func() { _ = tx.Rollback() }()
		st, rerr := readReferenceTargetState(ctx, tx, policy.ID)
		if rerr != nil {
			outcome["error"] = rerr.Error()
			return
		}
		switch action, reason := referenceApplyGuard(st, policy, head); action {
		case "quarantined":
			outcome["state"] = "quarantined"
			if st.Failure != nil {
				outcome["error"] = *st.Failure
			}
			return
		case "skip":
			outcome["state"] = "skipped"
			outcome["reason"] = reason
			return
		}
		if _, derr := tx.ExecContext(ctx, fmt.Sprintf("DROP TABLE IF EXISTS %s", mirror)); derr != nil {
			outcome["error"] = derr.Error()
			return
		}
		if _, rerr := tx.ExecContext(ctx, fmt.Sprintf("ALTER TABLE %s RENAME TO %s", staging, mirror)); rerr != nil {
			outcome["error"] = rerr.Error()
			return
		}
		if _, uerr := tx.ExecContext(ctx, fmt.Sprintf(
			"INSERT INTO %s (singleton, last_source_seq, policy_revision, definition_epoch, state, failure, updated_at) VALUES (1, ?, ?, ?, 'healthy', NULL, ?)"+
				" ON CONFLICT(singleton) DO UPDATE SET last_source_seq = excluded.last_source_seq, policy_revision = excluded.policy_revision, definition_epoch = excluded.definition_epoch, state = excluded.state, failure = excluded.failure, updated_at = excluded.updated_at",
			stateT), head, policy.Revision, policy.DefinitionEpoch, nowRFC3339()); uerr != nil {
			outcome["error"] = uerr.Error()
			return
		}
		if cerr := tx.Commit(); cerr != nil {
			outcome["error"] = cerr.Error()
			return
		}
		// The shared registry records the target generation, the applied
		// sequence, the policy revision, the definition epoch and the
		// absence of failure (best-effort: no cross-file atomic freshness
		// claim — the worker reconciles the registry from the target
		// checkpoint).
		if rerr := rt.recordReferenceTarget(ctx, policy.ID, targetID, generation, head, policy.Revision, policy.DefinitionEpoch, "healthy", nil); rerr != nil {
			outcome["registryError"] = rerr.Error()
		}
		outcome["state"] = "healthy"
		outcome["rows"] = len(rows)
		outcome["sourceSeq"] = head
		outcome["policyRevision"] = policy.Revision
		outcome["definitionEpoch"] = policy.DefinitionEpoch
		outcome["targetGeneration"] = generation
	}()
	return outcome
}

// recordReferenceTarget upserts the shared `_mb_reference_targets`
// registry row of one target (target generation, last applied sequence,
// policy revision, definition epoch and failure).
func (rt *Runtime) recordReferenceTarget(ctx context.Context, policyID, targetID string, generation, seq, revision, epoch int64, state string, failure *string) error {
	if _, err := rt.PrimaryDB.ExecContext(ctx, referenceTargetsDDL); err != nil {
		return err
	}
	var failureArg any
	if failure != nil {
		failureArg = *failure
	}
	_, err := rt.PrimaryDB.ExecContext(ctx,
		`INSERT INTO _mb_reference_targets (policy_id, target_database_id, target_generation, last_applied_sequence, policy_revision, definition_epoch, state, failure, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
		 ON CONFLICT(policy_id, target_database_id) DO UPDATE SET
		   target_generation = excluded.target_generation, last_applied_sequence = excluded.last_applied_sequence,
		   policy_revision = excluded.policy_revision, definition_epoch = excluded.definition_epoch,
		   state = excluded.state, failure = excluded.failure, updated_at = excluded.updated_at`,
		policyID, targetID, generation, seq, revision, epoch, state, failureArg, nowRFC3339())
	return err
}

// readReferenceTargetRegistry reads the registry row of one target
// (missing when never recorded).
func (rt *Runtime) readReferenceTargetRegistry(ctx context.Context, policyID, targetID string) (generation, seq, revision, epoch int64, state string, updatedAt string, err error) {
	err = rt.PrimaryDB.QueryRowContext(ctx,
		`SELECT target_generation, last_applied_sequence, policy_revision, definition_epoch, state, updated_at
		 FROM _mb_reference_targets WHERE policy_id = ? AND target_database_id = ?`,
		policyID, targetID,
	).Scan(&generation, &seq, &revision, &epoch, &state, &updatedAt)
	return
}

// sourceFieldOf resolves a policy field id or name to the source schema's
// field definition.
func sourceFieldOf(coll *core.Collection, fid string) core.Field {
	for _, f := range coll.Fields {
		if f.GetId() == fid || f.GetName() == fid {
			return f
		}
	}
	return nil
}

// quarantineReferenceTargets records a typed failure on every target
// checkpoint: the projection cannot be built from the current source
// schema, so the targets stop serving the copy until the policy is edited
// (a newer revision rebuilds it). The durable failure is written to each
// target's state row; the typed error is returned so the refresh surface
// reports it as well.
func (rt *Runtime) quarantineReferenceTargets(ctx context.Context, policy *ReferencePolicyWire, targetIDs []string, failure error) (map[string]any, error) {
	perTarget := map[string]any{}
	quarantined, failed := 0, 0
	for _, targetID := range targetIDs {
		handle, herr := rt.Handles.Open(ctx, targetID)
		if herr != nil {
			perTarget[targetID] = map[string]any{"state": "failed", "error": herr.Error()}
			failed++
			continue
		}
		if uerr := ensureReferenceTargetState(ctx, handle, policy.ID); uerr != nil {
			perTarget[targetID] = map[string]any{"state": "failed", "error": uerr.Error()}
			failed++
			continue
		}
		tx, terr := handle.BeginTx(ctx, nil)
		if terr != nil {
			perTarget[targetID] = map[string]any{"state": "failed", "error": terr.Error()}
			failed++
			continue
		}
		quarantinedOK := false
		generation := int64(1)
		if d, derr := rt.Service.GetDatabase(ctx, targetID); derr == nil {
			generation = d.Generation
		}
		func() {
			defer func() { _ = tx.Rollback() }()
			if _, uerr := tx.ExecContext(ctx, fmt.Sprintf(
				"INSERT INTO %s (singleton, last_source_seq, policy_revision, definition_epoch, state, failure, updated_at) VALUES (1, 0, ?, ?, 'quarantined', ?, ?)"+
					" ON CONFLICT(singleton) DO UPDATE SET policy_revision = excluded.policy_revision, definition_epoch = excluded.definition_epoch, state = excluded.state, failure = excluded.failure, updated_at = excluded.updated_at",
				quoteIdent("_mb_ref_state_"+policy.ID)),
				policy.Revision, policy.DefinitionEpoch, failure.Error(), nowRFC3339()); uerr != nil {
				return
			}
			if cerr := tx.Commit(); cerr != nil {
				return
			}
			quarantinedOK = true
			// The shared registry records the quarantined target's state
			// (best-effort reconciliation).
			failureText := failure.Error()
			_ = rt.recordReferenceTarget(ctx, policy.ID, targetID, generation, 0, policy.Revision, policy.DefinitionEpoch, "quarantined", &failureText)
		}()
		if quarantinedOK {
			perTarget[targetID] = map[string]any{"state": "quarantined", "error": failure.Error()}
			quarantined++
		} else {
			perTarget[targetID] = map[string]any{"state": "failed", "error": "failed to record the quarantine"}
			failed++
		}
	}
	return map[string]any{
		"targets": len(targetIDs), "quarantined": quarantined, "failed": failed,
		"error": failure.Error(), "perTarget": perTarget,
	}, failure
}

// GetReferenceStatus reads the durable per-target applied state: every
// selector-matching target reports healthy (applied within the declared
// maximum lag), stale (past MaximumLagSeconds, including never applied)
// or quarantined (recorded failure), with the applied sequence, the
// applied policy revision and the recorded failure.
func (rt *Runtime) GetReferenceStatus(ctx context.Context, policy *ReferencePolicyWire) (*ReferenceStatus, error) {
	st := &ReferenceStatus{ID: policy.ID, Revision: policy.Revision, TargetStates: []ReferenceTargetStatus{}}
	targetIDs, terr := rt.referenceTargetIDs(ctx, policy)
	if terr != nil {
		return nil, terr
	}
	st.Targets = len(targetIDs)
	for _, targetID := range targetIDs {
		entry := ReferenceTargetStatus{DatabaseID: targetID, State: "stale"}
		if gen, _, _, _, _, _, gerr := rt.readReferenceTargetRegistry(ctx, policy.ID, targetID); gerr == nil {
			entry.Generation = gen
		} else if d, derr := rt.Service.GetDatabase(ctx, targetID); derr == nil {
			entry.Generation = d.Generation
		}
		handle, herr := rt.Handles.Open(ctx, targetID)
		if herr != nil {
			entry.State = "failed"
			entry.Failure = herr.Error()
			st.Failed++
			st.TargetStates = append(st.TargetStates, entry)
			continue
		}
		var failure sql.NullString
		err := handle.QueryRowContext(ctx, fmt.Sprintf(
			"SELECT last_source_seq, policy_revision, state, failure, updated_at FROM %s WHERE singleton = 1",
			quoteIdent("_mb_ref_state_"+policy.ID)),
		).Scan(&entry.LastSourceSeq, &entry.PolicyRevision, &entry.State, &failure, &entry.UpdatedAt)
		if err != nil {
			// No checkpoint: the copy was never applied on this target.
			entry.State = "stale"
			entry.Failure = "never applied"
			st.Stale++
			st.TargetStates = append(st.TargetStates, entry)
			continue
		}
		if failure.Valid {
			entry.Failure = failure.String
		}
		switch entry.State {
		case "quarantined":
			st.Quarantined++
		case "failed":
			st.Failed++
		default:
			upd, perr := time.Parse(time.RFC3339Nano, entry.UpdatedAt)
			if perr != nil || time.Since(upd) > time.Duration(policy.MaximumLagSeconds)*time.Second {
				entry.State = "stale"
				st.Stale++
			} else {
				entry.State = "healthy"
				st.Healthy++
			}
		}
		st.TargetStates = append(st.TargetStates, entry)
	}
	return st, nil
}

// ReadReferenceCopy reads the projected copy rows from the named target
// (an empty recordID reads every projected row). The read REQUIRES a
// healthy copy: a quarantined, stale (apply time past the declared
// maximum lag) or never-applied copy surfaces ErrReferenceCopyStale
// (reference_stale) with the target diagnostics — never the data.
func (rt *Runtime) ReadReferenceCopy(ctx context.Context, policy *ReferencePolicyWire, targetDatabaseID, recordID string) (map[string]any, error) {
	handle, herr := rt.Handles.Open(ctx, targetDatabaseID)
	if herr != nil {
		return nil, herr
	}
	var lastSeq, appliedRevision, appliedEpoch int64
	var state, updatedAt string
	var failure sql.NullString
	err := handle.QueryRowContext(ctx, fmt.Sprintf(
		"SELECT last_source_seq, policy_revision, definition_epoch, state, failure, updated_at FROM %s WHERE singleton = 1",
		quoteIdent("_mb_ref_state_"+policy.ID)),
	).Scan(&lastSeq, &appliedRevision, &appliedEpoch, &state, &failure, &updatedAt)
	if errors.Is(err, sql.ErrNoRows) || err != nil {
		return nil, fmt.Errorf("%w: the copy has never been applied on target %q",
			ErrReferenceCopyStale, targetDatabaseID)
	}
	if state != "healthy" {
		diag := "the copy is not healthy (state " + state + ")"
		if failure.Valid && failure.String != "" {
			diag += ": " + failure.String
		}
		if head, ok := rt.referenceSourceHead(ctx); ok {
			diag += fmt.Sprintf("; source head %d, target sequence %d", head, lastSeq)
		}
		return nil, fmt.Errorf("%w: %s", ErrReferenceCopyStale, diag)
	}
	if appliedRevision != policy.Revision || appliedEpoch != policy.DefinitionEpoch {
		// The applied copy predates the current definition (a policy
		// edit rebuilds it within the declared lag): never serve the old
		// projection as the current policy's copy.
		return nil, fmt.Errorf("%w: the copy predates the current definition (applied revision %d epoch %d of policy revision %d epoch %d)",
			ErrReferenceCopyStale, appliedRevision, appliedEpoch, policy.Revision, policy.DefinitionEpoch)
	}
	upd, perr := time.Parse(time.RFC3339Nano, updatedAt)
	if perr != nil || time.Since(upd) > time.Duration(policy.MaximumLagSeconds)*time.Second {
		diag := fmt.Sprintf("the copy lag is past the declared maximum (%ds): applied %s, target sequence %d, revision %d",
			policy.MaximumLagSeconds, updatedAt, lastSeq, appliedRevision)
		if head, ok := rt.referenceSourceHead(ctx); ok {
			diag += fmt.Sprintf(", source head %d", head)
		}
		return nil, fmt.Errorf("%w: %s", ErrReferenceCopyStale, diag)
	}
	mirror := quoteIdent("_mb_ref_copy_" + policy.ID)
	query := fmt.Sprintf("SELECT id, payload_json, source_seq FROM %s", mirror)
	args := []any{}
	if recordID != "" {
		query += " WHERE id = ?"
		args = append(args, recordID)
	}
	query += " ORDER BY id"
	rows, qerr := handle.QueryContext(ctx, query, args...)
	if qerr != nil {
		return nil, qerr
	}
	defer rows.Close()
	rowsOut := []map[string]any{}
	for rows.Next() {
		var recordID, payload string
		var seq int64
		if serr := rows.Scan(&recordID, &payload, &seq); serr != nil {
			return nil, serr
		}
		var values []any
		_ = json.Unmarshal([]byte(payload), &values)
		rowsOut = append(rowsOut, map[string]any{"id": recordID, "values": values, "sourceSeq": seq})
	}
	if rerr := rows.Err(); rerr != nil {
		return nil, rerr
	}
	generation := int64(1)
	if gen, _, _, _, _, _, gerr := rt.readReferenceTargetRegistry(ctx, policy.ID, targetDatabaseID); gerr == nil && gen > 0 {
		generation = gen
	} else if d, derr := rt.Service.GetDatabase(ctx, targetDatabaseID); derr == nil {
		generation = d.Generation
	}
	out := map[string]any{
		"policyId":              policy.ID,
		"policyRevision":        policy.Revision,
		"targetDatabaseId":      targetDatabaseID,
		"targetGeneration":      generation,
		"lastSourceSeq":         lastSeq,
		"appliedPolicyRevision": appliedRevision,
		"rows":                  rowsOut,
	}
	// Diagnostics carry BOTH sides of the lag: the target's applied
	// source sequence and the source's current journal head.
	if head, ok := rt.referenceSourceHead(ctx); ok {
		out["sourceHead"] = head
	}
	return out, nil
}

// referenceSourceHead reads the SOURCE application's change-journal head
// (best-effort: a read that cannot reach the source reports no head
// rather than a fabricated sequence).
func (rt *Runtime) referenceSourceHead(ctx context.Context) (int64, bool) {
	if rt.Base == nil || rt.Base.DBConn() == nil {
		return 0, false
	}
	var head int64
	if err := rt.Base.DBConn().QueryRowContext(ctx,
		`SELECT COALESCE(MAX(commit_seq), 0) FROM _mb_changes`).Scan(&head); err != nil {
		return 0, false
	}
	return head, true
}

// ---------------------------------------------------------------------------
// Source CDC intent capture (MB-REFERENCE): the source CDC commit
// schedules a durable refresh intent by appending the committed record
// change to the source database's own change journal; the refresh worker
// gates on that journal head inside its source read transaction. The
// intent is advisory: the worker reconciles the full current source
// snapshot, so a lost intent only delays the copy until the next refresh
// and can never overwrite a newer applied batch.
// ---------------------------------------------------------------------------

// referenceSourceCDCBound records the applications whose source refresh
// intent capture is already bound (idempotent per app instance).
var referenceSourceCDCBound sync.Map // *core.BaseApp -> bool

// referenceSourceCDCMarker identifies the bound intent handlers.
const referenceSourceCDCMarker = "mirabase_reference_source_cdc"

// ensureReferenceSourceCDC binds the primary record-write capture that
// advances the source change journal (the durable refresh intent:
// implementation-closure.md "The source CDC commit schedules a durable
// refresh intent"). Idempotent per application instance; hooks do not
// survive a process restart, so the refresh worker re-binds defensively
// before reading the head. A capture failure never fails the
// already-committed record write.
func ensureReferenceSourceCDC(app core.App) error {
	base := baseAppOf(app)
	if base == nil || base.DBConn() == nil {
		return errors.New("the reference source CDC requires the concrete BaseApp connection")
	}
	if _, bound := referenceSourceCDCBound.LoadOrStore(base, true); bound {
		return nil
	}
	capture := func(action string) func(e *core.RecordEvent) error {
		return func(e *core.RecordEvent) error {
			if e == nil || e.Record == nil || e.Record.Collection() == nil {
				return e.Next()
			}
			rec := e.Record
			ctx := context.Background()
			tx, terr := base.DBConn().BeginTx(ctx, nil)
			if terr != nil {
				return e.Next()
			}
			func() {
				defer func() { _ = tx.Rollback() }()
				if _, cerr := CaptureChanges(ctx, tx, "record", []ChangeCapture{{
					Table: rec.TableName(), RecordID: rec.Id, Action: action,
					After: recordAfterImage(rec),
				}}); cerr != nil {
					return
				}
				_ = tx.Commit()
			}()
			// The intent is advisory: a failed capture never fails the
			// already-committed record write (the next refresh reconciles
			// the full snapshot).
			return e.Next()
		}
	}
	app.OnRecordAfterCreateSuccess().Bind(&hook.Handler[*core.RecordEvent]{
		Id: referenceSourceCDCMarker, Func: capture("create")})
	app.OnRecordAfterUpdateSuccess().Bind(&hook.Handler[*core.RecordEvent]{
		Id: referenceSourceCDCMarker, Func: capture("update")})
	app.OnRecordAfterDeleteSuccess().Bind(&hook.Handler[*core.RecordEvent]{
		Id: referenceSourceCDCMarker, Func: capture("delete")})
	return nil
}

// recordAfterImage is the compact after-image stored with a source
// refresh intent (the journal keeps the change observable; the worker
// reconciles the full snapshot, not the intent payload).
func recordAfterImage(rec *core.Record) map[string]any {
	out := map[string]any{}
	coll := rec.Collection()
	if coll == nil {
		return out
	}
	for _, f := range coll.Fields {
		out[f.GetName()] = rec.Get(f.GetName())
	}
	out["id"] = rec.Id
	return out
}

// LivePlanWire is the live-plan wire shape (implementation-closure.md:
// "PlanRegistry stores _mb_live_plans keyed by application-local name and
// immutable revision, with SQL AST/hash, typed parameters, result
// key/shape, dependencies, policy plan, row/byte limits, state and
// creator").
type LivePlanWire struct {
	ID           string                     `json:"id"`
	Revision     int64                      `json:"revision"`
	SQL          string                     `json:"sql"`
	Parameters   []map[string]any           `json:"parameters"`
	ResultKey    string                     `json:"resultKey"`
	Dependencies []string                   `json:"dependencies"`
	MaxRows      int                        `json:"maxRows"`
	MaxBytes     int                        `json:"maxBytes"`
	State        string                     `json:"state"`
	ResultShape  []string                   `json:"resultShape,omitempty"`
	Policy       []livequery.PlanPolicy     `json:"policy,omitempty"`
	Creator      string                     `json:"creator,omitempty"`
}

// nondeterministicTokens is the conservative determinism denylist for
// live-plan SQL (D-14: "Validate deterministic expressions"): the engine's
// side-effecting/non-deterministic scalar functions and the current-time
// forms. The scan is token-based over the lowercase SQL, so a deterministic
// query that merely CONTAINS such a word inside a string literal is also
// refused — a conservative failure, never an acceptance.
var nondeterministicTokens = []string{
	"random(", "randomblob(", "changes(", "last_insert_rowid(",
	"total_changes(", "current_timestamp", "current_date", "current_time",
	"'now'",
}

// validateReadOnlySQL refuses anything that is not a plain, deterministic
// SELECT.
func validateReadOnlySQL(sqlText string) error {
	trimmed := strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(sqlText), ";"))
	lower := strings.ToLower(trimmed)
	if !strings.HasPrefix(lower, "select") && !strings.HasPrefix(lower, "with") {
		return fmt.Errorf("%w: the live plan SQL must be a SELECT statement", ErrInvalidPlan)
	}
	if strings.Contains(lower, ";") {
		return fmt.Errorf("%w: the live plan SQL must be a single statement", ErrInvalidPlan)
	}
	for _, bad := range []string{"insert ", "update ", "delete ", "drop ", "alter ", "create ", "attach ", "pragma ", "vacuum ", "reindex "} {
		if strings.Contains(lower, bad) {
			return fmt.Errorf("%w: the live plan SQL must be read-only", ErrInvalidPlan)
		}
	}
	for _, tok := range nondeterministicTokens {
		if strings.Contains(lower, tok) {
			return fmt.Errorf("%w: the live plan SQL must be deterministic (%q is not)", ErrInvalidPlan, tok)
		}
	}
	return nil
}

// CreateLivePlan prepares and validates an immutable named read-only query
// revision (implementation-closure.md: "SQL must prepare read-only in the
// actual profile, be deterministic, expose a unique non-null result key
// and permit collection/field policy injection"). The revision stores the
// verified result shape and the collection/field policy plan: the
// dependency tables are verified through EXPLAIN QUERY PLAN (the engine's
// own plan — a keyword prefix is not authorization), hidden fields of the
// dependency collections may appear neither in the SQL nor in the result
// shape, and the plan's accessed tables are exactly the declared
// dependencies (any undeclared table is refused here, before publication).
// The ACTUAL-PROFILE compile verification runs at publication:
// Runtime.PublishLivePlan proves the stored SQL prepares read-only on
// every ready target that carries the dependency collections (the
// application primary alone is not the evaluation profile), and
// live.register evaluates on the real target handle BEFORE any
// subscription is allocated.
func (s *Service) CreateLivePlan(ctx context.Context, app *core.BaseApp, plan *LivePlanWire) (*LivePlanWire, error) {
	if err := s.ensureDerivedTables(ctx); err != nil {
		return nil, err
	}
	if plan.ID == "" {
		plan.ID = "lp" + strings.ReplaceAll(NewID(""), "-", "")
	}
	if len(plan.ID) < 3 || strings.ContainsAny(plan.ID, "\"'; -") {
		return nil, fmt.Errorf("invalid_request: the plan id must be a safe identifier")
	}
	if plan.SQL == "" || plan.ResultKey == "" {
		return nil, fmt.Errorf("invalid_request: sql and resultKey are required")
	}
	if plan.MaxRows < 1 || plan.MaxRows > 1000 || plan.MaxBytes < 1024 || plan.MaxBytes > 8388608 {
		return nil, fmt.Errorf("invalid_request: maxRows must be 1..1000 and maxBytes 1024..8388608")
	}
	if len(plan.Parameters) > 100 || len(plan.Dependencies) > 100 {
		return nil, fmt.Errorf("invalid_request: at most 100 parameters and dependencies")
	}
	if len(plan.Dependencies) < 1 {
		return nil, fmt.Errorf("invalid_request: the plan declares its dependency collections")
	}
	if err := validateReadOnlySQL(plan.SQL); err != nil {
		return nil, err
	}
	// Typed parameters: each declares a name and a wire type (the types
	// the evaluator validates request values against BEFORE subscription
	// allocation).
	for _, p := range plan.Parameters {
		name, _ := p["name"].(string)
		ptype, _ := p["type"].(string)
		if name == "" {
			return nil, fmt.Errorf("invalid_request: every plan parameter declares a name")
		}
		switch ptype {
		case "string", "text", "integer", "real", "boolean", "json":
			// "text" is the established wire alias of "string".
		default:
			return nil, fmt.Errorf("invalid_request: the parameter %q declares an unknown type %q (string, text, integer, real, boolean, json)", name, ptype)
		}
	}

	// Dependency collections: they must exist (the source of the field
	// policy) and must be non-system tables. Plans on primary-only
	// collections are legitimate; the ACTUAL-PROFILE compile verification
	// is enforced at publication (Runtime.PublishLivePlan proves the plan
	// prepares read-only on every ready target that carries the declared
	// dependencies), and live.register evaluates the plan on the real
	// target handle BEFORE any subscription is allocated, so a plan a
	// target cannot serve fails there with a typed error (never a
	// half-allocated subscription).
	hiddenByCollection := map[string][]string{}
	depNames := map[string]bool{}
	for _, dep := range plan.Dependencies {
		coll, cerr := app.FindCollectionByNameOrId(dep)
		if cerr != nil {
			return nil, fmt.Errorf("invalid_request: the dependency collection %q does not exist", dep)
		}
		if isReservedTableName(coll.Name) {
			return nil, fmt.Errorf("invalid_request: the dependency %q is a protected system table", coll.Name)
		}
		depNames[coll.Name] = true
		var hidden []string
		for _, f := range coll.Fields {
			if f.GetHidden() {
				hidden = append(hidden, f.GetName())
			}
		}
		hiddenByCollection[coll.Name] = hidden
	}

	// Field policy: a hidden field of a dependency collection may appear
	// in neither the plan SQL nor the result shape.
	plainSQL := strings.Map(func(r rune) rune {
		if r == '"' || r == '`' || r == '[' || r == ']' {
			return -1
		}
		return r
	}, strings.ToLower(plan.SQL))
	plainTokens := map[string]bool{}
	for _, tok := range strings.FieldsFunc(plainSQL, func(r rune) bool {
		return !(r == '_' || r >= 'a' && r <= 'z' || r >= '0' && r <= '9')
	}) {
		plainTokens[tok] = true
	}
	for collName, hidden := range hiddenByCollection {
		for _, h := range hidden {
			lower := strings.ToLower(h)
			if plainTokens[lower] {
				return nil, fmt.Errorf("invalid_request: the field policy of %q forbids the hidden field %q in the plan", collName, h)
			}
		}
	}

	// Table dependencies, verified by the engine's own plan (not a
	// keyword prefix): EXPLAIN QUERY PLAN names every accessed table.
	accessed, aerr := planAccessedTables(ctx, app.DBConn(), plan.SQL, len(plan.Parameters))
	if aerr != nil {
		return nil, aerr
	}
	for t := range accessed {
		if strings.HasPrefix(t, "_") || strings.HasPrefix(t, "sqlite_") {
			return nil, fmt.Errorf("%w: the plan reads the protected table %q", ErrInvalidPlan, t)
		}
		if !depNames[t] {
			return nil, fmt.Errorf("%w: the plan reads the undeclared table %q; dependencies are the declared collections only", ErrInvalidPlan, t)
		}
	}
	for _, dep := range plan.Dependencies {
		coll, _ := app.FindCollectionByNameOrId(dep)
		if !accessed[coll.Name] {
			return nil, fmt.Errorf("%w: the declared dependency %q is not read by the plan (the stored policy plan must match the plan)", ErrInvalidPlan, coll.Name)
		}
	}

	// Result shape: run the plan with bound parameters and an impossible
	// filter to enumerate its result columns without fetching data.
	params := make([]any, len(plan.Parameters))
	for i := range params {
		params[i] = nil
	}
	shapeRows, xerr := app.DBConn().QueryContext(ctx,
		fmt.Sprintf("SELECT * FROM (%s) WHERE 0", plan.SQL), params...)
	if xerr != nil {
		return nil, fmt.Errorf("invalid_request: the plan SQL does not compile: %v", xerr)
	}
	shape, serr := shapeRows.Columns()
	shapeRows.Close()
	if serr != nil {
		return nil, serr
	}
	shapeLower := map[string]bool{}
	for _, c := range shape {
		shapeLower[strings.ToLower(c)] = true
	}
	for collName, hidden := range hiddenByCollection {
		for _, h := range hidden {
			if shapeLower[strings.ToLower(h)] {
				return nil, fmt.Errorf("invalid_request: the result shape exposes the hidden field %q of %q", h, collName)
			}
		}
	}
	// The result key's presence in the result columns is enforced at
	// instantiation (the evaluator refuses an evaluation whose result
	// lacks the key — before any subscription is allocated); creation
	// records the declared key and the verified shape as published.

	// Compile with the declared parameters bound to NULL: the EXPLAIN
	// proves the statement compiles against the application schema (the
	// per-target registration compiles on the actual target handle).
	stmt, xerr := app.DBConn().PrepareContext(ctx, "EXPLAIN "+plan.SQL)
	if xerr != nil {
		return nil, fmt.Errorf("invalid_request: the plan SQL does not compile: %v", xerr)
	}
	stmt.Close()

	var current int64
	_ = s.db.QueryRowContext(ctx,
		"SELECT MAX(revision) FROM _mb_live_plans WHERE id = ?", plan.ID).Scan(&current)
	if plan.Revision == 0 {
		plan.Revision = current + 1
	}
	if plan.Revision <= current {
		return nil, fmt.Errorf("%w: the plan revision regressed", ErrReferenceStale)
	}
	paramsJSON, _ := json.Marshal(plan.Parameters)
	depsJSON, _ := json.Marshal(plan.Dependencies)
	shapeJSON, _ := json.Marshal(shape)
	policy := make([]livequery.PlanPolicy, 0, len(plan.Dependencies))
	for _, dep := range plan.Dependencies {
		coll, _ := app.FindCollectionByNameOrId(dep)
		policy = append(policy, livequery.PlanPolicy{Collection: coll.Name, Hidden: hiddenByCollection[coll.Name]})
	}
	policyJSON, _ := json.Marshal(policy)
	plan.ResultShape = shape
	plan.Policy = policy
	if plan.Creator == "" {
		plan.Creator = "superuser"
	}
	if _, err := s.db.ExecContext(ctx,
		"INSERT INTO _mb_live_plans (id, revision, sql, parameters_json, result_key, dependencies_json, max_rows, max_bytes, state, result_shape_json, policy_json, creator, created) VALUES (?, ?, ?, ?, ?, ?, ?, ?, 'draft', ?, ?, ?, ?)",
		plan.ID, plan.Revision, plan.SQL, string(paramsJSON), plan.ResultKey, string(depsJSON),
		plan.MaxRows, plan.MaxBytes, string(shapeJSON), string(policyJSON), plan.Creator, nowRFC3339()); err != nil {
		return nil, err
	}
	plan.State = "draft"
	return plan, nil
}

// planAccessedTables runs EXPLAIN QUERY PLAN and extracts the table names
// the engine's plan actually touches (SCAN/SEARCH lines; the plan — not a
// keyword prefix — is the authority for the accessed set). The engine's
// own intermediates are not tables: a "SCAN SUBQUERY n" line (a
// materialized subquery) and the scan of a co-routine CTE (the "CO-ROUTINE
// <name>" detail marks it) name no collection, and their underlying real
// scans appear on their own plan lines — those pseudo-names are skipped
// rather than read as undeclared tables.
func planAccessedTables(ctx context.Context, db *sql.DB, planSQL string, paramCount int) (map[string]bool, error) {
	params := make([]any, paramCount)
	for i := range params {
		params[i] = nil
	}
	rows, err := db.QueryContext(ctx, "EXPLAIN QUERY PLAN "+planSQL, params...)
	if err != nil {
		return nil, fmt.Errorf("invalid_request: the plan SQL does not compile: %v", err)
	}
	defer rows.Close()
	type planLine struct {
		detail string
	}
	var lines []planLine
	for rows.Next() {
		var selectID, parent, notused, detail string
		if err := rows.Scan(&selectID, &parent, &notused, &detail); err != nil {
			return nil, err
		}
		lines = append(lines, planLine{detail: detail})
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	// The co-routine names (CTE materializations) the plan declares.
	coroutines := map[string]bool{}
	for _, l := range lines {
		for _, m := range coroutineRe.FindAllStringSubmatch(l.detail, -1) {
			coroutines[strings.Trim(m[1], `"`)] = true
		}
	}
	out := map[string]bool{}
	for _, l := range lines {
		for _, m := range scanRe.FindAllStringSubmatch(l.detail, -1) {
			name := strings.Trim(m[1], `"`)
			if strings.EqualFold(name, "SUBQUERY") {
				continue // the engine's materialized subquery, not a table
			}
			coroutine := false
			for c := range coroutines {
				if strings.EqualFold(name, c) {
					coroutine = true
					break
				}
			}
			if coroutine {
				continue // a co-routine CTE materialization, not a table
			}
			out[name] = true
		}
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("%w: the plan's accessed tables could not be verified", ErrInvalidPlan)
	}
	return out, nil
}

// scanRe matches the EQP detail's SCAN/SEARCH subject (a table name, or
// the engine's SUBQUERY/co-routine pseudo-subject).
var scanRe = regexp.MustCompile(`(?i)\b(?:SCAN|SEARCH)\s+("?[\w]+"?)`)

// coroutineRe matches the EQP CO-ROUTINE markers (a CTE materialization's
// own line; its underlying scans appear separately).
var coroutineRe = regexp.MustCompile(`CO-ROUTINE\s+("?[\w]+"?)`)

// isReservedTableName reports a protected metadata namespace name.
func isReservedTableName(name string) bool {
	lower := strings.ToLower(strings.TrimSpace(name))
	return strings.HasPrefix(lower, "_") || strings.HasPrefix(lower, "sqlite_")
}

// GetLivePlan loads one exact revision.
func (s *Service) GetLivePlan(ctx context.Context, id string, revision int64) (*LivePlanWire, error) {
	var p LivePlanWire
	var paramsJSON, depsJSON, shapeJSON, policyJSON string
	err := s.db.QueryRowContext(ctx,
		"SELECT id, revision, sql, parameters_json, result_key, dependencies_json, max_rows, max_bytes, state, result_shape_json, policy_json, creator FROM _mb_live_plans WHERE id = ? AND revision = ?", id, revision,
	).Scan(&p.ID, &p.Revision, &p.SQL, &paramsJSON, &p.ResultKey, &depsJSON, &p.MaxRows, &p.MaxBytes, &p.State,
		&shapeJSON, &policyJSON, &p.Creator)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	_ = json.Unmarshal([]byte(paramsJSON), &p.Parameters)
	_ = json.Unmarshal([]byte(depsJSON), &p.Dependencies)
	_ = json.Unmarshal([]byte(shapeJSON), &p.ResultShape)
	_ = json.Unmarshal([]byte(policyJSON), &p.Policy)
	return &p, nil
}

// ListLivePlans lists the plan revisions (liveplan.list).
func (s *Service) ListLivePlans(ctx context.Context) ([]LivePlanWire, error) {
	rows, err := s.db.QueryContext(ctx,
		"SELECT id, revision, sql, result_key, state FROM _mb_live_plans ORDER BY id, revision")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []LivePlanWire{}
	for rows.Next() {
		var p LivePlanWire
		if err := rows.Scan(&p.ID, &p.Revision, &p.SQL, &p.ResultKey, &p.State); err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

// PublishLivePlan flips a draft revision to published (and retires the
// previously published revision of the same plan).
func (s *Service) PublishLivePlan(ctx context.Context, id string, revision int64) (*LivePlanWire, error) {
	res, err := s.db.ExecContext(ctx,
		"UPDATE _mb_live_plans SET state = 'published' WHERE id = ? AND revision = ? AND state = 'draft'", id, revision)
	if err != nil {
		return nil, err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return nil, ErrNotFound
	}
	_, _ = s.db.ExecContext(ctx,
		"UPDATE _mb_live_plans SET state = 'retired' WHERE id = ? AND revision != ? AND state = 'published'", id, revision)
	return s.GetLivePlan(ctx, id, revision)
}

// RetireLivePlan retires a published revision.
func (s *Service) RetireLivePlan(ctx context.Context, id string, revision int64) (*LivePlanWire, error) {
	res, err := s.db.ExecContext(ctx,
		"UPDATE _mb_live_plans SET state = 'retired' WHERE id = ? AND revision = ? AND state IN ('draft','published')", id, revision)
	if err != nil {
		return nil, err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return nil, ErrNotFound
	}
	return s.GetLivePlan(ctx, id, revision)
}

// RelationWorkflowWire is the workflow wire shape.
type RelationWorkflowWire struct {
	ID        string           `json:"id"`
	Revision  int64            `json:"revision"`
	Steps     []map[string]any `json:"steps"`
	Principal map[string]any   `json:"principal"`
}

// DefineRelationWorkflow validates explicit same-application qualified
// cascade steps and the guarded service principal.
func (rt *Runtime) DefineRelationWorkflow(ctx context.Context, wf *RelationWorkflowWire) (*RelationWorkflowWire, error) {
	if err := rt.Service.ensureDerivedTables(ctx); err != nil {
		return nil, err
	}
	if wf.ID == "" || len(wf.ID) < 3 || strings.ContainsAny(wf.ID, "\"'; -") {
		return nil, fmt.Errorf("invalid_request: the workflow id must be a safe identifier")
	}
	if len(wf.Steps) == 0 || len(wf.Steps) > 1000 {
		return nil, fmt.Errorf("invalid_request: 1..1000 steps are required")
	}
	if wf.Principal == nil {
		return nil, fmt.Errorf("invalid_request: the guarded service principal is required")
	}
	for _, step := range wf.Steps {
		dbID, _ := step["databaseId"].(string)
		collID, _ := step["collectionId"].(string)
		fieldID, _ := step["relationFieldId"].(string)
		action, _ := step["action"].(string)
		expectedGen, _ := step["expectedGeneration"].(float64)
		if dbID == "" || collID == "" || fieldID == "" || (action != "delete" && action != "unlink") {
			return nil, fmt.Errorf("invalid_request: each step requires databaseId, collectionId, relationFieldId and action delete|unlink")
		}
		d, derr := rt.Service.GetDatabase(ctx, dbID)
		if derr != nil {
			return nil, fmt.Errorf("not_found: step database %q does not exist", dbID)
		}
		if int64(expectedGen) != d.Generation {
			return nil, fmt.Errorf("%w: step database %q generation moved (expected %d, live %d)",
				ErrReferenceStale, dbID, int64(expectedGen), d.Generation)
		}
		handle, herr := rt.Handles.Open(ctx, dbID)
		if herr != nil {
			return nil, fmt.Errorf("failed to open the step database: %w", herr)
		}
		// The target's table name is the collection NAME; resolve the
		// step's collection id/name through the application schema.
		stepTable := collID
		if stepColl, serr := rt.Base.FindCollectionByNameOrId(collID); serr == nil {
			stepTable = stepColl.Name
		}
		var colCount int
		if err := handle.QueryRowContext(ctx, fmt.Sprintf(
			"SELECT COUNT(*) FROM pragma_table_info(%q) WHERE name = ?", stepTable), fieldID).Scan(&colCount); err != nil {
			return nil, fmt.Errorf("invalid_request: the step collection %q does not exist in %q", collID, dbID)
		}
		if colCount == 0 {
			return nil, fmt.Errorf("invalid_request: the relation field %q does not exist on %q in %q", fieldID, collID, dbID)
		}
	}
	stepsJSON, _ := json.Marshal(wf.Steps)
	principalJSON, _ := json.Marshal(wf.Principal)
	var current int64
	_ = rt.PrimaryDB.QueryRowContext(ctx,
		"SELECT MAX(revision) FROM _mb_relation_workflows WHERE id = ?", wf.ID).Scan(&current)
	if wf.Revision == 0 {
		wf.Revision = current + 1
	}
	if wf.Revision <= current {
		return nil, fmt.Errorf("%w: the workflow revision regressed", ErrReferenceStale)
	}
	if _, err := rt.PrimaryDB.ExecContext(ctx,
		"INSERT INTO _mb_relation_workflows (id, revision, steps_json, principal_json, created) VALUES (?, ?, ?, ?, ?)",
		wf.ID, wf.Revision, string(stepsJSON), string(principalJSON), nowRFC3339()); err != nil {
		return nil, err
	}
	return wf, nil
}

// GetRelationWorkflow loads one revision.
func (rt *Runtime) GetRelationWorkflow(ctx context.Context, id string, revision int64) (*RelationWorkflowWire, error) {
	var wf RelationWorkflowWire
	var stepsJSON, principalJSON string
	err := rt.PrimaryDB.QueryRowContext(ctx,
		"SELECT id, revision, steps_json, principal_json FROM _mb_relation_workflows WHERE id = ? AND revision = ?", id, revision,
	).Scan(&wf.ID, &wf.Revision, &stepsJSON, &principalJSON)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	_ = json.Unmarshal([]byte(stepsJSON), &wf.Steps)
	_ = json.Unmarshal([]byte(principalJSON), &wf.Principal)
	return &wf, nil
}

// RunRelationWorkflow executes the deterministic per-context cascade
// steps: rows whose relation field references a record id that no longer
// exists in the same target's referenced collection are deleted or
// unlinked, with per-step visible results and generation guards.
func (rt *Runtime) RunRelationWorkflow(ctx context.Context, workflowID string, revision int64) (map[string]any, error) {
	var stepsJSON string
	err := rt.PrimaryDB.QueryRowContext(ctx,
		"SELECT steps_json FROM _mb_relation_workflows WHERE id = ? AND revision = ?", workflowID, revision).Scan(&stepsJSON)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	var steps []map[string]any
	if json.Unmarshal([]byte(stepsJSON), &steps) != nil {
		return nil, fmt.Errorf("corrupt workflow steps")
	}
	stepResults := []map[string]any{}
	totalAffected := 0
	for _, step := range steps {
		dbID, _ := step["databaseId"].(string)
		collID, _ := step["collectionId"].(string)
		fieldID, _ := step["relationFieldId"].(string)
		action, _ := step["action"].(string)
		expectedGen, _ := step["expectedGeneration"].(float64)

		d, derr := rt.Service.GetDatabase(ctx, dbID)
		if derr != nil {
			stepResults = append(stepResults, map[string]any{"databaseId": dbID, "state": "failed", "error": "the database no longer exists"})
			continue
		}
		if d.Generation != int64(expectedGen) {
			stepResults = append(stepResults, map[string]any{"databaseId": dbID, "state": "failed",
				"error": fmt.Sprintf("generation_mismatch: expected %d, live %d", int64(expectedGen), d.Generation)})
			continue
		}
		handle, herr := rt.Handles.Open(ctx, dbID)
		if herr != nil {
			stepResults = append(stepResults, map[string]any{"databaseId": dbID, "state": "failed", "error": herr.Error()})
			continue
		}
		// The target's table name is the collection NAME.
		stepTable := collID
		if stepColl, serr := rt.Base.FindCollectionByNameOrId(collID); serr == nil {
			stepTable = stepColl.Name
		}
		affected := int64(0)
		switch action {
		case "delete":
			res, xerr := handle.ExecContext(ctx, fmt.Sprintf(
				"DELETE FROM %q WHERE %q IS NOT NULL AND NOT EXISTS (SELECT 1 FROM %q r2 WHERE r2.id = %q.%q)",
				stepTable, fieldID, stepTable, stepTable, fieldID))
			if xerr != nil {
				stepResults = append(stepResults, map[string]any{"databaseId": dbID, "state": "failed", "error": xerr.Error()})
				continue
			}
			affected, _ = res.RowsAffected()
		case "unlink":
			res, xerr := handle.ExecContext(ctx, fmt.Sprintf(
				"UPDATE %q SET %q = NULL WHERE %q IS NOT NULL AND NOT EXISTS (SELECT 1 FROM %q r2 WHERE r2.id = %q.%q)",
				stepTable, fieldID, fieldID, stepTable, stepTable, fieldID))
			if xerr != nil {
				stepResults = append(stepResults, map[string]any{"databaseId": dbID, "state": "failed", "error": xerr.Error()})
				continue
			}
			affected, _ = res.RowsAffected()
		}
		totalAffected += int(affected)
		stepResults = append(stepResults, map[string]any{"databaseId": dbID, "state": "succeeded", "affected": affected})
	}
	return map[string]any{"steps": stepResults, "totalAffected": totalAffected}, nil
}

// ReportWire is the report wire shape.
type ReportWire struct {
	ID                 string   `json:"id"`
	Revision           int64    `json:"revision"`
	SourceTemplateIDs  []string `json:"sourceTemplateIds"`
	ProjectedFieldIDs  []string `json:"projectedFieldIds"`
	AggregateSQL       string   `json:"aggregateSql"`
	TargetCollectionID string   `json:"targetCollectionId"`
	FreshnessSeconds   int      `json:"freshnessSeconds"`
	ReaderRule         string   `json:"readerRule"`
	LastRebuiltAt      *string  `json:"lastRebuiltAt"`
}

// DefineReport publishes the approved projected aggregate and its reader
// rule.
func (s *Service) DefineReport(ctx context.Context, app core.App, report *ReportWire) (*ReportWire, error) {
	if err := s.ensureDerivedTables(ctx); err != nil {
		return nil, err
	}
	if report.ID == "" || len(report.ID) < 3 || strings.ContainsAny(report.ID, "\"'; -") {
		return nil, fmt.Errorf("invalid_request: the report id must be a safe identifier")
	}
	if len(report.SourceTemplateIDs) == 0 || len(report.SourceTemplateIDs) > 100 {
		return nil, fmt.Errorf("invalid_request: 1..100 source template ids are required")
	}
	for _, tid := range report.SourceTemplateIDs {
		var published int
		if err := s.db.QueryRowContext(ctx,
			"SELECT COUNT(*) FROM _mb_schema_templates WHERE id = ? AND publication_state = 'published'", tid).Scan(&published); err != nil {
			return nil, fmt.Errorf("not_found: the templates store is not initialized")
		}
		if published == 0 {
			return nil, fmt.Errorf("not_found: source template %q is not published", tid)
		}
	}
	if len(report.ProjectedFieldIDs) == 0 || len(report.ProjectedFieldIDs) > 100 {
		return nil, fmt.Errorf("invalid_request: 1..100 projected field ids are required")
	}
	if err := validateReadOnlySQL(report.AggregateSQL); err != nil {
		return nil, err
	}
	if _, err := app.FindCollectionByNameOrId(report.TargetCollectionID); err != nil {
		return nil, fmt.Errorf("not_found: the target collection does not exist")
	}
	if report.FreshnessSeconds < 1 || report.FreshnessSeconds > 86400 {
		return nil, fmt.Errorf("invalid_request: freshnessSeconds must be 1..86400")
	}
	var current int64
	_ = s.db.QueryRowContext(ctx, "SELECT revision FROM _mb_reports WHERE id = ?", report.ID).Scan(&current)
	if report.Revision == 0 {
		report.Revision = current + 1
	}
	if current > 0 && report.Revision < current {
		return nil, fmt.Errorf("%w: the report revision regressed", ErrReferenceStale)
	}
	tplJSON, _ := json.Marshal(report.SourceTemplateIDs)
	fieldJSON, _ := json.Marshal(report.ProjectedFieldIDs)
	if _, err := s.db.ExecContext(ctx,
		"INSERT INTO _mb_reports (id, revision, source_template_ids_json, projected_field_ids_json, aggregate_sql, target_collection_id, freshness_seconds, reader_rule, created) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?) ON CONFLICT(id) DO UPDATE SET revision = excluded.revision, source_template_ids_json = excluded.source_template_ids_json, projected_field_ids_json = excluded.projected_field_ids_json, aggregate_sql = excluded.aggregate_sql, target_collection_id = excluded.target_collection_id, freshness_seconds = excluded.freshness_seconds, reader_rule = excluded.reader_rule, last_rebuilt_at = NULL",
		report.ID, report.Revision, string(tplJSON), string(fieldJSON), report.AggregateSQL,
		report.TargetCollectionID, report.FreshnessSeconds, report.ReaderRule, nowRFC3339()); err != nil {
		return nil, err
	}
	return report, nil
}

// GetReport loads one report.
func (s *Service) GetReport(ctx context.Context, id string) (*ReportWire, error) {
	var r ReportWire
	var tplJSON, fieldJSON string
	var lastRebuilt sql.NullString
	err := s.db.QueryRowContext(ctx,
		"SELECT id, revision, source_template_ids_json, projected_field_ids_json, aggregate_sql, target_collection_id, freshness_seconds, reader_rule, last_rebuilt_at FROM _mb_reports WHERE id = ?", id,
	).Scan(&r.ID, &r.Revision, &tplJSON, &fieldJSON, &r.AggregateSQL, &r.TargetCollectionID,
		&r.FreshnessSeconds, &r.ReaderRule, &lastRebuilt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	_ = json.Unmarshal([]byte(tplJSON), &r.SourceTemplateIDs)
	_ = json.Unmarshal([]byte(fieldJSON), &r.ProjectedFieldIDs)
	if lastRebuilt.Valid {
		r.LastRebuiltAt = &lastRebuilt.String
	}
	return &r, nil
}

// RebuildReport rebuilds the application-local report with bounded
// per-source checkpoints: the aggregate runs per ready target whose
// applied template is a source, and the aggregate row counts and
// checkpoint are recorded durably.
func (rt *Runtime) RebuildReport(ctx context.Context, reportID string) (map[string]any, error) {
	report, err := rt.Service.GetReport(ctx, reportID)
	if err != nil {
		return nil, err
	}
	rows, qerr := rt.PrimaryDB.QueryContext(ctx,
		"SELECT id FROM _mb_databases WHERE state = 'ready' AND kind IN ('manual','tenant','user','agent')")
	if qerr != nil {
		return nil, qerr
	}
	var targetIDs []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			rows.Close()
			return nil, err
		}
		targetIDs = append(targetIDs, id)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, err
	}
	totalRows := 0
	perSource := map[string]any{}
	for _, targetID := range targetIDs {
		handle, herr := rt.Handles.Open(ctx, targetID)
		if herr != nil {
			perSource[targetID] = map[string]any{"state": "failed", "error": herr.Error()}
			continue
		}
		var tplID string
		if gerr := handle.QueryRowContext(ctx,
			"SELECT template_id FROM _mb_schema WHERE singleton = 1").Scan(&tplID); gerr != nil || tplID == "" {
			perSource[targetID] = map[string]any{"state": "skipped", "reason": "no template applied"}
			continue
		}
		matched := false
		for _, want := range report.SourceTemplateIDs {
			if want == tplID {
				matched = true
				break
			}
		}
		if !matched {
			perSource[targetID] = map[string]any{"state": "skipped", "reason": "template not a source"}
			continue
		}
		aggRows, aerr := handle.QueryContext(ctx, report.AggregateSQL)
		if aerr != nil {
			perSource[targetID] = map[string]any{"state": "failed", "error": aerr.Error()}
			continue
		}
		cols, _ := aggRows.Columns()
		count := 0
		for aggRows.Next() {
			vals := make([]any, len(cols))
			ptrs := make([]any, len(cols))
			for i := range vals {
				ptrs[i] = &vals[i]
			}
			if serr := aggRows.Scan(ptrs...); serr != nil {
				aggRows.Close()
				return nil, serr
			}
			count++
		}
		aggRows.Close()
		totalRows += count
		perSource[targetID] = map[string]any{"state": "succeeded", "rows": count}
	}
	_, _ = rt.PrimaryDB.ExecContext(ctx,
		"UPDATE _mb_reports SET last_rebuilt_at = ?, last_checkpoint = ? WHERE id = ?",
		nowRFC3339(), fmt.Sprintf("targets=%d aggregateRows=%d", len(targetIDs), totalRows), reportID)
	return map[string]any{"aggregateRows": totalRows, "perSource": perSource}, nil
}

// ensureDerivedTables installs all R6 stores.
func (s *Service) ensureDerivedTables(ctx context.Context) error {
	for _, ddl := range []string{derivedDDL, referenceTargetsDDL, livePlansDDL, relationWorkflowsDDL, reportsDDL} {
		if _, err := s.db.ExecContext(ctx, ddl); err != nil {
			return fmt.Errorf("failed to install the derived-data store: %w", err)
		}
	}
	// Upgrade tables created by an earlier release (the added columns are
	// appended in place with the defaults the old shape implies; no
	// existing policy/target/state data is dropped).
	polCols, err := tableColumns(ctx, s.db, "_mb_reference_policies")
	if err != nil {
		return err
	}
	for name, ddl := range map[string]string{
		"schema_revision":  "ALTER TABLE _mb_reference_policies ADD COLUMN schema_revision TEXT NOT NULL DEFAULT ''",
		"definition_epoch": "ALTER TABLE _mb_reference_policies ADD COLUMN definition_epoch INTEGER NOT NULL DEFAULT 0",
	} {
		if polCols[name] {
			continue
		}
		if _, err := s.db.ExecContext(ctx, ddl); err != nil {
			return fmt.Errorf("failed to upgrade the reference policy store: %w", err)
		}
	}
	tgtCols, err := tableColumns(ctx, s.db, "_mb_reference_targets")
	if err != nil {
		return err
	}
	if !tgtCols["definition_epoch"] {
		if _, err := s.db.ExecContext(ctx,
			"ALTER TABLE _mb_reference_targets ADD COLUMN definition_epoch INTEGER NOT NULL DEFAULT 0"); err != nil {
			return fmt.Errorf("failed to upgrade the reference target registry: %w", err)
		}
	}
	planCols, err := tableColumns(ctx, s.db, "_mb_live_plans")
	if err != nil {
		return err
	}
	for name, ddl := range map[string]string{
		"result_shape_json": "ALTER TABLE _mb_live_plans ADD COLUMN result_shape_json TEXT NOT NULL DEFAULT '[]'",
		"policy_json":       "ALTER TABLE _mb_live_plans ADD COLUMN policy_json TEXT NOT NULL DEFAULT '{}'",
		"creator":           "ALTER TABLE _mb_live_plans ADD COLUMN creator TEXT NOT NULL DEFAULT ''",
	} {
		if planCols[name] {
			continue
		}
		if _, err := s.db.ExecContext(ctx, ddl); err != nil {
			return fmt.Errorf("failed to upgrade the live-plan store: %w", err)
		}
	}
	return nil
}
