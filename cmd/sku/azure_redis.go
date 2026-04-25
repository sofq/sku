package sku

import (
	"context"

	"github.com/spf13/cobra"

	"github.com/sofq/sku/internal/catalog"
	skuerrors "github.com/sofq/sku/internal/errors"
	"github.com/sofq/sku/internal/output"
)

const shardAzureRedis = "azure-redis"

var azureRedisTierDisplay = map[string]string{
	"basic":      "Basic",
	"standard":   "Standard",
	"premium":    "Premium",
	"enterprise": "Enterprise",
}

func newAzureRedisCmd() *cobra.Command {
	c := &cobra.Command{Use: "redis", Short: "Azure Cache for Redis pricing"}
	c.AddCommand(newAzureRedisPriceCmd())
	c.AddCommand(newAzureRedisListCmd())
	return c
}

type azureRedisFlags struct {
	tier   string
	size   string
	region string
}

func (f *azureRedisFlags) bind(c *cobra.Command) {
	c.Flags().StringVar(&f.tier, "tier", "standard", "basic | standard | premium | enterprise")
	c.Flags().StringVar(&f.size, "size", "", "Tier size: C0..C6 | P1..P5 | E5..E100")
	c.Flags().StringVar(&f.region, "region", "", "Azure region")
}

func (f *azureRedisFlags) resourceName() string {
	if f.size == "" {
		return ""
	}
	display, ok := azureRedisTierDisplay[f.tier]
	if !ok {
		return ""
	}
	return display + " " + f.size
}

func newAzureRedisPriceCmd() *cobra.Command {
	var f azureRedisFlags
	c := &cobra.Command{Use: "price",
		Short: "Price one Azure Redis SKU",
		RunE:  func(cmd *cobra.Command, _ []string) error { return runAzureRedis(cmd, &f, true) }}
	f.bind(c)
	return c
}

func newAzureRedisListCmd() *cobra.Command {
	var f azureRedisFlags
	c := &cobra.Command{Use: "list",
		Short: "List Azure Redis SKUs matching filters",
		RunE:  func(cmd *cobra.Command, _ []string) error { return runAzureRedis(cmd, &f, false) }}
	f.bind(c)
	return c
}

func runAzureRedis(cmd *cobra.Command, f *azureRedisFlags, requireRegion bool) error {
	s := globalSettings(cmd)
	if f.size == "" {
		e := skuerrors.Validation("flag_invalid", "size", "",
			"pass --size, e.g. --size C1 for Standard, P1 for Premium, E5 for Enterprise")
		skuerrors.Write(cmd.ErrOrStderr(), e)
		return e
	}
	if requireRegion && f.region == "" {
		e := skuerrors.Validation("flag_invalid", "region", "", "pass --region <azure-region>")
		skuerrors.Write(cmd.ErrOrStderr(), e)
		return e
	}
	if s.DryRun {
		return output.EmitDryRun(cmd.OutOrStdout(), output.DryRunPlan{
			Command: "azure redis " + cmd.Use,
			ResolvedArgs: map[string]any{
				"tier": f.tier, "size": f.size, "region": f.region,
			},
			Shards: []string{shardAzureRedis}, Preset: s.Preset,
		})
	}
	if err := ensureShard(cmd.Context(), shardAzureRedis, s.AutoFetch, cmd.ErrOrStderr()); err != nil {
		skuerrors.Write(cmd.ErrOrStderr(), err)
		return err
	}
	cat, err := catalog.Open(catalog.ShardPath(shardAzureRedis))
	if err != nil {
		e := &skuerrors.E{Code: skuerrors.CodeServer, Message: err.Error()}
		skuerrors.Write(cmd.ErrOrStderr(), e)
		return e
	}
	defer func() { _ = cat.Close() }()
	if stale := applyStaleGate(cmd, cat, shardAzureRedis, s); stale != nil {
		return stale
	}
	rows, err := cat.LookupCacheKV(context.Background(), catalog.CacheKVFilter{
		Provider:     "azure",
		Service:      "redis",
		ResourceName: f.resourceName(),
		Region:       f.region,
		Terms:        catalog.Terms{Commitment: "on_demand", Tenancy: "redis", OS: f.tier},
	})
	if err != nil {
		return skuerrors.WriteWrap(cmd.ErrOrStderr(), skuerrors.CodeGeneric, "azure redis %s: %w", cmd.Use, err)
	}
	if len(rows) == 0 {
		e := skuerrors.NotFound("azure", "redis",
			map[string]any{"tier": f.tier, "size": f.size, "region": f.region},
			"Try `sku schema azure redis` or drop --region for a list")
		skuerrors.Write(cmd.ErrOrStderr(), e)
		return e
	}
	return renderRows(cmd, rows, s)
}
