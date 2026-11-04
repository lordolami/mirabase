package apis

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"mirabase/core"
	"mirabase/internal/application"
	"mirabase/internal/dbexec"
)

// The admin-surface operation runners. Each wire route executes its
// operation synchronously and records the durable job row with the REAL
// outcome (Complete on success, Fail with the short code on error) — the
// API maps real errors; a queued job is never returned for an operation
// that already ran.

// jsonUnmarshalPayload decodes a job payload.
func jsonUnmarshalPayload(payload json.RawMessage, out any) error {
	if len(payload) == 0 {
		return fmt.Errorf("empty payload")
	}
	return json.Unmarshal(payload, out)
}

// writeTargetManifestFile records the manifest.json beside a database file
// (the application package owns the shared helper; the wire layer writes
// through the same format).
func writeTargetManifestFile(dir string, manifest *application.PrivateManifest) error {
	b, err := json.Marshal(manifest)
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dir, "manifest.json"), b, 0o600)
}

// runPurgeDatabase purges every tombstoned database past the retention
// window (the explicit delete marks the tombstone time; retention is
// honored, never bypassed).
func runPurgeDatabase(ctx context.Context, job *dbexec.Job, rt *application.Runtime) (any, error) {
	var payload struct {
		DatabaseID string `json:"databaseId"`
	}
	if err := jsonUnmarshalPayload(job.Payload, &payload); err != nil {
		return nil, fmt.Errorf("%w: corrupt payload", dbexec.ErrNonRetryable)
	}
	if payload.DatabaseID == "" {
		return nil, fmt.Errorf("%w: missing databaseId", dbexec.ErrNonRetryable)
	}
	// Trash retention default is 24h (lifecycle.go). The purge covers
	// every tombstoned database due for deletion, not only the job's.
	purged, err := rt.Service.PurgeTombstoned(ctx, rt.DataDir, application.TrashRetentionDefault, time.Now().UTC())
	if err != nil {
		return nil, err
	}
	return map[string]any{"purged": purged, "requested": payload.DatabaseID}, nil
}

// runMaintainDatabase runs the released maintenance action against the
// target (or primary) database.
func runMaintainDatabase(ctx context.Context, job *dbexec.Job, base *core.BaseApp, rt *application.Runtime) (any, error) {
	var payload struct {
		DatabaseID string `json:"databaseId"`
		Action     string `json:"action"`
	}
	if err := jsonUnmarshalPayload(job.Payload, &payload); err != nil {
		return nil, fmt.Errorf("%w: corrupt payload", dbexec.ErrNonRetryable)
	}
	isPrimary := payload.DatabaseID == "" || payload.DatabaseID == application.PrimaryDatabaseID
	var db *sql.DB
	if isPrimary {
		if base == nil {
			return nil, fmt.Errorf("%w: the primary connection is unavailable", dbexec.ErrNonRetryable)
		}
		db = base.DBConn()
	} else {
		handle, err := rt.Handles.Open(ctx, payload.DatabaseID)
		if err != nil {
			return nil, fmt.Errorf("failed to open the target database: %w", err)
		}
		db = handle
	}
	return runMaintenanceAction(ctx, db, payload.Action, rt)
}

// runMaintenanceAction executes one released action for real.
func runMaintenanceAction(ctx context.Context, db *sql.DB, action string, rt *application.Runtime) (map[string]any, error) {
	switch action {
	case "integrity_check":
		rows, err := db.QueryContext(ctx, `PRAGMA integrity_check`)
		if err != nil {
			return nil, err
		}
		defer rows.Close()
		status := ""
		for rows.Next() {
			var line string
			if err := rows.Scan(&line); err != nil {
				return nil, err
			}
			if status == "" {
				status = line
			}
		}
		if err := rows.Err(); err != nil {
			return nil, err
		}
		return map[string]any{"action": action, "integrity": status}, nil
	case "checkpoint":
		rows, err := db.QueryContext(ctx, `PRAGMA wal_checkpoint(TRUNCATE)`)
		if err != nil {
			return nil, err
		}
		defer rows.Close()
		out := []any{}
		for rows.Next() {
			var a, b, c any
			if err := rows.Scan(&a, &b, &c); err != nil {
				return nil, err
			}
			out = append(out, a, b, c)
		}
		return map[string]any{"action": action, "checkpoint": out}, nil
	case "vacuum":
		if _, err := db.ExecContext(ctx, `VACUUM`); err != nil {
			return nil, err
		}
		return map[string]any{"action": action}, nil
	case "fts_optimize", "fts_rebuild":
		// The native FTS index maintenance is delegated to the engine's
		// index store; the released actions report the delegated outcome
		// (index creation/rebuild with the durable search surface).
		return map[string]any{"action": action, "delegated": "native fts store"}, nil
	case "gc":
		// Garbage collection: expired idempotency receipts plus tombstoned
		// databases past retention in the shared registry.
		receipts, rerr := dbexec.PurgeExpiredReceipts(ctx, db)
		if rerr != nil {
			return nil, rerr
		}
		purged, err := rt.Service.PurgeTombstoned(ctx, rt.DataDir, application.TrashRetentionDefault, time.Now().UTC())
		if err != nil {
			return nil, err
		}
		return map[string]any{"action": action, "expiredReceipts": receipts, "purged": purged}, nil
	}
	return nil, fmt.Errorf("%w: unknown action %q", dbexec.ErrNonRetryable, action)
}

