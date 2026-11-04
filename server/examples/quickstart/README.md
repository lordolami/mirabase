# First frontend

Follow [the complete quick start](../../docs/user/quickstart.md) to build/start MiraBase and create the exact `members` and `notes` schemas/rules. Start the server from the repository root with `--publicDir=./examples/quickstart`, then open its HTTP root.

This small frontend has sign-in, create-note, list/refresh and sign-out behavior. It uses the inherited record API, no external frontend packages or CDN, and retains its ordinary member token only in memory. It renders record content with textContent. The client module runs in a browser or Node with built-in fetch.

It demonstrates shared storage with owner-based rules, not a separate physical database per member. The owned SDK's managed-record path has a documented policy-parity gap; this tutorial deliberately uses the normal inherited record path. See [database layouts](../../docs/user/databases.md) before private-storage setup.

This is a local development example: it has no self-signup, token refresh, password reset, full pagination or offline store. Use your own member password and add application account/recovery flows before production. Never enter a superuser token/password into this member form.

## Executed check

The [verification record](verification.json) covers the actual Windows native artifact, API creation of the exact documented schemas/rules, member login/create/list/logout/re-login through this example client, cross-account read/forged-owner/owner-transfer refusals, required/title-length validation, served frontend assets and persistence after process stop/restart. The client was executed under Node; dashboard clicks and browser event wiring were not automated by that check. Temporary test data was removed. This is specific tutorial evidence, not production qualification.
