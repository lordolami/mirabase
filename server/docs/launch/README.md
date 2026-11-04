# Launch and release procedure

The owned source boundary is the `server` folder. The current [feature inventory](../current-product.md) supersedes the initial v0.1 planning allocation. Preserve all integrated code; qualify its actual behavior. [Status](status.md) is the executed evidence report, not this checklist.

## Build and native trust

From this folder, with Go 1.27:

```sh
go run ./tools/release --version dev
go run ./tools/release --check
cd internal/mbengine/platform
go test ./...
```

The builder selects the exact embedded payload, hashes it and compiles both trusted SHA-256 pins. It uses `no_default_driver`; no live default SQLite fallback is shipped. It emits an executable and manifest under `dist/`. Its manifest intentionally starts `releaseQualified:false`; compilation does not attest tests, signing or deployment.

Engine source rebuild: `pwsh -File tools/build-native.ps1` fetches the exact pinned engine into ignored `native/engine` and uses the owned bridge/toolchain/lock. It never resets an existing changed checkout. This script's existence is not evidence that a payload was built reproducibly; execute and retain source/compiler/linker/feature/patch/SBOM and ABI evidence. Install only a qualified library into the correct payload directory, then rebuild the executable pins.

Only Windows amd64 and Linux amd64 payload files currently exist. Both have actual execution evidence in [status](status.md); Linux runs use Ubuntu under WSL. Other target builds must fail explicitly. Preserve all promised target obligations in the matrix; don't call an unavailable payload a successful cross-platform release.

## Clients and dashboard

```sh
cd sdk/js
npm ci
npm test
npm pack
cd ../../ui
npm ci
npm run build
```

Rebuild the backend after UI assets change. For Dart run `dart pub get`, `dart analyze` and real native runtime tests; the current dart:io transport does not prove browser support. Archive the Go framework and SDK source/package with licenses and exact method/platform coverage. No registry publication is implied by `npm pack`.

## Runtime acceptance

Run a newly built artifact on a clean machine with no Rust/Go installation and a new persistent root. Verify native extraction/pins, setup/superuser, health, dashboard, auth/rules, private contexts, local/S3/protected files, every search mode, changes/live, sync, references, PG/TLS/SCRAM, jobs/rollouts, workspace snapshots, imports, backup/restore and server move. Include wrong actor/target/generation, policy revocation and hidden fields on each access path.

Exercise conflicts, cancellation/unknown commits, interrupted publication, disk/inode pressure, missing/tampered payload, wrong keys, malformed archives, native crash and restart. Real rows/bytes/hashes and receipts are the assertions; skipped/zero tests and feature flags are not success. Compare the untouched inherited reference where a failure is suspected environmental, without silently dismissing it.

Measure total executable plus extracted native bytes, startup, idle/native memory, cold DB population and active concurrency, query/write latencies, FTS/vector cost, migration/backup recovery. Publish exact workload/toolchain/hardware/cache/durability and failure counts. No 40 MiB/96 MiB target becomes a measured claim automatically.

`python tools/qualify-artifact.py <absolute-executable> --output <report.json>` runs disposable-data CRUD, anonymous denial and historical workspace-byte restoration against a real child server. It stops that child and removes only its own temporary root. A single startup/RSS observation is not the specified idle/load or 50-start percentile acceptance.

Keep Go's native JSONL output for every package, including failures and reruns. `tools/qualification-report.py --windows <logs...> --linux <logs...>` indexes selected source symbols against those actual outcomes; it retains original failures even when later reruns pass. On Windows, archive symlink tests require the OS privilege: run that package with approved administrator access rather than skipping it. Package entries with no test files are reported separately from test skips.

## Reviewable launch artifacts

- Per-target executable and SHA-256 manifest, real qualification report, release notes and known limits.
- UI asset digest, SDK archive/types, public user/agent/API documentation and third-party license/SBOM notices.
- Data-format/engine upgrade compatibility and verified recovery instructions, including Windows stop/restore/start.
- Signature/checksum verification and owned update-source configuration; generate signing material in a protected release environment, never commit private keys.
- Package allowlist excludes customer data, `.zcode`, local caches, test roots, keys, `.env`, scratch binaries and private evidence.

Create a local review package only. Public repositories, releases, package registries, DNS, deployment and customer messages require explicit authorization. Never publish an artifact while the required gate report is failed, blocked or missing.

## Cloud is a separate launch

The [managed cloud gates](../roadmap/platform/acceptance.md) add native commit/replay, quorum durability, fencing, failover, placement, capacity/economics, migration, isolation and billing. Passing standalone checks does not launch that platform.
