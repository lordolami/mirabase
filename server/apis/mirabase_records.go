package apis

import (
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"mirabase/core"
	"mirabase/internal/application"
	"mirabase/tools/hook"
	"mirabase/tools/osutils"
	"mirabase/tools/router"

	validation "ganigeorgiev.com/pocketbase/ozzo-validation/v4"
)

// bindMirabaseApi registers the /api/mirabase/v1 partitioning surface:
// context-bound record CRUD against managed target databases. Access
// rules: the authenticated record must own the database or hold a live
// grant; ordinary record rules still filter per the template's published
// rule configuration (partitioning.md: "A scope permits an access path;
// ordinary record rules still filter each operation").
func bindMirabaseApi(app core.App, apiGroup *router.RouterGroup[*core.RequestEvent]) {
	subGroup := apiGroup.Group("/mirabase/v1")
	bindMirabaseRecordsApi(app, subGroup)
	bindMirabaseReferenceReadApi(app, subGroup)
	bindMirabaseSetupApi(app, subGroup)
	bindMirabaseReferenceWorker(app)
	bindMirabaseAdminApi(app, subGroup)
	bindMirabaseSearchApi(app, subGroup)
	bindMirabaseLiveApi(app, subGroup)
	bindMirabaseImportApi(app, subGroup)
	bindMirabaseCapabilitiesApi(app, subGroup)
	bindMirabaseOpenApiApi(app, subGroup)
	bindMirabasePGGateway(app)
}

func bindMirabaseRecordsApi(app core.App, subGroup *router.RouterGroup[*core.RequestEvent]) {
	subGroup.Bind(RequireAuth())

	subGroup.GET("/db/{databaseId}/collections/{collection}/records", func(e *core.RequestEvent) error {
		databaseID, store, done := mirabaseRecordContext(e, app, application.ScopeRecordRead)
		if done != nil {
			return done
		}
		limit, _ := strconv.Atoi(e.Request.URL.Query().Get("limit"))
		rows, err := store.List(e.Request.Context(), limit)
		if err != nil {
			return mirabaseRecordError(e, err)
		}
		return e.JSON(http.StatusOK, map[string]any{
			"database": databaseID,
			"items":    rows,
			"page":     1,
			"perPage":  limit,
		})
	})

	subGroup.GET("/db/{databaseId}/collections/{collection}/records/{recordId}", func(e *core.RequestEvent) error {
		_, store, done := mirabaseRecordContext(e, app, application.ScopeRecordRead)
		if done != nil {
			return done
		}
		row, err := store.Get(e.Request.Context(), e.Request.PathValue("recordId"))
		if err != nil {
			return mirabaseRecordError(e, err)
		}
		return e.JSON(http.StatusOK, row)
	})

	subGroup.POST("/db/{databaseId}/collections/{collection}/records", func(e *core.RequestEvent) error {
		_, store, done := mirabaseRecordContext(e, app, application.ScopeRecordWrite)
		if done != nil {
			return done
		}
		var data map[string]any
		if err := json.NewDecoder(e.Request.Body).Decode(&data); err != nil {
			return e.BadRequestError("An error occurred while loading the submitted data.", err)
		}
		row, err := store.CreateWithCapture(e.Request.Context(), data, mirabaseTableCapture(e))
		if err != nil {
			return mirabaseRecordError(e, err)
		}
		return e.JSON(http.StatusOK, row)
	})

	subGroup.PATCH("/db/{databaseId}/collections/{collection}/records/{recordId}", func(e *core.RequestEvent) error {
		_, store, done := mirabaseRecordContext(e, app, application.ScopeRecordWrite)
		if done != nil {
			return done
		}
		var data map[string]any
		if err := json.NewDecoder(e.Request.Body).Decode(&data); err != nil {
			return e.BadRequestError("An error occurred while loading the submitted data.", err)
		}
		row, err := store.UpdateWithCapture(e.Request.Context(), e.Request.PathValue("recordId"), data, mirabaseTableCapture(e))
		if err != nil {
			return mirabaseRecordError(e, err)
		}
		return e.JSON(http.StatusOK, row)
	})

	subGroup.DELETE("/db/{databaseId}/collections/{collection}/records/{recordId}", func(e *core.RequestEvent) error {
		_, store, done := mirabaseRecordContext(e, app, application.ScopeRecordWrite)
		if done != nil {
			return done
		}
		if err := store.DeleteWithCapture(e.Request.Context(), e.Request.PathValue("recordId"), mirabaseTableCapture(e)); err != nil {
			return mirabaseRecordError(e, err)
		}
		return e.NoContent(http.StatusNoContent)
	})
}

