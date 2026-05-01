package sku

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/require"

	"github.com/sofq/sku/internal/catalog"
)

func testutilSeededAzureEventHubsCatalog(t *testing.T) {
	t.Helper()
	dir := t.TempDir()
	ddl, err := os.ReadFile(filepath.Join("..", "..", "internal", "catalog", "testdata", "seed_azure_event_hubs.sql"))
	require.NoError(t, err)
	require.NoError(t, catalog.BuildFromSQL(filepath.Join(dir, "azure-event-hubs.db"), string(ddl)))
	t.Setenv("SKU_DATA_DIR", dir)
}

// newTestAzureEventHubsRoot returns the root command for event-hubs tests.
// azure.go already wires newAzureEventHubsCmd() into the azure subcommand, so
// no extra AddCommand is needed here. The helper exists to give tests a
// consistent entry point without duplicating that wiring.
func newTestAzureEventHubsRoot() *cobra.Command {
	return newRootCmd()
}

func TestAzureEventHubs_Price_SeededEastUS(t *testing.T) {
	testutilSeededAzureEventHubsCatalog(t)

	root := newTestAzureEventHubsRoot()
	root.SetArgs([]string{"azure", "event-hubs", "price", "--tier", "standard", "--region", "eastus", "--stale-ok"})
	var out, errb bytes.Buffer
	root.SetOut(&out)
	root.SetErr(&errb)
	if err := root.Execute(); err != nil {
		t.Fatalf("execute: %v\nstderr=%s", err, errb.String())
	}
	if !strings.Contains(out.String(), `"name":"standard"`) {
		t.Fatalf("stdout missing standard resource name: %s", out.String())
	}
}

func TestAzureEventHubs_Price_MissingTier(t *testing.T) {
	root := newTestAzureEventHubsRoot()
	root.SetArgs([]string{"azure", "event-hubs", "price", "--region", "eastus"})
	var out, errb bytes.Buffer
	root.SetOut(&out)
	root.SetErr(&errb)
	err := root.Execute()
	if err == nil {
		t.Fatal("want error for missing --tier")
	}
}

func TestAzureEventHubs_List_StandardAcrossRegions(t *testing.T) {
	testutilSeededAzureEventHubsCatalog(t)
	root := newTestAzureEventHubsRoot()
	root.SetArgs([]string{"azure", "event-hubs", "list", "--tier", "standard", "--stale-ok"})
	var out bytes.Buffer
	root.SetOut(&out)
	root.SetErr(&bytes.Buffer{})
	if err := root.Execute(); err != nil {
		t.Fatalf("execute: %v", err)
	}
	if !strings.Contains(out.String(), `"name":"standard"`) {
		t.Fatalf("stdout missing standard: %s", out.String())
	}
}
