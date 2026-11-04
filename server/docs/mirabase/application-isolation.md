# Application isolation in one process

**Current release authority:** [current implementation inventory](../current-product.md), [user documentation](../user/index.md) and [launch status](../launch/status.md). Initial v0.1/roadmap delivery labels below preserve planning contracts; they do not establish today's implementation or permit deleting subsequently integrated features. [Open-source platform roadmap](../roadmap/platform/README.md) has separate design and gates.

**Delivery: roadmap.** This chapter specifies later behavior; v0.1 does not build this service. Shared safety rules still apply where used by released features.

## Product and vocabulary

MiraBase is PocketBase extended with Turso. One server process can serve one application or consolidate several independent applications. This is a required standalone capability, not a distributed engine or managed hosting control plane. All requirements in this chapter belong to the complete build; phases express dependencies, not release omissions.

An **instance** is a running MiraBase process with one exclusive data root and listener set. An **application** owns its schemas, identities, rules, settings, extensions and files. A **tenant** is a customer boundary: it can mean a school/workspace within one application, or an independent application admitted through the consolidation workflow. A **user** is an authenticated account, not a synonym for tenant. An **agent** is an explicitly bound ordinary application identity. A **database** is one storage resource selected inside an application context. Many users can share one tenant database; one user can have memberships in several tenants.

The dashboard labels the distinction as "Workspace/customer within this application" and "Independent application on this server". It never presents adding an application as inviting another user. Independent applications may have unrelated software, schemas and settings. Importing each source instance as a tenant preserves its application boundary and does not split its users again. Per-user applications may instead run as separate MiraBase instances; no arrangement requires a process per user or database.

## Identifiers and layout

The default application has reserved ID `default`, existing root HTTP routes and `mb_data/data.db`, `auxiliary.db`, `storage/`, keys and application artifacts. Creating it needs no organization, network account or application catalog. Additional application IDs are server-generated 32-character lowercase hex values. Their roots are `mb_data/applications/<applicationId>/`, each containing its own data.db, auxiliary.db, storage, keys, application manifest and selected managed database directories. Display names, hosts and source IDs never become paths.

Adding the first extra application lazily installs `mb_data/instance.db`, with the instance-only tables in storage-format-reference.md. This stores application routing, operator bindings, artifacts and publication intents; it contains no customer account hashes or duplicated collection catalog. Existing per-application lifecycle/partition tables remain in that application's data.db, using storage-format-reference.md. A file-local reference to `primary` means that application's primary database, never another application or instance.db. Physical files are not exposed through HTTP.

Application manifests bind format version, ID, generation, root identity, storage layout, engine dependencies, settings-key reference and artifact digest. Registry state is authoritative; manifests detect wrong or missing files. Paths are canonicalized and checked for traversal/reparse substitution before opening. Unknown data is quarantined rather than replaced with an empty database.

## Context and service ownership

Go owns `server/internal/application.Registry`, `Resolver`, `ContextView`, `ArtifactRegistry` and `Lifecycle`. The internal ApplicationContext contains applicationId, applicationGeneration, applicationPolicyEpoch, routeRevision, settingsRevision and artifactRevision. DatabaseContext embeds it and adds databaseId, generation, schemaRevision, profile, transaction ID and authenticated actor. Internal actors contain applicationId, collectionId and recordId; account equality requires all three. Server operators use a separate operator principal and cannot be manufactured by supplying an application ID.

`Resolver.ResolveApplication(request)` runs before authentication. `Authenticate(applicationContext, request)` reads only that application's auth configuration. `ResolveDatabase(applicationContext, actor, selector)` performs current membership/grant checks and returns a leased immutable context. Helpers, rules, `$app`, hooks, mail, files, callbacks and jobs receive that view. There is no mutable global selected application/database. Explicit instance administration is a different internal capability; omitting a parameter never grants it.

One native provider, listener/router, scheduler, bounded JS pool and worker pool serve the process. A context view is not another bootstrapped PocketBase server. Do not create a BaseApp with a new cron, HTTP server, warm JS pool or unrestricted cache for every database. Metadata caches use application/revision-qualified keys and bounded eviction. Auth/config/schema state is loaded from the selected application and invalidated on its published revision. Cold application artifacts can be reloaded from verified disk records.

## Routes and authority

