package application

// The reference-copy refresh worker (implementation-closure.md,
// Reference-copy refresh): "The source CDC commit schedules a durable
// refresh intent. A shared worker reads a consistent authorized source
// snapshot and upserts/deletes the exact read-only projection in bounded
// target transactions". The intent is the source change journal the CDC
// hooks advance; the worker is the shared consumer that keeps every
// policy's copies inside their declared maximum lag WITHOUT a manual
// refresh call.
//
// One worker instance serves the whole process (bounded shared services,
// no per-database scheduler). Every tick is cheap when idle: it reads the
// source journal head and the per-target checkpoints and refreshes only
// the policies whose copies are behind or past their lag; duplicate
// execution is a safe skip and an older event can never overwrite a newer
// sequence (the apply guard).

import (
	"context"
	"fmt"
	"database/sql"
	"log/slog"
	"sync"
	"time"

	"mirabase/core"
)

// DefaultReferenceRefreshInterval is the worker's tick interval: short
// enough to honor the 60-second default lag with margin, cheap when idle
// (one journal-head read per policy per tick).
const DefaultReferenceRefreshInterval = 5 * time.Second

// ReferenceWorker is the shared reference-copy refresh worker.
type ReferenceWorker struct {
	rt       *Runtime
	base     *core.BaseApp
	interval time.Duration
	stopc    chan struct{}
	donec    chan struct{}
	stopOnce sync.Once
	log      *slog.Logger
}

// StartReferenceWorker starts the shared worker; Stop terminates it and
// waits for the loop to exit.
func StartReferenceWorker(rt *Runtime, base *core.BaseApp, interval time.Duration, log *slog.Logger) *ReferenceWorker {
	if interval <= 0 {
		interval = DefaultReferenceRefreshInterval
	}
	if log == nil {
		log = slog.Default()
	}
	w := &ReferenceWorker{
		rt:       rt,
		base:     base,
		interval: interval,
		stopc:    make(chan struct{}),
		donec:    make(chan struct{}),
		log:      log,
	}
	go w.loop()
	return w
}

// Stop terminates the worker and waits for exit. Idempotent: repeated
// stops (a test cleanup after an explicit stop) wait on the same exit
// without re-closing the channel.
func (w *ReferenceWorker) Stop() {
	w.stopOnce.Do(func() { close(w.stopc) })
	<-w.donec
}

// Done reports the worker's exit signal (lifecycle ownership tests).
func (w *ReferenceWorker) Done() <-chan struct{} { return w.donec }

// minTickBudget is the floor of a tick's refresh budget: a small worker
// interval (a fast test tick) still gets a real budget to complete one
// refresh cycle, while the default interval keeps a generous multiple.
const minTickBudget = 2 * time.Second

func (w *ReferenceWorker) loop() {
	defer close(w.donec)
	ticker := time.NewTicker(w.interval)
	defer ticker.Stop()
	for {
		select {
		case <-w.stopc:
			return
		case <-ticker.C:
			// A tick failure never stops the worker: the next tick
			// retries (the intent is durable; the copy lag reports it).
			w.tick()
		}
	}
}

// tick refreshes the policies whose copies are behind the source head,
// past their declared maximum lag or stale against a policy EDIT (the
// definition epoch). Panic-isolated: a panicked refresh is logged and
// retried on the next tick.
func (w *ReferenceWorker) tick() {
	defer func() {
		if r := recover(); r != nil {
			w.log.Error("reference-copy refresh worker tick panicked", "panic", fmt.Sprintf("%v", r))
		}
	}()
	if w.rt == nil || w.base == nil || w.base.DBConn() == nil {
		return
	}
	budget := w.interval * 3
	if budget < minTickBudget {
		budget = minTickBudget
	}
	ctx, cancel := context.WithTimeout(context.Background(), budget)
	defer cancel()
	policies, err := w.rt.Service.ListReferencePolicies(ctx)
	if err != nil {
		w.log.Warn("reference-copy worker could not list the policies", "error", err)
		return
	}
	source := w.base.DBConn()
	if _, err := source.ExecContext(ctx, changesJournalDDL); err != nil {
		w.log.Warn("reference-copy worker could not prepare the source change journal", "error", err)
		return
	}
	for _, policy := range policies {
		policy := policy
		need, reason, err := w.needsRefresh(ctx, source, &policy)
		if err != nil {
			w.log.Warn("reference-copy worker refresh decision failed", "policy", policy.ID, "error", err)
			continue
		}
		if !need {
			continue
		}
		if _, rerr := w.rt.RefreshReferencePolicy(ctx, w.base, &policy); rerr != nil {
			w.log.Warn("reference-copy refresh failed", "policy", policy.ID, "reason", reason, "error", rerr)
			continue
		}
		w.log.Info("reference-copy refreshed", "policy", policy.ID, "reason", reason)
	}
}

// needsRefresh decides whether the policy's copies are behind the source
// head, past the declared maximum healthy lag or invalidated by a policy
// EDIT (a definition-epoch mismatch — any edit, same revision included).
// The durable registry is the shared checkpoint; a missing registry row
// means the copy was never recorded and must be reconciled.
func (w *ReferenceWorker) needsRefresh(ctx context.Context, source *sql.DB, policy *ReferencePolicyWire) (bool, string, error) {
	var head int64
	if err := source.QueryRowContext(ctx,
		`SELECT COALESCE(MAX(commit_seq), 0) FROM _mb_changes`).Scan(&head); err != nil {
		return false, "", err
	}
	targetIDs, err := w.rt.referenceTargetIDs(ctx, policy)
	if err != nil {
		return false, "", err
	}
	if len(targetIDs) == 0 {
		return false, "", nil
	}
	for _, targetID := range targetIDs {
		_, seq, revision, epoch, state, updatedAt, rerr := w.rt.readReferenceTargetRegistry(ctx, policy.ID, targetID)
		if rerr != nil {
			return true, "the target checkpoint was never recorded", nil
		}
		if epoch != policy.DefinitionEpoch {
			return true, fmt.Sprintf("target %s was applied under definition epoch %d while the policy is at %d (the edit invalidates the old refresh work)",
				targetID, epoch, policy.DefinitionEpoch), nil
		}
		if revision != policy.Revision {
			return true, fmt.Sprintf("target %s is at revision %d of %d",
				targetID, revision, policy.Revision), nil
		}
		if state != "healthy" {
			continue // quarantined/failed states reconcile only on a policy edit
		}
		if seq < head {
			return true, fmt.Sprintf("target %s is behind (applied %d of %d at revision %d of %d)",
				targetID, seq, head, revision, policy.Revision), nil
		}
		upd, perr := time.Parse(time.RFC3339Nano, updatedAt)
		if perr != nil || time.Since(upd) > time.Duration(policy.MaximumLagSeconds)*time.Second {
			return true, fmt.Sprintf("target %s is past the declared maximum lag (%ds)", targetID, policy.MaximumLagSeconds), nil
		}
	}
	return false, "", nil
}
