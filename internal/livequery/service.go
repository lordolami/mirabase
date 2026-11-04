package livequery

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"time"
)

// ---------------------------------------------------------------------------
// Service state, durable tables, Register / Step / Cancel
// (docs/mirabase/integration-contracts.md D-14 and the streaming section
// of docs/mirabase/realtime-and-cdc.md).
// ---------------------------------------------------------------------------

// Contract defaults (D-14): "Default bounds are 1000 rows and 8 MiB per
// subscription" (plan-declared and DDL-checked at plan creation; the
// service enforces them per evaluation) and "Default replay lease is five
// minutes/8 MiB" (retained replay is pruned by lease and byte budget).
const (
	ReplayLease     = 5 * time.Minute
	ReplayMaxBytes  = 8 << 20 // 8 MiB
	DefaultMaxBytes = 8 << 20
	// SubscriptionTTL is the v0.1 default subscription lease; every step
	// refreshes it once it is half spent (the configuration knob is the
	// roadmap remainder).
	SubscriptionTTL = 30 * time.Minute
)

// Errors surface as documented operation failures: not_found, forbidden,
// invalid_request (cursor/request), resource_exhausted, schema_mismatch /
// policy_changed (context reset), generation_mismatch.
var (
	ErrNotFound        = errors.New("livequery: subscription not found")
	ErrForbidden       = errors.New("livequery: the subscription belongs to another actor")
	ErrInvalidCursor   = errors.New("livequery: invalid or unauthentic cursor")
	ErrInvalidRequest  = errors.New("livequery: invalid request")
	ErrCancelled       = errors.New("livequery: subscription cancelled")
	ErrPlanUnpublished = errors.New("livequery: the plan revision is not published")
	ErrPlanRetired     = errors.New("livequery: the plan revision was retired")
	ErrExhausted       = errors.New("livequery: subscription bounds exceeded")
	ErrResultKey       = errors.New("livequery: the result key is null or not unique")
	// ErrShapeMismatch reports an evaluation whose result columns no
	// longer match the published policy shape (schema/policy drift).
	ErrShapeMismatch = errors.New("livequery: the plan result shape no longer matches the published policy")
)

// Durable subscription and replay tables (per target database — the same
// file the change journal lives in, so the journal head and the snapshot
// rows are one consistent read scope).
const subscriptionsDDL = `CREATE TABLE IF NOT EXISTS _mb_live_subscriptions (
 id TEXT PRIMARY KEY,
 database_id TEXT NOT NULL,
 plan_id TEXT NOT NULL,
 plan_revision INTEGER NOT NULL CHECK(plan_revision > 0),
 query_hash TEXT NOT NULL,
 actor_collection_id TEXT NOT NULL,
 actor_record_id TEXT NOT NULL,
 state TEXT NOT NULL CHECK(state IN ('active','cancelled')),
 params_json TEXT NOT NULL CHECK(json_valid(params_json)),
 generation INTEGER NOT NULL CHECK(generation > 0),
 schema_revision INTEGER NOT NULL CHECK(schema_revision > 0),
 policy_epoch INTEGER NOT NULL CHECK(policy_epoch > 0),
 head_seq INTEGER NOT NULL CHECK(head_seq >= 0),
 expires_at TEXT NOT NULL,
 created TEXT NOT NULL,
 updated TEXT NOT NULL
)`

const subscriptionIndexDDL = `CREATE INDEX IF NOT EXISTS _mb_live_subscriptions_query
 ON _mb_live_subscriptions(database_id, plan_id, actor_collection_id, actor_record_id, state)`

const replayDDL = `CREATE TABLE IF NOT EXISTS _mb_live_replay (
 subscription_id TEXT NOT NULL,
 to_seq INTEGER NOT NULL CHECK(to_seq > 0),
 from_seq INTEGER NOT NULL CHECK(to_seq > from_seq),
 patch_json BLOB NOT NULL CHECK(json_valid(patch_json)),
 bytes INTEGER NOT NULL CHECK(bytes > 0),
 created TEXT NOT NULL,
 PRIMARY KEY(subscription_id, to_seq)
)`

// subRow is one durable subscription row.
type subRow struct {
	ID           string
	DatabaseID   string
	PlanID       string
	PlanRevision int64
	QueryHash    string
	ActorCol     string
	ActorRec     string
	State        string
	ParamsJSON   string
	Generation   int64
	SchemaRev    int64
	PolicyEpoch  int64
	HeadSeq      int64
	ExpiresAt    string
	Created      string
}

// subState is the in-memory live state of one subscription: the current
// keyed result rows and the sequence they reflect (the register snapshot
// head or the last replay-patch boundary). The result rows are NOT
// persisted — after a process restart the next step resets with a fresh
// snapshot ("After process restart, reset if prior replay state was not
// persisted"); the durable replay chain persists and restarts with it.
type subState struct {
	mu   sync.Mutex
	rows []liveRow
	seq  int64
}

// Service keeps the current result states and bounded replay patches in
// the shared per-process service.
type Service struct {
	mu      sync.Mutex
	subs    map[string]*subState
	secrets map[string][]byte
}

// NewService builds the shared live-query service.
func NewService() *Service {
	return &Service{subs: map[string]*subState{}, secrets: map[string][]byte{}}
}

// state returns (creating if absent) the in-memory state of one
// subscription.
func (s *Service) state(id string) *subState {
	s.mu.Lock()
	defer s.mu.Unlock()
	st, ok := s.subs[id]
	if !ok {
		st = &subState{}
		s.subs[id] = st
	}
	return st
}

