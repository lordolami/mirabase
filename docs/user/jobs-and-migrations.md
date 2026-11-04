# Jobs and migrations

MiraBase has trusted application cron/hooks and a separate durable operation-job
kernel inside the server process. Cron registration, migration history and
durable operation receipts are different mechanisms; configuring a cron hook
does not automatically give every external effect exactly-once delivery.

## Durable operation jobs

Long-running provisioning, maintenance/import/recovery operations use registered
handlers with persisted jobs. The kernel supports enqueue/lease/renew, expired
lease reconciliation, status/results, failure phases, cancellation and recorded
completed steps. Preserve job IDs and inspect actual terminal results. A cancel
request does not undo completed effects. See [operations](operations.md) for
reconciliation and the [job kernel](../../internal/dbexec/jobs.go) for implementation.

## Scheduled jobs

Cron expressions use the standard five-field form (minute, hour, day of month,
month, day of week). Cron is an application-wide trusted extension surface,
not a scheduler per private DB. Independent-application runtime isolation is
not fully accepted; application metadata alone does not prove isolated
settings/hooks/background work.

## Migrations

Schema and data migrations are versioned, ordered, and recorded. A migration
runs once per database; re-running the server does not re-apply it. Migrations
that fail stop the upgrade with a typed error rather than leaving a half-applied
schema behind.

The inherited migration framework records application migration versions.
Evolving a set of managed private databases uses the separately tracked
[schema/template rollout](schema-and-workflows.md), not an assumption that one
application migration automatically migrated every database file. Down
migrations are not a universal lossless inverse; verify recovery before use.

New projects use `mb_migrations`; legacy `.pb.js`/`.pb.ts` hook and migration
files are discovered only in explicit compatibility mode, per the branding and
compatibility contract.

## Hooks

Trusted hooks (`.mb.js`/`.mb.ts` for new projects) run with server authority and
must be treated as code, not data: only operators who can already administer
the server should be able to install them. Hooks execute in a bounded shared
pool — never one interpreter process per database.
