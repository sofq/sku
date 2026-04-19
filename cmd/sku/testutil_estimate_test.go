package sku

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/sofq/sku/internal/catalog/testdata"
)

// testutilSeededEstimateCatalog points SKU_DATA_DIR at a temp dir containing
// an aws-ec2.db built from the m3a.1 seed SQL. Returns the dir path.
func testutilSeededEstimateCatalog(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("SKU_DATA_DIR", dir)
	if err := testdata.BuildAWSShard(filepath.Join(dir, "aws-ec2.db")); err != nil {
		t.Fatalf("seed aws-ec2: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "aws-ec2.db")); err != nil {
		t.Fatalf("shard missing: %v", err)
	}
	return dir
}
