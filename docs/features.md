# MiraBase feature catalog

**One backend. A private database for every user, tenant, and agent.**

MiraBase extends the PocketBase application backend and dashboard with an in-process Turso engine and optional private database contexts. This catalog covers the inherited backend, MiraBase additions, developer tools, operations and the designed roadmap. It describes what each capability lets a developer do and where its implementation stops.

[Start an application](user/quickstart.md) · [Task guides](user/index.md) · [Actual endpoints](user/api-reference.md) · [Source and evidence](current-product.md) · [Launch status](launch/status.md)

## How to read status

| Term | Meaning |
|---|---|
| Inherited | PocketBase behavior retained as part of the backend; attribution and compatibility obligations remain. |
| Integrated | Concrete MiraBase implementation exists. This does not assert complete acceptance for every actor, platform or combination. |
| Partial | Concrete implementation exists, with an important unfinished integration identified here. |
| Unavailable | The current path rejects the requested feature or the implementation is missing. |
| Planned | Target design, not a current runtime feature. |

**Testing status:** production-focused tests have been executed, with the recorded Windows results, earlier Linux baseline and focused runtime checks available in [the evidence report](launch/status.md). Specific implementation and platform/signing/fault gates remain open; the [feature matrix](../release/feature-qualification.json) preserves those boundaries. This catalog describes individual capabilities rather than assigning a blanket acceptance verdict to every feature. Future open-source platform work is tracked separately from current standalone testing.

## Navigation

