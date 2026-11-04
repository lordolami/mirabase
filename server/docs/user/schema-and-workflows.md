# Schemas, invitations and derived workflows

Schema plan/apply separates inspection and mutation. Bind actual target generation/revision, proposed objects, destructive effects and reviewed plan hash. Invalid data or engine/profile incompatibility blocks publication; no coercion or dropped rows.

## Pick the operation

| Need | Mechanism | Scope |
|---|---|---|
| Give new databases a known schema | Published template | Applied during provisioning before readiness. |
| Change a specific managed DB | Schema plan/apply | Reviewed hash and expected revision. |
| Evolve a group of DBs | Template rollout | Captured targets and per-target results. |
| Share selected common fields | Reference-copy policy | Primary source and explicit destination copies. |
| Clean up cross-context relations | Versioned relation workflow | Explicit delete/unlink steps and visible partial effects. |
| Read an aggregate across contexts | Persisted report | Definition, rebuild and freshness. |
| Add a person to a tenant | Membership invitation | Intended identity, expiry and membership policy. |

## Apply a schema change

1. Inspect the actual destination generation/schema and the intended collection/index objects.
2. Submit operations to `POST /api/mirabase/v1/databases/{id}/schema/plans` using the current expected schema revision.
3. Inspect blockers, destructive effects and the returned plan hash. The planner supports managed collection create/update/delete and supported ordinary/FTS index create/drop.
4. Apply the reviewed plan through `POST /api/mirabase/v1/databases/{id}/schema/plans/{planId}/apply`, carrying its hash and expected revision.
5. Verify the resulting schema/indexes and rows. A stale revision requires rereading/replanning, not removing the guard.

The additive planner explicitly refuses `type.define`, `type.drop`, `view.define` and `sequence.define`. This does not remove inherited SQL view collections. Added int64/decimal/UUID/date/time/timestamp/network/vector adapters do not establish complete UI/SDK parity; see [fields](fields.md).

## Run a rollout

Publish the actual template and create a rollout through `/templates/{templateId}/rollouts`. Inspect `/rollouts/{rolloutId}` and `/rollouts/{rolloutId}/targets`, then use its run/pause/resume/retry controls. Keep the rollout ID across reconnects. Inspect every target, including failed and not-yet-run targets; completed targets are not undone by cancellation or pause. A rollout with no targets does not prove a migration.

## Distribute shared reference data

Define a policy at `PUT /reference-copies/{policyId}` with explicit source fields and targets, inspect it with `GET`, and request `/refresh`. Read destination copies through the bound reference-copy record routes, rather than assuming ordinary cross-file relations. Track lag and quarantine/failure state. Secret/auth projections are refused. Changes to fields, filters or deletion policy require reconciliation, not merely updating a display label.

## Workflows, reports and invitations

Relation workflows are versioned definitions at `/relation-workflows/{workflowId}` with a guarded principal and explicit target collection/relation/action steps. Supported actions are delete/unlink. Run the definition and inspect step effects; earlier changes can remain committed if later work fails.

Reports use definition/rebuild/read operations at `/reports/{reportId}`. The materialized report result has freshness/coverage; it is not a cross-database SQL transaction or an always-current replacement for source records.

Membership invitation create/accept/revoke is separate from account creation or email verification. An invited account must satisfy the intended identity/expiry checks. See [actual API](api-reference.md) for full DTOs and operator wrappers; paths in this section use the additive base `/api/mirabase/v1`.

Templates publish immutable revisions. Rollout snapshots targets and records per-target outcomes. Inspect/pause/resume/retry through bound controls. Cancellation can retain completed targets; mixed revisions remain visible. One successful target or an empty target set is not proof of complete migration.

Invitation create/accept/revoke uses current intended identity, expiry and membership rules. Accepting an invitation differs from account signup/verification. Clients cannot choose arbitrary role elevation and retries cannot duplicate memberships.

References select a source collection, explicit nonsecret fields, parsed filter, targets, deletion behavior and lag. Hidden/auth-secret projection is refused. Define/status/refresh share a bounded worker (normal tick five seconds; default healthy lag 60 seconds). Primary is authoritative; copies require update/delete/policy-edit reconciliation.

Relation workflows and reports are privileged definitions with actual persisted steps/results/freshness. They do not make cross-file ACID transactions. Failed/cancelled workflows can have committed earlier steps; resume by recorded step identity and show partial effects.

Inspect actual destination schema/rows/hashes and target counts, not only a job success envelope. See [source](../../internal/application/derived.go), [reference tests](../../internal/application/reference_copy_test.go), [setup tests](../../apis/mirabase_setup_acceptance_test.go), [API](api-reference.md) and [launch results](../launch/status.md).
