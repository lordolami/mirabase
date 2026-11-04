# Architecture

**Current release authority:** [current implementation inventory](../current-product.md), [user documentation](../user/index.md) and [launch status](../launch/status.md). Initial v0.1/roadmap delivery labels below preserve planning contracts; they do not establish today's implementation or permit deleting subsequently integrated features. [Open-source platform roadmap](../roadmap/platform/README.md) has separate design and gates.

**Delivery: v0.1 and roadmap.** [v0.1-build.md](v0.1-build.md) selects the current portion; [roadsmap.md](roadsmap.md) retains the remainder. Detailed schemas/configuration/ABI and full-family acceptance below describe the target superset unless narrowed by that allocation.

## Component model

One MiraBase process serves one or several isolated applications and contains the existing HTTP router, auth/rules/forms, collection and record services, file/mail services, dashboard, Go/JS extension runtime, scheduler, logging and backup services. It adds a Turso provider, database-context resolver, bounded handle manager, optional database registry, durable change/sync adapters and optional PostgreSQL gateway. Components share runtime services. They do not instantiate an entire PocketBase app per private database.

Default startup opens primary data.db and auxiliary.db. Optional partitioning introduces additional application-data files under a managed root. Each application primary is the source of that application's auth identities, settings, collection definitions, grants, templates and lifecycle coordination. These are never shared with an unrelated application. Additional application identity/routing uses the lazy instance registry in [application isolation](application-isolation.md). Private databases contain their configured application collections and minimal local schema/change/receipt metadata, not copies of global secrets or unrelated auth records.

## Request path

1. Apply trusted proxy/host handling, resolve the immutable ApplicationContext and its scoped body/origin/rate policy.
2. Authenticate using only the selected application's auth collections and token verification.
3. Resolve the requested database ID or default binding from server policy.
4. Check operation-specific grant and lifecycle state.
5. Lease a handle and immutable DatabaseContext.
6. Resolve collection schema at its published template revision.
7. Run inherited rule evaluation, forms, hooks and transaction handling through a context-aware app view.
8. Commit data, change frames and applicable idempotency receipts together.
9. Deliver after-commit effects; release leases even on panic/cancellation.

DatabaseContext embeds ApplicationContext and carries databaseId, generation, auth subject (collectionId plus recordId), grant/policy epoch, schema revision, engine profile, request deadline and transaction identity. Tenant contexts additionally bind tenant namespace, typed-key hash, active route epoch and current membership policy. Guest identity is explicit and permitted only by the reviewed public-access policy. It contains no client-controlled filesystem path. Context survives helpers, hooks, relation expansion, files, realtime and jobs. Escaping to primary is an explicit internal operation, not an omitted context parameter.

## Extension approach

Retain the core.App contract where possible. Add a narrow context-bound builder/service layer and transaction wrapper rather than global mutable current-database state. The default app view is the primary context, preserving existing extensions. New context-aware helpers must require a context for private operations. JS $app derives from the current invocation; global callback closures cannot capture a previous user's database.

Provider-owned handles implement DBX/database/sql semantics. No request performs arbitrary Rust FFI directly. Driver errors cross one normalization layer. The native service owns library initialization, capability probing, panic isolation, handles and operation cancellation.

## Startup

Parse inherited flags and MiraBase config; validate paths and combinations before effects. Acquire the single-writer application lock, resolve native payload, verify digest/ABI, open primary/auxiliary, load settings and secrets, apply supported core migrations, initialize caches/services, recover incomplete lifecycle jobs, then bind listeners. The readiness endpoint becomes true only after required recovery and schema checks succeed. A failed startup does not expose a partially ready public service.

Only enabled optional features create their metadata. The ordinary application must not bootstrap an unrelated organization/role/platform catalog. Registering a feature lazily does not excuse missing upgrade migrations or tests for it.

## Shutdown and concurrency

Stop accepting new mutating work, reject new leases, drain bounded in-flight requests, cancel remaining native work at the grace deadline, flush known durable receipts and logs, checkpoint only through qualified engine APIs, close statements/rows/connections, then unload nothing still referenced. A cancelled commit can have an unknown outcome; recovery/idempotency determines the final effect, never an assumed rollback.

Handle eviction affects idle files only, never live transactions, sync transfers, backups or subscription jobs. There is one process-wide JS pool and scheduler. Background work consumes separate bounded admission slots so bulk schema fan-out cannot starve record requests.

## Trust boundaries and external effects

Global administration, whole-database SQL, ordinary records, native replication and offline record sync are distinct capabilities. All converge on the same database lifecycle resolver, but raw administrative SQL does not pretend to call record hooks. Untrusted plugins are not sandboxed by Goja alone; operator-installed code has documented server capabilities.

Mail, webhooks, local/S3 storage and callbacks cannot join a database ACID transaction. Stage files and persist durable outbox/job intent where necessary. A failure after a database commit is surfaced as a committed operation with pending side effects, not a fake full rollback.

## Deliberate limits

Single-context transactions are atomic. Cross-database lifecycle/fan-out is a resumable job with per-database results. No hidden cross-file or cross-node transaction guarantee exists. Native whole-database clients receive a clearly privileged contract. Deployment remains a portable backend; additional managed operations do not change these boundaries.



## Complete application boundary

[Application isolation](application-isolation.md) owns application routing before auth, local superusers/settings/schemas/artifacts, shared runtime services and independent lifecycle. [New application setup](new-application-setup.md) owns empty-app configuration. [Application consolidation](application-consolidation.md) owns multiple-source preservation and append publication. These are required parts of this architecture, not separate future products. The process remains one writer/owner of its root.
