# Embedded engine payload: windows_amd64

Artifact: `mirabase_engine.dll`. Bytes: 29828608. SHA-256: `fb0e6862e11ba39a33bf5b739f01576ac499eff1455318d1af1a9abc3ed0cc70`.

The source engine identity is pinned in the owned provider to `b0e1b8fde8eff66ef9843fdd1139fa3305cb16dd`, with the exact owned patch set in [the patch manifest](../../../../../native/patches/manifest.json). The Windows payload includes the source-schema refresh for concurrent `VACUUM INTO`; the Linux payload has not yet been rebuilt with that patch. The release builder compiles this actual file digest into both trusted loader/provider pins. An adjacent checksum is not authorization to modify the library.

Payload presence does not establish rebuild reproducibility or target qualification. See [launch evidence](../../../../../docs/launch/status.md) and [native bridge](../../../../../native/bridge/README.md). Complete compiler/linker/feature/license/SBOM and target execution evidence is required before publication. Never replace a payload with a mismatched engine or silent fallback.
