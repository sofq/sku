package sku

import (
	"bytes"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
)

func runAzure(t *testing.T, args ...string) (stdout, stderr string, code int) {
	t.Helper()
	var out, errb bytes.Buffer
	cmd := newRootCmd()
	cmd.SetOut(&out)
	cmd.SetErr(&errb)
	cmd.SetArgs(args)
	if err := cmd.Execute(); err != nil {
		return out.String(), errb.String(), 1
	}
	return out.String(), errb.String(), 0
}

func TestAzureVMPrice_HappyPath(t *testing.T) {
	testutilSeededAzureCatalogM3b1(t)

	out, _, code := runAzure(t, "azure", "vm", "price",
		"--arm-sku-name", "Standard_D2_v3",
		"--region", "eastus",
		"--os", "linux",
	)
	require.Zero(t, code)
	lines := splitLines(out)
	require.Len(t, lines, 1)
	var env map[string]any
	require.NoError(t, json.Unmarshal([]byte(lines[0]), &env))
	require.Equal(t, "azure", env["provider"])
	require.Equal(t, "vm", env["service"])
	resource := env["resource"].(map[string]any)
	require.Equal(t, "Standard_D2_v3", resource["name"])
	require.Contains(t, out, "0.096")
}

func TestAzureVMPrice_RequiresInstance(t *testing.T) {
	testutilSeededAzureCatalogM3b1(t)

	_, stderr, code := runAzure(t, "azure", "vm", "price", "--region", "eastus")
	require.NotZero(t, code)
	require.Contains(t, stderr, "arm-sku-name")
}

func TestAzureVMPrice_NotFound_ReturnsExit3(t *testing.T) {
	testutilSeededAzureCatalogM3b1(t)

	_, stderr, code := runAzure(t, "azure", "vm", "price",
		"--arm-sku-name", "Standard_D99_vmx",
		"--region", "eastus",
		"--os", "linux",
	)
	require.NotZero(t, code)
	var env map[string]any
	require.NoError(t, json.Unmarshal([]byte(stderr), &env))
	body := env["error"].(map[string]any)
	require.Equal(t, "not_found", body["code"])
}

func TestAzureVMPrice_DryRun(t *testing.T) {
	t.Setenv("SKU_DATA_DIR", t.TempDir())
	out, _, code := runAzure(t, "azure", "vm", "price",
		"--arm-sku-name", "Standard_D2_v3",
		"--region", "eastus",
		"--dry-run",
	)
	require.Zero(t, code)
	require.Contains(t, out, `"dry_run":true`)
	require.Contains(t, out, `"command":"azure vm price"`)
	require.Contains(t, out, "azure-vm")
}
