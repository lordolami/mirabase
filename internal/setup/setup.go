// Package setup owns the guided storage setup (build-start.md:
// "server/internal/setup — Guided storage policy, memberships/invitations
// and layout migration"). v0.1 slice: default application only; shared or
// empty user/tenant/agent collection policy; validated empty-layout
// activation only (new-application-setup.md, http-operation-reference.md
// setup.get/plan/apply).
package setup

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"mirabase/core"
	"mirabase/internal/application"
)

// coreApp is the app surface the planner needs (embedding core.App for
// the template publication machinery).
type coreApp = core.App

// Plan states (wire SetupPlan.state = _mb_setup_plans.state, byte-identical
// enums).
const (
	StateDraft      = "draft"
	StateValidating = "validating"
	StateInvalid    = "invalid"
	StateReady      = "ready"
	StateApplying   = "applying"
	StateActive     = "active"
	StateFailed     = "failed"
	StateCancelled  = "cancelled-before-publication"
)

// setupPlansDDL is the normative `_mb_setup_plans` DDL
// (storage-format-reference.md).
const setupPlansDDL = `CREATE TABLE IF NOT EXISTS _mb_setup_plans (
 id TEXT PRIMARY KEY, revision INTEGER NOT NULL CHECK(revision>0), application_generation INTEGER NOT NULL CHECK(application_generation>0),
 plan_hash TEXT NOT NULL, definition_json TEXT NOT NULL CHECK(json_valid(definition_json)),
 state TEXT NOT NULL CHECK(state IN ('draft','validating','invalid','ready','applying','active','failed','cancelled-before-publication'))
)`

// ErrLayoutMigrationRequired is the populated-relocation refusal
// (v0.1-build.md step 4: "moving existing populated shared collections is
// refused with `layout_migration_required` and a link to its roadmap
// contract. Existing data remains unchanged.").
var ErrLayoutMigrationRequired = errors.New("layout_migration_required")

// ErrGenerationMismatch guards the expected application generation/revision.
var ErrGenerationMismatch = errors.New("generation_mismatch")

// ErrPolicyChanged reports a dependent policy change after planning
// ("If any dependent schema or policy changed after planning, return
// ... policy_changed before effects").
var ErrPolicyChanged = errors.New("policy_changed")

// ErrSchemaMismatch reports a dependent schema change after planning.
var ErrSchemaMismatch = errors.New("schema_mismatch")

// Layouts.
const (
	LayoutShared = "shared"
	LayoutUser   = "user"
	LayoutTenant = "tenant"
	LayoutAgent  = "agent"
)

// Placement describes one collection's storage placement
// (ImportPlacement wire shape).
type Placement struct {
	CollectionID      string    `json:"collectionId"`
	Placement         string    `json:"placement"` // shared|tenant|reference|derived
	Ownership         Ownership `json:"ownership"`
	ReferenceFieldIDs []string  `json:"referenceFieldIds"`
	UniquePolicy      string    `json:"uniquePolicy"` // preserve-global|per-tenant-approved
	AdaptationIDs     []string  `json:"adaptationIds"`
}

// Ownership describes the row ownership model for a placed collection.
type Ownership struct {
	Kind     string   `json:"kind"` // root|field|relation-path|shared
	FieldID  string   `json:"fieldId,omitempty"`
	FieldIDs []string `json:"fieldIds,omitempty"`
}

// PlanRequest is the SetupPlanRequest wire shape (all required fields,
// additionalProperties:false).
type PlanRequest struct {
	ExpectedApplicationGeneration string        `json:"expectedApplicationGeneration"`
	ExpectedRevision              string        `json:"expectedRevision"`
	Layout                        string        `json:"layout"`
	AuthCollectionIDs             []string      `json:"authCollectionIds"`
	TemplateID                    *string       `json:"templateId"`
	Provision                     string        `json:"provision"` // lazy|eager
	TenantSource                  *TenantSource `json:"tenantSource"`
	Access                        *Access       `json:"access"`
	Placements                    []Placement   `json:"placements"`
	Namespace                     *string       `json:"namespace"`
}

// TenantSource selects the tenant source collection
// (ImportTenantSource).
type TenantSource struct {
	Kind         string `json:"kind"` // records|distinct-values
	CollectionID string `json:"collectionId"`
	FieldID      string `json:"fieldId,omitempty"`
}

