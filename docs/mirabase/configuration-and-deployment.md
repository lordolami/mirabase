# Configuration and deployment

**Current release authority:** [current implementation inventory](../current-product.md), [user documentation](../user/index.md) and [launch status](../launch/status.md). Initial v0.1/roadmap delivery labels below preserve planning contracts; they do not establish today's implementation or permit deleting subsequently integrated features. [Managed cloud](../cloud/README.md) has separate design and gates.

**Delivery: v0.1 and roadmap.** [v0.1-build.md](v0.1-build.md) selects the current portion; [roadsmap.md](roadsmap.md) retains the remainder. Detailed schemas/configuration/ABI and full-family acceptance below describe the target superset unless narrowed by that allocation.

## First run

The release target is: download the MiraBase executable, run mirabase serve, create the first superuser through the inherited setup flow, open /_/, configure collections/rules, then connect the frontend to the HTTP API. Implement and qualify this workflow against the complete application/setup contracts.

Default paths are mb_* and default HTTP bind is inherited loopback :8090. No cloud account, organization, database-count subscription or configuration file is required. Existing flags for --dir, encryption environment key, query timeout, HTTP/HTTPS/domain handling and JS/migration plugins remain with documented MiraBase identities.

## Configuration model

New config is optional JSON: mirabase.json selected by --config. The [configuration reference](configuration-reference.md) defines its exact schema and complete default expansion. Precedence: built-in defaults < config file < explicit MIRABASE_* settings < explicit CLI flags. Validate the fully merged configuration against the complete schema; partial file overrides are allowed but unknown keys are rejected during merge. New environment names use the uppercase JSON path separated by double underscores, for example MIRABASE_DATABASES__MAXOPEN. Inherited database settings retain their existing settings/API precedence; avoid configuring the same option independently in two stores.

engine.nativeCacheDir="auto" resolves to an owner-private OS user-cache directory under mirabase, using the service user's profile. An explicit alternative must be an absolute path with equivalent verified permissions. Do not literally create a directory named auto or search the working directory. Failure to obtain a private extraction directory stops startup before app data opens.

Unknown keys, malformed values, conflicting settings and insecure optional listener combinations fail startup with field paths and no secret values. Diagnostics expose effective nonsecret config and its source. Secrets use reference fields resolved from environment/private files, never a returned plaintext effective config.

Database profiles and page codec/encryption changes require maintenance jobs, not hot reload. Log level/rate/admission budgets may reload atomically with validation. Reload failure preserves the last valid config.

## Deployment choices

Run directly as a service, under a process supervisor, or in a container on any qualified host with persistent local storage. Fly.io is one example, not a required provider. Ephemeral root filesystems need a persistent mounted data directory. One active application writer owns the app lock; automatic restarts must not create overlapping instances.

A reverse proxy may terminate TLS and route to loopback/private HTTP. Configure public URL, trusted proxy CIDRs, request/upload limits and timeouts. The dashboard/setup endpoint needs controlled access. Keep persistent storage and backups separate; remote object storage does not replace database disk.

Do not use a network filesystem or shared active data directory unless the native VFS/locking/recovery combination is explicitly qualified. Multi-process engine access is not an app clustering guarantee.

## Resource defaults

The administrative PocketBase importer is enabled by default, with lazy optional metadata. imports settings bound source archives to 20 GiB by default, upload chunks to 8 MiB, active sessions to two and copy batches to 1000 records/4 MiB. Recovery retention defaults to seven days after a safe terminal state. Operators can raise resource limits explicitly. Tenant layout selection prepares the required database/partition configuration in the staged app; a schema-only import never enables it silently.

Defaults are in the schema/example: optional handles 64, concurrent opens 8, optional reads 4, MVCC writers 4, global SQL connections 256, idle close 30 seconds, new search deadline 5 seconds, background workers 2, schema rollout parallelism 2, new sync group 50 mutations/5 MiB, change retention 7 days/1 GiB, receipt retention 7 days and trash retention 24 hours. Operator budgets can be raised with evidence; no vendor database-count quota is imposed.

Reserve primary/auxiliary capacity and cap aggregate page/native/JS memory. Report backpressure explicitly. Disk/inode free-space thresholds reject dangerous work before staging unbounded uploads/backups. Health is liveness; readiness checks recoverability and required services. A nonready server must not receive public writes.

## Growth and transfer

More CPU/RAM/faster SSD and a larger server can increase capacity. Measure hot queries, contention, disk/WAL, cache and file workloads before changing hardware. A million stored cold databases and a million simultaneously busy databases are different resource demands. No unlimited-capacity statement follows from having no license quota.

Vertical transfer follows backups-and-recovery.md. Preserve secrets, hooks/migrations/public assets, remote object credentials and public URL behavior. Use maintenance/cutover to prevent split writers. Rollback restores the prior coherent state; it cannot magically merge writes accepted independently on both hosts.

## Operations requirements

Document firewall/service user/permissions, log rotation, restart policy, trusted clock, backup destination, certificate renewal and disk alerts in release examples. These are operator tasks, not a new hosting control plane. Runtime works offline after installation except explicitly configured remote auth/mail/storage/sync services.



## Application settings and admission

Default setup is shared and simple. Each application owns settings/secrets; instance config owns listeners/payload/proxies/aggregate budgets. Optional application metadata is lazy. Connection pools use actual aggregate admission with 16 reserved recovery/auth slots, not a full permanent pool per application. See [application settings and admission](implementation-closure.md).
