# Engine, field and compatibility reference

**Current release authority:** [current implementation inventory](../current-product.md), [user documentation](../user/index.md) and [launch status](../launch/status.md). Initial v0.1/roadmap delivery labels below preserve planning contracts; they do not establish today's implementation or permit deleting subsequently integrated features. [Managed cloud](../cloud/README.md) has separate design and gates.

**Delivery: v0.1 and roadmap.** [v0.1-build.md](v0.1-build.md) selects the current portion; [roadsmap.md](roadsmap.md) retains the remainder. Detailed schemas/configuration/ABI and full-family acceptance below describe the target superset unless narrowed by that allocation.

Pinned upstream capabilities are distinct from MiraBase integration requirements. Each promised integration needs real acceptance on its advertised platform/profile/frontend. Source IDs refer to [the source reference](source-reference.md). Constraints must be enforced before effects; a rejected unpromised combination does not qualify an absent promised feature.

## Capability inventory

### CAP-embedded


```json
{
  "id": "CAP-embedded",
  "title": "embedded native engine",
  "requirement": "MB-PROVIDER",
  "upstreamStatus": "implemented upstream",
  "evidence": [
    "TURSO:README.md"
  ],
  "frontends": [
    "sqlite"
  ],
  "profiles": [
    "wal",
    "mvcc"
  ],
  "constraint": "Go adapter and inherited API qualification"
}
```


### CAP-sqlite-compat


```json
{
  "id": "CAP-sqlite-compat",
  "title": "SQLite SQL/file/C API compatibility target",
  "requirement": "MB-PROVIDER",
  "upstreamStatus": "partial upstream",
  "evidence": [
    "TURSO:README.md"
  ],
  "frontends": [
    "sqlite"
  ],
  "profiles": [
    "wal",
    "mvcc"
  ],
  "constraint": "Actual compatibility suite; not 100 percent assumed"
}
```


### CAP-relational


```json
{
  "id": "CAP-relational",
  "title": "joins/subqueries/CTEs/windows/upsert/RETURNING",
  "requirement": "MB-SQL",
  "upstreamStatus": "implemented subset",
  "evidence": [
    "TURSO:postgres/COMPAT.md"
  ],
  "frontends": [
    "sqlite",
    "postgres"
  ],
  "profiles": [
    "wal",
    "mvcc"
  ],
  "constraint": "Per-frontend fixtures"
}
```


### CAP-lateral


```json
{
  "id": "CAP-lateral",
  "title": "LATERAL/default in VALUES/null ordering/ordered-set",
  "requirement": "MB-SQL",
  "upstreamStatus": "implemented subset",
  "evidence": [
    "TURSO:postgres/COMPAT.md"
  ],
  "frontends": [
    "sqlite",
    "postgres"
  ],
  "profiles": [
    "wal",
    "mvcc"
  ],
  "constraint": "Dialect/schema parser qualification"
}
```


### CAP-mvcc


```json
{
  "id": "CAP-mvcc",
  "title": "snapshot isolation/BEGIN CONCURRENT",
  "requirement": "MB-MVCC",
  "upstreamStatus": "experimental upstream",
  "evidence": [
    "TURSO:core/connection.rs"
  ],
  "frontends": [
    "sqlite",
    "postgres"
  ],
  "profiles": [
    "mvcc"
  ],
  "constraint": "DBX connector plus concurrent application pool"
}
```


### CAP-async


```json
{
  "id": "CAP-async",
  "title": "async I/O/io_uring/IOCP/custom VFS",
  "requirement": "MB-ASYNC",
  "upstreamStatus": "platform dependent",
  "evidence": [
    "TURSO:README.md"
  ],
  "frontends": [
    "native"
  ],
  "profiles": [
    "wal",
    "mvcc"
  ],
  "constraint": "OS/driver callback and cancellation coverage"
}
```


### CAP-simd


```json
{
  "id": "CAP-simd",
  "title": "SIMD paths",
  "requirement": "MB-VECTOR",
  "upstreamStatus": "platform dependent",
  "evidence": [
    "TURSO:README.md"
  ],
  "frontends": [
    "native"
  ],
  "profiles": [
    "wal",
    "mvcc"
  ],
  "constraint": "CPU dispatch/fallback"
}
```


### CAP-wal


```json
{
  "id": "CAP-wal",
  "title": "WAL/savepoints/checkpoint/recovery",
  "requirement": "MB-RECOVERY",
  "upstreamStatus": "implemented upstream",
  "evidence": [
    "TURSO:core/connection.rs"
  ],
  "frontends": [
    "sqlite",
    "postgres"
  ],
  "profiles": [
    "wal"
  ],
  "constraint": "Crash and power-loss guarantees separate"
}
```


### CAP-group-commit


```json
{
  "id": "CAP-group-commit",
  "title": "MVCC group commit/GC/checkpoint thresholds",
  "requirement": "MB-RECOVERY",
  "upstreamStatus": "experimental upstream",
  "evidence": [
    "TURSO:core/mvcc/database/group_commit.rs"
  ],
  "frontends": [
    "native"
  ],
  "profiles": [
    "mvcc"
  ],
  "constraint": "Durability/load measurement"
}
```


### CAP-multiprocess


```json
{
  "id": "CAP-multiprocess",
  "title": "multi-process shared-WAL coordination",
  "requirement": "MB-RESOURCES",
  "upstreamStatus": "experimental upstream",
  "evidence": [
    "TURSO:README.md"
  ],
  "frontends": [
    "native"
  ],
  "profiles": [
    "wal"
  ],
  "constraint": "Not supported multi-active-app topology"
}
```


### CAP-schema


