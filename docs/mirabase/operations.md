# Operator workflows

**Current release authority:** [current implementation inventory](../current-product.md), [user documentation](../user/index.md) and [launch status](../launch/status.md). Initial v0.1/roadmap delivery labels below preserve planning contracts; they do not establish today's implementation or permit deleting subsequently integrated features. [Managed cloud](../cloud/README.md) has separate design and gates.

**Delivery: v0.1 and roadmap.** [v0.1-build.md](v0.1-build.md) selects the current portion; [roadsmap.md](roadsmap.md) retains the remainder. Detailed schemas/configuration/ABI and full-family acceptance below describe the target superset unless narrowed by that allocation.

## Observability

Retain logs, log stats and health. Add nonsecret metrics for requests/latency/errors by operation, open/leased/cold handles, queue length, SQL/native memory, file descriptors, disk/WAL/FTS/storage/trash bytes, conflict/retry counts, change retention/lag, sync work, job states, schema revisions and backup age. Avoid per-record/unbounded database labels in global metrics; use bounded aggregate metrics plus paginated per-context diagnostics.

Logs carry request/job/database/transaction IDs and error codes, not passwords, bearer tokens, signing keys, raw SQL parameters, protected records or file URL secrets. Audit administrative changes with actor, operation, target, timestamp, result and request hash. Audit retention is configured; absence of a compliance certification must not be disguised by having an audit table.

## Diagnostics and maintenance

Superusers can inspect effective capabilities/config, manifest/ABI/native revision, context state, schema integrity, connection leases, active queries, queues, retention floor, replication/checkpoint status and job effects. Default diagnostics are read-only. An integrity/repair action states exclusive access, estimated scope and rollback prerequisites.

Checkpoint, vacuum, FTS optimize/rebuild, GC, codec/key rotation and schema migration are separate actions with profile support checks. Do not run blanket VACUUM/checkpoint against a synced/MVCC/encrypted context. Maintenance enters/leaves through the lifecycle manager and records durable intent.

## Runbooks

Startup fails: preserve data; read sanitized error; verify payload/ABI/key/path/permissions/disk/manifest; run read-only validation on a copy; recover job state; never initialize over an unopenable database.

Disk pressure: stop new bulk writes/uploads/backups, report categories, expire safe staging/trash/retention according to policy, checkpoint only where supported, expand disk, validate integrity. Do not delete live WAL/SHM/sync sidecars manually.

Sync retention expiry: preserve pending local edits, create a new authorized base, reconcile/export unresolved mutations and resume with new cursor. Do not reset the local app and lose unsent data.

Failed rollout: inspect exact target/phase/revision, keep unaffected contexts serving where compatible, fix/retry idempotently or restore a verified target backup. Completed targets remain recorded.

Lost key: locate the operator recovery key/copy; do not promise recovery from encrypted pages without a key. Rotate only after a recovery-tested replacement.

Server move: follow the coherent transfer procedure, prevent split writers, verify public endpoint/auth/files/sync on the destination, then cut over. Log the rollback boundary.

## Upgrades

Upgrade dry run checks app/native/schema/codec/protocol/SDK compatibility and free disk. Take/verify backup. Stop/drain, install signed artifacts, migrate in maintenance, run smoke/integrity checks and publish readiness. A failed migration preserves old copy and detailed state. Native-format downgrade requires a supported export/import path; replacing binaries alone is not guaranteed rollback.

## Qualification

Test real runbooks with an empty machine, a representative app, private databases, remote file outages and an unavailable dashboard. Every repair command requires actual result checks. Monitoring a queue without detecting permanently failed or orphaned work is insufficient.



## Scoped recovery and operator controls

Document exact ownership transfer, quarantine recovery, reference refresh, layout migration, live-plan retirement, rollout pause/resume/retry, per-app backup/restore and reservation reconciliation. Diagnostics/audit obey the selected application or explicit instance-operator boundary. See [scoped recovery and operator controls](implementation-closure.md).
