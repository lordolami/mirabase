# Go and JavaScript extensions

The Go framework remains embeddable. Use the actual [embedded example](../../examples/embedded/main.go) and native provider initialization/digest trust when embedding Turso. An inherited comparison build without `no_default_driver` is not the product engine build.

Goja JavaScript hooks/migrations are not Node.js. Use supported generated bindings, routes/middleware, cron, file/mail/log/template helpers and existing extension seams. New discovery uses MiraBase conventions; legacy aliases are deliberate compatibility settings.

Trusted Go/JS extensions have process privileges; they are not hostile-code tenant sandboxes. Never accept arbitrary hook source from an ordinary account. Carry current actor/database/generation through context-bound helpers; don't cache mutable selected-database state or one actor's authorized results for another.

Use the transaction callback's bound app. External mail/HTTP/payment effects in replayable transactions can duplicate; use durable intents/idempotency and explicit safe-retry boundaries. File operations need finalization/recovery. Preserve after-commit ordering.

Migration up/down/history changes have real effects. Preview destructive operations and all target/template revisions. Down migrations are not a universal inverse; retain compatible backups. Test hooks/routes/cron/schema SQL on the actual engine/profile through cancellation, restart and disk pressure.

[Hooks/jobs contract](../mirabase/extensions-and-jobs.md) · [migration guide](jobs-and-migrations.md) · [release builder](../../tools/release/main.go)
