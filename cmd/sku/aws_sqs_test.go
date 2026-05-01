package sku

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/sofq/sku/internal/catalog"
)

func testutilSeededAWSSQSCatalog(t *testing.T) {
	t.Helper()
	dir := t.TempDir()
	ddl, err := os.ReadFile(filepath.Join("..", "..", "internal", "catalog", "testdata", "seed_messaging_queue.sql"))
	require.NoError(t, err)
	require.NoError(t, catalog.BuildFromSQL(filepath.Join(dir, "aws-sqs.db"), string(ddl)))
	t.Setenv("SKU_DATA_DIR", dir)
}

func TestAWSSQS_Price_SeededUSE1(t *testing.T) {
	testutilSeededAWSSQSCatalog(t)

	rc := newRootCmd()
	rc.SetArgs([]string{"aws", "sqs", "price", "--queue-type", "standard", "--region", "us-east-1", "--stale-ok"})
	var out, errb bytes.Buffer
	rc.SetOut(&out)
	rc.SetErr(&errb)
	if err := rc.Execute(); err != nil {
		t.Fatalf("execute: %v\nstderr=%s", err, errb.String())
	}
	if !strings.Contains(out.String(), `"name":"standard"`) {
		t.Fatalf("stdout missing standard resource name: %s", out.String())
	}
}

func TestAWSSQS_Price_MissingQueueType(t *testing.T) {
	rc := newRootCmd()
	rc.SetArgs([]string{"aws", "sqs", "price", "--region", "us-east-1"})
	var out, errb bytes.Buffer
	rc.SetOut(&out)
	rc.SetErr(&errb)
	err := rc.Execute()
	if err == nil {
		t.Fatal("want error for missing --queue-type")
	}
}

func TestAWSSQS_List_StandardAcrossRegions(t *testing.T) {
	testutilSeededAWSSQSCatalog(t)
	rc := newRootCmd()
	rc.SetArgs([]string{"aws", "sqs", "list", "--queue-type", "standard", "--stale-ok"})
	var out bytes.Buffer
	rc.SetOut(&out)
	rc.SetErr(&bytes.Buffer{})
	if err := rc.Execute(); err != nil {
		t.Fatalf("execute: %v", err)
	}
	if !strings.Contains(out.String(), `"name":"standard"`) {
		t.Fatalf("stdout missing standard: %s", out.String())
	}
}
