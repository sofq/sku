package sku

import (
	"context"

	"github.com/spf13/cobra"

	"github.com/sofq/sku/internal/catalog"
	skuerrors "github.com/sofq/sku/internal/errors"
	"github.com/sofq/sku/internal/output"
)

const shardAWSElastiCache = "aws-elasticache"

func newAWSElastiCacheCmd() *cobra.Command {
	c := &cobra.Command{Use: "elasticache", Short: "AWS ElastiCache pricing"}
	c.AddCommand(newAWSElastiCachePriceCmd())
	c.AddCommand(newAWSElastiCacheListCmd())
	return c
}

type elastiCacheFlags struct {
	instanceType string
	region       string
	engine       string
	commitment   string
}

func (f *elastiCacheFlags) bind(c *cobra.Command) {
	c.Flags().StringVar(&f.instanceType, "instance-type", "", "ElastiCache instance type, e.g. cache.r6g.large")
	c.Flags().StringVar(&f.region, "region", "", "AWS region")
	c.Flags().StringVar(&f.engine, "engine", "redis", "redis | memcached")
	c.Flags().StringVar(&f.commitment, "commitment", "on_demand", "on_demand")
}

func (f *elastiCacheFlags) terms() catalog.Terms {
	return catalog.Terms{Commitment: f.commitment, Tenancy: f.engine}
}

func newAWSElastiCachePriceCmd() *cobra.Command {
	var f elastiCacheFlags
	c := &cobra.Command{
		Use:   "price",
		Short: "Price one ElastiCache SKU",
		RunE:  func(cmd *cobra.Command, _ []string) error { return runAWSElastiCache(cmd, &f, true) },
	}
	f.bind(c)
	return c
}

func newAWSElastiCacheListCmd() *cobra.Command {
	var f elastiCacheFlags
	c := &cobra.Command{
		Use:   "list",
		Short: "List ElastiCache SKUs matching filters",
		RunE:  func(cmd *cobra.Command, _ []string) error { return runAWSElastiCache(cmd, &f, false) },
	}
	f.bind(c)
	return c
}

func runAWSElastiCache(cmd *cobra.Command, f *elastiCacheFlags, requireRegion bool) error {
	s := globalSettings(cmd)
	if f.instanceType == "" {
		e := skuerrors.Validation("flag_invalid", "instance-type", "",
			"pass --instance-type <type>, e.g. --instance-type cache.r6g.large")
		skuerrors.Write(cmd.ErrOrStderr(), e)
		return e
	}
	if requireRegion && f.region == "" {
		e := skuerrors.Validation("flag_invalid", "region", "", "pass --region <aws-region>")
		skuerrors.Write(cmd.ErrOrStderr(), e)
		return e
	}
	if s.DryRun {
		return output.EmitDryRun(cmd.OutOrStdout(), output.DryRunPlan{
			Command: "aws elasticache " + cmd.Use,
			ResolvedArgs: map[string]any{
				"instance_type": f.instanceType,
				"region":        f.region,
				"engine":        f.engine,
				"commitment":    f.commitment,
			},
			Shards: []string{shardAWSElastiCache},
			Preset: s.Preset,
		})
	}
	if err := ensureShard(cmd.Context(), shardAWSElastiCache, s.AutoFetch, cmd.ErrOrStderr()); err != nil {
		skuerrors.Write(cmd.ErrOrStderr(), err)
		return err
	}
	cat, err := catalog.Open(catalog.ShardPath(shardAWSElastiCache))
	if err != nil {
		e := &skuerrors.E{Code: skuerrors.CodeServer, Message: err.Error()}
		skuerrors.Write(cmd.ErrOrStderr(), e)
		return e
	}
	defer func() { _ = cat.Close() }()
	if stale := applyStaleGate(cmd, cat, shardAWSElastiCache, s); stale != nil {
		return stale
	}
	rows, err := cat.LookupCacheKV(context.Background(), catalog.CacheKVFilter{
		Provider:     "aws",
		Service:      "elasticache",
		ResourceName: f.instanceType,
		Region:       f.region,
		Terms:        f.terms(),
	})
	if err != nil {
		return skuerrors.WriteWrap(cmd.ErrOrStderr(), skuerrors.CodeGeneric, "aws elasticache %s: %w", cmd.Use, err)
	}
	if len(rows) == 0 {
		e := skuerrors.NotFound("aws", "elasticache",
			map[string]any{"instance_type": f.instanceType, "region": f.region, "engine": f.engine},
			"Try `sku schema aws elasticache` or drop --region for a list")
		skuerrors.Write(cmd.ErrOrStderr(), e)
		return e
	}
	return renderRows(cmd, rows, s)
}
