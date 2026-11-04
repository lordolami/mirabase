# Current product implementation inventory

This is the current source baseline, beyond the initial v0.1 allocation. Source presence, operation advertisement, executed test results and release qualification are distinct. Do not remove later code because an initial phase document classified it as roadmap. Current standalone contracts and cloud design have separate launch gates.

For the complete user-facing inventory, read the [42-family feature catalog](features.md). The [README](../README.md) explains the product and the [task guides](user/index.md) explain usage. This page owns source/test navigation and integration limits; [launch status](launch/status.md) owns actual executions. No family below is automatically release-qualified because a source/test link exists.

| Family | Concrete implementation | Related acceptance source |
|---|---|---|
| Foundation | [core/collection_model.go](../core/collection_model.go) | [apis/record_crud_test.go](../apis/record_crud_test.go) |
| Private storage/rules | [internal/application/context.go](../internal/application/context.go) | [apis/mirabase_isolation_test.go](../apis/mirabase_isolation_test.go) |
| Vector/text/hybrid/fuzzy | [internal/search/search.go](../internal/search/search.go) | [internal/search/hybrid_fuzzy_test.go](../internal/search/hybrid_fuzzy_test.go) |
| Schema/types | [internal/application/fieldkinds.go](../internal/application/fieldkinds.go) | [apis/mirabase_setup_acceptance_test.go](../apis/mirabase_setup_acceptance_test.go) |
| Templates/rollouts | [internal/setup/rollout_controls.go](../internal/setup/rollout_controls.go) | [apis/mirabase_setup_acceptance_test.go](../apis/mirabase_setup_acceptance_test.go) |
| Changes/record sync | [internal/application/recordsync.go](../internal/application/recordsync.go) | [apis/mirabase_records_journal_acceptance_test.go](../apis/mirabase_records_journal_acceptance_test.go) |
| Live queries | [internal/livequery/service.go](../internal/livequery/service.go) | [apis/mirabase_live_acceptance_test.go](../apis/mirabase_live_acceptance_test.go) |
| References/workflows/reports | [internal/application/derived.go](../internal/application/derived.go) | [internal/application/reference_copy_test.go](../internal/application/reference_copy_test.go) |
| PostgreSQL gateway | [internal/pgateway/gateway.go](../internal/pgateway/gateway.go) | [internal/pgateway/pgateway_extended_test.go](../internal/pgateway/pgateway_extended_test.go) |
| Workspace files/snapshots | [internal/application/agentfs.go](../internal/application/agentfs.go) | [Historical-byte and integrity regressions](../internal/application/agentfs_test.go), plus actual artifact HTTP runs |
| PocketBase sessions/cutover | [internal/pbimport/pipeline.go](../internal/pbimport/pipeline.go) | [apis/mirabase_pbimport_acceptance_test.go](../apis/mirabase_pbimport_acceptance_test.go) |
| Application metadata | [internal/application/apps.go](../internal/application/apps.go) | [Metadata API tests](../apis/mirabase_setup_acceptance_test.go); these do not prove full independent runtime isolation |
| Backups/portable interchange | [internal/application/portable.go](../internal/application/portable.go) | [apis/mirabase_admin_acceptance_test.go](../apis/mirabase_admin_acceptance_test.go); native page-key rotation remains refused |
| SDK contexts | [sdk/js/src/context.ts](../sdk/js/src/context.ts) | [JS transport regressions](../sdk/js/test/transport.test.mjs), Dart analyzer and real server CRUD; see launch evidence for exact scope |

## Additional implementation ownership

