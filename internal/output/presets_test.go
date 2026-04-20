package output_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/sofq/sku/internal/output"
)

func buildFullSample() output.Envelope {
	return output.Render(sampleRow(), output.PresetFull)
}

func TestProject_Agent_KeepsSpecFields(t *testing.T) {
	full := buildFullSample()
	trimmed := output.Project(full, output.PresetAgent, "llm.text")
	require.NotZero(t, trimmed.Provider)
	require.NotEmpty(t, trimmed.Price)
	require.Nil(t, trimmed.Health)
	require.Nil(t, trimmed.Source)
}

func TestProject_Price_DropsAllButPrice(t *testing.T) {
	full := buildFullSample()
	trimmed := output.Project(full, output.PresetPrice, "llm.text")
	require.Empty(t, trimmed.Provider)
	require.Empty(t, trimmed.Service)
	require.Nil(t, trimmed.Resource)
	require.Nil(t, trimmed.Location)
	require.Nil(t, trimmed.Terms)
	require.NotEmpty(t, trimmed.Price)
}

func TestProject_Compare_LLMText_IncludesKindFields(t *testing.T) {
	full := buildFullSample()
	trimmed := output.Project(full, output.PresetCompare, "llm.text")
	require.NotNil(t, trimmed.Resource)
	require.NotZero(t, trimmed.Resource.Name)
	require.NotNil(t, trimmed.Resource.ContextLength)
	require.NotEmpty(t, trimmed.Resource.Capabilities)
	require.NotNil(t, trimmed.Health)
	require.NotNil(t, trimmed.Health.Uptime30d)
	require.NotNil(t, trimmed.Health.LatencyP95Ms)
}
