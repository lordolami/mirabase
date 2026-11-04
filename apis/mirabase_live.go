package apis

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"time"

	"mirabase/core"
	"mirabase/internal/application"
	"mirabase/internal/livequery"
	"mirabase/tools/router"
)

// ---------------------------------------------------------------------------
// D-14 live-query surface (live.register / live.stream / live.cancel,
// MB-LIVE): durable query-result subscriptions over published live plans,
// resolved through the application's plan registry (livequery.PlanResolver
// via application.Runtime.ResolvePublishedLivePlan). Authority is the
// existing record-auth path: the authenticated record must own the
// database or hold a live grant carrying record.read, re-verified on every
// stream step ("Recheck authorization before evaluation/publication").
//
// A later wiring step binds bindMirabaseLiveApi into bindMirabaseApi; the
// handlers enforce record authentication themselves (the changes.*
// pattern) so the surface is self-contained either way.
// ---------------------------------------------------------------------------

// bindMirabaseLiveApi registers the three live-query operations.
func bindMirabaseLiveApi(app core.App, subGroup *router.RouterGroup[*core.RequestEvent]) {
	service := livequery.NewService()

	// live.register: POST /databases/{id}/live-queries → the LiveQuery
	// wire shape (200 application/json).
	subGroup.POST("/databases/{id}/live-queries", func(e *core.RequestEvent) error {
		rt, target, dbRow, done := mirabaseLiveContext(e, app)
		if done != nil {
			return done
		}
		body, done := decodeLiveRegister(e)
		if done != nil {
			return done
		}
		if body.SchemaRevision != dbRow.SchemaRevision {
			return e.JSON(http.StatusConflict, map[string]any{
				"data":    map[string]any{"mirabase": map[string]any{"code": "schema_mismatch"}},
				"message": fmt.Sprintf("schema_mismatch: the registered schema revision %d does not match the current %d", body.SchemaRevision, dbRow.SchemaRevision),
				"status":  409,
			})
		}
		reg, err := service.Register(e.Request.Context(), rt, &livequery.RegisterInput{
			DatabaseID:        dbRow.ID,
			Handle:            target,
			QueryID:           body.QueryID,
			PlanRevision:      body.QueryRevision,
			Parameters:        body.Parameters,
			ActorCollectionID: e.Auth.Collection().Id,
			ActorRecordID:     e.Auth.Id,
			Generation:        dbRow.Generation,
			SchemaRevision:    dbRow.SchemaRevision,
			PolicyEpoch:       dbRow.PolicyEpoch,
		})
		if err != nil {
			return liveError(e, err)
		}
		return e.JSON(http.StatusOK, liveQueryWire(reg, body.QueryID))
	})

	// live.stream: GET /databases/{id}/live-queries/{queryId}/stream —
	// SSE over ordered authorized query deltas; the cursor (required)
	// binds the subscription, actor, context and sequence. Bounded loop:
	// the client reconnects with the last frame's cursor ("clients
	// reconnect with the cursor rather than assuming no losses").
	subGroup.GET("/databases/{id}/live-queries/{queryId}/stream", func(e *core.RequestEvent) error {
		rt, target, dbRow, done := mirabaseLiveContext(e, app)
		if done != nil {
			return done
		}
		queryID := e.Request.PathValue("queryId")
		cursor := e.Request.URL.Query().Get("cursor")
		if cursor == "" {
			return e.JSON(http.StatusBadRequest, map[string]any{
				"data":    map[string]any{"mirabase": map[string]any{"code": "invalid_request"}},
				"message": "invalid_request: the live stream requires its cursor",
				"status":  400,
			})
		}
		// Pre-flight validation before the SSE headers: cursor
		// authenticity, actor binding and subscription cross-check.
		subID, perr := service.ProbeCursor(e.Request.Context(), target, dbRow.ID,
			e.Auth.Collection().Id, e.Auth.Id, queryID, cursor)
		if perr != nil {
			return liveError(e, perr)
		}
		e.Response.Header().Set("Content-Type", "text/event-stream")
		e.Response.Header().Set("Cache-Control", "no-cache")
		flusher, _ := e.Response.(http.Flusher)
		if flusher != nil {
			flusher.Flush()
		}
		ctx := e.Request.Context()
		cursorToken := cursor
		for i := 0; i < 120; i++ {
			select {
			case <-ctx.Done():
				return nil
			default:
			}
			// The current registry context is re-read every step so a
			// schema/generation/policy change is detected by the step's
			// context re-check.
			dbNow, derr := rt.Service.GetDatabase(ctx, dbRow.ID)
			if derr != nil {
				writeLiveErrorFrame(e, flusher, dbRow.ID, queryID, "not_found", derr.Error())
				return nil
			}
			// Recheck authorization before every evaluation/publication
			// (revocation follows the five-second bound through this
			// per-step recheck).
			if err := rt.AuthorizeSubjects(ctx, dbRow.ID, subjectCollections(e.Auth), e.Auth.Id, application.ScopeRecordRead); err != nil {
				writeLiveErrorFrame(e, flusher, dbRow.ID, queryID, "forbidden", err.Error())
				return nil
			}
			res, err := service.Step(ctx, rt, &livequery.StepInput{
				DatabaseID:        dbRow.ID,
				Handle:            target,
				SubscriptionID:    subID,
				QueryID:           queryID,
				Cursor:            cursorToken,
				ActorCollectionID: e.Auth.Collection().Id,
				ActorRecordID:     e.Auth.Id,
				Generation:        dbNow.Generation,
				SchemaRevision:    dbNow.SchemaRevision,
				PolicyEpoch:       dbNow.PolicyEpoch,
			})
			if err != nil {
				writeLiveStepError(e, flusher, dbRow.ID, queryID, err)
				return nil
			}
			delivered := false
			for _, ev := range res.Events {
				writeLiveFrame(e, flusher, ev.Kind, ev.Data)
				delivered = true
			}
			cursorToken = res.Cursor
			if !delivered {
				fmt.Fprint(e.Response, ": keepalive\n\n")
			}
			if flusher != nil {
				flusher.Flush()
			}
			select {
			case <-ctx.Done():
				return nil
			case <-time.After(500 * time.Millisecond):
			}
		}
		return nil
	})

	// live.cancel: DELETE /databases/{id}/live-queries/{queryId} —
	// idempotent {id, closed:true}. An optional body selects the exact
	// subscription: {"id": "..."} or {"parameters": [...]} — an actor may
	// hold several subscriptions of the same plan with different
	// parameter sets, and the older one is only reachable this way.
	subGroup.DELETE("/databases/{id}/live-queries/{queryId}", func(e *core.RequestEvent) error {
		rt, target, dbRow, done := mirabaseLiveContext(e, app)
		if done != nil {
			return done
		}
		in := &livequery.CancelInput{
			DatabaseID:        dbRow.ID,
			Handle:            target,
			QueryID:           e.Request.PathValue("queryId"),
			ActorCollectionID: e.Auth.Collection().Id,
			ActorRecordID:     e.Auth.Id,
		}
		var raw map[string]json.RawMessage
		if err := json.NewDecoder(e.Request.Body).Decode(&raw); err == nil {
			for k := range raw {
				if k != "id" && k != "parameters" {
					return liveInvalidRequest(e, fmt.Sprintf("unknown request property %q", k))
				}
			}
			if v, ok := raw["id"]; ok {
				if err := json.Unmarshal(v, &in.SubscriptionID); err != nil {
					return liveInvalidRequest(e, "id must be a subscription id string")
				}
			}
			if v, ok := raw["parameters"]; ok {
				if err := json.Unmarshal(v, &in.Parameters); err != nil {
					return liveInvalidRequest(e, "parameters must be an array of typed values")
				}
			}
		}
		out, err := service.Cancel(e.Request.Context(), rt, in)
		if err != nil {
			return liveError(e, err)
		}
		return e.JSON(http.StatusOK, map[string]any{"id": out.ID, "closed": true})
	})
}

