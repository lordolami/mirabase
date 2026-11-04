# MiraBase specification

**Current release authority:** [current implementation inventory](../current-product.md), [user documentation](../user/index.md) and [launch status](../launch/status.md). Initial v0.1/roadmap delivery labels below preserve planning contracts; they do not establish today's implementation or permit deleting subsequently integrated features. [Future open-source platform](../cloud/README.md) has separate design and gates.

**Start with current behavior:** the [feature catalog](../features.md), [task guides](../user/index.md), [source inventory](../current-product.md) and [launch status](../launch/status.md). These specifications retain complete target contracts, including unfinished work. The [v0.1 build contract](v0.1-build.md) and [original roadmap](roadsmap.md) preserve planning ownership; they are not today's implementation allocation. The source baseline remains PocketBase v0.41.1 and the pinned Turso snapshot in [upstream evidence](upstream-evidence.md).

MiraBase extends PocketBase with Turso. It remains a complete, portable application backend with generated APIs, authentication, access rules, files, realtime, an existing admin dashboard, and Go/JavaScript extensibility. Developers configure their backend and build their frontend. The existing extensibility remains available for application-specific behavior.

## Original planning allocation

v0.1 includes the complete inherited PocketBase backend on Turso, one application with optional user/tenant/agent databases, WAL and qualified MVCC, vector and full-text search, basic schema fan-out, safe lifecycle/admission, coherent backups/server moves, full shared-layout PocketBase import, the extended existing dashboard and remote SDKs.

The roadmap includes independent-application consolidation, guided splitting of existing PocketBase data, populated-layout migration, further native types/SQL, fuzzy/hybrid search, materialized views/live queries, CDC/offline/native sync, PostgreSQL/SQL-HTTP, page encryption/codecs/extensions, advanced relations/reporting/workflows/rollout controls, remaining setup/client coverage and the full AgentFS integration. [Roadmap details](roadsmap.md) retain the detailed contracts and dependencies.

Read [current product](../current-product.md) before selecting work. The chapters below retain complete target requirements; historical delivery labels do not authorize deleting integrated features or calling them unimplemented. They do not establish that an obligation has passed acceptance.

## Read order

