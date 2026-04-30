package sku

import (
	"github.com/spf13/cobra"

	"github.com/sofq/sku/internal/catalog"
	skuerrors "github.com/sofq/sku/internal/errors"
	"github.com/sofq/sku/internal/output"
)

const shardGCPBigQuery = "gcp-bigquery"

func newGCPBigQueryCmd() *cobra.Command {
	c := &cobra.Command{Use: "bigquery", Short: "GCP BigQuery pricing (on-demand, capacity, storage)"}
	c.AddCommand(newGCPBigQueryPriceCmd())
	c.AddCommand(newGCPBigQueryListCmd())
	return c
}

type bigqueryFlags struct {
	mode   string
	region string
}

func (f *bigqueryFlags) bind(c *cobra.Command) {
	c.Flags().StringVar(&f.mode, "mode", "", "on-demand | capacity-standard | capacity-enterprise | capacity-enterprise-plus | storage-active | storage-long-term")
	c.Flags().StringVar(&f.region, "region", "", "BigQuery region, e.g. bq-us, bq-eu, us-central1")
}

func (f *bigqueryFlags) terms() catalog.Terms {
	return catalog.Terms{Commitment: "on_demand", Tenancy: "shared", OS: "on-demand"}
}

var bigqueryModes = map[string]bool{
	"on-demand":                true,
	"capacity-standard":        true,
	"capacity-enterprise":      true,
	"capacity-enterprise-plus": true,
	"storage-active":           true,
	"storage-long-term":        true,
}

func newGCPBigQueryPriceCmd() *cobra.Command {
	var f bigqueryFlags
	c := &cobra.Command{
		Use:   "price",
		Short: "Price one BigQuery SKU",
		RunE:  func(cmd *cobra.Command, _ []string) error { return runGCPBigQuery(cmd, &f, true) },
	}
	f.bind(c)
	return c
}

func newGCPBigQueryListCmd() *cobra.Command {
	var f bigqueryFlags
	c := &cobra.Command{
		Use:   "list",
		Short: "List BigQuery SKUs matching filters",
		RunE:  func(cmd *cobra.Command, _ []string) error { return runGCPBigQuery(cmd, &f, false) },
	}
	f.bind(c)
	return c
}

func runGCPBigQuery(cmd *cobra.Command, f *bigqueryFlags, requireRegion bool) error {
	s := globalSettings(cmd)
	if f.mode == "" {
		e := skuerrors.Validation("flag_invalid", "mode", "",
			"pass --mode on-demand, --mode capacity-standard, --mode capacity-enterprise, --mode capacity-enterprise-plus, --mode storage-active, or --mode storage-long-term")
		skuerrors.Write(cmd.ErrOrStderr(), e)
		return e
	}
	if !bigqueryModes[f.mode] {
		e := skuerrors.Validation("flag_invalid", "mode", f.mode,
			"allowed: on-demand | capacity-standard | capacity-enterprise | capacity-enterprise-plus | storage-active | storage-long-term")
		skuerrors.Write(cmd.ErrOrStderr(), e)
		return e
	}
	if requireRegion && f.region == "" {
		e := skuerrors.Validation("flag_invalid", "region", "", "pass --region <gcp-region>, e.g. --region bq-us")
		skuerrors.Write(cmd.ErrOrStderr(), e)
		return e
	}
	if s.DryRun {
		return output.EmitDryRun(cmd.OutOrStdout(), output.DryRunPlan{
			Command: "gcp bigquery " + cmd.Use,
			ResolvedArgs: map[string]any{
				"mode":   f.mode,
				"region": f.region,
			},
			Shards: []string{shardGCPBigQuery},
			Preset: s.Preset,
		})
	}
	if err := ensureShard(cmd.Context(), shardGCPBigQuery, s.AutoFetch, cmd.ErrOrStderr()); err != nil {
		skuerrors.Write(cmd.ErrOrStderr(), err)
		return err
	}
	cat, err := catalog.Open(catalog.ShardPath(shardGCPBigQuery))
	if err != nil {
		e := &skuerrors.E{Code: skuerrors.CodeServer, Message: err.Error()}
		skuerrors.Write(cmd.ErrOrStderr(), e)
		return e
	}
	defer func() { _ = cat.Close() }()
	if stale := applyStaleGate(cmd, cat, shardGCPBigQuery, s); stale != nil {
		return stale
	}
	rows, err := cat.LookupWarehouseQuery(cmd.Context(), catalog.WarehouseQueryFilter{
		Provider:     "gcp",
		Service:      "bigquery",
		ResourceName: f.mode,
		Region:       f.region,
		Terms:        f.terms(),
	})
	if err != nil {
		return skuerrors.WriteWrap(cmd.ErrOrStderr(), skuerrors.CodeGeneric, "gcp bigquery %s: %w", cmd.Use, err)
	}
	if len(rows) == 0 {
		e := skuerrors.NotFound("gcp", "bigquery",
			map[string]any{"mode": f.mode, "region": f.region},
			"Try `sku gcp bigquery list` or drop --region for a list")
		skuerrors.Write(cmd.ErrOrStderr(), e)
		return e
	}
	return renderRows(cmd, rows, s)
}
