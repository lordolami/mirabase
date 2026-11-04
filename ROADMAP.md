# MiraBase delivery roadmap

Current implementation goes beyond the initial v0.1 scope. [Source inventory](docs/current-product.md) and [actual routes](docs/user/api-reference.md) identify integrated code; [launch status](docs/launch/status.md) identifies verification. Existing contracts remain obligations until implemented and qualified, not permission to remove features.

## Launch completion

Resolve all failing acceptance, field/protocol/SDK/UI parity and security/recovery gaps. Qualify every promised engine profile, target and client. Complete native memory-budget enforcement, missing application operations, isolated application runtime coverage, ordinary-client offline bootstrap, upstream native protocol adapters and full AgentFS SDK/files/KV/mount/overlay integration where promised. Prepare signed packages, provenance, data-format upgrades and tested recovery before release.

## Packaging and portability

Wazero is a later engine-provider investigation. It can embed a Wasm engine inside the Go executable, but needs Go-hosted durable storage, supported Wasm features, all current Turso functionality, lifetime/concurrency/cancellation and real resource benchmarks. The browser package is not automatically a drop-in Go module. Preserve existing DB/API/SDK contracts and qualify any file migration. Native packaging continues until a replacement proves parity.

## Managed distributed platform

Build the same customer backend experience with our placement, quorum journal/replication, routing, fenced failover, balancing, cold population, backups/PITR/DR, app/secret isolation, project console, metrics, quotas, usage/billing and migration/exit path. See [complete cloud design](docs/cloud/README.md) and [acceptance](docs/cloud/acceptance.md). It is not implemented or deployed by current standalone route registration.

Other-BaaS adapters, Mongo/NoSQL frontends and an MCP server are not silently introduced. Cross-DB distributed SQL transactions and transparent sharding inside a hot DB require a separate approved engine design.
