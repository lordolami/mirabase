package apis

import (
	"net/http"

	"mirabase/core"
	"mirabase/internal/application"
	"mirabase/internal/setup"
	"mirabase/tools/router"
)

// bindMirabaseOpenApiApi registers GET /api/mirabase/v1/openapi.json —
// the combined OpenAPI 3.1 document emitted from the ACTUAL registered
// v1 surface (api-and-sdk.md: "Release tooling emits combined OpenAPI
// ... and capability manifest").
func bindMirabaseOpenApiApi(app core.App, subGroup *router.RouterGroup[*core.RequestEvent]) {
	// The document is public (operation catalog and shapes; no secrets);
	// the group-level record-auth middleware is excluded for this route.
	route := subGroup.GET("/openapi.json", func(e *core.RequestEvent) error {
		if _, err := application.BindRuntime(app); err != nil {
			return e.InternalServerError("The partitioning runtime is not available.", err)
		}
		doc, err := setup.OpenAPIDocument("b0e1b8fde8eff66ef9843fdd1139fa3305cb16dd")
		if err != nil {
			return e.InternalServerError("Failed to generate the OpenAPI document.", err)
		}
		e.Response.Header().Set("Content-Type", "application/json")
		e.Response.WriteHeader(http.StatusOK)
		_, _ = e.Response.Write(doc)
		return nil
	})
	// The document is public (operation catalog and shapes; no secrets):
	// exclude the group-level record-auth middleware.
	route.Unbind("pbRequireAuth")
}
