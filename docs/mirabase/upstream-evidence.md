# Upstream evidence and reproducibility

**Current release authority:** [current implementation inventory](../current-product.md), [user documentation](../user/index.md) and [launch status](../launch/status.md). Initial v0.1/roadmap delivery labels below preserve planning contracts; they do not establish today's implementation or permit deleting subsequently integrated features. [Managed cloud](../cloud/README.md) has separate design and gates.

**Delivery: source baseline.** Apply the [v0.1 allocation](v0.1-build.md) to current features; [roadmap](roadsmap.md) owns later integrations. Source facts alone do not qualify runtime behavior.

## Pins

PocketBase v0.41.1: f70b24e186df98df2e7ebc703f103cb94dc3ca63. The local pocketbase checkout is the source reference. Its go.mod requires Go 1.27. The official project moved to the author's source host; the archived GitHub repository is not the current release authority.

Turso selected engine input: audited development snapshot b0e1b8fde8eff66ef9843fdd1139fa3305cb16dd, observed 2026-10-29, plus versioned owned bridge patches. Selecting this input does not make it a stable upstream release or establish runtime qualification.

[Official PocketBase documentation](https://pocketbase.io/docs/) and [current source project](https://ganigeorgiev.com/pocketbase/pocketbase) identify the foundation. [Pinned Turso source](https://github.com/tursodatabase/turso/tree/b0e1b8fde8eff66ef9843fdd1139fa3305cb16dd) owns the native observations.

## Source identities and API ownership

[Pinned source identities](source-reference.md) give the exact upstream input bytes. [Inherited HTTP routes](http-operation-reference.md#inherited-http-routes) identify the owning handlers and middleware. Verify those sources before relying on a behavior, and collect real build/runtime evidence for the implementation. A source component's existence does not establish service interoperability or a qualified combination.

## Source ownership

| Observation | Owning evidence |
|---|---|
| Provider injection and serialized write pool | pocketbase.go; core/base.go; core/db_connect.go; core/db_tx.go |
| Field/query/schema extension seams | core/field.go; core/collection_model.go; core/db_table.go; tools/search |
| Administrative raw SQL boundary | apis/sql.go and its superuser middleware |
| Existing collection/auth/files/SSE/settings/backups | apis registration/handlers; core backup source |
| Existing JS/dashboard extension model | plugins/jsvm/jsvm.go; core/events.go; ui/src/router.js |
| Go native driver and sync API | bindings/go/README.md, bindings*.go, driver*.go |
| PG parser/frontend/session/wire limits | postgres/COMPAT.md, parser/translator.rs, frontend/*.rs, server/lib.rs |
| Native sync routes and messages | cli/sync_server.rs; sync/engine/src/client_proto.rs and server_proto.rs |
| Sync reconciliation/lazy storage | database_sync_engine.rs, database_sync_operations.rs, database_sync_lazy_storage.rs |
| CDC and binary capture | cli/manuals/cdc.md; core/cdc.rs |
| Encryption and custom types | cli/manuals/encryption.md/custom-types.md; core/storage/encryption.rs |
| FTS/ANN distinction | core/index_method/fts/mod.rs; toy_vector_sparse_ivf.rs |
| Materialized views | cli/manuals/materialized-views.md plus mode checks in engine |
| MVCC log/group commit | core/mvcc/database/group_commit.rs; persistent_storage/logical_log.rs |
| Extension/client tooling | extensions/core/README.md; serverless/go/README.md; root README |

Source references prove those components exist and explain boundaries. They do not establish that every claim passes, every combination works or MiraBase implements them.

## Supplementary current documentation

[Turso documentation index](https://docs.turso.tech/llms.txt), [SQL compatibility](https://docs.turso.tech/sql-reference/compatibility), [experimental features](https://docs.turso.tech/sql-reference/experimental-features), [concurrent writes](https://docs.turso.tech/tursodb/concurrent-writes), [native CDC](https://docs.turso.tech/tursodb/cdc), [encryption](https://docs.turso.tech/tursodb/encryption), [sync usage](https://docs.turso.tech/sync/usage), [partial sync](https://docs.turso.tech/sync/partial), [conflict resolution](https://docs.turso.tech/sync/conflict-resolution), [checkpoint](https://docs.turso.tech/sync/checkpoint), [local development sync server](https://docs.turso.tech/sync/local-sync-server), [SDK overview](https://docs.turso.tech/sdk/introduction).

Read current docs as supplementary evidence. When docs/source disagree, record the discrepancy and test it. Do not settle it by choosing the more attractive claim. Pinned source paths/hashes keep the design reproducible even when upstream changes.

## Qualification boundary

The pinned sources define inputs and explain upstream behavior. Build and execute the complete implementation and acceptance matrix before claiming working native packaging, protocol conformance, migration, UI, performance or recovery.
