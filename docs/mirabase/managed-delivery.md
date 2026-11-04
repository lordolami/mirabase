# Commercial and managed delivery boundary

**Current release authority:** [current implementation inventory](../current-product.md), [user documentation](../user/index.md) and [launch status](../launch/status.md). Initial v0.1/roadmap delivery labels below preserve planning contracts; they do not establish today's implementation or permit deleting subsequently integrated features. [Managed cloud](../cloud/README.md) has separate design and gates.

**Delivery: roadmap.** This chapter specifies later behavior; v0.1 does not build this service. Shared safety rules still apply where used by released features.

MiraBase is a complete locally usable backend. A commercial deployment may operate the same runtime for a customer, or sell independently agreed services/extensions. This specification does not decide pricing, licensing or which new commercial product the company will sell.

The current runtime must not be reduced to force a hosting subscription. Local collections/auth/files/realtime/dashboard and the agreed Turso additions remain in scope. Nothing here authorizes a feature paywall, closed source relicensing or a mandatory account.

If managed delivery is selected, its application interfaces remain the same: frontend HTTP/SDK behavior, collection rules, data types, optional private contexts and export/backup contracts. Operator provisioning, metering, billing, account recovery, fleet placement, external observability and support belong outside the single backend process.

A managed instance is not automatically a distributed database. Sharding, cross-node consensus, automatic multi-node failover, distributed transactions and millions-of-databases placement are excluded from this standalone design. They cannot be claimed from Turso's embedded engine, multi-process WAL or optional native sync.


Self-hosted operation needs no company-hosted telemetry, license check, control plane, MCP gateway or external data service. Optional third-party auth/mail/storage/sync retain their configured network dependencies. Exports and backups remain usable without the company's availability.