The default application retains `/api/...`, `/_/`, static assets and existing SDK base URLs. Additional applications use `/a/<applicationId>/api/...`, `/a/<applicationId>/_/` and `/a/<applicationId>/...` for their frontend assets. The SDK base URL includes the `/a/<id>` prefix; collection methods remain unchanged. Instance administration stays at root `/api/mirabase/v1/applications` and `/api/mirabase/v1/application-import-batches` and requires a server operator. Prefixes cannot be used to reinterpret those routes as application administration.

Optional domain aliases are registered by a server operator with an exact normalized host and explicit external URL. Trusted-proxy handling precedes host selection; arbitrary forwarded hosts are ignored. Alias ownership must be unique. Unknown/conflicting host, prefix or explicit selector fails without fallback. Certificate renewal belongs to the shared instance service. Application CORS, redirect allowlists, authentication limits and public URL remain scoped to the selected application. Cookie/auth-store namespaces and cache keys include application identity.

Default bootstrap creates the familiar default-application superuser. That account is the initial server-operator binding only through an explicit durable instance binding when consolidation is first enabled. Extra-application `_superusers` have full application administration but no instance administration. Imported superusers retain their own app authority. Agent bindings and SQL/native credentials never imply instance authority. Listing applications is operator-only; ordinary users do not receive an existence oracle.

The first consolidation operation is authorized by the current default-application superuser, under the instance lifecycle guard. It atomically installs the instance schema, default application row and explicit operator binding before reserving another app. Later operations require that binding. Fresh app setup tokens are 256-bit random secrets, hashed in `_mb_application_bootstrap_tokens`, purpose/application/generation bound and valid for 30 minutes. First-admin creation runs under the app policy guard and commits `_mb_bootstrap_receipts` with the account in the app primary transaction before marking the instance token used. Crash recovery reconciles that receipt and cannot create a second administrator from the token. A used/expired/revoked token never grants login or instance authority. Plaintext tokens are returned once to the authorized creating operator and are never logged or listed.

The additional-app setup form submits `POST /a/<id>/api/mirabase/v1/setup/administrator` with its token, email, password confirmation and expected generation. Apply inherited password/auth forms and require no existing administrator. Receipt identity includes the canonical request hash; a changed replay is rejected. Successful recovery returns only administrator-established status and requires ordinary login. Never issue another bootstrap token after an administrator exists unless the separate documented privileged recovery procedure verifies authority.

New tokens include application identity and purpose. Legacy token preservation is allowed only when that source's key/identity domain remains unambiguous and token verification is confined to its selected application. Preflight detects repeated signing/token key material across sources; preserve-sessions fails with an exact issue, and the operator must select session revocation/rekeying for affected applications. Tokens from one application must fail in another even when IDs and emails match. No automatic cross-application account federation or email merge exists.

SQL console, PG, SQL/HTTP and native replication are explicitly privileged within the selected application/database. Ordinary auth/record tokens cannot acquire those scopes. An application-primary native replica can contain that application's account secrets, so it remains a trusted administrative replica. Record sync bootstrap excludes them. Whole-database access never stands in for record rules.

For native/SQL-HTTP, issue purpose-bound JWT credentials signed with encrypted application-local signing material. The protected header `kid` is `mb/<applicationId>/<credentialId>`; claims bind application/database/generation, purpose, scopes, expiry and policy epoch. The unverified header is only a bounded routing hint before authentication, never proof. Resolve that application, verify the signature/claims and current credential row, then authorize the database. This supports root `/db/<id>/v2/...`, `/pull-updates` and `/db/<id>/v3/...` clients without scanning every application's credential store. An explicit `/a/<appId>` prefix must agree with the credential. PG username uses the same application/credential lookup identity and a separately issued SCRAM secret; startup database is resolved only inside it. Ordinary record JWTs have no native/SQL purpose and are rejected on these paths. Responses do not distinguish unknown from unauthorized application identities.

## Settings, artifacts and files

Each application owns collection/type/index definitions; migration history; auth options and keys; SMTP/templates/sender; public URL/OAuth callbacks; S3 record/backups settings; file policy; rate/security policy; logs and backup schedules. Process-owned settings are listener addresses, native payload, trusted proxy ranges, extraction paths and aggregate budgets. Per-application settings cannot alter these process settings.

Artifact packages are operator-installed, hashed manifests of JS hooks, migrations and public assets plus required Go extension IDs. Analysis/import never executes uploaded code. JS registrations are keyed by application and artifact revision; shared VM leases install only the current application's bindings and clear invocation state on return. Hook ordering stays inherited. Route and cron names collide only within an application. Go extensions are compiled into the executable through an operator-owned registry keyed by extension ID; missing compiled code is a preflight error with a concrete rebuild command from the package manifest, not an automatic binary download. One artifact revision is published only after validation and draining incompatible invocations.

