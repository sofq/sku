package output_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/sofq/sku/internal/output"
)

func TestApplyFields_KeepsSelected(t *testing.T) {
	doc := map[string]any{
		"provider": "anthropic",
		"service":  "llm",
		"price": []any{
			map[string]any{"amount": 1.5e-5, "dimension": "prompt"},
		},
		"health": map[string]any{"uptime_30d": 0.999},
	}
	got := output.ApplyFields(doc, "provider,price.0.amount")
	require.Equal(t, "anthropic", got["provider"])
	require.Nil(t, got["service"])
	require.Nil(t, got["health"])
	price := got["price"].([]any)
	inner := price[0].(map[string]any)
	require.Equal(t, 1.5e-5, inner["amount"])
	_, hasDim := inner["dimension"]
	require.False(t, hasDim)
}

func TestApplyFields_MissingPath_SilentlyDropped(t *testing.T) {
	doc := map[string]any{"provider": "aws"}
	got := output.ApplyFields(doc, "nope.nested.thing")
	require.Empty(t, got)
}

func TestApplyFields_EmptyExpr_ReturnsInput(t *testing.T) {
	doc := map[string]any{"a": 1}
	require.Equal(t, doc, output.ApplyFields(doc, ""))
}
