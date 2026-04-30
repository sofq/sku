package schema

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestKindTermOverridesCatalog_StableShape(t *testing.T) {
	cat := KindTermOverridesCatalog()

	require.Equal(t, 1, cat["schema_version"])

	overrides, ok := cat["kind_term_overrides"].(map[string]KindTermOverrides)
	require.True(t, ok, "kind_term_overrides should be a typed map")

	for _, kind := range []string{"db.relational", "cache.kv", "container.orchestration"} {
		require.Contains(t, overrides, kind)
	}

	dbRel := overrides["db.relational"]
	require.NotNil(t, dbRel.Tenancy)
	require.Equal(t, "engine", dbRel.Tenancy.SemanticName)
	require.Contains(t, dbRel.Tenancy.Values, "postgres")
	require.NotNil(t, dbRel.OS)
	require.Equal(t, "deployment_option", dbRel.OS.SemanticName)
	require.Contains(t, dbRel.OS.Values, "single-az")

	cache := overrides["cache.kv"]
	require.NotNil(t, cache.Tenancy)
	require.Contains(t, cache.Tenancy.Values, "redis")

	containers := overrides["container.orchestration"]
	require.NotNil(t, containers.Tenancy)
	require.Equal(t, []string{"kubernetes"}, containers.Tenancy.Values)
}

func TestKindTermOverridesCatalog_RoundTripsJSON(t *testing.T) {
	cat := KindTermOverridesCatalog()
	buf, err := json.Marshal(cat)
	require.NoError(t, err)

	var doc map[string]any
	require.NoError(t, json.Unmarshal(buf, &doc))

	overrides, ok := doc["kind_term_overrides"].(map[string]any)
	require.True(t, ok)
	dbRel, ok := overrides["db.relational"].(map[string]any)
	require.True(t, ok)
	tenancy, ok := dbRel["tenancy"].(map[string]any)
	require.True(t, ok)
	require.Equal(t, "engine", tenancy["semantic_name"])
	require.Equal(t, "--engine", tenancy["cli_flag"])
}
