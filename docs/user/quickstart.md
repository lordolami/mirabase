# Start an application

Use an artifact qualified for your OS/architecture and verify its checksum/signature. Before publication, build from the source root with `go run ./tools/release --version dev`. [Launch status](../launch/status.md) records qualification limits.

Run `mirabase serve --http=127.0.0.1:8090`. Add `--dir=/absolute/path/mb_data` for an explicit persistent data root; absolute Windows paths work too. Public access needs HTTPS or a correctly configured TLS reverse proxy.

Use the setup URL printed by the process to create the first superuser. Open `/_/`, create an auth collection for application accounts and ordinary collections with list/view/create/update/delete rules. Locked rules differ from empty public rules. Never put a superuser token into your frontend.

A first shared-storage app needs no tenant setup. Connect using a compatible record client or HTTP. Add optional private storage through [database setup](databases.md).

Check `GET /api/health`. Read the additive catalog at `GET /api/mirabase/v1/openapi.json`; capability discovery at `/api/mirabase/v1/capabilities` is authenticated. Advertisement does not prove qualification.

Before production, exercise two unrelated users, access rules, protected files, mail/password recovery, restart, backup restored into a separate root and complete server move. Configure trusted proxies explicitly; keep secrets out of static assets and source control. Use normal shutdown to drain native work and files.
