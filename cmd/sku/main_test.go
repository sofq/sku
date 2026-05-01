package sku

import (
	"os"
	"testing"
)

func TestMain(m *testing.M) {
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
