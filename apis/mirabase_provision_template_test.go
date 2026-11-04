package apis_test

import (
	"encoding/json"
	"fmt"
	"testing"

	"mirabase/core"
	"mirabase/tests"
)

// Provisioning must publish usable tables before the HTTP job says succeeded.
// No test-side ApplyTemplate call is allowed in this regression.
func TestDatabaseCreateAppliesPublishedTemplateBeforeReady(t *testing.T) {
	app, err := tests.NewTestApp()
	if err != nil {
		t.Fatal(err)
	}
	defer app.Cleanup()
	mux, err := apisRouterFor(app)
	if err != nil {
		t.Fatal(err)
	}
	ts := newHTTPServer(mux)
	defer ts.Close()
	token := createSuperuserAndGetToken(t, ts.URL, app)
	coll := core.NewBaseCollection("provision_documents")
	coll.Fields.Add(&core.TextField{Name: "body", Required: true})
	if err := app.Save(coll); err != nil {
		t.Fatal(err)
	}
	status, body := v1JSON(t, ts.URL, token, "/api/mirabase/v1/templates", "POST", map[string]any{
		"templateId": "provision-template", "collectionNames": []string{coll.Name}, "expectedSchemaRevision": "1",
	})
	if status != 200 {
		t.Fatalf("publish: %d %s", status, body)
	}
	status, body = v1JSON(t, ts.URL, token, "/api/mirabase/v1/databases", "POST", map[string]any{
		"kind": "manual", "profile": "wal", "displayName": "provisioned", "templateId": "provision-template",
	})
	var job struct{ State, ResultRef string }
	if status != 202 || json.Unmarshal([]byte(body), &job) != nil || job.State != "succeeded" {
		t.Fatalf("create: %d %s", status, body)
	}
	var result struct {
		DatabaseID string `json:"databaseId"`
	}
	if err := json.Unmarshal([]byte(job.ResultRef), &result); err != nil || result.DatabaseID == "" {
		t.Fatalf("result: %s", job.ResultRef)
	}
	path := fmt.Sprintf("/api/mirabase/v1/db/%s/collections/%s/records", result.DatabaseID, coll.Name)
	status, body = v1JSON(t, ts.URL, token, path, "POST", map[string]any{"body": "usable immediately"})
	if status != 200 {
		t.Fatalf("first record: %d %s", status, body)
	}
	var row map[string]any
	if err := json.Unmarshal([]byte(body), &row); err != nil || row["body"] != "usable immediately" {
		t.Fatalf("persisted row: %s", body)
	}
	status, body = v1JSON(t, ts.URL, token, path+"/"+row["id"].(string), "GET", nil)
	if status != 200 {
		t.Fatalf("read after create: %d %s", status, body)
	}
}