// replaceState installs a fresh result state.
func (s *Service) replaceState(id string, rows []liveRow, seq int64) *subState {
	s.mu.Lock()
	defer s.mu.Unlock()
	st := &subState{rows: rows, seq: seq}
	s.subs[id] = st
	return st
}

// dropState forgets one subscription's in-memory state.
func (s *Service) dropState(id string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.subs, id)
}

// ensureLiveTables installs the durable stores on the target.
func ensureLiveTables(ctx context.Context, handle *sql.DB) error {
	for _, ddl := range []string{subscriptionsDDL, subscriptionIndexDDL, replayDDL, metaDDL} {
		if _, err := handle.ExecContext(ctx, ddl); err != nil {
			return fmt.Errorf("failed to install the live-query store: %w", err)
		}
	}
	return nil
}

// newSubscriptionID generates a server-side opaque subscription id.
func newSubscriptionID() string {
	b := make([]byte, 12)
	_, _ = rand.Read(b)
	return "lq" + hex.EncodeToString(b)
}

// queryHashOf binds the plan id, revision and parameters (a repeat
// registration with different parameters is a DIFFERENT subscription).
func queryHashOf(plan *Plan, params []any) string {
	b, err := json.Marshal([]any{plan.ID, plan.Revision, params})
	if err != nil {
		b = []byte(fmt.Sprintf("%v|%v|%v", plan.ID, plan.Revision, params))
	}
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}

// policyPredicate resolves the actor's CURRENT-rules predicate through
// the resolver's optional PolicyBinder (D-14: "instantiates it under the
// actor's current rules"). Every registration/evaluation with a binding
// resolver goes through the binder — an empty stored policy is the
// BINDER's refusal ("the plan revision carries no verified policy plan"),
// never an unfiltered stream; only resolvers without a binder (mechanics
// tests) evaluate without injection.
func policyPredicate(ctx context.Context, resolver PlanResolver, plan *Plan, actorCollectionID, actorRecordID string) (string, []any, error) {
	binder, ok := resolver.(PolicyBinder)
	if !ok {
		return "", nil, nil
	}
	return binder.BindLivePlanPolicy(ctx, plan, actorCollectionID, actorRecordID)
}

// Register instantiates one published plan for the actor's authorized
// context. The change-journal head is captured INSIDE the same read
// transaction BEFORE the snapshot rows are read, so no committed change
// can fall between the captured boundary and the snapshot (the
// subscribe/query race closure: commits landing during evaluation carry
// sequences greater than the captured boundary and are picked up by the
// next step).
func (s *Service) Register(ctx context.Context, resolver PlanResolver, in *RegisterInput) (*Registration, error) {
	if resolver == nil {
		return nil, fmt.Errorf("%w: the plan resolver is unavailable", ErrInvalidRequest)
	}
	if in == nil || in.Handle == nil || in.QueryID == "" || in.ActorRecordID == "" {
		return nil, fmt.Errorf("%w: database, query and a valid record authorization are required", ErrInvalidRequest)
	}
	if len(in.Parameters) > 100 {
		return nil, fmt.Errorf("%w: at most 100 parameters are accepted", ErrInvalidRequest)
	}
	plan, err := resolver.ResolvePublishedLivePlan(ctx, in.QueryID, in.PlanRevision)
	if err != nil {
		return nil, err
	}
	args, err := bindParams(plan, in.Parameters)
	if err != nil {
		return nil, err
	}
	if plan.MaxRows < 1 || plan.MaxRows > 1000 {
		return nil, fmt.Errorf("%w: the plan's maxRows bound is invalid", ErrInvalidRequest)
	}
	predicate, predArgs, err := policyPredicate(ctx, resolver, plan, in.ActorCollectionID, in.ActorRecordID)
	if err != nil {
		return nil, err
	}
	queryHash := queryHashOf(plan, in.Parameters)
	actor := actorOf(in.ActorCollectionID, in.ActorRecordID)
	if err := ensureLiveTables(ctx, in.Handle); err != nil {
		return nil, err
	}
	// Idempotent repeat registration: same database + actor + plan
	// revision + parameters → the SAME active subscription.
	reg, ok, err := s.reissue(ctx, in, plan, queryHash, actor, args, predicate, predArgs)
	if ok || err != nil {
		return reg, err
	}
	// Fresh registration: one consistent read transaction — head first,
	// then the bounded snapshot.
	tx, err := in.Handle.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	committed := false
	defer func() {
		if !committed {
			_ = tx.Rollback()
		}
	}()
	head, err := readJournalHead(ctx, tx)
	if err != nil {
		return nil, err
	}
	rows, err := evalSnapshot(ctx, tx, plan, args, predicate, predArgs)
	if err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	committed = true

	id := newSubscriptionID()
	now := nowRFC3339()
	expires := time.Now().UTC().Add(SubscriptionTTL).Format(time.RFC3339Nano)
	paramsJSON, _ := json.Marshal(in.Parameters)
	if _, err := in.Handle.ExecContext(ctx,
		`INSERT INTO _mb_live_subscriptions
		 (id, database_id, plan_id, plan_revision, query_hash, actor_collection_id, actor_record_id,
		  state, params_json, generation, schema_revision, policy_epoch, head_seq, expires_at, created, updated)
		 VALUES (?, ?, ?, ?, ?, ?, ?, 'active', ?, ?, ?, ?, ?, ?, ?, ?)`,
		id, in.DatabaseID, plan.ID, plan.Revision, queryHash, in.ActorCollectionID, in.ActorRecordID,
		string(paramsJSON), in.Generation, in.SchemaRevision, in.PolicyEpoch, head, expires, now, now); err != nil {
		return nil, err
	}
	s.replaceState(id, rows, head)
	secret, err := s.cursorSecretCached(ctx, in.Handle)
	if err != nil {
		return nil, err
	}
	cursor := encodeCursor(secret, &cursorClaims{
		Sub: id, Database: in.DatabaseID, Actor: actor, Plan: plan.ID, Revision: plan.Revision,
		Generation: in.Generation, SchemaRev: in.SchemaRevision, Policy: in.PolicyEpoch,
		Seq: head, QueryHash: queryHash, Snap: true,
	})
	return &Registration{
		ID: id, DatabaseID: in.DatabaseID, QueryID: plan.ID, PlanRevision: plan.Revision,
		Generation: in.Generation, SchemaRevision: in.SchemaRevision, PolicyEpoch: in.PolicyEpoch,
		Cursor: cursor, Sequence: head, ExpiresAt: expires,
	}, nil
}