// liveRegisterBody is the validated live.register request.
type liveRegisterBody struct {
	QueryID        string
	Parameters     []any
	SchemaRevision int64
	QueryRevision  int64
}

// decodeLiveRegister decodes the strict request schema: required queryId,
// parameters (array, maxItems 100) and schemaRevision; optional
// queryRevision; additional properties refused.
func decodeLiveRegister(e *core.RequestEvent) (*liveRegisterBody, error) {
	var raw map[string]json.RawMessage
	if err := json.NewDecoder(e.Request.Body).Decode(&raw); err != nil {
		return nil, liveInvalidRequest(e, "failed to decode the live.register request")
	}
	allowed := map[string]bool{"queryId": true, "parameters": true, "schemaRevision": true, "queryRevision": true}
	for k := range raw {
		if !allowed[k] {
			return nil, liveInvalidRequest(e, fmt.Sprintf("unknown request property %q", k))
		}
	}
	body := &liveRegisterBody{}
	if v, ok := raw["queryId"]; ok {
		if err := json.Unmarshal(v, &body.QueryID); err != nil || body.QueryID == "" {
			return nil, liveInvalidRequest(e, "queryId is required")
		}
	} else {
		return nil, liveInvalidRequest(e, "queryId is required")
	}
	if v, ok := raw["parameters"]; ok {
		if err := json.Unmarshal(v, &body.Parameters); err != nil {
			return nil, liveInvalidRequest(e, "parameters must be an array of typed values")
		}
	} else {
		return nil, liveInvalidRequest(e, "parameters is required")
	}
	if len(body.Parameters) > 100 {
		return nil, liveInvalidRequest(e, "at most 100 parameters are accepted")
	}
	if v, ok := raw["schemaRevision"]; ok {
		if err := json.Unmarshal(v, &body.SchemaRevision); err != nil {
			return nil, liveInvalidRequest(e, "schemaRevision must be a revision number")
		}
	} else {
		return nil, liveInvalidRequest(e, "schemaRevision is required")
	}
	if v, ok := raw["queryRevision"]; ok {
		if err := json.Unmarshal(v, &body.QueryRevision); err != nil {
			return nil, liveInvalidRequest(e, "queryRevision must be a revision number")
		}
	}
	return body, nil
}

