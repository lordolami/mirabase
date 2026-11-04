# MiraBase implementation review copy

This is a copy of the implementation packages in [server/internal/](../server/internal/). The complete application, build entry points and dependencies are in [server/](../server/README.md). Go builds use that canonical source, not this copy. This directory is not an independent Go module or separately importable public library.

| Package | Responsibility |
|---|---|
| admission | Shared resource admission. |
| application | Database lifecycle, immutable context, grants/credentials, records, changes, synchronization, schema and workspace services. |
| dbexec | Database execution, writers, transactions, durable jobs, receipts and outbox primitives. |
| livequery | Live-query plans, evaluation, cursors and subscriptions. |
| mbengine | Native Turso loading, digest trust, provider setup and probes. |
| pbimport | PocketBase backup adoption, validation and cutover. |
| pgateway | PostgreSQL wire gateway and session/authentication handling. |
| search | Vector, full-text, hybrid and fuzzy search. |
| setup | Storage layouts, membership, templates and rollouts. |

More MiraBase changes live in the server's API handlers, inherited core integration points, SDKs, dashboard, native bridge, tests and documentation. Package names and source presence are not acceptance evidence; see the [source inventory](../server/docs/current-product.md).

Upstream-derived platform/driver material and native payloads retain their original attribution. Read [third-party notices](../server/THIRD_PARTY_NOTICES.md) and [the license](../server/LICENSE.md). This review copy does not claim that every included line was written during the hackathon. Refresh it from the canonical source when publishing source changes; edit `server/internal/` first.
