package sku

import (
	"context"

	"github.com/spf13/cobra"

	"github.com/sofq/sku/internal/catalog"
	skuerrors "github.com/sofq/sku/internal/errors"
	"github.com/sofq/sku/internal/output"
)

const shardGCPMemorystore = "gcp-memorystore"

func newGCPMemorystoreCmd() *cobra.Command {
	c := &cobra.Command{Use: "memorystore", Short: "GCP Memorystore pricing"}
	c.AddCommand(newGCPMemorystorePriceCmd())
	c.AddCommand(newGCPMemorystoreListCmd())
	return c
}

type gcpMemorystoreFlags struct {
	instanceType string
	region       string
	engine       string
	commitment   string
}

func (f *gcpMemorystoreFlags) bind(c *cobra.Command) {
	c.Flags().StringVar(&f.instanceType, "instance-type", "",
		"Memorystore instance type, e.g. memorystore-redis-standard-16gb")
	c.Flags().StringVar(&f.region, "region", "", "GCP region (e.g. us-east1)")
	c.Flags().StringVar(&f.engine, "engine", "redis", "redis | memcached")
	c.Flags().StringVar(&f.commitment, "commitment", "on_demand", "on_demand")
}

func (f *gcpMemorystoreFlags) terms() catalog.Terms {
	return catalog.Terms{Commitment: f.commitment, Tenancy: f.engine}
}

func newGCPMemorystorePriceCmd() *cobra.Command {
	var f gcpMemorystoreFlags
	c := &cobra.Command{
		Use:   "price",
		Short: "Price one GCP Memorystore SKU",
		RunE:  func(cmd *cobra.Command, _ []string) error { return runGCPMemorystore(cmd, &f, true) },
	}
	f.bind(c)
	return c
}

func newGCPMemorystoreListCmd() *cobra.Command {
	var f gcpMemorystoreFlags
	c := &cobra.Command{
		Use:   "list",
		Short: "List GCP Memorystore SKUs matching filters",
		RunE:  func(cmd *cobra.Command, _ []string) error { return runGCPMemorystore(cmd, &f, false) },
	}
	f.bind(c)
	return c
}

func runGCPMemorystore(cmd *cobra.Command, f *gcpMemorystoreFlags, requireRegion bool) error {
	s := globalSettings(cmd)
	if f.instanceType == "" {
		e := skuerrors.Validation("flag_invalid", "instance-type", "",
			"pass --instance-type <type>, e.g. --instance-type memorystore-redis-standard-16gb")
		skuerrors.Write(cmd.ErrOrStderr(), e)
		return e
	}
	if requireRegion && f.region == "" {
		e := skuerrors.Validation("flag_invalid", "region", "", "pass --region <gcp-region>")
		skuerrors.Write(cmd.ErrOrStderr(), e)
		return e
	}
	if s.DryRun {
		return output.EmitDryRun(cmd.OutOrStdout(), output.DryRunPlan{
			Command: "gcp memorystore " + cmd.Use,
			ResolvedArgs: map[string]any{
				"instance_type": f.instanceType,
				"region":        f.region,
				"engine":        f.engine,
				"commitment":    f.commitment,
			},
			Shards: []string{shardGCPMemorystore},
			Preset: s.Preset,
		})
	}
	if err := ensureShard(cmd.Context(), shardGCPMemorystore, s.AutoFetch, cmd.ErrOrStderr()); err != nil {
		skuerrors.Write(cmd.ErrOrStderr(), err)
		return err
	}
	cat, err := catalog.Open(catalog.ShardPath(shardGCPMemorystore))
	if err != nil {
		e := &skuerrors.E{Code: skuerrors.CodeServer, Message: err.Error()}
		skuerrors.Write(cmd.ErrOrStderr(), e)
		return e
	}
	defer func() { _ = cat.Close() }()
	if stale := applyStaleGate(cmd, cat, shardGCPMemorystore, s); stale != nil {
		return stale
	}
	rows, err := cat.LookupCacheKV(context.Background(), catalog.CacheKVFilter{
		Provider:     "gcp",
		Service:      "memorystore",
		ResourceName: f.instanceType,
		Region:       f.region,
		Terms:        f.terms(),
	})
	if err != nil {
		return skuerrors.WriteWrap(cmd.ErrOrStderr(), skuerrors.CodeGeneric, "gcp memorystore %s: %w", cmd.Use, err)
	}
	if len(rows) == 0 {
		e := skuerrors.NotFound("gcp", "memorystore",
			map[string]any{"instance_type": f.instanceType, "region": f.region, "engine": f.engine},
			"Try `sku schema gcp memorystore` or drop --region for a list")
		skuerrors.Write(cmd.ErrOrStderr(), e)
		return e
	}
	return renderRows(cmd, rows, s)
}
