# MiraBase open-source roadmap

**Current release: the standalone backend. No MiraBase cloud or managed hosting service is planned.** One process provides the existing backend, dashboard, Turso integration and documented additions. Indie developers deploy that process and build their application frontend. Zero-infra multi-tenancy means no new process/container/service per database; the host and storage still need to be operated.

The current distribution is defined in [release status](release/qualification.json), with actual test evidence and [known endpoint limitations](docs/current-product.md). Integrated code is retained. The original full-design audit is preserved separately; moving an integration here does not claim it is implemented, tested or safe in combinations not checked.

## Backend and access integration

Complete inherited rules/forms/hooks/hidden-field parity for the additive managed-record path. Keep its current owner/grant boundary explicit until then. Complete remaining core-field/dashboard/SDK adapter combinations and unsupported schema operations. Improve ordinary-agent workspace permissions without distributing operator tokens.

## Offline and synchronization

Complete ordinary-client bootstrap, local browser/mobile/native stores, native protocol interoperability, cursor/generation recovery and full SDK integration. Record HTTP synchronization and upstream whole-database replication remain different interfaces.

## Full AgentFS

Integrate the upstream SDK/provider, files/KV/inode/chunks, overlays, sessions/diffs/apply, mounts and qualified backup/sync. Current workspace files and historical snapshots remain available with their documented operator and atomicity limits. See [full design](docs/mirabase/agentfs-roadmap.md).

## Independent applications and administration

Complete independent auth/settings/hooks/files/background-runtime isolation and remaining application operations. Application registry entries and import batches do not by themselves implement that isolation. Improve migration/adoption coverage while preserving source data and explicit ownership mapping.

## Platforms, packaging and assurance

Add Mac/ARM targets, rebuild/qualify the Linux payload with current patches, improve signing/SBOM/reproducibility/update trust, native memory admission, startup/load measurements and broader disk/inode/crash/recovery coverage. Current checksums and exact executed evidence remain available; unsigned does not mean signed. Multi-file crash-atomic workspace restoration remains additional work.

## Wasm packaging investigation

Evaluate wazero with actual durable filesystem support, Turso feature parity, resource/concurrency/cancellation behavior and benchmarks. Preserve existing database/API contracts and prove any format migration. The current native provider remains in use.

## Future open-source distribution and fleet operations

Retain placement, replication, routing, fenced failover, balancing, cold storage/wake-up, read replicas, backup/PITR/DR, app/secret isolation, an operator console, metrics, quotas, capacity/usage accounting and verified migration/exit as future open-source capabilities. See [platform design](docs/roadmap/platform/README.md) and [acceptance](docs/roadmap/platform/acceptance.md). These are not current standalone capabilities and do not introduce a vendor-operated hosting product.

One hot database's transparent write sharding and cross-database ACID need separate engine designs. No MCP, Mongo frontend or other-BaaS adapter is silently added.
