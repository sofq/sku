package sku

import (
	"bytes"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/require"
)

// newTestAPIMRoot builds a minimal root with persistent flags and the apim
// subcommand wired. This mirrors what azure.go will do once the command is
// registered there, and allows tests to exercise global flags (e.g. --dry-run)
// without depending on newRootCmd().
func newTestAPIMRoot() *cobra.Command {
	root := newRootCmd()
	// newRootCmd already calls newAzureCmd; if apim is not yet wired there,
	// find the azure sub-command and add apim directly in the test context.
	azureCmd, _, _ := root.Find([]string{"azure"})
	if azureCmd != nil && azureCmd != root {
		apimAlreadyWired := false
		for _, sub := range azureCmd.Commands() {
			if sub.Use == "apim" {
				apimAlreadyWired = true
				break
			}
		}
		if !apimAlreadyWired {
			azureCmd.AddCommand(newAzureAPIMCmd())
		}
	}
	return root
}

func TestAzureAPIMPriceCmd_RequiresTier(t *testing.T) {
	root := newTestAPIMRoot()
	root.SetArgs([]string{"azure", "apim", "price", "--region", "eastus"})
	var buf bytes.Buffer
	root.SetOut(&buf)
	root.SetErr(&buf)
	err := root.Execute()
	require.Error(t, err)
	require.Contains(t, buf.String(), "tier")
}

func TestAzureAPIMPriceCmd_RequiresRegion(t *testing.T) {
	root := newTestAPIMRoot()
	root.SetArgs([]string{"azure", "apim", "price", "--tier", "standard"})
	var buf bytes.Buffer
	root.SetOut(&buf)
	root.SetErr(&buf)
	err := root.Execute()
	require.Error(t, err)
	require.Contains(t, buf.String(), "region")
}

func TestAzureAPIMPriceCmd_InvalidTierReturnsError(t *testing.T) {
	root := newTestAPIMRoot()
	root.SetArgs([]string{"azure", "apim", "price", "--tier", "enterprise", "--region", "eastus"})
	var buf bytes.Buffer
	root.SetOut(&buf)
	root.SetErr(&buf)
	err := root.Execute()
	require.Error(t, err)
	require.Contains(t, buf.String(), "flag_invalid")
}

func TestAzureAPIMPriceCmd_DryRun(t *testing.T) {
	root := newTestAPIMRoot()
	root.SetArgs([]string{
		"azure", "apim", "price", "--tier", "consumption", "--region", "eastus", "--dry-run",
	})
	var buf bytes.Buffer
	root.SetOut(&buf)
	root.SetErr(&bytes.Buffer{})
	require.NoError(t, root.Execute())
	require.Contains(t, buf.String(), `"command":"azure apim price"`)
	require.Contains(t, buf.String(), `"shards":["azure-apim"]`)
	require.Contains(t, buf.String(), `"tier":"consumption"`)
}

func TestAzureAPIMListCmd_DryRunNoTierRequired(t *testing.T) {
	root := newTestAPIMRoot()
	root.SetArgs([]string{"azure", "apim", "list", "--dry-run"})
	var buf bytes.Buffer
	root.SetOut(&buf)
	root.SetErr(&bytes.Buffer{})
	require.NoError(t, root.Execute())
	require.Contains(t, buf.String(), `"command":"azure apim list"`)
	require.Contains(t, buf.String(), `"shards":["azure-apim"]`)
}

func TestAzureAPIMPriceCmd_AllValidTiers(t *testing.T) {
	tiers := []string{
		"consumption", "developer", "basic", "standard",
		"premium", "isolated", "premium-v2",
	}
	for _, tier := range tiers {
		t.Run(tier, func(t *testing.T) {
			root := newTestAPIMRoot()
			root.SetArgs([]string{
				"azure", "apim", "price", "--tier", tier, "--region", "eastus", "--dry-run",
			})
			var buf bytes.Buffer
			root.SetOut(&buf)
			root.SetErr(&bytes.Buffer{})
			require.NoError(t, root.Execute())
			require.Contains(t, buf.String(), `"azure-apim"`)
		})
	}
}
