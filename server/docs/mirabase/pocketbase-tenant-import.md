# PocketBase import into tenant databases

**Current release authority:** [current implementation inventory](../current-product.md), [user documentation](../user/index.md) and [launch status](../launch/status.md). Initial v0.1/roadmap delivery labels below preserve planning contracts; they do not establish today's implementation or permit deleting subsequently integrated features. [Open-source platform roadmap](../roadmap/platform/README.md) has separate design and gates.

**Delivery: v0.1 and roadmap.** [v0.1-build.md](v0.1-build.md) selects the current portion; [roadsmap.md](roadsmap.md) retains the remainder. Detailed schemas/configuration/ABI and full-family acceptance below describe the target superset unless narrowed by that allocation.

## Required outcome and scope

PocketBase users can bring an existing application into MiraBase and convert its shared data into a database for each tenant. This is a core import requirement. It includes records, files, identities, memberships, rules, relationships and routing, not only a collection JSON upload. The application still runs in one MiraBase server and is administered through its existing dashboard.

A tenant may be a company, organization, workspace, account, user or agent. Tenant identity and authenticated user identity are separate: one user may belong to several workspaces, and one workspace may have many users. Do not implement this as one database for every login unless that is the selected mapping.

Other-BaaS provider adapters remain deferred. This chapter covers the pinned PocketBase baseline, the inherited schema importer, and its full-application adoption/conversion paths. It does not silently add Firebase/Supabase/Appwrite adapters.

## Three choices in the existing import area

| Choice | Input | Result |
|---|---|---|
| Collections/schema only | PocketBase-compatible collection JSON | Existing collection/field/index/rule import, extended for MiraBase types; no records/files transferred |
| Full application, shared layout | Coherent PocketBase backup or stopped data-directory snapshot, plus referenced remote files | Adopt the application while retaining its shared-database layout |
| Full application, tenant layout | Same full source plus a reviewed ownership/access plan | One primary/shared application context and separate tenant data databases |

The ordinary PUT /api/collections/import and its merge/replacement semantics remain. Full import is an additive guided workflow, not a hidden change to what that endpoint accepts. Uploading schema JSON cannot be reported as a completed full-data migration.

Full adoption targets an empty/new application, an explicitly selected replacement of the destination application, or append-application adoption that creates another isolated application in a populated instance. It does not silently merge unrelated accounts, system collections and IDs from two existing applications. Replacement requires the destination generation and a verified destination backup. Existing destination state remains available for recovery.

## Source acquisition and immutability

Supported sources are a PocketBase backup ZIP and a snapshot of its stopped data directory. The local CLI packages directory input through the same archive pipeline. Never read a changing directory and call it a coherent snapshot. The operator places the source application in maintenance/stops writes before capture and keeps it stopped/read-only until cutover, or explicitly imports a historical snapshot without claiming that later source writes were transferred.

The server proves the uploaded bytes and snapshot consistency it can inspect; it cannot prove that a remote source server remained stopped. The wizard records the operator's source-write attestation and capture time. There is no implied live source CDC connector or zero-downtime guarantee.

Stream resumable uploads in chunks of at most 8 MiB into an owner-private session directory. Validate Content-Range, chunk boundaries and SHA-256. Repeated identical chunks are safe; a different chunk at the same offset is a conflict. Only an exact length/hash match finalizes the source. Body size and total staging disk are bounded; never load a multi-gigabyte backup into RAM.

Reject ZIP traversal, absolute paths, duplicate entries, symlinks/reparse points, decompression bombs and unknown encrypted archives. Record original archive hash and normalized snapshot hash separately. If SQLite recovery is needed, recover a private copy, never the original. Inspect PocketBase data with the reference SQLite reader used only by import/adoption tooling; it is not a fallback engine for live MiraBase databases.

Inventory data.db, auxiliary.db, migration history, collection/field/index/view definitions, rules, auth collections, IDs, settings encryption, local file references and source storage configuration. Uploaded S3 record files are not automatically inside a PocketBase backup. Require a configured read-only source object-store reference or an accompanying object manifest/content export. Secret values never enter plans or reports.

