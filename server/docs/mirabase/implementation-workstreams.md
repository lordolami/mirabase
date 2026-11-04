# Implementation phases and release gates

**Current release authority:** [current implementation inventory](../current-product.md), [user documentation](../user/index.md) and [launch status](../launch/status.md). Initial v0.1/roadmap delivery labels below preserve planning contracts; they do not establish today's implementation or permit deleting subsequently integrated features. [Open-source platform roadmap](../roadmap/platform/README.md) has separate design and gates.

**Delivery: v0.1 and roadmap.** [v0.1-build.md](v0.1-build.md) selects the current portion; [roadsmap.md](roadsmap.md) retains the remainder. Detailed schemas/configuration/ABI and full-family acceptance below describe the target superset unless narrowed by that allocation.

## Active v0.1 sequence

Use [v0.1-build.md](v0.1-build.md) as the release contract and [roadsmap.md](roadsmap.md) for later work. Each gate includes the required recovery, authorization, UI/SDK and tests for its actual effects; later integration gates rerun affected cases. A numbered gate is not a claim of completion.

| Gate | Build output | Required evidence |
|---|---|---|
| V01-01 | Source baseline, Go framework/executable identity, inherited UI and remote client derivation | Correct pins, untouched reference, legal notices and real branding/parity harness. |
| V01-02 | In-process Turso provider, DBX, native lifetime/async/manifest and secure payload | Actual native DBs, tamper negatives, cleanup/cancel/fault cases, no live SQLite fallback. |
| V01-03 | Complete inherited backend on Turso | All fields/collections/auth/rules/CRUD/batch/files/mail/SSE/admin/hooks/migrations and maintenance behavior, with provider-specific differences tested. |
| V01-04 | WAL/MVCC, transaction outcomes and shared safety kernel | Real concurrent writers, safe retries/savepoints, receipts, lifecycle/file/delivery journals, coherent snapshot primitive and crash recovery. |
| V01-05 | One-app private DB lifecycle, policy/grants/agents/memberships and admission | Current authority before routing, idempotent provisioning, bounded shared services, delete/quarantine/transfer/restore and no cross-context leaks. |
| V01-06 | Empty-app guided layouts, immutable templates/basic schema fan-out, context-aware clients/UI | Actual user/tenant/agent journeys, mixed revisions and per-target recovery; reject populated relocation without mutation. |
| V01-07 | Vector and native full-text search | Real field/index lifecycle, authorized exact results, concurrent writes/rebuilds, bounded cursors and profile correctness. |
| V01-08 | Full shared-layout PocketBase directory/backup adoption | Actual auth/rule/row/file/settings/artifact preservation, preview/validation, journaled cutover/recovery and no accepted-write loss. |
| V01-09 | Complete released dashboard/remote SDK, backup/restore/move, diagnostics and packages | Real browser/client workflows; full default-app state including managed DBs; six-target installs/upgrade/restore and measured footprint. |
| V01-10 | v0.1 release qualification | Every allocated acceptance joined to real symbols/runs/artifacts. No missing target, skipped case, mock or documentation check counts as runtime success. |

## Full-product completion ownership

P01-P18 below assign each original requirement family one full-product completion owner. They describe dependency ownership across v0.1 and roadmap work, not the active v0.1 execution order. A family with a release slice is not fully complete until its retained remainder passes. AgentFS uses A1-A9 in its roadmap design.

Read [build entry instructions](build-start.md), [product contract](product-contract.md), [application isolation](application-isolation.md), [new application setup](new-application-setup.md), [application consolidation](application-consolidation.md) and each affected contract before coding. Preserve the pinned PocketBase reference; implement in the selected owned source layout.