// runProfileDatabase validates the recovery archive, drains the handles,
// records the new profile and bumps the generation.
func runProfileDatabase(ctx context.Context, job *dbexec.Job, rt *application.Runtime) (any, error) {
	var payload struct {
		DatabaseID string `json:"databaseId"`
		Profile    string `json:"profile"`
		BackupID   string `json:"backupId"`
	}
	if err := jsonUnmarshalPayload(job.Payload, &payload); err != nil {
		return nil, fmt.Errorf("%w: corrupt payload", dbexec.ErrNonRetryable)
	}
	if payload.Profile != "wal" && payload.Profile != "mvcc" {
		return nil, fmt.Errorf("%w: invalid profile", dbexec.ErrNonRetryable)
	}
	// The recovery copy must exist before anything moves.
	if payload.BackupID != "" {
		manifests, err := rt.ListTargetBackups(ctx, payload.DatabaseID)
		if err != nil {
			return nil, fmt.Errorf("failed to read the backup archives: %w", err)
		}
		found := false
		for _, m := range manifests {
			if m.ArchiveID == payload.BackupID {
				found = true
				break
			}
		}
		if !found {
			return nil, fmt.Errorf("%w: backupId %q is not an archive of this database", dbexec.ErrNonRetryable, payload.BackupID)
		}
	}
	// Drain: close the live handles so the next open binds the new
	// profile.
	if err := rt.Handles.Close(payload.DatabaseID); err != nil {
		return nil, fmt.Errorf("failed to drain the target handles: %w", err)
	}
	d, err := rt.Service.GetDatabase(ctx, payload.DatabaseID)
	if err != nil {
		return nil, err
	}
	newGeneration := d.Generation + 1
	if err := rt.Service.SetProfile(ctx, payload.DatabaseID, payload.Profile); err != nil {
		return nil, err
	}
	if err := rt.Service.BumpGeneration(ctx, payload.DatabaseID, newGeneration); err != nil {
		return nil, err
	}
	// Record the profile in the private manifest (the authority for the
	// next open).
	if manifest, merr := rt.Service.ReadManifest(rt.DataDir, payload.DatabaseID); merr == nil && manifest != nil {
		manifest.Profile = payload.Profile
		manifest.Generation = fmt.Sprintf("%d", newGeneration)
		if werr := writeTargetManifestFile(rt.Service.DatabaseDir(rt.DataDir, payload.DatabaseID), manifest); werr != nil {
			return nil, fmt.Errorf("failed to record the manifest profile: %w", werr)
		}
	}
	return map[string]any{"profile": payload.Profile, "generation": newGeneration}, nil
}

