package output_test

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/sofq/sku/internal/output"
)

func TestPipeline_AgentPreset_EmitsCompactJSON(t *testing.T) {
	row := sampleRow()
	b, err := output.Pipeline(row, output.Options{Preset: output.PresetAgent, Format: "json"})
	require.NoError(t, err)
	require.Contains(t, string(b), `"provider":"anthropic"`)
	require.NotContains(t, string(b), `"health"`)
	require.NotContains(t, string(b), `"source"`)
}

func TestPipeline_PricePreset_DropsEverythingExceptPrice(t *testing.T) {
	row := sampleRow()
	b, err := output.Pipeline(row, output.Options{Preset: output.PresetPrice, Format: "json"})
	require.NoError(t, err)
	require.NotContains(t, string(b), `"provider"`)
	require.Contains(t, string(b), `"price":[`)
}

func TestPipeline_FullPreset_IncludesHealth(t *testing.T) {
	row := sampleRow()
	b, err := output.Pipeline(row, output.Options{Preset: output.PresetFull, Format: "json"})
	require.NoError(t, err)
	require.Contains(t, string(b), `"health"`)
}

func TestPipeline_Pretty_IndentsJSON(t *testing.T) {
	row := sampleRow()
	b, err := output.Pipeline(row, output.Options{Preset: output.PresetAgent, Format: "json", Pretty: true})
	require.NoError(t, err)
	require.True(t, strings.Contains(string(b), "\n  "), "pretty JSON must have indent")
}