Hooks/migrations/public assets are separate application artifacts when absent from the backup. Detect required missing artifacts. Imported code is not automatically executed during analysis. Test compatibility in isolated copies with outbound delivery disabled before enabling those artifacts.

## Plan and tenant identity

The immutable plan binds source fingerprint, normalized snapshot, destination application generation, source version, target layout/profile, template revision, key/storage references and every routing/schema/access decision. Hash canonical plan bytes. Editing a mapping creates a new plan revision/hash and invalidates older previews, staging and validation; never reuse an obsolete approval.

A tenant source is either a collection of tenant records, or distinct typed values in a declared collection field. Preserve type in a tenant key: numeric 1 and string "1" are different. The canonical key is JSON over namespace, source collection/field and value/record ID; SHA-256 indexes it. The opaque generated database ID is separate and never derived into a path from a tenant name or email.

Example: organizations define tenants; projects.organization selects a tenant; tasks.project.organization derives the same tenant through a parent. Existing IDs stay stable. Renaming an organization does not rename its database or change the tenant identity. Missing/null keys produce specific unresolved-row issues; no "default tenant" is invented.

## Collection placement and ownership graph

Every source collection receives one declared placement:

- Shared: one canonical collection in the primary database. Authentication, membership and application settings normally stay here.
- Tenant: records move to their derived tenant database, retaining collection/field/record IDs.
- Reference copy: selected nonsecret shared fields are copied read-only into each applicable context where explicitly required. The primary remains authoritative; updates have a defined refresh path.
- Derived: views/materialized views are recreated from a validated query and dependency plan rather than copying stale result rows.

Ownership is direct from a tenant field/root record, or follows declared relation-field IDs to a tenant root. Check field IDs/types against the source schema. Do not infer ownership from a field name such as userId. Scalar paths yield exactly one tenant per private record. Multi-valued paths collect tenant candidates; conflicting candidates are a resolution issue.

Build an indexed mapping ledger on disk, keyed by source collection/record, instead of loading all data into memory. Traverse the relationship graph with bounded batches. Collapse strongly connected relation components and propagate root ownership to a fixed point. A cycle without a tenant root is explicitly unassigned. Record the provenance path for every assignment.

The preview lists orphaned references, null owners, conflicting paths, multiple-tenant records, unknown relations, cross-tenant links and global dependencies. Resolutions are part of the plan: correct the root/path, choose a stated tenant for named exceptions, or classify the relevant data as shared/reference. No silent deletion, arbitrary first-owner choice or mutable duplication is allowed.

For user/agent layouts, tenant roots can reference auth identities/bindings and reuse the existing owner policy. Workspace tenant bindings use kind=tenant and do not pretend an organization record is an auth record. Explicit grants and source membership policy remain distinct.

## Relationships, queries and rules

Relations within one tenant retain their existing IDs and relation semantics. A private-to-shared relation receives declared global resolution metadata; the primary record remains canonical. Native private replicas receive no unrelated global identities/secrets.

A cross-private relation must be resolved in the plan by shared placement or the explicitly supported context-qualified relation/grant adapter. Do not convert an ordinary relation into an unrestricted cross-database join. Cross-context cascades are planned workflows, not claimed cross-file transactions.

Preserve the original rule text and generate the adapted expression plus a report. Translate through the existing rule/filter AST and context-aware resolver. Preserve @request.auth, request/body/query semantics, hidden fields and source membership conditions. Context isolation adds a database boundary; an empty source rule is not permission to ignore that boundary.

Audit list/filter/sort/expand, views, index expressions, custom routes, Go/JS hooks, raw SQL, batch transactions, uniqueness and cascades. Keep tenant-eligible operations in one context. A uniqueness constraint that was global remains global only through an explicit shared registry/reservation implementation; it must not silently become per-tenant. Alternatively, the operator can explicitly accept per-tenant uniqueness in the reviewed plan.

Generate concrete compatibility actions and affected endpoints. For example, a multi-workspace list needs an active workspace selector; an application-wide reporting view needs a declared shared aggregate/reporting workflow. Do not promise that every arbitrary custom SQL join or hook is automatically equivalent after splitting files.

