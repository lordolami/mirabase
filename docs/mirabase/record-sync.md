# Record-aware offline synchronization

**Current release authority:** [current implementation inventory](../current-product.md), [user documentation](../user/index.md) and [launch status](../launch/status.md). Initial v0.1/roadmap delivery labels below preserve planning contracts; they do not establish today's implementation or permit deleting subsequently integrated features. [Managed cloud](../cloud/README.md) has separate design and gates.

**Delivery: roadmap.** This chapter specifies later behavior; v0.1 does not build this service. Shared safety rules still apply where used by released features.

## Purpose

Ordinary frontend users may keep authorized application data locally and work offline. Their sync path must preserve the server's collection rules, validation, hooks and file lifecycle. It is not arbitrary SQL replication of the primary database. Global auth hashes, settings, superuser records, credentials and unrelated users' data never enter a user bootstrap.

The client module maintains a local Turso database containing declared application schema, authorized records, local revisions, pending mutations, file manifests and a sync state record. It is an optional companion to the remote SDK; the full Go backend is not embedded in every JavaScript frontend.

## Protocol v1

All operations are under /api/mirabase/v1/databases/{id}/sync and require record.sync plus applicable rules.

POST /bootstrap accepts collections, projection, filter preset ID, knownSchemaRevision and clientInstanceId. The server returns a snapshot session, immutable manifest hash, generation, policy epoch, schema revision, cursor boundary, page count and expiry. Pages contain records and file metadata only; GET /bootstrap/{session}/pages/{index} is idempotent and authorized to that subject. Default session lifetime is 30 minutes. Read a consistent snapshot; queued post-boundary changes are pulled afterward. A permission change invalidates the session.

POST /push accepts clientInstanceId, schemaRevision, generation, policyEpoch and ordered atomic groups. Each group has a UUID groupId and mutations: mutationId, collection, recordId, action=create|update|delete, baseVersion, data and stagedFileIds. Maximum 50 mutations/group and 5 MiB body by default. One group commits atomically in one selected database; groups have explicit independent results.

GET /pull uses the durable change contract. POST /ack records a verified applied cursor and client lease. POST /files/stage initializes an authorized upload using file name, size, content hash and destination field; upload/finalize binds staged bytes to the mutation and expires unused staging after 24 hours.

## Apply and conflict semantics

The server reevaluates current rules/forms and runs record hooks. Base versions are opaque commit-derived record versions. Create conflicts on an existing unrelated ID; update/delete require matching baseVersion unless a superuser-approved collection conflict policy says otherwise. Default policy is reject-and-return-conflict, preserving local pending changes. Never silently discard edits or claim a universal automatic merge.

Response per group is committed, duplicate, conflict or rejected, with mutation field errors/conflict-safe server values and committed cursor/version results. Duplicate groupId with identical canonical hash returns the original receipt; different hash returns idempotency_conflict. Receipt and mutations commit together. A failed mutation rolls back its whole group. Hooks with external effects follow the outbox/idempotency rules.

Remote changes apply atomically to the local authorized base; pending local mutations are replayed against it. Reconciliation failure preserves the prior consistent base and pending queue, with a recoverable conflict report. Retry never wipes unsent edits.

## Local workflow

Open local store; validate manifest/schema and device key; render cached data immediately where available; queue local mutations durably; authenticate online; negotiate capabilities/epochs; bootstrap if needed; push/pull under the configured policy; acknowledge only after local commit. Network loss pauses transfer and keeps pending state. Closing the app flushes local state; it does not guarantee a completed network sync.

Client-generated IDs follow existing record ID constraints; temporary IDs are mapped transactionally with relations/files. Local validation improves feedback but is never server authorization. Offline access is possession of previously downloaded data; revocation cannot guarantee erasure from a malicious/offline device.

## Schema, files and limits

Additive schema changes can be applied locally when the module has a verified migration. Incompatible changes require a fresh base while exporting/preserving pending mutations for resolution. Never execute untrusted server-provided arbitrary JS as a schema migration. Private files download separately with hashes, limits and protected access. Missing bytes are visibly pending.

Expose last successful push/pull, pending groups, conflicts, bootstrap progress, bytes and retention deadline. SDK methods include openLocal, bootstrap, push, pull, sync, pending, conflicts, resolveConflict, checkpoint and close. Native payload availability is language/platform-specific; browser WASM coverage is separately qualified.

## Acceptance

Use real server/client databases and restart both around receipt commit, response loss, local apply and file finalize. Verify duplicate prevention, no credential leakage, revocation, changed rules, relation/file ordering, retention expiry, schema mismatch, device clock skew, network partitions and unsent-edit preservation. A mocked push response is not synchronization evidence.



## Concrete delivery and local identity

Ship the selected browser/native adapters and assets, namespace stores by application/subject/context, preserve unsent edits on logout/switch and reconcile expired uncertain receipts. Deletion compaction requires a generation-changing maintenance publication. See [concrete delivery and local identity](offline-client-delivery.md).
