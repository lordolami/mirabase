# Product contract

**Current release authority:** [current implementation inventory](../current-product.md), [user documentation](../user/index.md) and [launch status](../launch/status.md). Initial v0.1/roadmap delivery labels below preserve planning contracts; they do not establish today's implementation or permit deleting subsequently integrated features. [Open-source platform roadmap](../roadmap/platform/README.md) has separate design and gates.

**Delivery: v0.1 and roadmap.** [v0.1-build.md](v0.1-build.md) selects the current portion; [roadsmap.md](roadsmap.md) retains the remainder. Detailed schemas/configuration/ABI and full-family acceptance below describe the target superset unless narrowed by that allocation.

## Identity and objective

The product is **MiraBase**: PocketBase extended with Turso, preserving its complete backend, framework and dashboard experience in a standalone process.

The user-facing objective is a complete lightweight backend: start an application, configure collections and permissions in the existing dashboard or API, and connect a frontend. v0.1 supports one application with optional private databases; independent-application consolidation is a roadmap capability. Its database implementation becomes more capable without making developers learn a different backend model. The Go package remains embeddable in a Go application; the ready-made executable remains a complete backend. Client SDKs communicate with the backend and do not contain that entire backend.

## Binding invariants

1. Existing single-database collection, record, authentication, authorization, file, realtime, and SDK behavior remains the default compatibility contract.
2. The existing dashboard is extended. A second admin product is not a replacement for it.
3. New capabilities integrate into existing fields, schemas, filters, settings, hooks, APIs and operational workflows whenever that is their semantic home.
4. The SQLite frontend of Turso is the provider for the ordinary PocketBase-derived application model. PostgreSQL is an additional explicitly configured access path, not a mass rewrite of existing generated SQL.
5. Application consolidation and optional database-per-tenant/user/agent use one process and bounded shared runtime services. Each independent application retains its own schemas, identities, settings, files and trusted extension bindings. A database does not receive its own executable, container, scheduler, warm JavaScript pool or HTTP server.
6. Authentication identifies an actor; database selection identifies a resource. A client-supplied owner, filename, database ID or agent ID never proves permission.
7. User record access and privileged administrative SQL remain distinct, as in the inherited backend. New native paths must not accidentally grant administrative privileges to ordinary clients.
8. Native writes, migrations, sync and PostgreSQL access have explicit metadata, hook, validation and event semantics. No documentation may imply that an arbitrary SQL write already traverses the record lifecycle.
9. Capability availability includes engine revision, build, platform, frontend, execution mode, schema requirements and language-adapter coverage. A compiled symbol alone is insufficient.
10. All committed effects and recovery boundaries are specified. A successful HTTP response must not hide uncommitted work, uncertain application state or missing durable receipts.
11. Existing public names without PocketBase branding, including `/api/collections`, `/_/`, collection methods, `$app`, ordinary hook names and JSON record envelopes, are retained unless a separately approved compatibility change requires otherwise.
12. Product branding becomes MiraBase. Legal provenance, upstream names in citations, imported-format identifiers and explicitly supported legacy aliases are intentional exceptions.
13. No feature is called implemented, tested or production-qualified merely because it is specified, wrapped, advertised upstream, switched on in configuration or represented in a generated registry.
14. v0.1 requires every acceptance selected by v0.1-build.md, including all safety dependencies. The full product additionally requires all roadmap families. A scoped release cannot claim full-family completion, and no release feature is fulfilled by a placeholder or unsupported stub.

## Included foundation

MiraBase inherits and preserves the pinned baseline's executable and Go framework; Go and JavaScript hooks; base/auth/view collections; all field kinds; generated CRUD and batch APIs; filters, sorting, relation expansion and field projection; password/OAuth2/OTP/MFA authentication; account verification and recovery; rule-based record access; local/S3 files and protected download tokens; thumbnail generation; SSE record notifications; superuser administration; SQL console; settings; email; logging; health; cron; migrations; backup/restore; static frontend hosting; HTTP/HTTPS deployment; updater mechanisms; tests; and existing UI workflows. Exact baselines and known inherited restrictions are owned by `pocketbase-foundation.md` and the compatibility registry.

## Complete-product additions and release allocation

Full PocketBase adoption must offer conversion into databases per tenant, user or agent. The importer maps ownership through direct fields and relation paths, preserves records/files/IDs/account access, adapts memberships/rules/shared references, previews the complete split, validates real staged data and performs a recoverable cutover. Source schema JSON alone is insufficient for this full-data path. A shared-layout adoption choice remains available. The normative flow is [PocketBase import into tenant databases](pocketbase-tenant-import.md).

