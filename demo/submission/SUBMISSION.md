# MiraBase

**One backend. A private database for every user, tenant, and agent.**

**Empowering the next generation of indie hackers and AI agents with zero-infra multi-tenancy.**

## Short description

MiraBase extends PocketBase with the Turso engine and private databases inside one backend process. Developers keep collections, authentication, files, realtime and the dashboard, then add user, tenant and agent database contexts, search, change history, live queries and recoverable working files. Developed with 1.3-contributor for implementation and Muse Wonder for architecture documentation, alongside a separate Quint-based verification harness.

## The problem

A solo developer building a customer portal, a team workspace or a multi-agent application needs durable state without assembling another backend deployment for every tenant. PocketBase supplies the familiar application experience; Turso adds in-process database capabilities. Zero-infra multi-tenancy means no additional process/container/service per database, while one server and its persistent storage still need to be operated.

A customer portal, a team workspace and an AI agent all need durable state. They also need clear ownership. Developers should not have to assemble a new backend stack or run another backend process whenever they want a separate database.

MiraBase makes a private database a backend capability. An individual can have a user database; a company can share a tenant database; an agent can have an agent-owned database. These are optional layouts within one application backend. A separate file does not replace authentication, authorization or resource limits.

## What we built

We extended the PocketBase application framework and dashboard with an in-process Turso provider and the following implementation families. PocketBase and Turso remain upstream projects with their own authors and licenses. Our work is the extension, integration and supporting tooling; we do not claim to have invented their inherited capabilities.

| Area | Concrete work and developer benefit |
|---|---|
| Backend foundation | Preserve collections, typed records, relations, authentication, access rules, file storage, realtime, mail, migrations and Go/JavaScript extension points. Developers connect a frontend to an existing backend foundation. |
| In-process engine | Compile the pinned Turso engine into a native library and bridge it into Go's database interfaces. Data and auxiliary stores use the selected provider. The current executable embeds and extracts a verified native payload; it is not a Wasm build. |
| Payload trust | Compile expected digests into the executable and verify the payload before loading. An adjacent checksum cannot authorize a modified library. Pin source patches and reject unexpected engine changes during the native build. |
| Private database lifecycle | Provision and inspect user, tenant and agent databases, with owner bindings, durable jobs, templates and lifecycle state. Shared services serve many database files without a process, scheduler or warm JavaScript VM for every database. |
| Target and authority | Resolve immutable application/database context, then enforce the implemented database ownership and explicit grant boundaries. Record access, privileged SQL and replication are distinct surfaces. Complete collection-policy parity on managed CRUD remains open. |
| Search | Expose vector similarity, full-text search, hybrid ranking and fuzzy matching over the selected database. The demo shows actual returned results. |
| Durable changes | Retain ordered committed record changes with cursors and explicit retention/reset behavior in the design and APIs. |
| Record synchronization | Implement bootstrap/page, mutation push, incremental pull and acknowledgement surfaces. These are record-sync APIs, not proof of upstream native whole-database protocol compatibility or a completed offline SDK. |
| Live queries | Publish selected SQL queries and subscribe to result updates, with cancellation and managed subscriptions. |
| Schema management | Add database templates, explicit schema plans and rollout controls so private databases can evolve under one application. Provisioning applies the published template before readiness. |
| Shared reference data | Copy selected shared fields into private contexts through managed reference definitions and refresh operations. |
| Derived workflows | Implement workflow/report families, including aggregation across matching database contexts. These do not imply cross-database distributed SQL transactions. |
| Agent working state | Add selected local working-file, snapshot, historical-byte and restore helpers. Current workspace HTTP operations use superuser authority. Full upstream AgentFS KV/inode/mount/overlay parity is additional work. |
| PostgreSQL client access | Add a scoped PostgreSQL wire gateway with TLS and SCRAM. The recorded external client uses node-postgres, including prepared queries and verified TLS. This is not full PostgreSQL server compatibility. |
| PocketBase adoption | Provide source upload, analysis, preview, staging, validation and recoverable cutover. The actual dashboard validates a backup created by the pinned PocketBase executable. Arbitrary historical versions and independent application runtime isolation remain separate acceptance obligations. |
| Operations and recovery | Add diagnostics, durable jobs, receipts, backups, export/import and maintenance surfaces. The backup regression independently reopens archives created during concurrent writes. |
| Client libraries | Add immutable context/managed-record helpers to JavaScript/TypeScript and Dart clients. Preserve target scope across concurrent requests. Full SDK and offline parity remain open. |
| Documentation and evidence | Provide user and agent guides, API registrations, source-linked implementation inventory, contracts, cloud architecture and a qualification ledger that distinguishes implementation from executed acceptance. |

## The three-minute demonstration

The film is an edited recording of actual application use and server responses, with narration, captions and chapter markers. The original feature-tour film and full desktop session are preserved separately.

