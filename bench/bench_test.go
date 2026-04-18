package bench

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/sofq/sku/internal/catalog"
	"github.com/sofq/sku/internal/output"
)

// BenchmarkPointLookup_Warm measures in-process point-lookup latency with
// the catalog already open — the §5 "warm" number.
func BenchmarkPointLookup_Warm(b *testing.B) {
	path := os.Getenv("SKU_BENCH_SHARD")
	if path == "" {
		b.Skip("SKU_BENCH_SHARD not set; run via `make bench`")
	}
	cat, err := catalog.Open(path)
	if err != nil {
		b.Fatal(err)
	}
	b.Cleanup(func() { _ = cat.Close() })

	filter := catalog.LLMFilter{Model: "anthropic/claude-opus-4.6"}
	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		rows, err := cat.LookupLLM(ctx, filter)
		if err != nil {
			b.Fatal(err)
		}
		var buf bytes.Buffer
		for _, r := range rows {
			env := output.Render(r, output.PresetAgent)
			if err := output.EncodeEnvelope(&buf, env, false); err != nil {
				b.Fatal(err)
			}
		}
		_ = json.Valid(buf.Bytes())
	}
}

// BenchmarkEC2PointLookup_Warm measures an in-process EC2 point lookup
// with the catalog already open. Absolute numbers are only meaningful
// against a production-scale shard; m3a.3 wires that in CI.
func BenchmarkEC2PointLookup_Warm(b *testing.B) {
	path := os.Getenv("SKU_BENCH_EC2_SHARD")
	if path == "" {
		b.Skip("SKU_BENCH_EC2_SHARD not set")
	}
	cat, err := catalog.Open(path)
	if err != nil {
		b.Fatal(err)
	}
	b.Cleanup(func() { _ = cat.Close() })
	ctx := context.Background()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		rows, err := cat.LookupVM(ctx, catalog.VMFilter{
			Provider: "aws", Service: "ec2",
			InstanceType: "m5.large", Region: "us-east-1",
			Terms: catalog.Terms{Commitment: "on_demand", Tenancy: "shared", OS: "linux"},
		})
		if err != nil || len(rows) != 1 {
			b.Fatalf("unexpected: %v %d", err, len(rows))
		}
	}
}

// BenchmarkEC2PointLookup_Cold re-opens the shard per iteration to measure
// the cold-start path (the number §5's "<60 ms cold" target tracks).
func BenchmarkEC2PointLookup_Cold(b *testing.B) {
	path := os.Getenv("SKU_BENCH_EC2_SHARD")
	if path == "" {
		b.Skip("SKU_BENCH_EC2_SHARD not set")
	}
	ctx := context.Background()
	for i := 0; i < b.N; i++ {
		b.StopTimer()
		cat, err := catalog.Open(path)
		if err != nil {
			b.Fatal(err)
		}
		b.StartTimer()
		rows, err := cat.LookupVM(ctx, catalog.VMFilter{
			Provider: "aws", Service: "ec2",
			InstanceType: "m5.large", Region: "us-east-1",
			Terms: catalog.Terms{Commitment: "on_demand", Tenancy: "shared", OS: "linux"},
		})
		if err != nil || len(rows) != 1 {
			b.Fatalf("unexpected: %v %d", err, len(rows))
		}
		b.StopTimer()
		_ = cat.Close()
		b.StartTimer()
	}
}

// BenchmarkPointLookup_Cold measures the whole process-startup path: exec of
// the real binary + shard open + lookup + render + exit. This is the number
// that matches §5 "<60 ms cold".
func BenchmarkPointLookup_Cold(b *testing.B) {
	shard := os.Getenv("SKU_BENCH_SHARD")
	if shard == "" {
		b.Skip("SKU_BENCH_SHARD not set")
	}
	// Find the binary — bench target builds it at ../bin/sku.
	bin, err := filepath.Abs(filepath.Join("..", "bin", "sku"))
	if err != nil {
		b.Fatal(err)
	}
	if _, err := os.Stat(bin); err != nil {
		b.Skipf("bin/sku missing: %v (run `make build` first)", err)
	}
	dataDir := filepath.Dir(shard)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		cmd := exec.Command(bin, "llm", "price", "--model", "anthropic/claude-opus-4.6") //nolint:gosec // G204: bin is a validated absolute path checked with os.Stat above
		cmd.Env = append(os.Environ(), "SKU_DATA_DIR="+dataDir)
		if err := cmd.Run(); err != nil {
			b.Fatal(err)
		}
	}
}
