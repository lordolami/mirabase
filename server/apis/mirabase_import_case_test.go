package apis_test

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"mirabase/internal/application"
	"mirabase/tests"
)

// Table-name casing in sqlite_master is not a count of its stored records.
// A real mixed-case auth table must validate, while a missing row must fail.
func TestPocketBaseImportMixedCaseValidation(t *testing.T) {
	for _, damage := range []bool{false, true} {
		t.Run(fmt.Sprintf("missing_row_%t", damage), func(t *testing.T) {
			app, err := tests.NewTestApp()
			if err != nil {
				t.Fatal(err)
			}
			defer app.Cleanup()
			rt, err := application.BindRuntime(app)
			if err != nil {
				t.Fatal(err)
			}
			mux, err := apisRouterFor(app)
			if err != nil {
				t.Fatal(err)
			}
			ts := newHTTPServer(mux)
			defer ts.Close()
			token := createSuperuserAndGetToken(t, ts.URL, app)
			source := filepath.Join(t.TempDir(), "data.db")
			if err := buildPocketBaseFixture(source); err != nil {
				t.Fatal(err)
			}
			reader, err := sql.Open("sqlite", source)
			if err != nil {
				t.Fatal(err)
			}
			if _, err = reader.Exec(`ALTER TABLE users RENAME TO UsersAuth`); err != nil {
				t.Fatal(err)
			}
			if _, err = reader.Exec(`UPDATE _collections SET name='UsersAuth' WHERE name='users'`); err != nil {
				t.Fatal(err)
			}
			reader.Close()
			request := func(path, method string, data any) map[string]any {
				t.Helper()
				status, body := v1JSON(t, ts.URL, token, path, method, data)
				if status != 200 && status != 202 {
					t.Fatalf("%s %s: %d %s", method, path, status, body)
				}
				var out map[string]any
				if err := json.Unmarshal([]byte(body), &out); err != nil {
					t.Fatal(err)
				}
				return out
			}
			session := request("/api/mirabase/v1/pocketbase-imports", "POST", map[string]any{"sourceKind": "pocketbase-directory", "sourceBytes": fileLen(source), "sourceSha256": fileSha(source), "targetMode": "new-application", "expectedPrimaryGeneration": "1"})
			id := session["id"].(string)
			path := "/api/mirabase/v1/pocketbase-imports/" + id
			bytes, err := os.ReadFile(source)
			if err != nil {
				t.Fatal(err)
			}
			if status, body := v1Upload(t, ts.URL, token, path+"/source", bytes); status != 200 {
				t.Fatalf("upload: %d %s", status, body)
			}
			analysis := request(path+"/analyze", "POST", nil)
			request(path+"/plan", "PUT", map[string]any{"expectedRevision": "1", "sourceHash": fileSha(source), "snapshotHash": analysis["snapshotHash"], "layout": "shared", "namespace": "shared", "placements": []map[string]any{{"collectionId": "UsersAuth", "placement": "shared", "ownership": map[string]any{"kind": "shared"}, "uniquePolicy": "preserve-global"}, {"collectionId": "posts", "placement": "shared", "ownership": map[string]any{"kind": "shared"}, "uniquePolicy": "preserve-global"}}, "accountPolicy": map[string]any{"sessionPolicy": "preserve-if-compatible", "operatorPolicy": "use-source-superuser"}, "selector": map[string]any{"kind": "none"}, "profile": "wal", "sideEffects": "suppressed-during-import"})
			request(path+"/preview", "POST", nil)
			request(path+"/stage", "POST", nil)
			if damage {
				staged, err := sql.Open("turso", filepath.Join(rt.DataDir, "imports", id, "staged", "data.db"))
				if err != nil {
					t.Fatal(err)
				}
				if _, err := staged.Exec(`DELETE FROM UsersAuth WHERE id=(SELECT id FROM UsersAuth LIMIT 1)`); err != nil {
					t.Fatal(err)
				}
				staged.Close()
			}
			result := request(path+"/validate", "POST", nil)
			passed, ok := result["passed"].(bool)
			if !ok || passed == damage {
				t.Fatalf("validation damage=%t: %v", damage, result)
			}
			if !damage && result["sourceRecords"] != float64(3) {
				t.Fatalf("record accounting: %v", result)
			}
		})
	}
}
