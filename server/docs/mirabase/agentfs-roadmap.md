# AgentFS integration build design

**Current release authority:** [current implementation inventory](../current-product.md), [user documentation](../user/index.md) and [launch status](../launch/status.md). Initial v0.1/roadmap delivery labels below preserve planning contracts; they do not establish today's implementation or permit deleting subsequently integrated features. [Open-source platform roadmap](../roadmap/platform/README.md) has separate design and gates.

**Delivery: roadmap R9.** No AgentFS SDK, workspace API, mount helper or dashboard feature belongs to [v0.1](v0.1-build.md). This is the full integration to build later, including its adapters, storage, authority, failures and acceptance. It adds agent workspace storage to MiraBase while preserving ordinary collections/files/auth and the one-process server model.

## Source pin and verified integration points

Use AgentFS commit `0a014ebd4918615baff589ed17486e557e7c6a23` as the initial integration input. Record every owned patch and its digest. Upgrade only with source/schema/engine compatibility review and executed tests. These observations were checked against source on 2026-11-01:

| Source | Verified fact and engineering consequence |
|---|---|
| [Go entry point](https://github.com/tursodatabase/agentfs/blob/0a014ebd4918615baff589ed17486e557e7c6a23/sdk/go/agentfs.go) | `Open` opens the `sqlite` driver and imports modernc. `OpenWith(ctx, *sql.DB, ...OpenWithOption)` accepts an externally owned DB; its `Close` does not close that pool. Use the latter with the MiraBase provider. It is not proof of Turso SQL compatibility. |
| [Go module](https://github.com/tursodatabase/agentfs/blob/0a014ebd4918615baff589ed17486e557e7c6a23/sdk/go/go.mod) | The upstream default dependency is modernc.org/sqlite. Remove/build-isolate default-driver registration in the owned adapter; no live workspace uses that engine as fallback. The PocketBase import-only reader remains a separate explicitly accounted dependency. |
| [Go schema](https://github.com/tursodatabase/agentfs/blob/0a014ebd4918615baff589ed17486e557e7c6a23/sdk/go/schema.go), [SPEC](https://github.com/tursodatabase/agentfs/blob/0a014ebd4918615baff589ed17486e557e7c6a23/SPEC.md) | Schema version 0.4: inode/dentry/chunks/symlinks, configuration, KV and tool-call tables, overlay whiteouts/origin maps. Go initialization ignores errors from nanosecond-column migrations. The owned migration runner must instead introspect existing columns, validate their exact definitions, transact changes and propagate every genuine error. |
| [Filesystem](https://github.com/tursodatabase/agentfs/blob/0a014ebd4918615baff589ed17486e557e7c6a23/sdk/go/filesystem.go), [file handles](https://github.com/tursodatabase/agentfs/blob/0a014ebd4918615baff589ed17486e557e7c6a23/sdk/go/file.go), [KV](https://github.com/tursodatabase/agentfs/blob/0a014ebd4918615baff589ed17486e557e7c6a23/sdk/go/kvstore.go), [tools](https://github.com/tursodatabase/agentfs/blob/0a014ebd4918615baff589ed17486e557e7c6a23/sdk/go/toolcalls.go) | Actual operations are integration inputs. They have no MiraBase grant enforcement. Wrap them behind the workspace service, including handle operations and metadata changes. |
| [Go overlay](https://github.com/tursodatabase/agentfs/blob/0a014ebd4918615baff589ed17486e557e7c6a23/sdk/go/overlay.go), [mount dispatcher](https://github.com/tursodatabase/agentfs/blob/0a014ebd4918615baff589ed17486e557e7c6a23/cli/src/cmd/mount.rs) | Reuse overlay rules and Linux FUSE/macOS NFS machinery. Existing CLI resolves local DB IDs/paths and opens them; pointing it at an arbitrary MiraBase URL is not a remote adapter. |
| [FUSE](https://github.com/tursodatabase/agentfs/blob/0a014ebd4918615baff589ed17486e557e7c6a23/cli/src/fuse.rs), [NFS](https://github.com/tursodatabase/agentfs/blob/0a014ebd4918615baff589ed17486e557e7c6a23/cli/src/nfs.rs), [Rust filesystem trait](https://github.com/tursodatabase/agentfs/tree/0a014ebd4918615baff589ed17486e557e7c6a23/sdk/rust/src) | Add an HTTPS-backed filesystem implementation to the optional upstream-derived helper. Preserve inode/handle/cache/error semantics. No server data directory is opened directly by it. |
| [README](https://github.com/tursodatabase/agentfs/blob/0a014ebd4918615baff589ed17486e557e7c6a23/README.md), [Rust manifest](https://github.com/tursodatabase/agentfs/blob/0a014ebd4918615baff589ed17486e557e7c6a23/sdk/rust/Cargo.toml) | Project declares MIT and beta status. Preserve attribution and dependency notices; verify the exact source distribution's license files/manifests and include its complete dependency SBOM before shipping. An SDK dependency does not automatically include a CLI or OS mount service. |

The MiraBase engine pin remains the engine selected by its provider, not an arbitrary engine pulled by the AgentFS SDK. Qualify AgentFS statements/schema against that engine. A discovered SQL/transaction gap is fixed in the owned adapter or pinned bridge with regression evidence. Never swap engines silently.

## User model and journeys

A workspace holds files/directories, JSON state and explicitly recorded tool history for a user, agent or authorized tenant group. Models and tools run in the developer's chosen process or service. MiraBase stores the workspace and provides authenticated access; it does not become an agent execution/cloud compute service.

1. Configure normal MiraBase accounts/agents and storage. Create a workspace for an authorized target DB. Default sharing is private to the bound subject; tenant sharing requires an explicit workspace policy.
2. Obtain an ordinary application session for the agent's own identity. Use its scoped workspace client to read/write files, persist checkpoints and record tool results. Do not distribute the owner or superuser session.
3. Inspect workspace files, JSON values, operation metadata and tool timeline in the existing dashboard. Reopening the same workspace reads committed progress. The developer decides which checkpoint means a task can resume.
4. If Bash/Git/Python needs real paths, install the optional host helper on that execution machine and mount the authorized workspace. No mount is needed for API/SDK tools.
5. Optionally start a named overlay session against an explicitly registered base, review its diff and explicitly apply or discard it. Snapshot or export the workspace for coherent recovery/transfer.

End users can interact only with the developer's frontend. They need no AgentFS installation or filesystem knowledge. A tenant-shared workspace does not grant access to every member's private agent database.

## Owned components and interfaces

| Component | Interface and ownership |
|---|---|
| `server/internal/workspace.Service` | `Create`, `Get`, `List`, `UpdatePolicy`, `Delete`, `Snapshot`, `Restore`, `Export`, `Import`; takes immutable application/database/actor context and generation guards. Owns identity, grants, lifecycle, quotas and admission. |
| `server/internal/workspace.Store` | Leases a provider-managed pool and opens the owned AgentFS adapter with `OpenWith`. Provides FS/KV/tool operations and transactions through the same coordinator. Returns a lease-bound wrapper, never exposes `DB()` to remote clients. |
| `server/internal/workspace/fsadapter` | Owned AgentFS Go integration with verified migrations, transaction-bound operations, mutation revisions, path/symlink checks and normalized errors. Close releases statements/handles and the manager lease; only the handle manager closes the pool. |
| `server/internal/workspace/http` | Normal authenticated HTTP routes, strict request validation, streaming bytes, idempotency, opaque handle/session tokens and revocation checks. No MCP route. |
| `server/internal/workspace/session` | Overlay manifest, base identity, whiteouts/origin map, diff planning, apply journal and conflict checks. Host base application belongs to the optional helper, not arbitrary server file access. |
| `sdk/js`, `sdk/dart`, Go framework | Immutable `.workspace(id)` clients for filesystem, KV, tool history, snapshots and sessions. Preserve ordinary collection methods. Streaming wrappers expose real backpressure/cancellation. |
| `tools/workspace-mount` | Optional upstream-derived FUSE/NFS helper with an HTTPS `FileSystem` adapter. Its Rust code is a reusable OS bridge, separate from the Go backend. No helper process per server DB; helpers exist only for explicitly requested client mounts. |
| Existing dashboard | Workspace list, file browser/editor/upload/download, KV inspector, tool timeline, sharing, usage, snapshot/restore and session diff/apply views. Server authorization is authoritative. |

Initialization cannot issue uncontrolled PRAGMAs, change pool sizing/profile or swallow migration errors. Configure foreign keys and transaction mode in the provider. Adapt SDK methods that internally start transactions so that compound operations and receipts use one leased connection/transaction. Do not nest independent pool transactions and call them atomic.

## Storage, identity and lifecycle

Workspace IDs are server-generated 32 lowercase hexadecimal characters. Binding includes `applicationId`, parent `databaseId`, parent generation, workspace ID/generation, owner subject or explicit tenant policy, schema revision and policy epoch. Names/paths/emails are never host directory components.

Use one managed companion AgentFS database per workspace at `<contextRoot>/workspaces/<workspaceId>/agent.db`, with native sidecars enumerated in the parent manifest. This avoids collisions between upstream `fs_*`, `kv_store`, `tool_calls` and ordinary user collections. Files/state/history reside together in this workspace database. Whole application state still includes its normal DBs/files/secrets. All workspace pools count against the same aggregate handle/connection/cache budgets; no second unbounded pool cache is added.

Application primary metadata contains `_mb_workspaces` (ID, target/generation, owner/policy, epoch, workspace generation, state, schema/adapter version, manifest hash, timestamps), `_mb_workspace_grants` (workspace, subject, scopes, expiry/revocation, epoch) and lifecycle jobs. A unique idempotency identity guards create. Target-local workspace metadata adds `_mb_workspace_state` (format/generation/revision), `_mb_workspace_receipts`, `_mb_workspace_events`, `_mb_workspace_sessions`, `_mb_workspace_handles` and staged upload/apply intents. Use native integer storage with decimal-string wire revisions.

Upstream AgentFS tables retain their pinned definitions and schema-version marker. Owned `_mb_*` tables do not pretend to be upstream fields. Every mutating transaction increments workspace revision and commits data, result receipt and event metadata together. Directory/inode/chunk/link-count integrity is checked after recovery/import. Read-side atime changes use a bounded best-effort metadata path and never grant a read-only actor content-write authority; read-only snapshots do not update atime.

States: requested -> provisioning -> ready -> maintenance/restoring/deleting/quarantined -> tombstoned -> purged. Stage schema/manifest under a deterministic job directory, verify, publish by atomic rename and reconcile the registry gap. No routing while provisioning. Parent deletion/quarantine fences workspace access before closing handles. Parent restore invalidates every old workspace token and reconciles the companion file manifest; absent files never become empty replacements.

Deleting a workspace revokes policy, drains/cancels leases, closes file handles, moves its complete persistent set to managed trash and reports retained snapshots/object cleanup. Default trash retention follows the target policy. Snapshot/backup keys and dependency manifests remain explicit. Restoration publishes a new workspace generation and invalidates open handles, cursors, sessions and remote mounts.

## Authority and paths

Separate scopes: `workspace.inspect`, `workspace.files.read`, `workspace.files.write`, `workspace.state.read`, `workspace.state.write`, `workspace.tools.read`, `workspace.tools.append`, `workspace.snapshot`, `workspace.restore`, `workspace.share`, `workspace.session`. Parent DB authorization plus a current workspace grant is required. Workspace rights do not imply record/file/SQL/native-replica access. Default tenant membership policies are explicit; removing membership advances epochs and revokes workspace access. Only the configured manager/superuser can grant rights or restore/export a whole workspace.

No HTTP request supplies a host path, SQL statement or arbitrary primary auth query. Filesystem paths use UTF-8 POSIX semantics, absolute within virtual root, at most 4096 bytes with segments at most 255 bytes. Reject NUL, invalid encoding and parent traversal escaping root. Do not clean an escaping path into an allowed one. Preserve case; do not Unicode-normalize names silently. Percent decoding occurs exactly once before validation.

Resolve symlinks within the virtual workspace root with a maximum of 40 traversals; loops/escape fail. Hardlinks stay in the same workspace/generation. UID/GID/mode are virtual metadata subject to the mount policy, not MiraBase identities. Special-file metadata can round-trip for schema compatibility, but the host helper must not instantiate device nodes, privileged FIFOs or sockets from remote data. Mark that restriction in the filesystem compatibility matrix. A mount does not sandbox arbitrary subprocess network/host access.

Mountable workspaces accept only relative symlink targets whose resolved virtual destination stays inside the workspace. The helper refuses mounting a tree containing an absolute or escaping imported symlink until it is corrected; it must not hand the host kernel an absolute `/etc/...` target while describing it as workspace-rooted. Import can retain such a link as inert metadata for inspection/export, but neither API traversal nor mounting follows it. Symlink policy is enforced again on each mutation to prevent a post-mount escape.

Every handle binds actor, workspace/generations, policy epoch, allowed flags, inode, expiry and request-session identity. Recheck on each operation. Revocation closes or invalidates handles within the existing five-second bound and before the next committed mutation. Random opaque handle IDs reveal no file descriptor/path. Denied and absent workspaces use existence-hiding responses.

## HTTP operations and wire contracts

Base path: `/api/mirabase/v1/databases/{databaseId}/workspaces`. Additional application prefixes apply only when R3 is delivered. All mutation requests use `Idempotency-Key`, expected parent/workspace generation and revision; conflicts fail before effects. Normal errors retain `{status,message,data.mirabase}` with `committed=true|false|null`. Large integer offsets/sizes/inodes/revisions are decimal strings. Bytes use streaming bodies or explicitly declared Base64, never ambiguous JSON text.

| Operation | Method and suffix | Contract |
|---|---|---|
| Create/list | POST /, GET / | Create takes display name, reviewed owner/tenant binding and requested quota; returns durable job. List is paginated and authorized. No arbitrary file path or client-assigned identity. |
| Inspect/policy/delete | GET /{id}, PATCH /{id}/policy, DELETE /{id} | Inspect reports state/generation/revision/format/usage/scopes. Policy validates current subject/membership and advances epoch. Delete is a guarded lifecycle job. |
| Stat/list/read | POST /{id}/fs/stat, /fs/list, /fs/read | Typed path; stat has followSymlinks; list uses limit/cursor; read has offset/length and optional expected content revision, returns bytes plus revision/size/hash metadata. |
| Link target/capacity | POST /{id}/fs/readlink, /fs/statfs | Readlink takes a validated virtual path and returns the stored target without following it. Statfs takes no path and reports authorized workspace capacity/free bytes/inodes under actual parent quota and disk reserve, without disclosing other tenants' usage. |
| Namespace/metadata | POST /{id}/fs/mutate | Discriminated `mkdir|mkdirAll|unlink|rmdir|rename|link|symlink|chmod|chown|utimens`, each with exact required fields; reject unknown fields. Rename/link involve two validated virtual paths in the same workspace. Guard content/parent versions and commit one transaction. |
| Atomic whole-file write | POST /{id}/uploads; PUT /{id}/uploads/{uploadId}; POST /{id}/uploads/{uploadId}/commit | Stage under bounded reservation with offset/hash; commit takes path, expected prior content version or create-only, mode, total hash. Atomically publish chunks/inode/receipt. Cancel/expiry cannot erase a committed file. |
| Open/readAt/writeAt/truncate/fsync/close | POST /{id}/handles; POST /{id}/handles/{handleId}/{action} | Open flags are discriminated read/write/create/exclusive/append; actions have exact offset/length/data/size fields. Each write/truncate has its own idempotency key. `fsync` acknowledges the selected native durability boundary, not an invented OS flush. Handle close is idempotent. |
| KV | POST /{id}/state/get, /set, /delete, /list | Key UTF-8 <=1024 bytes; value valid JSON <=1 MiB; set/delete support expected value revision and missing/create-only guards. List is prefix-filtered, paginated and revision-bound. |
| Tool runs | POST /{id}/runs, /runs/{runId}/complete; GET /{id}/runs | Start creates an owned pending invocation with random ID, tool name, sanitized JSON arguments and timestamps. Completion appends exactly one upstream `tool_calls` result/error and binds it to the invocation receipt. Query is current-authority, paginated and redacted. |
| Snapshot/restore | POST /{id}/snapshots; GET /{id}/snapshots; POST /{id}/restores | Quiesced verified native archive or typed export, scope/key/base dependency manifest and checksums. Restore returns a guarded job and new generation only after actual validation/publication. |
| Export/import | POST /{id}/exports, /imports | Native archive or versioned files/KV/tool-history interchange with checksums, stable identities and explicit secret/base coverage. Imports stage and validate with no code execution. |
| Sessions | POST /{id}/sessions; GET /{id}/sessions/{sid}; POST .../{sid}/diff, /apply, /discard | Create binds a registered base/snapshot hash and separate managed delta. Diff is paginated and reviewable; apply requires exact diff/base/version hash and destination permission. Discard cannot undo already applied host or external effects. |

Schema generation must define each union arm, flag combination, numeric bound, response and error before implementation acceptance. The table fixes the API decisions; unrestricted `any`/generic SQL adapters are not substitutes. Reuse the full-product schema tooling and add real SDK/UI conformance fixtures when implementing R9; there are no shipped AgentFS wire types in v0.1.

The namespace union has these exact payload fields, in addition to common expected-generation/revision guards: `mkdir`/`mkdirAll`: path, mode; `unlink`/`rmdir`: path; `rename`: oldPath, newPath, replace boolean; `link`: existingPath, newPath; `symlink`: target, linkPath; `chmod`: path, mode, followSymlinks; `chown`: path, uid, gid, followSymlinks; `utimens`: path, atime and mtime, each tagged `now|omit|value` where value has signed decimal seconds and integer nanoseconds 0..999999999, plus followSymlinks. Mode is integer 0..4095; UID/GID are decimal strings in the permitted unsigned 32-bit range, with null meaning unchanged for chown. No `mknod` public mutation is exposed: special-file metadata import is inert and validated, never host device creation.

Handle action fields are `readAt:{offset,length,expectedContentRevision?}`, `writeAt:{offset,dataBase64,expectedContentRevision}`, `truncate:{size,expectedContentRevision}`, `fsync:{}` and `close:{}`. Reject negative/out-of-range offsets or arithmetic overflow before allocation; maximum logical file size is signed 64-bit, further bounded by actual quota/storage admission. A handle `open` request has path and `{read,write,create,exclusive,truncate,append}` booleans; at least read or write is required, and create/truncate/append require write. Exclusive requires create. Creation defaults to mode 0600; directories default 0700. Never let POSIX mode bits substitute for workspace authorization.

Filesystem reads return consistent ranges from one revision or fail a stale-version precondition; never combine chunks from different revisions. Listings use an opaque cursor binding actor/context, parent inode, generation, revision and sort position. Expired/changed snapshots return `cursor_expired` and require restart. Listings never hold an unbounded directory array in the HTTP wrapper even if an upstream SDK helper does; add bounded SQL pagination to the owned adapter.

Each mounted write is acknowledged only after the corresponding server transaction commits. Retry of a lost response uses the same mutation identity and receipt; uncertain expired outcomes require reconciliation, not blind replay. Concurrent append chooses the final offset under the inode write transaction. Atomic rename preserves inode/link counts and validates both parents; directory cycles, root rename and cross-workspace links fail. Read/write/truncate/unlink-open semantics need a lease-held inode until its last active handle closes, not immediate use-after-delete. Persist/reconcile orphaned open-inode references on server restart.

## Tool history, state and observability

Instrumentation is explicit: SDK wrappers surround actual tool calls or developers call start/complete. Adding AgentFS does not automatically observe every network call, file action or model decision. Tool completion records success or error once; start-only runs become `interrupted` after expired ownership is reconciled. Do not invent a completed result after process loss.

Store owned run state separately because upstream `tool_calls` is an insert-on-completion history. Parameters/results are JSON; apply configured field redaction before persistence, with encrypted protected payloads only under an explicitly configured key policy. Default result/argument limit is 1 MiB each; larger artifacts are separate workspace files referenced by hash. Do not log credentials or hidden auth fields. Server timestamps record receipt time; supplied execution timestamps remain labeled client-reported and cannot establish ordering authority.

Workspace operation events store actor, target generation, operation, paths or approved path hashes, byte count, content revision, request hash and outcome. Content/prompt values are excluded by default. Upstream tool history is operational evidence, not an unforgeable compliance log from an untrusted agent. Owners can have append permission without rewrite/delete rights; privileged retention/restore events are audited separately. Apply the existing 90-day/256 MiB audit policy to operation metadata; tool payload retention is a separate visible policy with default 30 days/256 MiB per workspace and explicit floor/expiry reporting. Active run receipts remain for at least the deduplication window.

JSON KV is persistent application state, not automatic semantic memory. A checkpoint may reference a task version, files and tool results; resuming code validates that checkpoint. Backups do not capture an LLM process, reproduce nondeterministic outputs or undo emails/payments/remote effects.

## Overlay sessions, snapshots and apply

Support database-contained snapshots and host-backed overlays. A contained snapshot is a coherent immutable AgentFS base plus a delta; export includes both or materializes the merged tree. A host-backed base is registered by the helper on the execution host with a stable root identity and content manifest; never accept an arbitrary server host path through HTTP. Host-backed exports explicitly declare the external base dependency or materialize all reachable base files before claiming self-containment.

Read precedence is delta, whiteout denial, then base. On first modification copy the whole affected base file into delta using the pinned upstream semantics; do not claim byte-level database branching. Preserve inode origin maps across copy-up. Metadata changes, hardlinks, symlinks, directory merges and delete/recreate need the same rules. Missing or changed base content fails with `base_changed`/`base_unavailable`; do not silently use the current unrelated file.

Named sessions bind exactly one base revision and one delta. Concurrent collaborators require current grants; all mutations have revision/CAS semantics. Independent experiments get separate deltas. Restoring a session keeps its base dependency and invalidates previous handles/cursors. SQL access to session data remains privileged.

Diff reports path, change kind, old/new hashes, metadata and dependencies. Apply freezes the delta revision, validates every expected base hash under an apply lock and reserves space before effects. For a MiraBase-contained destination, apply data and manifest changes in guarded transactions/journal steps. For a host directory, the helper uses a durable local journal and same-directory temp/rename with parent-directory fsync where supported; revalidate each destination without following escaping symlinks. Record per-path publication and recover partial host application. A host tree is not atomically replaced by a universal cross-file transaction. Conflicts never overwrite unrelated changes. Apply completion requires actual destination hashes; cancellation retains already published paths and a recovery report.

## Optional mount helper

Reuse upstream Linux FUSE and macOS NFS adapters by supplying their filesystem trait with an HTTPS backend. Do not run upstream `Open(localPath)` against the MiraBase server's live files. The helper runs where tools execute; the Go server remains the only writer/coordinator for its managed workspace.

Helper connection configuration contains server URL, application/context/workspace IDs, expected generation, credential reference, mountpoint and read-only/read-write mode. Credentials use the host key store/private file and never CLI output, URLs, NFS metadata or log payloads. TLS verification is mandatory for remote access; development loopback follows the same explicit local policy as the main product. Renewal uses the agent's session and cannot broaden its workspace grant.

Map lookup/getattr/readdir/open/read/write/flush/fsync/release/create/mkdir/unlink/rmdir/rename/link/symlink/readlink/setattr/statfs into the authenticated FS/handle operations. Preserve upstream errno mapping: absent -> ENOENT; denied -> EACCES; collision -> EEXIST; not-directory -> ENOTDIR; nonempty -> ENOTEMPTY; quota/disk -> ENOSPC; stale generation/handle -> ESTALE; lost connectivity -> EIO or bounded retry. Never report success on transport error. File byte-range locks require server-owned leases with expiry and conflict checks; unsupported lock semantics are reported in the mount matrix before clients depend on them.

Linux needs working FUSE and device/user permissions. macOS uses a localhost NFS export from the helper to the OS client; upstream NFS is not the public remote authentication boundary. Bind loopback only, restrict export to the invoking user/mount, choose a free local port and clean it up. Do not expose an unauthenticated NFS export of customer data to the network. If host policy cannot restrict the local export adequately, refuse writable mounting and state the limitation.

Default attribute/content cache validity is at most one second online, with revision validation and zero cached reads after grant/generation invalidation. SDK/HTTP reads always check current authority; the helper cannot erase bytes an authorized process already read. A lost connection never changes a read-write remote mount into an unadvertised offline writer. Flush errors surface to applications; buffered-but-unacknowledged bytes remain clearly uncommitted and can be exported for recovery.

Windows uses the normal workspace API/SDK and dashboard. No upstream native Windows mount is claimed. A future WinFsp/other Windows adapter would require a separate explicit design/qualification; its absence does not remove Windows workspace API support. Containers without mount permissions can use API tools. Filesystem mounting alone is not an execution sandbox; reuse separately qualified upstream sandbox machinery only with explicit host policy and platform evidence.

## Offline and sync integration

This subfeature depends on R4's real native gateway, engine/client pin compatibility and workspace-level authorization. Online HTTPS mounting does not require sync. Offline workspace clients use a private local AgentFS DB; the current identity/generation/store-key rules in [offline delivery](offline-client-delivery.md) apply.

Native replication grants cover the entire workspace companion DB, not just a virtual path. Issue a purpose-bound workspace replica credential only to an actor authorized for all replicated files/state/history. Path-filtered clients use the logical workspace API and explicit file/version mutations; selective bootstrap is a bandwidth feature, not authorization.

Do not merge SQLite pages as independent file edits. Qualify the native engine's actual write-conflict model. The logical reconciliation path uses expected inode/content/KV versions, durable operation IDs and conflict artifacts preserving both versions; no automatic lossy last-writer-wins is implied. Renames/deletes and shared-session edits require an ordered transaction group. Stale parent/workspace generation requires fresh bootstrap while preserving/exporting unsent edits. Wrong keys, expired receipts, revoked access and incomplete file uploads cannot turn into successful sync.

No model process, machine mount or host overlay base is synchronized merely by syncing its AgentFS DB. Host-base transfer requires its own manifest/materialization and destination registration. Publish separate coverage for local SDK, online remote API, mounted remote access and offline sync.

## Budgets, backup and failure handling

Initial owned limits: 8 active workspace queries per target within existing global slots; 32 open handles per principal and 256 per mounted session; maximum one read/write RPC chunk 1 MiB; 8 MiB streamed request cap; 5-second ordinary-operation deadline, bounded longer transfers with cancellation; 1000 entries per directory/KV page; 4 MiB default AgentFS storage chunk size fixed at creation; 7-day mutation receipt retention; 60-second idle handle lease renewed by use. Validate stored chunk configuration; no in-place chunk-size switch.

Workspace operator byte quotas default unset. Reserve staged upload/snapshot/overlay bytes against real disk and the shared reserve; account DB/WAL/index/files/trash/staging separately. Check hard file/JSON/request/inode limits before allocation and native memory budgets before opening pools. The 4 MiB storage chunks do not force 4 MiB HTTP buffers: stream/read ranges through bounded internal buffers. Long writes stage before one bounded publication; test large files without loading the whole file into the Go heap.

Backup quiesces the workspace, drains handles/mutations, closes all native state and copies the exact persistent set with matching parent metadata. Whole-parent backups include every workspace and snapshot/base dependency policy. Restore uses a private clone, validates schema/inode/dentry/chunk/link/KV/tool invariants, required keys and hashes, then publishes a new generation. Missing companion/base/key files fail visibly. Never copy an open DB with plain file copy and call it coherent.

Handle exhaustion returns retryable admission errors without evicting active handles. Process crash discards uncommitted transactions and reconciles deterministic upload/jobs/receipts/open-inode references. Response loss queries the same operation receipt. Disk full retains the old published content and staged failure status. Malformed schema/unsupported engine quarantines the workspace, not its unrelated peers. Parent deletion/revocation cancels new operations and fences existing writes.

## Build phases and acceptance

| Family | Phase and required implemented outcome |
|---|---|
| AF-PROVIDER | A1: source/patch/license manifests; owned Go adapter on the exact Turso provider, no default live SQLite open/registration; real schema initialization/migration/rollback and pool ownership. |
| AF-FS | A2: all filesystem and random-access/handle operations, namespace/link/inode semantics, large-file streaming, bounded listings and real FUSE/NFS-facing error cases. |
| AF-STATE | A2: typed JSON KV, compare-and-set/version behavior, limits and persistent checkpoints without an autonomous-memory claim. |
| AF-TOOLS | A3: explicit start/complete instrumentation, exactly one completion history row per operation identity, interruption handling, redaction/retention and queryable actual results. |
| AF-AUTH | A3: workspace scopes, owner/tenant/agent sharing, current membership and generation/epoch binding across API/handles/exports/snapshots/mounts. |
| AF-RECOVERY | A4: durable create/delete/restore/upload/receipt journals, quotas and coherent target/whole-app backup with actual preserved contents after crash. |
| AF-CLIENT | A5: real JS/TS, Dart and Go helpers plus HTTP contract fixtures, generated types and examples; no invented packages or methods. |
| AF-UI | A5: extend existing dashboard with actual file/KV/tool/policy/snapshot/session workflows, authorization and stale/error states. |
| AF-SESSION | A6: contained/host-backed bases, overlays/whiteouts/origin maps, named shared sessions, diffs, conflict-safe explicit apply/discard and dependency-complete exports. |
| AF-MOUNT | A7: optional upstream-derived helper with HTTPS trait adapter; real Linux FUSE/macOS local NFS mounts with common tools, denied/stale/offline cases and clean unmount. Windows API behavior verified separately. |
| AF-SYNC | A8 after R4: real local DB/native and logical sync interoperability, authority, conflicts/files/generation/receipt recovery; no page-merge or path-filter secrecy fiction. |
| AF-QUALIFY | A9: all families joined to actual source symbols, test counts, target runs, artifacts and measured budgets. Pin/server/SDK/helper compatibility matrix and upgrade evidence required. |

Required fixtures include two auth collections with colliding IDs, two private agents, one shared tenant workspace and one forbidden tenant. Exercise concurrent create/append/rename/unlink-open/link/symlink/truncate, snapshot during writes, permission changes during mounts, loop/escape attempts, wrong-app/workspace/generation handles, incomplete migrations, zero-byte/sparse/large files, Unicode/case, invalid JSON, interrupted tools and duplicate completions.

Crash at every intent, chunk/metadata commit, rename, registry publication, receipt and HTTP acknowledgment boundary. Restore actual bytes and compare link counts, JSON values and tool rows; test tampered archives/payloads, missing keys/bases, disk exhaustion, native cancellation and shutdown. Run Bash/Git/Python workflows on actual mounts; compare metadata/cache/lock/durability behavior with the declared compatibility matrix. Include negative controls that would detect a cross-workspace leak, false write acknowledgment or discarded accepted mutation.

Package helpers separately with pinned native dependencies, signatures, SBOM/notices and no mandatory mount installation for ordinary users. Measure the server with AgentFS disabled and enabled, many cold workspaces, active streams/mounts and large directories. No released artifact or runtime result is claimed by this design.
