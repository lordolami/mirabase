# Branding and compatibility

**Current release authority:** [current implementation inventory](../current-product.md), [user documentation](../user/index.md) and [launch status](../launch/status.md). Initial v0.1/roadmap delivery labels below preserve planning contracts; they do not establish today's implementation or permit deleting subsequently integrated features. [Managed cloud](../cloud/README.md) has separate design and gates.

**Delivery: v0.1 and roadmap.** [v0.1-build.md](v0.1-build.md) selects the current portion; [roadsmap.md](roadsmap.md) retains the remainder. Detailed schemas/configuration/ABI and full-family acceptance below describe the target superset unless narrowed by that allocation.

## Public identity

The product name is MiraBase; the executable is mirabase. This is a fork with preserved provenance, not an assertion that the upstream project is ours. Public downloads, console banners, setup links, dashboard title, favicon, application icons, default mail identity, generated API examples, SDK examples, support links, release notes and update metadata must present MiraBase. Native Turso diagnostics can retain engine names when they identify the component involved.

Fresh applications use mb_data, mb_hooks, mb_migrations and mb_public. Environment variables introduced by this fork start MIRABASE_. The default port remains 8090 and the existing dashboard stays at /_/. Existing neutral paths, JSON keys, field type names, event names, rule syntax and ordinary SDK method names remain stable. New APIs use /api/mirabase/v1.

## Rename inventory

| Surface | Required treatment |
|---|---|
| Executable, CLI usage and version | mirabase; report product, engine revision and build capabilities separately |
| Go module/imports | Module mirabase throughout server source, examples and generators; source/replace distribution works without a public registry |
| Go package names | mirabase where the upstream root name is public; preserve core, apis, tools and other neutral package names |
| JS/Dart SDK package identity | @mirabase/sdk package archive and mirabase Dart source/path package; preserve client methods; registry publication is optional |
| Generated JS types and migration scaffolds | MiraBase examples, correct package references and mb_* defaults |
| Hooks/migrations discovery | .mb.js/.mb.ts for new hook files; documented legacy .pb.js/.pb.ts discovery in compatibility mode |
| Data/public/hooks/migration paths | mb_* for new projects; explicit legacy path selection for existing projects |
| Dashboard assets | Replace wordmarks, favicon, install/PWA metadata, titles, footer, external links, accessible names and default application label |
| Email and templates | Replace product defaults; preserve the operator's configured application name and templates |
| Update mechanism | Only a signed MiraBase release source; never update a MiraBase executable into PocketBase |
| Container/service/examples | MiraBase image names, labels, paths, sample service descriptions, probes and README text |
| Licenses and source history | Keep upstream copyright/license notices and commit attribution intact |
| Protocol/storage identifiers | Preserve identifiers required by SQLite, Turso, PostgreSQL and legacy imports; explain intentional compatibility values |
| Errors/logs | MiraBase product messages; component names retained when diagnostically useful; never rewrite user-provided data |

A branding scan reports every case-insensitive PocketBase/pocketbase/pb_ occurrence. Each retained occurrence needs a reason in the branding registry: attribution, upstream evidence, historical fixture, import compatibility or maintained alias. A blanket exclusion of source directories is not an acceptable scan.

## Existing project adoption

The existing schema-only collection import stays intact. Full PocketBase adoption separately offers shared layout or conversion into tenant databases, including data/files, ownership, account/membership/rule adaptation, routing preview and recoverable cutover. Follow [the complete tenant import contract](pocketbase-tenant-import.md); the other-BaaS migration deferral does not apply to it.

The compatibility import is a copy-and-verify operation, never an in-place engine experiment on the only source copy. Inputs are the pinned PocketBase data directory, hooks, migrations, public files and operator configuration. Inspect SQLite integrity, user_version, schema, indexes, triggers, view definitions, extensions, file references, settings encryption and current migration history. Unsupported SQL or codecs produce a report with affected objects and no cutover.

Copy a coherent stopped instance or verified backup. Import the primary and auxiliary databases using the supported Turso conversion path; preserve record/collection IDs, bcrypt hashes, token keys, timestamps, rules, indexes, file names, verification state and migration history. Secrets remain secrets. Boot the copy with outbound mail and external jobs disabled until the operator reviews the adoption report. Exercise existing API/SDK fixtures against both copies, compare normalized records and file hashes, then make cutover explicit. The original is retained.

Discovery never creates a new empty mb_data next to an existing pb_data without warning. If both paths exist and no --dir was supplied, refuse ambiguous startup. Legacy hooks and new hooks with the same logical name are a configuration error rather than double execution. Imports generate a path map and rollback instructions.

## Compatibility boundaries

The foundation is v0.41.1, not every historical PocketBase release. Older projects first need a documented supported upstream upgrade or a separately qualified importer. Existing ordinary JS and Dart clients must keep working for inherited endpoints. MiraBase additions require the extended SDK or direct HTTP.

Settings-encryption envelopes and native page encryption are different layers. A legacy environment name can be accepted as a one-time documented alias, with no secret printed in its warning. The stable alias window is one complete MiraBase major compatibility line; removing it requires an explicit breaking release and an adoption tool.

No new source rename is claimed complete by this document. Acceptance requires a real scan, built CLI/UI/SDK artifacts, import fixtures, preserved legal notices and updater negative tests.



## Branding and publication rules

```json
{
  "formatVersion": 1,
  "product": "MiraBase",
  "executable": "mirabase",
  "freshDirectories": [
    "mb_data",
    "mb_hooks",
    "mb_migrations",
    "mb_public"
  ],
  "newEnvironmentPrefix": "MIRABASE_",
  "publicApiPrefix": "/api/mirabase/v1",
  "preserve": [
    "/api/collections",
    "/api/files",
    "/api/realtime",
    "/_/",
    "$app",
    "routerAdd",
    "routerUse",
    "cronAdd",
    "cronRemove"
  ],
  "legacyAliases": [
    "pb_data",
    "pb_hooks",
    "pb_migrations",
    "pb_public",
    ".pb.js",
    ".pb.ts"
  ],
  "allowedLegacyReasons": [
    "upstream-attribution",
    "source-evidence",
    "historical-fixture",
    "explicit-import-compatibility",
    "maintained-alias"
  ],
  "publication": {
    "goModule": "mirabase",
    "goDistribution": "source module with local replace",
    "jsPackage": "@mirabase/sdk",
    "jsDistribution": "package archive",
    "dartPackage": "mirabase",
    "dartDistribution": "source/path package",
    "downloadOrigin": "relative release artifacts; configured feed",
    "updatePublicKey": "generated and embedded during protected release provisioning",
    "status": "identities selected; no public registry publication or ownership claimed"
  }
}
```
