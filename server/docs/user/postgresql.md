# PostgreSQL client access

The opt-in gateway speaks PostgreSQL wire protocol to a selected MiraBase database. It does not supply the entire PostgreSQL server, extension ecosystem or guaranteed compatibility with every ORM. Qualify your actual driver, queries, migrations and types.

## Enable and connect

| Setting | Purpose |
|---|---|
| `MIRABASE_POSTGRES_ENABLED=1` | Enable the listener; disabled by default. |
| `MIRABASE_POSTGRES_LISTEN` | Bind address; default loopback port 5433. |
| `MIRABASE_POSTGRES_TLS_CERT` | Listener certificate file. |
| `MIRABASE_POSTGRES_TLS_KEY` | Matching private key with protected filesystem access. |

1. Configure the environment and start the actual artifact. Missing/invalid required TLS material prevents successful enabled startup.
2. Have an operator issue a `pg`-purpose credential for the selected DB through `POST /api/mirabase/v1/databases/{id}/credentials`, with necessary SQL scopes/expiry.
3. Use exactly the returned connection values. Do not invent a username/database encoding or reuse dashboard/record passwords.
4. Configure client certificate verification and execute a simple query.
5. Check prepared queries, rollback, errors, cancellation, read-only refusal and revocation if your application depends on them.

Current external evidence covers node-postgres 8.23.1 with simple/prepared queries and verified TLS 1.3, not all drivers/ORMs. Disabling certificate verification is not that demonstrated connection.

## Protocol coverage

The implementation includes TLS/SCRAM, simple queries, extended parse/bind/describe/execute/close/flush/sync, prepared statements/portals, session settings, transaction/error state, cancellation and timeouts. Query execution uses the pinned Turso dialect. Binary results, COPY, catalog introspection and unsupported formats/types have restrictions; inspect [extended protocol source](../../internal/pgateway/extended.go). A handshake alone does not qualify migrations or an ORM.

Enable `MIRABASE_POSTGRES_ENABLED=1` with `MIRABASE_POSTGRES_TLS_CERT` and `MIRABASE_POSTGRES_TLS_KEY`. Default address is loopback port 5433; exact environment names/address behavior are in [startup wiring](../../apis/serve.go) and [gateway](../../apis/mirabase_pg.go). TLS is required.

Issue a separate `pg`-purpose credential through the operator credential API with necessary `sql.read|sql.write` scopes/expiry. Use actual returned username/database/password values; do not fabricate a format or use dashboard/record credentials. Verify the server certificate with psql/your driver. Keep secrets out of URLs, browser bundles and source control. Revoke and test existing-session termination.

Sessions must isolate prepared statements, portals, settings, transaction/error state and cancellation. Read-only authority cannot be bypassed by CTEs/functions/alternate protocol messages. Raw SQL is whole-DB authority, not ordinary record rules/validation; schema and CDC semantics need separate evidence. COPY, introspection, OIDs/binary encoding and SQL syntax have explicit compatibility limits.

[Protocol tests](../../internal/pgateway/pgateway_test.go) · [extended tests](../../internal/pgateway/pgateway_extended_test.go) · [design](../mirabase/postgresql-access.md) · [launch status](../launch/status.md)
