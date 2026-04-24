package sku

import (
	"context"
	"errors"
	"testing"

	"github.com/sofq/sku/internal/batch"
	skuerrors "github.com/sofq/sku/internal/errors"
)

func TestHandleLLMPrice_notFoundReturnsEnvelope(t *testing.T) {
	t.Setenv("SKU_DATA_DIR", t.TempDir())
	args := map[string]any{"model": "anthropic/claude-opus-4.6"}
	s := batch.Settings{}
	_, err := handleLLMPrice(context.Background(), args, batch.Env{Settings: &s})
	if err == nil {
		t.Fatal("expected error when shard missing")
	}
}

func TestHandleLLMPrice_registered(t *testing.T) {
	if _, ok := batch.Lookup("llm price"); !ok {
		t.Fatal("llm price handler not registered")
	}
}

func TestHandleLLMPrice_autoFetch_staticVsAttempted(t *testing.T) {
	t.Setenv("SKU_DATA_DIR", t.TempDir())

	// AutoFetch=false → static "shard not installed" (not_found, no network call).
	s := batch.Settings{AutoFetch: false}
	_, errNoFetch := handleLLMPrice(context.Background(), map[string]any{"model": "x/y"}, batch.Env{Settings: &s})
	if errNoFetch == nil {
		t.Fatal("expected error when shard missing and AutoFetch=false")
	}
	var e *skuerrors.E
	if !errors.As(errNoFetch, &e) {
		t.Fatalf("expected *skuerrors.E, got %T: %v", errNoFetch, errNoFetch)
	}
	if e.Code != skuerrors.CodeNotFound || e.Message != "openrouter shard not installed" {
		t.Fatalf("expected static shard-missing error, got code=%s msg=%q", e.Code, e.Message)
	}

	// AutoFetch=true with blocked URL → server error (auto-fetch attempted, not static).
	t.Setenv("SKU_UPDATE_BASE_URL", "http://127.0.0.1:1")
	sFetch := batch.Settings{AutoFetch: true}
	_, errFetch := handleLLMPrice(context.Background(), map[string]any{"model": "x/y"}, batch.Env{Settings: &sFetch})
	if errFetch == nil {
		t.Fatal("expected error with blocked URL")
	}
	var e2 *skuerrors.E
	if !errors.As(errFetch, &e2) {
		t.Fatalf("expected *skuerrors.E, got %T: %v", errFetch, errFetch)
	}
	if e2.Code == skuerrors.CodeNotFound && e2.Message == "openrouter shard not installed" {
		t.Fatal("auto-fetch was not attempted: got static shard-missing error")
	}
}
