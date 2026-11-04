# MiraBase v0.1

**One backend. A private database for every user, tenant, and agent.**

This is the **v0.1** release. The source tag is `v0.1`; the existing artifact retains its internal build identifier `standalone-2026.11.04`. Dated download URLs are preserved so existing links continue to work.

**Standalone release.** This publishes the existing built Windows package and SDK as the standard release. The executable is unsigned, and the implementation/qualification limitations below remain. The in-progress security patch is not included in these artifacts; the existing files and checksums are preserved.

[Full hackathon build account](https://github.com/lordolami/mirabase/blob/main/docs/hackathon-build.md) · [Formal Verification companion](https://github.com/lordolami/formal-verification) · [Three-minute demo](https://youtu.be/PVRYAADs-EQ)

## Downloads

- `mirabase-windows-amd64.zip`: ready-to-run Windows x64 backend, dashboard, embedded native payload, welcome page, docs and frontend example.
- `mirabase-sdk-0.1.0.tgz`: JavaScript/TypeScript context client package.
- `SHA256SUMS`: SHA-256 for the ZIP, SDK and scoped checks report.
- `standalone-checks.json`: actual artifact, extracted-package and SDK checks.
- GitHub's source ZIP/tar archives: complete published source, including Dart SDK source.

Extract the entire Windows ZIP. Run `start-mirabase.cmd` or open PowerShell in the extracted folder:

```powershell
.\mirabase.exe serve --http=127.0.0.1:8090
```

Open the private setup URL printed in the terminal, create your superuser, then use `http://127.0.0.1:8090/_/`. No Go, Rust, Node/npm or separate database server is needed to run the package. The native library requires the Microsoft Visual C++ v14 x64 runtime; see the [download/start guide](https://github.com/lordolami/mirabase/blob/main/docs/user/downloads.md).

## What is included

PocketBase-derived collections, record APIs, auth, rules, files, realtime, email, Go/Goja hooks and dashboard; in-process Turso; optional private database contexts; grants/lifecycle/templates; text/vector/hybrid/fuzzy search; changes/record-sync/live-query APIs; references/reports; PostgreSQL gateway; operator workspace files/snapshots; imports, jobs, backups and portable data tools. See the [full feature inventory](https://github.com/lordolami/mirabase/blob/main/docs/features.md) for exact integration boundaries.

## Executed package checks

The release also fixes bundled welcome-page discovery when the archive is extracted under a temporary/cache directory. A focused Go regression preserves the development fallback and checks that packaged assets take priority.

The exact executable passed health, record CRUD/access denial, historical workspace restoration and negative controls. The extracted ZIP was checked outside the source checkout, with the backend PATH limited to Windows System32 and the VC runtime installed. The exact tutorial client passed member login/create/list/logout/re-login, unrelated-account read denial, forged-owner/owner-transfer refusal, field validation, restart persistence and frontend/dashboard serving. Default executable-relative welcome-page serving passed. All four SDK transport tests passed without failures/skips. ZIP CRC and file digests were verified.

To reproduce the extracted-package check from the source checkout, with Python and Node installed for verification only:

```powershell
python tools/check-standalone-package.py path/to/mirabase-windows-amd64.zip --output package-checks.json
```

These are named checks for this package; they do not convert historical failures, unrun platforms or deferred integrations into passes. Existing qualification records remain intact.

## Known limits and roadmap

- The Windows executable is unsigned. Verify SHA256SUMS; checksums are not an Authenticode signature.
- Managed-record endpoints check owner/grants but do not yet implement complete inherited collection-rule/form/hook/hidden-field parity. The included member app uses the normal inherited record API.
- Workspace HTTP operations require operator authority. Full AgentFS and multi-file crash-atomic restore remain additional work.
- Complete offline/native-sync delivery, further SDK/schema coverage, independent-app isolation and broader platform/resource/fault/signing work remain in the open-source roadmap. Linux/Mac/ARM ready-to-run binaries are not claimed by this Windows package.
- There is no MiraBase cloud or managed hosting offering. Future distribution/operations are open-source roadmap work.

[Start guide](https://github.com/lordolami/mirabase/blob/main/docs/user/downloads.md) · [First application](https://github.com/lordolami/mirabase/blob/main/docs/user/quickstart.md) · [Roadmap](https://github.com/lordolami/mirabase/blob/main/ROADMAP.md) · [Evidence](https://github.com/lordolami/mirabase/blob/main/release/qualification.json)
