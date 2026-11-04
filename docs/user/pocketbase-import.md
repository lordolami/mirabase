# PocketBase adoption

Keep a coherent stopped directory or verified backup and its original data. The inherited `PUT /api/collections/import` is schema-only; it is not full application adoption.

## Choose the import outcome

| Starting point | Intended outcome | What must be reviewed |
|---|---|---|
| One PocketBase app, retaining shared data | Shared-layout adoption | Accounts, rules, records, settings and file coverage. |
| One app containing personal data | User databases | Ownership of every row/file and shared reference data. |
| One app containing companies/teams | Tenant databases | Tenant membership, owners, shared records, relations and uniqueness. |
| Agent-owned records | Agent contexts | Agent identities, owners and grants. |
| Several independent applications | Independent-app consolidation target | Separate auth/settings/hooks/runtime isolation, not merely several DB files. |

The last outcome remains partially integrated. Batch/session metadata alone is not proof that unrelated apps can safely share all runtime surfaces. Preserve this distinction when choosing an import plan.

## Operator workflow

All session paths below are relative to `/api/mirabase/v1/pocketbase-imports`. Use operator authority for the actual handler; do not share the source backup or operator token with ordinary frontend clients.

| Step | Operation | Result to inspect |
|---|---|---|
| 1 | `POST` session root | Session identity and source requirements. |
| 2 | `PUT /{importId}/source`, then `GET` | Uploaded source, hash and coverage. |
| 3 | `POST /{importId}/analyze` | Actual inventory and issues. |
| 4 | `PUT /{importId}/plan` | Reviewed source/destination/layout decisions. |
| 5 | `POST /{importId}/preview` | Proposed allocation, ownership, relations and unresolved cases. |
| 6 | `POST /{importId}/stage` | Isolated staged destination and completed job effects. |
| 7 | `POST /{importId}/validate` | Report/cases, missing records/files and unresolved errors. |
| 8 | `POST /{importId}/cutover` | Explicit activation and journaled outcome. |
| Recovery | `/{importId}/resume`, `/recovery` or `/cancel` | Current durable phase and preserved state. |

The dashboard exposes this workflow. Carry the real session/job/report/plan revisions through it. Inspect the [handler DTOs](../../apis/mirabase_pbimport.go) before scripting bodies; this table is a sequence, not a promise that every stage is synchronous or accepts an empty body.

## Validate conversion, not only upload

Check collection and row counts, stable IDs, ownership allocation, login behavior, rule behavior, unique constraints, relation targets and file bytes in each destination. A company with several members should map to the reviewed tenant database, not accidentally produce a separate company copy for every account. Define where shared/global records belong. Missing ownership decisions must remain issues, not arbitrary defaults.

The current real dashboard evidence validates a backup produced by the pinned PocketBase executable. Complete no-loss conversion for every user/tenant/agent mapping and arbitrary historical source version remains additional acceptance. See [launch status](../launch/status.md).

## Multiple sources and recovery

Batch operations live under `/api/mirabase/v1/import-batches`: create, inspect, inspect sources, run and cancel. Retain each source identity/hash and per-source result. Batch completion must not hide failed or omitted sources.

Keep the original source. Validate staging with mail, hooks and external jobs disabled. After activation, accepted new writes make the old snapshot stale: do not use rollback as a way to discard those writes. Reconcile a lost cutover response through durable session/journal state before attempting another activation. A full server move additionally follows [operations](operations.md).

Full sessions proceed through create/source upload/check, analyze/inventory, plan, preview, staging, validation/report, explicit cutover and resume/recovery/cancel. Exact paths and request structs are in [API reference](api-reference.md) and [import handlers](../../apis/mirabase_pbimport.go). Preserve source/hash, target generation and reviewed plan/report revision.

Inventory records, accounts/rules/settings, schema/migrations, local files, trusted artifacts and remote-object coverage. Use private references for source credentials/keys. Stage in job-owned paths and disable historical mail/hooks/external jobs during validation. Missing files, wrong keys or unresolved ownership cannot be success.

Shared adoption preserves source layout. Tenant/user/agent conversion additionally needs actual ownership/membership/rule mapping and per-destination reconciliation. Tenant/batch entry points exist, but the complete promise needs every source row/file, stable IDs, login, rules, uniqueness and no-loss cutover tested. Source acquisition alone does not prove the transform.

Independent apps differ from splitting one app by user. Batch metadata APIs alone do not establish independent auth/settings/hooks runtime isolation. Read [database boundaries](databases.md) and [current evidence](../current-product.md).

Retain old state for recovery, but never overwrite post-activation acknowledged writes with its snapshot. Lost cutover responses require journal reconciliation. Cancellation retains committed steps with explicit cleanup. Interrupt each publication/rename boundary and compare destination hashes/auth behavior.

Pinned baseline: PocketBase v0.41.1. Older versions require a supported upgrade or qualified adapter. Other-BaaS migration is separate. [Full contract](../mirabase/pocketbase-tenant-import.md) · [acceptance](../../apis/mirabase_pbimport_acceptance_test.go)
