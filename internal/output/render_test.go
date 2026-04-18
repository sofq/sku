package output_test

import (
	"bytes"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/sofq/sku/internal/catalog"
	"github.com/sofq/sku/internal/output"
)

func ptrS(s string) *string   { return &s } //nolint:unused // pointer helper for future tests
func ptrI(i int64) *int64     { return &i }
func ptrF(f float64) *float64 { return &f }

func sampleRow() catalog.Row {
	return catalog.Row{
		SKUID:          "anthropic/claude-opus-4.6::anthropic::default",
		Provider:       "anthropic",
		Service:        "llm",
		Kind:           "llm.text",
		ResourceName:   "anthropic/claude-opus-4.6",
		Region:         "",
		RegionGroup:    "",
		CatalogVersion: "2026.04.18",
		Currency:       "USD",
		Terms:          catalog.Terms{Commitment: "on_demand"},
		ResourceAttrs: catalog.ResourceAttrs{
			ContextLength:   ptrI(200000),
			MaxOutputTokens: ptrI(64000),
			Modality:        []string{"text"},
			Capabilities:    []string{"tools"},
		},
		Prices: []catalog.Price{
			{Dimension: "prompt", Amount: 1.5e-5, Unit: "token"},
			{Dimension: "completion", Amount: 7.5e-5, Unit: "token"},
		},
		Health: &catalog.Health{
			Uptime30d:    ptrF(0.998),
			LatencyP50Ms: ptrI(420),
			LatencyP95Ms: ptrI(1100),
			ObservedAt:   ptrI(1745020800),
		},
	}
}

func TestRender_FullPresetProducesSpecShape(t *testing.T) {
	env := output.Render(sampleRow(), output.PresetFull)

	var buf bytes.Buffer
	require.NoError(t, output.Encode(&buf, env, false))

	var got map[string]any
	require.NoError(t, json.Unmarshal(buf.Bytes(), &got))

	require.Equal(t, "anthropic", got["provider"])
	require.Equal(t, "llm", got["service"])
	require.Equal(t, "anthropic/claude-opus-4.6::anthropic::default", got["sku_id"])

	resource := got["resource"].(map[string]any)
	require.Equal(t, "llm.text", resource["kind"])
	require.Equal(t, "anthropic/claude-opus-4.6", resource["name"])

	// Location: nullable empty region_normalized -> null in output
	location := got["location"].(map[string]any)
	require.Nil(t, location["provider_region"])
	require.Nil(t, location["normalized_region"])

	// Price array
	prices := got["price"].([]any)
	require.Len(t, prices, 2)
	first := prices[0].(map[string]any)
	require.Contains(t, []string{"prompt", "completion"}, first["dimension"])
	require.Equal(t, "USD", first["currency"])
	require.Equal(t, "token", first["unit"])

	// Terms: commitment populated, tenancy/os nulls
	terms := got["terms"].(map[string]any)
	require.Equal(t, "on_demand", terms["commitment"])
	require.Nil(t, terms["tenancy"])
	require.Nil(t, terms["os"])

	// Health populated for non-aggregated row
	require.NotNil(t, got["health"])

	// Source + catalog_version
	source := got["source"].(map[string]any)
	require.Equal(t, "2026.04.18", source["catalog_version"])

	// Raw absent in M1
	require.Nil(t, got["raw"])
}

func TestRender_AgentPresetTrimsFields(t *testing.T) {
	env := output.Render(sampleRow(), output.PresetAgent)

	var buf bytes.Buffer
	require.NoError(t, output.Encode(&buf, env, false))

	var got map[string]any
	require.NoError(t, json.Unmarshal(buf.Bytes(), &got))

	// agent preset keeps: provider, service, resource.name,
	// location.provider_region, price, terms.commitment
	require.Equal(t, "anthropic", got["provider"])
	require.Equal(t, "llm", got["service"])
	resource := got["resource"].(map[string]any)
	require.Equal(t, "anthropic/claude-opus-4.6", resource["name"])
	require.NotContains(t, resource, "attributes")
	require.Nil(t, resource["vcpu"])

	require.NotContains(t, got, "health")
	require.NotContains(t, got, "source")
	require.NotContains(t, got, "raw")

	location := got["location"].(map[string]any)
	require.Nil(t, location["provider_region"])

	terms := got["terms"].(map[string]any)
	require.Equal(t, "on_demand", terms["commitment"])
	require.NotContains(t, terms, "tenancy")
}

func TestEncode_CompactAndPretty(t *testing.T) {
	env := output.Render(sampleRow(), output.PresetAgent)

	var compact, pretty bytes.Buffer
	require.NoError(t, output.Encode(&compact, env, false))
	require.NoError(t, output.Encode(&pretty, env, true))

	require.NotContains(t, compact.String(), "\n  ", "compact has no indentation")
	require.Contains(t, pretty.String(), "\n  ", "pretty is indented")

	// Both end with exactly one trailing newline.
	require.Equal(t, byte('\n'), compact.Bytes()[compact.Len()-1])
	require.Equal(t, byte('\n'), pretty.Bytes()[pretty.Len()-1])
}

func TestRender_AggregatedMarkedInAttributes(t *testing.T) {
	r := sampleRow()
	r.Provider = "openrouter"
	r.SKUID = "anthropic/claude-opus-4.6::openrouter::default"
	r.Aggregated = true
	r.Health = nil

	env := output.Render(r, output.PresetFull)
	var buf bytes.Buffer
	require.NoError(t, output.Encode(&buf, env, false))

	var got map[string]any
	require.NoError(t, json.Unmarshal(buf.Bytes(), &got))
	resource := got["resource"].(map[string]any)
	attrs := resource["attributes"].(map[string]any)
	require.Equal(t, true, attrs["aggregated"])
	require.Nil(t, got["health"])
}
