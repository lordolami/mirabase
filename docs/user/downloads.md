# Download and run MiraBase

## Windows x64 standalone

The current download is a **release candidate**. It is unsigned and not production-qualified; see the [release notes and current limits](https://github.com/lordolami/mirabase/releases/tag/standalone-2026-11-04). The package checks below establish specific behavior, not completion of the outstanding production gates.

[Download the Windows x64 ZIP](https://github.com/lordolami/mirabase/releases/download/standalone-2026-11-04/mirabase-windows-amd64.zip) · [Release notes and all assets](https://github.com/lordolami/mirabase/releases/tag/standalone-2026-11-04) · [SHA256SUMS](https://github.com/lordolami/mirabase/releases/download/standalone-2026-11-04/SHA256SUMS)

1. Download the ZIP and extract the **whole archive** to a directory you control.
2. Double-click `start-mirabase.cmd`, or open PowerShell in that directory and run:

```powershell
.\mirabase.exe serve --http=127.0.0.1:8090
```

3. Open the private setup URL printed in the terminal. Create your first superuser.
4. Open `http://127.0.0.1:8090/_/` for the dashboard and `http://127.0.0.1:8090/` for the welcome page.
5. Create collections/auth accounts/rules, then connect your frontend. The [first application](quickstart.md) provides an exact notes example.

No Git, Go, Rust, Node, npm or external database server is needed to run this package. The current native Windows engine imports `VCRUNTIME140.dll`: the **Microsoft Visual C++ v14 x64 runtime** must be installed. If Windows reports it missing, use [Microsoft's official x64 installer](https://aka.ms/vc14/vc_redist.x64.exe), following the [Microsoft runtime guidance](https://learn.microsoft.com/en-us/cpp/windows/latest-supported-vc-redist). Do not download individual DLLs from unrelated sites. Runtime execution was checked on Windows 11 amd64; this does not establish every Windows version or ARM target.

## What is in the ZIP?

| Item | Purpose |
|---|---|
| `mirabase.exe` | Backend, dashboard and embedded verified native payload. |
| `start-mirabase.cmd` | Starts loopback service from the extracted directory. |
| `mb_public/index.html` | Welcome page pointing to setup/docs/dashboard. |
| `examples/quickstart/` | Plain HTML/JavaScript member-login and notes frontend. |
| `docs/`, README and ROADMAP | Full user guides, feature inventory and deferred work. |
| `manifest.json` | Version, executable/native/file digests and current known-limit links. |
| LICENSE/notices | Preserved project and upstream/component notices. |

The native payload is extracted to the process's private cache and verified before loading. Data remains outside the executable. Keep `mb_data` and your complete application files across upgrades/restarts. Stop with Ctrl+C. Starting from another working directory without the same explicit `--dir` can select different data.

## Run the included example

Stop the welcome-page server, then run:

```powershell
.\mirabase.exe serve --http=127.0.0.1:8090 --publicDir=./examples/quickstart
```

Follow [quick start from operator setup onward](quickstart.md#4-create-the-operator-account) to create the exact `members` and `notes` schemas/rules. Sign in at `/` with an ordinary member, create a note, then check a second account. Superuser credentials never go in the member form. The tutorial uses shared storage with record rules; additive private-DB CRUD has the [documented endpoint limits](../current-product.md).

## Verify the download

Download SHA256SUMS from the same release and compare its ZIP entry with:

```powershell
Get-FileHash .\mirabase-windows-amd64.zip -Algorithm SHA256
```

The package is unsigned. A checksum detects byte differences against the published manifest; it is not an Authenticode signature. Follow your machine's execution policy and keep security settings enabled. Signing and broader platform/integration work remain in the [roadmap](../../ROADMAP.md).

## SDK and source downloads

[JavaScript/TypeScript SDK package](https://github.com/lordolami/mirabase/releases/download/standalone-2026-11-04/mirabase-sdk-0.1.0.tgz) can be installed into your own frontend project with `npm install ./mirabase-sdk-0.1.0.tgz`. It provides the [documented context/managed-record methods](sdks.md); it is not a complete inherited auth/files/realtime or offline client. Running the server does not require this SDK.

GitHub provides source ZIP/tar downloads on the release page. [Source build instructions](quickstart.md#2-clone-and-build) remain available. This release's ready-to-run binary package is Windows amd64. Linux payload rebuild/current-candidate execution and Mac/ARM binaries remain roadmap work; source availability is not an executed package for those platforms. There is no MiraBase cloud or managed hosting service.