```json
{
  "id": "CAP-schema",
  "title": "triggers/virtual generated columns/schema mutation",
  "requirement": "MB-COLLECTIONS",
  "upstreamStatus": "implemented subset",
  "evidence": [
    "TURSO:core/connection.rs"
  ],
  "frontends": [
    "sqlite",
    "postgres"
  ],
  "profiles": [
    "wal",
    "mvcc"
  ],
  "constraint": "STORED generated columns unavailable"
}
```


### CAP-attach


```json
{
  "id": "CAP-attach",
  "title": "ATTACH/WITHOUT ROWID/VACUUM/auto-vacuum",
  "requirement": "MB-SQL",
  "upstreamStatus": "mode dependent",
  "evidence": [
    "TURSO:core/connection.rs"
  ],
  "frontends": [
    "sqlite",
    "postgres"
  ],
  "profiles": [
    "wal",
    "mvcc"
  ],
  "constraint": "Privileged paths; exact profile matrix"
}
```


### CAP-explain


```json
{
  "id": "CAP-explain",
  "title": "EXPLAIN/bytecode/PRAGMAs/integrity tools",
  "requirement": "MB-OPERATE",
  "upstreamStatus": "implemented subset",
  "evidence": [
    "TURSO:core/connection.rs"
  ],
  "frontends": [
    "sqlite"
  ],
  "profiles": [
    "wal",
    "mvcc"
  ],
  "constraint": "Read-only diagnostics and path policy"
}
```


### CAP-arrays


```json
{
  "id": "CAP-arrays",
  "title": "typed multidimensional arrays/functions",
  "requirement": "MB-ARRAY",
  "upstreamStatus": "experimental upstream",
  "evidence": [
    "TURSO:cli/manuals/custom-types.md"
  ],
  "frontends": [
    "sqlite",
    "postgres"
  ],
  "profiles": [
    "wal",
    "mvcc"
  ],
  "constraint": "STRICT schema and wire adapters"
}
```


### CAP-custom-types


```json
{
  "id": "CAP-custom-types",
  "title": "BASE/ENCODE/DECODE/COERCE/operators/parameters",
  "requirement": "MB-TYPES",
  "upstreamStatus": "experimental upstream",
  "evidence": [
    "TURSO:cli/manuals/custom-types.md"
  ],
  "frontends": [
    "sqlite",
    "postgres"
  ],
  "profiles": [
    "wal",
    "mvcc"
  ],
  "constraint": "Versioned types and total codecs"
}
```


### CAP-domains


```json
{
  "id": "CAP-domains",
  "title": "domains/defaults/constraints",
  "requirement": "MB-TYPES",
  "upstreamStatus": "experimental upstream",
  "evidence": [
    "TURSO:postgres/COMPAT.md"
  ],
  "frontends": [
    "sqlite",
    "postgres"
  ],
  "profiles": [
    "wal",
    "mvcc"
  ],
  "constraint": "Dependency and validation integration"
}
```


### CAP-composites


```json
{
  "id": "CAP-composites",
  "title": "STRUCT/UNION/field/tag access",
  "requirement": "MB-TYPES",
  "upstreamStatus": "experimental upstream",
  "evidence": [
    "TURSO:cli/manuals/custom-types.md"
  ],
  "frontends": [
    "sqlite",
    "postgres"
  ],
  "profiles": [
    "wal",
    "mvcc"
  ],
  "constraint": "Composite index restrictions"
}
```


### CAP-type-aliases


```json
{
  "id": "CAP-type-aliases",
  "title": "bool/integer/varchar/time/numeric/uuid/network/bytea/JSON types",
  "requirement": "MB-TYPES",
  "upstreamStatus": "experimental upstream",
  "evidence": [
    "TURSO:cli/manuals/custom-types.md"
  ],
  "frontends": [
    "sqlite",
    "postgres"
  ],
  "profiles": [
    "wal",
    "mvcc"
  ],
  "constraint": "Application validation; timezone/network limitations"
}
```


### CAP-sequences


```json
{
  "id": "CAP-sequences",
  "title": "named sequences/bounds/cycle/session operations",
  "requirement": "MB-DEFAULTS",
  "upstreamStatus": "implemented subset",
  "evidence": [
    "TURSO:postgres/COMPAT.md"
  ],
  "frontends": [
    "sqlite",
    "postgres"
  ],
  "profiles": [
    "wal",
    "mvcc"
  ],
  "constraint": "Session/rollback semantics not gapless"
}
```


### CAP-uuid


```json
{
  "id": "CAP-uuid",
  "title": "UUID4/UUID7/timestamp helpers",
  "requirement": "MB-DEFAULTS",
  "upstreamStatus": "implemented upstream",
  "evidence": [
    "TURSO:extensions/core/README.md"
  ],
  "frontends": [
    "sqlite",
    "postgres"
  ],
  "profiles": [
    "wal",
    "mvcc"
  ],
  "constraint": "Default evaluation and SDK values"
}
```


### CAP-vectors


```json
{
  "id": "CAP-vectors",
  "title": "float32/64/sparse/8-bit/1-bit and exact distances",
  "requirement": "MB-VECTOR",
  "upstreamStatus": "implemented subset",
  "evidence": [
    "TURSO:cli/manuals/vector.md"
  ],
  "frontends": [
    "sqlite",
    "postgres"
  ],
  "profiles": [
    "wal",
    "mvcc"
  ],
  "constraint": "Metric/type/dimension authorization integration"
}
```


### CAP-ann-prototype


```json
{
  "id": "CAP-ann-prototype",
  "title": "toy sparse IVF approximate index",
  "requirement": null,
  "upstreamStatus": "prototype only",
  "evidence": [
    "TURSO:core/index_method/toy_vector_sparse_ivf.rs"
  ],
  "frontends": [
    "native"
  ],
  "profiles": [
    "wal",
    "mvcc"
  ],
  "constraint": "Not an existing mature ANN promise"
}
```


