# What we built during the hackathon

**MiraBase — One backend. A private database for every user, tenant, and agent.**

**Empowering the next generation of indie hackers and AI agents with zero-infra multi-tenancy.**

Our submission comprises the MiraBase extensions and integration, plus a separate [Formal Verification assurance harness](https://github.com/lordolami/formal-verification). We used **1.3-contributor for implementation** and **Muse Wonder for architecture documentation**. The account below describes our hackathon contribution and the current boundary of each implementation.

[MiraBase source](https://github.com/lordolami/mirabase) · [Formal Verification source](https://github.com/lordolami/formal-verification) · [Three-minute demo](https://youtu.be/PVRYAADs-EQ) · [Source-linked inventory](current-product.md) · [Executed evidence](launch/status.md)

## The problem and the product

A solo developer building a SaaS, customer portal, team workspace or multi-agent application needs accounts, permissions, persistent records, files, search, schema changes and recovery. Giving each customer or agent private storage should not require another backend deployment for every database.

MiraBase extends the existing PocketBase backend/dashboard with Turso and a multi-database application layer. Shared application data, individual user data, team/tenant data and agent data can have different storage arrangements under one backend process. Zero-infra multi-tenancy means no extra process, container, scheduler or warm JavaScript VM per database. The operator still supplies a host, persistent storage and access policies; hardware limits remain real.

## Every MiraBase addition and integration

### Engine and delivery

1. **In-process Turso provider.** Native engine integration with Go's database/sql and DBX interfaces; data and auxiliary stores use the selected provider. Connection, statement and result ownership, preparation, execution, asynchronous progress, cancellation, cleanup and provider-error translation are part of the bridge. WAL/MVCC writer infrastructure and no_default_driver builds avoid accidentally shipping the inherited comparison provider.

2. **Native source provenance.** Pinned upstream revision, owned native fixes, patch manifests, patch/applied-diff digests and rejection of unexpected native source changes. Audit checkouts and upstream notices are preserved.

3. **Payload trust.** Expected digests compiled into the executable, verified extraction/cache handling, pin-aware loaded-library caching, startup probes, and refusal of missing/malformed pins or modified payloads. Replacing an adjacent checksum does not authorize a modified library.

4. **Standalone delivery tools.** Target-aware release builder, executable/native manifests, checksums, standalone archive builder, archived-file digest verification, launcher, welcome page, packaged documentation/frontend example, SDK tarball and clean-package checks. The bundled-public-directory regression covers archives extracted under an OS temporary directory.

### Private databases and authority

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

### Schemas and retrieval

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

### Changes, sync, agents and SQL

28. **Durable record changes.** Ordered committed changes, incremental pull/stream, commit boundaries, durable positions, retained replay and reset/rebootstrap behavior. Each additional writer path still requires capture evidence.

29. **Record synchronization.** Bootstrap snapshots/pages, client instances, grouped creates/updates/deletes, push/pull/ack, file staging/upload, stable operation identities, receipts and generation/schema/policy recovery boundaries. Bootstrap/page is currently operator-bound; ordinary-client offline stores and native whole-DB protocol interoperability remain unfinished.

30. **Live-query plans.** Named revisioned SQL, typed parameters, dependencies, unique result keys, limits, list/create/get/publish/retire controls.

31. **Live subscriptions.** Authorized registration, initial result snapshots, keyed insert/update/delete/order changes, bounded replay, reset and cancellation. Ordinary-user shapes that cannot be safely narrowed may be refused.

32. **Agent workspaces.** Owner-bound workspace create/list; working-file write/read/delete; metadata, versions, content hashes and shared bounded coordination.

33. **Historical snapshots.** Snapshot manifests and historical bytes, overwritten/deleted file restoration, path/hash preflight and preservation of unrelated files. Server helpers record explicitly instrumented tool history. Current HTTP workspaces are operator-only; full upstream AgentFS and crash-atomic multi-file restore remain additional work.

34. **PostgreSQL client gateway.** Optional TLS-only listener, SCRAM, scoped SQL credentials, read/write boundaries, simple/extended queries, prepared statements, parameter binding, statements/portals, describe/execute/close/flush/sync, session settings, transaction/error state, cancellation, timeouts and protocol errors. External node-postgres testing includes verified TLS and prepared queries; this is not complete PostgreSQL server/ORM compatibility.

### Adoption, operations and developer access

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

These are work areas, not a manufactured count of independently accepted features. [Current product](current-product.md) links source and tests; [the catalog](features.md) preserves inherited/additive/partial/planned distinctions.

## Formal Verification: the second project

**[Explore the full Formal Verification repository](https://github.com/lordolami/formal-verification).**

We built an AI-assisted assurance harness around Quint for designs, new software, existing implementations and maintenance. It connects claims to executable models, actual source symbols, live behavior and evidence. Its scope includes APIs/business logic, identity/session/token flows, schemas/transactions/migrations/recovery, coordination/sync, jobs/retries, files/backups and protocols. It does not provide universal automatic proof of arbitrary code or frontend-UI testing.

### All 17 tool entry points

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

### Model and claim library

The **15 starter models** cover key/value staged-versus-committed state/crash recovery; leader election; idempotent jobs; token issue/use/revoke/expire/rotate; CRUD validation/auth/idempotency; leases/fencing; sync cursors/conflicts/generations; backup/restore publication/crash boundaries; file protection/publication/deletion; tenant isolation against forged hints; privileged boundaries; query/subscription/index visibility; queues/leases/poison handling/payload budgets; migration/constraint integrity; and secrets/artifact/lockout/step-up/cache boundaries.

The **42-claim catalog** has 38 core and four extension claims, plus domain packs for multi-tenant SaaS, payments ledgers, synchronization engines and healthcare records. Examples cover a todo API, secure/insecure probe targets, trace replay and fault execution records; CI templates and self-qualification tooling are included.

Evidence records methods, bounds and provenance. PASS with exhaustive checking is limited to recorded bounds; PASS-WITHOUT-EXHAUSTIVE remains distinct. Zero-test/vacuous runs, missing checks and surviving mutants are not hidden. These are harness capabilities, not a claim that every MiraBase behavior has been formally proven.

## Tests, real demonstrations and fixes

Recorded Windows native execution has **5,236 test/subtest passes**, with four archive assertions passing separately under authorized symlink privileges. The earlier Linux baseline has **5,238 passes**, tied to its earlier source/payload. The current Windows ZIP has **23 named package checks** and **four SDK transport tests**. Reports bind exact hashes and execution scope; historical runs are not presented as a new full run of every later artifact.

Real checks and fixes cover template-before-ready provisioning, concurrent-write backup exports and independent archive reopening, mixed-case import inventory and missing-row negatives, native tamper rejection, SDK transport handling, verified-TLS/prepared PostgreSQL queries, workspace historical-byte recovery, ordinary-account denial/validation and restart persistence.

The three-minute film uses real dashboard footage and a separately labelled example client, returned search results, private-context grant/revoke behavior, live updates, historical workspace restoration and adoption workflows. Captions, narration, chapters and evidence/coverage records accompany it. No million-database benchmark is claimed.

## Existing foundations and timeline

PocketBase v0.41.1 (`f70b24e186df98df2e7ebc703f103cb94dc3ca63`) supplies the pre-existing backend/dashboard/auth/collections/files/realtime/extensions and upstream tests. Turso (`b0e1b8fde8eff66ef9843fdd1139fa3305cb16dd`) supplies the engine and vendored driver. Quint supplies the specification/checking engine used by the separate harness. Their authors, licenses and notices remain credited.

The feature additions and integration described here are our hackathon contribution. The event's contribution window begins October 26, 2026 at 12:24 PM ET. MiraBase's public repository was created November 4; its first commit imports a development snapshot, with subsequent real commits retained. That import is not a claim of contemporaneous incremental history for all development. The companion repository is separately linked above.

## Release and roadmap

Publication and a passing smoke test do not substitute for production qualification. The existing Windows artifact is unsigned. [Launch evidence](launch/status.md) preserves current implementation/platform/recovery/signing obligations, including the managed-record policy gap described above.

Full offline/native sync, full AgentFS, independent application isolation, remaining schema/SDK/platform/resource/signing work and future placement/replication/routing/failover/balancing/PITR/fleet operations are [open-source roadmap work](../ROADMAP.md). There is no MiraBase-hosted cloud or managed hosting offering.