// liveInvalidRequest builds the additive invalid_request envelope.
func liveInvalidRequest(e *core.RequestEvent, message string) error {
	return e.JSON(http.StatusBadRequest, map[string]any{
		"data":    map[string]any{"mirabase": map[string]any{"code": "invalid_request"}},
		"message": "invalid_request: " + message,
		"status":  400,
	})
}

// mirabaseLiveContext resolves the runtime, record authentication, the
// database authorization and the target handle ("Resolve the database and
// current authority before rules/forms/hooks/files/SSE/search").
func mirabaseLiveContext(e *core.RequestEvent, app core.App) (*application.Runtime, *sql.DB, *application.Database, error) {
	rt, err := application.BindRuntime(app)
	if err != nil {
		return nil, nil, nil, e.InternalServerError("The partitioning runtime is not available.", err)
	}
	if e.Auth == nil {
		return nil, nil, nil, e.UnauthorizedError("", nil)
	}
	databaseID := e.Request.PathValue("id")
	if err := rt.AuthorizeSubjects(e.Request.Context(), databaseID, subjectCollections(e.Auth), e.Auth.Id, application.ScopeRecordRead); err != nil {
		return nil, nil, nil, liveError(e, err)
	}
	d, err := rt.Service.GetDatabase(e.Request.Context(), databaseID)
	if err != nil {
		return nil, nil, nil, liveError(e, err)
	}
	target, err := rt.Handles.Open(e.Request.Context(), databaseID)
	if err != nil {
		return nil, nil, nil, liveError(e, err)
	}
	return rt, target, d, nil
}