### CAP-fts


```json
{
  "id": "CAP-fts",
  "title": "Tantivy tokenizers/BM25/JSON/highlight/maintenance",
  "requirement": "MB-FTS",
  "upstreamStatus": "build gated",
  "evidence": [
    "TURSO:core/index_method/fts/mod.rs"
  ],
  "frontends": [
    "sqlite"
  ],
  "profiles": [
    "wal",
    "mvcc"
  ],
  "constraint": "Native-only; commit visibility; no snippet claim"
}
```


### CAP-fuzzy


```json
{
  "id": "CAP-fuzzy",
  "title": "edit/phonetic/transliteration/regexp functions",
  "requirement": "MB-FUZZY",
  "upstreamStatus": "build dependent",
  "evidence": [
    "TURSO:extensions/core/README.md"
  ],
  "frontends": [
    "sqlite"
  ],
  "profiles": [
    "wal",
    "mvcc"
  ],
  "constraint": "Typed bounded filter integration"
}
```


### CAP-extra-functions


```json
{
  "id": "CAP-extra-functions",
  "title": "crypto/encoding/nanosecond time/network/CSV/series/stats",
  "requirement": "MB-FUZZY",
  "upstreamStatus": "build dependent",
  "evidence": [
    "TURSO:extensions/core/README.md"
  ],
  "frontends": [
    "sqlite"
  ],
  "profiles": [
    "wal",
    "mvcc"
  ],
  "constraint": "Per-function availability and safety"
}
```


### CAP-materialized


```json
{
  "id": "CAP-materialized",
  "title": "DBSP incremental materialized views",
  "requirement": "MB-VIEWS",
  "upstreamStatus": "experimental upstream",
  "evidence": [
    "TURSO:cli/manuals/materialized-views.md"
  ],
  "frontends": [
    "sqlite",
    "postgres"
  ],
  "profiles": [
    "wal"
  ],
  "constraint": "No current MVCC or attached DB combination"
}
```


### CAP-live-query


```json
{
  "id": "CAP-live-query",
  "title": "query-result subscriptions",
  "requirement": "MB-LIVE",
  "upstreamStatus": "advertised; public Go adapter not established",
  "evidence": [
    "TURSO:README.md"
  ],
  "frontends": [
    "http",
    "native"
  ],
  "profiles": [
    "wal",
    "mvcc"
  ],
  "constraint": "Selected Go SQL/CDC service supplies coherent snapshots, keyed ordered result diffs, policy and bounded replay; no public native subscribe dependency"
}
```


### CAP-cdc


```json
{
  "id": "CAP-cdc",
  "title": "native row/DDL capture/transaction boundaries",
  "requirement": "MB-CDC",
  "upstreamStatus": "experimental upstream",
  "evidence": [
    "TURSO:cli/manuals/cdc.md"
  ],
  "frontends": [
    "native"
  ],
  "profiles": [
    "wal",
    "mvcc"
  ],
  "constraint": "Every connection/path; native binary decoder"
}
```


### CAP-encryption


```json
{
  "id": "CAP-encryption",
  "title": "AES-GCM/AEGIS page encryption",
  "requirement": "MB-ENCRYPT",
  "upstreamStatus": "experimental upstream",
  "evidence": [
    "TURSO:cli/manuals/encryption.md"
  ],
  "frontends": [
    "native"
  ],
  "profiles": [
    "wal",
    "mvcc"
  ],
  "constraint": "Algorithm/mode/target matrix and key management"
}
```


### CAP-codec


```json
{
  "id": "CAP-codec",
  "title": "custom database/WAL page transformation",
  "requirement": "MB-CODEC",
  "upstreamStatus": "experimental upstream",
  "evidence": [
    "TURSO:core/connection.rs"
  ],
  "frontends": [
    "native"
  ],
  "profiles": [
    "wal"
  ],
  "constraint": "No assumed MVCC/ATTACH/multiprocess/partial sync"
}
```


### CAP-native-sync


```json
{
  "id": "CAP-native-sync",
  "title": "offline logical push/frame pull/rollback-replay",
  "requirement": "MB-NSYNC",
  "upstreamStatus": "implemented client; development server",
  "evidence": [
    "TURSO:cli/sync_server.rs"
  ],
  "frontends": [
    "native"
  ],
  "profiles": [
    "wal",
    "mvcc"
  ],
  "constraint": "Production gateway plus native interoperability"
}
```


### CAP-partial-sync


```json
{
  "id": "CAP-partial-sync",
  "title": "prefix/query/lazy pages/segments/prefetch",
  "requirement": "MB-PARTIAL",
  "upstreamStatus": "experimental upstream",
  "evidence": [
    "TURSO:sync/engine/src/database_sync_lazy_storage.rs"
  ],
  "frontends": [
    "native"
  ],
  "profiles": [
    "wal",
    "mvcc"
  ],
  "constraint": "Whole DB privilege, uncached offline unavailable"
}
```


### CAP-sync-checkpoint


```json
{
  "id": "CAP-sync-checkpoint",
  "title": "sync/revert WAL checkpoint and statistics",
  "requirement": "MB-SYNCCHECK",
  "upstreamStatus": "implemented subset",
  "evidence": [
    "TURSO:bindings/go/bindings_sync.go"
  ],
  "frontends": [
    "native"
  ],
  "profiles": [
    "wal",
    "mvcc"
  ],
  "constraint": "Do not discard pending/replay state"
}
```


### CAP-postgres


```json
{
  "id": "CAP-postgres",
  "title": "PG grammar/frontend/session/wire/CLI/catalogs/COPY",
  "requirement": "MB-PG",
  "upstreamStatus": "experimental compatibility",
  "evidence": [
    "TURSO:postgres/COMPAT.md"
  ],
  "frontends": [
    "postgres"
  ],
  "profiles": [
    "wal",
    "mvcc"
  ],
  "constraint": "Native trust server needs auth/TLS/session isolation"
}
```