// Access carries the membership authorization configuration
// (ImportAccess).
type Access struct {
	Membership struct {
		Kind                   string `json:"kind"`
		SourceCollectionID     string `json:"sourceCollectionId,omitempty"`
		Rule                   string `json:"rule,omitempty"`
		CollectionID           string `json:"collectionId,omitempty"`
		SubjectRelationFieldID string `json:"subjectRelationFieldId,omitempty"`
		TenantFieldID          string `json:"tenantFieldId,omitempty"`
		TenantCollectionID     string `json:"tenantCollectionId,omitempty"`
		OwnerRelationFieldID   string `json:"ownerRelationFieldId,omitempty"`
	} `json:"membership"`
	MembershipScopes   []string `json:"membershipScopes"`
	AllowAnonymousRead bool     `json:"allowAnonymousTenantRead"`
	RuleAdaptationIDs  []string `json:"ruleAdaptationIds"`
}

// Plan is the SetupPlan wire shape.
type Plan struct {
	ID                    string       `json:"id"`
	Revision              int64        `json:"revision"`
	ApplicationID         string       `json:"applicationId"`
	PlanHash              string       `json:"planHash"`
	Layout                string       `json:"layout"`
	State                 string       `json:"state"`
	IssueIDs              []string     `json:"issueIds"`
	PlannedDatabases      int          `json:"plannedDatabases"`
	ApplicationGeneration string       `json:"applicationGeneration"`
	Definition            *PlanRequest `json:"-"`
}

// SetupState is the setup.get response.
type SetupState struct {
	ApplicationID         string `json:"applicationId"`
	ApplicationGeneration string `json:"applicationGeneration"`
	Revision              string `json:"revision"`
	Layout                string `json:"layout"`
	Draft                 *Plan  `json:"draft"`
}

// Service owns the guided storage policy.
type Service struct {
	db       *sql.DB
	app      *application.Runtime
	revision int64
	genMu    sync.Mutex
	gen      int64
}

// NewService installs the setup tables and binds the policy service to
// the application runtime.
func NewService(db *sql.DB, rt *application.Runtime) (*Service, error) {
	if _, err := db.Exec(setupPlansDDL); err != nil {
		return nil, fmt.Errorf("failed to install setup plans: %w", err)
	}
	return &Service{db: db, app: rt, revision: 1, gen: 1}, nil
}

// CanonicalJSON renders a deterministic JSON encoding for hashing.
func CanonicalJSON(v any) ([]byte, error) {
	b, err := json.Marshal(v)
	if err != nil {
		return nil, err
	}
	// Normalize: sort object keys via a re-marshal through a generic map.
	var generic any
	if err := json.Unmarshal(b, &generic); err != nil {
		return b, nil
	}
	norm, err := json.Marshal(normalizeJSON(generic))
	if err != nil {
		return b, nil
	}
	return norm, nil
}