| Surface | Source to inspect | Detail and limits |
|---|---|---|
| Inherited fields/validation | [core](../core/collection_model.go) | [Fields](user/fields.md); core behavior differs from a native type merely being accepted. |
| Identity/provider adapters | [Provider registry](../tools/auth/auth.go) | [Authentication](user/authentication.md); providers require configuration. |
| Managed CRUD | [Handlers](../apis/mirabase_records.go) | Raw-store path with the collection-policy gap below. |
| Lifecycle and provisioning | [Lifecycle](../internal/application/lifecycle.go) | [Database guide](user/databases.md); applies template before readiness. |
| Membership/invitations | [Membership service](../internal/setup/membership.go) | [Schema/membership guide](user/schema-and-workflows.md). |
| Schema plans | [Planner/apply](../internal/application/schemaops.go) | Unsupported type/view/sequence operations are explicitly blocked. |
| Durable jobs | [Job kernel](../internal/dbexec/jobs.go) | Lease, cancellation, failure/steps and result persistence. |
| Writer/transaction infrastructure | [Writer pool](../internal/dbexec/writerpool.go) | WAL/MVCC, outcome classification and retry boundaries. |
| Outbox helpers | [Outbox](../internal/dbexec/outbox.go) | Delivery/envelope primitives, not automatic exactly-once external effects. |
| Managed-target backups | [Target backup](../internal/application/targetbackup.go) | [Operations](user/operations.md); actual restore/fault combinations still need acceptance. |
| Storage setup UI | [Setup page](../ui/src/settings/mirabase/pageMirabaseSetup.js) | Layout planning, templates and apply controls. |
| Adoption UI | [Import page](../ui/src/settings/mirabase/pageMirabaseImport.js) | Source/plan/preview/stage/validation/cutover workflow. |
| Gateway messages/session state | [Extended protocol](../internal/pgateway/extended.go) | [PG limits](user/postgresql.md); not full PostgreSQL compatibility. |
| Native delivery | [Release builder](../tools/release/main.go) | Native digest pins/payload selection; target execution remains separate. |

## Scope that still needs complete qualification

- Managed-record CRUD currently checks database ownership/grants but directly invokes the raw record store. Complete inherited collection-rule, validation/form, hook and hidden-field behavior is not applied on that path. The normal inherited record API is a separate path. This is a release-blocking implementation gap; passing the existing isolation tests does not establish collection-policy parity.
- The workspace implementation uses local files plus owned metadata; full upstream AgentFS schema/KV/mount/overlay integration is not established. Current HTTP workspace endpoints require superusers.
- Remote SDK wrappers provide context/managed-record helpers; full inherited SDK parity and actual offline stores are not established. Dart currently uses dart:io.
- Bootstrap/page sync authority and ordinary-client offline coverage must be reconciled; record sync is not upstream native whole-DB replication.
- Application metadata and import batches are not proof of independent app auth/settings/artifact runtime isolation.
- Four operations are explicitly unregistered in capability source: application.update, application.backup, application.restore, importbatch.source.list. Preserve these obligations; do not fabricate completion.
- Native memory-budget enforcement explicitly reports unsupported in the current bridge/probe. Passing an unsupported probe does not implement hard admission.
- Native page encryption, page-key rotation and custom codecs are refused by the current create/rotation handlers. Settings protection and outbox-envelope encryption are separate; route/config presence does not implement encrypted database pages.
- The additive schema planner refuses `type.define`, `type.drop`, `view.define` and `sequence.define`. Inherited read-only view collections are a different path. Further native types need complete collection/UI/SDK integration.
- Only Windows amd64 and Linux amd64 payload files are present; other required targets have no embedded payload. Linux runtime qualification is separate from its file existing.
- Future distribution and operations capabilities are [open-source roadmap work](roadmap/platform/README.md), not implemented by local route registration.

## Contracts and verification

[Actual API registrations](user/api-reference.md), [launch results](launch/status.md), [executed feature/test matrix](../release/feature-qualification.json), [detailed contracts](mirabase/README.md). Original requirement IDs remain in [requirements and acceptance](mirabase/requirements-and-acceptance.md); their initial delivery labels do not replace current source inspection. Every obligation still needs symbol/test/run/target evidence before a full completion claim.

## Keep documentation aligned

When a feature changes, update its user workflow and [catalog entry](features.md), then the [README](../README.md) summary where relevant. Keep the [bound API reference](user/api-reference.md) tied to actual handlers and this inventory tied to real symbols/tests. Update acceptance only after recording an actual execution with its target/artifact/limitations. Design obligations remain in the specification; moving a feature to a different planning label does not remove an integrated feature or satisfy acceptance.
