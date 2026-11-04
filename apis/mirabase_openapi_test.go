package apis_test

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"

	"mirabase/tests"
)

// TestOpenAPIDocument verifies the generated OpenAPI document reflects the
// actual registered v1 surface: every released operation is present, the
// error envelope schema is complete, and the roadmap selectors are absent.
func TestOpenAPIDocument(t *testing.T) {
	if mbenginePinnedDigestForIsolation() == "" {
		t.Skip("requires the pinned engine payload")
	}
	app, err := tests.NewTestApp()
	if err != nil {
		t.Fatalf("NewTestApp: %v", err)
	}
	defer app.Cleanup()
	handler, err := apisRouterFor(app)
	if err != nil {
		t.Fatalf("router: %v", err)
	}
	ts := newHTTPServer(handler)
	defer ts.Close()

	req, err := http.NewRequest("GET", ts.URL+"/api/mirabase/v1/openapi.json", nil)
	if err != nil {
		t.Fatal(err)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(resp.Body)
	if err != nil || resp.StatusCode != 200 {
		t.Fatalf("openapi: %d %v", resp.StatusCode, err)
	}
	var doc map[string]any
	if err := json.Unmarshal(raw, &doc); err != nil {
		t.Fatalf("parse: %v", err)
	}
	if doc["openapi"] != "3.1.0" {
		t.Fatalf("openapi version: %v", doc["openapi"])
	}
	paths, _ := doc["paths"].(map[string]any)
	required := []string{
		"/capabilities", "/setup", "/setup/plan", "/setup/plan/apply",
		"/templates", "/templates/{templateId}/rollouts", "/rollouts/{rolloutId}",
		"/db/{databaseId}/collections/{collection}/records",
		"/databases/{databaseId}/collections/{collection}/search",
		"/databases/{databaseId}/collections/{collection}/search-index",
		"/pocketbase-imports", "/pocketbase-imports/{importId}/source",
		"/pocketbase-imports/{importId}/analyze", "/pocketbase-imports/{importId}/plan",
		"/pocketbase-imports/{importId}/preview", "/pocketbase-imports/{importId}/stage",
		"/pocketbase-imports/{importId}/validate", "/pocketbase-imports/{importId}/cutover",
		"/pocketbase-imports/{importId}/cancel",
	}
	for _, p := range required {
		if _, ok := paths[p]; !ok {
			t.Errorf("missing path %s", p)
		}
	}
	// The error envelope's stable codes.
	components, _ := doc["components"].(map[string]any)
	schemas, _ := components["schemas"].(map[string]any)
	errSchema, _ := schemas["MirabaseError"].(map[string]any)
	if errSchema == nil {
		t.Fatalf("missing the MirabaseError schema")
	}
	rawErr, _ := json.Marshal(errSchema)
	for _, code := range []string{"capability_unavailable", "cursor_expired", "generation_mismatch", "layout_migration_required", "report_stale"} {
		if !strings.Contains(string(rawErr), code) {
			t.Errorf("error schema missing the code %s", code)
		}
	}
	// Roadmap operations must NOT appear as registered paths.
	for _, p := range []string{"/tenants", "/membership-unsupported-tpl"} {
		if _, ok := paths[p]; ok {
			t.Errorf("roadmap path %s is registered in the document", p)
		}
	}
	// The info block carries the engine revision separately.
	info, _ := doc["info"].(map[string]any)
	if !strings.Contains(info["description"].(string), "b0e1b8fde8eff66ef9843fdd1139fa3305cb16dd") {
		t.Fatalf("info description lacks the engine revision")
	}
}
