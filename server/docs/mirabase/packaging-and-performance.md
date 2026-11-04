# Packaging, footprint and performance

**Current release authority:** [current implementation inventory](../current-product.md), [user documentation](../user/index.md) and [launch status](../launch/status.md). Initial v0.1/roadmap delivery labels below preserve planning contracts; they do not establish today's implementation or permit deleting subsequently integrated features. [Open-source platform roadmap](../roadmap/platform/README.md) has separate design and gates.

**Delivery: v0.1 and roadmap.** [v0.1-build.md](v0.1-build.md) selects the current portion; [roadsmap.md](roadsmap.md) retains the remainder. Detailed schemas/configuration/ABI and full-family acceptance below describe the target superset unless narrowed by that allocation.

## Deliverables

Ship a MiraBase executable with the dashboard assets and all promised runtime adapters qualified for the target. Ship the Go framework source, JS/TypeScript and Dart remote clients, optional offline/native adapters, OpenAPI/type artifacts, migration/adoption tools, release manifest, notices and deployment examples. Native Turso packages in other languages remain their own engine distributions.

A single executable download may securely extract an embedded native library. Runtime data, native cache, WALs and uploaded files still occupy disk. Explain this honestly. A tar/zip containing many services is not equivalent to the promised compact backend.

## Targets and toolchain

Required qualification targets are Linux amd64/arm64, Windows amd64/arm64 and macOS amd64/arm64, matching the inherited delivery expectation. Each must actually build and run with its native assets and CPU baseline. If the upstream adapter is missing on a target, it is a blocker, not a silently removed target.

Use Go 1.27 for the v0.41.1 base and record its exact patch, Rust compiler, linker, UI Node/package manager and lockfiles in every build manifest. The selected engine is b0e1b8fde8eff66ef9843fdd1139fa3305cb16dd plus digest-pinned owned patches; release qualification tests that build rather than selecting a moving dependency. No toolchain download/build is claimed before execution.

## Measured baseline, not a MiraBase estimate

Observed PocketBase v0.41.1 packages on 2026-10-29:

| Target | Compressed bytes | Executable bytes |
|---|---:|---:|
| Windows amd64 | 13012018 | 34065920 |
| Linux amd64 | 12826122 | 33562784 |

These are upstream download/ZIP-entry observations. They are not MiraBase builds, resident-memory measurements or a promised MiraBase size. Native Turso, PostgreSQL, FTS and sync payloads can increase footprint. Do not repeat the upstream size as if all additions were free.

The initial engineering acceptance budgets are selected now in packaging-and-performance.md. These are targets, not measured results or a claim that the user supplied the numbers.

| Full-feature artifact/workload | Required target |
|---|---:|
| Compressed executable download | <= 40 MiB |
| Executable including embedded assets/native payload | <= 96 MiB |
| Extracted native payload | <= 64 MiB |
| Executable plus extracted payload, excluding app data | <= 160 MiB |
| Idle resident memory, empty app, listeners/features enabled without active clients | <= 96 MiB |
| Warm installed time-to-ready, p95 of 50 fresh starts | <= 2 seconds |
| First-run extraction plus readiness, p95 of 50 runs | <= 5 seconds |
| Peak resident memory, defined 32-context CRUD workload | <= 512 MiB |

The baseline machine has 2 dedicated vCPU, 4 GiB RAM and local SSD; exclude unrelated processes and record OS/storage/CPU details. CRUD workload: 32 active optional contexts, 10000 1-KiB records each, 32 concurrent clients, 80 percent indexed reads/20 percent single-record writes for ten minutes. Record latency/throughput without inventing a passed target. All six target platforms report native memory metrics with OS-specific accounting. No library stripping, lazy initialization or packaging optimization may remove promised functionality. A budget miss requires optimization and a failed acceptance record, not a feature cut or an undocumented budget increase.

## Performance suite

Measure cold start/time-to-ready, first CRUD/query/auth, idle RSS/PSS/native heap, executable/extracted footprint, per-open/idle DB cost, handle eviction/reopen, write contention, FTS/vector cost, sync bootstrap/push/pull/checkpoint, schema fan-out, backup/restore and shutdown. Report p50/p95/p99, throughput, errors and resource use with exact hardware/data/build/cache/durability settings.

Test 1, 100, 10000 and a generated large cold-database population separately from active concurrency. An empty registry benchmark is not evidence of millions of real DB files. Include file descriptors/inodes, directory scan costs, quota admission and fair scheduling. Never extrapolate a capacity guarantee from one hot small benchmark.

## Integrity and supply chain

Release manifests contain product/native/SDK versions, hashes, build flags, ABI, minimum OS/CPU, signed checksums, SBOM, licenses, capability matrix and known blockers. Verify extraction permissions/search paths and updater signatures. An update requires a coherent backup and compatible data migration; rollback cannot open a newer incompatible format just because the executable was replaced.

Selected distribution identities are module mirabase, @mirabase/sdk package archive and mirabase Dart source/path package. Release artifacts and checksums can be distributed without registry publication. Generate a protected release signing key during release provisioning and distribute its public key with the installed binary; test keys never sign real releases. Do not claim public namespace ownership, publish packages or invent a download domain. These are delivery actions, not open implementation architecture.

## Acceptance

A fresh machine with no Rust compiler or Go installation must run the ready executable and exercise CRUD/UI/auth/files/FTS/PG/sync per its matrix. Test offline installation, missing cache, cache tampering, antivirus/file-lock interactions, Unicode/space paths and no writable install directory. The total delivered footprint and all skipped/unqualified targets must be visible in the report.



## Full application and client qualification

Measure independent applications as well as DB files, including idle/cold artifact caches, active auth/hooks/files, fair queues and application restore. Build/test every advertised native/WASM client asset on its selected target; no source-only wrapper counts as delivery. See [full application and client qualification](offline-client-delivery.md).


## Exact engineering budgets

```json
{
  "formatVersion": 1,
  "requirements": [
    "MB-PERF",
    "MB-PACKAGE"
  ],
  "reference": {
    "dedicatedVcpu": 2,
    "ramMiB": 4096,
    "storage": "local SSD",
    "startupRuns": 50,
    "reportedPercentile": 95
  },
  "artifactBytes": {
    "compressedDownloadMax": 41943040,
    "executableMax": 100663296,
    "extractedNativeMax": 67108864,
    "installedPayloadMax": 167772160
  },
  "runtime": {
    "idleResidentBytesMax": 100663296,
    "warmReadyP95MsMax": 2000,
    "firstExtractionAndReadyP95MsMax": 5000,
    "crudWorkloadPeakResidentBytesMax": 536870912
  },
  "crudWorkload": {
    "activeContexts": 32,
    "recordsPerContext": 10000,
    "recordBytes": 1024,
    "concurrentClients": 32,
    "indexedReadPercent": 80,
    "writePercent": 20,
    "durationSeconds": 600
  },
  "rules": [
    "All native/UI/adapter features selected for the measured release included; v0.1 uses its allocation and the full product adds all roadmap groups",
    "Idle features/listeners enabled with no active clients on empty app",
    "Publish measured results per target and OS-specific memory accounting",
    "No feature cuts or silent budget increases"
  ],
  "applicationWorkload": {
    "activeIndependentApplications": 8,
    "coldApplicationPopulation": 1000,
    "activeManagedDatabases": 32,
    "fixture": "different schemas, auth, SMTP/S3, hooks and files; include one overloaded app and unchanged peer workloads",
    "durationSeconds": 600,
    "qualification": "Report actual memory/latency/errors and fair admission; all full-feature footprint limits still apply"
  }
}
```
