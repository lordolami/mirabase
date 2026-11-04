# Realtime, changes and live queries

Inherited record SSE is transient notification. Durable changes are transaction history. Live queries maintain a published result. One is not a substitute for another.

## Pick a change surface

| Surface | Consume | Recovery |
|---|---|---|
| Record SSE | Authorized create/update/delete notifications | Reconnect and resubscribe. |
| Durable changes | Ordered committed history | Resume inside retention or fetch a fresh snapshot. |
| Live query | Initial result and keyed deltas | Bounded replay or whole-result reset. |

## Live-query workflow

1. An operator creates a plan at `/api/mirabase/v1/live-query-plans` with reviewed SQL, typed parameters, dependencies, unique result key and limits.
2. Inspect/publish the exact revision; draft/retired plans are not interchangeable with the reviewed published revision.
3. Register an instance through `POST /api/mirabase/v1/databases/{id}/live-queries` with its actual request DTO.
4. Consume `/live-queries/{queryId}/stream`. Apply the initial snapshot before ordered keyed changes and persist complete boundaries.
5. Replace the result on reset, stop on revocation and cancel the instance when finished.

Typed parameters are not raw SQL slots. Do not turn arbitrary ordinary-user SQL into an operator plan or retry denied record access with SQL authority. See [actual DTOs/routes](api-reference.md) and [plan implementation](../../internal/application/derived.go).

Operators create a named immutable live-plan revision with typed parameters, dependencies, unique result key and limits, then publish it. Clients register the approved plan for an authorized target, consume its initial snapshot and keyed ordered deltas, persist the completed boundary and cancel when finished. Retiring/publishing revisions must not silently change existing instances.

Current ordinary-actor policy injection has restrictions: multi-collection, aggregate/window or other shapes that cannot be safely narrowed may be refused. Privileged SQL is not an ordinary record subscription. Recheck authorization during replay/evaluation and reset/terminate on revocation or incompatible changes.

The implementation captures a snapshot/journal boundary and computes insert/update/delete/order changes with bounded replay. The design defaults are five minutes/8 MiB, not unlimited history. Apply complete frames atomically. Reset means replace the local result with a fresh authorized snapshot.

Durable endpoints are `/databases/{id}/changes` and `/databases/{id}/changes/stream` under the additive base. Use actual handler query/response fields; a planned opaque cursor is not interchangeable with an implemented sequence argument. Never expose password/token history.

Capture on records, privileged SQL, PG and native replication needs transport-specific evidence. A journal table does not prove every writer participates. [Live tests](../../apis/mirabase_live_acceptance_test.go), [journal tests](../../apis/mirabase_records_journal_acceptance_test.go), [actual API](api-reference.md), [launch status](../launch/status.md).