The required additions are the Turso Go provider; SQLite compatibility adaptation; native lifetime/error handling; optional MVCC concurrent execution; async I/O integration; supported richer SQL; typed vectors and exact vector queries; full-text and fuzzy search; additional SQL functions; typed arrays; custom types/domains/composites; configurable defaults and sequences; supported materialized-view collections; page encryption and custom-codec integration; durable change capture; record-aware offline sync; qualified upstream native sync for privileged whole-database replicas; partial bootstrap; optional PostgreSQL access with MiraBase authentication; database-per-user/agent contexts and lifecycle; additive HTTP/SDK/dashboard coverage; native-aware backups and recovery; safe packaging; resource controls; and full qualification evidence.

Some upstream strengths are enabling infrastructure rather than new frontend controls: the shared VM, language frontends, extension API, C API compatibility, testing ecosystem and license. Their product mapping must be explicit; they do not justify inventing unrelated backend services. Upstream ANN vector indexing remains a roadmap item; the toy sparse index must not be represented as a mature approximate-nearest-neighbor product. Query-subscription claims require a working public adapter and qualification rather than an inference from internal incremental-view subscriptions.

## Database scope and simplicity

The default remains one project/application context, one primary application database, and one auxiliary database, following the inherited model. No organization, billing account, platform owner UUID, network account or cloud subscription is required. The initial setup and superuser model remain recognizable. Optional partitioning is selected deliberately and does not split an existing application silently.

Per-user/agent storage is an application feature, not an assertion that every existing cross-user query can be moved into different files unchanged. Its policy controls shared collections, ownership, relation resolution, authorization and migration. An incompatible partition policy is rejected with an exact explanation; data must never be silently duplicated, omitted or reassigned to make activation appear successful.

There is no vendor database-count cap in the self-hosted runtime. Operator limits and physical resource limits are real and visible. The specification does not claim that one machine can host an unlimited number of active databases. Handles, native memory, file descriptors, background work, templates, migrations, disk space and inodes are bounded and measured.

## Delivery and growth

The primary standalone experience is a MiraBase executable for a qualified OS/architecture. The Go framework exposes the same backend for embedding. Runtime databases, WALs, files, settings, certificate caches and secure native extraction are not part of a claim that all persistent state fits in one physical file.

The ordinary standalone scaling path is vertical: more suitable CPU, RAM and storage, followed by a verified transfer of the entire application state. Keeping the same endpoint and authentication configuration avoids a frontend rewrite. A bigger machine does not automatically fix hot rows, slow queries or resource leaks.

A paid managed offering operates the same runtime and application contracts. Operational management can add provisioning, billing and fleet concerns outside the runtime; those are not prerequisites for local use, and a complete distributed hosting architecture is not invented inside this specification. No consensus, sharding or multi-node transaction engine is promised.

## Explicitly deferred or excluded

- Migration from Firebase, Supabase, Appwrite and other BaaS products is deferred by the user's latest instruction. Preserve a migration-friendly export format and stable identifiers; do not implement provider adapters or claim seamless migration now.
- Pinned PocketBase shared-layout adoption and normal MiraBase schema upgrades are v0.1 requirements. Conversion into tenant/user/agent databases remains required roadmap work, separate from deferred other-BaaS adapters.
- An MCP server or hosted agent gateway is excluded. Authenticated HTTP and normal client tooling are the agent interface.
- A separate NoSQL/MongoDB frontend, distributed SQL engine, required cloud service and mandatory organization/control-plane model are excluded.
- Arbitrary foreign backend functions, all PostgreSQL extensions, every SQLite loadable-extension binary and upstream future features are not implied by the phrase "all Turso strengths."

## Completion and contradiction handling

Every requirement has one owner, an exact acceptance result, and linked implementation evidence. If two required behaviors conflict with the pinned engine, the issue is recorded as a blocker with a reproduction and resolution options. A rejected operation may be correct behavior for an unpromised combination; it is not completion of a required capability. A release matrix cannot turn a promised feature into an optional omission after the fact.

The documentation may be complete as a design while runtime evidence remains empty. The final documentation report must state that distinction. No source edits, runtime qualification or package publication are implied by authoring this specification.



## Release delivery

[v0.1](v0.1-build.md) is the current build contract. [Roadmap](roadsmap.md) retains the full-product additions: independent apps/consolidation, populated-layout splitting, complete invitations and setup, advanced types/search/SQL, sync/CDC/transports, encryption/extensions, derived workflows and remaining client/operational coverage. The [AgentFS design](agentfs-roadmap.md) is entirely roadmap work. No scheduled group implies a release date or an unqualified implementation claim.

The requirement allocation narrows current delivery without deleting the full contracts. Detailed chapters use full-product language unless their release labels select a v0.1 portion. Release constraints take precedence over a broader enum, config or ABI superset. No distributed engine, cooperating server processes or managed hosting control plane is added.
