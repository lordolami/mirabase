# MiraBase agent guide

Agents use normal authenticated HTTP APIs; no additional MCP server is required. Localhost access still has rules and credentials.

## Read before operating

| Need | Reference |
|---|---|
| Full feature inventory with integration boundaries | [Feature catalog](../features.md). |
| A task workflow | [User guide index](../user/index.md). |
| Actual methods, paths and handler authority | [Bound API reference](../user/api-reference.md). |
| Concrete implementation and tests | [Current product](../current-product.md). |
| Executed acceptance and missing targets | [Launch status](../launch/status.md). |

The current managed-record handlers do not yet apply complete inherited collection rules/forms/hooks/hidden-field behavior. Database owner/grant acceptance does not close that gap. Workspace HTTP is operator-only; record bootstrap/page is also operator-bound. Native page-key rotation is refused, not an operation to retry with more authority. Plan around the real current boundary instead of elevating a user's token or treating design prose as an implemented route.

## Connect and discover

Receive an explicit server URL, your own agent identity/session and authorized database/tenant. Discover health at `/api/health`, OpenAPI at `/api/mirabase/v1/openapi.json` and authenticated capabilities at `/api/mirabase/v1/capabilities`. Compare advertisement with [actual handlers](../user/api-reference.md) and [verification](../current-product.md).

Bind application then target immutably per request. A display name, owner field, record ID or selector header is not authority. Keep generation/schema/policy boundaries. Never switch a global client context while requests are pending or send host paths/native handles.

## Data work

Use inherited or managed record APIs with their actual rules, typed fields and query limits. Escape search highlights. Protected files, relations, search, change streams and live queries must use the same current identity/target. Do not replace a denied record action with administrative SQL.

Apply complete change/live frames, persist the boundary and reset from an authorized snapshot after expiry/gaps/policy change. Disconnection is not proof of no changes. Pending edits remain tied to their original account/target; already downloaded bytes cannot be reliably remotely erased.

## Operator actions

Provisioning, schemas, grants, imports, backups/restore, ownership and current workspace HTTP operations have operator requirements defined by their actual handlers. Workspace routes are currently superuser-only. Native page-key rotation is unavailable in the current handler. Ordinary agents must stop on authorization failure rather than borrowing an owner's session.

A trusted coding/operator agent may use deliberately delegated credentials. Before irreversible work, present exact targets, revisions, effects and recovery under the operator's approval policy. Do not invent approval steps for already authorized reversible work. Never invent job/backup success.

## Retry and recovery

Persist idempotency key, canonical body, generation and receipt/job IDs. Reuse them for a safe retry. Conflicts require rereading actual state, not blind overwrite. Poll jobs to durable terminal results and inspect per-target committed steps. Unknown outcomes require reconciliation; missing receipt history is not rollback proof.

Distinguish auth denial, not found, schema/generation mismatch, policy change, maintenance, resource exhaustion, conflict, cursor expiry and outcome unknown. Back off only on documented retryable conditions. External hooks/payment/network effects are not exactly-once. Redact secrets and hidden payloads.

## Workspace and build boundaries

Current workspaces provide selected local files/snapshots/history helpers. Full upstream AgentFS KV/inode storage, mounts, overlays/session apply and offline replication have additional implementation requirements. Snapshots do not preserve an LLM process or undo external effects.

For coding, read [build instructions](../../AGENTS.md), [current inventory](../current-product.md), owning symbols and actual tests. Old v0.1 allocations are planning records, not permission to delete later features. Separate design, implementation presence, testing and release qualification. [Cloud design](../cloud/README.md) is a separate managed service, not a hidden standalone capability.

Run [launch checks](../launch/README.md), retain real artifacts and report failures. Do not publish/deploy/send messages/create remote repositories without authorization.
