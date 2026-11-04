# Agent workspaces and AgentFS

Current code implements workspace create/list, file write/read/delete, snapshot create/restore and metadata/tool-history helpers. These are real local filesystem/DB operations in [agentfs.go](../../internal/application/agentfs.go). They do not establish integration of upstream AgentFS's Go SDK, inode/chunk schema, KV API, overlays, FUSE/NFS mounts or offline replication.

Current HTTP workspace routes are **superuser-only** in [admin handlers](../../apis/mirabase_admin.go), below `/api/mirabase/v1/agentfs/workspaces`. Do not give an untrusted agent a superuser token. Trusted operators can use these endpoints; ordinary agent-scoped workspace permissions need complete integration and acceptance.

## Operator API map

Paths are relative to `/api/mirabase/v1/agentfs/workspaces`; retain actual returned IDs and existing owner subjects.

| Operation | Method/path | Input |
|---|---|---|
| Create | `POST` root | Owner collection/record and display name. |
| List | `GET` root | Owner query fields where required. |
| Write | `PUT /{workspaceId}/files` | JSON path and string content. |
| Read/delete | `GET` / `DELETE /{workspaceId}/files` | Query path. |
| Snapshot | `POST /{workspaceId}/snapshots` | Actual workspace. |
| Restore | `POST /{workspaceId}/snapshots/{snapshotId}/restore` | Stored snapshot identity. |

Tool history is an explicitly instrumented server helper, not an automatic observer or an implied public endpoint.

## Check historical-byte recovery

1. Create a controlled workspace with an existing owner.
2. Write a note, read it back and retain its actual bytes/hash.
3. Create a snapshot and preserve its ID.
4. Replace/delete the note and confirm the live content changed.
5. Restore the snapshot and compare the read-back bytes/hash with the original.
6. Check files outside the snapshot and retain failure evidence. A success envelope alone does not prove restoration.

This recovers selected working files, not an agent process or external mail/payment/network effects. The current adapter is local files plus owned metadata/history; upstream SDK/KV/inode/chunk/overlay/mount/session/replication parity has additional obligations in the [full design](../mirabase/agentfs-roadmap.md).

Create takes `ownerCollectionId`, `ownerRecordId`, `displayName`. File write is `PUT /{workspaceId}/files` with `path` and string `content`; read/delete select `path` in the query. A string content API is not an arbitrary binary streaming protocol. Server roots remain managed paths.

Snapshots store manifests and historical file bytes together in the application database, with a 64 MiB total content limit per snapshot. Restore validates the manifest, paths and every stored file hash before changing live files. It restores overwritten or deleted files and leaves files outside the snapshot untouched. Legacy snapshots without stored bytes fail explicitly. Workspace writes, deletes, snapshots and restores share bounded in-process locks. Restore publishes files individually; multi-file crash atomicity and interrupted-write recovery are not qualified. Snapshots do not capture a model process or undo external effects. Tool history is explicit instrumentation, not automatic observation of every call.

The [full integration contract](../mirabase/agentfs-roadmap.md) specifies the pinned SDK/provider, sharing, files/KV/history, mounts, sessions/diffs/apply, coherent backup and sync. Mount helpers run where tools execute; API access needs no mount. Windows API support does not imply a Windows native mount.

[Agent guide](../agents/README.md) · [current inventory](../current-product.md) · [API](api-reference.md)
