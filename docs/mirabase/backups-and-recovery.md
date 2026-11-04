# Backup, restore and recovery

**Current release authority:** [current implementation inventory](../current-product.md), [user documentation](../user/index.md) and [launch status](../launch/status.md). Initial v0.1/roadmap delivery labels below preserve planning contracts; they do not establish today's implementation or permit deleting subsequently integrated features. [Open-source platform roadmap](../roadmap/platform/README.md) has separate design and gates.

**Delivery: v0.1 and roadmap.** [v0.1-build.md](v0.1-build.md) selects the current portion; [roadsmap.md](roadsmap.md) retains the remainder. Detailed schemas/configuration/ABI and full-family acceptance below describe the target superset unless narrowed by that allocation.

## Scope and manifest

Preserve inherited backup list/create/upload/download/delete/scheduled retention. A MiraBase archive is versioned and includes a manifest: product/engine/ABI, application format, selected context IDs/generations/revisions/profiles, file entries with role/size/SHA-256, encryption algorithm and key references, codec/extension IDs, snapshot boundary, migration history, file-store coverage and excluded data.

Keys are not embedded by default. An encrypted backup payload is distinct from native page encryption. Archive validation rejects absolute paths, .. traversal, duplicate/conflicting entries, symlinks/reparse points, decompression bombs, bad hashes, unsupported formats and missing required native dependencies.

## Coherent snapshot

Use the selected quiesced copy of closed persistent state in integration-contracts.md. Native online export is an optimization, not a prerequisite. Do not assume VACUUM INTO works across profiles/codecs/sync. Files, FTS, sidecars and metadata must match the same recoverable state.

Default full-app backup stops new affected writes/workers, drains transactions and file finalizations, closes handles and copies a stable app-wide set. Maintenance duration depends on copy size; do not promise it is brief. Record each context's final sequence and the coherent registry state. This is a quiesced application consistency boundary, not a cross-file ACID transaction. Large installations can choose explicit per-context backups.

Local file tracking covers concurrent updates. Remote S3 files require versioned-object manifest/export or a stated exclusion. A successful DB-only archive displays incomplete remote-file coverage rather than "all data backed up."

## Restore

Dry run validates archive, keys, engine compatibility, space, target IDs and destructive impact without replacing live state. Apply enters maintenance, revokes/drains leases, stages data into a new directory, verifies hashes and native integrity, opens it in isolated validation mode, checks collection/auth/file invariants, then atomically publishes with a new generation. Retain the prior copy for rollback according to policy.

Primary app restore must reconcile registry and private contexts. Partial restore cannot overwrite global identities/grants silently. Restore-to-new-context remaps resource identity and explicitly preserves or removes owner/grant references. No outbound jobs/mail execute during isolated validation.

On Windows use stop/restore/start or a verified supervisor workflow; the inherited live restore-and-restart implementation is unsupported. API returns a job/state, not fictitious completed restoration. Recovery instructions must work without the dashboard if startup fails.

## Failure handling

Crash before publication retains old active data and job-owned staging. Crash after publication reconciles manifest/registry generation before accepting traffic. Disk full/checksum/key errors preserve the old copy. Remote files unavailable after restore remain visible as missing with retry/repair options. Old cursors/replicas/tokens tied to prior generations must rebootstrap/reissue.

Schedule defaults remain inherited until explicitly configured. Backup retention and database-trash retention are separate. Deleting a backup never deletes live files or unrelated bucket prefixes. Protect downloads with superuser authorization and short-lived context-bound tokens.

## Export/import and server move

The full PocketBase import can retain shared layout or create tenant databases from the source records/files. It is not fulfilled by collection JSON or an unverified DB copy. Source acquisition, exact preview/report hashes, staged roots and cutover journal recovery are defined in pocketbase-tenant-import.md. Preserve both the old target recovery copy and accepted post-cutover writes; never restart an old snapshot over newer acknowledged data.

Provide portable schema/record/file manifests with stable IDs, type tags and checksums, not only a native file. Native dump/clone/import remain privileged tools and may expose all data. Other-BaaS provider adapters are deferred.

Moving servers means stop or take a coherent backup, copy the complete application data/config/secrets/files/native dependencies, verify restore on the destination, preserve public URL/proxy settings or update frontend endpoint, switch traffic and retain rollback. Copying only data.db is not a complete app move.

## Acceptance

Restore real archives into clean processes on every qualified OS/profile. Compare records/auth/rules/migrations/file hashes/search indexes/sync generations. Kill during stage/publish/restart, corrupt entries, remove keys/native payload, fill disk and simulate missing S3 objects. A ZIP opening successfully or a backup row existing is not recovery evidence.



## Application archives and restore authority

Support complete app archives and independent restore/move. Restore-to-new explicitly removes or reauthorizes owner/grant references; imported app superusers never become instance operators. Preserve other apps' generations and settings. See [application archives and restore authority](implementation-closure.md).
