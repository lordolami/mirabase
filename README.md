# MiraBase v0.1

## Codebase and dependencies

**`internal/` showcases our MiraBase implementation. `server/` contains the complete, runnable MiraBase application, including inherited PocketBase code, dependencies and our additions.**

| Directory | What it contains |
|---|---|
| [internal/](internal/README.md) | A review copy of `server/internal/`: database/application services, execution/jobs, engine integration, search, live queries, PostgreSQL gateway, PocketBase import and storage setup. |
| [server/](server/README.md) | The complete application and Go framework, including the canonical `server/internal/`, inherited foundation, APIs, dashboard, SDKs, native bridge, tests, docs and build/release tools. **Build from here.** |

The top-level copy makes the implementation easy to inspect; it is not a separate backend, extra feature set or independent Go module. `server/internal/` remains canonical. Changes inside the inherited PocketBase files and additions elsewhere in `server/` are also part of MiraBase; `internal/` alone is not the entire contribution.

**Pre-existing dependencies:** PocketBase supplies the original backend/dashboard/auth/collections/files/realtime foundation; Turso supplies the database engine and upstream driver. Their source, licenses and notices retain their original authorship. The platform adapter and native payloads in the review copy also include upstream-derived material; placing a file in `internal/` does not make that dependency newly authored work. See [third-party notices](server/THIRD_PARTY_NOTICES.md), [source provenance](server/docs/mirabase/upstream-evidence.md) and the [hackathon build account](server/docs/hackathon-build.md).