// reissue returns the actor's existing ACTIVE and unexpired subscription
// for the same query id + revision + parameters. The returned cursor is
// re-issued at the subscription's current boundary; when the in-memory
// result state was lost (process restart) the SAME subscription
// re-snapshots and the orphaned replay chain is truncated.
func (s *Service) reissue(ctx context.Context, in *RegisterInput, plan *Plan, queryHash, actor string, args []any, predicate string, predArgs []any) (*Registration, bool, error) {
	rows, err := in.Handle.QueryContext(ctx,
		`SELECT id, database_id, plan_id, plan_revision, query_hash, actor_collection_id, actor_record_id,
		        state, params_json, generation, schema_revision, policy_epoch, head_seq, expires_at, created
		 FROM _mb_live_subscriptions
		 WHERE database_id = ? AND plan_id = ? AND plan_revision = ? AND query_hash = ?
		   AND actor_collection_id = ? AND actor_record_id = ? AND state = 'active'
		 ORDER BY created DESC, id DESC LIMIT 5`,
		in.DatabaseID, plan.ID, plan.Revision, queryHash, in.ActorCollectionID, in.ActorRecordID)
	if err != nil {
		return nil, false, err
	}
	defer rows.Close()
	var row *subRow
	now := time.Now().UTC()
	for rows.Next() {
		var r subRow
		if serr := rows.Scan(&r.ID, &r.DatabaseID, &r.PlanID, &r.PlanRevision, &r.QueryHash, &r.ActorCol, &r.ActorRec,
			&r.State, &r.ParamsJSON, &r.Generation, &r.SchemaRev, &r.PolicyEpoch, &r.HeadSeq, &r.ExpiresAt, &r.Created); serr != nil {
			return nil, false, serr
		}
		if exp, perr := time.Parse(time.RFC3339Nano, r.ExpiresAt); perr != nil || now.After(exp) {
			continue // expired leases do not idempotently re-register
		}
		row = &r
		break
	}
	if err := rows.Err(); err != nil {
		return nil, false, err
	}
	if row == nil {
		return nil, false, nil
	}
	expiresAt := now.Add(SubscriptionTTL).Format(time.RFC3339Nano)
	if _, uerr := in.Handle.ExecContext(ctx,
		`UPDATE _mb_live_subscriptions SET expires_at = ?, updated = ? WHERE id = ?`,
		expiresAt, nowRFC3339(), row.ID); uerr != nil {
		return nil, false, uerr
	}
	st := s.state(row.ID)
	seq := row.HeadSeq
	st.mu.Lock()
	hasRows := st.rows != nil
	st.mu.Unlock()
	if !hasRows {
		// Prior result state was not persisted (restart): re-snapshot the
		// SAME subscription and restart its replay chain.
		tx, terr := in.Handle.BeginTx(ctx, nil)
		if terr != nil {
			return nil, false, terr
		}
		committed := false
		defer func() {
			if !committed {
				_ = tx.Rollback()
			}
		}()
		head, herr := readJournalHead(ctx, tx)
		if herr != nil {
			return nil, false, herr
		}
		newRows, rerr := evalSnapshot(ctx, tx, plan, args, predicate, predArgs)
		if rerr != nil {
			return nil, false, rerr
		}
		if cerr := tx.Commit(); cerr != nil {
			return nil, false, cerr
		}
		committed = true
		st = s.replaceState(row.ID, newRows, head)
		seq = head
		if _, derr := in.Handle.ExecContext(ctx,
			`DELETE FROM _mb_live_replay WHERE subscription_id = ?`, row.ID); derr != nil {
			return nil, false, derr
		}
		if _, uerr := in.Handle.ExecContext(ctx,
			`UPDATE _mb_live_subscriptions SET head_seq = ?, updated = ? WHERE id = ?`,
			head, nowRFC3339(), row.ID); uerr != nil {
			return nil, false, uerr
		}
	} else {
		st.mu.Lock()
		seq = st.seq
		st.mu.Unlock()
	}
	secret, serr := s.cursorSecretCached(ctx, in.Handle)
	if serr != nil {
		return nil, false, serr
	}
	cursor := encodeCursor(secret, &cursorClaims{
		Sub: row.ID, Database: row.DatabaseID, Actor: actor, Plan: plan.ID, Revision: plan.Revision,
		Generation: row.Generation, SchemaRev: row.SchemaRev, Policy: row.PolicyEpoch,
		Seq: seq, QueryHash: row.QueryHash, Snap: true,
	})
	return &Registration{
		ID: row.ID, DatabaseID: row.DatabaseID, QueryID: plan.ID, PlanRevision: plan.Revision,
		Generation: row.Generation, SchemaRevision: row.SchemaRev, PolicyEpoch: row.PolicyEpoch,
		Cursor: cursor, Sequence: seq, ExpiresAt: expiresAt,
	}, true, nil
}

