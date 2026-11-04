# Selected integration implementations

**Current release authority:** [current implementation inventory](../current-product.md), [user documentation](../user/index.md) and [launch status](../launch/status.md). Initial v0.1/roadmap delivery labels below preserve planning contracts; they do not establish today's implementation or permit deleting subsequently integrated features. [Open-source platform roadmap](../roadmap/platform/README.md) has separate design and gates.

**Delivery: v0.1 and roadmap.** [v0.1-build.md](v0.1-build.md) selects the current portion; [roadsmap.md](roadsmap.md) retains the remainder. Detailed schemas/configuration/ABI and full-family acceptance below describe the target superset unless narrowed by that allocation.

This chapter specifies the engineering choices under D-07 and D-12 through D-19. These are implementation contracts; their selection does not establish working code or passing tests. Implement their concrete interfaces, state, failures and acceptance as part of the complete build.

## Source layout, identities and selected pin

The target implementation lives in server/, a Go module named mirabase derived from the pinned PocketBase source. Its root package is mirabase; neutral core/apis/tools package names remain. The current pocketbase/ tree is the unchanged audit reference, not another shipped server. sdk/js is the @mirabase/sdk package; sdk/dart is the mirabase Dart package. Offline adapters live below sdk/offline. native/bridge contains a thin C-ABI integration crate and minimum source patches, not an HTTP service. _build contains disposable build inputs/artifacts, never application data.

The engine input is b0e1b8fde8eff66ef9843fdd1139fa3305cb16dd plus versioned MiraBase patches. It is selected now, not moving main. The release manifest identifies upstream commit plus patch-set digest, and the executable embeds the exact native library sha256 at link time; the adjacent payload checksum file is informational only. All tests use that exact result. A failed test is implementation work on this base; it does not trigger an unnoticed replacement.

Identities are usable without a public registry reservation: Go consumers use the source module plus a local replace directive; JS consumers install the released package archive; Dart consumers use the supplied source/path package. Registry publication is an optional channel. The executable is mirabase. Distribution manifests use relative artifact names and an operator-supplied release feed; no unowned website or public signing key is invented.

## Native API and Go provider

Use the existing Go driver and purego for exposed functions. Source confirms driver.ConnBeginTx exists but issues BEGIN. The MiraBase connector selects BEGIN CONCURRENT for its MVCC writer profile and returns a real driver.Tx, preserving database/sql and DBX semantics. Implement ResetSession and discard a connection after failed cleanup.

Missing PG, capture-decoder, protocol and snapshot primitives use the owned ABI in native-abi-reference.md. These are new MiraBase symbols, not names claimed upstream. One native payload contains engine and thin adapters. Go owns HTTP, auth, rules, routing, jobs, scheduling, wire authentication, policy and UI. There is no Rust server process.

Handles are generation-checked opaque 64-bit IDs. Buffers remain native-owned until mb_buffer_free; inputs are copied before return. No native call retains a Go memory pointer. Status distinguishes success, row, done, pending I/O, conflict, cancellation, constraint, unsupported syntax, corruption and fatal ABI errors. Normalize native errors without exposing secrets.

Every prepared statement has native-parsed classification: read-only, data-write, schema, transaction-control, maintenance, file-access or extension-load. The compiler/frontend AST determines this, including CTEs, multi-statements and function side effects. Reject unknown mutating classes in restricted channels. A keyword prefix is not authorization.

