# Storage and format reference

**Current release authority:** [current implementation inventory](../current-product.md), [user documentation](../user/index.md) and [launch status](../launch/status.md). Initial v0.1/roadmap delivery labels below preserve planning contracts; they do not establish today's implementation or permit deleting subsequently integrated features. [Open-source platform roadmap](../roadmap/platform/README.md) has separate design and gates.

**Delivery: v0.1 and roadmap.** [v0.1-build.md](v0.1-build.md) selects the current portion; [roadsmap.md](roadsmap.md) retains the remainder. Detailed schemas/configuration/ABI and full-family acceptance below describe the target superset unless narrowed by that allocation.

These are design definitions. Implement real migrations with their application/instance/target ownership, safe upgrade paths and recovery checks. Applying this text to a generic SQL parser is not native-engine qualification. The containing file/application supplies the context where references are file-local.

## Current owned workspace snapshot storage

The currently integrated workspace adapter in [agentfs.go](../../internal/application/agentfs.go) uses `_mb_agentfs_workspaces`, `_mb_agentfs_files`, `_mb_agentfs_snapshots` and `_mb_agentfs_history` in the application's primary database, with live bytes under `agentfs/{workspaceId}`. This is the owned adapter, not the full upstream AgentFS inode/chunk/KV schema.

Historical content is installed lazily with the following exact table:

```sql
CREATE TABLE IF NOT EXISTS _mb_agentfs_snapshot_bytes (
 snapshot_id TEXT NOT NULL REFERENCES _mb_agentfs_snapshots(id),
 path TEXT NOT NULL,
 content BLOB NOT NULL,
 PRIMARY KEY(snapshot_id,path)
);
```

Snapshot publication inserts its `_mb_agentfs_snapshots` manifest row and all content rows in one SQL transaction. SHA-256 values are in each manifest entry; the manifest's own SHA-256 is stored in its parent row. The total content admission limit is 64 MiB. Creation verifies current file sizes/hashes before publication. Restore validates all historical bytes, manifest hash and paths before publishing files individually. Nonempty legacy manifests without historical bytes fail explicitly. Files outside the restored set remain untouched. Multi-file crash atomicity, complete retention/garbage collection and full upstream AgentFS integration remain unqualified; see [current user contract](../user/agent-workspaces.md) and [executed findings](../launch/status.md).

## Instance metadata


