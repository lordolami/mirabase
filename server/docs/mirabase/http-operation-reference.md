# HTTP operation reference

**Current release authority:** [current implementation inventory](../current-product.md), [user documentation](../user/index.md) and [launch status](../launch/status.md). Initial v0.1/roadmap delivery labels below preserve planning contracts; they do not establish today's implementation or permit deleting subsequently integrated features. [Open-source platform roadmap](../roadmap/platform/README.md) has separate design and gates.

**Delivery: v0.1 and roadmap.** [v0.1-build.md](v0.1-build.md) selects the current portion; [roadsmap.md](roadsmap.md) retains the remainder. Detailed schemas/configuration/ABI and full-family acceptance below describe the target superset unless narrowed by that allocation.

All additive operations below are required for the complete product; their Delivery labels select the active v0.1 subset. Entirely roadmap-only routes are unregistered, not implemented as unsupported stubs. The default application uses these paths directly. Additional applications use the application prefix described in [application isolation](application-isolation.md); instance-scoped operations require the instance operator. The [wire schema reference](wire-schema-reference.md) defines all named types using `urn:mirabase:wire`. Compile the final OpenAPI 3.1 document from these operation definitions and the pinned inherited handlers when implementing API tooling.

## Operation capabilities

**Delivery:** v0.1. Applicable exact contract within one default application and released features.

`GET /api/mirabase/v1/capabilities`

**Authority:** authenticated actor; privileged build details redacted for ordinary clients. **Scope:** resolved-application.

**Preconditions:** authorized immutable DatabaseContext; ready target unless lifecycle operation; current generation/policy/schema.

**Effects:** No mutation; report actual loaded build/mode availability.

**Idempotency:** read-only.

**Request schema:**


```json
{
  "type": "null"
}
```


**Response:** 200; application/json.


```json
{
  "$ref": "urn:mirabase:wire#/$defs/Capabilities"
}
```