| Document | Owns |
|---|---|
| [Product contract](product-contract.md) | Exact scope, invariants, delivery model, exclusions, and full-scope completion |
| [Branding and compatibility](branding-and-compatibility.md) | MiraBase identity, every rename category, preserved interfaces, PocketBase import |
| [PocketBase import into tenant databases](pocketbase-tenant-import.md) | Full source adoption, ownership graph, accounts/rules, records/files, preview, validation, routing and recoverable cutover |
| [PocketBase foundation](pocketbase-foundation.md) | Existing capabilities, actual source ownership, and compatibility obligations |
| [Turso capabilities](turso-capabilities.md) | Complete capability inventory, status, product mappings, and unsupported combinations |
| [Application isolation](application-isolation.md) | Independent apps, auth/settings/schema/artifacts, routing and lifecycle in one process |
| [New application setup](new-application-setup.md) | Empty-app journey, storage choice, provisioning, membership and frontend examples |
| [Application consolidation](application-consolidation.md) | Multiple PocketBase sources, append/provenance and independent cutover |
| [Operational closure](implementation-closure.md) | Ownership, references, relations/reports, layout, live plans, rollout, retention and resources |
| [Offline client delivery](offline-client-delivery.md) | Concrete runtime adapters, assets, store identity, keys and recovery |
| [Build entry point](build-start.md) | Source ownership, phase gates and full-scope execution instructions |
| [Architecture](architecture.md) | In-process services, boundaries, contexts, dependencies, startup and shutdown |
| [Native provider](native-provider.md) | Go driver/DBX adaptation, native ABI/lifetime, build features and engine gating |
| [Selected integration implementations](integration-contracts.md) | Native sync, live queries, PG, SQL/HTTP, coherent backup, module identities, source pin and full-feature budgets |
| [Storage and lifecycle](storage-and-lifecycle.md) | File layout, registry, manifests, database state transitions, deletion and recovery |
| [Partitioning](partitioning.md) | Database-per-tenant/user/agent, identity/membership binding, shared collections, relations and routing |
| [Collections and types](collections-and-types.md) | Every inherited field and added type, serialization, defaults and schema rules |
| [Queries and search](queries-and-search.md) | Filter grammar, vector/text/fuzzy queries, authorization and pagination |
| [Transactions and concurrency](transactions-and-concurrency.md) | WAL/MVCC modes, pooling, retries, hooks and side effects |
| [Schemas and views](schemas-and-views.md) | Migrations, indexes, custom types, materialized views and schema fan-out |
| [Authentication and security](authentication-and-security.md) | Existing auth, database grants, agents, tokens, TLS, secret handling and SQL privilege |
| [Files and email](files-and-email.md) | Uploads, S3/local storage, protected access, thumbnails, mail and file lifecycles |
| [Realtime and CDC](realtime-and-cdc.md) | Existing SSE, durable changes, native mutations, cursors and retention |
| [Record sync](record-sync.md) | Offline record-aware synchronization preserving application semantics |
| [Native sync](native-sync.md) | Upstream protocol compatibility, trusted database replicas, bootstrap, replay and partial sync |
| [PostgreSQL access](postgresql-access.md) | Native frontend, administrative boundary, client protocol and metadata reconciliation |
| [API and SDK contracts](api-and-sdk.md) | Existing HTTP surface, additive versioned operations, errors, SDKs and generated API previews |
| [Dashboard](dashboard.md) | Extensions to the existing UI, all workflows, capability states and accessibility |
| [Extensions and jobs](extensions-and-jobs.md) | Existing Go/JS hooks, custom routes, cron, optional actions, durable jobs and callbacks |
| [Backups and recovery](backups-and-recovery.md) | Existing backup contract, encrypted and partitioned data, restore and crash recovery |
| [Configuration and deployment](configuration-and-deployment.md) | Complete configuration, local/public hosting, resources, paths and server moves |
| [Packaging and performance](packaging-and-performance.md) | One-download packaging, native payloads, SDK distribution, footprint and load qualification |
| [Operations](operations.md) | Logs, metrics, diagnostics, maintenance, upgrades, repair and operator runbooks |
| [Managed delivery](managed-delivery.md) | Same-runtime paid operation and the explicit boundary around a future hosting service |
| [Testing and acceptance](testing-and-acceptance.md) | Compatibility suite, feature fixtures, negative cases, fault tests and release gates |
| [Implementation workstreams](implementation-workstreams.md) | Active v0.1 gates and full-product completion owners |
| [Selected decisions](decisions.md) | Closed design choices, exact implementations and truthful engine constraints |
| [Upstream evidence](upstream-evidence.md) | Pinned sources, source hashes, observations and reproduction boundaries |

## Detailed design references

| Document | Owns |
|---|---|
| [Requirements and acceptance](requirements-and-acceptance.md) | Every required family, phase owner and real acceptance outcome |
| [HTTP operations](http-operation-reference.md) | Exact methods, paths, authority, input/output, guards, errors and inherited routes |
| [Wire schemas](wire-schema-reference.md) | Every named request/response definition and its constraints |
| [Configuration](configuration-reference.md) | Full merged defaults, exact configuration schema and bounds |
| [Storage and formats](storage-format-reference.md) | Instance/application/client DDL and archive/manifest formats |
| [Native ABI](native-abi-reference.md) | Exact proposed native interfaces and buffer/handle ownership |
| [Engine and fields](engine-reference.md) | Capability matrix, field options, mode restrictions and selected adapters |
| [Pinned sources](source-reference.md) | Upstream inputs and source identities |

These are readable specifications. Implement their interfaces and actual behavior; declarations, configuration, documentation and passing structural checks do not fulfill runtime requirements. Cross-references must resolve, and breaking interfaces need explicit protocol/compatibility treatment.

## Meaning of full scope

All inherited behavior and all selected additions must be present and qualified before v0.1 is called complete. The full product additionally requires every roadmap commitment. Optional runtime configuration is not an excuse to leave a selected release feature unimplemented. Engine restrictions remain visible in the capability matrix; an unsupported response does not satisfy a promised release combination.

The scope does not promise every imaginable application-specific business rule or every upstream roadmap feature. It does promise the exact inherited and additional behaviors listed here. Engine-only tools that the user explicitly excluded, such as a mandatory MCP interface, are not secretly reintroduced.
