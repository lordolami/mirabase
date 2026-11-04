# Production and superuser access

## Running the server

```text
mirabase serve --dir=<data directory> --http=127.0.0.1:8090
```

New projects store data in `mb_data`. Keep the data directory on durable
storage and back it up with the server's backup tooling before upgrades.

The present native payload targets are Windows amd64 and Linux amd64. Linux
execution has been tested on Ubuntu under WSL with glibc 2.43. The current
library links libc, libm and libgcc and references GLIBC_2.38 symbols; musl,
Alpine, older glibc and other operating-system/architecture combinations are
not qualified by that run. Check the [platform results](../launch/status.md)
before choosing an artifact. An embedded payload still needs compatible OS
libraries.

## Restricting superuser access

Superuser sign-in can be restricted to specific IPs/subnets in Settings. If
you lock yourself out:

Use a previously allowed address or your tested offline recovery procedure. Take a verified backup before any offline settings change; do not guess at encrypted settings or hand-edit live database files. See [operations](operations.md) and actual CLI help.

Never share the superuser account; create separate operator credentials where
the product supports them and revoke them when they are no longer needed.

## PostgreSQL gateway (opt-in)

The PostgreSQL wire gateway is disabled by default. It starts only when the
operator sets `MIRABASE_POSTGRES_ENABLED=1` together with a TLS certificate
pair (`MIRABASE_POSTGRES_TLS_CERT` / `MIRABASE_POSTGRES_TLS_KEY`); it refuses
to serve plaintext. Access uses separately issued `pg`-purpose credentials
with `sql.read` / `sql.write` scopes — dashboard superuser accounts do not
work on this surface.

## Updates

MiraBase must only ever update from a signed MiraBase release source. Never
point the updater at another project's release feed.