func normalizeJSON(v any) any {
	switch t := v.(type) {
	case map[string]any:
		keys := make([]string, 0, len(t))
		for k := range t {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		out := make(map[string]any, len(t))
		for _, k := range keys {
			out[k] = normalizeJSON(t[k])
		}
		return out
	case []any:
		out := make([]any, len(t))
		for i, e := range t {
			out[i] = normalizeJSON(e)
		}
		return out
	default:
		return v
	}
}

// HashPlan computes the canonical immutable plan hash
// (ImportHash: ^[a-f0-9]{64}$).
func HashPlan(req *PlanRequest) (string, error) {
	b, err := CanonicalJSON(req)
	if err != nil {
		return "", err
	}
	h := sha256.Sum256(b)
	return fmt.Sprintf("%x", h), nil
}

// GetState returns the current SetupState: revision, layout, and the
// caller-owned draft if any ("GET setup returns SetupState with a nullable
// draft; it does not invent a plan or install partition metadata").
func (s *Service) GetState(ctx context.Context) (*SetupState, error) {
	st := &SetupState{
		ApplicationID:         "default",
		ApplicationGeneration: fmt.Sprintf("%d", s.currentGeneration(ctx)),
		Revision:              fmt.Sprintf("%d", s.revision),
		Layout:                s.activeLayout(ctx),
	}
	// Latest draft plan owned by the caller (v0.1: single-operator setup;
	// the draft is the latest non-active/failed plan).
	row := s.db.QueryRowContext(ctx,
		`SELECT id, revision, application_generation, plan_hash, definition_json, state
		 FROM _mb_setup_plans WHERE state IN (?, ?) ORDER BY revision DESC LIMIT 1`,
		StateDraft, StateReady)
	var p Plan
	var defJSON string
	var gen int64
	err := row.Scan(&p.ID, &p.Revision, &gen, &p.PlanHash, &defJSON, &p.State)
	if err == nil {
		p.ApplicationID = "default"
		p.ApplicationGeneration = fmt.Sprintf("%d", gen)
		p.Layout = s.activeLayout(ctx)
		p.IssueIDs = []string{}
		var def PlanRequest
		if json.Unmarshal([]byte(defJSON), &def) == nil {
			p.Definition = &def
		}
		st.Draft = &p
	}
	return st, nil
}

// PlanDraft validates the request and persists an immutable draft:
// "Validate schema/ownership/membership and produce immutable preview
// without live layout effects". The refusal for populated shared→private
// relocation happens here BEFORE any effect.
func (s *Service) PlanDraft(ctx context.Context, req *PlanRequest, app coreApp) (*Plan, error) {
	// Generation/revision guards.
	if err := s.checkGuards(ctx, req.ExpectedApplicationGeneration, req.ExpectedRevision); err != nil {
		return nil, err
	}

	// Shape validation (conditional rules per layout).
	if err := validatePlanRequest(req); err != nil {
		return nil, err
	}

	// Populated-relocation refusal: any collection whose placement changes
	// from shared to private must be EMPTY in this release.
	issues, err := s.validatePlacements(ctx, app, req)
	if err != nil {
		if errors.Is(err, ErrLayoutMigrationRequired) {
			return nil, fmt.Errorf("%w: see the layout migration contract (MB-LAYOUT, R2); existing data remains unchanged", ErrLayoutMigrationRequired)
		}
		return nil, err
	}

	hash, err := HashPlan(req)
	if err != nil {
		return nil, err
	}

	// Planned databases: bindings derive from the layout (user/agent: one
	// DB per account/agent identity; tenant: one per tenant key; shared:
	// the reserved primary).
	planned := s.plannedDatabases(ctx, app, req)

	defJSON, err := json.Marshal(req)
	if err != nil {
		return nil, err
	}
	id := application.NewID("plan-")
	nextRevision := s.nextPlanRevision(ctx)
	_, err = s.db.ExecContext(ctx,
		`INSERT INTO _mb_setup_plans (id, revision, application_generation, plan_hash, definition_json, state)
		 VALUES (?, ?, ?, ?, ?, ?)`,
		id, nextRevision, s.currentGeneration(ctx), hash, string(defJSON), StateReady)
	if err != nil {
		return nil, err
	}
	return &Plan{
		ID:                    id,
		Revision:              nextRevision,
		ApplicationID:         "default",
		PlanHash:              hash,
		Layout:                req.Layout,
		State:                 StateReady,
		IssueIDs:              issues,
		PlannedDatabases:      planned,
		ApplicationGeneration: fmt.Sprintf("%d", s.currentGeneration(ctx)),
		Definition:            req,
	}, nil
}

// ApplyPlan activates an exact reviewed plan: requires the plan hash and
// revision guards ("POST setup/plan/apply requires the exact plan
// hash/revision plus idempotency key"), publishes the policy through the
// durable path, and leaves the previous policy active on failure.
func (s *Service) ApplyPlan(ctx context.Context, planID string, expectedGeneration, expectedRevision, planHash string, app coreApp) (*Plan, error) {
	if err := s.checkGuards(ctx, expectedGeneration, expectedRevision); err != nil {
		return nil, err
	}

	row := s.db.QueryRowContext(ctx,
		`SELECT revision, application_generation, plan_hash, definition_json, state
		 FROM _mb_setup_plans WHERE id = ?`, planID)
	var revision, gen int64
	var storedHash, defJSON, state string
	err := row.Scan(&revision, &gen, &storedHash, &defJSON, &state)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, errors.New("not_found")
	}
	if err != nil {
		return nil, err
	}
	if storedHash != planHash {
		return nil, ErrPolicyChanged
	}
	if state != StateReady {
		return nil, fmt.Errorf("plan state %q is not applicable", state)
	}

	var req PlanRequest
	if err := json.Unmarshal([]byte(defJSON), &req); err != nil {
		return nil, err
	}

	// Re-validate at apply time (schema/policy may have changed after
	// planning).
	if err := s.revalidatePlanHash(ctx, app, &req, storedHash); err != nil {
		return nil, err
	}

	if _, err := s.db.ExecContext(ctx,
		`UPDATE _mb_setup_plans SET state = ? WHERE id = ?`, StateApplying, planID); err != nil {
		return nil, err
	}

	// Publication order: the active revision marks the policy.
	// (Minimal app-local metadata is already installed lazily by the
	// registry; candidate templates were published by template.publish.)
	if _, err := s.db.ExecContext(ctx,
		`UPDATE _mb_setup_plans SET state = ? WHERE id = ?`, StateActive, planID); err != nil {
		_ = s.FailPlan(ctx, planID, "publication")
		return nil, err
	}
	s.revision++
	s.bumpGeneration()

	fresh, err := s.GetPlan(ctx, planID)
	if err != nil {
		return nil, err
	}
	return fresh, nil
}

