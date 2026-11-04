# Future open-source distribution and operations

**Roadmap only. There is no MiraBase cloud or managed hosting offering.** The current release is the standalone application backend. These proposals concern future open-source components for operators who need capabilities beyond one host; they do not require today's users to deploy a cluster.

## Capabilities to develop

- Database placement and bounded routing caches.
- Replication, fenced leadership/failover and explicit consistency modes.
- Balancing/draining with verified snapshot catch-up and cutover.
- Cold-database storage, bounded warm caches and controlled wake-up.
- Read replicas and larger-primary placement for hot workloads.
- Backup/PITR/disaster recovery, retention and restore drills.
- Project/application operator console, metrics, quotas and resource accounting.
- Secret/key isolation, signed upgrades and verified import/export/server moves.

These are implementation and acceptance obligations, not current services. Millions of cold databases remain a capacity objective requiring real workloads; empty database count does not establish active concurrency. The roadmap does not promise transparent sharding of one hot DB or cross-DB transactions.

## Design references

[Architecture](distributed-architecture.md) · [Operator journey](customer-journey.md) · [Capacity/operations](operations-and-economics.md) · [Acceptance](acceptance.md) · [Project roadmap](../../../ROADMAP.md)

Standalone continues to run without a company account, license server, telemetry service, subscription, provider console or external control plane. Users may run their own applications on infrastructure they choose; that does not create a MiraBase-hosted service.
