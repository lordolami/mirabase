# MiraBase user documentation

MiraBase provides the backend and dashboard; your frontend connects through authenticated APIs. Start shared, then choose optional user/tenant/agent databases when your application needs that storage layout. One process serves those database files through shared services.

Read the [complete feature catalog](../features.md) for the full product inventory, the [quick start](quickstart.md) to run an application, and [launch status](../launch/status.md) before choosing a production candidate. The release remains unqualified; managed-record policy parity and other integrations are still open. User guides explain current behavior, while [detailed contracts](../mirabase/README.md) also contain obligations not yet delivered.

## Choose your starting point

| You want to… | Start here | What it covers |
|---|---|---|
| Understand everything included | [Feature catalog](../features.md) | Inherited backend, additions, partial integrations and planned platform. |
| Build a new application | [Quick start](quickstart.md), then [foundation](foundation.md) | Start server, create identity/data collections, set rules and connect a frontend. |
| Choose user vs tenant storage | [Databases](databases.md) | Examples, memberships, provisioning, context and limits. |
| Connect an existing frontend | [Client APIs](client-apis.md), [SDKs](sdks.md) | Inherited API vs managed helpers, transport and current method coverage. |
| Bring a PocketBase backup | [Adoption](pocketbase-import.md) | Analyze/plan/preview/stage/validate/cutover and recovery. |
| Add search | [Search](search.md) | Index prerequisites, four modes, request examples and continuation. |
| React to data changes | [Realtime](realtime.md), [changes/live](changes-and-live.md) | Events vs retained history vs maintained query results. |
| Synchronize records | [Sync](synchronization.md) | Bootstrap/pages/push/pull/ack/files and unfinished offline integration. |
| Evolve many private databases | [Schema/workflows](schema-and-workflows.md) | Templates, plans, rollouts, shared reference copies and reports. |
| Store an agent's working files | [Workspaces](agent-workspaces.md) | Operator-only file/snapshot API and AgentFS boundary. |
| Connect a PostgreSQL client | [PostgreSQL](postgresql.md) | Enable TLS listener, issue SQL credentials, qualify the actual driver. |
| Back up or move a server | [Operations](operations.md) | Coverage, validation, generation, fencing and recovery. |
| Add custom backend behavior | [Extensions](extensions.md), [jobs/migrations](jobs-and-migrations.md) | Trusted Go/Goja hooks, routes, cron and migrations. |
| Let an agent operate the backend | [Agent guide](../agents/README.md) | Identity, context, authority, retry and evidence discipline. |
| Check completion claims | [Source inventory](../current-product.md), [launch status](../launch/status.md) | Concrete symbols/tests, actual executions and remaining blockers. |

## Understand the three authority surfaces

| Surface | Purpose | Current boundary |
|---|---|---|
| Inherited record API | Collections, forms, rules, files and realtime | Read the collection rules and actor context. |
| Additive managed-record API | Explicit-database CRUD | Owner/grant checks exist; full inherited policy/form/hook/hidden-field parity is unfinished. |
| Operator/SQL APIs | Provisioning, schemas, imports, workspaces and raw SQL | Explicit operator or scoped SQL authority; do not place these credentials in a frontend. |

Use [actual routes](api-reference.md) for method/path/handler details. A capability name or a design requirement does not establish that every actor can call it.

## All guides

- [Agent workspaces and AgentFS](agent-workspaces.md)
- [Bound MiraBase HTTP reference](api-reference.md)
- [Authentication and MFA](authentication.md)
- [Realtime, changes and live queries](changes-and-live.md)
- [Client APIs](client-apis.md)
- [User, tenant and agent databases](databases.md)
- [Go and JavaScript extensions](extensions.md)
- [Fields](fields.md)
- [Files](files.md)
- [Backend foundation](foundation.md)
- [Jobs and migrations](jobs-and-migrations.md)
- [Operations, recovery and server moves](operations.md)
- [PocketBase adoption](pocketbase-import.md)
- [PostgreSQL client access](postgresql.md)
- [Production and superuser access](production.md)
- [Start an application](quickstart.md)
- [Realtime](realtime.md)
- [Rules and filters](rules-and-filters.md)
- [Schemas, invitations and derived workflows](schema-and-workflows.md)
- [SDKs and programming languages](sdks.md)
- [Vector, text, hybrid and fuzzy search](search.md)
- [Record and native synchronization](synchronization.md)

- [Errors and troubleshooting](errors-and-troubleshooting.md)

## Read the right kind of document

The [feature catalog](../features.md) explains breadth; these guides explain tasks; the [API reference](api-reference.md) links bound handlers; [current product](../current-product.md) links implementation/test ownership; [launch status](../launch/status.md) records acceptance; [specifications](../mirabase/README.md) preserve complete target contracts. [Future open-source platform](../cloud/README.md) is a separate design, not a running distributed service.

[Agent guide](../agents/README.md) · [Future open-source platform](../cloud/README.md) · [Launch instructions](../launch/README.md) · [Launch status](../launch/status.md)