All required actions have selected adaptations or explicit plan corrections before readiness. A warning alone does not satisfy a required behavior. The application contract cannot be silently weakened because a rule or relation was inconvenient.

## Preserving global uniqueness

For preserve-global indexes on partitioned collections, use _mb_unique_reservations in the primary context. Derive the comparison key using the source index expression, predicate and collation; store the hash and canonical key bytes, not a guessed lowercase representation. Shared collections keep their ordinary native unique indexes.

The shared transaction coordinator reserves a private write's affected keys before its local commit. Reserved/committed/releasing entries all exclude competing writers. On local commit, finalize against the matching target transaction tag; on rollback, release only after proving that outcome. A process crash leaves the reservation until recovery checks the target change/receipt state. Lease expiry alone never proves an unknown transaction did not commit. Deletes release keys only after their local commit. This is a conservative recoverable reservation protocol, not a cross-file ACID claim.

Sort multi-key reservations deterministically, preserve NULL/partial-index semantics and keep the primary reservation transaction short. Never call back into a private transaction while holding the primary writer. Busy/cancel/conflict behavior follows the existing safe-retry policy. The record/private commit and primary reservation remain separately recoverable steps; unknown outcomes retain exclusion until reconciled.

All supported write paths use this coordinator, including native/SQL/PG channels. An index whose equality semantics cannot be preserved is an explicit plan adaptation error until its adapter is supplied or the operator reviews a per-tenant policy. Do not silently remove a source unique constraint to complete an import.

## Identities, memberships and permissions

Preserve auth collection/record IDs, compatible bcrypt hashes, verification state and configured login methods. Passwords do not need to be recovered as plaintext or reset merely because records move. Decrypt source settings with the provided key reference and re-protect destination secrets. OAuth credentials/redirect URLs and public application URL require explicit compatibility checks.

Default session policy preserves compatible source token keys; an operator can select revoke-existing and see exactly which sessions/reset/verification links will be invalidated. No token/key/password is displayed in the report. Destination import authority and imported source superuser access are validated before cutover so the operator cannot lock themselves out.

Keep one central auth identity within the same application when a user belongs to multiple workspace tenants. Independent applications keep separate accounts even if emails match. Adapt the source membership relation/predicate into the tenant resolver. This is not a mandatory new role system: source application role fields and rule semantics remain. Explicit privileged SQL/native-sync scopes are never automatically granted to every imported member.

Membership plans select an existing tenant record's list/view rule, a membership collection with subject/tenant fields, or a tenant owner relation. Relation metadata supplies the auth collection ID as well as the record ID. Anonymous tenant reads default off; a public source rule does not override that silently. Any selected public-access behavior or intentional tightening is explicit in the plan and permission fixtures.

Reviewed membership can derive only record.read/write, files.read/write, changes.read and record.sync channel scopes. Ordinary collection/field rules still apply. It never derives sql.read/write, native.sync, database.inspect or schema/lifecycle authority. Shared tenant metadata also receives the declared active-tenant filter where required; copying accounts into the primary does not make them publicly readable.

Account policy separately selects compatible-token preservation or session revocation and chooses operator access. retain-destination-superuser keeps only the explicitly identified bootstrap operator in addition to source accounts; that declared extra row is counted separately. Source IDs never change to resolve a destination operator collision: verify an existing source superuser or select an explicit unique destination operator identity. use-source-superuser requires proving access before publication. No unrelated account merge or privilege elevation is inferred.

Run request fixtures against isolated source and target copies: unauthenticated, member, nonmember, multi-tenant member, removed member, agent and superuser. Compare allow/deny decisions, visible fields, protected file access, relation expansion and response shape. Every intended difference is listed and approved in the plan; unexplained differences fail validation. Revocation must use current membership, not a stale import-time allowlist.

## Records and files

Use stable source keys and preassigned destination database IDs. Copy in bounded, restartable batches into unpublished contexts. Preserve field values, relation IDs, timestamps and schema IDs; validate exact encoding and provider conversion. The import service uses a trusted migration path with invariant checking, not ordinary create handlers that would regenerate IDs or send application notifications.

