package sku

import (
	"os"
	"testing"
)

func TestMain(m *testing.M) {
	// Test fixtures embed a fixed `as_of_utc` and drift past the 14-day warn
	// threshold. The staleness warning prints to stderr and breaks tests that
	// parse stderr as JSON. Tests that specifically exercise staleness behavior
	// can override with t.Setenv.
	if os.Getenv("SKU_STALE_OK") == "" {
		_ = os.Setenv("SKU_STALE_OK", "1")
	}
	// Isolate tests from the developer's local sku config (e.g. stale_error_days).
	// Individual tests that specifically need a config dir (configure_test.go) use
	// t.Setenv("SKU_CONFIG_DIR", ...) which overrides this for their duration.
	if os.Getenv("SKU_CONFIG_DIR") == "" {
		dir, err := os.MkdirTemp("", "sku-test-config-*")
		if err != nil {
			panic(err)
		}
		_ = os.Setenv("SKU_CONFIG_DIR", dir)
		code := m.Run()
		_ = os.RemoveAll(dir)
		os.Exit(code)
	}
	os.Exit(m.Run())
}