// loadSubscription reads one durable subscription row.
func loadSubscription(ctx context.Context, handle *sql.DB, id string) (*subRow, error) {
	var row subRow
	err := handle.QueryRowContext(ctx,
		`SELECT id, database_id, plan_id, plan_revision, query_hash, actor_collection_id, actor_record_id,
		        state, params_json, generation, schema_revision, policy_epoch, head_seq, expires_at, created
		 FROM _mb_live_subscriptions WHERE id = ?`, id,
	).Scan(&row.ID, &row.DatabaseID, &row.PlanID, &row.PlanRevision, &row.QueryHash, &row.ActorCol, &row.ActorRec,
		&row.State, &row.ParamsJSON, &row.Generation, &row.SchemaRev, &row.PolicyEpoch, &row.HeadSeq, &row.ExpiresAt, &row.Created)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &row, nil
}

// Step advances one authorized stream step: it re-checks the actor,
// re-resolves the plan (retirement/unpublish termination), re-checks the
// context (schema/generation/policy change → reset with a fresh
// snapshot), replays durable patches when the cursor is behind the last
// evaluated boundary (any gap in the replay chain means reset plus a new
// snapshot) and otherwise re-evaluates in one read transaction and
// produces the keyed ordered delta.
func (s *Service) Step(ctx context.Context, resolver PlanResolver, in *StepInput) (*StepResult, error) {
	if resolver == nil {
		return nil, fmt.Errorf("%w: the plan resolver is unavailable", ErrInvalidRequest)
	}
	if in == nil || in.Handle == nil || in.SubscriptionID == "" || in.Cursor == "" || in.ActorRecordID == "" {
		return nil, fmt.Errorf("%w: subscription, cursor and a valid record authorization are required", ErrInvalidRequest)
	}
	if err := ensureLiveTables(ctx, in.Handle); err != nil {
		return nil, err
	}
	row, err := loadSubscription(ctx, in.Handle, in.SubscriptionID)
	if err != nil {
		s.dropState(in.SubscriptionID)
		return nil, err
	}
	if row.State != "active" {
		s.dropState(row.ID)
		return nil, fmt.Errorf("%w: the subscription is %q", ErrCancelled, row.State)
	}
	if exp, perr := time.Parse(time.RFC3339Nano, row.ExpiresAt); perr != nil || time.Now().UTC().After(exp) {
		_, _ = in.Handle.ExecContext(ctx,
			`UPDATE _mb_live_subscriptions SET state = 'cancelled', updated = ? WHERE id = ?`, nowRFC3339(), row.ID)
		s.dropState(row.ID)
		return nil, fmt.Errorf("%w: the subscription lease expired", ErrCancelled)
	}
	// Re-check the actor: the subscription and the presented cursor must
	// both bind the presenting actor.
	actor := actorOf(in.ActorCollectionID, in.ActorRecordID)
	if actor != actorOf(row.ActorCol, row.ActorRec) {
		return nil, ErrForbidden
	}
	// The stream path's query id selects this subscription's plan.
	if in.QueryID != "" && in.QueryID != row.PlanID {
		return nil, ErrNotFound
	}
	// Re-resolve the plan: a retired/unpublished revision terminates the
	// subscription (retirement sends a reset/termination to subscribers).
	plan, err := resolver.ResolvePublishedLivePlan(ctx, row.PlanID, row.PlanRevision)
	if err != nil {
		if errors.Is(err, ErrPlanUnpublished) || errors.Is(err, ErrNotFound) {
			_, _ = in.Handle.ExecContext(ctx,
				`UPDATE _mb_live_subscriptions SET state = 'cancelled', updated = ? WHERE id = ?`, nowRFC3339(), row.ID)
			s.dropState(row.ID)
		}
		return nil, err
	}
	// The subscription's stored parameters bind the plan (the registered
	// parameter set is durable; a caller cannot re-bind different values
	// through the stream).
	var storedParams []any
	if jerr := json.Unmarshal([]byte(row.ParamsJSON), &storedParams); jerr != nil {
		return nil, fmt.Errorf("%w: the subscription's stored parameters are unreadable", ErrInvalidRequest)
	}
	args, err := bindParams(plan, storedParams)
	if err != nil {
		return nil, err
	}
	// The actor's CURRENT rules re-inject at every re-evaluation (the
	// recheck before evaluation/publication).
	predicate, predArgs, err := policyPredicate(ctx, resolver, plan, in.ActorCollectionID, in.ActorRecordID)
	if err != nil {
		return nil, err
	}
	secret, err := s.cursorSecretCached(ctx, in.Handle)
	if err != nil {
		return nil, err
	}
	claims, err := decodeCursor(secret, in.Cursor)
	if err != nil {
		return nil, err
	}
	// The cursor must bind THIS subscription, actor, database and query.
	if claims.Sub != row.ID {
		return nil, fmt.Errorf("%w: the cursor binds another subscription", ErrInvalidCursor)
	}
	if claims.Actor != actor {
		return nil, ErrForbidden
	}
	if claims.Database != row.DatabaseID || claims.Plan != row.PlanID ||
		claims.Revision != row.PlanRevision || claims.QueryHash != row.QueryHash {
		return nil, fmt.Errorf("%w: the cursor does not bind this subscription's query", ErrInvalidCursor)
	}

	st := s.state(row.ID)
	st.mu.Lock()
	defer st.mu.Unlock()

	// Context re-check: a schema, generation or policy change emits a
	// reset with a fresh snapshot under the new context.
	if claims.Generation != in.Generation || claims.SchemaRev != in.SchemaRevision || claims.Policy != in.PolicyEpoch {
		return s.resetLocked(ctx, in, row, plan, args, predicate, predArgs, "schema_or_policy_changed", row.QueryHash)
	}
	// Prior result state was not persisted (restart): reset with a fresh
	// snapshot and restart the replay chain.
	if st.rows == nil {
		return s.resetLocked(ctx, in, row, plan, args, predicate, predArgs, "state_not_persisted", row.QueryHash)
	}
	// A registration cursor delivers the full snapshot the registration
	// captured: the client has no local view yet, and the snapshot
	// supersedes any missed delta range.
	if claims.Snap {
		rawRows := make([]map[string]any, 0, len(st.rows))
		for _, r := range st.rows {
			rawRows = append(rawRows, r.Row)
		}
		snapSeq := st.seq
		cursor := encodeCursor(secret, &cursorClaims{
			Sub: row.ID, Database: row.DatabaseID, Actor: actor, Plan: row.PlanID, Revision: row.PlanRevision,
			Generation: in.Generation, SchemaRev: in.SchemaRevision, Policy: in.PolicyEpoch,
			Seq: snapSeq, QueryHash: row.QueryHash,
		})
		return &StepResult{Events: []Event{{
			Kind: KindSnapshot, Data: map[string]any{
				"subscriptionId": row.ID, "queryId": row.PlanID, "sequence": snapSeq,
				"schemaRevision": in.SchemaRevision, "policyEpoch": in.PolicyEpoch,
				"cursor": cursor, "rows": rawRows,
			},
		}}, Cursor: cursor, Sequence: snapSeq}, nil
	}

	events := []Event{}
	seq := claims.Seq

	if claims.Seq < st.seq {
		// The cursor is behind the last evaluated boundary: replay the
		// durable chain. ANY gap — missing patches, a non-contiguous
		// chain, an expired lease or an over-budget range — means reset
		// plus a new snapshot.
		patches, gerr := loadReplayChain(ctx, in.Handle, row.ID, claims.Seq, st.seq)
		if gerr != nil {
			return s.resetLocked(ctx, in, row, plan, args, predicate, predArgs, "replay_chain_broken", row.QueryHash)
		}
		totalBytes := 0
		ok := len(patches) > 0 && patches[0].From == claims.Seq && patches[len(patches)-1].To == st.seq
		for i, p := range patches {
			if i > 0 && p.From != patches[i-1].To {
				ok = false
				break
			}
			if age, perr := time.Parse(time.RFC3339Nano, p.Created); perr != nil || time.Now().UTC().Sub(age) > ReplayLease {
				ok = false
				break
			}
			totalBytes += p.Bytes
			if totalBytes > ReplayMaxBytes {
				ok = false
				break
			}
		}
		if !ok {
			return s.resetLocked(ctx, in, row, plan, args, predicate, predArgs, "replay_chain_broken", row.QueryHash)
		}
		for _, p := range patches {
			events = append(events, s.deltaEvent(row, p.From, p.To, p.Operations))
			seq = p.To
		}
		// Replay lease maintenance: prune expired patches (clients whose
		// cursor still needs them reset on their next step).
		_, _ = in.Handle.ExecContext(ctx,
			`DELETE FROM _mb_live_replay WHERE subscription_id = ? AND created < ?`,
			row.ID, time.Now().UTC().Add(-ReplayLease).Format(time.RFC3339Nano))
	} else if claims.Seq > st.seq {
		// The cursor is ahead of the evaluated state (stale service
		// state): reset with a fresh snapshot.
		return s.resetLocked(ctx, in, row, plan, args, predicate, predArgs, "state_not_persisted", row.QueryHash)
	}

	// Normal path: the boundary is current — re-evaluate in ONE read
	// transaction (head captured BEFORE the rows) and produce the keyed
	// ordered diff when the journal advanced. Multiple commits since the
	// last boundary coalesce into ONE result-state delta.
	if seq == st.seq {
		head, err := readJournalHead(ctx, in.Handle)
		if err != nil {
			return nil, err
		}
		if head > st.seq {
			tx, terr := in.Handle.BeginTx(ctx, nil)
			if terr != nil {
				return nil, terr
			}
			committed := false
			defer func() {
				if !committed {
					_ = tx.Rollback()
				}
			}()
			head2, herr := readJournalHead(ctx, tx)
			if herr != nil {
				return nil, herr
			}
			newRows, eerr := evalSnapshot(ctx, tx, plan, args, predicate, predArgs)
			if eerr != nil {
				return nil, eerr
			}
			if cerr := tx.Commit(); cerr != nil {
				return nil, cerr
			}
			committed = true
			if head2 > st.seq {
				ops := diffRows(st.rows, newRows)
				if len(ops) > 0 {
					if err := storePatch(ctx, in.Handle, row.ID, st.seq, head2, ops); err != nil {
						return nil, err
					}
					if perr := pruneReplay(ctx, in.Handle, row.ID); perr != nil {
						return nil, perr
					}
					events = append(events, s.deltaEvent(row, st.seq, head2, ops))
				}
				st.rows = newRows
				st.seq = head2
				seq = head2
				if _, uerr := in.Handle.ExecContext(ctx,
					`UPDATE _mb_live_subscriptions SET head_seq = ?, updated = ? WHERE id = ?`,
					head2, nowRFC3339(), row.ID); uerr != nil {
					return nil, uerr
				}
			}
		}
	}

	// Sliding subscription lease: refreshed once it is half spent.
	if exp, perr := time.Parse(time.RFC3339Nano, row.ExpiresAt); perr == nil {
		if time.Now().UTC().Add(SubscriptionTTL / 2).After(exp) {
			_, _ = in.Handle.ExecContext(ctx,
				`UPDATE _mb_live_subscriptions SET expires_at = ?, updated = ? WHERE id = ?`,
				time.Now().UTC().Add(SubscriptionTTL).Format(time.RFC3339Nano), nowRFC3339(), row.ID)
		}
	}

	cursor := encodeCursor(secret, &cursorClaims{
		Sub: row.ID, Database: row.DatabaseID, Actor: actor, Plan: row.PlanID, Revision: row.PlanRevision,
		Generation: in.Generation, SchemaRev: in.SchemaRevision, Policy: in.PolicyEpoch,
		Seq: seq, QueryHash: row.QueryHash,
	})
	// Every frame carries the boundary the caller continues from.
	for i := range events {
		events[i].Data["cursor"] = cursor
	}
	return &StepResult{Events: events, Cursor: cursor, Sequence: seq}, nil
}

