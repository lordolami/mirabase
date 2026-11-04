# Contributing to MiraBase

This repository contains the owned MiraBase server implementation. Changes must
preserve the product contracts in [docs/mirabase](docs/mirabase/README.md), the
[user documentation](docs/user/index.md), and the legal notices in `LICENSE.md`.

## Prerequisites

- Go toolchain required by `go.mod`
- Node.js and npm for dashboard changes
- Pinned native payloads for the actual build target, or the Rust/native toolchain
  to rebuild them as described in [native/bridge](native/bridge/README.md)

## Go changes

Build the real executable, not only the library root:

```sh
go run ./tools/release --version dev
go run ./tools/release --check
```

Run the built executable under `dist/` for your target. The release tool selects
`no_default_driver` and compiles the trusted payload digests. A plain inherited
`go run ./examples/base` is a comparison-provider build, not the Turso product.

Run focused tests and the affected suites using the real native provider and
required pins. Follow [launch instructions](docs/launch/README.md) for exact
invocations and nested-module, SDK, UI and fault checks. Current accepted runs
and remaining gaps are in [launch status](docs/launch/status.md).

Do not weaken, skip, or rewrite tests merely to obtain a pass. Report
environment-blocked failures with their actual output.

## Dashboard changes

The dashboard is the Vite application in `ui/` and is embedded from `ui/dist`.

```sh
cd ui
cp .env.example .env
npm ci
npm run build
```

After changing `ui/src` or `ui/.env`, rebuild the UI and then rebuild
the server through `tools/release` so it embeds the new assets. Use your shell's
equivalent of `cp` where needed. Never place credentials in the public UI env.

## Documentation

Product specifications live in `docs/mirabase/` and preserve complete target
contracts. Current source and acceptance can exceed old phase labels or leave
some target integrations unfinished; inspect current-product and launch status.
Operator and developer guides live in `docs/user/`. Keep the README, feature
catalog, affected task guides and source/evidence links aligned. Do not link the dashboard to third-party product
documentation as though it were MiraBase documentation.

## Safety

Do not modify the pinned `pocketbase/` or `turso/` reference trees. Do not
publish, deploy, create remote repositories, or send messages without explicit
authorization. Preserve user data directories and unrelated work.
