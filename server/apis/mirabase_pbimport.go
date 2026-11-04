package apis

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"mirabase/core"
	"mirabase/internal/application"
	"mirabase/internal/dbexec"
	"mirabase/internal/pbimport"
	"mirabase/tools/router"
)

// bindMirabaseImportApi registers the shared-layout PocketBase adoption
// surface (pbimport.* v0.1 slice: create, source upload/get, analyze,
// inventory, plan, preview, stage, validate, report, cutover, cancel,
// get — superuser authority; the session id never authorizes a path).
func bindMirabaseImportApi(app core.App, subGroup *router.RouterGroup[*core.RequestEvent]) {
	mustImport := func(e *core.RequestEvent) (*pbimport.Service, error) {
		rt, err := application.BindRuntime(app)
		if err != nil {
			return nil, err
		}
		return pbimport.NewService(rt.PrimaryDB, rt.DataDir)
	}

	// pbimport.create — POST /api/mirabase/v1/pocketbase-imports
	subGroup.POST("/pocketbase-imports", func(e *core.RequestEvent) error {
		svc, err := mustImport(e)
		if err != nil {
			return e.InternalServerError("Failed to initialize the import service.", err)
		}
		var req pbimport.CreateRequest
		if err := e.BindBody(&req); err != nil {
			return pbimportError(e, fmt.Errorf("%w: failed to decode the create request", pbimport.ErrInvalidRequest))
		}
		sess, err := svc.Create(e.Request.Context(), &req)
		if err != nil {
			return pbimportError(e, err)
		}
		if sess.Phase == "awaiting-source" {
			svc.DeclareSource(sess.ID, req.SourceBytes)
		}
		return e.JSON(http.StatusOK, sess)
	})

	// pbimport.source.upload — PUT .../{importId}/source (binary chunk)
	subGroup.PUT("/pocketbase-imports/{importId}/source", func(e *core.RequestEvent) error {
		svc, err := mustImport(e)
		if err != nil {
			return e.InternalServerError("Failed to initialize the import service.", err)
		}
		contentRange := e.Request.Header.Get("Content-Range")
		chunkSha := e.Request.Header.Get("X-Chunk-Sha256")
		if !contentRangeRe.MatchString(contentRange) {
			return pbimportError(e, fmt.Errorf("%w: the upload requires a valid Content-Range header", pbimport.ErrInvalidRequest))
		}
		if !shaRe.MatchString(chunkSha) {
			return pbimportError(e, fmt.Errorf("%w: the upload requires a valid X-Chunk-SHA256 header", pbimport.ErrInvalidRequest))
		}
		var start, end int64
		var total int64
		if _, err := fmt.Sscanf(contentRange, "bytes %d-%d/%d", &start, &end, &total); err != nil {
			return pbimportError(e, fmt.Errorf("%w: Content-Range is malformed", pbimport.ErrInvalidRequest))
		}
		if end < start || total < 1 {
			return pbimportError(e, fmt.Errorf("%w: Content-Range is malformed", pbimport.ErrInvalidRequest))
		}
		svc.DeclareSource(e.Request.PathValue("importId"), int64(total))
		data, err := io.ReadAll(io.LimitReader(e.Request.Body, pbimport.DefaultChunkSize+1))
		if err != nil {
			return pbimportError(e, fmt.Errorf("%w: failed to read the chunk", pbimport.ErrInvalidRequest))
		}
		received, complete, err := svc.UploadChunk(e.Request.Context(),
			e.Request.PathValue("importId"), start, data, chunkSha)
		if err != nil {
			return pbimportError(e, err)
		}
		return e.JSON(http.StatusOK, map[string]any{
			"receivedBytes": fmt.Sprintf("%d", received),
			"complete":      complete,
		})
	})

	// pbimport.source.get — GET .../{importId}/source
	subGroup.GET("/pocketbase-imports/{importId}/source", func(e *core.RequestEvent) error {
		svc, err := mustImport(e)
		if err != nil {
			return e.InternalServerError("Failed to initialize the import service.", err)
		}
		sess, err := svc.Get(e.Request.Context(), e.Request.PathValue("importId"))
		if err != nil {
			return pbimportError(e, err)
		}
		srcPath := filepath.Join(svc.SourceDir(sess.ID), "source.bin")
		f, err := os.Open(srcPath)
		if err != nil {
			return pbimportError(e, fmt.Errorf("%w: the source bytes are not finalized", pbimport.ErrSourceIncomplete))
		}
		defer f.Close()
		info, err := f.Stat()
		if err != nil {
			return pbimportError(e, err)
		}
		e.Response.Header().Set("Content-Type", "application/octet-stream")
		e.Response.Header().Set("Content-Length", fmt.Sprintf("%d", info.Size()))
		e.Response.WriteHeader(http.StatusOK)
		_, _ = io.Copy(e.Response, f)
		return nil
	})

	// pbimport.analyze — POST .../{importId}/analyze
	subGroup.POST("/pocketbase-imports/{importId}/analyze", func(e *core.RequestEvent) error {
		svc, err := mustImport(e)
		if err != nil {
			return e.InternalServerError("Failed to initialize the import service.", err)
		}
		analysis, err := svc.AnalyzeSource(e.Request.Context(), e.Request.PathValue("importId"))
		if err != nil {
			return pbimportError(e, err)
		}
		// The analyze job returns the inventory synchronously in the v0.1
		// slice (the analysis is bounded by the 20 GiB source cap and the
		// read-only reader's single connection).
		return e.JSON(http.StatusOK, analysis)
	})

	// pbimport.plan.put — PUT .../{importId}/plan
	subGroup.PUT("/pocketbase-imports/{importId}/plan", func(e *core.RequestEvent) error {
		svc, err := mustImport(e)
		if err != nil {
			return e.InternalServerError("Failed to initialize the import service.", err)
		}
		var req pbimport.PlanRequest
		if err := e.BindBody(&req); err != nil {
			return pbimportError(e, fmt.Errorf("%w: failed to decode the plan", pbimport.ErrInvalidRequest))
		}
		plan, err := svc.PutPlan(e.Request.Context(), e.Request.PathValue("importId"), &req)
		if err != nil {
			return pbimportError(e, err)
		}
		return e.JSON(http.StatusOK, plan)
	})

	// pbimport.preview.compute — POST .../{importId}/preview
	subGroup.POST("/pocketbase-imports/{importId}/preview", func(e *core.RequestEvent) error {
		svc, err := mustImport(e)
		if err != nil {
			return e.InternalServerError("Failed to initialize the import service.", err)
		}
		preview, err := svc.ComputePreview(e.Request.Context(), e.Request.PathValue("importId"))
		if err != nil {
			return pbimportError(e, err)
		}
		return e.JSON(http.StatusOK, preview)
	})

	// pbimport.stage — POST .../{importId}/stage
	subGroup.POST("/pocketbase-imports/{importId}/stage", func(e *core.RequestEvent) error {
		svc, err := mustImport(e)
		if err != nil {
			return e.InternalServerError("Failed to initialize the import service.", err)
		}
		result, err := svc.Stage(e.Request.Context(), e.Request.PathValue("importId"))
		if err != nil {
			return pbimportError(e, err)
		}
		return e.JSON(http.StatusAccepted, result)
	})

	// pbimport.validate — POST .../{importId}/validate
	subGroup.POST("/pocketbase-imports/{importId}/validate", func(e *core.RequestEvent) error {
		svc, err := mustImport(e)
		if err != nil {
			return e.InternalServerError("Failed to initialize the import service.", err)
		}
		val, err := svc.Validate(e.Request.Context(), e.Request.PathValue("importId"))
		if err != nil {
			return pbimportError(e, err)
		}
		return e.JSON(http.StatusOK, val)
	})

	// pbimport.report — GET .../{importId}/report
	subGroup.GET("/pocketbase-imports/{importId}/report", func(e *core.RequestEvent) error {
		svc, err := mustImport(e)
		if err != nil {
			return e.InternalServerError("Failed to initialize the import service.", err)
		}
		sess, err := svc.Get(e.Request.Context(), e.Request.PathValue("importId"))
		if err != nil {
			return pbimportError(e, err)
		}
		reportHash := ""
		if sess.ReportHash != nil {
			reportHash = *sess.ReportHash
		}
		if reportHash == "" {
			return pbimportError(e, fmt.Errorf("%w: no validation report exists", pbimport.ErrReportStale))
		}
		return e.JSON(http.StatusOK, map[string]any{
			"sessionId":  sess.ID,
			"reportHash": reportHash,
			"phase":      sess.Phase,
		})
	})

	// pbimport.cutover — POST .../{importId}/cutover
	subGroup.POST("/pocketbase-imports/{importId}/cutover", func(e *core.RequestEvent) error {
		svc, err := mustImport(e)
		if err != nil {
			return e.InternalServerError("Failed to initialize the import service.", err)
		}
		var req struct {
			ExpectedRevision string `json:"expectedRevision"`
			PlanHash         string `json:"planHash"`
			PreviewHash      string `json:"previewHash"`
			ReportHash       string `json:"reportHash"`
		}
		if err := e.BindBody(&req); err != nil {
			return pbimportError(e, fmt.Errorf("%w: failed to decode the cutover request", pbimport.ErrInvalidRequest))
		}
		sess, err := svc.Cutover(e.Request.Context(), e.Request.PathValue("importId"),
			req.PlanHash, req.PreviewHash, req.ReportHash)
		if err != nil {
			return pbimportError(e, err)
		}
		return e.JSON(http.StatusAccepted, sess)
	})

	// pbimport.cancel — POST .../{importId}/cancel
	subGroup.POST("/pocketbase-imports/{importId}/cancel", func(e *core.RequestEvent) error {
		svc, err := mustImport(e)
		if err != nil {
			return e.InternalServerError("Failed to initialize the import service.", err)
		}
		if err := svc.Cancel(e.Request.Context(), e.Request.PathValue("importId")); err != nil {
			return pbimportError(e, err)
		}
		sess, err := svc.Get(e.Request.Context(), e.Request.PathValue("importId"))
		if err != nil {
			return pbimportError(e, err)
		}
		return e.JSON(http.StatusAccepted, sess)
	})

	// pbimport.get — GET /api/mirabase/v1/pocketbase-imports/{importId}
	subGroup.GET("/pocketbase-imports/{importId}", func(e *core.RequestEvent) error {
		svc, err := mustImport(e)
		if err != nil {
			return e.InternalServerError("Failed to initialize the import service.", err)
		}
		sess, err := svc.Get(e.Request.Context(), e.Request.PathValue("importId"))
		if err != nil {
			return pbimportError(e, err)
		}
		return e.JSON(http.StatusOK, sess)
	})

	// pbimport.report.cases — paginated actual validation cases.
	subGroup.GET("/pocketbase-imports/{importId}/report/cases", func(e *core.RequestEvent) error {
		svc, err := mustImport(e)
		if err != nil {
			return e.InternalServerError("Failed to initialize the import service.", err)
		}
		page, _ := strconv.Atoi(e.Request.URL.Query().Get("page"))
		perPage, _ := strconv.Atoi(e.Request.URL.Query().Get("perPage"))
		items, total, err := svc.GetValidationCases(e.Request.Context(), e.Request.PathValue("importId"), page, perPage)
		if err != nil {
			return pbimportError(e, err)
		}
		if perPage < 1 || perPage > 1000 {
			perPage = 30
		}
		if page < 1 {
			page = 1
		}
		return e.JSON(http.StatusOK, map[string]any{
			"items": items, "page": page, "perPage": perPage,
			"totalItems": total, "totalPages": (total + int64(perPage) - 1) / int64(perPage),
		})
	})

	// pbimport.resume — reconcile the last durable phase of a failed
	// session (journal-aware; never repeats acknowledged effects).
	superuserOnlyImport(subGroup.POST("/pocketbase-imports/{importId}/resume", func(e *core.RequestEvent) error {
		svc, err := mustImport(e)
		if err != nil {
			return e.InternalServerError("Failed to initialize the import service.", err)
		}
		var body struct {
			ExpectedRevision int64   `json:"expectedRevision"`
			PlanHash         *string `json:"planHash"`
		}
		if err := json.NewDecoder(e.Request.Body).Decode(&body); err != nil {
			return e.BadRequestError("An error occurred while loading the submitted data.", err)
		}
		wireJob, runErr := recordJobRun(e, "pbimport.resume", map[string]any{
			"importId":         e.Request.PathValue("importId"),
			"expectedRevision": body.ExpectedRevision, "planHash": body.PlanHash,
		}, func(ctx context.Context, job *dbexec.Job) (any, error) {
			return runPbimportResume(ctx, svc, job)
		})
		if runErr != nil {
			return pbimportError(e, runErr)
		}
		return e.JSON(http.StatusAccepted, wireJob)
	}))

	// pbimport.recovery — verified prior-root rollback (before new writes)
	// or verified current-state export preserved as authoritative.
	superuserOnlyImport(subGroup.POST("/pocketbase-imports/{importId}/recovery", func(e *core.RequestEvent) error {
		svc, err := mustImport(e)
		if err != nil {
			return e.InternalServerError("Failed to initialize the import service.", err)
		}
		var body struct {
			ExpectedRevision         int64  `json:"expectedRevision"`
			PlanHash                 string `json:"planHash"`
			Action                   string `json:"action"`
			ExpectedActiveGeneration int64  `json:"expectedActiveGeneration"`
		}
		if err := json.NewDecoder(e.Request.Body).Decode(&body); err != nil {
			return e.BadRequestError("An error occurred while loading the submitted data.", err)
		}
		wireJob, runErr := recordJobRun(e, "pbimport.recovery", map[string]any{
			"importId": e.Request.PathValue("importId"), "expectedRevision": body.ExpectedRevision,
			"planHash": body.PlanHash, "action": body.Action,
			"expectedActiveGeneration": body.ExpectedActiveGeneration,
		}, func(ctx context.Context, job *dbexec.Job) (any, error) {
			return runPbimportRecovery(ctx, svc, job)
		})
		if runErr != nil {
			return pbimportError(e, runErr)
		}
		return e.JSON(http.StatusAccepted, wireJob)
	}))

	// importbatch.create/get/sources/run/cancel + pbimport.tenants.
	subGroup.POST("/import-batches", func(e *core.RequestEvent) error {
		svc, err := mustImport(e)
		if err != nil {
			return e.InternalServerError("Failed to initialize the import service.", err)
		}
		var body struct {
			Mode    string `json:"mode"`
			Sources []struct {
				SourceID   string `json:"sourceId"`
				Provenance string `json:"provenance"`
			} `json:"sources"`
		}
		if err := json.NewDecoder(e.Request.Body).Decode(&body); err != nil {
			return e.BadRequestError("invalid_request: failed to decode the batch request", err)
		}
		out, err := svc.CreateImportBatch(e.Request.Context(), body.Sources, body.Mode)
		if err != nil {
			return pbimportError(e, err)
		}
		return e.JSON(http.StatusOK, out)
	})
	subGroup.GET("/import-batches/{batchId}", func(e *core.RequestEvent) error {
		svc, err := mustImport(e)
		if err != nil {
			return e.InternalServerError("Failed to initialize the import service.", err)
		}
		out, err := svc.GetImportBatch(e.Request.Context(), e.Request.PathValue("batchId"))
		if err != nil {
			return pbimportError(e, err)
		}
		return e.JSON(http.StatusOK, out)
	})
	subGroup.GET("/import-batches/{batchId}/sources", func(e *core.RequestEvent) error {
		svc, err := mustImport(e)
		if err != nil {
			return e.InternalServerError("Failed to initialize the import service.", err)
		}
		page, _ := strconv.Atoi(e.Request.URL.Query().Get("page"))
		perPage, _ := strconv.Atoi(e.Request.URL.Query().Get("perPage"))
		items, total, err := svc.ListImportBatchSources(e.Request.Context(), e.Request.PathValue("batchId"), page, perPage)
		if err != nil {
			return pbimportError(e, err)
		}
		if perPage < 1 || perPage > 1000 {
			perPage = 30
		}
		if page < 1 {
			page = 1
		}
		return e.JSON(http.StatusOK, map[string]any{
			"items": items, "page": page, "perPage": perPage,
			"totalItems": total, "totalPages": (total + int64(perPage) - 1) / int64(perPage),
		})
	})
	subGroup.POST("/import-batches/{batchId}/run", func(e *core.RequestEvent) error {
		svc, err := mustImport(e)
		if err != nil {
			return e.InternalServerError("Failed to initialize the import service.", err)
		}
		out, err := svc.RunImportBatch(e.Request.Context(), e.Request.PathValue("batchId"))
		if err != nil {
			return pbimportError(e, err)
		}
		return e.JSON(http.StatusAccepted, out)
	})
	subGroup.POST("/import-batches/{batchId}/cancel", func(e *core.RequestEvent) error {
		svc, err := mustImport(e)
		if err != nil {
			return e.InternalServerError("Failed to initialize the import service.", err)
		}
		out, err := svc.CancelImportBatch(e.Request.Context(), e.Request.PathValue("batchId"))
		if err != nil {
			return pbimportError(e, err)
		}
		return e.JSON(http.StatusAccepted, out)
	})
	subGroup.GET("/pocketbase-imports/{importId}/tenants", func(e *core.RequestEvent) error {
		svc, err := mustImport(e)
		if err != nil {
			return e.InternalServerError("Failed to initialize the import service.", err)
		}
		out, err := svc.ListSourceTenants(e.Request.Context(), e.Request.PathValue("importId"))
		if err != nil {
			return pbimportError(e, err)
		}
		return e.JSON(http.StatusOK, out)
	})

	// pbimport.artifact.get — stream a manifest-listed artifact after
	// verifying the id IS the artifact's sha256 (never an arbitrary path).
	subGroup.GET("/pocketbase-imports/{importId}/artifacts/{artifactId}", func(e *core.RequestEvent) error {
		svc, err := mustImport(e)
		if err != nil {
			return e.InternalServerError("Failed to initialize the import service.", err)
		}
		path, _, aerr := svc.GetArtifact(e.Request.Context(), e.Request.PathValue("importId"), e.Request.PathValue("artifactId"))
		if aerr != nil {
			if strings.Contains(aerr.Error(), "artifact not found") {
				return e.NotFoundError("Unknown artifact.", nil)
			}
			return pbimportError(e, aerr)
		}
		e.Response.Header().Set("Content-Type", "application/octet-stream")
		http.ServeFile(e.Response, e.Request, path)
		return nil
	})
}

