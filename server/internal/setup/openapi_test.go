package setup

import (
	"encoding/json"
	"strings"
	"testing"
)

// TestOpenAPIDocumentShape verifies the generated OpenAPI document: the
// released operations only, the additive error envelope with the stable
// codes, and roadmap selectors absent.
func TestOpenAPIDocumentShape(t *testing.T) {
	raw, err := OpenAPIDocument("b0e1b8fde8eff66ef9843fdd1139fa3305cb16dd")
	if err != nil {
		t.Fatalf("generate: %v", err)
	}
	var doc map[string]any
	if err := json.Unmarshal(raw, &doc); err != nil {
		t.Fatalf("parse: %v", err)
	}
	if doc["openapi"] != "3.1.0" {
		t.Fatalf("openapi version: %v", doc["openapi"])
	}
	paths := doc["paths"].(map[string]any)
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
	// Roadmap selectors must NOT appear as registered paths. The
	// applications/changes surfaces are now released (waves 5/7); the
	// remaining roadmap selectors stay unregistered.
	for _, p := range []string{"/tenants", "/membership-invitations-tpl", "/changes-raw", "/sync", "/live"} {
		if _, ok := paths[p]; ok {
			t.Errorf("roadmap path %s is registered in the document", p)
		}
	}
	// The error envelope's stable codes.
	components := doc["components"].(map[string]any)
	schemas := components["schemas"].(map[string]any)
	errSchema := schemas["MirabaseError"].(map[string]any)
	rawErr, _ := json.Marshal(errSchema)
	for _, code := range []string{
		"capability_unavailable", "cursor_expired", "generation_mismatch",
		"layout_migration_required", "report_stale", "outcome_unknown",
		"resource_exhausted",
	} {
		if !strings.Contains(string(rawErr), code) {
			t.Errorf("error schema missing the code %s", code)
		}
	}
	// The SearchResult schema carries the required wire fields.
	sr, _ := schemas["SearchResult"].(map[string]any)
	if sr == nil {
		t.Fatalf("missing the SearchResult schema")
	}
	rawSR, _ := json.Marshal(sr)
	for _, f := range []string{"databaseId", "generation", "items", "complete", "scannedCount", "nextCursor"} {
		if !strings.Contains(string(rawSR), f) {
			t.Errorf("SearchResult missing %s", f)
		}
	}
	// The info block carries the engine revision separately from the product.
	info := doc["info"].(map[string]any)
	if !strings.Contains(info["description"].(string), "b0e1b8fde8eff66ef9843fdd1139fa3305cb16dd") {
		t.Fatalf("info description lacks the engine revision")
	}
	if info["title"] != "MiraBase" {
		t.Fatalf("info title: %v", info["title"])
	}
}