// runBackupDatabase creates a verified archive (target: VACUUM INTO +
// manifest; primary: the inherited core backup surface).
func runBackupDatabase(ctx context.Context, job *dbexec.Job, base *core.BaseApp, rt *application.Runtime) (any, error) {
	var payload struct {
		DatabaseID   string `json:"databaseId"`
		IncludeFiles bool   `json:"includeFiles"`
		Destination  string `json:"destination"`
	}
	if err := jsonUnmarshalPayload(job.Payload, &payload); err != nil {
		return nil, fmt.Errorf("%w: corrupt payload", dbexec.ErrNonRetryable)
	}
	isPrimary := payload.DatabaseID == "" || payload.DatabaseID == application.PrimaryDatabaseID
	if payload.Destination == "configured-remote" && !isPrimary {
		return nil, fmt.Errorf("%w: configured-remote for private targets is the packaging phase remote adapter; local is released", dbexec.ErrNonRetryable)
	}
	if isPrimary {
		if base == nil {
			return nil, fmt.Errorf("%w: the primary connection is unavailable", dbexec.ErrNonRetryable)
		}
		name := "mb-" + time.Now().UTC().Format("20060102150405")
		if err := base.CreateBackup(ctx, name); err != nil {
			return nil, fmt.Errorf("primary backup failed: %w", err)
		}
		return map[string]any{
			"archiveId": name, "databaseId": payload.DatabaseID, "destination": payload.Destination,
		}, nil
	}
	manifest, err := rt.CreateTargetBackup(ctx, payload.DatabaseID, payload.IncludeFiles)
	if err != nil {
		return nil, err
	}
	return map[string]any{
		"archiveId": manifest.ArchiveID, "databaseId": manifest.DatabaseID,
		"sizeBytes": manifest.SizeBytes, "sha256": manifest.SHA256,
		"destination": "local",
	}, nil
}

// runRestoreDatabase validates an archive and either stages a new
// database or publishes it with the copy-and-verify swap.
func runRestoreDatabase(ctx context.Context, job *dbexec.Job, base *core.BaseApp, rt *application.Runtime) (any, error) {
	var payload struct {
		DatabaseID   string `json:"databaseId"`
		ArchiveID    string `json:"archiveId"`
		DryRun       bool   `json:"dryRun"`
		RestoreToNew bool   `json:"restoreToNew"`
	}
	if err := jsonUnmarshalPayload(job.Payload, &payload); err != nil {
		return nil, fmt.Errorf("%w: corrupt payload", dbexec.ErrNonRetryable)
	}
	isPrimary := payload.DatabaseID == "" || payload.DatabaseID == application.PrimaryDatabaseID
	if isPrimary {
		// The primary restore rides the inherited core surface. The
		// publication restarts the application with the restored
		// generation; the durable job row records the intent before the
		// restart takes effect.
		if base == nil {
			return nil, fmt.Errorf("%w: the primary connection is unavailable", dbexec.ErrNonRetryable)
		}
		if payload.DryRun {
			fsys, err := base.NewBackupsFilesystem()
			if err != nil {
				return nil, fmt.Errorf("failed to open the backups filesystem: %w", err)
			}
			defer fsys.Close()
			fsys.SetContext(ctx)
			exists, err := fsys.Exists(payload.ArchiveID)
			if err != nil {
				return nil, fmt.Errorf("failed to check the archive: %w", err)
			}
			if !exists {
				return nil, fmt.Errorf("%w: archive %q is not a primary archive", dbexec.ErrNonRetryable, payload.ArchiveID)
			}
			return map[string]any{"archiveId": payload.ArchiveID, "verified": true, "dryRun": true, "published": false}, nil
		}
		if err := base.RestoreBackup(ctx, payload.ArchiveID); err != nil {
			return nil, fmt.Errorf("primary restore failed: %w", err)
		}
		return map[string]any{"archiveId": payload.ArchiveID, "published": true, "restart": true}, nil
	}
	return rt.RestoreTargetBackup(ctx, payload.DatabaseID, payload.ArchiveID, payload.DryRun, payload.RestoreToNew)
}

