# PostgreSQL access

**Current release authority:** [current implementation inventory](../current-product.md), [user documentation](../user/index.md) and [launch status](../launch/status.md). Initial v0.1/roadmap delivery labels below preserve planning contracts; they do not establish today's implementation or permit deleting subsequently integrated features. [Managed cloud](../cloud/README.md) has separate design and gates.

**Delivery: roadmap.** This chapter specifies later behavior; v0.1 does not build this service. Shared safety rules still apply where used by released features.

## What is included

Turso has an implemented PostgreSQL grammar/parser, translator, frontend/session layer, CLI and pgwire server. MiraBase adds optional authenticated database access through that frontend while retaining SQLite-generated SQL for the normal backend. PostgreSQL-compatible access is not a claim that the engine is PostgreSQL or supports its full extension ecosystem.

Source baseline: postgres/COMPAT.md, parser/translator.rs, frontend/lib.rs, frontend/session.rs and server/lib.rs at the pinned revision. The current compatibility label is experimental. Report supported/partial/unsupported behavior per client operation instead of dismissing the implemented frontend or promising full compatibility.

## Listener and authentication

Disabled by default. Default enabled bind is 127.0.0.1:5433. External access requires TLS and MiraBase authentication. The upstream trust-only/no-TLS server cannot be exposed. Implement a verified pgwire authentication gateway/adapter with SCRAM-SHA-256 credential verification and TLS negotiation, then pass an immutable authorized DatabaseContext to the native frontend. Do not splice authentication into SQL text or rely on a forgeable startup parameter.

Database startup name is the opaque MiraBase database ID or the reserved primary alias for a superuser credential. Username identifies an issued credential; password is its secret. Credential scope, expiry and policy epoch are checked at connection and transaction boundaries. Reject trust fallback. Rotation/revocation terminates sessions within the documented bounded check.

## SQL privilege and metadata

This is whole-database administrative SQL, analogous to the superuser SQL console. An ordinary record token does not become SQL permission. sql.read denies writes/unsafe PRAGMAs/extension/file access through parsed native enforcement; sql.write still cannot alter protected application metadata/schema outside the controlled administrative workflow.

Managed collection DDL must reconcile _collections/type/template/index metadata and invalidate caches atomically where supported. Until a statement has that reconciliation adapter, reject it for managed objects and direct the operator to the schema planner. Unmanaged SQL tables are visible as unmanaged SQL objects, not invented collections. Native DML generates CDC but does not run record validation/hooks.

Native schemas/search_path map to the current namespace implementation inside one database. Do not implement the obsolete attached-file schema design. Prevent cross-session leakage of search_path, settings, temporary state and custom session parameters; the audited server's state handling needs explicit isolation qualification.

## Wire compatibility

Qualify startup, cancellation, errors/notices, Simple Query and Extended Parse/Bind/Describe/Execute/Sync, prepared statements, parameter types, text/binary values, transaction status and connection close. The current upstream text-parameter, binary-result, Execute-row-limit and portal-suspension restrictions remain explicit until fixed. A GUI connecting is not proof all protocols work.

COPY TO STDOUT text and supported file/text COPY features are documented individually. COPY FROM STDIN, CSV/binary and unsupported protocol combinations must fail safely rather than hang. Disallow server filesystem COPY except a trusted configured administrative path.

## SQL compatibility

Track supported schemas/search_path/catalog inspection, arrays/custom types/domains/enums/serial/sequences, query/transaction syntax, functions and views against the exact matrix. No built-in promise covers PG role/GRANT/RLS, PL/pgSQL, arbitrary extensions, full information_schema, every isolation level or identical type semantics.

Schema/materialized-view/mode restrictions apply here too. Cross-frontend differences such as recursive CTE support need actual tests; a PostgreSQL parse success does not prove execution correctness. Keep type precision and timezone behavior explicit.

## Required clients and evidence

Release qualification includes psql, a Go PG client, a JavaScript PG client and one GUI/ORM introspection workflow at pinned versions. Add Python/JDBC/.NET matrices where advertised. Compare results, errors and metadata with the documented contract, not an unrestricted PostgreSQL server.

Tests cover failed auth/TLS downgrade, credential revocation, cross-DB selectors, session state reuse, cancellation, prepared statements, managed DDL bypass, filesystem operations, connection exhaustion and process restart. The native authentication bridge and interoperability suite are required implementation outputs.



## Application identity and sessions

PG credentials bind application and database identity; a primary alias refers only to that credential's application. Schema/session/cache and managed DDL reconciliation cannot cross apps. The Go gateway and real client qualification are required build outputs. See [application identity and sessions](application-isolation.md).