```sql
-- Instance-only design DDL. Install lazily when application consolidation is enabled.
-- Default application setup does not require this database. One process owns it.
PRAGMA foreign_keys=ON;
CREATE TABLE _mb_applications (
 id TEXT PRIMARY KEY, generation INTEGER NOT NULL CHECK(generation>0), revision INTEGER NOT NULL CHECK(revision>0),
 state TEXT NOT NULL CHECK(state IN ('provisioning','ready','maintenance','suspended','restoring','deleting','quarantined','tombstoned','purged','failed')),
 layout TEXT NOT NULL CHECK(layout IN ('shared','user','tenant','agent')),
 root_relative TEXT NOT NULL UNIQUE, manifest_hash TEXT, artifact_hash TEXT,
 policy_epoch INTEGER NOT NULL CHECK(policy_epoch>0), created TEXT NOT NULL, updated TEXT NOT NULL
);
CREATE TABLE _mb_application_routes (
 route_kind TEXT NOT NULL CHECK(route_kind IN ('prefix','host')), route_key TEXT NOT NULL,
 application_id TEXT NOT NULL REFERENCES _mb_applications(id), revision INTEGER NOT NULL CHECK(revision>0),
 active INTEGER NOT NULL CHECK(active IN (0,1)), PRIMARY KEY(route_kind,route_key)
);
CREATE TABLE _mb_instance_operators (
 application_id TEXT NOT NULL REFERENCES _mb_applications(id), collection_id TEXT NOT NULL, record_id TEXT NOT NULL,
 scopes_json TEXT NOT NULL CHECK(json_valid(scopes_json)), revoked_at TEXT,
 PRIMARY KEY(application_id,collection_id,record_id)
);
CREATE TABLE _mb_application_artifacts (
 application_id TEXT NOT NULL REFERENCES _mb_applications(id), revision INTEGER NOT NULL CHECK(revision>0),
 digest TEXT NOT NULL, manifest_json TEXT NOT NULL CHECK(json_valid(manifest_json)),
 state TEXT NOT NULL CHECK(state IN ('staged','validated','active','retired')), PRIMARY KEY(application_id,revision)
);
CREATE TABLE _mb_application_publications (
 id TEXT PRIMARY KEY, application_id TEXT NOT NULL REFERENCES _mb_applications(id),
 expected_generation INTEGER NOT NULL CHECK(expected_generation>0), expected_revision INTEGER NOT NULL CHECK(expected_revision>0),
 phase TEXT NOT NULL CHECK(phase IN ('intent','staged','verified','renamed','routed','active','failed')),
 manifest_hash TEXT NOT NULL, staging_relative TEXT NOT NULL, recovery_relative TEXT,
 intent_json TEXT NOT NULL CHECK(json_valid(intent_json)), created TEXT NOT NULL, updated TEXT NOT NULL
);
CREATE TABLE _mb_import_batches (
 id TEXT PRIMARY KEY, revision INTEGER NOT NULL CHECK(revision>0),
 state TEXT NOT NULL CHECK(state IN ('draft','acquiring','planning','executing','complete','partial','failed','cancelled')),
 actor_json TEXT NOT NULL CHECK(json_valid(actor_json)),
 activation_policy TEXT NOT NULL CHECK(activation_policy='per-source-after-validation'), created TEXT NOT NULL, updated TEXT NOT NULL
);
CREATE TABLE _mb_import_batch_sources (
 batch_id TEXT NOT NULL REFERENCES _mb_import_batches(id), source_id TEXT NOT NULL,
 session_application_id TEXT NOT NULL, session_id TEXT NOT NULL, origin_id TEXT NOT NULL, source_hash TEXT NOT NULL,
 destination_application_id TEXT NOT NULL REFERENCES _mb_applications(id),
 repeat_policy TEXT NOT NULL CHECK(repeat_policy IN ('resume-existing','create-independent-copy','replace-matching-origin')),
 state TEXT NOT NULL, PRIMARY KEY(batch_id,source_id), UNIQUE(batch_id,destination_application_id),
 UNIQUE(session_application_id,session_id)
);
CREATE INDEX _mb_import_source_origin ON _mb_import_batch_sources(origin_id,source_hash);
CREATE TABLE _mb_resource_reservations (
 id TEXT PRIMARY KEY, application_id TEXT NOT NULL REFERENCES _mb_applications(id), owner_kind TEXT NOT NULL, owner_id TEXT NOT NULL,
 category TEXT NOT NULL CHECK(category IN ('upload','import','backup','restore','files','recovery')),
 bytes INTEGER NOT NULL CHECK(bytes>=0), state TEXT NOT NULL CHECK(state IN ('reserved','committed','reconciling','released')),
 expires_at TEXT, UNIQUE(application_id,owner_kind,owner_id,category)
);
-- Expiry cannot release an uncertain reservation before reconciling actual files/jobs.
CREATE TABLE _mb_application_bootstrap_tokens (
 id TEXT PRIMARY KEY, application_id TEXT NOT NULL REFERENCES _mb_applications(id),
 generation INTEGER NOT NULL CHECK(generation>0), token_hash TEXT NOT NULL UNIQUE,
 state TEXT NOT NULL CHECK(state IN ('available','consuming','used','revoked')),
 expires_at TEXT NOT NULL, receipt_id TEXT
);
CREATE TABLE _mb_import_origins (
 origin_id TEXT NOT NULL, source_hash TEXT NOT NULL, copy_key TEXT NOT NULL,
 session_application_id TEXT NOT NULL, session_id TEXT NOT NULL,
 destination_application_id TEXT NOT NULL REFERENCES _mb_applications(id),
 state TEXT NOT NULL CHECK(state IN ('reserved','staging','active','failed','retired')),
 PRIMARY KEY(origin_id,source_hash,copy_key)
);
```


## Application, target and auxiliary metadata