Suppress ordinary record hooks, cron, mail, webhooks, auth alerts and realtime during bulk copy. Copying historical data must not trigger a second order/payment/email workflow. After cutover emit a bounded import-completed/schema-reset signal and require affected replicas to bootstrap; do not flood clients with historical create events.

Copy each referenced local/remote file to the correct destination context namespace. Verify byte length and SHA-256; preserve stored filename, record association, protection and configured thumbnail behavior. Thumbnails may be regenerated from verified originals. Shared file records stay shared. Explicit reference copies must maintain their authorized field projection.

All referenced originals must be available and verified before cutover. Missing remote objects remain visible issues; an inaccessible bucket is not reported as a complete import. Unreferenced source files are inventoried/quarantined or retained in the recovery copy, never silently assigned to a tenant. Protected files do not become public; existing public file behavior is reported rather than silently reinterpreted.

The map/reconciliation ledger records each completed collection/record/file batch and result hash. A resumed job checks source, plan, schema and target IDs before continuing. Upsert is permitted only for this session's unpublished exact-ID targets. Existing unrelated destination data is never overwritten by a retry.

## Preview and validation gates

Preview is computed from actual source rows/files, not schema guesses. It shows per-tenant record/file counts and bytes, shared data, auth/membership treatment, every ownership issue, adapted rules/views/hooks, routing changes, target resources, required source keys/files and destructive effects on an explicitly replaced destination. Paginate large inventories.

Validation then checks the staged result:

1. Every source record has exactly its declared placement or documented reference-copy mapping; no unexplained dropped/extra row exists.
2. Counts and canonical value hashes reconcile per collection and tenant, not merely in aggregate.
3. IDs, relations, uniqueness, indexes, views and native integrity match the plan.
4. File hashes/protection/reference counts and remote-file coverage are complete.
5. Login/account/membership/rule fixtures pass, including cross-tenant negatives on HTTP, files, SSE, search and record sync.
6. Endpoint/SDK/routing and required hook/query adaptations are verified; source/target side effects remain disabled in validation.
7. Disk/admission/native profile checks and recovery backup checks pass; operator access survives.
8. All relevant source/plan/session/destination revisions are unchanged.

The report contains exact counts, hashes, issue IDs/resolutions, results and retained artifacts. A reviewed preview is not a validation pass. An empty fixture set, skipped rule case or staged database that has never been opened is not verification. Cutover requires the exact successful report hash and zero unresolved error-level issues.

Validated staging is immutable to app workers. Immediately before publication, verify the stored report's staged manifest against the actual staged root and recheck all guards. Any changed bytes, plan or target generation invalidates readiness and requires verification again.

## Routing and application changes

A route namespace has an active epoch. Bindings map namespace plus canonical tenant key and template to a database ID/generation. Prepare the candidate epoch in bounded batches; it stays invisible until the final active pointer publishes. No organization name becomes a physical path.

The resolver accepts an authorized active tenant selector, including a mapped existing workspace selector where configured. With one eligible tenant it can derive the default. With several and no selector, return tenant_selection_required for private collections; never choose the first tenant or query every database. A tenant selector selects a resource, not proof of membership.

X-MiraBase-Tenant carries namespace plus a base64url-encoded canonical typed tenant key, separated by a colon. The SDK's per-request tenant option generates it. A mapped existing selector is translated to that same typed key and checked against current membership. Conflicting tenant/database selectors fail before data access. Query/file/SSE tokens carry the resolved context rather than a client-controlled path.

Global auth/shared operations continue against the primary context with the applicable actor/tenant policy. Existing collection CRUD methods and neutral API paths stay. The preview identifies the minimal frontend/SDK selector changes instead of guaranteeing zero edits to every application.

Files, SSE, query cursors and sync sessions bind to the resolved context/generation/epoch. New tokens are issued where the old token lacks those claims. No ambiguous legacy URL redirects to another tenant based solely on record ID. Produce an authorized old/new file URL map where needed.

## Publication and recoverable cutover

Import state is persisted in _mb_import_sessions with immutable manifest references, plus ordinary durable jobs. Phases are awaiting-source → source-ready → analyzing → draft → previewing → previewed → staging → staged → validating → ready-for-cutover → cutting-over → active. Errors preserve the failed phase and checkpoints; cancel is allowed only before publication. Recovery/rollback states are explicit.

