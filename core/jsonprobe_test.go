package core_test

import (
	"fmt"
	"testing"

	"mirabase/tests"
)

func TestJSONProbeDebug(t *testing.T) {
	app, err := tests.NewTestApp()
	if err != nil {
		t.Fatalf("NewTestApp: %v", err)
	}
	defer app.Cleanup()

	for _, col := range []string{"select_one", "select_many", "file_one", "file_many", "rel_one", "rel_many", "json"} {
		var bad int
		q := fmt.Sprintf("SELECT count(*) FROM demo1 WHERE json_valid([[%s]]) = 0 AND COALESCE([[%s]],'') != ''", col, col)
		if err := app.DB().NewQuery(q).Row(&bad); err != nil {
			t.Logf("%s: probe error: %v", col, err)
			continue
		}
		t.Logf("%s: non-json non-empty rows = %d", col, bad)
		if bad > 0 {
			var sample string
			if err := app.DB().NewQuery(fmt.Sprintf("SELECT [[%s]] FROM demo1 WHERE json_valid([[%s]]) = 0 AND COALESCE([[%s]],'') != '' LIMIT 1", col, col, col)).Row(&sample); err == nil {
				t.Logf("%s: sample=%q", col, sample)
			}
		}
	}
}
