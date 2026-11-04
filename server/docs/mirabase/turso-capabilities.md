# Turso capability inventory

**Current release authority:** [current implementation inventory](../current-product.md), [user documentation](../user/index.md) and [launch status](../launch/status.md). Initial v0.1/roadmap delivery labels below preserve planning contracts; they do not establish today's implementation or permit deleting subsequently integrated features. [Open-source platform roadmap](../roadmap/platform/README.md) has separate design and gates.

**Delivery: v0.1 and roadmap.** [v0.1-build.md](v0.1-build.md) selects the current portion; [roadsmap.md](roadsmap.md) retains the remainder. Detailed schemas/configuration/ABI and full-family acceptance below describe the target superset unless narrowed by that allocation.

## Evidence and meaning

The selected engine input is audited development snapshot b0e1b8fde8eff66ef9843fdd1139fa3305cb16dd plus digest-pinned MiraBase patches; the latest release observed during the audit was v0.8.3. The snapshot and a release are not interchangeable. Release qualification runs the compatibility/fault suite against this selected input. Sources and paths are recorded in upstream-evidence.md.

## Core, SQL and concurrency

| Capability | MiraBase mapping and condition |
|---|---|
| Embedded, SQLite-file-oriented engine | In-process Go provider; local files/memory; no process per database |
| Shared VM and multiple SQL frontends | Existing generated SQLite SQL plus optional PostgreSQL access |
| SQLite SQL/file/C API compatibility goals | Exact tested compatibility matrix; not a claim of complete drop-in compatibility |
| Relational queries | Joins, subqueries, CTEs, windows, aggregates, upsert and RETURNING where supported by the selected frontend |
| LATERAL and DEFAULT in VALUES | Advanced administrative SQL and validated view/query definitions |
| NULLS FIRST/LAST index constraints | Schema builder extensions; explicit warning that stock SQLite cannot parse all resulting schemas |
| Ordered-set aggregates | mode and supported percentile forms in views/SQL |
| Triggers and virtual generated columns | Schema-aware lifecycle; STORED generated columns are not supported in this snapshot |
| Arrays and typed STRICT schemas | Required field/type adapters, native SQL and round-trip-safe SDK values; implement and qualify core adapters, STRICT rebuilds and REST/SDK/CDC/sync integration together under their owning phases |
| MVCC snapshot isolation / BEGIN CONCURRENT | Optional concurrent transaction profile; existing serialized default remains available |
| Async I/O | Native driver scheduling, cancellation and bounded operation queues |
| io_uring / experimental IOCP / custom VFS | Target-specific build matrix, no platform-independent speed claim |
| SIMD vector paths | CPU dispatch testing; preserve correct fallback |
| WAL, savepoints, checkpoints, recovery | Transaction/recovery integration and operator tools |
| MVCC GC / checkpoint thresholds | Profile-specific maintenance and resource controls |
| Group commit | Optional measured fsync batching; durability contract remains explicit |
| Multi-process WAL coordination | Experimental .tshm facility; not the supported multi-server application topology |
| ATTACH, WITHOUT ROWID, VACUUM, auto-vacuum | Privileged, tested operations with mode restrictions and file-path controls |
| EXPLAIN, bytecode, PRAGMAs, integrity checks | Dashboard/CLI diagnostics with grants and cancellation |
| require_where, retry/fsync controls | Administrative safety settings; platform-specific fullfsync remains platform-specific |

## Data types and functions

Typed arrays include dimensions, literals, indexing/slicing/updating, concatenation, append/remove, containment, overlap, comparisons, array aggregation and string conversion. Custom types include BASE/ENCODE/DECODE, parameters, coercion, defaults, operators, validation errors, casts and introspection. Domains add constrained reusable types and defaults. Named STRUCT and UNION types add structured values, member/tag access and packing/extraction; native composite-column indexing is restricted.

Document and test bool, smallint, bigint, varchar(N), date, time, timestamp, timestamptz, numeric(P,S), uuid, inet, cidr, macaddr, macaddr8, bytea, json and jsonb. Names do not guarantee identical PostgreSQL behavior: timezone offsets are not automatically preserved by timestamptz, and some network-address aliases are textual rather than fully validated. MiraBase field validators must close the specified application-level gaps.

Named sequences support nextval/currval/setval, increments, bounds and cycling; session and rollback behavior are tested per frontend/profile. Keep collection record IDs compatible; adding sequences does not silently replace them.

Additional functions include UUID v4/v7 and timestamp extraction; regexp matching, captures and replacement; digest/encoding functions including MD5, SHA families and BLAKE3; edit/Levenshtein/Damerau/OSA/Hamming/Jaro-Winkler and phonetic/transliteration helpers; nanosecond time/duration parsing/formatting/rounding; IPv4/IPv6 and containment; CSV virtual tables; generate_series; standard deviation, median, percentiles and mode; JSON, date/time and math functions. Digest availability is not permission to use weak hashes for passwords.