// resetLocked emits reset + a fresh snapshot under the current context:
// one consistent read transaction (head first), the result state
// replaced, the replay chain restarted from the new boundary.
func (s *Service) resetLocked(ctx context.Context, in *StepInput, row *subRow, plan *Plan, args []any, predicate string, predArgs []any, reason, queryHash string) (*StepResult, error) {
	tx, terr := in.Handle.BeginTx(ctx, nil)
	if terr != nil {
		return nil, terr
	}
	committed := false
	defer func() {
		if !committed {
			_ = tx.Rollback()
		}
	}()
	head, herr := readJournalHead(ctx, tx)
	if herr != nil {
		return nil, herr
	}
	rows, rerr := evalSnapshot(ctx, tx, plan, args, predicate, predArgs)
	if rerr != nil {
		return nil, rerr
	}
	if cerr := tx.Commit(); cerr != nil {
		return nil, cerr
	}
	committed = true

	st := s.state(row.ID)
	st.rows = rows
	st.seq = head
	if _, derr := in.Handle.ExecContext(ctx,
		`DELETE FROM _mb_live_replay WHERE subscription_id = ?`, row.ID); derr != nil {
		return nil, derr
	}
	expires := time.Now().UTC().Add(SubscriptionTTL).Format(time.RFC3339Nano)
	if _, uerr := in.Handle.ExecContext(ctx,
		`UPDATE _mb_live_subscriptions SET head_seq = ?, generation = ?, schema_revision = ?, policy_epoch = ?, expires_at = ?, updated = ? WHERE id = ?`,
		head, in.Generation, in.SchemaRevision, in.PolicyEpoch, expires, nowRFC3339(), row.ID); uerr != nil {
		return nil, uerr
	}
	secret, serr := s.cursorSecretCached(ctx, in.Handle)
	if serr != nil {
		return nil, serr
	}
	actor := actorOf(in.ActorCollectionID, in.ActorRecordID)
	cursor := encodeCursor(secret, &cursorClaims{
		Sub: row.ID, Database: row.DatabaseID, Actor: actor, Plan: row.PlanID, Revision: row.PlanRevision,
		Generation: in.Generation, SchemaRev: in.SchemaRevision, Policy: in.PolicyEpoch,
		Seq: head, QueryHash: queryHash,
	})
	rawRows := make([]map[string]any, 0, len(rows))
	for _, r := range rows {
		rawRows = append(rawRows, r.Row)
	}
	events := []Event{
		{Kind: KindReset, Data: map[string]any{
			"subscriptionId": row.ID, "queryId": row.PlanID, "sequence": head,
			"cursor": cursor, "reason": reason,
		}},
		{Kind: KindSnapshot, Data: map[string]any{
			"subscriptionId": row.ID, "queryId": row.PlanID, "sequence": head,
			"schemaRevision": in.SchemaRevision, "policyEpoch": in.PolicyEpoch,
			"cursor": cursor, "rows": rawRows,
		}},
	}
	return &StepResult{Events: events, Cursor: cursor, Sequence: head}, nil
}

