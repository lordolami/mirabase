# Requirements and acceptance

**Current release authority:** [current implementation inventory](../current-product.md), [user documentation](../user/index.md) and [launch status](../launch/status.md). Initial v0.1/roadmap delivery labels below preserve planning contracts; they do not establish today's implementation or permit deleting subsequently integrated features. [Managed cloud](../cloud/README.md) has separate design and gates.

**Delivery: v0.1 and roadmap.** [v0.1-build.md](v0.1-build.md) selects the current portion; [roadsmap.md](roadsmap.md) retains the remainder. Detailed schemas/configuration/ABI and full-family acceptance below describe the target superset unless narrowed by that allocation.

Every requirement below belongs to the complete standalone product. Its Delivery label selects v0.1, a precise v0.1 slice, or roadmap work; the full-family Completion phase is not the active release order. The [implementation phases](implementation-workstreams.md) specify dependency order. Each requirement must be joined to actual implementation and executed acceptance as the build proceeds. A written requirement or scenario establishes no runtime result.

## MB-BRAND — Complete MiraBase identity

**Delivery:** v0.1. Full applicable behavior for released features; see owning acceptance and v0.1 gates. [Release contract](v0.1-build.md#requirement-allocation).

**Owner:** [branding-and-compatibility.md](branding-and-compatibility.md). **Completion phase:** P01. **Required for the complete build:** yes.

**AT-BRAND — Complete MiraBase identity**

Build CLI/UI/SDK artifacts, scan every owned surface and verify only reviewed attribution/compatibility exceptions remain.

Required evidence: actual target state; real execution record; negative control; restart/failure result where stateful.

## MB-ADOPT — PocketBase v0.41.1 adoption

**Delivery:** v0.1 slice. Complete shared-layout adoption now; tenant/user/agent conversion in R2. [Release contract](v0.1-build.md#requirement-allocation).

**Owner:** [branding-and-compatibility.md](branding-and-compatibility.md). **Completion phase:** P11. **Required for the complete build:** yes.

**AT-ADOPT — PocketBase v0.41.1 adoption**

Copy a real stopped app with auth/files/hooks/migrations; compare IDs, hashes, rules and API responses; preserve source and reject ambiguous paths.

Required evidence: actual target state; real execution record; negative control; restart/failure result where stateful.

## MB-CORE — Executable and Go framework

**Delivery:** v0.1. Full applicable behavior for released features; see owning acceptance and v0.1 gates. [Release contract](v0.1-build.md#requirement-allocation).

**Owner:** [pocketbase-foundation.md](pocketbase-foundation.md). **Completion phase:** P01. **Required for the complete build:** yes.

**AT-CORE — Executable and Go framework**

Start the ready binary and an embedded Go application with identical configured collections and exercise CRUD through both.

Required evidence: actual target state; real execution record; negative control; restart/failure result where stateful.

## MB-FIELDS — Every inherited field and modifier

**Delivery:** v0.1. Full applicable behavior for released features; see owning acceptance and v0.1 gates. [Release contract](v0.1-build.md#requirement-allocation).

**Owner:** [pocketbase-foundation.md](pocketbase-foundation.md). **Completion phase:** P03. **Required for the complete build:** yes.

**AT-FIELDS — Every inherited field and modifier**

Round-trip all field kinds and boundary validation/modifier fixtures on both providers; compare stored and returned values.

Required evidence: actual target state; real execution record; negative control; restart/failure result where stateful.

## MB-COLLECTIONS — Base/auth/view collections and schema tools

**Delivery:** v0.1. Full applicable behavior for released features; see owning acceptance and v0.1 gates. [Release contract](v0.1-build.md#requirement-allocation).

**Owner:** [pocketbase-foundation.md](pocketbase-foundation.md). **Completion phase:** P03. **Required for the complete build:** yes.

**AT-COLLECTIONS — Base/auth/view collections and schema tools**

Create/import/export/truncate/rebuild all collection kinds; preserve IDs, rules, indexes and failed-operation rollback.

Required evidence: actual target state; real execution record; negative control; restart/failure result where stateful.

## MB-CRUD — Generated CRUD/filter/sort/expand/projection

**Delivery:** v0.1. Full applicable behavior for released features; see owning acceptance and v0.1 gates. [Release contract](v0.1-build.md#requirement-allocation).

**Owner:** [pocketbase-foundation.md](pocketbase-foundation.md). **Completion phase:** P03. **Required for the complete build:** yes.

**AT-CRUD — Generated CRUD/filter/sort/expand/projection**

Replay positive and malicious query fixtures against both providers with authorized and unauthorized actors.

Required evidence: actual target state; real execution record; negative control; restart/failure result where stateful.

## MB-BATCH — Atomic inherited record batches

**Delivery:** v0.1. Full applicable behavior for released features; see owning acceptance and v0.1 gates. [Release contract](v0.1-build.md#requirement-allocation).

**Owner:** [transactions-and-concurrency.md](transactions-and-concurrency.md). **Completion phase:** P03. **Required for the complete build:** yes.

**AT-BATCH — Atomic inherited record batches**

Fail a middle mutation and verify no records/files/events commit; reject mixed-context batches before any write.

Required evidence: actual target state; real execution record; negative control; restart/failure result where stateful.

## MB-PASSWORD — Password auth and bcrypt compatibility

**Delivery:** v0.1. Full applicable behavior for released features; see owning acceptance and v0.1 gates. [Release contract](v0.1-build.md#requirement-allocation).

**Owner:** [authentication-and-security.md](authentication-and-security.md). **Completion phase:** P03. **Required for the complete build:** yes.

**AT-PASSWORD — Password auth and bcrypt compatibility**

Adopt a real hash, authenticate/refresh, reject wrong credentials and verify no hash/token material in output.

Required evidence: actual target state; real execution record; negative control; restart/failure result where stateful.

## MB-OAUTH — OAuth2 providers and redirects

**Delivery:** v0.1. Full applicable behavior for released features; see owning acceptance and v0.1 gates. [Release contract](v0.1-build.md#requirement-allocation).

**Owner:** [authentication-and-security.md](authentication-and-security.md). **Completion phase:** P03. **Required for the complete build:** yes.

**AT-OAUTH — OAuth2 providers and redirects**

Run provider callback/state/error fixtures with real local test-provider traffic and verify identity linking/redirect security.

Required evidence: actual target state; real execution record; negative control; restart/failure result where stateful.

## MB-OTP — OTP and MFA

**Delivery:** v0.1. Full applicable behavior for released features; see owning acceptance and v0.1 gates. [Release contract](v0.1-build.md#requirement-allocation).

**Owner:** [authentication-and-security.md](authentication-and-security.md). **Completion phase:** P03. **Required for the complete build:** yes.

**AT-OTP — OTP and MFA**

Exercise expiry, single use, wrong code, challenge completion and rate limits without account enumeration leaks.

Required evidence: actual target state; real execution record; negative control; restart/failure result where stateful.

## MB-ACCOUNT — Reset/verify/email-change/auth-alert flows

**Delivery:** v0.1. Full applicable behavior for released features; see owning acceptance and v0.1 gates. [Release contract](v0.1-build.md#requirement-allocation).

**Owner:** [authentication-and-security.md](authentication-and-security.md). **Completion phase:** P03. **Required for the complete build:** yes.

**AT-ACCOUNT — Reset/verify/email-change/auth-alert flows**

Execute mailed test tokens through every success/expiry/reuse case and verify committed account state and alert delivery intent.

Required evidence: actual target state; real execution record; negative control; restart/failure result where stateful.

## MB-RULES — API/auth/manage rules and superusers

**Delivery:** v0.1. Full applicable behavior for released features; see owning acceptance and v0.1 gates. [Release contract](v0.1-build.md#requirement-allocation).

**Owner:** [authentication-and-security.md](authentication-and-security.md). **Completion phase:** P03. **Required for the complete build:** yes.

**AT-RULES — API/auth/manage rules and superusers**

Evaluate all five rules plus auth/manage with null/empty/expressions and superuser/IP restrictions across old/new endpoints.

Required evidence: actual target state; real execution record; negative control; restart/failure result where stateful.

## MB-IMPERSONATE — Scoped superuser impersonation

**Delivery:** v0.1. Full applicable behavior for released features; see owning acceptance and v0.1 gates. [Release contract](v0.1-build.md#requirement-allocation).

**Owner:** [authentication-and-security.md](authentication-and-security.md). **Completion phase:** P03. **Required for the complete build:** yes.

**AT-IMPERSONATE — Scoped superuser impersonation**

Issue/expire impersonation tokens and verify target identity, duration and denied non-superuser access.

Required evidence: actual target state; real execution record; negative control; restart/failure result where stateful.

## MB-FILES — Local and S3 file behavior

**Delivery:** v0.1. Full applicable behavior for released features; see owning acceptance and v0.1 gates. [Release contract](v0.1-build.md#requirement-allocation).

**Owner:** [files-and-email.md](files-and-email.md). **Completion phase:** P03. **Required for the complete build:** yes.

**AT-FILES — Local and S3 file behavior**

Upload/update/delete/protect/range/thumbnail files on both backends, interrupt finalization and compare byte hashes.

Required evidence: actual target state; real execution record; negative control; restart/failure result where stateful.

## MB-MAIL — Mail settings/templates/tests

**Delivery:** v0.1. Full applicable behavior for released features; see owning acceptance and v0.1 gates. [Release contract](v0.1-build.md#requirement-allocation).

**Owner:** [files-and-email.md](files-and-email.md). **Completion phase:** P03. **Required for the complete build:** yes.

**AT-MAIL — Mail settings/templates/tests**

Use local SMTP/sendmail fixtures, validate links/templates and verify no duplicate notification after response loss/retry.

Required evidence: actual target state; real execution record; negative control; restart/failure result where stateful.

## MB-SSE — Inherited SSE compatibility

**Delivery:** v0.1. Full applicable behavior for released features; see owning acceptance and v0.1 gates. [Release contract](v0.1-build.md#requirement-allocation).

**Owner:** [realtime-and-cdc.md](realtime-and-cdc.md). **Completion phase:** P03. **Required for the complete build:** yes.

**AT-SSE — Inherited SSE compatibility**

Use the existing JS/Dart subscription methods, change/delete records, rollback and revoke access; inspect delivered payloads.

Required evidence: actual target state; real execution record; negative control; restart/failure result where stateful.

## MB-ADMIN — Complete inherited dashboard

**Delivery:** v0.1. Full applicable behavior for released features; see owning acceptance and v0.1 gates. [Release contract](v0.1-build.md#requirement-allocation).

**Owner:** [dashboard.md](dashboard.md). **Completion phase:** P03. **Required for the complete build:** yes.

**AT-ADMIN — Complete inherited dashboard**

Exercise collection/auth/file/log/settings/backup/cron/schema/SQL screens against the real server including error states.

Required evidence: actual target state; real execution record; negative control; restart/failure result where stateful.

## MB-LOGS — Logs/stats/health/rate/proxy controls

**Delivery:** v0.1. Full applicable behavior for released features; see owning acceptance and v0.1 gates. [Release contract](v0.1-build.md#requirement-allocation).

**Owner:** [operations.md](operations.md). **Completion phase:** P03. **Required for the complete build:** yes.

**AT-LOGS — Logs/stats/health/rate/proxy controls**

Generate requests through trusted/untrusted proxies, verify limits/stats and assert diagnostics redact secrets.

Required evidence: actual target state; real execution record; negative control; restart/failure result where stateful.

## MB-HOOKS — Go and Goja hooks/routes/helpers

**Delivery:** v0.1. Full applicable behavior for released features; see owning acceptance and v0.1 gates. [Release contract](v0.1-build.md#requirement-allocation).

**Owner:** [extensions-and-jobs.md](extensions-and-jobs.md). **Completion phase:** P03. **Required for the complete build:** yes.

**AT-HOOKS — Go and Goja hooks/routes/helpers**

Run representative inherited hook/migration/route/cron fixtures, errors and nested transactions on the Turso provider.

Required evidence: actual target state; real execution record; negative control; restart/failure result where stateful.

## MB-MIGRATIONS — Go/JS migrations and auto-generation

**Delivery:** v0.1. Full applicable behavior for released features; see owning acceptance and v0.1 gates. [Release contract](v0.1-build.md#requirement-allocation).

**Owner:** [schemas-and-views.md](schemas-and-views.md). **Completion phase:** P03. **Required for the complete build:** yes.

**AT-MIGRATIONS — Go/JS migrations and auto-generation**

Generate/apply/down/history-sync schema changes and compare data/rules/history; ensure secrets are not generated into files.

Required evidence: actual target state; real execution record; negative control; restart/failure result where stateful.

## MB-PROVIDER — Turso DBX/database/sql integration

**Delivery:** v0.1. Full applicable behavior for released features; see owning acceptance and v0.1 gates. [Release contract](v0.1-build.md#requirement-allocation).

**Owner:** [native-provider.md](native-provider.md). **Completion phase:** P02. **Required for the complete build:** yes.

**AT-PROVIDER — Turso DBX/database/sql integration**

Run inherited integration tests on data/auxiliary DBs with prepare/cancel/rows/error/FK/schema behavior; no fallback engine.

Required evidence: actual target state; real execution record; negative control; restart/failure result where stateful.

## MB-ABI — Native lifetime and build manifest

**Delivery:** v0.1. Full applicable behavior for released features; see owning acceptance and v0.1 gates. [Release contract](v0.1-build.md#requirement-allocation).

**Owner:** [native-provider.md](native-provider.md). **Completion phase:** P02. **Required for the complete build:** yes.

**AT-ABI — Native lifetime and build manifest**

Inject missing/tampered/wrong-ABI libraries and callback/cancel races; assert safe failure before app files mutate.

Required evidence: actual target state; real execution record; negative control; restart/failure result where stateful.

## MB-ASYNC — Async I/O and platform VFS

**Delivery:** v0.1. Full applicable behavior for released features; see owning acceptance and v0.1 gates. [Release contract](v0.1-build.md#requirement-allocation).

**Owner:** [native-provider.md](native-provider.md). **Completion phase:** P02. **Required for the complete build:** yes.

**AT-ASYNC — Async I/O and platform VFS**

Run platform I/O completion/cancellation/fault fixtures and verify bounded native operations and buffer lifetime.

Required evidence: actual target state; real execution record; negative control; restart/failure result where stateful.

## MB-SQL — Qualified richer relational SQL

**Delivery:** v0.1 slice. Inherited plus vector/FTS SQL now; full richer-SQL surface in R1. [Release contract](v0.1-build.md#requirement-allocation).

**Owner:** [queries-and-search.md](queries-and-search.md). **Completion phase:** P04. **Required for the complete build:** yes.

**AT-SQL — Qualified richer relational SQL**

Execute frontend-specific SQL corpus including joins/CTEs/windows/LATERAL/RETURNING/defaults/null ordering and compare declared results/errors.

Required evidence: actual target state; real execution record; negative control; restart/failure result where stateful.

## MB-MVCC — Concurrent MVCC writes

**Delivery:** v0.1. Full applicable behavior for released features; see owning acceptance and v0.1 gates. [Release contract](v0.1-build.md#requirement-allocation).

**Owner:** [transactions-and-concurrency.md](transactions-and-concurrency.md). **Completion phase:** P05. **Required for the complete build:** yes.

**AT-MVCC — Concurrent MVCC writes**

Use concurrent real record transactions for independent and conflicting rows, verify actual overlap, conflict semantics and no duplicate hook effects.

Required evidence: actual target state; real execution record; negative control; restart/failure result where stateful.

## MB-RECOVERY — WAL/savepoints/checkpoint/durability

**Delivery:** v0.1. Full applicable behavior for released features; see owning acceptance and v0.1 gates. [Release contract](v0.1-build.md#requirement-allocation).

**Owner:** [transactions-and-concurrency.md](transactions-and-concurrency.md). **Completion phase:** P05. **Required for the complete build:** yes.

**AT-RECOVERY — WAL/savepoints/checkpoint/durability**

Kill at commit/WAL boundaries, reopen and compare acknowledged effects under each advertised durability profile.

Required evidence: actual target state; real execution record; negative control; restart/failure result where stateful.

## MB-TYPES — Custom/domain/structured native types

**Delivery:** roadmap. R1; full requirement retained in the owning chapter. [Release contract](v0.1-build.md#requirement-allocation).

**Owner:** [collections-and-types.md](collections-and-types.md). **Completion phase:** P04. **Required for the complete build:** yes.

**AT-TYPES — Custom/domain/structured native types**

Create every declared type/domain/struct/union, round-trip boundaries through UI/REST/SQL/CDC/sync and reject invalid data.

Required evidence: actual target state; real execution record; negative control; restart/failure result where stateful.

## MB-ARRAY — Typed multidimensional arrays

**Delivery:** roadmap. R1; full requirement retained in the owning chapter. [Release contract](v0.1-build.md#requirement-allocation).

**Owner:** [collections-and-types.md](collections-and-types.md). **Completion phase:** P04. **Required for the complete build:** yes.

**AT-ARRAY — Typed multidimensional arrays**

Exercise valid arrays, slices/updates/functions and reject ragged/nullability/length violations without coercion.

Required evidence: actual target state; real execution record; negative control; restart/failure result where stateful.

## MB-DEFAULTS — UUID/defaults/sequences

**Delivery:** roadmap. R1; full requirement retained in the owning chapter. [Release contract](v0.1-build.md#requirement-allocation).

**Owner:** [collections-and-types.md](collections-and-types.md). **Completion phase:** P04. **Required for the complete build:** yes.

**AT-DEFAULTS — UUID/defaults/sequences**

Generate defaults under concurrent transactions and replay; verify sequence/session/rollback semantics and stable record IDs.

Required evidence: actual target state; real execution record; negative control; restart/failure result where stateful.

## MB-VECTOR — Vector fields and exact search

**Delivery:** v0.1. Full applicable behavior for released features; see owning acceptance and v0.1 gates. [Release contract](v0.1-build.md#requirement-allocation).

**Owner:** [queries-and-search.md](queries-and-search.md). **Completion phase:** P09. **Required for the complete build:** yes.

**AT-VECTOR — Vector fields and exact search**

Compare every supported encoding/metric against a brute-force authorized reference with ties, zero vectors and malformed sparse inputs.

Required evidence: actual target state; real execution record; negative control; restart/failure result where stateful.

## MB-FTS — Native full-text search

**Delivery:** v0.1. Full applicable behavior for released features; see owning acceptance and v0.1 gates. [Release contract](v0.1-build.md#requirement-allocation).

**Owner:** [queries-and-search.md](queries-and-search.md). **Completion phase:** P09. **Required for the complete build:** yes.

**AT-FTS — Native full-text search**

Test all declared tokenizers, syntax/weights/highlights/JSON, CRUD/rollback/crash and committed visibility; no unauthorized candidates.

Required evidence: actual target state; real execution record; negative control; restart/failure result where stateful.

## MB-FUZZY — Fuzzy/regex/time/network/crypto functions

**Delivery:** roadmap. R1; full requirement retained in the owning chapter. [Release contract](v0.1-build.md#requirement-allocation).

**Owner:** [queries-and-search.md](queries-and-search.md). **Completion phase:** P09. **Required for the complete build:** yes.

**AT-FUZZY — Fuzzy/regex/time/network/crypto functions**

Run typed edge fixtures and resource-exhaustion cases for advertised extension functions; reject unsafe algorithms for password use.

Required evidence: actual target state; real execution record; negative control; restart/failure result where stateful.

## MB-HYBRID — Hybrid authorized search

**Delivery:** roadmap. R1; full requirement retained in the owning chapter. [Release contract](v0.1-build.md#requirement-allocation).

**Owner:** [queries-and-search.md](queries-and-search.md). **Completion phase:** P09. **Required for the complete build:** yes.

**AT-HYBRID — Hybrid authorized search**

Compare deterministic RRF results and pagination under per-user filters and index rebuilds; expose incomplete scans.

Required evidence: actual target state; real execution record; negative control; restart/failure result where stateful.

## MB-VIEWS — Materialized View collections

**Delivery:** roadmap. R1; full requirement retained in the owning chapter. [Release contract](v0.1-build.md#requirement-allocation).

**Owner:** [schemas-and-views.md](schemas-and-views.md). **Completion phase:** P09. **Required for the complete build:** yes.

**AT-VIEWS — Materialized View collections**

Maintain supported derived results across commit/rollback/restart and reject incompatible query/MVCC/ATTACH plans before effects.

Required evidence: actual target state; real execution record; negative control; restart/failure result where stateful.

## MB-LIVE — Authorized live-query subscriptions

**Delivery:** roadmap. R6; full requirement retained in the owning chapter. [Release contract](v0.1-build.md#requirement-allocation).

**Owner:** [realtime-and-cdc.md](realtime-and-cdc.md). **Completion phase:** P09. **Required for the complete build:** yes.

**AT-LIVE — Authorized live-query subscriptions**

Prove actual SQL snapshot/CDC invalidation and keyed result diffs, updates/deletes, ordering, revoke/schema/epoch behavior and bounded replay; ordinary record SSE alone fails.

Required evidence: actual target state; real execution record; negative control; restart/failure result where stateful.

## MB-CDC — Transaction-grouped native and record CDC

**Delivery:** roadmap. R4; full requirement retained in the owning chapter. [Release contract](v0.1-build.md#requirement-allocation).

**Owner:** [realtime-and-cdc.md](realtime-and-cdc.md). **Completion phase:** P13. **Required for the complete build:** yes.

**AT-CDC — Transaction-grouped native and record CDC**

Mutate through record/SQL/PG/native sync/trigger paths and verify complete deduplicated commit frames, rollback exclusion and decoder versioning.

Required evidence: actual target state; real execution record; negative control; restart/failure result where stateful.

## MB-RETENTION — Durable cursors/expiry/redaction

**Delivery:** roadmap. R4; full requirement retained in the owning chapter. [Release contract](v0.1-build.md#requirement-allocation).

**Owner:** [realtime-and-cdc.md](realtime-and-cdc.md). **Completion phase:** P13. **Required for the complete build:** yes.

**AT-RETENTION — Durable cursors/expiry/redaction**

Expire cursors under byte/time budgets, change rules and delete records; verify tombstones/rebootstrap and no hidden before-values.

Required evidence: actual target state; real execution record; negative control; restart/failure result where stateful.

## MB-RSYNC — Offline record-aware synchronization

**Delivery:** roadmap. R4; full requirement retained in the owning chapter. [Release contract](v0.1-build.md#requirement-allocation).

**Owner:** [record-sync.md](record-sync.md). **Completion phase:** P14. **Required for the complete build:** yes.

**AT-RSYNC — Offline record-aware synchronization**

Run actual client/server DBs offline, push/pull/restart and verify server validation/hooks, receipt deduplication and unsent-edit preservation.

Required evidence: actual target state; real execution record; negative control; restart/failure result where stateful.

## MB-SYNCFILES — Offline file synchronization

**Delivery:** roadmap. R4; full requirement retained in the owning chapter. [Release contract](v0.1-build.md#requirement-allocation).

**Owner:** [record-sync.md](record-sync.md). **Completion phase:** P14. **Required for the complete build:** yes.

**AT-SYNCFILES — Offline file synchronization**

Interrupt stage/upload/commit/download, retry by hash and verify exact bytes, protection and orphan cleanup.

Required evidence: actual target state; real execution record; negative control; restart/failure result where stateful.

## MB-NSYNC — Upstream-compatible native synchronization

**Delivery:** roadmap. R4; full requirement retained in the owning chapter. [Release contract](v0.1-build.md#requirement-allocation).

**Owner:** [native-sync.md](native-sync.md). **Completion phase:** P15. **Required for the complete build:** yes.

**AT-NSYNC — Upstream-compatible native synchronization**

Use real pinned upstream clients for full bootstrap/logical push/frame pull/replay/restart and prove whole-DB authorization boundary.

Required evidence: actual target state; real execution record; negative control; restart/failure result where stateful.

## MB-PARTIAL — Selective bootstrap/lazy pages

**Delivery:** roadmap. R4; full requirement retained in the owning chapter. [Release contract](v0.1-build.md#requirement-allocation).

**Owner:** [native-sync.md](native-sync.md). **Completion phase:** P15. **Required for the complete build:** yes.

**AT-PARTIAL — Selective bootstrap/lazy pages**

Exercise prefix/query bootstrap, segments/prefetch, missing offline pages and page-level privilege; no row-security claims.

Required evidence: actual target state; real execution record; negative control; restart/failure result where stateful.

## MB-SYNCCHECK — Sync checkpoint and statistics

**Delivery:** roadmap. R4; full requirement retained in the owning chapter. [Release contract](v0.1-build.md#requirement-allocation).

**Owner:** [native-sync.md](native-sync.md). **Completion phase:** P15. **Required for the complete build:** yes.

**AT-SYNCCHECK — Sync checkpoint and statistics**

Grow WAL/revert state, checkpoint via native API, restart/reconcile and prove no pending edits lost.

Required evidence: actual target state; real execution record; negative control; restart/failure result where stateful.

## MB-PG — PostgreSQL frontend and wire access

**Delivery:** roadmap. R5; full requirement retained in the owning chapter. [Release contract](v0.1-build.md#requirement-allocation).

**Owner:** [postgresql-access.md](postgresql-access.md). **Completion phase:** P15. **Required for the complete build:** yes.

**AT-PG — PostgreSQL frontend and wire access**

Run pinned clients through auth/TLS/Simple/Extended/COPY/catalog/type fixtures and compare the declared compatibility matrix.

Required evidence: actual target state; real execution record; negative control; restart/failure result where stateful.

## MB-PGAUTH — PG credential/session isolation

**Delivery:** roadmap. R5; full requirement retained in the owning chapter. [Release contract](v0.1-build.md#requirement-allocation).

**Owner:** [postgresql-access.md](postgresql-access.md). **Completion phase:** P15. **Required for the complete build:** yes.

**AT-PGAUTH — PG credential/session isolation**

Attempt trust/TLS downgrade, revoked credentials, DB changes and pooled session leakage; every unauthorized path must fail.

Required evidence: actual target state; real execution record; negative control; restart/failure result where stateful.

## MB-SQLHTTP — Serverless SQL/HTTP adapter

**Delivery:** roadmap. R5; full requirement retained in the owning chapter. [Release contract](v0.1-build.md#requirement-allocation).

**Owner:** [api-and-sdk.md](api-and-sdk.md). **Completion phase:** P15. **Required for the complete build:** yes.

**AT-SQLHTTP — Serverless SQL/HTTP adapter**

Run upstream v3 clients through typed pipeline/cursor/baton transactions and enforce whole-DB SQL grants and resource limits.

Required evidence: actual target state; real execution record; negative control; restart/failure result where stateful.

## MB-LIFECYCLE — Database create/inspect/delete/restore

**Delivery:** v0.1. Full applicable behavior for released features; see owning acceptance and v0.1 gates. [Release contract](v0.1-build.md#requirement-allocation).

**Owner:** [storage-and-lifecycle.md](storage-and-lifecycle.md). **Completion phase:** P07. **Required for the complete build:** yes.

**AT-LIFECYCLE — Database create/inspect/delete/restore**

Interrupt every provision/publish/delete/tombstone/purge transition, reconcile after restart and inspect actual paths/hashes/state.

Required evidence: actual target state; real execution record; negative control; restart/failure result where stateful.

## MB-PARTITION — Per-user database routing

**Delivery:** v0.1. Full applicable behavior for released features; see owning acceptance and v0.1 gates. [Release contract](v0.1-build.md#requirement-allocation).

**Owner:** [partitioning.md](partitioning.md). **Completion phase:** P07. **Required for the complete build:** yes.

**AT-PARTITION — Per-user database routing**

Create colliding auth IDs in two auth collections, route concurrent REST/files/search/SSE calls and prove isolation across cold reopen.

Required evidence: actual target state; real execution record; negative control; restart/failure result where stateful.

## MB-AGENT — Per-agent databases and grants

**Delivery:** v0.1. Full applicable behavior for released features; see owning acceptance and v0.1 gates. [Release contract](v0.1-build.md#requirement-allocation).

**Owner:** [partitioning.md](partitioning.md). **Completion phase:** P07. **Required for the complete build:** yes.

**AT-AGENT — Per-agent databases and grants**

Register/revoke separate agent identities, deny forged owner/scopes and verify no inherited owner/superuser authority.

Required evidence: actual target state; real execution record; negative control; restart/failure result where stateful.

## MB-RELATIONS — Explicit shared/private relation semantics

**Delivery:** v0.1 slice. Same-context and bounded authorized primary references now; cross-private workflows in R6. [Release contract](v0.1-build.md#requirement-allocation).

**Owner:** [partitioning.md](partitioning.md). **Completion phase:** P07. **Required for the complete build:** yes.

**AT-RELATIONS — Explicit shared/private relation semantics**

Exercise within-context/global references, expand/rules/cascades and reject unsupported cross-context atomic operations before writes.

Required evidence: actual target state; real execution record; negative control; restart/failure result where stateful.

## MB-RESOURCES — Bounded handles/queues/resources

**Delivery:** v0.1. Full applicable behavior for released features; see owning acceptance and v0.1 gates. [Release contract](v0.1-build.md#requirement-allocation).

**Owner:** [storage-and-lifecycle.md](storage-and-lifecycle.md). **Completion phase:** P07. **Required for the complete build:** yes.

**AT-RESOURCES — Bounded handles/queues/resources**

Exceed handles/connections/disk/queue budgets with real leased databases, verify fair backpressure and no active handle eviction.

Required evidence: actual target state; real execution record; negative control; restart/failure result where stateful.

## MB-ROLLOUT — Template/schema fan-out

**Delivery:** v0.1 slice. Basic explicit-target, journaled fan-out now; advanced selectors/controls in R6. [Release contract](v0.1-build.md#requirement-allocation).

**Owner:** [schemas-and-views.md](schemas-and-views.md). **Completion phase:** P10. **Required for the complete build:** yes.

**AT-ROLLOUT — Template/schema fan-out**

Apply immutable revision to a fixed target set, crash/fail/retry/cancel targets and verify exact per-target schema/data/results.

Required evidence: actual target state; real execution record; negative control; restart/failure result where stateful.

## MB-ENCRYPT — Page encryption and key rotation

**Delivery:** roadmap. R7; full requirement retained in the owning chapter. [Release contract](v0.1-build.md#requirement-allocation).

**Owner:** [authentication-and-security.md](authentication-and-security.md). **Completion phase:** P13. **Required for the complete build:** yes.

**AT-ENCRYPT — Page encryption and key rotation**

Round-trip each advertised cipher on its target, inspect disk/WAL/header coverage, rotate/recover and reject wrong/missing keys.

Required evidence: actual target state; real execution record; negative control; restart/failure result where stateful.

## MB-CODEC — Custom page codec integration

**Delivery:** roadmap. R7; full requirement retained in the owning chapter. [Release contract](v0.1-build.md#requirement-allocation).

**Owner:** [native-provider.md](native-provider.md). **Completion phase:** P13. **Required for the complete build:** yes.

**AT-CODEC — Custom page codec integration**

Load a versioned test codec, recover DB/WAL and reject ABI/format and incompatible-mode combinations before mutation.

Required evidence: actual target state; real execution record; negative control; restart/failure result where stateful.

## MB-EXTENSIONS — Native function/table/VFS/index extension seams

**Delivery:** roadmap. R7; full requirement retained in the owning chapter. [Release contract](v0.1-build.md#requirement-allocation).

**Owner:** [extensions-and-jobs.md](extensions-and-jobs.md). **Completion phase:** P13. **Required for the complete build:** yes.

**AT-EXTENSIONS — Native function/table/VFS/index extension seams**

Register real qualified native extensions, validate lifecycle/ABI/limits and reject arbitrary SQLite binary loading.

Required evidence: actual target state; real execution record; negative control; restart/failure result where stateful.

## MB-BACKUP — Coherent native-aware backup/restore

**Delivery:** v0.1. Full applicable behavior for released features; see owning acceptance and v0.1 gates. [Release contract](v0.1-build.md#requirement-allocation).

**Owner:** [backups-and-recovery.md](backups-and-recovery.md). **Completion phase:** P17. **Required for the complete build:** yes.

**AT-BACKUP — Coherent native-aware backup/restore**

Restore real archives on each target/profile with files/keys/sidecars and compare auth/schema/records/hashes after injected failure.

Required evidence: actual target state; real execution record; negative control; restart/failure result where stateful.

## MB-PORTABLE — Portable export/import/server move

**Delivery:** v0.1 slice. Verified native archives and server move now; typed portable interchange in R7. [Release contract](v0.1-build.md#requirement-allocation).

**Owner:** [backups-and-recovery.md](backups-and-recovery.md). **Completion phase:** P17. **Required for the complete build:** yes.

**AT-PORTABLE — Portable export/import/server move**

Transfer a representative app to a clean host, verify IDs/typed values/files/auth and reject corrupt/traversal archives.

Required evidence: actual target state; real execution record; negative control; restart/failure result where stateful.

## MB-JOBS — Durable jobs and outbox

**Delivery:** v0.1 slice. Internal lifecycle, receipts, file/delivery intents and recovery now; general workflow/callback product in R6. [Release contract](v0.1-build.md#requirement-allocation).

**Owner:** [extensions-and-jobs.md](extensions-and-jobs.md). **Completion phase:** P05. **Required for the complete build:** yes.

**AT-JOBS — Durable jobs and outbox**

Crash after external effect before ack, expire leases and cancel/retry; verify target effects and at-least-once dedup contracts.

Required evidence: actual target state; real execution record; negative control; restart/failure result where stateful.

## MB-SDK — Remote SDK parity and additions

**Delivery:** v0.1 slice. Inherited and every released remote operation now; remaining/offline services alongside roadmap owners. [Release contract](v0.1-build.md#requirement-allocation).

**Owner:** [api-and-sdk.md](api-and-sdk.md). **Completion phase:** P16. **Required for the complete build:** yes.

**AT-SDK — Remote SDK parity and additions**

Run JS/TS/Dart public-method fixtures against real inherited/additive endpoints including concurrent context options.

Required evidence: actual target state; real execution record; negative control; restart/failure result where stateful.

## MB-LOCALSDK — Optional local sync modules

**Delivery:** roadmap. R4; full requirement retained in the owning chapter. [Release contract](v0.1-build.md#requirement-allocation).

**Owner:** [record-sync.md](record-sync.md). **Completion phase:** P14. **Required for the complete build:** yes.

**AT-LOCALSDK — Optional local sync modules**

Run supported browser/mobile/desktop/server adapters on real native/WASM stores and disclose per-target feature coverage.

Required evidence: actual target state; real execution record; negative control; restart/failure result where stateful.

## MB-UI — Every new dashboard workflow

**Delivery:** v0.1 slice. Inherited and every released workflow now; remaining screens alongside roadmap owners. [Release contract](v0.1-build.md#requirement-allocation).

**Owner:** [dashboard.md](dashboard.md). **Completion phase:** P16. **Required for the complete build:** yes.

**AT-UI — Every new dashboard workflow**

Exercise field/search/context/grant/sync/PG/key/job/backup flows with keyboard/error/stale/revoked/large-list states.

Required evidence: actual target state; real execution record; negative control; restart/failure result where stateful.

## MB-DEPLOY — Portable self-hosting/configuration

**Delivery:** v0.1. Full applicable behavior for released features; see owning acceptance and v0.1 gates. [Release contract](v0.1-build.md#requirement-allocation).

**Owner:** [configuration-and-deployment.md](configuration-and-deployment.md). **Completion phase:** P17. **Required for the complete build:** yes.

**AT-DEPLOY — Portable self-hosting/configuration**

Start fresh direct/service/container fixtures on persistent disk and validate precedence/unknown keys/TLS/readiness/restart.

Required evidence: actual target state; real execution record; negative control; restart/failure result where stateful.

## MB-PACKAGE — One-download release and target matrix

**Delivery:** v0.1. Full applicable behavior for released features; see owning acceptance and v0.1 gates. [Release contract](v0.1-build.md#requirement-allocation).

**Owner:** [packaging-and-performance.md](packaging-and-performance.md). **Completion phase:** P17. **Required for the complete build:** yes.

**AT-PACKAGE — One-download release and target matrix**

Install offline on all six targets without Go/Rust, verify secure native extraction, updater signature and full capability smoke suite.

Required evidence: actual target state; real execution record; negative control; restart/failure result where stateful.

## MB-PERF — Measured lightweight resource envelope

**Delivery:** v0.1. Full applicable behavior for released features; see owning acceptance and v0.1 gates. [Release contract](v0.1-build.md#requirement-allocation).

**Owner:** [packaging-and-performance.md](packaging-and-performance.md). **Completion phase:** P17. **Required for the complete build:** yes.

**AT-PERF — Measured lightweight resource envelope**

Publish reproducible startup/RSS/size/concurrency/cold-DB results for the full feature build against an accepted explicit budget.

Required evidence: actual target state; real execution record; negative control; restart/failure result where stateful.

## MB-OPERATE — Operator diagnostics/maintenance/upgrades

**Delivery:** v0.1. Full applicable behavior for released features; see owning acceptance and v0.1 gates. [Release contract](v0.1-build.md#requirement-allocation).

**Owner:** [operations.md](operations.md). **Completion phase:** P17. **Required for the complete build:** yes.

**AT-OPERATE — Operator diagnostics/maintenance/upgrades**

Execute documented disk/startup/retention/rollout/upgrade runbooks with real faults and unavailable dashboard.

Required evidence: actual target state; real execution record; negative control; restart/failure result where stateful.

## MB-EVIDENCE — Truthful release evidence

**Delivery:** v0.1. Full applicable behavior for released features; see owning acceptance and v0.1 gates. [Release contract](v0.1-build.md#requirement-allocation).

**Owner:** [testing-and-acceptance.md](testing-and-acceptance.md). **Completion phase:** P18. **Required for the complete build:** yes.

**AT-EVIDENCE — Truthful release evidence**

Join every requirement to code/tests/run artifacts/negative controls; missing/skipped/blocked items fail full-scope release.

Required evidence: actual target state; real execution record; negative control; restart/failure result where stateful.

## MB-PBIMPORT — Coherent full PocketBase source acquisition

**Delivery:** v0.1. Full applicable behavior for released features; see owning acceptance and v0.1 gates. [Release contract](v0.1-build.md#requirement-allocation).

**Owner:** [pocketbase-tenant-import.md](pocketbase-tenant-import.md). **Completion phase:** P11. **Required for the complete build:** yes.

**AT-PBIMPORT — Coherent full PocketBase source acquisition**

Read a real stopped PocketBase backup/directory, resume/hash uploads, recover only a private copy and detect missing remote files; schema JSON alone cannot complete adoption.

Required evidence: actual target state; real execution record; negative control; restart/failure result where stateful.

## MB-TENANTPLAN — Actual-row tenant ownership and placement planning

**Delivery:** roadmap. R2; full requirement retained in the owning chapter. [Release contract](v0.1-build.md#requirement-allocation).

**Owner:** [pocketbase-tenant-import.md](pocketbase-tenant-import.md). **Completion phase:** P11. **Required for the complete build:** yes.

**AT-TENANTPLAN — Actual-row tenant ownership and placement planning**

Partition fixtures with direct/indirect ownership, typed keys, cycles, shared/reference data and conflicts; account for every row and preserve explicit global uniqueness semantics.

Required evidence: actual target state; real execution record; negative control; restart/failure result where stateful.

## MB-TENANTACCESS — Preserved identities, memberships and adapted access rules

**Delivery:** roadmap. R2; full requirement retained in the owning chapter. [Release contract](v0.1-build.md#requirement-allocation).

**Owner:** [pocketbase-tenant-import.md](pocketbase-tenant-import.md). **Completion phase:** P11. **Required for the complete build:** yes.

**AT-TENANTACCESS — Preserved identities, memberships and adapted access rules**

Keep IDs/password login, multi-workspace memberships and protected fields; compare real source/target rule fixtures and deny unauthorized tenant/native scopes without unexplained changes.

Required evidence: actual target state; real execution record; negative control; restart/failure result where stateful.

## MB-TENANTDATA — Restartable full record and file conversion

**Delivery:** roadmap. R2; full requirement retained in the owning chapter. [Release contract](v0.1-build.md#requirement-allocation).

**Owner:** [pocketbase-tenant-import.md](pocketbase-tenant-import.md). **Completion phase:** P11. **Required for the complete build:** yes.

**AT-TENANTDATA — Restartable full record and file conversion**

Copy all planned records/relations/local and S3 files with exact IDs/value/file hashes; interrupt batches, resume without duplicates and suppress historical side effects.

Required evidence: actual target state; real execution record; negative control; restart/failure result where stateful.

## MB-TENANTCHECK — Reviewable tenant preview and verified reconciliation

**Delivery:** roadmap. R2; full requirement retained in the owning chapter. [Release contract](v0.1-build.md#requirement-allocation).

**Owner:** [pocketbase-tenant-import.md](pocketbase-tenant-import.md). **Completion phase:** P11. **Required for the complete build:** yes.

**AT-TENANTCHECK — Reviewable tenant preview and verified reconciliation**

Compute actual per-tenant counts/hashes, require zero unresolved errors, detect dropped/misassigned rows and missing files, invalidate stale plan/report approval and verify native/auth/routing fixtures.

Required evidence: actual target state; real execution record; negative control; restart/failure result where stateful.

## MB-TENANTCUTOVER — Tenant routing and guarded application cutover

**Delivery:** roadmap. R2; full requirement retained in the owning chapter. [Release contract](v0.1-build.md#requirement-allocation).

**Owner:** [pocketbase-tenant-import.md](pocketbase-tenant-import.md). **Completion phase:** P11. **Required for the complete build:** yes.

**AT-TENANTCUTOVER — Tenant routing and guarded application cutover**

Publish only the verified staged app/route epoch under generation/hash guards; route one and multiple memberships correctly and test HTTP/files/SSE/search/sync isolation.

Required evidence: actual target state; real execution record; negative control; restart/failure result where stateful.

## MB-TENANTRECOVERY — Import resume, cancellation and no-loss recovery

**Delivery:** v0.1 slice. Shared-layout source/cutover recovery now; split-layout recovery in R2. [Release contract](v0.1-build.md#requirement-allocation).

**Owner:** [pocketbase-tenant-import.md](pocketbase-tenant-import.md). **Completion phase:** P11. **Required for the complete build:** yes.

**AT-TENANTRECOVERY — Import resume, cancellation and no-loss recovery**

Kill before/after every rename/active marker, resolve journal state, preserve original source/old target and accepted new writes; reject unsafe rollback and retain verified recovery exports.

Required evidence: actual target state; real execution record; negative control; restart/failure result where stateful.

## MB-TENANTUI — Integrated tenant adoption dashboard/API/CLI flow

**Delivery:** roadmap. R2; full requirement retained in the owning chapter. [Release contract](v0.1-build.md#requirement-allocation).

**Owner:** [pocketbase-tenant-import.md](pocketbase-tenant-import.md). **Completion phase:** P11. **Required for the complete build:** yes.

**AT-TENANTUI — Integrated tenant adoption dashboard/API/CLI flow**

Exercise all three import choices, ownership/access editors, paginated previews/issues, validation, resume and cutover/recovery on real data; no queued/schema-only state may appear as full migration success.

Required evidence: actual target state; real execution record; negative control; restart/failure result where stateful.

## MB-APPLICATION — Independent application identity, routing and schemas

**Delivery:** roadmap. R3; full requirement retained in the owning chapter. [Release contract](v0.1-build.md#requirement-allocation).

**Owner:** [application-isolation.md](application-isolation.md). **Completion phase:** P06. **Required for the complete build:** yes.

**AT-APPLICATION — Independent application identity, routing and schemas**

Run three unrelated applications with colliding IDs and names; verify selected schemas and routes and reject every wrong-application selector before authentication.

Required evidence: actual target state; executed positive and negative controls; authorization on each access path; restart/cancellation/failure evidence where stateful.

## MB-APPAUTH — Application-local identity and administrative authority

**Delivery:** roadmap. R3; full requirement retained in the owning chapter. [Release contract](v0.1-build.md#requirement-allocation).

**Owner:** [application-isolation.md](application-isolation.md). **Completion phase:** P06. **Required for the complete build:** yes.

**AT-APPAUTH — Application-local identity and administrative authority**

Use identical IDs/emails and repeated token keys in different apps; deny cross-app sessions and imported-superuser instance authority while preserving local auth flows.

Required evidence: actual target state; executed positive and negative controls; authorization on each access path; restart/cancellation/failure evidence where stateful.

## MB-APPRUNTIME — Shared bounded application runtime and artifacts

**Delivery:** roadmap. R3; full requirement retained in the owning chapter. [Release contract](v0.1-build.md#requirement-allocation).

**Owner:** [application-isolation.md](application-isolation.md). **Completion phase:** P06. **Required for the complete build:** yes.

**AT-APPRUNTIME — Shared bounded application runtime and artifacts**

Execute different trusted hooks/routes/cron concurrently; verify scoped settings and effects, artifact revision fencing, bounded caches and no per-database warm runtime.

Required evidence: actual target state; executed positive and negative controls; authorization on each access path; restart/cancellation/failure evidence where stateful.

## MB-APPRECOVERY — Independent application lifecycle and recovery

**Delivery:** roadmap. R3; full requirement retained in the owning chapter. [Release contract](v0.1-build.md#requirement-allocation).

**Owner:** [application-isolation.md](application-isolation.md). **Completion phase:** P12. **Required for the complete build:** yes.

**AT-APPRECOVERY — Independent application lifecycle and recovery**

Kill at each registry/root publication step, quarantine one corrupt app and restore/delete another; compare untouched app hashes, routes, keys and generations.

Required evidence: actual target state; executed positive and negative controls; authorization on each access path; restart/cancellation/failure evidence where stateful.

## MB-ONBOARD — Complete new-application setup and examples

**Delivery:** v0.1 slice. Default and new empty user/tenant/agent layouts now; populated relocation and independent apps in R2/R3. [Release contract](v0.1-build.md#requirement-allocation).

**Owner:** [new-application-setup.md](new-application-setup.md). **Completion phase:** P08. **Required for the complete build:** yes.

**AT-ONBOARD — Complete new-application setup and examples**

Start empty through UI and HTTP/SDK in shared/user/tenant/agent layouts; verify actual provisioning, schema/rule validation, stale-plan rejection and runnable frontend examples.

Required evidence: actual target state; executed positive and negative controls; authorization on each access path; restart/cancellation/failure evidence where stateful.

## MB-MEMBERSHIP — Tenant creation, membership and invitation lifecycle

**Delivery:** v0.1 slice. Existing membership CRUD/current-policy enforcement now; built-in invitations in R8. [Release contract](v0.1-build.md#requirement-allocation).

**Owner:** [new-application-setup.md](new-application-setup.md). **Completion phase:** P08. **Required for the complete build:** yes.

**AT-MEMBERSHIP — Tenant creation, membership and invitation lifecycle**

Two accounts join one tenant and share exactly one database; test multiple memberships, invitation replay/expiry/revocation, role escalation denial and concurrent acceptance.

Required evidence: actual target state; executed positive and negative controls; authorization on each access path; restart/cancellation/failure evidence where stateful.

## MB-CONSOLIDATE — Multiple independent PocketBase source adoption

**Delivery:** roadmap. R3; full requirement retained in the owning chapter. [Release contract](v0.1-build.md#requirement-allocation).

**Owner:** [application-consolidation.md](application-consolidation.md). **Completion phase:** P12. **Required for the complete build:** yes.

**AT-CONSOLIDATE — Multiple independent PocketBase source adoption**

Consolidate three pinned sources with different schemas/auth/files/extensions; reconcile every source and prove application administrators and credentials remain isolated.

Required evidence: actual target state; executed positive and negative controls; authorization on each access path; restart/cancellation/failure evidence where stateful.

## MB-APPEND — Append and repeat import into populated instances

**Delivery:** roadmap. R3; full requirement retained in the owning chapter. [Release contract](v0.1-build.md#requirement-allocation).

**Owner:** [application-consolidation.md](application-consolidation.md). **Completion phase:** P12. **Required for the complete build:** yes.

**AT-APPEND — Append and repeat import into populated instances**

Append into a serving instance, interrupt each cutover step and retry/repeat/replace one source; preserve other apps and reject rollback over post-activation committed writes.

Required evidence: actual target state; executed positive and negative controls; authorization on each access path; restart/cancellation/failure evidence where stateful.

## MB-OWNERTRANSFER — Ownership reassignment and quarantine recovery

**Delivery:** v0.1. Full applicable behavior for released features; see owning acceptance and v0.1 gates. [Release contract](v0.1-build.md#requirement-allocation).

**Owner:** [implementation-closure.md](implementation-closure.md). **Completion phase:** P07. **Required for the complete build:** yes.

**AT-OWNERTRANSFER — Ownership reassignment and quarantine recovery**

Transfer a real populated database after owner deletion; revoke old grants/agents, establish the new owner and restart each intent step without a dual-authority interval.

Required evidence: actual target state; executed positive and negative controls; authorization on each access path; restart/cancellation/failure evidence where stateful.

## MB-REFERENCE — Versioned read-only reference-copy refresh

**Delivery:** roadmap. R6; full requirement retained in the owning chapter. [Release contract](v0.1-build.md#requirement-allocation).

**Owner:** [implementation-closure.md](implementation-closure.md). **Completion phase:** P09. **Required for the complete build:** yes.

**AT-REFERENCE — Versioned read-only reference-copy refresh**

Refresh real updates/deletes across target generations; reject secret projections and stale authorization, replay duplicate work and recover sequence checkpoints after crash.

Required evidence: actual target state; executed positive and negative controls; authorization on each access path; restart/cancellation/failure evidence where stateful.

## MB-RELWORKFLOW — Qualified relation, cascade and reporting workflows

**Delivery:** roadmap. R6; full requirement retained in the owning chapter. [Release contract](v0.1-build.md#requirement-allocation).

**Owner:** [implementation-closure.md](implementation-closure.md). **Completion phase:** P09. **Required for the complete build:** yes.

**AT-RELWORKFLOW — Qualified relation, cascade and reporting workflows**

Verify authorized bounded global/qualified expansion and actual cascade/report effects; deny cross-app references, hidden joins, unbounded fan-out and false cross-file atomicity.

Required evidence: actual target state; executed positive and negative controls; authorization on each access path; restart/cancellation/failure evidence where stateful.

## MB-LAYOUT — Recoverable storage layout transitions

**Delivery:** roadmap. R2; full requirement retained in the owning chapter. [Release contract](v0.1-build.md#requirement-allocation).

**Owner:** [implementation-closure.md](implementation-closure.md). **Completion phase:** P08. **Required for the complete build:** yes.

**AT-LAYOUT — Recoverable storage layout transitions**

Split and merge populated application fixtures with rows/files/rules; verify hashes and guarded publication, interrupt each cut point and preserve all acknowledged destination writes.

Required evidence: actual target state; executed positive and negative controls; authorization on each access path; restart/cancellation/failure evidence where stateful.

## MB-LIVEPLAN — Named live-query definition lifecycle

**Delivery:** roadmap. R6; full requirement retained in the owning chapter. [Release contract](v0.1-build.md#requirement-allocation).

**Owner:** [implementation-closure.md](implementation-closure.md). **Completion phase:** P09. **Required for the complete build:** yes.

**AT-LIVEPLAN — Named live-query definition lifecycle**

Create, publish, version and retire actual parameterized plans; deny unsafe SQL/policy bypass, terminate retired subscriptions and reject another application query identity.

Required evidence: actual target state; executed positive and negative controls; authorization on each access path; restart/cancellation/failure evidence where stateful.

## MB-ROLLOUTCTL — Scalable rollout selection and operator controls

**Delivery:** roadmap. R6; full requirement retained in the owning chapter. [Release contract](v0.1-build.md#requirement-allocation).

**Owner:** [implementation-closure.md](implementation-closure.md). **Completion phase:** P10. **Required for the complete build:** yes.

**AT-ROLLOUTCTL — Scalable rollout selection and operator controls**

Snapshot more than 10000 real generated target entries, pause/resume/retry with changed generations and restart; compare exact target revisions/results and bounded memory.

Required evidence: actual target state; executed positive and negative controls; authorization on each access path; restart/cancellation/failure evidence where stateful.

## MB-SYNCCOMPACT — Safe deletion metadata compaction

**Delivery:** roadmap. R4; full requirement retained in the owning chapter. [Release contract](v0.1-build.md#requirement-allocation).

**Owner:** [implementation-closure.md](implementation-closure.md). **Completion phase:** P14. **Required for the complete build:** yes.

**AT-SYNCCOMPACT — Safe deletion metadata compaction**

Compact after deleting real records with an old offline client; publish a new generation, preserve present record versions, deny stale resurrection and reconcile pending local edits.

Required evidence: actual target state; executed positive and negative controls; authorization on each access path; restart/cancellation/failure evidence where stateful.

## MB-FAIRNESS — Application-aware resource reservations and admission

**Delivery:** v0.1 slice. Single-app target admission/reservations now; independent-app fairness in R3/R8. [Release contract](v0.1-build.md#requirement-allocation).

**Owner:** [implementation-closure.md](implementation-closure.md). **Completion phase:** P07. **Required for the complete build:** yes.

**AT-FAIRNESS — Application-aware resource reservations and admission**

Overload one app while others perform CRUD/auth/recovery; measure connection/native/JS/disk/queue bounds and fairness and reconcile reservations after disk exhaustion and restart.

Required evidence: actual target state; executed positive and negative controls; authorization on each access path; restart/cancellation/failure evidence where stateful.

## MB-LOCALIDENTITY — Offline store identity, keys and switching

**Delivery:** roadmap. R4; full requirement retained in the owning chapter. [Release contract](v0.1-build.md#requirement-allocation).

**Owner:** [offline-client-delivery.md](offline-client-delivery.md). **Completion phase:** P14. **Required for the complete build:** yes.

**AT-LOCALIDENTITY — Offline store identity, keys and switching**

Use real stores across accounts/apps/tenants and process restarts; prove logout/switch never relabels pending edits and test key loss, storage eviction and expired uncertain groups.

Required evidence: actual target state; executed positive and negative controls; authorization on each access path; restart/cancellation/failure evidence where stateful.

## MB-APPOBS — Scoped dashboard, logs and audit visibility

**Delivery:** v0.1 slice. Authorized default-app context visibility/audit now; independent-app observability in R3/R8. [Release contract](v0.1-build.md#requirement-allocation).

**Owner:** [application-isolation.md](application-isolation.md). **Completion phase:** P16. **Required for the complete build:** yes.

**AT-APPOBS — Scoped dashboard, logs and audit visibility**

Use app superusers and an instance operator through browser/API; verify scoped listings, context-bound deep links, redacted audits and denial of other application diagnostics.

Required evidence: actual target state; executed positive and negative controls; authorization on each access path; restart/cancellation/failure evidence where stateful.

## MB-APPCONFIG — Independent settings, secrets and file configuration

**Delivery:** roadmap. R3; full requirement retained in the owning chapter. [Release contract](v0.1-build.md#requirement-allocation).

**Owner:** [application-isolation.md](application-isolation.md). **Completion phase:** P06. **Required for the complete build:** yes.

**AT-APPCONFIG — Independent settings, secrets and file configuration**

Run distinct SMTP/S3/OAuth/public URL policies in three apps with colliding file IDs; verify real file/token/cache separation and atomic settings revision invalidation.

Required evidence: actual target state; executed positive and negative controls; authorization on each access path; restart/cancellation/failure evidence where stateful.

## MB-APPBACKUP — Complete application backup and portable transfer

**Delivery:** v0.1 slice. Complete default-app backup/move now; different-app identity and peer/operator reconciliation in R3/R7. [Release contract](v0.1-build.md#requirement-allocation).

**Owner:** [application-isolation.md](application-isolation.md). **Completion phase:** P17. **Required for the complete build:** yes.

**AT-APPBACKUP — Complete application backup and portable transfer**

Restore and move one complete app into a clean instance with different app identity; reconcile auth/settings/artifacts/files and verify other app generations and operator bindings remain unchanged.

Required evidence: actual target state; executed positive and negative controls; authorization on each access path; restart/cancellation/failure evidence where stateful.

## Design coverage
| Topic | Requirement | Owning design |
|---|---|---|
| Vocabulary and deployment models | MB-APPLICATION | [application-isolation.md](application-isolation.md) |
| Simple startup | MB-ONBOARD | [new-application-setup.md](new-application-setup.md) |
| New application walkthrough and storage choice | MB-ONBOARD | [new-application-setup.md](new-application-setup.md) |
| Collection placement and ownership binding | MB-ONBOARD | [new-application-setup.md](new-application-setup.md) |
| Provisioning triggers and retries | MB-ONBOARD | [new-application-setup.md](new-application-setup.md) |
| Frontend signup/invite/join/switch examples | MB-MEMBERSHIP | [new-application-setup.md](new-application-setup.md) |
| Layout change | MB-LAYOUT | [implementation-closure.md](implementation-closure.md) |
| Application identity and roots | MB-APPLICATION | [application-isolation.md](application-isolation.md) |
| Fresh application creation | MB-APPLICATION | [application-isolation.md](application-isolation.md) |
| Pre-auth application routing | MB-APPAUTH | [application-isolation.md](application-isolation.md) |
| Independent schemas | MB-APPLICATION | [application-isolation.md](application-isolation.md) |
| Independent accounts and sessions | MB-APPAUTH | [application-isolation.md](application-isolation.md) |
| Application versus instance administrators | MB-APPAUTH | [application-isolation.md](application-isolation.md) |
| Independent settings/secrets | MB-APPCONFIG | [application-isolation.md](application-isolation.md) |
| Trusted hooks/routes/artifacts | MB-APPRUNTIME | [application-isolation.md](application-isolation.md) |
| Shared cron/jobs/outbox | MB-APPRUNTIME | [application-isolation.md](application-isolation.md) |
| Every access path context | MB-APPAUTH | [application-isolation.md](application-isolation.md) |
| File/token namespace | MB-APPCONFIG | [application-isolation.md](application-isolation.md) |
| Dashboard and audit visibility | MB-APPOBS | [application-isolation.md](application-isolation.md) |
| Application lifecycle/failure/readiness | MB-APPRECOVERY | [application-isolation.md](application-isolation.md) |
| Application backup/export/transfer | MB-APPBACKUP | [application-isolation.md](application-isolation.md) |
| Resource reservations/fairness/footprint | MB-FAIRNESS | [implementation-closure.md](implementation-closure.md) |
| Two PocketBase source arrangements | MB-CONSOLIDATE | [application-consolidation.md](application-consolidation.md) |
| Source provenance and inventory | MB-CONSOLIDATE | [application-consolidation.md](application-consolidation.md) |
| Append into populated instance | MB-APPEND | [application-consolidation.md](application-consolidation.md) |
| Source-local ID/email/admin collision handling | MB-APPAUTH | [application-consolidation.md](application-consolidation.md) |
| Publication/partial failure/post-write recovery | MB-APPEND | [application-consolidation.md](application-consolidation.md) |
| Repeat import/resume/replacement | MB-APPEND | [application-consolidation.md](application-consolidation.md) |
| Source version and extension compatibility | MB-CONSOLIDATE | [application-consolidation.md](application-consolidation.md) |
| Import UI/API/CLI/SDK verification | MB-CONSOLIDATE | [application-consolidation.md](application-consolidation.md) |
| Owner transfer/quarantine | MB-OWNERTRANSFER | [implementation-closure.md](implementation-closure.md) |
| Shared reference refresh | MB-REFERENCE | [implementation-closure.md](implementation-closure.md) |
| Qualified relations/cascades/reporting | MB-RELWORKFLOW | [implementation-closure.md](implementation-closure.md) |
| Concrete local client adapters/assets | MB-LOCALSDK | [offline-client-delivery.md](offline-client-delivery.md) |
| Local identity/logout/key/unsent edits | MB-LOCALIDENTITY | [offline-client-delivery.md](offline-client-delivery.md) |
| Receipt expiry and tombstone compaction | MB-SYNCCOMPACT | [implementation-closure.md](implementation-closure.md) |
| Named live-plan management | MB-LIVEPLAN | [implementation-closure.md](implementation-closure.md) |
| Rollout control/selection | MB-ROLLOUTCTL | [implementation-closure.md](implementation-closure.md) |
| Exact contracts and schema definitions | MB-EVIDENCE | [implementation-closure.md](implementation-closure.md) |
| Full runtime/package/security qualification | MB-EVIDENCE | [testing-and-acceptance.md](testing-and-acceptance.md) |
