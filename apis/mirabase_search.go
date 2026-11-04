package apis

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"mirabase/core"
	"mirabase/internal/application"
	"mirabase/internal/search"
	"mirabase/tools/router"
)

// bindMirabaseSearchApi registers the search operation
// (POST /api/mirabase/v1/databases/{id}/collections/{collection}/search)
// and the FTS index maintenance action. Record-authenticated; the caller
// must own the database or hold a live grant ("record.read and collection
// list/view/field rules").
//
// Rules and field protection run BEFORE results leave the server
// (queries-and-search.md): the collection's list rule — from the target's
// published template, or the primary collection itself — compiles through
// the parsed filter machinery into a parameterized predicate applied
// inside every scanned query; hidden fields never leave the response; the
// request's field projection narrows every returned record.
func bindMirabaseSearchApi(app core.App, subGroup *router.RouterGroup[*core.RequestEvent]) {
	subGroup.POST("/databases/{databaseId}/collections/{collection}/search", func(e *core.RequestEvent) error {
		// The request info (the rules' @request context) binds BEFORE the
		// body is consumed (BindBody reads the same stream).
		requestInfo, ierr := e.RequestInfo()
		if ierr != nil {
			return firstApiError(ierr, e.BadRequestError("", ierr))
		}
		target, done := mirabaseSearchContext(e, app, application.ScopeRecordRead)
		if done != nil {
			return done
		}
		var req search.Request
		if err := json.NewDecoder(e.Request.Body).Decode(&req); err != nil {
			return searchError(e, fmt.Errorf("%w: failed to decode the search request", search.ErrInvalidRequest))
		}
		rt, _ := application.BindRuntime(app)
		databaseID := e.Request.PathValue("databaseId")
		collectionName := e.Request.PathValue("collection")
		var generation, schemaRevision, policyEpoch int64 = 1, 1, 1
		if d, err := rt.Service.GetDatabase(e.Request.Context(), databaseID); err == nil {
			generation = d.Generation
			schemaRevision = d.SchemaRevision
			policyEpoch = d.PolicyEpoch
		}

		// List/view rules and field protection resolve from the target's
		// published template (the primary database uses its own
		// collections).
		superuser := requestInfo.HasSuperuserAuth()
		listRule, hidden, rerr := mirabaseSearchCollectionRules(e, app, rt, databaseID, collectionName)
		if rerr != nil {
			return searchError(e, rerr)
		}
		opts := search.DefaultOptions()
		// The search surface's authority is the database ownership/grant
		// (record.read, verified above). The collection's list rule, when
		// PRESENT, is the actor's narrowing rule and injects as a
		// parameterized predicate inside every scanned query (a rule-less
		// collection does not gate the surface: an approved private
		// database serves its published collections to its authorized
		// actor).
		if listRule != nil && *listRule != "" && !superuser {
			coll, cerr := app.FindCollectionByNameOrId(collectionName)
			if cerr != nil {
				return searchError(e, fmt.Errorf("%w: the collection rules of %q are not resolvable", search.ErrInvalidRequest, collectionName))
			}
			predicate, predArgs, perr := application.BuildRulePredicate(application.BaseAppOf(app), coll, *listRule, requestInfo)
			if perr != nil {
				return searchError(e, perr)
			}
			opts.PolicyWhere = predicate
			opts.PolicyArgs = predArgs
		}

		// Field protection BEFORE the search runs: a protected (hidden)
		// field is not searchable — naming it in any clause is a typed
		// invalid_request, because ranking/ordering/highlights computed
		// over protected content would otherwise leak it.
		if err := rejectProtectedSearchFields(&req, hidden); err != nil {
			return searchError(e, err)
		}

		res, err := search.Search(e.Request.Context(), target, collectionName,
			&req, opts, databaseID, generation, schemaRevision, policyEpoch)
		if err != nil {
			return searchError(e, err)
		}
		applySearchFieldPolicy(res, req.Fields, hidden)
		return e.JSON(http.StatusOK, res)
	}) // The FTS index maintenance action ("Index creation/rebuild is a
	// durable job with old index serving or explicit unavailable status"):
	// verifies an existing same-name index actually covers the requested
	// fields (rebuilding it otherwise) and reports the VERIFIED state —
	// "ready" only when the stored index was checked to cover the
	// requested fields, "unavailable" with the error after a failed
	// rebuild (text search then fails with schema_mismatch until the
	// maintenance is redone).
	subGroup.POST("/databases/{databaseId}/collections/{collection}/search-index", func(e *core.RequestEvent) error {
		target, done := mirabaseSearchContext(e, app, application.ScopeRecordWrite)
		if done != nil {
			return done
		}
		var req struct {
			IndexName string             `json:"indexName"`
			Fields    []string           `json:"fields"`
			Tokenizer string             `json:"tokenizer"`
			Weights   map[string]float64 `json:"weights"`
		}
		if err := json.NewDecoder(e.Request.Body).Decode(&req); err != nil {
			return searchError(e, fmt.Errorf("%w: failed to decode the index request", search.ErrInvalidRequest))
		}
		if req.IndexName == "" {
			req.IndexName = "idx_" + e.Request.PathValue("collection") + "_fts"
		}
		state, err := search.EnsureFTSIndex(e.Request.Context(), target, e.Request.PathValue("collection"),
			req.IndexName, req.Fields, req.Tokenizer, req.Weights)
		if err != nil {
			return searchError(e, err)
		}
		return e.JSON(http.StatusOK, state)
	})
}