For the full-product payload build with core fs/uuid/time, fts and encryption, including the PG frontend, sync engine and qualified native extensions. The v0.1 payload uses the explicit ABI/feature subset in [its build contract](v0.1-build.md#native-abi-and-artifacts); PG/sync/codecs/native extension loading are roadmap groups. io_uring is Linux-specific, experimental_win_iocp Windows-specific, and the fallback remains. All six targets include every feature promised for that release. A target failure requires fixing the bridge/build rather than silently removing a selected release feature.

## Shared transaction coordinator

Implement server/internal/dbexec.Coordinator. Record CRUD/DBX, trusted framework SQL, SQL console, PG, SQL/HTTP and native logical pushes use it. External listeners never receive a raw engine handle or the native trust server.

The coordinator owns a physical transaction for every write. Parsed explicit transaction controls are handled centrally; an autocommit write receives an implicit transaction. Multi-statement requests preserve their protocol's statement/group boundaries. Track logical protocol autocommit separately from an internal implicit transaction so conditional batches remain correct.

After the final user statement finishes, before COMMIT, read its native CDC rows using mb_capture_read. Decode the pinned native record format; apply the table/collection mapping; write normalized envelopes, persistent record versions, receipt references and outbox intent through that same transaction. Internal _mb_* writes are excluded. Then native COMMIT runs. Publish SSE/live-query notifications only after successful completion. Failed normalization rolls back the write.

Imported preserve-global uniqueness policies also use the primary reservation service before local commit. All write transports reserve the normalized source index keys, then finalize/reconcile through target transaction tags. The exact crash/lease behavior is specified in pocketbase-tenant-import.md; expiry never frees a key from an uncertain commit.

Capture includes triggers/cascades and respects savepoint rollback. Schema statements use the schema planner. Maintenance that cannot run inside a transaction uses an exclusive lifecycle job and explicit generation/schema invalidation. No post-commit process is relied upon to reconstruct missing atomic frames.

Commit uncertainty is resolved using receipts/recovery. Trusted code bypassing the provider to open files is outside supported application access; the single-writer lock prevents accidental concurrent MiraBase processes. Test all access paths for attempts to commit around the coordinator.

## D-07: production native sync

Implement server/internal/nativesync.Handler on the Go HTTP server. Mount POST /db/{id}/v2/pipeline and /db/{id}/pull-updates. Root equivalents require a credential bound to one database. Validate grant, state, generation, size, encoding and admission before native calls. Origins use inherited configured policy.

Use exact PipelineReqBody/PipelineRespBody and protobuf definitions in pinned sync/engine/src/server_proto.rs. The native adapter handles wire codecs; pipeline execute/batch/conditions call the shared coordinator. Preserve NULL, integer strings, blobs, step errors, logical transaction behavior and replication indices. Request paths never select filenames.

Pull uses leased page/MVCC-log snapshots and the pinned protocol's page numbering, revisions, replace-base/incremental indicators, checksums and log metadata. Expired/incompatible generations force resync. Raw encoding is required; compression is negotiated and rejected if unavailable. Partial prefix/query bootstrap and lazy segment reads use the same snapshot/grant. Bootstrap never narrows whole-database authority.

Serialize sessions per connection/transaction, not globally across databases. Eviction cannot close a live transfer. Cancellation affects only its operation. Checkpoint uses the native sync-aware API; never truncate replay state through ordinary WAL checkpoint.

AT-NSYNC, AT-PARTIAL and AT-SYNCCHECK use real upstream Go/JS/Python clients, reference-server wire fixtures, malformed frames, concurrent contexts, response loss, stale generations and filesystem-escape negatives.

## D-14: live queries without a public native subscription API

Implement server/internal/livequery.Service using SQL plus durable commit capture. Query-result subscriptions do not depend on an upstream public Go subscribe method. Native materialized views remain valid query sources in their supported profile.

A superuser registers a named parameterized read-only plan. Validate deterministic expressions, a bounded result and a unique non-null result key. Inject collection/field policy using the shared query planner. Store table dependencies, revision, parameter types and result shape. Ordinary users instantiate approved queries in their authorized context. Default bounds are 1000 rows and 8 MiB per subscription; exceeding them is an error, not silent truncation.

Register the invalidation listener before the initial read transaction. In that transaction read commitSequence and evaluate the query. Queue invalidations during execution; after publishing the snapshot process sequences greater than its boundary. This closes the subscribe/query race.

On a dependent commit, reevaluate within a consistent read transaction and capture its sequence. Compute keyed inserts/deletes/updates and position changes for ordered/limited results. A delta includes fromSequence, toSequence and a complete ordered patch applied atomically. Multiple commits may be coalesced into one result-state delta. Durable changes remain the interface for every transaction. This is an explicit query-state contract, not a claim to replay every intermediate result.

Keep current results and bounded replay patches in the shared service. Cursor binds subscription, actor, context/generation, query hash and sequence. Default replay lease is five minutes/8 MiB. Missing history produces reset plus a new snapshot. Slow consumers disconnect with a known boundary. No per-database JS runtime or full database copy is created.

Recheck authorization before evaluation/publication. Schema/policy changes reset or terminate; revoke follows the five-second bound. After process restart, reset if prior replay state was not persisted. AT-LIVE checks races, joins/aggregates/top-N, order changes, revocation, coalescing, deletion, overflow and reconnect against real query results.

## D-16: PostgreSQL auth and session isolation

Implement server/internal/pgwire using Go networking/TLS and a bounded protocol-v3 state machine. Do not run the upstream trust/no-TLS server. SSLRequest negotiates TLS; reject external plaintext/trust fallback. Resolve Startup database to one opaque authorized context. Purpose=pg credentials use SCRAM-SHA-256 with random salt and stored verifier, never a record's bcrypt hash.

After auth, create one native PgConnection session against the selected handle. Source exposes connect/connect_with, prepare, query/query_runner, transaction_state and inner; the owned bridge wraps these. Each client gets independent settings, search_path, statements, portals and transaction state. Move any globally held native session parameters into this per-session boundary.

States: startup → TLS/auth → idle → simple-query or parse/bind/describe/execute → idle/in-transaction/failed → close. Bound message lengths, parameter counts and memory. Sync restores extended-query error state. Cancellation uses a random backend secret and affects one session. Revoke closes it at the next policy boundary.

Transaction controls use the shared coordinator. Data statements use native PG translation, not SQL text substitution. Row types/codecs follow the declared compatibility matrix. Managed DDL reconciles schema or fails before effects. COPY follows supported forms and filesystem policy. AT-PG/AT-PGAUTH cover actual psql/Go/JS/GUI clients, TLS/SCRAM negatives, independent sessions, prepared statements, transaction errors, cancellation, COPY and schema security.

## D-19: SQL over HTTP

Implement server/internal/sqlhttp on the existing router: /db/{id}/v3/pipeline and /db/{id}/v3/cursor. Root paths require database-bound credentials. Port the upstream DTO/codec and compare actual serverless clients. SQL passes native classification and the coordinator under purpose=sql-http authority.

Opaque authenticated batons bind actor, credential epoch, context/generation, leased session and expiry. Default idle lease is 60 seconds. Close/cancel/expiry rolls back unfinished transactions and frees statements. Batons cannot change users/databases and fail after restore/revocation. Bound steps, stored SQL, rows and bytes. Distinguish response interruption from commit outcome; reconcile with receipts.

AT-SQLHTTP covers positional/named values, conditional batches, stored SQL, streaming, multi-request transactions, expiry/reconnect and denied filesystem/extension operations.

## D-18: coherent backup across profiles

The required correctness path is a quiesced snapshot. Online snapshots are optimizations, so a missing universal VACUUM/export API does not prevent backup implementation.

Persist job intent and target list. Enter app-wide write maintenance for full backup, or target maintenance for context backup. Stop affected record/file/SQL/PG/sync writes and workers; drain transactions/finalizations; reconcile uncertain commits; close affected rows/statements/connections. Native quiesce completes pending I/O and flushes persistent state without discarding sync/revert history. Cold contexts need no reopen.

Enumerate native manifest persistent files, including WAL/MVCC/replay/FTS/codec state; copy with matching local files and coherent registry state to job-owned staging. Reject unknown required files/external writers. Hash bytes, record key references and remote-object coverage. Do not silently embed encryption keys. Reopen live handles after the stable copy; compression/upload may continue outside maintenance.

Verify an isolated disposable clone using the same engine/patch/profile/codec and required keys, with outbound hooks/mail disabled. Compare registry/schema/records/files. Archive the untouched stable copy and mark succeeded only after verification/manifest publication. Failure preserves live files and names the failed step.

Restore uses quiesce and staged new-generation publication. Windows uses stop/restore/start. AT-BACKUP proves recovery for WAL/MVCC/native-sync/FTS/encryption/codec contexts; none waits for an upstream online snapshot API.

## Design closure and implementation evidence

D-12 identities, D-13 performance targets, D-15 source pin and D-17 native coverage are selected here, in [the engine reference](engine-reference.md) and in [packaging and performance](packaging-and-performance.md). Implement the complete target on all six platforms; optimize linking/lazy runtime initialization rather than removing features to meet budgets.

The ABI header, plan and closure records are design artifacts. Runtime evidence stays empty until real code and runs exist. No design-resolution record may advance an implementation-ledger status.

