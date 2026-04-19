package sku

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestCompare_crossProvider_minVCPU(t *testing.T) {
	testutilSeededComputeVMCatalog(t)
	var stdout, stderr bytes.Buffer
	cmd := newRootCmd()
	cmd.SetOut(&stdout)
	cmd.SetErr(&stderr)
	cmd.SetArgs([]string{"compare", "--kind", "compute.vm", "--vcpu", "2", "--memory", "4", "--regions", "us-east", "--limit", "5"})
	require.NoError(t, cmd.Execute())

	lines := strings.Split(strings.TrimRight(stdout.String(), "\n"), "\n")
	require.NotEmpty(t, lines)
	providersSeen := map[string]bool{}
	for _, ln := range lines {
		var env map[string]any
		require.NoError(t, json.Unmarshal([]byte(ln), &env))
		if p, ok := env["provider"].(string); ok {
			providersSeen[p] = true
		}
		loc, _ := env["location"].(map[string]any)
		require.NotNil(t, loc, "compare preset must include location.normalized_region")
		require.NotEmpty(t, loc["normalized_region"])
	}
	require.GreaterOrEqual(t, len(providersSeen), 2, "fan-out should surface at least two providers; got %v", providersSeen)
}

func TestCompare_unsupportedKind(t *testing.T) {
	testutilSeededComputeVMCatalog(t)
	var stdout, stderr bytes.Buffer
	cmd := newRootCmd()
	cmd.SetOut(&stdout)
	cmd.SetErr(&stderr)
	cmd.SetArgs([]string{"compare", "--kind", "llm.text"})
	err := cmd.Execute()
	require.Error(t, err)
	require.Contains(t, stderr.String(), "llm.text")
}

func TestCompare_dryRun(t *testing.T) {
	testutilSeededComputeVMCatalog(t)
	var stdout bytes.Buffer
	cmd := newRootCmd()
	cmd.SetOut(&stdout)
	cmd.SetArgs([]string{"compare", "--kind", "compute.vm", "--vcpu", "2", "--dry-run"})
	require.NoError(t, cmd.Execute())
	require.Contains(t, stdout.String(), `"dry_run":true`)
	require.Contains(t, stdout.String(), `"command":"compare"`)
}