### CAP-go


```json
{
  "id": "CAP-go",
  "title": "database/sql purego native Go bindings",
  "requirement": "MB-PROVIDER",
  "upstreamStatus": "implemented upstream",
  "evidence": [
    "TURSO:bindings/go/README.md"
  ],
  "frontends": [
    "native"
  ],
  "profiles": [
    "wal",
    "mvcc"
  ],
  "constraint": "No CGO still needs native payload"
}
```


### CAP-other-bindings


```json
{
  "id": "CAP-other-bindings",
  "title": "Rust/JS/Python/C/Java/.NET/Dart/React Native/WASM",
  "requirement": "MB-LOCALSDK",
  "upstreamStatus": "coverage varies",
  "evidence": [
    "TURSO:README.md"
  ],
  "frontends": [
    "native"
  ],
  "profiles": [
    "wal",
    "mvcc"
  ],
  "constraint": "Not equivalent backend SDKs or uniform features"
}
```


### CAP-sql-http


```json
{
  "id": "CAP-sql-http",
  "title": "serverless v3 pipeline/cursor/baton client tooling",
  "requirement": "MB-SQLHTTP",
  "upstreamStatus": "implemented upstream client",
  "evidence": [
    "TURSO:serverless/go/README.md"
  ],
  "frontends": [
    "http"
  ],
  "profiles": [
    "wal",
    "mvcc"
  ],
  "constraint": "Authenticated MiraBase adapter required"
}
```


### CAP-extensions


```json
{
  "id": "CAP-extensions",
  "title": "scalar/aggregate/vtable/VFS/custom index methods",
  "requirement": "MB-EXTENSIONS",
  "upstreamStatus": "native ABI",
  "evidence": [
    "TURSO:extensions/core/README.md"
  ],
  "frontends": [
    "native"
  ],
  "profiles": [
    "wal",
    "mvcc"
  ],
  "constraint": "Not ordinary SQLite extension binary compatibility"
}
```


### CAP-cli


```json
{
  "id": "CAP-cli",
  "title": "dump/import/clone/parameters/output/introspection/manual",
  "requirement": "MB-OPERATE",
  "upstreamStatus": "implemented upstream",
  "evidence": [
    "TURSO:README.md"
  ],
  "frontends": [
    "sqlite",
    "postgres"
  ],
  "profiles": [
    "wal",
    "mvcc"
  ],
  "constraint": "Integrate through privileged MiraBase tools"
}
```


### CAP-testing


```json
{
  "id": "CAP-testing",
  "title": "simulator/differential/fuzz/stress tooling",
  "requirement": "MB-EVIDENCE",
  "upstreamStatus": "upstream engineering tooling",
  "evidence": [
    "TURSO:README.md"
  ],
  "frontends": [
    "native"
  ],
  "profiles": [
    "wal",
    "mvcc"
  ],
  "constraint": "Not evidence of MiraBase correctness or whole-engine formal proof"
}
```


### CAP-mcp


```json
{
  "id": "CAP-mcp",
  "title": "upstream MCP tools",
  "requirement": null,
  "upstreamStatus": "excluded from MiraBase by user",
  "evidence": [
    "TURSO:README.md"
  ],
  "frontends": [
    "native"
  ],
  "profiles": [
    "wal",
    "mvcc"
  ],
  "constraint": "Normal authenticated APIs serve agents"
}
```


### CAP-open-license


```json
{
  "id": "CAP-open-license",
  "title": "upstream MIT source",
  "requirement": "MB-PACKAGE",
  "upstreamStatus": "upstream licensing",
  "evidence": [
    "TURSO:README.md"
  ],
  "frontends": [
    "native"
  ],
  "profiles": [
    "wal",
    "mvcc"
  ],
  "constraint": "Preserve notices; commercial model not decided"
}
```


## Profile and protocol compatibility


