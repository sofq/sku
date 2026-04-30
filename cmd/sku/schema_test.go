package sku

import (
	"bytes"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
)

func runSchema(t *testing.T, args ...string) (string, string, int) {
	t.Helper()
	var out, errb bytes.Buffer
	root := newRootCmd()
	root.SetOut(&out)
	root.SetErr(&errb)
	root.SetArgs(append([]string{"schema"}, args...))
	err := root.Execute()
	code := 0
	if err != nil {
		code = 1
	}
	return out.String(), errb.String(), code
}

func TestSchema_List_EmitsShardNames(t *testing.T) {
	out, _, code := runSchema(t, "--list")
	require.Zero(t, code)
	var doc map[string]any
	require.NoError(t, json.Unmarshal([]byte(out), &doc))
	shards := doc["shards"].([]any)
	require.Contains(t, shards, "openrouter")
}

func TestSchema_Errors_EmitsCatalog(t *testing.T) {
	out, _, code := runSchema(t, "--errors")
	require.Zero(t, code)
	var doc map[string]any
	require.NoError(t, json.Unmarshal([]byte(out), &doc))
	codes := doc["codes"].(map[string]any)
	_, ok := codes["not_found"]
	require.True(t, ok)
}

func TestSchema_ListServingProviders_ReadsShard(t *testing.T) {
	seedTestDataDir(t)
	out, _, code := runSchema(t, "--list-serving-providers")
	require.Zero(t, code)
	require.Contains(t, out, "aws-bedrock")
	require.Contains(t, out, "anthropic")
}

func TestSchema_Base_ListsProvidersAndGlobals(t *testing.T) {
	out, _, code := runSchema(t)
	require.Zero(t, code)
	var doc map[string]any
	require.NoError(t, json.Unmarshal([]byte(out), &doc))
	providers := doc["providers"].([]any)
	require.Contains(t, providers, "openrouter")
	require.NotEmpty(t, doc["globals"])
}

func TestSchema_KindTermOverrides_EmitsCatalog(t *testing.T) {
	out, _, code := runSchema(t, "--kind-term-overrides")
	require.Zero(t, code)
	var doc map[string]any
	require.NoError(t, json.Unmarshal([]byte(out), &doc))
	overrides, ok := doc["kind_term_overrides"].(map[string]any)
	require.True(t, ok)
	dbRel, ok := overrides["db.relational"].(map[string]any)
	require.True(t, ok)
	tenancy := dbRel["tenancy"].(map[string]any)
	require.Equal(t, "engine", tenancy["semantic_name"])
}

func TestSchema_listCommands_includesBatchRegistry(t *testing.T) {
	out, _, code := runSchema(t, "--list-commands")
	require.Zero(t, code)
	for _, n := range []string{"aws ec2 price", "compare", "estimate", "llm price"} {
		require.Contains(t, out, n, "schema list missing %q", n)
	}
}
