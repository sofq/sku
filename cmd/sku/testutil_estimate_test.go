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

// testutilSeededEstimateCatalogS3 seeds both aws-ec2.db and aws-s3.db under
// SKU_DATA_DIR so a single test process can exercise compute.vm and
// storage.object estimators without re-seeding.
func testutilSeededEstimateCatalogS3(t *testing.T) string {
	t.Helper()
	dir := testutilSeededEstimateCatalog(t)
	if err := testdata.BuildAWSS3Shard(filepath.Join(dir, "aws-s3.db")); err != nil {
		t.Fatalf("seed aws-s3: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "aws-s3.db")); err != nil {
		t.Fatalf("aws-s3 shard missing: %v", err)
	}
	return dir
}