Full adoption creates a staged application root containing the converted primary/auxiliary data, tenant contexts, route/grant/schema metadata and files. Stop affected target admissions/workers, drain leases and recheck the expected target generation. Retain a verified old target backup/root. Source PocketBase data remains untouched.

Publish with an owner-private cutover journal outside the old/new data roots. Record verified paths/digests, fsync intent, rename the old target to its recovery location, rename the staged root into the configured target location and verify startup recovery checks while readiness remains false. Durably mark active, then publish readiness/accept writes. There is no claim of one filesystem transaction across two renames. API maintenance and startup recovery prevent clients from observing an incomplete switch.

Crash before the active marker permits restoring the old root or completing the verified candidate according to the journal, with no accepted target writes. After the active marker, recovery completes the new state; it never silently restores old data over acknowledged writes. Do not restart the old PocketBase source against the same frontend traffic while the target accepts writes.

Before any post-cutover application write, rollback can restore the retained previous target. After writes occur, automatic reverse migration into the old PocketBase snapshot is not claimed. The recovery action first quiesces and creates a verified current-target snapshot plus complete typed change/file export. It retains the new deployment as authoritative until an explicit restore/reconciliation plan accounts for every accepted write. No rollback endpoint discards them to manufacture success.

Cancellation/cleanup only removes job-owned unpublished targets. Recovery/source archives default to seven-day retention after the session reaches a safe terminal state; active data is never staging garbage. Report the deadline and verified recovery artifacts. Failed publication cannot expire itself into unrecoverable deletion.

## Persistence, interfaces and acceptance

The optional metadata tables _mb_import_sessions, _mb_tenant_routes, _mb_tenant_bindings and _mb_unique_reservations are defined in storage-format-reference.md. Tenant routing does not add a process, scheduler or JS runtime per tenant. The disk mapping ledger is session-owned temporary data, not an unbounded resident map.

Default import limits are 20 GiB source archive, 8 MiB upload chunks, two active sessions and copy batches of 1000 rows/4 MiB. These are configurable resource limits; referenced object-store bytes and staging/recovery disk are separately reserved. A large row uses its declared field/streaming limits and a smaller batch rather than unbounded buffering. The source reader uses one bounded connection. Full import is enabled as an administrative workflow; optional tenant metadata is installed only when selected.

Stopping a phase job at a safe checkpoint returns the session to its last stable phase; cancelling the whole import is terminal and never applies to active publication. Failed publication resumes from its journal. Failed pre-publication plans may be edited only after active work stops and only if no active cutover marker exists.

Versioned import APIs, schemas, transitions, guards and acceptance are in pocketbase-tenant-import.md, http-operation-reference.md and wire-schema-reference.md. The dashboard integrates this workflow into Settings → Import collections with a full-application import choice and resumable job pages. API/CLI/Go/SDK actions use the same service, including plan/review/report hashes.

GET /api/mirabase/v1/pocketbase-imports/{importId}/artifacts/{artifactId} streams a manifest-listed preview, mapping, compatibility, validation or recovery artifact. IDs resolve only within that authorized session; no client path is accepted. Verify stored hash, supply sanitized download name/type, exclude key material and return gone after declared expiry. Reports/maps are paginated or streamed, never loaded wholesale into the dashboard.

Eight required acceptance families cover source handling, ownership planning, identity/rule adaptation, records/files, preview/reconciliation, cutover/routing, recovery/resume and the dashboard. Real source/target fixtures must include indirect ownership, cycles, shared references, multiple memberships, ambiguous rows, same IDs across auth collections, protected/public files, S3 gaps, schema/query/hook incompatibilities, malicious archives and crashes at every publication step.

This chapter defines the implementation. Neither a generated plan nor these documents constitute an executed migration or a release-qualified importer.



## Independent source applications

