# Wire schema reference

**Current release authority:** [current implementation inventory](../current-product.md), [user documentation](../user/index.md) and [launch status](../launch/status.md). Initial v0.1/roadmap delivery labels below preserve planning contracts; they do not establish today's implementation or permit deleting subsequently integrated features. [Open-source platform roadmap](../roadmap/platform/README.md) has separate design and gates.

**Delivery: v0.1 and roadmap.** [v0.1-build.md](v0.1-build.md) selects the current portion; [roadsmap.md](roadsmap.md) retains the remainder. Detailed schemas/configuration/ABI and full-family acceptance below describe the target superset unless narrowed by that allocation.

The named definitions below specify the complete request/response types. Assemble them under `$defs` in a JSON Schema 2020-12 document with `$id: "urn:mirabase:wire"`. References beginning `urn:mirabase:wire#/$defs/` select another named definition in this document; they do not name files to locate. JSON Schema constraints and the owning chapter's semantic checks both apply. Unknown fields are rejected where `additionalProperties` is false. Dynamic field/collection option resolvers must generate and enforce the exact pinned schema at runtime.

## Schema DatabaseId


```json
{
  "type": "string",
  "pattern": "^(primary|[a-f0-9]{32})$"
}
```


## Schema OpaqueId


```json
{
  "type": "string",
  "pattern": "^[A-Za-z0-9_-]{1,128}$"
}
```


## Schema Revision


```json
{
  "type": "string",
  "pattern": "^[1-9][0-9]*$"
}
```


## Schema Timestamp


```json
{
  "type": "string",
  "format": "date-time"
}
```


## Schema Subject


```json
{
  "type": "object",
  "properties": {
    "collectionId": {
      "type": "string"
    },
    "recordId": {
      "type": "string"
    },
    "applicationId": {
      "$ref": "urn:mirabase:wire#/$defs/ApplicationId"
    }
  },
  "required": [
    "collectionId",
    "recordId"
  ],
  "additionalProperties": false,
  "description": "Omitted applicationId is resolved from the authenticated application route; a conflicting supplied ID is rejected. Internal principals always use ApplicationSubject."
}
```


