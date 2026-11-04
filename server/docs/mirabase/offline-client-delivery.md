# Offline client delivery

**Current release authority:** [current implementation inventory](../current-product.md), [user documentation](../user/index.md) and [launch status](../launch/status.md). Initial v0.1/roadmap delivery labels below preserve planning contracts; they do not establish today's implementation or permit deleting subsequently integrated features. [Open-source platform roadmap](../roadmap/platform/README.md) has separate design and gates.

**Delivery: roadmap.** This chapter specifies later behavior; v0.1 does not build this service. Shared safety rules still apply where used by released features.

## Selected implementations

Record-aware sync is an optional local companion to the remote SDK. It does not embed the Go backend in every frontend and does not give an ordinary client page access to an application auth/settings database. The wire protocol remains record-sync.md; this chapter selects local storage, identity, delivery and recovery adapters.

The local engine is built from the same pinned Turso commit identified in upstream-evidence.md. MiraBase owns the thin local-store adapters and async VFS where the upstream language wrapper does not expose a required primitive. Local builds have separate target manifests and capability probes. A native package name or upstream binding directory does not establish that its store implementation works.

| Runtime | Owned adapter and persistence | Delivery |
|---|---|---|
| Browser | `BrowserTursoStore`: WASM engine in a dedicated worker; OPFS file backend and Web Locks for one writer per local store | JS/TS package with relative worker/WASM assets and manifest digests; no runtime CDN download |
| Node desktop/server | `NodeTursoStore`: pinned engine through the owned native binding; local private directory and OS file lock | Platform-specific native assets in the offline package archive |
| Dart desktop/mobile | `DartTursoStore`: `dart:ffi` wrapper over the owned local ABI; application-support directory and OS lock | Dart path/source package plus native payloads for the selected desktop/iOS/Android targets |
| Go desktop/server | `GoTursoStore`: selected Go connector and owned sync queue layer | Go source package using the same pinned payload contract |

The supported browser baseline is Chromium 120+, Firefox 122+ and Safari 17.4+, provided required WASM, dedicated-worker, OPFS and Web Locks capabilities pass real startup probes. Node baseline is 22 LTS or 24 LTS; Dart SDK baseline is 3.6+ with Flutter 3.27+ where Flutter is used; mobile targets are Android API 26+ and iOS 15+. These are chosen minimum qualification targets, not claims of published packages or passing runs. Desktop targets match Linux/Windows/macOS amd64/arm64. Platform dependencies are locked by source revision/toolchain/artifact digest during the package build; no floating latest dependency is allowed in a release manifest.

Browser persistence never silently falls back to volatile memory. An explicitly requested temporary in-memory store is labeled nonpersistent and cannot claim durable offline edits. Unsupported APIs or asset/hash failures return an exact local-store error. Browser FTS and codec limitations remain in the capability matrix; offline local query availability does not imply server/native feature parity. OPFS storage eviction and user-cleared site data cannot be prevented by the SDK; request persistence where available and expose/export pending edits before destructive actions.

## Local store and owned interfaces

`LocalStore` implements open, close, transaction, executePrepared, readManifest, writeManifest, putPendingGroup, listPendingGroups, applyAuthorizedBase, applyRemoteTransaction, stageFile, checkpoint and exportPending. The local schema contains `_mb_local_manifest`, `_mb_local_pending`, `_mb_local_receipts`, `_mb_local_file_intents` and the authorized record tables. A committed local mutation records its pending intent atomically with the local optimistic view; app termination cannot leave a visible edit without a durable queue entry.

Browser VFS owns open/read/write/truncate/sync/lock/unlock/close over OPFS. Calls are marshalled through one worker with bounded operation IDs and completion events. WASM pending I/O suspends and resumes only the owning operation. A Web Lock keyed by local-store identity excludes another tab's writer; follower tabs use explicit messages or open a distinct store. Missing lock support fails persistent open. File sync completes before the local transaction is acknowledged. Worker termination/crash tests verify recovery and avoid claiming stronger durability than the browser API provides.

Native local ABI uses generation-checked connection/statement handles, copied inputs, owned output buffers, cancellation and the provider's normalized statuses. Dart/Node adapters must not retain movable VM pointers across async native operations. Node packaging uses an owned N-API addon around that ABI; Dart uses FFI with asynchronous work off the UI isolate. This is local client glue, not a Rust HTTP backend. Build all native/WASM artifacts and bindings from pinned inputs before advertising the target.

## Identity, keys and switching

Store identity is the hash of canonical server origin/base path, application ID, subject auth collection/record ID, database ID/generation and projection/filter identity. It never keys solely by email, display name or database filename. A manifest records that identity, schema revision, policy epoch, applied cursor, receipt horizon, engine format and key references. Opening a mismatched store fails without changing it.

Credentials use OS credential storage on native mobile/desktop and the SDK's explicit browser auth-store policy; never write them into synced record tables. Pending intent/file envelopes use AES-256-GCM with random nonces and associated data binding local-store identity and mutation ID. Native keys use Apple Keychain on macOS/iOS, Android Keystore-wrapped keys, user-scoped DPAPI on Windows and owner-private 0600 key files on Linux. Linux Secret Service is an optional key provider, not a required daemon. Missing provider/key material fails store unlock without deleting pending edits. Browser keys are nonextractable WebCrypto keys persisted with their origin-scoped key records. Browser origin compromise can still access unlocked application data; this is not a hostile-device secrecy guarantee. Page encryption is enabled only for a qualified native profile; a device key alone is not a claim that browser database pages are encrypted.

Logout stops network work/subscriptions, clears credentials and locks the store. Default behavior retains the local base and encrypted pending edits under their original identity for later reauthentication. Explicit delete-local-data first exports or requires acknowledgement of pending-edit loss. Signing into another account opens another store; it never relabels old edits. Tenant/application switching closes old scoped sessions and opens the correctly keyed store. Reauthentication as the original subject is required to unlock/reconcile its queue. Server revocation stops future sync; it cannot guarantee remote erasure of an offline or malicious device.

## Apply, conflicts and recovery

Bootstrap writes a candidate authorized base while preserving the pending queue, verifies manifest/page hashes and swaps the base in one local transaction. Pull changes are applied as complete server transactions. Pending edits replay only after that commit; a failed replay preserves the previous consistent base plus the queue and an explicit conflict. Files remain pending until hashes and finalization are verified. No arbitrary server-supplied JS migration is executed.

Schema changes use owned declarative mappings supported by the adapter. An incompatible change exports/preserves pending mutations and creates a new authorized base. A generation/policy reset, compacted tombstone history or receipt expiry follows implementation-closure.md. Unknown commit outcome is not converted to a new mutation automatically. Conflict resolution accepts keep-server, reviewed-local-reapply or application-supplied deterministic merge only after current server authorization; every new group has traceable lineage to the preserved intent.

The module exposes last push/pull, pending groups/files, conflicts, retention horizon, bootstrap progress, local durability mode and errors. Background synchronization respects runtime lifecycle and does not promise execution after a browser tab or mobile app is suspended. Network reconnect never silently drops unsent edits.

## Acceptance

MB-LOCALSDK and MB-LOCALIDENTITY require real engine stores on the advertised runtime matrix. Kill workers/apps before/after queue commit, remote receipt commit, response loss, local base swap and file finalize. Use two accounts with the same email in different apps, two tenants, multiple tabs, removed memberships, revoked tokens, device-key loss, disk/quota exhaustion, browser storage removal, incompatible schemas and expired receipts. Verify actual rows, intent hashes and identities after restart. Package installation on a clean device must resolve all assets offline without Go/Rust toolchains or a CDN.
