# MiraBase managed cloud

**Status: selected target design; not an implemented or launched service.** This documentation defines what the managed product must build and prove. It does not grant the standalone binary distributed capabilities or establish a public endpoint, tariff or SLA.

## Customer product

A developer can start with self-hosted MiraBase, then move the same backend/application and private-database model onto our managed platform. Frontend collection/auth/file/realtime/SDK contracts remain; the server URL or custom domain, credentials and allowed origins are updated through a reviewed migration. Stored IDs, rules, identity and data retain their meaning. A change of endpoint is not an application rewrite.

The managed offer operates database/backend infrastructure: provisioning, placement, persistent storage, replicas, routing, balancing, recovery, backups, monitoring, usage and quotas. A private DB per user, tenant or agent remains optional. Millions of cold DBs are a capacity goal to qualify, not a claim about current tests. We charge for resources/service, not automatically a process or fixed fee for every DB.

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

The [distributed architecture](distributed-architecture.md) fixes state/transaction/recovery decisions. [Customer journey](customer-journey.md) covers signup, migration and dashboard/API. [Operations/economics](operations-and-economics.md) covers resource accounting, pricing units and runbooks. [Acceptance](acceptance.md) defines launch gates.

## Product boundaries

Standalone remains one process on one host with no required cloud account. Managed distribution is our infrastructure and additional adapters, not a requirement that self-hosters assemble a distributed cluster. Customer application databases are not nodes in a consensus group.

Horizontal placement of many independent DBs differs from splitting one hot database's writes across nodes. The selected platform scales one hot DB through larger primary capacity and qualified read replicas; transparent intra-database SQL sharding and cross-database distributed transactions are not claimed. They would require a separate engine design and qualification, not a routing switch.

Runtime code already contains application metadata and record sync. Those alone do not establish multi-node isolation, native redo replication, cloud metering or failover. No managed production claim may be emitted until [cloud gates](acceptance.md) pass.
