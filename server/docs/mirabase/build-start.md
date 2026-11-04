# Build entry point

**Current release authority:** [current implementation inventory](../current-product.md), [user documentation](../user/index.md) and [launch status](../launch/status.md). Initial v0.1/roadmap delivery labels below preserve planning contracts; they do not establish today's implementation or permit deleting subsequently integrated features. [Open-source platform roadmap](../roadmap/platform/README.md) has separate design and gates.

**Delivery: v0.1 and roadmap.** [v0.1-build.md](v0.1-build.md) selects the current portion; [roadsmap.md](roadsmap.md) retains the remainder. Detailed schemas/configuration/ABI and full-family acceptance below describe the target superset unless narrowed by that allocation.

## Read and execute

Read the root [build instructions](../../AGENTS.md), [product contract](product-contract.md), [selected decisions](decisions.md), [architecture](architecture.md) and the [phase map](implementation-workstreams.md). Start with [v0.1-build.md](v0.1-build.md) and its requirement/surface allocation. Execute the V01 gates in the phase map. The complete design remains in the indexed chapters and requirement catalog, while [roadsmap.md](roadsmap.md) owns later delivery. Do not build roadmap services or create their catalogs merely because their target interfaces are specified.

The source reference is PocketBase v0.41.1 at f70b24e186df98df2e7ebc703f103cb94dc3ca63 in pocketbase/. Verify its clean Git status and commit before deriving the owned server tree. Do not edit that reference to make provider comparisons pass. Use Go 1.27 and record its exact patch. The engine input is b0e1b8fde8eff66ef9843fdd1139fa3305cb16dd; fetch it by commit, implement the specified bridge and keep every owned patch in native/patches with an ordered hash manifest. The patch set is an implementation output, not an external unpublished dependency. Pin Rust/linker/UI and local-client toolchains in build manifests.

## Source ownership to create

| Location | Responsibility |
|---|---|
| server/ | Owned Go module mirabase derived from PocketBase; preserve neutral core/API semantics |
| server/internal/application | Immutable app context, registry/routing, operator boundary, artifacts and lifecycle |
| server/internal/dbexec and provider packages | Native execution, pooling, transactions, policy guards and capture |
| server/internal/setup | Guided storage policy, memberships/invitations and layout migration |
| server/internal/pbimport | Bounded source acquisition, shared/split adoption, independent-source batches and publication |
| server/internal/referencecopy, relations, reports, livequery | Exact derived-data contracts and operational lifecycles |
| native/bridge and native/patches | Thin owned ABI/client glue and the minimum pinned engine patches; no Rust HTTP server |
| sdk/js, sdk/dart, sdk/offline | Branded remote clients, immutable contexts, real local-store adapters and assets |
| server/ui | Extend the existing dashboard, including complete setup and consolidation workflows |

Default startup remains one application with primary/auxiliary DB and familiar superuser setup. Additional app metadata is lazy and minimal. Use bound core.App views; never instantiate a complete per-database HTTP/scheduler/JS runtime. Application routing precedes authentication and every SQL/native/record access path carries the same immutable context.

## Design review before implementation

Read the complete [requirements and acceptance catalog](requirements-and-acceptance.md), [HTTP operation reference](http-operation-reference.md), [wire schemas](wire-schema-reference.md), [configuration](configuration-reference.md), [storage definitions](storage-format-reference.md) and [native ABI](native-abi-reference.md). Every family retains one full-product completion owner and a real acceptance outcome; its release allocation determines current implementation. Scoped families need evidence for their exact slice without being marked fully qualified.

Check application/database identity and authorization on every access path; exact parameters, types, enums and errors; engine/profile restrictions; default configuration and resource invariants; schema/format ownership; cancellation, restart and publication boundaries; and agreement between the readable chapters and detailed definitions. Verify upstream identities using [the source reference](source-reference.md). Keep PocketBase unchanged when deriving the server implementation.

Create build/test tooling as implementation work when needed. Run it against actual native libraries, databases, files, clients and UI, retaining real outcomes. A schema parser, generated interface or documentation review is not product acceptance.

## Completion rules

Treat every required capability as work to implement and verify. Advance specified -> implemented -> tested -> release-qualified only with actual symbols and evidence. Use the owning phase's target-state assertions and application-qualified authorization/failure matrix. Keep record semantics separate from privileged SQL/native access. A known engine limitation or budget miss requires an owned fix and qualification for any promised behavior, not an unsupported stub or removed feature.

Do not add cloud infrastructure, multiprocess cooperation, sharding/consensus, MCP or other-BaaS provider migration. Full shared-layout PocketBase adoption and ordinary schema migrations are current scope. Independent application consolidation, automatic populated-data splitting and AgentFS belong to the roadmap. Native/core SDK source locations are shared infrastructure; offline and workspace adapters are created only with their roadmap work.
