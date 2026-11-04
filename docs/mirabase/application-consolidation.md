# Consolidating PocketBase applications

**Current release authority:** [current implementation inventory](../current-product.md), [user documentation](../user/index.md) and [launch status](../launch/status.md). Initial v0.1/roadmap delivery labels below preserve planning contracts; they do not establish today's implementation or permit deleting subsequently integrated features. [Managed cloud](../cloud/README.md) has separate design and gates.

**Delivery: roadmap.** This chapter specifies later behavior; v0.1 does not build this service. Shared safety rules still apply where used by released features.

## Two source arrangements, one adoption service

The full-data importer must support (1) a shared source split by a reviewed user/tenant/agent ownership plan and (2) multiple independent PocketBase instances preserved as separate tenant/application boundaries inside one MiraBase process. Sources in the second arrangement can be different software. Do not force a common schema or merge their accounts/settings. Schema-only collection import remains a separate inherited operation.

Use the source acquisition, bounded reader, record/file verification and resumable jobs in pocketbase-tenant-import.md. This chapter adds source grouping, application-local authority, append publication and source identity; it does not replace that chapter's safety/verification guarantees. Full adoption target modes are `new-application`, `replace-application`, and `append-application`. New/replace acts on the selected application, not whichever root is convenient. Append creates a new independent application without replacing any existing one. Server operators authorize append/batches; application superusers can import/replace only their own application under the documented guards.

## Batch and source contracts

`server/internal/pbimport.BatchService` creates a batch with activation policy `per-source-after-validation`. A batch is not an atomic transaction across applications. It reports exact total/pending/staging/validated/active/failed counts; a source failure never becomes a silently skipped success. Each source has a stable server-assigned sourceId, operator-supplied origin label, capture timestamp/write policy, source version, archive/snapshot hashes, artifact manifest hash, source object-store reference, settings-key reference, destination application ID and session ID. Source labels are descriptive, never paths or authentication.

Each source still uploads through an independent chunked/hash-verified session. A manifest can reference already uploaded sessions; it cannot supply arbitrary filesystem paths or mutable URLs that bypass source verification. Source credentials are encrypted reference bindings, not plan/report values. Source provenance is indexed in instance.db; identical origin/hash submissions return the existing active/session result unless the operator explicitly selects `create-independent-copy` with a new destination identity. Replacing an imported application requires its application generation, complete verified backup and explicit source lineage.

For a normal append, `pbimport.create` receives the source hash, origin ID and expected instance revision; it allocates source/application IDs and a provisioning registry reservation server-side. The response returns those assigned IDs. A caller-supplied destination ID is accepted only if it matches a server-owned authorized reservation; clients cannot manufacture paths or application identities. `_mb_import_origins` uniquely binds origin/hash/copy key to the session/destination. The default copy key is `original`; explicit independent copies use the durable idempotency request identity as their copy key. Concurrent equal submissions converge on one reservation. A failed unpublished reservation can be retried or retired after journal reconciliation; active origins are never deleted as expired staging.

Batch-create/add-source inputs reference those session IDs with origin/hash/repeat guards. They do not invent source/application IDs. `GET application-import-batches/{batchId}/sources` pages the assigned identities, current session phase and exact error. The server rejects binding one source/destination into competing active batches. Imported sources stay in provisioning without a route until validation/publication succeeds. Standalone append and batch append share the same reservation/publication service.

Batch states are draft, acquiring, planning, executing, complete, partial, failed and cancelled. Per-source states reuse the full import phases. Cancelling a batch stops unpublished sources at safe checkpoints; active sources remain active with their outcomes. Retry only retries selected failed/unpublished sources and rechecks all manifests/revisions. Batch completion requires each selected source active; partial is a distinct terminal result. A retry never republishes an already active source under a new ID by accident.

## Preflight and collision policy

Inventory schemas, collection/field/record IDs, rules, auth methods, settings, migration history, local/S3 files, public assets, hooks, custom Go requirements, external URLs and encryption. Retain original source-local IDs; qualify service/cache/token references by destination application. Source emails and duplicate collection names do not conflict across independent applications and are never evidence of shared identity. Application-local uniqueness stays local. A source that is being split retains the explicit global-within-application reservation policy from the tenant importer.

Source `_superusers` become application superusers. They are never added to instance operator bindings. Verify the destination operator's management access and at least one reviewed application administrator before publication. Session policy is preserve-compatible or revoke-existing. Repeated token keys/signing domains across sources are an error for preservation; the plan must explicitly rekey/revoke the affected source domains. OAuth callbacks and public URLs get an exact old/new map, with operator-confirmed provider updates. Existing URLs are retained through explicitly registered domain aliases or documented base-URL changes, not guessed redirects using record IDs.