Consolidating several PocketBase sources, even different software, preserves one application boundary per source. [Application consolidation](application-consolidation.md) defines batch/source provenance, append-application mode, local admin/session/settings/schema behavior, endpoint changes and per-source publication/recovery. Whole-root replacement in this chapter applies only to the selected replaced application. Append never swaps the existing instance/default application root. It stages and publishes a new isolated root while other applications continue serving. The acquisition and reconciliation guarantees above apply to every source.


## Worked request shapes

The payloads below illustrate the target API. IDs/hashes are demonstration values; live previews use actual discovered IDs/revisions. They are not executed migration or runtime results.

### Tenant ownership plan

```json
{
  "expectedRevision": "1",
  "sourceHash": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
  "snapshotHash": "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb",
  "layout": "tenant",
  "namespace": "workspaces",
  "tenantKind": "tenant",
  "tenantSource": {
    "kind": "records",
    "collectionId": "organizations"
  },
  "placements": [
    {
      "collectionId": "organizations",
      "placement": "shared",
      "ownership": {
        "kind": "shared"
      },
      "referenceFieldIds": [],
      "uniquePolicy": "preserve-global",
      "adaptationIds": []
    },
    {
      "collectionId": "users",
      "placement": "shared",
      "ownership": {
        "kind": "shared"
      },
      "referenceFieldIds": [],
      "uniquePolicy": "preserve-global",
      "adaptationIds": []
    },
    {
      "collectionId": "memberships",
      "placement": "shared",
      "ownership": {
        "kind": "shared"
      },
      "referenceFieldIds": [],
      "uniquePolicy": "preserve-global",
      "adaptationIds": []
    },
    {
      "collectionId": "projects",
      "placement": "tenant",
      "ownership": {
        "kind": "field",
        "fieldId": "fld_organization"
      },
      "referenceFieldIds": [],
      "uniquePolicy": "preserve-global",
      "adaptationIds": [
        "global-org-relation"
      ]
    },
    {
      "collectionId": "tasks",
      "placement": "tenant",
      "ownership": {
        "kind": "relation-path",
        "fieldIds": [
          "fld_project",
          "fld_organization"
        ]
      },
      "referenceFieldIds": [],
      "uniquePolicy": "preserve-global",
      "adaptationIds": []
    },
    {
      "collectionId": "documents",
      "placement": "tenant",
      "ownership": {
        "kind": "field",
        "fieldId": "fld_organization"
      },
      "referenceFieldIds": [],
      "uniquePolicy": "preserve-global",
      "adaptationIds": [
        "global-org-relation"
      ]
    }
  ],
  "accountPolicy": {
    "sessionPolicy": "preserve-if-compatible",
    "operatorPolicy": "retain-destination-superuser"
  },
  "access": {
    "membership": {
      "kind": "membership-collection",
      "collectionId": "memberships",
      "subjectRelationFieldId": "fld_user",
      "tenantFieldId": "fld_organization"
    },
    "membershipScopes": [
      "record.read",
      "record.write",
      "files.read",
      "files.write",
      "changes.read",
      "record.sync"
    ],
    "allowAnonymousTenantRead": false,
    "ruleAdaptationIds": []
  },
  "selector": {
    "kind": "mirabase-header",
    "source": null
  },
  "profile": "wal",
  "sourceFileStoreRef": null,
  "settingsKeyRef": null,
  "resolutions": [],
  "adaptations": [
    {
      "id": "global-org-relation",
      "kind": "global-relation",
      "sourceObjectId": "organizations",
      "actionId": "relation.global-reference",
      "testFixtureIds": [
        "member-org-expand",
        "nonmember-org-expand-denied"
      ],
      "expectedDifference": null
    }
  ],
  "sideEffects": "suppressed-during-import"
}
```

### Shared adoption plan

