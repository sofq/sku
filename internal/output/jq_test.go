package output_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/sofq/sku/internal/output"
)

func TestApplyJQ_IdentityPassthrough(t *testing.T) {
	doc := map[string]any{"a": 1.0}
	got, err := output.ApplyJQ(doc, ".")
	require.NoError(t, err)
	require.Equal(t, doc, got)
}

func TestApplyJQ_Projection(t *testing.T) {
	doc := map[string]any{"price": []any{map[string]any{"amount": 0.002}}}
	got, err := output.ApplyJQ(doc, ".price[0].amount")
	require.NoError(t, err)
	require.InEpsilon(t, 0.002, got, 1e-9)
}

func TestApplyJQ_SyntaxError(t *testing.T) {
	_, err := output.ApplyJQ(map[string]any{}, "=== not jq ===")
	require.Error(t, err)
}
