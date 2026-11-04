# Build and use your first MiraBase application

This tutorial takes a fresh checkout through build, startup, dashboard setup, ordinary-member login, record creation and a second-account isolation check. It uses the included [HTML/JavaScript frontend](../../examples/quickstart/README.md) and normal inherited record API. No frontend package installation is needed.

**Windows package available:** [download and run](downloads.md) without building from source, then continue here from operator setup. To serve this example from the extracted ZIP, run `.\mirabase.exe serve --http=127.0.0.1:8090 --publicDir=./examples/quickstart`. Source-building steps below are optional for package users. Check [launch status](../launch/status.md) for exact evidence and limitations. This example uses shared storage with per-record owner rules; a physical DB per user/tenant/agent is a separate [storage layout](databases.md).

## 1. Install and check tools

Install Git and Go 1.27.x, as required by `go.mod`, using [Git downloads](https://git-scm.com/downloads) and [Go downloads](https://go.dev/dl/). Reopen your terminal after installation and check:

```sh
git --version
go version
```

| Platform | Current development path |
|---|---|
| Windows x64/amd64 | Included native payload; clean-checkout build/start evidence exists. Use PowerShell below. |
| Linux x86-64/amd64 | Included older payload references GLIBC_2.38; recorded baseline is Ubuntu/WSL with glibc 2.43. Current full candidate remains unqualified. |
| macOS / ARM | No matching native payload currently supplied; missing targets fail explicitly. |

On Linux, inspect `uname -m` and `ldd --version`. Alpine/musl and older glibc are not covered. See [platform details](production.md). Rust, a C compiler and Node/npm are not needed to use the included compatible native payload and built dashboard; rebuilding those components needs separate toolchains.

## 2. Clone and build

```sh
git clone https://github.com/lordolami/mirabase.git
cd mirabase
go run ./tools/release --version dev
```

In the public checkout, run `cd server` from the repository root before the remaining source-build commands. This application directory contains `go.mod`, `tools`, `docs` and `examples`. Downloaded standalone packages use their packaged layout. A first Go build needs network access for module dependencies. Upgrade an old Go toolchain rather than lowering `go.mod`.

Expected output includes `Built dist/...`. The executable and adjacent `.manifest.json` are under `dist`; the manifest records build facts separately from full qualification:

- Windows: `mirabase-windows-amd64.exe`.
- Linux: `mirabase-linux-amd64`.

The release builder selects `no_default_driver` and both trusted payload digest pins. Plain `go run ./examples/base` selects the inherited comparison provider, not this Turso product build. `--check` runs the full test invocation and is not needed merely to launch the tutorial.

## 3. Start the server

Windows PowerShell:

```powershell
.\dist\mirabase-windows-amd64.exe serve --http=127.0.0.1:8090 --publicDir=./examples/quickstart
```

Compatible Linux amd64:

```sh
./dist/mirabase-linux-amd64 serve --http=127.0.0.1:8090 --publicDir=./examples/quickstart
```

Keep this terminal running. It serves loopback only. Persistent data defaults to `mb_data` in the working directory. The explicit public directory serves the included example without copying/deleting your own public files.

Open [health](http://127.0.0.1:8090/api/health); expect HTTP 200 with JSON. If 8090 is occupied, choose an unused port and use that port in every URL below. Do not stop an unrelated process.

## 4. Create the operator account

Open the private first-superuser setup URL printed in this server's terminal, create an operator with your own email/password, then open [the dashboard](http://127.0.0.1:8090/_/). Existing data with a superuser requires its existing login rather than a new first-install flow.

The operator administers the backend. Keep its credentials/setup link out of frontend source, public screenshots and logs. The tutorial's member form is for ordinary accounts, not this operator.

## 5. Create members

In Collections choose **New collection**, type **Auth**, name **members**. Keep password auth enabled with email as identity. No additional member fields are needed. Keep the list/view/create/update/delete rules locked: the operator creates tutorial accounts in the dashboard; public signup/profile editing is not part of this example. Password login is a separate auth operation.

Create two records in `members`:

| Member | Example email | Password |
|---|---|---|
| Alice | `alice@example.test` | Choose your own password satisfying field restrictions. |
| Bob | `bob@example.test` | Choose a different password. |

These domains are for local test accounts. Do not configure email delivery to them or reuse tutorial credentials for real accounts. Leave MFA/verification requirements at tutorial defaults; if you enable those policies, complete them before expecting login to work. See [authentication](authentication.md).

## 6. Create notes and rules

Create a **Base** collection named **notes**:

| Field | Type | Configuration |
|---|---|---|
| `title` | Text | Required; maximum length 200. |
| `body` | Text | Optional. |
| `owner` | Relation | Collection `members`; required; maximum select 1. |
| `created` | Autodate | Creation timestamp; retain/create for the example's `-created` sorting. |

Retain the normal ID and any update timestamp fields. In **API rules**, enter these exact expressions and save. Empty/public and locked rules differ from these expressions.

| Rule | Expression |
|---|---|
| List | `@request.auth.id != "" && owner = @request.auth.id` |
| View | `@request.auth.id != "" && owner = @request.auth.id` |
| Create | `@request.auth.id != "" && owner = @request.auth.id` |
| Update | `@request.auth.id != "" && owner = @request.auth.id && @request.body.owner:changed = false` |
| Delete | `@request.auth.id != "" && owner = @request.auth.id` |

The owner relation stores the member record ID. Rules enforce ownership; a relation by itself does not. Update additionally prevents an ordinary user from changing the owner. An operator has separate administrative authority.

## 7. Use the frontend

Open [the application](http://127.0.0.1:8090/), sign in as Alice's **member** account, enter title/body and choose **Save note**. The list refreshes with the saved note.

The real [client](../../examples/quickstart/client.mjs) calls:

1. `POST /api/collections/members/auth-with-password` to sign in.
2. `POST /api/collections/notes/records` with the ordinary token and owner from the returned member ID.
3. `GET /api/collections/notes/records?page=1&perPage=20&sort=-created` to list authorized notes.

The [frontend wiring](../../examples/quickstart/app.mjs) uses this same client. Token state is memory-only: reload signs out. This example has no CDN, public signup, refresh/recovery flows, full pagination or offline store. It displays the first 20 notes and renders returned content with textContent.

## 8. Check another account and persistence

Sign out and sign in as Bob. Bob's list should not contain Alice's note. Save a Bob note, then sign in as Alice: Alice should see her own note and not Bob's. A superuser seeing both in the dashboard is expected and does not test ordinary-user access.

For API checks, request Alice's note ID using Bob's token: expect HTTP 404. A forged create assigning Bob's note to Alice should fail, as should an ordinary update changing owner. An unauthorized list may return HTTP 200 with an empty list; do not require every rule refusal to be 401/403.

Stop with Ctrl+C and restart the same command from the same working/data directory. Sign in and verify the saved note persists. Keep the complete data root; deleting it is not a normal restart/upgrade.

## 9. Add optional private databases

This tutorial uses one shared physical store. A separate DB per individual, team/company or agent uses [storage setup](databases.md), published templates, owner/membership bindings and context resolution. A tenant is a shared team context, not a synonym for an individual user.

Current additive managed CRUD has incomplete inherited rule/form/hook/hidden-field parity, and ordinary-client offline bootstrap is unfinished. Do not convert this example by blindly replacing its record URLs or giving the browser operator credentials. Read [current implementation limits](../current-product.md) before evaluating those paths.

## Troubleshooting

| Symptom | Check |
|---|---|
| `git`/`go` not found | Install and reopen the terminal. |
| Go/module syntax error | Use Go 1.27.x; check `go version`. |
| Missing `go.mod` | Work at the public repository root. |
| Missing native payload | Match OS/architecture to current supported payloads. |
| GLIBC loader error | Actual Linux runtime compatibility, not just Go compilation. |
| Pin/digest error | Rebuild with `tools/release`; do not bypass verification or load arbitrary libraries. |
| Address in use | Select an unused loopback port and update browser URLs. |
| Frontend missing but dashboard works | Set `--publicDir=./examples/quickstart` from repository root. |
| Login fails | Use a `members` account; inspect password/verification/MFA policy. |
| Save fails | Exact field names, owner relation, rules, limits and returned validation errors. |
| List fails | Check `created` field exists and actual actor/rules. |
| Reload signs out | Intentional memory-only token state. |

## Continue building

[Feature catalog](../features.md) · [Databases](databases.md) · [Files](files.md) · [Search](search.md) · [Realtime](realtime.md) · [Schemas](schema-and-workflows.md) · [SDKs](sdks.md) · [Import](pocketbase-import.md) · [Operations](operations.md)

Before public hosting, configure TLS/proxy, real identity/recovery mail, trusted origins/proxies, resource limits, backup and isolated restore. Check unrelated accounts and protected files against the actual candidate. The local tutorial does not qualify production behavior or every optional feature.