// mirabaseRecordContext resolves the runtime, verifies authorization for
// the selected database (owner or live grant carrying the scope) and
// returns the record store over the target handle. Invalid or
// unauthorized selection never falls back to primary.
func mirabaseRecordContext(e *core.RequestEvent, app core.App, scope string) (string, *application.RecordStore, error) {
	rt, err := application.BindRuntime(app)
	if err != nil {
		return "", nil, e.InternalServerError("The partitioning runtime is not available for this application.", err)
	}
	if err != nil {
		return "", nil, e.InternalServerError("Failed to initialize the partitioning runtime.", err)
	}

	databaseID := e.Request.PathValue("databaseId")
	subjectRecord := ""
	if e.Auth != nil {
		subjectRecord = e.Auth.Id
	}

	if err := rt.AuthorizeSubjects(e.Request.Context(), databaseID, subjectCollections(e.Auth), subjectRecord, scope); err != nil {
		return "", nil, mirabaseRecordError(e, err)
	}

	target, err := rt.Handles.Open(e.Request.Context(), databaseID)
	if err != nil {
		return "", nil, mirabaseRecordError(e, err)
	}

	store := application.NewRecordStore(target, e.Request.PathValue("collection"))
	return databaseID, store, nil
}

// mirabaseTableCapture binds the CDC capture to the resolved collection.
func mirabaseTableCapture(e *core.RequestEvent) *application.TableCapture {
	return &application.TableCapture{CollectionID: e.Request.PathValue("collection")}
}

// bindMirabaseReferenceReadApi registers the reference-copy read surface
// (implementation-closure.md: "Reads that require a healthy copy return
// reference_stale when its declared lag is exceeded; diagnostics show the
// source/target sequence"): the authenticated record must hold record.read
// on the TARGET database; a quarantined, never-applied or stale copy
// surfaces reference_stale with the diagnostics, never the data.
func bindMirabaseReferenceReadApi(app core.App, subGroup *router.RouterGroup[*core.RequestEvent]) {
	read := func(e *core.RequestEvent, recordID string) error {
		databaseID := e.Request.PathValue("databaseId")
		policyID := e.Request.PathValue("policyId")
		rt, err := application.BindRuntime(app)
		if err != nil {
			return e.InternalServerError("The partitioning runtime is not available.", err)
		}
		if err := rt.AuthorizeSubjects(e.Request.Context(), databaseID, subjectCollections(e.Auth), e.Auth.Id, application.ScopeRecordRead); err != nil {
			return mirabaseRecordError(e, err)
		}
		policy, err := rt.Service.GetReferencePolicy(e.Request.Context(), policyID)
		if err != nil {
			return mirabaseRecordError(e, err)
		}
		out, err := rt.ReadReferenceCopy(e.Request.Context(), policy, databaseID, recordID)
		if err != nil {
			if errors.Is(err, application.ErrReferenceCopyStale) {
				return e.JSON(http.StatusServiceUnavailable, map[string]any{
					"data":        map[string]any{"mirabase": map[string]any{"code": "reference_stale", "retryable": true}},
					"message":     err.Error(),
					"Retry-After": 1,
					"status":      503,
				})
			}
			return mirabaseRecordError(e, err)
		}
		return e.JSON(http.StatusOK, out)
	}
	subGroup.GET("/databases/{databaseId}/reference-copies/{policyId}/records", func(e *core.RequestEvent) error {
		return read(e, "")
	})
	subGroup.GET("/databases/{databaseId}/reference-copies/{policyId}/records/{recordId}", func(e *core.RequestEvent) error {
		return read(e, e.Request.PathValue("recordId"))
	})
}

func mirabaseRecordError(e *core.RequestEvent, err error) error {
	switch {
	case errors.Is(err, application.ErrUnauthorizedDatabase):
		return e.ForbiddenError("You are not allowed to access this database context.", nil)
	case errors.Is(err, application.ErrUnknownDatabase):
		return e.NotFoundError("Unknown database context.", nil)
	case errors.Is(err, application.ErrCacheFull):
		return e.JSON(http.StatusServiceUnavailable, map[string]any{
			"data":        map[string]any{},
			"message":     "resource_exhausted",
			"retryable":   true,
			"Retry-After": 1,
			"status":      503,
		})
	case errors.Is(err, application.ErrNotFound):
		return e.NotFoundError("The requested resource wasn't found.", nil)
	case errors.Is(err, application.ErrUnknownCollection):
		return e.NotFoundError("Unknown collection in this database context.", nil)
	case strings.Contains(err.Error(), "unknown field"):
		return e.BadRequestError("Failed to create record: "+err.Error(), nil)
	case strings.Contains(err.Error(), "constraint failed"), strings.Contains(err.Error(), "UNIQUE"):
		return e.BadRequestError("Failed to save record: "+err.Error(), nil)
	case strings.Contains(err.Error(), "not grantable"):
		return e.BadRequestError(err.Error(), nil)
	}
	var valErr validation.Errors
	if errors.As(err, &valErr) {
		return e.BadRequestError("Failed to save record.", valErr)
	}
	return e.BadRequestError("Failed to perform the record operation: "+err.Error(), nil)
}