```sql
-- Normative design DDL, not an executed migration or implementation claim.
-- Apply PRIMARY section to the app DB when optional lifecycle features are enabled.
-- Apply TARGET section to each enabled managed target, including primary if selected.
-- TEXT *_json values are canonical UTF-8 JSON, validated against the owned wire schema.
-- Timestamps are UTC RFC3339. *_until_ms are UTC epoch milliseconds for leases.
-- Global auth references are composite application references, not foreign keys to guessed tables.

-- PRIMARY
CREATE TABLE _mb_databases (
 id TEXT PRIMARY KEY,
 owner_collection_id TEXT,
 owner_record_id TEXT,
 kind TEXT NOT NULL CHECK(kind IN ('primary','manual','tenant','user','agent')),
 state TEXT NOT NULL CHECK(state IN ('requested','provisioning','ready','failed','maintenance','migrating','restoring','deleting','tombstoned','purged','quarantined')),
 generation INTEGER NOT NULL CHECK(generation > 0),
 profile TEXT NOT NULL CHECK(profile IN ('wal','mvcc')),
 template_id TEXT,
 schema_revision INTEGER NOT NULL CHECK(schema_revision > 0),
 policy_epoch INTEGER NOT NULL CHECK(policy_epoch > 0),
 display_name TEXT NOT NULL,
 created TEXT NOT NULL,
 updated TEXT NOT NULL,
 last_error_code TEXT,
 CHECK((owner_collection_id IS NULL) = (owner_record_id IS NULL))
);
CREATE INDEX _mb_databases_owner ON _mb_databases(owner_collection_id,owner_record_id,kind);
CREATE INDEX _mb_databases_state ON _mb_databases(state,id);
CREATE TABLE _mb_database_grants (
 id TEXT PRIMARY KEY,
 database_id TEXT NOT NULL REFERENCES _mb_databases(id),
 subject_collection_id TEXT NOT NULL,
 subject_record_id TEXT NOT NULL,
 scopes_json TEXT NOT NULL CHECK(json_valid(scopes_json)),
 expires_at TEXT,
 revoked_at TEXT,
 epoch INTEGER NOT NULL CHECK(epoch > 0),
 UNIQUE(database_id,subject_collection_id,subject_record_id)
);
CREATE TABLE _mb_agent_bindings (
 id TEXT PRIMARY KEY,
 auth_collection_id TEXT NOT NULL,
 auth_record_id TEXT NOT NULL,
 owner_collection_id TEXT NOT NULL,
 owner_record_id TEXT NOT NULL,
 registration_state TEXT NOT NULL CHECK(registration_state IN ('active','revoked')),
 created TEXT NOT NULL,
 UNIQUE(auth_collection_id,auth_record_id)
);
CREATE INDEX _mb_agents_owner ON _mb_agent_bindings(owner_collection_id,owner_record_id);
CREATE TABLE _mb_jobs (
 id TEXT PRIMARY KEY,
 kind TEXT NOT NULL,
 database_id TEXT,
 request_hash TEXT NOT NULL,
 idempotency_key TEXT NOT NULL,
 actor_ref_json TEXT NOT NULL CHECK(json_valid(actor_ref_json)),
 state TEXT NOT NULL CHECK(state IN ('queued','running','waiting_retry','succeeded','failed','cancelled')),
 phase TEXT NOT NULL,
 progress_counters_json TEXT NOT NULL CHECK(json_valid(progress_counters_json)),
 lease_owner TEXT,
 lease_until_ms INTEGER,
 attempts INTEGER NOT NULL DEFAULT 0,
 cancel_requested INTEGER NOT NULL DEFAULT 0 CHECK(cancel_requested IN (0,1)),
 result_ref TEXT,
 error_code TEXT,
 created TEXT NOT NULL,
 updated TEXT NOT NULL
);
CREATE UNIQUE INDEX _mb_jobs_idempotency ON _mb_jobs(kind,COALESCE(database_id,'primary'),actor_ref_json,idempotency_key);
CREATE INDEX _mb_jobs_admission ON _mb_jobs(state,lease_until_ms,created);
CREATE TABLE _mb_schema_templates (
 id TEXT NOT NULL,
 revision INTEGER NOT NULL CHECK(revision > 0),
 definitions_hash TEXT NOT NULL,
 definitions_json TEXT NOT NULL CHECK(json_valid(definitions_json)),
 requirements_json TEXT NOT NULL CHECK(json_valid(requirements_json)),
 publication_state TEXT NOT NULL CHECK(publication_state IN ('draft','published')),
 PRIMARY KEY(id,revision)
);
CREATE TABLE _mb_schema_rollouts (
 id TEXT PRIMARY KEY,
 template_id TEXT NOT NULL,
 revision INTEGER NOT NULL,
 target_snapshot_json TEXT NOT NULL CHECK(json_valid(target_snapshot_json)),
 results_json TEXT NOT NULL CHECK(json_valid(results_json)),
 stop_on_failure INTEGER NOT NULL CHECK(stop_on_failure IN (0,1)),
 created TEXT NOT NULL,
 FOREIGN KEY(template_id,revision) REFERENCES _mb_schema_templates(id,revision)
);
CREATE TABLE _mb_credentials (
 id TEXT PRIMARY KEY,
 database_id TEXT NOT NULL REFERENCES _mb_databases(id),
 generation INTEGER NOT NULL CHECK(generation > 0),
 purpose TEXT NOT NULL CHECK(purpose IN ('pg','native-sync','sql-http')),
 verifier BLOB NOT NULL,
 scopes_json TEXT NOT NULL CHECK(json_valid(scopes_json)),
 expires_at TEXT NOT NULL,
 revoked_at TEXT,
 policy_epoch INTEGER NOT NULL CHECK(policy_epoch > 0)
);

-- PRIMARY OPTIONAL IMPORT/TENANT ROUTING
CREATE TABLE _mb_import_sessions (
 id TEXT PRIMARY KEY,
 actor_ref_json TEXT NOT NULL CHECK(json_valid(actor_ref_json)),
 state TEXT NOT NULL CHECK(state IN ('awaiting-source','source-ready','analyzing','draft','previewing','previewed','staging','staged','validating','ready-for-cutover','cutting-over','active','failed','cancelled','rolling-back','rolled-back','recovery-exported','retired')),
 revision INTEGER NOT NULL CHECK(revision > 0),
 source_kind TEXT NOT NULL CHECK(source_kind IN ('pocketbase-backup','pocketbase-directory')),
 target_mode TEXT NOT NULL CHECK(target_mode IN ('new-application','replace-application','append-application')),
 source_bytes INTEGER NOT NULL CHECK(source_bytes >= 0),
 source_hash TEXT,
 normalized_snapshot_hash TEXT,
 plan_hash TEXT,
 preview_hash TEXT,
 validation_hash TEXT,
 source_ref TEXT,
 plan_ref TEXT,
 staging_ref TEXT,
 recovery_ref TEXT,
 expected_primary_generation INTEGER NOT NULL CHECK(expected_primary_generation > 0),
 current_job_id TEXT,
 last_error_code TEXT,
 created TEXT NOT NULL,
 updated TEXT NOT NULL,
 expires_at TEXT
);
CREATE INDEX _mb_import_sessions_state ON _mb_import_sessions(state,updated);
CREATE TABLE _mb_tenant_routes (
 namespace TEXT PRIMARY KEY,
 active_epoch INTEGER NOT NULL DEFAULT 0 CHECK(active_epoch >= 0),
 policy_epoch INTEGER NOT NULL CHECK(policy_epoch > 0),
 policy_json TEXT NOT NULL CHECK(json_valid(policy_json)),
 template_id TEXT NOT NULL,
 active_import_session_id TEXT,
 created TEXT NOT NULL,
 updated TEXT NOT NULL
);
CREATE TABLE _mb_tenant_bindings (
 namespace TEXT NOT NULL REFERENCES _mb_tenant_routes(namespace),
 epoch INTEGER NOT NULL CHECK(epoch > 0),
 tenant_key_hash TEXT NOT NULL,
 tenant_key_json TEXT NOT NULL CHECK(json_valid(tenant_key_json)),
 template_id TEXT NOT NULL,
 database_id TEXT NOT NULL REFERENCES _mb_databases(id),
 generation INTEGER NOT NULL CHECK(generation > 0),
 PRIMARY KEY(namespace,epoch,tenant_key_hash,template_id),
 UNIQUE(namespace,epoch,database_id)
);
CREATE TABLE _mb_unique_reservations (
 namespace TEXT NOT NULL,
 index_id TEXT NOT NULL,
 value_hash TEXT NOT NULL,
 value_blob BLOB NOT NULL,
 database_id TEXT NOT NULL REFERENCES _mb_databases(id),
 generation INTEGER NOT NULL CHECK(generation > 0),
 record_id TEXT NOT NULL,
 transaction_tag TEXT NOT NULL,
 state TEXT NOT NULL CHECK(state IN ('reserved','committed','releasing')),
 lease_until_ms INTEGER,
 updated TEXT NOT NULL,
 PRIMARY KEY(namespace,index_id,value_hash)
);
CREATE INDEX _mb_unique_reservations_recovery ON _mb_unique_reservations(state,lease_until_ms);

-- TARGET: metadata only; collection tables are produced by the schema planner.
CREATE TABLE _mb_schema (
 singleton INTEGER PRIMARY KEY CHECK(singleton = 1),
 format_version INTEGER NOT NULL CHECK(format_version = 1),
 template_id TEXT,
 revision INTEGER NOT NULL CHECK(revision > 0),
 definitions_hash TEXT NOT NULL
);
CREATE TABLE _mb_changes (
 commit_sequence INTEGER PRIMARY KEY AUTOINCREMENT,
 transaction_id TEXT NOT NULL UNIQUE,
 generation INTEGER NOT NULL CHECK(generation > 0),
 schema_revision INTEGER NOT NULL CHECK(schema_revision > 0),
 committed_at TEXT NOT NULL,
 source TEXT NOT NULL CHECK(source IN ('record','admin-sql','pg','native-sync','migration')),
 encoding_version INTEGER NOT NULL CHECK(encoding_version > 0)
);
CREATE TABLE _mb_change_rows (
 commit_sequence INTEGER NOT NULL REFERENCES _mb_changes(commit_sequence) ON DELETE CASCADE,
 ordinal INTEGER NOT NULL,
 collection_id TEXT,
 table_name TEXT NOT NULL,
 record_id TEXT NOT NULL,
 action TEXT NOT NULL CHECK(action IN ('create','update','delete','schema','evict')),
 before_value BLOB,
 after_value BLOB,
 visibility_metadata BLOB,
 PRIMARY KEY(commit_sequence,ordinal)
);
CREATE TABLE _mb_record_versions (
 collection_id TEXT NOT NULL,
 record_id TEXT NOT NULL,
 version INTEGER NOT NULL CHECK(version > 0),
 deleted INTEGER NOT NULL CHECK(deleted IN (0,1)),
 PRIMARY KEY(collection_id,record_id)
);
CREATE TABLE _mb_sync_clients (
 id TEXT PRIMARY KEY,
 subject_collection_id TEXT NOT NULL,
 subject_record_id TEXT NOT NULL,
 generation INTEGER NOT NULL,
 policy_epoch INTEGER NOT NULL,
 applied_sequence INTEGER NOT NULL DEFAULT 0,
 lease_until_ms INTEGER NOT NULL,
 last_seen TEXT NOT NULL
);
CREATE TABLE _mb_receipts (
 id TEXT PRIMARY KEY,
 actor_ref TEXT NOT NULL,
 generation INTEGER NOT NULL,
 operation_id TEXT NOT NULL,
 idempotency_key TEXT NOT NULL,
 request_hash TEXT NOT NULL,
 result_json TEXT NOT NULL CHECK(json_valid(result_json)),
 commit_sequence INTEGER,
 expires_at TEXT NOT NULL,
 UNIQUE(actor_ref,generation,operation_id,idempotency_key)
);
CREATE TABLE _mb_outbox (
 id TEXT PRIMARY KEY,
 transaction_id TEXT NOT NULL,
 effect_kind TEXT NOT NULL,
 payload_version INTEGER NOT NULL,
 encrypted_payload BLOB NOT NULL,
 state TEXT NOT NULL CHECK(state IN ('queued','running','waiting_retry','succeeded','failed','cancelled')),
 attempts INTEGER NOT NULL DEFAULT 0,
 next_attempt_ms INTEGER NOT NULL,
 lease_until_ms INTEGER,
 error_code TEXT
);
CREATE INDEX _mb_outbox_admission ON _mb_outbox(state,next_attempt_ms,lease_until_ms);

-- APPLICATION-LOCAL OPTIONAL SERVICES, scoped by the containing application database.
CREATE TABLE _mb_setup_plans (
 id TEXT PRIMARY KEY, revision INTEGER NOT NULL CHECK(revision>0), application_generation INTEGER NOT NULL CHECK(application_generation>0),
 plan_hash TEXT NOT NULL, definition_json TEXT NOT NULL CHECK(json_valid(definition_json)),
 state TEXT NOT NULL CHECK(state IN ('draft','validating','invalid','ready','applying','active','failed','cancelled-before-publication'))
);
CREATE TABLE _mb_bootstrap_receipts (
 token_id TEXT PRIMARY KEY, application_generation INTEGER NOT NULL CHECK(application_generation>0),
 superuser_record_id TEXT NOT NULL, request_hash TEXT NOT NULL, committed_at TEXT NOT NULL
);
CREATE TABLE _mb_membership_invites (
 id TEXT PRIMARY KEY, revision INTEGER NOT NULL DEFAULT 1 CHECK(revision>0), token_hash TEXT NOT NULL UNIQUE, namespace TEXT NOT NULL,
 tenant_key_json TEXT NOT NULL CHECK(json_valid(tenant_key_json)), intended_email_hash TEXT,
 subject_collection_id TEXT, subject_record_id TEXT,
 membership_values_json TEXT NOT NULL CHECK(json_valid(membership_values_json)), inviter_json TEXT NOT NULL CHECK(json_valid(inviter_json)),
 expires_at TEXT NOT NULL, used_at TEXT, revoked_at TEXT,
 CHECK(intended_email_hash IS NOT NULL OR (subject_collection_id IS NOT NULL AND subject_record_id IS NOT NULL)),
 CHECK((subject_collection_id IS NULL)=(subject_record_id IS NULL))
);
CREATE TABLE _mb_owner_transfers (
 id TEXT PRIMARY KEY, database_id TEXT NOT NULL REFERENCES _mb_databases(id), expected_generation INTEGER NOT NULL CHECK(expected_generation>0),
 previous_owner_json TEXT, new_owner_json TEXT NOT NULL CHECK(json_valid(new_owner_json)), policy_epoch INTEGER NOT NULL CHECK(policy_epoch>0),
 agent_policy TEXT NOT NULL CHECK(agent_policy IN ('revoke','rebind-reviewed')),
 phase TEXT NOT NULL CHECK(phase IN ('intent','fenced','revoked','bound','complete','failed'))
);
CREATE TABLE _mb_reference_policies (
 id TEXT PRIMARY KEY, revision INTEGER NOT NULL CHECK(revision>0), definition_json TEXT NOT NULL CHECK(json_valid(definition_json)),
 state TEXT NOT NULL CHECK(state IN ('active','rebuilding','retired','failed'))
);
CREATE TABLE _mb_reference_targets (
 policy_id TEXT NOT NULL REFERENCES _mb_reference_policies(id), database_id TEXT NOT NULL REFERENCES _mb_databases(id),
 generation INTEGER NOT NULL CHECK(generation>0), policy_revision INTEGER NOT NULL CHECK(policy_revision>0), applied_sequence INTEGER NOT NULL DEFAULT 0,
 state TEXT NOT NULL CHECK(state IN ('pending','refreshing','healthy','stale','failed')), error_code TEXT,
 PRIMARY KEY(policy_id,database_id,generation)
);
CREATE TABLE _mb_live_plans (
 id TEXT NOT NULL, revision INTEGER NOT NULL CHECK(revision>0), sql_hash TEXT NOT NULL,
 definition_json TEXT NOT NULL CHECK(json_valid(definition_json)), state TEXT NOT NULL CHECK(state IN ('draft','published','retired')),
 PRIMARY KEY(id,revision)
);
CREATE TABLE _mb_schema_rollout_targets (
 rollout_id TEXT NOT NULL REFERENCES _mb_schema_rollouts(id), database_id TEXT NOT NULL, generation INTEGER NOT NULL CHECK(generation>0),
 state TEXT NOT NULL CHECK(state IN ('pending','running','succeeded','failed','skipped','cancelled')),
 revision INTEGER NOT NULL CHECK(revision>0), result_ref TEXT, error_code TEXT, PRIMARY KEY(rollout_id,database_id)
);
CREATE INDEX _mb_rollout_targets_admission ON _mb_schema_rollout_targets(rollout_id,state,database_id);
CREATE TABLE _mb_relation_workflows (
 id TEXT PRIMARY KEY, revision INTEGER NOT NULL CHECK(revision>0), definition_json TEXT NOT NULL CHECK(json_valid(definition_json))
);
CREATE TABLE _mb_reports (
 id TEXT PRIMARY KEY, revision INTEGER NOT NULL CHECK(revision>0), definition_json TEXT NOT NULL CHECK(json_valid(definition_json)),
 state TEXT NOT NULL CHECK(state IN ('building','ready','stale','failed','retired'))
);
CREATE TABLE _mb_report_checkpoints (
 report_id TEXT NOT NULL REFERENCES _mb_reports(id), database_id TEXT NOT NULL,
 generation INTEGER NOT NULL CHECK(generation>0), applied_sequence INTEGER NOT NULL DEFAULT 0,
 PRIMARY KEY(report_id,database_id,generation)
);
CREATE TABLE _mb_layout_migrations (
 id TEXT PRIMARY KEY, plan_hash TEXT NOT NULL, expected_application_generation INTEGER NOT NULL CHECK(expected_application_generation>0),
 destination_layout TEXT NOT NULL CHECK(destination_layout IN ('shared','user','tenant','agent')), backup_id TEXT NOT NULL,
 phase TEXT NOT NULL, journal_ref TEXT NOT NULL
);
-- AUXILIARY: this table belongs in application auxiliary.db, not public record storage.
CREATE TABLE _mb_audit (
 id TEXT PRIMARY KEY, actor_json TEXT NOT NULL CHECK(json_valid(actor_json)), operation_id TEXT NOT NULL,
 target_json TEXT NOT NULL CHECK(json_valid(target_json)), request_hash TEXT NOT NULL, outcome TEXT NOT NULL, created TEXT NOT NULL
);

-- Native CDC owns its binary-format capture table via the pinned native API.
-- It must not recursively capture these internal receipts/change/outbox tables.
-- Permission enforcement, immutable template publication, owner uniqueness policy,
-- path validation and request-hash conflict checks are service invariants in addition to DDL.
```