```json
{
  "expectedRevision": "1",
  "sourceHash": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
  "snapshotHash": "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb",
  "layout": "shared",
  "namespace": "workspaces",
  "tenantKind": null,
  "tenantSource": null,
  "placements": [
    {
      "collectionId": "organizations",
      "placement": "shared",
      "ownership": {
        "kind": "shared"
      },
      "referenceFieldIds": [],
      "uniquePolicy": "preserve-global",
      "adaptationIds": []
    },
    {
      "collectionId": "users",
      "placement": "shared",
      "ownership": {
        "kind": "shared"
      },
      "referenceFieldIds": [],
      "uniquePolicy": "preserve-global",
      "adaptationIds": []
    },
    {
      "collectionId": "memberships",
      "placement": "shared",
      "ownership": {
        "kind": "shared"
      },
      "referenceFieldIds": [],
      "uniquePolicy": "preserve-global",
      "adaptationIds": []
    },
    {
      "collectionId": "projects",
      "placement": "shared",
      "ownership": {
        "kind": "shared"
      },
      "referenceFieldIds": [],
      "uniquePolicy": "preserve-global",
      "adaptationIds": []
    },
    {
      "collectionId": "tasks",
      "placement": "shared",
      "ownership": {
        "kind": "shared"
      },
      "referenceFieldIds": [],
      "uniquePolicy": "preserve-global",
      "adaptationIds": []
    },
    {
      "collectionId": "documents",
      "placement": "shared",
      "ownership": {
        "kind": "shared"
      },
      "referenceFieldIds": [],
      "uniquePolicy": "preserve-global",
      "adaptationIds": []
    }
  ],
  "accountPolicy": {
    "sessionPolicy": "preserve-if-compatible",
    "operatorPolicy": "retain-destination-superuser"
  },
  "access": null,
  "selector": {
    "kind": "none",
    "source": null
  },
  "profile": "wal",
  "sourceFileStoreRef": null,
  "settingsKeyRef": null,
  "resolutions": [],
  "adaptations": [],
  "sideEffects": "suppressed-during-import"
}
```


## Import state and publication rules

