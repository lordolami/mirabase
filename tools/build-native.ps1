param([string]$Target = '')
$ErrorActionPreference = 'Stop'
$repoRoot = [IO.Path]::GetFullPath((Join-Path $PSScriptRoot '..'))
$engineRoot = [IO.Path]::GetFullPath((Join-Path $repoRoot 'native/engine'))
if (-not $engineRoot.StartsWith($repoRoot + [IO.Path]::DirectorySeparatorChar)) { throw 'Engine path escaped the source root' }
$pin = 'b0e1b8fde8eff66ef9843fdd1139fa3305cb16dd'
if (-not (Test-Path -LiteralPath $engineRoot)) {
 git clone --no-checkout https://github.com/tursodatabase/turso.git $engineRoot
 if ($LASTEXITCODE -ne 0) { throw 'Engine clone failed' }
 git -C $engineRoot checkout --detach $pin
 if ($LASTEXITCODE -ne 0) { throw 'Pinned checkout failed' }
}
if ((git -C $engineRoot rev-parse HEAD).Trim() -ne $pin) { throw 'Wrong engine revision; existing checkout left untouched' }
$patchRoot = Join-Path $repoRoot 'native/patches'
$patchManifest = Get-Content -LiteralPath (Join-Path $patchRoot 'manifest.json') -Raw | ConvertFrom-Json
if ($patchManifest.engineRevision -ne $pin) { throw 'Patch set uses a different engine revision' }
foreach ($patch in $patchManifest.patches) {
 $patchPath = [IO.Path]::GetFullPath((Join-Path $patchRoot $patch.file))
 if ([IO.Path]::GetDirectoryName($patchPath) -ne $patchRoot) { throw 'Patch path escaped its directory' }
 if ((Get-FileHash -LiteralPath $patchPath -Algorithm SHA256).Hash.ToLowerInvariant() -ne $patch.sha256) { throw 'Patch digest mismatch' }
}
if (-not (git -C $engineRoot status --porcelain)) {
 foreach ($patch in $patchManifest.patches) {
  git -C $engineRoot apply -- (Join-Path $patchRoot $patch.file)
  if ($LASTEXITCODE -ne 0) { throw 'Owned patch application failed' }
 }
}
# The Cargo build script independently checks the full diff digest, staged
# changes and untracked files before it emits the embedded patch provenance.
$nativeToolchain = '1.88'
if ($IsWindows) {
 $hostArch = [Runtime.InteropServices.RuntimeInformation]::OSArchitecture.ToString()
 if ($hostArch -eq 'X64') { $nativeToolchain = '1.88-x86_64-pc-windows-msvc' }
 elseif ($hostArch -eq 'Arm64') { $nativeToolchain = '1.88-aarch64-pc-windows-msvc' }
 else { throw "Unsupported Windows build host architecture: $hostArch" }
}
$buildArgs = @("+$nativeToolchain",'build','--manifest-path',(Join-Path $repoRoot 'native/bridge/Cargo.toml'),'--release','--locked')
if ($Target) { $buildArgs += @('--target',$Target) }
Push-Location -LiteralPath (Join-Path $repoRoot 'native/bridge')
try {
 cargo @buildArgs
 if ($LASTEXITCODE -ne 0) { throw 'Native bridge build failed' }
} finally { Pop-Location }
Write-Output 'Native build completed. Run ABI/feature/fault qualification before installing the resulting library into internal/mbengine/platform/payload/<os>_<arch> and building the server.'