## Offline client storage


```sql
-- Client-store design DDL; local engine qualification is separate.
PRAGMA foreign_keys=ON;
CREATE TABLE _mb_local_manifest (
 singleton INTEGER PRIMARY KEY CHECK(singleton=1), server_origin TEXT NOT NULL, application_id TEXT NOT NULL,
 application_generation INTEGER NOT NULL CHECK(application_generation>0), subject_collection_id TEXT NOT NULL, subject_record_id TEXT NOT NULL,
 database_id TEXT NOT NULL, database_generation INTEGER NOT NULL CHECK(database_generation>0), projection_hash TEXT NOT NULL,
 schema_revision INTEGER NOT NULL CHECK(schema_revision>0), policy_epoch INTEGER NOT NULL CHECK(policy_epoch>0), applied_cursor TEXT,
 engine_format TEXT NOT NULL, key_ref TEXT NOT NULL, receipt_horizon TEXT
);
CREATE TABLE _mb_local_pending (
 group_id TEXT PRIMARY KEY, canonical_hash TEXT NOT NULL, encrypted_intent BLOB NOT NULL, encryption_nonce BLOB NOT NULL,
 state TEXT NOT NULL CHECK(state IN ('queued','sending','committed','conflict','rejected','outcome_unknown')),
 base_generation INTEGER NOT NULL CHECK(base_generation>0), expires_at TEXT, lineage_group_id TEXT, created TEXT NOT NULL
);
CREATE TABLE _mb_local_receipts (
 group_id TEXT PRIMARY KEY REFERENCES _mb_local_pending(group_id), canonical_hash TEXT NOT NULL,
 result_json TEXT NOT NULL CHECK(json_valid(result_json)), committed_cursor TEXT, received_at TEXT NOT NULL
);
CREATE TABLE _mb_local_file_intents (
 id TEXT PRIMARY KEY, group_id TEXT NOT NULL REFERENCES _mb_local_pending(group_id), content_hash TEXT NOT NULL,
 bytes INTEGER NOT NULL CHECK(bytes>=0), encrypted_content_ref TEXT NOT NULL,
 state TEXT NOT NULL CHECK(state IN ('pending','uploading','staged','finalized','failed'))
);
-- Authorized record tables are generated from the current projected schema.
-- Queue/view and base/cursor updates each commit atomically in this same store.
```


