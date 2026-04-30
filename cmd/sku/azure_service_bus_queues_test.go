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

func testutilSeededAzureServiceBusQueuesCatalog(t *testing.T) {
	t.Helper()
	dir := t.TempDir()
	ddl, err := os.ReadFile(filepath.Join("..", "..", "internal", "catalog", "testdata", "seed_azure_service_bus_queues.sql"))
	require.NoError(t, err)
	require.NoError(t, catalog.BuildFromSQL(filepath.Join(dir, "azure-service-bus-queues.db"), string(ddl)))
	t.Setenv("SKU_DATA_DIR", dir)
}

func TestAzureServiceBusQueues_Price_SeededEastUS(t *testing.T) {
	testutilSeededAzureServiceBusQueuesCatalog(t)

	rc := newRootCmd()
	rc.SetArgs([]string{"azure", "service-bus-queues", "price", "--tier", "standard", "--region", "eastus", "--stale-ok"})
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

func TestAzureServiceBusQueues_Price_MissingTier(t *testing.T) {
	rc := newRootCmd()
	rc.SetArgs([]string{"azure", "service-bus-queues", "price", "--region", "eastus"})
	var out, errb bytes.Buffer
	rc.SetOut(&out)
	rc.SetErr(&errb)
	err := rc.Execute()
	if err == nil {
		t.Fatal("want error for missing --tier")
	}
}

func TestAzureServiceBusQueues_Price_InvalidTier(t *testing.T) {
	rc := newRootCmd()
	rc.SetArgs([]string{"azure", "service-bus-queues", "price", "--tier", "basic", "--region", "eastus"})
	var out, errb bytes.Buffer
	rc.SetOut(&out)
	rc.SetErr(&errb)
	err := rc.Execute()
	if err == nil {
		t.Fatal("want error for invalid tier 'basic'")
	}
}

func TestAzureServiceBusQueues_List_StandardAcrossRegions(t *testing.T) {
	testutilSeededAzureServiceBusQueuesCatalog(t)
	rc := newRootCmd()
	rc.SetArgs([]string{"azure", "service-bus-queues", "list", "--tier", "standard", "--stale-ok"})
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

func TestAzureServiceBusQueues_Price_Premium(t *testing.T) {
	testutilSeededAzureServiceBusQueuesCatalog(t)

	rc := newRootCmd()
	rc.SetArgs([]string{"azure", "service-bus-queues", "price", "--tier", "premium", "--region", "eastus", "--stale-ok"})
	var out, errb bytes.Buffer
	rc.SetOut(&out)
	rc.SetErr(&errb)
	if err := rc.Execute(); err != nil {
		t.Fatalf("execute: %v\nstderr=%s", err, errb.String())
	}
	if !strings.Contains(out.String(), `"name":"premium"`) {
		t.Fatalf("stdout missing premium resource name: %s", out.String())
	}
}
