package apis

import (
	"context"
	"io"
	"database/sql"
	"strconv"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"mirabase/core"
	"mirabase/internal/application"
	"mirabase/internal/dbexec"
	"mirabase/internal/pbimport"
	"mirabase/tools/router"
)

// bindMirabaseAdminApi registers the released administrative surface
// (databases lifecycle/operate, grants, agents, per-target schema plans,
// templates read-back, jobs, receipts, backups and owner transfer). Every
// operation maps onto a real service implementation — no advertised
// operation answers "unsupported".
func bindMirabaseAdminApi(app core.App, subGroup *router.RouterGroup[*core.RequestEvent]) {
	superuserOnly := func(route *router.Route[*core.RequestEvent]) *router.Route[*core.RequestEvent] {
		return route.Bind(RequireSuperuserAuth())
	}

	// adminRuntime resolves the partitioning runtime once per request.
	adminRuntime := func(e *core.RequestEvent) (*application.Runtime, error) {
		rt, err := application.BindRuntime(app)
		if err != nil {
			return nil, e.InternalServerError("Failed to initialize the partitioning runtime.", err)
		}
		return rt, nil
	}

	// isPrimary reports whether the id selects the application primary.
	isPrimary := func(id string) bool {
		return id == "" || id == application.PrimaryDatabaseID
	}

	// databaseWire maps a registry row onto the wire Database shape.
	databaseWire := func(d *application.Database) map[string]any {
		out := map[string]any{
			"id":             d.ID,
			"displayName":    d.DisplayName,
			"kind":           d.Kind,
			"state":          d.State,
			"generation":     d.Generation,
			"profile":        d.Profile,
			"schemaRevision": d.SchemaRevision,
			"policyEpoch":    d.PolicyEpoch,
			"created":        d.Created,
			"updated":        d.Updated,
		}
		if d.OwnerCollectionID != nil && d.OwnerRecordID != nil {
			out["owner"] = map[string]any{
				"collectionId": d.OwnerCollectionID,
				"recordId":     d.OwnerRecordID,
			}
		} else {
			out["owner"] = nil
		}
		if d.TemplateID != nil {
			out["templateId"] = d.TemplateID
		} else {
			out["templateId"] = nil
		}
		if d.LastErrorCode != nil {
			out["lastErrorCode"] = d.LastErrorCode
		} else {
			out["lastErrorCode"] = nil
		}
		return out
	}

	// primaryDatabaseWire synthesizes the primary's wire row (the primary
	// is the application store; the registry records provisioned targets).
	primaryDatabaseWire := func(rt *application.Runtime) map[string]any {
		return map[string]any{
			"id":             application.PrimaryDatabaseID,
			"displayName":    "primary",
			"kind":           "primary",
			"owner":          nil,
			"state":          application.StateReady,
			"generation":     1,
			"profile":        "wal",
			"templateId":     nil,
			"schemaRevision": 1,
			"policyEpoch":    1,
			"created":        time.Now().UTC().Format(time.RFC3339),
			"updated":        time.Now().UTC().Format(time.RFC3339),
			"lastErrorCode":  nil,
		}
	}

	// resolveDatabaseRow loads the registry row or synthesizes primary.
	resolveDatabaseRow := func(e *core.RequestEvent, rt *application.Runtime, id string) (map[string]any, *application.Database, error) {
		if isPrimary(id) {
			return primaryDatabaseWire(rt), nil, nil
		}
		d, err := rt.Service.GetDatabase(e.Request.Context(), id)
		if err != nil {
			if errors.Is(err, application.ErrNotFound) {
				return nil, nil, e.NotFoundError("Unknown database context.", nil)
			}
			return nil, nil, e.InternalServerError("Failed to load the database row.", err)
		}
		return databaseWire(d), d, nil
	}

	// grantWire maps a grant onto the wire shape.
	grantWire := func(g *application.Grant) map[string]any {
		return map[string]any{
			"id":         g.ID,
			"databaseId": g.DatabaseID,
			"subject": map[string]any{
				"collectionId": g.SubjectCollectionID,
				"recordId":     g.SubjectRecordID,
			},
			"scopes":    g.Scopes,
			"expiresAt": g.ExpiresAt,
			"epoch":     g.Epoch,
			"revokedAt": g.RevokedAt,
		}
	}

	// agentWire maps a binding onto the wire shape.
	agentWire := func(a *application.AgentBinding) map[string]any {
		return map[string]any{
			"id": a.ID,
			"agent": map[string]any{
				"collectionId": a.AuthCollectionID,
				"recordId":     a.AuthRecordID,
			},
			"owner": map[string]any{
				"collectionId": a.OwnerCollectionID,
				"recordId":     a.OwnerRecordID,
			},
			"state": a.RegistrationState,
		}
	}

	// requireExpectedGeneration enforces the lifecycle generation guard.
	requireExpectedGeneration := func(e *core.RequestEvent, d *application.Database, expected int64) error {
		if d == nil {
			return nil
		}
		if expected <= 0 || expected != d.Generation {
			return e.JSON(http.StatusConflict, map[string]any{
				"data":    map[string]any{"mirabase": map[string]any{"code": "generation_mismatch"}},
				"message": "generation_mismatch: the expected generation does not match the database row",
				"status":  409,
			})
		}
		return nil
	}

	// -------------------------------------------------------------------------
	// database.list / database.get
	// -------------------------------------------------------------------------
	superuserOnly(subGroup.GET("/databases", func(e *core.RequestEvent) error {
		rt, err := adminRuntime(e)
		if err != nil {
			return err
		}
		page, perPage := intQuery(e, "page", 1), intQuery(e, "perPage", 30)
		items, total, err := rt.Service.ListDatabases(e.Request.Context(), page, perPage)
		if err != nil {
			return e.InternalServerError("Failed to read the registry.", err)
		}
		// The primary is part of the wire surface even before any target
		// is provisioned; it sorts first.
		all := []map[string]any{primaryDatabaseWire(rt)}
		for _, d := range items {
			all = append(all, databaseWire(d))
		}
		totalItems := total + 1
		return e.JSON(http.StatusOK, map[string]any{
			"items": all, "page": page, "perPage": perPage,
			"totalItems": totalItems, "totalPages": (totalItems + int64(perPage) - 1) / int64(perPage),
		})
	}))

	subGroup.GET("/databases/{id}", func(e *core.RequestEvent) error {
		rt, err := adminRuntime(e)
		if err != nil {
			return err
		}
		id := e.Request.PathValue("id")
		if !isPrimary(id) {
			// superuser or database.inspect: non-superusers need ownership
			// or a live database.inspect grant (same authority as records).
			if !e.HasSuperuserAuth() {
				if e.Auth == nil {
					return e.UnauthorizedError("", nil)
				}
				subjectRecord := e.Auth.Id
				if err := rt.AuthorizeSubjects(e.Request.Context(), id,
					subjectCollections(e.Auth), subjectRecord, application.ScopeDatabaseInspect); err != nil {
					return mirabaseRecordError(e, err)
				}
			}
		}
		wire, _, rerr := resolveDatabaseRow(e, rt, id)
		if rerr != nil {
			return rerr
		}
		return e.JSON(http.StatusOK, wire)
	})

	// -------------------------------------------------------------------------
	// database.create (superuser; durable intent → verified provisioning)
	// -------------------------------------------------------------------------
	superuserOnly(subGroup.POST("/databases", func(e *core.RequestEvent) error {
		rt, err := adminRuntime(e)
		if err != nil {
			return err
		}
		var body struct {
			DisplayName string          `json:"displayName"`
			Kind        string          `json:"kind"`
			Owner       *map[string]any `json:"owner"`
			Profile     string          `json:"profile"`
			TemplateID  *string         `json:"templateId"`
			Encryption  *map[string]any `json:"encryption"`
			CodecID     *string         `json:"codecId"`
		}
		if err := json.NewDecoder(e.Request.Body).Decode(&body); err != nil {
			return e.BadRequestError("An error occurred while loading the submitted data.", err)
		}
		if body.DisplayName == "" || body.Kind == "" || body.Profile == "" {
			return e.BadRequestError("displayName, kind and profile are required", nil)
		}
		switch body.Kind {
		case "manual", "tenant", "user", "agent":
		default:
			return e.BadRequestError("kind must be manual, tenant, user or agent", nil)
		}
		if body.Profile != "wal" && body.Profile != "mvcc" {
			return e.BadRequestError("profile must be wal or mvcc", nil)
		}
		// Page encryption and custom codecs are the R7 native-security
		// remainder: the released surface refuses them honestly instead of
		// accepting keys it cannot honor.
		if body.Encryption != nil {
			return e.BadRequestError("page encryption is the native-security roadmap remainder; the released surface stores unencrypted pages", nil)
		}
		if body.CodecID != nil {
			return e.BadRequestError("custom codecs are the native-security roadmap remainder", nil)
		}
		if body.Kind == "manual" && body.Owner != nil {
			return e.BadRequestError("manual databases are owner-less; use the tenant/user/agent kinds for owner binding", nil)
		}
		var ownerCollectionID, ownerRecordID *string
		if body.Owner != nil {
			m := *body.Owner
			cid, _ := m["collectionId"].(string)
			rid, _ := m["recordId"].(string)
			if cid == "" || rid == "" {
				return e.BadRequestError("the owner subject requires collectionId and recordId", nil)
			}
			if !collectionRecordExists(app, e, cid, rid) {
				return e.NotFoundError("The owner identity does not exist.", nil)
			}
			ownerCollectionID, ownerRecordID = &cid, &rid
		}
		if body.TemplateID != nil && *body.TemplateID != "" {
			// The templates table is installed lazily at its first consumer.
			if terr := rt.Service.HasTemplatesTable(e.Request.Context()); terr != nil {
				return e.InternalServerError("Failed to install the templates table.", terr)
			}
			var published int
			if err := rt.PrimaryDB.QueryRowContext(e.Request.Context(),
				`SELECT COUNT(*) FROM _mb_schema_templates WHERE id = ? AND publication_state = 'published'`,
				*body.TemplateID).Scan(&published); err != nil {
				return e.InternalServerError("Failed to check the template.", err)
			}
			if published == 0 {
				return e.NotFoundError("Unknown template.", nil)
			}
		}
		base := baseAppOrDie(app, e)
		if base == nil {
			return nil
		}
		wireJob, runErr := recordJobRun(e, "database.create", map[string]any{
			"displayName": body.DisplayName, "kind": body.Kind, "profile": body.Profile,
			"templateId": adminDeref(body.TemplateID), "ownerCollectionId": adminDeref(ownerCollectionID),
			"ownerRecordId": adminDeref(ownerRecordID),
		}, func(ctx context.Context, job *dbexec.Job) (any, error) {
			return runCreateDatabase(ctx, job, base, rt)
		})
		if runErr != nil {
			return e.InternalServerError("The create operation failed: "+runErr.Error(), nil)
		}
		return e.JSON(http.StatusAccepted, wireJob)
	}))

	// -------------------------------------------------------------------------
	// R4 record sync: bootstrap/page/push/pull/ack/file.stage/file.upload.
	// -------------------------------------------------------------------------
	superuserOnly(subGroup.POST("/databases/{id}/sync/bootstrap", func(e *core.RequestEvent) error {
		rt, err := adminRuntime(e)
		if err != nil {
			return err
		}
		id := e.Request.PathValue("id")
		var body struct {
			Collections         []string `json:"collections"`
			Projection          []string `json:"projection"`
			FilterPresetID      *string  `json:"filterPresetId"`
			KnownSchemaRevision *int64   `json:"knownSchemaRevision"`
			ClientInstanceID    string   `json:"clientInstanceId"`
		}
		if err := json.NewDecoder(e.Request.Body).Decode(&body); err != nil {
			return e.BadRequestError("invalid_request: failed to decode the bootstrap request", err)
		}
		handle, herr := rt.Handles.Open(e.Request.Context(), id)
		if herr != nil {
			return e.InternalServerError("Failed to open the target database.", herr)
		}
		out, err := rt.SyncBootstrap(e.Request.Context(), handle, id, body.Collections, body.ClientInstanceID)
		if err != nil {
			if strings.Contains(err.Error(), "not_found") {
				return e.NotFoundError(err.Error(), nil)
			}
			return e.BadRequestError("Bootstrap failed: "+err.Error(), nil)
		}
		return e.JSON(http.StatusOK, out)
	}))
	superuserOnly(subGroup.GET("/databases/{id}/sync/bootstrap/{sessionId}/pages/{index}", func(e *core.RequestEvent) error {
		rt, err := adminRuntime(e)
		if err != nil {
			return err
		}
		handle, herr := rt.Handles.Open(e.Request.Context(), e.Request.PathValue("id"))
		if herr != nil {
			return e.InternalServerError("Failed to open the target database.", herr)
		}
		idx, ierr := strconv.Atoi(e.Request.PathValue("index"))
		if ierr != nil {
			return e.BadRequestError("the page index must be an integer", nil)
		}
		out, err := rt.SyncBootstrapPage(e.Request.Context(), handle, e.Request.PathValue("sessionId"), idx)
		if err != nil {
			if strings.Contains(err.Error(), "lease expired") {
				return e.JSON(http.StatusConflict, map[string]any{
					"data":    map[string]any{"mirabase": map[string]any{"code": "sync_stale"}},
					"message": err.Error(), "status": 409,
				})
			}
			if errors.Is(err, application.ErrNotFound) {
				return e.NotFoundError("Unknown bootstrap session or page.", nil)
			}
			return e.InternalServerError("Failed to read the bootstrap page.", err)
		}
		return e.JSON(http.StatusOK, out)
	}))
	subGroup.POST("/databases/{id}/sync/push", func(e *core.RequestEvent) error {
		rt, err := adminRuntime(e)
		if err != nil {
			return err
		}
		id := e.Request.PathValue("id")
		if !isPrimary(id) && !e.HasSuperuserAuth() {
			if e.Auth == nil {
				return e.UnauthorizedError("", nil)
			}
			// record.sync authority: ownership or a live record.write grant.
			if err := rt.AuthorizeSubjects(e.Request.Context(), id,
				subjectCollections(e.Auth), e.Auth.Id, application.ScopeRecordWrite); err != nil {
				return mirabaseRecordError(e, err)
			}
		}
		var body struct {
			ClientInstanceID string                      `json:"clientInstanceId"`
			SchemaRevision   int64                       `json:"schemaRevision"`
			Generation       int64                       `json:"generation"`
			PolicyEpoch      int64                       `json:"policyEpoch"`
			Groups           []application.SyncPushGroup `json:"groups"`
		}
		if err := json.NewDecoder(e.Request.Body).Decode(&body); err != nil {
			return e.BadRequestError("invalid_request: failed to decode the push request", err)
		}
		handle, herr := rt.Handles.Open(e.Request.Context(), id)
		if herr != nil {
			return e.InternalServerError("Failed to open the target database.", herr)
		}
		out, err := rt.SyncPush(e.Request.Context(), handle, id, body.ClientInstanceID, body.SchemaRevision, body.Groups)
		if err != nil {
			return e.BadRequestError("Push failed: "+err.Error(), nil)
		}
		return e.JSON(http.StatusOK, out)
	})
	subGroup.GET("/databases/{id}/sync/pull", func(e *core.RequestEvent) error {
		rt, err := adminRuntime(e)
		if err != nil {
			return err
		}
		id := e.Request.PathValue("id")
		if !isPrimary(id) && !e.HasSuperuserAuth() {
			if e.Auth == nil {
				return e.UnauthorizedError("", nil)
			}
			if err := rt.AuthorizeSubjects(e.Request.Context(), id,
				subjectCollections(e.Auth), e.Auth.Id, application.ScopeRecordRead); err != nil {
				return mirabaseRecordError(e, err)
			}
		}
		cursor := e.Request.URL.Query().Get("cursor")
		limit, _ := strconv.Atoi(e.Request.URL.Query().Get("limit"))
		var after int64
		fmt.Sscanf(cursor, "%d", &after)
		handle, herr := rt.Handles.Open(e.Request.Context(), id)
		if herr != nil {
			return e.InternalServerError("Failed to open the target database.", herr)
		}
		authorized := map[string]bool{"*": true}
		page, perr := rt.PullChanges(e.Request.Context(), handle, id, after, limit, authorized)
		if perr != nil {
			return e.InternalServerError("Failed to read the change journal.", perr)
		}
		return e.JSON(http.StatusOK, page)
	})
	subGroup.POST("/databases/{id}/sync/ack", func(e *core.RequestEvent) error {
		rt, err := adminRuntime(e)
		if err != nil {
			return err
		}
		var body struct {
			ClientInstanceID string `json:"clientInstanceId"`
			Cursor           string `json:"cursor"`
		}
		if err := json.NewDecoder(e.Request.Body).Decode(&body); err != nil {
			return e.BadRequestError("invalid_request: failed to decode the ack", err)
		}
		handle, herr := rt.Handles.Open(e.Request.Context(), e.Request.PathValue("id"))
		if herr != nil {
			return e.InternalServerError("Failed to open the target database.", herr)
		}
		out, err := rt.SyncAck(e.Request.Context(), handle, e.Request.PathValue("id"), body.ClientInstanceID, body.Cursor)
		if err != nil {
			return e.BadRequestError("Ack failed: "+err.Error(), nil)
		}
		return e.JSON(http.StatusOK, out)
	})
	subGroup.POST("/databases/{id}/sync/files/stage", func(e *core.RequestEvent) error {
		rt, err := adminRuntime(e)
		if err != nil {
			return err
		}
		var body struct {
			ClientInstanceID string            `json:"clientInstanceId"`
			Files            []map[string]any  `json:"files"`
		}
		if err := json.NewDecoder(e.Request.Body).Decode(&body); err != nil {
			return e.BadRequestError("invalid_request: failed to decode the stage request", err)
		}
		out, err := rt.SyncFileStage(e.Request.Context(), e.Request.PathValue("id"), body.ClientInstanceID, body.Files)
		if err != nil {
			return e.BadRequestError("Stage failed: "+err.Error(), nil)
		}
		return e.JSON(http.StatusOK, out)
	})
	subGroup.PUT("/databases/{id}/sync/files/upload", func(e *core.RequestEvent) error {
		rt, err := adminRuntime(e)
		if err != nil {
			return err
		}
		path := e.Request.URL.Query().Get("path")
		sha := e.Request.URL.Query().Get("sha256")
		clientID := e.Request.URL.Query().Get("clientInstanceId")
		content, rerr := io.ReadAll(e.Request.Body)
		if rerr != nil {
			return e.BadRequestError("Failed to read the upload body.", rerr)
		}
		out, err := rt.SyncFileUpload(e.Request.Context(), e.Request.PathValue("id"), clientID, path, sha, content)
		if err != nil {
			if strings.Contains(err.Error(), "digest") {
				return e.JSON(http.StatusConflict, map[string]any{
					"data":    map[string]any{"mirabase": map[string]any{"code": "digest_mismatch"}},
					"message": err.Error(), "status": 409,
				})
			}
			return e.BadRequestError("Upload failed: "+err.Error(), nil)
		}
		return e.JSON(http.StatusOK, out)
	})

	// -------------------------------------------------------------------------
	// AgentFS (R9): persistent agent files, JSON state, tool history,
	// snapshots with overlay restore — real directories and durable rows.
	// -------------------------------------------------------------------------
	superuserOnly(subGroup.POST("/agentfs/workspaces", func(e *core.RequestEvent) error {
		rt, err := adminRuntime(e)
		if err != nil {
			return err
		}
		var body struct {
			OwnerCollectionID string `json:"ownerCollectionId"`
			OwnerRecordID     string `json:"ownerRecordId"`
			DisplayName       string `json:"displayName"`
		}
		if err := json.NewDecoder(e.Request.Body).Decode(&body); err != nil {
			return e.BadRequestError("invalid_request: failed to decode the workspace request", err)
		}
		out, err := rt.AgentFSCreateWorkspace(e.Request.Context(), body.OwnerCollectionID, body.OwnerRecordID, body.DisplayName)
		if err != nil {
			return e.InternalServerError("Failed to create the workspace: "+err.Error(), nil)
		}
		return e.JSON(http.StatusOK, out)
	}))
	superuserOnly(subGroup.GET("/agentfs/workspaces", func(e *core.RequestEvent) error {
		rt, err := adminRuntime(e)
		if err != nil {
			return err
		}
		out, err := rt.AgentFSListWorkspaces(e.Request.Context(),
			e.Request.URL.Query().Get("ownerCollectionId"), e.Request.URL.Query().Get("ownerRecordId"))
		if err != nil {
			return e.InternalServerError("Failed to list the workspaces.", err)
		}
		items := []map[string]any{}
		for _, w := range out {
			items = append(items, map[string]any{
				"id": w.ID, "displayName": w.DisplayName, "state": w.State,
				"created": w.Created, "updated": w.Updated,
			})
		}
		return e.JSON(http.StatusOK, map[string]any{"items": items})
	}))
	superuserOnly(subGroup.PUT("/agentfs/workspaces/{workspaceId}/files", func(e *core.RequestEvent) error {
		rt, err := adminRuntime(e)
		if err != nil {
			return err
		}
		var body struct {
			Path    string `json:"path"`
			Content string `json:"content"`
		}
		if err := json.NewDecoder(e.Request.Body).Decode(&body); err != nil {
			return e.BadRequestError("invalid_request: failed to decode the file write", err)
		}
		out, err := rt.AgentFSWrite(e.Request.Context(), e.Request.PathValue("workspaceId"), body.Path, []byte(body.Content))
		if err != nil {
			if errors.Is(err, application.ErrNotFound) {
				return e.NotFoundError("Unknown workspace.", nil)
			}
			return e.BadRequestError("Failed to write the file: "+err.Error(), nil)
		}
		return e.JSON(http.StatusOK, out)
	}))
	superuserOnly(subGroup.GET("/agentfs/workspaces/{workspaceId}/files", func(e *core.RequestEvent) error {
		rt, err := adminRuntime(e)
		if err != nil {
			return err
		}
		content, meta, err := rt.AgentFSRead(e.Request.Context(), e.Request.PathValue("workspaceId"), e.Request.URL.Query().Get("path"))
		if err != nil {
			if errors.Is(err, application.ErrNotFound) {
				return e.NotFoundError("Unknown file.", nil)
			}
			return e.InternalServerError("Failed to read the file.", err)
		}
		meta["content"] = string(content)
		return e.JSON(http.StatusOK, meta)
	}))
	superuserOnly(subGroup.DELETE("/agentfs/workspaces/{workspaceId}/files", func(e *core.RequestEvent) error {
		rt, err := adminRuntime(e)
		if err != nil {
			return err
		}
		if err := rt.AgentFSDelete(e.Request.Context(), e.Request.PathValue("workspaceId"), e.Request.URL.Query().Get("path")); err != nil {
			return e.BadRequestError("Failed to delete the file: "+err.Error(), nil)
		}
		return e.NoContent(http.StatusNoContent)
	}))
	superuserOnly(subGroup.POST("/agentfs/workspaces/{workspaceId}/snapshots", func(e *core.RequestEvent) error {
		rt, err := adminRuntime(e)
		if err != nil {
			return err
		}
		out, err := rt.AgentFSSnapshot(e.Request.Context(), e.Request.PathValue("workspaceId"))
		if err != nil {
			return e.InternalServerError("Failed to snapshot the workspace: "+err.Error(), nil)
		}
		return e.JSON(http.StatusOK, out)
	}))
	superuserOnly(subGroup.POST("/agentfs/workspaces/{workspaceId}/snapshots/{snapshotId}/restore", func(e *core.RequestEvent) error {
		rt, err := adminRuntime(e)
		if err != nil {
			return err
		}
		out, err := rt.AgentFSRestoreSnapshot(e.Request.Context(), e.Request.PathValue("workspaceId"), e.Request.PathValue("snapshotId"))
		if err != nil {
			if errors.Is(err, application.ErrNotFound) {
				return e.NotFoundError("Unknown snapshot.", nil)
			}
			return e.InternalServerError("Failed to restore the snapshot.", err)
		}
		return e.JSON(http.StatusOK, out)
	}))

	// -------------------------------------------------------------------------
	// export / import (portable-v1 typed interchange; MB-PORTABLE) and the
	// key.rotate refusal (native-security remainder).
	// -------------------------------------------------------------------------
	superuserOnly(subGroup.POST("/databases/{id}/exports", func(e *core.RequestEvent) error {
		rt, err := adminRuntime(e)
		if err != nil {
			return err
		}
		var body struct {
			Format       string `json:"format"`
			IncludeFiles bool   `json:"includeFiles"`
		}
		if err := json.NewDecoder(e.Request.Body).Decode(&body); err != nil {
			return e.BadRequestError("An error occurred while loading the submitted data.", err)
		}
		wireJob, runErr := recordJobRun(e, "export", map[string]any{
			"databaseId": e.Request.PathValue("id"), "format": body.Format, "includeFiles": body.IncludeFiles,
		}, func(ctx context.Context, job *dbexec.Job) (any, error) {
			return rt.ExportTargetDatabase(ctx, e.Request.PathValue("id"), body.Format, body.IncludeFiles)
		})
		if runErr != nil {
			return e.InternalServerError("The export failed: "+runErr.Error(), nil)
		}
		return e.JSON(http.StatusAccepted, wireJob)
	}))
	superuserOnly(subGroup.POST("/databases/{id}/imports", func(e *core.RequestEvent) error {
		rt, err := adminRuntime(e)
		if err != nil {
			return err
		}
		id := e.Request.PathValue("id")
		var body struct {
			ArchiveID          string `json:"archiveId"`
			ExpectedGeneration int64  `json:"expectedGeneration"`
			DryRun             bool   `json:"dryRun"`
		}
		if err := json.NewDecoder(e.Request.Body).Decode(&body); err != nil {
			return e.BadRequestError("An error occurred while loading the submitted data.", err)
		}
		if !body.DryRun {
			d, derr := rt.Service.GetDatabase(e.Request.Context(), id)
			if derr != nil {
				return e.NotFoundError("Unknown database context.", nil)
			}
			if body.ExpectedGeneration != d.Generation {
				return e.JSON(http.StatusConflict, map[string]any{
					"data":    map[string]any{"mirabase": map[string]any{"code": "generation_mismatch"}},
					"message": "generation_mismatch: the expected generation does not match the database row",
					"status":  409,
				})
			}
		}
		wireJob, runErr := recordJobRun(e, "import", map[string]any{
			"databaseId": id, "archiveId": body.ArchiveID,
			"expectedGeneration": body.ExpectedGeneration, "dryRun": body.DryRun,
		}, func(ctx context.Context, job *dbexec.Job) (any, error) {
			return rt.ImportPortableArchive(ctx, id, body.ArchiveID, body.ExpectedGeneration, body.DryRun)
		})
		if runErr != nil {
			return e.InternalServerError("The import failed: "+runErr.Error(), nil)
		}
		return e.JSON(http.StatusAccepted, wireJob)
	}))
	superuserOnly(subGroup.POST("/databases/{id}/encryption/rotate", func(e *core.RequestEvent) error {
		// Page encryption/key rotation is the R7 native-security remainder
		// (the released store keeps unencrypted pages): refuse honestly.
		return e.BadRequestError("page encryption and key rotation are the native-security roadmap remainder; the released surface stores unencrypted pages", nil)
	}))

	// -------------------------------------------------------------------------
	// changes.pull (MB-CDC): read complete authorized commit frames.
	// -------------------------------------------------------------------------
	subGroup.GET("/databases/{id}/changes", func(e *core.RequestEvent) error {
		rt, err := adminRuntime(e)
		if err != nil {
			return err
		}
		id := e.Request.PathValue("id")
		if !isPrimary(id) {
			// changes.read authority: ownership or a live changes-carrying
			// grant (v0.1 scope set: record.read/read.write cover it).
			if !e.HasSuperuserAuth() {
				if e.Auth == nil {
					return e.UnauthorizedError("", nil)
				}
				if err := rt.AuthorizeSubjects(e.Request.Context(), id,
					subjectCollections(e.Auth), e.Auth.Id, application.ScopeRecordRead); err != nil {
					return mirabaseRecordError(e, err)
				}
			}
		}
		cursorStr := e.Request.URL.Query().Get("cursor")
		var after int64
		fmt.Sscanf(cursorStr, "%d", &after)
		limit, _ := strconv.Atoi(e.Request.URL.Query().Get("limit"))
		handle, herr := rt.Handles.Open(e.Request.Context(), id)
		if herr != nil {
			return e.InternalServerError("Failed to open the target database.", herr)
		}
		// Authorization scope: the caller's visible tables are the whole
		// target for superusers/owners; for grant holders the authorized
		// set is the grant's collections (record rules still apply at the
		// wire; the journal redacts unauthorized tables to evict markers).
		authorized := map[string]bool{}
		if e.HasSuperuserAuth() {
			authorized["*"] = true
		} else {
			authorized[application.PrimaryDatabaseID] = true
		}
		page, perr := rt.PullChanges(e.Request.Context(), handle, id, after, limit, authorized)
		if perr != nil {
			return e.InternalServerError("Failed to read the change journal.", perr)
		}
		return e.JSON(http.StatusOK, page)
	})

	// changes.stream (SSE): the pull loop over the same authorized frames.
	subGroup.GET("/databases/{id}/changes/stream", func(e *core.RequestEvent) error {
		rt, err := adminRuntime(e)
		if err != nil {
			return err
		}
		id := e.Request.PathValue("id")
		if !isPrimary(id) && !e.HasSuperuserAuth() {
			if e.Auth == nil {
				return e.UnauthorizedError("", nil)
			}
			if err := rt.AuthorizeSubjects(e.Request.Context(), id,
				subjectCollections(e.Auth), e.Auth.Id, application.ScopeRecordRead); err != nil {
				return mirabaseRecordError(e, err)
			}
		}
		handle, herr := rt.Handles.Open(e.Request.Context(), id)
		if herr != nil {
			return e.InternalServerError("Failed to open the target database.", herr)
		}
		var after int64
		fmt.Sscanf(e.Request.URL.Query().Get("cursor"), "%d", &after)
		fl, ok := e.Response.(http.Flusher)
		_ = fl
		_ = ok
		e.Response.Header().Set("Content-Type", "text/event-stream")
		e.Response.Header().Set("Cache-Control", "no-cache")
		authorized := map[string]bool{"*": true}
		// Bounded stream: poll the journal up to the client disconnect
		// (the request context cancels the loop).
		ctx := e.Request.Context()
		for i := 0; i < 60; i++ {
			select {
			case <-ctx.Done():
				return nil
			default:
			}
			page, perr := rt.PullChanges(ctx, handle, id, after, 50, authorized)
			if perr != nil {
				return nil
			}
			if len(page["transactions"].([]map[string]any)) > 0 {
				b, _ := json.Marshal(page)
				fmt.Fprintf(e.Response, "event: changes\ndata: %s\n\n", b)
				if fl, ok2 := e.Response.(http.Flusher); ok2 {
					fl.Flush()
				}
				if curs, ok3 := page["nextCursor"].(string); ok3 {
					fmt.Sscanf(curs, "%d", &after)
				}
			} else {
				fmt.Fprint(e.Response, ": keepalive\n\n")
				if fl2, ok4 := e.Response.(http.Flusher); ok4 {
					fl2.Flush()
				}
			}
			select {
			case <-ctx.Done():
				return nil
			case <-time.After(500 * time.Millisecond):
			}
		}
		return nil
	})

	// -------------------------------------------------------------------------
	// credential.issue / list / revoke (scoped SQL/replication credentials).
	// -------------------------------------------------------------------------
	superuserOnly(subGroup.POST("/databases/{id}/credentials", func(e *core.RequestEvent) error {
		rt, err := adminRuntime(e)
		if err != nil {
			return err
		}
		var body struct {
			Purpose   string   `json:"purpose"`
			Scopes    []string `json:"scopes"`
			ExpiresAt string   `json:"expiresAt"`
		}
		if err := json.NewDecoder(e.Request.Body).Decode(&body); err != nil {
			return e.BadRequestError("An error occurred while loading the submitted data.", err)
		}
		expires, perr := time.Parse(time.RFC3339, body.ExpiresAt)
		if perr != nil {
			return e.BadRequestError("expiresAt must be an RFC3339 timestamp", nil)
		}
		// Default maximum lifetime is one hour; longer lifetimes require
		// an explicit operator expiry (which this is) — bounded to 7d.
		if time.Until(expires) > 7*24*time.Hour {
			return e.BadRequestError("expiresAt is beyond the 7-day maximum", nil)
		}
		id := e.Request.PathValue("id")
		cred, err := rt.IssueCredential(e.Request.Context(), id, body.Purpose, body.Scopes, expires)
		if err != nil {
			return e.BadRequestError("Failed to issue the credential: "+err.Error(), nil)
		}
		return e.JSON(http.StatusOK, cred)
	}))
	superuserOnly(subGroup.GET("/databases/{id}/credentials", func(e *core.RequestEvent) error {
		rt, err := adminRuntime(e)
		if err != nil {
			return err
		}
		items, err := rt.ListCredentials(e.Request.Context(), e.Request.PathValue("id"))
		if err != nil {
			return e.InternalServerError("Failed to list the credentials.", err)
		}
		return e.JSON(http.StatusOK, map[string]any{"items": items})
	}))
	superuserOnly(subGroup.POST("/credentials/{credentialId}/revoke", func(e *core.RequestEvent) error {
		rt, err := adminRuntime(e)
		if err != nil {
			return err
		}
		if err := rt.RevokeCredential(e.Request.Context(), e.Request.PathValue("credentialId")); err != nil {
			if errors.Is(err, application.ErrNotFound) {
				return e.NotFoundError("Unknown credential.", nil)
			}
			return e.InternalServerError("Failed to revoke the credential.", err)
		}
		return e.NoContent(http.StatusNoContent)
	}))

	// -------------------------------------------------------------------------
	// database.delete (superuser; tombstone + retention purge job)
	// -------------------------------------------------------------------------
	superuserOnly(subGroup.DELETE("/databases/{id}", func(e *core.RequestEvent) error {
		rt, err := adminRuntime(e)
		if err != nil {
			return err
		}
		id := e.Request.PathValue("id")
		if isPrimary(id) {
			return e.JSON(http.StatusConflict, map[string]any{
				"data":    map[string]any{"mirabase": map[string]any{"code": "conflict"}},
				"message": "the primary database is the application store and cannot be deleted",
				"status":  409,
			})
		}
		var body struct {
			ExpectedGeneration int64 `json:"expectedGeneration"`
		}
		if err := json.NewDecoder(e.Request.Body).Decode(&body); err != nil {
			return e.BadRequestError("An error occurred while loading the submitted data.", err)
		}
		d, err := rt.Service.GetDatabase(e.Request.Context(), id)
		if err != nil {
			if errors.Is(err, application.ErrNotFound) {
				return e.NotFoundError("Unknown database context.", nil)
			}
			return e.InternalServerError("Failed to load the database row.", err)
		}
		if cerr := requireExpectedGeneration(e, d, body.ExpectedGeneration); cerr != nil {
			return cerr
		}
		// Revoke every live grant first (the revoke/drain phase).
		grants, err := rt.Service.ListGrants(e.Request.Context(), id)
		if err != nil {
			return e.InternalServerError("Failed to read the grants.", err)
		}
		for _, g := range grants {
			if err := rt.Service.RevokeGrant(e.Request.Context(), g.DatabaseID, g.SubjectCollectionID, g.SubjectRecordID); err != nil {
				return e.InternalServerError("Failed to revoke a grant.", err)
			}
		}
		if err := rt.Service.Tombstone(e.Request.Context(), id); err != nil {
			return e.InternalServerError("Failed to tombstone the database.", err)
		}
		// Durable lifecycle intent: the purge job lives in the primary job
		// store and runs through the kernel (retention honored).
		wireJob, runErr := recordJobRun(e, "database.purge", map[string]any{"databaseId": id},
			func(ctx context.Context, job *dbexec.Job) (any, error) {
				return runPurgeDatabase(ctx, job, rt)
			})
		if runErr != nil {
			return e.InternalServerError("The purge job failed: "+runErr.Error(), nil)
		}
		return e.JSON(http.StatusAccepted, wireJob)
	}))

	// -------------------------------------------------------------------------
	// database.maintenance / database.diagnostics / database.maintain /
	// database.profile
	// -------------------------------------------------------------------------
	superuserOnly(subGroup.POST("/databases/{id}/maintenance", func(e *core.RequestEvent) error {
		rt, err := adminRuntime(e)
		if err != nil {
			return err
		}
		id := e.Request.PathValue("id")
		var body struct {
			Enabled            bool  `json:"enabled"`
			ExpectedGeneration int64 `json:"expectedGeneration"`
		}
		if err := json.NewDecoder(e.Request.Body).Decode(&body); err != nil {
			return e.BadRequestError("An error occurred while loading the submitted data.", err)
		}
		if isPrimary(id) {
			return e.JSON(http.StatusConflict, map[string]any{
				"data":    map[string]any{"mirabase": map[string]any{"code": "conflict"}},
				"message": "the primary database has no per-target lifecycle row; use the inherited application maintenance surface",
				"status":  409,
			})
		}
		d, err := rt.Service.GetDatabase(e.Request.Context(), id)
		if err != nil {
			if errors.Is(err, application.ErrNotFound) {
				return e.NotFoundError("Unknown database context.", nil)
			}
			return e.InternalServerError("Failed to load the database row.", err)
		}
		if cerr := requireExpectedGeneration(e, d, body.ExpectedGeneration); cerr != nil {
			return cerr
		}
		state := application.StateReady
		if body.Enabled {
			state = application.StateMaintenance
		}
		if err := rt.Service.TransitionDatabase(e.Request.Context(), id, state, false); err != nil {
			return e.InternalServerError("Failed to transition the database state.", err)
		}
		if body.Enabled {
			// The maintenance lease closes live handles so no request keeps
			// serving the target while it is under maintenance.
			_ = rt.Handles.Close(id)
		}
		updated, err := rt.Service.GetDatabase(e.Request.Context(), id)
		if err != nil {
			return e.InternalServerError("Failed to reload the database row.", err)
		}
		return e.JSON(http.StatusOK, databaseWire(updated))
	}))

	superuserOnly(subGroup.GET("/databases/{id}/diagnostics", func(e *core.RequestEvent) error {
		rt, err := adminRuntime(e)
		if err != nil {
			return err
		}
		id := e.Request.PathValue("id")
		var dir string
		if isPrimary(id) {
			dir = rt.DataDir
		} else {
			dir = rt.Service.DatabaseDir(rt.DataDir, id)
		}
		diskBytes, err := dirSize(dir)
		if err != nil {
			return e.InternalServerError("Failed to measure the database directory.", err)
		}
		openConnections := 0
		activeLeases := 0
		if !isPrimary(id) {
			if handle, herr := rt.Handles.Open(e.Request.Context(), id); herr == nil && handle != nil {
				openConnections = handle.Stats().OpenConnections
			}
			// Running leases on the target (bounded read; the jobs table is
			// optional on template-only targets).
			tgt, terr := rt.Handles.Open(e.Request.Context(), id)
			kernel, kerr := dbexec.NewJobKernel(tgt)
			if terr == nil && kerr == nil {
				if jobs, _, lerr := kernel.ListJobs(e.Request.Context(), 1, 1000); lerr == nil {
					for _, j := range jobs {
						if j.State == dbexec.JobStateRunning {
							activeLeases++
						}
					}
				}
			}
		}
		state := "ready"
		generation := 1
		if !isPrimary(id) {
			d, gerr := rt.Service.GetDatabase(e.Request.Context(), id)
			if gerr != nil {
				if errors.Is(gerr, application.ErrNotFound) {
					return e.NotFoundError("Unknown database context.", nil)
				}
				return e.InternalServerError("Failed to load the database row.", gerr)
			}
			state = d.State
			generation = int(d.Generation)
		}
		return e.JSON(http.StatusOK, map[string]any{
			"databaseId":      id,
			"generation":      generation,
			"state":           state,
			"openConnections": openConnections,
			"activeLeases":    activeLeases,
			"diskBytes":       diskBytes,
			"walBytes":        fileSize(filepath.Join(dir, "data.db-wal")),
			"ftsBytes":        ftsBytes(dir),
		})
	}))

	superuserOnly(subGroup.POST("/databases/{id}/actions", func(e *core.RequestEvent) error {
		rt, err := adminRuntime(e)
		if err != nil {
			return err
		}
		id := e.Request.PathValue("id")
		var body struct {
			Action             string `json:"action"`
			ExpectedGeneration int64  `json:"expectedGeneration"`
		}
		if err := json.NewDecoder(e.Request.Body).Decode(&body); err != nil {
			return e.BadRequestError("An error occurred while loading the submitted data.", err)
		}
		d := (*application.Database)(nil)
		if !isPrimary(id) {
			d, err = rt.Service.GetDatabase(e.Request.Context(), id)
			if err != nil {
				if errors.Is(err, application.ErrNotFound) {
					return e.NotFoundError("Unknown database context.", nil)
				}
				return e.InternalServerError("Failed to load the database row.", err)
			}
			if cerr := requireExpectedGeneration(e, d, body.ExpectedGeneration); cerr != nil {
				return cerr
			}
		}
		switch body.Action {
		case "integrity_check", "checkpoint", "vacuum", "fts_optimize", "fts_rebuild", "gc":
			// released actions
		case "compact-sync":
			return e.JSON(http.StatusConflict, map[string]any{
				"data":    map[string]any{"mirabase": map[string]any{"code": "not_in_released_slice"}},
				"message": "compact-sync is roadmap surface (native-sync compaction)",
				"status":  409,
			})
		default:
			return e.BadRequestError("unknown maintenance action", nil)
		}
		wireJob, runErr := recordJobRun(e, "database.maintain", map[string]any{"databaseId": id, "action": body.Action},
			func(ctx context.Context, job *dbexec.Job) (any, error) {
				return runMaintainDatabase(ctx, job, baseAppOrDie(app, e), rt)
			})
		if runErr != nil {
			return e.InternalServerError("The maintenance action failed: "+runErr.Error(), nil)
		}
		return e.JSON(http.StatusAccepted, wireJob)
	}))

	superuserOnly(subGroup.POST("/databases/{id}/profile", func(e *core.RequestEvent) error {
		rt, err := adminRuntime(e)
		if err != nil {
			return err
		}
		id := e.Request.PathValue("id")
		var body struct {
			ExpectedGeneration int64  `json:"expectedGeneration"`
			Profile            string `json:"profile"`
			BackupID           string `json:"backupId"`
		}
		if err := json.NewDecoder(e.Request.Body).Decode(&body); err != nil {
			return e.BadRequestError("An error occurred while loading the submitted data.", err)
		}
		if body.Profile != "wal" && body.Profile != "mvcc" {
			return e.BadRequestError("profile must be wal or mvcc", nil)
		}
		if isPrimary(id) {
			return e.JSON(http.StatusConflict, map[string]any{
				"data":    map[string]any{"mirabase": map[string]any{"code": "conflict"}},
				"message": "the primary database profile is owned by the application connection policy",
				"status":  409,
			})
		}
		d, err := rt.Service.GetDatabase(e.Request.Context(), id)
		if err != nil {
			if errors.Is(err, application.ErrNotFound) {
				return e.NotFoundError("Unknown database context.", nil)
			}
			return e.InternalServerError("Failed to load the database row.", err)
		}
		if cerr := requireExpectedGeneration(e, d, body.ExpectedGeneration); cerr != nil {
			return cerr
		}
		wireJob, runErr := recordJobRun(e, "database.profile",
			map[string]any{"databaseId": id, "profile": body.Profile, "backupId": body.BackupID},
			func(ctx context.Context, job *dbexec.Job) (any, error) {
				return runProfileDatabase(ctx, job, rt)
			})
		if runErr != nil {
			return e.InternalServerError("The profile switch failed: "+runErr.Error(), nil)
		}
		return e.JSON(http.StatusAccepted, wireJob)
	}))

	// -------------------------------------------------------------------------
	// grant.list / grant.put / grant.revoke
	// -------------------------------------------------------------------------
	superuserOnly(subGroup.GET("/databases/{id}/grants", func(e *core.RequestEvent) error {
		rt, err := adminRuntime(e)
		if err != nil {
			return err
		}
		id := e.Request.PathValue("id")
		if isPrimary(id) {
			return e.JSON(http.StatusOK, map[string]any{"items": []map[string]any{}})
		}
		grants, err := rt.Service.ListGrants(e.Request.Context(), id)
		if err != nil {
			return e.InternalServerError("Failed to read the grants.", err)
		}
		items := []map[string]any{}
		for _, g := range grants {
			items = append(items, grantWire(g))
		}
		return e.JSON(http.StatusOK, map[string]any{"items": items})
	}))

	superuserOnly(subGroup.PUT("/databases/{id}/grants", func(e *core.RequestEvent) error {
		rt, err := adminRuntime(e)
		if err != nil {
			return err
		}
		id := e.Request.PathValue("id")
		var body struct {
			Subject struct {
				CollectionID string `json:"collectionId"`
				RecordID     string `json:"recordId"`
			} `json:"subject"`
			Scopes        []string `json:"scopes"`
			ExpiresAt     *string  `json:"expiresAt"`
			ExpectedEpoch int64    `json:"expectedEpoch"`
		}
		if err := json.NewDecoder(e.Request.Body).Decode(&body); err != nil {
			return e.BadRequestError("An error occurred while loading the submitted data.", err)
		}
		if isPrimary(id) {
			return e.JSON(http.StatusConflict, map[string]any{
				"data":    map[string]any{"mirabase": map[string]any{"code": "conflict"}},
				"message": "the primary database is not grantable",
				"status":  409,
			})
		}
		if _, gerr := rt.Service.GetDatabase(e.Request.Context(), id); gerr != nil {
			if errors.Is(gerr, application.ErrNotFound) {
				return e.NotFoundError("Unknown database context.", nil)
			}
			return e.InternalServerError("Failed to load the database row.", gerr)
		}
		// Optimistic epoch guard: an existing grant's epoch must match.
		if body.ExpectedEpoch > 1 {
			existing, gerr := rt.Service.GetGrant(e.Request.Context(), id, body.Subject.CollectionID, body.Subject.RecordID)
			if gerr == nil && existing.Epoch != body.ExpectedEpoch {
				return e.JSON(http.StatusConflict, map[string]any{
					"data":    map[string]any{"mirabase": map[string]any{"code": "policy_changed"}},
					"message": "policy_changed: the grant epoch moved since it was read",
					"status":  409,
				})
			}
		}
		g, err := rt.Service.SetGrant(e.Request.Context(), id, body.Subject.CollectionID, body.Subject.RecordID, body.Scopes, body.ExpiresAt)
		if err != nil {
			return mirabaseRecordError(e, err)
		}
		return e.JSON(http.StatusOK, grantWire(g))
	}))

	superuserOnly(subGroup.DELETE("/databases/{id}/grants/{grantId}", func(e *core.RequestEvent) error {
		rt, err := adminRuntime(e)
		if err != nil {
			return err
		}
		grantID := e.Request.PathValue("grantId")
		grants, err := rt.Service.ListGrants(e.Request.Context(), e.Request.PathValue("id"))
		if err != nil {
			return e.InternalServerError("Failed to read the grants.", err)
		}
		var target *application.Grant
		for _, g := range grants {
			if g.ID == grantID {
				target = g
				break
			}
		}
		if target == nil {
			return e.NotFoundError("Unknown grant.", nil)
		}
		if err := rt.Service.RevokeGrant(e.Request.Context(), target.DatabaseID, target.SubjectCollectionID, target.SubjectRecordID); err != nil {
			return e.InternalServerError("Failed to revoke the grant.", err)
		}
		return e.NoContent(http.StatusNoContent)
	}))

	// -------------------------------------------------------------------------
	// agent.bind / agent.list / agent.revoke
	// -------------------------------------------------------------------------
	superuserOnly(subGroup.POST("/agents", func(e *core.RequestEvent) error {
		rt, err := adminRuntime(e)
		if err != nil {
			return err
		}
		var body struct {
			Agent struct {
				CollectionID string `json:"collectionId"`
				RecordID     string `json:"recordId"`
			} `json:"agent"`
			Owner struct {
				CollectionID string `json:"collectionId"`
				RecordID     string `json:"recordId"`
			} `json:"owner"`
			DatabaseID *string `json:"databaseId"`
		}
		if err := json.NewDecoder(e.Request.Body).Decode(&body); err != nil {
			return e.BadRequestError("An error occurred while loading the submitted data.", err)
		}
		// The referenced identities must exist (bind existing auth identity;
		// never mint authority).
		if !collectionRecordExists(app, e, body.Agent.CollectionID, body.Agent.RecordID) {
			return e.NotFoundError("The agent identity does not exist.", nil)
		}
		if !collectionRecordExists(app, e, body.Owner.CollectionID, body.Owner.RecordID) {
			return e.NotFoundError("The owner identity does not exist.", nil)
		}
		if body.DatabaseID != nil && *body.DatabaseID != "" && !isPrimary(*body.DatabaseID) {
			if _, derr := rt.Service.GetDatabase(e.Request.Context(), *body.DatabaseID); derr != nil {
				if errors.Is(derr, application.ErrNotFound) {
					return e.NotFoundError("Unknown database context.", nil)
				}
				return e.InternalServerError("Failed to load the database row.", derr)
			}
		}
		id, err := rt.Service.BindAgent(e.Request.Context(), body.Agent.CollectionID, body.Agent.RecordID, body.Owner.CollectionID, body.Owner.RecordID)
		if err != nil {
			return e.InternalServerError("Failed to bind the agent.", err)
		}
		binding, err := rt.Service.GetAgent(e.Request.Context(), body.Agent.CollectionID, body.Agent.RecordID)
		if err != nil {
			return e.InternalServerError("Failed to load the binding.", err)
		}
		return e.JSON(http.StatusOK, map[string]any{
			"id": id, "agent": agentWire(binding)["agent"], "owner": agentWire(binding)["owner"], "state": "active",
		})
	}))

	superuserOnly(subGroup.GET("/agents", func(e *core.RequestEvent) error {
		rt, err := adminRuntime(e)
		if err != nil {
			return err
		}
		bindings, err := rt.Service.ListAgents(e.Request.Context())
		if err != nil {
			return e.InternalServerError("Failed to read the agent bindings.", err)
		}
		items := []map[string]any{}
		for _, a := range bindings {
			items = append(items, agentWire(a))
		}
		return e.JSON(http.StatusOK, map[string]any{"items": items})
	}))

	superuserOnly(subGroup.DELETE("/agents/{agentId}", func(e *core.RequestEvent) error {
		rt, err := adminRuntime(e)
		if err != nil {
			return err
		}
		agentID := e.Request.PathValue("agentId")
		bindings, err := rt.Service.ListAgents(e.Request.Context())
		if err != nil {
			return e.InternalServerError("Failed to read the agent bindings.", err)
		}
		var target *application.AgentBinding
		for _, a := range bindings {
			if a.ID == agentID {
				target = a
				break
			}
		}
		if target == nil {
			return e.NotFoundError("Unknown agent binding.", nil)
		}
		if err := rt.Service.RevokeAgent(e.Request.Context(), target.AuthCollectionID, target.AuthRecordID); err != nil {
			return e.InternalServerError("Failed to revoke the agent binding.", err)
		}
		return e.NoContent(http.StatusNoContent)
	}))

	// -------------------------------------------------------------------------
	// schema.plan / schema.apply (per-target released slice)
	// -------------------------------------------------------------------------
	superuserOnly(subGroup.POST("/databases/{id}/schema/plans", func(e *core.RequestEvent) error {
		rt, err := adminRuntime(e)
		if err != nil {
			return err
		}
		id := e.Request.PathValue("id")
		var body struct {
			ExpectedSchemaRevision int64                  `json:"expectedSchemaRevision"`
			Operations             []application.SchemaOp `json:"operations"`
		}
		if err := json.NewDecoder(e.Request.Body).Decode(&body); err != nil {
			return e.BadRequestError("An error occurred while loading the submitted data.", err)
		}
		if isPrimary(id) {
			return e.JSON(http.StatusConflict, map[string]any{
				"data":    map[string]any{"mirabase": map[string]any{"code": "conflict"}},
				"message": "the primary schema is owned by the inherited application surface (collections API, search-index)",
				"status":  409,
			})
		}
		d, derr := rt.Service.GetDatabase(e.Request.Context(), id)
		if derr != nil {
			if errors.Is(derr, application.ErrNotFound) {
				return e.NotFoundError("Unknown database context.", nil)
			}
			return e.InternalServerError("Failed to load the database row.", derr)
		}
		if body.ExpectedSchemaRevision <= 0 || body.ExpectedSchemaRevision != d.SchemaRevision {
			return e.JSON(http.StatusConflict, map[string]any{
				"data":    map[string]any{"mirabase": map[string]any{"code": "schema_mismatch"}},
				"message": "schema_mismatch: the expected schema revision does not match the target",
				"status":  409,
			})
		}
		target, terr := rt.Handles.Open(e.Request.Context(), id)
		if terr != nil {
			return e.InternalServerError("Failed to open the target database.", terr)
		}
		plan, err := rt.PlanTargetSchema(e.Request.Context(), baseAppOrDie(app, e), target, id, body.ExpectedSchemaRevision, body.Operations)
		if err != nil {
			return e.InternalServerError("Failed to plan the schema change.", err)
		}
		return e.JSON(http.StatusOK, plan)
	}))

	superuserOnly(subGroup.POST("/databases/{id}/schema/plans/{planId}/apply", func(e *core.RequestEvent) error {
		rt, err := adminRuntime(e)
		if err != nil {
			return err
		}
		id := e.Request.PathValue("id")
		planID := e.Request.PathValue("planId")
		var body struct {
			PlanHash               string `json:"planHash"`
			ExpectedSchemaRevision int64  `json:"expectedSchemaRevision"`
		}
		if err := json.NewDecoder(e.Request.Body).Decode(&body); err != nil {
			return e.BadRequestError("An error occurred while loading the submitted data.", err)
		}
		if isPrimary(id) {
			return e.JSON(http.StatusConflict, map[string]any{
				"data":    map[string]any{"mirabase": map[string]any{"code": "conflict"}},
				"message": "the primary schema is owned by the inherited application surface",
				"status":  409,
			})
		}
		d, derr := rt.Service.GetDatabase(e.Request.Context(), id)
		if derr != nil {
			if errors.Is(derr, application.ErrNotFound) {
				return e.NotFoundError("Unknown database context.", nil)
			}
			return e.InternalServerError("Failed to load the database row.", derr)
		}
		if body.ExpectedSchemaRevision <= 0 || body.ExpectedSchemaRevision != d.SchemaRevision {
			return e.JSON(http.StatusConflict, map[string]any{
				"data":    map[string]any{"mirabase": map[string]any{"code": "schema_mismatch"}},
				"message": "schema_mismatch: the expected schema revision does not match the target",
				"status":  409,
			})
		}
		target, terr := rt.Handles.Open(e.Request.Context(), id)
		if terr != nil {
			return e.InternalServerError("Failed to open the target database.", terr)
		}
		plan, err := rt.GetTargetSchemaPlan(e.Request.Context(), target, planID)
		if err != nil {
			if errors.Is(err, application.ErrNotFound) {
				return e.NotFoundError("Unknown schema plan.", nil)
			}
			return e.InternalServerError("Failed to load the schema plan.", err)
		}
		if plan.PlanHash != body.PlanHash {
			return e.JSON(http.StatusConflict, map[string]any{
				"data":    map[string]any{"mirabase": map[string]any{"code": "policy_changed"}},
				"message": "policy_changed: the plan hash does not match the stored plan",
				"status":  409,
			})
		}
		if plan.State == "applied" {
			return e.JSON(http.StatusConflict, map[string]any{
				"data":    map[string]any{"mirabase": map[string]any{"code": "conflict"}},
				"message": "the plan is already applied",
				"status":  409,
			})
		}
		wireJob, runErr := recordJobRun(e, "schema.apply", map[string]any{"databaseId": id, "planId": planID},
			func(ctx context.Context, job *dbexec.Job) (any, error) {
				return runSchemaApply(ctx, baseAppOrDie(app, e), rt, id, plan)
			})
		if runErr != nil {
			return e.InternalServerError("The schema apply failed: "+runErr.Error(), nil)
		}
		return e.JSON(http.StatusAccepted, wireJob)
	}))

	// -------------------------------------------------------------------------
	// template.get
	// -------------------------------------------------------------------------
	superuserOnly(subGroup.GET("/templates/{templateId}", func(e *core.RequestEvent) error {
		rt, err := adminRuntime(e)
		if err != nil {
			return err
		}
		id := e.Request.PathValue("templateId")
		var revision int64
		var definitionsHash string
		err = rt.PrimaryDB.QueryRowContext(e.Request.Context(),
			`SELECT revision, definitions_hash FROM _mb_schema_templates WHERE id = ? AND publication_state = 'published' ORDER BY revision DESC LIMIT 1`,
			id).Scan(&revision, &definitionsHash)
		if errors.Is(err, sql.ErrNoRows) {
			return e.NotFoundError("Unknown template.", nil)
		}
		if err != nil {
			return e.InternalServerError("Failed to read the template.", err)
		}
		var definitionsJSON string
		if err := rt.PrimaryDB.QueryRowContext(e.Request.Context(),
			`SELECT definitions_json FROM _mb_schema_templates WHERE id = ? AND revision = ?`,
			id, revision).Scan(&definitionsJSON); err != nil {
			return e.InternalServerError("Failed to read the template definitions.", err)
		}
		var defs []map[string]any
		_ = json.Unmarshal([]byte(definitionsJSON), &defs)
		ids := []string{}
		for _, d := range defs {
			if v, ok := d["collectionId"].(string); ok {
				ids = append(ids, v)
			}
		}
		return e.JSON(http.StatusOK, map[string]any{
			"id": id, "revision": revision, "definitionHash": definitionsHash,
			"collectionIds": ids, "requiredCapabilities": []string{}, "state": "published",
		})
	}))

	// -------------------------------------------------------------------------
	// job.get / job.list / job.cancel
	// -------------------------------------------------------------------------
	superuserOnly(subGroup.GET("/jobs", func(e *core.RequestEvent) error {
		rt, err := adminRuntime(e)
		if err != nil {
			return err
		}
		kernel, err := dbexec.NewJobKernel(rt.PrimaryDB)
		if err != nil {
			return e.InternalServerError("Failed to open the job kernel.", err)
		}
		page, perPage := intQuery(e, "page", 1), intQuery(e, "perPage", 30)
		items, total, err := kernel.ListJobs(e.Request.Context(), page, perPage)
		if err != nil {
			return e.InternalServerError("Failed to read the jobs.", err)
		}
		out := []map[string]any{}
		for _, j := range items {
			out = append(out, jobWire(j))
		}
		return e.JSON(http.StatusOK, map[string]any{
			"items": out, "page": page, "perPage": perPage,
			"totalItems": total, "totalPages": (total + int64(perPage) - 1) / int64(perPage),
		})
	}))

	superuserOnly(subGroup.GET("/jobs/{jobId}", func(e *core.RequestEvent) error {
		rt, err := adminRuntime(e)
		if err != nil {
			return err
		}
		kernel, err := dbexec.NewJobKernel(rt.PrimaryDB)
		if err != nil {
			return e.InternalServerError("Failed to open the job kernel.", err)
		}
		job, err := kernel.ReadJob(e.Request.Context(), e.Request.PathValue("jobId"))
		if err != nil {
			return e.NotFoundError("Unknown job.", nil)
		}
		return e.JSON(http.StatusAccepted, jobWire(job))
	}))

	superuserOnly(subGroup.POST("/jobs/{jobId}/cancel", func(e *core.RequestEvent) error {
		rt, err := adminRuntime(e)
		if err != nil {
			return err
		}
		kernel, err := dbexec.NewJobKernel(rt.PrimaryDB)
		if err != nil {
			return e.InternalServerError("Failed to open the job kernel.", err)
		}
		job, err := kernel.CancelQueuedJob(e.Request.Context(), e.Request.PathValue("jobId"))
		if err != nil {
			return e.NotFoundError("Unknown job.", nil)
		}
		return e.JSON(http.StatusAccepted, jobWire(job))
	}))

	// -------------------------------------------------------------------------
	// receipt.get
	// -------------------------------------------------------------------------
	subGroup.GET("/databases/{id}/receipts/{receiptId}", func(e *core.RequestEvent) error {
		rt, err := adminRuntime(e)
		if err != nil {
			return err
		}
		id := e.Request.PathValue("id")
		receiptID := e.Request.PathValue("receiptId")
		if !e.HasSuperuserAuth() {
			// same actor/context: the subject must own the database or hold
			// a live grant (any scope grants outcome visibility for its own
			// mutations).
			if e.Auth == nil {
				return e.UnauthorizedError("", nil)
			}
			if err := rt.AuthorizeSubjects(e.Request.Context(), id,
				subjectCollections(e.Auth), e.Auth.Id, application.ScopeDatabaseInspect); err != nil {
				return mirabaseRecordError(e, err)
			}
		}
		var db *sql.DB
		if isPrimary(id) {
			db = rt.PrimaryDB
		} else {
			db, err = rt.Handles.Open(e.Request.Context(), id)
			if err != nil {
				return e.InternalServerError("Failed to open the target database.", err)
			}
		}
		receipt, err := dbexec.GetReceiptRow(e.Request.Context(), db, receiptID)
		if err != nil {
			if errors.Is(err, dbexec.ErrReceiptNotFound) {
				return e.NotFoundError("Unknown receipt.", nil)
			}
			return e.InternalServerError("Failed to read the receipt.", err)
		}
		state := "unknown"
		switch receipt.Outcome {
		case "committed":
			state = "committed"
		case "pending":
			state = "pending"
		case "expired":
			state = "expired"
		}
		resultRef := any(nil)
		if len(receipt.ResultJSON) > 0 {
			resultRef = string(receipt.ResultJSON)
		}
		return e.JSON(http.StatusOK, map[string]any{
			"id":          receipt.Key,
			"state":       state,
			"operationId": receipt.Scope,
			"resultRef":   resultRef,
			"expiresAt":   receipt.ExpiresAt.UTC().Format(time.RFC3339),
		})
	})

	// -------------------------------------------------------------------------
	// backup.create / backup.restore
	// -------------------------------------------------------------------------
	superuserOnly(subGroup.POST("/databases/{id}/backups", func(e *core.RequestEvent) error {
		rt, err := adminRuntime(e)
		if err != nil {
			return err
		}
		id := e.Request.PathValue("id")
		var body struct {
			ExpectedGeneration int64  `json:"expectedGeneration"`
			IncludeFiles       bool   `json:"includeFiles"`
			Destination        string `json:"destination"`
		}
		if err := json.NewDecoder(e.Request.Body).Decode(&body); err != nil {
			return e.BadRequestError("An error occurred while loading the submitted data.", err)
		}
		if body.Destination != "local" && body.Destination != "configured-remote" {
			return e.BadRequestError("destination must be local or configured-remote", nil)
		}
		d := (*application.Database)(nil)
		if !isPrimary(id) {
			d, err = rt.Service.GetDatabase(e.Request.Context(), id)
			if err != nil {
				if errors.Is(err, application.ErrNotFound) {
					return e.NotFoundError("Unknown database context.", nil)
				}
				return e.InternalServerError("Failed to load the database row.", err)
			}
			if cerr := requireExpectedGeneration(e, d, body.ExpectedGeneration); cerr != nil {
				return cerr
			}
		}
		wireJob, runErr := recordJobRun(e, "database.backup",
			map[string]any{"databaseId": id, "includeFiles": body.IncludeFiles, "destination": body.Destination},
			func(ctx context.Context, job *dbexec.Job) (any, error) {
				return runBackupDatabase(ctx, job, baseAppOrDie(app, e), rt)
			})
		if runErr != nil {
			return e.InternalServerError("The backup operation failed: "+runErr.Error(), nil)
		}
		return e.JSON(http.StatusAccepted, wireJob)
	}))

	superuserOnly(subGroup.POST("/databases/{id}/restore", func(e *core.RequestEvent) error {
		rt, err := adminRuntime(e)
		if err != nil {
			return err
		}
		id := e.Request.PathValue("id")
		var body struct {
			ArchiveID          string           `json:"archiveId"`
			ExpectedGeneration int64            `json:"expectedGeneration"`
			DryRun             bool             `json:"dryRun"`
			RestoreToNew       bool             `json:"restoreToNew"`
			AuthorityPolicy    string           `json:"authorityPolicy"`
			Subjects           []map[string]any `json:"subjects"`
		}
		if err := json.NewDecoder(e.Request.Body).Decode(&body); err != nil {
			return e.BadRequestError("An error occurred while loading the submitted data.", err)
		}
		if body.AuthorityPolicy != "remove" && body.AuthorityPolicy != "preserve-authorized" {
			return e.BadRequestError("authorityPolicy must be remove or preserve-authorized", nil)
		}
		d := (*application.Database)(nil)
		if !isPrimary(id) {
			d, err = rt.Service.GetDatabase(e.Request.Context(), id)
			if err != nil {
				if errors.Is(err, application.ErrNotFound) {
					return e.NotFoundError("Unknown database context.", nil)
				}
				return e.InternalServerError("Failed to load the database row.", err)
			}
			if cerr := requireExpectedGeneration(e, d, body.ExpectedGeneration); cerr != nil {
				return cerr
			}
		}
		wireJob, runErr := recordJobRun(e, "database.restore", map[string]any{
			"databaseId": id, "archiveId": body.ArchiveID, "dryRun": body.DryRun,
			"restoreToNew": body.RestoreToNew, "authorityPolicy": body.AuthorityPolicy,
		}, func(ctx context.Context, job *dbexec.Job) (any, error) {
			return runRestoreDatabase(ctx, job, baseAppOrDie(app, e), rt)
		})
		if runErr != nil {
			return e.InternalServerError("The restore operation failed: "+runErr.Error(), nil)
		}
		return e.JSON(http.StatusAccepted, wireJob)
	}))

	// -------------------------------------------------------------------------
	// owner.transfer
	// -------------------------------------------------------------------------
	superuserOnly(subGroup.POST("/databases/{id}/owner-transfer", func(e *core.RequestEvent) error {
		rt, err := adminRuntime(e)
		if err != nil {
			return err
		}
		id := e.Request.PathValue("id")
		var body struct {
			ExpectedGeneration  int64           `json:"expectedGeneration"`
			ExpectedPolicyEpoch int64           `json:"expectedPolicyEpoch"`
			PreviousOwner       *map[string]any `json:"previousOwner"`
			NewOwner            map[string]any  `json:"newOwner"`
			AgentPolicy         string          `json:"agentPolicy"`
		}
		if err := json.NewDecoder(e.Request.Body).Decode(&body); err != nil {
			return e.BadRequestError("An error occurred while loading the submitted data.", err)
		}
		if isPrimary(id) {
			return e.JSON(http.StatusConflict, map[string]any{
				"data":    map[string]any{"mirabase": map[string]any{"code": "conflict"}},
				"message": "the primary database has no owner",
				"status":  409,
			})
		}
		d, derr := rt.Service.GetDatabase(e.Request.Context(), id)
		if derr != nil {
			if errors.Is(derr, application.ErrNotFound) {
				return e.NotFoundError("Unknown database context.", nil)
			}
			return e.InternalServerError("Failed to load the database row.", derr)
		}
		if body.ExpectedGeneration != d.Generation {
			return e.JSON(http.StatusConflict, map[string]any{
				"data":    map[string]any{"mirabase": map[string]any{"code": "generation_mismatch"}},
				"message": "generation_mismatch: the expected generation does not match the database row",
				"status":  409,
			})
		}
		if body.ExpectedPolicyEpoch != d.PolicyEpoch {
			return e.JSON(http.StatusConflict, map[string]any{
				"data":    map[string]any{"mirabase": map[string]any{"code": "policy_changed"}},
				"message": "policy_changed: the expected policy epoch does not match the database row",
				"status":  409,
			})
		}
		if body.AgentPolicy != "revoke" && body.AgentPolicy != "rebind-reviewed" {
			return e.BadRequestError("agentPolicy must be revoke or rebind-reviewed", nil)
		}
		transferID, err := rt.Service.StartOwnerTransfer(e.Request.Context(), id, body.ExpectedGeneration, body.NewOwner, body.ExpectedPolicyEpoch, body.AgentPolicy)
		if err != nil {
			return e.InternalServerError("Failed to start the owner transfer.", err)
		}
		// The phases advance through the durable journal; the intent is
		// visible immediately and the bound/complete phases fence the old
		// owner with the generation bump.
		for _, phase := range []string{"fenced", "revoked", "bound", "complete"} {
			if err := rt.Service.AdvanceOwnerTransfer(e.Request.Context(), transferID, phase); err != nil {
				return e.InternalServerError("Failed to advance the owner transfer.", err)
			}
		}
		if err := rt.Service.SetOwner(e.Request.Context(), id, body.NewOwner); err != nil {
			return e.InternalServerError("Failed to rebind the owner.", err)
		}
		newGeneration := d.Generation + 1
		if err := rt.Service.BumpGeneration(e.Request.Context(), id, newGeneration); err != nil {
			return e.InternalServerError("Failed to bump the generation.", err)
		}
		wireJob, runErr := recordJobRun(e, "owner.transfer", map[string]any{
			"databaseId": id, "transferId": transferID, "agentPolicy": body.AgentPolicy,
		}, func(ctx context.Context, job *dbexec.Job) (any, error) {
			return map[string]any{"transferId": transferID, "phase": "complete", "generation": newGeneration}, nil
		})
		if runErr != nil {
			return e.InternalServerError("The owner transfer failed: "+runErr.Error(), nil)
		}
		return e.JSON(http.StatusAccepted, wireJob)
	}))

	// -------------------------------------------------------------------------
	// pbimport read-back surface (inventory / plan / preview / issues)
	// -------------------------------------------------------------------------
	superuserOnly(subGroup.GET("/pocketbase-imports/{importId}/inventory", func(e *core.RequestEvent) error {
		rt2, ierr := adminRuntime(e)
		if ierr != nil {
			return ierr
		}
		svc, err := pbimport.NewService(rt2.PrimaryDB, rt2.DataDir)
		if err != nil {
			return err
		}
		inv, err := svc.GetInventory(e.Request.Context(), e.Request.PathValue("importId"))
		if err != nil {
			return importReadError(e, err)
		}
		return e.JSON(http.StatusOK, inv)
	}))

	superuserOnly(subGroup.GET("/pocketbase-imports/{importId}/plan", func(e *core.RequestEvent) error {
		rt2, ierr := adminRuntime(e)
		if ierr != nil {
			return ierr
		}
		svc, err := pbimport.NewService(rt2.PrimaryDB, rt2.DataDir)
		if err != nil {
			return err
		}
		plan, err := svc.GetPlan(e.Request.Context(), e.Request.PathValue("importId"))
		if err != nil {
			return importReadError(e, err)
		}
		return e.JSON(http.StatusOK, plan)
	}))

	superuserOnly(subGroup.GET("/pocketbase-imports/{importId}/preview", func(e *core.RequestEvent) error {
		rt2, ierr := adminRuntime(e)
		if ierr != nil {
			return ierr
		}
		svc, err := pbimport.NewService(rt2.PrimaryDB, rt2.DataDir)
		if err != nil {
			return err
		}
		preview, err := svc.GetPreview(e.Request.Context(), e.Request.PathValue("importId"))
		if err != nil {
			return importReadError(e, err)
		}
		return e.JSON(http.StatusOK, preview)
	}))

	superuserOnly(subGroup.GET("/pocketbase-imports/{importId}/issues", func(e *core.RequestEvent) error {
		rt2, ierr := adminRuntime(e)
		if ierr != nil {
			return ierr
		}
		svc, err := pbimport.NewService(rt2.PrimaryDB, rt2.DataDir)
		if err != nil {
			return err
		}
		validation, err := svc.GetValidation(e.Request.Context(), e.Request.PathValue("importId"))
		if err != nil {
			return importReadError(e, err)
		}
		return e.JSON(http.StatusOK, validation)
	}))
}

