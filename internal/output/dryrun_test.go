package output_test

import (
	"bytes"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/sofq/sku/internal/output"
)

func TestEmitDryRun_StableSchema(t *testing.T) {
	var buf bytes.Buffer
	require.NoError(t, output.EmitDryRun(&buf, output.DryRunPlan{
		Command:      "llm price",
		ResolvedArgs: map[string]any{"model": "anthropic/claude-opus-4.6"},
		Shards:       []string{"openrouter"},
		TermsHash:    "deadbeef",
		SQL:          "SELECT ...",
		Preset:       "agent",
	}))
	var got map[string]any
	require.NoError(t, json.Unmarshal(buf.Bytes(), &got))
	require.Equal(t, true, got["dry_run"])
	require.Equal(t, "llm price", got["command"])
	require.Equal(t, "openrouter", got["shards"].([]any)[0])
	require.Equal(t, float64(1), got["schema_version"])
}
