# Storage and database lifecycle

**Current release authority:** [current implementation inventory](../current-product.md), [user documentation](../user/index.md) and [launch status](../launch/status.md). Initial v0.1/roadmap delivery labels below preserve planning contracts; they do not establish today's implementation or permit deleting subsequently integrated features. [Open-source platform roadmap](../roadmap/platform/README.md) has separate design and gates.

**Delivery: v0.1 and roadmap.** [v0.1-build.md](v0.1-build.md) selects the current portion; [roadsmap.md](roadsmap.md) retains the remainder. Detailed schemas/configuration/ABI and full-family acceptance below describe the target superset unless narrowed by that allocation.

## Layout and format

The application root contains mb_data/data.db, auxiliary.db, storage/, backups/ and certificates/settings artifacts retained from the inherited layout. Optional partitioned databases live at mb_data/databases/<databaseId>/data.db; their record files live below that same managed context's storage/ root. Database IDs are 32 lowercase hexadecimal characters generated server-side. No display name, email, auth ID or request path becomes a filesystem component.

Native WAL/SHM/MVCC/sync sidecars belong to the engine format, not a guessed fixed list. A versioned manifest enumerates required files and their engine roles. Native library extraction lives outside application data in the private versioned cache. Backup staging and uploaded temporary files use separate bounded private directories.

A private manifest contains formatVersion=1, databaseId, generation (positive 64-bit integer encoded as decimal text in JSON), profile, schemaRevision, templateId, encryptionKeyRef, codecId and creation timestamp. It never contains encryption key bytes. The primary registry is authoritative for routing; the manifest detects mismatched files, not authorization.

## Optional system metadata

The registry is installed only when database lifecycle/partition features are enabled. These are protected internal tables/collections, not public record endpoints.

| Object | Required columns and constraints |
|---|---|
| _mb_databases | id PK; ownerCollectionId and ownerRecordId nullable together; kind primary/tenant/user/agent/manual; state; generation; profile; templateId; schemaRevision; policyEpoch; displayName; created; updated; lastErrorCode nullable |
| _mb_database_grants | id PK; databaseId FK; subjectCollectionId; subjectRecordId; sorted scopes; expiresAt nullable; revokedAt nullable; epoch; unique database/subject binding |
| _mb_agent_bindings | id PK; authCollectionId plus authRecordId unique; ownerCollectionId plus ownerRecordId; registrationState; created |
| _mb_jobs | id PK; kind; databaseId nullable; requestHash; idempotencyKey; actorRef; state; phase; progressCounters; leaseOwner; leaseUntil; attempts; resultRef; errorCode; created; updated |
| _mb_schema_templates | id PK; immutable revision; definitionsHash; normalized collection/type/index definitions; compatibility requirements; publicationState |
| _mb_schema_rollouts | id PK; template/revision; target snapshot; per-target job/result; stop policy; created |
| _mb_import_sessions | source/plan/preview/validation hashes, revision, phase, staging references, generation guard, actor, job/recovery artifacts and retention |
| _mb_tenant_routes | route namespace PK; active epoch; membership/selector policy; template; policy epoch |
| _mb_tenant_bindings | namespace/epoch/typed-key hash/template → database/generation; unique candidate binding; activation through the route pointer |
| _mb_unique_reservations | namespace/index/normalized-value hash+bytes; database/generation/record/transaction tag; reserved/committed/releasing; reconcile before releasing uncertain writes |

Target-local metadata includes _mb_schema(version/template/revision), _mb_changes and _mb_change_rows(transaction/change records), _mb_record_versions(current commit-derived record version and deletion marker), _mb_sync_clients(lease/ack epoch), _mb_receipts(idempotency key/request hash/result reference), and _mb_outbox(side-effect intent/state). Record versions survive change-log expiry; otherwise a still-present record would lose its conflict identity. Tombstone versions prevent stale offline resurrection and remain until a separately verified compaction/rebootstrap policy permits removal. System names are reserved. Dates are UTC; binary data stays binary in storage; hashes include algorithm/version. Table constraints, indexes and migrations must be emitted from the storage contract and reviewed, not improvised by clients.

## State machine

Create: requested → provisioning → ready; errors → failed. Provision into an unpublished temporary directory; create schema, validate native capabilities, write manifest, fsync according to the selected durability profile, atomically publish path, then mark registry ready. Recovery reconciles the atomic rename/registry gap by database ID and job ID. A request never routes to provisioning or failed state.

Ready may enter maintenance, migrating, restoring, deleting or quarantined. Maintenance returns retryable 503 to data operations except allowed diagnostics. Schema rollout keeps an explicit per-target state. Unknown manifest/generation or missing required sidecar quarantines the database. Do not create an empty replacement.

Delete: ready → deleting → tombstoned → purged. Revoke grants/increment policy epoch first; stop new leases; drain/cancel active operations; close handle; move the complete context to managed trash; record tombstone; purge after operator-configured retention. Default trash retention is 24 hours; zero requires an explicit setting. Expired grants/replica URLs cannot read trash. Deletion includes context files, previews, native sidecars, sync state and applicable remote-object cleanup jobs. Report pending object cleanup.

Restore publishes a new generation and invalidates all earlier cursors, native replica revisions and outstanding context leases. Database IDs are never reused for unrelated data. Restore to a new ID is supported and does not overwrite a live target by accident.

## Admission and caching

The default maximum is 64 open optional database handles, 8 concurrent opens and a 30-second idle-close delay. These are operator-tunable resource limits, not vendor database-count quotas. Primary/auxiliary use separately budgeted reserved handles. File handles, native memory, page caches, query slots, subscriptions, WAL bytes, sync clients and jobs all have global and context limits.

Evict least-recently-used unleased handles. A fully leased cache queues bounded work, then returns resource_exhausted with retry guidance; it never closes an active transaction. Access after eviction reopens/verifies the manifest. Storage usage includes main files, sidecars, FTS artifacts, local files, trash and staging; report categories separately.

## Recovery and durability

Every mutating lifecycle job is idempotent, leased and resumable. The filesystem is not ACID with the registry: persist intent, use deterministic job-owned staging, detect completed steps and reconcile. Never delete a directory solely because it is absent from a stale registry snapshot. Diagnostics first identify orphaned/unknown contexts; operator repair chooses import or quarantine.

Acceptance interrupts every transition before and after registry writes/rename/fsync, restarts the server, retries identical and conflicting idempotency keys, and verifies exact ownership, file hashes and routing state.



## Instance versus application metadata

storage-format-reference.md defines the lazy instance registry; storage-format-reference.md is application-local. Default startup needs no instance catalog. Manifest and credential identities include application context. Operational closure defines transfer, reservation and compaction recovery. See [instance versus application metadata](application-isolation.md).