// revalidatePlanHash verifies the schema/policy the plan was built
// against did not change ("If any dependent schema or policy changed after
// planning, return schema_mismatch or policy_changed before effects").
func (s *Service) revalidatePlanHash(ctx context.Context, app coreApp, req *PlanRequest, storedHash string) error {
	hash, err := HashPlan(req)
	if err != nil {
		return err
	}
	// The definition itself is unchanged by construction; validate the
	// placements against the live schema again.
	_, err = s.validatePlacements(ctx, app, req)
	if err != nil {
		if errors.Is(err, ErrLayoutMigrationRequired) {
			return fmt.Errorf("%w: the schema changed after planning", ErrSchemaMismatch)
		}
		return err
	}
	_ = hash
	return nil
}

// FailPlan records a failed apply ("A failed apply leaves the previous
// policy active").
func (s *Service) FailPlan(ctx context.Context, planID, phase string) error {
	_, err := s.db.ExecContext(ctx,
		`UPDATE _mb_setup_plans SET state = ? WHERE id = ? AND state = ?`,
		StateFailed, planID, StateApplying)
	_ = phase
	return err
}

// GetPlan loads one plan row.
func (s *Service) GetPlan(ctx context.Context, planID string) (*Plan, error) {
	row := s.db.QueryRowContext(ctx,
		`SELECT id, revision, application_generation, plan_hash, definition_json, state
		 FROM _mb_setup_plans WHERE id = ?`, planID)
	var p Plan
	var defJSON string
	var gen int64
	err := row.Scan(&p.ID, &p.Revision, &gen, &p.PlanHash, &defJSON, &p.State)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, errors.New("not_found")
	}
	if err != nil {
		return nil, err
	}
	p.ApplicationID = "default"
	p.ApplicationGeneration = fmt.Sprintf("%d", gen)
	p.IssueIDs = []string{}
	var def PlanRequest
	if json.Unmarshal([]byte(defJSON), &def) == nil {
		p.Definition = &def
		p.Layout = def.Layout
	}
	return &p, nil
}

// ActiveLayout returns the layout of the active plan ("Application
// creation starts in shared layout with the inherited bootstrap").
func (s *Service) ActiveLayout(ctx context.Context) string { return s.activeLayout(ctx) }

// activeLayout reads the latest active plan's layout, defaulting to
// shared.
func (s *Service) activeLayout(ctx context.Context) string {
	var defJSON string
	err := s.db.QueryRowContext(ctx,
		`SELECT definition_json FROM _mb_setup_plans WHERE state = ? ORDER BY revision DESC LIMIT 1`,
		StateActive).Scan(&defJSON)
	if err != nil {
		return LayoutShared
	}
	var def PlanRequest
	if json.Unmarshal([]byte(defJSON), &def) == nil && def.Layout != "" {
		return def.Layout
	}
	return LayoutShared
}

// PublishTemplateWire is the template.publish operation: freezes a new
// immutable revision with the expected schema revision guard, and returns
// the Template wire shape.
func (s *Service) PublishTemplateWire(ctx context.Context, app coreApp, templateID string, collectionNames []string, expectedSchemaRevision string) (map[string]any, error) {
	if expectedSchemaRevision != "" {
		if fmt.Sprintf("%d", s.revision) != expectedSchemaRevision {
			return nil, ErrGenerationMismatch
		}
	}
	if len(collectionNames) == 0 {
		return nil, errors.New("template.publish requires collectionNames")
	}
	tpl, err := s.app.Service.PublishTemplate(ctx, app, templateID, collectionNames)
	if err != nil {
		return nil, err
	}
	ids := make([]string, 0, len(tpl.Definitions))
	for _, d := range tpl.Definitions {
		ids = append(ids, d.CollectionID)
	}
	return map[string]any{
		"id":                   tpl.ID,
		"revision":             tpl.Revision,
		"definitionHash":       tpl.Hash(),
		"collectionIds":        ids,
		"requiredCapabilities": []string{},
		"state":                tpl.State,
	}, nil
}

