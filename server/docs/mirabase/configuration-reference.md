# Configuration reference

**Current release authority:** [current implementation inventory](../current-product.md), [user documentation](../user/index.md) and [launch status](../launch/status.md). Initial v0.1/roadmap delivery labels below preserve planning contracts; they do not establish today's implementation or permit deleting subsequently integrated features. [Open-source platform roadmap](../roadmap/platform/README.md) has separate design and gates.

**Delivery: v0.1 and roadmap.** [v0.1-build.md](v0.1-build.md) selects the current portion; [roadsmap.md](roadsmap.md) retains the remainder. Detailed schemas/configuration/ABI and full-family acceptance below describe the target superset unless narrowed by that allocation.

This is the complete effective configuration. Accept partial operator overrides, merge using the precedence in [configuration and deployment](configuration-and-deployment.md), then validate every key and cross-field invariant before effects. Secret references do not disclose values. Global connection capacity must exceed the auth/recovery reservation; application query limits cannot exceed aggregate capacity.

## Effective defaults

```json
{
  "formatVersion": 1,
  "engine": {
    "defaultProfile": "wal",
    "nativeCacheDir": "auto",
    "requireVerifiedPayload": true
  },
  "databases": {
    "enabled": false,
    "maxOpen": 64,
    "maxConcurrentOpens": 8,
    "idleCloseSeconds": 30,
    "readsPerHandle": 4,
    "mvccWritersPerHandle": 4,
    "globalConnections": 256,
    "globalPageCacheMiB": 256,
    "trashRetentionHours": 24
  },
  "partitioning": {
    "enabled": false,
    "provision": "lazy",
    "ownerDeletion": "quarantine",
    "defaultTemplateId": null
  },
  "changes": {
    "enabled": false,
    "retentionDays": 7,
    "maxBytesPerDatabase": 1073741824,
    "streamQueueFrames": 256,
    "streamQueueBytes": 8388608
  },
  "sync": {
    "records": false,
    "native": false,
    "groupMaxMutations": 50,
    "maxBodyBytes": 5242880,
    "bootstrapSessionSeconds": 1800,
    "receiptRetentionDays": 7,
    "stageRetentionHours": 24
  },
  "postgres": {
    "enabled": false,
    "listen": "127.0.0.1:5433",
    "tlsCertRef": null,
    "tlsKeyRef": null,
    "auth": "scram-sha-256"
  },
  "sqlHttp": {
    "enabled": false
  },
  "liveQueries": {
    "enabled": false
  },
  "jobs": {
    "workers": 2,
    "schemaParallelism": 2,
    "leaseSeconds": 30,
    "renewSeconds": 10,
    "maxAttempts": 8
  },
  "search": {
    "deadlineMs": 5000,
    "candidateBudget": 10000,
    "defaultLimit": 20,
    "maxLimit": 100,
    "cursorLeaseSeconds": 60,
    "maxCursorLeases": 16
  },
  "security": {
    "credentialMaxSeconds": 3600,
    "revocationCheckSeconds": 5,
    "secretProvider": "environment"
  },
  "shutdown": {
    "graceSeconds": 30
  },
  "imports": {
    "enabled": true,
    "maxSourceBytes": 21474836480,
    "maxChunkBytes": 8388608,
    "copyBatchRecords": 1000,
    "copyBatchBytes": 4194304,
    "maxActiveSessions": 2,
    "recoveryRetentionDays": 7
  },
  "applications": {
    "maxActiveMetadata": 16,
    "routeMode": "prefix",
    "defaultApplicationId": "default",
    "bootstrapTokenSeconds": 1800
  },
  "admission": {
    "globalQueries": 128,
    "queriesPerApplication": 16,
    "reservedConnections": 16,
    "queueWaitSeconds": 5,
    "globalQueuedJobs": 1024,
    "queuedJobsPerApplication": 128,
    "minimumFreeDiskBytes": 1073741824,
    "walWarningBytes": 268435456,
    "diskLimitPerApplicationBytes": null,
    "diskLimitPerDatabaseBytes": null,
    "jsMaxVMs": 4,
    "jsHeapMiB": 64
  },
  "audit": {
    "retentionDays": 90,
    "maxBytesPerScope": 268435456
  },
  "membership": {
    "invitationSeconds": 604800
  },
  "references": {
    "maximumLagSeconds": 60
  }
}
```

## Exact configuration schema