// replayPatch is one durable replay row.
type replayPatch struct {
	From       int64
	To         int64
	Bytes      int
	Created    string
	Operations []patchOp
}

// loadReplayChain loads the durable patches covering (afterSeq, toSeq].
func loadReplayChain(ctx context.Context, handle *sql.DB, subID string, afterSeq, toSeq int64) ([]replayPatch, error) {
	rows, err := handle.QueryContext(ctx,
		`SELECT from_seq, to_seq, patch_json, bytes, created FROM _mb_live_replay
		 WHERE subscription_id = ? AND to_seq > ? AND to_seq <= ? ORDER BY to_seq`,
		subID, afterSeq, toSeq)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []replayPatch
	for rows.Next() {
		var p replayPatch
		var patchJSON []byte
		if err := rows.Scan(&p.From, &p.To, &patchJSON, &p.Bytes, &p.Created); err != nil {
			return nil, err
		}
		var ops []patchOp
		if jerr := json.Unmarshal(patchJSON, &ops); jerr != nil {
			return nil, fmt.Errorf("corrupt replay patch: %w", jerr)
		}
		p.Operations = ops
		out = append(out, p)
	}
	return out, rows.Err()
}

// storePatch appends one durable replay patch at the chain tail.
func storePatch(ctx context.Context, handle *sql.DB, subID string, fromSeq, toSeq int64, ops []patchOp) error {
	b, err := json.Marshal(ops)
	if err != nil {
		return err
	}
	if len(b) == 0 {
		return fmt.Errorf("empty replay patch")
	}
	_, err = handle.ExecContext(ctx,
		`INSERT INTO _mb_live_replay (subscription_id, to_seq, from_seq, patch_json, bytes, created)
		 VALUES (?, ?, ?, ?, ?, ?)
		 ON CONFLICT(subscription_id, to_seq) DO UPDATE SET
		   from_seq = excluded.from_seq, patch_json = excluded.patch_json,
		   bytes = excluded.bytes, created = excluded.created`,
		subID, toSeq, fromSeq, string(b), len(b), nowRFC3339())
	return err
}

