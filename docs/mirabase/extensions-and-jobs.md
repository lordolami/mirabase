# Extensibility, hooks, cron and durable jobs

**Current release authority:** [current implementation inventory](../current-product.md), [user documentation](../user/index.md) and [launch status](../launch/status.md). Initial v0.1/roadmap delivery labels below preserve planning contracts; they do not establish today's implementation or permit deleting subsequently integrated features. [Open-source platform roadmap](../roadmap/platform/README.md) has separate design and gates.

**Delivery: v0.1 and roadmap.** [v0.1-build.md](v0.1-build.md) selects the current portion; [roadsmap.md](roadsmap.md) retains the remainder. Detailed schemas/configuration/ABI and full-family acceptance below describe the target superset unless narrowed by that allocation.

## Existing extension contract

Keep Go framework APIs and JavaScript hooks/migrations/helpers. Preserve event ordering, cancellation, hook continuation and transaction behavior. New typed fields register through the existing field registry. Query functions use the typed search registry. Dashboard extensions use the existing UI extension mechanism. Do not add a second plugin runtime as a prerequisite.

Goja is not Node.js. Document available globals and limits from generated bindings, not assumed npm support. Go/JS filesystem/network/native helpers are trusted operator capabilities. Do not accept uploaded arbitrary hook source from an ordinary auth record.

## Context propagation

New events include immutable context metadata: databaseId, generation, schemaRevision, actorRef, requestId, transactionId and source. Existing default-context callbacks continue to work. Added events include database lifecycle, template rollout, change normalization, sync conflict and credential revocation. A hook runs only in its documented phase and cannot mutate a closed transaction.

One process-wide JS runtime pool serves the applications; leases are bounded and released after exception/cancellation. No persistent per-database VM, scheduler or router exists. Callback bindings resolve current invocation context rather than sharing mutable global actor state.

## Cron

Preserve registered jobs, scheduling, manual run and log behavior. Cron registration is Go/JS application configuration. API list/run does not become arbitrary remote code installation. Multi-database jobs iterate using paginated snapshots and bounded admission; do not open every database at once.

The supported runtime is one process with explicitly isolated applications. Shared .notify/native multi-process WAL features are not a guarantee of exactly-once cron across multiple active replicas. An external leader/job mechanism would require separate qualification.

## Durable jobs

Use durable jobs for provisioning/deletion, schema rollout, index rebuild, backup/restore, encryption rotation, bulk export/import and remote-file cleanup. States are queued Ã¢â€ ’ running Ã¢â€ ’ succeeded|failed|cancelled. Running may enter waiting_retry; cancellation is requested before it is acknowledged. A succeeded job has a durable result manifest. Failure has code, phase, retryability and committed-step records.

A worker leases a job for 30 seconds, renews every 10 seconds, and reconciles an expired lease before retrying. Retry uses deterministic step IDs and idempotent effects. Default maximum 8 attempts; nonretryable schema/auth/capability failures stop immediately. No job result stores credentials or raw record payloads unnecessarily.

Cancellation stops at a safe step boundary, completes/rolls back active native work and records already committed effects. Restore's publication point is non-cancellable; UI states that before apply. A cancelled rollout can have completed targets, which remain visible.

## Outbox and callbacks

Persist side-effect intent with the relevant data transaction. Workers deliver at least once using event IDs/idempotency keys; external destinations need deduplication. Never claim exactly-once remote HTTP/mail delivery. Signed callbacks require destination allowlists/SSRF controls, secret rotation, bounded bodies/timeouts and redacted logs. Custom user hooks doing direct I/O retain their explicit unsafe-retry boundary.

Outbox payloads can include reset tokens and are encrypted with AES-256-GCM and a random 12-byte nonce. Associated data binds database ID, generation, outbox event ID and payload version. The envelope stores version, key reference, nonce, tag and ciphertext; key bytes are separate. Default local delivery keys are generated atomically into owner-private mb_data/keys with platform-appropriate ACLs; no cloud account or manual key setup is required. Operators may replace the local key reference with their configured secret provider. Rotation retains keys needed by pending jobs. A backup must declare whether required delivery keys are externally recoverable; missing keys stop delivery rather than dropping encrypted pending effects or sending corrupted messages.

## Verification

Test worker crash after effect/before acknowledgment, lease expiry, duplicate workers, repeated cancel, shutdown/restart and context mixing. Preserve inherited custom route/migration/hook fixtures. A scheduler ticking or a job row changing to succeeded is insufficient without verifying the target files/schema/effects.



## Multiple applications, shared bounded services

One scheduler, bounded JS pool and workers use application/revision-qualified artifact and callback registries. Do not bootstrap a new complete app runtime per database. Go extensions are compiled trusted modules; JS imports require operator-installed verified artifacts. See [multiple applications, shared bounded services](application-isolation.md).
