# Future open-source platform roadmap

**Status: future open-source design; not an implemented distributed runtime or launched managed service.** The current product is the self-hosted MiraBase backend for indie developers and AI-agent applications. This folder retains earlier managed-deployment designs as roadmap work for the open-source platform. Existing paths are retained to preserve references. No public service, tariff or SLA is established.

## Future operator and developer experience

A developer starts with one self-hosted MiraBase process. A future platform implementation should retain the application/private-database model while adding the infrastructure described here. A managed deployment is a possible operating model for that future software, not a requirement to use today's backend or a currently offered MiraBase service. Frontend collection/auth/file/realtime/SDK compatibility remains a migration requirement; server URL/custom domain, credentials and origins change explicitly where necessary.

Future open-source work includes provisioning, placement, persistent storage, replicas, routing, balancing, recovery, backups, monitoring, usage and quotas. A private DB per user/tenant/agent remains optional. Millions of cold DBs remain a capacity goal to qualify. Metering/billing designs are future operator capabilities, not a selected charge or commercial dependency for self-hosters.

## Complete platform feature inventory

| Surface | Required behavior |
|---|---|
| Projects and applications | Account/project lifecycle, regions, app identity, auth/settings/secrets/artifacts, environments, scoped operators and service credentials |
| Database lifecycle | Idempotent create, inspect, suspend/wake, clone, backup, restore/new generation, export/import, quarantine, delete/trash/purge |
| Placement | Cell/region/storage class/affinity policies, weighted admission, reservations, explicit placement revision and migration |
| Replication | Three-zone quorum durability for acknowledged regional commits, validated replicas, snapshots/log tailing and schema/format fences |
| Routing | Stable endpoint/custom domains, cached epoch routes, primary/session/explicit eventual reads, stale-route rejection |
| Availability | Fenced leadership, health detection, promotion only from committed durable history, bounded recovery and availability metrics |
| Balancing | Hotspot detection, draining, snapshot catch-up and atomic cutover; no duplicate active writer or discarded accepted commit |
| Cold population | Persistent registry plus object snapshots/logs, bounded warm handles/cache, controlled wake queue and visible cold-start behavior |
| Scaling | Stateless API/router scaling, worker/cell capacity growth, primary compute upgrades and read replicas; no implicit cross-DB ACID |
| Customer console | Apps/DBs, schema/records, grants, credentials, metrics, jobs, replicas, backups, migrations, usage, billing and status/history |
| Storage/security | TLS/mTLS, customer isolation, envelope encryption/rotation, key separation, private networks, credential epochs and secret audit |
| Recovery | PITR from qualified native snapshots/committed log, retention floors, disaster recovery, corruption quarantine and restore drills |
| Agent product | Same ordinary API identities/rules plus qualified workspace features; no mandatory MCP gateway or agent execution service |
| Delivery/operations | Signed artifacts, canaries, format compatibility, observability, incident runbooks, support and published tested limits |

The [distributed architecture](distributed-architecture.md) retains state/transaction/recovery proposals. [Operator journey](customer-journey.md) covers future signup/migration/dashboard/API. [Operations/economics](operations-and-economics.md) covers resource accounting, optional operator charging and runbooks. [Acceptance](acceptance.md) defines future delivery gates. All are governed by the [open-source roadmap](../../ROADMAP.md).

## Product boundaries

Standalone remains one process on one host with no required cloud account or distributed cluster. Future distribution requires additional platform services/adapters and real acceptance. Customer application databases are not individual consensus nodes.

Horizontal placement of many independent DBs differs from splitting one hot database's writes across nodes. The selected platform scales one hot DB through larger primary capacity and qualified read replicas; transparent intra-database SQL sharding and cross-database distributed transactions are not claimed. They would require a separate engine design and qualification, not a routing switch.

Runtime code already contains application metadata and record sync. Those alone do not establish multi-node isolation, native redo replication, cloud metering or failover. No managed production claim may be emitted until [cloud gates](acceptance.md) pass.