| Phase | Output | Completion requirements | Gate |
|---|---|---|---|
| P01 | Foundation and identity | MB-BRAND, MB-CORE | Pinned source ownership, Go module and complete branding; default setup and source parity harness. No capability status advances from renaming alone. |
| P02 | Native provider and lifetime | MB-PROVIDER, MB-ABI, MB-ASYNC | Owned native library/ABI, database/sql/DBX injection, payload trust, native handle cleanup/cancellation and both application databases on Turso. |
| P03 | Inherited application behavior | MB-FIELDS, MB-COLLECTIONS, MB-CRUD, MB-BATCH, MB-PASSWORD, MB-OAUTH, MB-OTP, MB-ACCOUNT, MB-RULES, MB-IMPERSONATE, MB-FILES, MB-MAIL, MB-SSE, MB-ADMIN, MB-LOGS, MB-HOOKS, MB-MIGRATIONS | Run every inherited API/UI/auth/rule/file/hook/migration fixture on the actual provider with explicit provider differences and negative controls. |
| P04 | Typed schema and SQL | MB-TYPES, MB-ARRAY, MB-DEFAULTS, MB-SQL | Field adapters, exact serialization, type/domain/composite/array/default/sequence mapping and recoverable physical schema planner. |
| P05 | Concurrency and durable primitives | MB-MVCC, MB-RECOVERY, MB-JOBS | Qualified profiles, bounded writers, safe retries, commit uncertainty, receipts, durable jobs/outbox and real crash recovery. |
| P06 | Application boundary and shared runtime | MB-APPLICATION, MB-APPAUTH, MB-APPRUNTIME, MB-APPCONFIG | Application context before authentication; isolated schemas/identities/settings/artifacts; bounded shared workers/VMs/router; correct app versus instance authority. |
| P07 | Database lifecycle and admission | MB-LIFECYCLE, MB-PARTITION, MB-AGENT, MB-RELATIONS, MB-RESOURCES, MB-OWNERTRANSFER, MB-FAIRNESS | Context resolver, bounded caches, provision/delete/restore/maintenance, memberships/grants/agents, ownership recovery and measured fair resource reservations. |
| P08 | New application setup and layout | MB-ONBOARD, MB-MEMBERSHIP, MB-LAYOUT | Complete shared/user/tenant/agent setup, actual lazy/eager provisioning, invitation/join flows, runnable frontend examples and verified layout transitions. |
| P09 | Search and derived workflows | MB-VECTOR, MB-FTS, MB-FUZZY, MB-HYBRID, MB-VIEWS, MB-LIVE, MB-LIVEPLAN, MB-REFERENCE, MB-RELWORKFLOW | Real authorized search, materialized views, named live plans, reference update/delete propagation and bounded relation/cascade/report workflows. |
| P10 | Schemas and rollout controls | MB-ROLLOUT, MB-ROLLOUTCTL | Immutable template revisions, typed plan/apply contracts, disk-backed target snapshots and pause/resume/retry with actual per-target outcomes. |
| P11 | Full PocketBase adoption | MB-ADOPT, MB-PBIMPORT, MB-TENANTPLAN, MB-TENANTACCESS, MB-TENANTDATA, MB-TENANTCHECK, MB-TENANTCUTOVER, MB-TENANTRECOVERY, MB-TENANTUI | Schema-only preservation plus shared/split full adoption, coherent source reader, account/rule/file mapping, actual verification and recoverable cutover. |
| P12 | Independent application consolidation | MB-CONSOLIDATE, MB-APPEND, MB-APPRECOVERY | Multiple unrelated sources preserved as application tenants, populated-instance append, provenance/repeat/replacement and per-app publication/recovery without changing other apps. |
| P13 | Changes, encryption and security | MB-CDC, MB-RETENTION, MB-ENCRYPT, MB-CODEC, MB-EXTENSIONS | All-write-path capture, authorized frames, purpose-bound credentials/secrets, qualified encryption/codecs and extension restrictions with crash/security evidence. |
| P14 | Offline record clients | MB-RSYNC, MB-SYNCFILES, MB-LOCALSDK, MB-LOCALIDENTITY, MB-SYNCCOMPACT | Real local stores and assets, authorized snapshots/push/pull, file sync, local account/context keys, preserved unsent edits and safe receipt/tombstone expiry recovery. |
| P15 | Native and SQL transports | MB-NSYNC, MB-PARTIAL, MB-SYNCCHECK, MB-PG, MB-PGAUTH, MB-SQLHTTP | Production authenticated native sync and partial/bootstrap/checkpoint adapters; isolated PG/TLS/SCRAM and SQL/HTTP sessions tested with real upstream clients. |
| P16 | Complete dashboard and SDK coverage | MB-UI, MB-SDK, MB-APPOBS | Every required operation available through the existing dashboard and selected clients; scoped administration, setup/import examples, audit and real browser/client evidence. |
| P17 | Operations and delivery | MB-BACKUP, MB-PORTABLE, MB-APPBACKUP, MB-DEPLOY, MB-OPERATE, MB-PACKAGE, MB-PERF | Coherent DB/application/instance backup/restore/export/move, all target packaging/upgrades/signing, full client assets, measured footprint and exercised runbooks. |
| P18 | Full acceptance and evidence | MB-EVIDENCE | Join every required family to actual symbols, tests, runs and artifacts. Resolve every promised-feature failure and qualify full supported target/profile/client matrices. |

## Per-phase evidence and failure discipline

Shared safety primitives are built at their first consumer, even when a later phase owns the full public feature. Phase P05 includes the real quiesced-copy/hash/integrity primitive needed for safe profile changes and the commit-capture seam needed by derived data. P07 extends coherent snapshot/publication to managed contexts. P08 owns a bounded transfer kernel for layout migration; P11 adds the pinned PocketBase source adapter to that same kernel. P12 extends it to complete application roots. P13 completes every-channel capture/security and P17 the full public backup/delivery matrix. A verified-backup requirement is never replaced by a boolean assertion while waiting for P17. Later integrations must rerun affected acceptance before full qualification.

A completion gate requires real target behavior, authorization negatives, cancellation/fault/restart cases and preserved artifacts for every owned requirement. Record exact source/toolchain/platform/engine digests, discovered/executed/skipped counts and actual outcomes in the ledger. A partial implementation can be recorded truthfully but cannot pass its phase gate. No missing library, skipped target, mock, zero-case test, documentation checker, interface declaration or source build alone fulfills a product requirement.

Native constraints are inputs to adapters and qualification. Use the selected Go services and thin bridge; do not wait for an unspecified upstream server or replace the product with a different platform. Profile restrictions remain explicit, and promised combinations require actual engineering closure.

## Scope

The complete build includes separate-application consolidation, both source arrangements, guided greenfield setup, tenant/user/agent storage, concrete offline delivery and all operational closures. No cooperating multiprocess application support, distributed engine, MCP or other-BaaS migration adapter is added. Normal app extensibility and the optional managed-delivery boundary do not require a cloud control plane.