// superuserOnlyImport binds the superuser middleware to one route.
func superuserOnlyImport(route *router.Route[*core.RequestEvent]) *router.Route[*core.RequestEvent] {
	return route.Bind(RequireSuperuserAuth())
}

// adminJobKernelFor opens the primary job kernel for the request.
func adminJobKernelFor(e *core.RequestEvent) (*dbexec.JobKernel, error) {
	rt, err := application.BindRuntime(e.App)
	if err != nil {
		return nil, e.InternalServerError("Failed to initialize the partitioning runtime.", err)
	}
	return dbexec.NewJobKernel(rt.PrimaryDB)
}

// runPbimportResume reconciles a failed session (direct runner).
func runPbimportResume(ctx context.Context, svc *pbimport.Service, job *dbexec.Job) (any, error) {
	var payload struct {
		ImportID         string  `json:"importId"`
		ExpectedRevision int64   `json:"expectedRevision"`
		PlanHash         *string `json:"planHash"`
	}
	if err := jsonUnmarshalPayload(job.Payload, &payload); err != nil {
		return nil, err
	}
	sess, note, err := svc.Resume(ctx, payload.ImportID, payload.ExpectedRevision, payload.PlanHash)
	if err != nil {
		return nil, err
	}
	return map[string]any{"session": sess, "reconcile": note}, nil
}

