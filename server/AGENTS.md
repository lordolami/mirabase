# MiraBase build instructions

This folder is the owned release source. Read its README, docs/current-product.md, docs/user/index.md, docs/launch/status.md and all affected contracts/source/tests before changing behavior. The product has implementations beyond the initial v0.1 allocation; retain those features. Initial phase/roadmap labels are design ownership, not authority to delete integrated code or assert qualification.

- Preserve pinned PocketBase and Turso audit checkouts and all legal notices. SDKs, native bridge, vendored Go driver, documentation and launch tools belong in the release source folder.
- Preserve collections, records, auth/rules, files, realtime, Go/JS hooks and the dashboard. One process serves optional tenant/user/agent DBs through bounded shared services; no process/scheduler/warm JS VM per DB.
- Resolve immutable application/target context before authority checks; record access, privileged SQL and replication remain distinct. Never bypass rules, weaken tests or substitute success-shaped stubs.
- Treat every agreed feature and safety dependency as an obligation. Separate specified, source present, implemented complete, tested and release-qualified. Record actual symbols, fixtures, target/toolchain/digests, executions and limitations.
- Current source inventory and launch evidence supersede stale marketing claims. Full AgentFS parity, every SDK/platform and native protocol interoperability require actual integration and acceptance; names/config/routes alone do not prove them.
- There is no MiraBase-hosted cloud or managed hosting offering. Future distribution/operations are open-source roadmap design in docs/roadmap/platform. Do not modify the standalone into a consensus engine or claim roadmap work is implemented. No MCP, Mongo frontend or other-BaaS adapter is implied.
- Run relevant real authorization, concurrency, cancellation, recovery, schema, disk and packaging checks. Do not classify skipped, blocked, zero-test, unsupported or documentation-only outcomes as product acceptance.
- Preserve user data, existing changes and unrelated projects. Do not publish/deploy/create remote repositories/send messages without explicit authorization.
