package mirabase

import (
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"mirabase/apis"
	"mirabase/core"

	"github.com/spf13/cobra"
)

func TestNew(t *testing.T) {
	// copy os.Args
	originalArgs := make([]string, len(os.Args))
	copy(originalArgs, os.Args)
	defer func() {
		// restore os.Args
		os.Args = originalArgs
	}()

	// change os.Args
	os.Args = os.Args[:1]
	os.Args = append(
		os.Args,
		"--dir=test_dir",
		"--encryptionEnv=test_encryption_env",
		"--debug=true",
	)

	app := New()

	if app == nil {
		t.Fatal("Expected initialized MiraBase instance, got nil")
	}

	if app.RootCmd == nil {
		t.Fatal("Expected RootCmd to be initialized, got nil")
	}

	if app.App == nil {
		t.Fatal("Expected App to be initialized, got nil")
	}

	if app.DataDir() != "test_dir" {
		t.Fatalf("Expected app.DataDir() %q, got %q", "test_dir", app.DataDir())
	}

	if app.EncryptionEnv() != "test_encryption_env" {
		t.Fatalf("Expected app.EncryptionEnv() test_encryption_env, got %q", app.EncryptionEnv())
	}
}

func TestNewWithConfig(t *testing.T) {
	app := NewWithConfig(Config{
		DefaultDataDir:       "test_dir",
		DefaultEncryptionEnv: "test_encryption_env",
		HideStartBanner:      true,
	})

	if app == nil {
		t.Fatal("Expected initialized MiraBase instance, got nil")
	}

	if app.RootCmd == nil {
		t.Fatal("Expected RootCmd to be initialized, got nil")
	}

	if app.App == nil {
		t.Fatal("Expected App to be initialized, got nil")
	}

	if app.hideStartBanner != true {
		t.Fatal("Expected app.hideStartBanner to be true, got false")
	}

	if app.DataDir() != "test_dir" {
		t.Fatalf("Expected app.DataDir() %q, got %q", "test_dir", app.DataDir())
	}

	if app.EncryptionEnv() != "test_encryption_env" {
		t.Fatalf("Expected app.EncryptionEnv() %q, got %q", "test_encryption_env", app.EncryptionEnv())
	}
}

func TestNewWithConfigAndFlags(t *testing.T) {
	// copy os.Args
	originalArgs := make([]string, len(os.Args))
	copy(originalArgs, os.Args)
	defer func() {
		// restore os.Args
		os.Args = originalArgs
	}()

	// change os.Args
	os.Args = os.Args[:1]
	os.Args = append(
		os.Args,
		"--dir=test_dir_flag",
		"--encryptionEnv=test_encryption_env_flag",
		"--debug=false",
	)

	app := NewWithConfig(Config{
		DefaultDataDir:       "test_dir",
		DefaultEncryptionEnv: "test_encryption_env",
		HideStartBanner:      true,
	})

	if app == nil {
		t.Fatal("Expected initialized MiraBase instance, got nil")
	}

	if app.RootCmd == nil {
		t.Fatal("Expected RootCmd to be initialized, got nil")
	}

	if app.App == nil {
		t.Fatal("Expected App to be initialized, got nil")
	}

	if app.hideStartBanner != true {
		t.Fatal("Expected app.hideStartBanner to be true, got false")
	}

	if app.DataDir() != "test_dir_flag" {
		t.Fatalf("Expected app.DataDir() %q, got %q", "test_dir_flag", app.DataDir())
	}

	if app.EncryptionEnv() != "test_encryption_env_flag" {
		t.Fatalf("Expected app.EncryptionEnv() %q, got %q", "test_encryption_env_flag", app.EncryptionEnv())
	}
}

func TestSkipBootstrap(t *testing.T) {
	// copy os.Args
	originalArgs := make([]string, len(os.Args))
	copy(originalArgs, os.Args)
	defer func() {
		// restore os.Args
		os.Args = originalArgs
	}()

	tempDir := filepath.Join(os.TempDir(), "temp_pb_data")
	defer os.RemoveAll(tempDir)

	// already bootstrapped
	app0 := NewWithConfig(Config{DefaultDataDir: tempDir, DBConnect: core.TestDBConnect()})
	app0.Bootstrap()
	if v := app0.skipBootstrap(); !v {
		t.Fatal("[bootstrapped] Expected true, got false")
	}

	// unknown command
	os.Args = os.Args[:1]
	os.Args = append(os.Args, "demo")
	app1 := NewWithConfig(Config{DefaultDataDir: tempDir, DBConnect: core.TestDBConnect()})
	app1.RootCmd.AddCommand(&cobra.Command{Use: "test"})
	if v := app1.skipBootstrap(); !v {
		t.Fatal("[unknown] Expected true, got false")
	}

	// default flags
	flagScenarios := []struct {
		name  string
		short string
	}{
		{"help", "h"},
		{"version", "v"},
	}

	for _, s := range flagScenarios {
		// base flag
		os.Args = os.Args[:1]
		os.Args = append(os.Args, "--"+s.name)
		app1 := NewWithConfig(Config{DefaultDataDir: tempDir, DBConnect: core.TestDBConnect()})
		if v := app1.skipBootstrap(); !v {
			t.Fatalf("[--%s] Expected true, got false", s.name)
		}

		// short flag
		os.Args = os.Args[:1]
		os.Args = append(os.Args, "-"+s.short)
		app2 := NewWithConfig(Config{DefaultDataDir: tempDir, DBConnect: core.TestDBConnect()})
		if v := app2.skipBootstrap(); !v {
			t.Fatalf("[-%s] Expected true, got false", s.short)
		}

		customCmd := &cobra.Command{Use: "custom"}
		customCmd.PersistentFlags().BoolP(s.name, s.short, false, "")

		// base flag in custom command
		os.Args = os.Args[:1]
		os.Args = append(os.Args, "custom")
		os.Args = append(os.Args, "--"+s.name)
		app3 := NewWithConfig(Config{DefaultDataDir: tempDir, DBConnect: core.TestDBConnect()})
		app3.RootCmd.AddCommand(customCmd)
		if v := app3.skipBootstrap(); v {
			t.Fatalf("[--%s custom] Expected false, got true", s.name)
		}

		// short flag in custom command
		os.Args = os.Args[:1]
		os.Args = append(os.Args, "custom")
		os.Args = append(os.Args, "-"+s.short)
		app4 := NewWithConfig(Config{DefaultDataDir: tempDir, DBConnect: core.TestDBConnect()})
		app4.RootCmd.AddCommand(customCmd)
		if v := app4.skipBootstrap(); v {
			t.Fatalf("[-%s custom] Expected false, got true", s.short)
		}
	}
}