```json
{
  "formatVersion": 1,
  "owner": "pocketbase-tenant-import.md",
  "scope": {
    "current": [
      "schema-only PocketBase import",
      "full PocketBase shared adoption",
      "full PocketBase tenant/user/agent conversion",
      "multiple independent PocketBase applications consolidated in one process",
      "append-application import into populated instances",
      "source provenance and resumable per-source batch publication"
    ],
    "deferred": [
      "Firebase",
      "Supabase",
      "Appwrite",
      "other-BaaS adapters"
    ],
    "noImplicitPromise": [
      "live source CDC",
      "zero-downtime migration",
      "arbitrary post-write automatic reverse migration"
    ]
  },
  "requirements": [
    "MB-PBIMPORT",
    "MB-TENANTPLAN",
    "MB-TENANTACCESS",
    "MB-TENANTDATA",
    "MB-TENANTCHECK",
    "MB-TENANTCUTOVER",
    "MB-TENANTRECOVERY",
    "MB-TENANTUI"
  ],
  "phases": [
    "awaiting-source",
    "source-ready",
    "analyzing",
    "draft",
    "previewing",
    "previewed",
    "staging",
    "staged",
    "validating",
    "ready-for-cutover",
    "cutting-over",
    "active",
    "failed",
    "cancelled",
    "rolling-back",
    "rolled-back",
    "recovery-exported",
    "retired"
  ],
  "service": "server/internal/pbimport.Service; shared by dashboard/HTTP/CLI/Go/SDK",
  "metadata": [
    "storage-format-reference.md:_mb_import_sessions",
    "storage-format-reference.md:_mb_tenant_routes",
    "storage-format-reference.md:_mb_tenant_bindings",
    "storage-format-reference.md:_mb_unique_reservations"
  ],
  "sources": [
    "coherent PocketBase backup ZIP",
    "stopped data-directory snapshot packaged through the same pipeline"
  ],
  "preconditions": [
    "superuser and expected target generation",
    "immutable original/normalized source fingerprints",
    "explicit source storage/settings-key references",
    "empty target or explicit verified replacement backup"
  ],
  "planRules": [
    "every collection has placement",
    "every private row has one tenant with provenance",
    "typed keys retain type",
    "relation cycles use bounded fixed-point resolution",
    "global uniqueness is explicitly retained or reviewed as per-tenant",
    "current memberships authorize routing",
    "no unlisted row/file drops or historical side effects"
  ],
  "cutoverGuards": [
    "sourceHash",
    "planHash",
    "previewHash",
    "reportHash",
    "expectedRevision",
    "expectedPrimaryGeneration",
    "stagedManifestHash matches immutable validated target",
    "passed report with zero unresolved errors",
    "complete row/file accounting",
    "verified operator access",
    "source-write-policy attestation",
    "recovery target backup for replacement"
  ],
  "publication": {
    "protocol": "maintenance, drain, durable external journal, old-root rename, staged-root rename, readiness verification, active marker, accept writes",
    "beforeMarker": "restore old or complete verified candidate; no target writes accepted",
    "afterMarker": "complete new state; never overwrite accepted writes with old snapshot",
    "append": "Stage a complete independent root, validate all hashes/authority, persist intent, rename, publish instance route; recover without altering other apps."
  },
  "recovery": {
    "preWriteRollback": "verified previous target root may be restored",
    "postWriteAction": "quiesce, verified current snapshot/change/file export, keep new state authoritative until explicit complete reconciliation",
    "retentionDays": 7
  },
  "semanticNegatives": [
    "schema-only input cannot stage full migration",
    "unknown/ambiguous tenant ownership cannot publish",
    "missing referenced file cannot be counted complete",
    "stale plan/report cannot cut over",
    "native/SQL scopes cannot arise from tenant membership alone",
    "post-write rollback cannot discard accepted records/files"
  ],
  "fixtures": [
    {
      "id": "PBI-SCHEMA-ONLY",
      "requirement": "MB-PBIMPORT",
      "negative": "A collection JSON file is rejected as a full-data source"
    },
    {
      "id": "PBI-INDIRECT",
      "requirement": "MB-TENANTPLAN",
      "positive": "tasks.project.organization derives the same tenant as its parent"
    },
    {
      "id": "PBI-CYCLE",
      "requirement": "MB-TENANTPLAN",
      "negative": "A rootless cycle remains unassigned; conflicting roots cannot publish"
    },
    {
      "id": "PBI-MEMBERSHIP",
      "requirement": "MB-TENANTACCESS",
      "positive": "One account keeps login and accesses two declared workspaces only"
    },
    {
      "id": "PBI-CROSS-TENANT",
      "requirement": "MB-TENANTACCESS",
      "negative": "Forged selector/ID cannot expose another tenant record, file or subscription"
    },
    {
      "id": "PBI-GLOBAL-UNIQUE",
      "requirement": "MB-TENANTPLAN",
      "negative": "Two databases cannot reserve the same preserved-global key; expired uncertain leases are reconciled, not freed"
    },
    {
      "id": "PBI-REMOTE-FILE",
      "requirement": "MB-TENANTDATA",
      "negative": "A source S3 object absent from archive and source store prevents readiness"
    },
    {
      "id": "PBI-COPY-RESTART",
      "requirement": "MB-TENANTDATA",
      "positive": "Killed batches resume to identical rows/IDs/files without notifications"
    },
    {
      "id": "PBI-STALE-PLAN",
      "requirement": "MB-TENANTCHECK",
      "negative": "Editing ownership invalidates previous preview/report hashes"
    },
    {
      "id": "PBI-COUNT-HASH",
      "requirement": "MB-TENANTCHECK",
      "negative": "Equal totals cannot hide swapped/misassigned rows; per-tenant hashes must reconcile"
    },
    {
      "id": "PBI-ROUTE-EPOCH",
      "requirement": "MB-TENANTCUTOVER",
      "positive": "Only the published epoch resolves; multiple memberships require selection"
    },
    {
      "id": "PBI-CUTOVER-CRASH",
      "requirement": "MB-TENANTRECOVERY",
      "positive": "Crash before/after every rename/marker recovers to a complete authoritative root"
    },
    {
      "id": "PBI-POST-WRITE",
      "requirement": "MB-TENANTRECOVERY",
      "negative": "New accepted writes forbid blind old-root restoration and remain in verified recovery artifacts"
    },
    {
      "id": "PBI-WIZARD",
      "requirement": "MB-TENANTUI",
      "positive": "All three choices and real preview/validation/job states work through the existing dashboard"
    }
  ],
  "applicationAuthority": "Instance operators authorize append/batches; application superusers cannot become instance operators through imported accounts."
}
```