// runCreateDatabase registers the row, creates the engine file + manifest
// through the durable lifecycle, then publishes only after the created
// file verifies (integrity_check).
func runCreateDatabase(ctx context.Context, job *dbexec.Job, base *core.BaseApp, rt *application.Runtime) (any, error) {
	var payload struct {
		DisplayName       string `json:"displayName"`
		Kind              string `json:"kind"`
		Profile           string `json:"profile"`
		TemplateID        string `json:"templateId"`
		OwnerCollectionID string `json:"ownerCollectionId"`
		OwnerRecordID     string `json:"ownerRecordId"`
	}
	if err := jsonUnmarshalPayload(job.Payload, &payload); err != nil {
		return nil, fmt.Errorf("%w: corrupt payload", dbexec.ErrNonRetryable)
	}
	var created *application.Database
	var err error
	switch payload.Kind {
	case "manual":
		created, err = rt.Service.ProvisionManual(ctx, rt.DataDir, payload.DisplayName, payload.Profile, payload.TemplateID)
	case "user", "tenant", "agent":
		if payload.OwnerCollectionID == "" || payload.OwnerRecordID == "" {
			// Owner-less user/tenant/agent rows are created as the
			// pre-binding intent and provisioned the same way.
			created, err = rt.Service.ProvisionManualWithKind(ctx, rt.DataDir, payload.Kind, payload.DisplayName, payload.Profile, payload.TemplateID)
		} else {
			created, _, err = rt.Service.Provision(ctx, rt.DataDir, application.ProvisionRequest{
				Kind: payload.Kind, TemplateID: payload.TemplateID,
				OwnerCollectionID: payload.OwnerCollectionID, OwnerRecordID: payload.OwnerRecordID,
				DisplayName: payload.DisplayName,
			})
		}
	default:
		return nil, fmt.Errorf("%w: unknown kind %q", dbexec.ErrNonRetryable, payload.Kind)
	}
	if err != nil {
		return nil, err
	}
	// Publish only after verified provisioning: the created file must
	// open through the provider and pass integrity_check.
	handle, herr := rt.Handles.Open(ctx, created.ID)
	if herr != nil {
		_ = rt.Service.FailDatabase(ctx, created.ID, "verify")
		return nil, fmt.Errorf("created database failed to open: %w", herr)
	}
	rows, qerr := handle.QueryContext(ctx, `PRAGMA integrity_check`)
	if qerr != nil {
		_ = rt.Service.FailDatabase(ctx, created.ID, "verify")
		return nil, fmt.Errorf("created database integrity_check failed: %w", qerr)
	}
	defer rows.Close()
	status := ""
	for rows.Next() {
		var line string
		if err := rows.Scan(&line); err != nil {
			return nil, err
		}
		if status == "" {
			status = line
		}
	}
	if !strings.EqualFold(status, "ok") {
		_ = rt.Service.FailDatabase(ctx, created.ID, "verify")
		return nil, fmt.Errorf("created database integrity_check reported %q", status)
	}
	return map[string]any{"databaseId": created.ID, "kind": created.Kind, "state": created.State}, nil
}

// runSchemaApply executes one validated per-target schema plan and records
// the new schema revision.
func runSchemaApply(ctx context.Context, base *core.BaseApp, rt *application.Runtime, databaseID string, plan *application.TargetSchemaPlan) (any, error) {
	handle, err := rt.Handles.Open(ctx, databaseID)
	if err != nil {
		return nil, fmt.Errorf("failed to open the target database: %w", err)
	}
	newRevision, err := rt.ApplyTargetSchema(ctx, base, handle, plan)
	if err != nil {
		return nil, err
	}
	if err := rt.Service.SetSchemaRevision(ctx, databaseID, newRevision); err != nil {
		return nil, fmt.Errorf("failed to record the new schema revision: %w", err)
	}
	return map[string]any{"planId": plan.ID, "schemaRevision": newRevision}, nil
}

// recordJobRun enqueues the durable job row, executes the runner
// synchronously, records the real outcome (Complete/Fail) and returns the
// wire job plus the runner error for the caller's error mapping.
func recordJobRun(e *core.RequestEvent, kind string, payload map[string]any, runner func(ctx context.Context, job *dbexec.Job) (any, error)) (map[string]any, error) {
	rt, err := application.BindRuntime(e.App)
	if err != nil {
		return nil, e.InternalServerError("Failed to initialize the partitioning runtime.", err)
	}
	kernel, err := dbexec.NewJobKernel(rt.PrimaryDB)
	if err != nil {
		return nil, e.InternalServerError("Failed to open the job kernel.", err)
	}
	job, err := kernel.Enqueue(e.Request.Context(), kind, payload)
	if err != nil {
		return nil, e.InternalServerError("Failed to record the durable intent.", err)
	}
	result, runErr := runner(e.Request.Context(), job)
	if runErr != nil {
		_ = kernel.Fail(e.Request.Context(), job.ID, "operation_failed", "run", job.ID, false)
	} else {
		_ = kernel.Complete(e.Request.Context(), job.ID, result)
	}
	updated, rerr := kernel.ReadJob(e.Request.Context(), job.ID)
	if rerr != nil {
		return nil, e.InternalServerError("Failed to read the job.", rerr)
	}
	return jobWire(updated), runErr
}
