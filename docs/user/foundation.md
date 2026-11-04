# Backend foundation

Base collections store records, auth collections add identity, and view collections expose read-only results. Configure schemas in the existing dashboard or collection APIs. Neutral routes remain `/api/collections`, `/api/files`, `/api/realtime` and `/_/`.

## Build a first backend

1. Follow [quick start](quickstart.md), open the printed setup URL and create your operator account.
2. Create an auth collection for application users. Configure the login methods and account emails you need.
3. Create a base collection for your application data, such as `notes`. Add `title` and `body` fields and a relation named `owner` to the auth collection.
4. Decide who can list/view/create/update/delete records. Set those rules before connecting an untrusted frontend. A locked rule, empty public rule and rule expression have different meanings.
5. Create test accounts and records in the actual application. Exercise each account through an ordinary user session; a superuser session bypasses normal policy and does not demonstrate user access.
6. Open the collection's API preview and connect your frontend with a compatible inherited client or HTTP.
7. Add files, account email, realtime or trusted extensions as the application requires. Optional private storage is a separate [database setup](databases.md).

An owner relation does not automatically enforce ownership. Configure the rules and test both permitted access and access by an unrelated account. Choose relation targets and field names from your actual schema; do not paste a rule whose referenced fields do not exist.

## Foundation capability reference

| Area | Available behavior | Configure or verify |
|---|---|---|
| Collections | Base/auth/view, schemas, indexes and collection administration | Data model and required query shapes. |
| Fields | Text/numbers/bools/email/URL/editor/password/date/autodate/select/JSON/relation/file/geopoint | Validation, uniqueness, hidden fields and serialization. |
| Records | CRUD, filters, sort, pagination, projection, expansion and modifiers | Actor rules and actual response fields. |
| Atomic batches | Configured record operations in one physical DB | Batch limits and failure rollback. |
| Accounts | Password/OAuth2/OIDC/OTP/MFA, refresh and account flows | Provider credentials, token lifetimes and mail. |
| Files | Multipart uploads, local/S3 storage, protected downloads and thumbnails | Field limits, access tokens and backup coverage. |
| Realtime | Authorized record SSE notifications | Subscribe with the real user identity and reconnect correctly. |
| Email | SMTP/sendmail, templates, test email and account messages | Real delivery and recovery flows. |
| Dashboard | Collection/record/auth/settings/log/backup/SQL tools | Keep operator credentials out of the frontend. |
| Extensions | Go framework and trusted Goja hooks/routes/cron/migrations | Actual engine behavior, privileges and replay effects. |

The [feature catalog](../features.md) lists every family, while the [API reference](api-reference.md) separates inherited and additive routes.

## Choose the record path deliberately

| Path | Intended surface | Current integration |
|---|---|---|
| `/api/collections/{collection}/records` | Inherited application records | Normal forms, validation, hooks and collection rules. |
| `/api/mirabase/v1/db/{databaseId}/collections/{collection}/records` | Explicit managed database records | Ownership/grants and raw-store CRUD; full inherited policy/form/hook/hidden-field parity remains unfinished. |
| Dashboard SQL / PostgreSQL gateway | Privileged database work | SQL authority is not ordinary record access. |

Do not assume managed-record filters, file uploads, expansion, validation or paging match the inherited API. See [database guide](databases.md) before using the managed helpers.

Inherited fields include text, number, bool, email, URL, editor, password, date, autodate, select, JSON, relation, file and geopoint. Required/hidden/system/unique options, modifiers and validation retain their contracts. Added field adapters are inventoried in [current product](../current-product.md); a native type alone is not a complete collection/UI/SDK feature.

CRUD, filters, sorting, paging, expansion, projection, multipart files and configured batches have their owning endpoint semantics. A batch covers one physical DB. Bind untrusted values rather than concatenating SQL. See [rules](rules-and-filters.md), [fields](fields.md), [files](files.md), [auth](authentication.md) and [realtime](realtime.md).

Password/OAuth2/OTP/MFA, verification/recovery/email changes and impersonation have distinct policies and token lifetimes. Account IDs/selectors alone are not authorization. Keep password/token metadata hidden across search/change/live paths.

Settings, mail/templates, local/S3 storage, protected URLs, thumbnails, logs/stats, health, static hosting, hooks, cron, migrations and the existing dashboard remain backend functions. S3 files are not automatically inside a DB-only archive. Administrative SQL does not acquire record forms/hooks simply by touching a collection table.

[Actual routes](api-reference.md), [foundation contract](../mirabase/pocketbase-foundation.md) and [launch results](../launch/status.md) provide the detail and evidence.
