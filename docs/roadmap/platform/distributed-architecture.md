# Future open-source distributed platform architecture

**Design status:** implementation required. Internal service/API names below are owned contracts to build, not symbols claimed in the standalone source.

This is future work under the [open-source roadmap](../../../ROADMAP.md). Managed operation is a possible deployment of those components, not a current MiraBase-hosted product or a requirement for standalone users. Preserve these design/acceptance obligations without implementing consensus in today's standalone by implication.

## Topology and components

A deployment consists of independent regional cells. Each cell has stateless HTTPS/PG ingress, application-policy/API workers, bounded Turso execution workers, shared replication-journal groups across three fault zones, object snapshot storage, metadata/placement services, queues and monitoring. A project is assigned one home cell; region changes are explicit migrations.

Use a replicated SQL registry for durable account/project/app/DB metadata and resource-usage records. Use etcd for cell membership, short-lived worker leases and compare-and-swap placement publication. Keep per-group placement maps in bounded cache, not a watch per cold DB. Start with 128 virtual placement partitions per cell; this is a selected engineering initial value to benchmark, not a measured capacity. A directory maps project/database to partition/cell; partition count can grow with versioned reassignment.

Journal groups multiplex many databases into one replicated log rather than starting a Raft group/process per DB. Three journal replicas live in distinct fault zones; a quorum is two durable replicas. Membership changes add/catch up a learner before voter promotion. Existing consensus libraries/services are reused; no homemade election algorithm is accepted.

