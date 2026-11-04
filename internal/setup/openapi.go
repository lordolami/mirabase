package setup

import (
	"encoding/json"
	"sort"
	"strconv"
	"strings"
)

var _ = strconv.Itoa

// OpenAPIDocument generates the OpenAPI 3.1 document for the released
// /api/mirabase/v1 surface from the ACTUAL registered operations (api-
// and-sdk.md: "Release tooling emits combined OpenAPI ... and capability
// manifest"). The operations mirror the bound routes; the additive error
// envelope is the shared response schema.
func OpenAPIDocument(engineRevision string) ([]byte, error) {
	ops := releasedOperations()
	doc := map[string]any{
		"openapi": "3.1.0",
		"info": map[string]any{
			"title":       "MiraBase",
			"version":     "0.41.1",
			"description": "MiraBase additive API (/api/mirabase/v1). Engine revision " + engineRevision + ". Released operations only; unregistered roadmap selectors return capability_unavailable.",
			"license":     map[string]any{"name": "MIT"},
		},
		"servers": []map[string]any{
			{"url": "/api/mirabase/v1"},
		},
		"tags": []map[string]any{
			{"name": "setup"}, {"name": "databases"}, {"name": "records"},
			{"name": "search"}, {"name": "templates"}, {"name": "imports"},
			{"name": "capabilities"}, {"name": "live"},
		},
		"components": map[string]any{
			"securitySchemes": map[string]any{
				"AuthToken": map[string]any{
					"type":        "apiKey",
					"in":          "header",
					"name":        "Authorization",
					"description": "The inherited record/superuser auth token (raw).",
				},
				"IdempotencyKey": map[string]any{
					"type":        "apiKey",
					"in":          "header",
					"name":        "Idempotency-Key",
					"description": "16..128 printable ASCII, no whitespace; required on mutations.",
				},
			},
			"schemas": map[string]any{
				"MirabaseError": map[string]any{
					"type": "object",
					"properties": map[string]any{
						"status":  map[string]any{"type": "integer"},
						"message": map[string]any{"type": "string"},
						"data": map[string]any{
							"type": "object",
							"properties": map[string]any{
								"mirabase": map[string]any{
									"type": "object",
									"properties": map[string]any{
										"code": map[string]any{
											"type": "string",
											"enum": []string{
												"invalid_request", "unauthenticated", "forbidden", "not_found",
												"generation_mismatch", "schema_mismatch", "policy_changed",
												"capability_unavailable", "incompatible_profile", "conflict",
												"idempotency_conflict", "cursor_expired", "resync_required",
												"resource_exhausted", "maintenance", "storage_failure",
												"outcome_unknown", "layout_migration_required",
												"source_data_required", "source_changed", "source_incomplete",
												"plan_stale", "unresolved_import_issues", "report_stale",
											},
										},
										"retryable":  map[string]any{"type": "boolean"},
										"requestId":  map[string]any{"type": "string"},
										"databaseId": map[string]any{"type": "string"},
										"committed": map[string]any{
											"type": "boolean", "nullable": true,
											"description": "true, false or null for an unknown outcome.",
										},
										"fieldErrors": map[string]any{"type": "object", "additionalProperties": map[string]any{"type": "string"}},
										"recovery":    map[string]any{"type": "string"},
									},
								},
							},
						},
					},
				},
				"SearchResult": map[string]any{
					"type":     "object",
					"required": []string{"databaseId", "generation", "schemaRevision", "items", "mode", "metric", "complete", "scannedCount", "elapsedMs", "nextCursor"},
					"properties": map[string]any{
						"databaseId":     map[string]any{"type": "string"},
						"generation":     map[string]any{"type": "string", "description": "positive 64-bit integer as decimal text"},
						"schemaRevision": map[string]any{"type": "string"},
						"items": map[string]any{"type": "array", "maxItems": 100, "items": map[string]any{
							"type": "object",
							"properties": map[string]any{
								"record":     map[string]any{"type": "object"},
								"score":      map[string]any{"type": "object", "properties": map[string]any{"kind": map[string]any{"type": "string"}, "value": map[string]any{"type": "number"}}},
								"highlights": map[string]any{"type": "array", "items": map[string]any{"type": "object", "properties": map[string]any{"field": map[string]any{"type": "string"}, "fragments": map[string]any{"type": "array", "maxItems": 20, "items": map[string]any{"type": "string"}}}}},
							},
						}},
						"mode":         map[string]any{"type": "string", "enum": []string{"vector", "text", "hybrid", "fuzzy"}},
						"metric":       map[string]any{"type": "string", "nullable": true},
						"complete":     map[string]any{"type": "boolean"},
						"scannedCount": map[string]any{"type": "integer", "minimum": 0},
						"elapsedMs":    map[string]any{"type": "number", "minimum": 0},
						"nextCursor":   map[string]any{"type": "string", "nullable": true},
					},
				},
			},
		},
		"paths": map[string]any{},
	}
	paths := doc["paths"].(map[string]any)
	for _, op := range ops {
		item, ok := paths[op.Path]
		if !ok {
			item = map[string]any{}
			paths[op.Path] = item
		}
		pathItem := item.(map[string]any)
		pathItem[strings.ToLower(op.Method)] = map[string]any{
			"operationId": op.OperationID,
			"tags":        []string{op.Tag},
			"summary":     op.Summary,
			"security":    op.Security,
			"requestBody": op.RequestBody,
			"responses": map[string]any{
				strconv.Itoa(op.SuccessStatus): map[string]any{
					"description": op.SuccessDescription,
				},
				"default": map[string]any{
					"description": "The additive error envelope",
					"content": map[string]any{
						"application/json": map[string]any{
							"schema": map[string]any{"$ref": "#/components/schemas/MirabaseError"},
						},
					},
				},
			},
		}
	}
	return json.MarshalIndent(doc, "", "  ")
}

// ReleasedOp describes one released operation in the generated document.
type ReleasedOp struct {
	Method             string
	Path               string
	OperationID        string
	Tag                string
	Summary            string
	Security           []map[string]any
	RequestBody        map[string]any
	SuccessStatus      int
	SuccessDescription string
}

