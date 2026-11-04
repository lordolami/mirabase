# Embedded engine payload: linux_amd64

Artifact: `mirabase_engine.so`. Bytes: 32938344. SHA-256: `409617922e2fcdb711beb94f76cea2343e3d5dba8690d20b569b80277c86e89c`.

The source engine identity is pinned in the owned provider to `b0e1b8fde8eff66ef9843fdd1139fa3305cb16dd`. The release builder compiles this actual file digest into both trusted loader/provider pins. An adjacent checksum is not authorization to modify the library.

Payload presence does not establish rebuild reproducibility or target qualification. See [launch evidence](../../../../../docs/launch/status.md) and [native bridge](../../../../../native/bridge/README.md). Complete compiler/linker/feature/license/SBOM and target execution evidence is required before publication. Never replace a payload with a mismatched engine or silent fallback.
