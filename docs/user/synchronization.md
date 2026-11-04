# Record and native synchronization

The current server binds record bootstrap/page/push/pull/ack and file stage/upload. These HTTP operations are different from upstream Turso whole-database native replication.

## Which change channel do you need?

| Need | Surface | Persistence |
|---|---|---|
| Tell an online UI that a record changed | Inherited realtime SSE | Notification; reconnect/resubscribe as required. |
| Read committed record history | Changes pull/stream | Retained journal with reset/expiry boundaries. |
| Maintain a published query result | Live-query snapshot/deltas | Bounded result replay, reset and cancellation. |
| Exchange client record edits | Record sync | Bootstrap, grouped push, pull and acknowledgement. |
| Replicate native database state | Upstream native protocol integration target | Not qualified by the record HTTP endpoints. |

## Current record-sync sequence

All paths below start with `/api/mirabase/v1/databases/{id}/sync`.

1. An authorized operator creates `/bootstrap`; fetch its `/bootstrap/{sessionId}/pages/{index}` pages and retain the returned snapshot boundary.
2. Keep edits scoped to the original server/application/account/database/generation. Preserve stable mutation identities and expected versions.
3. Submit mutation groups to `/push`, inspecting actual per-group/receipt outcomes rather than assuming a transport success means every edit committed.
4. Read `/pull` from the retained cursor and apply complete committed boundaries.
5. Acknowledge durable local application through `/ack`.
6. For file edits, use `/files/stage` and `/files/upload` with the expected path/hash/client identity and reconcile interrupted publication.

Use the [actual handler declarations](api-reference.md) for request bodies and query fields. The snapshot/page steps are currently superuser-bound, so this is an operator/API workflow, not a completed ordinary-user offline onboarding recipe. The SDK wrappers do not implement a durable local database for these steps.

## Reconnect and recover

Keep unsent intent after a disconnect. Reuse operation identities when reconciling lost responses. Expired history, generation/schema/policy changes, failed file publication and revocation are distinct outcomes. Rebootstrap only through an authorized workflow and preserve unresolved edits for reconciliation; do not attach one account's pending writes to another account after a login switch.

Bootstrap an authorized snapshot, fetch all pages, preserve its boundary, push grouped local changes with stable operation identities/expected versions, pull committed changes and acknowledge the durable position. Keep local intent until the outcome is unambiguous. Lost responses reuse the same identity; never fabricate new IDs to hide uncertain commits.

Current authority is not uniform: bootstrap/page handlers are superuser-bound; push/pull/ack/files have their own target checks. See [actual routes](api-reference.md). Do not put superuser credentials into a frontend to make bootstrap work; ordinary-user offline qualification must resolve that authority integration.

Stage file bytes using upload/hash expectations and publish only after record/file metadata commits. Interrupted uploads are not success. Deletes, expired receipts and generation changes require conflict/rebootstrap behavior that preserves unsent edits. Revocation stops future access but cannot guarantee erasure of already downloaded bytes.

Namespace local state by server/application, subject collection+record, DB and generation. Switching accounts/tenants cannot attach pending edits to another identity.

The current JS/Dart packages are remote context clients, not complete browser/mobile/native offline stores. OPFS, device keys, local Turso assets, native partial bootstrap and checkpoint compatibility need real adapters and target tests. No upstream protocol interoperability is established merely by record-sync routes.

See [record design](../mirabase/record-sync.md), [native design](../mirabase/native-sync.md), [client delivery](../mirabase/offline-client-delivery.md), [actual API](api-reference.md) and [launch evidence](../launch/status.md).
