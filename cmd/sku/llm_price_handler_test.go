package sku

import (
	"context"
	"testing"

	"github.com/sofq/sku/internal/batch"
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
