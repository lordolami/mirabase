# Executed evidence and production qualification

Run date: 2026-11-04. **The development preview has build/runtime evidence; production release qualification remains open.** This is a production-assurance distinction, not a statement of hackathon eligibility. Public source is available at [lordolami/mirabase](https://github.com/lordolami/mirabase). No qualified production binary release or deployed hosting service is claimed. Remaining implementation and cross-platform obligations retain their actual status.

## Current executed evidence

| Gate | Actual result |
|---|---|
| Windows amd64 full native invocation | 5,236 test/subtest passes. Four archive assertions failed because Windows denied symbolic-link creation. All other tested packages passed; no tests were skipped. |
| Approved Windows archive invocation | Those same four archive assertions pass under the previously approved administrator access. No Windows security settings changed. This remains a separate invocation, not a claimed clean full run. |
| Backup regression | Three executions each create and independently reopen eight real archives during concurrent writes. All retain the two-row committed-write invariant. The full Windows backup/restore API package run also passes. |
| Provisioning regression | Real HTTP database creation applies the published template before readiness; the first record is immediately created and read without test-side schema application. Concurrent provisioning and lifecycle assertions pass. |
| Import regression | Mixed-case auth-table imports pass; deleting an actual staged row fails validation. Existing adoption acceptance also passes. The real dashboard imports a fresh backup authored by the pinned PocketBase executable through staging and successful validation. |
| Native payload trust | Latest Windows extraction/load, pin rejection and cache-tamper assertions pass with the patched payload. The previous Linux loader run passed its older payload. |
| PostgreSQL external client | node-postgres 8.23.1 executes simple and prepared queries against the current server with a scoped SQL credential, verified certificate and TLS 1.3. Other clients and full native-frontend compatibility remain separate. |
| Browser demonstration | Real admin screens, a labelled connected example frontend, actual subscriptions, files, search, record sync, historical workspace restoration and SDK requests were exercised. The recording is not full-family qualification. |
| First-application tutorial | Exact documented shared-storage schemas/rules were provisioned through HTTP. The actual tutorial client under Node passed member login/create/list/logout/re-login, cross-account read denial, forged-owner/owner-transfer refusal, required/title-length validation and persistence after process stop/restart. Frontend HTML/modules/dashboard were served. This was not automated browser click-through or managed-CRUD parity; see [scoped evidence](../../examples/quickstart/verification.json). Disposable data was removed. |
| Linux amd64 baseline | Previous full run: 5,238 test/subtest passes, zero failures/skips; 42 tested packages plus 14 without tests. It uses the earlier Linux source/payload and does not qualify the new Windows patch or subsequent source changes. |
| Other platforms | macOS amd64/arm64, Linux arm64 and Windows arm64 payloads/executions remain missing. Earlier negative builds correctly refused missing payloads. |

Tests and subtests are counted explicitly. The merged historical index contains additional observations from focused runs; it is not a current full-run test count. Exact source symbols and raw-log digests are in the [30-area feature matrix](../../release/feature-qualification.json) and [qualification report](../../release/qualification.json).

## Corrections and their scope

The Windows native engine now refreshes its source schema inside the `VACUUM INTO` read snapshot. This prevents stale-cookie helper statements from reparsing and restarting an export after its output was created. The old binary deterministically failed the concurrent-backup regression; the updated payload passes all three executions. The owned [patch manifest](../../native/patches/manifest.json) pins the upstream revision, each patch and the complete applied diff. The Cargo build rejects unlisted/staged/untracked engine changes and embeds the patch-manifest digest. The upstream audit checkouts remain untouched.

Database provisioning loads and applies its actual published schema before marking the target ready. A bounded set of shared locks protects same-binding provisioning. Template index errors are no longer silently ignored. The new [HTTP regression](../../apis/mirabase_provision_template_test.go) deliberately performs no out-of-band template application.

Import validation used case-sensitive `sqlite_master` comparisons even though table identifiers are case-insensitive and the engine normalizes stored names. A real `_authOrigins` table exposed this. Bound case-insensitive inventory comparisons now find the copied rows, and metadata/count query errors fail validation. The [mixed-case regression](../../apis/mirabase_import_case_test.go) includes a real missing-row negative. The dashboard file picker also keeps its selected filename readable. These checks do not prove every source version, auth collection, rule, file store or tenant transform.

Previously verified fixes remain: UTC S3 metadata, historical workspace bytes with hash preflight, owned backup scratch directories, real-clock expiry fixtures, bounded asynchronous cleanup waits, SDK transport/path protection and completed DBX predicate quoting. Their original failures and corrected executions remain indexed.

## Release blockers

**Managed record policy parity:** the managed CRUD handlers in [mirabase_records.go](../../apis/mirabase_records.go) authorize database ownership/grants and invoke the raw record store. Complete inherited per-operation collection rules, forms/validation, hooks and hidden-field behavior are not applied on that path. The normal inherited record API is separate. Green owner/grant tests do not close this implementation gap.

**Remaining implementation families:** full upstream AgentFS files/KV/inode/mount/overlay integration, complete SDK/offline delivery, ordinary-client bootstrap, upstream native-sync interoperability, independent-application runtime isolation, four unregistered application/import operations and hard native memory admission remain open. The native memory-budget ABI explicitly reports unsupported. Names, routes and configuration do not fulfill these obligations.

**Platform and candidate consistency:** only amd64 Windows and Linux payloads exist. Linux must be rebuilt and retested with the owned patch and current Go source. Mac/ARM execution, clean release machines and supported distribution baselines remain unqualified. The older Linux payload references GLIBC_2.38 and ran under WSL2 with glibc 2.43; this is not Musl/Alpine or older-glibc acceptance.

**Release and fault gates:** full SBOM/signing/update trust, full reproducibility, the specified startup percentiles and idle-memory/load measurements, 32-context and large cold-database workloads, native allocation limits, and complete disk/inode/crash/recovery acceptance remain required. Passing size budgets is not memory or performance qualification. Multi-file workspace restoration is not yet crash-atomic.

Earlier watcher/OAuth full-run failures pass in the latest complete Windows package execution; their failed evidence is retained. An earlier artifact connection reset has not been reproduced in the current tour. Its failed run lacked an exit code, so no native-crash cause is claimed; long-running stability remains a separate gate.

## Artifacts and video

The current Windows executable and native digest pins are recorded in its build manifest and the qualification report. The standalone Windows download, checksums and scoped package checks have their own [distribution record](../../release/distribution.json) and [download guide](../user/downloads.md). The executable remains unsigned; publishing an artifact does not change the production-assurance verdict above. The Linux archive is an older baseline, explicitly distinguished above.

The three-minute demo contains real application footage, local narration, burned-in captions, a separate subtitle file, chapter metadata and an explicitly planned roadmap. The coverage manifest describes each scene. A short feature overview does not certify every operation, platform, security boundary or recovery path.

[Current inventory](../current-product.md) · [Requirement ledger](../../release/implementation-ledger.json) · [Launch procedure](README.md) · [Open-source platform roadmap](../roadmap/platform/README.md)

Private raw logs and recording material are retained in the workspace `_build` directory. There is no MiraBase cloud or managed hosting product. Future distribution/operations remain open-source roadmap work.