Referenced types: [ApplicationId](wire-schema-reference.md#schema-applicationid).

## Schema Scope


```json
{
  "type": "string",
  "enum": [
    "record.read",
    "record.write",
    "files.read",
    "files.write",
    "changes.read",
    "record.sync",
    "database.inspect",
    "sql.read",
    "sql.write",
    "native.sync"
  ]
}
```


## Schema Profile


```json
{
  "type": "string",
  "enum": [
    "wal",
    "mvcc"
  ]
}
```


## Schema DatabaseState


```json
{
  "type": "string",
  "enum": [
    "requested",
    "provisioning",
    "ready",
    "failed",
    "maintenance",
    "migrating",
    "restoring",
    "deleting",
    "tombstoned",
    "purged",
    "quarantined"
  ]
}
```


## Schema Database


```json
{
  "type": "object",
  "properties": {
    "id": {
      "$ref": "urn:mirabase:wire#/$defs/DatabaseId"
    },
    "displayName": {
      "type": "string"
    },
    "kind": {
      "type": "string",
      "enum": [
        "primary",
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
    "state": {
      "$ref": "urn:mirabase:wire#/$defs/DatabaseState"
    },
    "generation": {
      "$ref": "urn:mirabase:wire#/$defs/Revision"
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
    "schemaRevision": {
      "$ref": "urn:mirabase:wire#/$defs/Revision"
    },
    "policyEpoch": {
      "$ref": "urn:mirabase:wire#/$defs/Revision"
    },
    "created": {
      "$ref": "urn:mirabase:wire#/$defs/Timestamp"
    },
    "updated": {
      "$ref": "urn:mirabase:wire#/$defs/Timestamp"
    },
    "applicationId": {
      "$ref": "urn:mirabase:wire#/$defs/ApplicationId"
    }
  },
  "required": [
    "id",
    "displayName",
    "kind",
    "owner",
    "state",
    "generation",
    "profile",
    "templateId",
    "schemaRevision",
    "policyEpoch",
    "created",
    "updated"
  ],
  "additionalProperties": false
}
```


Referenced types: [ApplicationId](wire-schema-reference.md#schema-applicationid), [DatabaseId](wire-schema-reference.md#schema-databaseid), [DatabaseState](wire-schema-reference.md#schema-databasestate), [Profile](wire-schema-reference.md#schema-profile), [Revision](wire-schema-reference.md#schema-revision), [Subject](wire-schema-reference.md#schema-subject), [Timestamp](wire-schema-reference.md#schema-timestamp).

## Schema Job


```json
{
  "type": "object",
  "properties": {
    "id": {
      "type": "string"
    },
    "kind": {
      "type": "string"
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
    },
    "state": {
      "type": "string",
      "enum": [
        "queued",
        "running",
        "waiting_retry",
        "succeeded",
        "failed",
        "cancelled"
      ]
    },
    "phase": {
      "type": "string"
    },
    "completed": {
      "type": "integer",
      "minimum": 0,
      "maximum": 9007199254740991
    },
    "total": {
      "anyOf": [
        {
          "type": "integer",
          "minimum": 0,
          "maximum": 9007199254740991
        },
        {
          "type": "null"
        }
      ]
    },
    "attempts": {
      "type": "integer",
      "minimum": 0,
      "maximum": 9007199254740991
    },
    "cancelRequested": {
      "type": "boolean"
    },
    "resultRef": {
      "anyOf": [
        {
          "type": "string"
        },
        {
          "type": "null"
        }
      ]
    },
    "errorCode": {
      "anyOf": [
        {
          "type": "string"
        },
        {
          "type": "null"
        }
      ]
    },
    "created": {
      "$ref": "urn:mirabase:wire#/$defs/Timestamp"
    },
    "updated": {
      "$ref": "urn:mirabase:wire#/$defs/Timestamp"
    }
  },
  "required": [
    "id",
    "kind",
    "databaseId",
    "state",
    "phase",
    "completed",
    "total",
    "attempts",
    "cancelRequested",
    "resultRef",
    "errorCode",
    "created",
    "updated"
  ],
  "additionalProperties": false
}
```


Referenced types: [DatabaseId](wire-schema-reference.md#schema-databaseid), [Timestamp](wire-schema-reference.md#schema-timestamp).

## Schema Error


```json
{
  "type": "object",
  "properties": {
    "status": {
      "type": "integer",
      "minimum": 400,
      "maximum": 599
    },
    "message": {
      "type": "string"
    },
    "data": {
      "type": "object",
      "properties": {
        "mirabase": {
          "type": "object",
          "properties": {
            "code": {
              "type": "string"
            },
            "retryable": {
              "type": "boolean"
            },
            "requestId": {
              "type": "string"
            },
            "operationId": {
              "type": "string"
            },
            "databaseId": {
              "$ref": "urn:mirabase:wire#/$defs/DatabaseId"
            },
            "committed": {
              "anyOf": [
                {
                  "type": "boolean"
                },
                {
                  "type": "null"
                }
              ]
            },
            "fieldErrors": {
              "type": "array",
              "items": {
                "type": "object",
                "properties": {
                  "field": {
                    "type": "string"
                  },
                  "code": {
                    "type": "string"
                  },
                  "message": {
                    "type": "string"
                  }
                },
                "required": [
                  "field",
                  "code",
                  "message"
                ],
                "additionalProperties": false
              },
              "maxItems": 100
            },
            "recovery": {
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
            "code",
            "retryable",
            "requestId",
            "committed"
          ],
          "additionalProperties": false
        }
      },
      "required": [
        "mirabase"
      ],
      "additionalProperties": false
    }
  },
  "required": [
    "status",
    "message",
    "data"
  ],
  "additionalProperties": false
}
```


Referenced types: [DatabaseId](wire-schema-reference.md#schema-databaseid).

## Schema Grant


```json
{
  "type": "object",
  "properties": {
    "id": {
      "type": "string"
    },
    "databaseId": {
      "$ref": "urn:mirabase:wire#/$defs/DatabaseId"
    },
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
    "revokedAt": {
      "anyOf": [
        {
          "$ref": "urn:mirabase:wire#/$defs/Timestamp"
        },
        {
          "type": "null"
        }
      ]
    },
    "epoch": {
      "$ref": "urn:mirabase:wire#/$defs/Revision"
    }
  },
  "required": [
    "id",
    "databaseId",
    "subject",
    "scopes",
    "expiresAt",
    "revokedAt",
    "epoch"
  ],
  "additionalProperties": false
}
```


Referenced types: [DatabaseId](wire-schema-reference.md#schema-databaseid), [Revision](wire-schema-reference.md#schema-revision), [Scope](wire-schema-reference.md#schema-scope), [Subject](wire-schema-reference.md#schema-subject), [Timestamp](wire-schema-reference.md#schema-timestamp).

## Schema Capability


```json
{
  "type": "object",
  "properties": {
    "id": {
      "type": "string"
    },
    "available": {
      "type": "boolean"
    },
    "reason": {
      "anyOf": [
        {
          "type": "string"
        },
        {
          "type": "null"
        }
      ]
    },
    "frontends": {
      "type": "array",
      "items": {
        "type": "string",
        "enum": [
          "sqlite",
          "postgres",
          "http",
          "native"
        ]
      },
      "maxItems": 4
    },
    "profiles": {
      "type": "array",
      "items": {
        "$ref": "urn:mirabase:wire#/$defs/Profile"
      },
      "maxItems": 2
    },
    "qualified": {
      "type": "boolean"
    }
  },
  "required": [
    "id",
    "available",
    "reason",
    "frontends",
    "profiles",
    "qualified"
  ],
  "additionalProperties": false
}
```


Referenced types: [Profile](wire-schema-reference.md#schema-profile).

## Schema Capabilities


```json
{
  "type": "object",
  "properties": {
    "productVersion": {
      "type": "string"
    },
    "engineRevision": {
      "type": "string"
    },
    "nativeDigest": {
      "type": "string"
    },
    "abiVersion": {
      "type": "string"
    },
    "target": {
      "type": "string"
    },
    "capabilities": {
      "type": "array",
      "items": {
        "$ref": "urn:mirabase:wire#/$defs/Capability"
      },
      "maxItems": 300
    }
  },
  "required": [
    "productVersion",
    "engineRevision",
    "nativeDigest",
    "abiVersion",
    "target",
    "capabilities"
  ],
  "additionalProperties": false
}
```


Referenced types: [Capability](wire-schema-reference.md#schema-capability).

## Schema TypedValue


```json
{
  "oneOf": [
    {
      "type": "null"
    },
    {
      "type": "boolean"
    },
    {
      "type": "number"
    },
    {
      "type": "string"
    },
    {
      "type": "object",
      "properties": {
        "type": {
          "type": "string",
          "enum": [
            "int64",
            "decimal"
          ]
        },
        "value": {
          "type": "string"
        }
      },
      "required": [
        "type",
        "value"
      ],
      "additionalProperties": false
    },
    {
      "type": "object",
      "properties": {
        "type": {
          "type": "string",
          "enum": [
            "blob"
          ]
        },
        "base64": {
          "type": "string"
        }
      },
      "required": [
        "type",
        "base64"
      ],
      "additionalProperties": false
    },
    {
      "type": "object",
      "properties": {
        "type": {
          "type": "string",
          "enum": [
            "array"
          ]
        },
        "elementType": {
          "type": "string"
        },
        "values": {
          "type": "array",
          "items": {
            "$ref": "urn:mirabase:wire#/$defs/TypedValue"
          },
          "maxItems": 10000
        }
      },
      "required": [
        "type",
        "elementType",
        "values"
      ],
      "additionalProperties": false
    },
    {
      "type": "object",
      "properties": {
        "type": {
          "type": "string",
          "enum": [
            "struct"
          ]
        },
        "typeId": {
          "type": "string"
        },
        "members": {
          "type": "object",
          "additionalProperties": {
            "$ref": "urn:mirabase:wire#/$defs/TypedValue"
          }
        }
      },
      "required": [
        "type",
        "typeId",
        "members"
      ],
      "additionalProperties": false
    },
    {
      "type": "object",
      "properties": {
        "type": {
          "type": "string",
          "enum": [
            "union"
          ]
        },
        "typeId": {
          "type": "string"
        },
        "tag": {
          "type": "string"
        },
        "value": {
          "$ref": "urn:mirabase:wire#/$defs/TypedValue"
        }
      },
      "required": [
        "type",
        "typeId",
        "tag",
        "value"
      ],
      "additionalProperties": false
    }
  ]
}
```


Referenced types: [TypedValue](wire-schema-reference.md#schema-typedvalue).

## Schema Record


```json
{
  "type": "object",
  "description": "Exact properties are generated from the selected collection schema at schemaRevision; hidden/system/auth protection applies. Dynamic schema is mandatory, not arbitrary object acceptance.",
  "properties": {
    "id": {
      "type": "string"
    },
    "collectionId": {
      "type": "string"
    },
    "collectionName": {
      "type": "string"
    }
  },
  "required": [
    "id",
    "collectionId",
    "collectionName"
  ],
  "x-schema-resolver": "collection+schemaRevision+actorProjection"
}
```


## Schema SearchItem


```json
{
  "type": "object",
  "properties": {
    "record": {
      "$ref": "urn:mirabase:wire#/$defs/Record"
    },
    "score": {
      "type": "object",
      "properties": {
        "kind": {
          "type": "string"
        },
        "value": {
          "type": "number"
        }
      },
      "required": [
        "kind",
        "value"
      ],
      "additionalProperties": false
    },
    "highlights": {
      "type": "array",
      "items": {
        "type": "object",
        "properties": {
          "field": {
            "type": "string"
          },
          "fragments": {
            "type": "array",
            "items": {
              "type": "string"
            },
            "maxItems": 20
          }
        },
        "required": [
          "field",
          "fragments"
        ],
        "additionalProperties": false
      },
      "maxItems": 30
    }
  },
  "required": [
    "record",
    "score",
    "highlights"
  ],
  "additionalProperties": false
}
```


Referenced types: [Record](wire-schema-reference.md#schema-record).

## Schema Vector


```json
{
  "oneOf": [
    {
      "type": "array",
      "items": {
        "type": "number"
      },
      "maxItems": 4096
    },
    {
      "type": "object",
      "properties": {
        "dimensions": {
          "type": "integer",
          "minimum": 1,
          "maximum": 4096
        },
        "indices": {
          "type": "array",
          "items": {
            "type": "integer",
            "minimum": 0,
            "maximum": 4095
          },
          "maxItems": 4096
        },
        "values": {
          "type": "array",
          "items": {
            "type": "number"
          },
          "maxItems": 4096
        }
      },
      "required": [
        "dimensions",
        "indices",
        "values"
      ],
      "additionalProperties": false
    }
  ]
}
```


## Schema VectorClause


```json
{
  "type": "object",
  "properties": {
    "field": {
      "type": "string"
    },
    "query": {
      "$ref": "urn:mirabase:wire#/$defs/Vector"
    },
    "metric": {
      "type": "string",
      "enum": [
        "cosine",
        "l2",
        "negative_dot",
        "jaccard"
      ]
    }
  },
  "required": [
    "field",
    "query",
    "metric"
  ],
  "additionalProperties": false
}
```


Referenced types: [Vector](wire-schema-reference.md#schema-vector).

## Schema TextClause


```json
{
  "type": "object",
  "properties": {
    "fields": {
      "type": "array",
      "items": {
        "type": "string"
      },
      "maxItems": 30
    },
    "query": {
      "type": "string"
    },
    "syntax": {
      "type": "string",
      "enum": [
        "simple",
        "advanced"
      ]
    },
    "weights": {
      "type": "array",
      "items": {
        "type": "object",
        "properties": {
          "field": {
            "type": "string"
          },
          "weight": {
            "type": "number",
            "exclusiveMinimum": 0
          }
        },
        "required": [
          "field",
          "weight"
        ],
        "additionalProperties": false
      },
      "maxItems": 30
    },
    "highlight": {
      "type": "boolean"
    }
  },
  "required": [
    "fields",
    "query"
  ],
  "additionalProperties": false
}
```


## Schema SearchRequest


```json
{
  "oneOf": [
    {
      "type": "object",
      "properties": {
        "mode": {
          "type": "string",
          "enum": [
            "vector"
          ]
        },
        "vector": {
          "$ref": "urn:mirabase:wire#/$defs/VectorClause"
        },
        "limit": {
          "type": "integer",
          "minimum": 1,
          "maximum": 100
        },
        "filter": {
          "type": "string"
        },
        "fields": {
          "type": "array",
          "items": {
            "type": "string"
          },
          "maxItems": 100
        },
        "cursor": {
          "type": "string"
        }
      },
      "required": [
        "mode",
        "vector"
      ],
      "additionalProperties": false
    },
    {
      "type": "object",
      "properties": {
        "mode": {
          "type": "string",
          "enum": [
            "text"
          ]
        },
        "text": {
          "$ref": "urn:mirabase:wire#/$defs/TextClause"
        },
        "limit": {
          "type": "integer",
          "minimum": 1,
          "maximum": 100
        },
        "filter": {
          "type": "string"
        },
        "fields": {
          "type": "array",
          "items": {
            "type": "string"
          },
          "maxItems": 100
        },
        "cursor": {
          "type": "string"
        }
      },
      "required": [
        "mode",
        "text"
      ],
      "additionalProperties": false
    },
    {
      "type": "object",
      "properties": {
        "mode": {
          "type": "string",
          "enum": [
            "hybrid"
          ]
        },
        "vector": {
          "$ref": "urn:mirabase:wire#/$defs/VectorClause"
        },
        "text": {
          "$ref": "urn:mirabase:wire#/$defs/TextClause"
        },
        "fusion": {
          "type": "object",
          "properties": {
            "kind": {
              "type": "string",
              "enum": [
                "rrf"
              ]
            },
            "k": {
              "type": "integer",
              "minimum": 1,
              "maximum": 1000
            }
          },
          "required": [
            "kind",
            "k"
          ],
          "additionalProperties": false
        },
        "limit": {
          "type": "integer",
          "minimum": 1,
          "maximum": 100
        },
        "filter": {
          "type": "string"
        },
        "fields": {
          "type": "array",
          "items": {
            "type": "string"
          },
          "maxItems": 100
        },
        "cursor": {
          "type": "string"
        }
      },
      "required": [
        "mode",
        "vector",
        "text",
        "fusion"
      ],
      "additionalProperties": false
    },
    {
      "type": "object",
      "properties": {
        "mode": {
          "type": "string",
          "enum": [
            "fuzzy"
          ]
        },
        "field": {
          "type": "string"
        },
        "query": {
          "type": "string"
        },
        "algorithm": {
          "type": "string",
          "enum": [
            "levenshtein",
            "damerau",
            "osa",
            "hamming",
            "jaro_winkler",
            "soundex"
          ]
        },
        "maximumDistance": {
          "type": "integer",
          "minimum": 0,
          "maximum": 1000
        },
        "minimumSimilarity": {
          "type": "number",
          "minimum": 0,
          "maximum": 1
        },
        "limit": {
          "type": "integer",
          "minimum": 1,
          "maximum": 100
        },
        "filter": {
          "type": "string"
        },
        "cursor": {
          "type": "string"
        }
      },
      "required": [
        "mode",
        "field",
        "query",
        "algorithm"
      ],
      "additionalProperties": false
    }
  ]
}
```


Referenced types: [TextClause](wire-schema-reference.md#schema-textclause), [VectorClause](wire-schema-reference.md#schema-vectorclause).

## Schema SearchResult


```json
{
  "type": "object",
  "properties": {
    "databaseId": {
      "$ref": "urn:mirabase:wire#/$defs/DatabaseId"
    },
    "generation": {
      "$ref": "urn:mirabase:wire#/$defs/Revision"
    },
    "schemaRevision": {
      "$ref": "urn:mirabase:wire#/$defs/Revision"
    },
    "items": {
      "type": "array",
      "items": {
        "$ref": "urn:mirabase:wire#/$defs/SearchItem"
      },
      "maxItems": 100
    },
    "mode": {
      "type": "string"
    },
    "metric": {
      "anyOf": [
        {
          "type": "string"
        },
        {
          "type": "null"
        }
      ]
    },
    "complete": {
      "type": "boolean"
    },
    "scannedCount": {
      "type": "integer",
      "minimum": 0,
      "maximum": 9007199254740991
    },
    "elapsedMs": {
      "type": "number",
      "minimum": 0
    },
    "nextCursor": {
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
    "databaseId",
    "generation",
    "schemaRevision",
    "items",
    "mode",
    "metric",
    "complete",
    "scannedCount",
    "elapsedMs",
    "nextCursor"
  ],
  "additionalProperties": false
}
```


Referenced types: [DatabaseId](wire-schema-reference.md#schema-databaseid), [Revision](wire-schema-reference.md#schema-revision), [SearchItem](wire-schema-reference.md#schema-searchitem).

## Schema Change


```json
{
  "type": "object",
  "properties": {
    "ordinal": {
      "type": "integer",
      "minimum": 0,
      "maximum": 9007199254740991
    },
    "collectionId": {
      "anyOf": [
        {
          "type": "string"
        },
        {
          "type": "null"
        }
      ]
    },
    "table": {
      "type": "string"
    },
    "recordId": {
      "type": "string"
    },
    "action": {
      "type": "string",
      "enum": [
        "create",
        "update",
        "delete",
        "schema",
        "evict"
      ]
    },
    "before": {
      "anyOf": [
        {
          "$ref": "urn:mirabase:wire#/$defs/Record"
        },
        {
          "type": "null"
        }
      ]
    },
    "after": {
      "anyOf": [
        {
          "$ref": "urn:mirabase:wire#/$defs/Record"
        },
        {
          "type": "null"
        }
      ]
    },
    "version": {
      "$ref": "urn:mirabase:wire#/$defs/Revision"
    }
  },
  "required": [
    "ordinal",
    "collectionId",
    "table",
    "recordId",
    "action",
    "before",
    "after",
    "version"
  ],
  "additionalProperties": false
}
```


Referenced types: [Record](wire-schema-reference.md#schema-record), [Revision](wire-schema-reference.md#schema-revision).

## Schema ChangeTransaction


```json
{
  "type": "object",
  "properties": {
    "databaseId": {
      "$ref": "urn:mirabase:wire#/$defs/DatabaseId"
    },
    "generation": {
      "$ref": "urn:mirabase:wire#/$defs/Revision"
    },
    "transactionId": {
      "type": "string"
    },
    "commitSequence": {
      "$ref": "urn:mirabase:wire#/$defs/Revision"
    },
    "schemaRevision": {
      "$ref": "urn:mirabase:wire#/$defs/Revision"
    },
    "timestamp": {
      "$ref": "urn:mirabase:wire#/$defs/Timestamp"
    },
    "source": {
      "type": "string",
      "enum": [
        "record",
        "admin-sql",
        "pg",
        "native-sync",
        "migration"
      ]
    },
    "changes": {
      "type": "array",
      "items": {
        "$ref": "urn:mirabase:wire#/$defs/Change"
      },
      "maxItems": 100000
    }
  },
  "required": [
    "databaseId",
    "generation",
    "transactionId",
    "commitSequence",
    "schemaRevision",
    "timestamp",
    "source",
    "changes"
  ],
  "additionalProperties": false
}
```


Referenced types: [Change](wire-schema-reference.md#schema-change), [DatabaseId](wire-schema-reference.md#schema-databaseid), [Revision](wire-schema-reference.md#schema-revision), [Timestamp](wire-schema-reference.md#schema-timestamp).

## Schema ChangeChunk


```json
{
  "type": "object",
  "properties": {
    "kind": {
      "type": "string",
      "enum": [
        "chunk"
      ]
    },
    "transactionId": {
      "type": "string"
    },
    "chunkIndex": {
      "type": "integer",
      "minimum": 0,
      "maximum": 9007199254740991
    },
    "chunkCount": {
      "type": "integer",
      "minimum": 1,
      "maximum": 65536
    },
    "payloadEncoding": {
      "type": "string",
      "enum": [
        "json-utf8-base64"
      ]
    },
    "payloadBase64": {
      "type": "string"
    },
    "transactionSha256": {
      "type": "string",
      "pattern": "^[a-f0-9]{64}$"
    },
    "finalCursor": {
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
    "kind",
    "transactionId",
    "chunkIndex",
    "chunkCount",
    "payloadEncoding",
    "payloadBase64",
    "transactionSha256",
    "finalCursor"
  ],
  "additionalProperties": false
}
```


## Schema ChangePage


```json
{
  "type": "object",
  "properties": {
    "transactions": {
      "type": "array",
      "items": {
        "oneOf": [
          {
            "$ref": "urn:mirabase:wire#/$defs/ChangeTransaction"
          },
          {
            "$ref": "urn:mirabase:wire#/$defs/ChangeChunk"
          }
        ]
      },
      "maxItems": 1000
    },
    "nextCursor": {
      "type": "string"
    },
    "hasMore": {
      "type": "boolean"
    },
    "schemaRevision": {
      "$ref": "urn:mirabase:wire#/$defs/Revision"
    },
    "retentionFloor": {
      "type": "string"
    }
  },
  "required": [
    "transactions",
    "nextCursor",
    "hasMore",
    "schemaRevision",
    "retentionFloor"
  ],
  "additionalProperties": false
}
```


Referenced types: [ChangeChunk](wire-schema-reference.md#schema-changechunk), [ChangeTransaction](wire-schema-reference.md#schema-changetransaction), [Revision](wire-schema-reference.md#schema-revision).

## Schema Mutation


```json
{
  "type": "object",
  "properties": {
    "mutationId": {
      "type": "string"
    },
    "collection": {
      "type": "string"
    },
    "recordId": {
      "type": "string"
    },
    "action": {
      "type": "string",
      "enum": [
        "create",
        "update",
        "delete"
      ]
    },
    "baseVersion": {
      "anyOf": [
        {
          "$ref": "urn:mirabase:wire#/$defs/Revision"
        },
        {
          "type": "null"
        }
      ]
    },
    "data": {
      "type": "object",
      "x-schema-resolver": "writable-fields-of-selected-collection"
    },
    "stagedFileIds": {
      "type": "array",
      "items": {
        "type": "string"
      },
      "maxItems": 100
    }
  },
  "required": [
    "mutationId",
    "collection",
    "recordId",
    "action",
    "baseVersion"
  ],
  "additionalProperties": false
}
```


Referenced types: [Revision](wire-schema-reference.md#schema-revision).

## Schema PushGroup


```json
{
  "type": "object",
  "properties": {
    "groupId": {
      "type": "string",
      "format": "uuid"
    },
    "mutations": {
      "type": "array",
      "items": {
        "$ref": "urn:mirabase:wire#/$defs/Mutation"
      },
      "maxItems": 50
    }
  },
  "required": [
    "groupId",
    "mutations"
  ],
  "additionalProperties": false
}
```


Referenced types: [Mutation](wire-schema-reference.md#schema-mutation).

## Schema Push


```json
{
  "type": "object",
  "properties": {
    "clientInstanceId": {
      "type": "string"
    },
    "schemaRevision": {
      "$ref": "urn:mirabase:wire#/$defs/Revision"
    },
    "generation": {
      "$ref": "urn:mirabase:wire#/$defs/Revision"
    },
    "policyEpoch": {
      "$ref": "urn:mirabase:wire#/$defs/Revision"
    },
    "groups": {
      "type": "array",
      "items": {
        "$ref": "urn:mirabase:wire#/$defs/PushGroup"
      },
      "maxItems": 20
    }
  },
  "required": [
    "clientInstanceId",
    "schemaRevision",
    "generation",
    "policyEpoch",
    "groups"
  ],
  "additionalProperties": false
}
```


Referenced types: [PushGroup](wire-schema-reference.md#schema-pushgroup), [Revision](wire-schema-reference.md#schema-revision).

## Schema PushResult


```json
{
  "type": "object",
  "properties": {
    "groups": {
      "type": "array",
      "items": {
        "type": "object",
        "properties": {
          "groupId": {
            "type": "string"
          },
          "status": {
            "type": "string",
            "enum": [
              "committed",
              "duplicate",
              "conflict",
              "rejected"
            ]
          },
          "cursor": {
            "anyOf": [
              {
                "type": "string"
              },
              {
                "type": "null"
              }
            ]
          },
          "mutations": {
            "type": "array",
            "items": {
              "type": "object",
              "properties": {
                "mutationId": {
                  "type": "string"
                },
                "version": {
                  "anyOf": [
                    {
                      "$ref": "urn:mirabase:wire#/$defs/Revision"
                    },
                    {
                      "type": "null"
                    }
                  ]
                },
                "code": {
                  "anyOf": [
                    {
                      "type": "string"
                    },
                    {
                      "type": "null"
                    }
                  ]
                },
                "fieldErrors": {
                  "type": "array",
                  "items": {
                    "type": "object",
                    "properties": {
                      "field": {
                        "type": "string"
                      },
                      "code": {
                        "type": "string"
                      },
                      "message": {
                        "type": "string"
                      }
                    },
                    "required": [
                      "field",
                      "code",
                      "message"
                    ],
                    "additionalProperties": false
                  },
                  "maxItems": 100
                },
                "serverRecord": {
                  "anyOf": [
                    {
                      "$ref": "urn:mirabase:wire#/$defs/Record"
                    },
                    {
                      "type": "null"
                    }
                  ]
                }
              },
              "required": [
                "mutationId",
                "version",
                "code",
                "fieldErrors",
                "serverRecord"
              ],
              "additionalProperties": false
            },
            "maxItems": 50
          }
        },
        "required": [
          "groupId",
          "status",
          "cursor",
          "mutations"
        ],
        "additionalProperties": false
      },
      "maxItems": 20
    }
  },
  "required": [
    "groups"
  ],
  "additionalProperties": false
}
```


Referenced types: [Record](wire-schema-reference.md#schema-record), [Revision](wire-schema-reference.md#schema-revision).

## Schema Bootstrap


```json
{
  "type": "object",
  "properties": {
    "sessionId": {
      "type": "string"
    },
    "manifestHash": {
      "type": "string"
    },
    "generation": {
      "$ref": "urn:mirabase:wire#/$defs/Revision"
    },
    "policyEpoch": {
      "$ref": "urn:mirabase:wire#/$defs/Revision"
    },
    "schemaRevision": {
      "$ref": "urn:mirabase:wire#/$defs/Revision"
    },
    "cursor": {
      "type": "string"
    },
    "pageCount": {
      "type": "integer",
      "minimum": 0,
      "maximum": 9007199254740991
    },
    "expiresAt": {
      "$ref": "urn:mirabase:wire#/$defs/Timestamp"
    }
  },
  "required": [
    "sessionId",
    "manifestHash",
    "generation",
    "policyEpoch",
    "schemaRevision",
    "cursor",
    "pageCount",
    "expiresAt"
  ],
  "additionalProperties": false
}
```


Referenced types: [Revision](wire-schema-reference.md#schema-revision), [Timestamp](wire-schema-reference.md#schema-timestamp).

## Schema FileEntry


```json
{
  "type": "object",
  "properties": {
    "collectionId": {
      "type": "string"
    },
    "recordId": {
      "type": "string"
    },
    "field": {
      "type": "string"
    },
    "filename": {
      "type": "string"
    },
    "size": {
      "type": "integer",
      "minimum": 0,
      "maximum": 9007199254740991
    },
    "sha256": {
      "type": "string",
      "pattern": "^[a-f0-9]{64}$"
    },
    "protected": {
      "type": "boolean"
    }
  },
  "required": [
    "collectionId",
    "recordId",
    "field",
    "filename",
    "size",
    "sha256",
    "protected"
  ],
  "additionalProperties": false
}
```


## Schema BootstrapPage


```json
{
  "type": "object",
  "properties": {
    "sessionId": {
      "type": "string"
    },
    "index": {
      "type": "integer",
      "minimum": 0,
      "maximum": 9007199254740991
    },
    "records": {
      "type": "array",
      "items": {
        "$ref": "urn:mirabase:wire#/$defs/Record"
      },
      "maxItems": 1000
    },
    "files": {
      "type": "array",
      "items": {
        "$ref": "urn:mirabase:wire#/$defs/FileEntry"
      },
      "maxItems": 1000
    },
    "sha256": {
      "type": "string"
    }
  },
  "required": [
    "sessionId",
    "index",
    "records",
    "files",
    "sha256"
  ],
  "additionalProperties": false
}
```


Referenced types: [FileEntry](wire-schema-reference.md#schema-fileentry), [Record](wire-schema-reference.md#schema-record).

## Schema Receipt


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
        "committed",
        "pending",
        "unknown",
        "expired"
      ]
    },
    "operationId": {
      "type": "string"
    },
    "resultRef": {
      "anyOf": [
        {
          "type": "string"
        },
        {
          "type": "null"
        }
      ]
    },
    "expiresAt": {
      "$ref": "urn:mirabase:wire#/$defs/Timestamp"
    }
  },
  "required": [
    "id",
    "state",
    "operationId",
    "resultRef",
    "expiresAt"
  ],
  "additionalProperties": false
}
```


Referenced types: [Timestamp](wire-schema-reference.md#schema-timestamp).

## Schema Credential


```json
{
  "type": "object",
  "properties": {
    "id": {
      "type": "string"
    },
    "databaseId": {
      "$ref": "urn:mirabase:wire#/$defs/DatabaseId"
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
    "secret": {
      "type": "string",
      "description": "Returned exactly once; never logged or listed"
    }
  },
  "required": [
    "id",
    "databaseId",
    "scopes",
    "expiresAt",
    "secret"
  ],
  "additionalProperties": false
}
```


Referenced types: [DatabaseId](wire-schema-reference.md#schema-databaseid), [Scope](wire-schema-reference.md#schema-scope), [Timestamp](wire-schema-reference.md#schema-timestamp).

## Schema Diagnostic


```json
{
  "type": "object",
  "properties": {
    "databaseId": {
      "$ref": "urn:mirabase:wire#/$defs/DatabaseId"
    },
    "generation": {
      "$ref": "urn:mirabase:wire#/$defs/Revision"
    },
    "state": {
      "$ref": "urn:mirabase:wire#/$defs/DatabaseState"
    },
    "openConnections": {
      "type": "integer",
      "minimum": 0,
      "maximum": 9007199254740991
    },
    "activeLeases": {
      "type": "integer",
      "minimum": 0,
      "maximum": 9007199254740991
    },
    "diskBytes": {
      "type": "integer",
      "minimum": 0,
      "maximum": 9007199254740991
    },
    "walBytes": {
      "type": "integer",
      "minimum": 0,
      "maximum": 9007199254740991
    },
    "ftsBytes": {
      "type": "integer",
      "minimum": 0,
      "maximum": 9007199254740991
    },
    "pendingJobs": {
      "type": "integer",
      "minimum": 0,
      "maximum": 9007199254740991
    },
    "schemaRevision": {
      "$ref": "urn:mirabase:wire#/$defs/Revision"
    },
    "retentionFloor": {
      "anyOf": [
        {
          "type": "string"
        },
        {
          "type": "null"
        }
      ]
    },
    "errors": {
      "type": "array",
      "items": {
        "type": "string"
      },
      "maxItems": 100
    }
  },
  "required": [
    "databaseId",
    "generation",
    "state",
    "openConnections",
    "activeLeases",
    "diskBytes",
    "walBytes",
    "ftsBytes",
    "pendingJobs",
    "schemaRevision",
    "retentionFloor",
    "errors"
  ],
  "additionalProperties": false
}
```


Referenced types: [DatabaseId](wire-schema-reference.md#schema-databaseid), [DatabaseState](wire-schema-reference.md#schema-databasestate), [Revision](wire-schema-reference.md#schema-revision).

## Schema SchemaOperation


```json
{
  "oneOf": [
    {
      "type": "object",
      "properties": {
        "kind": {
          "const": "collection.create"
        },
        "objectId": {
          "type": "string"
        },
        "definition": {
          "$ref": "urn:mirabase:wire#/$defs/CollectionDefinition"
        }
      },
      "required": [
        "kind",
        "objectId",
        "definition"
      ],
      "additionalProperties": false
    },
    {
      "type": "object",
      "properties": {
        "kind": {
          "const": "collection.update"
        },
        "objectId": {
          "type": "string"
        },
        "definition": {
          "$ref": "urn:mirabase:wire#/$defs/CollectionDefinition"
        }
      },
      "required": [
        "kind",
        "objectId",
        "definition"
      ],
      "additionalProperties": false
    },
    {
      "type": "object",
      "properties": {
        "kind": {
          "const": "collection.delete"
        },
        "objectId": {
          "type": "string"
        }
      },
      "required": [
        "kind",
        "objectId"
      ],
      "additionalProperties": false
    },
    {
      "type": "object",
      "properties": {
        "kind": {
          "const": "type.define"
        },
        "objectId": {
          "type": "string"
        },
        "definition": {
          "$ref": "urn:mirabase:wire#/$defs/TypeDefinition"
        }
      },
      "required": [
        "kind",
        "objectId",
        "definition"
      ],
      "additionalProperties": false
    },
    {
      "type": "object",
      "properties": {
        "kind": {
          "const": "type.drop"
        },
        "objectId": {
          "type": "string"
        }
      },
      "required": [
        "kind",
        "objectId"
      ],
      "additionalProperties": false
    },
    {
      "type": "object",
      "properties": {
        "kind": {
          "const": "index.create"
        },
        "objectId": {
          "type": "string"
        },
        "definition": {
          "$ref": "urn:mirabase:wire#/$defs/IndexDefinition"
        }
      },
      "required": [
        "kind",
        "objectId",
        "definition"
      ],
      "additionalProperties": false
    },
    {
      "type": "object",
      "properties": {
        "kind": {
          "const": "index.drop"
        },
        "objectId": {
          "type": "string"
        }
      },
      "required": [
        "kind",
        "objectId"
      ],
      "additionalProperties": false
    },
    {
      "type": "object",
      "properties": {
        "kind": {
          "const": "view.define"
        },
        "objectId": {
          "type": "string"
        },
        "definition": {
          "$ref": "urn:mirabase:wire#/$defs/ViewDefinition"
        }
      },
      "required": [
        "kind",
        "objectId",
        "definition"
      ],
      "additionalProperties": false
    },
    {
      "type": "object",
      "properties": {
        "kind": {
          "const": "sequence.define"
        },
        "objectId": {
          "type": "string"
        },
        "definition": {
          "$ref": "urn:mirabase:wire#/$defs/SequenceDefinition"
        }
      },
      "required": [
        "kind",
        "objectId",
        "definition"
      ],
      "additionalProperties": false
    }
  ]
}
```


Referenced types: [CollectionDefinition](wire-schema-reference.md#schema-collectiondefinition), [IndexDefinition](wire-schema-reference.md#schema-indexdefinition), [SequenceDefinition](wire-schema-reference.md#schema-sequencedefinition), [TypeDefinition](wire-schema-reference.md#schema-typedefinition), [ViewDefinition](wire-schema-reference.md#schema-viewdefinition).

## Schema SchemaPlan


```json
{
  "type": "object",
  "properties": {
    "id": {
      "type": "string"
    },
    "databaseId": {
      "$ref": "urn:mirabase:wire#/$defs/DatabaseId"
    },
    "expectedSchemaRevision": {
      "$ref": "urn:mirabase:wire#/$defs/Revision"
    },
    "planHash": {
      "type": "string"
    },
    "operations": {
      "type": "array",
      "items": {
        "$ref": "urn:mirabase:wire#/$defs/SchemaOperation"
      },
      "maxItems": 1000
    },
    "destructiveEffects": {
      "type": "array",
      "items": {
        "type": "string"
      },
      "maxItems": 1000
    },
    "blockers": {
      "type": "array",
      "items": {
        "type": "object",
        "properties": {
          "objectId": {
            "type": "string"
          },
          "code": {
            "type": "string"
          },
          "message": {
            "type": "string"
          }
        },
        "required": [
          "objectId",
          "code",
          "message"
        ],
        "additionalProperties": false
      },
      "maxItems": 1000
    },
    "expiresAt": {
      "$ref": "urn:mirabase:wire#/$defs/Timestamp"
    }
  },
  "required": [
    "id",
    "databaseId",
    "expectedSchemaRevision",
    "planHash",
    "operations",
    "destructiveEffects",
    "blockers",
    "expiresAt"
  ],
  "additionalProperties": false
}
```


Referenced types: [DatabaseId](wire-schema-reference.md#schema-databaseid), [Revision](wire-schema-reference.md#schema-revision), [SchemaOperation](wire-schema-reference.md#schema-schemaoperation), [Timestamp](wire-schema-reference.md#schema-timestamp).

## Schema Template


```json
{
  "type": "object",
  "properties": {
    "id": {
      "type": "string"
    },
    "revision": {
      "$ref": "urn:mirabase:wire#/$defs/Revision"
    },
    "definitionHash": {
      "type": "string"
    },
    "collectionIds": {
      "type": "array",
      "items": {
        "type": "string"
      },
      "maxItems": 10000
    },
    "requiredCapabilities": {
      "type": "array",
      "items": {
        "type": "string"
      },
      "maxItems": 300
    },
    "state": {
      "type": "string",
      "enum": [
        "draft",
        "published"
      ]
    }
  },
  "required": [
    "id",
    "revision",
    "definitionHash",
    "collectionIds",
    "requiredCapabilities",
    "state"
  ],
  "additionalProperties": false
}
```


Referenced types: [Revision](wire-schema-reference.md#schema-revision).

## Schema LiveQuery


```json
{
  "type": "object",
  "properties": {
    "id": {
      "type": "string"
    },
    "databaseId": {
      "$ref": "urn:mirabase:wire#/$defs/DatabaseId"
    },
    "queryId": {
      "type": "string"
    },
    "schemaRevision": {
      "$ref": "urn:mirabase:wire#/$defs/Revision"
    },
    "policyEpoch": {
      "$ref": "urn:mirabase:wire#/$defs/Revision"
    },
    "cursor": {
      "type": "string"
    },
    "streamUrl": {
      "type": "string"
    },
    "expiresAt": {
      "$ref": "urn:mirabase:wire#/$defs/Timestamp"
    },
    "applicationId": {
      "$ref": "urn:mirabase:wire#/$defs/ApplicationId"
    }
  },
  "required": [
    "id",
    "databaseId",
    "queryId",
    "schemaRevision",
    "policyEpoch",
    "cursor",
    "streamUrl",
    "expiresAt"
  ],
  "additionalProperties": false
}
```


Referenced types: [ApplicationId](wire-schema-reference.md#schema-applicationid), [DatabaseId](wire-schema-reference.md#schema-databaseid), [Revision](wire-schema-reference.md#schema-revision), [Timestamp](wire-schema-reference.md#schema-timestamp).

## Schema ImportHash


```json
{
  "type": "string",
  "pattern": "^[a-f0-9]{64}$"
}
```


## Schema ImportCount


```json
{
  "type": "string",
  "pattern": "^(0|[1-9][0-9]*)$"
}
```


## Schema ImportPhase


```json
{
  "type": "string",
  "enum": [
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
    "cutting-over",
    "active",
    "failed",
    "cancelled",
    "rolling-back",
    "rolled-back",
    "recovery-exported",
    "retired"
  ]
}
```


## Schema ImportTenantKey


```json
{
  "oneOf": [
    {
      "type": "object",
      "properties": {
        "kind": {
          "type": "string",
          "enum": [
            "record"
          ]
        },
        "sourceCollectionId": {
          "type": "string"
        },
        "recordId": {
          "type": "string"
        }
      },
      "required": [
        "kind",
        "sourceCollectionId",
        "recordId"
      ],
      "additionalProperties": false
    },
    {
      "type": "object",
      "properties": {
        "kind": {
          "type": "string",
          "enum": [
            "field-value"
          ]
        },
        "sourceCollectionId": {
          "type": "string"
        },
        "fieldId": {
          "type": "string"
        },
        "value": {
          "$ref": "urn:mirabase:wire#/$defs/TypedValue"
        }
      },
      "required": [
        "kind",
        "sourceCollectionId",
        "fieldId",
        "value"
      ],
      "additionalProperties": false
    }
  ]
}
```


Referenced types: [TypedValue](wire-schema-reference.md#schema-typedvalue).

## Schema ImportTenantSource


```json
{
  "oneOf": [
    {
      "type": "object",
      "properties": {
        "kind": {
          "type": "string",
          "enum": [
            "records"
          ]
        },
        "collectionId": {
          "type": "string"
        }
      },
      "required": [
        "kind",
        "collectionId"
      ],
      "additionalProperties": false
    },
    {
      "type": "object",
      "properties": {
        "kind": {
          "type": "string",
          "enum": [
            "distinct-values"
          ]
        },
        "collectionId": {
          "type": "string"
        },
        "fieldId": {
          "type": "string"
        }
      },
      "required": [
        "kind",
        "collectionId",
        "fieldId"
      ],
      "additionalProperties": false
    }
  ]
}
```


## Schema ImportOwnership


```json
{
  "oneOf": [
    {
      "type": "object",
      "properties": {
        "kind": {
          "type": "string",
          "enum": [
            "root"
          ]
        }
      },
      "required": [
        "kind"
      ],
      "additionalProperties": false
    },
    {
      "type": "object",
      "properties": {
        "kind": {
          "type": "string",
          "enum": [
            "field"
          ]
        },
        "fieldId": {
          "type": "string"
        }
      },
      "required": [
        "kind",
        "fieldId"
      ],
      "additionalProperties": false
    },
    {
      "type": "object",
      "properties": {
        "kind": {
          "type": "string",
          "enum": [
            "relation-path"
          ]
        },
        "fieldIds": {
          "type": "array",
          "items": {
            "type": "string"
          },
          "maxItems": 1000,
          "minItems": 1
        }
      },
      "required": [
        "kind",
        "fieldIds"
      ],
      "additionalProperties": false
    },
    {
      "type": "object",
      "properties": {
        "kind": {
          "type": "string",
          "enum": [
            "shared"
          ]
        }
      },
      "required": [
        "kind"
      ],
      "additionalProperties": false
    }
  ]
}
```


## Schema ImportPlacement


```json
{
  "type": "object",
  "properties": {
    "collectionId": {
      "type": "string"
    },
    "placement": {
      "type": "string",
      "enum": [
        "shared",
        "tenant",
        "reference",
        "derived"
      ]
    },
    "ownership": {
      "$ref": "urn:mirabase:wire#/$defs/ImportOwnership"
    },
    "referenceFieldIds": {
      "type": "array",
      "items": {
        "type": "string"
      },
      "maxItems": 1000
    },
    "uniquePolicy": {
      "type": "string",
      "enum": [
        "preserve-global",
        "per-tenant-approved"
      ]
    },
    "adaptationIds": {
      "type": "array",
      "items": {
        "type": "string"
      },
      "maxItems": 1000
    }
  },
  "required": [
    "collectionId",
    "placement",
    "ownership",
    "referenceFieldIds",
    "uniquePolicy",
    "adaptationIds"
  ],
  "additionalProperties": false
}
```


Referenced types: [ImportOwnership](wire-schema-reference.md#schema-importownership).

## Schema ImportMembership


```json
{
  "oneOf": [
    {
      "type": "object",
      "properties": {
        "kind": {
          "type": "string",
          "enum": [
            "tenant-record-rule"
          ]
        },
        "sourceCollectionId": {
          "type": "string"
        },
        "rule": {
          "type": "string",
          "enum": [
            "listRule",
            "viewRule"
          ]
        }
      },
      "required": [
        "kind",
        "sourceCollectionId",
        "rule"
      ],
      "additionalProperties": false
    },
    {
      "type": "object",
      "properties": {
        "kind": {
          "type": "string",
          "enum": [
            "membership-collection"
          ]
        },
        "collectionId": {
          "type": "string"
        },
        "subjectRelationFieldId": {
          "type": "string"
        },
        "tenantFieldId": {
          "type": "string"
        }
      },
      "required": [
        "kind",
        "collectionId",
        "subjectRelationFieldId",
        "tenantFieldId"
      ],
      "additionalProperties": false
    },
    {
      "type": "object",
      "properties": {
        "kind": {
          "type": "string",
          "enum": [
            "owner-relation"
          ]
        },
        "tenantCollectionId": {
          "type": "string"
        },
        "ownerRelationFieldId": {
          "type": "string"
        }
      },
      "required": [
        "kind",
        "tenantCollectionId",
        "ownerRelationFieldId"
      ],
      "additionalProperties": false
    }
  ]
}
```


## Schema ImportAccountPolicy


```json
{
  "type": "object",
  "properties": {
    "sessionPolicy": {
      "type": "string",
      "enum": [
        "preserve-if-compatible",
        "revoke-existing"
      ]
    },
    "operatorPolicy": {
      "type": "string",
      "enum": [
        "retain-destination-superuser",
        "use-source-superuser"
      ]
    }
  },
  "required": [
    "sessionPolicy",
    "operatorPolicy"
  ],
  "additionalProperties": false
}
```


## Schema ImportAccess


```json
{
  "type": "object",
  "properties": {
    "membership": {
      "$ref": "urn:mirabase:wire#/$defs/ImportMembership"
    },
    "membershipScopes": {
      "type": "array",
      "items": {
        "type": "string",
        "enum": [
          "record.read",
          "record.write",
          "files.read",
          "files.write",
          "changes.read",
          "record.sync"
        ]
      },
      "maxItems": 6
    },
    "allowAnonymousTenantRead": {
      "type": "boolean"
    },
    "ruleAdaptationIds": {
      "type": "array",
      "items": {
        "type": "string"
      },
      "maxItems": 1000
    }
  },
  "required": [
    "membership",
    "membershipScopes",
    "allowAnonymousTenantRead",
    "ruleAdaptationIds"
  ],
  "additionalProperties": false
}
```


Referenced types: [ImportMembership](wire-schema-reference.md#schema-importmembership).

## Schema ImportAdaptation


```json
{
  "type": "object",
  "properties": {
    "id": {
      "$ref": "urn:mirabase:wire#/$defs/OpaqueId"
    },
    "kind": {
      "type": "string",
      "enum": [
        "global-relation",
        "qualified-relation",
        "rule",
        "view",
        "index",
        "hook",
        "route",
        "selector",
        "file-url",
        "global-unique"
      ]
    },
    "sourceObjectId": {
      "type": "string"
    },
    "actionId": {
      "type": "string"
    },
    "testFixtureIds": {
      "type": "array",
      "items": {
        "type": "string"
      },
      "maxItems": 1000,
      "minItems": 1
    },
    "expectedDifference": {
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
    "id",
    "kind",
    "sourceObjectId",
    "actionId",
    "testFixtureIds",
    "expectedDifference"
  ],
  "additionalProperties": false
}
```


Referenced types: [OpaqueId](wire-schema-reference.md#schema-opaqueid).

## Schema ImportResolution


```json
{
  "type": "object",
  "properties": {
    "issueId": {
      "$ref": "urn:mirabase:wire#/$defs/OpaqueId"
    },
    "action": {
      "type": "string",
      "enum": [
        "assign-tenant",
        "make-shared",
        "reference-copy",
        "replace-ownership-path",
        "approved-adaptation"
      ]
    },
    "tenantKey": {
      "anyOf": [
        {
          "$ref": "urn:mirabase:wire#/$defs/ImportTenantKey"
        },
        {
          "type": "null"
        }
      ]
    },
    "placementCollectionId": {
      "anyOf": [
        {
          "type": "string"
        },
        {
          "type": "null"
        }
      ]
    },
    "adaptationId": {
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
    "issueId",
    "action",
    "tenantKey",
    "placementCollectionId",
    "adaptationId"
  ],
  "additionalProperties": false
}
```


Referenced types: [ImportTenantKey](wire-schema-reference.md#schema-importtenantkey), [OpaqueId](wire-schema-reference.md#schema-opaqueid).

## Schema ImportPlanDraft


```json
{
  "type": "object",
  "properties": {
    "expectedRevision": {
      "$ref": "urn:mirabase:wire#/$defs/Revision"
    },
    "sourceHash": {
      "$ref": "urn:mirabase:wire#/$defs/ImportHash"
    },
    "snapshotHash": {
      "$ref": "urn:mirabase:wire#/$defs/ImportHash"
    },
    "layout": {
      "type": "string",
      "enum": [
        "shared",
        "tenant"
      ]
    },
    "namespace": {
      "$ref": "urn:mirabase:wire#/$defs/OpaqueId"
    },
    "tenantKind": {
      "anyOf": [
        {
          "type": "string",
          "enum": [
            "tenant",
            "user",
            "agent"
          ]
        },
        {
          "type": "null"
        }
      ]
    },
    "tenantSource": {
      "anyOf": [
        {
          "$ref": "urn:mirabase:wire#/$defs/ImportTenantSource"
        },
        {
          "type": "null"
        }
      ]
    },
    "placements": {
      "type": "array",
      "items": {
        "$ref": "urn:mirabase:wire#/$defs/ImportPlacement"
      },
      "maxItems": 1000,
      "minItems": 1
    },
    "accountPolicy": {
      "$ref": "urn:mirabase:wire#/$defs/ImportAccountPolicy"
    },
    "access": {
      "anyOf": [
        {
          "$ref": "urn:mirabase:wire#/$defs/ImportAccess"
        },
        {
          "type": "null"
        }
      ]
    },
    "selector": {
      "type": "object",
      "properties": {
        "kind": {
          "type": "string",
          "enum": [
            "none",
            "mirabase-header",
            "mapped-existing-selector"
          ]
        },
        "source": {
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
        "kind",
        "source"
      ],
      "additionalProperties": false
    },
    "profile": {
      "$ref": "urn:mirabase:wire#/$defs/Profile"
    },
    "sourceFileStoreRef": {
      "anyOf": [
        {
          "type": "string"
        },
        {
          "type": "null"
        }
      ]
    },
    "settingsKeyRef": {
      "anyOf": [
        {
          "type": "string"
        },
        {
          "type": "null"
        }
      ]
    },
    "resolutions": {
      "type": "array",
      "items": {
        "$ref": "urn:mirabase:wire#/$defs/ImportResolution"
      },
      "maxItems": 100000
    },
    "adaptations": {
      "type": "array",
      "items": {
        "$ref": "urn:mirabase:wire#/$defs/ImportAdaptation"
      },
      "maxItems": 10000
    },
    "sideEffects": {
      "type": "string",
      "enum": [
        "suppressed-during-import"
      ]
    }
  },
  "required": [
    "expectedRevision",
    "sourceHash",
    "snapshotHash",
    "layout",
    "namespace",
    "tenantKind",
    "tenantSource",
    "placements",
    "accountPolicy",
    "access",
    "selector",
    "profile",
    "sourceFileStoreRef",
    "settingsKeyRef",
    "resolutions",
    "adaptations",
    "sideEffects"
  ],
  "additionalProperties": false
}
```


Referenced types: [ImportAccess](wire-schema-reference.md#schema-importaccess), [ImportAccountPolicy](wire-schema-reference.md#schema-importaccountpolicy), [ImportAdaptation](wire-schema-reference.md#schema-importadaptation), [ImportHash](wire-schema-reference.md#schema-importhash), [ImportPlacement](wire-schema-reference.md#schema-importplacement), [ImportResolution](wire-schema-reference.md#schema-importresolution), [ImportTenantSource](wire-schema-reference.md#schema-importtenantsource), [OpaqueId](wire-schema-reference.md#schema-opaqueid), [Profile](wire-schema-reference.md#schema-profile), [Revision](wire-schema-reference.md#schema-revision).

## Schema ImportPlan


```json
{
  "type": "object",
  "properties": {
    "id": {
      "$ref": "urn:mirabase:wire#/$defs/OpaqueId"
    },
    "sessionId": {
      "$ref": "urn:mirabase:wire#/$defs/OpaqueId"
    },
    "revision": {
      "$ref": "urn:mirabase:wire#/$defs/Revision"
    },
    "planHash": {
      "$ref": "urn:mirabase:wire#/$defs/ImportHash"
    },
    "draft": {
      "$ref": "urn:mirabase:wire#/$defs/ImportPlanDraft"
    },
    "status": {
      "type": "string",
      "enum": [
        "draft",
        "previewed",
        "staged",
        "validated",
        "superseded"
      ]
    }
  },
  "required": [
    "id",
    "sessionId",
    "revision",
    "planHash",
    "draft",
    "status"
  ],
  "additionalProperties": false
}
```


Referenced types: [ImportHash](wire-schema-reference.md#schema-importhash), [ImportPlanDraft](wire-schema-reference.md#schema-importplandraft), [OpaqueId](wire-schema-reference.md#schema-opaqueid), [Revision](wire-schema-reference.md#schema-revision).

## Schema ImportSession


```json
{
  "type": "object",
  "properties": {
    "id": {
      "$ref": "urn:mirabase:wire#/$defs/OpaqueId"
    },
    "revision": {
      "$ref": "urn:mirabase:wire#/$defs/Revision"
    },
    "phase": {
      "$ref": "urn:mirabase:wire#/$defs/ImportPhase"
    },
    "sourceKind": {
      "type": "string",
      "enum": [
        "pocketbase-backup",
        "pocketbase-directory"
      ]
    },
    "sourceHash": {
      "anyOf": [
        {
          "$ref": "urn:mirabase:wire#/$defs/ImportHash"
        },
        {
          "type": "null"
        }
      ]
    },
    "snapshotHash": {
      "anyOf": [
        {
          "$ref": "urn:mirabase:wire#/$defs/ImportHash"
        },
        {
          "type": "null"
        }
      ]
    },
    "planHash": {
      "anyOf": [
        {
          "$ref": "urn:mirabase:wire#/$defs/ImportHash"
        },
        {
          "type": "null"
        }
      ]
    },
    "previewHash": {
      "anyOf": [
        {
          "$ref": "urn:mirabase:wire#/$defs/ImportHash"
        },
        {
          "type": "null"
        }
      ]
    },
    "reportHash": {
      "anyOf": [
        {
          "$ref": "urn:mirabase:wire#/$defs/ImportHash"
        },
        {
          "type": "null"
        }
      ]
    },
    "targetMode": {
      "type": "string",
      "enum": [
        "new-application",
        "replace-application",
        "append-application"
      ]
    },
    "expectedPrimaryGeneration": {
      "$ref": "urn:mirabase:wire#/$defs/Revision"
    },
    "currentJobId": {
      "anyOf": [
        {
          "type": "string"
        },
        {
          "type": "null"
        }
      ]
    },
    "lastErrorCode": {
      "anyOf": [
        {
          "type": "string"
        },
        {
          "type": "null"
        }
      ]
    },
    "created": {
      "$ref": "urn:mirabase:wire#/$defs/Timestamp"
    },
    "updated": {
      "$ref": "urn:mirabase:wire#/$defs/Timestamp"
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
    "destinationApplicationId": {
      "$ref": "urn:mirabase:wire#/$defs/ApplicationId"
    },
    "sourceId": {
      "$ref": "urn:mirabase:wire#/$defs/OpaqueId"
    },
    "batchId": {
      "$ref": "urn:mirabase:wire#/$defs/OpaqueId"
    },
    "expectedInstanceRevision": {
      "$ref": "urn:mirabase:wire#/$defs/Revision"
    },
    "originId": {
      "$ref": "urn:mirabase:wire#/$defs/OpaqueId"
    }
  },
  "required": [
    "id",
    "revision",
    "phase",
    "sourceKind",
    "sourceHash",
    "snapshotHash",
    "planHash",
    "previewHash",
    "reportHash",
    "targetMode",
    "expectedPrimaryGeneration",
    "currentJobId",
    "lastErrorCode",
    "created",
    "updated",
    "expiresAt"
  ],
  "additionalProperties": false
}
```


Referenced types: [ApplicationId](wire-schema-reference.md#schema-applicationid), [ImportHash](wire-schema-reference.md#schema-importhash), [ImportPhase](wire-schema-reference.md#schema-importphase), [OpaqueId](wire-schema-reference.md#schema-opaqueid), [Revision](wire-schema-reference.md#schema-revision), [Timestamp](wire-schema-reference.md#schema-timestamp).

## Schema ImportCreate


```json
{
  "type": "object",
  "properties": {
    "sourceKind": {
      "type": "string",
      "enum": [
        "pocketbase-backup",
        "pocketbase-directory"
      ]
    },
    "sourceBytes": {
      "type": "integer",
      "minimum": 1,
      "maximum": 9007199254740991
    },
    "sourceSha256": {
      "$ref": "urn:mirabase:wire#/$defs/ImportHash"
    },
    "targetMode": {
      "type": "string",
      "enum": [
        "new-application",
        "replace-application",
        "append-application"
      ]
    },
    "expectedPrimaryGeneration": {
      "$ref": "urn:mirabase:wire#/$defs/Revision"
    },
    "sourceLabel": {
      "type": "string"
    },
    "destinationApplicationId": {
      "$ref": "urn:mirabase:wire#/$defs/ApplicationId"
    },
    "sourceId": {
      "$ref": "urn:mirabase:wire#/$defs/OpaqueId"
    },
    "batchId": {
      "$ref": "urn:mirabase:wire#/$defs/OpaqueId"
    },
    "expectedInstanceRevision": {
      "$ref": "urn:mirabase:wire#/$defs/Revision"
    },
    "originId": {
      "$ref": "urn:mirabase:wire#/$defs/OpaqueId"
    },
    "repeatPolicy": {
      "type": "string",
      "enum": [
        "resume-existing",
        "create-independent-copy",
        "replace-matching-origin"
      ]
    }
  },
  "required": [
    "sourceKind",
    "sourceBytes",
    "sourceSha256",
    "targetMode",
    "expectedPrimaryGeneration",
    "sourceLabel"
  ],
  "additionalProperties": false,
  "description": "Append requires expectedInstanceRevision and originId and reserves a server-generated destination application at generation 1. A supplied destination ID must reference an authorized server reservation. New/replace guard the selected application. No implicit account merge.",
  "allOf": [
    {
      "if": {
        "properties": {
          "targetMode": {
            "const": "append-application"
          }
        }
      },
      "then": {
        "required": [
          "expectedInstanceRevision",
          "originId"
        ],
        "properties": {
          "expectedPrimaryGeneration": {
            "const": "1"
          }
        }
      }
    }
  ]
}
```


Referenced types: [ApplicationId](wire-schema-reference.md#schema-applicationid), [ImportHash](wire-schema-reference.md#schema-importhash), [OpaqueId](wire-schema-reference.md#schema-opaqueid), [Revision](wire-schema-reference.md#schema-revision).

## Schema ImportUpload


```json
{
  "type": "object",
  "properties": {
    "id": {
      "$ref": "urn:mirabase:wire#/$defs/OpaqueId"
    },
    "receivedBytes": {
      "$ref": "urn:mirabase:wire#/$defs/ImportCount"
    },
    "complete": {
      "type": "boolean"
    },
    "sourceHash": {
      "anyOf": [
        {
          "$ref": "urn:mirabase:wire#/$defs/ImportHash"
        },
        {
          "type": "null"
        }
      ]
    },
    "revision": {
      "$ref": "urn:mirabase:wire#/$defs/Revision"
    }
  },
  "required": [
    "id",
    "receivedBytes",
    "complete",
    "sourceHash",
    "revision"
  ],
  "additionalProperties": false
}
```


Referenced types: [ImportCount](wire-schema-reference.md#schema-importcount), [ImportHash](wire-schema-reference.md#schema-importhash), [OpaqueId](wire-schema-reference.md#schema-opaqueid), [Revision](wire-schema-reference.md#schema-revision).

## Schema ImportCollectionInventory


```json
{
  "type": "object",
  "properties": {
    "collectionId": {
      "type": "string"
    },
    "name": {
      "type": "string"
    },
    "type": {
      "type": "string",
      "enum": [
        "base",
        "auth",
        "view"
      ]
    },
    "recordCount": {
      "$ref": "urn:mirabase:wire#/$defs/ImportCount"
    },
    "fileCount": {
      "$ref": "urn:mirabase:wire#/$defs/ImportCount"
    },
    "fileBytes": {
      "$ref": "urn:mirabase:wire#/$defs/ImportCount"
    },
    "schemaHash": {
      "$ref": "urn:mirabase:wire#/$defs/ImportHash"
    },
    "relationFieldIds": {
      "type": "array",
      "items": {
        "type": "string"
      },
      "maxItems": 1000
    },
    "hasCustomCode": {
      "type": "boolean"
    }
  },
  "required": [
    "collectionId",
    "name",
    "type",
    "recordCount",
    "fileCount",
    "fileBytes",
    "schemaHash",
    "relationFieldIds",
    "hasCustomCode"
  ],
  "additionalProperties": false
}
```


Referenced types: [ImportCount](wire-schema-reference.md#schema-importcount), [ImportHash](wire-schema-reference.md#schema-importhash).

## Schema ImportInventoryPage


```json
{
  "type": "object",
  "properties": {
    "items": {
      "type": "array",
      "items": {
        "$ref": "urn:mirabase:wire#/$defs/ImportCollectionInventory"
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
      "$ref": "urn:mirabase:wire#/$defs/ImportCount"
    },
    "totalPages": {
      "$ref": "urn:mirabase:wire#/$defs/ImportCount"
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


Referenced types: [ImportCollectionInventory](wire-schema-reference.md#schema-importcollectioninventory), [ImportCount](wire-schema-reference.md#schema-importcount).

## Schema ImportIssue


```json
{
  "type": "object",
  "properties": {
    "id": {
      "$ref": "urn:mirabase:wire#/$defs/OpaqueId"
    },
    "severity": {
      "type": "string",
      "enum": [
        "error",
        "warning",
        "info"
      ]
    },
    "code": {
      "type": "string"
    },
    "collectionId": {
      "anyOf": [
        {
          "type": "string"
        },
        {
          "type": "null"
        }
      ]
    },
    "recordId": {
      "anyOf": [
        {
          "type": "string"
        },
        {
          "type": "null"
        }
      ]
    },
    "fieldId": {
      "anyOf": [
        {
          "type": "string"
        },
        {
          "type": "null"
        }
      ]
    },
    "tenantKey": {
      "anyOf": [
        {
          "$ref": "urn:mirabase:wire#/$defs/ImportTenantKey"
        },
        {
          "type": "null"
        }
      ]
    },
    "message": {
      "type": "string"
    },
    "resolutionActions": {
      "type": "array",
      "items": {
        "type": "string",
        "enum": [
          "assign-tenant",
          "make-shared",
          "reference-copy",
          "replace-ownership-path",
          "approved-adaptation",
          "supply-source-artifact",
          "repair-source"
        ]
      },
      "maxItems": 10
    },
    "resolved": {
      "type": "boolean"
    }
  },
  "required": [
    "id",
    "severity",
    "code",
    "collectionId",
    "recordId",
    "fieldId",
    "tenantKey",
    "message",
    "resolutionActions",
    "resolved"
  ],
  "additionalProperties": false
}
```


Referenced types: [ImportTenantKey](wire-schema-reference.md#schema-importtenantkey), [OpaqueId](wire-schema-reference.md#schema-opaqueid).

## Schema ImportIssuePage


```json
{
  "type": "object",
  "properties": {
    "items": {
      "type": "array",
      "items": {
        "$ref": "urn:mirabase:wire#/$defs/ImportIssue"
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
      "$ref": "urn:mirabase:wire#/$defs/ImportCount"
    },
    "totalPages": {
      "$ref": "urn:mirabase:wire#/$defs/ImportCount"
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


Referenced types: [ImportCount](wire-schema-reference.md#schema-importcount), [ImportIssue](wire-schema-reference.md#schema-importissue).

## Schema ImportTenantPreview


```json
{
  "type": "object",
  "properties": {
    "tenantKey": {
      "$ref": "urn:mirabase:wire#/$defs/ImportTenantKey"
    },
    "databaseId": {
      "$ref": "urn:mirabase:wire#/$defs/DatabaseId"
    },
    "recordCount": {
      "$ref": "urn:mirabase:wire#/$defs/ImportCount"
    },
    "fileCount": {
      "$ref": "urn:mirabase:wire#/$defs/ImportCount"
    },
    "fileBytes": {
      "$ref": "urn:mirabase:wire#/$defs/ImportCount"
    },
    "memberCount": {
      "$ref": "urn:mirabase:wire#/$defs/ImportCount"
    },
    "issueCount": {
      "$ref": "urn:mirabase:wire#/$defs/ImportCount"
    },
    "assignmentHash": {
      "$ref": "urn:mirabase:wire#/$defs/ImportHash"
    }
  },
  "required": [
    "tenantKey",
    "databaseId",
    "recordCount",
    "fileCount",
    "fileBytes",
    "memberCount",
    "issueCount",
    "assignmentHash"
  ],
  "additionalProperties": false
}
```


Referenced types: [DatabaseId](wire-schema-reference.md#schema-databaseid), [ImportCount](wire-schema-reference.md#schema-importcount), [ImportHash](wire-schema-reference.md#schema-importhash), [ImportTenantKey](wire-schema-reference.md#schema-importtenantkey).

## Schema ImportTenantPreviewPage


```json
{
  "type": "object",
  "properties": {
    "items": {
      "type": "array",
      "items": {
        "$ref": "urn:mirabase:wire#/$defs/ImportTenantPreview"
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
      "$ref": "urn:mirabase:wire#/$defs/ImportCount"
    },
    "totalPages": {
      "$ref": "urn:mirabase:wire#/$defs/ImportCount"
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


Referenced types: [ImportCount](wire-schema-reference.md#schema-importcount), [ImportTenantPreview](wire-schema-reference.md#schema-importtenantpreview).

## Schema ImportPreview


```json
{
  "type": "object",
  "properties": {
    "sessionId": {
      "$ref": "urn:mirabase:wire#/$defs/OpaqueId"
    },
    "planHash": {
      "$ref": "urn:mirabase:wire#/$defs/ImportHash"
    },
    "previewHash": {
      "$ref": "urn:mirabase:wire#/$defs/ImportHash"
    },
    "tenantCount": {
      "$ref": "urn:mirabase:wire#/$defs/ImportCount"
    },
    "sourceRecords": {
      "$ref": "urn:mirabase:wire#/$defs/ImportCount"
    },
    "assignedRecords": {
      "$ref": "urn:mirabase:wire#/$defs/ImportCount"
    },
    "referenceCopies": {
      "$ref": "urn:mirabase:wire#/$defs/ImportCount"
    },
    "sharedRecords": {
      "$ref": "urn:mirabase:wire#/$defs/ImportCount"
    },
    "unassignedRecords": {
      "$ref": "urn:mirabase:wire#/$defs/ImportCount"
    },
    "conflictingRecords": {
      "$ref": "urn:mirabase:wire#/$defs/ImportCount"
    },
    "requiredFiles": {
      "$ref": "urn:mirabase:wire#/$defs/ImportCount"
    },
    "missingFiles": {
      "$ref": "urn:mirabase:wire#/$defs/ImportCount"
    },
    "requiredBytes": {
      "$ref": "urn:mirabase:wire#/$defs/ImportCount"
    },
    "unresolvedErrors": {
      "$ref": "urn:mirabase:wire#/$defs/ImportCount"
    },
    "compatibilityActionCount": {
      "$ref": "urn:mirabase:wire#/$defs/ImportCount"
    },
    "complete": {
      "type": "boolean"
    }
  },
  "required": [
    "sessionId",
    "planHash",
    "previewHash",
    "tenantCount",
    "sourceRecords",
    "assignedRecords",
    "referenceCopies",
    "sharedRecords",
    "unassignedRecords",
    "conflictingRecords",
    "requiredFiles",
    "missingFiles",
    "requiredBytes",
    "unresolvedErrors",
    "compatibilityActionCount",
    "complete"
  ],
  "additionalProperties": false
}
```


Referenced types: [ImportCount](wire-schema-reference.md#schema-importcount), [ImportHash](wire-schema-reference.md#schema-importhash), [OpaqueId](wire-schema-reference.md#schema-opaqueid).

## Schema ImportValidationCase


```json
{
  "type": "object",
  "properties": {
    "id": {
      "type": "string"
    },
    "kind": {
      "type": "string",
      "enum": [
        "row-placement",
        "value-hash",
        "file-hash",
        "relation",
        "integrity",
        "auth",
        "rule",
        "routing",
        "hook",
        "recovery"
      ]
    },
    "status": {
      "type": "string",
      "enum": [
        "passed",
        "failed",
        "not-run"
      ]
    },
    "artifactHash": {
      "anyOf": [
        {
          "$ref": "urn:mirabase:wire#/$defs/ImportHash"
        },
        {
          "type": "null"
        }
      ]
    },
    "message": {
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
    "id",
    "kind",
    "status",
    "artifactHash",
    "message"
  ],
  "additionalProperties": false
}
```


Referenced types: [ImportHash](wire-schema-reference.md#schema-importhash).

## Schema ImportValidationCasePage


```json
{
  "type": "object",
  "properties": {
    "items": {
      "type": "array",
      "items": {
        "$ref": "urn:mirabase:wire#/$defs/ImportValidationCase"
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
      "$ref": "urn:mirabase:wire#/$defs/ImportCount"
    },
    "totalPages": {
      "$ref": "urn:mirabase:wire#/$defs/ImportCount"
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


Referenced types: [ImportCount](wire-schema-reference.md#schema-importcount), [ImportValidationCase](wire-schema-reference.md#schema-importvalidationcase).

## Schema ImportValidation


```json
{
  "type": "object",
  "properties": {
    "sessionId": {
      "$ref": "urn:mirabase:wire#/$defs/OpaqueId"
    },
    "sourceHash": {
      "$ref": "urn:mirabase:wire#/$defs/ImportHash"
    },
    "planHash": {
      "$ref": "urn:mirabase:wire#/$defs/ImportHash"
    },
    "previewHash": {
      "$ref": "urn:mirabase:wire#/$defs/ImportHash"
    },
    "reportHash": {
      "$ref": "urn:mirabase:wire#/$defs/ImportHash"
    },
    "stagedManifestHash": {
      "$ref": "urn:mirabase:wire#/$defs/ImportHash"
    },
    "expectedPrimaryGeneration": {
      "$ref": "urn:mirabase:wire#/$defs/Revision"
    },
    "caseCount": {
      "$ref": "urn:mirabase:wire#/$defs/ImportCount"
    },
    "passedCases": {
      "$ref": "urn:mirabase:wire#/$defs/ImportCount"
    },
    "failedCases": {
      "$ref": "urn:mirabase:wire#/$defs/ImportCount"
    },
    "notRunCases": {
      "$ref": "urn:mirabase:wire#/$defs/ImportCount"
    },
    "artifactRef": {
      "$ref": "urn:mirabase:wire#/$defs/OpaqueId"
    },
    "unresolvedErrors": {
      "$ref": "urn:mirabase:wire#/$defs/ImportCount"
    },
    "sourceRecords": {
      "$ref": "urn:mirabase:wire#/$defs/ImportCount"
    },
    "accountedRecords": {
      "$ref": "urn:mirabase:wire#/$defs/ImportCount"
    },
    "requiredFiles": {
      "$ref": "urn:mirabase:wire#/$defs/ImportCount"
    },
    "verifiedFiles": {
      "$ref": "urn:mirabase:wire#/$defs/ImportCount"
    },
    "unexplainedExtraRecords": {
      "$ref": "urn:mirabase:wire#/$defs/ImportCount"
    },
    "operatorAccessVerified": {
      "type": "boolean"
    },
    "passed": {
      "type": "boolean"
    },
    "created": {
      "$ref": "urn:mirabase:wire#/$defs/Timestamp"
    }
  },
  "required": [
    "sessionId",
    "sourceHash",
    "planHash",
    "previewHash",
    "reportHash",
    "stagedManifestHash",
    "expectedPrimaryGeneration",
    "caseCount",
    "passedCases",
    "failedCases",
    "notRunCases",
    "artifactRef",
    "unresolvedErrors",
    "sourceRecords",
    "accountedRecords",
    "requiredFiles",
    "verifiedFiles",
    "unexplainedExtraRecords",
    "operatorAccessVerified",
    "passed",
    "created"
  ],
  "additionalProperties": false
}
```


Referenced types: [ImportCount](wire-schema-reference.md#schema-importcount), [ImportHash](wire-schema-reference.md#schema-importhash), [OpaqueId](wire-schema-reference.md#schema-opaqueid), [Revision](wire-schema-reference.md#schema-revision), [Timestamp](wire-schema-reference.md#schema-timestamp).

## Schema ImportGuard


```json
{
  "type": "object",
  "properties": {
    "expectedRevision": {
      "$ref": "urn:mirabase:wire#/$defs/Revision"
    },
    "planHash": {
      "$ref": "urn:mirabase:wire#/$defs/ImportHash"
    }
  },
  "required": [
    "expectedRevision",
    "planHash"
  ],
  "additionalProperties": false
}
```


Referenced types: [ImportHash](wire-schema-reference.md#schema-importhash), [Revision](wire-schema-reference.md#schema-revision).

## Schema ImportSessionGuard


```json
{
  "type": "object",
  "properties": {
    "expectedRevision": {
      "$ref": "urn:mirabase:wire#/$defs/Revision"
    },
    "planHash": {
      "anyOf": [
        {
          "$ref": "urn:mirabase:wire#/$defs/ImportHash"
        },
        {
          "type": "null"
        }
      ]
    }
  },
  "required": [
    "expectedRevision",
    "planHash"
  ],
  "additionalProperties": false
}
```


Referenced types: [ImportHash](wire-schema-reference.md#schema-importhash), [Revision](wire-schema-reference.md#schema-revision).

## Schema ImportCutover


```json
{
  "type": "object",
  "properties": {
    "expectedRevision": {
      "$ref": "urn:mirabase:wire#/$defs/Revision"
    },
    "sourceHash": {
      "$ref": "urn:mirabase:wire#/$defs/ImportHash"
    },
    "planHash": {
      "$ref": "urn:mirabase:wire#/$defs/ImportHash"
    },
    "previewHash": {
      "$ref": "urn:mirabase:wire#/$defs/ImportHash"
    },
    "reportHash": {
      "$ref": "urn:mirabase:wire#/$defs/ImportHash"
    },
    "expectedPrimaryGeneration": {
      "$ref": "urn:mirabase:wire#/$defs/Revision"
    },
    "destinationBackupId": {
      "anyOf": [
        {
          "type": "string"
        },
        {
          "type": "null"
        }
      ]
    },
    "sourceWritePolicy": {
      "type": "string",
      "enum": [
        "stopped-or-readonly-since-capture",
        "historical-snapshot"
      ]
    },
    "expectedInstanceRevision": {
      "$ref": "urn:mirabase:wire#/$defs/Revision"
    },
    "destinationApplicationId": {
      "$ref": "urn:mirabase:wire#/$defs/ApplicationId"
    }
  },
  "required": [
    "expectedRevision",
    "sourceHash",
    "planHash",
    "previewHash",
    "reportHash",
    "expectedPrimaryGeneration",
    "destinationBackupId",
    "sourceWritePolicy"
  ],
  "additionalProperties": false
}
```


Referenced types: [ApplicationId](wire-schema-reference.md#schema-applicationid), [ImportHash](wire-schema-reference.md#schema-importhash), [Revision](wire-schema-reference.md#schema-revision).

## Schema ImportRecovery


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
    "action": {
      "type": "string",
      "enum": [
        "rollback-before-new-writes",
        "export-current-and-preserve"
      ]
    },
    "expectedActiveGeneration": {
      "$ref": "urn:mirabase:wire#/$defs/Revision"
    }
  },
  "required": [
    "expectedRevision",
    "planHash",
    "action",
    "expectedActiveGeneration"
  ],
  "additionalProperties": false
}
```


Referenced types: [ImportHash](wire-schema-reference.md#schema-importhash), [Revision](wire-schema-reference.md#schema-revision).

## Schema ApplicationId


```json
{
  "type": "string",
  "pattern": "^(default|[a-f0-9]{32})$"
}
```


## Schema ApplicationState


```json
{
  "type": "string",
  "enum": [
    "provisioning",
    "ready",
    "maintenance",
    "suspended",
    "restoring",
    "deleting",
    "quarantined",
    "tombstoned",
    "purged",
    "failed"
  ]
}
```


## Schema StorageLayout


```json
{
  "type": "string",
  "enum": [
    "shared",
    "user",
    "tenant",
    "agent"
  ]
}
```


## Schema ApplicationSubject


```json
{
  "type": "object",
  "properties": {
    "applicationId": {
      "$ref": "urn:mirabase:wire#/$defs/ApplicationId"
    },
    "collectionId": {
      "type": "string"
    },
    "recordId": {
      "type": "string"
    }
  },
  "required": [
    "applicationId",
    "collectionId",
    "recordId"
  ],
  "additionalProperties": false
}
```


Referenced types: [ApplicationId](wire-schema-reference.md#schema-applicationid).

## Schema Application


```json
{
  "type": "object",
  "properties": {
    "id": {
      "$ref": "urn:mirabase:wire#/$defs/ApplicationId"
    },
    "generation": {
      "$ref": "urn:mirabase:wire#/$defs/Revision"
    },
    "revision": {
      "$ref": "urn:mirabase:wire#/$defs/Revision"
    },
    "displayName": {
      "type": "string"
    },
    "state": {
      "$ref": "urn:mirabase:wire#/$defs/ApplicationState"
    },
    "layout": {
      "$ref": "urn:mirabase:wire#/$defs/StorageLayout"
    },
    "basePath": {
      "type": "string"
    },
    "artifactHash": {
      "anyOf": [
        {
          "$ref": "urn:mirabase:wire#/$defs/ImportHash"
        },
        {
          "type": "null"
        }
      ]
    },
    "lastErrorCode": {
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
    "id",
    "generation",
    "revision",
    "displayName",
    "state",
    "layout",
    "basePath",
    "artifactHash",
    "lastErrorCode"
  ],
  "additionalProperties": false
}
```


Referenced types: [ApplicationId](wire-schema-reference.md#schema-applicationid), [ApplicationState](wire-schema-reference.md#schema-applicationstate), [ImportHash](wire-schema-reference.md#schema-importhash), [Revision](wire-schema-reference.md#schema-revision), [StorageLayout](wire-schema-reference.md#schema-storagelayout).

## Schema ApplicationPage


```json
{
  "type": "object",
  "properties": {
    "items": {
      "type": "array",
      "items": {
        "$ref": "urn:mirabase:wire#/$defs/Application"
      },
      "maxItems": 100
    },
    "page": {
      "type": "integer",
      "minimum": 1,
      "maximum": 9007199254740991
    },
    "perPage": {
      "type": "integer",
      "minimum": 1,
      "maximum": 100
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


Referenced types: [Application](wire-schema-reference.md#schema-application).

## Schema ApplicationCreate


```json
{
  "type": "object",
  "properties": {
    "displayName": {
      "type": "string"
    },
    "layout": {
      "const": "shared"
    },
    "profile": {
      "$ref": "urn:mirabase:wire#/$defs/Profile"
    },
    "artifactRef": {
      "anyOf": [
        {
          "$ref": "urn:mirabase:wire#/$defs/OpaqueId"
        },
        {
          "type": "null"
        }
      ]
    },
    "expectedInstanceRevision": {
      "$ref": "urn:mirabase:wire#/$defs/Revision"
    },
    "adminPolicy": {
      "const": "setup-token"
    }
  },
  "required": [
    "displayName",
    "layout",
    "profile",
    "artifactRef",
    "expectedInstanceRevision",
    "adminPolicy"
  ],
  "additionalProperties": false
}
```


Referenced types: [OpaqueId](wire-schema-reference.md#schema-opaqueid), [Profile](wire-schema-reference.md#schema-profile), [Revision](wire-schema-reference.md#schema-revision).

## Schema ApplicationGuard


```json
{
  "type": "object",
  "properties": {
    "expectedGeneration": {
      "$ref": "urn:mirabase:wire#/$defs/Revision"
    },
    "expectedRevision": {
      "$ref": "urn:mirabase:wire#/$defs/Revision"
    }
  },
  "required": [
    "expectedGeneration",
    "expectedRevision"
  ],
  "additionalProperties": false
}
```


Referenced types: [Revision](wire-schema-reference.md#schema-revision).

## Schema ApplicationStateChange


```json
{
  "type": "object",
  "properties": {
    "expectedGeneration": {
      "$ref": "urn:mirabase:wire#/$defs/Revision"
    },
    "expectedRevision": {
      "$ref": "urn:mirabase:wire#/$defs/Revision"
    },
    "action": {
      "type": "string",
      "enum": [
        "maintenance",
        "suspend",
        "resume"
      ]
    }
  },
  "required": [
    "expectedGeneration",
    "expectedRevision",
    "action"
  ],
  "additionalProperties": false
}
```


Referenced types: [Revision](wire-schema-reference.md#schema-revision).

## Schema ApplicationUpdate


```json
{
  "type": "object",
  "properties": {
    "expectedGeneration": {
      "$ref": "urn:mirabase:wire#/$defs/Revision"
    },
    "expectedRevision": {
      "$ref": "urn:mirabase:wire#/$defs/Revision"
    },
    "displayName": {
      "type": "string"
    },
    "hosts": {
      "type": "array",
      "items": {
        "type": "string"
      },
      "maxItems": 100
    },
    "publicUrl": {
      "type": "string"
    }
  },
  "required": [
    "expectedGeneration",
    "expectedRevision",
    "displayName",
    "hosts",
    "publicUrl"
  ],
  "additionalProperties": false
}
```


Referenced types: [Revision](wire-schema-reference.md#schema-revision).

## Schema ApplicationBackup


```json
{
  "type": "object",
  "properties": {
    "expectedGeneration": {
      "$ref": "urn:mirabase:wire#/$defs/Revision"
    },
    "includeLocalFiles": {
      "type": "boolean"
    },
    "remoteFilePolicy": {
      "type": "string",
      "enum": [
        "verified-export",
        "versioned-manifest",
        "explicit-exclusion"
      ]
    },
    "destinationRef": {
      "$ref": "urn:mirabase:wire#/$defs/OpaqueId"
    }
  },
  "required": [
    "expectedGeneration",
    "includeLocalFiles",
    "remoteFilePolicy",
    "destinationRef"
  ],
  "additionalProperties": false
}
```


Referenced types: [OpaqueId](wire-schema-reference.md#schema-opaqueid), [Revision](wire-schema-reference.md#schema-revision).

## Schema ApplicationRestore


```json
{
  "type": "object",
  "properties": {
    "archiveId": {
      "$ref": "urn:mirabase:wire#/$defs/OpaqueId"
    },
    "expectedGeneration": {
      "$ref": "urn:mirabase:wire#/$defs/Revision"
    },
    "dryRun": {
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
    "authorityPolicy",
    "subjects"
  ],
  "additionalProperties": false
}
```


Referenced types: [OpaqueId](wire-schema-reference.md#schema-opaqueid), [Revision](wire-schema-reference.md#schema-revision), [Subject](wire-schema-reference.md#schema-subject).

## Schema BootstrapToken


```json
{
  "type": "object",
  "properties": {
    "applicationId": {
      "$ref": "urn:mirabase:wire#/$defs/ApplicationId"
    },
    "token": {
      "type": "string"
    },
    "expiresAt": {
      "$ref": "urn:mirabase:wire#/$defs/Timestamp"
    }
  },
  "required": [
    "applicationId",
    "token",
    "expiresAt"
  ],
  "additionalProperties": false
}
```


Referenced types: [ApplicationId](wire-schema-reference.md#schema-applicationid), [Timestamp](wire-schema-reference.md#schema-timestamp).

## Schema ImportBatchSource


```json
{
  "type": "object",
  "properties": {
    "sourceId": {
      "$ref": "urn:mirabase:wire#/$defs/OpaqueId"
    },
    "sessionId": {
      "$ref": "urn:mirabase:wire#/$defs/OpaqueId"
    },
    "originId": {
      "$ref": "urn:mirabase:wire#/$defs/OpaqueId"
    },
    "destinationApplicationId": {
      "$ref": "urn:mirabase:wire#/$defs/ApplicationId"
    },
    "sourceHash": {
      "$ref": "urn:mirabase:wire#/$defs/ImportHash"
    },
    "repeatPolicy": {
      "type": "string",
      "enum": [
        "resume-existing",
        "create-independent-copy",
        "replace-matching-origin"
      ]
    }
  },
  "required": [
    "sourceId",
    "sessionId",
    "originId",
    "destinationApplicationId",
    "sourceHash",
    "repeatPolicy"
  ],
  "additionalProperties": false
}
```


Referenced types: [ApplicationId](wire-schema-reference.md#schema-applicationid), [ImportHash](wire-schema-reference.md#schema-importhash), [OpaqueId](wire-schema-reference.md#schema-opaqueid).

## Schema ImportBatchCreate


```json
{
  "type": "object",
  "properties": {
    "expectedInstanceRevision": {
      "$ref": "urn:mirabase:wire#/$defs/Revision"
    },
    "activationPolicy": {
      "type": "string",
      "enum": [
        "per-source-after-validation"
      ]
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
    "expectedInstanceRevision",
    "activationPolicy",
    "sources"
  ],
  "additionalProperties": false
}
```


Referenced types: [ImportBatchSourceInput](wire-schema-reference.md#schema-importbatchsourceinput), [Revision](wire-schema-reference.md#schema-revision).

## Schema ImportBatch


```json
{
  "type": "object",
  "properties": {
    "id": {
      "$ref": "urn:mirabase:wire#/$defs/OpaqueId"
    },
    "revision": {
      "$ref": "urn:mirabase:wire#/$defs/Revision"
    },
    "state": {
      "type": "string",
      "enum": [
        "draft",
        "acquiring",
        "planning",
        "executing",
        "complete",
        "partial",
        "failed",
        "cancelled"
      ]
    },
    "total": {
      "type": "integer",
      "minimum": 0,
      "maximum": 9007199254740991
    },
    "active": {
      "type": "integer",
      "minimum": 0,
      "maximum": 9007199254740991
    },
    "failed": {
      "type": "integer",
      "minimum": 0,
      "maximum": 9007199254740991
    },
    "pending": {
      "type": "integer",
      "minimum": 0,
      "maximum": 9007199254740991
    }
  },
  "required": [
    "id",
    "revision",
    "state",
    "total",
    "active",
    "failed",
    "pending"
  ],
  "additionalProperties": false
}
```


Referenced types: [OpaqueId](wire-schema-reference.md#schema-opaqueid), [Revision](wire-schema-reference.md#schema-revision).

## Schema TargetSelector


```json
{
  "type": "object",
  "properties": {
    "kinds": {
      "type": "array",
      "items": {
        "type": "string",
        "enum": [
          "manual",
          "tenant",
          "user",
          "agent"
        ]
      },
      "maxItems": 4
    },
    "states": {
      "type": "array",
      "items": {
        "$ref": "urn:mirabase:wire#/$defs/DatabaseState"
      },
      "maxItems": 12
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
    "maximumRevision": {
      "anyOf": [
        {
          "$ref": "urn:mirabase:wire#/$defs/Revision"
        },
        {
          "type": "null"
        }
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
    }
  },
  "required": [
    "kinds",
    "states",
    "templateId",
    "maximumRevision",
    "owner"
  ],
  "additionalProperties": false
}
```


Referenced types: [DatabaseState](wire-schema-reference.md#schema-databasestate), [Revision](wire-schema-reference.md#schema-revision), [Subject](wire-schema-reference.md#schema-subject).

## Schema SetupPlanRequest


```json
{
  "type": "object",
  "properties": {
    "expectedApplicationGeneration": {
      "$ref": "urn:mirabase:wire#/$defs/Revision"
    },
    "expectedRevision": {
      "$ref": "urn:mirabase:wire#/$defs/Revision"
    },
    "layout": {
      "$ref": "urn:mirabase:wire#/$defs/StorageLayout"
    },
    "authCollectionIds": {
      "type": "array",
      "items": {
        "type": "string"
      },
      "maxItems": 100
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
    "provision": {
      "type": "string",
      "enum": [
        "lazy",
        "eager"
      ]
    },
    "tenantSource": {
      "anyOf": [
        {
          "$ref": "urn:mirabase:wire#/$defs/ImportTenantSource"
        },
        {
          "type": "null"
        }
      ]
    },
    "access": {
      "anyOf": [
        {
          "$ref": "urn:mirabase:wire#/$defs/ImportAccess"
        },
        {
          "type": "null"
        }
      ]
    },
    "placements": {
      "type": "array",
      "items": {
        "$ref": "urn:mirabase:wire#/$defs/ImportPlacement"
      },
      "maxItems": 1000
    },
    "namespace": {
      "anyOf": [
        {
          "$ref": "urn:mirabase:wire#/$defs/OpaqueId"
        },
        {
          "type": "null"
        }
      ]
    }
  },
  "required": [
    "expectedApplicationGeneration",
    "expectedRevision",
    "layout",
    "authCollectionIds",
    "templateId",
    "provision",
    "tenantSource",
    "access",
    "placements",
    "namespace"
  ],
  "additionalProperties": false,
  "allOf": [
    {
      "if": {
        "properties": {
          "layout": {
            "const": "shared"
          }
        }
      },
      "then": {
        "properties": {
          "namespace": {
            "type": "null"
          },
          "tenantSource": {
            "type": "null"
          },
          "access": {
            "type": "null"
          }
        }
      }
    },
    {
      "if": {
        "properties": {
          "layout": {
            "enum": [
              "user",
              "agent"
            ]
          }
        }
      },
      "then": {
        "properties": {
          "authCollectionIds": {
            "minItems": 1
          },
          "templateId": {
            "type": "string"
          },
          "namespace": {
            "type": "string"
          }
        }
      }
    },
    {
      "if": {
        "properties": {
          "layout": {
            "const": "tenant"
          }
        }
      },
      "then": {
        "properties": {
          "tenantSource": {
            "$ref": "urn:mirabase:wire#/$defs/ImportTenantSource"
          },
          "access": {
            "$ref": "urn:mirabase:wire#/$defs/ImportAccess"
          },
          "namespace": {
            "$ref": "urn:mirabase:wire#/$defs/OpaqueId"
          },
          "templateId": {
            "type": "string"
          }
        }
      }
    }
  ]
}
```


Referenced types: [ImportAccess](wire-schema-reference.md#schema-importaccess), [ImportPlacement](wire-schema-reference.md#schema-importplacement), [ImportTenantSource](wire-schema-reference.md#schema-importtenantsource), [OpaqueId](wire-schema-reference.md#schema-opaqueid), [Revision](wire-schema-reference.md#schema-revision), [StorageLayout](wire-schema-reference.md#schema-storagelayout).

## Schema SetupPlan


```json
{
  "type": "object",
  "properties": {
    "id": {
      "$ref": "urn:mirabase:wire#/$defs/OpaqueId"
    },
    "revision": {
      "$ref": "urn:mirabase:wire#/$defs/Revision"
    },
    "applicationId": {
      "$ref": "urn:mirabase:wire#/$defs/ApplicationId"
    },
    "planHash": {
      "$ref": "urn:mirabase:wire#/$defs/ImportHash"
    },
    "layout": {
      "$ref": "urn:mirabase:wire#/$defs/StorageLayout"
    },
    "state": {
      "type": "string",
      "enum": [
        "draft",
        "validating",
        "invalid",
        "ready",
        "applying",
        "active",
        "failed",
        "cancelled-before-publication"
      ]
    },
    "issueIds": {
      "type": "array",
      "items": {
        "$ref": "urn:mirabase:wire#/$defs/OpaqueId"
      },
      "maxItems": 10000
    },
    "plannedDatabases": {
      "type": "integer",
      "minimum": 0,
      "maximum": 9007199254740991
    }
  },
  "required": [
    "id",
    "revision",
    "applicationId",
    "planHash",
    "layout",
    "state",
    "issueIds",
    "plannedDatabases"
  ],
  "additionalProperties": false
}
```


Referenced types: [ApplicationId](wire-schema-reference.md#schema-applicationid), [ImportHash](wire-schema-reference.md#schema-importhash), [OpaqueId](wire-schema-reference.md#schema-opaqueid), [Revision](wire-schema-reference.md#schema-revision), [StorageLayout](wire-schema-reference.md#schema-storagelayout).

## Schema SetupApply


```json
{
  "type": "object",
  "properties": {
    "expectedApplicationGeneration": {
      "$ref": "urn:mirabase:wire#/$defs/Revision"
    },
    "expectedRevision": {
      "$ref": "urn:mirabase:wire#/$defs/Revision"
    },
    "planHash": {
      "$ref": "urn:mirabase:wire#/$defs/ImportHash"
    }
  },
  "required": [
    "expectedApplicationGeneration",
    "expectedRevision",
    "planHash"
  ],
  "additionalProperties": false
}
```


Referenced types: [ImportHash](wire-schema-reference.md#schema-importhash), [Revision](wire-schema-reference.md#schema-revision).

## Schema MembershipFieldValue


```json
{
  "type": "object",
  "properties": {
    "fieldId": {
      "type": "string"
    },
    "value": {
      "$ref": "urn:mirabase:wire#/$defs/TypedValue"
    }
  },
  "required": [
    "fieldId",
    "value"
  ],
  "additionalProperties": false
}
```


Referenced types: [TypedValue](wire-schema-reference.md#schema-typedvalue).

## Schema InvitationCreate


```json
{
  "type": "object",
  "properties": {
    "namespace": {
      "$ref": "urn:mirabase:wire#/$defs/OpaqueId"
    },
    "tenantKey": {
      "$ref": "urn:mirabase:wire#/$defs/ImportTenantKey"
    },
    "email": {
      "anyOf": [
        {
          "type": "string"
        },
        {
          "type": "null"
        }
      ]
    },
    "subject": {
      "anyOf": [
        {
          "$ref": "urn:mirabase:wire#/$defs/Subject"
        },
        {
          "type": "null"
        }
      ]
    },
    "membershipValues": {
      "type": "array",
      "items": {
        "$ref": "urn:mirabase:wire#/$defs/MembershipFieldValue"
      },
      "maxItems": 100
    },
    "expiresInSeconds": {
      "type": "integer",
      "minimum": 60,
      "maximum": 604800
    }
  },
  "required": [
    "namespace",
    "tenantKey",
    "email",
    "subject",
    "membershipValues",
    "expiresInSeconds"
  ],
  "additionalProperties": false
}
```


Referenced types: [ImportTenantKey](wire-schema-reference.md#schema-importtenantkey), [MembershipFieldValue](wire-schema-reference.md#schema-membershipfieldvalue), [OpaqueId](wire-schema-reference.md#schema-opaqueid), [Subject](wire-schema-reference.md#schema-subject).

## Schema Invitation


```json
{
  "type": "object",
  "properties": {
    "id": {
      "$ref": "urn:mirabase:wire#/$defs/OpaqueId"
    },
    "state": {
      "type": "string",
      "enum": [
        "pending",
        "used",
        "revoked",
        "expired"
      ]
    },
    "expiresAt": {
      "$ref": "urn:mirabase:wire#/$defs/Timestamp"
    },
    "revision": {
      "$ref": "urn:mirabase:wire#/$defs/Revision"
    }
  },
  "required": [
    "id",
    "state",
    "expiresAt",
    "revision"
  ],
  "additionalProperties": false
}
```


Referenced types: [OpaqueId](wire-schema-reference.md#schema-opaqueid), [Revision](wire-schema-reference.md#schema-revision), [Timestamp](wire-schema-reference.md#schema-timestamp).

## Schema InvitationAccept


```json
{
  "type": "object",
  "properties": {
    "token": {
      "type": "string"
    },
    "expectedApplicationGeneration": {
      "$ref": "urn:mirabase:wire#/$defs/Revision"
    }
  },
  "required": [
    "token",
    "expectedApplicationGeneration"
  ],
  "additionalProperties": false
}
```


Referenced types: [Revision](wire-schema-reference.md#schema-revision).

## Schema OwnerTransfer


```json
{
  "type": "object",
  "properties": {
    "expectedGeneration": {
      "$ref": "urn:mirabase:wire#/$defs/Revision"
    },
    "expectedPolicyEpoch": {
      "$ref": "urn:mirabase:wire#/$defs/Revision"
    },
    "previousOwner": {
      "anyOf": [
        {
          "$ref": "urn:mirabase:wire#/$defs/Subject"
        },
        {
          "type": "null"
        }
      ]
    },
    "newOwner": {
      "$ref": "urn:mirabase:wire#/$defs/Subject"
    },
    "agentPolicy": {
      "type": "string",
      "enum": [
        "revoke",
        "rebind-reviewed"
      ]
    }
  },
  "required": [
    "expectedGeneration",
    "expectedPolicyEpoch",
    "previousOwner",
    "newOwner",
    "agentPolicy"
  ],
  "additionalProperties": false
}
```


Referenced types: [Revision](wire-schema-reference.md#schema-revision), [Subject](wire-schema-reference.md#schema-subject).

## Schema ReferencePolicy


```json
{
  "type": "object",
  "properties": {
    "id": {
      "$ref": "urn:mirabase:wire#/$defs/OpaqueId"
    },
    "revision": {
      "$ref": "urn:mirabase:wire#/$defs/Revision"
    },
    "sourceCollectionId": {
      "type": "string"
    },
    "fieldIds": {
      "type": "array",
      "items": {
        "type": "string"
      },
      "maxItems": 100
    },
    "filter": {
      "type": "string"
    },
    "targets": {
      "$ref": "urn:mirabase:wire#/$defs/TargetSelector"
    },
    "deleteBehavior": {
      "type": "string",
      "enum": [
        "remove-copy"
      ]
    },
    "maximumLagSeconds": {
      "type": "integer",
      "minimum": 1,
      "maximum": 86400
    }
  },
  "required": [
    "id",
    "revision",
    "sourceCollectionId",
    "fieldIds",
    "filter",
    "targets",
    "deleteBehavior",
    "maximumLagSeconds"
  ],
  "additionalProperties": false
}
```


Referenced types: [OpaqueId](wire-schema-reference.md#schema-opaqueid), [Revision](wire-schema-reference.md#schema-revision), [TargetSelector](wire-schema-reference.md#schema-targetselector).

## Schema ReferenceStatus


```json
{
  "type": "object",
  "properties": {
    "id": {
      "$ref": "urn:mirabase:wire#/$defs/OpaqueId"
    },
    "revision": {
      "$ref": "urn:mirabase:wire#/$defs/Revision"
    },
    "targets": {
      "type": "integer",
      "minimum": 0,
      "maximum": 9007199254740991
    },
    "healthy": {
      "type": "integer",
      "minimum": 0,
      "maximum": 9007199254740991
    },
    "stale": {
      "type": "integer",
      "minimum": 0,
      "maximum": 9007199254740991
    },
    "failed": {
      "type": "integer",
      "minimum": 0,
      "maximum": 9007199254740991
    }
  },
  "required": [
    "id",
    "revision",
    "targets",
    "healthy",
    "stale",
    "failed"
  ],
  "additionalProperties": false
}
```


Referenced types: [OpaqueId](wire-schema-reference.md#schema-opaqueid), [Revision](wire-schema-reference.md#schema-revision).

## Schema LiveParameter


```json
{
  "type": "object",
  "properties": {
    "name": {
      "type": "string"
    },
    "type": {
      "type": "string"
    },
    "nullable": {
      "type": "boolean"
    }
  },
  "required": [
    "name",
    "type",
    "nullable"
  ],
  "additionalProperties": false
}
```


## Schema LivePlan


```json
{
  "type": "object",
  "properties": {
    "id": {
      "$ref": "urn:mirabase:wire#/$defs/OpaqueId"
    },
    "revision": {
      "$ref": "urn:mirabase:wire#/$defs/Revision"
    },
    "sql": {
      "type": "string"
    },
    "parameters": {
      "type": "array",
      "items": {
        "$ref": "urn:mirabase:wire#/$defs/LiveParameter"
      },
      "maxItems": 100
    },
    "resultKey": {
      "type": "string"
    },
    "dependencies": {
      "type": "array",
      "items": {
        "type": "string"
      },
      "maxItems": 100
    },
    "maxRows": {
      "type": "integer",
      "minimum": 1,
      "maximum": 1000
    },
    "maxBytes": {
      "type": "integer",
      "minimum": 1024,
      "maximum": 8388608
    },
    "state": {
      "type": "string",
      "enum": [
        "draft",
        "published",
        "retired"
      ]
    }
  },
  "required": [
    "id",
    "revision",
    "sql",
    "parameters",
    "resultKey",
    "dependencies",
    "maxRows",
    "maxBytes",
    "state"
  ],
  "additionalProperties": false
}
```


Referenced types: [LiveParameter](wire-schema-reference.md#schema-liveparameter), [OpaqueId](wire-schema-reference.md#schema-opaqueid), [Revision](wire-schema-reference.md#schema-revision).

## Schema LivePlanPage


```json
{
  "type": "object",
  "properties": {
    "items": {
      "type": "array",
      "items": {
        "$ref": "urn:mirabase:wire#/$defs/LivePlan"
      },
      "maxItems": 100
    },
    "page": {
      "type": "integer",
      "minimum": 1,
      "maximum": 9007199254740991
    },
    "perPage": {
      "type": "integer",
      "minimum": 1,
      "maximum": 100
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


Referenced types: [LivePlan](wire-schema-reference.md#schema-liveplan).

## Schema RevisionGuard


```json
{
  "type": "object",
  "properties": {
    "expectedRevision": {
      "$ref": "urn:mirabase:wire#/$defs/Revision"
    }
  },
  "required": [
    "expectedRevision"
  ],
  "additionalProperties": false
}
```


Referenced types: [Revision](wire-schema-reference.md#schema-revision).

## Schema RolloutControl


```json
{
  "type": "object",
  "properties": {
    "expectedRevision": {
      "$ref": "urn:mirabase:wire#/$defs/Revision"
    },
    "selectionHash": {
      "$ref": "urn:mirabase:wire#/$defs/ImportHash"
    }
  },
  "required": [
    "expectedRevision",
    "selectionHash"
  ],
  "additionalProperties": false
}
```


Referenced types: [ImportHash](wire-schema-reference.md#schema-importhash), [Revision](wire-schema-reference.md#schema-revision).

## Schema RolloutTarget


```json
{
  "type": "object",
  "properties": {
    "databaseId": {
      "$ref": "urn:mirabase:wire#/$defs/DatabaseId"
    },
    "generation": {
      "$ref": "urn:mirabase:wire#/$defs/Revision"
    },
    "state": {
      "type": "string",
      "enum": [
        "pending",
        "running",
        "succeeded",
        "failed",
        "skipped",
        "cancelled"
      ]
    },
    "errorCode": {
      "anyOf": [
        {
          "type": "string"
        },
        {
          "type": "null"
        }
      ]
    },
    "revision": {
      "$ref": "urn:mirabase:wire#/$defs/Revision"
    }
  },
  "required": [
    "databaseId",
    "generation",
    "state",
    "errorCode",
    "revision"
  ],
  "additionalProperties": false
}
```


Referenced types: [DatabaseId](wire-schema-reference.md#schema-databaseid), [Revision](wire-schema-reference.md#schema-revision).

## Schema RolloutTargetPage


```json
{
  "type": "object",
  "properties": {
    "items": {
      "type": "array",
      "items": {
        "$ref": "urn:mirabase:wire#/$defs/RolloutTarget"
      },
      "maxItems": 100
    },
    "nextCursor": {
      "anyOf": [
        {
          "type": "string"
        },
        {
          "type": "null"
        }
      ]
    },
    "hasMore": {
      "type": "boolean"
    }
  },
  "required": [
    "items",
    "nextCursor",
    "hasMore"
  ],
  "additionalProperties": false
}
```


Referenced types: [RolloutTarget](wire-schema-reference.md#schema-rollouttarget).

## Schema RelationWorkflowStep


```json
{
  "type": "object",
  "properties": {
    "databaseId": {
      "$ref": "urn:mirabase:wire#/$defs/DatabaseId"
    },
    "collectionId": {
      "type": "string"
    },
    "relationFieldId": {
      "type": "string"
    },
    "action": {
      "type": "string",
      "enum": [
        "delete",
        "unlink"
      ]
    },
    "expectedGeneration": {
      "$ref": "urn:mirabase:wire#/$defs/Revision"
    }
  },
  "required": [
    "databaseId",
    "collectionId",
    "relationFieldId",
    "action",
    "expectedGeneration"
  ],
  "additionalProperties": false
}
```


Referenced types: [DatabaseId](wire-schema-reference.md#schema-databaseid), [Revision](wire-schema-reference.md#schema-revision).

## Schema RelationWorkflow


```json
{
  "type": "object",
  "properties": {
    "id": {
      "$ref": "urn:mirabase:wire#/$defs/OpaqueId"
    },
    "revision": {
      "$ref": "urn:mirabase:wire#/$defs/Revision"
    },
    "steps": {
      "type": "array",
      "items": {
        "$ref": "urn:mirabase:wire#/$defs/RelationWorkflowStep"
      },
      "maxItems": 1000
    },
    "principal": {
      "$ref": "urn:mirabase:wire#/$defs/Subject"
    }
  },
  "required": [
    "id",
    "revision",
    "steps",
    "principal"
  ],
  "additionalProperties": false
}
```


Referenced types: [OpaqueId](wire-schema-reference.md#schema-opaqueid), [RelationWorkflowStep](wire-schema-reference.md#schema-relationworkflowstep), [Revision](wire-schema-reference.md#schema-revision), [Subject](wire-schema-reference.md#schema-subject).

## Schema ReportDefinition


```json
{
  "type": "object",
  "properties": {
    "id": {
      "$ref": "urn:mirabase:wire#/$defs/OpaqueId"
    },
    "revision": {
      "$ref": "urn:mirabase:wire#/$defs/Revision"
    },
    "sourceTemplateIds": {
      "type": "array",
      "items": {
        "type": "string"
      },
      "maxItems": 100
    },
    "projectedFieldIds": {
      "type": "array",
      "items": {
        "type": "string"
      },
      "maxItems": 100
    },
    "aggregateSql": {
      "type": "string"
    },
    "targetCollectionId": {
      "type": "string"
    },
    "freshnessSeconds": {
      "type": "integer",
      "minimum": 1,
      "maximum": 86400
    },
    "readerRule": {
      "type": "string"
    }
  },
  "required": [
    "id",
    "revision",
    "sourceTemplateIds",
    "projectedFieldIds",
    "aggregateSql",
    "targetCollectionId",
    "freshnessSeconds",
    "readerRule"
  ],
  "additionalProperties": false
}
```


Referenced types: [OpaqueId](wire-schema-reference.md#schema-opaqueid), [Revision](wire-schema-reference.md#schema-revision).

## Schema LayoutChange


```json
{
  "type": "object",
  "properties": {
    "expectedApplicationGeneration": {
      "$ref": "urn:mirabase:wire#/$defs/Revision"
    },
    "expectedRevision": {
      "$ref": "urn:mirabase:wire#/$defs/Revision"
    },
    "planHash": {
      "$ref": "urn:mirabase:wire#/$defs/ImportHash"
    },
    "destinationLayout": {
      "$ref": "urn:mirabase:wire#/$defs/StorageLayout"
    },
    "backupId": {
      "$ref": "urn:mirabase:wire#/$defs/OpaqueId"
    }
  },
  "required": [
    "expectedApplicationGeneration",
    "expectedRevision",
    "planHash",
    "destinationLayout",
    "backupId"
  ],
  "additionalProperties": false
}
```


Referenced types: [ImportHash](wire-schema-reference.md#schema-importhash), [OpaqueId](wire-schema-reference.md#schema-opaqueid), [Revision](wire-schema-reference.md#schema-revision), [StorageLayout](wire-schema-reference.md#schema-storagelayout).

## Schema FieldDefinition


```json
{
  "type": "object",
  "properties": {
    "id": {
      "type": "string"
    },
    "name": {
      "type": "string"
    },
    "type": {
      "type": "string",
      "enum": [
        "text",
        "number",
        "bool",
        "email",
        "url",
        "editor",
        "password",
        "date",
        "autodate",
        "select",
        "json",
        "relation",
        "file",
        "geoPoint",
        "vector",
        "int64",
        "decimal",
        "uuid",
        "array",
        "binary",
        "typed-date",
        "typed-time",
        "typed-timestamp",
        "network-address",
        "struct",
        "union",
        "domain",
        "custom"
      ]
    },
    "required": {
      "type": "boolean"
    },
    "hidden": {
      "type": "boolean"
    },
    "system": {
      "type": "boolean"
    },
    "options": {
      "type": "object",
      "x-schema-resolver": "exact inherited field options or added optionsSchema in engine-reference.md; forbid unknown option keys"
    }
  },
  "required": [
    "id",
    "name",
    "type",
    "options"
  ],
  "additionalProperties": false
}
```


## Schema CollectionDefinition


```json
{
  "type": "object",
  "properties": {
    "id": {
      "type": "string"
    },
    "name": {
      "type": "string"
    },
    "type": {
      "type": "string",
      "enum": [
        "base",
        "auth",
        "view"
      ]
    },
    "fields": {
      "type": "array",
      "items": {
        "$ref": "urn:mirabase:wire#/$defs/FieldDefinition"
      },
      "maxItems": 1000
    },
    "indexes": {
      "type": "array",
      "items": {
        "type": "string"
      },
      "maxItems": 100
    },
    "listRule": {
      "type": [
        "string",
        "null"
      ]
    },
    "viewRule": {
      "type": [
        "string",
        "null"
      ]
    },
    "createRule": {
      "type": [
        "string",
        "null"
      ]
    },
    "updateRule": {
      "type": [
        "string",
        "null"
      ]
    },
    "deleteRule": {
      "type": [
        "string",
        "null"
      ]
    },
    "options": {
      "type": "object",
      "x-schema-resolver": "exact pinned base/auth/view collection options, generated from collection type; no unknown keys"
    }
  },
  "required": [
    "id",
    "name",
    "type",
    "fields",
    "indexes",
    "listRule",
    "viewRule",
    "createRule",
    "updateRule",
    "deleteRule",
    "options"
  ],
  "additionalProperties": false
}
```


Referenced types: [FieldDefinition](wire-schema-reference.md#schema-fielddefinition).

## Schema IndexDefinition


```json
{
  "type": "object",
  "properties": {
    "name": {
      "type": "string"
    },
    "kind": {
      "type": "string",
      "enum": [
        "btree",
        "fts"
      ]
    },
    "unique": {
      "type": "boolean"
    },
    "fields": {
      "type": "array",
      "maxItems": 100,
      "minItems": 1,
      "items": {
        "type": "object",
        "properties": {
          "expression": {
            "type": "string"
          },
          "direction": {
            "type": "string",
            "enum": [
              "asc",
              "desc"
            ]
          },
          "nulls": {
            "type": "string",
            "enum": [
              "default",
              "first",
              "last"
            ]
          },
          "weight": {
            "type": [
              "number",
              "null"
            ]
          },
          "jsonPaths": {
            "type": "array",
            "items": {
              "type": "string"
            },
            "maxItems": 100
          }
        },
        "required": [
          "expression",
          "direction",
          "nulls",
          "weight",
          "jsonPaths"
        ],
        "additionalProperties": false
      }
    },
    "predicate": {
      "type": [
        "string",
        "null"
      ]
    },
    "tokenizer": {
      "type": [
        "string",
        "null"
      ]
    },
    "tokenizerVersion": {
      "type": [
        "string",
        "null"
      ]
    },
    "language": {
      "type": [
        "string",
        "null"
      ]
    }
  },
  "required": [
    "name",
    "kind",
    "unique",
    "fields",
    "predicate",
    "tokenizer",
    "tokenizerVersion",
    "language"
  ],
  "additionalProperties": false
}
```


## Schema TypeDefinition


```json
{
  "type": "object",
  "properties": {
    "id": {
      "type": "string"
    },
    "kind": {
      "type": "string",
      "enum": [
        "domain",
        "struct",
        "union",
        "custom"
      ]
    },
    "baseType": {
      "type": [
        "string",
        "null"
      ]
    },
    "constraints": {
      "type": "array",
      "items": {
        "type": "string"
      },
      "maxItems": 100
    },
    "members": {
      "type": "array",
      "items": {
        "type": "object",
        "properties": {
          "name": {
            "type": "string"
          },
          "typeId": {
            "type": "string"
          }
        },
        "required": [
          "name",
          "typeId"
        ],
        "additionalProperties": false
      },
      "maxItems": 1000
    },
    "encoderId": {
      "type": [
        "string",
        "null"
      ]
    }
  },
  "required": [
    "id",
    "kind",
    "baseType",
    "constraints",
    "members",
    "encoderId"
  ],
  "additionalProperties": false
}
```


## Schema ViewDefinition


```json
{
  "type": "object",
  "properties": {
    "name": {
      "type": "string"
    },
    "query": {
      "type": "string"
    },
    "resultKey": {
      "type": "string"
    },
    "dependencies": {
      "type": "array",
      "items": {
        "type": "string"
      },
      "maxItems": 100
    },
    "materialized": {
      "type": "boolean"
    },
    "refreshPolicy": {
      "type": "string",
      "enum": [
        "none",
        "incremental"
      ]
    }
  },
  "required": [
    "name",
    "query",
    "resultKey",
    "dependencies",
    "materialized",
    "refreshPolicy"
  ],
  "additionalProperties": false
}
```


## Schema SequenceDefinition


```json
{
  "type": "object",
  "properties": {
    "name": {
      "type": "string"
    },
    "start": {
      "type": "string",
      "pattern": "^-?(0|[1-9][0-9]*)$"
    },
    "increment": {
      "type": "string",
      "pattern": "^-?[1-9][0-9]*$"
    },
    "minimum": {
      "type": "string",
      "pattern": "^-?[0-9]+$"
    },
    "maximum": {
      "type": "string",
      "pattern": "^-?[0-9]+$"
    },
    "cycle": {
      "type": "boolean"
    }
  },
  "required": [
    "name",
    "start",
    "increment",
    "minimum",
    "maximum",
    "cycle"
  ],
  "additionalProperties": false
}
```


## Schema SetupState


```json
{
  "type": "object",
  "properties": {
    "applicationId": {
      "$ref": "urn:mirabase:wire#/$defs/ApplicationId"
    },
    "applicationGeneration": {
      "$ref": "urn:mirabase:wire#/$defs/Revision"
    },
    "revision": {
      "$ref": "urn:mirabase:wire#/$defs/Revision"
    },
    "layout": {
      "$ref": "urn:mirabase:wire#/$defs/StorageLayout"
    },
    "draft": {
      "anyOf": [
        {
          "$ref": "urn:mirabase:wire#/$defs/SetupPlan"
        },
        {
          "type": "null"
        }
      ]
    }
  },
  "required": [
    "applicationId",
    "applicationGeneration",
    "revision",
    "layout",
    "draft"
  ],
  "additionalProperties": false
}
```


Referenced types: [ApplicationId](wire-schema-reference.md#schema-applicationid), [Revision](wire-schema-reference.md#schema-revision), [SetupPlan](wire-schema-reference.md#schema-setupplan), [StorageLayout](wire-schema-reference.md#schema-storagelayout).

## Schema ImportBatchSourceInput


```json
{
  "type": "object",
  "properties": {
    "sessionId": {
      "$ref": "urn:mirabase:wire#/$defs/OpaqueId"
    },
    "originId": {
      "$ref": "urn:mirabase:wire#/$defs/OpaqueId"
    },
    "sourceHash": {
      "$ref": "urn:mirabase:wire#/$defs/ImportHash"
    },
    "repeatPolicy": {
      "type": "string",
      "enum": [
        "resume-existing",
        "create-independent-copy",
        "replace-matching-origin"
      ]
    }
  },
  "required": [
    "sessionId",
    "originId",
    "sourceHash",
    "repeatPolicy"
  ],
  "additionalProperties": false
}
```


Referenced types: [ImportHash](wire-schema-reference.md#schema-importhash), [OpaqueId](wire-schema-reference.md#schema-opaqueid).

## Schema ImportBatchSourceState


```json
{
  "type": "object",
  "properties": {
    "sourceId": {
      "$ref": "urn:mirabase:wire#/$defs/OpaqueId"
    },
    "sessionId": {
      "$ref": "urn:mirabase:wire#/$defs/OpaqueId"
    },
    "originId": {
      "$ref": "urn:mirabase:wire#/$defs/OpaqueId"
    },
    "destinationApplicationId": {
      "$ref": "urn:mirabase:wire#/$defs/ApplicationId"
    },
    "sourceHash": {
      "$ref": "urn:mirabase:wire#/$defs/ImportHash"
    },
    "repeatPolicy": {
      "type": "string",
      "enum": [
        "resume-existing",
        "create-independent-copy",
        "replace-matching-origin"
      ]
    },
    "phase": {
      "$ref": "urn:mirabase:wire#/$defs/ImportPhase"
    },
    "errorCode": {
      "type": [
        "string",
        "null"
      ]
    }
  },
  "required": [
    "sourceId",
    "sessionId",
    "originId",
    "destinationApplicationId",
    "sourceHash",
    "repeatPolicy",
    "phase",
    "errorCode"
  ],
  "additionalProperties": false
}
```


Referenced types: [ApplicationId](wire-schema-reference.md#schema-applicationid), [ImportHash](wire-schema-reference.md#schema-importhash), [ImportPhase](wire-schema-reference.md#schema-importphase), [OpaqueId](wire-schema-reference.md#schema-opaqueid).

## Schema ImportBatchSourcePage


```json
{
  "type": "object",
  "properties": {
    "items": {
      "type": "array",
      "items": {
        "$ref": "urn:mirabase:wire#/$defs/ImportBatchSourceState"
      },
      "maxItems": 100
    },
    "page": {
      "type": "integer",
      "minimum": 1
    },
    "perPage": {
      "type": "integer",
      "minimum": 1,
      "maximum": 100
    },
    "totalItems": {
      "type": "integer",
      "minimum": 0
    },
    "totalPages": {
      "type": "integer",
      "minimum": 0
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


Referenced types: [ImportBatchSourceState](wire-schema-reference.md#schema-importbatchsourcestate).

## Schema BootstrapAdministrator


```json
{
  "type": "object",
  "properties": {
    "token": {
      "type": "string"
    },
    "email": {
      "type": "string",
      "format": "email"
    },
    "password": {
      "type": "string",
      "minLength": 8
    },
    "passwordConfirm": {
      "type": "string",
      "minLength": 8
    },
    "expectedApplicationGeneration": {
      "$ref": "urn:mirabase:wire#/$defs/Revision"
    }
  },
  "required": [
    "token",
    "email",
    "password",
    "passwordConfirm",
    "expectedApplicationGeneration"
  ],
  "additionalProperties": false
}
```


Referenced types: [Revision](wire-schema-reference.md#schema-revision).

## Schema BootstrapComplete


```json
{
  "type": "object",
  "properties": {
    "applicationId": {
      "$ref": "urn:mirabase:wire#/$defs/ApplicationId"
    },
    "status": {
      "const": "administrator-established"
    }
  },
  "required": [
    "applicationId",
    "status"
  ],
  "additionalProperties": false
}
```


Referenced types: [ApplicationId](wire-schema-reference.md#schema-applicationid).
