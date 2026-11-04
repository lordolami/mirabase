# Database per tenant, user and agent

**Current release authority:** [current implementation inventory](../current-product.md), [user documentation](../user/index.md) and [launch status](../launch/status.md). Initial v0.1/roadmap delivery labels below preserve planning contracts; they do not establish today's implementation or permit deleting subsequently integrated features. [Open-source platform roadmap](../roadmap/platform/README.md) has separate design and gates.

**Delivery: v0.1 and roadmap.** [v0.1-build.md](v0.1-build.md) selects the current portion; [roadsmap.md](roadsmap.md) retains the remainder. Detailed schemas/configuration/ABI and full-family acceptance below describe the target superset unless narrowed by that allocation.

## Default and policy

A normal app retains one primary data database. Partitioning is opt-in per collection template. Each collection is classified as global, private or derived. Auth collections, settings and internal policy remain global within the selected application by default, never across independent applications. Private application records exist in the selected context. A template may expose global read-only reference collections through explicitly supported resolution; it cannot make an entire global auth table available to a private replica.

Creating a user record does not always create a database. Policy selects eager-on-registration or lazy-on-first-authorized-access; the default when partitioning is enabled is lazy. Creation is idempotent for the pair of auth collection ID and auth record ID plus template. Multiple auth collections with the same record ID cannot collide.

Existing PocketBase projects use the required guided [tenant import](pocketbase-tenant-import.md). It constructs an ownership graph from actual records, supports direct and indirect tenant relationships, accounts for every source row/file, adapts shared references/memberships/rules and previews every destination. Users resolve ambiguous ownership in that plan instead of manually splitting files. Other-BaaS adapters remain deferred. Unresolved assignment/compatibility errors prevent publication with exact record/object issue reports.

Tenants can be organizations/workspaces with multiple members. The primary route namespace and active epoch map typed tenant keys to optional kind=tenant contexts. Membership comes from the declared source-compatible policy, not an invented auth owner for the organization record. User/agent roots reuse their identity bindings. One tenant can have many users, and one user can select among several authorized tenants.

## Selecting a context

Inherited endpoints use the primary context unless policy binds the authenticated actor to a private default. An authorized explicit selector uses X-MiraBase-Database. The same selector is an SDK per-request option and cannot be a mutable global shared by concurrent requests. Resolve it before forms, rules and queries. Primary is a stable reserved identifier in the resolver; physical path is never exposed.

A new /api/mirabase/v1/databases/{id}/... administrative request uses its path ID; a contradictory header is rejected. Browser, SSE, file and sync requests need equivalent context selection, using signed/context-bound tokens where custom headers are unavailable. No fallback to primary after an invalid selector.

Imported applications can bind their existing workspace selector to X-MiraBase-Tenant through the reviewed plan. Validate current membership before resolving the active route epoch/database. One eligible tenant can be the default; several with no selector return tenant_selection_required. A client-provided tenant key is not authorization. The preview identifies required SDK/frontend selector changes.

## Owners, grants and agents

An owner is an auth record reference, not a string named userId. Database grants use record.read, record.write, files.read, files.write, changes.read, record.sync, database.inspect and explicitly privileged sql.read/sql.write/native.sync scopes. A scope permits an access path; ordinary record rules still filter each operation. Schema/lifecycle/global settings remain superuser operations unless a separately specified scoped operator policy authorizes them.

Agents are ordinary identities in an application auth collection with a protected binding to an owner. Register/revoke through authenticated owner policy or superuser administration. An agent receives only its own scopes and context bindings. It does not inherit the owner's token, impersonate a superuser, provision another user's database or modify its binding via normal record update.

Revocation increments policyEpoch; requests recheck the epoch at durable operation boundaries. Already downloaded offline data cannot be remotely erased with certainty. Publish that limitation without calling offline storage revocable secrecy.

## Relations and rules

Within-context relations keep existing collection+record semantics. Global references require a declared global relation field and bounded authorized secondary reads; they cannot masquerade as native same-file SQL joins. Cross-private relations are disabled by default and require explicit database-qualified reference types and grants. An ordinary relation never resolves another database merely because a record ID matches.

Expand, filter traversal, cascades, uniqueness, view queries and transactions respect these boundaries. A requested cross-context atomic cascade is rejected during schema validation; a separately configured durable workflow may execute per-context steps with visible partial results. Global uniqueness is not implied by a unique index in every private file.

@request.auth resolves the global authenticated actor through a trusted internal view. Arbitrary global auth scans are unavailable to raw private SQL. Rules referencing global private fields need a reviewed resolver and bounded reads; unsupported SQL-only translation is a schema-policy error, not permission to skip the rule.

## Schema and lifecycle fan-out

A published template is immutable. New private databases start at the current approved revision. Existing contexts migrate through a durable rollout with a fixed target snapshot and recorded per-database results. Mixed revisions remain visible; old SDK/schema compatibility is enforced. One failed database does not disappear from totals or let the rollout claim completion.

Deleting an auth record follows explicit policy: retain unowned for operator reassignment, quarantine, or schedule deletion. Default is quarantine and revoke access. Deleting an agent affects only its grants/bindings and declared agent database policy, not its owner's database.

## User outcome and evidence

One backend may hold Alice's database, Bob's database and separate agent databases without separate processes. The dashboard lists/searches them and opens records/files/schema/activity in a selected context. Ordinary users never receive that global listing.

Qualification uses two auth collections with colliding record IDs, multiple users/agents, cold-cache reopen, concurrent context requests, malicious headers, protected files, realtime, relation expansion, SQL and sync. Verify no cross-context records, paths, credentials, side effects or timing-sensitive routing leaks.



## Setup and application identity

A user is not a tenant synonym. Schools/workspaces may contain many members; one member may select several tenants. Empty applications use [guided setup](new-application-setup.md). Separate source applications use [application consolidation](application-consolidation.md), preserving each source boundary instead of repartitioning its users. Every policy, root key, grant, cache and subject is application-qualified under [application isolation](application-isolation.md). Ownership transfer, reference refresh and qualified relation/report workflows have exact contracts in [operational closure](implementation-closure.md).