```json
{
  "formatVersion": 1,
  "baseline": {
    "pocketbase": "v0.41.1",
    "commit": "f70b24e186df98df2e7ebc703f103cb94dc3ca63"
  },
  "rules": [
    {
      "id": "MODE-01",
      "features": [
        "mvcc",
        "materialized"
      ],
      "result": "reject",
      "reason": "Native materialized view maintenance currently incompatible with MVCC"
    },
    {
      "id": "MODE-02",
      "features": [
        "attach",
        "materialized"
      ],
      "result": "reject",
      "reason": "Current materialized-view attached-database restriction"
    },
    {
      "id": "MODE-03",
      "features": [
        "codec",
        "mvcc"
      ],
      "result": "reject",
      "reason": "Current native page-codec limitation"
    },
    {
      "id": "MODE-04",
      "features": [
        "codec",
        "partial-sync"
      ],
      "result": "reject",
      "reason": "Page representation/lazy fetch integration unqualified"
    },
    {
      "id": "MODE-05",
      "features": [
        "codec",
        "multiprocess"
      ],
      "result": "reject",
      "reason": "Current native codec multi-process limitation"
    },
    {
      "id": "MODE-06",
      "features": [
        "codec",
        "attach"
      ],
      "result": "reject",
      "reason": "Current native codec attach limitation"
    },
    {
      "id": "MODE-07",
      "features": [
        "fts",
        "wasm"
      ],
      "result": "reject",
      "reason": "Current FTS native build requirement"
    },
    {
      "id": "MODE-08",
      "features": [
        "pg-wire",
        "trust-auth",
        "external-bind"
      ],
      "result": "reject",
      "reason": "No unauthenticated native wire exposure"
    },
    {
      "id": "MODE-09",
      "features": [
        "partial-sync",
        "row-rule-security"
      ],
      "result": "reject",
      "reason": "Pages may expose any database rows"
    },
    {
      "id": "MODE-10",
      "features": [
        "native-sql",
        "record-hooks"
      ],
      "result": "not-automatic",
      "reason": "Raw administrative writes do not traverse record forms/hooks"
    },
    {
      "id": "MODE-11",
      "features": [
        "cross-db",
        "record-transaction"
      ],
      "result": "reject",
      "reason": "No implicit cross-file ACID"
    },
    {
      "id": "MODE-12",
      "features": [
        "stored-generated-column"
      ],
      "result": "reject",
      "reason": "Native snapshot supports virtual generated columns, not STORED"
    },
    {
      "id": "MODE-13",
      "features": [
        "encryption",
        "native-sync"
      ],
      "result": "qualification-required",
      "reason": "Explicit cipher/profile/client matrix; do not assume or silently disable"
    },
    {
      "id": "MODE-14",
      "features": [
        "multiple-active-apps",
        "shared-data-dir"
      ],
      "result": "reject",
      "reason": "Engine multi-process support is not app scheduler/identity clustering"
    },
    {
      "id": "MODE-15",
      "features": [
        "sqlite",
        "recursive-cte"
      ],
      "result": "frontend-specific",
      "reason": "SQLite documentation and PG compatibility claims differ; execute both corpora"
    },
    {
      "id": "MODE-16",
      "features": [
        "fts",
        "read-uncommitted-write"
      ],
      "result": "not-promised",
      "reason": "Current native FTS is commit-visible; do not advertise read-your-writes before commit"
    }
  ],
  "targets": [
    "linux-amd64",
    "linux-arm64",
    "windows-amd64",
    "windows-arm64",
    "darwin-amd64",
    "darwin-arm64"
  ],
  "cipherCandidates": [
    "aes128gcm",
    "aes256gcm",
    "aegis256",
    "aegis128l",
    "aegis128x2",
    "aegis128x4",
    "aegis256x2",
    "aegis256x4"
  ],
  "qualificationStatus": "none-run"
}
```


## Field definitions

Common field settings: `id`, `name`, `required`, `hidden`, `system`, `unique`, `default`.


```json
{
  "owner": "pocketbase/core/field*.go at pinned revision",
  "types": [
    "text",
    "number",
    "bool",
    "email",
    "url",
    "editor",
    "password",
    "date",
    "autodate",
    "select",
    "json",
    "relation",
    "file",
    "geoPoint"
  ],
  "rule": "Preserve every configured option, modifier, validator, getter/setter and JSON representation; source implementations and parity tests are normative"
}
```


### Field vector


```json
{
  "id": "vector",
  "optionsSchema": {
    "type": "object",
    "properties": {
      "encoding": {
        "type": "string",
        "enum": [
          "float32",
          "float64",
          "sparse32",
          "int8",
          "bit1"
        ]
      },
      "dimensions": {
        "type": "integer",
        "minimum": 1,
        "maximum": 4096
      },
      "allowedMetrics": {
        "type": "array",
        "items": {
          "type": "string",
          "enum": [
            "cosine",
            "l2",
            "negative_dot",
            "jaccard"
          ]
        },
        "maxItems": 4
      }
    },
    "required": [
      "encoding",
      "dimensions",
      "allowedMetrics"
    ],
    "additionalProperties": false
  },
  "wire": "Finite JSON number array, or sparse dimensions/indices/values; native packed vector storage",
  "validation": "Dimensions fixed; sparse sorted unique; metric compatibility probed; reject undefined cosine",
  "owner": "collections-and-types.md",
  "requiredIntegrations": [
    "prepare",
    "validate-settings",
    "validate-value",
    "column-type",
    "driver-conversion",
    "filter",
    "sort",
    "index-compatibility",
    "default",
    "REST",
    "SDK",
    "dashboard",
    "hook",
    "CDC",
    "record-sync",
    "native-sync",
    "export",
    "backup"
  ]
}
```


### Field int64


```json
{
  "id": "int64",
  "optionsSchema": {
    "type": "object",
    "properties": {},
    "required": [],
    "additionalProperties": false
  },
  "wire": "Canonical signed decimal string; native 64-bit integer",
  "validation": "No leading plus/zeros except zero; reject negative zero, fractions/exponents and overflow",
  "owner": "collections-and-types.md",
  "requiredIntegrations": [
    "prepare",
    "validate-settings",
    "validate-value",
    "column-type",
    "driver-conversion",
    "filter",
    "sort",
    "index-compatibility",
    "default",
    "REST",
    "SDK",
    "dashboard",
    "hook",
    "CDC",
    "record-sync",
    "native-sync",
    "export",
    "backup"
  ]
}
```


### Field decimal


```json
{
  "id": "decimal",
  "optionsSchema": {
    "type": "object",
    "properties": {
      "precision": {
        "type": "integer",
        "minimum": 1,
        "maximum": 1000
      },
      "scale": {
        "type": "integer",
        "minimum": 0,
        "maximum": 1000
      },
      "rounding": {
        "type": "string",
        "enum": [
          "reject"
        ]
      }
    },
    "required": [
      "precision",
      "scale",
      "rounding"
    ],
    "additionalProperties": false
  },
  "wire": "Canonical decimal string; native numeric(P,S)",
  "validation": "scale <= precision; finite exact value; overflow or nonzero truncated digits rejected",
  "owner": "collections-and-types.md",
  "requiredIntegrations": [
    "prepare",
    "validate-settings",
    "validate-value",
    "column-type",
    "driver-conversion",
    "filter",
    "sort",
    "index-compatibility",
    "default",
    "REST",
    "SDK",
    "dashboard",
    "hook",
    "CDC",
    "record-sync",
    "native-sync",
    "export",
    "backup"
  ]
}
```


