package sku

import (
	"context"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/sofq/sku/internal/catalog"
	skuerrors "github.com/sofq/sku/internal/errors"
	"github.com/sofq/sku/internal/output"
)

type searchFlags struct {
	provider     string
	service      string
	kind         string
	resourceName string
	region       string
	minVCPU      int64
	minMemoryGB  float64
	maxPrice     float64
	sort         string
	limit        int
}

func (f *searchFlags) bind(c *cobra.Command) {
	c.Flags().StringVar(&f.provider, "provider", "", "cloud provider (aws | azure | gcp)")
	c.Flags().StringVar(&f.service, "service", "", "service within provider (ec2 | rds | ...)")
	c.Flags().StringVar(&f.kind, "kind", "", "resource kind (compute.vm | db.relational | ...)")
	c.Flags().StringVar(&f.resourceName, "resource-name", "", "exact resource name (e.g. m5.large)")
	c.Flags().StringVar(&f.region, "region", "", "provider region (e.g. us-east-1)")
	c.Flags().Int64Var(&f.minVCPU, "min-vcpu", 0, "minimum vCPU count")
	c.Flags().Float64Var(&f.minMemoryGB, "min-memory", 0, "minimum memory in GB")
	c.Flags().Float64Var(&f.maxPrice, "max-price", 0, "maximum unit price across any dimension")
	c.Flags().StringVar(&f.sort, "sort", "", "sort column: resource_name | price | vcpu | memory")
	c.Flags().IntVar(&f.limit, "limit", 50, "maximum rows to return (0 = unlimited)")
}

func newSearchCmd() *cobra.Command {
	var f searchFlags
	c := &cobra.Command{
		Use:   "search",
		Short: "List SKUs matching filters within a single shard",
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runSearch(cmd, &f)
		},
	}
	f.bind(c)
	return c
}

func runSearch(cmd *cobra.Command, f *searchFlags) error {
	if f.provider == "" {
		e := skuerrors.Validation("flag_invalid", "provider", "",
			"pass --provider <aws|azure|gcp>; multi-provider search arrives in M4.2")
		skuerrors.Write(cmd.ErrOrStderr(), e)
		return e
	}
	if f.service == "" {
		e := skuerrors.Validation("flag_invalid", "service", "",
			"pass --service <service>, e.g. --service ec2")
		skuerrors.Write(cmd.ErrOrStderr(), e)
		return e
	}
	if f.maxPrice < 0 {
		e := skuerrors.Validation("flag_invalid", "max-price", "",
			"--max-price must be non-negative")
		skuerrors.Write(cmd.ErrOrStderr(), e)
		return e
	}
	if f.minVCPU < 0 {
		e := skuerrors.Validation("flag_invalid", "min-vcpu", "",
			"--min-vcpu must be non-negative")
		skuerrors.Write(cmd.ErrOrStderr(), e)
		return e
	}
	if f.minMemoryGB < 0 {
		e := skuerrors.Validation("flag_invalid", "min-memory", "",
			"--min-memory must be non-negative")
		skuerrors.Write(cmd.ErrOrStderr(), e)
		return e
	}

	s := globalSettings(cmd)
	shard := f.provider + "-" + f.service

	if s.DryRun {
		return output.EmitDryRun(cmd.OutOrStdout(), output.DryRunPlan{
			Command: "search",
			ResolvedArgs: map[string]any{
				"provider":      f.provider,
				"service":       f.service,
				"kind":          f.kind,
				"resource_name": f.resourceName,
				"region":        f.region,
				"min_vcpu":      f.minVCPU,
				"min_memory_gb": f.minMemoryGB,
				"max_price":     f.maxPrice,
				"sort":          f.sort,
				"limit":         f.limit,
			},
			Shards: []string{shard},
			Preset: s.Preset,
		})
	}

	shardPath := catalog.ShardPath(shard)
	if _, err := os.Stat(shardPath); err != nil {
		e := shardMissingErr(shard)
		skuerrors.Write(cmd.ErrOrStderr(), e)
		return e
	}
	cat, err := catalog.Open(shardPath)
	if err != nil {
		e := &skuerrors.E{Code: skuerrors.CodeServer, Message: err.Error()}
		skuerrors.Write(cmd.ErrOrStderr(), e)
		return e
	}
	defer func() { _ = cat.Close() }()

	if stale := applyStaleGate(cmd, cat, shard, s); stale != nil {
		return stale
	}

	rows, err := cat.Search(context.Background(), catalog.SearchFilter{
		Provider:     f.provider,
		Service:      f.service,
		Kind:         f.kind,
		ResourceName: f.resourceName,
		Region:       f.region,
		MinVCPU:      f.minVCPU,
		MinMemoryGB:  f.minMemoryGB,
		MaxPrice:     f.maxPrice,
		Sort:         f.sort,
		Limit:        f.limit,
	})
	if err != nil {
		// Builder errors (unknown --sort) map to validation; DB errors to server.
		if strings.Contains(err.Error(), "--sort") {
			e := skuerrors.Validation("flag_invalid", "sort", f.sort,
				"allowed: resource_name | price | vcpu | memory")
			skuerrors.Write(cmd.ErrOrStderr(), e)
			return e
		}
		return skuerrors.WriteWrap(cmd.ErrOrStderr(), skuerrors.CodeGeneric, "search: %w", err)
	}
	if len(rows) == 0 {
		e := skuerrors.NotFound(f.provider, f.service,
			map[string]any{
				"kind":          f.kind,
				"region":        f.region,
				"resource_name": f.resourceName,
				"min_vcpu":      f.minVCPU,
				"min_memory_gb": f.minMemoryGB,
				"max_price":     f.maxPrice,
			},
			"Try widening filters or run `sku schema "+f.provider+" "+f.service+"`")
		skuerrors.Write(cmd.ErrOrStderr(), e)
		return e
	}
	return renderRows(cmd, rows, s)
}