Referenced types: [Capabilities](wire-schema-reference.md#schema-capabilities).

**Errors:** `invalid_request`, `forbidden`, `not_found`, `generation_mismatch`, `schema_mismatch`, `policy_changed`, `resource_exhausted`.

**Requirement:** MB-PROVIDER. **Acceptance:** AT-PROVIDER.

**Additional parameter/phase constraints:**


```json
{
  "pathParameters": []
}
```


## Operation database.list

**Delivery:** v0.1. Applicable exact contract within one default application and released features.

`GET /api/mirabase/v1/databases`

**Authority:** superuser. **Scope:** resolved-application.

**Preconditions:** authorized immutable DatabaseContext; ready target unless lifecycle operation; current generation/policy/schema.

**Effects:** Indexed paginated registry read.

**Idempotency:** read-only.

**Request schema:**


```json
{
  "type": "object",
  "properties": {
    "page": {
      "type": "integer",
      "minimum": 1,
      "maximum": 9007199254740991
    },
    "perPage": {
      "type": "integer",
      "minimum": 1,
      "maximum": 1000
    },
    "filter": {
      "type": "string"
    },
    "sort": {
      "type": "string"
    }
  },
  "required": [],
  "additionalProperties": false
}
```


**Response:** 200; application/json.


```json
{
  "type": "object",
  "properties": {
    "items": {
      "type": "array",
      "items": {
        "$ref": "urn:mirabase:wire#/$defs/Database"
      },
      "maxItems": 1000
    },
    "page": {
      "type": "integer",
      "minimum": 1,
      "maximum": 9007199254740991
    },
    "perPage": {
      "type": "integer",
      "minimum": 1,
      "maximum": 1000
    },
    "totalItems": {
      "type": "integer",
      "minimum": 0,
      "maximum": 9007199254740991
    },
    "totalPages": {
      "type": "integer",
      "minimum": 0,
      "maximum": 9007199254740991
    }
  },
  "required": [
    "items",
    "page",
    "perPage",
    "totalItems",
    "totalPages"
  ],
  "additionalProperties": false
}
```


Referenced types: [Database](wire-schema-reference.md#schema-database).

**Errors:** `invalid_request`, `forbidden`, `not_found`, `generation_mismatch`, `schema_mismatch`, `policy_changed`, `resource_exhausted`.

**Requirement:** MB-LIFECYCLE. **Acceptance:** AT-LIFECYCLE.

**Additional parameter/phase constraints:**


```json
{
  "pathParameters": []
}
```


## Operation database.create

**Delivery:** v0.1. Applicable exact contract within one default application and released features.

`POST /api/mirabase/v1/databases`

**Authority:** superuser. **Scope:** resolved-application.

**Preconditions:** authorized immutable DatabaseContext; ready target unless lifecycle operation; current generation/policy/schema.

**Effects:** Persist create intent; publish only after verified provisioning.

**Idempotency:** Idempotency-Key; canonical hash and target-local receipt for mutations, durable intent for lifecycle jobs.

**Request schema:**


```json
{
  "type": "object",
  "properties": {
    "displayName": {
      "type": "string"
    },
    "kind": {
      "type": "string",
      "enum": [
        "manual",
        "tenant",
        "user",
        "agent"
      ]
    },
    "owner": {
      "anyOf": [
        {
          "$ref": "urn:mirabase:wire#/$defs/Subject"
        },
        {
          "type": "null"
        }
      ]
    },
    "profile": {
      "$ref": "urn:mirabase:wire#/$defs/Profile"
    },
    "templateId": {
      "anyOf": [
        {
          "type": "string"
        },
        {
          "type": "null"
        }
      ]
    },
    "encryption": {
      "anyOf": [
        {
          "type": "object",
          "properties": {
            "algorithm": {
              "type": "string"
            },
            "keyRef": {
              "type": "string"
            }
          },
          "required": [
            "algorithm",
            "keyRef"
          ],
          "additionalProperties": false
        },
        {
          "type": "null"
        }
      ]
    },
    "codecId": {
      "anyOf": [
        {
          "type": "string"
        },
        {
          "type": "null"
        }
      ]
    }
  },
  "required": [
    "displayName",
    "kind",
    "profile"
  ],
  "additionalProperties": false
}
```


**Response:** 202; application/json.


```json
{
  "$ref": "urn:mirabase:wire#/$defs/Job"
}
```


Referenced types: [Job](wire-schema-reference.md#schema-job), [Profile](wire-schema-reference.md#schema-profile), [Subject](wire-schema-reference.md#schema-subject).

**Errors:** `invalid_request`, `forbidden`, `not_found`, `generation_mismatch`, `schema_mismatch`, `policy_changed`, `resource_exhausted`.

**Requirement:** MB-LIFECYCLE. **Acceptance:** AT-LIFECYCLE.

**Additional parameter/phase constraints:**


```json
{
  "pathParameters": []
}
```


## Operation database.get

**Delivery:** v0.1. Applicable exact contract within one default application and released features.

`GET /api/mirabase/v1/databases/{id}`

**Authority:** superuser or database.inspect. **Scope:** resolved-application.

**Preconditions:** authorized immutable DatabaseContext; ready target unless lifecycle operation; current generation/policy/schema.

**Effects:** Inspect authorized metadata only.

**Idempotency:** read-only.

**Request schema:**


```json
{
  "type": "null"
}
```


**Response:** 200; application/json.


```json
{
  "$ref": "urn:mirabase:wire#/$defs/Database"
}
```


Referenced types: [Database](wire-schema-reference.md#schema-database), [DatabaseId](wire-schema-reference.md#schema-databaseid).

**Errors:** `invalid_request`, `forbidden`, `not_found`, `generation_mismatch`, `schema_mismatch`, `policy_changed`, `resource_exhausted`.

**Requirement:** MB-LIFECYCLE. **Acceptance:** AT-LIFECYCLE.

**Additional parameter/phase constraints:**


```json
{
  "pathParameters": [
    {
      "name": "id",
      "schema": {
        "$ref": "urn:mirabase:wire#/$defs/DatabaseId"
      }
    }
  ]
}
```


## Operation database.delete

**Delivery:** v0.1. Applicable exact contract within one default application and released features.

`DELETE /api/mirabase/v1/databases/{id}`

**Authority:** superuser. **Scope:** resolved-application.

**Preconditions:** authorized immutable DatabaseContext; ready target unless lifecycle operation; current generation/policy/schema.

**Effects:** Revoke/drain/trash/tombstone then retention purge.

**Idempotency:** Idempotency-Key; canonical hash and target-local receipt for mutations, durable intent for lifecycle jobs.

**Request schema:**


```json
{
  "type": "object",
  "properties": {
    "expectedGeneration": {
      "$ref": "urn:mirabase:wire#/$defs/Revision"
    }
  },
  "required": [
    "expectedGeneration"
  ],
  "additionalProperties": false
}
```


**Response:** 202; application/json.


```json
{
  "$ref": "urn:mirabase:wire#/$defs/Job"
}
```


Referenced types: [DatabaseId](wire-schema-reference.md#schema-databaseid), [Job](wire-schema-reference.md#schema-job), [Revision](wire-schema-reference.md#schema-revision).

**Errors:** `invalid_request`, `forbidden`, `not_found`, `generation_mismatch`, `schema_mismatch`, `policy_changed`, `resource_exhausted`.

**Requirement:** MB-LIFECYCLE. **Acceptance:** AT-LIFECYCLE.

**Additional parameter/phase constraints:**


```json
{
  "pathParameters": [
    {
      "name": "id",
      "schema": {
        "$ref": "urn:mirabase:wire#/$defs/DatabaseId"
      }
    }
  ]
}
```


## Operation database.maintenance

**Delivery:** v0.1. Applicable exact contract within one default application and released features.

`POST /api/mirabase/v1/databases/{id}/maintenance`

**Authority:** superuser. **Scope:** resolved-application.

**Preconditions:** authorized immutable DatabaseContext; ready target unless lifecycle operation; current generation/policy/schema.

**Effects:** Enter/drain or leave maintenance under lifecycle lease.

**Idempotency:** Idempotency-Key; canonical hash and target-local receipt for mutations, durable intent for lifecycle jobs.

**Request schema:**


```json
{
  "type": "object",
  "properties": {
    "enabled": {
      "type": "boolean"
    },
    "expectedGeneration": {
      "$ref": "urn:mirabase:wire#/$defs/Revision"
    }
  },
  "required": [
    "enabled",
    "expectedGeneration"
  ],
  "additionalProperties": false
}
```


**Response:** 200; application/json.


```json
{
  "$ref": "urn:mirabase:wire#/$defs/Database"
}
```


Referenced types: [Database](wire-schema-reference.md#schema-database), [DatabaseId](wire-schema-reference.md#schema-databaseid), [Revision](wire-schema-reference.md#schema-revision).

**Errors:** `invalid_request`, `forbidden`, `not_found`, `generation_mismatch`, `schema_mismatch`, `policy_changed`, `resource_exhausted`.

**Requirement:** MB-LIFECYCLE. **Acceptance:** AT-LIFECYCLE.

**Additional parameter/phase constraints:**


```json
{
  "pathParameters": [
    {
      "name": "id",
      "schema": {
        "$ref": "urn:mirabase:wire#/$defs/DatabaseId"
      }
    }
  ]
}
```


## Operation database.diagnostics

**Delivery:** v0.1. Applicable exact contract within one default application and released features.

`GET /api/mirabase/v1/databases/{id}/diagnostics`

**Authority:** superuser. **Scope:** resolved-application.

**Preconditions:** authorized immutable DatabaseContext; ready target unless lifecycle operation; current generation/policy/schema.

**Effects:** Read bounded nonsecret native/resource status.

**Idempotency:** read-only.

**Request schema:**


```json
{
  "type": "null"
}
```


**Response:** 200; application/json.


```json
{
  "$ref": "urn:mirabase:wire#/$defs/Diagnostic"
}
```


Referenced types: [DatabaseId](wire-schema-reference.md#schema-databaseid), [Diagnostic](wire-schema-reference.md#schema-diagnostic).

**Errors:** `invalid_request`, `forbidden`, `not_found`, `generation_mismatch`, `schema_mismatch`, `policy_changed`, `resource_exhausted`.

**Requirement:** MB-OPERATE. **Acceptance:** AT-OPERATE.

**Additional parameter/phase constraints:**


```json
{
  "pathParameters": [
    {
      "name": "id",
      "schema": {
        "$ref": "urn:mirabase:wire#/$defs/DatabaseId"
      }
    }
  ]
}
```


## Operation database.maintain

**Delivery:** slice. Only released integrity/vacuum/checkpoint operations under qualified profile and maintenance guards; no sync compaction/key/codec actions.

`POST /api/mirabase/v1/databases/{id}/actions`

**Authority:** superuser. **Scope:** resolved-application.

**Preconditions:** authorized immutable DatabaseContext; ready target unless lifecycle operation; current generation/policy/schema.

**Effects:** Capability-checked durable maintenance action.

**Idempotency:** Idempotency-Key; canonical hash and target-local receipt for mutations, durable intent for lifecycle jobs.

**Request schema:**


```json
{
  "type": "object",
  "properties": {
    "action": {
      "type": "string",
      "enum": [
        "integrity_check",
        "checkpoint",
        "vacuum",
        "fts_optimize",
        "fts_rebuild",
        "gc",
        "compact-sync"
      ]
    },
    "expectedGeneration": {
      "$ref": "urn:mirabase:wire#/$defs/Revision"
    }
  },
  "required": [
    "action",
    "expectedGeneration"
  ],
  "additionalProperties": false
}
```


**Response:** 202; application/json.


```json
{
  "$ref": "urn:mirabase:wire#/$defs/Job"
}
```


Referenced types: [DatabaseId](wire-schema-reference.md#schema-databaseid), [Job](wire-schema-reference.md#schema-job), [Revision](wire-schema-reference.md#schema-revision).

**Errors:** `invalid_request`, `forbidden`, `not_found`, `generation_mismatch`, `schema_mismatch`, `policy_changed`, `resource_exhausted`.

**Requirement:** MB-OPERATE. **Acceptance:** AT-OPERATE.

**Additional parameter/phase constraints:**


```json
{
  "pathParameters": [
    {
      "name": "id",
      "schema": {
        "$ref": "urn:mirabase:wire#/$defs/DatabaseId"
      }
    }
  ]
}
```


## Operation grant.list

**Delivery:** v0.1. Applicable exact contract within one default application and released features.

`GET /api/mirabase/v1/databases/{id}/grants`

**Authority:** superuser. **Scope:** resolved-application.

**Preconditions:** authorized immutable DatabaseContext; ready target unless lifecycle operation; current generation/policy/schema.

**Effects:** Read grant metadata without secrets.

**Idempotency:** read-only.

**Request schema:**


```json
{
  "type": "null"
}
```


**Response:** 200; application/json.


```json
{
  "type": "object",
  "properties": {
    "items": {
      "type": "array",
      "items": {
        "$ref": "urn:mirabase:wire#/$defs/Grant"
      },
      "maxItems": 1000
    },
    "page": {
      "type": "integer",
      "minimum": 1,
      "maximum": 9007199254740991
    },
    "perPage": {
      "type": "integer",
      "minimum": 1,
      "maximum": 1000
    },
    "totalItems": {
      "type": "integer",
      "minimum": 0,
      "maximum": 9007199254740991
    },
    "totalPages": {
      "type": "integer",
      "minimum": 0,
      "maximum": 9007199254740991
    }
  },
  "required": [
    "items",
    "page",
    "perPage",
    "totalItems",
    "totalPages"
  ],
  "additionalProperties": false
}
```


Referenced types: [DatabaseId](wire-schema-reference.md#schema-databaseid), [Grant](wire-schema-reference.md#schema-grant).

**Errors:** `invalid_request`, `forbidden`, `not_found`, `generation_mismatch`, `schema_mismatch`, `policy_changed`, `resource_exhausted`.

**Requirement:** MB-AGENT. **Acceptance:** AT-AGENT.

**Additional parameter/phase constraints:**


```json
{
  "pathParameters": [
    {
      "name": "id",
      "schema": {
        "$ref": "urn:mirabase:wire#/$defs/DatabaseId"
      }
    }
  ]
}
```


## Operation grant.put

**Delivery:** slice. Only record.read/write, files.read/write and database.inspect; current record rules still apply.

`PUT /api/mirabase/v1/databases/{id}/grants/{grantId}`

**Authority:** superuser. **Scope:** resolved-application.

**Preconditions:** authorized immutable DatabaseContext; ready target unless lifecycle operation; current generation/policy/schema.

**Effects:** Validate subject/owner; persist grant and advance policy epoch.

**Idempotency:** Idempotency-Key; canonical hash and target-local receipt for mutations, durable intent for lifecycle jobs.

**Request schema:**


```json
{
  "type": "object",
  "properties": {
    "subject": {
      "$ref": "urn:mirabase:wire#/$defs/Subject"
    },
    "scopes": {
      "type": "array",
      "items": {
        "$ref": "urn:mirabase:wire#/$defs/Scope"
      },
      "maxItems": 10
    },
    "expiresAt": {
      "anyOf": [
        {
          "$ref": "urn:mirabase:wire#/$defs/Timestamp"
        },
        {
          "type": "null"
        }
      ]
    },
    "expectedEpoch": {
      "$ref": "urn:mirabase:wire#/$defs/Revision"
    }
  },
  "required": [
    "subject",
    "scopes",
    "expiresAt",
    "expectedEpoch"
  ],
  "additionalProperties": false
}
```


**Response:** 200; application/json.


```json
{
  "$ref": "urn:mirabase:wire#/$defs/Grant"
}
```


Referenced types: [DatabaseId](wire-schema-reference.md#schema-databaseid), [Grant](wire-schema-reference.md#schema-grant), [OpaqueId](wire-schema-reference.md#schema-opaqueid), [Revision](wire-schema-reference.md#schema-revision), [Scope](wire-schema-reference.md#schema-scope), [Subject](wire-schema-reference.md#schema-subject), [Timestamp](wire-schema-reference.md#schema-timestamp).

**Errors:** `invalid_request`, `forbidden`, `not_found`, `generation_mismatch`, `schema_mismatch`, `policy_changed`, `resource_exhausted`.

**Requirement:** MB-AGENT. **Acceptance:** AT-AGENT.

**Additional parameter/phase constraints:**


```json
{
  "pathParameters": [
    {
      "name": "id",
      "schema": {
        "$ref": "urn:mirabase:wire#/$defs/DatabaseId"
      }
    },
    {
      "name": "grantId",
      "schema": {
        "$ref": "urn:mirabase:wire#/$defs/OpaqueId"
      }
    }
  ]
}
```


## Operation grant.revoke

**Delivery:** v0.1. Applicable exact contract within one default application and released features.

`DELETE /api/mirabase/v1/databases/{id}/grants/{grantId}`

**Authority:** superuser. **Scope:** resolved-application.

**Preconditions:** authorized immutable DatabaseContext; ready target unless lifecycle operation; current generation/policy/schema.

**Effects:** Revoke grant and invalidate streams.

**Idempotency:** Idempotency-Key; canonical hash and target-local receipt for mutations, durable intent for lifecycle jobs.

**Request schema:**


```json
{
  "type": "object",
  "properties": {
    "expectedEpoch": {
      "$ref": "urn:mirabase:wire#/$defs/Revision"
    }
  },
  "required": [
    "expectedEpoch"
  ],
  "additionalProperties": false
}
```


**Response:** 200; application/json.


```json
{
  "$ref": "urn:mirabase:wire#/$defs/Grant"
}
```


Referenced types: [DatabaseId](wire-schema-reference.md#schema-databaseid), [Grant](wire-schema-reference.md#schema-grant), [OpaqueId](wire-schema-reference.md#schema-opaqueid), [Revision](wire-schema-reference.md#schema-revision).

**Errors:** `invalid_request`, `forbidden`, `not_found`, `generation_mismatch`, `schema_mismatch`, `policy_changed`, `resource_exhausted`.

**Requirement:** MB-AGENT. **Acceptance:** AT-AGENT.

**Additional parameter/phase constraints:**


```json
{
  "pathParameters": [
    {
      "name": "id",
      "schema": {
        "$ref": "urn:mirabase:wire#/$defs/DatabaseId"
      }
    },
    {
      "name": "grantId",
      "schema": {
        "$ref": "urn:mirabase:wire#/$defs/OpaqueId"
      }
    }
  ]
}
```


## Operation agent.bind

**Delivery:** v0.1. Applicable exact contract within one default application and released features.

`POST /api/mirabase/v1/agents`

**Authority:** superuser or configured owner registration policy. **Scope:** resolved-application.

**Preconditions:** authorized immutable DatabaseContext; ready target unless lifecycle operation; current generation/policy/schema.

**Effects:** Bind existing auth identity; never mint superuser authority.

**Idempotency:** Idempotency-Key; canonical hash and target-local receipt for mutations, durable intent for lifecycle jobs.

**Request schema:**


```json
{
  "type": "object",
  "properties": {
    "agent": {
      "$ref": "urn:mirabase:wire#/$defs/Subject"
    },
    "owner": {
      "$ref": "urn:mirabase:wire#/$defs/Subject"
    },
    "databaseId": {
      "anyOf": [
        {
          "$ref": "urn:mirabase:wire#/$defs/DatabaseId"
        },
        {
          "type": "null"
        }
      ]
    }
  },
  "required": [
    "agent",
    "owner",
    "databaseId"
  ],
  "additionalProperties": false
}
```


**Response:** 200; application/json.


```json
{
  "type": "object",
  "properties": {
    "id": {
      "type": "string"
    },
    "agent": {
      "$ref": "urn:mirabase:wire#/$defs/Subject"
    },
    "owner": {
      "$ref": "urn:mirabase:wire#/$defs/Subject"
    },
    "state": {
      "type": "string",
      "enum": [
        "active"
      ]
    }
  },
  "required": [
    "id",
    "agent",
    "owner",
    "state"
  ],
  "additionalProperties": false
}
```


Referenced types: [DatabaseId](wire-schema-reference.md#schema-databaseid), [Subject](wire-schema-reference.md#schema-subject).

**Errors:** `invalid_request`, `forbidden`, `not_found`, `generation_mismatch`, `schema_mismatch`, `policy_changed`, `resource_exhausted`.

**Requirement:** MB-AGENT. **Acceptance:** AT-AGENT.

**Additional parameter/phase constraints:**


```json
{
  "pathParameters": []
}
```


## Operation agent.revoke

**Delivery:** v0.1. Applicable exact contract within one default application and released features.

`DELETE /api/mirabase/v1/agents/{agentId}`

**Authority:** superuser or authorized bound owner. **Scope:** resolved-application.

**Preconditions:** authorized immutable DatabaseContext; ready target unless lifecycle operation; current generation/policy/schema.

**Effects:** Revoke agent grants and follow configured lifecycle policy.

**Idempotency:** Idempotency-Key; canonical hash and target-local receipt for mutations, durable intent for lifecycle jobs.

**Request schema:**


```json
{
  "type": "null"
}
```


**Response:** 200; application/json.


```json
{
  "type": "object",
  "properties": {
    "id": {
      "type": "string"
    },
    "state": {
      "type": "string",
      "enum": [
        "revoked"
      ]
    }
  },
  "required": [
    "id",
    "state"
  ],
  "additionalProperties": false
}
```


Referenced types: [OpaqueId](wire-schema-reference.md#schema-opaqueid).

**Errors:** `invalid_request`, `forbidden`, `not_found`, `generation_mismatch`, `schema_mismatch`, `policy_changed`, `resource_exhausted`.

**Requirement:** MB-AGENT. **Acceptance:** AT-AGENT.

**Additional parameter/phase constraints:**


```json
{
  "pathParameters": [
    {
      "name": "agentId",
      "schema": {
        "$ref": "urn:mirabase:wire#/$defs/OpaqueId"
      }
    }
  ]
}
```


## Operation schema.plan

**Delivery:** slice. Inherited schema plus vector/FTS; no roadmap types/views/sequences.

`POST /api/mirabase/v1/databases/{id}/schema/plans`

**Authority:** superuser. **Scope:** resolved-application.

**Preconditions:** authorized immutable DatabaseContext; ready target unless lifecycle operation; current generation/policy/schema.

**Effects:** Validate/prepare plan without modifying application data.

**Idempotency:** read-only.

**Request schema:**


```json
{
  "type": "object",
  "properties": {
    "expectedSchemaRevision": {
      "$ref": "urn:mirabase:wire#/$defs/Revision"
    },
    "operations": {
      "type": "array",
      "items": {
        "$ref": "urn:mirabase:wire#/$defs/SchemaOperation"
      },
      "maxItems": 1000
    }
  },
  "required": [
    "expectedSchemaRevision",
    "operations"
  ],
  "additionalProperties": false
}
```


**Response:** 200; application/json.


```json
{
  "$ref": "urn:mirabase:wire#/$defs/SchemaPlan"
}
```


Referenced types: [DatabaseId](wire-schema-reference.md#schema-databaseid), [Revision](wire-schema-reference.md#schema-revision), [SchemaOperation](wire-schema-reference.md#schema-schemaoperation), [SchemaPlan](wire-schema-reference.md#schema-schemaplan).

**Errors:** `invalid_request`, `forbidden`, `not_found`, `generation_mismatch`, `schema_mismatch`, `policy_changed`, `resource_exhausted`.

**Requirement:** MB-MIGRATIONS. **Acceptance:** AT-MIGRATIONS.

**Additional parameter/phase constraints:**


```json
{
  "pathParameters": [
    {
      "name": "id",
      "schema": {
        "$ref": "urn:mirabase:wire#/$defs/DatabaseId"
      }
    }
  ]
}
```


## Operation schema.apply

**Delivery:** slice. Same released schema subset; actual guarded journal and per-target effects.

`POST /api/mirabase/v1/databases/{id}/schema/plans/{planId}/apply`

**Authority:** superuser. **Scope:** resolved-application.

**Preconditions:** authorized immutable DatabaseContext; ready target unless lifecycle operation; current generation/policy/schema.

**Effects:** Apply verified same plan under schema lease.

**Idempotency:** Idempotency-Key; canonical hash and target-local receipt for mutations, durable intent for lifecycle jobs.

**Request schema:**


```json
{
  "type": "object",
  "properties": {
    "planHash": {
      "type": "string"
    },
    "expectedSchemaRevision": {
      "$ref": "urn:mirabase:wire#/$defs/Revision"
    }
  },
  "required": [
    "planHash",
    "expectedSchemaRevision"
  ],
  "additionalProperties": false
}
```


**Response:** 202; application/json.


```json
{
  "$ref": "urn:mirabase:wire#/$defs/Job"
}
```


Referenced types: [DatabaseId](wire-schema-reference.md#schema-databaseid), [Job](wire-schema-reference.md#schema-job), [OpaqueId](wire-schema-reference.md#schema-opaqueid), [Revision](wire-schema-reference.md#schema-revision).

**Errors:** `invalid_request`, `forbidden`, `not_found`, `generation_mismatch`, `schema_mismatch`, `policy_changed`, `resource_exhausted`.

**Requirement:** MB-MIGRATIONS. **Acceptance:** AT-MIGRATIONS.

**Additional parameter/phase constraints:**


```json
{
  "pathParameters": [
    {
      "name": "id",
      "schema": {
        "$ref": "urn:mirabase:wire#/$defs/DatabaseId"
      }
    },
    {
      "name": "planId",
      "schema": {
        "$ref": "urn:mirabase:wire#/$defs/OpaqueId"
      }
    }
  ]
}
```


## Operation template.publish

**Delivery:** slice. Immutable templates containing released field/index kinds only.

`POST /api/mirabase/v1/templates`

**Authority:** superuser. **Scope:** resolved-application.

**Preconditions:** authorized immutable DatabaseContext; ready target unless lifecycle operation; current generation/policy/schema.

**Effects:** Freeze hash/revision and schema requirements.

**Idempotency:** Idempotency-Key; canonical hash and target-local receipt for mutations, durable intent for lifecycle jobs.

**Request schema:**


```json
{
  "type": "object",
  "properties": {
    "template": {
      "$ref": "urn:mirabase:wire#/$defs/Template"
    },
    "sourceDatabaseId": {
      "$ref": "urn:mirabase:wire#/$defs/DatabaseId"
    },
    "expectedSchemaRevision": {
      "$ref": "urn:mirabase:wire#/$defs/Revision"
    }
  },
  "required": [
    "template",
    "sourceDatabaseId",
    "expectedSchemaRevision"
  ],
  "additionalProperties": false
}
```


**Response:** 200; application/json.


```json
{
  "$ref": "urn:mirabase:wire#/$defs/Template"
}
```


Referenced types: [DatabaseId](wire-schema-reference.md#schema-databaseid), [Revision](wire-schema-reference.md#schema-revision), [Template](wire-schema-reference.md#schema-template).

**Errors:** `invalid_request`, `forbidden`, `not_found`, `generation_mismatch`, `schema_mismatch`, `policy_changed`, `resource_exhausted`.

**Requirement:** MB-ROLLOUT. **Acceptance:** AT-ROLLOUT.

**Additional parameter/phase constraints:**


```json
{
  "pathParameters": []
}
```


## Operation template.rollout

**Delivery:** slice. Explicit fixed IDs/generations; journaled per-target results; no structured fleet selector or advanced control API.

`POST /api/mirabase/v1/templates/{templateId}/rollouts`

**Authority:** superuser. **Scope:** resolved-application.

**Preconditions:** authorized immutable DatabaseContext; ready target unless lifecycle operation; current generation/policy/schema.

**Effects:** Snapshot target set and run bounded per-context migrations.

**Idempotency:** Idempotency-Key; canonical hash and target-local receipt for mutations, durable intent for lifecycle jobs.

**Request schema:**


```json
{
  "type": "object",
  "properties": {
    "revision": {
      "$ref": "urn:mirabase:wire#/$defs/Revision"
    },
    "databaseIds": {
      "type": "array",
      "items": {
        "$ref": "urn:mirabase:wire#/$defs/DatabaseId"
      },
      "maxItems": 10000
    },
    "stopOnFailure": {
      "type": "boolean"
    },
    "targetSelection": {
      "$ref": "urn:mirabase:wire#/$defs/TargetSelector"
    }
  },
  "required": [
    "revision",
    "stopOnFailure"
  ],
  "additionalProperties": false,
  "oneOf": [
    {
      "required": [
        "databaseIds"
      ],
      "not": {
        "required": [
          "targetSelection"
        ]
      }
    },
    {
      "required": [
        "targetSelection"
      ],
      "not": {
        "required": [
          "databaseIds"
        ]
      }
    }
  ]
}
```


**Response:** 202; application/json.


```json
{
  "$ref": "urn:mirabase:wire#/$defs/Job"
}
```


Referenced types: [DatabaseId](wire-schema-reference.md#schema-databaseid), [Job](wire-schema-reference.md#schema-job), [OpaqueId](wire-schema-reference.md#schema-opaqueid), [Revision](wire-schema-reference.md#schema-revision), [TargetSelector](wire-schema-reference.md#schema-targetselector).

**Errors:** `invalid_request`, `forbidden`, `not_found`, `generation_mismatch`, `schema_mismatch`, `policy_changed`, `resource_exhausted`.

**Requirement:** MB-ROLLOUT. **Acceptance:** AT-ROLLOUT.

**Additional parameter/phase constraints:**


```json
{
  "pathParameters": [
    {
      "name": "templateId",
      "schema": {
        "$ref": "urn:mirabase:wire#/$defs/OpaqueId"
      }
    }
  ]
}
```


## Operation search

**Delivery:** slice. Vector or text only; no fuzzy/hybrid branch.

`POST /api/mirabase/v1/databases/{id}/collections/{collection}/search`

**Authority:** record.read and collection list/view/field rules. **Scope:** resolved-application.

**Preconditions:** authorized immutable DatabaseContext; ready target unless lifecycle operation; current generation/policy/schema.

**Effects:** Bounded typed read; no hidden global unauthorized candidate search.

**Idempotency:** read-only.

**Request schema:**


```json
{
  "$ref": "urn:mirabase:wire#/$defs/SearchRequest"
}
```


**Response:** 200; application/json.


```json
{
  "$ref": "urn:mirabase:wire#/$defs/SearchResult"
}
```


Referenced types: [DatabaseId](wire-schema-reference.md#schema-databaseid), [OpaqueId](wire-schema-reference.md#schema-opaqueid), [SearchRequest](wire-schema-reference.md#schema-searchrequest), [SearchResult](wire-schema-reference.md#schema-searchresult).

**Errors:** `invalid_request`, `forbidden`, `not_found`, `generation_mismatch`, `schema_mismatch`, `policy_changed`, `resource_exhausted`.

**Requirement:** MB-HYBRID. **Acceptance:** AT-HYBRID.

**Additional parameter/phase constraints:**


```json
{
  "pathParameters": [
    {
      "name": "id",
      "schema": {
        "$ref": "urn:mirabase:wire#/$defs/DatabaseId"
      }
    },
    {
      "name": "collection",
      "schema": {
        "$ref": "urn:mirabase:wire#/$defs/OpaqueId"
      }
    }
  ]
}
```


## Operation changes.pull

**Delivery:** roadmap. Unregistered in v0.1; full operation retained for its owning roadmap group.

`GET /api/mirabase/v1/databases/{id}/changes`

**Authority:** changes.read and current record rules. **Scope:** resolved-application.

**Preconditions:** authorized immutable DatabaseContext; ready target unless lifecycle operation; current generation/policy/schema.

**Effects:** Read complete authorized commit frames.

**Idempotency:** read-only.

**Request schema:**


```json
{
  "type": "object",
  "properties": {
    "cursor": {
      "type": "string"
    },
    "limit": {
      "type": "integer",
      "minimum": 1,
      "maximum": 1000
    },
    "collections": {
      "type": "array",
      "items": {
        "type": "string"
      },
      "maxItems": 100
    }
  },
  "required": [
    "cursor"
  ],
  "additionalProperties": false
}
```


**Response:** 200; application/json.


```json
{
  "$ref": "urn:mirabase:wire#/$defs/ChangePage"
}
```


Referenced types: [ChangePage](wire-schema-reference.md#schema-changepage), [DatabaseId](wire-schema-reference.md#schema-databaseid).

**Errors:** `invalid_request`, `forbidden`, `not_found`, `generation_mismatch`, `schema_mismatch`, `policy_changed`, `resource_exhausted`.

**Requirement:** MB-CDC. **Acceptance:** AT-CDC.

**Additional parameter/phase constraints:**


```json
{
  "pathParameters": [
    {
      "name": "id",
      "schema": {
        "$ref": "urn:mirabase:wire#/$defs/DatabaseId"
      }
    }
  ]
}
```


## Operation changes.stream

**Delivery:** roadmap. Unregistered in v0.1; full operation retained for its owning roadmap group.

`GET /api/mirabase/v1/databases/{id}/changes/stream`

**Authority:** changes.read and current record rules. **Scope:** resolved-application.

**Preconditions:** authorized immutable DatabaseContext; ready target unless lifecycle operation; current generation/policy/schema.

**Effects:** Bounded durable SSE session with resume cursors.

**Idempotency:** read-only.

**Request schema:**


```json
{
  "type": "object",
  "properties": {
    "cursor": {
      "type": "string"
    }
  },
  "required": [
    "cursor"
  ],
  "additionalProperties": false
}
```


**Response:** 200; text/event-stream.


```json
{
  "type": "object",
  "properties": {
    "contentType": {
      "type": "string",
      "enum": [
        "text/event-stream"
      ]
    },
    "frameSchema": {
      "type": "string",
      "enum": [
        "ChangeTransaction"
      ]
    },
    "heartbeatSeconds": {
      "type": "integer",
      "minimum": 1,
      "maximum": 60
    }
  },
  "required": [
    "contentType",
    "frameSchema",
    "heartbeatSeconds"
  ],
  "additionalProperties": false
}
```


Referenced types: [DatabaseId](wire-schema-reference.md#schema-databaseid).

**Errors:** `invalid_request`, `forbidden`, `not_found`, `generation_mismatch`, `schema_mismatch`, `policy_changed`, `resource_exhausted`.

**Requirement:** MB-RETENTION. **Acceptance:** AT-RETENTION.

**Additional parameter/phase constraints:**


```json
{
  "pathParameters": [
    {
      "name": "id",
      "schema": {
        "$ref": "urn:mirabase:wire#/$defs/DatabaseId"
      }
    }
  ]
}
```


## Operation sync.bootstrap

**Delivery:** roadmap. Unregistered in v0.1; full operation retained for its owning roadmap group.

`POST /api/mirabase/v1/databases/{id}/sync/bootstrap`

**Authority:** record.sync and collection rules. **Scope:** resolved-application.

**Preconditions:** authorized immutable DatabaseContext; ready target unless lifecycle operation; current generation/policy/schema.

**Effects:** Create authorized consistent snapshot lease; no global secrets.

**Idempotency:** Idempotency-Key; canonical hash and target-local receipt for mutations, durable intent for lifecycle jobs.

**Request schema:**


```json
{
  "type": "object",
  "properties": {
    "collections": {
      "type": "array",
      "items": {
        "type": "string"
      },
      "maxItems": 100
    },
    "projection": {
      "type": "array",
      "items": {
        "type": "string"
      },
      "maxItems": 100
    },
    "filterPresetId": {
      "anyOf": [
        {
          "type": "string"
        },
        {
          "type": "null"
        }
      ]
    },
    "knownSchemaRevision": {
      "anyOf": [
        {
          "$ref": "urn:mirabase:wire#/$defs/Revision"
        },
        {
          "type": "null"
        }
      ]
    },
    "clientInstanceId": {
      "type": "string"
    }
  },
  "required": [
    "collections",
    "projection",
    "filterPresetId",
    "knownSchemaRevision",
    "clientInstanceId"
  ],
  "additionalProperties": false
}
```


**Response:** 200; application/json.


```json
{
  "$ref": "urn:mirabase:wire#/$defs/Bootstrap"
}
```


Referenced types: [Bootstrap](wire-schema-reference.md#schema-bootstrap), [DatabaseId](wire-schema-reference.md#schema-databaseid), [Revision](wire-schema-reference.md#schema-revision).

**Errors:** `invalid_request`, `forbidden`, `not_found`, `generation_mismatch`, `schema_mismatch`, `policy_changed`, `resource_exhausted`.

**Requirement:** MB-RSYNC. **Acceptance:** AT-RSYNC.

**Additional parameter/phase constraints:**


```json
{
  "pathParameters": [
    {
      "name": "id",
      "schema": {
        "$ref": "urn:mirabase:wire#/$defs/DatabaseId"
      }
    }
  ]
}
```


## Operation sync.page

**Delivery:** roadmap. Unregistered in v0.1; full operation retained for its owning roadmap group.

`GET /api/mirabase/v1/databases/{id}/sync/bootstrap/{sessionId}/pages/{index}`

**Authority:** same subject and snapshot policy epoch. **Scope:** resolved-application.

**Preconditions:** authorized immutable DatabaseContext; ready target unless lifecycle operation; current generation/policy/schema.

**Effects:** Read one authorized immutable bootstrap page.

**Idempotency:** read-only.

**Request schema:**


```json
{
  "type": "null"
}
```


**Response:** 200; application/json.


```json
{
  "$ref": "urn:mirabase:wire#/$defs/BootstrapPage"
}
```


Referenced types: [BootstrapPage](wire-schema-reference.md#schema-bootstrappage), [DatabaseId](wire-schema-reference.md#schema-databaseid), [OpaqueId](wire-schema-reference.md#schema-opaqueid).

**Errors:** `invalid_request`, `forbidden`, `not_found`, `generation_mismatch`, `schema_mismatch`, `policy_changed`, `resource_exhausted`.

**Requirement:** MB-RSYNC. **Acceptance:** AT-RSYNC.

**Additional parameter/phase constraints:**


```json
{
  "pathParameters": [
    {
      "name": "id",
      "schema": {
        "$ref": "urn:mirabase:wire#/$defs/DatabaseId"
      }
    },
    {
      "name": "sessionId",
      "schema": {
        "$ref": "urn:mirabase:wire#/$defs/OpaqueId"
      }
    },
    {
      "name": "index",
      "schema": {
        "type": "integer",
        "minimum": 0,
        "maximum": 9007199254740991
      }
    }
  ]
}
```


## Operation sync.push

**Delivery:** roadmap. Unregistered in v0.1; full operation retained for its owning roadmap group.

`POST /api/mirabase/v1/databases/{id}/sync/push`

**Authority:** record.sync plus per-mutation record/file rules. **Scope:** resolved-application.

**Preconditions:** authorized immutable DatabaseContext; ready target unless lifecycle operation; current generation/policy/schema.

**Effects:** Validate/hooks and commit each group with receipt.

**Idempotency:** Idempotency-Key; canonical hash and target-local receipt for mutations, durable intent for lifecycle jobs.

**Request schema:**


```json
{
  "$ref": "urn:mirabase:wire#/$defs/Push"
}
```


**Response:** 200; application/json.


```json
{
  "$ref": "urn:mirabase:wire#/$defs/PushResult"
}
```


Referenced types: [DatabaseId](wire-schema-reference.md#schema-databaseid), [Push](wire-schema-reference.md#schema-push), [PushResult](wire-schema-reference.md#schema-pushresult).

**Errors:** `invalid_request`, `forbidden`, `not_found`, `generation_mismatch`, `schema_mismatch`, `policy_changed`, `resource_exhausted`.

**Requirement:** MB-RSYNC. **Acceptance:** AT-RSYNC.

**Additional parameter/phase constraints:**


```json
{
  "pathParameters": [
    {
      "name": "id",
      "schema": {
        "$ref": "urn:mirabase:wire#/$defs/DatabaseId"
      }
    }
  ]
}
```


## Operation sync.pull

**Delivery:** roadmap. Unregistered in v0.1; full operation retained for its owning roadmap group.

`GET /api/mirabase/v1/databases/{id}/sync/pull`

**Authority:** record.sync plus changes.read/current rules. **Scope:** resolved-application.

**Preconditions:** authorized immutable DatabaseContext; ready target unless lifecycle operation; current generation/policy/schema.

**Effects:** Read authorized change page.

**Idempotency:** read-only.

**Request schema:**


```json
{
  "type": "object",
  "properties": {
    "cursor": {
      "type": "string"
    },
    "limit": {
      "type": "integer",
      "minimum": 1,
      "maximum": 1000
    }
  },
  "required": [
    "cursor"
  ],
  "additionalProperties": false
}
```


**Response:** 200; application/json.


```json
{
  "$ref": "urn:mirabase:wire#/$defs/ChangePage"
}
```


Referenced types: [ChangePage](wire-schema-reference.md#schema-changepage), [DatabaseId](wire-schema-reference.md#schema-databaseid).

**Errors:** `invalid_request`, `forbidden`, `not_found`, `generation_mismatch`, `schema_mismatch`, `policy_changed`, `resource_exhausted`.

**Requirement:** MB-RSYNC. **Acceptance:** AT-RSYNC.

**Additional parameter/phase constraints:**


```json
{
  "pathParameters": [
    {
      "name": "id",
      "schema": {
        "$ref": "urn:mirabase:wire#/$defs/DatabaseId"
      }
    }
  ]
}
```


## Operation sync.ack

**Delivery:** roadmap. Unregistered in v0.1; full operation retained for its owning roadmap group.

`POST /api/mirabase/v1/databases/{id}/sync/ack`

**Authority:** record.sync and matching client identity. **Scope:** resolved-application.

**Preconditions:** authorized immutable DatabaseContext; ready target unless lifecycle operation; current generation/policy/schema.

**Effects:** Persist acknowledgement no later than applied authorized cursor.

**Idempotency:** Idempotency-Key; canonical hash and target-local receipt for mutations, durable intent for lifecycle jobs.

**Request schema:**


```json
{
  "type": "object",
  "properties": {
    "clientInstanceId": {
      "type": "string"
    },
    "cursor": {
      "type": "string"
    }
  },
  "required": [
    "clientInstanceId",
    "cursor"
  ],
  "additionalProperties": false
}
```


**Response:** 200; application/json.


```json
{
  "type": "object",
  "properties": {
    "accepted": {
      "type": "boolean"
    },
    "leaseExpiresAt": {
      "$ref": "urn:mirabase:wire#/$defs/Timestamp"
    }
  },
  "required": [
    "accepted",
    "leaseExpiresAt"
  ],
  "additionalProperties": false
}
```


Referenced types: [DatabaseId](wire-schema-reference.md#schema-databaseid), [Timestamp](wire-schema-reference.md#schema-timestamp).

**Errors:** `invalid_request`, `forbidden`, `not_found`, `generation_mismatch`, `schema_mismatch`, `policy_changed`, `resource_exhausted`.

**Requirement:** MB-RETENTION. **Acceptance:** AT-RETENTION.

**Additional parameter/phase constraints:**


```json
{
  "pathParameters": [
    {
      "name": "id",
      "schema": {
        "$ref": "urn:mirabase:wire#/$defs/DatabaseId"
      }
    }
  ]
}
```


## Operation sync.file.stage

**Delivery:** roadmap. Unregistered in v0.1; full operation retained for its owning roadmap group.

`POST /api/mirabase/v1/databases/{id}/sync/files/stage`

**Authority:** record.sync/files.write and field rules. **Scope:** resolved-application.

**Preconditions:** authorized immutable DatabaseContext; ready target unless lifecycle operation; current generation/policy/schema.

**Effects:** Create bounded authorized staging intent.

**Idempotency:** Idempotency-Key; canonical hash and target-local receipt for mutations, durable intent for lifecycle jobs.

**Request schema:**


```json
{
  "type": "object",
  "properties": {
    "clientInstanceId": {
      "type": "string"
    },
    "file": {
      "$ref": "urn:mirabase:wire#/$defs/FileEntry"
    }
  },
  "required": [
    "clientInstanceId",
    "file"
  ],
  "additionalProperties": false
}
```


**Response:** 200; application/json.


```json
{
  "type": "object",
  "properties": {
    "stageId": {
      "type": "string"
    },
    "uploadUrl": {
      "type": "string"
    },
    "expiresAt": {
      "$ref": "urn:mirabase:wire#/$defs/Timestamp"
    }
  },
  "required": [
    "stageId",
    "uploadUrl",
    "expiresAt"
  ],
  "additionalProperties": false
}
```


Referenced types: [DatabaseId](wire-schema-reference.md#schema-databaseid), [FileEntry](wire-schema-reference.md#schema-fileentry), [Timestamp](wire-schema-reference.md#schema-timestamp).

**Errors:** `invalid_request`, `forbidden`, `not_found`, `generation_mismatch`, `schema_mismatch`, `policy_changed`, `resource_exhausted`.

**Requirement:** MB-SYNCFILES. **Acceptance:** AT-SYNCFILES.

**Additional parameter/phase constraints:**


```json
{
  "pathParameters": [
    {
      "name": "id",
      "schema": {
        "$ref": "urn:mirabase:wire#/$defs/DatabaseId"
      }
    }
  ]
}
```


## Operation sync.file.upload

**Delivery:** roadmap. Unregistered in v0.1; full operation retained for its owning roadmap group.

`PUT /api/mirabase/v1/databases/{id}/sync/files/{stageId}`

**Authority:** same subject/context and unexpired staging token. **Scope:** resolved-application.

**Preconditions:** authorized immutable DatabaseContext; ready target unless lifecycle operation; current generation/policy/schema.

**Effects:** Stream/hash/validate bytes; atomic stage finalization.

**Idempotency:** Idempotency-Key; canonical hash and target-local receipt for mutations, durable intent for lifecycle jobs.

**Request schema:**


```json
{
  "type": "string",
  "contentMediaType": "application/octet-stream"
}
```


**Response:** 200; application/json.


```json
{
  "type": "object",
  "properties": {
    "stageId": {
      "type": "string"
    },
    "size": {
      "type": "integer",
      "minimum": 0,
      "maximum": 9007199254740991
    },
    "sha256": {
      "type": "string"
    },
    "state": {
      "type": "string",
      "enum": [
        "ready"
      ]
    }
  },
  "required": [
    "stageId",
    "size",
    "sha256",
    "state"
  ],
  "additionalProperties": false
}
```


Referenced types: [DatabaseId](wire-schema-reference.md#schema-databaseid), [OpaqueId](wire-schema-reference.md#schema-opaqueid).

**Errors:** `invalid_request`, `forbidden`, `not_found`, `generation_mismatch`, `schema_mismatch`, `policy_changed`, `resource_exhausted`.

**Requirement:** MB-SYNCFILES. **Acceptance:** AT-SYNCFILES.

**Additional parameter/phase constraints:**


```json
{
  "pathParameters": [
    {
      "name": "id",
      "schema": {
        "$ref": "urn:mirabase:wire#/$defs/DatabaseId"
      }
    },
    {
      "name": "stageId",
      "schema": {
        "$ref": "urn:mirabase:wire#/$defs/OpaqueId"
      }
    }
  ]
}
```


## Operation live.register

**Delivery:** roadmap. Unregistered in v0.1; full operation retained for its owning roadmap group.

`POST /api/mirabase/v1/databases/{id}/live-queries`

**Authority:** record.read/current query rules. **Scope:** resolved-application.

**Preconditions:** authorized immutable DatabaseContext; ready target unless lifecycle operation; current generation/policy/schema.

**Effects:** Create authorized snapshot/diff subscription.

**Idempotency:** Idempotency-Key; canonical hash and target-local receipt for mutations, durable intent for lifecycle jobs.

**Request schema:**


```json
{
  "type": "object",
  "properties": {
    "queryId": {
      "type": "string"
    },
    "parameters": {
      "type": "array",
      "items": {
        "$ref": "urn:mirabase:wire#/$defs/TypedValue"
      },
      "maxItems": 100
    },
    "schemaRevision": {
      "$ref": "urn:mirabase:wire#/$defs/Revision"
    },
    "queryRevision": {
      "$ref": "urn:mirabase:wire#/$defs/Revision"
    }
  },
  "required": [
    "queryId",
    "parameters",
    "schemaRevision"
  ],
  "additionalProperties": false
}
```


**Response:** 200; application/json.


```json
{
  "$ref": "urn:mirabase:wire#/$defs/LiveQuery"
}
```


Referenced types: [DatabaseId](wire-schema-reference.md#schema-databaseid), [LiveQuery](wire-schema-reference.md#schema-livequery), [Revision](wire-schema-reference.md#schema-revision), [TypedValue](wire-schema-reference.md#schema-typedvalue).

**Errors:** `invalid_request`, `forbidden`, `not_found`, `generation_mismatch`, `schema_mismatch`, `policy_changed`, `resource_exhausted`.

**Requirement:** MB-LIVE. **Acceptance:** AT-LIVE.

**Additional parameter/phase constraints:**


```json
{
  "pathParameters": [
    {
      "name": "id",
      "schema": {
        "$ref": "urn:mirabase:wire#/$defs/DatabaseId"
      }
    }
  ]
}
```


## Operation live.stream

**Delivery:** roadmap. Unregistered in v0.1; full operation retained for its owning roadmap group.

`GET /api/mirabase/v1/databases/{id}/live-queries/{queryId}/stream`

**Authority:** matching subject/policy epoch. **Scope:** resolved-application.

**Preconditions:** authorized immutable DatabaseContext; ready target unless lifecycle operation; current generation/policy/schema.

**Effects:** Deliver ordered authorized query deltas with bounded queues.

**Idempotency:** read-only.

**Request schema:**


```json
{
  "type": "object",
  "properties": {
    "cursor": {
      "type": "string"
    }
  },
  "required": [
    "cursor"
  ],
  "additionalProperties": false
}
```


**Response:** 200; text/event-stream.


```json
{
  "type": "object",
  "properties": {
    "contentType": {
      "type": "string",
      "enum": [
        "text/event-stream"
      ]
    },
    "eventKinds": {
      "type": "array",
      "items": {
        "type": "string",
        "enum": [
          "snapshot",
          "insert",
          "update",
          "delete",
          "reset",
          "error"
        ]
      },
      "maxItems": 6
    }
  },
  "required": [
    "contentType",
    "eventKinds"
  ],
  "additionalProperties": false
}
```


Referenced types: [DatabaseId](wire-schema-reference.md#schema-databaseid), [OpaqueId](wire-schema-reference.md#schema-opaqueid).

**Errors:** `invalid_request`, `forbidden`, `not_found`, `generation_mismatch`, `schema_mismatch`, `policy_changed`, `resource_exhausted`.

**Requirement:** MB-LIVE. **Acceptance:** AT-LIVE.

**Additional parameter/phase constraints:**


```json
{
  "pathParameters": [
    {
      "name": "id",
      "schema": {
        "$ref": "urn:mirabase:wire#/$defs/DatabaseId"
      }
    },
    {
      "name": "queryId",
      "schema": {
        "$ref": "urn:mirabase:wire#/$defs/OpaqueId"
      }
    }
  ]
}
```


## Operation credential.issue

**Delivery:** roadmap. Unregistered in v0.1; full operation retained for its owning roadmap group.

`POST /api/mirabase/v1/databases/{id}/credentials`

**Authority:** superuser. **Scope:** resolved-application.

**Preconditions:** authorized immutable DatabaseContext; ready target unless lifecycle operation; current generation/policy/schema.

**Effects:** Issue one-time secret; retain verifier and audit only.

**Idempotency:** Idempotency-Key; canonical hash and target-local receipt for mutations, durable intent for lifecycle jobs.

**Request schema:**


```json
{
  "type": "object",
  "properties": {
    "purpose": {
      "type": "string",
      "enum": [
        "pg",
        "native-sync",
        "sql-http"
      ]
    },
    "scopes": {
      "type": "array",
      "items": {
        "$ref": "urn:mirabase:wire#/$defs/Scope"
      },
      "maxItems": 10
    },
    "expiresAt": {
      "$ref": "urn:mirabase:wire#/$defs/Timestamp"
    }
  },
  "required": [
    "purpose",
    "scopes",
    "expiresAt"
  ],
  "additionalProperties": false
}
```


**Response:** 200; application/json.


```json
{
  "$ref": "urn:mirabase:wire#/$defs/Credential"
}
```


Referenced types: [Credential](wire-schema-reference.md#schema-credential), [DatabaseId](wire-schema-reference.md#schema-databaseid), [Scope](wire-schema-reference.md#schema-scope), [Timestamp](wire-schema-reference.md#schema-timestamp).

**Errors:** `invalid_request`, `forbidden`, `not_found`, `generation_mismatch`, `schema_mismatch`, `policy_changed`, `resource_exhausted`.

**Requirement:** MB-PGAUTH. **Acceptance:** AT-PGAUTH.

**Additional parameter/phase constraints:**


```json
{
  "pathParameters": [
    {
      "name": "id",
      "schema": {
        "$ref": "urn:mirabase:wire#/$defs/DatabaseId"
      }
    }
  ]
}
```


## Operation credential.revoke

**Delivery:** roadmap. Unregistered in v0.1; full operation retained for its owning roadmap group.

`DELETE /api/mirabase/v1/databases/{id}/credentials/{credentialId}`

**Authority:** superuser. **Scope:** resolved-application.

**Preconditions:** authorized immutable DatabaseContext; ready target unless lifecycle operation; current generation/policy/schema.

**Effects:** Revoke credential and terminate sessions within bounded interval.

**Idempotency:** Idempotency-Key; canonical hash and target-local receipt for mutations, durable intent for lifecycle jobs.

**Request schema:**


```json
{
  "type": "null"
}
```


**Response:** 200; application/json.


```json
{
  "type": "object",
  "properties": {
    "id": {
      "type": "string"
    },
    "revoked": {
      "type": "boolean"
    }
  },
  "required": [
    "id",
    "revoked"
  ],
  "additionalProperties": false
}
```


Referenced types: [DatabaseId](wire-schema-reference.md#schema-databaseid), [OpaqueId](wire-schema-reference.md#schema-opaqueid).

**Errors:** `invalid_request`, `forbidden`, `not_found`, `generation_mismatch`, `schema_mismatch`, `policy_changed`, `resource_exhausted`.

**Requirement:** MB-PGAUTH. **Acceptance:** AT-PGAUTH.

**Additional parameter/phase constraints:**


```json
{
  "pathParameters": [
    {
      "name": "id",
      "schema": {
        "$ref": "urn:mirabase:wire#/$defs/DatabaseId"
      }
    },
    {
      "name": "credentialId",
      "schema": {
        "$ref": "urn:mirabase:wire#/$defs/OpaqueId"
      }
    }
  ]
}
```


## Operation job.get

**Delivery:** v0.1. Applicable exact contract within one default application and released features.

`GET /api/mirabase/v1/jobs/{jobId}`

**Authority:** superuser or explicit job actor visibility policy. **Scope:** resolved-application.

**Preconditions:** authorized immutable DatabaseContext; ready target unless lifecycle operation; current generation/policy/schema.

**Effects:** Inspect actual job phase/effects.

**Idempotency:** read-only.

**Request schema:**


```json
{
  "type": "null"
}
```


**Response:** 202; application/json.


```json
{
  "$ref": "urn:mirabase:wire#/$defs/Job"
}
```


Referenced types: [Job](wire-schema-reference.md#schema-job), [OpaqueId](wire-schema-reference.md#schema-opaqueid).

**Errors:** `invalid_request`, `forbidden`, `not_found`, `generation_mismatch`, `schema_mismatch`, `policy_changed`, `resource_exhausted`.

**Requirement:** MB-JOBS. **Acceptance:** AT-JOBS.

**Additional parameter/phase constraints:**


```json
{
  "pathParameters": [
    {
      "name": "jobId",
      "schema": {
        "$ref": "urn:mirabase:wire#/$defs/OpaqueId"
      }
    }
  ]
}
```


## Operation job.cancel

**Delivery:** v0.1. Applicable exact contract within one default application and released features.

`POST /api/mirabase/v1/jobs/{jobId}/cancel`

**Authority:** superuser. **Scope:** resolved-application.

**Preconditions:** authorized immutable DatabaseContext; ready target unless lifecycle operation; current generation/policy/schema.

**Effects:** Request safe cancellation; no pretend rollback of committed steps.

**Idempotency:** Idempotency-Key; canonical hash and target-local receipt for mutations, durable intent for lifecycle jobs.

**Request schema:**


```json
{
  "type": "null"
}
```


**Response:** 202; application/json.


```json
{
  "$ref": "urn:mirabase:wire#/$defs/Job"
}
```


Referenced types: [Job](wire-schema-reference.md#schema-job), [OpaqueId](wire-schema-reference.md#schema-opaqueid).

**Errors:** `invalid_request`, `forbidden`, `not_found`, `generation_mismatch`, `schema_mismatch`, `policy_changed`, `resource_exhausted`.

**Requirement:** MB-JOBS. **Acceptance:** AT-JOBS.

**Additional parameter/phase constraints:**


```json
{
  "pathParameters": [
    {
      "name": "jobId",
      "schema": {
        "$ref": "urn:mirabase:wire#/$defs/OpaqueId"
      }
    }
  ]
}
```


## Operation receipt.get

**Delivery:** v0.1. Applicable exact contract within one default application and released features.

`GET /api/mirabase/v1/databases/{id}/receipts/{receiptId}`

**Authority:** same actor/context or superuser. **Scope:** resolved-application.

**Preconditions:** authorized immutable DatabaseContext; ready target unless lifecycle operation; current generation/policy/schema.

**Effects:** Resolve uncertain mutation outcome without re-executing.

**Idempotency:** read-only.

**Request schema:**


```json
{
  "type": "null"
}
```


**Response:** 200; application/json.


```json
{
  "$ref": "urn:mirabase:wire#/$defs/Receipt"
}
```


Referenced types: [DatabaseId](wire-schema-reference.md#schema-databaseid), [OpaqueId](wire-schema-reference.md#schema-opaqueid), [Receipt](wire-schema-reference.md#schema-receipt).

**Errors:** `invalid_request`, `forbidden`, `not_found`, `generation_mismatch`, `schema_mismatch`, `policy_changed`, `resource_exhausted`.

**Requirement:** MB-MVCC. **Acceptance:** AT-MVCC.

**Additional parameter/phase constraints:**


```json
{
  "pathParameters": [
    {
      "name": "id",
      "schema": {
        "$ref": "urn:mirabase:wire#/$defs/DatabaseId"
      }
    },
    {
      "name": "receiptId",
      "schema": {
        "$ref": "urn:mirabase:wire#/$defs/OpaqueId"
      }
    }
  ]
}
```


## Operation backup.create

**Delivery:** slice. Primary/private/default-application coverage for WAL/MVCC/FTS; no roadmap profile claim.

`POST /api/mirabase/v1/databases/{id}/backups`

**Authority:** superuser. **Scope:** resolved-application.

**Preconditions:** authorized immutable DatabaseContext; ready target unless lifecycle operation; current generation/policy/schema.

**Effects:** Create verified context archive and file-coverage manifest.

**Idempotency:** Idempotency-Key; canonical hash and target-local receipt for mutations, durable intent for lifecycle jobs.

**Request schema:**


```json
{
  "type": "object",
  "properties": {
    "expectedGeneration": {
      "$ref": "urn:mirabase:wire#/$defs/Revision"
    },
    "includeFiles": {
      "type": "boolean"
    },
    "destination": {
      "type": "string",
      "enum": [
        "local",
        "configured-remote"
      ]
    }
  },
  "required": [
    "expectedGeneration",
    "includeFiles",
    "destination"
  ],
  "additionalProperties": false
}
```


**Response:** 202; application/json.


```json
{
  "$ref": "urn:mirabase:wire#/$defs/Job"
}
```


Referenced types: [DatabaseId](wire-schema-reference.md#schema-databaseid), [Job](wire-schema-reference.md#schema-job), [Revision](wire-schema-reference.md#schema-revision).

**Errors:** `invalid_request`, `forbidden`, `not_found`, `generation_mismatch`, `schema_mismatch`, `policy_changed`, `resource_exhausted`.

**Requirement:** MB-BACKUP. **Acceptance:** AT-BACKUP.

**Additional parameter/phase constraints:**


```json
{
  "pathParameters": [
    {
      "name": "id",
      "schema": {
        "$ref": "urn:mirabase:wire#/$defs/DatabaseId"
      }
    }
  ]
}
```


## Operation backup.restore

**Delivery:** slice. Same released formats; new generation, authority guards and verified recovery.

`POST /api/mirabase/v1/databases/{id}/restore`

**Authority:** superuser. **Scope:** resolved-application.

**Preconditions:** authorized immutable DatabaseContext; ready target unless lifecycle operation; current generation/policy/schema.

**Effects:** Validate/stage then publish with new generation if not dry-run.

**Idempotency:** Idempotency-Key; canonical hash and target-local receipt for mutations, durable intent for lifecycle jobs.

**Request schema:**


```json
{
  "type": "object",
  "properties": {
    "archiveId": {
      "type": "string"
    },
    "expectedGeneration": {
      "$ref": "urn:mirabase:wire#/$defs/Revision"
    },
    "dryRun": {
      "type": "boolean"
    },
    "restoreToNew": {
      "type": "boolean"
    },
    "authorityPolicy": {
      "type": "string",
      "enum": [
        "remove",
        "preserve-authorized"
      ]
    },
    "subjects": {
      "type": "array",
      "items": {
        "$ref": "urn:mirabase:wire#/$defs/Subject"
      },
      "maxItems": 1000
    }
  },
  "required": [
    "archiveId",
    "expectedGeneration",
    "dryRun",
    "restoreToNew",
    "authorityPolicy",
    "subjects"
  ],
  "additionalProperties": false
}
```


**Response:** 202; application/json.


```json
{
  "$ref": "urn:mirabase:wire#/$defs/Job"
}
```


Referenced types: [DatabaseId](wire-schema-reference.md#schema-databaseid), [Job](wire-schema-reference.md#schema-job), [Revision](wire-schema-reference.md#schema-revision), [Subject](wire-schema-reference.md#schema-subject).

**Errors:** `invalid_request`, `forbidden`, `not_found`, `generation_mismatch`, `schema_mismatch`, `policy_changed`, `resource_exhausted`.

**Requirement:** MB-BACKUP. **Acceptance:** AT-BACKUP.

**Additional parameter/phase constraints:**


```json
{
  "pathParameters": [
    {
      "name": "id",
      "schema": {
        "$ref": "urn:mirabase:wire#/$defs/DatabaseId"
      }
    }
  ]
}
```


## Operation export

**Delivery:** roadmap. Unregistered in v0.1; full operation retained for its owning roadmap group.

`POST /api/mirabase/v1/databases/{id}/exports`

**Authority:** superuser. **Scope:** resolved-application.

**Preconditions:** authorized immutable DatabaseContext; ready target unless lifecycle operation; current generation/policy/schema.

**Effects:** Create stable-ID typed manifest and data/file archive.

**Idempotency:** Idempotency-Key; canonical hash and target-local receipt for mutations, durable intent for lifecycle jobs.

**Request schema:**


```json
{
  "type": "object",
  "properties": {
    "format": {
      "type": "string",
      "enum": [
        "portable-v1",
        "native"
      ]
    },
    "includeFiles": {
      "type": "boolean"
    }
  },
  "required": [
    "format",
    "includeFiles"
  ],
  "additionalProperties": false
}
```


**Response:** 202; application/json.


```json
{
  "$ref": "urn:mirabase:wire#/$defs/Job"
}
```


Referenced types: [DatabaseId](wire-schema-reference.md#schema-databaseid), [Job](wire-schema-reference.md#schema-job).

**Errors:** `invalid_request`, `forbidden`, `not_found`, `generation_mismatch`, `schema_mismatch`, `policy_changed`, `resource_exhausted`.

**Requirement:** MB-PORTABLE. **Acceptance:** AT-PORTABLE.

**Additional parameter/phase constraints:**


```json
{
  "pathParameters": [
    {
      "name": "id",
      "schema": {
        "$ref": "urn:mirabase:wire#/$defs/DatabaseId"
      }
    }
  ]
}
```


## Operation import

**Delivery:** roadmap. Unregistered in v0.1; full operation retained for its owning roadmap group.

`POST /api/mirabase/v1/databases/{id}/imports`

**Authority:** superuser. **Scope:** resolved-application.

**Preconditions:** authorized immutable DatabaseContext; ready target unless lifecycle operation; current generation/policy/schema.

**Effects:** Validate portable schema/data/files, preserving IDs with explicit plan.

**Idempotency:** Idempotency-Key; canonical hash and target-local receipt for mutations, durable intent for lifecycle jobs.

**Request schema:**


```json
{
  "type": "object",
  "properties": {
    "archiveId": {
      "type": "string"
    },
    "expectedGeneration": {
      "$ref": "urn:mirabase:wire#/$defs/Revision"
    },
    "dryRun": {
      "type": "boolean"
    }
  },
  "required": [
    "archiveId",
    "expectedGeneration",
    "dryRun"
  ],
  "additionalProperties": false
}
```


**Response:** 202; application/json.


```json
{
  "$ref": "urn:mirabase:wire#/$defs/Job"
}
```


Referenced types: [DatabaseId](wire-schema-reference.md#schema-databaseid), [Job](wire-schema-reference.md#schema-job), [Revision](wire-schema-reference.md#schema-revision).

**Errors:** `invalid_request`, `forbidden`, `not_found`, `generation_mismatch`, `schema_mismatch`, `policy_changed`, `resource_exhausted`.

**Requirement:** MB-PORTABLE. **Acceptance:** AT-PORTABLE.

**Additional parameter/phase constraints:**


```json
{
  "pathParameters": [
    {
      "name": "id",
      "schema": {
        "$ref": "urn:mirabase:wire#/$defs/DatabaseId"
      }
    }
  ]
}
```


## Operation key.rotate

**Delivery:** roadmap. Unregistered in v0.1; full operation retained for its owning roadmap group.

`POST /api/mirabase/v1/databases/{id}/encryption/rotate`

**Authority:** superuser. **Scope:** resolved-application.

**Preconditions:** authorized immutable DatabaseContext; ready target unless lifecycle operation; current generation/policy/schema.

**Effects:** Verify backup/key/profile, stage re-encryption and publish safely.

**Idempotency:** Idempotency-Key; canonical hash and target-local receipt for mutations, durable intent for lifecycle jobs.

**Request schema:**


```json
{
  "type": "object",
  "properties": {
    "expectedGeneration": {
      "$ref": "urn:mirabase:wire#/$defs/Revision"
    },
    "newKeyRef": {
      "type": "string"
    },
    "algorithm": {
      "type": "string"
    },
    "backupId": {
      "type": "string"
    }
  },
  "required": [
    "expectedGeneration",
    "newKeyRef",
    "algorithm",
    "backupId"
  ],
  "additionalProperties": false
}
```


**Response:** 202; application/json.


```json
{
  "$ref": "urn:mirabase:wire#/$defs/Job"
}
```


Referenced types: [DatabaseId](wire-schema-reference.md#schema-databaseid), [Job](wire-schema-reference.md#schema-job), [Revision](wire-schema-reference.md#schema-revision).

**Errors:** `invalid_request`, `forbidden`, `not_found`, `generation_mismatch`, `schema_mismatch`, `policy_changed`, `resource_exhausted`.

**Requirement:** MB-ENCRYPT. **Acceptance:** AT-ENCRYPT.

**Additional parameter/phase constraints:**


```json
{
  "pathParameters": [
    {
      "name": "id",
      "schema": {
        "$ref": "urn:mirabase:wire#/$defs/DatabaseId"
      }
    }
  ]
}
```


## Operation database.profile

**Delivery:** v0.1. Applicable exact contract within one default application and released features.

`POST /api/mirabase/v1/databases/{id}/profile`

**Authority:** superuser. **Scope:** resolved-application.

**Preconditions:** authorized immutable DatabaseContext; ready target unless lifecycle operation; current generation/policy/schema.

**Effects:** Drain/validate/transition profile and manifest with recovery copy.

**Idempotency:** Idempotency-Key; canonical hash and target-local receipt for mutations, durable intent for lifecycle jobs.

**Request schema:**


```json
{
  "type": "object",
  "properties": {
    "expectedGeneration": {
      "$ref": "urn:mirabase:wire#/$defs/Revision"
    },
    "profile": {
      "$ref": "urn:mirabase:wire#/$defs/Profile"
    },
    "backupId": {
      "type": "string"
    }
  },
  "required": [
    "expectedGeneration",
    "profile",
    "backupId"
  ],
  "additionalProperties": false
}
```


**Response:** 202; application/json.


```json
{
  "$ref": "urn:mirabase:wire#/$defs/Job"
}
```


Referenced types: [DatabaseId](wire-schema-reference.md#schema-databaseid), [Job](wire-schema-reference.md#schema-job), [Profile](wire-schema-reference.md#schema-profile), [Revision](wire-schema-reference.md#schema-revision).

**Errors:** `invalid_request`, `forbidden`, `not_found`, `generation_mismatch`, `schema_mismatch`, `policy_changed`, `resource_exhausted`.

**Requirement:** MB-MVCC. **Acceptance:** AT-MVCC.

**Additional parameter/phase constraints:**


```json
{
  "pathParameters": [
    {
      "name": "id",
      "schema": {
        "$ref": "urn:mirabase:wire#/$defs/DatabaseId"
      }
    }
  ]
}
```


## Operation job.list

**Delivery:** v0.1. Applicable exact contract within one default application and released features.

`GET /api/mirabase/v1/jobs`

**Authority:** superuser. **Scope:** resolved-application.

**Preconditions:** authorized immutable DatabaseContext; ready target unless lifecycle operation; current generation/policy/schema.

**Effects:** Paginated authorized job status and actual results.

**Idempotency:** read-only.

**Request schema:**


```json
{
  "type": "object",
  "properties": {
    "page": {
      "type": "integer",
      "minimum": 1,
      "maximum": 9007199254740991
    },
    "perPage": {
      "type": "integer",
      "minimum": 1,
      "maximum": 1000
    },
    "databaseId": {
      "$ref": "urn:mirabase:wire#/$defs/DatabaseId"
    },
    "state": {
      "type": "string"
    }
  },
  "required": [],
  "additionalProperties": false
}
```


**Response:** 200; application/json.


```json
{
  "type": "object",
  "properties": {
    "items": {
      "type": "array",
      "items": {
        "$ref": "urn:mirabase:wire#/$defs/Job"
      },
      "maxItems": 1000
    },
    "page": {
      "type": "integer",
      "minimum": 1,
      "maximum": 9007199254740991
    },
    "perPage": {
      "type": "integer",
      "minimum": 1,
      "maximum": 1000
    },
    "totalItems": {
      "type": "integer",
      "minimum": 0,
      "maximum": 9007199254740991
    },
    "totalPages": {
      "type": "integer",
      "minimum": 0,
      "maximum": 9007199254740991
    }
  },
  "required": [
    "items",
    "page",
    "perPage",
    "totalItems",
    "totalPages"
  ],
  "additionalProperties": false
}
```


Referenced types: [DatabaseId](wire-schema-reference.md#schema-databaseid), [Job](wire-schema-reference.md#schema-job).

**Errors:** `invalid_request`, `forbidden`, `not_found`, `generation_mismatch`, `schema_mismatch`, `policy_changed`, `resource_exhausted`.

**Requirement:** MB-JOBS. **Acceptance:** AT-JOBS.

**Additional parameter/phase constraints:**


```json
{
  "pathParameters": []
}
```


## Operation template.list

**Delivery:** v0.1. Applicable exact contract within one default application and released features.

`GET /api/mirabase/v1/templates`

**Authority:** superuser. **Scope:** resolved-application.

**Preconditions:** authorized immutable DatabaseContext; ready target unless lifecycle operation; current generation/policy/schema.

**Effects:** Read immutable published template revisions.

**Idempotency:** read-only.

**Request schema:**


```json
{
  "type": "null"
}
```


**Response:** 200; application/json.


```json
{
  "type": "object",
  "properties": {
    "items": {
      "type": "array",
      "items": {
        "$ref": "urn:mirabase:wire#/$defs/Template"
      },
      "maxItems": 1000
    },
    "page": {
      "type": "integer",
      "minimum": 1,
      "maximum": 9007199254740991
    },
    "perPage": {
      "type": "integer",
      "minimum": 1,
      "maximum": 1000
    },
    "totalItems": {
      "type": "integer",
      "minimum": 0,
      "maximum": 9007199254740991
    },
    "totalPages": {
      "type": "integer",
      "minimum": 0,
      "maximum": 9007199254740991
    }
  },
  "required": [
    "items",
    "page",
    "perPage",
    "totalItems",
    "totalPages"
  ],
  "additionalProperties": false
}
```


Referenced types: [Template](wire-schema-reference.md#schema-template).

**Errors:** `invalid_request`, `forbidden`, `not_found`, `generation_mismatch`, `schema_mismatch`, `policy_changed`, `resource_exhausted`.

**Requirement:** MB-ROLLOUT. **Acceptance:** AT-ROLLOUT.

**Additional parameter/phase constraints:**


```json
{
  "pathParameters": []
}
```


## Operation template.get

**Delivery:** v0.1. Applicable exact contract within one default application and released features.

`GET /api/mirabase/v1/templates/{templateId}`

**Authority:** superuser. **Scope:** resolved-application.

**Preconditions:** authorized immutable DatabaseContext; ready target unless lifecycle operation; current generation/policy/schema.

**Effects:** Read requested immutable revision.

**Idempotency:** read-only.

**Request schema:**


```json
{
  "type": "object",
  "properties": {
    "revision": {
      "$ref": "urn:mirabase:wire#/$defs/Revision"
    }
  },
  "required": [],
  "additionalProperties": false
}
```


**Response:** 200; application/json.


```json
{
  "$ref": "urn:mirabase:wire#/$defs/Template"
}
```


Referenced types: [OpaqueId](wire-schema-reference.md#schema-opaqueid), [Revision](wire-schema-reference.md#schema-revision), [Template](wire-schema-reference.md#schema-template).

**Errors:** `invalid_request`, `forbidden`, `not_found`, `generation_mismatch`, `schema_mismatch`, `policy_changed`, `resource_exhausted`.

**Requirement:** MB-ROLLOUT. **Acceptance:** AT-ROLLOUT.

**Additional parameter/phase constraints:**


```json
{
  "pathParameters": [
    {
      "name": "templateId",
      "schema": {
        "$ref": "urn:mirabase:wire#/$defs/OpaqueId"
      }
    }
  ]
}
```


## Operation credential.list

**Delivery:** roadmap. Unregistered in v0.1; full operation retained for its owning roadmap group.

`GET /api/mirabase/v1/databases/{id}/credentials`

**Authority:** superuser. **Scope:** resolved-application.

**Preconditions:** authorized immutable DatabaseContext; ready target unless lifecycle operation; current generation/policy/schema.

**Effects:** List metadata only; no issued secret.

**Idempotency:** read-only.

**Request schema:**


```json
{
  "type": "null"
}
```


**Response:** 200; application/json.


```json
{
  "type": "object",
  "properties": {
    "items": {
      "type": "array",
      "items": {
        "type": "object",
        "properties": {
          "id": {
            "type": "string"
          },
          "purpose": {
            "type": "string",
            "enum": [
              "pg",
              "native-sync",
              "sql-http"
            ]
          },
          "scopes": {
            "type": "array",
            "items": {
              "$ref": "urn:mirabase:wire#/$defs/Scope"
            },
            "maxItems": 10
          },
          "expiresAt": {
            "$ref": "urn:mirabase:wire#/$defs/Timestamp"
          },
          "revokedAt": {
            "anyOf": [
              {
                "$ref": "urn:mirabase:wire#/$defs/Timestamp"
              },
              {
                "type": "null"
              }
            ]
          }
        },
        "required": [
          "id",
          "purpose",
          "scopes",
          "expiresAt",
          "revokedAt"
        ],
        "additionalProperties": false
      },
      "maxItems": 1000
    }
  },
  "required": [
    "items"
  ],
  "additionalProperties": false
}
```


Referenced types: [DatabaseId](wire-schema-reference.md#schema-databaseid), [Scope](wire-schema-reference.md#schema-scope), [Timestamp](wire-schema-reference.md#schema-timestamp).

**Errors:** `invalid_request`, `forbidden`, `not_found`, `generation_mismatch`, `schema_mismatch`, `policy_changed`, `resource_exhausted`.

**Requirement:** MB-PGAUTH. **Acceptance:** AT-PGAUTH.

**Additional parameter/phase constraints:**


```json
{
  "pathParameters": [
    {
      "name": "id",
      "schema": {
        "$ref": "urn:mirabase:wire#/$defs/DatabaseId"
      }
    }
  ]
}
```


## Operation agent.list

**Delivery:** v0.1. Applicable exact contract within one default application and released features.

`GET /api/mirabase/v1/agents`

**Authority:** superuser or configured owner registration policy. **Scope:** resolved-application.

**Preconditions:** authorized immutable DatabaseContext; ready target unless lifecycle operation; current generation/policy/schema.

**Effects:** List only authorized protected bindings.

**Idempotency:** read-only.

**Request schema:**


```json
{
  "type": "object",
  "properties": {
    "page": {
      "type": "integer",
      "minimum": 1,
      "maximum": 9007199254740991
    },
    "perPage": {
      "type": "integer",
      "minimum": 1,
      "maximum": 1000
    }
  },
  "required": [],
  "additionalProperties": false
}
```


**Response:** 200; application/json.


```json
{
  "type": "object",
  "properties": {
    "items": {
      "type": "array",
      "items": {
        "type": "object",
        "properties": {
          "id": {
            "type": "string"
          },
          "agent": {
            "$ref": "urn:mirabase:wire#/$defs/Subject"
          },
          "owner": {
            "$ref": "urn:mirabase:wire#/$defs/Subject"
          },
          "state": {
            "type": "string",
            "enum": [
              "active",
              "revoked"
            ]
          }
        },
        "required": [
          "id",
          "agent",
          "owner",
          "state"
        ],
        "additionalProperties": false
      },
      "maxItems": 1000
    },
    "page": {
      "type": "integer",
      "minimum": 1,
      "maximum": 9007199254740991
    },
    "perPage": {
      "type": "integer",
      "minimum": 1,
      "maximum": 1000
    },
    "totalItems": {
      "type": "integer",
      "minimum": 0,
      "maximum": 9007199254740991
    },
    "totalPages": {
      "type": "integer",
      "minimum": 0,
      "maximum": 9007199254740991
    }
  },
  "required": [
    "items",
    "page",
    "perPage",
    "totalItems",
    "totalPages"
  ],
  "additionalProperties": false
}
```


Referenced types: [Subject](wire-schema-reference.md#schema-subject).

**Errors:** `invalid_request`, `forbidden`, `not_found`, `generation_mismatch`, `schema_mismatch`, `policy_changed`, `resource_exhausted`.

**Requirement:** MB-AGENT. **Acceptance:** AT-AGENT.

**Additional parameter/phase constraints:**


```json
{
  "pathParameters": []
}
```


## Operation live.cancel

**Delivery:** roadmap. Unregistered in v0.1; full operation retained for its owning roadmap group.

`DELETE /api/mirabase/v1/databases/{id}/live-queries/{queryId}`

**Authority:** same subject or superuser. **Scope:** resolved-application.

**Preconditions:** authorized immutable DatabaseContext; ready target unless lifecycle operation; current generation/policy/schema.

**Effects:** Release subscription and native resources idempotently.

**Idempotency:** Idempotency-Key; canonical hash and target-local receipt for mutations, durable intent for lifecycle jobs.

**Request schema:**


```json
{
  "type": "null"
}
```


**Response:** 200; application/json.


```json
{
  "type": "object",
  "properties": {
    "id": {
      "type": "string"
    },
    "closed": {
      "type": "boolean"
    }
  },
  "required": [
    "id",
    "closed"
  ],
  "additionalProperties": false
}
```


Referenced types: [DatabaseId](wire-schema-reference.md#schema-databaseid), [OpaqueId](wire-schema-reference.md#schema-opaqueid).

**Errors:** `invalid_request`, `forbidden`, `not_found`, `generation_mismatch`, `schema_mismatch`, `policy_changed`, `resource_exhausted`.

**Requirement:** MB-LIVE. **Acceptance:** AT-LIVE.

**Additional parameter/phase constraints:**


```json
{
  "pathParameters": [
    {
      "name": "id",
      "schema": {
        "$ref": "urn:mirabase:wire#/$defs/DatabaseId"
      }
    },
    {
      "name": "queryId",
      "schema": {
        "$ref": "urn:mirabase:wire#/$defs/OpaqueId"
      }
    }
  ]
}
```


## Operation pbimport.create

**Delivery:** slice. Shared-layout pinned PocketBase source only; real rows/files/auth, preview, validation, cutover and recovery; no ownership split.

`POST /api/mirabase/v1/pocketbase-imports`

**Authority:** superuser; session actor or authorized superuser; opaque session IDs never authorize paths. **Scope:** resolved-application.

**Preconditions:** superuser and authorized session ownership; allowed import phase; active work is stopped at a safe checkpoint before edits/cancel; session/source/plan/report hashes and primary generation match the operation contract; plan edits/staging/cancellation require that no active publication marker exists; accepted application writes remain authoritative.

**Effects:** Create isolated full-app import session and durable source-upload intent.

**Idempotency:** Idempotency-Key; canonical hash and target-local receipt for mutations, durable intent for lifecycle jobs.

**Request schema:**


```json
{
  "$ref": "urn:mirabase:wire#/$defs/ImportCreate"
}
```


**Response:** 200; application/json.


```json
{
  "$ref": "urn:mirabase:wire#/$defs/ImportSession"
}
```


Referenced types: [ImportCreate](wire-schema-reference.md#schema-importcreate), [ImportSession](wire-schema-reference.md#schema-importsession).

**Errors:** `invalid_request`, `unauthenticated`, `forbidden`, `not_found`, `source_data_required`, `source_changed`, `source_incomplete`, `plan_stale`, `unresolved_import_issues`, `report_stale`, `generation_mismatch`, `post_cutover_writes_present`, `resource_exhausted`, `storage_failure`, `outcome_unknown`.

**Requirement:** MB-PBIMPORT. **Acceptance:** AT-PBIMPORT.

**Additional parameter/phase constraints:**


```json
{
  "importPhases": [
    "no-session"
  ],
  "pathParameters": []
}
```


## Operation pbimport.get

**Delivery:** slice. Shared-layout pinned PocketBase source only; real rows/files/auth, preview, validation, cutover and recovery; no ownership split.

`GET /api/mirabase/v1/pocketbase-imports/{importId}`

**Authority:** superuser; session actor or authorized superuser; opaque session IDs never authorize paths. **Scope:** resolved-application.

**Preconditions:** superuser and authorized session ownership; allowed import phase; active work is stopped at a safe checkpoint before edits/cancel; session/source/plan/report hashes and primary generation match the operation contract; plan edits/staging/cancellation require that no active publication marker exists; accepted application writes remain authoritative.

**Effects:** Read sanitized source/plan/job/recovery state.

**Idempotency:** read-only.

**Request schema:**


```json
{
  "type": "null"
}
```


**Response:** 200; application/json.


```json
{
  "$ref": "urn:mirabase:wire#/$defs/ImportSession"
}
```


Referenced types: [ImportSession](wire-schema-reference.md#schema-importsession), [OpaqueId](wire-schema-reference.md#schema-opaqueid).

**Errors:** `invalid_request`, `unauthenticated`, `forbidden`, `not_found`, `source_data_required`, `source_changed`, `source_incomplete`, `plan_stale`, `unresolved_import_issues`, `report_stale`, `generation_mismatch`, `post_cutover_writes_present`, `resource_exhausted`, `storage_failure`, `outcome_unknown`.

**Requirement:** MB-TENANTUI. **Acceptance:** AT-TENANTUI.

**Additional parameter/phase constraints:**


```json
{
  "importPhases": [
    "any"
  ],
  "pathParameters": [
    {
      "name": "importId",
      "schema": {
        "$ref": "urn:mirabase:wire#/$defs/OpaqueId"
      }
    }
  ]
}
```


## Operation pbimport.source.upload

**Delivery:** slice. Shared-layout pinned PocketBase source only; real rows/files/auth, preview, validation, cutover and recovery; no ownership split.

`PUT /api/mirabase/v1/pocketbase-imports/{importId}/source`

**Authority:** superuser; session actor or authorized superuser; opaque session IDs never authorize paths. **Scope:** resolved-application.

**Preconditions:** superuser and authorized session ownership; allowed import phase; active work is stopped at a safe checkpoint before edits/cancel; session/source/plan/report hashes and primary generation match the operation contract; plan edits/staging/cancellation require that no active publication marker exists; accepted application writes remain authoritative.

**Effects:** Stream a bounded resumable archive chunk; verify range/hash; publish source-ready only after complete digest match.

**Idempotency:** Idempotency-Key; canonical hash and target-local receipt for mutations, durable intent for lifecycle jobs.

**Request schema:**


```json
{
  "type": "string",
  "contentMediaType": "application/octet-stream"
}
```


**Response:** 200; application/json.


```json
{
  "$ref": "urn:mirabase:wire#/$defs/ImportUpload"
}
```


Referenced types: [ImportUpload](wire-schema-reference.md#schema-importupload), [OpaqueId](wire-schema-reference.md#schema-opaqueid).

**Errors:** `invalid_request`, `unauthenticated`, `forbidden`, `not_found`, `source_data_required`, `source_changed`, `source_incomplete`, `plan_stale`, `unresolved_import_issues`, `report_stale`, `generation_mismatch`, `post_cutover_writes_present`, `resource_exhausted`, `storage_failure`, `outcome_unknown`.

**Requirement:** MB-PBIMPORT. **Acceptance:** AT-PBIMPORT.

**Additional parameter/phase constraints:**


```json
{
  "importPhases": [
    "awaiting-source"
  ],
  "extraParameters": [
    {
      "name": "Content-Range",
      "in": "header",
      "required": true,
      "schema": {
        "type": "string",
        "pattern": "^bytes [0-9]+-[0-9]+/[0-9]+$"
      }
    },
    {
      "name": "X-Chunk-SHA256",
      "in": "header",
      "required": true,
      "schema": {
        "type": "string",
        "pattern": "^[a-f0-9]{64}$"
      }
    },
    {
      "name": "If-Match",
      "in": "header",
      "required": true,
      "schema": {
        "type": "string",
        "description": "Expected import-session revision ETag"
      }
    }
  ],
  "pathParameters": [
    {
      "name": "importId",
      "schema": {
        "$ref": "urn:mirabase:wire#/$defs/OpaqueId"
      }
    }
  ]
}
```


## Operation pbimport.source.get

**Delivery:** slice. Shared-layout pinned PocketBase source only; real rows/files/auth, preview, validation, cutover and recovery; no ownership split.

`GET /api/mirabase/v1/pocketbase-imports/{importId}/source`

**Authority:** superuser; session actor or authorized superuser; opaque session IDs never authorize paths. **Scope:** resolved-application.

**Preconditions:** superuser and authorized session ownership; allowed import phase; active work is stopped at a safe checkpoint before edits/cancel; session/source/plan/report hashes and primary generation match the operation contract; plan edits/staging/cancellation require that no active publication marker exists; accepted application writes remain authoritative.

**Effects:** Read actual received-byte offset/hash state for a resumed upload.

**Idempotency:** read-only.

**Request schema:**


```json
{
  "type": "null"
}
```


**Response:** 200; application/json.


```json
{
  "$ref": "urn:mirabase:wire#/$defs/ImportUpload"
}
```


Referenced types: [ImportUpload](wire-schema-reference.md#schema-importupload), [OpaqueId](wire-schema-reference.md#schema-opaqueid).

**Errors:** `invalid_request`, `unauthenticated`, `forbidden`, `not_found`, `source_data_required`, `source_changed`, `source_incomplete`, `plan_stale`, `unresolved_import_issues`, `report_stale`, `generation_mismatch`, `post_cutover_writes_present`, `resource_exhausted`, `storage_failure`, `outcome_unknown`.

**Requirement:** MB-PBIMPORT. **Acceptance:** AT-PBIMPORT.

**Additional parameter/phase constraints:**


```json
{
  "importPhases": [
    "awaiting-source",
    "source-ready",
    "failed"
  ],
  "pathParameters": [
    {
      "name": "importId",
      "schema": {
        "$ref": "urn:mirabase:wire#/$defs/OpaqueId"
      }
    }
  ]
}
```


## Operation pbimport.analyze

**Delivery:** slice. Shared-layout pinned PocketBase source only; real rows/files/auth, preview, validation, cutover and recovery; no ownership split.

`POST /api/mirabase/v1/pocketbase-imports/{importId}/analyze`

**Authority:** superuser; session actor or authorized superuser; opaque session IDs never authorize paths. **Scope:** resolved-application.

**Preconditions:** superuser and authorized session ownership; allowed import phase; active work is stopped at a safe checkpoint before edits/cancel; session/source/plan/report hashes and primary generation match the operation contract; plan edits/staging/cancellation require that no active publication marker exists; accepted application writes remain authoritative.

**Effects:** Recover a private source copy and build versioned actual-row/schema/file/auth inventory.

**Idempotency:** Idempotency-Key; canonical hash and target-local receipt for mutations, durable intent for lifecycle jobs.

**Request schema:**


```json
{
  "type": "object",
  "properties": {
    "expectedRevision": {
      "$ref": "urn:mirabase:wire#/$defs/Revision"
    },
    "sourceHash": {
      "$ref": "urn:mirabase:wire#/$defs/ImportHash"
    }
  },
  "required": [
    "expectedRevision",
    "sourceHash"
  ],
  "additionalProperties": false
}
```


**Response:** 202; application/json.


```json
{
  "$ref": "urn:mirabase:wire#/$defs/Job"
}
```


Referenced types: [ImportHash](wire-schema-reference.md#schema-importhash), [Job](wire-schema-reference.md#schema-job), [OpaqueId](wire-schema-reference.md#schema-opaqueid), [Revision](wire-schema-reference.md#schema-revision).

**Errors:** `invalid_request`, `unauthenticated`, `forbidden`, `not_found`, `source_data_required`, `source_changed`, `source_incomplete`, `plan_stale`, `unresolved_import_issues`, `report_stale`, `generation_mismatch`, `post_cutover_writes_present`, `resource_exhausted`, `storage_failure`, `outcome_unknown`.

**Requirement:** MB-PBIMPORT. **Acceptance:** AT-PBIMPORT.

**Additional parameter/phase constraints:**


```json
{
  "importPhases": [
    "source-ready"
  ],
  "pathParameters": [
    {
      "name": "importId",
      "schema": {
        "$ref": "urn:mirabase:wire#/$defs/OpaqueId"
      }
    }
  ]
}
```


## Operation pbimport.inventory

**Delivery:** slice. Shared-layout pinned PocketBase source only; real rows/files/auth, preview, validation, cutover and recovery; no ownership split.

`GET /api/mirabase/v1/pocketbase-imports/{importId}/inventory`

**Authority:** superuser; session actor or authorized superuser; opaque session IDs never authorize paths. **Scope:** resolved-application.

**Preconditions:** superuser and authorized session ownership; allowed import phase; active work is stopped at a safe checkpoint before edits/cancel; session/source/plan/report hashes and primary generation match the operation contract; plan edits/staging/cancellation require that no active publication marker exists; accepted application writes remain authoritative.

**Effects:** Paginated real source collection counts and compatibility metadata.

**Idempotency:** read-only.

**Request schema:**


```json
{
  "type": "object",
  "properties": {
    "page": {
      "type": "integer",
      "minimum": 1,
      "maximum": 9007199254740991
    },
    "perPage": {
      "type": "integer",
      "minimum": 1,
      "maximum": 1000
    }
  },
  "required": [],
  "additionalProperties": false
}
```


**Response:** 200; application/json.


```json
{
  "$ref": "urn:mirabase:wire#/$defs/ImportInventoryPage"
}
```


Referenced types: [ImportInventoryPage](wire-schema-reference.md#schema-importinventorypage), [OpaqueId](wire-schema-reference.md#schema-opaqueid).

**Errors:** `invalid_request`, `unauthenticated`, `forbidden`, `not_found`, `source_data_required`, `source_changed`, `source_incomplete`, `plan_stale`, `unresolved_import_issues`, `report_stale`, `generation_mismatch`, `post_cutover_writes_present`, `resource_exhausted`, `storage_failure`, `outcome_unknown`.

**Requirement:** MB-PBIMPORT. **Acceptance:** AT-PBIMPORT.

**Additional parameter/phase constraints:**


```json
{
  "importPhases": [
    "draft",
    "previewing",
    "previewed",
    "staging",
    "staged",
    "validating",
    "ready-for-cutover",
    "active",
    "failed"
  ],
  "pathParameters": [
    {
      "name": "importId",
      "schema": {
        "$ref": "urn:mirabase:wire#/$defs/OpaqueId"
      }
    }
  ]
}
```


## Operation pbimport.plan.put

**Delivery:** slice. Shared-layout pinned PocketBase source only; real rows/files/auth, preview, validation, cutover and recovery; no ownership split.

`PUT /api/mirabase/v1/pocketbase-imports/{importId}/plan`

**Authority:** superuser; session actor or authorized superuser; opaque session IDs never authorize paths. **Scope:** resolved-application.

**Preconditions:** superuser and authorized session ownership; allowed import phase; active work is stopped at a safe checkpoint before edits/cancel; session/source/plan/report hashes and primary generation match the operation contract; plan edits/staging/cancellation require that no active publication marker exists; accepted application writes remain authoritative.

**Effects:** Compile ownership/access/adaptation plan; new hash invalidates old preview/staging/report.

**Idempotency:** Idempotency-Key; canonical hash and target-local receipt for mutations, durable intent for lifecycle jobs.

**Request schema:**


```json
{
  "$ref": "urn:mirabase:wire#/$defs/ImportPlanDraft"
}
```


**Response:** 200; application/json.


```json
{
  "$ref": "urn:mirabase:wire#/$defs/ImportPlan"
}
```


Referenced types: [ImportPlan](wire-schema-reference.md#schema-importplan), [ImportPlanDraft](wire-schema-reference.md#schema-importplandraft), [OpaqueId](wire-schema-reference.md#schema-opaqueid).

**Errors:** `invalid_request`, `unauthenticated`, `forbidden`, `not_found`, `source_data_required`, `source_changed`, `source_incomplete`, `plan_stale`, `unresolved_import_issues`, `report_stale`, `generation_mismatch`, `post_cutover_writes_present`, `resource_exhausted`, `storage_failure`, `outcome_unknown`.

**Requirement:** MB-TENANTPLAN. **Acceptance:** AT-TENANTPLAN.

**Additional parameter/phase constraints:**


```json
{
  "importPhases": [
    "draft",
    "previewed",
    "staged",
    "ready-for-cutover",
    "failed"
  ],
  "pathParameters": [
    {
      "name": "importId",
      "schema": {
        "$ref": "urn:mirabase:wire#/$defs/OpaqueId"
      }
    }
  ]
}
```


## Operation pbimport.plan.get

**Delivery:** slice. Shared-layout pinned PocketBase source only; real rows/files/auth, preview, validation, cutover and recovery; no ownership split.

`GET /api/mirabase/v1/pocketbase-imports/{importId}/plan`

**Authority:** superuser; session actor or authorized superuser; opaque session IDs never authorize paths. **Scope:** resolved-application.

**Preconditions:** superuser and authorized session ownership; allowed import phase; active work is stopped at a safe checkpoint before edits/cancel; session/source/plan/report hashes and primary generation match the operation contract; plan edits/staging/cancellation require that no active publication marker exists; accepted application writes remain authoritative.

**Effects:** Read exact current reviewed mapping without secrets.

**Idempotency:** read-only.

**Request schema:**


```json
{
  "type": "null"
}
```


**Response:** 200; application/json.


```json
{
  "$ref": "urn:mirabase:wire#/$defs/ImportPlan"
}
```


Referenced types: [ImportPlan](wire-schema-reference.md#schema-importplan), [OpaqueId](wire-schema-reference.md#schema-opaqueid).

**Errors:** `invalid_request`, `unauthenticated`, `forbidden`, `not_found`, `source_data_required`, `source_changed`, `source_incomplete`, `plan_stale`, `unresolved_import_issues`, `report_stale`, `generation_mismatch`, `post_cutover_writes_present`, `resource_exhausted`, `storage_failure`, `outcome_unknown`.

**Requirement:** MB-TENANTPLAN. **Acceptance:** AT-TENANTPLAN.

**Additional parameter/phase constraints:**


```json
{
  "importPhases": [
    "draft",
    "previewing",
    "previewed",
    "staging",
    "staged",
    "validating",
    "ready-for-cutover",
    "active",
    "failed"
  ],
  "pathParameters": [
    {
      "name": "importId",
      "schema": {
        "$ref": "urn:mirabase:wire#/$defs/OpaqueId"
      }
    }
  ]
}
```


## Operation pbimport.preview.compute

**Delivery:** slice. Shared-layout pinned PocketBase source only; real rows/files/auth, preview, validation, cutover and recovery; no ownership split.

`POST /api/mirabase/v1/pocketbase-imports/{importId}/preview`

**Authority:** superuser; session actor or authorized superuser; opaque session IDs never authorize paths. **Scope:** resolved-application.

**Preconditions:** superuser and authorized session ownership; allowed import phase; active work is stopped at a safe checkpoint before edits/cancel; session/source/plan/report hashes and primary generation match the operation contract; plan edits/staging/cancellation require that no active publication marker exists; accepted application writes remain authoritative.

**Effects:** Assign actual source rows/files through ownership graph; retain all issues and provenance.

**Idempotency:** Idempotency-Key; canonical hash and target-local receipt for mutations, durable intent for lifecycle jobs.

**Request schema:**


```json
{
  "$ref": "urn:mirabase:wire#/$defs/ImportGuard"
}
```


**Response:** 202; application/json.


```json
{
  "$ref": "urn:mirabase:wire#/$defs/Job"
}
```


Referenced types: [ImportGuard](wire-schema-reference.md#schema-importguard), [Job](wire-schema-reference.md#schema-job), [OpaqueId](wire-schema-reference.md#schema-opaqueid).

**Errors:** `invalid_request`, `unauthenticated`, `forbidden`, `not_found`, `source_data_required`, `source_changed`, `source_incomplete`, `plan_stale`, `unresolved_import_issues`, `report_stale`, `generation_mismatch`, `post_cutover_writes_present`, `resource_exhausted`, `storage_failure`, `outcome_unknown`.

**Requirement:** MB-TENANTCHECK. **Acceptance:** AT-TENANTCHECK.

**Additional parameter/phase constraints:**


```json
{
  "importPhases": [
    "draft",
    "previewed"
  ],
  "pathParameters": [
    {
      "name": "importId",
      "schema": {
        "$ref": "urn:mirabase:wire#/$defs/OpaqueId"
      }
    }
  ]
}
```


## Operation pbimport.preview.get

**Delivery:** slice. Shared-layout pinned PocketBase source only; real rows/files/auth, preview, validation, cutover and recovery; no ownership split.

`GET /api/mirabase/v1/pocketbase-imports/{importId}/preview`

**Authority:** superuser; session actor or authorized superuser; opaque session IDs never authorize paths. **Scope:** resolved-application.

**Preconditions:** superuser and authorized session ownership; allowed import phase; active work is stopped at a safe checkpoint before edits/cancel; session/source/plan/report hashes and primary generation match the operation contract; plan edits/staging/cancellation require that no active publication marker exists; accepted application writes remain authoritative.

**Effects:** Return real per-plan reconciliation totals; not a validation success.

**Idempotency:** read-only.

**Request schema:**


```json
{
  "type": "null"
}
```


**Response:** 200; application/json.


```json
{
  "$ref": "urn:mirabase:wire#/$defs/ImportPreview"
}
```


Referenced types: [ImportPreview](wire-schema-reference.md#schema-importpreview), [OpaqueId](wire-schema-reference.md#schema-opaqueid).

**Errors:** `invalid_request`, `unauthenticated`, `forbidden`, `not_found`, `source_data_required`, `source_changed`, `source_incomplete`, `plan_stale`, `unresolved_import_issues`, `report_stale`, `generation_mismatch`, `post_cutover_writes_present`, `resource_exhausted`, `storage_failure`, `outcome_unknown`.

**Requirement:** MB-TENANTCHECK. **Acceptance:** AT-TENANTCHECK.

**Additional parameter/phase constraints:**


```json
{
  "importPhases": [
    "previewed",
    "staging",
    "staged",
    "validating",
    "ready-for-cutover",
    "active",
    "failed"
  ],
  "pathParameters": [
    {
      "name": "importId",
      "schema": {
        "$ref": "urn:mirabase:wire#/$defs/OpaqueId"
      }
    }
  ]
}
```


## Operation pbimport.tenants

**Delivery:** roadmap. Unregistered in v0.1; full operation retained for its owning roadmap group.

`GET /api/mirabase/v1/pocketbase-imports/{importId}/tenants`

**Authority:** superuser; session actor or authorized superuser; opaque session IDs never authorize paths. **Scope:** resolved-application.

**Preconditions:** superuser and authorized session ownership; allowed import phase; active work is stopped at a safe checkpoint before edits/cancel; session/source/plan/report hashes and primary generation match the operation contract; plan edits/staging/cancellation require that no active publication marker exists; accepted application writes remain authoritative.

**Effects:** Paginated tenant databases, memberships, data/file counts and assignment hashes.

**Idempotency:** read-only.

**Request schema:**


```json
{
  "type": "object",
  "properties": {
    "page": {
      "type": "integer",
      "minimum": 1,
      "maximum": 9007199254740991
    },
    "perPage": {
      "type": "integer",
      "minimum": 1,
      "maximum": 1000
    }
  },
  "required": [],
  "additionalProperties": false
}
```


**Response:** 200; application/json.


```json
{
  "$ref": "urn:mirabase:wire#/$defs/ImportTenantPreviewPage"
}
```


Referenced types: [ImportTenantPreviewPage](wire-schema-reference.md#schema-importtenantpreviewpage), [OpaqueId](wire-schema-reference.md#schema-opaqueid).

**Errors:** `invalid_request`, `unauthenticated`, `forbidden`, `not_found`, `source_data_required`, `source_changed`, `source_incomplete`, `plan_stale`, `unresolved_import_issues`, `report_stale`, `generation_mismatch`, `post_cutover_writes_present`, `resource_exhausted`, `storage_failure`, `outcome_unknown`.

**Requirement:** MB-TENANTCHECK. **Acceptance:** AT-TENANTCHECK.

**Additional parameter/phase constraints:**


```json
{
  "importPhases": [
    "previewed",
    "staging",
    "staged",
    "validating",
    "ready-for-cutover",
    "active",
    "failed"
  ],
  "pathParameters": [
    {
      "name": "importId",
      "schema": {
        "$ref": "urn:mirabase:wire#/$defs/OpaqueId"
      }
    }
  ]
}
```


## Operation pbimport.issues

**Delivery:** slice. Shared-layout pinned PocketBase source only; real rows/files/auth, preview, validation, cutover and recovery; no ownership split.

`GET /api/mirabase/v1/pocketbase-imports/{importId}/issues`

**Authority:** superuser; session actor or authorized superuser; opaque session IDs never authorize paths. **Scope:** resolved-application.

**Preconditions:** superuser and authorized session ownership; allowed import phase; active work is stopped at a safe checkpoint before edits/cancel; session/source/plan/report hashes and primary generation match the operation contract; plan edits/staging/cancellation require that no active publication marker exists; accepted application writes remain authoritative.

**Effects:** List exact ownership/file/access/compatibility issues and allowed plan resolutions.

**Idempotency:** read-only.

**Request schema:**


```json
{
  "type": "object",
  "properties": {
    "page": {
      "type": "integer",
      "minimum": 1,
      "maximum": 9007199254740991
    },
    "perPage": {
      "type": "integer",
      "minimum": 1,
      "maximum": 1000
    },
    "severity": {
      "type": "string",
      "enum": [
        "error",
        "warning",
        "info"
      ]
    }
  },
  "required": [],
  "additionalProperties": false
}
```


**Response:** 200; application/json.


```json
{
  "$ref": "urn:mirabase:wire#/$defs/ImportIssuePage"
}
```


Referenced types: [ImportIssuePage](wire-schema-reference.md#schema-importissuepage), [OpaqueId](wire-schema-reference.md#schema-opaqueid).

**Errors:** `invalid_request`, `unauthenticated`, `forbidden`, `not_found`, `source_data_required`, `source_changed`, `source_incomplete`, `plan_stale`, `unresolved_import_issues`, `report_stale`, `generation_mismatch`, `post_cutover_writes_present`, `resource_exhausted`, `storage_failure`, `outcome_unknown`.

**Requirement:** MB-TENANTPLAN. **Acceptance:** AT-TENANTPLAN.

**Additional parameter/phase constraints:**


```json
{
  "importPhases": [
    "draft",
    "previewed",
    "staged",
    "ready-for-cutover",
    "active",
    "failed"
  ],
  "pathParameters": [
    {
      "name": "importId",
      "schema": {
        "$ref": "urn:mirabase:wire#/$defs/OpaqueId"
      }
    }
  ]
}
```


## Operation pbimport.stage

**Delivery:** slice. Shared-layout pinned PocketBase source only; real rows/files/auth, preview, validation, cutover and recovery; no ownership split.

`POST /api/mirabase/v1/pocketbase-imports/{importId}/stage`

**Authority:** superuser; session actor or authorized superuser; opaque session IDs never authorize paths. **Scope:** resolved-application.

**Preconditions:** superuser and authorized session ownership; allowed import phase; active work is stopped at a safe checkpoint before edits/cancel; session/source/plan/report hashes and primary generation match the operation contract; plan edits/staging/cancellation require that no active publication marker exists; accepted application writes remain authoritative.

**Effects:** Copy verified source into unpublished context roots with stable IDs/file hashes and suppressed side effects.

**Idempotency:** Idempotency-Key; canonical hash and target-local receipt for mutations, durable intent for lifecycle jobs.

**Request schema:**


```json
{
  "type": "object",
  "properties": {
    "expectedRevision": {
      "$ref": "urn:mirabase:wire#/$defs/Revision"
    },
    "planHash": {
      "$ref": "urn:mirabase:wire#/$defs/ImportHash"
    },
    "previewHash": {
      "$ref": "urn:mirabase:wire#/$defs/ImportHash"
    }
  },
  "required": [
    "expectedRevision",
    "planHash",
    "previewHash"
  ],
  "additionalProperties": false
}
```


**Response:** 202; application/json.


```json
{
  "$ref": "urn:mirabase:wire#/$defs/Job"
}
```


Referenced types: [ImportHash](wire-schema-reference.md#schema-importhash), [Job](wire-schema-reference.md#schema-job), [OpaqueId](wire-schema-reference.md#schema-opaqueid), [Revision](wire-schema-reference.md#schema-revision).

**Errors:** `invalid_request`, `unauthenticated`, `forbidden`, `not_found`, `source_data_required`, `source_changed`, `source_incomplete`, `plan_stale`, `unresolved_import_issues`, `report_stale`, `generation_mismatch`, `post_cutover_writes_present`, `resource_exhausted`, `storage_failure`, `outcome_unknown`.

**Requirement:** MB-TENANTDATA. **Acceptance:** AT-TENANTDATA.

**Additional parameter/phase constraints:**


```json
{
  "importPhases": [
    "previewed"
  ],
  "pathParameters": [
    {
      "name": "importId",
      "schema": {
        "$ref": "urn:mirabase:wire#/$defs/OpaqueId"
      }
    }
  ]
}
```


## Operation pbimport.validate

**Delivery:** slice. Shared-layout pinned PocketBase source only; real rows/files/auth, preview, validation, cutover and recovery; no ownership split.

`POST /api/mirabase/v1/pocketbase-imports/{importId}/validate`

**Authority:** superuser; session actor or authorized superuser; opaque session IDs never authorize paths. **Scope:** resolved-application.

**Preconditions:** superuser and authorized session ownership; allowed import phase; active work is stopped at a safe checkpoint before edits/cancel; session/source/plan/report hashes and primary generation match the operation contract; plan edits/staging/cancellation require that no active publication marker exists; accepted application writes remain authoritative.

**Effects:** Run real staged reconciliation/integrity/auth/rule/routing/recovery fixture checks.

**Idempotency:** Idempotency-Key; canonical hash and target-local receipt for mutations, durable intent for lifecycle jobs.

**Request schema:**


```json
{
  "$ref": "urn:mirabase:wire#/$defs/ImportGuard"
}
```


**Response:** 202; application/json.


```json
{
  "$ref": "urn:mirabase:wire#/$defs/Job"
}
```


Referenced types: [ImportGuard](wire-schema-reference.md#schema-importguard), [Job](wire-schema-reference.md#schema-job), [OpaqueId](wire-schema-reference.md#schema-opaqueid).

**Errors:** `invalid_request`, `unauthenticated`, `forbidden`, `not_found`, `source_data_required`, `source_changed`, `source_incomplete`, `plan_stale`, `unresolved_import_issues`, `report_stale`, `generation_mismatch`, `post_cutover_writes_present`, `resource_exhausted`, `storage_failure`, `outcome_unknown`.

**Requirement:** MB-TENANTACCESS. **Acceptance:** AT-TENANTACCESS.

**Additional parameter/phase constraints:**


```json
{
  "importPhases": [
    "staged"
  ],
  "pathParameters": [
    {
      "name": "importId",
      "schema": {
        "$ref": "urn:mirabase:wire#/$defs/OpaqueId"
      }
    }
  ]
}
```


## Operation pbimport.report

**Delivery:** slice. Shared-layout pinned PocketBase source only; real rows/files/auth, preview, validation, cutover and recovery; no ownership split.

`GET /api/mirabase/v1/pocketbase-imports/{importId}/report`

**Authority:** superuser; session actor or authorized superuser; opaque session IDs never authorize paths. **Scope:** resolved-application.

**Preconditions:** superuser and authorized session ownership; allowed import phase; active work is stopped at a safe checkpoint before edits/cancel; session/source/plan/report hashes and primary generation match the operation contract; plan edits/staging/cancellation require that no active publication marker exists; accepted application writes remain authoritative.

**Effects:** Read hash-bound actual verification artifacts and any unexecuted/failed case.

**Idempotency:** read-only.

**Request schema:**


```json
{
  "type": "null"
}
```


**Response:** 200; application/json.


```json
{
  "$ref": "urn:mirabase:wire#/$defs/ImportValidation"
}
```


Referenced types: [ImportValidation](wire-schema-reference.md#schema-importvalidation), [OpaqueId](wire-schema-reference.md#schema-opaqueid).

**Errors:** `invalid_request`, `unauthenticated`, `forbidden`, `not_found`, `source_data_required`, `source_changed`, `source_incomplete`, `plan_stale`, `unresolved_import_issues`, `report_stale`, `generation_mismatch`, `post_cutover_writes_present`, `resource_exhausted`, `storage_failure`, `outcome_unknown`.

**Requirement:** MB-TENANTCHECK. **Acceptance:** AT-TENANTCHECK.

**Additional parameter/phase constraints:**


```json
{
  "importPhases": [
    "ready-for-cutover",
    "active",
    "failed"
  ],
  "pathParameters": [
    {
      "name": "importId",
      "schema": {
        "$ref": "urn:mirabase:wire#/$defs/OpaqueId"
      }
    }
  ]
}
```


## Operation pbimport.report.cases

**Delivery:** slice. Shared-layout pinned PocketBase source only; real rows/files/auth, preview, validation, cutover and recovery; no ownership split.

`GET /api/mirabase/v1/pocketbase-imports/{importId}/report/cases`

**Authority:** superuser; session actor or authorized superuser; opaque session IDs never authorize paths. **Scope:** resolved-application.

**Preconditions:** superuser and authorized session ownership; allowed import phase; active work is stopped at a safe checkpoint before edits/cancel; session/source/plan/report hashes and primary generation match the operation contract; plan edits/staging/cancellation require that no active publication marker exists; accepted application writes remain authoritative.

**Effects:** Paginate actual validation cases and artifact hashes without loading all verification results.

**Idempotency:** read-only.

**Request schema:**


```json
{
  "type": "object",
  "properties": {
    "page": {
      "type": "integer",
      "minimum": 1,
      "maximum": 9007199254740991
    },
    "perPage": {
      "type": "integer",
      "minimum": 1,
      "maximum": 1000
    }
  },
  "required": [],
  "additionalProperties": false
}
```


**Response:** 200; application/json.


```json
{
  "$ref": "urn:mirabase:wire#/$defs/ImportValidationCasePage"
}
```


Referenced types: [ImportValidationCasePage](wire-schema-reference.md#schema-importvalidationcasepage), [OpaqueId](wire-schema-reference.md#schema-opaqueid).

**Errors:** `invalid_request`, `unauthenticated`, `forbidden`, `not_found`, `source_data_required`, `source_changed`, `source_incomplete`, `plan_stale`, `unresolved_import_issues`, `report_stale`, `generation_mismatch`, `post_cutover_writes_present`, `resource_exhausted`, `storage_failure`, `outcome_unknown`.

**Requirement:** MB-TENANTCHECK. **Acceptance:** AT-TENANTCHECK.

**Additional parameter/phase constraints:**


```json
{
  "importPhases": [
    "ready-for-cutover",
    "active",
    "failed"
  ],
  "pathParameters": [
    {
      "name": "importId",
      "schema": {
        "$ref": "urn:mirabase:wire#/$defs/OpaqueId"
      }
    }
  ]
}
```


## Operation pbimport.cutover

**Delivery:** slice. Shared-layout pinned PocketBase source only; real rows/files/auth, preview, validation, cutover and recovery; no ownership split.

`POST /api/mirabase/v1/pocketbase-imports/{importId}/cutover`

**Authority:** superuser; session actor or authorized superuser; opaque session IDs never authorize paths. **Scope:** resolved-application.

**Preconditions:** superuser and authorized session ownership; allowed import phase; active work is stopped at a safe checkpoint before edits/cancel; session/source/plan/report hashes and primary generation match the operation contract; plan edits/staging/cancellation require that no active publication marker exists; accepted application writes remain authoritative.

**Effects:** Require exact passing report/source/plan/generation, then quiesce and publish recoverable app root/route epoch.

**Idempotency:** Idempotency-Key; canonical hash and target-local receipt for mutations, durable intent for lifecycle jobs.

**Request schema:**


```json
{
  "$ref": "urn:mirabase:wire#/$defs/ImportCutover"
}
```


**Response:** 202; application/json.


```json
{
  "$ref": "urn:mirabase:wire#/$defs/Job"
}
```


Referenced types: [ImportCutover](wire-schema-reference.md#schema-importcutover), [Job](wire-schema-reference.md#schema-job), [OpaqueId](wire-schema-reference.md#schema-opaqueid).

**Errors:** `invalid_request`, `unauthenticated`, `forbidden`, `not_found`, `source_data_required`, `source_changed`, `source_incomplete`, `plan_stale`, `unresolved_import_issues`, `report_stale`, `generation_mismatch`, `post_cutover_writes_present`, `resource_exhausted`, `storage_failure`, `outcome_unknown`.

**Requirement:** MB-TENANTCUTOVER. **Acceptance:** AT-TENANTCUTOVER.

**Additional parameter/phase constraints:**


```json
{
  "importPhases": [
    "ready-for-cutover"
  ],
  "pathParameters": [
    {
      "name": "importId",
      "schema": {
        "$ref": "urn:mirabase:wire#/$defs/OpaqueId"
      }
    }
  ]
}
```


## Operation pbimport.resume

**Delivery:** slice. Shared-layout pinned PocketBase source only; real rows/files/auth, preview, validation, cutover and recovery; no ownership split.

`POST /api/mirabase/v1/pocketbase-imports/{importId}/resume`

**Authority:** superuser; session actor or authorized superuser; opaque session IDs never authorize paths. **Scope:** resolved-application.

**Preconditions:** superuser and authorized session ownership; allowed import phase; active work is stopped at a safe checkpoint before edits/cancel; session/source/plan/report hashes and primary generation match the operation contract; plan edits/staging/cancellation require that no active publication marker exists; accepted application writes remain authoritative.

**Effects:** Reconcile last durable phase and batch/journal checkpoints; never repeat acknowledged effects or trust stale source.

**Idempotency:** Idempotency-Key; canonical hash and target-local receipt for mutations, durable intent for lifecycle jobs.

**Request schema:**


```json
{
  "$ref": "urn:mirabase:wire#/$defs/ImportSessionGuard"
}
```


**Response:** 202; application/json.


```json
{
  "$ref": "urn:mirabase:wire#/$defs/Job"
}
```


Referenced types: [ImportSessionGuard](wire-schema-reference.md#schema-importsessionguard), [Job](wire-schema-reference.md#schema-job), [OpaqueId](wire-schema-reference.md#schema-opaqueid).

**Errors:** `invalid_request`, `unauthenticated`, `forbidden`, `not_found`, `source_data_required`, `source_changed`, `source_incomplete`, `plan_stale`, `unresolved_import_issues`, `report_stale`, `generation_mismatch`, `post_cutover_writes_present`, `resource_exhausted`, `storage_failure`, `outcome_unknown`.

**Requirement:** MB-TENANTRECOVERY. **Acceptance:** AT-TENANTRECOVERY.

**Additional parameter/phase constraints:**


```json
{
  "importPhases": [
    "failed"
  ],
  "pathParameters": [
    {
      "name": "importId",
      "schema": {
        "$ref": "urn:mirabase:wire#/$defs/OpaqueId"
      }
    }
  ]
}
```


## Operation pbimport.cancel

**Delivery:** slice. Shared-layout pinned PocketBase source only; real rows/files/auth, preview, validation, cutover and recovery; no ownership split.

`POST /api/mirabase/v1/pocketbase-imports/{importId}/cancel`

**Authority:** superuser; session actor or authorized superuser; opaque session IDs never authorize paths. **Scope:** resolved-application.

**Preconditions:** superuser and authorized session ownership; allowed import phase; active work is stopped at a safe checkpoint before edits/cancel; session/source/plan/report hashes and primary generation match the operation contract; plan edits/staging/cancellation require that no active publication marker exists; accepted application writes remain authoritative.

**Effects:** Cancel only before publication; preserve recovery/source evidence and remove only session-owned unpublished data.

**Idempotency:** Idempotency-Key; canonical hash and target-local receipt for mutations, durable intent for lifecycle jobs.

**Request schema:**


```json
{
  "$ref": "urn:mirabase:wire#/$defs/ImportSessionGuard"
}
```


**Response:** 202; application/json.


```json
{
  "$ref": "urn:mirabase:wire#/$defs/Job"
}
```


Referenced types: [ImportSessionGuard](wire-schema-reference.md#schema-importsessionguard), [Job](wire-schema-reference.md#schema-job), [OpaqueId](wire-schema-reference.md#schema-opaqueid).

**Errors:** `invalid_request`, `unauthenticated`, `forbidden`, `not_found`, `source_data_required`, `source_changed`, `source_incomplete`, `plan_stale`, `unresolved_import_issues`, `report_stale`, `generation_mismatch`, `post_cutover_writes_present`, `resource_exhausted`, `storage_failure`, `outcome_unknown`.

**Requirement:** MB-TENANTRECOVERY. **Acceptance:** AT-TENANTRECOVERY.

**Additional parameter/phase constraints:**


```json
{
  "importPhases": [
    "awaiting-source",
    "source-ready",
    "analyzing",
    "draft",
    "previewing",
    "previewed",
    "staging",
    "staged",
    "validating",
    "ready-for-cutover",
    "failed"
  ],
  "pathParameters": [
    {
      "name": "importId",
      "schema": {
        "$ref": "urn:mirabase:wire#/$defs/OpaqueId"
      }
    }
  ]
}
```


## Operation pbimport.recovery

**Delivery:** slice. Shared-layout pinned PocketBase source only; real rows/files/auth, preview, validation, cutover and recovery; no ownership split.

`POST /api/mirabase/v1/pocketbase-imports/{importId}/recovery`

**Authority:** superuser; session actor or authorized superuser; opaque session IDs never authorize paths. **Scope:** resolved-application.

**Preconditions:** superuser and authorized session ownership; allowed import phase; active work is stopped at a safe checkpoint before edits/cancel; session/source/plan/report hashes and primary generation match the operation contract; plan edits/staging/cancellation require that no active publication marker exists; accepted application writes remain authoritative.

**Effects:** Before new writes allow verified prior-root restoration; afterward export verified current state and preserve it as authoritative.

**Idempotency:** Idempotency-Key; canonical hash and target-local receipt for mutations, durable intent for lifecycle jobs.

**Request schema:**


```json
{
  "$ref": "urn:mirabase:wire#/$defs/ImportRecovery"
}
```


**Response:** 202; application/json.


```json
{
  "$ref": "urn:mirabase:wire#/$defs/Job"
}
```


Referenced types: [ImportRecovery](wire-schema-reference.md#schema-importrecovery), [Job](wire-schema-reference.md#schema-job), [OpaqueId](wire-schema-reference.md#schema-opaqueid).

**Errors:** `invalid_request`, `unauthenticated`, `forbidden`, `not_found`, `source_data_required`, `source_changed`, `source_incomplete`, `plan_stale`, `unresolved_import_issues`, `report_stale`, `generation_mismatch`, `post_cutover_writes_present`, `resource_exhausted`, `storage_failure`, `outcome_unknown`.

**Requirement:** MB-TENANTRECOVERY. **Acceptance:** AT-TENANTRECOVERY.

**Additional parameter/phase constraints:**


```json
{
  "importPhases": [
    "cutting-over",
    "active",
    "failed",
    "recovery-exported"
  ],
  "pathParameters": [
    {
      "name": "importId",
      "schema": {
        "$ref": "urn:mirabase:wire#/$defs/OpaqueId"
      }
    }
  ]
}
```


## Operation pbimport.artifact.get

**Delivery:** slice. Shared-layout pinned PocketBase source only; real rows/files/auth, preview, validation, cutover and recovery; no ownership split.

`GET /api/mirabase/v1/pocketbase-imports/{importId}/artifacts/{artifactId}`

**Authority:** superuser; session actor or authorized superuser; opaque session IDs never authorize paths. **Scope:** resolved-application.

**Preconditions:** superuser and authorized session ownership; allowed import phase; active work is stopped at a safe checkpoint before edits/cancel; session/source/plan/report hashes and primary generation match the operation contract; plan edits/staging/cancellation require that no active publication marker exists; accepted application writes remain authoritative.

**Effects:** Stream a manifest-listed preview/map/report/recovery artifact after session authorization; verified IDs/hashes never select arbitrary paths.

**Idempotency:** read-only.

**Request schema:**


```json
{
  "type": "null"
}
```


**Response:** 200; application/octet-stream.


```json
{
  "type": "string",
  "format": "binary",
  "contentMediaType": "application/octet-stream"
}
```


Referenced types: [OpaqueId](wire-schema-reference.md#schema-opaqueid).

**Errors:** `invalid_request`, `unauthenticated`, `forbidden`, `not_found`, `source_data_required`, `source_changed`, `source_incomplete`, `plan_stale`, `unresolved_import_issues`, `report_stale`, `generation_mismatch`, `post_cutover_writes_present`, `resource_exhausted`, `storage_failure`, `outcome_unknown`.

**Requirement:** MB-TENANTCHECK. **Acceptance:** AT-TENANTCHECK.

**Additional parameter/phase constraints:**


```json
{
  "importPhases": [
    "previewed",
    "staged",
    "ready-for-cutover",
    "cutting-over",
    "active",
    "failed",
    "cancelled",
    "rolled-back",
    "recovery-exported"
  ],
  "pathParameters": [
    {
      "name": "importId",
      "schema": {
        "$ref": "urn:mirabase:wire#/$defs/OpaqueId"
      }
    },
    {
      "name": "artifactId",
      "schema": {
        "$ref": "urn:mirabase:wire#/$defs/OpaqueId"
      }
    }
  ]
}
```


## Operation application.list

**Delivery:** roadmap. Unregistered in v0.1; full operation retained for its owning roadmap group.

`GET /api/mirabase/v1/applications`

**Authority:** instance operator. **Scope:** instance.

**Preconditions:** resolved instance operator; current application and target generations/revisions; validated canonical request and resource reservation.

**Effects:** Page authorized application registry without opening all application roots.

**Idempotency:** Mutations require Idempotency-Key and canonical request hash; durable intent/result; conflicting reuse is rejected.

No request body.

**Response:** 200; application/json.


```json
{
  "$ref": "urn:mirabase:wire#/$defs/ApplicationPage"
}
```


Referenced types: [ApplicationPage](wire-schema-reference.md#schema-applicationpage).

**Errors:** `invalid_request`, `unauthenticated`, `forbidden`, `not_found`, `generation_mismatch`, `schema_mismatch`, `policy_changed`, `conflict`, `resource_exhausted`, `storage_failure`, `outcome_unknown`.

**Requirement:** MB-APPLICATION. **Acceptance:** AT-APPLICATION.

**Additional parameter/phase constraints:**


```json
{
  "pathParameters": []
}
```


## Operation application.create

**Delivery:** roadmap. Unregistered in v0.1; full operation retained for its owning roadmap group.

`POST /api/mirabase/v1/applications`

**Authority:** instance operator. **Scope:** instance.

**Preconditions:** resolved instance operator; current application and target generations/revisions; validated canonical request and resource reservation.

**Effects:** Reserve identity, stage/verify application root and publish guarded route.

**Idempotency:** Mutations require Idempotency-Key and canonical request hash; durable intent/result; conflicting reuse is rejected.

**Request schema:**


```json
{
  "$ref": "urn:mirabase:wire#/$defs/ApplicationCreate"
}
```


**Response:** 202; application/json.


```json
{
  "$ref": "urn:mirabase:wire#/$defs/Job"
}
```


Referenced types: [ApplicationCreate](wire-schema-reference.md#schema-applicationcreate), [Job](wire-schema-reference.md#schema-job).

**Errors:** `invalid_request`, `unauthenticated`, `forbidden`, `not_found`, `generation_mismatch`, `schema_mismatch`, `policy_changed`, `conflict`, `resource_exhausted`, `storage_failure`, `outcome_unknown`.

**Requirement:** MB-APPLICATION. **Acceptance:** AT-APPLICATION.

**Additional parameter/phase constraints:**


```json
{
  "pathParameters": []
}
```


## Operation application.get

**Delivery:** roadmap. Unregistered in v0.1; full operation retained for its owning roadmap group.

`GET /api/mirabase/v1/applications/{applicationId}`

**Authority:** instance operator. **Scope:** instance.

**Preconditions:** resolved instance operator; current application and target generations/revisions; validated canonical request and resource reservation.

**Effects:** Read application manifest, generation and lifecycle state without exposing secrets.

**Idempotency:** Mutations require Idempotency-Key and canonical request hash; durable intent/result; conflicting reuse is rejected.

No request body.

**Response:** 200; application/json.


```json
{
  "$ref": "urn:mirabase:wire#/$defs/Application"
}
```


Referenced types: [Application](wire-schema-reference.md#schema-application), [ApplicationId](wire-schema-reference.md#schema-applicationid).

**Errors:** `invalid_request`, `unauthenticated`, `forbidden`, `not_found`, `generation_mismatch`, `schema_mismatch`, `policy_changed`, `conflict`, `resource_exhausted`, `storage_failure`, `outcome_unknown`.

**Requirement:** MB-APPLICATION. **Acceptance:** AT-APPLICATION.

**Additional parameter/phase constraints:**


```json
{
  "pathParameters": [
    {
      "name": "applicationId",
      "schema": {
        "$ref": "urn:mirabase:wire#/$defs/ApplicationId"
      }
    }
  ]
}
```


## Operation application.update

**Delivery:** roadmap. Unregistered in v0.1; full operation retained for its owning roadmap group.

`PATCH /api/mirabase/v1/applications/{applicationId}`

**Authority:** instance operator. **Scope:** instance.

**Preconditions:** resolved instance operator; current application and target generations/revisions; validated canonical request and resource reservation.

**Effects:** Publish validated unique host aliases and application metadata at a new revision.

**Idempotency:** Mutations require Idempotency-Key and canonical request hash; durable intent/result; conflicting reuse is rejected.

**Request schema:**


```json
{
  "$ref": "urn:mirabase:wire#/$defs/ApplicationUpdate"
}
```


**Response:** 200; application/json.


```json
{
  "$ref": "urn:mirabase:wire#/$defs/Application"
}
```


Referenced types: [Application](wire-schema-reference.md#schema-application), [ApplicationId](wire-schema-reference.md#schema-applicationid), [ApplicationUpdate](wire-schema-reference.md#schema-applicationupdate).

**Errors:** `invalid_request`, `unauthenticated`, `forbidden`, `not_found`, `generation_mismatch`, `schema_mismatch`, `policy_changed`, `conflict`, `resource_exhausted`, `storage_failure`, `outcome_unknown`.

**Requirement:** MB-APPCONFIG. **Acceptance:** AT-APPCONFIG.

**Additional parameter/phase constraints:**


```json
{
  "pathParameters": [
    {
      "name": "applicationId",
      "schema": {
        "$ref": "urn:mirabase:wire#/$defs/ApplicationId"
      }
    }
  ]
}
```


## Operation application.state

**Delivery:** roadmap. Unregistered in v0.1; full operation retained for its owning roadmap group.

`POST /api/mirabase/v1/applications/{applicationId}/state`

**Authority:** instance operator. **Scope:** instance.

**Preconditions:** resolved instance operator; current application and target generations/revisions; validated canonical request and resource reservation.

**Effects:** Fence application admissions and transition maintenance/suspension/resumption safely.

**Idempotency:** Mutations require Idempotency-Key and canonical request hash; durable intent/result; conflicting reuse is rejected.

**Request schema:**


```json
{
  "$ref": "urn:mirabase:wire#/$defs/ApplicationStateChange"
}
```


**Response:** 202; application/json.


```json
{
  "$ref": "urn:mirabase:wire#/$defs/Job"
}
```


Referenced types: [ApplicationId](wire-schema-reference.md#schema-applicationid), [ApplicationStateChange](wire-schema-reference.md#schema-applicationstatechange), [Job](wire-schema-reference.md#schema-job).

**Errors:** `invalid_request`, `unauthenticated`, `forbidden`, `not_found`, `generation_mismatch`, `schema_mismatch`, `policy_changed`, `conflict`, `resource_exhausted`, `storage_failure`, `outcome_unknown`.

**Requirement:** MB-APPRECOVERY. **Acceptance:** AT-APPRECOVERY.

**Additional parameter/phase constraints:**


```json
{
  "pathParameters": [
    {
      "name": "applicationId",
      "schema": {
        "$ref": "urn:mirabase:wire#/$defs/ApplicationId"
      }
    }
  ]
}
```


## Operation application.delete

**Delivery:** roadmap. Unregistered in v0.1; full operation retained for its owning roadmap group.

`DELETE /api/mirabase/v1/applications/{applicationId}`

**Authority:** instance operator. **Scope:** instance.

**Preconditions:** resolved instance operator; current application and target generations/revisions; validated canonical request and resource reservation.

**Effects:** Revoke scoped authority, drain app leases and trash/purge only this complete application.

**Idempotency:** Mutations require Idempotency-Key and canonical request hash; durable intent/result; conflicting reuse is rejected.

**Request schema:**


```json
{
  "$ref": "urn:mirabase:wire#/$defs/ApplicationGuard"
}
```


**Response:** 202; application/json.


```json
{
  "$ref": "urn:mirabase:wire#/$defs/Job"
}
```


Referenced types: [ApplicationGuard](wire-schema-reference.md#schema-applicationguard), [ApplicationId](wire-schema-reference.md#schema-applicationid), [Job](wire-schema-reference.md#schema-job).

**Errors:** `invalid_request`, `unauthenticated`, `forbidden`, `not_found`, `generation_mismatch`, `schema_mismatch`, `policy_changed`, `conflict`, `resource_exhausted`, `storage_failure`, `outcome_unknown`.

**Requirement:** MB-APPRECOVERY. **Acceptance:** AT-APPRECOVERY.

**Additional parameter/phase constraints:**


```json
{
  "pathParameters": [
    {
      "name": "applicationId",
      "schema": {
        "$ref": "urn:mirabase:wire#/$defs/ApplicationId"
      }
    }
  ]
}
```


## Operation application.bootstrap

**Delivery:** roadmap. Unregistered in v0.1; full operation retained for its owning roadmap group.

`POST /api/mirabase/v1/applications/{applicationId}/bootstrap-token`

**Authority:** instance operator. **Scope:** instance.

**Preconditions:** resolved instance operator; current application and target generations/revisions; validated canonical request and resource reservation.

**Effects:** Issue a one-use 30-minute application setup token without granting instance authority.

**Idempotency:** Mutations require Idempotency-Key and canonical request hash; durable intent/result; conflicting reuse is rejected.

**Request schema:**


```json
{
  "$ref": "urn:mirabase:wire#/$defs/ApplicationGuard"
}
```


**Response:** 200; application/json.


```json
{
  "$ref": "urn:mirabase:wire#/$defs/BootstrapToken"
}
```


Referenced types: [ApplicationGuard](wire-schema-reference.md#schema-applicationguard), [ApplicationId](wire-schema-reference.md#schema-applicationid), [BootstrapToken](wire-schema-reference.md#schema-bootstraptoken).

**Errors:** `invalid_request`, `unauthenticated`, `forbidden`, `not_found`, `generation_mismatch`, `schema_mismatch`, `policy_changed`, `conflict`, `resource_exhausted`, `storage_failure`, `outcome_unknown`.

**Requirement:** MB-APPAUTH. **Acceptance:** AT-APPAUTH.

**Additional parameter/phase constraints:**


```json
{
  "pathParameters": [
    {
      "name": "applicationId",
      "schema": {
        "$ref": "urn:mirabase:wire#/$defs/ApplicationId"
      }
    }
  ]
}
```


## Operation application.backup

**Delivery:** roadmap. Unregistered in v0.1; full operation retained for its owning roadmap group.

`POST /api/mirabase/v1/applications/{applicationId}/backups`

**Authority:** instance operator. **Scope:** instance.

**Preconditions:** resolved instance operator; current application and target generations/revisions; validated canonical request and resource reservation.

**Effects:** Create a coherent complete application archive and declare key/remote-file dependencies.

**Idempotency:** Mutations require Idempotency-Key and canonical request hash; durable intent/result; conflicting reuse is rejected.

**Request schema:**


```json
{
  "$ref": "urn:mirabase:wire#/$defs/ApplicationBackup"
}
```


**Response:** 202; application/json.


```json
{
  "$ref": "urn:mirabase:wire#/$defs/Job"
}
```


Referenced types: [ApplicationBackup](wire-schema-reference.md#schema-applicationbackup), [ApplicationId](wire-schema-reference.md#schema-applicationid), [Job](wire-schema-reference.md#schema-job).

**Errors:** `invalid_request`, `unauthenticated`, `forbidden`, `not_found`, `generation_mismatch`, `schema_mismatch`, `policy_changed`, `conflict`, `resource_exhausted`, `storage_failure`, `outcome_unknown`.

**Requirement:** MB-APPBACKUP. **Acceptance:** AT-APPBACKUP.

**Additional parameter/phase constraints:**


```json
{
  "pathParameters": [
    {
      "name": "applicationId",
      "schema": {
        "$ref": "urn:mirabase:wire#/$defs/ApplicationId"
      }
    }
  ]
}
```


## Operation application.restore

**Delivery:** roadmap. Unregistered in v0.1; full operation retained for its owning roadmap group.

`POST /api/mirabase/v1/applications/{applicationId}/restore`

**Authority:** instance operator. **Scope:** instance.

**Preconditions:** resolved instance operator; current application and target generations/revisions; validated canonical request and resource reservation.

**Effects:** Validate and publish a new application generation without modifying other applications.

**Idempotency:** Mutations require Idempotency-Key and canonical request hash; durable intent/result; conflicting reuse is rejected.

**Request schema:**


```json
{
  "$ref": "urn:mirabase:wire#/$defs/ApplicationRestore"
}
```


**Response:** 202; application/json.


```json
{
  "$ref": "urn:mirabase:wire#/$defs/Job"
}
```


Referenced types: [ApplicationId](wire-schema-reference.md#schema-applicationid), [ApplicationRestore](wire-schema-reference.md#schema-applicationrestore), [Job](wire-schema-reference.md#schema-job).

**Errors:** `invalid_request`, `unauthenticated`, `forbidden`, `not_found`, `generation_mismatch`, `schema_mismatch`, `policy_changed`, `conflict`, `resource_exhausted`, `storage_failure`, `outcome_unknown`.

**Requirement:** MB-APPBACKUP. **Acceptance:** AT-APPBACKUP.

**Additional parameter/phase constraints:**


```json
{
  "pathParameters": [
    {
      "name": "applicationId",
      "schema": {
        "$ref": "urn:mirabase:wire#/$defs/ApplicationId"
      }
    }
  ]
}
```


## Operation importbatch.create

**Delivery:** roadmap. Unregistered in v0.1; full operation retained for its owning roadmap group.

`POST /api/mirabase/v1/application-import-batches`

**Authority:** instance operator. **Scope:** instance.

**Preconditions:** resolved instance operator; current application and target generations/revisions; validated canonical request and resource reservation.

**Effects:** Create immutable source provenance bindings and per-source guarded import sessions.

**Idempotency:** Mutations require Idempotency-Key and canonical request hash; durable intent/result; conflicting reuse is rejected.

**Request schema:**


```json
{
  "$ref": "urn:mirabase:wire#/$defs/ImportBatchCreate"
}
```


**Response:** 200; application/json.


```json
{
  "$ref": "urn:mirabase:wire#/$defs/ImportBatch"
}
```


Referenced types: [ImportBatch](wire-schema-reference.md#schema-importbatch), [ImportBatchCreate](wire-schema-reference.md#schema-importbatchcreate).

**Errors:** `invalid_request`, `unauthenticated`, `forbidden`, `not_found`, `generation_mismatch`, `schema_mismatch`, `policy_changed`, `conflict`, `resource_exhausted`, `storage_failure`, `outcome_unknown`.

**Requirement:** MB-CONSOLIDATE. **Acceptance:** AT-CONSOLIDATE.

**Additional parameter/phase constraints:**


```json
{
  "pathParameters": []
}
```


## Operation importbatch.get

**Delivery:** roadmap. Unregistered in v0.1; full operation retained for its owning roadmap group.

`GET /api/mirabase/v1/application-import-batches/{batchId}`

**Authority:** instance operator. **Scope:** instance.

**Preconditions:** resolved instance operator; current application and target generations/revisions; validated canonical request and resource reservation.

**Effects:** Read exact per-source aggregate counts and partial success without fabricated completion.

**Idempotency:** Mutations require Idempotency-Key and canonical request hash; durable intent/result; conflicting reuse is rejected.

No request body.

**Response:** 200; application/json.


```json
{
  "$ref": "urn:mirabase:wire#/$defs/ImportBatch"
}
```


Referenced types: [ImportBatch](wire-schema-reference.md#schema-importbatch), [OpaqueId](wire-schema-reference.md#schema-opaqueid).

**Errors:** `invalid_request`, `unauthenticated`, `forbidden`, `not_found`, `generation_mismatch`, `schema_mismatch`, `policy_changed`, `conflict`, `resource_exhausted`, `storage_failure`, `outcome_unknown`.

**Requirement:** MB-CONSOLIDATE. **Acceptance:** AT-CONSOLIDATE.

**Additional parameter/phase constraints:**


```json
{
  "pathParameters": [
    {
      "name": "batchId",
      "schema": {
        "$ref": "urn:mirabase:wire#/$defs/OpaqueId"
      }
    }
  ]
}
```


## Operation importbatch.sources

**Delivery:** roadmap. Unregistered in v0.1; full operation retained for its owning roadmap group.

`POST /api/mirabase/v1/application-import-batches/{batchId}/sources`

**Authority:** instance operator. **Scope:** instance.

**Preconditions:** resolved instance operator; current application and target generations/revisions; validated canonical request and resource reservation.

**Effects:** Append bounded source pages before executing without replacing existing applications.

**Idempotency:** Mutations require Idempotency-Key and canonical request hash; durable intent/result; conflicting reuse is rejected.

**Request schema:**


```json
{
  "type": "object",
  "properties": {
    "expectedRevision": {
      "$ref": "urn:mirabase:wire#/$defs/Revision"
    },
    "sources": {
      "type": "array",
      "items": {
        "$ref": "urn:mirabase:wire#/$defs/ImportBatchSourceInput"
      },
      "maxItems": 1000
    }
  },
  "required": [
    "expectedRevision",
    "sources"
  ],
  "additionalProperties": false
}
```


**Response:** 200; application/json.


```json
{
  "$ref": "urn:mirabase:wire#/$defs/ImportBatch"
}
```


Referenced types: [ImportBatch](wire-schema-reference.md#schema-importbatch), [ImportBatchSourceInput](wire-schema-reference.md#schema-importbatchsourceinput), [OpaqueId](wire-schema-reference.md#schema-opaqueid), [Revision](wire-schema-reference.md#schema-revision).

**Errors:** `invalid_request`, `unauthenticated`, `forbidden`, `not_found`, `generation_mismatch`, `schema_mismatch`, `policy_changed`, `conflict`, `resource_exhausted`, `storage_failure`, `outcome_unknown`.

**Requirement:** MB-APPEND. **Acceptance:** AT-APPEND.

**Additional parameter/phase constraints:**


```json
{
  "pathParameters": [
    {
      "name": "batchId",
      "schema": {
        "$ref": "urn:mirabase:wire#/$defs/OpaqueId"
      }
    }
  ]
}
```


## Operation importbatch.run

**Delivery:** roadmap. Unregistered in v0.1; full operation retained for its owning roadmap group.

`POST /api/mirabase/v1/application-import-batches/{batchId}/run`

**Authority:** instance operator. **Scope:** instance.

**Preconditions:** resolved instance operator; current application and target generations/revisions; validated canonical request and resource reservation.

**Effects:** Run reviewed source plans with independent verified publication and exact partial results.

**Idempotency:** Mutations require Idempotency-Key and canonical request hash; durable intent/result; conflicting reuse is rejected.

**Request schema:**


```json
{
  "$ref": "urn:mirabase:wire#/$defs/RevisionGuard"
}
```


**Response:** 202; application/json.


```json
{
  "$ref": "urn:mirabase:wire#/$defs/Job"
}
```


Referenced types: [Job](wire-schema-reference.md#schema-job), [OpaqueId](wire-schema-reference.md#schema-opaqueid), [RevisionGuard](wire-schema-reference.md#schema-revisionguard).

**Errors:** `invalid_request`, `unauthenticated`, `forbidden`, `not_found`, `generation_mismatch`, `schema_mismatch`, `policy_changed`, `conflict`, `resource_exhausted`, `storage_failure`, `outcome_unknown`.

**Requirement:** MB-CONSOLIDATE. **Acceptance:** AT-CONSOLIDATE.

**Additional parameter/phase constraints:**


```json
{
  "pathParameters": [
    {
      "name": "batchId",
      "schema": {
        "$ref": "urn:mirabase:wire#/$defs/OpaqueId"
      }
    }
  ]
}
```


## Operation importbatch.cancel

**Delivery:** roadmap. Unregistered in v0.1; full operation retained for its owning roadmap group.

`POST /api/mirabase/v1/application-import-batches/{batchId}/cancel`

**Authority:** instance operator. **Scope:** instance.

**Preconditions:** resolved instance operator; current application and target generations/revisions; validated canonical request and resource reservation.

**Effects:** Cancel only unpublished source work and retain all active application outcomes.

**Idempotency:** Mutations require Idempotency-Key and canonical request hash; durable intent/result; conflicting reuse is rejected.

**Request schema:**


```json
{
  "$ref": "urn:mirabase:wire#/$defs/RevisionGuard"
}
```


**Response:** 200; application/json.


```json
{
  "$ref": "urn:mirabase:wire#/$defs/ImportBatch"
}
```


Referenced types: [ImportBatch](wire-schema-reference.md#schema-importbatch), [OpaqueId](wire-schema-reference.md#schema-opaqueid), [RevisionGuard](wire-schema-reference.md#schema-revisionguard).

**Errors:** `invalid_request`, `unauthenticated`, `forbidden`, `not_found`, `generation_mismatch`, `schema_mismatch`, `policy_changed`, `conflict`, `resource_exhausted`, `storage_failure`, `outcome_unknown`.

**Requirement:** MB-APPEND. **Acceptance:** AT-APPEND.

**Additional parameter/phase constraints:**


```json
{
  "pathParameters": [
    {
      "name": "batchId",
      "schema": {
        "$ref": "urn:mirabase:wire#/$defs/OpaqueId"
      }
    }
  ]
}
```


## Operation setup.get

**Delivery:** slice. Default application only; shared or empty user/tenant/agent collection policy.

`GET /api/mirabase/v1/setup`

**Authority:** application superuser. **Scope:** resolved-application.

**Preconditions:** immutable authorized application/database context; current application and target generations/revisions; validated canonical request and resource reservation.

**Effects:** Read current application storage policy and caller-owned setup draft.

**Idempotency:** Mutations require Idempotency-Key and canonical request hash; durable intent/result; conflicting reuse is rejected.

No request body.

**Response:** 200; application/json.


```json
{
  "$ref": "urn:mirabase:wire#/$defs/SetupState"
}
```


Referenced types: [SetupState](wire-schema-reference.md#schema-setupstate).

**Errors:** `invalid_request`, `unauthenticated`, `forbidden`, `not_found`, `generation_mismatch`, `schema_mismatch`, `policy_changed`, `conflict`, `resource_exhausted`, `storage_failure`, `outcome_unknown`.

**Requirement:** MB-ONBOARD. **Acceptance:** AT-ONBOARD.

**Additional parameter/phase constraints:**


```json
{
  "pathParameters": []
}
```


## Operation setup.plan

**Delivery:** slice. No populated collection relocation, invitations or independent-app configuration.

`PUT /api/mirabase/v1/setup/plan`

**Authority:** application superuser. **Scope:** resolved-application.

**Preconditions:** immutable authorized application/database context; current application and target generations/revisions; validated canonical request and resource reservation.

**Effects:** Validate schema/ownership/membership and produce immutable preview without live layout effects.

**Idempotency:** Mutations require Idempotency-Key and canonical request hash; durable intent/result; conflicting reuse is rejected.

**Request schema:**


```json
{
  "$ref": "urn:mirabase:wire#/$defs/SetupPlanRequest"
}
```


**Response:** 200; application/json.


```json
{
  "$ref": "urn:mirabase:wire#/$defs/SetupPlan"
}
```


Referenced types: [SetupPlan](wire-schema-reference.md#schema-setupplan), [SetupPlanRequest](wire-schema-reference.md#schema-setupplanrequest).

**Errors:** `invalid_request`, `unauthenticated`, `forbidden`, `not_found`, `generation_mismatch`, `schema_mismatch`, `policy_changed`, `conflict`, `resource_exhausted`, `storage_failure`, `outcome_unknown`.

**Requirement:** MB-ONBOARD. **Acceptance:** AT-ONBOARD.

**Additional parameter/phase constraints:**


```json
{
  "pathParameters": []
}
```


## Operation setup.apply

**Delivery:** slice. Validated empty-layout activation only; real memberships/provisioning required.

`POST /api/mirabase/v1/setup/plan/apply`

**Authority:** application superuser. **Scope:** resolved-application.

**Preconditions:** immutable authorized application/database context; current application and target generations/revisions; validated canonical request and resource reservation.

**Effects:** Apply exact reviewed setup and publish policy through guarded durable job.

**Idempotency:** Mutations require Idempotency-Key and canonical request hash; durable intent/result; conflicting reuse is rejected.

**Request schema:**


```json
{
  "$ref": "urn:mirabase:wire#/$defs/SetupApply"
}
```


**Response:** 202; application/json.


```json
{
  "$ref": "urn:mirabase:wire#/$defs/Job"
}
```


Referenced types: [Job](wire-schema-reference.md#schema-job), [SetupApply](wire-schema-reference.md#schema-setupapply).

**Errors:** `invalid_request`, `unauthenticated`, `forbidden`, `not_found`, `generation_mismatch`, `schema_mismatch`, `policy_changed`, `conflict`, `resource_exhausted`, `storage_failure`, `outcome_unknown`.

**Requirement:** MB-ONBOARD. **Acceptance:** AT-ONBOARD.

**Additional parameter/phase constraints:**


```json
{
  "pathParameters": []
}
```


## Operation membership.invite

**Delivery:** roadmap. Unregistered in v0.1; full operation retained for its owning roadmap group.

`POST /api/mirabase/v1/membership-invitations`

**Authority:** current tenant invite rule. **Scope:** resolved-application.

**Preconditions:** immutable authorized application/database context; current application and target generations/revisions; validated canonical request and resource reservation.

**Effects:** Persist hashed scoped invite and enqueue mail without arbitrary role or owner elevation.

**Idempotency:** Mutations require Idempotency-Key and canonical request hash; durable intent/result; conflicting reuse is rejected.

**Request schema:**


```json
{
  "$ref": "urn:mirabase:wire#/$defs/InvitationCreate"
}
```


**Response:** 200; application/json.


```json
{
  "$ref": "urn:mirabase:wire#/$defs/Invitation"
}
```


Referenced types: [Invitation](wire-schema-reference.md#schema-invitation), [InvitationCreate](wire-schema-reference.md#schema-invitationcreate).

**Errors:** `invalid_request`, `unauthenticated`, `forbidden`, `not_found`, `generation_mismatch`, `schema_mismatch`, `policy_changed`, `conflict`, `resource_exhausted`, `storage_failure`, `outcome_unknown`.

**Requirement:** MB-MEMBERSHIP. **Acceptance:** AT-MEMBERSHIP.

**Additional parameter/phase constraints:**


```json
{
  "pathParameters": []
}
```


## Operation membership.accept

**Delivery:** roadmap. Unregistered in v0.1; full operation retained for its owning roadmap group.

`POST /api/mirabase/v1/membership-invitations/accept`

**Authority:** authenticated intended account. **Scope:** resolved-application.

**Preconditions:** immutable authorized application/database context; current application and target generations/revisions; validated canonical request and resource reservation.

**Effects:** Validate account/tenant/current policy and atomically consume invite with one membership.

**Idempotency:** Mutations require Idempotency-Key and canonical request hash; durable intent/result; conflicting reuse is rejected.

**Request schema:**


```json
{
  "$ref": "urn:mirabase:wire#/$defs/InvitationAccept"
}
```


**Response:** 200; application/json.


```json
{
  "$ref": "urn:mirabase:wire#/$defs/Invitation"
}
```


Referenced types: [Invitation](wire-schema-reference.md#schema-invitation), [InvitationAccept](wire-schema-reference.md#schema-invitationaccept).

**Errors:** `invalid_request`, `unauthenticated`, `forbidden`, `not_found`, `generation_mismatch`, `schema_mismatch`, `policy_changed`, `conflict`, `resource_exhausted`, `storage_failure`, `outcome_unknown`.

**Requirement:** MB-MEMBERSHIP. **Acceptance:** AT-MEMBERSHIP.

**Additional parameter/phase constraints:**


```json
{
  "pathParameters": []
}
```


## Operation membership.revoke

**Delivery:** roadmap. Unregistered in v0.1; full operation retained for its owning roadmap group.

`DELETE /api/mirabase/v1/membership-invitations/{inviteId}`

**Authority:** current tenant invite rule. **Scope:** resolved-application.

**Preconditions:** immutable authorized application/database context; current application and target generations/revisions; validated canonical request and resource reservation.

**Effects:** Revoke only the selected authorized tenant invitation before use.

**Idempotency:** Mutations require Idempotency-Key and canonical request hash; durable intent/result; conflicting reuse is rejected.

**Request schema:**


```json
{
  "$ref": "urn:mirabase:wire#/$defs/RevisionGuard"
}
```


**Response:** 200; application/json.


```json
{
  "$ref": "urn:mirabase:wire#/$defs/Invitation"
}
```


Referenced types: [Invitation](wire-schema-reference.md#schema-invitation), [OpaqueId](wire-schema-reference.md#schema-opaqueid), [RevisionGuard](wire-schema-reference.md#schema-revisionguard).

**Errors:** `invalid_request`, `unauthenticated`, `forbidden`, `not_found`, `generation_mismatch`, `schema_mismatch`, `policy_changed`, `conflict`, `resource_exhausted`, `storage_failure`, `outcome_unknown`.

**Requirement:** MB-MEMBERSHIP. **Acceptance:** AT-MEMBERSHIP.

**Additional parameter/phase constraints:**


```json
{
  "pathParameters": [
    {
      "name": "inviteId",
      "schema": {
        "$ref": "urn:mirabase:wire#/$defs/OpaqueId"
      }
    }
  ]
}
```


## Operation owner.transfer

**Delivery:** v0.1. Applicable exact contract within one default application and released features.

`POST /api/mirabase/v1/databases/{id}/owner-transfer`

**Authority:** application superuser. **Scope:** resolved-application.

**Preconditions:** immutable authorized application/database context; current application and target generations/revisions; validated canonical request and resource reservation.

**Effects:** Fence old policy, revoke/reconcile grants and publish reviewed owner after durable transfer intent.

**Idempotency:** Mutations require Idempotency-Key and canonical request hash; durable intent/result; conflicting reuse is rejected.

**Request schema:**


```json
{
  "$ref": "urn:mirabase:wire#/$defs/OwnerTransfer"
}
```


**Response:** 202; application/json.


```json
{
  "$ref": "urn:mirabase:wire#/$defs/Job"
}
```


Referenced types: [Job](wire-schema-reference.md#schema-job), [OpaqueId](wire-schema-reference.md#schema-opaqueid), [OwnerTransfer](wire-schema-reference.md#schema-ownertransfer).

**Errors:** `invalid_request`, `unauthenticated`, `forbidden`, `not_found`, `generation_mismatch`, `schema_mismatch`, `policy_changed`, `conflict`, `resource_exhausted`, `storage_failure`, `outcome_unknown`.

**Requirement:** MB-OWNERTRANSFER. **Acceptance:** AT-OWNERTRANSFER.

**Additional parameter/phase constraints:**


```json
{
  "pathParameters": [
    {
      "name": "id",
      "schema": {
        "$ref": "urn:mirabase:wire#/$defs/OpaqueId"
      }
    }
  ]
}
```


## Operation reference.define

**Delivery:** roadmap. Unregistered in v0.1; full operation retained for its owning roadmap group.

`PUT /api/mirabase/v1/reference-copies/{policyId}`

**Authority:** application superuser. **Scope:** resolved-application.

**Preconditions:** immutable authorized application/database context; current application and target generations/revisions; validated canonical request and resource reservation.

**Effects:** Publish validated nonsecret projection policy and schedule generation-bound refresh.

**Idempotency:** Mutations require Idempotency-Key and canonical request hash; durable intent/result; conflicting reuse is rejected.

**Request schema:**


```json
{
  "$ref": "urn:mirabase:wire#/$defs/ReferencePolicy"
}
```


**Response:** 200; application/json.


```json
{
  "$ref": "urn:mirabase:wire#/$defs/ReferencePolicy"
}
```


Referenced types: [OpaqueId](wire-schema-reference.md#schema-opaqueid), [ReferencePolicy](wire-schema-reference.md#schema-referencepolicy).

**Errors:** `invalid_request`, `unauthenticated`, `forbidden`, `not_found`, `generation_mismatch`, `schema_mismatch`, `policy_changed`, `conflict`, `resource_exhausted`, `storage_failure`, `outcome_unknown`.

**Requirement:** MB-REFERENCE. **Acceptance:** AT-REFERENCE.

**Additional parameter/phase constraints:**


```json
{
  "pathParameters": [
    {
      "name": "policyId",
      "schema": {
        "$ref": "urn:mirabase:wire#/$defs/OpaqueId"
      }
    }
  ]
}
```


## Operation reference.status

**Delivery:** roadmap. Unregistered in v0.1; full operation retained for its owning roadmap group.

`GET /api/mirabase/v1/reference-copies/{policyId}`

**Authority:** application superuser. **Scope:** resolved-application.

**Preconditions:** immutable authorized application/database context; current application and target generations/revisions; validated canonical request and resource reservation.

**Effects:** Read durable per-target applied sequence, health and refresh failure counts.

**Idempotency:** Mutations require Idempotency-Key and canonical request hash; durable intent/result; conflicting reuse is rejected.

No request body.

**Response:** 200; application/json.


```json
{
  "$ref": "urn:mirabase:wire#/$defs/ReferenceStatus"
}
```


Referenced types: [OpaqueId](wire-schema-reference.md#schema-opaqueid), [ReferenceStatus](wire-schema-reference.md#schema-referencestatus).

**Errors:** `invalid_request`, `unauthenticated`, `forbidden`, `not_found`, `generation_mismatch`, `schema_mismatch`, `policy_changed`, `conflict`, `resource_exhausted`, `storage_failure`, `outcome_unknown`.

**Requirement:** MB-REFERENCE. **Acceptance:** AT-REFERENCE.

**Additional parameter/phase constraints:**


```json
{
  "pathParameters": [
    {
      "name": "policyId",
      "schema": {
        "$ref": "urn:mirabase:wire#/$defs/OpaqueId"
      }
    }
  ]
}
```


## Operation reference.refresh

**Delivery:** roadmap. Unregistered in v0.1; full operation retained for its owning roadmap group.

`POST /api/mirabase/v1/reference-copies/{policyId}/refresh`

**Authority:** application superuser. **Scope:** resolved-application.

**Preconditions:** immutable authorized application/database context; current application and target generations/revisions; validated canonical request and resource reservation.

**Effects:** Refresh real source snapshots and apply idempotent updates/deletes with target checkpoints.

**Idempotency:** Mutations require Idempotency-Key and canonical request hash; durable intent/result; conflicting reuse is rejected.

**Request schema:**


```json
{
  "$ref": "urn:mirabase:wire#/$defs/RevisionGuard"
}
```


**Response:** 202; application/json.


```json
{
  "$ref": "urn:mirabase:wire#/$defs/Job"
}
```


Referenced types: [Job](wire-schema-reference.md#schema-job), [OpaqueId](wire-schema-reference.md#schema-opaqueid), [RevisionGuard](wire-schema-reference.md#schema-revisionguard).

**Errors:** `invalid_request`, `unauthenticated`, `forbidden`, `not_found`, `generation_mismatch`, `schema_mismatch`, `policy_changed`, `conflict`, `resource_exhausted`, `storage_failure`, `outcome_unknown`.

**Requirement:** MB-REFERENCE. **Acceptance:** AT-REFERENCE.

**Additional parameter/phase constraints:**


```json
{
  "pathParameters": [
    {
      "name": "policyId",
      "schema": {
        "$ref": "urn:mirabase:wire#/$defs/OpaqueId"
      }
    }
  ]
}
```


## Operation layout.apply

**Delivery:** roadmap. Unregistered in v0.1; full operation retained for its owning roadmap group.

`POST /api/mirabase/v1/layout-migrations`

**Authority:** application superuser. **Scope:** resolved-application.

**Preconditions:** immutable authorized application/database context; current application and target generations/revisions; validated canonical request and resource reservation.

**Effects:** Stage verified layout conversion and publish through the recoverable application journal.

**Idempotency:** Mutations require Idempotency-Key and canonical request hash; durable intent/result; conflicting reuse is rejected.

**Request schema:**


```json
{
  "$ref": "urn:mirabase:wire#/$defs/LayoutChange"
}
```


**Response:** 202; application/json.


```json
{
  "$ref": "urn:mirabase:wire#/$defs/Job"
}
```


Referenced types: [Job](wire-schema-reference.md#schema-job), [LayoutChange](wire-schema-reference.md#schema-layoutchange).

**Errors:** `invalid_request`, `unauthenticated`, `forbidden`, `not_found`, `generation_mismatch`, `schema_mismatch`, `policy_changed`, `conflict`, `resource_exhausted`, `storage_failure`, `outcome_unknown`.

**Requirement:** MB-LAYOUT. **Acceptance:** AT-LAYOUT.

**Additional parameter/phase constraints:**


```json
{
  "pathParameters": []
}
```


## Operation liveplan.list

**Delivery:** roadmap. Unregistered in v0.1; full operation retained for its owning roadmap group.

`GET /api/mirabase/v1/live-query-plans`

**Authority:** application superuser. **Scope:** resolved-application.

**Preconditions:** immutable authorized application/database context; current application and target generations/revisions; validated canonical request and resource reservation.

**Effects:** Page named query definitions without instantiating subscriptions or exposing privileged SQL to users.

**Idempotency:** Mutations require Idempotency-Key and canonical request hash; durable intent/result; conflicting reuse is rejected.

No request body.

**Response:** 200; application/json.


```json
{
  "$ref": "urn:mirabase:wire#/$defs/LivePlanPage"
}
```


Referenced types: [LivePlanPage](wire-schema-reference.md#schema-liveplanpage).

**Errors:** `invalid_request`, `unauthenticated`, `forbidden`, `not_found`, `generation_mismatch`, `schema_mismatch`, `policy_changed`, `conflict`, `resource_exhausted`, `storage_failure`, `outcome_unknown`.

**Requirement:** MB-LIVEPLAN. **Acceptance:** AT-LIVEPLAN.

**Additional parameter/phase constraints:**


```json
{
  "pathParameters": []
}
```


## Operation liveplan.create

**Delivery:** roadmap. Unregistered in v0.1; full operation retained for its owning roadmap group.

`POST /api/mirabase/v1/live-query-plans`

**Authority:** application superuser. **Scope:** resolved-application.

**Preconditions:** immutable authorized application/database context; current application and target generations/revisions; validated canonical request and resource reservation.

**Effects:** Prepare and validate immutable named read-only query revision with policy injection.

**Idempotency:** Mutations require Idempotency-Key and canonical request hash; durable intent/result; conflicting reuse is rejected.

**Request schema:**


```json
{
  "$ref": "urn:mirabase:wire#/$defs/LivePlan"
}
```


**Response:** 200; application/json.


```json
{
  "$ref": "urn:mirabase:wire#/$defs/LivePlan"
}
```


Referenced types: [LivePlan](wire-schema-reference.md#schema-liveplan).

**Errors:** `invalid_request`, `unauthenticated`, `forbidden`, `not_found`, `generation_mismatch`, `schema_mismatch`, `policy_changed`, `conflict`, `resource_exhausted`, `storage_failure`, `outcome_unknown`.

**Requirement:** MB-LIVEPLAN. **Acceptance:** AT-LIVEPLAN.

**Additional parameter/phase constraints:**


```json
{
  "pathParameters": []
}
```


## Operation liveplan.get

**Delivery:** roadmap. Unregistered in v0.1; full operation retained for its owning roadmap group.

`GET /api/mirabase/v1/live-query-plans/{planId}/revisions/{revision}`

**Authority:** application superuser. **Scope:** resolved-application.

**Preconditions:** immutable authorized application/database context; current application and target generations/revisions; validated canonical request and resource reservation.

**Effects:** Read exact application-local parameterized query revision and publication state.

**Idempotency:** Mutations require Idempotency-Key and canonical request hash; durable intent/result; conflicting reuse is rejected.

No request body.

**Response:** 200; application/json.


```json
{
  "$ref": "urn:mirabase:wire#/$defs/LivePlan"
}
```


Referenced types: [LivePlan](wire-schema-reference.md#schema-liveplan), [OpaqueId](wire-schema-reference.md#schema-opaqueid).

**Errors:** `invalid_request`, `unauthenticated`, `forbidden`, `not_found`, `generation_mismatch`, `schema_mismatch`, `policy_changed`, `conflict`, `resource_exhausted`, `storage_failure`, `outcome_unknown`.

**Requirement:** MB-LIVEPLAN. **Acceptance:** AT-LIVEPLAN.

**Additional parameter/phase constraints:**


```json
{
  "pathParameters": [
    {
      "name": "planId",
      "schema": {
        "$ref": "urn:mirabase:wire#/$defs/OpaqueId"
      }
    },
    {
      "name": "revision",
      "schema": {
        "$ref": "urn:mirabase:wire#/$defs/OpaqueId"
      }
    }
  ]
}
```


## Operation liveplan.publish

**Delivery:** roadmap. Unregistered in v0.1; full operation retained for its owning roadmap group.

`POST /api/mirabase/v1/live-query-plans/{planId}/publish`

**Authority:** application superuser. **Scope:** resolved-application.

**Preconditions:** immutable authorized application/database context; current application and target generations/revisions; validated canonical request and resource reservation.

**Effects:** Publish reviewed query revision without changing SQL of existing instances.

**Idempotency:** Mutations require Idempotency-Key and canonical request hash; durable intent/result; conflicting reuse is rejected.

**Request schema:**


```json
{
  "$ref": "urn:mirabase:wire#/$defs/RevisionGuard"
}
```


**Response:** 200; application/json.


```json
{
  "$ref": "urn:mirabase:wire#/$defs/LivePlan"
}
```


Referenced types: [LivePlan](wire-schema-reference.md#schema-liveplan), [OpaqueId](wire-schema-reference.md#schema-opaqueid), [RevisionGuard](wire-schema-reference.md#schema-revisionguard).

**Errors:** `invalid_request`, `unauthenticated`, `forbidden`, `not_found`, `generation_mismatch`, `schema_mismatch`, `policy_changed`, `conflict`, `resource_exhausted`, `storage_failure`, `outcome_unknown`.

**Requirement:** MB-LIVEPLAN. **Acceptance:** AT-LIVEPLAN.

**Additional parameter/phase constraints:**


```json
{
  "pathParameters": [
    {
      "name": "planId",
      "schema": {
        "$ref": "urn:mirabase:wire#/$defs/OpaqueId"
      }
    }
  ]
}
```


## Operation liveplan.retire

**Delivery:** roadmap. Unregistered in v0.1; full operation retained for its owning roadmap group.

`POST /api/mirabase/v1/live-query-plans/{planId}/retire`

**Authority:** application superuser. **Scope:** resolved-application.

**Preconditions:** immutable authorized application/database context; current application and target generations/revisions; validated canonical request and resource reservation.

**Effects:** Retire a query revision and reset/terminate its active authorized subscriptions.

**Idempotency:** Mutations require Idempotency-Key and canonical request hash; durable intent/result; conflicting reuse is rejected.

**Request schema:**


```json
{
  "$ref": "urn:mirabase:wire#/$defs/RevisionGuard"
}
```


**Response:** 200; application/json.


```json
{
  "$ref": "urn:mirabase:wire#/$defs/LivePlan"
}
```


Referenced types: [LivePlan](wire-schema-reference.md#schema-liveplan), [OpaqueId](wire-schema-reference.md#schema-opaqueid), [RevisionGuard](wire-schema-reference.md#schema-revisionguard).

**Errors:** `invalid_request`, `unauthenticated`, `forbidden`, `not_found`, `generation_mismatch`, `schema_mismatch`, `policy_changed`, `conflict`, `resource_exhausted`, `storage_failure`, `outcome_unknown`.

**Requirement:** MB-LIVEPLAN. **Acceptance:** AT-LIVEPLAN.

**Additional parameter/phase constraints:**


```json
{
  "pathParameters": [
    {
      "name": "planId",
      "schema": {
        "$ref": "urn:mirabase:wire#/$defs/OpaqueId"
      }
    }
  ]
}
```


## Operation rollout.pause

**Delivery:** roadmap. Unregistered in v0.1; full operation retained for its owning roadmap group.

`POST /api/mirabase/v1/rollouts/{rolloutId}/pause`

**Authority:** application superuser. **Scope:** resolved-application.

**Preconditions:** immutable authorized application/database context; current application and target generations/revisions; validated canonical request and resource reservation.

**Effects:** Persist pause intent and reconcile actual selected target generations and committed steps.

**Idempotency:** Mutations require Idempotency-Key and canonical request hash; durable intent/result; conflicting reuse is rejected.

**Request schema:**


```json
{
  "$ref": "urn:mirabase:wire#/$defs/RolloutControl"
}
```


**Response:** 202; application/json.


```json
{
  "$ref": "urn:mirabase:wire#/$defs/Job"
}
```


Referenced types: [Job](wire-schema-reference.md#schema-job), [OpaqueId](wire-schema-reference.md#schema-opaqueid), [RolloutControl](wire-schema-reference.md#schema-rolloutcontrol).

**Errors:** `invalid_request`, `unauthenticated`, `forbidden`, `not_found`, `generation_mismatch`, `schema_mismatch`, `policy_changed`, `conflict`, `resource_exhausted`, `storage_failure`, `outcome_unknown`.

**Requirement:** MB-ROLLOUTCTL. **Acceptance:** AT-ROLLOUTCTL.

**Additional parameter/phase constraints:**


```json
{
  "pathParameters": [
    {
      "name": "rolloutId",
      "schema": {
        "$ref": "urn:mirabase:wire#/$defs/OpaqueId"
      }
    }
  ]
}
```


## Operation rollout.resume

**Delivery:** roadmap. Unregistered in v0.1; full operation retained for its owning roadmap group.

`POST /api/mirabase/v1/rollouts/{rolloutId}/resume`

**Authority:** application superuser. **Scope:** resolved-application.

**Preconditions:** immutable authorized application/database context; current application and target generations/revisions; validated canonical request and resource reservation.

**Effects:** Persist resume intent and reconcile actual selected target generations and committed steps.

**Idempotency:** Mutations require Idempotency-Key and canonical request hash; durable intent/result; conflicting reuse is rejected.

**Request schema:**


```json
{
  "$ref": "urn:mirabase:wire#/$defs/RolloutControl"
}
```


**Response:** 202; application/json.


```json
{
  "$ref": "urn:mirabase:wire#/$defs/Job"
}
```


Referenced types: [Job](wire-schema-reference.md#schema-job), [OpaqueId](wire-schema-reference.md#schema-opaqueid), [RolloutControl](wire-schema-reference.md#schema-rolloutcontrol).

**Errors:** `invalid_request`, `unauthenticated`, `forbidden`, `not_found`, `generation_mismatch`, `schema_mismatch`, `policy_changed`, `conflict`, `resource_exhausted`, `storage_failure`, `outcome_unknown`.

**Requirement:** MB-ROLLOUTCTL. **Acceptance:** AT-ROLLOUTCTL.

**Additional parameter/phase constraints:**


```json
{
  "pathParameters": [
    {
      "name": "rolloutId",
      "schema": {
        "$ref": "urn:mirabase:wire#/$defs/OpaqueId"
      }
    }
  ]
}
```


## Operation rollout.retry

**Delivery:** roadmap. Unregistered in v0.1; full operation retained for its owning roadmap group.

`POST /api/mirabase/v1/rollouts/{rolloutId}/retry`

**Authority:** application superuser. **Scope:** resolved-application.

**Preconditions:** immutable authorized application/database context; current application and target generations/revisions; validated canonical request and resource reservation.

**Effects:** Persist retry intent and reconcile actual selected target generations and committed steps.

**Idempotency:** Mutations require Idempotency-Key and canonical request hash; durable intent/result; conflicting reuse is rejected.

**Request schema:**


```json
{
  "$ref": "urn:mirabase:wire#/$defs/RolloutControl"
}
```


**Response:** 202; application/json.


```json
{
  "$ref": "urn:mirabase:wire#/$defs/Job"
}
```


Referenced types: [Job](wire-schema-reference.md#schema-job), [OpaqueId](wire-schema-reference.md#schema-opaqueid), [RolloutControl](wire-schema-reference.md#schema-rolloutcontrol).

**Errors:** `invalid_request`, `unauthenticated`, `forbidden`, `not_found`, `generation_mismatch`, `schema_mismatch`, `policy_changed`, `conflict`, `resource_exhausted`, `storage_failure`, `outcome_unknown`.

**Requirement:** MB-ROLLOUTCTL. **Acceptance:** AT-ROLLOUTCTL.

**Additional parameter/phase constraints:**


```json
{
  "pathParameters": [
    {
      "name": "rolloutId",
      "schema": {
        "$ref": "urn:mirabase:wire#/$defs/OpaqueId"
      }
    }
  ]
}
```


## Operation rollout.targets

**Delivery:** roadmap. Unregistered in v0.1; full operation retained for its owning roadmap group.

`GET /api/mirabase/v1/rollouts/{rolloutId}/targets`

**Authority:** application superuser. **Scope:** resolved-application.

**Preconditions:** immutable authorized application/database context; current application and target generations/revisions; validated canonical request and resource reservation.

**Effects:** Page fixed target snapshot and exact per-target state without unbounded result JSON.

**Idempotency:** Mutations require Idempotency-Key and canonical request hash; durable intent/result; conflicting reuse is rejected.

No request body.

**Response:** 200; application/json.


```json
{
  "$ref": "urn:mirabase:wire#/$defs/RolloutTargetPage"
}
```


Referenced types: [OpaqueId](wire-schema-reference.md#schema-opaqueid), [RolloutTargetPage](wire-schema-reference.md#schema-rollouttargetpage).

**Errors:** `invalid_request`, `unauthenticated`, `forbidden`, `not_found`, `generation_mismatch`, `schema_mismatch`, `policy_changed`, `conflict`, `resource_exhausted`, `storage_failure`, `outcome_unknown`.

**Requirement:** MB-ROLLOUTCTL. **Acceptance:** AT-ROLLOUTCTL.

**Additional parameter/phase constraints:**


```json
{
  "pathParameters": [
    {
      "name": "rolloutId",
      "schema": {
        "$ref": "urn:mirabase:wire#/$defs/OpaqueId"
      }
    }
  ]
}
```


## Operation relationworkflow.define

**Delivery:** roadmap. Unregistered in v0.1; full operation retained for its owning roadmap group.

`PUT /api/mirabase/v1/relation-workflows/{workflowId}`

**Authority:** application superuser. **Scope:** resolved-application.

**Preconditions:** immutable authorized application/database context; current application and target generations/revisions; validated canonical request and resource reservation.

**Effects:** Validate explicit same-application qualified cascade steps and guarded service principal.

**Idempotency:** Mutations require Idempotency-Key and canonical request hash; durable intent/result; conflicting reuse is rejected.

**Request schema:**


```json
{
  "$ref": "urn:mirabase:wire#/$defs/RelationWorkflow"
}
```


**Response:** 200; application/json.


```json
{
  "$ref": "urn:mirabase:wire#/$defs/RelationWorkflow"
}
```


Referenced types: [OpaqueId](wire-schema-reference.md#schema-opaqueid), [RelationWorkflow](wire-schema-reference.md#schema-relationworkflow).

**Errors:** `invalid_request`, `unauthenticated`, `forbidden`, `not_found`, `generation_mismatch`, `schema_mismatch`, `policy_changed`, `conflict`, `resource_exhausted`, `storage_failure`, `outcome_unknown`.

**Requirement:** MB-RELWORKFLOW. **Acceptance:** AT-RELWORKFLOW.

**Additional parameter/phase constraints:**


```json
{
  "pathParameters": [
    {
      "name": "workflowId",
      "schema": {
        "$ref": "urn:mirabase:wire#/$defs/OpaqueId"
      }
    }
  ]
}
```


## Operation relationworkflow.run

**Delivery:** roadmap. Unregistered in v0.1; full operation retained for its owning roadmap group.

`POST /api/mirabase/v1/relation-workflows/{workflowId}/run`

**Authority:** application superuser. **Scope:** resolved-application.

**Preconditions:** immutable authorized application/database context; current application and target generations/revisions; validated canonical request and resource reservation.

**Effects:** Execute deterministic per-context cascade steps with visible partial results and version guards.

**Idempotency:** Mutations require Idempotency-Key and canonical request hash; durable intent/result; conflicting reuse is rejected.

**Request schema:**


```json
{
  "$ref": "urn:mirabase:wire#/$defs/RevisionGuard"
}
```


**Response:** 202; application/json.


```json
{
  "$ref": "urn:mirabase:wire#/$defs/Job"
}
```


Referenced types: [Job](wire-schema-reference.md#schema-job), [OpaqueId](wire-schema-reference.md#schema-opaqueid), [RevisionGuard](wire-schema-reference.md#schema-revisionguard).

**Errors:** `invalid_request`, `unauthenticated`, `forbidden`, `not_found`, `generation_mismatch`, `schema_mismatch`, `policy_changed`, `conflict`, `resource_exhausted`, `storage_failure`, `outcome_unknown`.

**Requirement:** MB-RELWORKFLOW. **Acceptance:** AT-RELWORKFLOW.

**Additional parameter/phase constraints:**


```json
{
  "pathParameters": [
    {
      "name": "workflowId",
      "schema": {
        "$ref": "urn:mirabase:wire#/$defs/OpaqueId"
      }
    }
  ]
}
```


## Operation report.define

**Delivery:** roadmap. Unregistered in v0.1; full operation retained for its owning roadmap group.

`PUT /api/mirabase/v1/reports/{reportId}`

**Authority:** application superuser. **Scope:** resolved-application.

**Preconditions:** immutable authorized application/database context; current application and target generations/revisions; validated canonical request and resource reservation.

**Effects:** Publish approved projected CDC aggregate and current output access rule.

**Idempotency:** Mutations require Idempotency-Key and canonical request hash; durable intent/result; conflicting reuse is rejected.

**Request schema:**


```json
{
  "$ref": "urn:mirabase:wire#/$defs/ReportDefinition"
}
```


**Response:** 200; application/json.


```json
{
  "$ref": "urn:mirabase:wire#/$defs/ReportDefinition"
}
```


Referenced types: [OpaqueId](wire-schema-reference.md#schema-opaqueid), [ReportDefinition](wire-schema-reference.md#schema-reportdefinition).

**Errors:** `invalid_request`, `unauthenticated`, `forbidden`, `not_found`, `generation_mismatch`, `schema_mismatch`, `policy_changed`, `conflict`, `resource_exhausted`, `storage_failure`, `outcome_unknown`.

**Requirement:** MB-RELWORKFLOW. **Acceptance:** AT-RELWORKFLOW.

**Additional parameter/phase constraints:**


```json
{
  "pathParameters": [
    {
      "name": "reportId",
      "schema": {
        "$ref": "urn:mirabase:wire#/$defs/OpaqueId"
      }
    }
  ]
}
```


## Operation report.rebuild

**Delivery:** roadmap. Unregistered in v0.1; full operation retained for its owning roadmap group.

`POST /api/mirabase/v1/reports/{reportId}/rebuild`

**Authority:** application superuser. **Scope:** resolved-application.

**Preconditions:** immutable authorized application/database context; current application and target generations/revisions; validated canonical request and resource reservation.

**Effects:** Rebuild an application-local report with bounded per-source checkpoints and authorized output.

**Idempotency:** Mutations require Idempotency-Key and canonical request hash; durable intent/result; conflicting reuse is rejected.

**Request schema:**


```json
{
  "$ref": "urn:mirabase:wire#/$defs/RevisionGuard"
}
```


**Response:** 202; application/json.


```json
{
  "$ref": "urn:mirabase:wire#/$defs/Job"
}
```


Referenced types: [Job](wire-schema-reference.md#schema-job), [OpaqueId](wire-schema-reference.md#schema-opaqueid), [RevisionGuard](wire-schema-reference.md#schema-revisionguard).

**Errors:** `invalid_request`, `unauthenticated`, `forbidden`, `not_found`, `generation_mismatch`, `schema_mismatch`, `policy_changed`, `conflict`, `resource_exhausted`, `storage_failure`, `outcome_unknown`.

**Requirement:** MB-RELWORKFLOW. **Acceptance:** AT-RELWORKFLOW.

**Additional parameter/phase constraints:**


```json
{
  "pathParameters": [
    {
      "name": "reportId",
      "schema": {
        "$ref": "urn:mirabase:wire#/$defs/OpaqueId"
      }
    }
  ]
}
```


## Operation importbatch.source.list

**Delivery:** roadmap. Unregistered in v0.1; full operation retained for its owning roadmap group.

`GET /api/mirabase/v1/application-import-batches/{batchId}/sources`

**Authority:** instance operator. **Scope:** instance.

**Preconditions:** resolved instance operator; current application and target generations/revisions; validated canonical request and resource reservation.

**Effects:** Page assigned source/application identities and exact session phases/errors without loading the entire batch.

**Idempotency:** Mutations require Idempotency-Key and canonical request hash; durable intent/result; conflicting reuse is rejected.

No request body.

**Response:** 200; application/json.


```json
{
  "$ref": "urn:mirabase:wire#/$defs/ImportBatchSourcePage"
}
```


Referenced types: [ImportBatchSourcePage](wire-schema-reference.md#schema-importbatchsourcepage), [OpaqueId](wire-schema-reference.md#schema-opaqueid).

**Errors:** `invalid_request`, `unauthenticated`, `forbidden`, `not_found`, `generation_mismatch`, `schema_mismatch`, `policy_changed`, `conflict`, `resource_exhausted`, `storage_failure`, `outcome_unknown`.

**Requirement:** MB-CONSOLIDATE. **Acceptance:** AT-CONSOLIDATE.

**Additional parameter/phase constraints:**


```json
{
  "pathParameters": [
    {
      "name": "batchId",
      "schema": {
        "$ref": "urn:mirabase:wire#/$defs/OpaqueId"
      }
    }
  ]
}
```


## Operation setup.administrator

**Delivery:** roadmap. Unregistered in v0.1; full operation retained for its owning roadmap group.

`POST /api/mirabase/v1/setup/administrator`

**Authority:** purpose-bound bootstrap token for an application with no administrator. **Scope:** resolved-application.

**Preconditions:** immutable authorized application/database context; current application and target generations/revisions; validated canonical request and resource reservation.

**Effects:** Create the first application administrator through inherited forms with a durable bootstrap receipt and consume the scoped token.

**Idempotency:** Mutations require Idempotency-Key and canonical request hash; durable intent/result; conflicting reuse is rejected.

**Request schema:**


```json
{
  "$ref": "urn:mirabase:wire#/$defs/BootstrapAdministrator"
}
```


**Response:** 200; application/json.


```json
{
  "$ref": "urn:mirabase:wire#/$defs/BootstrapComplete"
}
```


Referenced types: [BootstrapAdministrator](wire-schema-reference.md#schema-bootstrapadministrator), [BootstrapComplete](wire-schema-reference.md#schema-bootstrapcomplete).

**Errors:** `invalid_request`, `unauthenticated`, `forbidden`, `not_found`, `generation_mismatch`, `schema_mismatch`, `policy_changed`, `conflict`, `resource_exhausted`, `storage_failure`, `outcome_unknown`.

**Requirement:** MB-APPAUTH. **Acceptance:** AT-APPAUTH.

**Additional parameter/phase constraints:**


```json
{
  "pathParameters": []
}
```


## Inherited HTTP routes
Pinned handler and middleware behavior is authoritative. Preserve the existing method, route, auth, request/response, file and streaming semantics.
| Method | Path | Handler | Pinned source |
|---|---|---|---|
| GET | `/api/settings` | `settingsList` | [pocketbase/apis/settings.go](https://ganigeorgiev.com/git/pocketbase/pocketbase/src/commit/f70b24e186df98df2e7ebc703f103cb94dc3ca63/apis/settings.go) line 15 |
| PATCH | `/api/settings` | `settingsSet` | [pocketbase/apis/settings.go](https://ganigeorgiev.com/git/pocketbase/pocketbase/src/commit/f70b24e186df98df2e7ebc703f103cb94dc3ca63/apis/settings.go) line 16 |
| POST | `/api/settings/test/s3` | `settingsTestS3` | [pocketbase/apis/settings.go](https://ganigeorgiev.com/git/pocketbase/pocketbase/src/commit/f70b24e186df98df2e7ebc703f103cb94dc3ca63/apis/settings.go) line 17 |
| POST | `/api/settings/test/email` | `settingsTestEmail` | [pocketbase/apis/settings.go](https://ganigeorgiev.com/git/pocketbase/pocketbase/src/commit/f70b24e186df98df2e7ebc703f103cb94dc3ca63/apis/settings.go) line 18 |
| POST | `/api/settings/apple/generate-client-secret` | `settingsGenerateAppleClientSecret` | [pocketbase/apis/settings.go](https://ganigeorgiev.com/git/pocketbase/pocketbase/src/commit/f70b24e186df98df2e7ebc703f103cb94dc3ca63/apis/settings.go) line 21 |
| GET | `/api/collections` | `collectionsList` | [pocketbase/apis/collection.go](https://ganigeorgiev.com/git/pocketbase/pocketbase/src/commit/f70b24e186df98df2e7ebc703f103cb94dc3ca63/apis/collection.go) line 20 |
| POST | `/api/collections` | `collectionCreate` | [pocketbase/apis/collection.go](https://ganigeorgiev.com/git/pocketbase/pocketbase/src/commit/f70b24e186df98df2e7ebc703f103cb94dc3ca63/apis/collection.go) line 21 |
| GET | `/api/collections/{collection}` | `collectionView` | [pocketbase/apis/collection.go](https://ganigeorgiev.com/git/pocketbase/pocketbase/src/commit/f70b24e186df98df2e7ebc703f103cb94dc3ca63/apis/collection.go) line 22 |
| PATCH | `/api/collections/{collection}` | `collectionUpdate` | [pocketbase/apis/collection.go](https://ganigeorgiev.com/git/pocketbase/pocketbase/src/commit/f70b24e186df98df2e7ebc703f103cb94dc3ca63/apis/collection.go) line 23 |
| DELETE | `/api/collections/{collection}` | `collectionDelete` | [pocketbase/apis/collection.go](https://ganigeorgiev.com/git/pocketbase/pocketbase/src/commit/f70b24e186df98df2e7ebc703f103cb94dc3ca63/apis/collection.go) line 24 |
| DELETE | `/api/collections/{collection}/truncate` | `collectionTruncate` | [pocketbase/apis/collection.go](https://ganigeorgiev.com/git/pocketbase/pocketbase/src/commit/f70b24e186df98df2e7ebc703f103cb94dc3ca63/apis/collection.go) line 25 |
| PUT | `/api/collections/import` | `collectionsImport` | [pocketbase/apis/collection.go](https://ganigeorgiev.com/git/pocketbase/pocketbase/src/commit/f70b24e186df98df2e7ebc703f103cb94dc3ca63/apis/collection.go) line 26 |
| GET | `/api/collections/meta/scaffolds` | `collectionScaffolds` | [pocketbase/apis/collection.go](https://ganigeorgiev.com/git/pocketbase/pocketbase/src/commit/f70b24e186df98df2e7ebc703f103cb94dc3ca63/apis/collection.go) line 27 |
| GET | `/api/collections/meta/oauth2-providers` | `collectionListOAuth2Providers` | [pocketbase/apis/collection.go](https://ganigeorgiev.com/git/pocketbase/pocketbase/src/commit/f70b24e186df98df2e7ebc703f103cb94dc3ca63/apis/collection.go) line 30 |
| POST | `/api/collections/meta/dry-run-view` | `collectionDryRunView` | [pocketbase/apis/collection.go](https://ganigeorgiev.com/git/pocketbase/pocketbase/src/commit/f70b24e186df98df2e7ebc703f103cb94dc3ca63/apis/collection.go) line 31 |
| GET | `/api/collections/{collection}/records` | `recordsList` | [pocketbase/apis/record_crud.go](https://ganigeorgiev.com/git/pocketbase/pocketbase/src/commit/f70b24e186df98df2e7ebc703f103cb94dc3ca63/apis/record_crud.go) line 29 |
| GET | `/api/collections/{collection}/records/{id}` | `recordView` | [pocketbase/apis/record_crud.go](https://ganigeorgiev.com/git/pocketbase/pocketbase/src/commit/f70b24e186df98df2e7ebc703f103cb94dc3ca63/apis/record_crud.go) line 30 |
| POST | `/api/collections/{collection}/records` | `recordCreate` | [pocketbase/apis/record_crud.go](https://ganigeorgiev.com/git/pocketbase/pocketbase/src/commit/f70b24e186df98df2e7ebc703f103cb94dc3ca63/apis/record_crud.go) line 31 |
| PATCH | `/api/collections/{collection}/records/{id}` | `recordUpdate` | [pocketbase/apis/record_crud.go](https://ganigeorgiev.com/git/pocketbase/pocketbase/src/commit/f70b24e186df98df2e7ebc703f103cb94dc3ca63/apis/record_crud.go) line 32 |
| DELETE | `/api/collections/{collection}/records/{id}` | `recordDelete` | [pocketbase/apis/record_crud.go](https://ganigeorgiev.com/git/pocketbase/pocketbase/src/commit/f70b24e186df98df2e7ebc703f103cb94dc3ca63/apis/record_crud.go) line 33 |
| GET | `/api/oauth2-redirect` | `oauth2SubscriptionRedirect` | [pocketbase/apis/record_auth.go](https://ganigeorgiev.com/git/pocketbase/pocketbase/src/commit/f70b24e186df98df2e7ebc703f103cb94dc3ca63/apis/record_auth.go) line 12 |
| POST | `/api/oauth2-redirect` | `oauth2SubscriptionRedirect` | [pocketbase/apis/record_auth.go](https://ganigeorgiev.com/git/pocketbase/pocketbase/src/commit/f70b24e186df98df2e7ebc703f103cb94dc3ca63/apis/record_auth.go) line 16 |
| GET | `/api/collections/{collection}/auth-methods` | `recordAuthMethods` | [pocketbase/apis/record_auth.go](https://ganigeorgiev.com/git/pocketbase/pocketbase/src/commit/f70b24e186df98df2e7ebc703f103cb94dc3ca63/apis/record_auth.go) line 22 |
| POST | `/api/collections/{collection}/auth-refresh` | `recordAuthRefresh` | [pocketbase/apis/record_auth.go](https://ganigeorgiev.com/git/pocketbase/pocketbase/src/commit/f70b24e186df98df2e7ebc703f103cb94dc3ca63/apis/record_auth.go) line 26 |
| POST | `/api/collections/{collection}/auth-with-password` | `recordAuthWithPassword` | [pocketbase/apis/record_auth.go](https://ganigeorgiev.com/git/pocketbase/pocketbase/src/commit/f70b24e186df98df2e7ebc703f103cb94dc3ca63/apis/record_auth.go) line 31 |
| POST | `/api/collections/{collection}/auth-with-oauth2` | `recordAuthWithOAuth2` | [pocketbase/apis/record_auth.go](https://ganigeorgiev.com/git/pocketbase/pocketbase/src/commit/f70b24e186df98df2e7ebc703f103cb94dc3ca63/apis/record_auth.go) line 35 |
| POST | `/api/collections/{collection}/request-otp` | `recordRequestOTP` | [pocketbase/apis/record_auth.go](https://ganigeorgiev.com/git/pocketbase/pocketbase/src/commit/f70b24e186df98df2e7ebc703f103cb94dc3ca63/apis/record_auth.go) line 39 |
| POST | `/api/collections/{collection}/auth-with-otp` | `recordAuthWithOTP` | [pocketbase/apis/record_auth.go](https://ganigeorgiev.com/git/pocketbase/pocketbase/src/commit/f70b24e186df98df2e7ebc703f103cb94dc3ca63/apis/record_auth.go) line 42 |
| POST | `/api/collections/{collection}/request-password-reset` | `recordRequestPasswordReset` | [pocketbase/apis/record_auth.go](https://ganigeorgiev.com/git/pocketbase/pocketbase/src/commit/f70b24e186df98df2e7ebc703f103cb94dc3ca63/apis/record_auth.go) line 46 |
| POST | `/api/collections/{collection}/confirm-password-reset` | `recordConfirmPasswordReset` | [pocketbase/apis/record_auth.go](https://ganigeorgiev.com/git/pocketbase/pocketbase/src/commit/f70b24e186df98df2e7ebc703f103cb94dc3ca63/apis/record_auth.go) line 49 |
| POST | `/api/collections/{collection}/request-verification` | `recordRequestVerification` | [pocketbase/apis/record_auth.go](https://ganigeorgiev.com/git/pocketbase/pocketbase/src/commit/f70b24e186df98df2e7ebc703f103cb94dc3ca63/apis/record_auth.go) line 53 |
| POST | `/api/collections/{collection}/confirm-verification` | `recordConfirmVerification` | [pocketbase/apis/record_auth.go](https://ganigeorgiev.com/git/pocketbase/pocketbase/src/commit/f70b24e186df98df2e7ebc703f103cb94dc3ca63/apis/record_auth.go) line 56 |
| POST | `/api/collections/{collection}/request-email-change` | `recordRequestEmailChange` | [pocketbase/apis/record_auth.go](https://ganigeorgiev.com/git/pocketbase/pocketbase/src/commit/f70b24e186df98df2e7ebc703f103cb94dc3ca63/apis/record_auth.go) line 60 |
| POST | `/api/collections/{collection}/confirm-email-change` | `recordConfirmEmailChange` | [pocketbase/apis/record_auth.go](https://ganigeorgiev.com/git/pocketbase/pocketbase/src/commit/f70b24e186df98df2e7ebc703f103cb94dc3ca63/apis/record_auth.go) line 64 |
| POST | `/api/collections/{collection}/impersonate/{id}` | `recordAuthImpersonate` | [pocketbase/apis/record_auth.go](https://ganigeorgiev.com/git/pocketbase/pocketbase/src/commit/f70b24e186df98df2e7ebc703f103cb94dc3ca63/apis/record_auth.go) line 68 |
| GET | `/api/logs` | `logsList` | [pocketbase/apis/logs.go](https://ganigeorgiev.com/git/pocketbase/pocketbase/src/commit/f70b24e186df98df2e7ebc703f103cb94dc3ca63/apis/logs.go) line 19 |
| DELETE | `/api/logs` | `logsTruncate` | [pocketbase/apis/logs.go](https://ganigeorgiev.com/git/pocketbase/pocketbase/src/commit/f70b24e186df98df2e7ebc703f103cb94dc3ca63/apis/logs.go) line 20 |
| GET | `/api/logs/stats` | `logsStats` | [pocketbase/apis/logs.go](https://ganigeorgiev.com/git/pocketbase/pocketbase/src/commit/f70b24e186df98df2e7ebc703f103cb94dc3ca63/apis/logs.go) line 21 |
| GET | `/api/logs/{id}` | `logsView` | [pocketbase/apis/logs.go](https://ganigeorgiev.com/git/pocketbase/pocketbase/src/commit/f70b24e186df98df2e7ebc703f103cb94dc3ca63/apis/logs.go) line 22 |
| GET | `/api/backups` | `backupsList` | [pocketbase/apis/backup.go](https://ganigeorgiev.com/git/pocketbase/pocketbase/src/commit/f70b24e186df98df2e7ebc703f103cb94dc3ca63/apis/backup.go) line 19 |
| POST | `/api/backups` | `backupCreate` | [pocketbase/apis/backup.go](https://ganigeorgiev.com/git/pocketbase/pocketbase/src/commit/f70b24e186df98df2e7ebc703f103cb94dc3ca63/apis/backup.go) line 20 |
| POST | `/api/backups/upload` | `backupUpload` | [pocketbase/apis/backup.go](https://ganigeorgiev.com/git/pocketbase/pocketbase/src/commit/f70b24e186df98df2e7ebc703f103cb94dc3ca63/apis/backup.go) line 21 |
| GET | `/api/backups/{key}` | `backupDownload` | [pocketbase/apis/backup.go](https://ganigeorgiev.com/git/pocketbase/pocketbase/src/commit/f70b24e186df98df2e7ebc703f103cb94dc3ca63/apis/backup.go) line 22 |
| DELETE | `/api/backups/{key}` | `backupDelete` | [pocketbase/apis/backup.go](https://ganigeorgiev.com/git/pocketbase/pocketbase/src/commit/f70b24e186df98df2e7ebc703f103cb94dc3ca63/apis/backup.go) line 23 |
| POST | `/api/backups/{key}/restore` | `backupRestore` | [pocketbase/apis/backup.go](https://ganigeorgiev.com/git/pocketbase/pocketbase/src/commit/f70b24e186df98df2e7ebc703f103cb94dc3ca63/apis/backup.go) line 24 |
| GET | `/api/crons` | `cronsList` | [pocketbase/apis/cron.go](https://ganigeorgiev.com/git/pocketbase/pocketbase/src/commit/f70b24e186df98df2e7ebc703f103cb94dc3ca63/apis/cron.go) line 17 |
| POST | `/api/crons/{id}` | `cronRun` | [pocketbase/apis/cron.go](https://ganigeorgiev.com/git/pocketbase/pocketbase/src/commit/f70b24e186df98df2e7ebc703f103cb94dc3ca63/apis/cron.go) line 18 |
| POST | `/api/files/token` | `api.fileToken` | [pocketbase/apis/file.go](https://ganigeorgiev.com/git/pocketbase/pocketbase/src/commit/f70b24e186df98df2e7ebc703f103cb94dc3ca63/apis/file.go) line 44 |
| GET | `/api/files/{collection}/{recordId}/{filename}` | `api.download` | [pocketbase/apis/file.go](https://ganigeorgiev.com/git/pocketbase/pocketbase/src/commit/f70b24e186df98df2e7ebc703f103cb94dc3ca63/apis/file.go) line 45 |
| POST | `/api/batch` | `batchTransaction` | [pocketbase/apis/batch.go](https://ganigeorgiev.com/git/pocketbase/pocketbase/src/commit/f70b24e186df98df2e7ebc703f103cb94dc3ca63/apis/batch.go) line 28 |
| GET | `/api/realtime` | `realtimeConnect` | [pocketbase/apis/realtime.go](https://ganigeorgiev.com/git/pocketbase/pocketbase/src/commit/f70b24e186df98df2e7ebc703f103cb94dc3ca63/apis/realtime.go) line 37 |
| POST | `/api/realtime` | `realtimeSetSubscriptions` | [pocketbase/apis/realtime.go](https://ganigeorgiev.com/git/pocketbase/pocketbase/src/commit/f70b24e186df98df2e7ebc703f103cb94dc3ca63/apis/realtime.go) line 38 |
| GET | `/api/health` | `healthCheck` | [pocketbase/apis/health.go](https://ganigeorgiev.com/git/pocketbase/pocketbase/src/commit/f70b24e186df98df2e7ebc703f103cb94dc3ca63/apis/health.go) line 14 |
| POST | `/api/sql` | `runSQL` | [pocketbase/apis/sql.go](https://ganigeorgiev.com/git/pocketbase/pocketbase/src/commit/f70b24e186df98df2e7ebc703f103cb94dc3ca63/apis/sql.go) line 24 |