### Field uuid


```json
{
  "id": "uuid",
  "optionsSchema": {
    "type": "object",
    "properties": {
      "version": {
        "type": "string",
        "enum": [
          "any",
          "4",
          "7"
        ]
      }
    },
    "required": [
      "version"
    ],
    "additionalProperties": false
  },
  "wire": "Lowercase canonical hyphenated UUID string",
  "validation": "Validate variant/version; do not replace collection record IDs",
  "owner": "collections-and-types.md",
  "requiredIntegrations": [
    "prepare",
    "validate-settings",
    "validate-value",
    "column-type",
    "driver-conversion",
    "filter",
    "sort",
    "index-compatibility",
    "default",
    "REST",
    "SDK",
    "dashboard",
    "hook",
    "CDC",
    "record-sync",
    "native-sync",
    "export",
    "backup"
  ]
}
```


### Field array


```json
{
  "id": "array",
  "optionsSchema": {
    "type": "object",
    "properties": {
      "elementTypeId": {
        "type": "string"
      },
      "dimensions": {
        "type": "integer",
        "minimum": 1,
        "maximum": 4
      },
      "maxElements": {
        "type": "integer",
        "minimum": 1,
        "maximum": 10000
      },
      "elementNullable": {
        "type": "boolean"
      }
    },
    "required": [
      "elementTypeId",
      "dimensions",
      "maxElements",
      "elementNullable"
    ],
    "additionalProperties": false
  },
  "wire": "Nested homogeneous JSON arrays; native typed STRICT array",
  "validation": "Rectangular dimensions, total count, element codec/nullability; empty arrays retain declared type",
  "owner": "collections-and-types.md",
  "requiredIntegrations": [
    "prepare",
    "validate-settings",
    "validate-value",
    "column-type",
    "driver-conversion",
    "filter",
    "sort",
    "index-compatibility",
    "default",
    "REST",
    "SDK",
    "dashboard",
    "hook",
    "CDC",
    "record-sync",
    "native-sync",
    "export",
    "backup"
  ]
}
```


### Field binary


```json
{
  "id": "binary",
  "optionsSchema": {
    "type": "object",
    "properties": {
      "maxBytes": {
        "type": "integer",
        "minimum": 1,
        "maximum": 1048576
      }
    },
    "required": [
      "maxBytes"
    ],
    "additionalProperties": false
  },
  "wire": "Record: canonical padded Base64 string; SQL/CDC typed value uses blob envelope",
  "validation": "Reject malformed Base64 before allocation; limit decoded bytes",
  "owner": "collections-and-types.md",
  "requiredIntegrations": [
    "prepare",
    "validate-settings",
    "validate-value",
    "column-type",
    "driver-conversion",
    "filter",
    "sort",
    "index-compatibility",
    "default",
    "REST",
    "SDK",
    "dashboard",
    "hook",
    "CDC",
    "record-sync",
    "native-sync",
    "export",
    "backup"
  ]
}
```


### Field typed-date


```json
{
  "id": "typed-date",
  "optionsSchema": {
    "type": "object",
    "properties": {},
    "required": [],
    "additionalProperties": false
  },
  "wire": "YYYY-MM-DD; native date",
  "validation": "Validate Gregorian date and supported native range; no implicit timezone",
  "owner": "collections-and-types.md",
  "requiredIntegrations": [
    "prepare",
    "validate-settings",
    "validate-value",
    "column-type",
    "driver-conversion",
    "filter",
    "sort",
    "index-compatibility",
    "default",
    "REST",
    "SDK",
    "dashboard",
    "hook",
    "CDC",
    "record-sync",
    "native-sync",
    "export",
    "backup"
  ]
}
```


### Field typed-time


```json
{
  "id": "typed-time",
  "optionsSchema": {
    "type": "object",
    "properties": {
      "fractionDigits": {
        "type": "integer",
        "minimum": 0,
        "maximum": 9
      }
    },
    "required": [
      "fractionDigits"
    ],
    "additionalProperties": false
  },
  "wire": "HH:mm:ss[.fraction]; native time",
  "validation": "00..23 hours, no implicit date/offset, declared fractional precision",
  "owner": "collections-and-types.md",
  "requiredIntegrations": [
    "prepare",
    "validate-settings",
    "validate-value",
    "column-type",
    "driver-conversion",
    "filter",
    "sort",
    "index-compatibility",
    "default",
    "REST",
    "SDK",
    "dashboard",
    "hook",
    "CDC",
    "record-sync",
    "native-sync",
    "export",
    "backup"
  ]
}
```


### Field typed-timestamp


```json
{
  "id": "typed-timestamp",
  "optionsSchema": {
    "type": "object",
    "properties": {
      "fractionDigits": {
        "type": "integer",
        "minimum": 0,
        "maximum": 9
      },
      "kind": {
        "type": "string",
        "enum": [
          "local",
          "instant"
        ]
      }
    },
    "required": [
      "fractionDigits",
      "kind"
    ],
    "additionalProperties": false
  },
  "wire": "Local: YYYY-MM-DDTHH:mm:ss[.fraction]; instant: canonical UTC ...Z",
  "validation": "Normalize instant offsets to UTC; original offset not retained; reject silent precision loss",
  "owner": "collections-and-types.md",
  "requiredIntegrations": [
    "prepare",
    "validate-settings",
    "validate-value",
    "column-type",
    "driver-conversion",
    "filter",
    "sort",
    "index-compatibility",
    "default",
    "REST",
    "SDK",
    "dashboard",
    "hook",
    "CDC",
    "record-sync",
    "native-sync",
    "export",
    "backup"
  ]
}
```


### Field network-address


