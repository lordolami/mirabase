# Future platform operations, quotas and economics

**Status:** product/engineering design. No prices, margin, SLA or certified capacity are established.

These are future open-source platform/operator capabilities under the [roadmap](../../ROADMAP.md). Cost accounting informs capacity and fair resource use. Optional charging/invoice mechanisms describe what an operator might implement; this document does not select a MiraBase commercial offering or impose a fee on standalone users.

## Resource economics

Database count is not free: every cold DB has metadata, snapshot/log retention, object operations, keys, inode/cache and support cost. Active DBs add CPU, native cache, connections, I/O, replication and network. Multiplexing reduces per-DB idle compute; it does not remove storage or maintenance costs.

Candidate operator accounting/charging units are compute time, retained storage GiB-time including replicas/backups where stated, operations/IO, network egress and optional service/isolation classes. A future managed operator must publish allowances/rounding/limits and bundled costs before charging. No tariff or MiraBase-operated service is selected. Avoid a process per DB while protecting metadata/create-churn abuse.

Every usage event has project/resource, meter kind/version, interval, integer quantity/unit, region, unique event ID and source watermark. Deduplicate ingestion and append adjustments rather than rewriting billed history. Use integer money units and explicit currency/tax records. Invoices reconcile with aggregated usage and retain disputed/provisional state.

Model monthly contribution as revenue minus compute, durable/replica/object storage, IOPS/object operations, ingress/egress, control-plane, monitoring, backup/DR, payments and support. Benchmark many cold DBs separately from active workloads; don't infer profitability from empty-file create speed.

## Quotas and admission

Account/project quotas cover stored bytes/inodes/DB metadata, active compute/handles, connection/query concurrency, transaction/body size, jobs/import/wake rate, snapshots, retention and egress. Limits are explicit abuse/resource controls even when no per-DB charge exists. Free/evaluation plans can cap resource usage; do not market infinite storage/DB count.

Reserve before provisioning/import/restore; finalize/release against actual durable state. Expired reservations reconcile uncertain effects first. Fair queues separate foreground, recovery and background work. Return retryable resource exhaustion with reason/wait information; never evict active transactions to fake capacity.

Scale controllers use sustained utilization/queue/lag/disk headroom and hysteresis, with cost ceilings and explicit maintenance windows. Promote hot DBs to dedicated capacity, spread placements and drain safely. Autoscaling cannot repair a missing index or serialize-away a hot-row bottleneck.

## Security and observability

mTLS/service identities, project-scoped secrets and envelope keys, purpose-bound credentials, current grant/policy epochs, private networking/domain verification, audit and support-access controls are required. Key rotation retains historical keys needed for recovery. Erasure must include retained snapshots/log/object versions according to the published retention/legal policy; deleting a registry row alone is not complete erasure.

Metrics distinguish native memory, handles, query latency/errors, commit quorum latency, replication durable/applied lag, cold wake, snapshot age, retention floor, restore time and per-project resource use. Logs redact data/secrets; audit administrative access and support actions. No compliance certification is implied by audit features.

## Runbooks

- Node loss: fence, select eligible committed history, recover/promote, CAS routing, validate client receipts and investigate durability.
- Quorum loss: stop writes, preserve logs, restore majority safely; never force two independent writable histories.
- Disk full/corruption: backpressure/quarantine affected targets, retain verified replicas/snapshots, restore without destructive guesswork.
- Overload: project-fair admission, throttle background churn, scale/reschedule with verified cutover.
- Key loss: stop affected recovery/access, recover through authorized key service/backups; do not silently create empty replacement DBs.
- Region outage: declare DR RPO/RTO, restore verified remote history, publish new routing only after fencing and integrity; report actual loss if any.
- Upgrade: signed version/format manifest, cell canary, rolling compatibility window, bounded drains, verified backup, rollback only if format/state allow it.
- Incident: preserve evidence, identify impacted projects/boundaries, communicate actual scope/times and keep follow-up recovery receipts.

Backups, replicas, PITR and DR each need actual restore drills. A healthy metric or three stored copies is not proof of recoverability.

## Initial SLO instrumentation

Record availability, strong-read/write latency distributions, cold-start latency, recovery time and accepted-write integrity by cell/tier. Choose public numeric SLOs only after representative multi-zone failure/load evidence and operating costs. Customer-facing terms must match the actual durability and region model.
