package osutils_test

import (
	"testing"

	"mirabase/tools/osutils"
)

func TestGetEnv(t *testing.T) {
	// MIRABASE_-prefixed value takes precedence over the legacy PB_ one
	t.Setenv("MIRABASE_TEST_VALUE", "new-value")
	t.Setenv("PB_TEST_VALUE", "legacy-value")

	if v := osutils.GetEnv("TEST_VALUE"); v != "new-value" {
		t.Fatalf("Expected the MIRABASE_-prefixed value %q, got %q", "new-value", v)
	}

	// legacy PB_-prefixed fallback
	t.Setenv("MIRABASE_TEST_VALUE", "")

	if v := osutils.GetEnv("TEST_VALUE"); v != "legacy-value" {
		t.Fatalf("Expected the legacy PB_-prefixed fallback %q, got %q", "legacy-value", v)
	}

	// both unset -> empty value
	t.Setenv("PB_TEST_VALUE", "")

	if v := osutils.GetEnv("TEST_VALUE"); v != "" {
		t.Fatalf("Expected empty value when both variables are unset, got %q", v)
	}
}