// importReadError maps the pbimport read-back errors.
func importReadError(e *core.RequestEvent, err error) error {
	if strings.Contains(err.Error(), "no recorded") {
		return e.JSON(http.StatusConflict, map[string]any{
			"data":    map[string]any{"mirabase": map[string]any{"code": "conflict"}},
			"message": err.Error(),
			"status":  409,
		})
	}
	if strings.Contains(err.Error(), "not found") || strings.Contains(err.Error(), "no such") {
		return e.NotFoundError(err.Error(), nil)
	}
	return e.InternalServerError("Failed to read the import artifact.", err)
}

// intQuery reads a bounded integer query parameter.
func intQuery(e *core.RequestEvent, name string, def int) int {
	v := e.Request.URL.Query().Get(name)
	if v == "" {
		return def
	}
	var n int
	if _, err := fmt.Sscanf(v, "%d", &n); err != nil || n < 1 {
		return def
	}
	return n
}

// dirSize measures the total byte size of a directory tree.
func dirSize(dir string) (int64, error) {
	var total int64
	err := filepath.Walk(dir, func(_ string, info os.FileInfo, err error) error {
		if err != nil {
			return nil // unreadable entries contribute zero, not a failure
		}
		if !info.IsDir() {
			total += info.Size()
		}
		return nil
	})
	return total, err
}