Every artifact gets a disposition: keep compatible JS/static/migration artifact; translate a known compatibility alias; rebuild a required compiled Go extension; or report an exact incompatible object with the chosen adaptation and fixture. No executable source runs during inventory. Ordinary application administrators cannot install code. Validate extensions on isolated copies with outbound mail/webhooks/cron disabled before enabling them. Per-app routes, caches and cron names must bind to their application; operator-installed code remains trusted and is not a security sandbox.

The directly supported source baseline is PocketBase v0.41.1 at the recorded commit. An older source is never relabeled as that baseline. The upgrade procedure is: retain an immutable backup; create an isolated copy; open it first with its matching upstream binary; follow official release-specific migrations and extension changes to the pinned baseline; disable outbound side effects; verify auth/files/schema/migration history and source API fixtures; capture a new coherent baseline snapshot; then import. The source-version report lists required steps and stops publication until the baseline reader accepts it. This is an explicit compatibility boundary, not a promise of tested imports from every historical release. Build acceptance includes the pinned source plus at least one documented older-source upgrade fixture selected from upstream-supported transitions.

## Stage, validate and publish

Append stages a complete application root in an import-owned sibling directory. Its auth/settings/data/auxiliary/files/managed contexts and artifact manifest belong to that application. It uses no active default-app tables for source accounts. Mapping and validation ledgers live on bounded disk storage. Verify exact per-collection canonical values, IDs, files/hashes, rules, auth flows and source-specific request fixtures. A report includes every intended difference and zero unexplained rows/files/permissions.

Before append publication, freeze staging and recheck source, plan, preview, report, artifact and staged-manifest hashes; reserved destination ID; instance registry revision; disk reservation; and operator/app access. Persist a publication intent outside candidate data, close stage handles, fsync the manifest/root, atomically rename into `applications/<id>` and then commit the route/ready registry row. The process-local lifecycle guard prevents competing publication/deletion of that ID. The journal records both filesystem and registry phases; no cross-file atomic rename claim is made.

Crash before route activation leaves existing apps serving and the candidate unpublished. Recovery either completes a matching fully verified candidate or removes only job-owned unpublished staging. After activation, finish reconciliation of the new state. App generation/accepted-write markers determine rollback eligibility. Once any new application write is acknowledged, recovery preserves that deployment and produces a verified current backup plus typed change/file export before any explicit reconciliation. It must not restore the old source over accepted writes. Each source remains stopped/read-only until its endpoint cutover or is explicitly declared a historical snapshot.

Replacing the default application's root follows the full-root external journal procedure in pocketbase-tenant-import.md and can require process maintenance. Replacing an additional app fences only its routes/workers/leases, stages a new generation and retains its previous verified root. Other applications' data, keys, settings, generations and routes must remain unchanged. Source objects are read-only; retries do not mutate the original source.

When replacing the default application in a consolidated instance, the reviewed account policy must retain the verified bound operator or explicitly map the operator binding to a proven destination superuser. Publish that binding as a guarded instance-journal step before accepting traffic. Preflight tests both application login and instance administration under the candidate state. An extra application's source superuser is never eligible for an implicit operator mapping. Failure to prove the selected default operator blocks publication rather than leaving a stranded binding.

## UI, API and acceptance

The existing import area offers schema only, full shared adoption, split shared source, and consolidate separate instances. The latter accepts a paginated source list with per-source artifacts/keys, target identity, compatibility issues, account/session policy, URL map, resource preview, validation and cutover results. Source statuses remain visible after partial success. Jobs and reports use the same HTTP/CLI/SDK service. Artifact IDs resolve within their authorized session; no raw path is accepted.

MB-CONSOLIDATE and MB-APPEND require three distinct PocketBase sources with colliding IDs/emails but different schemas, auth/settings, files and trusted extensions. Import into a server already serving an application; verify each new boundary and byte hashes of the existing application. Interrupt each source phase and publication cut point, retry identical/different hashes, repeat an active source, replace one source, deny imported-superuser instance access and verify post-write rollback refusal. UI batch partial/failed states must correspond to actual destination contents. A list of planned DBs is not acceptance.


## Worked request shapes

The payloads below illustrate the target API. IDs/hashes are demonstration values; live previews use actual discovered IDs/revisions. They are not executed migration or runtime results.

### Independent-source batch request

```json
{
  "expectedInstanceRevision": "1",
  "activationPolicy": "per-source-after-validation",
  "sources": [
    {
      "sessionId": "import-school",
      "originId": "original-school",
      "sourceHash": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
      "repeatPolicy": "resume-existing"
    },
    {
      "sessionId": "import-shop",
      "originId": "original-shop",
      "sourceHash": "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb",
      "repeatPolicy": "resume-existing"
    }
  ]
}
```

### Append source request

```json
{
  "sourceKind": "pocketbase-backup",
  "sourceBytes": 1024,
  "sourceSha256": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
  "targetMode": "append-application",
  "expectedPrimaryGeneration": "1",
  "sourceLabel": "School",
  "expectedInstanceRevision": "1",
  "originId": "school-source"
}
```
