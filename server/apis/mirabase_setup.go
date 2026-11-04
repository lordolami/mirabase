package apis

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/mail"
	"strconv"
	"strings"

	"mirabase/core"
	"mirabase/internal/application"
	"mirabase/internal/dbexec"
	"mirabase/tools/hook"
	"mirabase/internal/setup"
	"mirabase/tools/mailer"
	"mirabase/tools/router"
)

// bindMirabaseReferenceWorker starts the shared reference-copy refresh
// worker under the serve lifecycle (implementation-closure.md: "The
// source CDC commit schedules a durable refresh intent. A shared worker
// reads a consistent authorized source snapshot ..."): the worker binds
// at OnServe and closes with the app's bootstrap-clear, mirroring the
// gateway listener hook pair. One worker instance serves the whole
// process; its idle ticks are cheap journal-head reads, and every
// policy's copies stay inside their declared maximum lag without a
// manual refresh call.
func bindMirabaseReferenceWorker(app core.App) {
	app.OnServe().Bind(&hook.Handler[*core.ServeEvent]{
		Id:       "__mbReferenceWorkerStart__",
		Priority: 998,
		Func: func(se *core.ServeEvent) error {
			rt, err := application.BindRuntime(app)
			if err != nil {
				return se.Next()
			}
			worker := application.StartReferenceWorker(rt, application.BaseAppOf(app), 0, app.Logger())
			app.OnBootstrapClear().Bind(&hook.Handler[*core.BootstrapEvent]{
				Id:       "__mbReferenceWorkerStop__",
				Priority: 998,
				Func: func(be *core.BootstrapEvent) error {
					worker.Stop()
					return be.Next()
				},
			})
			app.Logger().Info("Reference-copy refresh worker started", "interval", "5s")
			return se.Next()
		},
	})
}

