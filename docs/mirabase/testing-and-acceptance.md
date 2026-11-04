# Testing and full-scope acceptance

**Current release authority:** [current implementation inventory](../current-product.md), [user documentation](../user/index.md) and [launch status](../launch/status.md). Initial v0.1/roadmap delivery labels below preserve planning contracts; they do not establish today's implementation or permit deleting subsequently integrated features. [Managed cloud](../cloud/README.md) has separate design and gates.

**Delivery: v0.1 and roadmap.** [v0.1-build.md](v0.1-build.md) selects the current portion; [roadsmap.md](roadsmap.md) retains the remainder. Detailed schemas/configuration/ABI and full-family acceptance below describe the target superset unless narrowed by that allocation.

## Evidence model

[Requirements and acceptance](requirements-and-acceptance.md) defines all required families and target outcomes. Record actual evidence as implementation proceeds. The pinned PocketBase checkout is an unmodified source reference.

For every family record implementation commit, exact symbol/file ownership, test IDs/source, actual command/toolchain, OS/architecture, engine/native digests, fixture hashes, start/end time, exit status, discovered/executed/skipped counts, real state assertions, negative controls and retained artifacts. A passing test with zero cases, a skipped native target or an expected JSON document is not success.

## Qualification layers

1. Static/document checks: references, source pins/hashes, schema/config constraints, route inventory, ownership and requirement coverage. This qualifies documentation consistency only.
2. Unit tests for real parsers/converters/policies/state machines where isolated tests add value.
3. Provider compatibility: run the inherited suite on original and Turso providers; compare differences and prevent unintended API changes.
4. End-to-end backend/UI/SDK/native protocol tests against real data/files/identities.
5. Concurrency/fault/security tests exercising committed effects and recovery.
6. Reproducible packaging, footprint and operator restore/cutover on every release target.

Do not add tests that merely mirror a registry builder or assert a feature flag is true. Add tests that would fail if the real authorization, atomicity, representation or recovery claim were broken.

## Required fixture matrix

Tenant import must pass the eight dedicated import families and their source/target fixtures: indirect ownership, multi-membership, shared/reference data, ownership cycles/conflicts, real auth/rule/file preservation, missing S3 objects, cross-tenant denial, exact reconciliation, resume/idempotency and every cutover rename/marker crash. Every source row/file needs declared placement. Schema-only success cannot satisfy these families.

Use all inherited fields/auth methods/rules, base/auth/view collections, Go/JS hooks/migrations, local/S3 files, SQL views/indexes, new native types/arrays/vectors/search, materialized views, encrypted/codec contexts, WAL/MVCC, primary/private/user/agent contexts and both sync models. Test supported combinations, and also verify forbidden combinations fail before mutation.

Use two auth collections with identical record IDs; two users with overlapping private record IDs; two agents with different grants; superuser and expired/revoked credentials. Include NULL/missing/empty, Unicode, large integers, decimal boundaries, timezone cases, invalid JSON/arrays/vectors, hidden fields and protected auth metadata.

## Cut-point matrix

For create/delete/restore/migrate/key rotation/backup/sync/outbox: interrupt before intent, after intent, after physical effect, before metadata publication, after publication, before response and after response loss. Restart with the original request/idempotency key. Verify exact target contents, manifest/generation/state, sidecars, files, hooks/events, receipts and pending jobs.

Faults include process kill, disk full, read-only filesystems, malformed pages/frames/checksums, missing key/library, ABI mismatch, network partition/timeouts/duplication, cancellation/commit races, slow consumers, lease expiry, stale schema/policy/generation and invalid archives. Simulated failures need real target-state assertions; simulator success does not replace native crash tests.

## Release gates

Every selected v0.1 family/slice needs implementation and passing qualification with no unresolved promised-release blocker. Roadmap families are not failed v0.1 gates and are never marked qualified from v0.1 evidence. Full-product completion requires their eventual passing acceptance too. Every advertised target and frontend must have exact feature coverage; a limitation cannot be hidden behind an "optional" UI switch. Negative/security/fault cases are release gates, not a later hardening phase.

Baseline parity must include custom extension fixtures, migration/import, frontend SDKs, setup/dashboard/static frontend and updater. Compare actual old/new responses while allowing only documented additive differences.

Publish measured full-feature size/startup/RAM/load/retention/restore evidence and the accepted performance budget. Security claims need auth/TLS/session and filesystem-escape negatives across HTTP/PG/native/sync, not just endpoint login tests.

## Honest completion

Specified → implemented → tested → release-qualified requires new evidence at each step. Documentation validation cannot advance runtime status. Blocked items stay open with exact source evidence and a resolution path. If a required upstream API is missing, implement/adapt it and qualify it; do not replace it with a permanently unsupported response and mark the requirement done.

The V01 workstreams give the active release order. The full-product P01-P18 owners and roadmap preserve later obligations. Use the exact allocation for release gating; a runtime feature within v0.1 cannot be cut to make tests pass.



## Complete application and setup matrix

Run the acceptance assertions for all requirements, including independent schemas/accounts/settings/artifacts/files; app-versus-instance authority; append into populated instances; greenfield setup for every storage model; tenant invitations; reference refresh; ownership transfer; offline switching/expiry; live-plan retirement; disk-backed rollout controls and fair resource admission. Use at least three unrelated source applications with colliding IDs/emails and changed token domains. Test old/new generations and wrong-app credentials across every channel. Keep the default shared-app setup free of optional catalogs. Validate real destination state and untouched-app hashes after every failure.