| Time | What the judge sees |
|---|---|
| 0:00 | The product idea: four private contexts under one backend. The opening diagram is an explanatory graphic. |
| 0:10 | Genuine desktop footage of the dashboard: collection fields, creating a task and changing its state. |
| 0:27 | Alice, Bob, Northstar and a research agent have distinct database contexts. |
| 0:47 | Bob is denied Alice's records, receives an explicit read grant, succeeds, and is denied after revocation. |
| 1:09 | Full-text, vector, hybrid and fuzzy results returned from the private database. |
| 1:26 | Actual live-query updates and the backend's related change/synchronization capabilities. |
| 1:36 | Write a working file, snapshot, overwrite and restore the original historical bytes. Operator authority is labelled. |
| 1:58 | Operational APIs and the PocketBase adoption workflow. |
| 2:10 | Engineering and AI build tooling, including the separate verification companion. |
| 2:28 | Clearly labelled roadmap. |
| 2:50 | The product promise and close. |

The custom frontend shown in several chapters is a connected example client, explicitly labelled on screen. It is not presented as the bundled administrator dashboard. No database stress job was run for this edit. The earlier cancelled scale exercise is not portrayed as a million-database result or a production benchmark.

## How we used Meta tooling

We used **1.3-contributor for implementation** and **Muse Wonder for full architecture documentation** across MiraBase and our separate [formal-verification companion](https://github.com/lordolami/formal-verification). This is our stated development-tool provenance; the film does not invent token counts, productivity multipliers or model-attribution percentages.

The companion supports verification during design, implementation, testing and maintenance, including proposed designs, new software and existing systems. It addresses a problem that became especially clear while building infrastructure with AI: a convincing completion report is not evidence that a boundary holds. It drives Quint models and links claims to source, checking methods and recorded results. Its repository includes model execution, bounded exhaustive checks, guard-weakening mutation tools, claim-linked HTTP probes, trace conformance and provenance-aware reports.

These are capabilities of a separate assurance tool. We do not claim that every MiraBase path has been mapped, exhaustively verified or proven correct. A successful bounded model check has the model's assumptions and bounds; a live authorization test has its actual endpoints and identities. They are valuable, different forms of evidence.

## Engineering depth

The engine bridge requires more than replacing a connection string. It must handle statement and connection lifetime, cancellation, asynchronous progress, provider errors and native packaging. Payload checks must reject a modified library even when an attacker also replaces an adjacent checksum. Schema preparation must finish before a new database is declared ready. Backup/export must remain coherent while writes continue.

The qualification work exposed and corrected actual defects. Examples include provisioning that advertised readiness before applying its published template; mixed-case import metadata comparisons that missed existing tables after engine normalization; and stale source-schema state that could restart a `VACUUM INTO` export. The owned native patch is pinned and its before/after regression evidence is retained. These corrections are implementation work, not simulated success screens.

## Verification status

The current Windows native full invocation recorded **5,236 test/subtest passes** and four archive failures caused by Windows denying symbolic-link creation. Those same four assertions passed in a separate authorized administrator invocation. We do not combine these into a claim of one clean full run. Focused provisioning, import, backup, payload-trust, SDK and verified-TLS PostgreSQL-client evidence is recorded in the launch report.

The earlier Linux baseline recorded **5,238 passes**, with no failures or skips, but used earlier source/payload and does not qualify subsequent changes. Other promised platform targets still require payloads and execution. The current release remains **unqualified**. In particular, managed CRUD does not yet apply complete inherited collection rules/forms/hooks/hidden-field behavior. Full offline/native-sync interoperability, AgentFS parity, independent application runtime isolation, hard native memory admission, signing and broader fault/platform acceptance also remain open. These limitations are disclosed so the submission can be assessed against what actually runs.

See [current implementation](../../docs/current-product.md), [launch status](../../docs/launch/status.md) and [feature evidence](../../release/feature-qualification.json). The delivered film's evidence manifest binds its actual recordings and executable/native digests.

## Roadmap

1. Complete collection-policy parity and outstanding safety dependencies; qualify platforms, SDKs, resource behavior, recovery and signed delivery.
2. Complete ordinary-client offline bootstrap/native sync and full upstream AgentFS integration. Investigate Wasm/wazero packaging against actual filesystem, feature, performance and lifetime requirements.
3. Develop the future open-source platform and managed-deployment capabilities: placement, replication, routing, fenced failover, balancing, cold-database handling, backups/recovery, project console and metering. Keep the application/private-database model while infrastructure changes underneath it. This is planned, not deployed. One hot database's write sharding and cross-database distributed transactions are not implied.

Self-hosted MiraBase remains useful without a managed-cloud account. Distribution and managed-deployment tooling remain part of the future open-source roadmap; no current hosted service or commercial offering is implied.

## Why this matters

Developers should spend more time building their application experience and less time recreating backend infrastructure for each customer or agent. MiraBase brings private database contexts, search and recoverable working state into a familiar backend foundation. The verification companion extends the same principle to AI development: make both the behavior and its evidence inspectable.

**Build your frontend. Own your backend.**