// pruneReplay enforces the byte-bounded replay budget: the oldest patches
// are pruned first (cursors needing them then reset with a fresh
// snapshot).
func pruneReplay(ctx context.Context, handle *sql.DB, subID string) error {
	for i := 0; i < 1_000_000; i++ {
		var total int64
		if err := handle.QueryRowContext(ctx,
			`SELECT COALESCE(SUM(bytes), 0) FROM _mb_live_replay WHERE subscription_id = ?`, subID).Scan(&total); err != nil {
			return err
		}
		if total <= ReplayMaxBytes {
			return nil
		}
		var oldest sql.NullInt64
		if err := handle.QueryRowContext(ctx,
			`SELECT MIN(to_seq) FROM _mb_live_replay WHERE subscription_id = ?`, subID).Scan(&oldest); err != nil {
			return err
		}
		if !oldest.Valid {
			return nil
		}
		if _, err := handle.ExecContext(ctx,
			`DELETE FROM _mb_live_replay WHERE subscription_id = ? AND to_seq = ?`, subID, oldest.Int64); err != nil {
			return err
		}
	}
	return nil
}

// deltaEvent builds one delta frame: the complete ordered patch is
// carried by a single frame (applied atomically); the SSE event kind is
// the patch's sole operation, or update when the patch is mixed.
func (s *Service) deltaEvent(row *subRow, fromSeq, toSeq int64, ops []patchOp) Event {
	kind := ops[0].Op
	if len(ops) != 1 {
		kind = KindUpdate
	}
	return Event{Kind: kind, Data: map[string]any{
		"subscriptionId": row.ID,
		"queryId":        row.PlanID,
		"fromSequence":   fromSeq,
		"toSequence":     toSeq,
		"patch":          ops,
	}}
}

