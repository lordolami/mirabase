# Files, storage and email

**Current release authority:** [current implementation inventory](../current-product.md), [user documentation](../user/index.md) and [launch status](../launch/status.md). Initial v0.1/roadmap delivery labels below preserve planning contracts; they do not establish today's implementation or permit deleting subsequently integrated features. [Managed cloud](../cloud/README.md) has separate design and gates.

**Delivery: v0.1 and roadmap.** [v0.1-build.md](v0.1-build.md) selects the current portion; [roadsmap.md](roadsmap.md) retains the remainder. Detailed schemas/configuration/ABI and full-family acceptance below describe the target superset unless narrowed by that allocation.

## Existing behavior

Preserve file field validation, multipart operations, multiple filenames/modifiers, MIME/size restrictions, protected tokens, local/S3 storage, thumbnails, streaming and existing mail settings/templates/test actions. The database provider change must not alter existing filename semantics or SDK getURL behavior in the default context.

A private file identity is (databaseId, generation, collectionId, recordId, filename). The context selector/token is resolved before filesystem lookup. The old /api/files path stays; default-context behavior is unchanged. Private URLs carry a context-bound signed token or an explicitly authorized context query reference. An ID match in another context never selects its file.

## Storage namespace

New local storage roots are private per-context directories. New remote keys are namespaced with product format version, application storage namespace and opaque database ID before inherited collection/record/filename components. Generation changes require an explicit manifest map so restore does not accidentally serve current-generation keys under an old signature. Never derive keys from emails or names.

Local and S3 backends implement put/get/head/delete/list, range and checksum semantics required by the inherited file service. Thumbnails are derived artifacts with their own resource budget, cache key including context/generation/options, bounded dimensions and cleanup. Protected original files cannot become public through thumbnail caches.

## Write protocol and recovery

Stage uploads in job/request-owned temporary storage, validate limits/type/hash, then run the record transaction with a file intent manifest. Finalization is idempotent. A DB commit with pending object upload/finalization remains a visible pending file effect; incomplete content is not served as a successful download. Retry using the same immutable content hash/object intent.

On rollback delete only newly staged request-owned content. Updating/removing a file schedules deletion only after the referencing commit. Reference-count checks or a reconciler prevent orphan cleanup from deleting still-referenced content. Concurrent record updates and restore use generation checks.

Native whole-DB sync does not transfer S3/local files automatically. Record sync has a separate authorized file manifest/staging API. A file row arriving without bytes is represented as pending/unavailable, not silently complete.

## Download and token contracts

Tokens bind context, generation, actor/permission epoch where applicable, expiry and file purpose. Default protected-file lifetime retains inherited configuration; the new context claim does not lengthen it. Range/HEAD and cache headers obey the same protection. A revoked user cannot acquire fresh signed URLs; previously issued URLs remain usable only for their documented short expiry unless an online revocation check is configured.

Do not log bearer query values. Content-Disposition and MIME headers are safely encoded; user uploads are not served as trusted dashboard code. Thumbnail decoding has memory/time limits and malformed-image tests.

## Email

SMTP/sendmail and inherited template editing remain. Application name/sender/links use MiraBase defaults for new installations and preserve imported operator configuration. Reset/verify/change-email links use the external public URL and existing token semantics. Do not send mail during adoption dry runs, rollback or a retried transaction.

Transactional notification workflows persist outbox intent with the record commit. Default worker retry is 8 attempts with exponential delay capped at one hour; permanent failures become inspectable failed jobs. Provider acceptance is not a guarantee of recipient delivery. Credentials and message bodies containing secrets are excluded from diagnostics.

## Backup and deletion

Local upload files can be included in the coherent application archive. Remote object storage needs an explicit snapshot/versioning/export manifest and retention policy; DB backup alone is not file backup. Database deletion schedules remote-prefix cleanup and reports bytes/counts/failures; no unrestricted bucket deletion.

Acceptance covers protected files across contexts, concurrent upload/delete, interrupted finalization, S3 outage, restore with missing objects, malformed thumbs, URL expiry/revocation and duplicate notification prevention.



## Application-scoped service configuration

Bind files, thumbnails, staging, mail, sender, S3 credentials and protected tokens to application and database generations. Identical IDs/filenames in separate apps cannot collide. See [application-scoped service configuration](application-isolation.md).