// releasedOperations lists the released /api/mirabase/v1 operations
// exactly as bound (mirabase_records/setup/search/pbimport/capabilities).
func releasedOperations() []ReleasedOp {
	auth := []map[string]any{{"AuthToken": []string{}}}
	superuser := []map[string]any{{"AuthToken": []string{}}}
	_ = auth
	body := func(props map[string]any, required []string) map[string]any {
		return map[string]any{
			"required": true,
			"content": map[string]any{
				"application/json": map[string]any{
					"schema": map[string]any{
						"type": "object", "properties": props, "required": required,
					},
				},
			},
		}
	}
	ops := []ReleasedOp{
		{Method: "GET", Path: "/capabilities", OperationID: "capabilities", Tag: "capabilities",
			Summary: "The released operation catalog, engine revision and build capabilities.", Security: auth, SuccessStatus: 200, SuccessDescription: "The capability manifest"},
		{Method: "GET", Path: "/setup", OperationID: "setup.get", Tag: "setup",
			Summary: "Read the current application storage policy and caller-owned setup draft.", Security: superuser, SuccessStatus: 200, SuccessDescription: "SetupState"},
		{Method: "PUT", Path: "/setup/plan", OperationID: "setup.plan", Tag: "setup",
			Summary: "Validate the storage plan and produce the immutable preview without live layout effects.", Security: superuser, SuccessStatus: 200, SuccessDescription: "SetupPlan",
			RequestBody: body(map[string]any{
				"expectedApplicationGeneration": map[string]any{"type": "string"},
				"expectedRevision":              map[string]any{"type": "string"},
				"layout":                        map[string]any{"type": "string", "enum": []string{"shared", "user", "tenant", "agent"}},
				"authCollectionIds":             map[string]any{"type": "array", "items": map[string]any{"type": "string"}},
				"templateId":                    map[string]any{"type": "string", "nullable": true},
				"provision":                     map[string]any{"type": "string", "enum": []string{"lazy", "eager"}},
				"namespace":                     map[string]any{"type": "string", "nullable": true},
				"placements":                    map[string]any{"type": "array", "items": map[string]any{"type": "object"}},
			}, []string{"expectedApplicationGeneration", "expectedRevision", "layout", "authCollectionIds", "provision", "placements"})},
		{Method: "POST", Path: "/setup/plan/apply", OperationID: "setup.apply", Tag: "setup",
			Summary: "Apply the exact reviewed setup and publish the policy through the guarded durable path.", Security: superuser, SuccessStatus: 202, SuccessDescription: "SetupPlan (publication accepted)",
			RequestBody: body(map[string]any{
				"planId":                        map[string]any{"type": "string"},
				"expectedApplicationGeneration": map[string]any{"type": "string"},
				"expectedRevision":              map[string]any{"type": "string"},
				"planHash":                      map[string]any{"type": "string", "pattern": "^[a-f0-9]{64}$"},
			}, []string{"planId", "expectedApplicationGeneration", "expectedRevision", "planHash"})},
		{Method: "POST", Path: "/templates", OperationID: "template.publish", Tag: "templates",
			Summary: "Freeze a new immutable template revision.", Security: superuser, SuccessStatus: 200, SuccessDescription: "Template",
			RequestBody: body(map[string]any{
				"templateId":      map[string]any{"type": "string"},
				"collectionNames": map[string]any{"type": "array", "items": map[string]any{"type": "string"}},
			}, []string{"templateId", "collectionNames"})},
		{Method: "GET", Path: "/templates", OperationID: "template.list", Tag: "templates",
			Summary: "List published template revisions.", Security: superuser, SuccessStatus: 200, SuccessDescription: "TemplatePage"},
		{Method: "POST", Path: "/templates/{templateId}/rollouts", OperationID: "template.rollout", Tag: "templates",
			Summary: "Snapshot the explicit target set and run bounded per-context migrations.", Security: superuser, SuccessStatus: 202, SuccessDescription: "Rollout (accepted)",
			RequestBody: body(map[string]any{
				"revision":      map[string]any{"type": "string"},
				"databaseIds":   map[string]any{"type": "array", "maxItems": 10000, "items": map[string]any{"type": "string"}},
				"stopOnFailure": map[string]any{"type": "boolean"},
			}, []string{"revision", "stopOnFailure"})},
		{Method: "GET", Path: "/rollouts/{rolloutId}", OperationID: "rollout.status", Tag: "templates",
			Summary: "Rollout status: total/succeeded/failed/remaining with exact per-target outcomes.", Security: superuser, SuccessStatus: 200, SuccessDescription: "RolloutStatus"},
		{Method: "POST", Path: "/rollouts/{rolloutId}/run", OperationID: "rollout.run", Tag: "templates",
			Summary: "Drive the bounded fan-out (resumable through the journal).", Security: superuser, SuccessStatus: 200, SuccessDescription: "RolloutSummary"},
		{Method: "POST", Path: "/agentfs/workspaces", OperationID: "agentfs.workspace.create", Tag: "agentfs",
			Summary: "Create one AgentFS workspace for the owner identity (real directories, durable rows).", Security: superuser, SuccessStatus: 200, SuccessDescription: "AgentWorkspace",
			RequestBody: body(map[string]any{
				"ownerCollectionId": map[string]any{"type": "string"},
				"ownerRecordId":     map[string]any{"type": "string"},
				"displayName":       map[string]any{"type": "string"},
			}, []string{"ownerCollectionId", "ownerRecordId", "displayName"})},
		{Method: "GET", Path: "/agentfs/workspaces", OperationID: "agentfs.workspace.list", Tag: "agentfs",
			Summary: "List the owner's workspaces.", Security: superuser, SuccessStatus: 200, SuccessDescription: "AgentWorkspaceList"},
		{Method: "PUT", Path: "/agentfs/workspaces/{workspaceId}/files", OperationID: "agentfs.file.write", Tag: "agentfs",
			Summary: "Write one file with versioned metadata and digest (path-safety enforced).", Security: superuser, SuccessStatus: 200, SuccessDescription: "AgentFile",
			RequestBody: body(map[string]any{
				"path":    map[string]any{"type": "string"},
				"content": map[string]any{"type": "string"},
			}, []string{"path", "content"})},
		{Method: "GET", Path: "/agentfs/workspaces/{workspaceId}/files", OperationID: "agentfs.file.read", Tag: "agentfs",
			Summary: "Read one file after the registered digest verify.", Security: superuser, SuccessStatus: 200, SuccessDescription: "AgentFile"},
		{Method: "DELETE", Path: "/agentfs/workspaces/{workspaceId}/files", OperationID: "agentfs.file.delete", Tag: "agentfs",
			Summary: "Remove one file (the deletion is recorded; history preserves versions).", Security: superuser, SuccessStatus: 204, SuccessDescription: "Deleted"},
		{Method: "POST", Path: "/agentfs/workspaces/{workspaceId}/snapshots", OperationID: "agentfs.snapshot.create", Tag: "agentfs",
			Summary: "Snapshot the workspace (the manifest hashes every active file).", Security: superuser, SuccessStatus: 200, SuccessDescription: "AgentSnapshot"},
		{Method: "POST", Path: "/agentfs/workspaces/{workspaceId}/snapshots/{snapshotId}/restore", OperationID: "agentfs.snapshot.restore", Tag: "agentfs",
			Summary: "Restore the snapshot as an overlay (unchanged files verify; changed/missing listed explicitly).", Security: superuser, SuccessStatus: 200, SuccessDescription: "SnapshotRestore"},
		{Method: "POST", Path: "/applications", OperationID: "application.create", Tag: "applications",
			Summary: "Register and provision one independent application (per-app data dir/identity/schema/settings in ONE process).", Security: superuser, SuccessStatus: 200, SuccessDescription: "Application",
			RequestBody: body(map[string]any{
				"displayName": map[string]any{"type": "string"},
				"layout":      map[string]any{"type": "string", "enum": []string{"shared", "user", "tenant", "agent"}},
			}, []string{"displayName", "layout"})},
		{Method: "GET", Path: "/applications", OperationID: "application.list", Tag: "applications",
			Summary: "Paginated applications registry.", Security: superuser, SuccessStatus: 200, SuccessDescription: "ApplicationPage"},
		{Method: "GET", Path: "/applications/{appId}", OperationID: "application.get", Tag: "applications",
			Summary: "Read one application.", Security: superuser, SuccessStatus: 200, SuccessDescription: "Application"},
		{Method: "POST", Path: "/applications/{appId}/state", OperationID: "application.state", Tag: "applications",
			Summary: "Maintenance/suspend/resume transitions (suspend closes the live instance; data retained).", Security: superuser, SuccessStatus: 200, SuccessDescription: "Application",
			RequestBody: body(map[string]any{
				"action":             map[string]any{"type": "string", "enum": []string{"maintenance", "suspend", "resume"}},
				"expectedGeneration": map[string]any{"type": "integer"},
			}, []string{"action", "expectedGeneration"})},
		{Method: "DELETE", Path: "/applications/{appId}", OperationID: "application.delete", Tag: "applications",
			Summary: "Tombstone the application (data retained; never deleted on a stale snapshot).", Security: superuser, SuccessStatus: 204, SuccessDescription: "Deleted",
			RequestBody: body(map[string]any{"expectedGeneration": map[string]any{"type": "integer"}}, []string{"expectedGeneration"})},
		{Method: "POST", Path: "/applications/{appId}/bootstrap", OperationID: "application.bootstrap", Tag: "applications",
			Summary: "Create the app's first administrator through inherited forms (app-local superusers).", Security: superuser, SuccessStatus: 200, SuccessDescription: "BootstrapComplete",
			RequestBody: body(map[string]any{
				"email":           map[string]any{"type": "string", "format": "email"},
				"password":        map[string]any{"type": "string", "minLength": 8},
				"passwordConfirm": map[string]any{"type": "string", "minLength": 8},
			}, []string{"email", "password", "passwordConfirm"})},
		{Method: "POST", Path: "/layout/apply", OperationID: "layout.apply", Tag: "layout",
			Summary: "Stage the verified layout conversion (populated relocation is refused with layout_migration_required).", Security: superuser, SuccessStatus: 202, SuccessDescription: "Job"},
		{Method: "POST", Path: "/databases/{id}/exports", OperationID: "export", Tag: "portable",
			Summary: "Create a stable-ID typed manifest and data archive (portable-v1 or native).", Security: superuser, SuccessStatus: 202, SuccessDescription: "Job",
			RequestBody: body(map[string]any{
				"format":       map[string]any{"type": "string", "enum": []string{"portable-v1", "native"}},
				"includeFiles": map[string]any{"type": "boolean"},
			}, []string{"format", "includeFiles"})},
		{Method: "POST", Path: "/databases/{id}/imports", OperationID: "import", Tag: "portable",
			Summary: "Validate the portable schema/data/files preserving IDs with an explicit plan (dryRun supported).", Security: superuser, SuccessStatus: 202, SuccessDescription: "Job",
			RequestBody: body(map[string]any{
				"archiveId":          map[string]any{"type": "string"},
				"expectedGeneration": map[string]any{"type": "integer"},
				"dryRun":             map[string]any{"type": "boolean"},
			}, []string{"archiveId", "expectedGeneration", "dryRun"})},
		{Method: "POST", Path: "/databases/{id}/encryption/rotate", OperationID: "key.rotate", Tag: "security",
			Summary: "Page encryption/key rotation — the native-security roadmap remainder; the released surface refuses with the honest reason.", Security: superuser, SuccessStatus: 400, SuccessDescription: "Refusal (roadmap remainder)"},
		{Method: "GET", Path: "/databases/{id}/changes", OperationID: "changes.pull", Tag: "cdc",
			Summary: "Read complete authorized commit frames after a cursor (unauthorized tables are redacted to evict markers).", Security: auth, SuccessStatus: 200, SuccessDescription: "ChangePage"},
		{Method: "POST", Path: "/databases/{id}/sync/bootstrap", OperationID: "sync.bootstrap", Tag: "sync",
			Summary: "Create an authorized consistent snapshot lease with immutable pages.", Security: superuser, SuccessStatus: 200, SuccessDescription: "Bootstrap",
			RequestBody: body(map[string]any{
				"collections":         map[string]any{"type": "array", "maxItems": 100, "items": map[string]any{"type": "string"}},
				"projection":          map[string]any{"type": "array", "maxItems": 100, "items": map[string]any{"type": "string"}},
				"filterPresetId":      map[string]any{"type": "string", "nullable": true},
				"knownSchemaRevision": map[string]any{"type": "integer", "nullable": true},
				"clientInstanceId":    map[string]any{"type": "string"},
			}, []string{"collections", "projection", "filterPresetId", "knownSchemaRevision", "clientInstanceId"})},
		{Method: "GET", Path: "/databases/{id}/sync/bootstrap/{sessionId}/pages/{index}", OperationID: "sync.page", Tag: "sync",
			Summary: "Read one authorized immutable bootstrap page (expired leases read nothing).", Security: superuser, SuccessStatus: 200, SuccessDescription: "BootstrapPage"},
		{Method: "POST", Path: "/databases/{id}/sync/push", OperationID: "sync.push", Tag: "sync",
			Summary: "Validate and commit each mutation group with receipts (preserving client IDs; version guards).", Security: auth, SuccessStatus: 200, SuccessDescription: "PushResult",
			RequestBody: body(map[string]any{
				"clientInstanceId": map[string]any{"type": "string"},
				"schemaRevision":   map[string]any{"type": "integer"},
				"generation":       map[string]any{"type": "integer"},
				"policyEpoch":      map[string]any{"type": "integer"},
				"groups":           map[string]any{"type": "array", "maxItems": 20, "items": map[string]any{"type": "object"}},
			}, []string{"clientInstanceId", "schemaRevision", "generation", "policyEpoch", "groups"})},
		{Method: "GET", Path: "/databases/{id}/sync/pull", OperationID: "sync.pull", Tag: "sync",
			Summary: "Read the authorized change page from the durable journal.", Security: auth, SuccessStatus: 200, SuccessDescription: "ChangePage"},
		{Method: "POST", Path: "/databases/{id}/sync/ack", OperationID: "sync.ack", Tag: "sync",
			Summary: "Persist the acknowledgement no later than the applied authorized cursor (beyond-journal refused).", Security: auth, SuccessStatus: 200, SuccessDescription: "Ack",
			RequestBody: body(map[string]any{
				"clientInstanceId": map[string]any{"type": "string"},
				"cursor":           map[string]any{"type": "string"},
			}, []string{"clientInstanceId", "cursor"})},
		{Method: "POST", Path: "/databases/{id}/sync/files/stage", OperationID: "sync.file.stage", Tag: "sync",
			Summary: "Record the staged file upload intents (path + sha256).", Security: superuser, SuccessStatus: 200, SuccessDescription: "StagedFiles",
			RequestBody: body(map[string]any{
				"clientInstanceId": map[string]any{"type": "string"},
				"files":            map[string]any{"type": "array", "items": map[string]any{"type": "object"}},
			}, []string{"clientInstanceId", "files"})},
		{Method: "PUT", Path: "/databases/{id}/sync/files/upload", OperationID: "sync.file.upload", Tag: "sync",
			Summary: "Write one staged file's bytes (digest-verified; staging root enforced).", Security: superuser, SuccessStatus: 200, SuccessDescription: "UploadedFile"},
		{Method: "GET", Path: "/databases/{id}/changes/stream", OperationID: "changes.stream", Tag: "cdc",
			Summary: "Stream the authorized commit frames (SSE over the pull loop).", Security: auth, SuccessStatus: 200, SuccessDescription: "ChangePage stream"},
		{Method: "POST", Path: "/databases/{id}/credentials", OperationID: "credential.issue", Tag: "credentials",
			Summary: "Issue one-time secret; retain verifier and audit only (generation-bound, expiring).", Security: superuser, SuccessStatus: 200, SuccessDescription: "Credential",
			RequestBody: body(map[string]any{
				"purpose":   map[string]any{"type": "string", "enum": []string{"pg", "native-sync", "sql-http"}},
				"scopes":    map[string]any{"type": "array", "maxItems": 10, "items": map[string]any{"type": "string"}},
				"expiresAt": map[string]any{"type": "string"},
			}, []string{"purpose", "scopes", "expiresAt"})},
		{Method: "GET", Path: "/databases/{id}/credentials", OperationID: "credential.list", Tag: "credentials",
			Summary: "List the credential metadata without secrets.", Security: superuser, SuccessStatus: 200, SuccessDescription: "CredentialList"},
		{Method: "POST", Path: "/credentials/{credentialId}/revoke", OperationID: "credential.revoke", Tag: "credentials",
			Summary: "Revoke one credential.", Security: superuser, SuccessStatus: 204, SuccessDescription: "Revoked"},
		{Method: "PUT", Path: "/reference-copies/{policyId}", OperationID: "reference.define", Tag: "reference",
			Summary: "Publish a validated nonsecret projection policy and schedule generation-bound refresh.", Security: superuser, SuccessStatus: 200, SuccessDescription: "ReferencePolicy"},
		{Method: "GET", Path: "/reference-copies/{policyId}", OperationID: "reference.status", Tag: "reference",
			Summary: "Durable per-target applied sequence, health and refresh failure counts.", Security: superuser, SuccessStatus: 200, SuccessDescription: "ReferenceStatus"},
		{Method: "POST", Path: "/reference-copies/{policyId}/refresh", OperationID: "reference.refresh", Tag: "reference",
			Summary: "Refresh real source snapshots and apply idempotent updates/deletes with target checkpoints.", Security: superuser, SuccessStatus: 202, SuccessDescription: "Job"},
		{Method: "GET", Path: "/databases/{databaseId}/reference-copies/{policyId}/records", OperationID: "reference.readRecords", Tag: "reference",
			Summary: "Read a healthy reference copy's projected rows (reference_stale past the declared lag).", SuccessStatus: 200, SuccessDescription: "ReferenceCopyRows"},
		{Method: "GET", Path: "/databases/{databaseId}/reference-copies/{policyId}/records/{recordId}", OperationID: "reference.readRecord", Tag: "reference",
			Summary: "Read one projected row of a healthy reference copy.", SuccessStatus: 200, SuccessDescription: "ReferenceCopyRows"},
		{Method: "POST", Path: "/live-query-plans", OperationID: "liveplan.create", Tag: "liveplan",
			Summary: "Prepare and validate an immutable named read-only query revision with policy injection.", Security: superuser, SuccessStatus: 200, SuccessDescription: "LivePlan"},
		{Method: "GET", Path: "/live-query-plans", OperationID: "liveplan.list", Tag: "liveplan",
			Summary: "List the named live query plan revisions.", Security: superuser, SuccessStatus: 200, SuccessDescription: "LivePlanList"},
		{Method: "GET", Path: "/live-query-plans/{planId}/revisions/{revision}", OperationID: "liveplan.get", Tag: "liveplan",
			Summary: "Read the exact parameterized query revision and publication state.", Security: superuser, SuccessStatus: 200, SuccessDescription: "LivePlan"},
		{Method: "POST", Path: "/live-query-plans/{planId}/revisions/{revision}/publish", OperationID: "liveplan.publish", Tag: "liveplan",
			Summary: "Publish the draft revision (retires the previously published revision).", Security: superuser, SuccessStatus: 200, SuccessDescription: "LivePlan"},
		{Method: "POST", Path: "/live-query-plans/{planId}/revisions/{revision}/retire", OperationID: "liveplan.retire", Tag: "liveplan",
			Summary: "Retire a published revision.", Security: superuser, SuccessStatus: 200, SuccessDescription: "LivePlan"},
		{Method: "POST", Path: "/databases/{id}/live-queries", OperationID: "live.register", Tag: "live",
			Summary: "Create an authorized snapshot/diff subscription over a published live plan (record-authenticated; the caller must own the database or hold a live grant carrying record.read).", Security: auth, SuccessStatus: 200, SuccessDescription: "LiveQuery",
			RequestBody: body(map[string]any{
				"queryId":        map[string]any{"type": "string", "description": "the published live plan name"},
				"parameters":     map[string]any{"type": "array", "maxItems": 100, "items": map[string]any{}},
				"schemaRevision": map[string]any{"type": "integer"},
				"queryRevision":  map[string]any{"type": "integer", "description": "optional; 0 selects the latest published revision"},
			}, []string{"queryId", "parameters", "schemaRevision"})},
		{Method: "GET", Path: "/databases/{id}/live-queries/{queryId}/stream", OperationID: "live.stream", Tag: "live",
			Summary: "Deliver ordered authorized query deltas (SSE; the required cursor binds subscription, actor, context and sequence; clients reconnect with the cursor rather than assuming no losses).", Security: auth, SuccessStatus: 200, SuccessDescription: "Snapshot/delta/replay/reset event stream (text/event-stream; cursor query parameter required)"},
		{Method: "DELETE", Path: "/databases/{id}/live-queries/{queryId}", OperationID: "live.cancel", Tag: "live",
			Summary: "Release the subscription and native resources idempotently.", Security: auth, SuccessStatus: 200, SuccessDescription: "{id, closed:true}"},
		{Method: "PUT", Path: "/relation-workflows/{workflowId}", OperationID: "relationworkflow.define", Tag: "workflow",
			Summary: "Validate explicit same-application qualified cascade steps and guarded service principal.", Security: superuser, SuccessStatus: 200, SuccessDescription: "RelationWorkflow"},
		{Method: "GET", Path: "/relation-workflows/{workflowId}/revisions/{revision}", OperationID: "relationworkflow.get", Tag: "workflow",
			Summary: "Read one workflow revision.", Security: superuser, SuccessStatus: 200, SuccessDescription: "RelationWorkflow"},
		{Method: "POST", Path: "/relation-workflows/{workflowId}/run", OperationID: "relationworkflow.run", Tag: "workflow",
			Summary: "Execute deterministic per-context cascade steps with visible partial results and version guards.", Security: superuser, SuccessStatus: 202, SuccessDescription: "Job",
			RequestBody: body(map[string]any{"revision": map[string]any{"type": "integer"}}, []string{"revision"})},
		{Method: "PUT", Path: "/reports/{reportId}", OperationID: "report.define", Tag: "reports",
			Summary: "Publish the approved projected aggregate and current output access rule.", Security: superuser, SuccessStatus: 200, SuccessDescription: "ReportDefinition"},
		{Method: "GET", Path: "/reports/{reportId}", OperationID: "report.get", Tag: "reports",
			Summary: "Read one report definition and its rebuild checkpoint.", Security: superuser, SuccessStatus: 200, SuccessDescription: "ReportDefinition"},
		{Method: "POST", Path: "/reports/{reportId}/rebuild", OperationID: "report.rebuild", Tag: "reports",
			Summary: "Rebuild with bounded per-source checkpoints and authorized output.", Security: superuser, SuccessStatus: 202, SuccessDescription: "Job"},
		{Method: "POST", Path: "/membership-invitations", OperationID: "membership.invite", Tag: "membership",
			Summary: "Persist a hashed scoped invite and enqueue mail without arbitrary role or owner elevation.", Security: superuser, SuccessStatus: 200, SuccessDescription: "Invitation",
			RequestBody: body(map[string]any{
				"namespace":        map[string]any{"type": "string"},
				"tenantKey":        map[string]any{"type": "object", "nullable": true},
				"email":            map[string]any{"type": "string", "nullable": true},
				"subject":          map[string]any{"type": "object", "nullable": true, "properties": map[string]any{"collectionId": map[string]any{"type": "string"}, "recordId": map[string]any{"type": "string"}}},
				"membershipValues": map[string]any{"type": "array", "maxItems": 100, "items": map[string]any{"type": "object"}},
				"expiresInSeconds": map[string]any{"type": "integer", "minimum": 60, "maximum": 604800},
			}, nil)},
		{Method: "POST", Path: "/membership-invitations/accept", OperationID: "membership.accept", Tag: "membership",
			Summary: "Validate the account/tenant policy and atomically consume the invite with one membership.", Security: auth, SuccessStatus: 200, SuccessDescription: "Invitation",
			RequestBody: body(map[string]any{
				"token":                         map[string]any{"type": "string"},
				"expectedApplicationGeneration": map[string]any{"type": "integer"},
			}, []string{"token", "expectedApplicationGeneration"})},
		{Method: "DELETE", Path: "/membership-invitations/{inviteId}", OperationID: "membership.revoke", Tag: "membership",
			Summary: "Revoke only the selected authorized tenant invitation before use.", Security: superuser, SuccessStatus: 200, SuccessDescription: "Invitation"},
		{Method: "POST", Path: "/setup/administrator", OperationID: "setup.administrator", Tag: "setup",
			Summary: "Create the first application administrator through inherited forms with a durable bootstrap receipt and consume the scoped token.", Security: nil, SuccessStatus: 200, SuccessDescription: "BootstrapComplete",
			RequestBody: body(map[string]any{
				"token":                         map[string]any{"type": "string"},
				"email":                         map[string]any{"type": "string", "format": "email"},
				"password":                      map[string]any{"type": "string", "minLength": 8},
				"passwordConfirm":               map[string]any{"type": "string", "minLength": 8},
				"expectedApplicationGeneration": map[string]any{"type": "integer"},
			}, []string{"token", "email", "password", "passwordConfirm", "expectedApplicationGeneration"})},
		{Method: "POST", Path: "/rollouts/{rolloutId}/pause", OperationID: "rollout.pause", Tag: "templates",
			Summary: "Stop new target admission; running targets complete and the journal records them.", Security: superuser, SuccessStatus: 200, SuccessDescription: "RolloutStatus"},
		{Method: "POST", Path: "/rollouts/{rolloutId}/resume", OperationID: "rollout.resume", Tag: "templates",
			Summary: "Clear the pause and revalidate each remaining target's current generation against the snapshot.", Security: superuser, SuccessStatus: 200, SuccessDescription: "RolloutStatus"},
		{Method: "POST", Path: "/rollouts/{rolloutId}/retry", OperationID: "rollout.retry", Tag: "templates",
			Summary: "Re-arm only failed targets (succeeded targets are never migrated twice).", Security: superuser, SuccessStatus: 200, SuccessDescription: "RolloutStatus"},
		{Method: "GET", Path: "/rollouts/{rolloutId}/targets", OperationID: "rollout.targets", Tag: "templates",
			Summary: "The exact paginated per-target journal (state, result reference, error code).", Security: superuser, SuccessStatus: 200, SuccessDescription: "RolloutTargetPage"},
		{Method: "GET", Path: "/db/{databaseId}/collections/{collection}/records", OperationID: "db.recordList", Tag: "records",
			Summary: "List records in the resolved database context.", Security: auth, SuccessStatus: 200, SuccessDescription: "RecordPage"},
		{Method: "GET", Path: "/db/{databaseId}/collections/{collection}/records/{recordId}", OperationID: "db.recordGet", Tag: "records",
			Summary: "Read one record.", Security: auth, SuccessStatus: 200, SuccessDescription: "Record"},
		{Method: "POST", Path: "/db/{databaseId}/collections/{collection}/records", OperationID: "db.recordCreate", Tag: "records",
			Summary: "Create a record (server-generated id).", Security: auth, SuccessStatus: 200, SuccessDescription: "Record",
			RequestBody: body(map[string]any{"field values": map[string]any{"type": "object"}}, nil)},
		{Method: "PATCH", Path: "/db/{databaseId}/collections/{collection}/records/{recordId}", OperationID: "db.recordUpdate", Tag: "records",
			Summary: "Patch a record.", Security: auth, SuccessStatus: 200, SuccessDescription: "Record",
			RequestBody: body(map[string]any{"field values": map[string]any{"type": "object"}}, nil)},
		{Method: "DELETE", Path: "/db/{databaseId}/collections/{collection}/records/{recordId}", OperationID: "db.recordDelete", Tag: "records",
			Summary: "Delete a record.", Security: auth, SuccessStatus: 204, SuccessDescription: "Deleted"},
		{Method: "POST", Path: "/databases/{databaseId}/collections/{collection}/search", OperationID: "search", Tag: "search",
			Summary: "Authorized search over the released modes: exact vector, native FTS text, reciprocal-rank-fusion hybrid and the bounded fuzzy field scan.", Security: auth, SuccessStatus: 200, SuccessDescription: "SearchResult",
			RequestBody: body(map[string]any{
				"mode": map[string]any{"type": "string", "enum": []string{"vector", "text", "hybrid", "fuzzy"}},
				"vector": map[string]any{"type": "object", "properties": map[string]any{
					"field":  map[string]any{"type": "string"},
					"query":  map[string]any{"description": "dense finite number array or sparse {dimensions,indices,values}"},
					"metric": map[string]any{"type": "string", "enum": []string{"cosine", "l2", "negative_dot", "jaccard"}},
				}},
				"text": map[string]any{"type": "object", "properties": map[string]any{
					"fields":    map[string]any{"type": "array", "maxItems": 30, "items": map[string]any{"type": "string"}},
					"query":     map[string]any{"type": "string"},
					"syntax":    map[string]any{"type": "string", "enum": []string{"simple", "advanced"}},
					"highlight": map[string]any{"type": "boolean"},
				}},
				"fusion": map[string]any{"type": "object", "properties": map[string]any{
					"kind": map[string]any{"type": "string", "enum": []string{"rrf"}},
					"k":    map[string]any{"type": "integer", "minimum": 1, "maximum": 1000},
				}, "required": []string{"kind", "k"}, "description": "required for mode=hybrid (reciprocal rank fusion over the vector and text legs)"},
				"field":             map[string]any{"type": "string", "description": "required for mode=fuzzy: the scanned field"},
				"algorithm":         map[string]any{"type": "string", "enum": []string{"levenshtein", "damerau", "osa", "hamming", "jaro_winkler", "soundex"}, "description": "required for mode=fuzzy"},
				"maximumDistance":   map[string]any{"type": "integer", "minimum": 0, "maximum": 1000, "description": "fuzzy threshold for distance algorithms"},
				"minimumSimilarity": map[string]any{"type": "number", "minimum": 0, "maximum": 1, "description": "fuzzy threshold for similarity algorithms"},
				"limit":             map[string]any{"type": "integer", "minimum": 1, "maximum": 100},
				"filter":            map[string]any{"type": "string"},
				"cursor":            map[string]any{"type": "string"},
			}, []string{"mode"})},
		{Method: "POST", Path: "/databases/{databaseId}/collections/{collection}/search-index", OperationID: "database.searchIndex", Tag: "search",
			Summary: "Create, verify or rebuild the native FTS index for the collection: an existing same-name index covering the requested fields keeps serving, anything else is rebuilt, and the response carries the verified state (ready, or unavailable with the error).", Security: auth, SuccessStatus: 200, SuccessDescription: "IndexState",
			RequestBody: body(map[string]any{
				"indexName": map[string]any{"type": "string"},
				"fields":    map[string]any{"type": "array", "items": map[string]any{"type": "string"}},
				"tokenizer": map[string]any{"type": "string", "enum": []string{"default", "en_stem", "raw", "simple", "whitespace", "ngram"}},
				"weights":   map[string]any{"type": "object", "additionalProperties": map[string]any{"type": "number"}},
			}, []string{"fields"})},
		{Method: "POST", Path: "/pocketbase-imports", OperationID: "pbimport.create", Tag: "imports",
			Summary: "Open a shared-layout PocketBase adoption session.", Security: superuser, SuccessStatus: 200, SuccessDescription: "ImportSession",
			RequestBody: body(map[string]any{
				"sourceKind":                map[string]any{"type": "string", "enum": []string{"pocketbase-backup", "pocketbase-directory"}},
				"sourceBytes":               map[string]any{"type": "integer", "minimum": 1},
				"sourceSha256":              map[string]any{"type": "string", "pattern": "^[a-f0-9]{64}$"},
				"targetMode":                map[string]any{"type": "string", "enum": []string{"new-application", "replace-application"}},
				"expectedPrimaryGeneration": map[string]any{"type": "string"},
				"sourceLabel":               map[string]any{"type": "string"},
			}, []string{"sourceKind", "sourceBytes", "sourceSha256", "targetMode", "expectedPrimaryGeneration"})},
		{Method: "PUT", Path: "/pocketbase-imports/{importId}/source", OperationID: "pbimport.source.upload", Tag: "imports",
			Summary: "Upload one resumable source chunk (≤8 MiB, Content-Range + X-Chunk-SHA256 verified).", Security: superuser, SuccessStatus: 200, SuccessDescription: "ImportUpload"},
		{Method: "GET", Path: "/pocketbase-imports/{importId}/source", OperationID: "pbimport.source.get", Tag: "imports",
			Summary: "Fetch the finalized source bytes.", Security: superuser, SuccessStatus: 200, SuccessDescription: "Binary source"},
		{Method: "POST", Path: "/pocketbase-imports/{importId}/analyze", OperationID: "pbimport.analyze", Tag: "imports",
			Summary: "Inspect the private source copy (integrity, schema, migrations, settings).", Security: superuser, SuccessStatus: 200, SuccessDescription: "Analysis"},
		{Method: "PUT", Path: "/pocketbase-imports/{importId}/plan", OperationID: "pbimport.plan.put", Tag: "imports",
			Summary: "Validate and hash the shared-layout plan (v0.1: all placements shared, preserve-global).", Security: superuser, SuccessStatus: 200, SuccessDescription: "ImportPlan",
			RequestBody: body(map[string]any{
				"expectedRevision": map[string]any{"type": "string"},
				"sourceHash":       map[string]any{"type": "string"},
				"snapshotHash":     map[string]any{"type": "string"},
				"layout":           map[string]any{"type": "string", "const": "shared"},
				"namespace":        map[string]any{"type": "string", "const": "shared"},
				"placements":       map[string]any{"type": "array", "items": map[string]any{"type": "object"}},
				"accountPolicy":    map[string]any{"type": "object"},
				"sideEffects":      map[string]any{"type": "string", "const": "suppressed-during-import"},
			}, []string{"expectedRevision", "sourceHash", "snapshotHash", "layout", "namespace", "placements", "accountPolicy", "selector", "profile", "sideEffects"})},
		{Method: "POST", Path: "/pocketbase-imports/{importId}/preview", OperationID: "pbimport.preview.compute", Tag: "imports",
			Summary: "Compute the actual per-collection totals and hashes from the source copy.", Security: superuser, SuccessStatus: 200, SuccessDescription: "ImportPreview"},
		{Method: "POST", Path: "/pocketbase-imports/{importId}/stage", OperationID: "pbimport.stage", Tag: "imports",
			Summary: "Copy rows/files verbatim into the unpublished staged target (bounded batches).", Security: superuser, SuccessStatus: 202, SuccessDescription: "StageResult"},
		{Method: "POST", Path: "/pocketbase-imports/{importId}/validate", OperationID: "pbimport.validate", Tag: "imports",
			Summary: "Run the count/hash/auth reconciliation cases over both copies.", Security: superuser, SuccessStatus: 200, SuccessDescription: "ImportValidation"},
		{Method: "GET", Path: "/pocketbase-imports/{importId}/report", OperationID: "pbimport.report", Tag: "imports",
			Summary: "The validation report (requires the exact report hash at cutover).", Security: superuser, SuccessStatus: 200, SuccessDescription: "ImportValidation"},
		{Method: "POST", Path: "/pocketbase-imports/{importId}/cutover", OperationID: "pbimport.cutover", Tag: "imports",
			Summary: "Publish the staged application through the durable guarded journal.", Security: superuser, SuccessStatus: 202, SuccessDescription: "ImportSession (active)",
			RequestBody: body(map[string]any{
				"planHash":    map[string]any{"type": "string"},
				"previewHash": map[string]any{"type": "string"},
				"reportHash":  map[string]any{"type": "string"},
			}, []string{"planHash", "previewHash", "reportHash"})},
		{Method: "POST", Path: "/pocketbase-imports/{importId}/cancel", OperationID: "pbimport.cancel", Tag: "imports",
			Summary: "Cancel before publication (terminal; never applies to active publication).", Security: superuser, SuccessStatus: 202, SuccessDescription: "ImportSession (cancelled)"},
		{Method: "GET", Path: "/pocketbase-imports/{importId}/report/cases", OperationID: "pbimport.report.cases", Tag: "imports",
			Summary: "Paginate the actual validation cases and artifact hashes without loading all results.", Security: superuser, SuccessStatus: 200, SuccessDescription: "ImportValidationCasePage"},
		{Method: "POST", Path: "/pocketbase-imports/{importId}/resume", OperationID: "pbimport.resume", Tag: "imports",
			Summary: "Reconcile the last durable phase and checkpoints of a failed session (journal-aware).", Security: superuser, SuccessStatus: 202, SuccessDescription: "Job",
			RequestBody: body(map[string]any{
				"expectedRevision": map[string]any{"type": "integer"},
				"planHash":         map[string]any{"type": "string", "nullable": true},
			}, []string{"expectedRevision", "planHash"})},
		{Method: "POST", Path: "/pocketbase-imports/{importId}/recovery", OperationID: "pbimport.recovery", Tag: "imports",
			Summary: "Verified prior-root rollback before new writes, or verified current-state export preserved as authoritative.", Security: superuser, SuccessStatus: 202, SuccessDescription: "Job",
			RequestBody: body(map[string]any{
				"expectedRevision":         map[string]any{"type": "integer"},
				"planHash":                 map[string]any{"type": "string"},
				"action":                   map[string]any{"type": "string", "enum": []string{"rollback-before-new-writes", "export-current-and-preserve"}},
				"expectedActiveGeneration": map[string]any{"type": "integer"},
			}, []string{"expectedRevision", "planHash", "action", "expectedActiveGeneration"})},
		{Method: "GET", Path: "/pocketbase-imports/{importId}/artifacts/{artifactId}", OperationID: "pbimport.artifact.get", Tag: "imports",
			Summary: "Stream a manifest-listed artifact after verifying the id is its sha256 (never an arbitrary path).", Security: superuser, SuccessStatus: 200, SuccessDescription: "Binary artifact"},
		{Method: "GET", Path: "/pocketbase-imports/{importId}", OperationID: "pbimport.get", Tag: "imports",
			Summary: "Read one import session.", Security: superuser, SuccessStatus: 200, SuccessDescription: "ImportSession"},

		// --- admin surface (databases lifecycle/operate, grants, agents,
		// --- per-target schema plans, jobs, receipts, backups, transfer) ---
		{Method: "GET", Path: "/databases", OperationID: "database.list", Tag: "databases",
			Summary: "Indexed paginated registry read (the primary and every provisioned target).", Security: superuser, SuccessStatus: 200, SuccessDescription: "DatabasePage"},
		{Method: "POST", Path: "/databases", OperationID: "database.create", Tag: "databases",
			Summary: "Persist the create intent and publish only after verified provisioning (manual/tenant/user/agent).", Security: superuser, SuccessStatus: 202, SuccessDescription: "Job",
			RequestBody: body(map[string]any{
				"displayName": map[string]any{"type": "string"},
				"kind":        map[string]any{"type": "string", "enum": []string{"manual", "tenant", "user", "agent"}},
				"owner":       map[string]any{"type": "object", "nullable": true, "properties": map[string]any{"collectionId": map[string]any{"type": "string"}, "recordId": map[string]any{"type": "string"}}},
				"profile":     map[string]any{"type": "string", "enum": []string{"wal", "mvcc"}},
				"templateId":  map[string]any{"type": "string", "nullable": true},
				"encryption":  map[string]any{"type": "object", "nullable": true},
				"codecId":     map[string]any{"type": "string", "nullable": true},
			}, []string{"displayName", "kind", "profile"})},
		{Method: "GET", Path: "/databases/{id}", OperationID: "database.get", Tag: "databases",
			Summary: "Inspect authorized metadata of one database (superuser or database.inspect).", Security: auth, SuccessStatus: 200, SuccessDescription: "Database"},
		{Method: "DELETE", Path: "/databases/{id}", OperationID: "database.delete", Tag: "databases",
			Summary: "Revoke grants, tombstone and enqueue the retention purge job (durable intent).", Security: superuser, SuccessStatus: 202, SuccessDescription: "Job",
			RequestBody: body(map[string]any{"expectedGeneration": map[string]any{"type": "integer"}}, []string{"expectedGeneration"})},
		{Method: "POST", Path: "/databases/{id}/maintenance", OperationID: "database.maintenance", Tag: "databases",
			Summary: "Enter or leave maintenance under the lifecycle lease (closes live handles).", Security: superuser, SuccessStatus: 200, SuccessDescription: "Database",
			RequestBody: body(map[string]any{
				"enabled":            map[string]any{"type": "boolean"},
				"expectedGeneration": map[string]any{"type": "integer"},
			}, []string{"enabled", "expectedGeneration"})},
		{Method: "GET", Path: "/databases/{id}/diagnostics", OperationID: "database.diagnostics", Tag: "databases",
			Summary: "Bounded nonsecret native/resource status (connections, leases, disk/wal/fts bytes).", Security: superuser, SuccessStatus: 200, SuccessDescription: "Diagnostic"},
		{Method: "POST", Path: "/databases/{id}/actions", OperationID: "database.maintain", Tag: "databases",
			Summary: "Capability-checked durable maintenance action (integrity_check/checkpoint/vacuum/gc).", Security: superuser, SuccessStatus: 202, SuccessDescription: "Job",
			RequestBody: body(map[string]any{
				"action":             map[string]any{"type": "string", "enum": []string{"integrity_check", "checkpoint", "vacuum", "fts_optimize", "fts_rebuild", "gc", "compact-sync"}},
				"expectedGeneration": map[string]any{"type": "integer"},
			}, []string{"action", "expectedGeneration"})},
		{Method: "POST", Path: "/databases/{id}/profile", OperationID: "database.profile", Tag: "databases",
			Summary: "Drain/validate/transition the durable profile with the recovery copy guard.", Security: superuser, SuccessStatus: 202, SuccessDescription: "Job",
			RequestBody: body(map[string]any{
				"expectedGeneration": map[string]any{"type": "integer"},
				"profile":            map[string]any{"type": "string", "enum": []string{"wal", "mvcc"}},
				"backupId":           map[string]any{"type": "string"},
			}, []string{"expectedGeneration", "profile", "backupId"})},
		{Method: "GET", Path: "/databases/{id}/grants", OperationID: "grant.list", Tag: "grants",
			Summary: "Read the live grant metadata without secrets.", Security: superuser, SuccessStatus: 200, SuccessDescription: "GrantList"},
		{Method: "PUT", Path: "/databases/{id}/grants", OperationID: "grant.put", Tag: "grants",
			Summary: "Upsert one subject grant (released scopes only; epoch guard).", Security: superuser, SuccessStatus: 200, SuccessDescription: "Grant",
			RequestBody: body(map[string]any{
				"subject":       map[string]any{"type": "object", "properties": map[string]any{"collectionId": map[string]any{"type": "string"}, "recordId": map[string]any{"type": "string"}}},
				"scopes":        map[string]any{"type": "array", "items": map[string]any{"type": "string", "enum": []string{"record.read", "record.write", "files.read", "files.write", "database.inspect"}}},
				"expiresAt":     map[string]any{"type": "string", "nullable": true},
				"expectedEpoch": map[string]any{"type": "integer"},
			}, []string{"subject", "scopes", "expiresAt", "expectedEpoch"})},
		{Method: "DELETE", Path: "/databases/{id}/grants/{grantId}", OperationID: "grant.revoke", Tag: "grants",
			Summary: "Revoke one grant and advance the policy epoch.", Security: superuser, SuccessStatus: 204, SuccessDescription: "Revoked"},
		{Method: "POST", Path: "/agents", OperationID: "agent.bind", Tag: "agents",
			Summary: "Bind an existing auth identity to its owner (never mints superuser authority).", Security: superuser, SuccessStatus: 200, SuccessDescription: "AgentBinding",
			RequestBody: body(map[string]any{
				"agent":      map[string]any{"type": "object", "properties": map[string]any{"collectionId": map[string]any{"type": "string"}, "recordId": map[string]any{"type": "string"}}},
				"owner":      map[string]any{"type": "object", "properties": map[string]any{"collectionId": map[string]any{"type": "string"}, "recordId": map[string]any{"type": "string"}}},
				"databaseId": map[string]any{"type": "string", "nullable": true},
			}, []string{"agent", "owner", "databaseId"})},
		{Method: "GET", Path: "/agents", OperationID: "agent.list", Tag: "agents",
			Summary: "List the active agent bindings.", Security: superuser, SuccessStatus: 200, SuccessDescription: "AgentList"},
		{Method: "DELETE", Path: "/agents/{agentId}", OperationID: "agent.revoke", Tag: "agents",
			Summary: "Revoke one agent binding.", Security: superuser, SuccessStatus: 204, SuccessDescription: "Revoked"},
		{Method: "POST", Path: "/databases/{id}/schema/plans", OperationID: "schema.plan", Tag: "schema",
			Summary: "Validate/prepare a per-target schema plan without modifying data (released slice: base collections plus btree/fts).", Security: superuser, SuccessStatus: 200, SuccessDescription: "SchemaPlan",
			RequestBody: body(map[string]any{
				"expectedSchemaRevision": map[string]any{"type": "integer"},
				"operations":             map[string]any{"type": "array", "maxItems": 1000, "items": map[string]any{"type": "object"}},
			}, []string{"expectedSchemaRevision", "operations"})},
		{Method: "POST", Path: "/databases/{id}/schema/plans/{planId}/apply", OperationID: "schema.apply", Tag: "schema",
			Summary: "Apply the verified same plan under the schema lease and record the new revision.", Security: superuser, SuccessStatus: 202, SuccessDescription: "Job",
			RequestBody: body(map[string]any{
				"planHash":               map[string]any{"type": "string"},
				"expectedSchemaRevision": map[string]any{"type": "integer"},
			}, []string{"planHash", "expectedSchemaRevision"})},
		{Method: "GET", Path: "/templates/{templateId}", OperationID: "template.get", Tag: "templates",
			Summary: "Read the latest published template revision.", Security: superuser, SuccessStatus: 200, SuccessDescription: "Template"},
		{Method: "GET", Path: "/jobs", OperationID: "job.list", Tag: "jobs",
			Summary: "Paginated read of the durable job store.", Security: superuser, SuccessStatus: 200, SuccessDescription: "JobPage"},
		{Method: "GET", Path: "/jobs/{jobId}", OperationID: "job.get", Tag: "jobs",
			Summary: "Inspect the actual job phase and effects.", Security: superuser, SuccessStatus: 202, SuccessDescription: "Job"},
		{Method: "POST", Path: "/jobs/{jobId}/cancel", OperationID: "job.cancel", Tag: "jobs",
			Summary: "Request safe cancellation; no pretend rollback of committed steps.", Security: superuser, SuccessStatus: 202, SuccessDescription: "Job"},
		{Method: "GET", Path: "/databases/{id}/receipts/{receiptId}", OperationID: "receipt.get", Tag: "receipts",
			Summary: "Resolve an uncertain mutation outcome without re-executing (same actor or superuser).", Security: auth, SuccessStatus: 200, SuccessDescription: "Receipt"},
		{Method: "POST", Path: "/databases/{id}/backups", OperationID: "backup.create", Tag: "backups",
			Summary: "Create a verified context archive with the file-coverage manifest.", Security: superuser, SuccessStatus: 202, SuccessDescription: "Job",
			RequestBody: body(map[string]any{
				"expectedGeneration": map[string]any{"type": "integer"},
				"includeFiles":       map[string]any{"type": "boolean"},
				"destination":        map[string]any{"type": "string", "enum": []string{"local", "configured-remote"}},
			}, []string{"expectedGeneration", "includeFiles", "destination"})},
		{Method: "POST", Path: "/databases/{id}/restore", OperationID: "backup.restore", Tag: "backups",
			Summary: "Validate/stage then publish with a new generation if not dry-run.", Security: superuser, SuccessStatus: 202, SuccessDescription: "Job",
			RequestBody: body(map[string]any{
				"archiveId":          map[string]any{"type": "string"},
				"expectedGeneration": map[string]any{"type": "integer"},
				"dryRun":             map[string]any{"type": "boolean"},
				"restoreToNew":       map[string]any{"type": "boolean"},
				"authorityPolicy":    map[string]any{"type": "string", "enum": []string{"remove", "preserve-authorized"}},
				"subjects":           map[string]any{"type": "array", "items": map[string]any{"type": "object"}},
			}, []string{"archiveId", "expectedGeneration", "dryRun", "restoreToNew", "authorityPolicy", "subjects"})},
		{Method: "POST", Path: "/databases/{id}/owner-transfer", OperationID: "owner.transfer", Tag: "owner",
			Summary: "Run the durable owner transfer phases over the guard journal.", Security: superuser, SuccessStatus: 202, SuccessDescription: "Job",
			RequestBody: body(map[string]any{
				"expectedGeneration":  map[string]any{"type": "integer"},
				"expectedPolicyEpoch": map[string]any{"type": "integer"},
				"previousOwner":       map[string]any{"type": "object", "nullable": true},
				"newOwner":            map[string]any{"type": "object", "properties": map[string]any{"collectionId": map[string]any{"type": "string"}, "recordId": map[string]any{"type": "string"}}},
				"agentPolicy":         map[string]any{"type": "string", "enum": []string{"revoke", "rebind-reviewed"}},
			}, []string{"expectedGeneration", "expectedPolicyEpoch", "previousOwner", "newOwner", "agentPolicy"})},
		{Method: "GET", Path: "/pocketbase-imports/{importId}/inventory", OperationID: "pbimport.inventory", Tag: "imports",
			Summary: "Read the recorded analysis inventory artifact.", Security: superuser, SuccessStatus: 200, SuccessDescription: "Analysis"},
		{Method: "GET", Path: "/pocketbase-imports/{importId}/plan", OperationID: "pbimport.plan.get", Tag: "imports",
			Summary: "Read the recorded plan artifact.", Security: superuser, SuccessStatus: 200, SuccessDescription: "ImportPlan"},
		{Method: "GET", Path: "/pocketbase-imports/{importId}/preview", OperationID: "pbimport.preview.get", Tag: "imports",
			Summary: "Read the recorded preview artifact.", Security: superuser, SuccessStatus: 200, SuccessDescription: "ImportPreview"},
		{Method: "GET", Path: "/pocketbase-imports/{importId}/issues", OperationID: "pbimport.issues", Tag: "imports",
			Summary: "Read the recorded validation artifact (unresolved cases).", Security: superuser, SuccessStatus: 200, SuccessDescription: "ImportValidation"},
		{Method: "GET", Path: "/pocketbase-imports/{importId}/tenants", OperationID: "pbimport.tenants", Tag: "imports",
			Summary: "Read the analyzed source's tenant inventory (the ownership split is the R2 remainder).", Security: superuser, SuccessStatus: 200, SuccessDescription: "TenantInventory"},
		{Method: "POST", Path: "/import-batches", OperationID: "importbatch.create", Tag: "imports",
			Summary: "Open a batch orchestrating verified source sessions with per-source provenance (append/repeat/replacement).", Security: superuser, SuccessStatus: 200, SuccessDescription: "ImportBatch",
			RequestBody: body(map[string]any{
				"mode":    map[string]any{"type": "string", "enum": []string{"append", "repeat", "replacement"}},
				"sources": map[string]any{"type": "array", "items": map[string]any{"type": "object"}},
			}, []string{"mode", "sources"})},
		{Method: "GET", Path: "/import-batches/{batchId}", OperationID: "importbatch.get", Tag: "imports",
			Summary: "Read one batch with its per-source rows.", Security: superuser, SuccessStatus: 200, SuccessDescription: "ImportBatch"},
		{Method: "GET", Path: "/import-batches/{batchId}/sources", OperationID: "importbatch.sources", Tag: "imports",
			Summary: "Paginate the per-source provenance journal.", Security: superuser, SuccessStatus: 200, SuccessDescription: "BatchSourcePage"},
		{Method: "POST", Path: "/import-batches/{batchId}/run", OperationID: "importbatch.run", Tag: "imports",
			Summary: "Drive the batch (per-source outcomes stay exact; a failed source is never marked applied).", Security: superuser, SuccessStatus: 202, SuccessDescription: "ImportBatch"},
		{Method: "POST", Path: "/import-batches/{batchId}/cancel", OperationID: "importbatch.cancel", Tag: "imports",
			Summary: "Cancel a not-yet-completed batch (per-source evidence retained).", Security: superuser, SuccessStatus: 202, SuccessDescription: "ImportBatch"},
	}
	sort.Slice(ops, func(i, j int) bool {
		if ops[i].Path != ops[j].Path {
			return ops[i].Path < ops[j].Path
		}
		return ops[i].Method < ops[j].Method
	})
	return ops
}