// liveQueryWire renders the LiveQuery wire shape (required: id,
// databaseId, queryId, schemaRevision, policyEpoch, cursor, streamUrl,
// expiresAt).
func liveQueryWire(reg *livequery.Registration, queryID string) map[string]any {
	return map[string]any{
		"id":             reg.ID,
		"databaseId":     reg.DatabaseID,
		"queryId":        queryID,
		"schemaRevision": reg.SchemaRevision,
		"policyEpoch":    reg.PolicyEpoch,
		"cursor":         reg.Cursor,
		"streamUrl":      fmt.Sprintf("/api/mirabase/v1/databases/%s/live-queries/%s/stream", reg.DatabaseID, queryID),
		"expiresAt":      reg.ExpiresAt,
	}
}

// liveError maps livequery/application errors onto the operation's error
// set: invalid_request, forbidden, not_found, schema_mismatch,
// policy_changed, resource_exhausted.
func liveError(e *core.RequestEvent, err error) error {
	switch {
	case errors.Is(err, application.ErrUnauthorizedDatabase), errors.Is(err, livequery.ErrForbidden),
		errors.Is(err, application.ErrLivePlanNotAuthorized):
		return e.ForbiddenError("You are not allowed to access this live query context.", nil)
	case errors.Is(err, application.ErrUnknownDatabase), errors.Is(err, application.ErrNotFound),
		errors.Is(err, livequery.ErrNotFound), errors.Is(err, livequery.ErrCancelled):
		return e.NotFoundError("The requested live query resource wasn't found.", nil)
	case errors.Is(err, livequery.ErrInvalidCursor), errors.Is(err, livequery.ErrInvalidRequest),
		errors.Is(err, livequery.ErrResultKey), errors.Is(err, livequery.ErrPlanUnpublished):
		return liveInvalidRequest(e, err.Error())
	case errors.Is(err, livequery.ErrPlanRetired), errors.Is(err, livequery.ErrShapeMismatch):
		return e.JSON(http.StatusConflict, map[string]any{
			"data":    map[string]any{"mirabase": map[string]any{"code": "policy_changed"}},
			"message": err.Error(),
			"status":  409,
		})
	case errors.Is(err, livequery.ErrExhausted):
		return e.JSON(http.StatusServiceUnavailable, map[string]any{
			"data":    map[string]any{"mirabase": map[string]any{"code": "resource_exhausted", "retryable": true}},
			"message": err.Error(),
			"status":  503,
		})
	}
	return e.InternalServerError("The live query operation failed.", err)
}

// writeLiveFrame writes one SSE frame.
func writeLiveFrame(e *core.RequestEvent, flusher http.Flusher, kind string, data map[string]any) {
	b, merr := json.Marshal(data)
	if merr != nil {
		return
	}
	fmt.Fprintf(e.Response, "event: %s\ndata: %s\n\n", kind, b)
	if flusher != nil {
		flusher.Flush()
	}
}

// writeLiveErrorFrame writes the stream's terminal error frame.
func writeLiveErrorFrame(e *core.RequestEvent, flusher http.Flusher, databaseID, queryID, code, message string) {
	writeLiveFrame(e, flusher, livequery.KindError, map[string]any{
		"databaseId": databaseID, "queryId": queryID,
		"code": code, "message": message,
	})
}

// writeLiveStepError maps a Step failure onto the terminal error frame;
// the stream then ends (the client re-registers or reconnects).
func writeLiveStepError(e *core.RequestEvent, flusher http.Flusher, databaseID, queryID string, err error) {
	code := "error"
	switch {
	case errors.Is(err, livequery.ErrCancelled):
		code = "subscription_cancelled"
	case errors.Is(err, livequery.ErrPlanRetired), errors.Is(err, livequery.ErrPlanUnpublished):
		code = "policy_changed"
	case errors.Is(err, livequery.ErrForbidden), errors.Is(err, application.ErrLivePlanNotAuthorized):
		code = "forbidden"
	case errors.Is(err, livequery.ErrExhausted):
		code = "resource_exhausted"
	case errors.Is(err, livequery.ErrResultKey):
		code = "invalid_request"
	}
	writeLiveErrorFrame(e, flusher, databaseID, queryID, code, err.Error())
}