```json
{
  "id": "network-address",
  "optionsSchema": {
    "type": "object",
    "properties": {
      "kind": {
        "type": "string",
        "enum": [
          "inet",
          "cidr",
          "macaddr",
          "macaddr8"
        ]
      }
    },
    "required": [
      "kind"
    ],
    "additionalProperties": false
  },
  "wire": "Canonical IPv4/IPv6/prefix/MAC string",
  "validation": "Validate full grammar/ranges; canonical network bits for cidr; no DNS lookup",
  "owner": "collections-and-types.md",
  "requiredIntegrations": [
    "prepare",
    "validate-settings",
    "validate-value",
    "column-type",
    "driver-conversion",
    "filter",
    "sort",
    "index-compatibility",
    "default",
    "REST",
    "SDK",
    "dashboard",
    "hook",
    "CDC",
    "record-sync",
    "native-sync",
    "export",
    "backup"
  ]
}
```


### Field struct


```json
{
  "id": "struct",
  "optionsSchema": {
    "type": "object",
    "properties": {
      "typeId": {
        "type": "string"
      },
      "revision": {
        "type": "integer",
        "minimum": 1,
        "maximum": 9007199254740991
      }
    },
    "required": [
      "typeId",
      "revision"
    ],
    "additionalProperties": false
  },
  "wire": "JSON object of declared members; native named STRUCT",
  "validation": "Reject undeclared members; validate member required/default/type; no assumed composite index",
  "owner": "collections-and-types.md",
  "requiredIntegrations": [
    "prepare",
    "validate-settings",
    "validate-value",
    "column-type",
    "driver-conversion",
    "filter",
    "sort",
    "index-compatibility",
    "default",
    "REST",
    "SDK",
    "dashboard",
    "hook",
    "CDC",
    "record-sync",
    "native-sync",
    "export",
    "backup"
  ]
}
```


### Field union


```json
{
  "id": "union",
  "optionsSchema": {
    "type": "object",
    "properties": {
      "typeId": {
        "type": "string"
      },
      "revision": {
        "type": "integer",
        "minimum": 1,
        "maximum": 9007199254740991
      }
    },
    "required": [
      "typeId",
      "revision"
    ],
    "additionalProperties": false
  },
  "wire": "{tag,value}; native named UNION",
  "validation": "Case-sensitive declared tag, exactly one valid payload; no assumed composite index",
  "owner": "collections-and-types.md",
  "requiredIntegrations": [
    "prepare",
    "validate-settings",
    "validate-value",
    "column-type",
    "driver-conversion",
    "filter",
    "sort",
    "index-compatibility",
    "default",
    "REST",
    "SDK",
    "dashboard",
    "hook",
    "CDC",
    "record-sync",
    "native-sync",
    "export",
    "backup"
  ]
}
```


### Field domain


```json
{
  "id": "domain",
  "optionsSchema": {
    "type": "object",
    "properties": {
      "typeId": {
        "type": "string"
      },
      "revision": {
        "type": "integer",
        "minimum": 1,
        "maximum": 9007199254740991
      }
    },
    "required": [
      "typeId",
      "revision"
    ],
    "additionalProperties": false
  },
  "wire": "Public underlying value with domain validation",
  "validation": "Apply all base/chained constraints; dependencies prevent dropping a live domain",
  "owner": "collections-and-types.md",
  "requiredIntegrations": [
    "prepare",
    "validate-settings",
    "validate-value",
    "column-type",
    "driver-conversion",
    "filter",
    "sort",
    "index-compatibility",
    "default",
    "REST",
    "SDK",
    "dashboard",
    "hook",
    "CDC",
    "record-sync",
    "native-sync",
    "export",
    "backup"
  ]
}
```


### Field custom


```json
{
  "id": "custom",
  "optionsSchema": {
    "type": "object",
    "properties": {
      "typeId": {
        "type": "string"
      },
      "revision": {
        "type": "integer",
        "minimum": 1,
        "maximum": 9007199254740991
      }
    },
    "required": [
      "typeId",
      "revision"
    ],
    "additionalProperties": false
  },
  "wire": "Declared versioned public codec over native encoded base",
  "validation": "Server-owned ENCODE/DECODE/COERCE/default/operator definitions; deterministic round trip for valid values",
  "owner": "collections-and-types.md",
  "requiredIntegrations": [
    "prepare",
    "validate-settings",
    "validate-value",
    "column-type",
    "driver-conversion",
    "filter",
    "sort",
    "index-compatibility",
    "default",
    "REST",
    "SDK",
    "dashboard",
    "hook",
    "CDC",
    "record-sync",
    "native-sync",
    "export",
    "backup"
  ]
}
```


## Selected integration strategies

### Decision D-07


```json
{
  "id": "D-07",
  "owner": "integration-contracts.md",
  "requirements": [
    "MB-NSYNC",
    "MB-PARTIAL",
    "MB-SYNCCHECK"
  ],
  "strategy": "Go authenticated native-sync handler with exact upstream codecs and native page/log leases",
  "interfaces": [
    "server/internal/nativesync.Handler",
    "mb_sync_decode_pipeline",
    "mb_sync_encode_pipeline",
    "mb_sync_pull",
    "mb_sync_checkpoint"
  ],
  "stateAndFailure": "Scoped context admission; coordinator owns writes; stale generation resync; bounded transfer leases",
  "evidence": [
    "TURSO:cli/sync_server.rs",
    "TURSO:sync/engine/src/server_proto.rs"
  ],
  "acceptance": [
    "AT-NSYNC",
    "AT-PARTIAL",
    "AT-SYNCCHECK"
  ]
}
```


### Decision D-12