// subjectCollections returns the subject's collection references (name
// and id) so registry rows stored with either resolve.
func subjectCollections(auth *core.Record) []string {
	if auth == nil {
		return nil
	}
	return []string{auth.Collection().Name, auth.Collection().Id}
}

// bindMirabasePGGateway starts the PostgreSQL wire gateway listener
// (mirabase_pg.go) under the existing serve lifecycle: an OnServe hook
// that brings the listener up as the last serve step and closes it with
// the app's bootstrap-clear, mirroring the __pbCronStart__/stop hook pair
// (core/base.go). The gateway is DISABLED BY DEFAULT (postgresql-access.md;
// configuration-reference.md "postgres".enabled is false): it is started
// only when the operator enables the feature through the environment
// (osutils.GetEnv checks MIRABASE_<name>, then the inherited PB_<name>):
//
//   - POSTGRES_ENABLED: truthy ("1"/"true"/"yes"/"on") enables the listener
//   - POSTGRES_LISTEN: the bind address (default 127.0.0.1:5433)
//   - POSTGRES_TLS_CERT / POSTGRES_TLS_KEY: the tlsCertRef/tlsKeyRef
//     certificate pair — the gateway is TLS-only and refuses to start
//     without a loadable pair, so an explicitly enabled gateway with
//     missing/broken TLS material fails the server startup
//
// Default startup therefore stays simple (no additional listener), and an
// enabled gateway is a real listener with real failure behavior, not a
// declaration.
func bindMirabasePGGateway(app core.App) {
	app.OnServe().Bind(&hook.Handler[*core.ServeEvent]{
		Id:       "__pbPGGatewayStart__",
		Priority: 999, // execute as latest as possible
		Func: func(se *core.ServeEvent) error {
			if !truthyEnv(osutils.GetEnv("POSTGRES_ENABLED")) {
				return se.Next()
			}
			certFile, keyFile := osutils.GetEnv("POSTGRES_TLS_CERT"), osutils.GetEnv("POSTGRES_TLS_KEY")
			if certFile == "" || keyFile == "" {
				return fmt.Errorf("the PostgreSQL gateway is enabled but its TLS material is not configured: set MIRABASE_POSTGRES_TLS_CERT and MIRABASE_POSTGRES_TLS_KEY (the gateway is TLS-only and never serves plaintext)")
			}
			tlsCert, err := tls.LoadX509KeyPair(certFile, keyFile)
			if err != nil {
				return fmt.Errorf("failed to load the PostgreSQL gateway TLS certificate: %w", err)
			}
			listener, err := StartPGGateway(app, osutils.GetEnv("POSTGRES_LISTEN"), &tls.Config{
				Certificates: []tls.Certificate{tlsCert},
				MinVersion:   tls.VersionTLS12,
			}, app.Logger())
			if err != nil {
				return err
			}
			// stop with the app's bootstrap-clear: OnTerminate triggers
			// ClearBootstrap on both serve exit and CLI exit, so the
			// listener (and its live sessions) close on graceful shutdown.
			app.OnBootstrapClear().Bind(&hook.Handler[*core.BootstrapEvent]{
				Id:       "__pbPGGatewayStop__",
				Priority: 999,
				Func: func(be *core.BootstrapEvent) error {
					if err := listener.Close(); err != nil {
						be.App.Logger().Warn("Failed to close the PostgreSQL gateway listener", "error", err)
					}
					return be.Next()
				},
			})
			app.Logger().Info("PostgreSQL wire gateway started", "addr", listener.Addr().String())
			return se.Next()
		},
	})
}

// truthyEnv reports whether an operator-supplied switch selects "on".
func truthyEnv(v string) bool {
	switch strings.ToLower(v) {
	case "1", "true", "yes", "on":
		return true
	}
	return false
}
