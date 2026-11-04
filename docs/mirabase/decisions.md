# Selected design decisions

**Current release authority:** [current implementation inventory](../current-product.md), [user documentation](../user/index.md) and [launch status](../launch/status.md). Initial v0.1/roadmap delivery labels below preserve planning contracts; they do not establish today's implementation or permit deleting subsequently integrated features. [Managed cloud](../cloud/README.md) has separate design and gates.

**Delivery: v0.1 and roadmap.** [v0.1-build.md](v0.1-build.md) selects the current portion; [roadsmap.md](roadsmap.md) retains the remainder. Detailed schemas/configuration/ABI and full-family acceptance below describe the target superset unless narrowed by that allocation.

## Fixed decisions

| ID | Decision |
|---|---|
| D-01 | Extend pinned PocketBase; preserve its application/backend/dashboard experience |
| D-02 | MiraBase is the product name; neutral APIs stay compatible and attribution remains |
| D-03 | Go owns the backend; Turso's Rust native engine is consumed through the Go provider |
| D-04 | Default is one app DB plus auxiliary DB; private user/agent DBs are optional, not separate processes |
| D-05 | Ordinary user access uses collection policy; raw SQL/native pages are separately privileged |
| D-06 | Record-aware offline sync and native whole-DB replication are different explicit contracts |
| D-08 | SQLite frontend serves inherited generated SQL; PG is an optional additional frontend |
| D-09 | No MCP, distributed engine, mandatory organization/control plane or NoSQL frontend |
| D-10 | Other-BaaS provider migration is deferred; PocketBase adoption/schema upgrades remain required |
| D-11 | Same-runtime managed operation is a delivery boundary; monetization/license changes are not assumed |
| D-20 | PocketBase full application import includes shared-layout adoption and guided conversion into tenant/user/agent databases; schema-only import remains; other-BaaS migration stays deferred. pocketbase-tenant-import.md owns mapping, access, copy, verification and cutover. |

## Resolved integration decisions

| ID | Selected implementation | Affected requirements |
|---|---|---|
| D-07 | Go HTTP native-sync handler, pinned upstream codecs, owned native page/log adapter and shared transaction coordinator | MB-NSYNC/PARTIAL/SYNCCHECK |
| D-12 | Go source module mirabase; JS @mirabase/sdk archive; Dart mirabase path/source package; relative signed release artifacts; registry publication optional | MB-BRAND/SDK/PACKAGE |
| D-13 | Fixed engineering budgets and benchmark workload in packaging-and-performance.md; optimize full build rather than cut features | MB-PERF |
| D-14 | Go live-query service evaluates actual SQL snapshots, listens to committed CDC, computes authorized keyed result diffs and bounded replay | MB-LIVE |
| D-15 | Engine b0e1b8fde8eff66ef9843fdd1139fa3305cb16dd plus digest-pinned owned patches; no moving upstream dependency | MB-PROVIDER/EVIDENCE |
| D-16 | Go protocol-v3/TLS/SCRAM server around isolated native PgConnection sessions; no native trust server exposure | MB-PG/PGAUTH |
| D-17 | Purego plus owned ABI v1, full native build manifest and six-target qualification; expose missing primitives in the thin bridge | MB-ABI/FTS/CODEC/CDC/ASYNC |
| D-18 | Quiesced coherent copy of closed persistent state; isolated recovery verification; online snapshot is optional optimization | MB-BACKUP |
| D-19 | Go v3 pipeline/cursor handler, native SQL execution, scoped connection batons and shared transaction coordination | MB-SQLHTTP |
| D-21 | Turso canonicalizes stored schema DDL and default-expression text and lowercases `sqlite_master` index names; MiraBase recovers original-case index keys from the stored DDL and asserts each provider's exact canonical text, while behavioral parity (defaults applied, view columns/results, index presence, wrapping) is tested independently | MB-COLLECTIONS/FIELDS/MIGRATIONS |
| D-22 | Typed fields use one Go adapter per public type; native Turso uses `bigint`, `numeric(P,S)`, `uuid`, `bytea`, native temporal/network aliases, named STRUCT/UNION/DOMAIN and `TYPE[]` columns, while the portable SQLite profile uses canonical TEXT/INTEGER/REAL/BLOB with JSON composites. Fractional seconds above 3 digits use TEXT because native temporal truncates to milliseconds. Arrays use SQLite record-format BLOBs on native; `boolean[]` maps to `INTEGER[]`, domain elements map to their base scalar with application checks, and nested array SELECT text is parsed with its `X'hex'` element literals. Domain defaults are applied and validated at the application layer in addition to native DDL. Planned rebuilds preflight every row, preserve indexes/views/triggers, and recover through `_mb_schema_journal`; triggers are captured in the journal for rename and crash recovery. | MB-TYPES/ARRAY/DEFAULTS/COLLECTIONS/MIGRATIONS |
| D-23 | Transaction concurrency is selected per database at startup, never per request. WAL remains the default; MVCC is opt-in and requires a native provider that verifies `experimental_mvcc` and executes `BEGIN CONCURRENT`. The MVCC writer pool defaults to 4 connections inside the global 256-connection budget, while `NonconcurrentDB` stays serialized at one connection. Native busy/conflict/cancellation/constraint/read-only/storage/schema statuses are normalized into stable classes; automatic retries occur only for replay-safe bodies and bounded attempts, never for arbitrary hook effects. Savepoint rollback truncates deferred after-success callbacks registered after the savepoint. Idempotency receipts are committed in the target transaction and scoped by actor, database generation and operation. Durable jobs use 30-second leases renewed every 10 seconds, deterministic committed steps, cancellation reconciliation and a default maximum of 8 attempts. Outbox intents are committed with their data, encrypted with AES-256-GCM and owner-private local keys, and delivered at least once with event IDs as the consumer deduplication key. Profile changes require maintenance, drained leases and a verified backup. | MB-MVCC/RECOVERY/JOBS |