Go/JS extensions remain trusted operator code with their documented capabilities. Application administrators cannot upload executable hooks through record APIs. One process is not a hostile-code sandbox. Shared process failure affects all applications; logical app isolation does not promise independent crash containment.

File identity is `(applicationId, applicationGeneration, databaseId, generation, collectionId, recordId, filename)`. Local paths, S3 prefixes, thumbnail keys, upload intents and protected tokens include that boundary. Each application resolves its own storage credentials. Restore changes its generation and invalidates stale signed references. Deleting one application can clean only its manifest-listed roots/prefixes, never an entire shared bucket.

## Lifecycle and recovery

Application states are `provisioning`, `ready`, `maintenance`, `suspended`, `restoring`, `deleting`, `quarantined`, `tombstoned`, `purged`, `failed`. Creation persists an instance intent, reserves resource budget, initializes an unpublished root, applies schemas, validates operator access/files/integrity, closes handles, writes/fsyncs its manifest, renames into place and then publishes the registry route. Each step has a deterministic job ID. A crash after rename but before routing is reconciled by matching ID, generation and manifest hash; it never publishes an unverified root.

Suspension revokes new admissions, advances the app policy epoch, closes streams within five seconds and drains/cancels current work. Resumption validates manifest, schema and secrets before routing. Maintenance/restore affect the selected application; instance-native upgrade and default-root replacement may require whole-process maintenance. Deletion revokes authority first, stops that application's scheduled work, drains its leases, moves its complete root to managed trash, records the tombstone and purges under the configured retention with explicit pending remote cleanup.

Recover publication journals before listeners become ready. Corrupt instance registry, native ABI or default-application startup stops public readiness. A corrupt additional application is quarantined and reported while other verified applications remain available. Never claim the process remains available after a process/native crash. Unknown commit outcomes use the same receipt/recovery boundary as the existing transaction contract.

Backups can select one database, one complete application or the instance. A complete application archive includes its auth/settings/schema/files/managed DB metadata, artifact manifest and dependency/key references. Importing it to another instance allocates a new application ID and preserves only explicitly mapped operator/grant references. It does not grant an app superuser instance authority. Restores change application generation and all affected database generations; other applications' identities, routes and generations remain unchanged.

For an additional application on Windows, the owned lifecycle service fences and closes all of that application's handles before renaming its roots and publishing the new context. It must not invoke the inherited whole-process live-restore/reexec path for that app. Default-root or instance-native replacement may use the documented stop/restore/start workflow. Generation changes revoke legacy session domains that cannot express application generation before readiness is restored.

Custom field/function registrations are selected by application/artifact/type ID before schema decoding and native preparation. The shared public built-in field factory remains uniform; conflicting custom implementation names cannot overwrite another application's factory. Native extension functions are allowed per selected connection/manifest and cannot be enabled for another application's SQL session by a global registration side effect. Store/cache keys include application and revision.

## Acceptance

MB-APPLICATION, MB-APPAUTH, MB-APPRUNTIME and MB-APPRECOVERY require real three-application fixtures: unrelated schemas with identical collection/record IDs and emails; different SMTP/S3/OAuth settings; application superusers and a server operator; simultaneous hooks/cron/files/streams and every SQL/sync access path. Wrong-app tokens, selectors, cache entries, job payloads and native credentials must fail. Kill at every registry/root publication step, corrupt one extra application, restore/delete another and verify untouched application hashes, routes and permissions. Measure cold/active metadata, JS/native memory and idle eviction; a count of registry rows is not capacity evidence.


## Metadata publication details

Each activation stores the starting commit sequence for every activated target. Any later committed target mutation conservatively forbids snapshot rollback, even if its response was lost. The check is performed under the lifecycle guard against actual target state; an asynchronous instance counter is not authority to discard a commit. Cross-file registry/filesystem recovery follows the journal and never assumes an unrecorded rollback.


## Worked request shapes

The payloads below illustrate the target API. IDs/hashes are demonstration values; live previews use actual discovered IDs/revisions. They are not executed migration or runtime results.

### Fresh application request

```json
{
  "displayName": "School application",
  "layout": "shared",
  "profile": "wal",
  "artifactRef": null,
  "expectedInstanceRevision": "1",
  "adminPolicy": "setup-token"
}
```