// runPbimportRecovery runs the requested recovery action (direct runner).
func runPbimportRecovery(ctx context.Context, svc *pbimport.Service, job *dbexec.Job) (any, error) {
	var payload struct {
		ImportID                 string `json:"importId"`
		ExpectedRevision         int64  `json:"expectedRevision"`
		PlanHash                 string `json:"planHash"`
		Action                   string `json:"action"`
		ExpectedActiveGeneration int64  `json:"expectedActiveGeneration"`
	}
	if err := jsonUnmarshalPayload(job.Payload, &payload); err != nil {
		return nil, err
	}
	switch payload.Action {
	case "rollback-before-new-writes":
		return svc.RecoveryBeforeNewWrites(ctx, payload.ImportID, payload.ExpectedRevision, payload.PlanHash, payload.ExpectedActiveGeneration)
	case "export-current-and-preserve":
		return svc.RecoveryExportCurrent(ctx, payload.ImportID, payload.ExpectedRevision, payload.PlanHash, payload.ExpectedActiveGeneration)
	}
	return nil, fmt.Errorf("unknown recovery action %q", payload.Action)
}

var contentRangeRe = regexp.MustCompile(`^bytes [0-9]+-[0-9]+/[0-9]+$`)
var shaRe = regexp.MustCompile(`^[a-f0-9]{64}$`)

