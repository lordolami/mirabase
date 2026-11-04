// Package livequery implements the D-14 live query service
// (docs/mirabase/integration-contracts.md#d-14): durable query-result
// subscriptions over published live plans, evaluated as a real SQL
// snapshot at a captured change-journal boundary, with keyed ordered
// deltas, bounded durable replay and per-step authorization rechecks.
//
// The package is deliberately engine-facing only: it works over
// database/sql handles opened by the caller and resolves plans through
// the PlanResolver interface. The dependency direction is
// application → livequery (never the reverse): the application package
// publishes its live-plan registry as a PlanResolver implementation.
package livequery

import (
	"context"
	"database/sql"
)

// PlanPolicy is one stored policy-plan entry of a published plan: the
// dependency collection the plan reads and the collection's hidden fields
// (which may never leave the evaluation; enforced at plan creation and
// re-checked through the actor's current rules at instantiation).
type PlanPolicy struct {
	Collection string   `json:"collection"`
	Hidden     []string `json:"hidden,omitempty"`
}

// Plan is one published live plan the resolver returns. The declared
// parameters bind positionally ('?' markers in declared order); a missing
// request parameter binds the declared default (or NULL). The plan is
// already validated by its publisher: read-only, deterministic, bounded
// (max rows/bytes), carrying a unique non-null result key column, a
// stored result shape and the collection/field policy plan the evaluator
// injects (D-14: "Inject collection/field policy using the shared query
// planner").
type Plan struct {
	ID           string
	Revision     int64
	SQL          string
	Parameters   []map[string]any
	ResultKey    string
	Dependencies []string
	MaxRows      int
	MaxBytes     int
	// Shape is the verified result column list (checked against every
	// evaluation; a drift is a policy error, never a silent re-shape).
	Shape []string
	// Policy is the stored policy plan (dependency collections + their
	// hidden fields).
	Policy []PlanPolicy
	// Creator is the publishing application-local subject (informational;
	// only an application superuser can create/publish).
	Creator string
}

// PolicyBinder is implemented by resolvers that can inject the ACTOR'S
// CURRENT collection/field policy into a plan evaluation (D-14:
// "Ordinary users instantiate approved queries in their authorized
// context ... instantiates it under the actor's current rules"). The
// returned predicate is a parameterized SQL fragment (? placeholders +
// positional args) correlated on the plan's result key; an error refuses
// the instantiation (forbidden).
type PolicyBinder interface {
	BindLivePlanPolicy(ctx context.Context, plan *Plan, actorCollectionID, actorRecordID string) (predicate string, args []any, err error)
}

// PlanResolver resolves one PUBLISHED plan revision. Draft and retired
// revisions never stream: the resolver must refuse them. revision == 0
// selects the latest published revision; a positive revision selects the
// exact published revision.
type PlanResolver interface {
	ResolvePublishedLivePlan(ctx context.Context, planID string, revision int64) (*Plan, error)
}

// Wire event kinds for the live stream (live.stream eventKinds; no other
// kinds exist — a result-position change folds into an update).
const (
	KindSnapshot = "snapshot"
	KindInsert   = "insert"
	KindUpdate   = "update"
	KindDelete   = "delete"
	KindReset    = "reset"
	KindError    = "error"
)

// RegisterInput is one live.register request body resolved to its
// authorized database context. The handle is the caller-opened target
// database; the generation/schema/policy values are the database row's
// CURRENT registry values.
type RegisterInput struct {
	DatabaseID string
	Handle     *sql.DB
	QueryID    string // the published plan id
	// PlanRevision pins one plan revision; 0 = latest published.
	PlanRevision int64
	// Parameters are the request's positional TypedValues.
	Parameters        []any
	ActorCollectionID string
	ActorRecordID     string
	Generation        int64
	SchemaRevision    int64
	PolicyEpoch       int64
}

// Registration is the register result (the cursor token binds the
// subscription, actor, plan/revision/query hash, context generation and
// the captured sequence).
type Registration struct {
	ID             string
	DatabaseID     string
	QueryID        string
	PlanRevision   int64
	Generation     int64
	SchemaRevision int64
	PolicyEpoch    int64
	Cursor         string
	Sequence       int64
	ExpiresAt      string
}

// StepInput is one stream step against the current authorized context.
// The caller re-reads the database row (generation/schema/policy) and
// re-runs its authorization before each step.
type StepInput struct {
	DatabaseID        string
	Handle            *sql.DB
	SubscriptionID    string
	QueryID           string // the stream path's plan id (cross-checked)
	Cursor            string
	ActorCollectionID string
	ActorRecordID     string
	Generation        int64
	SchemaRevision    int64
	PolicyEpoch       int64
}

// Event is one wire frame of the stream (kind + ready-to-encode payload).
type Event struct {
	Kind string
	Data map[string]any
}

// StepResult is one step's ordered frames plus the boundary the caller
// continues from. Terminated reports that the subscription is closed
// (cancelled or its plan retired) and the stream must end after writing.
type StepResult struct {
	Events     []Event
	Cursor     string
	Sequence   int64
	Terminated bool
}

// CancelInput identifies the actor's subscription of one query id. The
// subscription is addressed exactly by SubscriptionID or by the
// registration's Parameters; without either, a single active subscription
// cancels while several active ones are refused as ambiguous (an actor
// may hold multiple subscriptions of the same plan with different
// parameter sets).
type CancelInput struct {
	DatabaseID        string
	Handle            *sql.DB
	QueryID           string
	ActorCollectionID string
	ActorRecordID     string
	SubscriptionID    string
	Parameters        []any
}

// CancelResult is the idempotent cancel response ({id, closed:true}).
type CancelResult struct {
	ID     string
	Closed bool
}
