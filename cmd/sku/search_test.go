package sku

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func runSearchCmd(t *testing.T, args ...string) (stdout, stderr string, err error) {
	t.Helper()
	testutilSeededSearchCatalog(t)
	root := newRootCmd()
	var out, errb bytes.Buffer
	root.SetOut(&out)
	root.SetErr(&errb)
	root.SetArgs(append([]string{"search"}, args...))
	err = root.Execute()
	return out.String(), errb.String(), err
}

func TestSearch_Cmd_HappyPath(t *testing.T) {
	stdout, _, err := runSearchCmd(t,
		"--provider", "aws", "--service", "ec2",
		"--sort", "price", "--limit", "2",
		"--preset", "agent")
	require.NoError(t, err)

	// Output is one JSON object per line (agent preset, default compact).
	lines := strings.Split(strings.TrimSpace(stdout), "\n")
	require.Len(t, lines, 2)

	var first map[string]any
	require.NoError(t, json.Unmarshal([]byte(lines[0]), &first))
	// cheapest row is t3.medium.
	resource := first["resource"].(map[string]any)
	require.Equal(t, "t3.medium", resource["name"])
}

func TestSearch_Cmd_MissingProvider_ReturnsValidation(t *testing.T) {
	_, stderr, err := runSearchCmd(t, "--service", "ec2")
	require.Error(t, err)
	require.Contains(t, stderr, `"code":"validation"`)
	require.Contains(t, stderr, `"flag":"provider"`)
}

func TestSearch_Cmd_NoMatch_ReturnsNotFound(t *testing.T) {
	_, stderr, err := runSearchCmd(t,
		"--provider", "aws", "--service", "ec2",
		"--min-vcpu", "9999")
	require.Error(t, err)
	require.Contains(t, stderr, `"code":"not_found"`)
}

func TestSearch_Cmd_UnknownSort_ReturnsValidation(t *testing.T) {
	_, stderr, err := runSearchCmd(t,
		"--provider", "aws", "--service", "ec2",
		"--sort", "hostname")
	require.Error(t, err)
	require.Contains(t, stderr, `"code":"validation"`)
	require.Contains(t, stderr, `"flag":"sort"`)
}

func TestSearch_Cmd_DryRun_EmitsPlan(t *testing.T) {
	stdout, _, err := runSearchCmd(t,
		"--provider", "aws", "--service", "ec2",
		"--dry-run")
	require.NoError(t, err)
	var plan map[string]any
	require.NoError(t, json.Unmarshal([]byte(stdout), &plan))
	require.Equal(t, true, plan["dry_run"])
	require.Equal(t, "search", plan["command"])
	shards := plan["shards"].([]any)
	require.Equal(t, "aws-ec2", shards[0])
}

func TestSearch_Cmd_MissingShard_ReturnsNotFound(t *testing.T) {
	t.Setenv("SKU_DATA_DIR", t.TempDir()) // empty — no aws-ec2.db here
	root := newRootCmd()
	var errb bytes.Buffer
	root.SetErr(&errb)
	root.SetOut(new(bytes.Buffer))
	root.SetArgs([]string{"search", "--provider", "aws", "--service", "ec2"})
	err := root.Execute()
	require.Error(t, err)
	require.Contains(t, errb.String(), `"code":"not_found"`)
	require.Contains(t, errb.String(), "aws-ec2 shard not installed")
}