```json
{
  "id": "D-12",
  "owner": "integration-contracts.md",
  "requirements": [
    "MB-BRAND",
    "MB-SDK",
    "MB-PACKAGE"
  ],
  "strategy": "Selected source/archive identities; registry publication is optional",
  "interfaces": [
    "server/go.mod: module mirabase",
    "sdk/js: @mirabase/sdk",
    "sdk/dart: mirabase",
    "relative signed release artifacts"
  ],
  "stateAndFailure": "No unowned domain or registry claim; all local distribution channels work without a company account",
  "evidence": [
    "PB:go.mod"
  ],
  "acceptance": [
    "AT-BRAND",
    "AT-SDK",
    "AT-PACKAGE"
  ]
}
```


### Decision D-13


```json
{
  "id": "D-13",
  "owner": "integration-contracts.md",
  "requirements": [
    "MB-PERF"
  ],
  "strategy": "Hard initial full-feature engineering budgets and reproducible workload",
  "interfaces": [
    "packaging-and-performance.md",
    "packaging-and-performance.md"
  ],
  "stateAndFailure": "Budget miss fails qualification; optimize the full implementation without hiding features",
  "evidence": [
    "PB:go.mod"
  ],
  "acceptance": [
    "AT-PERF"
  ]
}
```


### Decision D-14


```json
{
  "id": "D-14",
  "owner": "integration-contracts.md",
  "requirements": [
    "MB-LIVE"
  ],
  "strategy": "Go actual-SQL/CDC query-state service with snapshot boundaries and keyed ordered diffs",
  "interfaces": [
    "server/internal/livequery.Service",
    "mb_statement_prepare",
    "mb_statement_step",
    "ChangeTransaction"
  ],
  "stateAndFailure": "Register before snapshot; coalesce net result states; revoke/reset; bounded replay; no missing public subscribe API dependency",
  "evidence": [
    "TURSO:core/connection.rs",
    "TURSO:core/cdc.rs"
  ],
  "acceptance": [
    "AT-LIVE"
  ]
}
```


### Decision D-15


```json
{
  "id": "D-15",
  "owner": "integration-contracts.md",
  "requirements": [
    "MB-PROVIDER",
    "MB-EVIDENCE"
  ],
  "strategy": "Selected engine commit plus owned patch-set digest",
  "interfaces": [
    "b0e1b8fde8eff66ef9843fdd1139fa3305cb16dd",
    "native/bridge",
    "release manifest engineRevision and patchSetDigest"
  ],
  "stateAndFailure": "No moving main; test the selected patched input and record failures without claiming stable upstream status",
  "evidence": [
    "TURSO:Cargo.toml"
  ],
  "acceptance": [
    "AT-PROVIDER",
    "AT-EVIDENCE"
  ]
}
```


### Decision D-16


```json
{
  "id": "D-16",
  "owner": "integration-contracts.md",
  "requirements": [
    "MB-PG",
    "MB-PGAUTH"
  ],
  "strategy": "Go TLS/SCRAM protocol-v3 server with one isolated native PG session per connection",
  "interfaces": [
    "server/internal/pgwire",
    "mb_pg_session_open",
    "mb_pg_session_state",
    "mb_pg_session_close"
  ],
  "stateAndFailure": "No trust server; actor/database bound credential; independent settings/portals; coordinator intercepts transaction controls",
  "evidence": [
    "TURSO:postgres/frontend/session.rs",
    "TURSO:postgres/server/lib.rs"
  ],
  "acceptance": [
    "AT-PG",
    "AT-PGAUTH"
  ]
}
```


### Decision D-17


```json
{
  "id": "D-17",
  "owner": "integration-contracts.md",
  "requirements": [
    "MB-ABI",
    "MB-FTS",
    "MB-CODEC",
    "MB-CDC",
    "MB-ASYNC",
    "MB-ENCRYPT"
  ],
  "strategy": "Existing Go/purego bindings plus owned ABI v1 for missing primitives on all six targets",
  "interfaces": [
    "native-abi-reference.md",
    "mb_abi_version",
    "mb_build_manifest",
    "mb_capture_read",
    "mb_database_quiesce"
  ],
  "stateAndFailure": "Generation-checked handles; copied inputs; explicit buffer ownership; fail ABI mismatch; no per-database process",
  "evidence": [
    "TURSO:bindings/go/driver_db.go",
    "TURSO:bindings/go/bindings_db.go"
  ],
  "acceptance": [
    "AT-ABI",
    "AT-FTS",
    "AT-CODEC",
    "AT-CDC",
    "AT-ASYNC",
    "AT-ENCRYPT"
  ]
}
```


### Decision D-18


```json
{
  "id": "D-18",
  "owner": "integration-contracts.md",
  "requirements": [
    "MB-BACKUP"
  ],
  "strategy": "Quiesced closed-file coherent backup with isolated recovery verification",
  "interfaces": [
    "mb_database_quiesce",
    "server/internal/backup",
    "backup-v1 archive manifest"
  ],
  "stateAndFailure": "Drain all affected writes/file intents; preserve replay/codec state; copy stable files; verify before success; staged new-generation restore",
  "evidence": [
    "PB:core/backup_create.go",
    "PB:core/backup_restore.go",
    "TURSO:bindings/go/bindings_sync.go"
  ],
  "acceptance": [
    "AT-BACKUP"
  ]
}
```


### Decision D-19


```json
{
  "id": "D-19",
  "owner": "integration-contracts.md",
  "requirements": [
    "MB-SQLHTTP"
  ],
  "strategy": "Go authenticated v3 pipeline/cursor handler and scoped leased batons",
  "interfaces": [
    "server/internal/sqlhttp",
    "server/internal/dbexec.Coordinator",
    "native statement classification"
  ],
  "stateAndFailure": "Same actor/context/generation per baton; expiry rolls back; native typed codecs; real client conformance",
  "evidence": [
    "TURSO:serverless/go/README.md"
  ],
  "acceptance": [
    "AT-SQLHTTP"
  ]
}
```