// Cancel releases the actor's subscription of one query id idempotently
// ({id, closed:true}). The subscription is identified exactly when the
// request carries a subscription id or the registration's parameter set;
// without an identifier, a single ACTIVE unexpired subscription cancels
// while an AMBIGUOUS plan (several active subscriptions with different
// parameters) is refused — "the latest row" is never an identity. An
// expired lease is not an active subscription (its next step cancels it),
// so expired rows neither satisfy a parameter-set cancel nor make the
// plan ambiguous.
func (s *Service) Cancel(ctx context.Context, resolver PlanResolver, in *CancelInput) (*CancelResult, error) {
	if in == nil || in.Handle == nil || in.QueryID == "" || in.ActorRecordID == "" {
		return nil, fmt.Errorf("%w: query and a valid record authorization are required", ErrInvalidRequest)
	}
	if err := ensureLiveTables(ctx, in.Handle); err != nil {
		return nil, err
	}
	actor := actorOf(in.ActorCollectionID, in.ActorRecordID)

	cancelRow := func(id string) (*CancelResult, error) {
		if _, uerr := in.Handle.ExecContext(ctx,
			`UPDATE _mb_live_subscriptions SET state = 'cancelled', updated = ? WHERE id = ? AND state = 'active'`,
			nowRFC3339(), id); uerr != nil {
			return nil, uerr
		}
		s.dropState(id)
		return &CancelResult{ID: id, Closed: true}, nil
	}

	// Exact subscription id.
	if in.SubscriptionID != "" {
		row, err := loadSubscription(ctx, in.Handle, in.SubscriptionID)
		if err != nil {
			return nil, err
		}
		if row.DatabaseID != in.DatabaseID || row.PlanID != in.QueryID ||
			actorOf(row.ActorCol, row.ActorRec) != actor {
			return nil, ErrNotFound
		}
		return cancelRow(row.ID)
	}

	// Exact parameter set: compile the registration's query hash (the
	// same bound the registration stored). The ACTIVE unexpired
	// subscription bound to this parameter set is the one cancelled; the
	// latest row of the set (idempotent closed) is answered only when no
	// active one remains.
	if in.Parameters != nil {
		if resolver == nil {
			return nil, fmt.Errorf("%w: the plan resolver is unavailable", ErrInvalidRequest)
		}
		plan, err := resolver.ResolvePublishedLivePlan(ctx, in.QueryID, 0)
		if err != nil {
			return nil, err
		}
		if _, err := bindParams(plan, in.Parameters); err != nil {
			return nil, err
		}
		hash := queryHashOf(plan, in.Parameters)
		rows, err := in.Handle.QueryContext(ctx,
			`SELECT id, state, expires_at FROM _mb_live_subscriptions
			 WHERE database_id = ? AND plan_id = ? AND query_hash = ?
			   AND actor_collection_id = ? AND actor_record_id = ?
			 ORDER BY created DESC, id DESC LIMIT 64`,
			in.DatabaseID, in.QueryID, hash, in.ActorCollectionID, in.ActorRecordID,
		)
		if err != nil {
			return nil, err
		}
		defer rows.Close()
		var active, latest string
		now := time.Now().UTC()
		for rows.Next() {
			var id, state, expires string
			if serr := rows.Scan(&id, &state, &expires); serr != nil {
				return nil, serr
			}
			if latest == "" {
				latest = id
			}
			if state != "active" {
				continue
			}
			if exp, perr := time.Parse(time.RFC3339Nano, expires); perr != nil || now.After(exp) {
				continue // an expired lease is not an active subscription
			}
			if active == "" {
				active = id
			}
		}
		if err := rows.Err(); err != nil {
			return nil, err
		}
		if active != "" {
			return cancelRow(active)
		}
		if latest == "" {
			return nil, ErrNotFound
		}
		return cancelRow(latest)
	}

	// No identifier: exactly one ACTIVE unexpired subscription cancels; an
	// ambiguous plan (several active parameter sets) is refused; no active
	// subscription is answered with the idempotent closed result for the
	// latest row.
	rows, err := in.Handle.QueryContext(ctx,
		`SELECT id, state, expires_at FROM _mb_live_subscriptions
		 WHERE database_id = ? AND plan_id = ? AND actor_collection_id = ? AND actor_record_id = ?
		 ORDER BY created DESC, id DESC LIMIT 64`,
		in.DatabaseID, in.QueryID, in.ActorCollectionID, in.ActorRecordID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var latestAny, latestActive string
	activeCount := 0
	now := time.Now().UTC()
	for rows.Next() {
		var id, state, expires string
		if serr := rows.Scan(&id, &state, &expires); serr != nil {
			return nil, serr
		}
		if latestAny == "" {
			latestAny = id
		}
		if state != "active" {
			continue
		}
		if exp, perr := time.Parse(time.RFC3339Nano, expires); perr != nil || now.After(exp) {
			continue // an expired lease is not an active subscription
		}
		activeCount++
		if latestActive == "" {
			latestActive = id
		}
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if latestAny == "" {
		return nil, ErrNotFound
	}
	if latestActive == "" {
		// Idempotent cancel of an already-cancelled/expired subscription.
		return &CancelResult{ID: latestAny, Closed: true}, nil
	}
	if activeCount > 1 {
		return nil, fmt.Errorf("%w: the plan has %d active subscriptions with different parameters; specify the subscription id or its parameters", ErrInvalidRequest, activeCount)
	}
	return cancelRow(latestActive)
}

// ProbeCursor validates a presented cursor BEFORE the SSE headers are
// written: decode/authenticity, actor binding, then the subscription
// cross-check. The subscription is identified EXACTLY by the cursor's
// subscription claim (an actor may hold several subscriptions of the same
// plan with different parameter sets; "the latest row" is not an
// identity). Read-only (no state change); Step re-checks
// authoritatively. It returns the resolved subscription id for the stream
// loop's steps.
func (s *Service) ProbeCursor(ctx context.Context, handle *sql.DB, databaseID, actorCollectionID, actorRecordID, queryID, token string) (string, error) {
	if handle == nil || token == "" || actorRecordID == "" {
		return "", fmt.Errorf("%w: cursor and a valid record authorization are required", ErrInvalidRequest)
	}
	if err := ensureLiveTables(ctx, handle); err != nil {
		return "", err
	}
	// The cursor's authenticity and actor binding are independent of the
	// subscription's state (a malformed or foreign cursor is refused even
	// when the queried subscription is gone or cancelled).
	secret, serr := s.cursorSecretCached(ctx, handle)
	if serr != nil {
		return "", serr
	}
	claims, derr := decodeCursor(secret, token)
	if derr != nil {
		return "", derr
	}
	if claims.Actor != actorOf(actorCollectionID, actorRecordID) {
		return "", ErrForbidden
	}
	// Exact identity: the cursor names the subscription.
	row, err := loadSubscription(ctx, handle, claims.Sub)
	if err != nil {
		return "", err
	}
	if row.DatabaseID != databaseID || row.PlanID != queryID {
		return "", fmt.Errorf("%w: the cursor does not bind a subscription of this query", ErrInvalidCursor)
	}
	if row.State != "active" {
		return "", fmt.Errorf("%w: the subscription is %q", ErrCancelled, row.State)
	}
	if exp, perr := time.Parse(time.RFC3339Nano, row.ExpiresAt); perr != nil || time.Now().UTC().After(exp) {
		return "", fmt.Errorf("%w: the subscription lease expired", ErrCancelled)
	}
	if claims.Database != row.DatabaseID || claims.Plan != row.PlanID ||
		claims.Revision != row.PlanRevision || claims.QueryHash != row.QueryHash {
		return "", fmt.Errorf("%w: the cursor does not bind this subscription's query", ErrInvalidCursor)
	}
	return row.ID, nil
}
