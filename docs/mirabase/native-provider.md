# Native provider and ABI

**Current release authority:** [current implementation inventory](../current-product.md), [user documentation](../user/index.md) and [launch status](../launch/status.md). Initial v0.1/roadmap delivery labels below preserve planning contracts; they do not establish today's implementation or permit deleting subsequently integrated features. [Managed cloud](../cloud/README.md) has separate design and gates.

**Delivery: v0.1 and roadmap.** [v0.1-build.md](v0.1-build.md) selects the current portion; [roadsmap.md](roadsmap.md) retains the remainder. Detailed schemas/configuration/ABI and full-family acceptance below describe the target superset unless narrowed by that allocation.

## Provider contract

Implement Config.DBConnect using a Turso database/sql connector and DBX's SQLite builder. Build with no_default_driver so a second SQLite engine is not accidentally used. Both data.db and auxiliary.db must go through the selected provider. Keep the original provider available only for reference qualification/import tooling, never as an unnoticed fallback after Turso errors.

The full PocketBase importer uses the pinned reference SQLite reader only on its protected source copy. Recovery, if required, changes that private copy before read-only inventory. This import-only reader is separately named/bounded and cannot open live MiraBase application files. It does not change the Turso-only destination provider or introduce a server process. Its compiled footprint is included in the full release budget.

The driver must support prepare/bind/step/reset/finalize, Exec/Query/Rows, positional/named values, NULL, text, bytes, exact integer boundaries, cancellation, BeginTx, Commit, Rollback, errors and Close. Test database/sql's context and connection reuse semantics, multiple result rows, early row closure, statement invalidation and panic recovery. A native handle remains live while rows/statements/transactions reference it.

## DBX and schema compatibility

Run the inherited PRAGMA configuration only after checking support and actual readback. Foreign keys must be active on every relevant connection. Busy timeout, cache, WAL/checkpoint and synchronization settings are profile-specific and visible in diagnostics. Never ignore a failed security/durability PRAGMA.

Audit sqlite_master/PRAGMA_TABLE_INFO/schema parsing, table rebuild migrations, index parsing, last insert/rows affected, placeholders, RETURNING and transaction nesting. Classify native errors into stable application errors with the native code preserved in internal diagnostics. User messages omit paths, secrets and raw SQL parameters.

Turso canonicalizes stored table/view DDL and default-expression text, and `sqlite_master.name` returns lowercased index names while the stored index DDL keeps the original case. MiraBase recovers the original-case index key from the DDL when it matches case-insensitively, and inherited introspection assertions use exact provider-specific canonical text. Behavior is qualified separately: defaults apply, view columns/results and wrapping match, and every declared index exists. Bare `VACUUM` requires the engine's experimental `vacuum` feature token; MiraBase enables it on every database open, and qualification covers real space reclamation and payload preservation, not only statement acceptance.

## Concurrent transaction adaptation

PocketBase's nonconcurrent pool has one open connection. Merely enabling MVCC does not allow concurrent application writes. Preserve the public serialized NonconcurrentDB semantics; introduce an internal bounded MVCC writer pool for context-aware record operations.

DBX expects a real database/sql transaction. The audited Go driver already implements driver.ConnBeginTx and currently issues BEGIN. Extend its connector to select BEGIN CONCURRENT for the MVCC writer profile on that pinned connection and return driver.Tx. database/sql then supplies the real *sql.Tx expected by DBX. Manual BEGIN on a random pooled connection is unacceptable. The owned ABI and all-path transaction coordinator are specified in integration-contracts.md.

No connection may leak an unfinished transaction, changed auth context, CDC configuration or unsafe PRAGMA back into the pool. ResetSession/check-on-return restores or discards it. Savepoint/nested transaction behavior follows the inherited DBX contract and the selected profile.

## Async operations and FFI

Bind each native async operation to a Go context and ownership record. Bound outstanding I/O, keep Go buffers alive across native calls, do not retain movable Go pointers in Rust, and marshal callback exceptions safely. Cancellation races, completion after cancellation, library faults and repeated Close are tested. Native buffers have exactly one allocator/free owner; no unbounded finalizer-only cleanup.

Go remains the product implementation language. The upstream engine is Rust compiled as a native library. No-CGO describes the Go FFI mechanism, not an absence of native code, runtime assets or platform qualification.

## Native feature manifest

A build manifest records engine commit, library digest, ABI version, compiler/target, enabled features, optional frontend/FTS/encryption/codec/extensions and CPU requirements. The expected library digest is compiled into the executable at link time with `-ldflags -X mirabase/internal/mbengine.pinnedLibraryDigest=<sha256>`; startup hashes the library file and requires an exact match before binding any symbol. A checksum file placed next to the library is not an authority: modifying the library and its adjacent checksum cannot authorize the payload. An unset or malformed pin is fatal. Startup then executes read-only capability probes in an isolated scratch database. An unexpected ABI or missing required symbol is fatal before application files open.

Profiles select among real compiled capabilities; they do not simulate absent functionality. Separate native libraries must not open the same file with incompatible engines. Page codec and extension ABI IDs are persistent format dependencies and belong in backups.

## Packaging and loading

Prefer a single executable download containing a signed/hash-verified native payload. Extract to a private, versioned cache with restrictive permissions, atomic rename and no writable search-path lookup. Resolve only an absolute verified path. Reject symlink/reparse/path traversal substitution and concurrent partial extraction. Do not load an arbitrary library from the working directory or PATH.

Static embedding is a separate qualified build strategy; it is not assumed available on every purego target. Record the total executable plus extracted native footprint, dependencies and supported OS/architecture. Do not promise a successful binary on an unbuilt target.

## Required qualification

Compare the complete inherited suite on the original and Turso providers. Include crash after WAL append, cancellation during step/commit, disk full, read-only filesystem, malformed DB, native ABI mismatch, missing payload, extension mismatch and secret-free errors. Native test success alone does not establish collection/API correctness. Installed Go 1.26.2 is below the pinned PocketBase Go 1.27 requirement; runtime build evidence must use the required toolchain and record its actual version.



## Import-only reference reader

Use modernc.org/sqlite v1.60.1 from the pinned baseline in a separately named import reader package. Its connector accepts only verified session-owned source-copy paths, is read-only after source-copy recovery, and is never registered as a live destination fallback. The no_default_driver build removes the inherited live default connector; it does not claim absence of the import reader's SQLite code. Include its full footprint in release budgets. Live primary/auxiliary/managed databases always use Turso.
