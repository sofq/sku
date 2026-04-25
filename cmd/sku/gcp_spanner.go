package sku

import (
	"context"

	"github.com/spf13/cobra"

	"github.com/sofq/sku/internal/catalog"
	skuerrors "github.com/sofq/sku/internal/errors"
	"github.com/sofq/sku/internal/output"
)

const shardGCPSpanner = "gcp-spanner"

func newGCPSpannerCmd() *cobra.Command {
	c := &cobra.Command{Use: "spanner", Short: "GCP Cloud Spanner pricing"}
	c.AddCommand(newGCPSpannerPriceCmd())
	c.AddCommand(newGCPSpannerListCmd())
	return c
}

type gcpSpannerFlags struct {
	edition string
	region  string
	pu      int
}

func (f *gcpSpannerFlags) bind(c *cobra.Command) {
	c.Flags().StringVar(&f.edition, "edition", "standard",
		"Spanner edition: standard | enterprise | enterprise-plus")
	c.Flags().StringVar(&f.region, "region", "", "GCP region (e.g. us-east1)")
	c.Flags().IntVar(&f.pu, "pu", 1000, "Number of Processing Units (display only)")
}

func (f *gcpSpannerFlags) resourceName() string {
	return "spanner-" + f.edition
}

func newGCPSpannerPriceCmd() *cobra.Command {
	var f gcpSpannerFlags
	c := &cobra.Command{
		Use:   "price",
		Short: "Price one GCP Cloud Spanner SKU",
		RunE:  func(cmd *cobra.Command, _ []string) error { return runGCPSpanner(cmd, &f, true) },
	}
	f.bind(c)
	return c
}

func newGCPSpannerListCmd() *cobra.Command {
	var f gcpSpannerFlags
	c := &cobra.Command{
		Use:   "list",
		Short: "List GCP Cloud Spanner SKUs matching filters (region optional)",
		RunE:  func(cmd *cobra.Command, _ []string) error { return runGCPSpanner(cmd, &f, false) },
	}
	f.bind(c)
	return c
}

func runGCPSpanner(cmd *cobra.Command, f *gcpSpannerFlags, requireRegion bool) error {
	s := globalSettings(cmd)
	validEditions := map[string]bool{
		"standard":        true,
		"enterprise":      true,
		"enterprise-plus": true,
	}
	if !validEditions[f.edition] {
		e := skuerrors.Validation("flag_invalid", "edition", f.edition,
			"must be one of: standard | enterprise | enterprise-plus")
		skuerrors.Write(cmd.ErrOrStderr(), e)
		return e
	}
	if requireRegion && f.region == "" {
		e := skuerrors.Validation("flag_invalid", "region", "",
			"pass --region <gcp-region>")
		skuerrors.Write(cmd.ErrOrStderr(), e)
		return e
	}
	if s.DryRun {
		return output.EmitDryRun(cmd.OutOrStdout(), output.DryRunPlan{
			Command: "gcp spanner " + cmd.Use,
			ResolvedArgs: map[string]any{
				"edition": f.edition,
				"region":  f.region,
				"pu":      f.pu,
			},
			Shards: []string{shardGCPSpanner},
			Preset: s.Preset,
		})
	}
	if err := ensureShard(cmd.Context(), shardGCPSpanner, s.AutoFetch, cmd.ErrOrStderr()); err != nil {
		skuerrors.Write(cmd.ErrOrStderr(), err)
		return err
	}
	cat, err := catalog.Open(catalog.ShardPath(shardGCPSpanner))
	if err != nil {
		e := &skuerrors.E{Code: skuerrors.CodeServer, Message: err.Error()}
		skuerrors.Write(cmd.ErrOrStderr(), e)
		return e
	}
	defer func() { _ = cat.Close() }()

	if stale := applyStaleGate(cmd, cat, shardGCPSpanner, s); stale != nil {
		return stale
	}

	rows, err := cat.LookupDBRelational(context.Background(), catalog.DBRelationalFilter{
		Provider:     "gcp",
		Service:      "spanner",
		InstanceType: f.resourceName(),
		Region:       f.region,
		Terms: catalog.Terms{
			Commitment: "on_demand",
			Tenancy:    "spanner-" + f.edition,
		},
	})
	if err != nil {
		return skuerrors.WriteWrap(cmd.ErrOrStderr(), skuerrors.CodeGeneric, "gcp spanner %s: %w", cmd.Use, err)
	}
	if len(rows) == 0 {
		e := skuerrors.NotFound("gcp", "spanner",
			map[string]any{
				"edition": f.edition,
				"region":  f.region,
			},
			"Try `sku schema gcp spanner` or drop --region for a list")
		skuerrors.Write(cmd.ErrOrStderr(), e)
		return e
	}
	return renderRows(cmd, rows, s)
}