**Our contribution:** the MiraBase extensions, integration changes and supporting work described below. The separate [Formal Verification project](https://github.com/lordolami/formal-verification) supplies the assurance harness; Quint is its upstream checking engine. Neither product requires a MiraBase-hosted private service. This layout identifies source ownership; it does not remove inherited dependency code from `server/`.

All source-build commands below run inside **`server/`**. Downloaded standalone packages retain their existing layout.

**One backend. A private database for every user, tenant, and agent.**

An open-source, self-hosted backend combining PocketBase simplicity with Turso’s multi-database power. Authentication, APIs, files, realtime, and private databases for users, tenants, and agents—all in one process.

## Meta AI Developer Hackathon — what we built

[Watch the three-minute demo](https://youtu.be/PVRYAADs-EQ) · [Formal Verification repository](https://github.com/lordolami/formal-verification) · [Full build account](server/docs/hackathon-build.md) · [Start MiraBase](#start-your-backend)

Our submission comprises the MiraBase extensions and integration, plus a separate [Formal Verification assurance harness](https://github.com/lordolami/formal-verification). We used **1.3-contributor for implementation** and **Muse Wonder for architecture documentation**. The account below describes our hackathon contribution and the current boundary of each implementation.

[MiraBase source](https://github.com/lordolami/mirabase) · [Formal Verification source](https://github.com/lordolami/formal-verification) · [Three-minute demo](https://youtu.be/PVRYAADs-EQ) · [Source-linked inventory](server/docs/current-product.md) · [Executed evidence](server/docs/launch/status.md)

### The problem and the product

A solo developer building a SaaS, customer portal, team workspace or multi-agent application needs accounts, permissions, persistent records, files, search, schema changes and recovery. Giving each customer or agent private storage should not require another backend deployment for every database.

MiraBase extends the existing PocketBase backend/dashboard with Turso and a multi-database application layer. Shared application data, individual user data, team/tenant data and agent data can have different storage arrangements under one backend process. Zero-infra multi-tenancy means no extra process, container, scheduler or warm JavaScript VM per database. The operator still supplies a host, persistent storage and access policies; hardware limits remain real.

### Every MiraBase addition and integration

#### Engine and delivery

1. **In-process Turso provider.** Native engine integration with Go's database/sql and DBX interfaces; data and auxiliary stores use the selected provider. Connection, statement and result ownership, preparation, execution, asynchronous progress, cancellation, cleanup and provider-error translation are part of the bridge. WAL/MVCC writer infrastructure and no_default_driver builds avoid accidentally shipping the inherited comparison provider.

2. **Native source provenance.** Pinned upstream revision, owned native fixes, patch manifests, patch/applied-diff digests and rejection of unexpected native source changes. Audit checkouts and upstream notices are preserved.

3. **Payload trust.** Expected digests compiled into the executable, verified extraction/cache handling, pin-aware loaded-library caching, startup probes, and refusal of missing/malformed pins or modified payloads. Replacing an adjacent checksum does not authorize a modified library.

4. **Standalone delivery tools.** Target-aware release builder, executable/native manifests, checksums, standalone archive builder, archived-file digest verification, launcher, welcome page, packaged documentation/frontend example, SDK tarball and clean-package checks. The bundled-public-directory regression covers archives extracted under an OS temporary directory.

#### Private databases and authority

5. **Multiple storage layouts.** Shared, database-per-user, database-per-tenant and database-per-agent arrangements, plus primary/manual entries. A tenant can have many members, and one account can belong to multiple tenants. These are database contexts within an application.

6. **Registry and manifests.** Server-generated IDs, display names, database kinds, owner/template bindings, profiles, generations, schema revisions, creation metadata, lifecycle states and private manifests. Registry state drives routing; a manifest detects mismatched files rather than granting authority.

7. **Storage setup.** Inspect current setup, draft/preview/apply plans, select authentication collections and templates, and configure provisioning policy. Added dashboard controls expose the setup workflow. Unsupported silent repartitioning of existing populated storage is refused.

8. **Automatic provisioning.** Lazy first-authorized-access and configured eager provisioning; existing-binding reuse; concurrent-request convergence; template application before readiness; durable failure state. Joining an existing tenant reuses its database; multiple eligible memberships require selection.

9. **Lifecycle controls.** Database create/list/get, ownership transfer, profile and maintenance operations; provisioning/ready/failure state; migration/restore machinery; generation guards; deletion, tombstone, purge, retention and quarantine machinery. Actual job/engine outcomes determine success.

10. **Membership and invitations.** Tenant source/owner/member relationships; invitation create/accept/revoke; intended-identity and expiry checks. Membership acceptance is distinct from signup and email verification.

11. **Database grants.** Subject-bound grants, list/put/revoke, expiry, policy epochs and record.read, record.write, files.read, files.write and database.inspect scopes. Database selection, grants, collection rules and privileged SQL are distinct authority surfaces.

12. **Scoped credentials.** Credential issue/list/revoke, expiry and selected-database authority, including separate PostgreSQL-purpose SQL credentials.

13. **Agent identities.** Bind/list/revoke agent identities and associate owners/databases. An agent binding does not confer operator privileges.

14. **Immutable context.** Explicit database and tenant selection, immutable request context, contradictory/invalid selection refusal and no silent fallback to primary after unauthorized selection. Applicable operations carry generation/schema/policy boundaries; SDK views retain their selected context across concurrent calls.

15. **Managed-record endpoints.** Explicit-database list/get/create/update/delete with database owner/grant checks. This additional path currently uses the raw store; complete inherited collection-rule/form/hook/hidden-field and query-feature parity is unfinished. The inherited record API remains separate.

#### Schemas and retrieval

16. **Additional field adapters.** Managed storage/validation conversions for int64, decimal, UUID, typed date/time/timestamp, network addresses and vectors. Complete dashboard/core-field/SDK parity for every adapter remains additional work.

17. **Reviewed schema plans.** Create/inspect plans, destructive effects and blockers, reviewed plan hashes, expected schema revisions, supported collection create/update/delete and ordinary/FTS index create/drop, with revision advancement. Unsupported type/view/sequence operations are refused by this planner.

18. **Templates.** Publish/list/get templates, bind databases to templates and provision from their published schema before readiness.

19. **Schema rollouts.** Capture targets, inspect status and target outcomes, run/pause/resume/retry, and retain mixed revisions/partial completion. Fan-out does not imply cross-file ACID.

20. **Vector search.** Caller-supplied vectors, dimension/finite checks, compatible cosine/L2/negative-dot/representation-dependent Jaccard metrics, filters, projections, candidate/result/deadline budgets and ranking/completion metadata. Embedding generation and universal ANN support are not bundled claims.

21. **Full-text search.** Search indexes, selected fields, simple/advanced syntax, field weights, ranked matches, highlights and default/English-stem/raw/simple/whitespace/ngram tokenization, with index inspection and maintenance.

22. **Hybrid retrieval.** Reciprocal-rank fusion combines lexical and vector rankings.

23. **Fuzzy retrieval.** Levenshtein, Damerau, optimal string alignment, Hamming, Jaro–Winkler and Soundex, with compatible distance/similarity thresholds.

24. **Controlled continuation.** Query/target/generation/schema/policy/snapshot-bound cursors, expiring bounded leases, deterministic ties, candidate budgets and scanned/elapsed/completion metadata. Incomplete scanning is not labelled complete exact top-N retrieval.

25. **Shared reference copies.** Define source collections, nonsecret projections, filters, destinations and deletion behavior; refresh/read copies; inspect freshness, lag, source progress and failed/quarantined targets. Secret projections are rejected.

26. **Relation workflows.** Versioned delete/unlink workflows over explicit collections/relation fields, definition inspection, generation-guarded execution and partial-outcome reporting.

27. **Reports across database contexts.** Define/inspect/rebuild reports, select matching databases, persist/read results and expose freshness.

#### Changes, sync, agents and SQL

28. **Durable record changes.** Ordered committed changes, incremental pull/stream, commit boundaries, durable positions, retained replay and reset/rebootstrap behavior. Each additional writer path still requires capture evidence.

29. **Record synchronization.** Bootstrap snapshots/pages, client instances, grouped creates/updates/deletes, push/pull/ack, file staging/upload, stable operation identities, receipts and generation/schema/policy recovery boundaries. Bootstrap/page is currently operator-bound; ordinary-client offline stores and native whole-DB protocol interoperability remain unfinished.

30. **Live-query plans.** Named revisioned SQL, typed parameters, dependencies, unique result keys, limits, list/create/get/publish/retire controls.

31. **Live subscriptions.** Authorized registration, initial result snapshots, keyed insert/update/delete/order changes, bounded replay, reset and cancellation. Ordinary-user shapes that cannot be safely narrowed may be refused.

32. **Agent workspaces.** Owner-bound workspace create/list; working-file write/read/delete; metadata, versions, content hashes and shared bounded coordination.

33. **Historical snapshots.** Snapshot manifests and historical bytes, overwritten/deleted file restoration, path/hash preflight and preservation of unrelated files. Server helpers record explicitly instrumented tool history. Current HTTP workspaces are operator-only; full upstream AgentFS and crash-atomic multi-file restore remain additional work.

34. **PostgreSQL client gateway.** Optional TLS-only listener, SCRAM, scoped SQL credentials, read/write boundaries, simple/extended queries, prepared statements, parameter binding, statements/portals, describe/execute/close/flush/sync, session settings, transaction/error state, cancellation, timeouts and protocol errors. External node-postgres testing includes verified TLS and prepared queries; this is not complete PostgreSQL server/ORM compatibility.

#### Adoption, operations and developer access

35. **PocketBase source analysis.** Adoption sessions, source upload/check/get, analysis, inventory, provenance, tenant and issue inspection, including collection/identity/settings/record/file/artifact concerns.

36. **Adoption plans.** Put/get plans, compute/get previews, destination and tenant allocation, and distinct shared-adoption versus user/tenant/agent transformation workflows.

37. **Staging and cutover.** Stage, validate, inspect reports/report cases, explicitly cut over, resume/recover/cancel sessions and retrieve artifacts. The pinned baseline is PocketBase v0.41.1; arbitrary old versions and complete no-loss transformations need their own acceptance.

38. **Multi-source batches.** Create/get batches, inspect sources, run and cancel multi-source adoption work. This does not establish independently isolated application runtime consolidation.

39. **Managed-target backups.** Per-target backups, manifests/checksums, declared file coverage, validation, dry-run and restore-to-new machinery, generation-aware restore and concurrent-write backup regression coverage.

40. **Portable interchange.** Export/import, typed row manifests, stable IDs, collection checksums, optional file manifests, archive/schema validation, dry runs and expected-generation checks. Actual format/coverage acceptance remains workload-specific.

41. **Durable jobs.** Persisted enqueueing, worker lease/renew/reconcile, list/get/cancel, terminal results, failure phases, completed steps and partial committed effects.

42. **Receipts and retries.** Stable idempotency identities, persisted receipts/lookups, unknown-outcome reconciliation, conflicting identity reuse detection, transaction outcome classification and safe-retry boundaries.

43. **Transactional outbox.** Transaction-bound enqueueing, pending deliveries, attempts/completion tracking and protected-envelope helpers. External effects are not automatically exactly-once.

44. **Maintenance.** Integrity checks, checkpoints, supported vacuum/FTS optimization/rebuild/GC requests, profile controls and generation-aware jobs.

45. **Database diagnostics.** State, generation, open connections, active leases, database/WAL/FTS bytes and failed operational work.

46. **JS/TypeScript client.** Application client, token provider, immutable database/tenant views, authenticated generic requests, managed-record CRUD helpers, ESM output, declarations and transport/path protection checks.

47. **Dart client.** Context/database/tenant views, generic requests and managed-record helpers through a source/path package. Current Dart uses dart:io; full inherited SDK/offline/Flutter Web parity is not asserted.

48. **Discovery, guides and examples.** Capabilities/OpenAPI/actual bound-route references; user/agent guides; SDK examples; context/authority/retry/recovery instructions; wire/storage contracts; troubleshooting; exact member/notes tutorial and cross-account/persistence checks.

49. **Application-registry foundations.** Create/list/get/state/delete/bootstrap APIs and directory/supervisor machinery. Full independent application auth/settings/hooks/files/runtime isolation is unfinished; update/backup/restore and the per-app import-source listing operation are not registered.

These are work areas, not a manufactured count of independently accepted features. [Current product](server/docs/current-product.md) links source and tests; [the catalog](server/docs/features.md) preserves inherited/additive/partial/planned distinctions.

### Formal Verification: the second project

**[Explore the full Formal Verification repository](https://github.com/lordolami/formal-verification).**

We built an AI-assisted assurance harness around Quint for designs, new software, existing implementations and maintenance. It connects claims to executable models, actual source symbols, live behavior and evidence. Its scope includes APIs/business logic, identity/session/token flows, schemas/transactions/migrations/recovery, coordination/sync, jobs/retries, files/backups and protocols. It does not provide universal automatic proof of arbitrary code or frontend-UI testing.

#### All 17 tool entry points

| Tool | Work delivered |
|---|---|
| fv-run | Parse, typecheck, simulate, test and bounded exhaustive verification with evidence. |
| fv-report | Aggregate recorded evidence and distinguish verdicts. |
| fv-mutate | Weaken guards in model copies and report killed/surviving mutants. |
| fv-probe | Execute claim-linked HTTP security probes. |
| fv-qualify | Test the harness against seeded verdict/model/mutation/probe examples. |
| fv-claims | Initialize/adopt/link claims, computed status, signoff/waiver, FV-0 through FV-5 and staleness. |
| fv-discover | OpenAPI/SQL/route discovery and per-surface threat-model scaffolds. |
| fv-trace | Recorded/live trace-conformance replay against model expectations. |
| fv-fault | F1–F12 fault plans and execution-record checks; does not inject faults into live systems itself. |
| fv-scan | SARIF/OSV/secrets intake as hypotheses pending executed evidence. |
| fv-cover | Model action/invariant/adversary and claim-evidence coverage. |
| fv-assurance | Structured assurance-case JSON/Markdown export. |
| fv-stale | Detect evidence/claim staleness for CI, pre-push or scheduled use. |
| fv-lint | Weak/shape-only properties, unreachable actions and adversary-declaration checks. |
| fv-map | Map model actions to existing implementation file/symbol locations. |
| fv-genprobes | Generate anonymous/BOLA/existence-oracle/mass-assignment probe matrices from discovery. |
| fv-fuzz | Schemathesis/RESTler schema-fuzz campaign adapters. |

#### Model and claim library

The **15 starter models** cover key/value staged-versus-committed state/crash recovery; leader election; idempotent jobs; token issue/use/revoke/expire/rotate; CRUD validation/auth/idempotency; leases/fencing; sync cursors/conflicts/generations; backup/restore publication/crash boundaries; file protection/publication/deletion; tenant isolation against forged hints; privileged boundaries; query/subscription/index visibility; queues/leases/poison handling/payload budgets; migration/constraint integrity; and secrets/artifact/lockout/step-up/cache boundaries.

The **42-claim catalog** has 38 core and four extension claims, plus domain packs for multi-tenant SaaS, payments ledgers, synchronization engines and healthcare records. Examples cover a todo API, secure/insecure probe targets, trace replay and fault execution records; CI templates and self-qualification tooling are included.

Evidence records methods, bounds and provenance. PASS with exhaustive checking is limited to recorded bounds; PASS-WITHOUT-EXHAUSTIVE remains distinct. Zero-test/vacuous runs, missing checks and surviving mutants are not hidden. These are harness capabilities, not a claim that every MiraBase behavior has been formally proven.

### Tests, real demonstrations and fixes

Recorded Windows native execution has **5,236 test/subtest passes**, with four archive assertions passing separately under authorized symlink privileges. The earlier Linux baseline has **5,238 passes**, tied to its earlier source/payload. The current Windows ZIP has **23 named package checks** and **four SDK transport tests**. Reports bind exact hashes and execution scope; historical runs are not presented as a new full run of every later artifact.

Real checks and fixes cover template-before-ready provisioning, concurrent-write backup exports and independent archive reopening, mixed-case import inventory and missing-row negatives, native tamper rejection, SDK transport handling, verified-TLS/prepared PostgreSQL queries, workspace historical-byte recovery, ordinary-account denial/validation and restart persistence.

The three-minute film uses real dashboard footage and a separately labelled example client, returned search results, private-context grant/revoke behavior, live updates, historical workspace restoration and adoption workflows. Captions, narration, chapters and evidence/coverage records accompany it. No million-database benchmark is claimed.

### Existing foundations and timeline

PocketBase v0.41.1 (`f70b24e186df98df2e7ebc703f103cb94dc3ca63`) supplies the pre-existing backend/dashboard/auth/collections/files/realtime/extensions and upstream tests. Turso (`b0e1b8fde8eff66ef9843fdd1139fa3305cb16dd`) supplies the engine and vendored driver. Quint supplies the specification/checking engine used by the separate harness. Their authors, licenses and notices remain credited.

The feature additions and integration described here are our hackathon contribution. The event's contribution window begins October 26, 2026 at 12:24 PM ET. MiraBase's public repository was created November 4; its first commit imports a development snapshot, with subsequent real commits retained. That import is not a claim of contemporaneous incremental history for all development. The companion repository is separately linked above.

### Release and roadmap

Publication and a passing smoke test do not substitute for production qualification. The existing Windows artifact is unsigned. [Launch evidence](server/docs/launch/status.md) preserves current implementation/platform/recovery/signing obligations, including the managed-record policy gap described above.

Full offline/native sync, full AgentFS, independent application isolation, remaining schema/SDK/platform/resource/signing work and future placement/replication/routing/failover/balancing/PITR/fleet operations are [open-source roadmap work](server/ROADMAP.md). There is no MiraBase-hosted cloud or managed hosting offering.

## Download and start

The current Windows download is the **standalone release**. It is unsigned; its tested behavior and remaining production limitations are recorded in [launch status](server/docs/launch/status.md) and [release notes](server/release/standalone-2026-11-04.md).

**[Download Windows x64 ZIP](https://github.com/lordolami/mirabase/releases/download/standalone-2026-11-04/mirabase-windows-amd64.zip)** · [Release notes, SDK and source](https://github.com/lordolami/mirabase/releases/tag/standalone-2026-11-04) · [Download/start guide](server/docs/user/downloads.md)

Extract the whole ZIP, then run `start-mirabase.cmd` or `.\mirabase.exe serve --http=127.0.0.1:8090`. The dashboard and native payload are included. Running it needs no Go/Rust/Node; the Windows engine requires the [Microsoft VC++ x64 runtime](server/docs/user/downloads.md).

## What users get

Configure the backend in the existing dashboard, connect your frontend, and extend application-specific behavior through Go or JavaScript when needed. A small application can start with shared storage. A larger application can give individuals, teams and agents separate database files while retaining shared backend services.

| Storage choice | Who shares the data? | Example |
|---|---|---|
| Shared | Application-defined accounts under collection rules | Product catalog or common application records. |
| User | One account's private database context | Alice's personal notes. |
| Tenant | Members of a company/team/workspace | Northstar's projects shared by its employees. |
| Agent | An explicitly bound agent identity | A research agent's records. |

A user can belong to multiple tenants. A tenant database is a storage boundary inside an application; full independent application auth/settings/hooks isolation is a separate unfinished integration. Separate files do not replace authorization.

## Explore the product

- [Run the backend](#start-your-backend).
- [Backend features](#backend-features): dashboard, data, identity, files, realtime and email.
- [MiraBase extensions](#mirabase-extensions): private storage, search, schemas, sync, agents and SQL clients.
- [Developer tools and operations](#developer-tools-and-operations).
- [Delivery and scaling](#one-process-many-databases).
- [Future open-source platform roadmap](#future-open-source-platform-roadmap).
- [Build from source](#build-from-this-folder).

## Start your backend

For the ready-to-run Windows package, follow [download and start](server/docs/user/downloads.md). The executable is unsigned and the current qualification/endpoint limits remain documented. To build from source instead, install **Git and Go 1.27.x**. The included dashboard and amd64 native payloads mean Rust/Node/npm are not needed for that first build on a compatible target. Rebuilding those components needs their separate toolchains.

```sh
git clone https://github.com/lordolami/mirabase.git
cd mirabase/server
go version
go run ./tools/release --version dev
```

Run source-build commands from `server/`, which contains `go.mod`. The top-level `internal/` directory is a review copy; the build uses `server/internal/`. The builder selects the native payload and compiles its digest pins. Plain `go build ./examples/base` is not the Turso product build.

**Windows amd64 / PowerShell:**

```powershell
.\dist\mirabase-windows-amd64.exe serve --http=127.0.0.1:8090 --publicDir=./examples/quickstart
```

**Compatible Linux amd64:** the current payload references GLIBC_2.38; its recorded baseline is Ubuntu/WSL with glibc 2.43. Musl/Alpine, older glibc and the current full Linux candidate are not qualified by that run.

```sh
./dist/mirabase-linux-amd64 serve --http=127.0.0.1:8090 --publicDir=./examples/quickstart
```

Mac/ARM payloads are not provided. See [platform details](server/docs/user/production.md). A cross-compile is not execution evidence.

Open the private setup URL printed by the server and create the first superuser. Administration is at `http://127.0.0.1:8090/_/`; the included frontend is at `http://127.0.0.1:8090/`. Follow the [complete first-application tutorial](server/docs/user/quickstart.md) to create its exact collections/rules, two member accounts, save a note and check another member cannot read it. This frontend uses ordinary member credentials and requires no external SDK installation.

The tutorial uses shared storage with owner-based record rules. Optional private storage has a separate [setup guide and current integration limits](server/docs/user/databases.md). Data defaults to `mb_data` under the working directory. Stop with Ctrl+C; restarting the same command/directory reuses that data. Public hosting needs separate TLS, account recovery, backup and operational acceptance.

## Backend features

### Dashboard and collections

- Existing administration UI, first-superuser setup, collection/record editors and auth administration.
- Base collections for data, auth collections for identities and read-only SQL view collections.
- Schema/index configuration, rule editors, relation selectors, schema import/export, relationship diagrams and generated API previews.
- Record filtering, sorting, pagination, projection, relation expansion, multipart files, field modifiers and configured atomic batches on the inherited API.
- Field-level validation errors and configurable required/hidden/system/unique behavior.

Inherited fields include text, number, boolean, email, URL, rich text, password, date, autodate, select, JSON, relation, file and geopoint. Additional managed adapters exist for int64, decimal, UUID, typed date/time/timestamp, network addresses and vectors; complete dashboard/core-field/SDK parity for every added adapter remains open.

[Foundation guide](server/docs/user/foundation.md) · [Field guide](server/docs/user/fields.md)

### Authentication and access rules

- Multiple authentication collections and configurable identity fields.
- Password, OAuth2/OpenID Connect, OTP and optional MFA.
- Refresh, verification, password reset, email changes, authentication alerts and token-lifetime configuration.
- Authorized impersonation, external-auth associations and authentication-management rules.
- Separate list/view/create/update/delete rules with locked, public or expression-based access.
- Request/auth/relation-aware filters and protected secret fields on inherited record paths.

Provider adapters include the major identity providers and generic OIDC; the [complete catalog](server/docs/features.md#8-authentication-and-accounts--inherited) lists the configured adapter families. Providers require their own credentials.

**Managed-record limitation:** the additional `/api/mirabase/v1/db/...` CRUD handlers currently check ownership/grants and invoke a raw store. Complete inherited record rules, forms, hooks and hidden-field behavior are not applied there. The normal inherited record API is separate. Do not treat private database grants as full collection-policy parity.

[Authentication](server/docs/user/authentication.md) · [Rules](server/docs/user/rules-and-filters.md) · [Implementation limits](server/docs/current-product.md)

### Files, email and realtime

- Local/S3-compatible file storage, single/multiple uploads, field limits, generated names, protected tokens, streaming downloads, thumbnails and cleanup.
- SMTP/sendmail, configurable templates, account-flow mail, test emails and extension helpers.
- SSE subscriptions to collections/records with create/update/delete notifications.
- Static frontend hosting and configurable server/network settings.

Realtime notifications, durable changes and live-query results are separate capabilities. DB-only backups do not automatically contain remote S3 objects.

[Files](server/docs/user/files.md) · [Realtime](server/docs/user/realtime.md) · [Account flows](server/docs/user/authentication.md)

## MiraBase extensions

### Private databases and provisioning

- Shared, user, tenant and agent layouts; primary and manual database kinds.
- Server-generated IDs, owner/template bindings, manifests, generation/schema tracking and lifecycle states.
- Storage setup plan/preview/apply; lazy or configured eager provisioning.
- Concurrent provisioning convergence and template application before readiness.
- Tenant membership and invitation create/accept/revoke.
- Explicit database/tenant context, owner transfer, scoped grants, expiry and revocation.
- Database create/list/inspect/delete, maintenance, profile and diagnostics operations.

Joining an existing tenant reuses its database. Several eligible memberships require a choice. Changing an existing populated layout requires reviewed transformation; it is not a silent toggle.

[Database layouts and workflow](server/docs/user/databases.md) · [Membership/schema operations](server/docs/user/schema-and-workflows.md)

### Four search modes

| Mode | Capabilities |
|---|---|
| Vector | Caller-supplied vectors; dimension/finite checks; compatible cosine, L2, negative-dot and representation-dependent Jaccard metrics. |
| Full text | Selected fields, simple/advanced syntax, weights, highlights and default/stem/raw/simple/whitespace/ngram tokenizers. |
| Hybrid | Text/vector rank combination through reciprocal-rank fusion. |
| Fuzzy | Levenshtein, Damerau, optimal string alignment, Hamming, Jaro–Winkler and Soundex with compatible thresholds. |

Search includes limits, candidate budgets, deadlines, deterministic ties, completion/scanned metadata and context-bound continuation leases. Embedding generation is not bundled. Exact vector search is not a claim of complete ANN coverage.

[Search examples and limits](server/docs/user/search.md)

### Schemas, shared references and reporting

- Plan schema changes, inspect destructive effects/blockers and apply a reviewed plan hash against an expected revision.
- Create/update/delete managed collections and create/drop supported ordinary/FTS indexes.
- Publish templates and run/pause/resume/retry database rollouts with target-level outcomes.
- Define filtered nonsecret reference copies; refresh/read copies and inspect lag/failure/quarantine state.
- Define versioned relation delete/unlink workflows and inspect partial outcomes.
- Define/rebuild/read reports across matching databases.

Unsupported custom-type/view/sequence operations in the additive planner are refused. Inherited view collections remain separate. References and workflows do not provide cross-file ACID transactions.

[Schema and derived-data guide](server/docs/user/schema-and-workflows.md)

### Durable changes, record sync and live queries

- Ordered committed record-change history with pull/stream endpoints and retained replay boundaries.
- Record bootstrap/pages, grouped mutation push, incremental pull, acknowledgements and file staging/upload.
- Named revisioned SQL live plans, typed parameters, dependencies, unique result keys, publish/retire controls and authorized registration.
- Initial live-result snapshots, keyed changes, bounded replay, resets and cancellation.

Whole-system CDC capture needs evidence for each writer. Bootstrap/page currently requires operator authority; complete ordinary-user offline stores and upstream native whole-DB sync interoperability remain open. Live-query shapes that cannot be safely narrowed for ordinary actors may be refused.

[Changes/live queries](server/docs/user/changes-and-live.md) · [Sync workflow](server/docs/user/synchronization.md)

### Agent workspaces

- Owner-bound workspace create/list and file write/read/delete.
- File metadata, versions and content hashes.
- Snapshots retaining historical bytes; hash/path checks before restoring overwritten/deleted files.
- Preservation of files outside the snapshot and explicitly instrumented tool-history helpers.

Current HTTP operations are superuser-only, use string content, and cap snapshot content at 64 MiB. Multi-file crash-atomic restore and full upstream AgentFS KV/inode/chunk/overlay/mount/replication integration remain open. Do not give an untrusted agent a superuser token.

[Workspace API and boundaries](server/docs/user/agent-workspaces.md)

### PostgreSQL client access

The optional gateway provides TLS/SCRAM, separately scoped SQL credentials, simple and extended queries, prepared statements/parameters, statement/portal/session lifecycle, transaction state, cancellation and timeouts. Actual external-client evidence includes node-postgres with verified TLS and prepared queries.

It is not a complete PostgreSQL server. COPY, binary formats, introspection, SQL dialect and ORM compatibility need specific qualification. Record rules do not constrain privileged SQL as if it were ordinary record access.

[Enable and use the PostgreSQL gateway](server/docs/user/postgresql.md)

### PocketBase adoption

- Upload/check sources; analyze collections, identities, settings, records, files and artifacts.
- Review plans, previews, destination/tenant allocation and issues.
- Stage, validate, inspect report cases and explicitly cut over.
- Resume/recover/cancel sessions and create/inspect/run/cancel multi-source batches.
- Preserve source provenance and distinguish shared adoption from per-user/tenant/agent conversion.

The pinned baseline is PocketBase v0.41.1. Complete no-loss conversion, arbitrary historical-version support and independent-app consolidation require further acceptance. Collection schema import alone is not full application adoption.

[Import procedure and recovery](server/docs/user/pocketbase-import.md)

## Developer tools and operations

### Extensions, scheduling and migrations

Go embedding, hooks, custom models/proxies/routes/middleware/commands, transaction/file/mail/log/template helpers and test utilities remain available. Goja adds trusted JavaScript hooks/migrations and route/cron bindings. Cron and migration history/up/down/generation are application-wide tools; managed schema rollout is a distinct workflow.

Trusted extensions have process privileges. Goja is not Node.js, and installed hooks are not hostile tenant-code sandboxes.

[Extensions](server/docs/user/extensions.md) · [Jobs/migrations](server/docs/user/jobs-and-migrations.md)

### SDKs and APIs

JS/TS and Dart packages provide application clients, immutable database/tenant views, generic authenticated requests and managed-record helpers. Other languages can use HTTP/streams or individually qualified compatible drivers. The normal inherited API remains under `/api/collections`, `/api/files` and `/api/realtime`; MiraBase additions use `/api/mirabase/v1`.

The owned wrappers are not complete inherited auth/files/realtime clients or offline local engines. Dart currently uses `dart:io`; Flutter Web is not established. Registry publication is not implied by local packages.

[SDK methods and examples](server/docs/user/sdks.md) · [Actual route reference](server/docs/user/api-reference.md)

### Jobs, recovery and diagnostics

- Durable jobs with enqueue/lease/renew/reconciliation, status/results, cancellation and recorded steps.
- Idempotency receipts, unknown-outcome reconciliation and transactional outbox helpers.
- Application/managed-target backups, checksums/manifests, configured file coverage, dry-run/restore-to-new behavior and portable interchange.
- Maintenance, integrity checks, checkpoints, supported vacuum/FTS/GC actions and generation-aware operations.
- Logs, statistics, disk/WAL/FTS size, open connections, leases and failed-job inspection.

Recovery remains subject to open acceptance gates. The latest Windows concurrent-backup regression passes; it does not establish full platform/fault coverage. Native page encryption/key rotation/custom codecs are currently refused, despite related route/config names. Outbox/settings protection is a separate layer.

[Operational procedure](server/docs/user/operations.md) · [Errors and recovery decisions](server/docs/user/errors-and-troubleshooting.md)

### Application registry

Create/list/get/state/delete/bootstrap APIs and per-app supervisor/directory machinery are present. Complete independently isolated application execution is not established. Application update/backup/restore and one per-application import-source listing operation remain unregistered; the batch-level sources endpoint is separate.

The [42-family feature catalog](server/docs/features.md) retains the detailed inventory and each important boundary. [Current product](server/docs/current-product.md) connects families to source/tests; [launch status](server/docs/launch/status.md) records actual acceptance.

## One process, many databases

One HTTP server and bounded shared services manage many files. Databases do not each get a process, scheduler or warm JS VM. Turso runs in-process through Go bindings. The current executable embeds a native payload, verifies its compiled-in digest and extracts it to a private cache before loading.

Standalone growth uses a larger machine and a verified move of the complete application state. There is no vendor database-count quota; CPU, RAM, disk, connections and admission limits remain real. Cold database count differs from active concurrency. One hot database does not automatically distribute its writes across machines.

Fresh paths are `mb_data`, `mb_hooks`, `mb_migrations` and `mb_public`. Databases, sidecars, files, keys and recovery state live outside the executable. Copying one `data.db` is not a full application backup.

## Future open-source platform roadmap

**There is no MiraBase cloud or managed hosting offering.** The current project is the self-hosted open-source backend. Future open-source platform work covers placement, replication, routing, failover, balancing, cold databases, backups/PITR, an operator console, quotas and resource accounting. Preserve the frontend/API and private-database model as those capabilities develop.

[Open-source roadmap](server/ROADMAP.md) · [Future platform scope](server/docs/roadmap/platform/README.md) · [Distributed design](server/docs/roadmap/platform/distributed-architecture.md) · [Operator journey](server/docs/roadmap/platform/customer-journey.md) · [Resources/economics](server/docs/roadmap/platform/operations-and-economics.md) · [Future acceptance](server/docs/roadmap/platform/acceptance.md)

These are open-source roadmap designs, not current distributed runtime features. They add no vendor account, subscription or distributed-cluster requirement to today's standalone. Details live in `docs/roadmap/platform`; no vendor-operated service is part of the product direction.

## Build from this folder

The owned release source contains SDKs, native bridge, pinned Go driver, docs and tools. `third_party/turso-go` removes the Go build's dependency on sibling workspace repositories. Use the Go version required by `go.mod` (currently Go 1.27). Rebuilding the engine additionally needs the pinned Rust/native toolchain and Git.

```sh
go run ./tools/release --version dev
go run ./tools/release --check
```

The tool hashes the target payload, compiles both trusted digest pins, uses `no_default_driver`, and writes an executable plus SHA-256 manifest under `dist/`. Missing target payloads fail explicitly. Plain `go build ./examples/base` selects the inherited comparison provider and must not be shipped as the Turso product.

Run nested loader-module, SDK, UI, security/fault and platform checks too. [Launch instructions](server/docs/launch/README.md) give the commands; [status](server/docs/launch/status.md) records actual failures and unverified targets. A successful build is not release qualification.

## Documentation and attribution

[Detailed contracts](server/docs/mirabase/README.md) retain wire/storage/field definitions and decisions. Their initial v0.1 allocation is not today's implementation inventory. See [roadmap](server/ROADMAP.md), [contributing](server/CONTRIBUTING.md), [security](server/SECURITY.md), [license](server/LICENSE.md) and [third-party notices](server/THIRD_PARTY_NOTICES.md).

Preserve PocketBase and Turso copyright/license notices. Downloadable artifact checks and publication are recorded separately in [distribution metadata](server/release/distribution.json); production-assurance evidence remains in [launch status](server/docs/launch/status.md).
