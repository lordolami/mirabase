# Future platform operator journey and dashboard

**Status:** designed interfaces, not a published console or deployed API.

This journey describes an optional managed deployment of future open-source platform components. It is roadmap work, not current service availability. Today's indie developer starts with the [standalone quick start](../user/quickstart.md); no cloud signup is required. See [scope](README.md) and [open-source roadmap](../../ROADMAP.md).

## Start cheaply

Create an account, project and application; choose home region and resource class. Default shared storage uses the ordinary MiraBase setup. Add user/tenant/agent databases through the same reviewed policies. No database needs its own container or paid fixed instance.

The stable application endpoint/custom domain hides placement changes. Frontends keep collection/auth/file/realtime methods; SDKs use the current token and immutable context. PG/native/sync capabilities are advertised only after their own compatibility qualification. DNS/custom-domain cutover and certificate issuance are explicit states.

## Graduate from self-hosted

1. Preflight source engine/profile/schema/artifacts/keys, supported features, file/S3 coverage and desired region. Produce a compatibility report; reject incompatible contracts without silently disabling them.
2. Take a coherent source snapshot/export and keep original/recovery copies. Transfer using resumable content hashes and scoped short-lived upload credentials.
3. Stage in an isolated destination. Retain application/DB/record IDs and auth/rule semantics where compatible; explicitly remap service credentials/origins/URLs. Custom Go/JS hooks require the managed isolation class and artifact review.
4. Validate real schema/counts/hashes, login, membership, rules, protected files, search, live/sync boundaries and required native formats. Disable outgoing historical effects.
5. Fence source writes; apply the qualified final delta or perform a declared maintenance snapshot. Validate final committed boundary, publish destination generation/routing and switch endpoint/custom domain.
6. Reissue invalidated credentials/cursors, resume clients and observe. Preserve original, migration receipt and recovery export; never overwrite destination accepted writes by restarting an old source snapshot.

Same experience means compatible contracts, not literal zero configuration. Region, endpoint/origins, credentials and isolation can need explicit changes. Reverse export/self-hosted exit uses the same verified data/artifact/key boundary; customers must retain a workable exit path.

## Dashboard pages

Account/security/SSO where implemented; projects; applications/environments; database inventory/search; context record/schema/file browser; templates/rollouts; users/agents/grants; service/PG/native credentials; search/live plans; changes/sync clients/lag; workspace files/snapshots; imports/migration jobs; placement/region/replicas; backup/PITR/restore; usage/quotas/billing; logs/audit/alerts; support/status; custom domains/network rules; upgrade history.

Each page requires backend permissions, exact job state, error/retry/recovery handling and real target data. A mock console is not cloud completion. Ordinary app users must not see operator placement/secrets or unrelated project usage.

## API contract to implement

Public hosted control-plane endpoints use a separate versioned namespace from collection data APIs. Families: projects/apps/environments, DB lifecycle, region/placement changes, replicas, backups/PITR, migration sessions, domains/network rules, credentials/policies, metrics/usage/quotas/billing and jobs/events.

Mutations require project authority, idempotency and expected revision/generation; lists are paginated; jobs expose committed steps and recovery; usage has a watermark/provisional indicator. Responses never return provider root credentials. Webhooks are signed and deduplicated with destination/SSRF policy.

Existing standalone API names are preserved on application endpoints. Platform billing/placement permission does not silently grant collection read authority. Human account membership and app end-user authentication remain distinct.
