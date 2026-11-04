# MiraBase native bridge

Owned Rust library consumed in-process through the Go provider; this is not another server executable. Normal users run the packaged backend without installing Rust.

From the release source root run `pwsh -File tools/build-native.ps1` to fetch the exact engine into ignored `native/engine` and build using this bridge's toolchain/lock. Path dependencies point to that engine checkout, not a sibling workspace project. The owned patch manifest in `native/patches` pins each patch and the complete engine diff. The build refuses unlisted edits, staged changes, untracked engine files, a different revision, or mismatched patch hashes. It never resets existing work. The bridge build nmanifest reports the digest of this exact patch manifest.

The build exports the owned mb_* ABI plus upstream driver symbols. Qualify lifetimes, cancellation, transaction/profile behavior, filesystem/recovery, FTS and all required operations on each target. Native memory budget enforcement currently reports unsupported; do not mark hard native admission complete from that response.

The build command selects Rust 1.88 explicitly. Windows uses the host's MSVC toolchain and requires a Developer PowerShell with the matching Visual Studio compiler and Windows SDK environment. It does not select the user's global default Rust compiler or change that default. Cross-compilation is not target execution evidence.

Only install a verified/qualified library into the matching `internal/mbengine/platform/payload/<os>_<arch>` directory, then run `go run ./tools/release` so both trusted pins match those exact bytes. Keep actual source/patch/compiler/linker/feature/SBOM manifests; no generated README or successful compile is runtime qualification.

See [ABI contract](../../docs/mirabase/native-abi-reference.md), [provider contract](../../docs/mirabase/native-provider.md) and [launch status](../../docs/launch/status.md).
