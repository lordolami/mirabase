package ghupdate

import (
	"testing"

	"github.com/spf13/cobra"

	"mirabase/tests"
)

// The branding contract requires updater negative tests: there must be no
// default release source, so an unconfigured executable can never be updated
// into an upstream project and no update target is ever implied implicitly.
func TestRegisterNegative(t *testing.T) {
	scenarios := []struct {
		name      string
		owner     string
		repo      string
		expectErr bool
	}{
		{"no config", "", "", true},
		{"owner only", "someowner", "", true},
		{"repo only", "", "somerepo", true},
		{"explicit source", "someowner", "somerepo", false},
	}

	for _, s := range scenarios {
		t.Run(s.name, func(t *testing.T) {
			app, _ := tests.NewTestApp()
			defer app.Cleanup()

			rootCmd := &cobra.Command{Version: "0.0.0-test"}

			err := Register(app, rootCmd, Config{Owner: s.owner, Repo: s.repo})

			if s.expectErr && err == nil {
				t.Fatal("Expected an error for a missing release source")
			}
			if !s.expectErr && err != nil {
				t.Fatalf("Did not expect an error for an explicit release source, got: %v", err)
			}
		})
	}
}
