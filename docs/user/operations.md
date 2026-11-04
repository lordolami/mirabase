# Operations, recovery and server moves

The latest Windows concurrent-backup regression passes with the owned native patch; its earlier backup-export failure is superseded by that execution. Complete backup/restore/fault/platform reliability remains unqualified. See [executed launch findings](../launch/status.md) for exact runs. An endpoint returning success is not full recovery acceptance.

## Operator capabilities

| Area | Current surface | Check before considering the task complete |
|---|---|---|
| Databases | List/get, maintenance, profile, lifecycle and owner transfer | Current generation, actual state and per-target effects. |
| Jobs | List/get/cancel | Terminal result, failures and completed steps. |
| Receipts | Read persisted operation outcome | Original identity/body and unknown/expired state. |
| Backups | Application and managed-target archives | Checksums, included files and independent reopen/restore. |
| Restore | Validation, dry run and target restore paths | Authority, complete coverage, new generation and application behavior. |
| Interchange | Portable manifest export/import | Typed rows, stable IDs, schema and file checksums. |
| Maintenance | Integrity/checkpoint/vacuum/FTS/GC requests | Engine support and actual job result, not accepted action name. |
| Diagnostics | State/generation/connections/leases/disk/WAL/FTS | Resource headroom and failures over the intended workload. |

Method/path/DTO details are in the [bound reference](api-reference.md). Unsupported native-sync compaction is refused. A maintenance action returning a failed job does not count as a completed repair.

## Follow a long-running operation

1. Read the target generation/revision and capture the intended operation.
2. Submit with the required guards and stable idempotency identity. Preserve the returned job/receipt ID.
3. Inspect `/api/mirabase/v1/jobs/{jobId}` until a real terminal result is known. An HTTP acknowledgement is not the result.
4. On timeout/lost response, reconcile the same operation identity. Do not create a new operation to disguise an unknown commit.
5. Inspect committed steps before cancelling or retrying. Cancellation is not rollback of already committed work.

## Verify backup coverage

| Component | Include or account for |
|---|---|
| Application state | Primary and auxiliary stores, schema/settings and needed secrets. |
| Managed databases | Database files, native sidecars, manifests and registry consistency. |
| Local assets | Record files, trusted hooks/migrations/public artifacts as appropriate. |
| Remote objects | Explicit S3 coverage or a verified independent recovery source. |
| Recovery metadata | Archive checksums, format/profile/generation and required protected settings/keys. |

A copied `data.db` alone is not a full application backup. Use coherent native snapshot/backup mechanisms, account for active writers/workers, and independently validate the archive. A successful archive download does not prove it contains your remote objects.

## Restore into an isolated destination

Preserve the current state first. Validate archive paths, sizes, hashes, schema/profile and authority before publication. Use dry-run/restore-to-new behavior where provided. Keep external mail/hooks/jobs disabled while checking recovered identities, records, protected files and operation state. Restore changes generation; stale cursors/leases/tokens must not be silently reused.

The inherited restore-and-restart route is not a qualified Windows solution. Use an explicitly verified stop/restore/start procedure for that platform, following the actual executable/operation interfaces; no generic restore command is invented here.

## Move to a larger server

1. Obtain and independently validate complete recovery state, including remote-object coverage.
2. Transfer application state and protected configuration through your chosen secure channel.
3. Validate the destination in isolation with external effects disabled.
4. Stop/fence the old writer and drain work before activation. Independent processes must not write the same live files.
5. Activate the destination, update endpoint/proxy/DNS as appropriate and check user login, rules, files and background work.
6. Preserve recovery evidence. Once new writes are accepted, an old snapshot is not a safe lossless rollback.

## Monitor and protect recovery state

Keep DBs on durable storage. Monitor disk/inodes, WAL/native sidecars, files/object storage, staging/trash, handles/connections, admission queues, journal/reference lag, failed jobs and payload extraction errors. Resource limits are different from vendor database-count quotas.

Native page encryption, page-key rotation and custom codecs are currently refused by the create/rotation handlers. Settings protection and outbox-envelope encryption are separate mechanisms; they do not encrypt native pages/WAL or certify archive encryption. Portable export/import contains code but needs actual format/coverage acceptance. Required existing protected configuration must remain recoverable; missing keys cannot justify deleting pending work.

Use an owned signed update feed. Test upgrades against existing data and clients; incompatible format downgrade requires a real recovery path. [Launch instructions](../launch/README.md) and [status](../launch/status.md) own qualification.