- [Run and administer](#run-and-administer): hosting, dashboard, operational controls.
- [Application backend](#application-backend): collections, fields, records, identity, files, email and realtime.
- [Private databases](#private-databases): layouts, provisioning, lifecycle, membership, grants and context.
- [Schema and retrieval](#schema-and-retrieval): plans, rollouts, search, references, workflows and reporting.
- [Changes and synchronization](#changes-and-synchronization): durable history, record sync and live queries.
- [Agents and SQL clients](#agents-and-sql-clients): workspace files, snapshots and PostgreSQL access.
- [Adoption and recovery](#adoption-and-recovery): PocketBase import, backups and portable data.
- [Developer and engine tools](#developer-and-engine-tools): jobs, extensions, SDKs, discovery and native delivery.
- [Remaining scope](#remaining-scope): partial application isolation, unavailable native features and managed-cloud design.

## Run and administer

### 1. Self-hosted backend — integrated

- Run on your own computer, VPS, dedicated server or bare metal using a compatible artifact.
- Connect a separately hosted frontend over HTTP or serve static frontend assets from the backend.
- Choose the persistent data directory and keep data outside the executable.
- Use the standalone without a MiraBase cloud account.
- Manage multiple database files inside one server process through bounded shared services.
- Use a larger machine and a verified move of complete application state when the standalone outgrows its host.
- No vendor database-count quota is imposed by self-hosted use. CPU, RAM, disk, file handles, connection limits and admission capacity remain real.

The current executable embeds a native library, verifies its compiled-in digest and extracts it to a private cache before loading. It is not currently a Wasm/wazero engine build. See [deployment](user/production.md) and [server moves](user/operations.md).

### 2. Dashboard — inherited with integrated extensions

- Set up and sign in as a superuser.
- Create and edit collections, fields, indexes, rules and records.
- Filter, sort and page records; select relations; upload and preview files.
- Administer auth collections, email settings/templates, storage, backups, logs and scheduled jobs.
- Use schema import/export, relationship diagrams, API previews and administrative SQL.
- Use the MiraBase storage-setup and PocketBase-import screens.

Not every additive API has a dedicated polished dashboard screen. The connected example client in the demo is labelled separately from the bundled dashboard. See [foundation](user/foundation.md).

### 3. Server administration — inherited/integrated

- Configure application settings, request limits and rate limits.
- Configure trusted proxies and superuser IP/subnet restrictions.
- Serve static assets and configure TLS/autocert or a reviewed reverse proxy.
- Inspect health, request logs and statistics; delete retained logs.
- Test email and configured storage connections.
- Use operator-only SQL for administrative inspection and changes.

Administrative SQL is privileged whole-database work; it does not invoke the normal record validation/hook pipeline. See [production](user/production.md).

## Application backend

### 4. Collections — inherited

- Base collections for application records.
- Auth collections for identities and authentication policies.
- Read-only view collections derived from SQL queries.
- Multiple collections and multiple auth collections.
- Collection IDs/names, schemas, indexes, API rules and system metadata.
- Schema import/export, view-query checks, truncation and deletion.

See [foundation](user/foundation.md) and the [preservation contract](mirabase/pocketbase-foundation.md).

### 5. Inherited field types — inherited

Text, number, boolean, email, URL, rich-text editor, password, date, autodate, select, JSON, relation, file and geographical point.

Applicable options include required values, length/range/pattern restrictions, allowed choices, single/multiple values, hidden/system behavior, unique indexes, relation targets/cascades, file count/size/MIME limits and image thumbnails. Autodate fields maintain configured creation/update timestamps. See [fields](user/fields.md).

### 6. Additional field adapters — partial

The managed-schema adapter registry contains signed 64-bit integers, decimal values, UUIDs, typed dates, typed times, typed timestamps, network addresses and vectors. Adapters provide concrete storage/validation conversion.

Complete core-field, dashboard, filter/sort and SDK parity for every adapter is not established. STRUCT/UNION/DOMAIN/custom type support is not a finished collection feature. The additive planner refuses its unsupported type/view/sequence operations. See [field registry](../internal/application/fieldkinds.go) and [schema guide](user/schema-and-workflows.md).

### 7. Record APIs — inherited; managed path partial

- Create, read, list, update and delete records.
- Filter, sort, page, project fields and expand relations on the inherited API.
- Multipart file uploads, supported field modifiers and field-level validation errors.
- Configured atomic batch operations within one physical database.
- Additional explicit-database list/get/create/update/delete endpoints.

**Managed CRUD gap:** `/api/mirabase/v1/db/...` checks owner/grant authority and invokes the raw record store. It does not yet apply complete inherited collection rules, forms, hooks and hidden-field handling. Its list endpoint is not full inherited pagination/filter/expand parity. See [database guide](user/databases.md), [handler](../apis/mirabase_records.go) and [current blockers](current-product.md).

### 8. Authentication and accounts — inherited

- Password authentication with configured identity fields.
- OAuth2 providers and generic OpenID Connect.
- One-time passwords and optional MFA.
- Auth refresh, email verification, password reset and email-change flows.
- Auth alerts, configured token durations and token-key invalidation.
- External-auth account associations and authorized superuser impersonation.
- Auth-management rules and protection of password/secret fields.

Provider adapters include Apple, Google, GitHub, GitLab, Facebook, Microsoft, Discord, Bitbucket, Gitea, Gitee, Instagram, Box, Kakao, Lark, Linear, LiveChat, Mailcow, Monday, Notion, Patreon, Planning Center, Spotify, Strava, Trakt, Twitch, Twitter, VK, WakaTime and Yandex, plus generic OIDC. Providers require configuration and credentials. See [authentication](user/authentication.md) and [provider source](../tools/auth/auth.go).

### 9. Collection authorization — inherited; additive integration varies

- Separate list, view, create, update and delete rules.
- Additional authentication/manage policy.
- Locked, public and expression-based rules.
- Request/auth references, relation conditions, comparisons, text matching and logical/time conditions.
- Hidden-field handling on the normal inherited record path.

Database grants, record rules and privileged SQL credentials are separate authority layers. Managed CRUD's missing policy parity is not closed by passing database-grant tests. See [rules](user/rules-and-filters.md).

### 10. Files and object storage — inherited

- Single/multiple record file fields and multipart uploads.
- Local filesystem and S3-compatible storage.
- Generated storage filenames, original filename handling and field restrictions.
- Protected file access, file tokens, streaming downloads and image thumbnails.
- File cleanup and configured backup coverage.

Remote S3 objects are not automatically included in a DB-only backup. See [files](user/files.md).

### 11. Email — inherited

SMTP/sendmail, rendered templates, verification/reset/email-change messages, OTP-related mail, auth alerts, test-email actions and trusted Go/JavaScript mail helpers. See [foundation](user/foundation.md) and [authentication](user/authentication.md).

### 12. Realtime record events — inherited

- SSE connection establishment and subscription updates.
- Collection and individual-record subscriptions.
- Authorized create/update/delete notifications.
- Reconnection and resubscription by clients.

This transient notification channel is distinct from durable history and maintained live-query results. See [realtime](user/realtime.md).

## Private databases

### 13. Storage layouts — integrated

| Layout | Unit of private data | Example |
|---|---|---|
| Shared | Application-level storage | A public product catalog. |
| User | Individual account | Alice's private notes. |
| Tenant | Team/company/workspace | A company's shared projects. |
| Agent | Bound agent identity | A research agent's records. |

The registry also represents the primary database and manual databases. Database IDs, owner bindings, display names, template bindings, generations, schema revisions, manifests and lifecycle states support routing and inspection.

A tenant can contain many users; a user can have several tenant memberships. Private database storage does not automatically create an independently isolated application runtime. See [database guide](user/databases.md).

### 14. Setup and provisioning — integrated

- Inspect current setup; draft, preview and apply a storage plan.
- Select auth collections, templates and provisioning policy.
- Lazy first-authorized-access provisioning and configured eager provisioning.
- Reuse existing owner/tenant bindings and converge concurrent first requests.
- Apply a published template before readiness.
- Require explicit selection when multiple eligible tenants exist.

Existing populated data cannot be silently repartitioned with a layout toggle. See [setup source](../internal/setup/setup.go) and [provisioning acceptance](../apis/mirabase_provision_template_test.go).

### 15. Database lifecycle — integrated, qualification incomplete

- Create/list/inspect databases; report requested, provisioning, ready and failed states.
- Maintenance, migration, restore, deletion, tombstone, purge and quarantine machinery.
- Ownership transfer and generation-aware operator actions.
- WAL/MVCC profile selection and guarded profile-switch operations.
- Retention/purge machinery for removed storage.
- Diagnostics for state/generation, open connections, active leases and disk/WAL/FTS bytes.
- Maintenance requests for integrity checks, checkpoints, vacuum, FTS optimization/rebuild and configured garbage collection; actual engine/job outcomes determine success.

Not every state is its own public action. Profile, maintenance and recovery combinations need actual engine/target acceptance. See [registry](../internal/application/registry.go), [lifecycle](../internal/application/lifecycle.go) and [operations](user/operations.md).

### 16. Membership and invitations — integrated

Configure tenant sources and membership/owner relationships; create, accept and revoke invitations; check intended identity and expiry; distinguish membership acceptance from signup/email verification. See [schema and membership guide](user/schema-and-workflows.md).

### 17. Grants and credentials — integrated

- List, put and revoke subject-bound database grants.
- Grant scopes and expiry; policy-epoch tracking.
- Issue/list/revoke scoped credentials with expiry.
- Separate record/file access from privileged SQL authority.
- Issue separate PostgreSQL-purpose credentials.

Record scopes include `record.read`, `record.write`, `files.read`, `files.write` and `database.inspect`; PostgreSQL uses SQL scopes. A selector is not a credential. See [API](user/api-reference.md).

### 18. Agent identities and immutable context — integrated

- Bind/list/revoke agent identities and associate owners/databases.
- Select an explicit database or reviewed tenant context.
- Bind request context immutably and reject invalid selection without primary fallback.
- Carry generation/schema/policy boundaries through applicable operations.
- Use immutable SDK context views during concurrent requests.

Ordinary agents do not automatically receive operator authority. See [agent guide](agents/README.md).

## Schema and retrieval

### 19. Schema plans — integrated with explicit unsupported operations

Create/inspect a plan, record destructive effects and blockers, bind a reviewed plan hash and expected schema revision, apply supported collection create/update/delete and ordinary/FTS index create/drop operations, then advance revision. Unsupported `type.define`, `type.drop`, `view.define` and `sequence.define` operations are refused by the additive planner. Inherited view collections are a separate path. See [schema source](../internal/application/schemaops.go).

### 20. Templates and rollouts — integrated

Publish/list/get templates, provision from templates, capture rollout targets, inspect status and individual outcomes, run/pause/resume/retry rollouts and retain visible mixed revisions/partial completion. A fan-out is not cross-file ACID. See [schema guide](user/schema-and-workflows.md).

### 21. Vector search — integrated

Search supplied vectors in a selected field with compatible cosine, L2, negative-dot or Jaccard metrics; validate dimension/finite components; filter/project results; apply candidate/result/deadline budgets; return ranking and completion information. Jaccard is representation-dependent. No embedding generation is bundled, and exact search is not a claim of mature ANN coverage. See [search](user/search.md).

### 22. Full-text search — integrated

Create search indexes; select fields, simple/advanced syntax, field weights and highlights; retrieve ranked matches; inspect/maintain indexes. Tokenizers include default, English stemming, raw, simple, whitespace and n-gram. Frontends must escape highlights. See [search](user/search.md).

### 23. Hybrid/fuzzy search and continuation — integrated

- Combine text/vector ranks with reciprocal-rank fusion.
- Fuzzy algorithms: Levenshtein, Damerau, optimal string alignment, Hamming, Jaro–Winkler and Soundex.
- Apply compatible distance/similarity thresholds.
- Use query/target/generation/schema/policy/snapshot-bound cursors.
- Observe completion/scanned/elapsed metadata and restart after expiry.

Defaults are 20 results, maximum 100, 10,000 candidates and a five-second deadline; default cursor leases are 60 seconds with 16 global active leases. Incomplete candidates do not guarantee a complete exact top-N. See [source](../internal/search/search.go).

### 24. Shared reference copies — integrated

Define source collections, explicit nonsecret projections, filters, target sets, deletion behavior and lag; refresh/read copies; inspect freshness, source progress and destination state; reject secret projections; report failed/quarantined targets. This distributes selected shared information without merging all private data. See [reference implementation](../internal/application/derived.go) and [guide](user/schema-and-workflows.md).

### 25. Relation workflows and reports — integrated

Define versioned delete/unlink workflows over explicit target collections and relation fields, run generation-guarded steps and inspect results. Define/rebuild/read reports across matching databases with persisted results and freshness. Partial committed effects remain possible; no distributed transaction is implied. See [guide](user/schema-and-workflows.md).

## Changes and synchronization

### 26. Durable change history — integrated, writer coverage varies

Persist ordered committed record changes, read incrementally, stream history, retain commit boundaries and durable positions, and reset/rebootstrap when retained history is unavailable. Every possible raw-SQL/PG/native writer needs separate capture evidence; a journal table alone does not establish complete CDC coverage. See [changes](user/changes-and-live.md).

### 27. Record synchronization — partial

Bootstrap snapshots/pages, associate client instances, push grouped creates/updates/deletes, pull changes, acknowledge durable positions, stage/upload file bytes and reconcile stable operation identities/receipts. Generation/schema/policy changes and expiry require recovery behavior.

Bootstrap/page handlers are currently superuser-bound. Ordinary-user offline bootstrap, complete offline client stores and upstream native whole-DB replication interoperability remain unfinished. Do not place an operator token in a frontend to bypass this. See [sync guide](user/synchronization.md).

### 28. Live queries — integrated with query-shape restrictions

Create named revisioned SQL plans with typed parameters, dependencies, unique result keys and limits; list/get/publish/retire plans; register authorized instances; consume snapshots and keyed insert/update/delete/order changes; replay within bounds; reset/cancel subscriptions. Ordinary-user shapes that cannot be safely narrowed may be refused. See [live guide](user/changes-and-live.md).

## Agents and SQL clients

### 29. Agent working files — partial AgentFS integration

Create/list owner-bound workspaces; write/read/delete files; track metadata/version/hash; create snapshots with manifests and historical bytes; validate and restore overwritten/deleted files while preserving files outside the snapshot. Server-side helpers can record explicitly instrumented tool history. Shared bounded locks coordinate workspace changes.

HTTP workspace operations are superuser-only and write string content. Snapshot content is capped at 64 MiB total. Multi-file crash atomicity is not qualified. Full upstream AgentFS SDK/KV/inode/chunk/overlay/mount/replication integration remains planned. See [workspace guide](user/agent-workspaces.md).

### 30. PostgreSQL gateway — integrated with protocol limits

Optional TLS-only listener, SCRAM authentication, scoped credentials, SQL read/write boundaries, simple and extended queries, prepared statements, parameter binding, statement/portal lifecycle, describe/execute/close/flush/sync messages, session settings, transaction/error state, cancellation, timeouts and protocol errors.

The current external-client evidence covers node-postgres, prepared queries and verified TLS. This is not the complete PostgreSQL server or its extension ecosystem. Binary formats, COPY, introspection, dialects and ORM compatibility have limits. See [PostgreSQL guide](user/postgresql.md).

## Adoption and recovery

### 31. PocketBase import/adoption — integrated workflow; transformation coverage partial

Create a session; upload/check a source; analyze inventory; define/inspect plans; preview allocation; inspect tenants/issues; stage; validate; retrieve reports and report cases; explicitly cut over; resume, recover or cancel; retrieve artifacts. Batch APIs create/inspect/run/cancel multi-source work.

Shared adoption and conversion to user/tenant/agent layouts have different ownership/rule/file reconciliation requirements. Complete no-loss conversion and independent-app consolidation are not established by API presence. The pinned source baseline is PocketBase v0.41.1; arbitrary historical versions and other-BaaS adapters are not qualified. See [adoption guide](user/pocketbase-import.md).

### 32. Backups and restore — inherited/integrated; recovery qualification incomplete

Application backup create/list/upload/download/delete/restore, scheduled retention and configured local/S3 destinations; managed-target backups with manifests/checksums/file coverage, validation, dry-run and restore-to-new behavior; generation changes after restore.

The latest Windows concurrent-backup regression passes with the owned native patch, superseding its earlier failure. This does not qualify every backup/restore/fault/platform combination. Windows needs a verified stop/restore/start procedure. See [operations](user/operations.md) and [executed evidence](launch/status.md).

### 33. Portable interchange — integrated

Export/import entry points, stable-ID typed row manifests, collection checksums, optional file manifests, archive/schema validation and dry-run behavior. Source and destination format/authority still need acceptance for the actual workload; an accepted format name alone is not complete format parity. See [portable source](../internal/application/portable.go).

## Developer and engine tools

### 34. Durable jobs and receipts — integrated

Enqueue/persist jobs, lease/renew/reconcile workers, list/get jobs, request cancellation, cancel queued work, record terminal results/failure phases/completed steps, expose partial committed effects, reconcile receipts and conflicting idempotency reuse. See [job kernel](../internal/dbexec/jobs.go) and [operations](user/operations.md).

### 35. Transactions and outbox helpers — integrated infrastructure

WAL/MVCC writer pools, bound transaction callbacks, outcome classification, safe-retry boundaries, transactional outbox enqueue, pending deliveries, attempt/completion tracking and protected outbox-envelope helpers. External HTTP/mail/payment effects are not automatically exactly-once. See [extensions](user/extensions.md) and [outbox](../internal/dbexec/outbox.go).

### 36. Go/JavaScript extensions — inherited/integrated

Embed the Go framework; add hooks, models, proxies, routes, middleware, commands, transaction/file/mail/log/template helpers and tests. Goja supports trusted JS hooks/migrations, generated bindings, route/middleware registration and cron helpers. It is not Node.js or an untrusted-code tenant sandbox. See [extensions](user/extensions.md).

### 37. Cron and migrations — inherited/integrated

Application-wide five-field cron registration/list/run, versioned Go/JS migrations, migration history/up/down, schema generation and history synchronization. Hooks/migrations use MiraBase paths/names with explicit legacy compatibility. Managed-database rollout is separate from inherited application migration. See [jobs/migrations](user/jobs-and-migrations.md).

### 38. SDKs and language access — partial

- JS/TS: application client, token provider, immutable database/tenant views, generic requests, managed-record CRUD helpers, ESM build and type declarations.
- Dart: context/tenant views, generic requests and managed-record helpers through a source/path package.
- Other languages: authenticated HTTP/streaming and individually qualified compatible clients or PostgreSQL drivers.

The owned wrappers do not contain full inherited auth/files/realtime SDKs or complete offline engines. Dart uses `dart:io`; Flutter Web is not established. Registry publication and equal coverage across languages are not implied. See [SDK guide](user/sdks.md).

### 39. API discovery and documentation — integrated

Health, authenticated capabilities, OpenAPI, actual route inventory, dashboard API examples, user/agent guides, SDK examples, wire/storage contracts, source/evidence records and troubleshooting. Actual bound handlers and current executions take precedence over advertised names. See [API reference](user/api-reference.md).

### 40. Engine bridge and delivery — integrated, target qualification incomplete

Pinned Turso source; Go database/sql and DBX adaptation; native connection/statement ownership; preparation, cancellation and error translation; shared admission; compiled-in digest pins; verified extraction/cache and startup probes; no_default_driver builds; patch manifests; rejection of unexpected native changes; target-aware release/checksum tooling.

Windows amd64 and Linux amd64 payload files exist; Linux's older tested baseline does not qualify newer source/payload changes. Other required targets, hard native memory admission, complete signing/SBOM/reproducibility and broad resource/fault acceptance remain open. See [launch status](launch/status.md).

## Remaining scope

### 41. Independent applications — partial

Application create/list/get/state/delete/bootstrap endpoints and per-app directory/supervisor machinery exist. Complete independent auth/settings/hooks/files/request/background-runtime isolation is not established. `application.update`, `application.backup`, `application.restore` and `importbatch.source.list` remain explicitly unregistered. This last operation is distinct from the bound batch-level sources endpoint. See [current source inventory](current-product.md).

### 42. Unavailable native integrations and designed platform — planned/unavailable

Current native page encryption, native page-key rotation and custom codec requests are refused. The presence of a rotation route or key-reference field does not implement encrypted pages. Settings protection and outbox-envelope encryption are separate mechanisms.

Remaining standalone commitments include managed record-policy parity, complete SDK/offline/native-sync delivery, full AgentFS, independent application isolation, further schema/type adapters, hard native memory admission, additional platforms, qualified recovery and signed delivery. Wasm/wazero is an investigation, not the current provider. See [roadmap](../ROADMAP.md).

The [future open-source platform design](cloud/README.md) covers projects/environments, regions, placement, replication, stable routing, fenced failover, balancing/draining, cold DB storage/wake-up, read replicas, backup/PITR/disaster recovery, console, metrics, quotas, metering/billing, secret/key management, upgrades and migration/exit. These are not deployed features. Standalone remains one process on one host; transparent write sharding inside one hot DB and cross-database distributed transactions are not implied.

## Find the right detail

| Need | Read |
|---|---|
| Understand product breadth and limits | This catalog. |
| Perform a task | [User guides](user/index.md). |
| Call an actual route | [Bound API reference](user/api-reference.md). |
| Locate symbols and tests | [Current implementation inventory](current-product.md). |
| Decide whether a candidate is accepted | [Launch status](launch/status.md) and linked evidence. |
| Implement an outstanding obligation | [Detailed contracts](mirabase/README.md), current source and relevant tests. |
| Understand future open-source distribution | [Open-source platform roadmap](cloud/README.md). |