```json
{
  "$schema": "https://json-schema.org/draft/2020-12/schema",
  "title": "Complete new MiraBase configuration; inherited flags/settings remain source-defined",
  "type": "object",
  "properties": {
    "formatVersion": {
      "const": 1
    },
    "engine": {
      "type": "object",
      "properties": {
        "defaultProfile": {
          "type": "string",
          "enum": [
            "wal",
            "mvcc"
          ]
        },
        "nativeCacheDir": {
          "type": "string"
        },
        "requireVerifiedPayload": {
          "type": "boolean"
        }
      },
      "required": [
        "defaultProfile",
        "nativeCacheDir",
        "requireVerifiedPayload"
      ],
      "additionalProperties": false
    },
    "databases": {
      "type": "object",
      "properties": {
        "enabled": {
          "type": "boolean"
        },
        "maxOpen": {
          "type": "integer",
          "minimum": 1,
          "maximum": 100000
        },
        "maxConcurrentOpens": {
          "type": "integer",
          "minimum": 1,
          "maximum": 1024
        },
        "idleCloseSeconds": {
          "type": "integer",
          "minimum": 0,
          "maximum": 86400
        },
        "readsPerHandle": {
          "type": "integer",
          "minimum": 1,
          "maximum": 120
        },
        "mvccWritersPerHandle": {
          "type": "integer",
          "minimum": 1,
          "maximum": 120
        },
        "globalConnections": {
          "type": "integer",
          "minimum": 17,
          "maximum": 100000
        },
        "globalPageCacheMiB": {
          "type": "integer",
          "minimum": 32,
          "maximum": 1048576
        },
        "trashRetentionHours": {
          "type": "integer",
          "minimum": 0,
          "maximum": 87600
        }
      },
      "required": [
        "enabled",
        "maxOpen",
        "maxConcurrentOpens",
        "idleCloseSeconds",
        "readsPerHandle",
        "mvccWritersPerHandle",
        "globalConnections",
        "globalPageCacheMiB",
        "trashRetentionHours"
      ],
      "additionalProperties": false
    },
    "partitioning": {
      "type": "object",
      "properties": {
        "enabled": {
          "type": "boolean"
        },
        "provision": {
          "type": "string",
          "enum": [
            "lazy",
            "eager"
          ]
        },
        "ownerDeletion": {
          "type": "string",
          "enum": [
            "quarantine",
            "retain-unowned",
            "delete"
          ]
        },
        "defaultTemplateId": {
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
        "enabled",
        "provision",
        "ownerDeletion",
        "defaultTemplateId"
      ],
      "additionalProperties": false
    },
    "changes": {
      "type": "object",
      "properties": {
        "enabled": {
          "type": "boolean"
        },
        "retentionDays": {
          "type": "integer",
          "minimum": 1,
          "maximum": 3650
        },
        "maxBytesPerDatabase": {
          "type": "integer",
          "minimum": 1048576,
          "maximum": 9007199254740991
        },
        "streamQueueFrames": {
          "type": "integer",
          "minimum": 1,
          "maximum": 100000
        },
        "streamQueueBytes": {
          "type": "integer",
          "minimum": 1024,
          "maximum": 9007199254740991
        }
      },
      "required": [
        "enabled",
        "retentionDays",
        "maxBytesPerDatabase",
        "streamQueueFrames",
        "streamQueueBytes"
      ],
      "additionalProperties": false
    },
    "sync": {
      "type": "object",
      "properties": {
        "records": {
          "type": "boolean"
        },
        "native": {
          "type": "boolean"
        },
        "groupMaxMutations": {
          "type": "integer",
          "minimum": 1,
          "maximum": 50
        },
        "maxBodyBytes": {
          "type": "integer",
          "minimum": 1024,
          "maximum": 9007199254740991
        },
        "bootstrapSessionSeconds": {
          "type": "integer",
          "minimum": 60,
          "maximum": 86400
        },
        "receiptRetentionDays": {
          "type": "integer",
          "minimum": 1,
          "maximum": 3650
        },
        "stageRetentionHours": {
          "type": "integer",
          "minimum": 1,
          "maximum": 168
        }
      },
      "required": [
        "records",
        "native",
        "groupMaxMutations",
        "maxBodyBytes",
        "bootstrapSessionSeconds",
        "receiptRetentionDays",
        "stageRetentionHours"
      ],
      "additionalProperties": false
    },
    "postgres": {
      "type": "object",
      "properties": {
        "enabled": {
          "type": "boolean"
        },
        "listen": {
          "type": "string"
        },
        "tlsCertRef": {
          "anyOf": [
            {
              "type": "string"
            },
            {
              "type": "null"
            }
          ]
        },
        "tlsKeyRef": {
          "anyOf": [
            {
              "type": "string"
            },
            {
              "type": "null"
            }
          ]
        },
        "auth": {
          "type": "string",
          "enum": [
            "scram-sha-256"
          ]
        }
      },
      "required": [
        "enabled",
        "listen",
        "tlsCertRef",
        "tlsKeyRef",
        "auth"
      ],
      "additionalProperties": false
    },
    "sqlHttp": {
      "type": "object",
      "properties": {
        "enabled": {
          "type": "boolean"
        }
      },
      "required": [
        "enabled"
      ],
      "additionalProperties": false
    },
    "liveQueries": {
      "type": "object",
      "properties": {
        "enabled": {
          "type": "boolean"
        }
      },
      "required": [
        "enabled"
      ],
      "additionalProperties": false
    },
    "jobs": {
      "type": "object",
      "properties": {
        "workers": {
          "type": "integer",
          "minimum": 1,
          "maximum": 1024
        },
        "schemaParallelism": {
          "type": "integer",
          "minimum": 1,
          "maximum": 1024
        },
        "leaseSeconds": {
          "type": "integer",
          "minimum": 10,
          "maximum": 3600
        },
        "renewSeconds": {
          "type": "integer",
          "minimum": 1,
          "maximum": 1200
        },
        "maxAttempts": {
          "type": "integer",
          "minimum": 1,
          "maximum": 100
        }
      },
      "required": [
        "workers",
        "schemaParallelism",
        "leaseSeconds",
        "renewSeconds",
        "maxAttempts"
      ],
      "additionalProperties": false
    },
    "search": {
      "type": "object",
      "properties": {
        "deadlineMs": {
          "type": "integer",
          "minimum": 10,
          "maximum": 300000
        },
        "candidateBudget": {
          "type": "integer",
          "minimum": 100,
          "maximum": 100000000
        },
        "defaultLimit": {
          "type": "integer",
          "minimum": 1,
          "maximum": 100
        },
        "maxLimit": {
          "type": "integer",
          "minimum": 1,
          "maximum": 100
        },
        "cursorLeaseSeconds": {
          "type": "integer",
          "minimum": 1,
          "maximum": 600
        },
        "maxCursorLeases": {
          "type": "integer",
          "minimum": 1,
          "maximum": 10000
        }
      },
      "required": [
        "deadlineMs",
        "candidateBudget",
        "defaultLimit",
        "maxLimit",
        "cursorLeaseSeconds",
        "maxCursorLeases"
      ],
      "additionalProperties": false
    },
    "security": {
      "type": "object",
      "properties": {
        "credentialMaxSeconds": {
          "type": "integer",
          "minimum": 60,
          "maximum": 31536000
        },
        "revocationCheckSeconds": {
          "type": "integer",
          "minimum": 1,
          "maximum": 60
        },
        "secretProvider": {
          "type": "string",
          "enum": [
            "environment",
            "private-file"
          ]
        }
      },
      "required": [
        "credentialMaxSeconds",
        "revocationCheckSeconds",
        "secretProvider"
      ],
      "additionalProperties": false
    },
    "shutdown": {
      "type": "object",
      "properties": {
        "graceSeconds": {
          "type": "integer",
          "minimum": 1,
          "maximum": 600
        }
      },
      "required": [
        "graceSeconds"
      ],
      "additionalProperties": false
    },
    "imports": {
      "type": "object",
      "properties": {
        "enabled": {
          "type": "boolean"
        },
        "maxSourceBytes": {
          "type": "integer",
          "minimum": 1048576,
          "maximum": 9007199254740991
        },
        "maxChunkBytes": {
          "type": "integer",
          "minimum": 65536,
          "maximum": 8388608
        },
        "copyBatchRecords": {
          "type": "integer",
          "minimum": 1,
          "maximum": 100000
        },
        "copyBatchBytes": {
          "type": "integer",
          "minimum": 65536,
          "maximum": 9007199254740991
        },
        "maxActiveSessions": {
          "type": "integer",
          "minimum": 1,
          "maximum": 64
        },
        "recoveryRetentionDays": {
          "type": "integer",
          "minimum": 1,
          "maximum": 3650
        }
      },
      "required": [
        "enabled",
        "maxSourceBytes",
        "maxChunkBytes",
        "copyBatchRecords",
        "copyBatchBytes",
        "maxActiveSessions",
        "recoveryRetentionDays"
      ],
      "additionalProperties": false
    },
    "applications": {
      "type": "object",
      "properties": {
        "maxActiveMetadata": {
          "type": "integer",
          "minimum": 1,
          "maximum": 9007199254740991
        },
        "routeMode": {
          "type": "string",
          "enum": [
            "prefix"
          ]
        },
        "defaultApplicationId": {
          "type": "string",
          "enum": [
            "default"
          ]
        },
        "bootstrapTokenSeconds": {
          "type": "integer",
          "minimum": 1,
          "maximum": 9007199254740991
        }
      },
      "required": [
        "maxActiveMetadata",
        "routeMode",
        "defaultApplicationId",
        "bootstrapTokenSeconds"
      ],
      "additionalProperties": false
    },
    "admission": {
      "type": "object",
      "properties": {
        "globalQueries": {
          "type": "integer",
          "minimum": 1,
          "maximum": 9007199254740991
        },
        "queriesPerApplication": {
          "type": "integer",
          "minimum": 1,
          "maximum": 9007199254740991
        },
        "reservedConnections": {
          "type": "integer",
          "minimum": 1,
          "maximum": 9007199254740991
        },
        "queueWaitSeconds": {
          "type": "integer",
          "minimum": 1,
          "maximum": 9007199254740991
        },
        "globalQueuedJobs": {
          "type": "integer",
          "minimum": 1,
          "maximum": 9007199254740991
        },
        "queuedJobsPerApplication": {
          "type": "integer",
          "minimum": 1,
          "maximum": 9007199254740991
        },
        "minimumFreeDiskBytes": {
          "type": "integer",
          "minimum": 1,
          "maximum": 9007199254740991
        },
        "walWarningBytes": {
          "type": "integer",
          "minimum": 1,
          "maximum": 9007199254740991
        },
        "diskLimitPerApplicationBytes": {
          "anyOf": [
            {
              "type": "integer",
              "minimum": 1,
              "maximum": 9007199254740991
            },
            {
              "type": "null"
            }
          ]
        },
        "diskLimitPerDatabaseBytes": {
          "anyOf": [
            {
              "type": "integer",
              "minimum": 1,
              "maximum": 9007199254740991
            },
            {
              "type": "null"
            }
          ]
        },
        "jsMaxVMs": {
          "type": "integer",
          "minimum": 1,
          "maximum": 9007199254740991
        },
        "jsHeapMiB": {
          "type": "integer",
          "minimum": 1,
          "maximum": 9007199254740991
        }
      },
      "required": [
        "globalQueries",
        "queriesPerApplication",
        "reservedConnections",
        "queueWaitSeconds",
        "globalQueuedJobs",
        "queuedJobsPerApplication",
        "minimumFreeDiskBytes",
        "walWarningBytes",
        "diskLimitPerApplicationBytes",
        "diskLimitPerDatabaseBytes",
        "jsMaxVMs",
        "jsHeapMiB"
      ],
      "additionalProperties": false
    },
    "audit": {
      "type": "object",
      "properties": {
        "retentionDays": {
          "type": "integer",
          "minimum": 1,
          "maximum": 9007199254740991
        },
        "maxBytesPerScope": {
          "type": "integer",
          "minimum": 1,
          "maximum": 9007199254740991
        }
      },
      "required": [
        "retentionDays",
        "maxBytesPerScope"
      ],
      "additionalProperties": false
    },
    "membership": {
      "type": "object",
      "properties": {
        "invitationSeconds": {
          "type": "integer",
          "minimum": 1,
          "maximum": 604800
        }
      },
      "required": [
        "invitationSeconds"
      ],
      "additionalProperties": false
    },
    "references": {
      "type": "object",
      "properties": {
        "maximumLagSeconds": {
          "type": "integer",
          "minimum": 1,
          "maximum": 86400
        }
      },
      "required": [
        "maximumLagSeconds"
      ],
      "additionalProperties": false
    }
  },
  "required": [
    "formatVersion",
    "engine",
    "databases",
    "partitioning",
    "changes",
    "sync",
    "postgres",
    "sqlHttp",
    "liveQueries",
    "jobs",
    "search",
    "security",
    "shutdown",
    "imports",
    "applications",
    "admission",
    "audit",
    "membership",
    "references"
  ],
  "additionalProperties": false
}
```