// TestLauncherV1Endpoints is the launcher-shaped e2e regression for the
// new /api/mirabase/v1 surface: it reproduces the EXACT app shape the
// serve command uses (the MiraBase launcher wrapping core.App as an
// interface — the shape that broke the partitioning runtime in real use
// and was invisible to the test-harness-shaped acceptance tests), builds
// the real router through it, and drives the v1 endpoints over HTTP.
func TestLauncherV1Endpoints(t *testing.T) {
	tempDir := filepath.Join(os.TempDir(), "mb_launcher_v1_e2e")
	defer os.RemoveAll(tempDir)

	app := NewWithConfig(Config{
		DefaultDataDir: tempDir,
		DBConnect:      core.TestDBConnect(),
	})
	if err := app.Bootstrap(); err != nil {
		t.Fatalf("bootstrap: %v", err)
	}
	defer app.ClearBootstrap()

	// The superuser the dashboard would use.
	superusers, err := app.FindCollectionByNameOrId(core.CollectionNameSuperusers)
	if err != nil {
		t.Fatalf("superusers collection: %v", err)
	}
	admin := core.NewRecord(superusers)
	admin.Set("email", "launcher-e2e@example.com")
	admin.Set("password", "launchere2epass123")
	if err := app.Save(admin); err != nil {
		t.Fatalf("admin save: %v", err)
	}
	token, err := admin.NewAuthToken()
	if err != nil {
		t.Fatalf("token: %v", err)
	}

	mux, err := apis.NewRouter(app)
	if err != nil {
		t.Fatalf("router: %v", err)
	}
	handler, err := mux.BuildMux()
	if err != nil {
		t.Fatalf("mux: %v", err)
	}
	ts := httptest.NewServer(handler)
	defer ts.Close()

	get := func(path string) (int, string) {
		req, err := http.NewRequest("GET", ts.URL+path, nil)
		if err != nil {
			t.Fatalf("request: %v", err)
		}
		req.Header.Set("Authorization", token)
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatalf("do: %v", err)
		}
		defer resp.Body.Close()
		body, _ := io.ReadAll(resp.Body)
		return resp.StatusCode, string(body)
	}

	// Every new v1 surface must answer 200 through the launcher shape —
	// a 500 "partitioning runtime is not available" here is exactly the
	// real-user failure this test exists to catch.
	for _, path := range []string{
		"/api/mirabase/v1/capabilities",
		"/api/mirabase/v1/setup",
		"/api/mirabase/v1/templates",
		"/api/mirabase/v1/openapi.json",
	} {
		status, body := get(path)
		if status != 200 {
			t.Errorf("launcher-shaped GET %s = %d (want 200): %s", path, status, body)
		}
	}

	// The capabilities catalog carries the released operations.
	status, body := get("/api/mirabase/v1/capabilities")
	if status != 200 || !strings.Contains(body, "pbimport.create") {
		t.Errorf("capabilities: %d %s", status, body)
	}

	// baseAppOrDie consumers must resolve the launcher's embedded BaseApp —
	// a nil unwrap used to panic on template.publish with real installs.
	post := func(path string, payload string) (int, string) {
		req, err := http.NewRequest("POST", ts.URL+path, strings.NewReader(payload))
		if err != nil {
			t.Fatalf("request: %v", err)
		}
		req.Header.Set("Authorization", token)
		req.Header.Set("Content-Type", "application/json")
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatalf("do: %v", err)
		}
		defer resp.Body.Close()
		body, _ := io.ReadAll(resp.Body)
		return resp.StatusCode, string(body)
	}
	status, body = post("/api/mirabase/v1/templates", `{"templateId":"tpl-launcher","collectionNames":["`+core.CollectionNameSuperusers+`"]}`)
	if status != 200 {
		t.Errorf("launcher-shaped template.publish = %d (want 200): %s", status, body)
	}
}
