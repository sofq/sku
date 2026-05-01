package sku

import (
	"bytes"
	"encoding/json"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/sofq/sku/internal/batch"
	"github.com/sofq/sku/internal/catalog"
)

// seedCompareDataDir creates an openrouter.db with two providers at different
// prompt prices for "vendor/cheap-model".
func seedCompareDataDir(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()

	ddl := `
CREATE TABLE metadata (key TEXT PRIMARY KEY, value TEXT NOT NULL);
INSERT INTO metadata VALUES ('schema_version','1'),('catalog_version','2026.04.24'),
  ('currency','USD'),('generated_at','2026-04-24T00:00:00Z');

CREATE TABLE skus (
  sku_id TEXT PRIMARY KEY, provider TEXT NOT NULL, service TEXT NOT NULL,
  kind TEXT NOT NULL, resource_name TEXT NOT NULL, region TEXT NOT NULL DEFAULT '',
  region_normalized TEXT NOT NULL DEFAULT '', terms_hash TEXT NOT NULL DEFAULT ''
);
INSERT INTO skus VALUES
  ('m/cheap','cheap-provider','llm','llm.text','vendor/cheap-model','','',''),
  ('m/pricey','pricey-provider','llm','llm.text','vendor/cheap-model','','','');

CREATE TABLE terms (
  sku_id TEXT PRIMARY KEY, commitment TEXT NOT NULL DEFAULT 'on_demand',
  tenancy TEXT NOT NULL DEFAULT '', os TEXT NOT NULL DEFAULT '',
  support_tier TEXT, upfront TEXT, payment_option TEXT
);
INSERT INTO terms VALUES ('m/cheap','on_demand','','',NULL,NULL,NULL);
INSERT INTO terms VALUES ('m/pricey','on_demand','','',NULL,NULL,NULL);

CREATE TABLE resource_attrs (
  sku_id TEXT PRIMARY KEY, vcpu INTEGER, memory_gb REAL, storage_gb REAL,
  gpu_count INTEGER, gpu_model TEXT, architecture TEXT,
  context_length INTEGER, max_output_tokens INTEGER,
  modality TEXT, capabilities TEXT, quantization TEXT,
  durability_nines INTEGER, availability_tier TEXT
);
INSERT INTO resource_attrs (sku_id, context_length) VALUES ('m/cheap', 200000);
INSERT INTO resource_attrs (sku_id, context_length) VALUES ('m/pricey', 200000);

CREATE TABLE prices (
  sku_id TEXT NOT NULL, dimension TEXT NOT NULL, tier TEXT NOT NULL DEFAULT '',
  tier_upper TEXT NOT NULL DEFAULT '',
  amount REAL NOT NULL, unit TEXT NOT NULL DEFAULT 'token',
  PRIMARY KEY (sku_id, dimension, tier)
);
INSERT INTO prices VALUES ('m/cheap','prompt','','',5e-7,'token');
INSERT INTO prices VALUES ('m/cheap','completion','','',1.5e-6,'token');
INSERT INTO prices VALUES ('m/pricey','prompt','','',1.5e-6,'token');
INSERT INTO prices VALUES ('m/pricey','completion','','',4.5e-6,'token');

CREATE TABLE health (
  sku_id TEXT PRIMARY KEY, uptime_30d REAL, latency_p50_ms INTEGER,
  latency_p95_ms INTEGER, throughput_tokens_per_sec REAL, observed_at INTEGER
);
`
	require.NoError(t, catalog.BuildFromSQL(filepath.Join(dir, "openrouter.db"), ddl))
	t.Setenv("SKU_DATA_DIR", dir)
	return dir
}

func runLLMCompare(t *testing.T, args ...string) (stdout, stderr string, code int) {
	t.Helper()
	var out, errb bytes.Buffer
	cmd := newRootCmd()
	cmd.SetOut(&out)
	cmd.SetErr(&errb)
	cmd.SetArgs(append([]string{"llm", "compare"}, args...))
	err := cmd.Execute()
	code = 0
	if err != nil {
		code = 1
	}
	return out.String(), errb.String(), code
}

func TestLLMCompare_HappyPath_SortedCheapestFirst(t *testing.T) {
	seedCompareDataDir(t)

	out, _, code := runLLMCompare(t, "--model", "vendor/cheap-model")
	require.Zero(t, code)

	lines := splitLines(out)
	require.Len(t, lines, 2)

	var providers []string
	for _, line := range lines {
		var env map[string]any
		require.NoError(t, json.Unmarshal([]byte(line), &env))
		providers = append(providers, env["provider"].(string))
	}
	require.Equal(t, "cheap-provider", providers[0], "cheapest provider must be first")
	require.Equal(t, "pricey-provider", providers[1])
}

func TestLLMCompare_MissingModel_ReturnsValidationError(t *testing.T) {
	seedCompareDataDir(t)
	_, stderr, code := runLLMCompare(t)
	require.NotZero(t, code)
	var env map[string]any
	require.NoError(t, json.Unmarshal([]byte(stderr), &env))
	require.Equal(t, "validation", env["error"].(map[string]any)["code"])
}

func TestLLMCompare_ShardMissing_ReturnsNotFound(t *testing.T) {
	t.Setenv("SKU_DATA_DIR", t.TempDir())
	_, stderr, code := runLLMCompare(t, "--model", "vendor/cheap-model")
	require.NotZero(t, code)
	var env map[string]any
	require.NoError(t, json.Unmarshal([]byte(stderr), &env))
	require.Equal(t, "not_found", env["error"].(map[string]any)["code"])
}

func TestLLMCompare_DryRun(t *testing.T) {
	t.Setenv("SKU_DATA_DIR", t.TempDir())
	out, _, code := runLLMCompare(t, "--model", "vendor/cheap-model", "--dry-run")
	require.Zero(t, code)
	require.Contains(t, out, `"dry_run":true`)
	require.Contains(t, out, `"command":"llm compare"`)
}

func TestLLMCompare_Registered(t *testing.T) {
	if _, ok := batch.Lookup("llm compare"); !ok {
		t.Fatal("llm compare handler not registered")
	}
}

func TestLLMCompare_SortStability(t *testing.T) {
	seedTestDataDir(t)
	out, _, code := runLLMCompare(t, "--model", "anthropic/claude-opus-4.6")
	require.Zero(t, code)
	lines := splitLines(out)
	require.Len(t, lines, 2)
	providersSeen := map[string]bool{}
	for _, line := range lines {
		var env map[string]any
		require.NoError(t, json.Unmarshal([]byte(line), &env))
		providersSeen[env["provider"].(string)] = true
	}
	require.True(t, providersSeen["anthropic"])
	require.True(t, providersSeen["aws-bedrock"])
}

func TestLLMCompare_ServingProviderFilter(t *testing.T) {
	seedCompareDataDir(t)
	out, _, code := runLLMCompare(t, "--model", "vendor/cheap-model", "--serving-provider", "pricey-provider")
	require.Zero(t, code)
	lines := splitLines(out)
	require.Len(t, lines, 1)
	var env map[string]any
	require.NoError(t, json.Unmarshal([]byte(lines[0]), &env))
	require.Equal(t, "pricey-provider", env["provider"])
}

func TestLLMCompare_ModelNotFound(t *testing.T) {
	seedCompareDataDir(t)
	_, stderr, code := runLLMCompare(t, "--model", "vendor/nonexistent-model")
	require.NotZero(t, code)
	var env map[string]any
	require.NoError(t, json.Unmarshal([]byte(stderr), &env))
	require.Equal(t, "not_found", env["error"].(map[string]any)["code"])
}