// pbimportError maps the import errors onto the additive envelope.
func pbimportError(e *core.RequestEvent, err error) error {
	msg := err.Error()
	code := "invalid_request"
	status := 400
	switch {
	case errors.Is(err, pbimport.ErrNotFound):
		code, status = "not_found", 404
	case errors.Is(err, pbimport.ErrSourceDataRequired):
		code, status = "source_data_required", 422
	case errors.Is(err, pbimport.ErrSourceChanged):
		code, status = "source_changed", 409
	case errors.Is(err, pbimport.ErrSourceIncomplete):
		code, status = "source_incomplete", 422
	case errors.Is(err, pbimport.ErrPlanStale):
		code, status = "plan_stale", 409
	case errors.Is(err, pbimport.ErrUnresolvedIssues):
		code, status = "unresolved_import_issues", 422
	case errors.Is(err, pbimport.ErrReportStale):
		code, status = "report_stale", 409
	case errors.Is(err, pbimport.ErrGenerationMismatch):
		code, status = "generation_mismatch", 409
	case errors.Is(err, pbimport.ErrBatchState):
		code, status = "batch_state", 409
	case errors.Is(err, pbimport.ErrConflict):
		code, status = "conflict", 409
	}
	if msg == code {
		msg = code
	} else if !strings.HasPrefix(msg, code) {
		msg = code + ": " + msg
	}
	return e.JSON(status, map[string]any{
		"data":    map[string]any{"mirabase": map[string]any{"code": code}},
		"message": msg,
		"status":  status,
	})
}
