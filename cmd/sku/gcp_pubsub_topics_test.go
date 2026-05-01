package sku

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/require"

	"github.com/sofq/sku/internal/catalog"
)

func testutilSeededGCPPubSubTopicsCatalog(t *testing.T) {
	t.Helper()
	dir := t.TempDir()
	ddl, err := os.ReadFile(filepath.Join("..", "..", "internal", "catalog", "testdata", "seed_gcp_pubsub_topics.sql"))
	require.NoError(t, err)
	require.NoError(t, catalog.BuildFromSQL(filepath.Join(dir, "gcp-pubsub-topics.db"), string(ddl)))
	t.Setenv("SKU_DATA_DIR", dir)
}

// newPubSubTopicsTestRoot builds a minimal root command with global flags and the
// pubsub-topics sub-tree, allowing tests to run without modifying gcp.go.
func newPubSubTopicsTestRoot() *cobra.Command {
	root := &cobra.Command{
		Use:          "sku",
		Short:        "test root",
		SilenceUsage: true, SilenceErrors: true,
		PersistentPreRunE: func(cmd *cobra.Command, _ []string) error {
			s, err := resolveSettings(cmd)
			if err != nil {
				return err
			}
			ctx := context.WithValue(cmd.Context(), settingsKey, s)
			cmd.SetContext(ctx)
			return nil
		},
	}
	pf := root.PersistentFlags()
	pf.String("profile", "", "")
	pf.String("preset", "", "")
	pf.String("jq", "", "")
	pf.String("fields", "", "")
	pf.Bool("include-raw", false, "")
	pf.Bool("include-aggregated", false, "")
	pf.Bool("pretty", false, "")
	pf.Bool("stale-ok", false, "")
	pf.Bool("auto-fetch", false, "")
	pf.Bool("dry-run", false, "")
	pf.Bool("verbose", false, "")
	pf.Bool("no-color", false, "")
	pf.Bool("json", false, "")
	pf.Bool("yaml", false, "")
	pf.Bool("toml", false, "")

	gcpParent := &cobra.Command{Use: "gcp", Short: "GCP subcommands"}
	gcpParent.AddCommand(newGCPPubSubTopicsCmd())
	root.AddCommand(gcpParent)
	return root
}

func TestGCPPubSubTopics_Price_RequiresMode(t *testing.T) {
	rc := newPubSubTopicsTestRoot()
	rc.SetArgs([]string{"gcp", "pubsub-topics", "price", "--mode", ""})
	var out, errb bytes.Buffer
	rc.SetOut(&out)
	rc.SetErr(&errb)
	err := rc.Execute()
	require.Error(t, err)
	require.Contains(t, errb.String(), "mode")
}

func TestGCPPubSubTopics_Price_DryRun(t *testing.T) {
	rc := newPubSubTopicsTestRoot()
	rc.SetArgs([]string{"gcp", "pubsub-topics", "price", "--mode", "throughput", "--dry-run"})
	var out bytes.Buffer
	rc.SetOut(&out)
	rc.SetErr(&bytes.Buffer{})
	require.NoError(t, rc.Execute())
	require.Contains(t, out.String(), `"command":"gcp pubsub-topics price"`)
	require.Contains(t, out.String(), `"shards":["gcp-pubsub-topics"]`)
}

func TestGCPPubSubTopics_Price_HappyPath(t *testing.T) {
	testutilSeededGCPPubSubTopicsCatalog(t)

	rc := newPubSubTopicsTestRoot()
	rc.SetArgs([]string{
		"gcp", "pubsub-topics", "price",
		"--mode", "throughput",
		"--region", "global",
		"--stale-ok",
	})
	var out, errb bytes.Buffer
	rc.SetOut(&out)
	rc.SetErr(&errb)
	if err := rc.Execute(); err != nil {
		t.Fatalf("execute: %v\nstderr=%s", err, errb.String())
	}
	require.Contains(t, out.String(), `"name":"throughput"`)
	require.Contains(t, out.String(), "gcp")
	require.Contains(t, out.String(), "pubsub-topics")
}

func TestGCPPubSubTopics_List_Throughput(t *testing.T) {
	testutilSeededGCPPubSubTopicsCatalog(t)

	rc := newPubSubTopicsTestRoot()
	rc.SetArgs([]string{"gcp", "pubsub-topics", "list", "--mode", "throughput", "--stale-ok"})
	var out bytes.Buffer
	rc.SetOut(&out)
	rc.SetErr(&bytes.Buffer{})
	require.NoError(t, rc.Execute())
	require.True(t, strings.Contains(out.String(), "throughput") || strings.Contains(out.String(), "pubsub"))
}

func TestGCPPubSubTopics_Price_NotFound(t *testing.T) {
	testutilSeededGCPPubSubTopicsCatalog(t)

	rc := newPubSubTopicsTestRoot()
	rc.SetArgs([]string{
		"gcp", "pubsub-topics", "price",
		"--mode", "nonexistent-mode",
		"--stale-ok",
	})
	var out, errb bytes.Buffer
	rc.SetOut(&out)
	rc.SetErr(&errb)
	err := rc.Execute()
	require.Error(t, err)
	require.Contains(t, errb.String(), "not_found")
}

func TestGCPPubSubTopics_List_DryRun(t *testing.T) {
	rc := newPubSubTopicsTestRoot()
	rc.SetArgs([]string{"gcp", "pubsub-topics", "list", "--mode", "throughput", "--dry-run"})
	var out bytes.Buffer
	rc.SetOut(&out)
	rc.SetErr(&bytes.Buffer{})
	require.NoError(t, rc.Execute())
	require.Contains(t, out.String(), shardGCPPubSubTopics)
}

func TestGCPPubSubTopics_CommandExists(t *testing.T) {
	gcpCmd := newGCPPubSubTopicsCmd()
	require.Equal(t, "pubsub-topics", gcpCmd.Use)
	uses := make([]string, 0)
	for _, sub := range gcpCmd.Commands() {
		uses = append(uses, sub.Use)
	}
	require.Contains(t, uses, "price")
	require.Contains(t, uses, "list")
}
