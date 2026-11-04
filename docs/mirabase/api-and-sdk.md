# HTTP, protocols and SDKs

**Current release authority:** [current implementation inventory](../current-product.md), [user documentation](../user/index.md) and [launch status](../launch/status.md). Initial v0.1/roadmap delivery labels below preserve planning contracts; they do not establish today's implementation or permit deleting subsequently integrated features. [Managed cloud](../cloud/README.md) has separate design and gates.

**Delivery: v0.1 and roadmap.** [v0.1-build.md](v0.1-build.md) selects the current portion; [roadsmap.md](roadsmap.md) retains the remainder. Detailed schemas/configuration/ABI and full-family acceptance below describe the target superset unless narrowed by that allocation.

## Inherited API

The complete method/path inventory is http-operation-reference.md, extracted from the pinned route-registration source. Its handlers and middleware remain the normative baseline together with the original source. Preserve error envelopes, auth flows, filters, multipart semantics, SDK auto-cancellation, pagination and realtime behavior. New context selection is additive.

Do not rename /api/collections, /api/files, /api/realtime or the neutral method names. The SQL console stays superuser-only. Collection import is PUT /api/collections/import, not an invented POST route. Backup restore acknowledgment does not mean restore has finished.

## Additive API conventions

Base path: /api/mirabase/v1. JSON is UTF-8. Reject unknown mutation keys, duplicate keys, invalid enum values and over-limit bodies before effects. Dates use UTC RFC3339 with declared precision. Large integers use decimal strings. IDs are opaque. Lists return items, page, perPage, totalItems and totalPages unless they are cursor streams. New mutations use Idempotency-Key and expected generation/revision where applicable.

Errors preserve the inherited top-level {status,message,data}. Add structured details under data.mirabase: code, retryable, requestId, operationId, databaseId when authorized, committed, fieldErrors and recovery when applicable. committed is true, false or null for an unknown outcome. Stable codes include invalid_request, unauthenticated, forbidden, not_found, generation_mismatch, schema_mismatch, policy_changed, capability_unavailable, incompatible_profile, conflict, idempotency_conflict, cursor_expired, resync_required, resource_exhausted, maintenance, storage_failure and outcome_unknown. Use HTTP 400/401/403/404/409/410/413/422/429/503/500 as appropriate; do not use HTTP 200 for an unsuccessful mutation.

Do not expose database existence to unauthorized actors. A 404 can intentionally cover both absent and invisible resources. Internal diagnostics retain the exact cause. Request IDs cross HTTP/PG/jobs/native adapters without embedding sensitive values.

## New operation families

Full PocketBase adoption has its own /api/mirabase/v1/pocketbase-imports session/plan/preview/stage/validate/cutover/recovery API. It moves existing data/files and maps tenant ownership/access using the same service as the dashboard, CLI and SDK. The inherited PUT /api/collections/import remains schema-only. Exact additive schemas/guards and the three import choices are in [PocketBase import into tenant databases](pocketbase-tenant-import.md) and pocketbase-tenant-import.md.

The machine operation catalog defines each method/path, authorization, request fields, response schema, preconditions, effects, idempotency, errors and acceptance. Families include capability discovery; database lifecycle/grants/agent bindings; schema plans/templates/rollouts; typed search; change pull/stream; record sync; live-query registration; native/PG credential issuance; diagnostics/maintenance; job inspect/cancel; and export/import/backup manifests.

Inherited operations are not reimplemented in a parallel platform API. New administrative endpoints call the same context-aware services used by the dashboard, SDKs and Go/JS helpers. No anonymous create-database endpoint is added.

## Remote client packages

Ship MiraBase-branded JavaScript/TypeScript and Dart clients with inherited method parity and additive typed services. Preserve authStore, collection(record methods), files, realtime, settings/admin methods and request options supported by the pinned SDK behavior. Context is immutable per request or scoped client instance; simultaneous calls cannot race through a shared currentDatabase property.

Add services: capabilities, databases, grants, agents, schemaPlans, rollouts, search, changes, jobs, sync and diagnostics. Errors expose structured codes without string parsing. API preview produces correct per-context examples and typed values.

Other languages can use normal HTTP/OpenAPI and qualified existing PocketBase-compatible remote clients for inherited APIs. Native Turso bindings are not full MiraBase SDKs: they expose an engine, not auth/files/collections/dashboard. Official Go server framework remains available. Do not invent equal SDK coverage for every language.

## Offline and native clients

The optional record-sync module owns local schema, queue, reconciliation and files. Native whole-DB clients use the qualified upstream-compatible protocol and privileged credentials. Browser WASM, mobile native, desktop and server runtimes each have explicit feature matrices; no runtime secretly downloads executable code from an untrusted source.

Serverless SQL/HTTP protocol v3 (/v3/pipeline and /v3/cursor, typed params, conditional batches, baton transactions, streaming and stored SQL) is an upstream transport capability requiring a MiraBase authenticated adapter before advertisement. It shares the privileged SQL boundary. Raw upstream client presence is not service interoperability.

## Versioning

The [operation reference](http-operation-reference.md) defines the additive API and lists the pinned inherited routes; generate OpenAPI 3.1 from those definitions during implementation. Inherited handler and middleware behavior remains authoritative. Release tooling emits combined OpenAPI, streaming/frame schemas, native protocol fixtures and capability manifest, then validates real clients. Breaking behavior requires a new version or negotiated capability. Package identities and source/archive delivery are selected in integration-contracts.md; public registry publication is optional and never claimed before it happens.



## Application paths and setup contracts

The default root APIs remain. Other apps use /a/<applicationId> as an immutable SDK base prefix. The application context is resolved before every auth/data path. Instance operations are operator-only. Exact additional operations and schemas are maintained in http-operation-reference.md and wire-schema-reference.md. See [application paths and setup contracts](new-application-setup.md).
