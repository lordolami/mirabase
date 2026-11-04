# Realtime, durable changes and live queries

**Current release authority:** [current implementation inventory](../current-product.md), [user documentation](../user/index.md) and [launch status](../launch/status.md). Initial v0.1/roadmap delivery labels below preserve planning contracts; they do not establish today's implementation or permit deleting subsequently integrated features. [Open-source platform roadmap](../roadmap/platform/README.md) has separate design and gates.

**Delivery: v0.1 and roadmap.** [v0.1-build.md](v0.1-build.md) selects the current portion; [roadsmap.md](roadsmap.md) retains the remainder. Detailed schemas/configuration/ABI and full-family acceptance below describe the target superset unless narrowed by that allocation.

## Three distinct contracts

Inherited SSE emits create/update/delete after successful record effects and applies collection rules. It stays compatible and does not gain a fake durable cursor. Durable changes expose transaction-grouped resumable frames. Live-query subscriptions maintain a validated query result/diff and require a separate native adapter. None of these names is a substitute for the others.

## Target-local durable log

Every accepted record mutation writes a transaction envelope and change rows in the same target transaction. The envelope contains databaseId, generation, transactionId, commitSequence, schemaRevision, timestamp and source=record|admin-sql|pg|native-sync|migration. Each row contains ordinal, collection/table identity, record key, operation, versioned before/after values as needed, and visibility metadata. Passwords/tokens are removed before public encoding, not merely hidden by the UI.

For native writes, configure native CDC on every relevant connection and collect/normalize it at commit. Native binary records need a revision-pinned decoder; unknown formats quarantine the stream rather than emit garbage. Verify DDL, triggers, savepoints, rollback, delete and replay. A connection that cannot capture required changes cannot perform a covered write.

Native transaction capture and application envelopes must be reconciled without duplicates. Administrative SQL changes to unmanaged tables are marked as raw SQL changes and are not fabricated record events. Schema mutations publish schema frames before data requiring that revision.

## Pull contract

GET /api/mirabase/v1/databases/{id}/changes accepts cursor, limit (default 100 transactions, maximum 1000) and optional collection IDs. Response contains complete transactions, nextCursor, hasMore, schemaRevision and retentionFloor. Never split a transaction just to meet a row limit; an oversized transaction uses ChangeChunk entries with transactionId, zero-based chunkIndex, chunkCount, JSON-UTF-8 bytes encoded as Base64, whole-transaction SHA-256 and finalCursor on the final chunk. Clients verify all chunks and hash, then apply the assembled transaction atomically before acknowledgment. Intermediate chunk cursors permit retransmission only and never assert a partially applied commit.

Cursor is opaque authenticated data binding database/generation, sequence, actor/policy epoch, filter hash and format version. Default log retention is 7 days or 1 GiB per database, whichever expires first; active clients do not override the hard budget. Track acknowledgements separately. Expired cursor returns 410 cursor_expired plus bootstrapRequired=true and floor. Generation/schema-policy incompatibility returns resync_required.

## Rule changes and redaction

Evaluate current authorization on every pull/delivery. Before/after data can reveal now-private records; only emit authorized projections. A visibility change emits an eviction/tombstone for a previously visible local record without disclosing hidden new values. If a rule/schema change prevents safe incremental determination, advance policy epoch and require a fresh authorized bootstrap. A client must never retain an asserted complete authorized replica after such an epoch change.

Delete authorization needs stored before-values/visibility metadata for the relevant committed record. Retention cleanup must not remove them prematurely. Superusers can inspect privileged raw frames; ordinary users cannot request them by changing a flag.

## Streaming and live queries

Durable SSE streams send cursor-bearing frames and heartbeats; clients reconnect with the cursor rather than assuming no losses. Bound per-client queues (default 256 frames/8 MiB) and disconnect slow consumers with an explicit resume point. Existing SSE topics and SDK subscription callbacks retain their format.

Live-query registration specifies a validated read-only query ID, typed parameters, context and authorized projection. The selected Go service evaluates a real SQL snapshot at a captured commit boundary, watches committed CDC dependencies and computes keyed/ordered result diffs. Register invalidation before the initial read so no change falls in a gap. Result deltas can coalesce commits; durable changes deliver every transaction. Bounded replay, authorization reset, top-N position changes and reconnect behavior are specified in integration-contracts.md. No missing upstream public subscribe API is required.

## Retention and evidence

Background retention uses commit sequences and durable client leases, never wall-clock timestamps alone. Report lag, floor, log bytes, decoder errors and dropped/slow connections without record payloads. Acceptance includes transaction rollback/replay, raw SQL/PG writes, trigger cascades, policy changes, deleted records, retained cursors, oversized transactions, reconnect and disk pressure.



## Application-specific subscriptions and definitions

Application identity and generations bind every SSE/change/live cursor, cache and notification. Named query definitions use the explicit registry lifecycle in implementation-closure.md; cross-app commit IDs never select another app subscription. See [application-specific subscriptions and definitions](application-isolation.md).
