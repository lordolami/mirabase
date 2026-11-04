# Authentication and security

**Current release authority:** [current implementation inventory](../current-product.md), [user documentation](../user/index.md) and [launch status](../launch/status.md). Initial v0.1/roadmap delivery labels below preserve planning contracts; they do not establish today's implementation or permit deleting subsequently integrated features. [Open-source platform roadmap](../roadmap/platform/README.md) has separate design and gates.

**Delivery: v0.1 and roadmap.** [v0.1-build.md](v0.1-build.md) selects the current portion; [roadsmap.md](roadsmap.md) retains the remainder. Detailed schemas/configuration/ABI and full-family acceptance below describe the target superset unless narrowed by that allocation.

## Inherited identity contract

Preserve auth collections, _superusers, password/OAuth2/OTP/MFA, refresh, verification/reset/email-change flows, impersonation, auth alerts, token durations, auth/manage rules, hidden password/token fields, rate limits and superuser IP restrictions. Password validation and bcrypt behavior remain inherited. Imported raw hashes are trusted server input; never accept a bcrypt string as a login password substitute.

Local startup still uses the inherited superuser setup and permissions. Loopback does not become universally trusted. Do not disable authentication because an AI agent or frontend runs on the same machine.

## New grants and credentials

Ordinary bearer tokens identify an auth record. Database grants bind that subject to databaseId, generation-independent resource identity, explicit scopes, expiry and policy epoch. Revocation invalidates new operations immediately and closes active streaming sessions at their next bounded check (default maximum 5 seconds). In-flight mutation commit checks and policy-epoch publication share a process-local policy guard: either the authorized commit precedes revocation publication, or the stale operation is rejected/cancelled. Never assert that already committed data is rolled back by revocation. No token contains a filesystem path or encryption key.

Native/PG credentials are separately issued restricted credentials for whole-database SQL/replication. Default maximum lifetime is one hour; operator-created long-lived credentials require an explicit expiry policy and rotation support. Store only verifiers or encrypted signing material using the inherited secret settings boundary. Credential listing never returns secrets after creation.

Credential rows bind the issued database generation as well as purpose, scopes and policy epoch. Verify that generation against the resolved target for PG password lookup and native/SQL-HTTP JWT use. Restore/replacement cannot leave an old SCRAM credential usable merely because its ID and password verifier survived the archive. Reissue is explicit after the new generation is ready.

Global schema changes, raw administrative SQL, backups/restores, settings and grants require superuser authorization in this scope. Ordinary data access still requires collection rules even when a database scope is granted. Authenticated agents have ordinary subject identities and least-privilege grants.

## Authorization matrix

| Path | Required boundary |
|---|---|
| Record CRUD/search/batch | Explicit private DB grant or reviewed current tenant-membership channel scope, plus inherited collection rules/forms/hooks |
| Files/thumbs | Context-bound file access + protected-file policy; no path fallback |
| Realtime/changes/record sync | Grant + current rule/field policy; recheck on delivery and push |
| Dashboard/settings/schema/lifecycle | Existing superuser authorization |
| SQL console/PG | Explicit whole-DB SQL authority; not a normal user's record token |
| Native replication/partial pages | Whole-DB replica authority; page access is never row filtering |
| Export/backup/restore/key changes | Superuser + target context; secrets/keys excluded from ordinary exports |
| Go/JS extensions | Trusted operator-installed code with documented server capabilities |

PocketBase tenant import preserves canonical identities and adapts source memberships/rules as defined in pocketbase-tenant-import.md. Membership-derived scopes cover only normal record/file/change/record-sync paths and are re-evaluated against current membership. They cannot authorize SQL/native whole-database access, schema changes or lifecycle administration. Anonymous/public tenant behavior must be explicitly selected and verified in the import plan.

Private SQL credentials are confined to one database. Reject filesystem-oriented ATTACH, DETACH misuse, VACUUM INTO arbitrary paths, extension loading, uncontrolled PRAGMAs and file/network virtual tables unless using a global trusted administrative channel with explicit allowlists. Parse syntax and enforce native capability gates; keyword filtering alone is insufficient. Bind variables do not make an arbitrary SQL program safe.

## Network and browser security

The inherited HTTP API remains available behind direct TLS or a trusted TLS-terminating reverse proxy. Public deployment guidance requires TLS, correct trusted-proxy CIDRs, secure cookie/token handling, restricted admin access and configured CORS/origin behavior. Avoid enabling a wildcard credentialed browser origin. Forwarded headers from an untrusted client are not trusted.

Optional PG/native listeners default disabled and loopback-only when enabled. External bind requires configured authentication and TLS or an explicit authenticated TLS gateway whose enforcement is verified at startup. Trust-only native PG code is never exposed directly. Rate/body/concurrency/time limits apply separately to SQL, sync, file and authentication work.

## Secrets and encryption

Keep inherited settings encryption and its operator key mechanism; document its scope. Native page encryption uses key references resolved by the operator secret provider/environment/file, not key bytes in API responses, logs, manifests or backup metadata. File secret providers require owner-only permissions. Secrets are redacted before structured logging and crash diagnostics.

Algorithm policy lists every native AES-GCM/AEGIS option with platform support. Default selection is a qualified modern authenticated cipher; algorithm availability is not automatic qualification. Rotation is maintenance + verified backup + re-encryption/export-import + key verification + atomic publication. Never discard the old key before a recovery-tested new copy exists. Key loss means unrecoverable data where no alternative key/copy exists.

## Threat and evidence

Test confused deputy/context reuse, forged owners, colliding auth IDs, revoked streams, stale epochs, native SQL filesystem escape, auth-field leakage, MIME/content spoofing, malicious filters, ZIP traversal, symlink/reparse paths, compromised payload lookup and secret logging. Never describe trusted Go/JS extensions as an untrusted sandbox. Security claims need negative tests on every access path, not just REST.



## Application-qualified authority

Resolve the application before authentication. App-local superusers, secrets, membership, tokens and SQL/native credentials never grant instance authority. Duplicate source signing domains must be rekeyed/revoked under the reviewed import policy. See [application-qualified authority](application-isolation.md).
