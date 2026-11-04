# Bound MiraBase HTTP reference

This inventory is extracted from actual route registrations in `apis/mirabase_*.go`. Paths below include `/api/mirabase/v1`. It is a source inventory, not an assertion that all endpoint behavior or schemas passed acceptance. Read the handler and service DTO before writing mutations. JSON fields listed are local request declarations; named service DTOs may add structure.

All endpoints use the normal authenticated group unless explicitly unbound. `superuserOnly(...)` is an operator wrapper; other handlers can have additional grant/owner/rule checks. A scope permits a path, not every row. The live OpenAPI document is useful but its static catalog must be compared with bound routes.

## Common request rules

Use current Authorization, generation/schema/policy values, stable idempotency keys where required, and exact JSON/content type. Do not infer successful completion from a 202/job acknowledgment. Poll actual jobs/results, reconcile lost responses and retain pending intent. File/stream/PG protocols have their own framing.

## Additive routes

### mirabase_admin

[Handler source](../../apis/mirabase_admin.go).

#### GET `/api/mirabase/v1/databases`

Authority: superuser middleware plus handler checks. Source: [line 168](../../apis/mirabase_admin.go#L168).
Related tests: [mirabase_admin_acceptance_test.go](../../apis/mirabase_admin_acceptance_test.go).

#### GET `/api/mirabase/v1/databases/{id}`

Authority: authenticated group plus handler-specific checks. Source: [line 191](../../apis/mirabase_admin.go#L191).
Path parameters: `id`.
Related tests: [mirabase_admin_acceptance_test.go](../../apis/mirabase_admin_acceptance_test.go).

#### POST `/api/mirabase/v1/databases`

Authority: superuser middleware plus handler checks. Source: [line 221](../../apis/mirabase_admin.go#L221).
Local JSON field names: `displayName`, `kind`, `owner`, `profile`, `templateId`, `encryption`, `codecId`.
Related tests: [mirabase_admin_acceptance_test.go](../../apis/mirabase_admin_acceptance_test.go).

#### POST `/api/mirabase/v1/databases/{id}/sync/bootstrap`

Authority: superuser middleware plus handler checks. Source: [line 309](../../apis/mirabase_admin.go#L309).
Path parameters: `id`.
Local JSON field names: `collections`, `projection`, `filterPresetId`, `knownSchemaRevision`, `clientInstanceId`.
Related tests: [mirabase_admin_acceptance_test.go](../../apis/mirabase_admin_acceptance_test.go).

#### GET `/api/mirabase/v1/databases/{id}/sync/bootstrap/{sessionId}/pages/{index}`

Authority: superuser middleware plus handler checks. Source: [line 338](../../apis/mirabase_admin.go#L338).
Path parameters: `id`, `sessionId`, `index`.
Related tests: [mirabase_admin_acceptance_test.go](../../apis/mirabase_admin_acceptance_test.go).

#### POST `/api/mirabase/v1/databases/{id}/sync/push`

Authority: authenticated group plus handler-specific checks. Source: [line 366](../../apis/mirabase_admin.go#L366).
Path parameters: `id`.
Local JSON field names: `clientInstanceId`, `schemaRevision`, `generation`, `policyEpoch`, `groups`.
Related tests: [mirabase_admin_acceptance_test.go](../../apis/mirabase_admin_acceptance_test.go).

#### GET `/api/mirabase/v1/databases/{id}/sync/pull`

Authority: authenticated group plus handler-specific checks. Source: [line 402](../../apis/mirabase_admin.go#L402).
Path parameters: `id`.
Query names read directly: `cursor`, `limit`.
Related tests: [mirabase_admin_acceptance_test.go](../../apis/mirabase_admin_acceptance_test.go).

#### POST `/api/mirabase/v1/databases/{id}/sync/ack`

Authority: authenticated group plus handler-specific checks. Source: [line 432](../../apis/mirabase_admin.go#L432).
Path parameters: `id`.
Local JSON field names: `clientInstanceId`, `cursor`.
Related tests: [mirabase_admin_acceptance_test.go](../../apis/mirabase_admin_acceptance_test.go).

#### POST `/api/mirabase/v1/databases/{id}/sync/files/stage`

Authority: authenticated group plus handler-specific checks. Source: [line 454](../../apis/mirabase_admin.go#L454).
Path parameters: `id`.
Local JSON field names: `clientInstanceId`, `files`.
Related tests: [mirabase_admin_acceptance_test.go](../../apis/mirabase_admin_acceptance_test.go).

#### PUT `/api/mirabase/v1/databases/{id}/sync/files/upload`

Authority: authenticated group plus handler-specific checks. Source: [line 472](../../apis/mirabase_admin.go#L472).
Path parameters: `id`.
Query names read directly: `path`, `sha256`, `clientInstanceId`.
Related tests: [mirabase_admin_acceptance_test.go](../../apis/mirabase_admin_acceptance_test.go).

#### POST `/api/mirabase/v1/agentfs/workspaces`

Authority: superuser middleware plus handler checks. Source: [line 501](../../apis/mirabase_admin.go#L501).
Local JSON field names: `ownerCollectionId`, `ownerRecordId`, `displayName`.
Related tests: [mirabase_admin_acceptance_test.go](../../apis/mirabase_admin_acceptance_test.go).

#### GET `/api/mirabase/v1/agentfs/workspaces`

Authority: superuser middleware plus handler checks. Source: [line 520](../../apis/mirabase_admin.go#L520).
Query names read directly: `ownerCollectionId`, `ownerRecordId`.
Related tests: [mirabase_admin_acceptance_test.go](../../apis/mirabase_admin_acceptance_test.go).

#### PUT `/api/mirabase/v1/agentfs/workspaces/{workspaceId}/files`

Authority: superuser middleware plus handler checks. Source: [line 539](../../apis/mirabase_admin.go#L539).
Path parameters: `workspaceId`.
Local JSON field names: `path`, `content`.
Related tests: [mirabase_admin_acceptance_test.go](../../apis/mirabase_admin_acceptance_test.go).

#### GET `/api/mirabase/v1/agentfs/workspaces/{workspaceId}/files`

Authority: superuser middleware plus handler checks. Source: [line 560](../../apis/mirabase_admin.go#L560).
Path parameters: `workspaceId`.
Query names read directly: `path`.
Related tests: [mirabase_admin_acceptance_test.go](../../apis/mirabase_admin_acceptance_test.go).

#### DELETE `/api/mirabase/v1/agentfs/workspaces/{workspaceId}/files`

Authority: superuser middleware plus handler checks. Source: [line 575](../../apis/mirabase_admin.go#L575).
Path parameters: `workspaceId`.
Query names read directly: `path`.
Related tests: [mirabase_admin_acceptance_test.go](../../apis/mirabase_admin_acceptance_test.go).

#### POST `/api/mirabase/v1/agentfs/workspaces/{workspaceId}/snapshots`

Authority: superuser middleware plus handler checks. Source: [line 585](../../apis/mirabase_admin.go#L585).
Path parameters: `workspaceId`.
Related tests: [mirabase_admin_acceptance_test.go](../../apis/mirabase_admin_acceptance_test.go).

#### POST `/api/mirabase/v1/agentfs/workspaces/{workspaceId}/snapshots/{snapshotId}/restore`

Authority: superuser middleware plus handler checks. Source: [line 596](../../apis/mirabase_admin.go#L596).
Path parameters: `workspaceId`, `snapshotId`.
Related tests: [mirabase_admin_acceptance_test.go](../../apis/mirabase_admin_acceptance_test.go).

#### POST `/api/mirabase/v1/databases/{id}/exports`

Authority: superuser middleware plus handler checks. Source: [line 615](../../apis/mirabase_admin.go#L615).
Path parameters: `id`.
Local JSON field names: `format`, `includeFiles`.
Related tests: [mirabase_admin_acceptance_test.go](../../apis/mirabase_admin_acceptance_test.go).

#### POST `/api/mirabase/v1/databases/{id}/imports`

Authority: superuser middleware plus handler checks. Source: [line 637](../../apis/mirabase_admin.go#L637).
Path parameters: `id`.
Local JSON field names: `archiveId`, `expectedGeneration`, `dryRun`.
Related tests: [mirabase_admin_acceptance_test.go](../../apis/mirabase_admin_acceptance_test.go).

#### POST `/api/mirabase/v1/databases/{id}/encryption/rotate`

Authority: superuser middleware plus handler checks. Source: [line 675](../../apis/mirabase_admin.go#L675).
Path parameters: `id`.
Related tests: [mirabase_admin_acceptance_test.go](../../apis/mirabase_admin_acceptance_test.go).

#### GET `/api/mirabase/v1/databases/{id}/changes`

Authority: authenticated group plus handler-specific checks. Source: [line 684](../../apis/mirabase_admin.go#L684).
Path parameters: `id`.
Query names read directly: `cursor`, `limit`.
Related tests: [mirabase_admin_acceptance_test.go](../../apis/mirabase_admin_acceptance_test.go).

#### GET `/api/mirabase/v1/databases/{id}/changes/stream`

Authority: authenticated group plus handler-specific checks. Source: [line 729](../../apis/mirabase_admin.go#L729).
Path parameters: `id`.
Query names read directly: `cursor`.
Related tests: [mirabase_admin_acceptance_test.go](../../apis/mirabase_admin_acceptance_test.go).

#### POST `/api/mirabase/v1/databases/{id}/credentials`

Authority: superuser middleware plus handler checks. Source: [line 796](../../apis/mirabase_admin.go#L796).
Path parameters: `id`.
Local JSON field names: `purpose`, `scopes`, `expiresAt`.
Related tests: [mirabase_admin_acceptance_test.go](../../apis/mirabase_admin_acceptance_test.go).

#### GET `/api/mirabase/v1/databases/{id}/credentials`

Authority: superuser middleware plus handler checks. Source: [line 825](../../apis/mirabase_admin.go#L825).
Path parameters: `id`.
Related tests: [mirabase_admin_acceptance_test.go](../../apis/mirabase_admin_acceptance_test.go).

#### POST `/api/mirabase/v1/credentials/{credentialId}/revoke`

Authority: superuser middleware plus handler checks. Source: [line 836](../../apis/mirabase_admin.go#L836).
Path parameters: `credentialId`.
Related tests: [mirabase_admin_acceptance_test.go](../../apis/mirabase_admin_acceptance_test.go).

#### DELETE `/api/mirabase/v1/databases/{id}`

Authority: superuser middleware plus handler checks. Source: [line 853](../../apis/mirabase_admin.go#L853).
Path parameters: `id`.
Local JSON field names: `expectedGeneration`.
Related tests: [mirabase_admin_acceptance_test.go](../../apis/mirabase_admin_acceptance_test.go).

#### POST `/api/mirabase/v1/databases/{id}/maintenance`

Authority: superuser middleware plus handler checks. Source: [line 911](../../apis/mirabase_admin.go#L911).
Path parameters: `id`.
Local JSON field names: `enabled`, `expectedGeneration`.
Related tests: [mirabase_admin_acceptance_test.go](../../apis/mirabase_admin_acceptance_test.go).

#### GET `/api/mirabase/v1/databases/{id}/diagnostics`

Authority: superuser middleware plus handler checks. Source: [line 960](../../apis/mirabase_admin.go#L960).
Path parameters: `id`.
Related tests: [mirabase_admin_acceptance_test.go](../../apis/mirabase_admin_acceptance_test.go).

#### POST `/api/mirabase/v1/databases/{id}/actions`

Authority: superuser middleware plus handler checks. Source: [line 1021](../../apis/mirabase_admin.go#L1021).
Path parameters: `id`.
Local JSON field names: `action`, `expectedGeneration`.
Related tests: [mirabase_admin_acceptance_test.go](../../apis/mirabase_admin_acceptance_test.go).

#### POST `/api/mirabase/v1/databases/{id}/profile`

Authority: superuser middleware plus handler checks. Source: [line 1069](../../apis/mirabase_admin.go#L1069).
Path parameters: `id`.
Local JSON field names: `expectedGeneration`, `profile`, `backupId`.
Related tests: [mirabase_admin_acceptance_test.go](../../apis/mirabase_admin_acceptance_test.go).

#### GET `/api/mirabase/v1/databases/{id}/grants`

Authority: superuser middleware plus handler checks. Source: [line 1117](../../apis/mirabase_admin.go#L1117).
Path parameters: `id`.
Related tests: [mirabase_admin_acceptance_test.go](../../apis/mirabase_admin_acceptance_test.go).

#### PUT `/api/mirabase/v1/databases/{id}/grants`

Authority: superuser middleware plus handler checks. Source: [line 1137](../../apis/mirabase_admin.go#L1137).
Path parameters: `id`.
Local JSON field names: `collectionId`, `recordId`, `subject`, `scopes`, `expiresAt`, `expectedEpoch`.
Related tests: [mirabase_admin_acceptance_test.go](../../apis/mirabase_admin_acceptance_test.go).

#### DELETE `/api/mirabase/v1/databases/{id}/grants/{grantId}`

Authority: superuser middleware plus handler checks. Source: [line 1186](../../apis/mirabase_admin.go#L1186).
Path parameters: `id`, `grantId`.
Related tests: [mirabase_admin_acceptance_test.go](../../apis/mirabase_admin_acceptance_test.go).

#### POST `/api/mirabase/v1/agents`

Authority: superuser middleware plus handler checks. Source: [line 1215](../../apis/mirabase_admin.go#L1215).
Local JSON field names: `collectionId`, `recordId`, `agent`, `owner`, `databaseId`.
Related tests: [mirabase_admin_acceptance_test.go](../../apis/mirabase_admin_acceptance_test.go).

#### GET `/api/mirabase/v1/agents`

Authority: superuser middleware plus handler checks. Source: [line 1263](../../apis/mirabase_admin.go#L1263).
Related tests: [mirabase_admin_acceptance_test.go](../../apis/mirabase_admin_acceptance_test.go).

#### DELETE `/api/mirabase/v1/agents/{agentId}`

Authority: superuser middleware plus handler checks. Source: [line 1279](../../apis/mirabase_admin.go#L1279).
Path parameters: `agentId`.
Related tests: [mirabase_admin_acceptance_test.go](../../apis/mirabase_admin_acceptance_test.go).

#### POST `/api/mirabase/v1/databases/{id}/schema/plans`

Authority: superuser middleware plus handler checks. Source: [line 1308](../../apis/mirabase_admin.go#L1308).
Path parameters: `id`.
Local JSON field names: `expectedSchemaRevision`, `operations`.
Related tests: [mirabase_admin_acceptance_test.go](../../apis/mirabase_admin_acceptance_test.go).

#### POST `/api/mirabase/v1/databases/{id}/schema/plans/{planId}/apply`

Authority: superuser middleware plus handler checks. Source: [line 1353](../../apis/mirabase_admin.go#L1353).
Path parameters: `id`, `planId`.
Local JSON field names: `planHash`, `expectedSchemaRevision`.
Related tests: [mirabase_admin_acceptance_test.go](../../apis/mirabase_admin_acceptance_test.go).

#### GET `/api/mirabase/v1/templates/{templateId}`

Authority: superuser middleware plus handler checks. Source: [line 1426](../../apis/mirabase_admin.go#L1426).
Path parameters: `templateId`.
Related tests: [mirabase_admin_acceptance_test.go](../../apis/mirabase_admin_acceptance_test.go).

#### GET `/api/mirabase/v1/jobs`

Authority: superuser middleware plus handler checks. Source: [line 1466](../../apis/mirabase_admin.go#L1466).
Related tests: [mirabase_admin_acceptance_test.go](../../apis/mirabase_admin_acceptance_test.go).

#### GET `/api/mirabase/v1/jobs/{jobId}`

Authority: superuser middleware plus handler checks. Source: [line 1490](../../apis/mirabase_admin.go#L1490).
Path parameters: `jobId`.
Related tests: [mirabase_admin_acceptance_test.go](../../apis/mirabase_admin_acceptance_test.go).

#### POST `/api/mirabase/v1/jobs/{jobId}/cancel`

Authority: superuser middleware plus handler checks. Source: [line 1506](../../apis/mirabase_admin.go#L1506).
Path parameters: `jobId`.
Related tests: [mirabase_admin_acceptance_test.go](../../apis/mirabase_admin_acceptance_test.go).

#### GET `/api/mirabase/v1/databases/{id}/receipts/{receiptId}`

Authority: authenticated group plus handler-specific checks. Source: [line 1525](../../apis/mirabase_admin.go#L1525).
Path parameters: `id`, `receiptId`.
Related tests: [mirabase_admin_acceptance_test.go](../../apis/mirabase_admin_acceptance_test.go).

#### POST `/api/mirabase/v1/databases/{id}/backups`

Authority: superuser middleware plus handler checks. Source: [line 1585](../../apis/mirabase_admin.go#L1585).
Path parameters: `id`.
Local JSON field names: `expectedGeneration`, `includeFiles`, `destination`.
Related tests: [mirabase_admin_acceptance_test.go](../../apis/mirabase_admin_acceptance_test.go).

#### POST `/api/mirabase/v1/databases/{id}/restore`

Authority: superuser middleware plus handler checks. Source: [line 1626](../../apis/mirabase_admin.go#L1626).
Path parameters: `id`.
Local JSON field names: `archiveId`, `expectedGeneration`, `dryRun`, `restoreToNew`, `authorityPolicy`, `subjects`.
Related tests: [mirabase_admin_acceptance_test.go](../../apis/mirabase_admin_acceptance_test.go).

#### POST `/api/mirabase/v1/databases/{id}/owner-transfer`

Authority: superuser middleware plus handler checks. Source: [line 1674](../../apis/mirabase_admin.go#L1674).
Path parameters: `id`.
Local JSON field names: `expectedGeneration`, `expectedPolicyEpoch`, `previousOwner`, `newOwner`, `agentPolicy`.
Related tests: [mirabase_admin_acceptance_test.go](../../apis/mirabase_admin_acceptance_test.go).

#### GET `/api/mirabase/v1/pocketbase-imports/{importId}/inventory`

Authority: superuser middleware plus handler checks. Source: [line 1754](../../apis/mirabase_admin.go#L1754).
Path parameters: `importId`.
Related tests: [mirabase_admin_acceptance_test.go](../../apis/mirabase_admin_acceptance_test.go).

#### GET `/api/mirabase/v1/pocketbase-imports/{importId}/plan`

Authority: superuser middleware plus handler checks. Source: [line 1770](../../apis/mirabase_admin.go#L1770).
Path parameters: `importId`.
Related tests: [mirabase_admin_acceptance_test.go](../../apis/mirabase_admin_acceptance_test.go).

#### GET `/api/mirabase/v1/pocketbase-imports/{importId}/preview`

Authority: superuser middleware plus handler checks. Source: [line 1786](../../apis/mirabase_admin.go#L1786).
Path parameters: `importId`.
Related tests: [mirabase_admin_acceptance_test.go](../../apis/mirabase_admin_acceptance_test.go).

#### GET `/api/mirabase/v1/pocketbase-imports/{importId}/issues`

Authority: superuser middleware plus handler checks. Source: [line 1802](../../apis/mirabase_admin.go#L1802).
Path parameters: `importId`.
Related tests: [mirabase_admin_acceptance_test.go](../../apis/mirabase_admin_acceptance_test.go).

### mirabase_capabilities

[Handler source](../../apis/mirabase_capabilities.go).

#### GET `/api/mirabase/v1/capabilities`

Authority: authenticated group plus handler-specific checks. Source: [line 18](../../apis/mirabase_capabilities.go#L18).

### mirabase_live

[Handler source](../../apis/mirabase_live.go).

#### POST `/api/mirabase/v1/databases/{id}/live-queries`

Authority: authenticated group plus handler-specific checks. Source: [line 37](../../apis/mirabase_live.go#L37).
Path parameters: `id`.
Related tests: [mirabase_live_acceptance_test.go](../../apis/mirabase_live_acceptance_test.go).

#### GET `/api/mirabase/v1/databases/{id}/live-queries/{queryId}/stream`

Authority: authenticated group plus handler-specific checks. Source: [line 76](../../apis/mirabase_live.go#L76).
Path parameters: `id`, `queryId`.
Query names read directly: `cursor`.
Related tests: [mirabase_live_acceptance_test.go](../../apis/mirabase_live_acceptance_test.go).

#### DELETE `/api/mirabase/v1/databases/{id}/live-queries/{queryId}`

Authority: authenticated group plus handler-specific checks. Source: [line 168](../../apis/mirabase_live.go#L168).
Path parameters: `id`, `queryId`.
Related tests: [mirabase_live_acceptance_test.go](../../apis/mirabase_live_acceptance_test.go).

### mirabase_openapi

[Handler source](../../apis/mirabase_openapi.go).

#### GET `/api/mirabase/v1/openapi.json`

Authority: public (group auth explicitly unbound). Source: [line 19](../../apis/mirabase_openapi.go#L19).
Related tests: [mirabase_openapi_test.go](../../apis/mirabase_openapi_test.go).

### mirabase_pbimport

[Handler source](../../apis/mirabase_pbimport.go).

#### POST `/api/mirabase/v1/pocketbase-imports`

Authority: authenticated group plus handler-specific checks. Source: [line 37](../../apis/mirabase_pbimport.go#L37).
Related tests: [mirabase_pbimport_acceptance_test.go](../../apis/mirabase_pbimport_acceptance_test.go).

#### PUT `/api/mirabase/v1/pocketbase-imports/{importId}/source`

Authority: authenticated group plus handler-specific checks. Source: [line 57](../../apis/mirabase_pbimport.go#L57).
Path parameters: `importId`.
Related tests: [mirabase_pbimport_acceptance_test.go](../../apis/mirabase_pbimport_acceptance_test.go).

#### GET `/api/mirabase/v1/pocketbase-imports/{importId}/source`

Authority: authenticated group plus handler-specific checks. Source: [line 95](../../apis/mirabase_pbimport.go#L95).
Path parameters: `importId`.
Related tests: [mirabase_pbimport_acceptance_test.go](../../apis/mirabase_pbimport_acceptance_test.go).

#### POST `/api/mirabase/v1/pocketbase-imports/{importId}/analyze`

Authority: authenticated group plus handler-specific checks. Source: [line 122](../../apis/mirabase_pbimport.go#L122).
Path parameters: `importId`.
Related tests: [mirabase_pbimport_acceptance_test.go](../../apis/mirabase_pbimport_acceptance_test.go).

#### PUT `/api/mirabase/v1/pocketbase-imports/{importId}/plan`

Authority: authenticated group plus handler-specific checks. Source: [line 138](../../apis/mirabase_pbimport.go#L138).
Path parameters: `importId`.
Related tests: [mirabase_pbimport_acceptance_test.go](../../apis/mirabase_pbimport_acceptance_test.go).

#### POST `/api/mirabase/v1/pocketbase-imports/{importId}/preview`

Authority: authenticated group plus handler-specific checks. Source: [line 155](../../apis/mirabase_pbimport.go#L155).
Path parameters: `importId`.
Related tests: [mirabase_pbimport_acceptance_test.go](../../apis/mirabase_pbimport_acceptance_test.go).

#### POST `/api/mirabase/v1/pocketbase-imports/{importId}/stage`

Authority: authenticated group plus handler-specific checks. Source: [line 168](../../apis/mirabase_pbimport.go#L168).
Path parameters: `importId`.
Related tests: [mirabase_pbimport_acceptance_test.go](../../apis/mirabase_pbimport_acceptance_test.go).

#### POST `/api/mirabase/v1/pocketbase-imports/{importId}/validate`

Authority: authenticated group plus handler-specific checks. Source: [line 181](../../apis/mirabase_pbimport.go#L181).
Path parameters: `importId`.
Related tests: [mirabase_pbimport_acceptance_test.go](../../apis/mirabase_pbimport_acceptance_test.go).

#### GET `/api/mirabase/v1/pocketbase-imports/{importId}/report`

Authority: authenticated group plus handler-specific checks. Source: [line 194](../../apis/mirabase_pbimport.go#L194).
Path parameters: `importId`.
Related tests: [mirabase_pbimport_acceptance_test.go](../../apis/mirabase_pbimport_acceptance_test.go).

#### POST `/api/mirabase/v1/pocketbase-imports/{importId}/cutover`

Authority: authenticated group plus handler-specific checks. Source: [line 218](../../apis/mirabase_pbimport.go#L218).
Path parameters: `importId`.
Local JSON field names: `expectedRevision`, `planHash`, `previewHash`, `reportHash`.
Related tests: [mirabase_pbimport_acceptance_test.go](../../apis/mirabase_pbimport_acceptance_test.go).

#### POST `/api/mirabase/v1/pocketbase-imports/{importId}/cancel`

Authority: authenticated group plus handler-specific checks. Source: [line 241](../../apis/mirabase_pbimport.go#L241).
Path parameters: `importId`.
Related tests: [mirabase_pbimport_acceptance_test.go](../../apis/mirabase_pbimport_acceptance_test.go).

#### GET `/api/mirabase/v1/pocketbase-imports/{importId}`

Authority: authenticated group plus handler-specific checks. Source: [line 257](../../apis/mirabase_pbimport.go#L257).
Path parameters: `importId`.
Related tests: [mirabase_pbimport_acceptance_test.go](../../apis/mirabase_pbimport_acceptance_test.go).

#### GET `/api/mirabase/v1/pocketbase-imports/{importId}/report/cases`

Authority: authenticated group plus handler-specific checks. Source: [line 270](../../apis/mirabase_pbimport.go#L270).
Path parameters: `importId`.
Query names read directly: `page`, `perPage`.
Related tests: [mirabase_pbimport_acceptance_test.go](../../apis/mirabase_pbimport_acceptance_test.go).

#### POST `/api/mirabase/v1/pocketbase-imports/{importId}/resume`

Authority: authenticated group plus handler-specific checks. Source: [line 295](../../apis/mirabase_pbimport.go#L295).
Path parameters: `importId`.
Local JSON field names: `expectedRevision`, `planHash`.
Related tests: [mirabase_pbimport_acceptance_test.go](../../apis/mirabase_pbimport_acceptance_test.go).

#### POST `/api/mirabase/v1/pocketbase-imports/{importId}/recovery`

Authority: authenticated group plus handler-specific checks. Source: [line 321](../../apis/mirabase_pbimport.go#L321).
Path parameters: `importId`.
Local JSON field names: `expectedRevision`, `planHash`, `action`, `expectedActiveGeneration`.
Related tests: [mirabase_pbimport_acceptance_test.go](../../apis/mirabase_pbimport_acceptance_test.go).

#### POST `/api/mirabase/v1/import-batches`

Authority: authenticated group plus handler-specific checks. Source: [line 349](../../apis/mirabase_pbimport.go#L349).
Local JSON field names: `mode`, `sourceId`, `provenance`, `sources`.
Related tests: [mirabase_pbimport_acceptance_test.go](../../apis/mirabase_pbimport_acceptance_test.go).

#### GET `/api/mirabase/v1/import-batches/{batchId}`

Authority: authenticated group plus handler-specific checks. Source: [line 370](../../apis/mirabase_pbimport.go#L370).
Path parameters: `batchId`.
Related tests: [mirabase_pbimport_acceptance_test.go](../../apis/mirabase_pbimport_acceptance_test.go).

#### GET `/api/mirabase/v1/import-batches/{batchId}/sources`

Authority: authenticated group plus handler-specific checks. Source: [line 381](../../apis/mirabase_pbimport.go#L381).
Path parameters: `batchId`.
Query names read directly: `page`, `perPage`.
Related tests: [mirabase_pbimport_acceptance_test.go](../../apis/mirabase_pbimport_acceptance_test.go).

#### POST `/api/mirabase/v1/import-batches/{batchId}/run`

Authority: authenticated group plus handler-specific checks. Source: [line 403](../../apis/mirabase_pbimport.go#L403).
Path parameters: `batchId`.
Related tests: [mirabase_pbimport_acceptance_test.go](../../apis/mirabase_pbimport_acceptance_test.go).

#### POST `/api/mirabase/v1/import-batches/{batchId}/cancel`

Authority: authenticated group plus handler-specific checks. Source: [line 414](../../apis/mirabase_pbimport.go#L414).
Path parameters: `batchId`.
Related tests: [mirabase_pbimport_acceptance_test.go](../../apis/mirabase_pbimport_acceptance_test.go).

#### GET `/api/mirabase/v1/pocketbase-imports/{importId}/tenants`

Authority: authenticated group plus handler-specific checks. Source: [line 425](../../apis/mirabase_pbimport.go#L425).
Path parameters: `importId`.
Related tests: [mirabase_pbimport_acceptance_test.go](../../apis/mirabase_pbimport_acceptance_test.go).

#### GET `/api/mirabase/v1/pocketbase-imports/{importId}/artifacts/{artifactId}`

Authority: authenticated group plus handler-specific checks. Source: [line 439](../../apis/mirabase_pbimport.go#L439).
Path parameters: `importId`, `artifactId`.
Local JSON field names: `importId`, `expectedRevision`, `planHash`, `action`, `expectedActiveGeneration`.
Related tests: [mirabase_pbimport_acceptance_test.go](../../apis/mirabase_pbimport_acceptance_test.go).

### mirabase_records

[Handler source](../../apis/mirabase_records.go).

#### GET `/api/mirabase/v1/db/{databaseId}/collections/{collection}/records`

Authority: authenticated group plus handler-specific checks. Source: [line 45](../../apis/mirabase_records.go#L45).
Path parameters: `databaseId`, `collection`.
Query names read directly: `limit`.
Related tests: [mirabase_records_journal_acceptance_test.go](../../apis/mirabase_records_journal_acceptance_test.go).

#### GET `/api/mirabase/v1/db/{databaseId}/collections/{collection}/records/{recordId}`

Authority: authenticated group plus handler-specific checks. Source: [line 63](../../apis/mirabase_records.go#L63).
Path parameters: `databaseId`, `collection`, `recordId`.
Related tests: [mirabase_records_journal_acceptance_test.go](../../apis/mirabase_records_journal_acceptance_test.go).

#### POST `/api/mirabase/v1/db/{databaseId}/collections/{collection}/records`

Authority: authenticated group plus handler-specific checks. Source: [line 75](../../apis/mirabase_records.go#L75).
Path parameters: `databaseId`, `collection`.
Related tests: [mirabase_records_journal_acceptance_test.go](../../apis/mirabase_records_journal_acceptance_test.go).

#### PATCH `/api/mirabase/v1/db/{databaseId}/collections/{collection}/records/{recordId}`

Authority: authenticated group plus handler-specific checks. Source: [line 91](../../apis/mirabase_records.go#L91).
Path parameters: `databaseId`, `collection`, `recordId`.
Related tests: [mirabase_records_journal_acceptance_test.go](../../apis/mirabase_records_journal_acceptance_test.go).

#### DELETE `/api/mirabase/v1/db/{databaseId}/collections/{collection}/records/{recordId}`

Authority: authenticated group plus handler-specific checks. Source: [line 107](../../apis/mirabase_records.go#L107).
Path parameters: `databaseId`, `collection`, `recordId`.
Related tests: [mirabase_records_journal_acceptance_test.go](../../apis/mirabase_records_journal_acceptance_test.go).

#### GET `/api/mirabase/v1/databases/{databaseId}/reference-copies/{policyId}/records`

Authority: authenticated group plus handler-specific checks. Source: [line 191](../../apis/mirabase_records.go#L191).
Path parameters: `databaseId`, `policyId`.
Related tests: [mirabase_records_journal_acceptance_test.go](../../apis/mirabase_records_journal_acceptance_test.go).

#### GET `/api/mirabase/v1/databases/{databaseId}/reference-copies/{policyId}/records/{recordId}`

Authority: authenticated group plus handler-specific checks. Source: [line 194](../../apis/mirabase_records.go#L194).
Path parameters: `databaseId`, `policyId`, `recordId`.
Related tests: [mirabase_records_journal_acceptance_test.go](../../apis/mirabase_records_journal_acceptance_test.go).

### mirabase_search

[Handler source](../../apis/mirabase_search.go).

#### POST `/api/mirabase/v1/databases/{databaseId}/collections/{collection}/search`

Authority: authenticated group plus handler-specific checks. Source: [line 30](../../apis/mirabase_search.go#L30).
Path parameters: `databaseId`, `collection`.
Related tests: [mirabase_search_acceptance_test.go](../../apis/mirabase_search_acceptance_test.go).

#### POST `/api/mirabase/v1/databases/{databaseId}/collections/{collection}/search-index`

Authority: authenticated group plus handler-specific checks. Source: [line 107](../../apis/mirabase_search.go#L107).
Path parameters: `databaseId`, `collection`.
Local JSON field names: `indexName`, `fields`, `tokenizer`, `weights`.
Related tests: [mirabase_search_acceptance_test.go](../../apis/mirabase_search_acceptance_test.go).

### mirabase_setup

[Handler source](../../apis/mirabase_setup.go).

#### GET `/api/mirabase/v1/setup`

Authority: superuser middleware plus handler checks. Source: [line 76](../../apis/mirabase_setup.go#L76).
Related tests: [mirabase_setup_acceptance_test.go](../../apis/mirabase_setup_acceptance_test.go).

#### PUT `/api/mirabase/v1/setup/plan`

Authority: superuser middleware plus handler checks. Source: [line 89](../../apis/mirabase_setup.go#L89).
Related tests: [mirabase_setup_acceptance_test.go](../../apis/mirabase_setup_acceptance_test.go).

#### POST `/api/mirabase/v1/setup/plan/apply`

Authority: superuser middleware plus handler checks. Source: [line 110](../../apis/mirabase_setup.go#L110).
Local JSON field names: `planId`, `expectedApplicationGeneration`, `expectedRevision`, `planHash`.
Related tests: [mirabase_setup_acceptance_test.go](../../apis/mirabase_setup_acceptance_test.go).

#### POST `/api/mirabase/v1/templates`

Authority: superuser middleware plus handler checks. Source: [line 138](../../apis/mirabase_setup.go#L138).
Local JSON field names: `templateId`, `collectionNames`, `expectedSchemaRevision`.
Related tests: [mirabase_setup_acceptance_test.go](../../apis/mirabase_setup_acceptance_test.go).

#### GET `/api/mirabase/v1/templates`

Authority: superuser middleware plus handler checks. Source: [line 163](../../apis/mirabase_setup.go#L163).
Related tests: [mirabase_setup_acceptance_test.go](../../apis/mirabase_setup_acceptance_test.go).

#### POST `/api/mirabase/v1/templates/{templateId}/rollouts`

Authority: superuser middleware plus handler checks. Source: [line 176](../../apis/mirabase_setup.go#L176).
Path parameters: `templateId`.
Related tests: [mirabase_setup_acceptance_test.go](../../apis/mirabase_setup_acceptance_test.go).

#### GET `/api/mirabase/v1/rollouts/{rolloutId}`

Authority: superuser middleware plus handler checks. Source: [line 198](../../apis/mirabase_setup.go#L198).
Path parameters: `rolloutId`.
Related tests: [mirabase_setup_acceptance_test.go](../../apis/mirabase_setup_acceptance_test.go).

#### POST `/api/mirabase/v1/applications`

Authority: superuser middleware plus handler checks. Source: [line 213](../../apis/mirabase_setup.go#L213).
Local JSON field names: `displayName`, `layout`.
Related tests: [mirabase_setup_acceptance_test.go](../../apis/mirabase_setup_acceptance_test.go).

#### GET `/api/mirabase/v1/applications`

Authority: superuser middleware plus handler checks. Source: [line 231](../../apis/mirabase_setup.go#L231).
Query names read directly: `page`, `perPage`.
Related tests: [mirabase_setup_acceptance_test.go](../../apis/mirabase_setup_acceptance_test.go).

#### GET `/api/mirabase/v1/applications/{appId}`

Authority: superuser middleware plus handler checks. Source: [line 257](../../apis/mirabase_setup.go#L257).
Path parameters: `appId`.
Related tests: [mirabase_setup_acceptance_test.go](../../apis/mirabase_setup_acceptance_test.go).

#### POST `/api/mirabase/v1/applications/{appId}/state`

Authority: superuser middleware plus handler checks. Source: [line 268](../../apis/mirabase_setup.go#L268).
Path parameters: `appId`.
Local JSON field names: `action`, `expectedGeneration`.
Related tests: [mirabase_setup_acceptance_test.go](../../apis/mirabase_setup_acceptance_test.go).

#### DELETE `/api/mirabase/v1/applications/{appId}`

Authority: superuser middleware plus handler checks. Source: [line 286](../../apis/mirabase_setup.go#L286).
Path parameters: `appId`.
Local JSON field names: `expectedGeneration`.
Related tests: [mirabase_setup_acceptance_test.go](../../apis/mirabase_setup_acceptance_test.go).

#### POST `/api/mirabase/v1/applications/{appId}/bootstrap`

Authority: superuser middleware plus handler checks. Source: [line 302](../../apis/mirabase_setup.go#L302).
Path parameters: `appId`.
Local JSON field names: `email`, `password`, `passwordConfirm`.
Related tests: [mirabase_setup_acceptance_test.go](../../apis/mirabase_setup_acceptance_test.go).

#### POST `/api/mirabase/v1/layout/apply`

Authority: superuser middleware plus handler checks. Source: [line 328](../../apis/mirabase_setup.go#L328).
Local JSON field names: `expectedApplicationGeneration`, `expectedRevision`, `planHash`, `destinationLayout`, `backupId`.
Related tests: [mirabase_setup_acceptance_test.go](../../apis/mirabase_setup_acceptance_test.go).

#### PUT `/api/mirabase/v1/reference-copies/{policyId}`

Authority: superuser middleware plus handler checks. Source: [line 385](../../apis/mirabase_setup.go#L385).
Path parameters: `policyId`.
Related tests: [mirabase_setup_acceptance_test.go](../../apis/mirabase_setup_acceptance_test.go).

#### GET `/api/mirabase/v1/reference-copies/{policyId}`

Authority: superuser middleware plus handler checks. Source: [line 404](../../apis/mirabase_setup.go#L404).
Path parameters: `policyId`.
Related tests: [mirabase_setup_acceptance_test.go](../../apis/mirabase_setup_acceptance_test.go).

#### POST `/api/mirabase/v1/reference-copies/{policyId}/refresh`

Authority: superuser middleware plus handler checks. Source: [line 426](../../apis/mirabase_setup.go#L426).
Path parameters: `policyId`.
Local JSON field names: `revision`.
Related tests: [mirabase_setup_acceptance_test.go](../../apis/mirabase_setup_acceptance_test.go).

#### POST `/api/mirabase/v1/live-query-plans`

Authority: superuser middleware plus handler checks. Source: [line 455](../../apis/mirabase_setup.go#L455).
Related tests: [mirabase_setup_acceptance_test.go](../../apis/mirabase_setup_acceptance_test.go).

#### GET `/api/mirabase/v1/live-query-plans`

Authority: superuser middleware plus handler checks. Source: [line 471](../../apis/mirabase_setup.go#L471).
Related tests: [mirabase_setup_acceptance_test.go](../../apis/mirabase_setup_acceptance_test.go).

#### POST `/api/mirabase/v1/live-query-plans/{planId}/revisions/{revision}/publish`

Authority: superuser middleware plus handler checks. Source: [line 483](../../apis/mirabase_setup.go#L483).
Path parameters: `planId`, `revision`.
Related tests: [mirabase_setup_acceptance_test.go](../../apis/mirabase_setup_acceptance_test.go).

#### GET `/api/mirabase/v1/live-query-plans/{planId}/revisions/{revision}`

Authority: superuser middleware plus handler checks. Source: [line 499](../../apis/mirabase_setup.go#L499).
Path parameters: `planId`, `revision`.
Related tests: [mirabase_setup_acceptance_test.go](../../apis/mirabase_setup_acceptance_test.go).

#### POST `/api/mirabase/v1/live-query-plans/{planId}/revisions/{revision}/retire`

Authority: superuser middleware plus handler checks. Source: [line 512](../../apis/mirabase_setup.go#L512).
Path parameters: `planId`, `revision`.
Related tests: [mirabase_setup_acceptance_test.go](../../apis/mirabase_setup_acceptance_test.go).

#### PUT `/api/mirabase/v1/relation-workflows/{workflowId}`

Authority: superuser middleware plus handler checks. Source: [line 527](../../apis/mirabase_setup.go#L527).
Path parameters: `workflowId`.
Related tests: [mirabase_setup_acceptance_test.go](../../apis/mirabase_setup_acceptance_test.go).

#### POST `/api/mirabase/v1/relation-workflows/{workflowId}/run`

Authority: superuser middleware plus handler checks. Source: [line 543](../../apis/mirabase_setup.go#L543).
Path parameters: `workflowId`.
Local JSON field names: `revision`.
Related tests: [mirabase_setup_acceptance_test.go](../../apis/mirabase_setup_acceptance_test.go).

#### GET `/api/mirabase/v1/relation-workflows/{workflowId}/revisions/{revision}`

Authority: superuser middleware plus handler checks. Source: [line 564](../../apis/mirabase_setup.go#L564).
Path parameters: `workflowId`, `revision`.
Related tests: [mirabase_setup_acceptance_test.go](../../apis/mirabase_setup_acceptance_test.go).

#### PUT `/api/mirabase/v1/reports/{reportId}`

Authority: superuser middleware plus handler checks. Source: [line 578](../../apis/mirabase_setup.go#L578).
Path parameters: `reportId`.
Related tests: [mirabase_setup_acceptance_test.go](../../apis/mirabase_setup_acceptance_test.go).

#### POST `/api/mirabase/v1/reports/{reportId}/rebuild`

Authority: superuser middleware plus handler checks. Source: [line 595](../../apis/mirabase_setup.go#L595).
Path parameters: `reportId`.
Related tests: [mirabase_setup_acceptance_test.go](../../apis/mirabase_setup_acceptance_test.go).

#### GET `/api/mirabase/v1/reports/{reportId}`

Authority: superuser middleware plus handler checks. Source: [line 610](../../apis/mirabase_setup.go#L610).
Path parameters: `reportId`.
Related tests: [mirabase_setup_acceptance_test.go](../../apis/mirabase_setup_acceptance_test.go).

#### POST `/api/mirabase/v1/membership-invitations`

Authority: superuser middleware plus handler checks. Source: [line 626](../../apis/mirabase_setup.go#L626).
Related tests: [mirabase_setup_acceptance_test.go](../../apis/mirabase_setup_acceptance_test.go).

#### POST `/api/mirabase/v1/membership-invitations/accept`

Authority: authenticated group plus handler-specific checks. Source: [line 656](../../apis/mirabase_setup.go#L656).
Local JSON field names: `token`, `expectedApplicationGeneration`.
Related tests: [mirabase_setup_acceptance_test.go](../../apis/mirabase_setup_acceptance_test.go).

#### DELETE `/api/mirabase/v1/membership-invitations/{inviteId}`

Authority: superuser middleware plus handler checks. Source: [line 682](../../apis/mirabase_setup.go#L682).
Path parameters: `inviteId`.
Related tests: [mirabase_setup_acceptance_test.go](../../apis/mirabase_setup_acceptance_test.go).

#### POST `/api/mirabase/v1/setup/administrator`

Authority: authenticated group plus handler-specific checks. Source: [line 707](../../apis/mirabase_setup.go#L707).
Local JSON field names: `token`, `email`, `password`, `passwordConfirm`, `expectedApplicationGeneration`.
Related tests: [mirabase_setup_acceptance_test.go](../../apis/mirabase_setup_acceptance_test.go).

#### POST `/api/mirabase/v1/rollouts/{rolloutId}/pause`

Authority: superuser middleware plus handler checks. Source: [line 772](../../apis/mirabase_setup.go#L772).
Path parameters: `rolloutId`.
Related tests: [mirabase_setup_acceptance_test.go](../../apis/mirabase_setup_acceptance_test.go).

#### POST `/api/mirabase/v1/rollouts/{rolloutId}/resume`

Authority: superuser middleware plus handler checks. Source: [line 786](../../apis/mirabase_setup.go#L786).
Path parameters: `rolloutId`.
Related tests: [mirabase_setup_acceptance_test.go](../../apis/mirabase_setup_acceptance_test.go).

#### POST `/api/mirabase/v1/rollouts/{rolloutId}/retry`

Authority: superuser middleware plus handler checks. Source: [line 808](../../apis/mirabase_setup.go#L808).
Path parameters: `rolloutId`.
Related tests: [mirabase_setup_acceptance_test.go](../../apis/mirabase_setup_acceptance_test.go).

#### GET `/api/mirabase/v1/rollouts/{rolloutId}/targets`

Authority: superuser middleware plus handler checks. Source: [line 821](../../apis/mirabase_setup.go#L821).
Path parameters: `rolloutId`.
Query names read directly: `page`, `perPage`.
Related tests: [mirabase_setup_acceptance_test.go](../../apis/mirabase_setup_acceptance_test.go).

#### POST `/api/mirabase/v1/rollouts/{rolloutId}/run`

Authority: superuser middleware plus handler checks. Source: [line 847](../../apis/mirabase_setup.go#L847).
Path parameters: `rolloutId`.
Related tests: [mirabase_setup_acceptance_test.go](../../apis/mirabase_setup_acceptance_test.go).

## Inherited routes

The pinned method/path/handler table is retained in [the detailed inherited reference](../mirabase/http-operation-reference.md#inherited-http-routes). Current inherited handlers are in [apis/](../../apis/). Preserve their auth/forms/multipart/error semantics; additive managed-record routes are not an automatic replacement for every inherited option.

## Non-HTTP surfaces

The opt-in PostgreSQL listener is documented in [PostgreSQL access](postgresql.md). No raw native-sync or SQL/HTTP service is implied unless its actual route/adapter and clients are qualified. [Agent guide](../agents/README.md) covers context, credentials and retries.