// mirabaseSearchCollectionRules resolves the collection's list rule and
// hidden-field set for the searched database context: a managed target
// resolves them from its published template (the template's rules are
// authoritative for the target); the primary database uses its own
// collection definitions.
func mirabaseSearchCollectionRules(e *core.RequestEvent, app core.App, rt *application.Runtime, databaseID, collectionName string) (*string, map[string]bool, error) {
	if d, derr := rt.Service.GetDatabase(e.Request.Context(), databaseID); derr == nil {
		if d.TemplateID != nil && *d.TemplateID != "" {
			tpl, terr := rt.Service.LoadLatestPublishedTemplate(e.Request.Context(), *d.TemplateID)
			if terr == nil {
				for _, def := range tpl.Definitions {
					if def.CollectionName != collectionName {
						continue
					}
					hidden := map[string]bool{}
					for _, f := range def.Fields {
						if f.Hidden {
							hidden[f.Name] = true
						}
					}
					return def.ListRule, hidden, nil
				}
				return nil, nil, fmt.Errorf("%w: the collection %q is not published in this database", search.ErrInvalidRequest, collectionName)
			}
			// Template resolution failure falls through to the primary
			// collection (the rules then mirror the application's live
			// definition).
		}
	}
	coll, cerr := app.FindCollectionByNameOrId(collectionName)
	if cerr != nil {
		return nil, nil, fmt.Errorf("%w: the collection %q is not published in this database", search.ErrInvalidRequest, collectionName)
	}
	hidden := map[string]bool{}
	for _, f := range coll.Fields {
		if f.GetHidden() {
			hidden[f.GetName()] = true
		}
	}
	return coll.ListRule, hidden, nil
}

// rejectProtectedSearchFields enforces field protection BEFORE the search
// runs: a protected (hidden) field is not searchable. Ranking, ordering
// and highlights computed over protected content would leak it, so any
// clause naming a hidden field is a typed invalid_request.
func rejectProtectedSearchFields(req *search.Request, hidden map[string]bool) error {
	if len(hidden) == 0 {
		return nil
	}
	reject := func(kind, field string) error {
		return fmt.Errorf("%w: the %s field %q is protected and cannot be searched", search.ErrInvalidRequest, kind, field)
	}
	if req.Vector != nil && hidden[strings.ToLower(req.Vector.Field)] {
		return reject("vector", req.Vector.Field)
	}
	if req.Text != nil {
		for _, f := range req.Text.Fields {
			if hidden[strings.ToLower(f)] {
				return reject("text", f)
			}
		}
	}
	if req.Fuzzy != nil && hidden[strings.ToLower(req.Fuzzy.Field)] {
		return reject("fuzzy", req.Fuzzy.Field)
	}
	if req.Field != "" && hidden[strings.ToLower(req.Field)] {
		return reject("fuzzy", req.Field)
	}
	return nil
}

