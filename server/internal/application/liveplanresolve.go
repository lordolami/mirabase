package application

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"mirabase/core"
	"mirabase/internal/livequery"
)

// ---------------------------------------------------------------------------
// D-14 live-query plan resolution (live.register, MB-LIVE): the Runtime
// publishes the R6 live-plan store (derived.go `_mb_live_plans`) as the
// livequery.PlanResolver implementation. Ordinary live.register resolves a
// PUBLISHED query id/revision: draft and retired revisions never stream,
// and a superuser-only draft cannot be exposed as an ordinary record
// subscription (implementation-closure.md, Live-query definitions).
//
// This file also implements livequery.PolicyBinder: the plan's stored
// policy plan (its verified dependency collections) is instantiated under
// the ACTOR'S CURRENT rules at every registration/evaluation — the
// collection's list rule compiles through the parsed filter machinery
// (BuildRulePredicate) and injects as a parameterized EXISTS predicate;
// a plan whose dependency collection is not readable by the actor is
// refused (a privileged SQL plan cannot be exposed as an ordinary record
// subscription).
//
// The dependency direction is application → livequery (never the
// reverse); derived.go is untouched by this file.
// ---------------------------------------------------------------------------

// ErrLivePlanUnpublished reports a plan revision that is not published
// (draft or retired). Wire mapping: invalid_request (the caller referenced
// a query that is not streamable), while a fully unknown query id maps to
// not_found.
var ErrLivePlanUnpublished = errors.New("liveplan_unpublished")

// ErrLivePlanNotAuthorized reports a plan that cannot be instantiated
// under the presenting actor's current rules (the dependency collection is
// not readable by the actor, or the plan's shape cannot carry the
// per-record rule correlation). Wire mapping: forbidden — "A privileged
// SQL plan cannot be exposed as an ordinary record subscription"
// (implementation-closure.md, Live-query definitions).
var ErrLivePlanNotAuthorized = errors.New("liveplan_not_authorized")