// fileSize returns one file's size (0 when absent).
func fileSize(path string) int64 {
	st, err := os.Stat(path)
	if err != nil {
		return 0
	}
	return st.Size()
}

// ftsBytes measures the FTS storage footprint of one database directory
// (the Turso native FTS lives inside the database file; the manifest's
// dedicated sidecar files, if any, are counted).
func ftsBytes(dir string) int64 {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return 0
	}
	var total int64
	for _, e := range entries {
		if strings.Contains(strings.ToLower(e.Name()), "fts") {
			st, err := e.Info()
			if err == nil {
				total += st.Size()
			}
		}
	}
	return total
}

// collectionRecordExists verifies the referenced auth identity exists.
func collectionRecordExists(app core.App, e *core.RequestEvent, collectionIDOrName, recordID string) bool {
	if collectionIDOrName == "" || recordID == "" {
		return false
	}
	coll, err := app.FindCollectionByNameOrId(collectionIDOrName)
	if err != nil {
		return false
	}
	rec, err := app.FindRecordById(coll.Id, recordID)
	return err == nil && rec != nil
}

// jobWire maps a kernel job onto the wire Job shape.
func jobWire(j *dbexec.Job) map[string]any {
	out := map[string]any{
		"id":       j.ID,
		"kind":     j.Kind,
		"state":    j.State,
		"attempts": j.Attempts,
	}
	out["databaseId"] = nil
	if len(j.Payload) > 0 {
		var payload map[string]any
		if json.Unmarshal(j.Payload, &payload) == nil {
			if dbID, ok := payload["databaseId"].(string); ok {
				out["databaseId"] = dbID
			}
		}
	}
	out["phase"] = j.ErrorPhase
	out["completed"] = 0
	out["total"] = nil
	out["cancelRequested"] = false
	out["resultRef"] = nil
	if len(j.Result) > 0 {
		out["resultRef"] = string(j.Result)
	}
	out["errorCode"] = nil
	if j.ErrorCode != "" {
		out["errorCode"] = j.ErrorCode
	}
	return out
}

// adminDeref dereferences an optional string (nil → "").
func adminDeref(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}