// applySearchFieldPolicy narrows every returned record to the request's
// field projection and strips the protected (hidden) fields — including
// their authorized highlights — before the response leaves the server.
// A highlight of a field that is not part of the projection (when one is
// requested) leaves with the field's value, so it is stripped too.
func applySearchFieldPolicy(res *search.Result, fields []string, hidden map[string]bool) {
	if res == nil {
		return
	}
	want := map[string]bool{}
	for _, f := range fields {
		want[f] = true
	}
	project := len(fields) > 0
	for i := range res.Items {
		rec := res.Items[i].Record
		if len(rec) == 0 {
			continue
		}
		out := map[string]any{}
		for k, v := range rec {
			if hidden[strings.ToLower(k)] {
				continue
			}
			if project && k != "id" && !want[k] {
				continue
			}
			out[k] = v
		}
		res.Items[i].Record = out
		// Highlights carry field content too: drop the protected fields'
		// fragments, and (under projection) the unprojected fields'.
		hls := res.Items[i].Highlights[:0:0]
		for _, h := range res.Items[i].Highlights {
			lf := strings.ToLower(h.Field)
			if hidden[lf] {
				continue
			}
			if project && !want[h.Field] {
				continue
			}
			hls = append(hls, h)
		}
		res.Items[i].Highlights = hls
	}
}

// mirabaseSearchContext resolves the runtime, authorizes the database
// context and opens the target handle ("Resolve the database and current
// authority before rules/forms/hooks/files/SSE/search").
func mirabaseSearchContext(e *core.RequestEvent, app core.App, scope string) (*sql.DB, error) {
	rt, err := application.BindRuntime(app)
	if err != nil {
		return nil, e.InternalServerError("The partitioning runtime is not available.", err)
	}
	databaseID := e.Request.PathValue("databaseId")
	subjectRecord := ""
	if e.Auth != nil {
		subjectRecord = e.Auth.Id
	}
	if err := rt.AuthorizeSubjects(e.Request.Context(), databaseID, subjectCollections(e.Auth), subjectRecord, scope); err != nil {
		return nil, searchError(e, err)
	}
	target, err := rt.Handles.Open(e.Request.Context(), databaseID)
	if err != nil {
		return nil, searchError(e, err)
	}
	return target, nil
}

// searchError maps search errors onto the additive envelope: the
// release-capability error for hybrid/fuzzy, cursor_expired, budget
// exhaustion as retryable resource_exhausted, schema errors for missing
// indexes.
func searchError(e *core.RequestEvent, err error) error {
	msg := err.Error()
	switch {
	case errors.Is(err, search.ErrCapabilityUnavailable):
		return e.JSON(http.StatusBadRequest, map[string]any{
			"data":    map[string]any{"mirabase": map[string]any{"code": "capability_unavailable"}},
			"message": msg,
			"status":  400,
		})
	case errors.Is(err, search.ErrCursorExpired):
		return e.JSON(http.StatusGone, map[string]any{
			"data":    map[string]any{"mirabase": map[string]any{"code": "cursor_expired"}},
			"message": msg,
			"status":  410,
		})
	case errors.Is(err, search.ErrBudgetExhausted):
		return e.JSON(http.StatusServiceUnavailable, map[string]any{
			"data":    map[string]any{"mirabase": map[string]any{"code": "resource_exhausted", "retryable": true}},
			"message": msg,
			"status":  503,
		})
	case errors.Is(err, search.ErrNotIndexed):
		return e.JSON(http.StatusConflict, map[string]any{
			"data":    map[string]any{"mirabase": map[string]any{"code": "schema_mismatch"}},
			"message": msg,
			"status":  409,
		})
	case errors.Is(err, application.ErrUnauthorizedDatabase):
		return e.ForbiddenError("You are not allowed to search this database context.", nil)
	case errors.Is(err, application.ErrUnknownDatabase):
		return e.NotFoundError("Unknown database context.", nil)
	case errors.Is(err, search.ErrInvalidRequest):
		return e.JSON(http.StatusBadRequest, map[string]any{
			"data":    map[string]any{"mirabase": map[string]any{"code": "invalid_request"}},
			"message": msg,
			"status":  400,
		})
	}
	if strings.Contains(msg, "no such") || strings.Contains(msg, "Parse error") {
		return e.JSON(http.StatusBadRequest, map[string]any{
			"data":    map[string]any{"mirabase": map[string]any{"code": "invalid_request"}},
			"message": msg,
			"status":  400,
		})
	}
	return e.InternalServerError("The search operation failed.", err)
}
