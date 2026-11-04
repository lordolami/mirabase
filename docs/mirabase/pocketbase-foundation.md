# PocketBase foundation

**Current release authority:** [current implementation inventory](../current-product.md), [user documentation](../user/index.md) and [launch status](../launch/status.md). Initial v0.1/roadmap delivery labels below preserve planning contracts; they do not establish today's implementation or permit deleting subsequently integrated features. [Managed cloud](../cloud/README.md) has separate design and gates.

**Delivery: source baseline.** Apply the [v0.1 allocation](v0.1-build.md) to current features; [roadmap](roadsmap.md) owns later integrations. Source facts alone do not qualify runtime behavior.

## Baseline and preservation rule

The untouched reference is PocketBase v0.41.1, commit f70b24e186df98df2e7ebc703f103cb94dc3ca63. It is both a Go framework and a ready-made backend. MiraBase extends this backend. A SQL shell, database-management catalog or SDK alone does not replace it.

Every public behavior below is required. Source paths refer to the pocketbase directory at the workspace root. Run inherited tests against the original provider and the Turso provider; compare behavior, including negative cases. A known upstream limitation stays visible and is not presented as a newly solved problem.

## Collections, data and API

Base collections store application records. Auth collections add identity/password/token/email/auth-policy behavior. View collections expose read-only SQL query results. Preserve IDs, collection names and aliases, schema/index management, API rules, timestamps and system/hidden metadata. Preserve schema import/export, scaffolds, view-query dry run, truncation, deletion and ERD/API-preview behavior. Source: core/collection_model.go, core/collection_import.go, apis/collection.go.

Inherited fields are text, number, bool, email, URL, editor, password, date, autodate, select, JSON, relation, file and geopoint. Keep every field's configured validation, required/unique semantics, defaults, hidden/system behavior, modifiers and JSON representation. Source: core/field*.go. Password raw-hash import is trusted server behavior, never a client password shortcut.

Keep record create/read/list/update/delete, filtering, sorting, pagination, relation expansion, field projection, multipart upload and modifiers. Preserve standard error envelopes and validation detail. The optional batch endpoint supports record operations atomically in one database; preserve its configuration and limits. Source: apis/record_crud.go, apis/batch.go, tools/search.

## Identity and authorization

Retain multiple auth collections; password login; OAuth2 provider configuration and redirect/realtime flows; OTP; optional MFA; auth refresh; password reset; email verification and email change; auth alerts; email templates; token durations and token-key invalidation; superuser impersonation and auth management rules.

The five collection rules are list, view, create, update and delete; auth/manage rules are additional auth-collection policy. A null rule is locked, an empty rule is public and an expression is evaluated in its existing request context. Preserve superuser semantics and hidden auth fields. Do not translate these policies into an invented global role system. Source: core/collection_model_auth_options.go, apis/record_auth*.go, apis/middlewares.go.

## Files and notifications

Local and S3-compatible file storage, single/multi-file fields, MIME/size restrictions, protected file tokens, filename generation, thumbnail options, streaming downloads and file cleanup remain. Protected files require the correct collection/record access in the selected database. Existing realtime uses SSE connection establishment and subscription updates; create/update/delete broadcasts are generated after successful record effects and evaluated against rules. It is not a durable offline change log.

Preserve SMTP/sendmail configuration, template rendering, test email/S3 actions and provider-specific helpers. Source: apis/file.go, apis/realtime.go, core/base.go, core/record_model.go and mail helpers.

## Administration and operations

Keep superuser setup, dashboard, settings, logs/list/stats/delete, health, rate limits, trusted proxy behavior, superuser IP restrictions, request limits, static frontend serving, TLS/autocert support, collection schema export/import and SQL console. The SQL console is superuser-only administrative SQL. Its raw writes do not acquire record validation simply because the same database is used.

Keep cron registration in Go/JS and API list/run. The existing HTTP cron API does not create arbitrary source-code jobs. Keep scheduled backup, backup list/create/upload/download/delete/restore, local/S3 backup destinations and retention. Preserve migration up/down/history synchronization and generated Go/JS schema migrations.

Inherited backup archives include coherent data.db/auxiliary.db copies and applicable local application files. S3-uploaded record files are not included automatically. The reference restore-and-restart path is unsupported on Windows; MiraBase must provide a safe stop/restore/start workflow on Windows rather than claim the inherited route works there.

## Developer framework and dashboard

Keep Go hooks, record/collection proxies, custom routes, middleware, server commands, cron, transactions, filesystem/mail/logging/template helpers, custom models and test helpers. Keep Goja JavaScript hooks and migrations, routerAdd/routerUse, cronAdd/cronRemove and $app bindings. This runtime is not a Node.js process. The ready binary currently prewarms 15 JS runtimes; this is application-wide, never multiplied by database count.

Keep the existing UI architecture, collections/records editors, auth administration, logs, settings, mail, storage, backups, jobs, schema tools, SQL editor, relation selection, file upload, API examples and extension mechanism. Current UI source is JavaScript with its existing build system, not an assumed Svelte rewrite.

## Provider-sensitive integration points

Config.DBConnect in pocketbase.go and core/base.go is the provider seam and returns *dbx.DB. The default provider is in core/db_connect.go; no_default_driver disables its SQLite import. core/base.go opens data.db and auxiliary.db and uses separate concurrent/nonconcurrent pools; the nonconcurrent writer has one open connection. core/db_tx.go uses real DBX transactions. SQLite introspection in core/db_table.go and schema mutations in core/collection_model.go must work with Turso.

Every one of these seams requires integration tests. Returning a driver handle is only the beginning: PRAGMAs, schema rebuilds, migration rollback, prepared statements, cancellation, error mapping, last-insert behavior, files, hooks and shutdown must retain their semantics.