// ResolvePublishedLivePlan implements livequery.PlanResolver over the
// live-plan store: revision 0 selects the LATEST published revision; a
// positive revision selects the exact PUBLISHED revision. Draft or
// retired revisions return ErrLivePlanUnpublished; an id with no revision
// at all returns ErrNotFound.
func (rt *Runtime) ResolvePublishedLivePlan(ctx context.Context, planID string, revision int64) (*livequery.Plan, error) {
	if err := rt.Service.ensureDerivedTables(ctx); err != nil {
		return nil, err
	}
	var p LivePlanWire
	var paramsJSON, depsJSON, shapeJSON, policyJSON string
	var q *sql.Row
	if revision <= 0 {
		q = rt.PrimaryDB.QueryRowContext(ctx,
			`SELECT id, revision, sql, parameters_json, result_key, dependencies_json, max_rows, max_bytes, state, result_shape_json, policy_json, creator
			 FROM _mb_live_plans WHERE id = ? AND state = 'published' ORDER BY revision DESC LIMIT 1`, planID)
	} else {
		q = rt.PrimaryDB.QueryRowContext(ctx,
			`SELECT id, revision, sql, parameters_json, result_key, dependencies_json, max_rows, max_bytes, state, result_shape_json, policy_json, creator
			 FROM _mb_live_plans WHERE id = ? AND revision = ? AND state = 'published'`, planID, revision)
	}
	err := q.Scan(&p.ID, &p.Revision, &p.SQL, &paramsJSON, &p.ResultKey, &depsJSON, &p.MaxRows, &p.MaxBytes, &p.State,
		&shapeJSON, &policyJSON, &p.Creator)
	if errors.Is(err, sql.ErrNoRows) {
		// Distinguish an unpublished (draft/retired) revision from a
		// wholly unknown query id.
		var revisions int64
		if lerr := rt.PrimaryDB.QueryRowContext(ctx,
			`SELECT COUNT(*) FROM _mb_live_plans WHERE id = ?`, planID).Scan(&revisions); lerr == nil && revisions > 0 {
			return nil, fmt.Errorf("%w: the plan revision of %q is not published", ErrLivePlanUnpublished, planID)
		}
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	_ = json.Unmarshal([]byte(paramsJSON), &p.Parameters)
	_ = json.Unmarshal([]byte(depsJSON), &p.Dependencies)
	_ = json.Unmarshal([]byte(shapeJSON), &p.ResultShape)
	var storedPolicy []livequery.PlanPolicy
	_ = json.Unmarshal([]byte(policyJSON), &storedPolicy)
	return &livequery.Plan{
		ID:           p.ID,
		Revision:     p.Revision,
		SQL:          p.SQL,
		Parameters:   p.Parameters,
		ResultKey:    p.ResultKey,
		Dependencies: p.Dependencies,
		MaxRows:      p.MaxRows,
		MaxBytes:     p.MaxBytes,
		Shape:        p.ResultShape,
		Policy:       storedPolicy,
		Creator:      p.Creator,
	}, nil
}

// BindLivePlanPolicy implements livequery.PolicyBinder: it resolves the
// actor's CURRENT authorization over the plan's dependency collections
// and compiles the injected rule predicate.
//
//   - a superuser actor streams the plan unfiltered (the plan's own
//     policy — verified tables/fields — still applies);
//   - a plan with no stored policy plan is refused (the revision cannot
//     prove what it reads — never streamed);
//   - a dependency collection whose list rule is nil is superuser-only:
//     an ordinary actor is refused (forbidden), never streamed
//     unfiltered — a privileged SQL plan cannot be exposed as an
//     ordinary record subscription;
//   - a collection with a list rule injects the rule as a parameterized
//     EXISTS predicate correlated on the plan's result key (which must
//     bind the record id for the correlation to be the record's own
//     rule);
//   - plans the parsed-filter planner cannot narrow per row for an
//     ordinary actor are refused (fail closed): multi-collection reads
//     (no single record id carries every dependency) and aggregate
//     result shapes (the aggregated value derives from rows the actor
//     may not see, which a per-row predicate cannot recompute).
func (rt *Runtime) BindLivePlanPolicy(ctx context.Context, plan *livequery.Plan, actorCollectionID, actorRecordID string) (string, []any, error) {
	if len(plan.Policy) == 0 {
		return "", nil, fmt.Errorf("%w: the plan revision carries no verified policy plan", ErrLivePlanUnpublished)
	}
	// Superuser authority: the plan is the superuser's own privileged
	// surface and streams unfiltered for them.
	if isSuperuserCollectionID(actorCollectionID) || actorCollectionID == core.CollectionNameSuperusers {
		return "", nil, nil
	}
	if len(plan.Policy) > 1 {
		return "", nil, fmt.Errorf("%w: the plan reads %d collections and cannot be instantiated under a single collection's rules", ErrLivePlanNotAuthorized, len(plan.Policy))
	}
	// Aggregate/window result shapes derive values from rows the actor's
	// rules may exclude; the parsed-filter planner compiles row rules and
	// cannot recompute an aggregate over the actor's visible subset. The
	// scan is token-based and conservative (a deterministic query that
	// merely contains such a word inside a string literal is also
	// refused) — a refusal, never an unfiltered stream.
	lowerSQL := strings.ToLower(plan.SQL)
	for _, tok := range []string{
		"group by", "having", "distinct", "count(", "sum(", "avg(",
		"min(", "max(", "total(", "group_concat(", " over (", "over(",
	} {
		if strings.Contains(lowerSQL, tok) {
			return "", nil, fmt.Errorf("%w: the plan's aggregate result shape cannot be narrowed to the actor's authorized rows", ErrLivePlanNotAuthorized)
		}
	}
	entry := plan.Policy[0]
	coll, cerr := rt.Base.FindCollectionByNameOrId(entry.Collection)
	if cerr != nil {
		return "", nil, fmt.Errorf("%w: the plan's dependency collection %q no longer exists", ErrLivePlanNotAuthorized, entry.Collection)
	}
	// The actor record must still exist (revocation deletes terminate
	// instantiations); the request context carries it for @request.auth
	// references in the rule.
	authRec, rerr := rt.Base.FindRecordById(actorCollectionID, actorRecordID)
	if rerr != nil {
		return "", nil, fmt.Errorf("%w: the actor record is not readable in %q", ErrLivePlanNotAuthorized, actorCollectionID)
	}
	// A dependency collection without a list rule is superuser-only under
	// the ordinary record rules: instantiating the plan for an ordinary
	// actor would stream rows the actor's rules never admit.
	listRule := coll.ListRule
	if listRule == nil || strings.TrimSpace(*listRule) == "" {
		return "", nil, fmt.Errorf("%w: the dependency collection %q grants no list rule to ordinary actors", ErrLivePlanNotAuthorized, coll.Name)
	}
	// The correlation binds the result key to the record identity: a
	// result key other than the record id cannot carry the per-record
	// rule correlation.
	if plan.ResultKey != "id" {
		return "", nil, fmt.Errorf("%w: the plan's result key %q does not bind the record id, so the actor's rules cannot be injected", ErrLivePlanNotAuthorized, plan.ResultKey)
	}
	info := &core.RequestInfo{Auth: authRec}
	predicate, predArgs, perr := BuildRulePredicate(rt.Base, coll, *listRule, info)
	if perr != nil {
		return "", nil, fmt.Errorf("%w: the actor's rules cannot be injected for %q: %v", ErrLivePlanNotAuthorized, coll.Name, perr)
	}
	return predicate, predArgs, nil
}

// PublishLivePlan publishes one draft revision after verifying the stored
// SQL against the ACTUAL TARGET PROFILES it will be evaluated on
// (implementation-closure.md: "SQL must prepare read-only in the actual
// profile" — the application primary alone is not the evaluation profile).
// For every ready managed target that carries all declared dependency
// collections, the plan must
//
//   - re-validate as a deterministic read-only SELECT (the stored SQL is
//     the published artifact; a tampered or legacy row never publishes),
//   - prepare read-only on that target's real connection, and
//   - touch exactly the declared tables on that target (the engine's own
//     EXPLAIN QUERY PLAN — never a keyword prefix).
//
// A target that does not carry the dependency collections cannot run the
// plan and is skipped (the plan is for other profiles); when NO ready
// target carries the dependencies, the creation-time verification stands
// and publication proceeds — such targets may still be provisioned later,
// and any per-target failure surfaces as a typed error BEFORE a
// subscription is allocated at live.register. A failure on a carrying
// target refuses publication with invalid_request.
func (rt *Runtime) PublishLivePlan(ctx context.Context, id string, revision int64) (*LivePlanWire, error) {
	plan, err := rt.Service.GetLivePlan(ctx, id, revision)
	if err != nil {
		return nil, err
	}
	if err := validateReadOnlySQL(plan.SQL); err != nil {
		return nil, err
	}
	// The ready targets of the application (database.list reads the
	// registry; the handle cache opens the actual profile connections).
	// The registry is paginated: walk every page so no ready target is
	// skipped by the verification.
	var dbs []*Database
	for page := 1; ; page++ {
		items, total, lerr := rt.Service.ListDatabases(ctx, page, 1000)
		if lerr != nil {
			return nil, lerr
		}
		dbs = append(dbs, items...)
		if int64(len(dbs)) >= total || len(items) == 0 {
			break
		}
	}
	for _, db := range dbs {
		if db.State != StateReady {
			continue // a non-ready target is not an evaluation profile
		}
		handle, herr := rt.Handles.Open(ctx, db.ID)
		if herr != nil {
			return nil, fmt.Errorf("%w: the target profile of database %q could not be opened: %v", ErrInvalidPlan, db.ID, herr)
		}
		// Skip targets that cannot run the plan: a dependency collection
		// absent from the target's schema means the plan is not for this
		// profile (an instantiation there fails typed, before allocation).
		carries := true
		for _, dep := range plan.Dependencies {
			coll, cerr := rt.Base.FindCollectionByNameOrId(dep)
			if cerr != nil {
				return nil, fmt.Errorf("%w: the dependency collection %q no longer exists", ErrInvalidPlan, dep)
			}
			cols, terr := tableColumns(ctx, handle, coll.Name)
			if terr != nil {
				return nil, fmt.Errorf("%w: the target profile of database %q could not be read: %v", ErrInvalidPlan, db.ID, terr)
			}
			if len(cols) == 0 {
				carries = false
				break
			}
		}
		if !carries {
			continue
		}
		// The plan must prepare read-only on this real profile and touch
		// only the declared tables there.
		accessed, aerr := planAccessedTables(ctx, handle, plan.SQL, len(plan.Parameters))
		if aerr != nil {
			return nil, fmt.Errorf("%w: the plan revision does not prepare read-only in the target profile of database %q: %v", ErrInvalidPlan, db.ID, aerr)
		}
		declared := map[string]bool{}
		for _, dep := range plan.Dependencies {
			if coll, cerr := rt.Base.FindCollectionByNameOrId(dep); cerr == nil {
				declared[coll.Name] = true
			}
		}
		for t := range accessed {
			if strings.HasPrefix(t, "_") || strings.HasPrefix(t, "sqlite_") {
				return nil, fmt.Errorf("%w: the plan revision reads the protected table %q in the target profile of database %q", ErrInvalidPlan, t, db.ID)
			}
			if !declared[t] {
				return nil, fmt.Errorf("%w: the plan revision reads the undeclared table %q in the target profile of database %q", ErrInvalidPlan, t, db.ID)
			}
		}
		stmt, perr := handle.PrepareContext(ctx, "EXPLAIN "+plan.SQL)
		if perr != nil {
			return nil, fmt.Errorf("%w: the plan revision does not prepare read-only in the target profile of database %q: %v", ErrInvalidPlan, db.ID, perr)
		}
		stmt.Close()
	}
	return rt.Service.PublishLivePlan(ctx, id, revision)
}
