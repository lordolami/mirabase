# Future open-source platform implementation and acceptance gates

This is a separate gate from the standalone release. No current binary or source registration satisfies these distributed tests automatically.

The platform is future [open-source roadmap](../../../ROADMAP.md) work. Managed-deployment acceptance remains documented here for any future operator, without asserting a live MiraBase hosting service. These production gates are distinct from hackathon eligibility and demonstration evidence.

| Gate | Required artifacts and real effects |
|---|---|
| C1 Identity/control plane | Real account/project/app/DB lifecycle, authority negatives, immutable revisions, resumable jobs, retention and API/dashboard parity |
| C2 Native prepare/replay | Exact WAL/MVCC/FTS/file/schema coverage, prepared visibility isolation, byte-identical redo, committed receipt/outbox atomicity and pinned-format compatibility |
| C3 Journal quorum | Three-zone durable replication, committed indexes, fenced epochs, learner membership, fault injection and no acknowledged-write loss |
| C4 Routing/consistency | Stable endpoints, stale-cache refresh, primary/session/explicit eventual reads, generation/policy fences, long HTTP/PG/SSE/sync reconnect |
| C5 Failover | Kill/pause/partition leaders at every prepare/quorum/apply/response boundary; prove one writer and correct uncertain-outcome reconciliation |
| C6 Placement/balancing | Concurrent create/reserve/move/drain/restore, exact snapshot catch-up, CAS cutover, post-cutover write preservation and no peer data impact |
| C7 Capacity/economics | Actual large cold population, representative active working set, inode/object/metadata costs, wake storms, fair admission, budget and margin inputs |
| C8 Recovery | PITR and disaster restore with files/keys/format coverage; actual RPO/RTO and corrupt/missing-log/key negatives |
| C9 Isolation | Wrong customer/app/agent identity, SQL/record boundaries, hook sandbox escape attempts, object prefixes, cache/metrics/support access and key separation |
| C10 Migration | Real standalone-to-platform and exit restores with schema/record/file/auth/rule invariants, reviewed URL/credential changes and recoverable cutover |
| C11 Usage/quotas | Deterministic resource usage, reservation uncertainty, quota/admission reconciliation and create-churn abuse |
| C12 Operations/release | Signed reproducible services/assets, multi-cell canary/upgrade, full runbooks, incident/restore drills, dashboard/API docs and published tested limits |

Maintain a platform implementation ledger with exact service/native symbols, source/toolchain/digests, fixtures, test counts and target runs. A simulator/model or interface is useful evidence of its own layer, never a substitute for real native/distributed effects.

Failure suite: power loss, paused old primary, leader/replica loss, zone partition, etcd outage, journal quorum loss, disk full, malformed redo/snapshots, object inconsistency/missing objects, key rotation/loss, certificate expiry, overload/wake storms, migration response loss, stale policy/schema/generation and rollback over later writes.

Performance runs identify hardware, storage, region distance, dataset sizes, DB population versus active concurrency, durability mode, cache state, hooks/files, errors and p50/p95/p99. A million metadata rows or empty DBs do not prove a million active backend workloads.

Formal models may verify epoch/ownership/publication state machines, but claims are bounded to modeled states/assumptions and linked to implementation tests. Do not substitute a passing documentation checker, zero tests, skipped target or mock console for a launch gate.

A launch report explicitly states standalone qualification and platform qualification separately. No deployment/publication is authorized by preparing this design.