The selected integration strategies are specified in [integration contracts](integration-contracts.md) and engine-reference.md. Every runtime requirement starts at specified and requires its own executed evidence.

## Known constraints, not hidden scope reductions

Materialized views with MVCC/attached DBs, custom codecs with MVCC/multi-process/ATTACH/partial sync, FTS on WASM, STORED generated columns, unrestricted PG compatibility and mature ANN are not established by the current engine. Exact constraints are in engine-reference.md.

Where the product promised a combination, the engineering team must fix/adapt the engine and qualify it; a permanent rejection does not fulfill that promise. Where the capability was never present upstream (for example mature ANN), do not invent it under "all current strengths." The required exact vector search remains in scope.

## Changing decisions

Record evidence, affected requirement/operation/storage/schema/SDK/UI/test contracts, compatibility/upgrade effects and user-approved scope changes. A code comment or an AI response cannot quietly supersede these decisions. Reassess constraints on upstream upgrade instead of copying experimental labels forever without review.



## Complete standalone scope decisions

| ID | Binding decision |
|---|---|
| D-24 | One process supports isolated applications; each owns auth/settings/schemas/artifacts/files. Default single-app startup stays simple. |
| D-25 | Both shared-source splitting and preservation of independent PocketBase source instances are required, including append into populated instances. |
| D-26 | Guided new-application setup, user/tenant/agent distinction, memberships and frontend examples are required. |
| D-27 | Ownership/reference/relation/layout/live-plan/rollout/retention/resource closure uses implementation-closure.md. |
| D-28 | Offline stores and identity/key/runtime delivery use offline-client-delivery.md. |
| D-29 | Deliver the explicit v0.1 allocation first; retain all remaining commitments in roadsmap.md. V01 gates are the active build order; P01-P18 remain full-product completion owners. No cooperating multiprocess or distributed engine is added. |
| D-30 | AgentFS is a separately pinned full roadmap integration under agentfs-roadmap.md, including Go provider integration and optional client mount helpers; it is absent from v0.1. |
