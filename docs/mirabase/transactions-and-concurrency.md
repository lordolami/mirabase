# Transactions and concurrency

**Current release authority:** [current implementation inventory](../current-product.md), [user documentation](../user/index.md) and [launch status](../launch/status.md). Initial v0.1/roadmap delivery labels below preserve planning contracts; they do not establish today's implementation or permit deleting subsequently integrated features. [Managed cloud](../cloud/README.md) has separate design and gates.

**Delivery: v0.1 and roadmap.** [v0.1-build.md](v0.1-build.md) selects the current portion; [roadsmap.md](roadsmap.md) retains the remainder. Detailed schemas/configuration/ABI and full-family acceptance below describe the target superset unless narrowed by that allocation.

## Profiles and guarantees

The default profile is wal. It preserves serialized application writes and native WAL recovery. mvcc is opt-in per database and enables the qualified BEGIN CONCURRENT path. Both remain single-server modes. Snapshot isolation is not a blanket promise of serializability; write skew across independently written rows needs application constraints/locking or a stronger verified mechanism.

Profile changes require maintenance, drained leases, a verified backup, native transition checks and updated manifest. Incompatible schemas/features block the change before mutation. A request cannot choose its own transaction profile.

## Transaction boundary

RunInTransaction and batch run inside one selected DatabaseContext. The context-bound app exposes the same transaction to reads/writes/hooks. Nested calls use the inherited DBX semantics rather than opening a second transaction on another connection. Externally visible record success requires committed data and applicable local change/receipt entries.

Multiple contexts cannot participate in one ordinary batch transaction. Reject a mixed-context batch before its first write. Cross-context workflows use explicit jobs with recorded per-step commits and compensation where possible; compensation is not rollback.

## Pools and connection state

Primary read defaults remain the inherited 120 open/15 idle and auxiliary 20/3 until load qualification justifies a change. Serialized writer remains one. Optional database handles default to 4 read connections and one WAL writer or 4 MVCC writers, constrained by a global default 256 connection budget. Primary reservations consume that budget explicitly; config validation rejects impossible reservations rather than silently oversubscribing.

The cache budget is global with per-handle shares; multiplying the inherited 32 MiB cache by thousands of files is forbidden. Per-context concurrency limits queue fairly with bounded waiting. Transaction timeouts default to the inherited endpoint limit where one exists; new sync/search/job limits are separately specified.

## Conflicts, retries and idempotency

Normalize busy, snapshot/write conflicts, deadlock-like conditions, constraint failures, cancellation and storage faults separately. A retryable response includes retryable=true and Retry-After when meaningful. Constraint/permission/schema errors are not retried.

Automatic retries are allowed only before any unsafe external hook effect, or for a registered replay-safe transaction body with durable idempotency. Default retry count is zero for arbitrary user hooks. Internal pure transactions may retry up to 3 attempts with bounded jitter under the request deadline. Never run mail, HTTP calls, file finalization or user code twice while claiming exactly-once behavior.

New mutation operations accept Idempotency-Key (16..128 printable ASCII, no whitespace). Scope it to actor, database generation and operation. Hash the canonical request, including staged file hashes. Same key/same hash returns the committed result; same key/different hash returns idempotency_conflict. Persist receipt in the target transaction. Default receipt retention is 7 days and is advertised to clients; retries after expiry have no exactly-once guarantee.

## Effects and hooks

Validation and before hooks run before commit. After-success hooks retain inherited ordering; durable optional delivery records are committed with the data. Hooks have context/transaction IDs and a clear after-commit phase. A rollback suppresses SSE/outbox success events and deletes only job-owned temporary files. An after-commit failure returns or logs committed=true with the delivery state; it cannot retroactively undo the database.

Native SQL/PG writes are administrative paths: they may generate CDC, but do not automatically execute record forms/hooks. Record API hooks cannot be promised for native replay without a real mutation interceptor and validated conflict contract.

## Durability and uncertain outcomes

Default synchronization settings match the inherited NORMAL profile unless a stronger operator profile is selected. Document process-crash recovery separately from power-loss durability. Group commit changes fsync batching, not the right to acknowledge nonexistent commits. Fault testing must demonstrate actual guarantees for the selected setting.

If a connection drops during Commit, return outcome_unknown when the outcome cannot be determined. Clients reconcile through idempotency receipt/status after reconnect. Cancellation never means rollback by assertion.

Acceptance includes concurrent same-row and independent-row writes, write skew examples, unique/FK conflicts, nested transaction failures, queue cancellation, cache eviction during active transactions, unsafe hook duplication controls and kill/restart at each commit boundary.

