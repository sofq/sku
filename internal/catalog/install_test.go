package catalog

import (
	"os"
	"path/filepath"
	"sort"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestInstalledShards_emptyDir(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("SKU_DATA_DIR", dir)
	got, err := InstalledShards()
	require.NoError(t, err)
	require.Empty(t, got)
}

func TestInstalledShards_filtersAndSorts(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("SKU_DATA_DIR", dir)
	for _, name := range []string{"gcp-gce.db", "aws-ec2.db", "README.md", "azure-vm.db", "stale.db-journal"} {
		require.NoError(t, os.WriteFile(filepath.Join(dir, name), []byte{}, 0o600))
	}
	got, err := InstalledShards()
	require.NoError(t, err)
	want := []string{"aws-ec2", "azure-vm", "gcp-gce"}
	sort.Strings(got)
	require.Equal(t, want, got)
}
