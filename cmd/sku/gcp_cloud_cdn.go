package sku

import (
	"context"

	"github.com/spf13/cobra"

	"github.com/sofq/sku/internal/catalog"
	skuerrors "github.com/sofq/sku/internal/errors"
	"github.com/sofq/sku/internal/output"
)

const shardGCPCloudCDN = "gcp-cloud-cdn"

func newGCPCloudCDNCmd() *cobra.Command {
	c := &cobra.Command{Use: "cloud-cdn", Short: "GCP Cloud CDN pricing"}
	c.AddCommand(newGCPCloudCDNPriceCmd())
	c.AddCommand(newGCPCloudCDNListCmd())
	return c
}

type gcpCloudCDNFlags struct {
	mode       string
	region     string
	commitment string
}

func (f *gcpCloudCDNFlags) bind(c *cobra.Command) {
	c.Flags().StringVar(&f.mode, "mode", "edge-egress",
		"edge-egress | request")
	c.Flags().StringVar(&f.region, "region", "",
		"canonical region, e.g. us-east1 (or 'global' for request rows)")
	c.Flags().StringVar(&f.commitment, "commitment", "on_demand", "on_demand")
}

func (f *gcpCloudCDNFlags) terms() catalog.Terms {
	return catalog.Terms{Commitment: f.commitment}
}

func newGCPCloudCDNPriceCmd() *cobra.Command {
	var f gcpCloudCDNFlags
	c := &cobra.Command{
		Use:   "price",
		Short: "Price one GCP Cloud CDN SKU",
		RunE:  func(cmd *cobra.Command, _ []string) error { return runGCPCloudCDN(cmd, &f, true) },
	}
	f.bind(c)
	return c
}

func newGCPCloudCDNListCmd() *cobra.Command {
	var f gcpCloudCDNFlags
	c := &cobra.Command{
		Use:   "list",
		Short: "List GCP Cloud CDN SKUs matching filters",
		RunE:  func(cmd *cobra.Command, _ []string) error { return runGCPCloudCDN(cmd, &f, false) },
	}
	f.bind(c)
	return c
}

func runGCPCloudCDN(cmd *cobra.Command, f *gcpCloudCDNFlags, requireRegion bool) error {
	s := globalSettings(cmd)
	if f.mode == "" {
		e := skuerrors.Validation("flag_invalid", "mode", "",
			"pass --mode edge-egress|request")
		skuerrors.Write(cmd.ErrOrStderr(), e)
		return e
	}
	if requireRegion && f.region == "" {
		e := skuerrors.Validation("flag_invalid", "region", "",
			"pass --region <gcp-region> or 'global' for request rows")
		skuerrors.Write(cmd.ErrOrStderr(), e)
		return e
	}
	if s.DryRun {
		return output.EmitDryRun(cmd.OutOrStdout(), output.DryRunPlan{
			Command: "gcp cloud-cdn " + cmd.Use,
			ResolvedArgs: map[string]any{
				"mode":       f.mode,
				"region":     f.region,
				"commitment": f.commitment,
			},
			Shards: []string{shardGCPCloudCDN},
			Preset: s.Preset,
		})
	}
	if err := ensureShard(cmd.Context(), shardGCPCloudCDN, s.AutoFetch, cmd.ErrOrStderr()); err != nil {
		skuerrors.Write(cmd.ErrOrStderr(), err)
		return err
	}
	cat, err := catalog.Open(catalog.ShardPath(shardGCPCloudCDN))
	if err != nil {
		e := &skuerrors.E{Code: skuerrors.CodeServer, Message: err.Error()}
		skuerrors.Write(cmd.ErrOrStderr(), e)
		return e
	}
	defer func() { _ = cat.Close() }()
	if stale := applyStaleGate(cmd, cat, shardGCPCloudCDN, s); stale != nil {
		return stale
	}
	rows, err := cat.LookupCDN(context.Background(), catalog.CDNFilter{
		Provider:     "gcp",
		Service:      "cloud-cdn",
		ResourceName: "standard",
		Region:       f.region,
		Terms:        f.terms(),
	})
	if err != nil {
		return skuerrors.WriteWrap(cmd.ErrOrStderr(), skuerrors.CodeGeneric, "gcp cloud-cdn %s: %w", cmd.Use, err)
	}
	if len(rows) == 0 {
		e := skuerrors.NotFound("gcp", "cloud-cdn",
			map[string]any{"mode": f.mode, "region": f.region},
			"Try `sku gcp cloud-cdn list` or drop --region for all regions")
		skuerrors.Write(cmd.ErrOrStderr(), e)
		return e
	}
	return renderRows(cmd, rows, s)
}