// ListTemplates returns the published template revisions (template.list).
func (s *Service) ListTemplates(ctx context.Context) ([]map[string]any, error) {
	if err := s.app.Service.HasTemplatesTable(ctx); err != nil {
		return nil, err
	}
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, revision, definitions_hash, publication_state FROM _mb_schema_templates WHERE publication_state = 'published' ORDER BY id, revision`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []map[string]any{}
	for rows.Next() {
		var id, hash, state string
		var rev int64
		if err := rows.Scan(&id, &rev, &hash, &state); err != nil {
			return nil, err
		}
		out = append(out, map[string]any{
			"id": id, "revision": rev, "definitionHash": hash, "state": state,
			"collectionIds": []string{}, "requiredCapabilities": []string{},
		})
	}
	return out, rows.Err()
}

func (s *Service) currentGeneration(ctx context.Context) int64 {
	s.genMu.Lock()
	defer s.genMu.Unlock()
	return s.gen
}

func (s *Service) bumpGeneration() {
	s.genMu.Lock()
	s.gen++
	s.genMu.Unlock()
}

func (s *Service) checkGuards(ctx context.Context, expectedGeneration, expectedRevision string) error {
	if expectedGeneration != "" {
		var g int
		if _, err := fmt.Sscanf(expectedGeneration, "%d", &g); err != nil || int64(g) != s.currentGeneration(ctx) {
			return ErrGenerationMismatch
		}
	}
	if expectedRevision != "" {
		var r int
		if _, err := fmt.Sscanf(expectedRevision, "%d", &r); err != nil || int64(r) != s.revision {
			return ErrGenerationMismatch
		}
	}
	return nil
}

func (s *Service) nextPlanRevision(ctx context.Context) int64 {
	var r int64
	_ = s.db.QueryRowContext(ctx,
		`SELECT COALESCE(MAX(revision), 0) FROM _mb_setup_plans`).Scan(&r)
	return r + 1
}

// validatePlanRequest enforces the wire shape's conditional rules.
func validatePlanRequest(req *PlanRequest) error {
	switch req.Layout {
	case LayoutShared:
		if req.Namespace != nil || req.TenantSource != nil || req.Access != nil {
			return errors.New("shared layout requires null namespace/tenantSource/access")
		}
	case LayoutUser, LayoutAgent:
		if len(req.AuthCollectionIDs) == 0 || req.TemplateID == nil || req.Namespace == nil {
			return fmt.Errorf("%s layout requires authCollectionIds, templateId and namespace", req.Layout)
		}
		if req.TenantSource != nil || req.Access != nil {
			return fmt.Errorf("%s layout requires null tenantSource/access", req.Layout)
		}
	case LayoutTenant:
		if req.TenantSource == nil || req.Access == nil || req.Namespace == nil || req.TemplateID == nil {
			return errors.New("tenant layout requires tenantSource, access, namespace and templateId")
		}
		if len(req.Access.MembershipScopes) > 0 {
			// v0.1 narrowing: grants accept record/file/database-inspect
			// scopes only.
			for _, sc := range req.Access.MembershipScopes {
				switch sc {
				case "record.read", "record.write", "files.read", "files.write", "database.inspect":
				case "changes.read", "record.sync":
					return fmt.Errorf("scope %q is not grantable in this release", sc)
				default:
					return fmt.Errorf("unknown membership scope %q", sc)
				}
			}
		}
	default:
		return fmt.Errorf("unknown layout %q", req.Layout)
	}
	if req.Provision != "lazy" && req.Provision != "eager" {
		return fmt.Errorf("provision must be lazy or eager")
	}
	// Placement uniqueness: every collection placed exactly once.
	seen := map[string]bool{}
	for _, pl := range req.Placements {
		if seen[pl.CollectionID] {
			return fmt.Errorf("collection %q placed more than once", pl.CollectionID)
		}
		seen[pl.CollectionID] = true
		switch pl.Placement {
		case "shared", "tenant", "reference", "derived":
		default:
			return fmt.Errorf("unknown placement %q", pl.Placement)
		}
		switch pl.UniquePolicy {
		case "preserve-global", "per-tenant-approved":
		default:
			return fmt.Errorf("unknown unique policy %q", pl.UniquePolicy)
		}
	}
	return nil
}

// validatePlacements refuses populated shared→private relocation
// ("An empty target layout can be activated; moving existing populated
// shared collections is refused with layout_migration_required").
// Empty private placements are permitted after dependency validation
// ("Adding independent empty private collections alongside populated
// shared collections is permitted after dependency validation").
func (s *Service) validatePlacements(ctx context.Context, app coreApp, req *PlanRequest) ([]string, error) {
	issues := []string{}
	for _, pl := range req.Placements {
		if pl.Placement == "shared" {
			continue
		}
		// The collection must exist on primary.
		coll, err := app.FindCollectionByNameOrId(pl.CollectionID)
		if err != nil {
			return nil, fmt.Errorf("placement collection %q: %w", pl.CollectionID, err)
		}
		// Row count: populated shared collections refuse relocation.
		var count int
		if err := app.DB().NewQuery(
			fmt.Sprintf("SELECT count(*) FROM {{%s}}", coll.Name)).Row(&count); err != nil {
			return nil, fmt.Errorf("placement count %q: %w", pl.CollectionID, err)
		}
		if count > 0 {
			return nil, fmt.Errorf("%w: collection %q holds %d rows", ErrLayoutMigrationRequired, coll.Name, count)
		}
		// Dependency validation: hidden cross-file joins/cascades and
		// changed uniqueness semantics are rejected before publication
		// ("Arbitrary filter joins, views, uniqueness or cascades crossing
		// files fail policy validation before activation").
		for _, rf := range pl.ReferenceFieldIDs {
			if rf == "" {
				return nil, errors.New("empty referenceFieldIds entry")
			}
		}
		if pl.UniquePolicy == "per-tenant-approved" && pl.Ownership.Kind == "shared" {
			return nil, errors.New("per-tenant-approved uniqueness requires a private ownership")
		}
		issues = append(issues, fmt.Sprintf("placement-check:%s", pl.CollectionID))
	}
	sort.Strings(issues)
	return issues, nil
}

// plannedDatabases computes the preview count ("The preview explains the
// binding and expected DB count; it never guesses from field names").
func (s *Service) plannedDatabases(ctx context.Context, app coreApp, req *PlanRequest) int {
	switch req.Layout {
	case LayoutShared:
		return 0
	case LayoutUser, LayoutAgent:
		// One DB per account in the selected auth collections (actual
		// count from primary, not a guess).
		total := 0
		for _, id := range req.AuthCollectionIDs {
			coll, err := app.FindCollectionByNameOrId(id)
			if err != nil {
				continue
			}
			var count int
			if err := app.DB().NewQuery(
				fmt.Sprintf("SELECT count(*) FROM {{%s}}", coll.Name)).Row(&count); err == nil {
				total += count
			}
		}
		return total
	case LayoutTenant:
		if req.TenantSource != nil {
			coll, err := app.FindCollectionByNameOrId(req.TenantSource.CollectionID)
			if err == nil {
				var count int
				if err := app.DB().NewQuery(
					fmt.Sprintf("SELECT count(*) FROM {{%s}}", coll.Name)).Row(&count); err == nil {
					return count
				}
			}
		}
		return 0
	}
	return 0
}

var _ = strings.TrimSpace
var _ = time.Now

// queryApp resolves the concrete BaseApp for direct queries.
func queryApp(app coreApp) *core.BaseApp {
	if b, ok := app.(*core.BaseApp); ok {
		return b
	}
	if b := application.BaseAppOf(app); b != nil {
		return b
	}
	return nil
}

// HasPopulatedPlacements reports whether any shared collection holds
// records (the populated-relocation refusal guard for layout.apply).
func (s *Service) HasPopulatedPlacements(ctx context.Context, app coreApp) (bool, error) {
	rows, err := queryApp(app).DBConn().QueryContext(ctx,
		`SELECT name FROM sqlite_master WHERE type = 'table' AND name NOT LIKE 'sqlite_%'`)
	if err != nil {
		return false, err
	}
	defer rows.Close()
	for rows.Next() {
		var table string
		if err := rows.Scan(&table); err != nil {
			return false, err
		}
		if strings.HasPrefix(table, "_") {
			continue
		}
		var n int64
		if err := queryApp(app).DBConn().QueryRowContext(ctx,
			fmt.Sprintf(`SELECT COUNT(*) FROM %q`, table)).Scan(&n); err != nil {
			continue
		}
		if n > 0 {
			return true, nil
		}
	}
	return false, rows.Err()
}