## Vectors, search and derived data

Vectors include float32/float64, sparse float32, 8-bit and 1-bit forms; cosine/L2/negative-dot/Jaccard distances where valid, extraction, concatenation and slicing. MiraBase exposes typed fields and exact authorized similarity queries. Mature general ANN indexing is not established by the snapshot; the toy sparse IVF index is a prototype, not evidence of a complete ANN feature.

Native FTS uses Tantivy and a compiled feature/index-method integration. Cover default, English stem, raw, simple, whitespace and n-gram tokenizers; Boolean/phrase/prefix/field syntax; BM25 with weights; highlighting; JSON object indexing; CRUD maintenance and optimization. Native/WASM/build differences are explicit. Current transactional visibility is commit-based; snippets are not supplied by the upstream path. New UI must not advertise unsupported snippets.

Materialized views use incremental DBSP maintenance for the supported query/aggregate/join subset, including nested derived views. Current MVCC/attached-database combinations are restricted. Upstream advertising is not a public Go subscription API. MiraBase supplies its own SQL/CDC live-query service with coherent snapshots, keyed result diffs, authorization and replay as selected in integration-contracts.md.

## Changes, sync and security

Native CDC captures IDs/before/after/full rows into a configurable table, with transaction IDs/commit boundaries and schema changes. It is connection-configured, contains native binary record encodings and does not capture its own table. MiraBase must configure every write path and decode/version data before exposing filtered public frames.

Native sync supports a local writable database, logical push, frame pull, rollback/apply/replay reconciliation and offline pending changes. Full/disabled bootstrap, prefix/query partial bootstrap, lazy page fetch, segment batching, prefetch, checkpoints and sync statistics are separate required adapters. Last-push-wins is the native policy; it is not an application-level merge guarantee. Query bootstrap is page selection, not row-level authorization.

Page encryption is experimental and includes AES-GCM and AEGIS families. Database/WAL pages are protected subject to the native format; the first header region remains visible and key management belongs to the application. Custom page codecs transform storage pages with strict ABI and mode constraints. Neither encrypts files, mail, RAM, logs or network traffic automatically.

## Frontends, bindings and tooling

The PostgreSQL parser, translator, session, native frontend, CLI and pgwire server are implemented source. The upstream compatibility status remains experimental and incomplete. Schemas/search_path/catalogs/types/COPY support are governed by postgres/COMPAT.md; the native wire server's trust authentication and absent TLS must be replaced or protected by MiraBase integration before external access.

Go uses database/sql and purego without CGO, but still loads a native Rust library. Rust, JavaScript/TypeScript native and WASM, Python, C API, Java JDBC, .NET ADO.NET/NativeAOT, Dart and React Native have upstream source/tooling with different coverage. A serverless HTTP client is a different package path from an embedded native driver. Publication and platform support require their own evidence.

The upstream CLI includes SQL, database/schema/index introspection, output modes, parameters, timing/stats/opcodes, VFS discovery, dump/import/clone/read/load/manual tools. Extension facilities include scalar/aggregate functions, virtual tables, VFS and custom index methods. Native Turso extensions are not binary-compatible with arbitrary SQLite extensions.

Upstream includes MCP tools; MiraBase does not include an MCP server under the user's scope. Existing APIs already permit authenticated agents to work. Upstream MIT licensing, simulators, compatibility/differential suites, fuzzing and stress infrastructure are engineering inputs, not a claim that MiraBase or all Turso code is formally verified.

## Engine versus cloud service

Cloud placement/replication fleets, cloud console/billing, provider account quotas, S3-backed cloud durability, cloud copy-on-write branches and cloud point-in-time/deleted-database recovery are service implementations, not automatically present in an embedded Turso library. MiraBase's explicit lifecycle, native backup/restore, change retention and export contracts must be built locally. Native clone/dump is not evidence of cloud-style copy-on-write branching or a complete PITR service. This scope does not silently import Turso Cloud's commercial control plane or the old Mira platform.

## Combination rules

Default WAL supports the inherited application path. MVCC is opt-in. Materialized views cannot currently be combined with MVCC/attached databases. Custom codecs cannot be assumed compatible with MVCC, multi-process, ATTACH or partial sync. FTS is native/build gated. PostgreSQL/SQLite dialect differences stay visible. Encryption algorithm, native sync and checkpoint combinations need explicit qualification. Unsupported combinations fail before opening or modifying the database and state the conflicting settings; they are never silently downgraded.