// bindMirabaseSetupApi registers the guided-setup, template and rollout
// operations on /api/mirabase/v1 (superuser authority; v0.1 slice: default
// application, validated empty-layout activation, explicit-target
// rollouts).
func bindMirabaseSetupApi(app core.App, subGroup *router.RouterGroup[*core.RequestEvent]) {
	// The setup/template/rollout operations are superuser-only; the record
	// endpoints on the shared group stay record-authenticated. The
	// superuser middleware binds PER ROUTE so it never widens to the
	// record routes on the shared group.
	superuserOnly := func(route *router.Route[*core.RequestEvent]) *router.Route[*core.RequestEvent] {
		return route.Bind(RequireSuperuserAuth())
	}

	mustSetup := func(e *core.RequestEvent) (*setup.Service, error) {
		rt, err := application.BindRuntime(app)
		if err != nil {
			return nil, err
		}
		return setup.NewService(rt.PrimaryDB, rt)
	}

	// setup.get — GET /api/mirabase/v1/setup
	superuserOnly(subGroup.GET("/setup", func(e *core.RequestEvent) error {
		svc, err := mustSetup(e)
		if err != nil {
			return e.InternalServerError("Failed to initialize the setup service.", err)
		}
		st, err := svc.GetState(e.Request.Context())
		if err != nil {
			return e.InternalServerError("Failed to load the setup state.", err)
		}
		return e.JSON(http.StatusOK, st)
	}))

	// setup.plan — PUT /api/mirabase/v1/setup/plan
	superuserOnly(subGroup.PUT("/setup/plan", func(e *core.RequestEvent) error {
		svc, err := mustSetup(e)
		if err != nil {
			return e.InternalServerError("Failed to initialize the setup service.", err)
		}
		var req setup.PlanRequest
		if err := json.NewDecoder(e.Request.Body).Decode(&req); err != nil {
			return e.BadRequestError("invalid_request: failed to decode the plan", err)
		}
		base := baseAppOrDie(app, e)
		if base == nil {
			return nil // the 500 response is already written
		}
		plan, err := svc.PlanDraft(e.Request.Context(), &req, base)
		if err != nil {
			return setupError(e, err)
		}
		return e.JSON(http.StatusOK, plan)
	}))

	// setup.apply — POST /api/mirabase/v1/setup/plan/apply
	superuserOnly(subGroup.POST("/setup/plan/apply", func(e *core.RequestEvent) error {
		svc, err := mustSetup(e)
		if err != nil {
			return e.InternalServerError("Failed to initialize the setup service.", err)
		}
		var req struct {
			PlanID                        string `json:"planId"`
			ExpectedApplicationGeneration string `json:"expectedApplicationGeneration"`
			ExpectedRevision              string `json:"expectedRevision"`
			PlanHash                      string `json:"planHash"`
		}
		if err := json.NewDecoder(e.Request.Body).Decode(&req); err != nil {
			return e.BadRequestError("invalid_request: failed to decode the apply request", err)
		}
		base := baseAppOrDie(app, e)
		if base == nil {
			return nil // the 500 response is already written
		}
		plan, err := svc.ApplyPlan(e.Request.Context(), req.PlanID,
			req.ExpectedApplicationGeneration, req.ExpectedRevision, req.PlanHash, base)
		if err != nil {
			return setupError(e, err)
		}
		// 202: the durable job owns schema/policy publication.
		return e.JSON(http.StatusAccepted, plan)
	}))

	// template.publish — POST /api/mirabase/v1/templates
	superuserOnly(subGroup.POST("/templates", func(e *core.RequestEvent) error {
		svc, err := mustSetup(e)
		if err != nil {
			return e.InternalServerError("Failed to initialize the setup service.", err)
		}
		var req struct {
			TemplateID             string   `json:"templateId"`
			CollectionNames        []string `json:"collectionNames"`
			ExpectedSchemaRevision string   `json:"expectedSchemaRevision"`
		}
		if err := json.NewDecoder(e.Request.Body).Decode(&req); err != nil {
			return e.BadRequestError("invalid_request: failed to decode the template publish request", err)
		}
		base := baseAppOrDie(app, e)
		if base == nil {
			return nil // the 500 response is already written
		}
		tpl, err := svc.PublishTemplateWire(e.Request.Context(), base, req.TemplateID, req.CollectionNames, req.ExpectedSchemaRevision)
		if err != nil {
			return setupError(e, err)
		}
		return e.JSON(http.StatusOK, tpl)
	}))

	// template.list — GET /api/mirabase/v1/templates
	superuserOnly(subGroup.GET("/templates", func(e *core.RequestEvent) error {
		svc, err := mustSetup(e)
		if err != nil {
			return e.InternalServerError("Failed to initialize the setup service.", err)
		}
		list, err := svc.ListTemplates(e.Request.Context())
		if err != nil {
			return setupError(e, err)
		}
		return e.JSON(http.StatusOK, map[string]any{"items": list, "page": 1, "perPage": len(list), "totalItems": len(list), "totalPages": 1})
	}))

	// template.rollout — POST /api/mirabase/v1/templates/{templateId}/rollouts
	superuserOnly(subGroup.POST("/templates/{templateId}/rollouts", func(e *core.RequestEvent) error {
		svc, err := mustSetup(e)
		if err != nil {
			return e.InternalServerError("Failed to initialize the setup service.", err)
		}
		var req setup.RolloutRequest
		if err := json.NewDecoder(e.Request.Body).Decode(&req); err != nil {
			return e.BadRequestError("invalid_request: failed to decode the rollout request", err)
		}
		rolloutID, total, err := svc.StartRollout(e.Request.Context(), e.Request.PathValue("templateId"), &req)
		if err != nil {
			return setupError(e, err)
		}
		// 202: the job owns the bounded per-context migrations.
		return e.JSON(http.StatusAccepted, map[string]any{
			"rolloutId": rolloutID,
			"total":     total,
			"state":     "queued",
		})
	}))

	// rollout status — GET /api/mirabase/v1/rollouts/{rolloutId}
	superuserOnly(subGroup.GET("/rollouts/{rolloutId}", func(e *core.RequestEvent) error {
		svc, err := mustSetup(e)
		if err != nil {
			return e.InternalServerError("Failed to initialize the setup service.", err)
		}
		st, err := svc.RolloutStatus(e.Request.Context(), e.Request.PathValue("rolloutId"))
		if err != nil {
			return setupError(e, err)
		}
		return e.JSON(http.StatusOK, st)
	}))

	// -------------------------------------------------------------------------
	// R3 independent applications (one process, per-app data dirs/identities).
	// -------------------------------------------------------------------------
	superuserOnly(subGroup.POST("/applications", func(e *core.RequestEvent) error {
		var body struct {
			DisplayName string `json:"displayName"`
			Layout      string `json:"layout"`
		}
		if err := json.NewDecoder(e.Request.Body).Decode(&body); err != nil {
			return e.BadRequestError("invalid_request: failed to decode the application create request", err)
		}
		rt, err := application.BindRuntime(app)
		if err != nil {
			return e.InternalServerError("Failed to bind the partitioning runtime.", err)
		}
		out, err := rt.CreateApplication(e.Request.Context(), body.DisplayName, body.Layout)
		if err != nil {
			return setupError(e, err)
		}
		return e.JSON(http.StatusOK, application.MarshalApplicationsRow(out))
	}))
	superuserOnly(subGroup.GET("/applications", func(e *core.RequestEvent) error {
		rt, err := application.BindRuntime(app)
		if err != nil {
			return e.InternalServerError("Failed to bind the partitioning runtime.", err)
		}
		page, _ := strconv.Atoi(e.Request.URL.Query().Get("page"))
		perPage, _ := strconv.Atoi(e.Request.URL.Query().Get("perPage"))
		items, total, err := rt.ListApplications(e.Request.Context(), page, perPage)
		if err != nil {
			return e.InternalServerError("Failed to read the applications registry.", err)
		}
		out := []map[string]any{}
		for _, r := range items {
			out = append(out, application.MarshalApplicationsRow(r))
		}
		if perPage < 1 || perPage > 1000 {
			perPage = 30
		}
		if page < 1 {
			page = 1
		}
		return e.JSON(http.StatusOK, map[string]any{
			"items": out, "page": page, "perPage": perPage,
			"totalItems": total, "totalPages": (total + int64(perPage) - 1) / int64(perPage),
		})
	}))
	superuserOnly(subGroup.GET("/applications/{appId}", func(e *core.RequestEvent) error {
		rt, err := application.BindRuntime(app)
		if err != nil {
			return e.InternalServerError("Failed to bind the partitioning runtime.", err)
		}
		out, err := rt.GetApplication(e.Request.Context(), e.Request.PathValue("appId"))
		if err != nil {
			return setupError(e, err)
		}
		return e.JSON(http.StatusOK, application.MarshalApplicationsRow(out))
	}))
	superuserOnly(subGroup.POST("/applications/{appId}/state", func(e *core.RequestEvent) error {
		var body struct {
			Action             string `json:"action"`
			ExpectedGeneration int64  `json:"expectedGeneration"`
		}
		if err := json.NewDecoder(e.Request.Body).Decode(&body); err != nil {
			return e.BadRequestError("invalid_request: failed to decode the state change", err)
		}
		rt, err := application.BindRuntime(app)
		if err != nil {
			return e.InternalServerError("Failed to bind the partitioning runtime.", err)
		}
		out, err := rt.ApplicationStateChange(e.Request.Context(), e.Request.PathValue("appId"), body.Action, body.ExpectedGeneration)
		if err != nil {
			return setupError(e, err)
		}
		return e.JSON(http.StatusOK, application.MarshalApplicationsRow(out))
	}))
	superuserOnly(subGroup.DELETE("/applications/{appId}", func(e *core.RequestEvent) error {
		var body struct {
			ExpectedGeneration int64 `json:"expectedGeneration"`
		}
		if err := json.NewDecoder(e.Request.Body).Decode(&body); err != nil {
			return e.BadRequestError("invalid_request: failed to decode the delete request", err)
		}
		rt, err := application.BindRuntime(app)
		if err != nil {
			return e.InternalServerError("Failed to bind the partitioning runtime.", err)
		}
		if err := rt.DeleteApplication(e.Request.Context(), e.Request.PathValue("appId"), body.ExpectedGeneration); err != nil {
			return setupError(e, err)
		}
		return e.NoContent(http.StatusNoContent)
	}))
	superuserOnly(subGroup.POST("/applications/{appId}/bootstrap", func(e *core.RequestEvent) error {
		var body struct {
			Email            string `json:"email"`
			Password         string `json:"password"`
			PasswordConfirm  string `json:"passwordConfirm"`
		}
		if err := json.NewDecoder(e.Request.Body).Decode(&body); err != nil {
			return e.BadRequestError("invalid_request: failed to decode the bootstrap request", err)
		}
		if body.Email == "" || body.Password == "" || body.Password != body.PasswordConfirm {
			return e.BadRequestError("email, password and matching passwordConfirm are required", nil)
		}
		rt, err := application.BindRuntime(app)
		if err != nil {
			return e.InternalServerError("Failed to bind the partitioning runtime.", err)
		}
		if err := rt.BootstrapApplicationAdministrator(e.Request.Context(), e.Request.PathValue("appId"), body.Email, body.Password); err != nil {
			return setupError(e, err)
		}
		return e.JSON(http.StatusOK, map[string]any{"status": "administrator-established"})
	}))

	// layout.apply — stage the verified layout conversion (R2 remainder:
	// populated shared collections are refused with the honest
	// layout_migration_required; empty targets activate through the
	// existing guarded plan publication).
	superuserOnly(subGroup.POST("/layout/apply", func(e *core.RequestEvent) error {
		var body struct {
			ExpectedApplicationGeneration int    `json:"expectedApplicationGeneration"`
			ExpectedRevision              int    `json:"expectedRevision"`
			PlanHash                      string `json:"planHash"`
			DestinationLayout             string `json:"destinationLayout"`
			BackupID                      string `json:"backupId"`
		}
		if err := json.NewDecoder(e.Request.Body).Decode(&body); err != nil {
			return e.BadRequestError("invalid_request: failed to decode the layout change", err)
		}
		svc, err := mustSetup(e)
		if err != nil {
			return e.InternalServerError("Failed to initialize the setup service.", err)
		}
		st, serr := svc.GetState(e.Request.Context())
		if serr != nil {
			return e.InternalServerError("Failed to load the setup state.", serr)
		}
		if st.Layout == body.DestinationLayout {
			return e.JSON(http.StatusConflict, map[string]any{
				"data":    map[string]any{"mirabase": map[string]any{"code": "conflict"}},
				"message": "the requested layout is already active",
				"status":  409,
			})
		}
		// The R2 populated-relocation contract: a populated shared
		// collection cannot be relocated in this release.
		if body.DestinationLayout != "shared" {
			base := baseAppOrDie(app, e)
			if base == nil {
				return nil
			}
			populated, perr := svc.HasPopulatedPlacements(e.Request.Context(), base)
			if perr != nil {
				return e.InternalServerError("Failed to inspect the populated collections.", perr)
			}
			if populated {
				return e.JSON(http.StatusUnprocessableEntity, map[string]any{
					"data": map[string]any{"mirabase": map[string]any{
						"code":     "layout_migration_required",
						"recovery": "the populated-data relocation is the R2 layout migration contract; existing data remains unchanged",
					}},
					"message": "layout_migration_required: populated shared collections cannot be relocated in this release",
					"status":  422,
				})
			}
		}
		// Empty targets activate through the existing guarded plan path.
		return e.JSON(http.StatusConflict, map[string]any{
			"data":    map[string]any{"mirabase": map[string]any{"code": "plan_required"}},
			"message": "layout activation requires a validated setup plan (setup.plan + setup.apply) whose planHash matches the requested destination",
			"status":  409,
		})
	}))

	// reference.define — PUT /api/mirabase/v1/reference-copies/{policyId}
	superuserOnly(subGroup.PUT("/reference-copies/{policyId}", func(e *core.RequestEvent) error {
		rtRuntime, rterr := application.BindRuntime(app)
		if rterr != nil {
			return e.InternalServerError("Failed to bind the partitioning runtime.", rterr)
		}
		rtService := rtRuntime.Service
		var policy application.ReferencePolicyWire
		if err := json.NewDecoder(e.Request.Body).Decode(&policy); err != nil {
			return e.BadRequestError("invalid_request: failed to decode the reference policy", err)
		}
		policy.ID = e.Request.PathValue("policyId")
		out, err := rtService.DefineReferencePolicy(e.Request.Context(), baseAppOrDie(app, e), &policy)
		if err != nil {
			return setupError(e, err)
		}
		return e.JSON(http.StatusOK, out)
	}))

	// reference.status — GET /api/mirabase/v1/reference-copies/{policyId}
	superuserOnly(subGroup.GET("/reference-copies/{policyId}", func(e *core.RequestEvent) error {
		rtRuntime, rterr := application.BindRuntime(app)
		if rterr != nil {
			return e.InternalServerError("Failed to bind the partitioning runtime.", rterr)
		}
		rtService := rtRuntime.Service
		policy, err := rtService.GetReferencePolicy(e.Request.Context(), e.Request.PathValue("policyId"))
		if err != nil {
			return setupError(e, err)
		}
		rt, err := application.BindRuntime(app)
		if err != nil {
			return e.InternalServerError("Failed to bind the partitioning runtime.", err)
		}
		st, err := rt.GetReferenceStatus(e.Request.Context(), policy)
		if err != nil {
			return setupError(e, err)
		}
		return e.JSON(http.StatusOK, st)
	}))

	// reference.refresh — POST /api/mirabase/v1/reference-copies/{policyId}/refresh
	superuserOnly(subGroup.POST("/reference-copies/{policyId}/refresh", func(e *core.RequestEvent) error {
		rtRuntime, rterr := application.BindRuntime(app)
		if rterr != nil {
			return e.InternalServerError("Failed to bind the partitioning runtime.", rterr)
		}
		rtService := rtRuntime.Service
		var body struct {
			Revision int `json:"revision"`
		}
		_ = json.NewDecoder(e.Request.Body).Decode(&body)
		policy, err := rtService.GetReferencePolicy(e.Request.Context(), e.Request.PathValue("policyId"))
		if err != nil {
			return setupError(e, err)
		}
		rt, err := application.BindRuntime(app)
		if err != nil {
			return e.InternalServerError("Failed to bind the partitioning runtime.", err)
		}
		wireJob, runErr := recordJobRun(e, "reference.refresh", map[string]any{"policyId": policy.ID},
			func(ctx context.Context, job *dbexec.Job) (any, error) {
				return rt.RefreshReferencePolicy(ctx, baseAppOrDie(app, e), policy)
			})
		if runErr != nil {
			return e.InternalServerError("The reference refresh failed: "+runErr.Error(), nil)
		}
		return e.JSON(http.StatusAccepted, wireJob)
	}))

	// liveplan.create/list/publish/get/retire
	superuserOnly(subGroup.POST("/live-query-plans", func(e *core.RequestEvent) error {
		rtRuntime, rterr := application.BindRuntime(app)
		if rterr != nil {
			return e.InternalServerError("Failed to bind the partitioning runtime.", rterr)
		}
		rtService := rtRuntime.Service
		var plan application.LivePlanWire
		if err := json.NewDecoder(e.Request.Body).Decode(&plan); err != nil {
			return e.BadRequestError("invalid_request: failed to decode the live plan", err)
		}
		out, err := rtService.CreateLivePlan(e.Request.Context(), baseAppOrDie(app, e), &plan)
		if err != nil {
			return setupError(e, err)
		}
		return e.JSON(http.StatusOK, out)
	}))
	superuserOnly(subGroup.GET("/live-query-plans", func(e *core.RequestEvent) error {
		rtRuntime, rterr := application.BindRuntime(app)
		if rterr != nil {
			return e.InternalServerError("Failed to bind the partitioning runtime.", rterr)
		}
		rtService := rtRuntime.Service
		items, err := rtService.ListLivePlans(e.Request.Context())
		if err != nil {
			return setupError(e, err)
		}
		return e.JSON(http.StatusOK, map[string]any{"items": items, "totalItems": len(items)})
	}))
	superuserOnly(subGroup.POST("/live-query-plans/{planId}/revisions/{revision}/publish", func(e *core.RequestEvent) error {
		rtRuntime, rterr := application.BindRuntime(app)
		if rterr != nil {
			return e.InternalServerError("Failed to bind the partitioning runtime.", rterr)
		}
		// Publication verifies the stored SQL against the ACTUAL target
		// profiles (prepare read-only + declared tables) before the state
		// flips (implementation-closure.md: "SQL must prepare read-only in
		// the actual profile").
		rev, _ := strconv.ParseInt(e.Request.PathValue("revision"), 10, 64)
		out, err := rtRuntime.PublishLivePlan(e.Request.Context(), e.Request.PathValue("planId"), rev)
		if err != nil {
			return setupError(e, err)
		}
		return e.JSON(http.StatusOK, out)
	}))
	superuserOnly(subGroup.GET("/live-query-plans/{planId}/revisions/{revision}", func(e *core.RequestEvent) error {
		rtRuntime, rterr := application.BindRuntime(app)
		if rterr != nil {
			return e.InternalServerError("Failed to bind the partitioning runtime.", rterr)
		}
		rtService := rtRuntime.Service
		rev, _ := strconv.ParseInt(e.Request.PathValue("revision"), 10, 64)
		out, err := rtService.GetLivePlan(e.Request.Context(), e.Request.PathValue("planId"), rev)
		if err != nil {
			return setupError(e, err)
		}
		return e.JSON(http.StatusOK, out)
	}))
	superuserOnly(subGroup.POST("/live-query-plans/{planId}/revisions/{revision}/retire", func(e *core.RequestEvent) error {
		rtRuntime, rterr := application.BindRuntime(app)
		if rterr != nil {
			return e.InternalServerError("Failed to bind the partitioning runtime.", rterr)
		}
		rtService := rtRuntime.Service
		rev, _ := strconv.ParseInt(e.Request.PathValue("revision"), 10, 64)
		out, err := rtService.RetireLivePlan(e.Request.Context(), e.Request.PathValue("planId"), rev)
		if err != nil {
			return setupError(e, err)
		}
		return e.JSON(http.StatusOK, out)
	}))

	// relationworkflow.define/run
	superuserOnly(subGroup.PUT("/relation-workflows/{workflowId}", func(e *core.RequestEvent) error {
		var wf application.RelationWorkflowWire
		if err := json.NewDecoder(e.Request.Body).Decode(&wf); err != nil {
			return e.BadRequestError("invalid_request: failed to decode the workflow", err)
		}
		wf.ID = e.Request.PathValue("workflowId")
		rt, err := application.BindRuntime(app)
		if err != nil {
			return e.InternalServerError("Failed to bind the partitioning runtime.", err)
		}
		out, err := rt.DefineRelationWorkflow(e.Request.Context(), &wf)
		if err != nil {
			return setupError(e, err)
		}
		return e.JSON(http.StatusOK, out)
	}))
	superuserOnly(subGroup.POST("/relation-workflows/{workflowId}/run", func(e *core.RequestEvent) error {
		var body struct {
			Revision int64 `json:"revision"`
		}
		if err := json.NewDecoder(e.Request.Body).Decode(&body); err != nil {
			return e.BadRequestError("invalid_request: failed to decode the workflow run request", err)
		}
		rt, err := application.BindRuntime(app)
		if err != nil {
			return e.InternalServerError("Failed to bind the partitioning runtime.", err)
		}
		wireJob, runErr := recordJobRun(e, "relationworkflow.run",
			map[string]any{"workflowId": e.Request.PathValue("workflowId"), "revision": body.Revision},
			func(ctx context.Context, job *dbexec.Job) (any, error) {
				return rt.RunRelationWorkflow(ctx, e.Request.PathValue("workflowId"), body.Revision)
			})
		if runErr != nil {
			return e.InternalServerError("The workflow run failed: "+runErr.Error(), nil)
		}
		return e.JSON(http.StatusAccepted, wireJob)
	}))
	superuserOnly(subGroup.GET("/relation-workflows/{workflowId}/revisions/{revision}", func(e *core.RequestEvent) error {
		rt, err := application.BindRuntime(app)
		if err != nil {
			return e.InternalServerError("Failed to bind the partitioning runtime.", err)
		}
		rev, _ := strconv.ParseInt(e.Request.PathValue("revision"), 10, 64)
		out, err := rt.GetRelationWorkflow(e.Request.Context(), e.Request.PathValue("workflowId"), rev)
		if err != nil {
			return setupError(e, err)
		}
		return e.JSON(http.StatusOK, out)
	}))

	// report.define/rebuild
	superuserOnly(subGroup.PUT("/reports/{reportId}", func(e *core.RequestEvent) error {
		rtRuntime, rterr := application.BindRuntime(app)
		if rterr != nil {
			return e.InternalServerError("Failed to bind the partitioning runtime.", rterr)
		}
		rtService := rtRuntime.Service
		var report application.ReportWire
		if err := json.NewDecoder(e.Request.Body).Decode(&report); err != nil {
			return e.BadRequestError("invalid_request: failed to decode the report", err)
		}
		report.ID = e.Request.PathValue("reportId")
		out, err := rtService.DefineReport(e.Request.Context(), baseAppOrDie(app, e), &report)
		if err != nil {
			return setupError(e, err)
		}
		return e.JSON(http.StatusOK, out)
	}))
	superuserOnly(subGroup.POST("/reports/{reportId}/rebuild", func(e *core.RequestEvent) error {
		rt, err := application.BindRuntime(app)
		if err != nil {
			return e.InternalServerError("Failed to bind the partitioning runtime.", err)
		}
		wireJob, runErr := recordJobRun(e, "report.rebuild",
			map[string]any{"reportId": e.Request.PathValue("reportId")},
			func(ctx context.Context, job *dbexec.Job) (any, error) {
				return rt.RebuildReport(ctx, e.Request.PathValue("reportId"))
			})
		if runErr != nil {
			return e.InternalServerError("The report rebuild failed: "+runErr.Error(), nil)
		}
		return e.JSON(http.StatusAccepted, wireJob)
	}))
	superuserOnly(subGroup.GET("/reports/{reportId}", func(e *core.RequestEvent) error {
		rtRuntime, rterr := application.BindRuntime(app)
		if rterr != nil {
			return e.InternalServerError("Failed to bind the partitioning runtime.", rterr)
		}
		rtService := rtRuntime.Service
		out, err := rtService.GetReport(e.Request.Context(), e.Request.PathValue("reportId"))
		if err != nil {
			return setupError(e, err)
		}
		return e.JSON(http.StatusOK, out)
	}))

	// membership.invite — POST /api/mirabase/v1/membership-invitations
	// (authority: superuser or the tenant key record's owner; the token
	// travels through the mail outbox, never the response).
	superuserOnly(subGroup.POST("/membership-invitations", func(e *core.RequestEvent) error {
		svc, err := mustSetup(e)
		if err != nil {
			return e.InternalServerError("Failed to initialize the setup service.", err)
		}
		var req setup.InvitationCreate
		if err := json.NewDecoder(e.Request.Body).Decode(&req); err != nil {
			return e.BadRequestError("invalid_request: failed to decode the invitation request", err)
		}
		inviterCollection, inviterRecord := "", ""
		isSuper := e.HasSuperuserAuth()
		if !isSuper {
			if e.Auth == nil {
				return e.UnauthorizedError("", nil)
			}
			inviterCollection, inviterRecord = e.Auth.Collection().Id, e.Auth.Id
		}
		inv, token, err := svc.CreateInvitation(e.Request.Context(), baseAppOrDie(app, e), &req, inviterCollection, inviterRecord, isSuper)
		if err != nil {
			return setupError(e, err)
		}
		// The mail outbox effect: the inherited mailer delivers the token
		// when SMTP is configured; without SMTP the message is logged by
		// the inherited sender (never returned over the wire).
		sendInvitationMail(app, inv.ID, token, req)
		return e.JSON(http.StatusOK, inv)
	}))

	// membership.accept — POST /api/mirabase/v1/membership-invitations/accept
	// (authority: the authenticated intended account).
	subGroup.POST("/membership-invitations/accept", func(e *core.RequestEvent) error {
		svc, err := mustSetup(e)
		if err != nil {
			return e.InternalServerError("Failed to initialize the setup service.", err)
		}
		var body struct {
			Token                         string `json:"token"`
			ExpectedApplicationGeneration int    `json:"expectedApplicationGeneration"`
		}
		if err := json.NewDecoder(e.Request.Body).Decode(&body); err != nil {
			return e.BadRequestError("invalid_request: failed to decode the acceptance request", err)
		}
		if e.Auth == nil {
			return e.UnauthorizedError("", nil)
		}
		inv, note, err := svc.AcceptInvitation(e.Request.Context(), baseAppOrDie(app, e), body.Token, e.Auth.Collection().Id, e.Auth.Id)
		if err != nil {
			return setupError(e, err)
		}
		return e.JSON(http.StatusOK, map[string]any{
			"id": inv.ID, "state": inv.State, "expiresAt": inv.ExpiresAt, "revision": inv.Revision,
			"membership": note,
		})
	}).Bind(RequireAuth())

	// membership.revoke — DELETE /api/mirabase/v1/membership-invitations/{inviteId}
	superuserOnly(subGroup.DELETE("/membership-invitations/{inviteId}", func(e *core.RequestEvent) error {
		svc, err := mustSetup(e)
		if err != nil {
			return e.InternalServerError("Failed to initialize the setup service.", err)
		}
		inviterCollection, inviterRecord := "", ""
		isSuper := e.HasSuperuserAuth()
		if !isSuper {
			if e.Auth == nil {
				return e.UnauthorizedError("", nil)
			}
			inviterCollection, inviterRecord = e.Auth.Collection().Id, e.Auth.Id
		}
		inv, err := svc.RevokeInvitation(e.Request.Context(), e.Request.PathValue("inviteId"), inviterCollection, inviterRecord, isSuper)
		if err != nil {
			return setupError(e, err)
		}
		return e.JSON(http.StatusOK, inv)
	}))

	// setup.administrator — POST /api/mirabase/v1/setup/administrator
	// (purpose-bound bootstrap token for an application with no
	// administrator; creates the first admin through inherited forms).
	// The bootstrap TOKEN is the authority: the group record-auth
	// middleware is excluded for this route.
	bootstrapRoute := subGroup.POST("/setup/administrator", func(e *core.RequestEvent) error {
		var body struct {
			Token                         string `json:"token"`
			Email                         string `json:"email"`
			Password                      string `json:"password"`
			PasswordConfirm               string `json:"passwordConfirm"`
			ExpectedApplicationGeneration int    `json:"expectedApplicationGeneration"`
		}
		if err := json.NewDecoder(e.Request.Body).Decode(&body); err != nil {
			return e.BadRequestError("invalid_request: failed to decode the bootstrap request", err)
		}
		base := baseAppOrDie(app, e)
		if base == nil {
			return nil
		}
		if body.Token == "" || body.Email == "" || body.Password == "" {
			return e.BadRequestError("token, email and password are required", nil)
		}
		if body.Password != body.PasswordConfirm {
			return e.BadRequestError("password and passwordConfirm do not match", nil)
		}
		// The token must authenticate the SYSTEM installer superuser.
		owner, terr := base.FindAuthRecordByToken(body.Token, core.TokenTypeAuth)
		if terr != nil || owner == nil || owner.Email() != core.DefaultInstallerEmail {
			return e.ForbiddenError("The bootstrap token is not a purpose-bound installer token.", nil)
		}
		// The application must have NO administrator yet (the installer
		// superuser is a system marker, not an administrator).
		total, terr := base.CountRecords(core.CollectionNameSuperusers)
		if terr != nil {
			return e.InternalServerError("Failed to count the administrators.", terr)
		}
		if total > 1 {
			return e.JSON(http.StatusConflict, map[string]any{
				"data":    map[string]any{"mirabase": map[string]any{"code": "conflict"}},
				"message": "the application already has an administrator",
				"status":  409,
			})
		}
		// Create the first administrator through the inherited forms (the
		// field validators run inside Save).
		superusers, serr := base.FindCollectionByNameOrId(core.CollectionNameSuperusers)
		if serr != nil {
			return e.InternalServerError("Failed to load the superusers collection.", serr)
		}
		admin := core.NewRecord(superusers)
		admin.Set("email", body.Email)
		admin.Set("password", body.Password)
		if serr := app.Save(admin); serr != nil {
			return e.BadRequestError("Failed to create the administrator.", serr)
		}
		// Consume the scoped token: the installer superuser (and therefore
		// its token) is deleted.
		if derr := app.Delete(owner); derr != nil {
			return e.InternalServerError("Failed to consume the bootstrap token.", derr)
		}
		return e.JSON(http.StatusOK, map[string]any{
			"applicationId": application.ApplicationID,
			"status":        "administrator-established",
		})
	})
	// The bootstrap TOKEN is the authority: exclude the group record-auth.
	bootstrapRoute.Unbind("pbRequireAuth")

	// rollout.pause — stop new target admission (running targets complete).
	superuserOnly(subGroup.POST("/rollouts/{rolloutId}/pause", func(e *core.RequestEvent) error {
		svc, err := mustSetup(e)
		if err != nil {
			return e.InternalServerError("Failed to initialize the setup service.", err)
		}
		st, err := svc.PauseRollout(e.Request.Context(), e.Request.PathValue("rolloutId"))
		if err != nil {
			return setupError(e, err)
		}
		return e.JSON(http.StatusOK, st)
	}))

	// rollout.resume — clear the pause and revalidate each remaining
	// target's current generation against the snapshot.
	superuserOnly(subGroup.POST("/rollouts/{rolloutId}/resume", func(e *core.RequestEvent) error {
		svc, err := mustSetup(e)
		if err != nil {
			return e.InternalServerError("Failed to initialize the setup service.", err)
		}
		base := baseAppOrDie(app, e)
		if base == nil {
			return nil
		}
		rt, err := application.BindRuntime(app)
		if err != nil {
			return e.InternalServerError("Failed to bind the partitioning runtime.", err)
		}
		st, err := svc.ResumeRollout(e.Request.Context(), e.Request.PathValue("rolloutId"), base.DataDir(), rt.Handles)
		if err != nil {
			return setupError(e, err)
		}
		return e.JSON(http.StatusOK, st)
	}))

	// rollout.retry — re-arm ONLY failed targets (succeeded targets are
	// never migrated twice).
	superuserOnly(subGroup.POST("/rollouts/{rolloutId}/retry", func(e *core.RequestEvent) error {
		svc, err := mustSetup(e)
		if err != nil {
			return e.InternalServerError("Failed to initialize the setup service.", err)
		}
		st, err := svc.RetryRollout(e.Request.Context(), e.Request.PathValue("rolloutId"))
		if err != nil {
			return setupError(e, err)
		}
		return e.JSON(http.StatusOK, st)
	}))

	// rollout.targets — the exact paginated per-target journal.
	superuserOnly(subGroup.GET("/rollouts/{rolloutId}/targets", func(e *core.RequestEvent) error {
		svc, err := mustSetup(e)
		if err != nil {
			return e.InternalServerError("Failed to initialize the setup service.", err)
		}
		page, _ := strconv.Atoi(e.Request.URL.Query().Get("page"))
		perPage, _ := strconv.Atoi(e.Request.URL.Query().Get("perPage"))
		items, total, err := svc.ListRolloutTargets(e.Request.Context(), e.Request.PathValue("rolloutId"), page, perPage)
		if err != nil {
			return setupError(e, err)
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
	}))

	// rollout run — POST /api/mirabase/v1/rollouts/{rolloutId}/run
	// (drives the bounded fan-out; v0.1 runs the job synchronously from
	// the operator action, the journal makes it resumable)
	superuserOnly(subGroup.POST("/rollouts/{rolloutId}/run", func(e *core.RequestEvent) error {
		svc, err := mustSetup(e)
		if err != nil {
			return e.InternalServerError("Failed to initialize the setup service.", err)
		}
		base := baseAppOrDie(app, e)
		if base == nil {
			return nil // the 500 response is already written
		}
		rt, err := application.BindRuntime(app)
		if err != nil {
			return e.InternalServerError("Failed to bind the partitioning runtime.", err)
		}
		done, summary, err := svc.RunRollout(e.Request.Context(), e.Request.PathValue("rolloutId"), base.DataDir(), rt.Handles)
		if err != nil {
			// A failed rollout leaves its journal resumable.
			resp := map[string]any{"error": err.Error(), "summary": summary}
			return e.JSON(http.StatusInternalServerError, resp)
		}
		summary["done"] = done
		return e.JSON(http.StatusOK, summary)
	}))
}

// baseAppOrDie resolves the concrete BaseApp through the embed-aware
// application helper (launcher wrappers embed the core.App interface,
// test apps embed *core.BaseApp) and fails the request otherwise. A nil
// return is never passed to the caller's handler body: every failure
// response is written here, and callers must still nil-check before use.
func baseAppOrDie(app core.App, e *core.RequestEvent) *core.BaseApp {
	if b := application.BaseAppOf(app); b != nil {
		return b
	}
	_ = e.InternalServerError("The concrete BaseApp is not available.", nil)
	return nil
}

// setupError maps setup/rollout errors onto the additive error envelope
// ("Errors preserve the inherited top-level {status,message,data}; use
// HTTP 400/401/403/404/409/410/413/422/429/503/500 as appropriate").
func setupError(e *core.RequestEvent, err error) error {
	msg := err.Error()
	switch {
	case errors.Is(err, setup.ErrGenerationMismatch):
		return e.JSON(http.StatusConflict, map[string]any{
			"data":    map[string]any{"mirabase": map[string]any{"code": "generation_mismatch"}},
			"message": msg, "status": 409,
		})
	case errors.Is(err, setup.ErrPolicyChanged):
		return e.JSON(http.StatusConflict, map[string]any{
			"data":    map[string]any{"mirabase": map[string]any{"code": "policy_changed"}},
			"message": msg, "status": 409,
		})
	case errors.Is(err, setup.ErrSchemaMismatch):
		return e.JSON(http.StatusConflict, map[string]any{
			"data":    map[string]any{"mirabase": map[string]any{"code": "schema_mismatch"}},
			"message": msg, "status": 409,
		})
	case errors.Is(err, setup.ErrInvitationNotFound):
		return e.JSON(http.StatusNotFound, map[string]any{
			"data":    map[string]any{"mirabase": map[string]any{"code": "not_found"}},
			"message": msg, "status": 404,
		})
	case errors.Is(err, setup.ErrInvitationPolicy):
		return e.ForbiddenError(msg, nil)
	case errors.Is(err, application.ErrReferenceStale):
		return e.JSON(http.StatusConflict, map[string]any{
			"data":    map[string]any{"mirabase": map[string]any{"code": "policy_changed"}},
			"message": msg, "status": 409,
		})
	case errors.Is(err, setup.ErrInvitationState):
		return e.JSON(http.StatusConflict, map[string]any{
			"data":    map[string]any{"mirabase": map[string]any{"code": "conflict"}},
			"message": msg, "status": 409,
		})
	case errors.Is(err, setup.ErrRolloutNotFound):
		return e.JSON(http.StatusNotFound, map[string]any{
			"data":    map[string]any{"mirabase": map[string]any{"code": "not_found"}},
			"message": msg, "status": 404,
		})
	case errors.Is(err, setup.ErrLayoutMigrationRequired):
		return e.JSON(http.StatusUnprocessableEntity, map[string]any{
			"data": map[string]any{"mirabase": map[string]any{
				"code":     "layout_migration_required",
				"recovery": "the populated-data relocation is the R2 layout migration contract; existing data remains unchanged",
			}},
			"message": msg, "status": 422,
		})
	case strings.Contains(msg, "not_found"):
		return e.JSON(http.StatusNotFound, map[string]any{
			"data":    map[string]any{"mirabase": map[string]any{"code": "not_found"}},
			"message": msg, "status": 404,
		})
	}
	return e.BadRequestError(msg, nil)
}

// sendInvitationMail delivers the invitation token (net/mail is the
// inherited mailer's address type). through the inherited
// mail outbox (with SMTP configured it is emailed; without SMTP the
// inherited sender logs the message — the token never crosses the wire).
func sendInvitationMail(app core.App, inviteID, token string, req setup.InvitationCreate) {
	to := ""
	if req.Email != nil {
		to = *req.Email
	}
	if to == "" && req.Subject != nil {
		if rec, err := app.FindRecordById(req.Subject.CollectionID, req.Subject.RecordID); err == nil {
			to = rec.Email()
		}
	}
	if to == "" {
		return
	}
	message := &mailer.Message{
		From:    mail.Address{Name: app.Settings().Meta.SenderName, Address: app.Settings().Meta.SenderAddress},
		To:      []mail.Address{{Address: to}},
		Subject: "MiraBase invitation",
		Text:    "You have been invited. Accept with this token: " + token + " (invitation " + inviteID + ")",
	}
	_ = app.NewMailClient().Send(message)
}
