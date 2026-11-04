# Native Turso synchronization

**Current release authority:** [current implementation inventory](../current-product.md), [user documentation](../user/index.md) and [launch status](../launch/status.md). Initial v0.1/roadmap delivery labels below preserve planning contracts; they do not establish today's implementation or permit deleting subsequently integrated features. [Managed cloud](../cloud/README.md) has separate design and gates.

**Delivery: roadmap.** This chapter specifies later behavior; v0.1 does not build this service. Shared safety rules still apply where used by released features.

## Privileged contract

Native Turso sync is whole-database replication for explicitly trusted database clients. It preserves the upstream sync protocol and client behavior. It is separate from record-aware sync. Whole-database clients may observe every page in their granted database, including deleted-page remnants subject to the engine format; a query bootstrap is not row isolation.

Do not expose the global primary app database to ordinary users. Private native-replica databases must exclude global credentials/settings. Native SQL writes bypass record forms/hooks just like administrative SQL, and the UI/SDK must state that boundary.

## Adapter and mount

The gateway resolves an opaque database ID, validates native.sync and relevant sql.read/sql.write authority, checks generation/state, and passes authenticated traffic to a revision-pinned upstream-compatible protocol adapter. Preserve upstream request/response formats and negotiation; a custom REST push/pull called "Turso sync" is insufficient.

The audited development server accepts POST /v2/pipeline (JSON PipelineReqBody) and POST /pull-updates (protobuf PullUpdatesReqProtoBody), with directory routes /db/{name}/v2/pipeline and /db/{name}/pull-updates. MiraBase maps /db/{databaseId} through the authorized resolver, preserves the client-facing suffixes and never permits caller-defined filenames. Root routes require an explicitly database-bound credential. OPTIONS uses the configured origin policy rather than the development server's wildcard behavior.

The exact message definitions are pinned sync/engine/src/client_proto.rs and server_proto.rs. Pull carries server_revision, client_revision, encoding and server_pages_selector; the audited development path rejects Zstd encoding. WAL page IDs use the protocol's numbering, not guessed native page offsets. MVCC logical pulls have their own revision offsets and header/frame checksum validation. Preserve all message fields from those source definitions; capture byte-level fixtures and malformed-message negatives before claiming interoperability. No public stable production server API is assumed from the development server. D-07 tracks the production adapter and interoperability evidence, not an unknown route name.

The upstream local development server is not a production multi-tenant service: its open-handle cache, request serialization, unauthenticated mode and client-provided DB naming must not become MiraBase defaults. Replace those boundaries with the common resolver, admission, grants and lifecycle manager.

## Local and remote behavior

Clients open local Turso, read/write locally and retain pending changes offline. Logical mutations push to the remote database; remote frames pull into the local engine. Native reconciliation rolls back local pending changes to the synced base, applies remote changes and replays pending local changes atomically. A failed apply/replay preserves the previous coherent state.

Native conflict resolution is last-push-wins under the qualified protocol, not an application-aware merge. It does not recreate collection validation. Interrupted transfers resume using protocol revisions/receipts and checksums. Generation changes force a new bootstrap; never apply old frames to a restored/new database.

## Bootstrap and partial access

Support full bootstrap and disabled initial bootstrap for an empty offline start when compatible. Partial bootstrap supports prefix length or an approved server-side query, lazy missing-page fetch, segment size (default upstream 128 KiB) and optional prefetch. Prefix/query options are validated and capability negotiated.

Uncached pages need network access; offline reads of them return an explicit unavailable result. Query bootstrap does not prevent later lazy access to other pages. It must only be used where the client already has whole-database permission. Server-side query execution uses the same SQL resource and filesystem restrictions as the granted native SQL channel.

Custom page codecs/partial sync and encryption/transport combinations require explicit compatibility qualification; currently unsupported combinations fail before a replica opens.

## Checkpoint, lifecycle and observability

Do not run an ordinary SQLite checkpoint that destroys native sync/revert state. Use the qualified native sync checkpoint API, with no open write transaction and a verified revision boundary. Report WAL/revert-WAL bytes, pending changes, last push/pull, remote revision, bytes/network work and checkpoint status. Explain when open transactions delay pushes.

Deletion/revocation closes active transfers and invalidates future URLs. Restore changes generation and native replica epoch. Server transfer snapshots are leased and bounded; hard retention/storage limits can require rebootstrap. Backups include native sync metadata only through a proven coherent snapshot; a remote URL alone is not a backup.

## SDK and acceptance

Qualify actual upstream Go, JS, Python and other selected native clients against the gateway, including old/current supported versions. Language existence does not prove every sync feature works. Test unauthenticated traffic, cross-DB path changes, SQL filesystem escape, partial-page access, interrupted pull/push, replay conflicts, transaction grouping, checksums, expired revision, restore and checkpoint growth.

The implementation is selected in integration-contracts.md: an authenticated Go handler, exact pinned protocol codecs, native page/log bridge and shared transaction coordinator. Real client interoperability and fault runs are mandatory acceptance work. A route or mocked client is not completion.



## Application-qualified native access

Application prefixes and credential purposes resolve before the native database ID. App-primary pages can contain local auth secrets and remain privileged; record clients use authorized record sync. Native protocol support does not add shared-process or multi-node topology. See [application-qualified native access](application-isolation.md).

The application prefix is `/a/<applicationId>` before the existing native path, not an extra `/api` segment. Root native clients can route through the authenticated credential `kid` described in application-isolation.md. Native message suffixes and payloads remain upstream-compatible; application selection and authentication are gateway concerns.