Source rationale: [etcd linearizability/ordering](https://etcd.io/docs/v3.6/learning/api_guarantees/) and [learner changes](https://etcd.io/docs/v3.6/learning/design-learner/). Consensus metadata does not make application files durable by itself.

## Identity and metadata

Durable registry rows include customer/project/app IDs; database ID/generation/profile/schema revision/key reference; region/cell/partition/group; placement epoch; primary/replica assignments; lifecycle state; committed sequence; snapshot manifest hash; retention floor; quota/reservation and job IDs. Identifiers remain opaque and application-qualified.

An internal execution envelope carries verified actor and auth-collection ID, app/database/generation, policy/schema revision, placement epoch, request/idempotency identity, deadline, operation class and credential purpose. Sign/mTLS-bind it between services; never trust a customer-supplied worker path or unverified internal header.

Registry is authority for lifecycle/ownership; committed journal is authority for data recovery. Object manifests use immutable content-addressed keys. etcd route CAS publishes only a validated new placement/generation; an eventual controller metric is not permission to overwrite commits.

## Requests and consistency

Ingress authenticates project/application routing, resolves current placement, and sends work to the assigned API/engine worker. The worker revalidates epoch, generation, lease, authority and budget. Cached stale routes return a typed redirect/stale-placement result internally; ingress refreshes once without replaying an uncertain mutation under a new identity.

Default reads are strong against the current primary's applied committed boundary. A session token binds DB/generation/epoch/last observed sequence; a replica can serve only after applying that boundary and current policy. Wait at most 500 ms, then route to primary or return a retryable condition. Explicit eventual reads can expose bounded measured lag, labeled with applied sequence. Do not weaken auth/schema reads to eventual consistency.

Initially, strong reads obtain a quorum-authorized journal ReadIndex and wait until the local target has applied it. Cached leadership alone is insufficient: a paused old primary must not serve a strong read after losing authority. A future lease-read optimization needs a separately proved monotonic-clock/lease validity protocol and partition tests before use.

HTTP/PG/file/live/sync channels preserve the same actor/target boundary. Streams pin routing epochs; migration sends reconnect/resume information. No ordinary request fans out unboundedly over all customer databases. A cross-DB report is its own authorized workflow with per-source checkpoints.

## Required native commit adapter

The current bridge does not implement this platform commit contract. Add and qualify an owned native prepare/export/apply adapter. Ordinary SQL re-execution is not deterministic recovery for random defaults, hooks, triggers, file changes or external effects.

The proposed interface is:
- `PrepareCommit(tx, expectedGeneration, expectedSequence)` freezes the candidate transaction and returns an immutable redo batch, schema/format version, before/after sequence, page/log checksums and receipt/outbox metadata.
- `ApplyCommitted(batch, durableCommitIndex)` validates the exact predecessor/hash and applies the accepted native change atomically and idempotently.
- `DiscardPrepared(tx)` abandons work that never entered committed journal history.
- `SnapshotAt(sequence)`, `ValidateSnapshot(manifest)`, `ReplayRange(from,to)` support coherent recovery and replica catch-up.

A redo batch is a versioned envelope over actual native persistent mutations, not invented generic WAL bytes. The engine adapter defines WAL/MVCC/FTS/codec coverage and crash ordering from source. It must not expose candidate data before journal acceptance. If the engine cannot safely export a prepared transaction, implement isolated candidate state and a recoverable atomic publication adapter; do not accept an unsupported stub.

For object-backed files, stage and verify immutable object versions/hashes before committing their references in the journal. The log records the exact object/version and coverage policy. Garbage collection reconciles committed references and uncertain jobs before deleting orphan staging. Never acknowledge a file record whose required bytes are neither durable nor recoverable at the declared boundary.

Commit protocol:
1. Check current fenced leadership/placement and reserve resources. Execute authorization, record validation and hooks against the correct target transaction.
2. Prepare and hash one ordered native redo batch. No external effect is delivered yet.
3. Append batch and commit identity through the journal group's consensus state machine; acknowledge durable quorum replication only after the group's commit index advances.
4. Apply the committed batch on the primary. Atomically publish target state/sequence, receipt and delivery intents. Only then return success; the response includes committed boundary.
5. Replicas apply the same accepted bytes in order. Deliver external effects from idempotent outbox consumers after commitment, never from log replay of arbitrary user hooks.

If the journal commits but the primary crashes before response/apply, recovery applies that committed batch and the caller reconciles its original receipt. A journal-rejected/uncommitted candidate is discarded. Unknown outcomes are preserved, not renamed rollback. Concurrent transactions may execute speculatively, but final per-DB commits serialize predecessor sequences.

## Fencing and failover

An etcd lease by itself does not prevent a paused old process writing local files. The journal state machine must reject stale placement epochs on every commit. A worker unable to renew loses write admission immediately; even if it resumes, the journal rejects its epoch. No replica/ingress exposes a local suffix not confirmed by durable committed history.

Controller transitions ready -> suspect -> fenced -> promoting -> catching-up -> ready, with explicit job intent and placement revision. A majority-authorized promotion selects a replica whose applied/history hash covers all committed boundaries. If none does, recover snapshot plus committed journal before readiness. Publish the new epoch by CAS only after verification. Zero accepted-write loss is conditional on actual quorum durability and replay acceptance; no benchmark or lease alone proves it.

Loss of quorum stops writes. Stale reads require explicit eventual policy and a valid generation; strong/auth-sensitive reads fail closed. On cell partition, retain one authoritative writer group. Never promote two disjoint groups from independent cache state.

## Snapshots, replicas and PITR

Create snapshots at an applied committed sequence with writes fenced/drained as required by the engine. Manifest lists every DB/WAL/MVCC/FTS/codec/file artifact, exact hashes, key references and external-object coverage. Upload immutable objects, verify them and only then publish the manifest. Object storage is not a database transaction or leader election service; [S3 consistency](https://docs.aws.amazon.com/AmazonS3/latest/userguide/Welcome.html) applies only to its documented operations.

A replica verifies snapshot then replays committed redo through a known index. Detect missing segments, wrong epochs, incompatible codecs/schema and checksum divergence before serving. Track received/durable/applied positions separately.

PITR restores to an explicit committed transaction boundary, into a new generation, with associated files/metadata and keys. Restore does not undo external emails/payments. Async cross-region DR exposes measured RPO and RTO; regional quorum durability does not imply synchronous cross-region zero RPO.

## Placement, balancing and cold databases

Scheduler considers region residency, failure domains, CPU/memory/query load, disk/inodes/IOPS, replica lag, tenant affinity, reservations and isolation class. Bound opens/wakes and admit by project fairness. Operators see why a target is queued/rejected.

Move: reserve destination -> copy verified snapshot -> tail journal -> fence/drain source at sequence -> verify destination coverage -> CAS placement epoch -> route new traffic -> reconcile old leases -> retain old copy through recovery retention -> purge safely. Restart at each phase from durable job intent. Any post-cutover committed write forbids naive snapshot rollback.

Cold DBs retain registry/snapshot/log state without warm connections/VMs. Idle suspension flushes a verified checkpoint/snapshot boundary; wake hydrates validated state and journal before ready. Deduplicate concurrent wakes. Rate-limit churn and retain explicit cold-start latency/queue status. Never fetch an unbounded object set into memory per request.

## Application code and isolation

Customer hooks are untrusted relative to other customers even if standalone hooks are operator-trusted. Shared API/storage workers must not execute arbitrary customer Go/JS with host-wide access. Use isolated project execution pools/microVMs for custom code and a scoped internal storage API; Go custom backends may require dedicated runtimes. No process per tenant DB is required. Persist artifact hashes and sandbox policies.

Secrets/KMS references remain project-specific; caches include app/generation/policy. Files/object prefixes, credentials, logs, snapshots and support tooling enforce the same boundary. Native replication is whole-DB authority and cannot masquerade as path/record-filtered access.

[platform acceptance](acceptance.md) must prove these protocols before customers receive distributed availability claims.
