# Schema evolution, indexes and views

**Current release authority:** [current implementation inventory](../current-product.md), [user documentation](../user/index.md) and [launch status](../launch/status.md). Initial v0.1/roadmap delivery labels below preserve planning contracts; they do not establish today's implementation or permit deleting subsequently integrated features. [Open-source platform roadmap](../roadmap/platform/README.md) has separate design and gates.

**Delivery: v0.1 and roadmap.** [v0.1-build.md](v0.1-build.md) selects the current portion; [roadsmap.md](roadsmap.md) retains the remainder. Detailed schemas/configuration/ABI and full-family acceptance below describe the target superset unless narrowed by that allocation.

## Schema authority

Collection definitions and versioned type/index metadata are authoritative for application APIs. Physical tables alone are not collections. The schema planner produces a normalized diff with object IDs, destructive effects, capability constraints, rule dependencies, estimated work and rollback requirements. Superuser approval is implicit in an authorized apply request, but a dry run itself has no effects.

Each apply request includes expectedSchemaRevision and a plan hash. Reject stale or changed plans. Lock the target schema, drain incompatible statements, persist job intent, migrate using supported transactions/rebuild steps, validate, publish cache/template revision and emit schema change. Requests pinned to an incompatible revision retry or fail explicitly; never execute with a half-updated cache.

## Operations and preservation

Support inherited create/rename/delete collections and fields, field settings, indexes, triggers, relation constraints, base/auth/view options and import/export. Added types/domains/STRUCT/UNION/sequences/defaults/generated columns/FTS/materialized views join the same dependency graph. Preserve IDs, auth secrets, file references and migration history.

Index definitions are structured: name, kind=btree|fts, unique, ordered fields/expressions, null ordering, predicate and kind-specific options. Compile only approved expressions. Preserve inherited SQL index input for compatibility through parsing/validation. Native composite-index limitations are rejected at plan time. A new upstream custom index method needs a declared adapter and lifecycle before it appears in UI.

## Views

Ordinary View collections stay read-only and retain SQL validation and ID requirements. A materialized View collection adds query, refreshPolicy="incremental", dependency list, native capability set and storage profile. The native DBSP subset is validated by actual preparation, not keyword guesses.

Materialized views are transactionally maintained only where upstream supports the combination. mvcc and attached-database combinations currently block plan application. The UI explains which query or mode conflicts. A SQL REFRESH statement being a no-op upstream does not establish an independent refresh scheduler. Rebuild is an explicit durable job with a consistency boundary.

Live-query subscriptions use the selected Go SQL/CDC service described in integration-contracts.md, with per-actor policy, snapshot boundaries and keyed result diffs. They are distinct from ordinary record SSE and materialized storage; the implementation does not wait for an upstream public subscription API.

## Fan-out

Template publication freezes a revision. Rollout snapshots target IDs/generations, then applies in bounded parallelism (default 2). Each target records pending/running/succeeded/failed/skipped-with-reason; skipped is not succeeded. A failed target can be retried idempotently. New contexts use the approved revision selected by policy, not an arbitrary partially applied one.

No global transaction covers a million files. Rollout status reports total, completed, failed and remaining, with exact revisions and compatibility windows. Operator stop pauses future work and safely completes/rolls back the current native step. Resume validates changed targets.

## Migration files

Retain up/down/history-sync and auto-generated Go/JS migrations with MiraBase branding. Export includes hashes, dependencies and version. Secrets are excluded from generated code. Down migrations can be destructive and need a clear effect report; a backup copy is not called an automatic inverse for every operation.

Acceptance creates representative inherited and new schemas, imports/exports/rebuilds them, kills during every physical/cache publication step, and compares records, indexes, rules, IDs, files and native integrity. Explicitly test incompatible mode/type/query combinations and ensure zero partial publication.



## Query-plan and rollout management

Implement named live-query plan create/publish/retire contracts, app-qualified schema caches, typed kind-specific schema definitions and disk-backed rollout target snapshots with pause/resume/retry and exact per-target states. See [query-plan and rollout management](implementation-closure.md).
