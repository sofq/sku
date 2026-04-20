package sku

import (
	"context"
	"testing"

	"github.com/sofq/sku/internal/batch"
)

func TestHandleAWSEC2_registered(t *testing.T) {
	for _, name := range []string{"aws ec2 price", "aws ec2 list"} {
		if _, ok := batch.Lookup(name); !ok {
			t.Fatalf("%q not registered", name)
		}
	}
}

func TestHandleAWSEC2Price_notFoundWithoutShard(t *testing.T) {
	t.Setenv("SKU_DATA_DIR", t.TempDir())
	args := map[string]any{"instance_type": "m5.large", "region": "us-east-1"}
	_, err := handleAWSEC2Price(context.Background(), args, batch.Env{Settings: &batch.Settings{}})
	if err == nil {
		t.Fatal("expected error when shard missing")
	}
}
