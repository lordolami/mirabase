# Existing dashboard extensions

**Current release authority:** [current implementation inventory](../current-product.md), [user documentation](../user/index.md) and [launch status](../launch/status.md). Initial v0.1/roadmap delivery labels below preserve planning contracts; they do not establish today's implementation or permit deleting subsequently integrated features. [Open-source platform roadmap](../roadmap/platform/README.md) has separate design and gates.

**Delivery: v0.1 and roadmap.** [v0.1-build.md](v0.1-build.md) selects the current portion; [roadsmap.md](roadsmap.md) retains the remainder. Detailed schemas/configuration/ABI and full-family acceptance below describe the target superset unless narrowed by that allocation.

## Preserve the application

Extend the shipped PocketBase dashboard source and UI extension seams. Retain navigation, collection/record/auth editors, filters, relation expansion, files/thumbs, rule editor, logs/stats, settings, backup UI, cron list/run, SQL editor, schema import/export, ERD and API preview. Rebrand all owned assets/labels. Do not create a second platform console that users must learn.

Default single-database navigation is unchanged apart from MiraBase identity and relevant new field/search controls. Optional database features appear when enabled. A capability being disabled must have an explanation and enablement requirements; absent implementation is never labeled "coming from your configuration."

## Required workflows

| Area | User actions and mandatory feedback |
|---|---|
| Database browser | Search/page/filter by ID, owner, kind, state, profile, revision and storage; create, inspect, select, maintenance, export, backup, restore, delete; explicit job/result |
| Context bar | Selected database ID/name/generation visible in record/file/schema/SQL panes; no ambiguous global writes |
| PocketBase import | Existing schema JSON flow plus full-app shared/tenant choices; resumable source upload, tenant root/ownership path editor, shared placement, memberships/rules, per-tenant records/files preview, issue resolution, validation artifacts, cutover/recovery status |
| Ownership and agents | Inspect owner binding, grants/scopes/expiry, register/revoke agent; no exposed secret after creation |
| Fields | Editors for every inherited/new field, dimensions/precision/variants/defaults, capability and index constraints |
| Search | Vector input/metric, text tokenizer/query/weights/highlights, fuzzy policy and hybrid fusion; scores and completeness labeled |
| Schema | Dry-run diff, destructive effects, dependency errors, expected revision, plan apply and per-target rollout progress |
| Views | Query validation, ordinary/materialized distinction, dependency graph, mode constraints, rebuild/live-query status |
| Changes | Transactions/cursors/retention/lag, authorized projections, raw privileged inspection clearly scoped |
| Sync | Client instances, pending/conflict state where server-visible, bootstrap/retention/errors, credential issue/revoke |
| Encryption | Key reference/algorithm/state; rotate job with backup requirement; never display key bytes |
| PostgreSQL | Listener/auth/TLS settings, credential management, supported client/SQL matrix and active sessions |
| Backup/recovery | Contents, local/remote object coverage, hashes, generations, restore dry run/status and Windows workflow |
| Jobs/operations | Phase, bounded counters, exact error, retry/cancel eligibility and result; no success from queued state |
| Capability diagnostics | Engine/library digests, profile/build/platform availability, incompatible settings and qualification status |

## Interaction rules

Every create/update/delete form has validation, loading, success, failure, stale-revision and cancellation states. Disabling a button is not the only authorization check. Destructive operations identify the target by context and require the submitted action to include its expected generation/revision; UI confirmation is a product interaction, not an agent permission requirement.

Don't expose million-database lists by loading all rows into the browser. Use indexed server pagination/search. Schema fan-out shows actual total/succeeded/failed/pending counts and per-target details. No progress percentage is invented when the denominator is unknown.

Rule/type/query previews execute in a safe scratch/read-only context and state which actor/context is being simulated. Schema SQL and native administrative SQL have explicit privileged labels. File previews never execute uploaded code.

The full import workflow is specified in pocketbase-tenant-import.md. Preview, validation and cutover are separate states. Show actual row/file coverage, assignments and access changes; never describe an uploaded schema or queued job as a successful tenant conversion. Preserve the plan when resolving errors and invalidate stale preview/report hashes after changes.

## Quality and evidence

Support keyboard navigation, visible focus, form labels/errors, screen-reader status announcements, meaningful contrast, narrow screens and timezone/large-number formatting. Secret fields are write-only with replace/test flows. URLs/deep links encode context without embedding credentials.

Browser tests exercise all workflows on the real backend, with empty/loading/error/large-list states, revocation mid-session, failed rollouts and protected files. Screenshot review checks branding and layout; UI tests alone do not establish native correctness.



## Complete guided setup and consolidation

Provide the full storage-choice walkthrough and runnable generated frontend examples. Add instance-operator application selection and separate app-local administration. Consolidation shows per-source inventory, issues, validation, partial outcomes and recovery; never merge application creation with membership invitation. See [complete guided setup and consolidation](new-application-setup.md).
