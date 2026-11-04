# Third-party source and notices

MiraBase is derived from PocketBase. Preserve [LICENSE.md](LICENSE.md), inherited notices, neutral compatibility APIs and attribution; branding does not transfer authorship.

The pinned Turso Go driver is copied unchanged into [third_party/turso-go](third_party/turso-go/README.md), including its [license](third_party/turso-go/LICENSE.md), from engine revision `b0e1b8fde8eff66ef9843fdd1139fa3305cb16dd`. Its Go module points at the owned platform loader. The native bridge consumes the same pinned Turso Rust source when rebuilding; use its complete license/dependency manifests and record any patches.

Go dependencies are identified in `go.mod`/`go.sum`; native dependencies in `native/bridge/Cargo.toml`/`Cargo.lock`; UI and JS SDK dependencies in their package/lock files; Dart in its package manifest. Generate the complete per-artifact SBOM and license bundle during release. This notice is not a substitute for those dependency licenses or a claim that the SBOM has already been generated.

The pinned Turso engine's upstream license, NOTICE and supplemental licenses are
retained in [third_party/turso-notices](third_party/turso-notices/README.md), copied
from revision `b0e1b8fde8eff66ef9843fdd1139fa3305cb16dd`. Preserve them with the
engine integration and review the upstream NOTICE's component-specific terms.

No private signing keys, source credentials, customer data or local native cache belongs in a source archive or published package.
