package sku

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/require"

	"github.com/sofq/sku/internal/catalog"
)

// testutilSeededGCPFirestoreCatalog creates a SKU_DATA_DIR containing
// gcp-firestore.db populated from seed_gcp_firestore.sql.
func testutilSeededGCPFirestoreCatalog(t *testing.T) {
	t.Helper()
	dir := t.TempDir()
	ddl, err := os.ReadFile(filepath.Join("..", "..", "internal", "catalog", "testdata", "seed_gcp_firestore.sql"))
	require.NoError(t, err)
	require.NoError(t, catalog.BuildFromSQL(filepath.Join(dir, "gcp-firestore.db"), string(ddl)))
	t.Setenv("SKU_DATA_DIR", dir)
}

// newTestGCPFirestoreRoot returns a root command with only the GCP firestore
// sub-tree wired in. Used by tests when gcp.go has not yet registered
// newGCPFirestoreCmd (it is wired separately).
func newTestGCPFirestoreRoot() *cobra.Command {
	root := newRootCmd()
	// If the firestore command is not yet wired via gcp.go, add it here
	// so tests can exercise the full flag/handler surface.
	gcpCmd := findSubCmd(root, "gcp")
	if gcpCmd != nil && findSubCmd(gcpCmd, "firestore") == nil {
		gcpCmd.AddCommand(newGCPFirestoreCmd())
	}
	return root
}

// findSubCmd finds a named sub-command among cmd's children, or nil.
func findSubCmd(cmd *cobra.Command, name string) *cobra.Command {
	for _, sub := range cmd.Commands() {
		if sub.Name() == name {
			return sub
		}
	}
	return nil
}

func TestGCPFirestorePriceCmd_RequiresRegion(t *testing.T) {
	cmd := newTestGCPFirestoreRoot()
	cmd.SetArgs([]string{"gcp", "firestore", "price", "--mode", "native"})
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	err := cmd.Execute()
	require.Error(t, err)
	require.Contains(t, buf.String(), "region")
}

func TestGCPFirestorePriceCmd_DryRun(t *testing.T) {
	cmd := newTestGCPFirestoreRoot()
	cmd.SetArgs([]string{
		"gcp", "firestore", "price",
		"--mode", "native",
		"--region", "us-east1",
		"--dry-run",
	})
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	require.NoError(t, cmd.Execute())
	require.Contains(t, buf.String(), `"command":"gcp firestore price"`)
	require.Contains(t, buf.String(), `"shards":["gcp-firestore"]`)
}

func TestGCPFirestorePrice_HappyPath(t *testing.T) {
	testutilSeededGCPFirestoreCatalog(t)

	cmd := newTestGCPFirestoreRoot()
	cmd.SetArgs([]string{
		"gcp", "firestore", "price",
		"--mode", "native",
		"--region", "us-east1",
		"--stale-ok",
	})
	var out, errb bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&errb)
	require.NoError(t, cmd.Execute())

	lines := splitLines(out.String())
	require.Len(t, lines, 1)
	var env map[string]any
	require.NoError(t, json.Unmarshal([]byte(lines[0]), &env))
	require.Equal(t, "gcp", env["provider"])
	require.Equal(t, "firestore", env["service"])
	resource, ok := env["resource"].(map[string]any)
	require.True(t, ok, "expected resource object in output")
	require.Equal(t, "native", resource["name"])
}

func TestGCPFirestorePrice_NotFound(t *testing.T) {
	testutilSeededGCPFirestoreCatalog(t)

	cmd := newTestGCPFirestoreRoot()
	cmd.SetArgs([]string{
		"gcp", "firestore", "price",
		"--mode", "native",
		"--region", "europe-west1",
		"--stale-ok",
	})
	var out, errb bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&errb)
	err := cmd.Execute()
	require.Error(t, err)
	require.Contains(t, errb.String(), "not_found")
}