## Database manifest and lifecycle format


```json
{
  "formatVersion": 1,
  "manifestSchema": {
    "type": "object",
    "properties": {
      "formatVersion": {
        "const": 1
      },
      "databaseId": {
        "$ref": "urn:mirabase:wire#/$defs/DatabaseId"
      },
      "generation": {
        "$ref": "urn:mirabase:wire#/$defs/Revision"
      },
      "profile": {
        "$ref": "urn:mirabase:wire#/$defs/Profile"
      },
      "schemaRevision": {
        "$ref": "urn:mirabase:wire#/$defs/Revision"
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
      "encryptionKeyRef": {
        "anyOf": [
          {
            "type": "string"
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
      },
      "created": {
        "$ref": "urn:mirabase:wire#/$defs/Timestamp"
      },
      "applicationId": {
        "$ref": "urn:mirabase:wire#/$defs/ApplicationId"
      },
      "applicationGeneration": {
        "$ref": "urn:mirabase:wire#/$defs/Revision"
      },
      "artifactManifestHash": {
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
      "formatVersion",
      "databaseId",
      "generation",
      "profile",
      "schemaRevision",
      "templateId",
      "encryptionKeyRef",
      "codecId",
      "created",
      "applicationId",
      "applicationGeneration",
      "artifactManifestHash"
    ],
    "additionalProperties": false
  },
  "schemaRefs": "wire-schema-reference.md#/$defs",
  "registryDDL": "storage-format-reference.md",
  "privateDDL": "storage-format-reference.md",
  "formats": {
    "nativeDatabase": "engine-defined; revision/profile/codec recorded; never guess sidecars",
    "nativeCdc": "version-pinned binary encoding normalized to ChangeTransaction",
    "recordSync": "MiraBase v1 wire schemas",
    "nativeSync": "pinned client_proto/server_proto message definitions",
    "portableArchive": "portable-v1 manifest plus typed records and file hashes",
    "backupArchive": "backup-v1 coherent snapshot manifest plus SHA-256 entries"
  },
  "ownership": {
    "primary": [
      "auth identities",
      "settings",
      "collection definitions",
      "registry",
      "grants",
      "templates",
      "jobs",
      "import sessions",
      "tenant route epochs/bindings",
      "global unique reservations"
    ],
    "private": [
      "selected application tables",
      "schema revision",
      "changes",
      "receipts",
      "sync client leases",
      "outbox"
    ]
  },
  "states": [
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


## Backup and portable archive format


```json
{
  "formatVersion": 1,
  "manifestSchema": {
    "type": "object",
    "properties": {
      "format": {
        "type": "string",
        "enum": [
          "backup-v1",
          "portable-v1"
        ]
      },
      "created": {
        "$ref": "urn:mirabase:wire#/$defs/Timestamp"
      },
      "productVersion": {
        "type": "string"
      },
      "engineRevision": {
        "type": "string"
      },
      "abiVersion": {
        "type": "string"
      },
      "contexts": {
        "type": "array",
        "items": {
          "type": "object",
          "properties": {
            "id": {
              "$ref": "urn:mirabase:wire#/$defs/DatabaseId"
            },
            "generation": {
              "$ref": "urn:mirabase:wire#/$defs/Revision"
            },
            "schemaRevision": {
              "$ref": "urn:mirabase:wire#/$defs/Revision"
            },
            "profile": {
              "$ref": "urn:mirabase:wire#/$defs/Profile"
            },
            "keyRef": {
              "anyOf": [
                {
                  "type": "string"
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
            },
            "snapshotBoundary": {
              "type": "string"
            }
          },
          "required": [
            "id",
            "generation",
            "schemaRevision",
            "profile",
            "keyRef",
            "codecId",
            "snapshotBoundary"
          ],
          "additionalProperties": false
        },
        "maxItems": 1000000
      },
      "entries": {
        "type": "array",
        "items": {
          "type": "object",
          "properties": {
            "path": {
              "type": "string"
            },
            "role": {
              "type": "string",
              "enum": [
                "native-db",
                "native-sidecar",
                "local-file",
                "schema",
                "records",
                "settings",
                "migration",
                "hook",
                "public",
                "index"
              ]
            },
            "bytes": {
              "type": "integer",
              "minimum": 0,
              "maximum": 9007199254740991
            },
            "sha256": {
              "type": "string",
              "pattern": "^[a-f0-9]{64}$"
            },
            "contextId": {
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
            "path",
            "role",
            "bytes",
            "sha256",
            "contextId"
          ],
          "additionalProperties": false
        },
        "maxItems": 10000000
      },
      "fileCoverage": {
        "type": "string",
        "enum": [
          "local-complete",
          "remote-version-manifest",
          "db-only",
          "partial"
        ]
      },
      "remoteObjects": {
        "type": "array",
        "items": {
          "type": "object",
          "properties": {
            "backend": {
              "type": "string"
            },
            "key": {
              "type": "string"
            },
            "versionId": {
              "anyOf": [
                {
                  "type": "string"
                },
                {
                  "type": "null"
                }
              ]
            },
            "bytes": {
              "type": "integer",
              "minimum": 0,
              "maximum": 9007199254740991
            },
            "sha256": {
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
            "backend",
            "key",
            "versionId",
            "bytes",
            "sha256"
          ],
          "additionalProperties": false
        },
        "maxItems": 10000000
      },
      "exclusions": {
        "type": "array",
        "items": {
          "type": "string"
        },
        "maxItems": 10000
      },
      "applicationId": {
        "$ref": "urn:mirabase:wire#/$defs/ApplicationId"
      },
      "applicationGeneration": {
        "$ref": "urn:mirabase:wire#/$defs/Revision"
      },
      "artifactManifestHash": {
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
      "format",
      "created",
      "productVersion",
      "engineRevision",
      "abiVersion",
      "contexts",
      "entries",
      "fileCoverage",
      "remoteObjects",
      "exclusions",
      "applicationId",
      "applicationGeneration",
      "artifactManifestHash"
    ],
    "additionalProperties": false
  },
  "portableRecords": "UTF-8 NDJSON: collectionId, recordId, values using declared field codecs, relation references and file manifest IDs. Includes hidden nonsecret application fields only under superuser export; auth secrets require an explicitly protected adoption/backup format, not ordinary user export.",
  "rules": [
    "No absolute/parent/duplicate archive paths",
    "Declared extracted-size budget before allocation",
    "SHA-256 verifies actual bytes",
    "Do not include native key bytes by default",
    "Restore changes generation",
    "No provider migration adapter implied"
  ]
}
```
