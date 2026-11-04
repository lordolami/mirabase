# MiraBase roadmap

**Current release authority:** [current implementation inventory](../current-product.md), [user documentation](../user/index.md) and [launch status](../launch/status.md). Initial v0.1/roadmap delivery labels below preserve planning contracts; they do not establish today's implementation or permit deleting subsequently integrated features. [Open-source platform roadmap](../roadmap/platform/README.md) has separate design and gates.

[v0.1 build contract](v0.1-build.md) defines the current release. This roadmap preserves the rest of the complete design and adds the AgentFS integration. A roadmap entry is planned work, with its own implementation and qualification gates. It is never recorded as shipped because a native symbol, configuration key, schema or document exists.

## v0.1

The release includes the full inherited PocketBase experience on Turso; one application with optional user/tenant/agent DBs; WAL and qualified MVCC; vector and full-text search; basic templates/schema fan-out; safe lifecycle and admission; coherent backups/server moves; full shared-layout PocketBase import; dashboard, remote SDKs and packaging. Safety primitives needed by those features are included now. See the exhaustive [requirement allocation](v0.1-build.md#requirement-allocation) and operation allocation below.

## Roadmap workstreams

These are dependency groups, not release dates or implied v0.2 promises. Every group extends the same product and preserves the v0.1 upgrade contracts. The detailed chapters remain normative for their assigned later behavior.

| Group | Deliverable | Owners and dependencies | Acceptance required before advertising |
|---|---|---|---|
| R1: schemas and search | Thirteen further field kinds, native arrays/custom types/domains/STRUCT/UNION, sequences/new defaults, full richer SQL, fuzzy/regex/time/network/crypto helpers, hybrid ranking, materialized View collections | [Types](collections-and-types.md), [SQL/search](queries-and-search.md), [Schema](schemas-and-views.md), [Engine](engine-reference.md); extend qualified provider and schema planner | Exact mixed-type storage/wire/UI/SDK round trips; rebuild rollback; supported-profile matrix; authorized fuzzy/hybrid results and materialized-view consistency. No toy ANN maturity claim. |
| R2: adoption and layouts | Full ownership-graph PocketBase conversion into user/tenant/agent DBs, rule/account/membership/reference adaptation, actual per-tenant preview/validation, recoverable route cutover; split/merge of populated MiraBase layouts | [PocketBase adoption](pocketbase-tenant-import.md), [Layout closure](implementation-closure.md#safe-layout-transitions); extend the v0.1 shared source/copy/publication kernel | Every source row/file accounted for, actual hashes/rule fixtures, multi-membership/cycles/uniqueness, interrupted stage/cutover, retained source and accepted target writes. |
| R3: independent apps | Separate auth/schema/settings/secrets/artifacts/files and app-local superusers inside one process; instance authority; routing; multiple PocketBase sources, append/repeat/replacement provenance and per-source recovery | [Application isolation](application-isolation.md), [Consolidation](application-consolidation.md); shared bounded runtime; R2 for any source that is also split | Three unrelated applications with colliding IDs/emails, different schemas/hooks/storage/mail settings, wrong-app credentials, unchanged peers through backup/import/restore/failure. No process per app/DB. |
| R4: durable changes and clients | Transaction-grouped CDC across all supported writers; cursor retention/redaction; record-aware offline data/files; local runtime/identity/key support; tombstone and receipt expiry; native whole-DB sync, partial bootstrap, checkpoint/stats | [Changes](realtime-and-cdc.md), [Record sync](record-sync.md), [Native sync](native-sync.md), [Clients](offline-client-delivery.md); atomic capture and pinned native protocol | Real upstream/native/WASM clients, rollback/trigger/savepoint capture, current rules, replay gaps/response loss, offline conflicts, old-generation denial, files and no stale-client resurrection. |
| R5: SQL transports | Authenticated PostgreSQL protocol-v3/TLS/SCRAM gateway, independent native sessions and metadata reconciliation; SQL/HTTP v3 pipelines/cursors/batons | [PG](postgresql-access.md), [Integration](integration-contracts.md); all-path transaction coordination/capture and scoped credentials | Real psql/driver/GUI/serverless clients; session/cancel/TLS/auth isolation; prepared/conditional transactions and unknown outcomes; no record-policy bypass. |
| R6: derived data and operations | Named live plans/keyed result subscriptions, versioned reference copies, qualified cross-private references, cascade/report workflows, scalable rollout selection/controls, richer diagnostics/audit, general durable jobs/outbox/signed callbacks | [Closure](implementation-closure.md), [Jobs](extensions-and-jobs.md), [Schema](schemas-and-views.md); R4 for commit-driven projections | Snapshot/listener race tests; source updates/deletes and lag; bounded authorized expansions; actual workflow outcomes; per-target disk-backed snapshots/pause/resume/retry; external-effect dedup/SSRF controls. |
| R7: native security and portability | Page encryption/key rotation, custom codecs and native extension loading/lifetime, complete typed portable export/import, full application export into another isolated application, remaining target/client matrix | [Security](authentication-and-security.md), [Native](native-provider.md), [Backup](backups-and-recovery.md), [Delivery](packaging-and-performance.md) | Key loss/rotation crash tests, page/WAL coverage, codec ABI/mode checks, real extension effects, typed/file hashes on independent restore and qualified target artifacts. |
| R8: complete setup and product coverage | Built-in invitations, advanced setup/layout examples, all roadmap UI and JS/TS/Dart services, per-app observability/fairness/settings, full documentation and acceptance | [Setup](new-application-setup.md), [UI](dashboard.md), [SDK](api-and-sdk.md), [Acceptance](testing-and-acceptance.md); delivered alongside each owning feature | Real frontend/browser/client workflows and failures; no dead controls, invented methods, source-only client assets or unqualified cross-language parity. |
| R9: AgentFS | Persistent agent files, JSON state, explicit tool history, workspace lifecycle/sharing, snapshots, overlay sessions/diffs/apply, remote access, optional host mounts, offline workspace sync and dashboard/SDK support | [Full AgentFS build design](agentfs-roadmap.md); builds on v0.1 identities/lifecycle/provider; R4 for offline sync and R6 for optional indexing | Source-pinned Go integration, isolation/lifetime/crash evidence, no second live engine, real mount semantics and permissions on Linux/macOS, truthful API-only Windows support, coherent recovery and qualified sync. |

No managed hosting infrastructure, billing system, distributed engine, sharding, consensus, cooperating server processes, MCP server or MongoDB frontend is introduced by this roadmap. Other-BaaS migration remains deferred outside these scheduled workstreams. A future backend rewrite must honor the same external/storage upgrade contracts; it is not a v0.1 implementation task.

## Requirement allocation and completion

Every one of the 93 existing requirement families has a delivery classification in [v0.1](v0.1-build.md#requirement-allocation). Scoped families retain their full-family remainder here. No family is deleted, renumbered or marked qualified by the split. New AgentFS families use `AF-*` identifiers in its design and are roadmap-only.

| Family with a v0.1 slice | Roadmap remainder |
|---|---|
| MB-ADOPT, MB-TENANTRECOVERY | All split layouts, ownership/rule adaptation and tenant-route cutover/recovery; shared-layout source preservation and recovery remain v0.1 gates. |
| MB-SQL | Full additional relational/native-function compatibility beyond the inherited, vector and FTS statements required by v0.1. |
| MB-RELATIONS | Explicit database-qualified cross-private references and their complete filter/expand/dependency matrix; current same-context and bounded primary references remain v0.1. |
| MB-ROLLOUT | Complete large-population selector/control surface; v0.1 still needs immutable revisions, durable target snapshots and actual retryable per-target outcomes. |
| MB-PORTABLE, MB-APPBACKUP | Typed portable interchange and importing an app under a different app identity with peer/operator reconciliation. Coherent native archives and whole default-app moves remain v0.1. |
| MB-JOBS | Public reusable workflow/actions, general signed callbacks and their delivery/secret lifecycle; required internal lifecycle and built-in delivery safety remain v0.1. |
| MB-SDK, MB-UI | Methods, types and screens for each roadmap feature; complete parity for released operations remains v0.1. |
| MB-ONBOARD, MB-MEMBERSHIP | Populated-layout transitions, independent-app bootstrap and turnkey invitation actions/examples. Empty-layout choices and current secure membership checks remain v0.1. |
| MB-FAIRNESS, MB-APPOBS | Independent-app reservations/round-robin queues, settings-aware diagnostics and cross-app operator visibility. Single-app DB admission and authorized context visibility remain v0.1. |

## Build order and upgrade contract

[Implementation workstreams](implementation-workstreams.md) gives the active v0.1 sequence and full-product completion owners. Extend shared primitives at their first consumer, then qualify each roadmap group against existing v0.1 data and clients. Do not defer authorization, backup/recovery or schema safety to another workstream when a new feature first needs them.

Preserve default application identity, DB/collection/record IDs, generations, auth/rules, files and migration history. Introduce additional apps lazily. Format/profile changes require compatible backup/migration evidence. Enabling a roadmap capability cannot silently split data, broaden grants, change uniqueness, run imported code or overwrite post-cutover commits.

Existing complete configuration, wire, ABI and storage references describe the target superset. The v0.1 [surface rules](v0.1-build.md#exact-implementation-surfaces) are normative for the released subset. Later work adds required groups through owned migrations/protocol negotiation, not stub routes or success-shaped responses. Unreleased feature selectors return a release-capability validation error; entirely unreleased routes/listeners are unregistered.

## Operation allocation

The exhaustive table below is generated from the 107 operation headings in [HTTP reference](http-operation-reference.md). `v0.1` requires the exact applicable contract; `slice` requires the stated restricted schema and semantics; `roadmap` is unregistered in v0.1. The 54 inherited routes remain required with their pinned middleware and behavior. Examples, generated OpenAPI, SDK types and dashboard controls must agree with this allocation.

| Operation | Delivery | Constraint |
|---|---|---|
| `capabilities` | v0.1 | Applicable exact contract within one default application and released features. |
| `database.list` | v0.1 | Applicable exact contract within one default application and released features. |
| `database.create` | v0.1 | Applicable exact contract within one default application and released features. |
| `database.get` | v0.1 | Applicable exact contract within one default application and released features. |
| `database.delete` | v0.1 | Applicable exact contract within one default application and released features. |
| `database.maintenance` | v0.1 | Applicable exact contract within one default application and released features. |
| `database.diagnostics` | v0.1 | Applicable exact contract within one default application and released features. |
| `database.maintain` | slice | Only released integrity/vacuum/checkpoint operations under qualified profile and maintenance guards; no sync compaction/key/codec actions. |
| `grant.list` | v0.1 | Applicable exact contract within one default application and released features. |
| `grant.put` | slice | Only record.read/write, files.read/write and database.inspect; current record rules still apply. |
| `grant.revoke` | v0.1 | Applicable exact contract within one default application and released features. |
| `agent.bind` | v0.1 | Applicable exact contract within one default application and released features. |
| `agent.revoke` | v0.1 | Applicable exact contract within one default application and released features. |
| `schema.plan` | slice | Inherited schema plus vector/FTS; no roadmap types/views/sequences. |
| `schema.apply` | slice | Same released schema subset; actual guarded journal and per-target effects. |
| `template.publish` | slice | Immutable templates containing released field/index kinds only. |
| `template.rollout` | slice | Explicit fixed IDs/generations; journaled per-target results; no structured fleet selector or advanced control API. |
| `search` | slice | Vector or text only; no fuzzy/hybrid branch. |
| `changes.pull` | roadmap | Unregistered in v0.1; full operation retained for its owning roadmap group. |
| `changes.stream` | roadmap | Unregistered in v0.1; full operation retained for its owning roadmap group. |
| `sync.bootstrap` | roadmap | Unregistered in v0.1; full operation retained for its owning roadmap group. |
| `sync.page` | roadmap | Unregistered in v0.1; full operation retained for its owning roadmap group. |
| `sync.push` | roadmap | Unregistered in v0.1; full operation retained for its owning roadmap group. |
| `sync.pull` | roadmap | Unregistered in v0.1; full operation retained for its owning roadmap group. |
| `sync.ack` | roadmap | Unregistered in v0.1; full operation retained for its owning roadmap group. |
| `sync.file.stage` | roadmap | Unregistered in v0.1; full operation retained for its owning roadmap group. |
| `sync.file.upload` | roadmap | Unregistered in v0.1; full operation retained for its owning roadmap group. |
| `live.register` | roadmap | Unregistered in v0.1; full operation retained for its owning roadmap group. |
| `live.stream` | roadmap | Unregistered in v0.1; full operation retained for its owning roadmap group. |
| `credential.issue` | roadmap | Unregistered in v0.1; full operation retained for its owning roadmap group. |
| `credential.revoke` | roadmap | Unregistered in v0.1; full operation retained for its owning roadmap group. |
| `job.get` | v0.1 | Applicable exact contract within one default application and released features. |
| `job.cancel` | v0.1 | Applicable exact contract within one default application and released features. |
| `receipt.get` | v0.1 | Applicable exact contract within one default application and released features. |
| `backup.create` | slice | Primary/private/default-application coverage for WAL/MVCC/FTS; no roadmap profile claim. |
| `backup.restore` | slice | Same released formats; new generation, authority guards and verified recovery. |
| `export` | roadmap | Unregistered in v0.1; full operation retained for its owning roadmap group. |
| `import` | roadmap | Unregistered in v0.1; full operation retained for its owning roadmap group. |
| `key.rotate` | roadmap | Unregistered in v0.1; full operation retained for its owning roadmap group. |
| `database.profile` | v0.1 | Applicable exact contract within one default application and released features. |
| `job.list` | v0.1 | Applicable exact contract within one default application and released features. |
| `template.list` | v0.1 | Applicable exact contract within one default application and released features. |
| `template.get` | v0.1 | Applicable exact contract within one default application and released features. |
| `credential.list` | roadmap | Unregistered in v0.1; full operation retained for its owning roadmap group. |
| `agent.list` | v0.1 | Applicable exact contract within one default application and released features. |
| `live.cancel` | roadmap | Unregistered in v0.1; full operation retained for its owning roadmap group. |
| `pbimport.create` | slice | Shared-layout pinned PocketBase source only; real rows/files/auth, preview, validation, cutover and recovery; no ownership split. |
| `pbimport.get` | slice | Shared-layout pinned PocketBase source only; real rows/files/auth, preview, validation, cutover and recovery; no ownership split. |
| `pbimport.source.upload` | slice | Shared-layout pinned PocketBase source only; real rows/files/auth, preview, validation, cutover and recovery; no ownership split. |
| `pbimport.source.get` | slice | Shared-layout pinned PocketBase source only; real rows/files/auth, preview, validation, cutover and recovery; no ownership split. |
| `pbimport.analyze` | slice | Shared-layout pinned PocketBase source only; real rows/files/auth, preview, validation, cutover and recovery; no ownership split. |
| `pbimport.inventory` | slice | Shared-layout pinned PocketBase source only; real rows/files/auth, preview, validation, cutover and recovery; no ownership split. |
| `pbimport.plan.put` | slice | Shared-layout pinned PocketBase source only; real rows/files/auth, preview, validation, cutover and recovery; no ownership split. |
| `pbimport.plan.get` | slice | Shared-layout pinned PocketBase source only; real rows/files/auth, preview, validation, cutover and recovery; no ownership split. |
| `pbimport.preview.compute` | slice | Shared-layout pinned PocketBase source only; real rows/files/auth, preview, validation, cutover and recovery; no ownership split. |
| `pbimport.preview.get` | slice | Shared-layout pinned PocketBase source only; real rows/files/auth, preview, validation, cutover and recovery; no ownership split. |
| `pbimport.tenants` | roadmap | Unregistered in v0.1; full operation retained for its owning roadmap group. |
| `pbimport.issues` | slice | Shared-layout pinned PocketBase source only; real rows/files/auth, preview, validation, cutover and recovery; no ownership split. |
| `pbimport.stage` | slice | Shared-layout pinned PocketBase source only; real rows/files/auth, preview, validation, cutover and recovery; no ownership split. |
| `pbimport.validate` | slice | Shared-layout pinned PocketBase source only; real rows/files/auth, preview, validation, cutover and recovery; no ownership split. |
| `pbimport.report` | slice | Shared-layout pinned PocketBase source only; real rows/files/auth, preview, validation, cutover and recovery; no ownership split. |
| `pbimport.report.cases` | slice | Shared-layout pinned PocketBase source only; real rows/files/auth, preview, validation, cutover and recovery; no ownership split. |
| `pbimport.cutover` | slice | Shared-layout pinned PocketBase source only; real rows/files/auth, preview, validation, cutover and recovery; no ownership split. |
| `pbimport.resume` | slice | Shared-layout pinned PocketBase source only; real rows/files/auth, preview, validation, cutover and recovery; no ownership split. |
| `pbimport.cancel` | slice | Shared-layout pinned PocketBase source only; real rows/files/auth, preview, validation, cutover and recovery; no ownership split. |
| `pbimport.recovery` | slice | Shared-layout pinned PocketBase source only; real rows/files/auth, preview, validation, cutover and recovery; no ownership split. |
| `pbimport.artifact.get` | slice | Shared-layout pinned PocketBase source only; real rows/files/auth, preview, validation, cutover and recovery; no ownership split. |
| `application.list` | roadmap | Unregistered in v0.1; full operation retained for its owning roadmap group. |
| `application.create` | roadmap | Unregistered in v0.1; full operation retained for its owning roadmap group. |
| `application.get` | roadmap | Unregistered in v0.1; full operation retained for its owning roadmap group. |
| `application.update` | roadmap | Unregistered in v0.1; full operation retained for its owning roadmap group. |
| `application.state` | roadmap | Unregistered in v0.1; full operation retained for its owning roadmap group. |
| `application.delete` | roadmap | Unregistered in v0.1; full operation retained for its owning roadmap group. |
| `application.bootstrap` | roadmap | Unregistered in v0.1; full operation retained for its owning roadmap group. |
| `application.backup` | roadmap | Unregistered in v0.1; full operation retained for its owning roadmap group. |
| `application.restore` | roadmap | Unregistered in v0.1; full operation retained for its owning roadmap group. |
| `importbatch.create` | roadmap | Unregistered in v0.1; full operation retained for its owning roadmap group. |
| `importbatch.get` | roadmap | Unregistered in v0.1; full operation retained for its owning roadmap group. |
| `importbatch.sources` | roadmap | Unregistered in v0.1; full operation retained for its owning roadmap group. |
| `importbatch.run` | roadmap | Unregistered in v0.1; full operation retained for its owning roadmap group. |
| `importbatch.cancel` | roadmap | Unregistered in v0.1; full operation retained for its owning roadmap group. |
| `setup.get` | slice | Default application only; shared or empty user/tenant/agent collection policy. |
| `setup.plan` | slice | No populated collection relocation, invitations or independent-app configuration. |
| `setup.apply` | slice | Validated empty-layout activation only; real memberships/provisioning required. |
| `membership.invite` | roadmap | Unregistered in v0.1; full operation retained for its owning roadmap group. |
| `membership.accept` | roadmap | Unregistered in v0.1; full operation retained for its owning roadmap group. |
| `membership.revoke` | roadmap | Unregistered in v0.1; full operation retained for its owning roadmap group. |
| `owner.transfer` | v0.1 | Applicable exact contract within one default application and released features. |
| `reference.define` | roadmap | Unregistered in v0.1; full operation retained for its owning roadmap group. |
| `reference.status` | roadmap | Unregistered in v0.1; full operation retained for its owning roadmap group. |
| `reference.refresh` | roadmap | Unregistered in v0.1; full operation retained for its owning roadmap group. |
| `layout.apply` | roadmap | Unregistered in v0.1; full operation retained for its owning roadmap group. |
| `liveplan.list` | roadmap | Unregistered in v0.1; full operation retained for its owning roadmap group. |
| `liveplan.create` | roadmap | Unregistered in v0.1; full operation retained for its owning roadmap group. |
| `liveplan.get` | roadmap | Unregistered in v0.1; full operation retained for its owning roadmap group. |
| `liveplan.publish` | roadmap | Unregistered in v0.1; full operation retained for its owning roadmap group. |
| `liveplan.retire` | roadmap | Unregistered in v0.1; full operation retained for its owning roadmap group. |
| `rollout.pause` | roadmap | Unregistered in v0.1; full operation retained for its owning roadmap group. |
| `rollout.resume` | roadmap | Unregistered in v0.1; full operation retained for its owning roadmap group. |
| `rollout.retry` | roadmap | Unregistered in v0.1; full operation retained for its owning roadmap group. |
| `rollout.targets` | roadmap | Unregistered in v0.1; full operation retained for its owning roadmap group. |
| `relationworkflow.define` | roadmap | Unregistered in v0.1; full operation retained for its owning roadmap group. |
| `relationworkflow.run` | roadmap | Unregistered in v0.1; full operation retained for its owning roadmap group. |
| `report.define` | roadmap | Unregistered in v0.1; full operation retained for its owning roadmap group. |
| `report.rebuild` | roadmap | Unregistered in v0.1; full operation retained for its owning roadmap group. |
| `importbatch.source.list` | roadmap | Unregistered in v0.1; full operation retained for its owning roadmap group. |
| `setup.administrator` | roadmap | Unregistered in v0.1; full operation retained for its owning roadmap group. |
