package sku

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/sofq/sku/internal/catalog"
)

// seedTestDataDir creates a temp SKU_DATA_DIR containing an openrouter.db
// built from internal/catalog/testdata/seed.sql.
func seedTestDataDir(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	ddl, err := os.ReadFile(filepath.Join("..", "..", "internal", "catalog", "testdata", "seed.sql"))
	require.NoError(t, err)
	require.NoError(t, catalog.BuildFromSQL(filepath.Join(dir, "openrouter.db"), string(ddl)))
	t.Setenv("SKU_DATA_DIR", dir)
	return dir
}

func runLLMPrice(t *testing.T, args ...string) (stdout, stderr string, code int) {
	t.Helper()
	var out, errb bytes.Buffer
	cmd := newRootCmd()
	cmd.SetOut(&out)
	cmd.SetErr(&errb)
	cmd.SetArgs(append([]string{"llm", "price"}, args...))
	err := cmd.Execute()
	code = 0
	if err != nil {
		// The command sets its own exit code via the returned error;
		// for unit testing we just check that the error surface is populated.
		// The process-level exit code is exercised in the e2e test.
		code = 1
	}
	return out.String(), errb.String(), code
}

func TestLLMPrice_HappyPath_ReturnsJSONPerEnvelope(t *testing.T) {
	seedTestDataDir(t)

	out, _, _ := runLLMPrice(t, "--model", "anthropic/claude-opus-4.6")

	// stdout is NDJSON (one row per line) since multiple serving providers match.
	lines := splitLines(out)
	require.Len(t, lines, 2)

	providers := map[string]bool{}
	for _, line := range lines {
		var env map[string]any
		require.NoError(t, json.Unmarshal([]byte(line), &env))
		require.Equal(t, "llm", env["service"])
		providers[env["provider"].(string)] = true
		require.NotEmpty(t, env["price"])
	}
	require.True(t, providers["anthropic"])
	require.True(t, providers["aws-bedrock"])
	require.False(t, providers["openrouter"], "aggregated row excluded by default")
}

func TestLLMPrice_ServingProviderFlag(t *testing.T) {
	seedTestDataDir(t)

	out, _, _ := runLLMPrice(t,
		"--model", "anthropic/claude-opus-4.6",
		"--serving-provider", "aws-bedrock",
	)
	lines := splitLines(out)
	require.Len(t, lines, 1)

	var env map[string]any
	require.NoError(t, json.Unmarshal([]byte(lines[0]), &env))
	require.Equal(t, "aws-bedrock", env["provider"])
}

func TestLLMPrice_IncludeAggregated(t *testing.T) {
	seedTestDataDir(t)

	out, _, _ := runLLMPrice(t,
		"--model", "anthropic/claude-opus-4.6",
		"--include-aggregated",
	)
	require.Len(t, splitLines(out), 3)
}

func TestLLMPrice_NotFound_ReturnsExit3Envelope(t *testing.T) {
	seedTestDataDir(t)

	_, stderr, code := runLLMPrice(t, "--model", "fake/model")
	require.NotZero(t, code)

	var env map[string]any
	require.NoError(t, json.Unmarshal([]byte(stderr), &env))
	body := env["error"].(map[string]any)
	require.Equal(t, "not_found", body["code"])
	require.Contains(t, body["suggestion"].(string), "sku update")
}

func TestLLMPrice_MissingModel_ReturnsValidationError(t *testing.T) {
	seedTestDataDir(t)

	_, stderr, code := runLLMPrice(t)
	require.NotZero(t, code)

	var env map[string]any
	require.NoError(t, json.Unmarshal([]byte(stderr), &env))
	body := env["error"].(map[string]any)
	require.Equal(t, "validation", body["code"])
}

func TestLLMPrice_ShardMissing_ReturnsExit3(t *testing.T) {
	t.Setenv("SKU_DATA_DIR", t.TempDir())

	_, stderr, code := runLLMPrice(t, "--model", "anthropic/claude-opus-4.6")
	require.NotZero(t, code)

	var env map[string]any
	require.NoError(t, json.Unmarshal([]byte(stderr), &env))
	body := env["error"].(map[string]any)
	require.Equal(t, "not_found", body["code"])
	require.Contains(t, body["suggestion"].(string), "sku update")
	details := body["details"].(map[string]any)
	require.Equal(t, "openrouter", details["shard"])
}

func TestLLMPrice_YAMLOutput(t *testing.T) {
	seedTestDataDir(t)
	out, _, _ := runLLMPrice(t,
		"--model", "anthropic/claude-opus-4.6",
		"--serving-provider", "aws-bedrock",
		"--yaml",
	)
	require.Contains(t, out, "provider: aws-bedrock")
}

func TestLLMPrice_JQReducesOutput(t *testing.T) {
	seedTestDataDir(t)
	out, _, _ := runLLMPrice(t,
		"--model", "anthropic/claude-opus-4.6",
		"--serving-provider", "aws-bedrock",
		"--jq", ".price[0].amount",
	)
	require.Regexp(t, `^[\d.e-]+\n$`, out)
}

func TestLLMPrice_FieldsProjection(t *testing.T) {
	seedTestDataDir(t)
	out, _, _ := runLLMPrice(t,
		"--model", "anthropic/claude-opus-4.6",
		"--serving-provider", "aws-bedrock",
		"--fields", "provider,price.0.amount",
	)
	require.Contains(t, out, `"provider":"aws-bedrock"`)
	require.NotContains(t, out, `"service"`)
}

func TestLLMPrice_DryRun_DoesNotTouchShard(t *testing.T) {
	t.Setenv("SKU_DATA_DIR", t.TempDir())
	out, _, code := runLLMPrice(t, "--model", "anthropic/claude-opus-4.6", "--dry-run")
	require.Zero(t, code)
	require.Contains(t, out, `"dry_run":true`)
	require.Contains(t, out, `"command":"llm price"`)
}

func TestLLMPrice_PresetPrice(t *testing.T) {
	seedTestDataDir(t)
	out, _, _ := runLLMPrice(t,
		"--model", "anthropic/claude-opus-4.6",
		"--serving-provider", "aws-bedrock",
		"--preset", "price",
	)
	require.Contains(t, out, `"price":[`)
	require.NotContains(t, out, `"provider"`)
}

func splitLines(s string) []string {
	var out []string
	start := 0
	for i := 0; i < len(s); i++ {
		if s[i] == '\n' {
			if i > start {
				out = append(out, s[start:i])
			}
			start = i + 1
		}
	}
	if start < len(s) {
		out = append(out, s[start:])
	}
	return out
}
