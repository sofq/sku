package sku

import (
	"context"

	"github.com/spf13/cobra"

	"github.com/sofq/sku/internal/catalog"
	skuerrors "github.com/sofq/sku/internal/errors"
	"github.com/sofq/sku/internal/output"
)

const shardGCPCloudDNS = "gcp-cloud-dns"

func newGCPCloudDNSCmd() *cobra.Command {
	c := &cobra.Command{Use: "cloud-dns", Short: "GCP Cloud DNS pricing"}
	c.AddCommand(newGCPCloudDNSPriceCmd())
	c.AddCommand(newGCPCloudDNSListCmd())
	return c
}

type gcpCloudDNSFlags struct {
	zoneType   string
	region     string
	commitment string
}

func (f *gcpCloudDNSFlags) bind(c *cobra.Command) {
	c.Flags().StringVar(&f.zoneType, "zone-type", "public",
		"Zone type: public")
	c.Flags().StringVar(&f.region, "region", "global",
		"Region (Cloud DNS is global; defaults to 'global')")
	c.Flags().StringVar(&f.commitment, "commitment", "on_demand",
		"on_demand (only on-demand supported)")
}

func (f *gcpCloudDNSFlags) terms() catalog.Terms {
	return catalog.Terms{Commitment: f.commitment, OS: "dns-public"}
}

func newGCPCloudDNSPriceCmd() *cobra.Command {
	var f gcpCloudDNSFlags
	c := &cobra.Command{
		Use:   "price",
		Short: "Price one GCP Cloud DNS zone type",
		RunE:  func(cmd *cobra.Command, _ []string) error { return runGCPCloudDNS(cmd, &f, true) },
	}
	f.bind(c)
	return c
}

func newGCPCloudDNSListCmd() *cobra.Command {
	var f gcpCloudDNSFlags
	c := &cobra.Command{
		Use:   "list",
		Short: "List GCP Cloud DNS zone types",
		RunE:  func(cmd *cobra.Command, _ []string) error { return runGCPCloudDNS(cmd, &f, false) },
	}
	f.bind(c)
	return c
}

func runGCPCloudDNS(cmd *cobra.Command, f *gcpCloudDNSFlags, requireZoneType bool) error {
	s := globalSettings(cmd)
	if requireZoneType && f.zoneType == "" {
		e := skuerrors.Validation("flag_invalid", "zone-type", "",
			"pass --zone-type public")
		skuerrors.Write(cmd.ErrOrStderr(), e)
		return e
	}
	if s.DryRun {
		return output.EmitDryRun(cmd.OutOrStdout(), output.DryRunPlan{
			Command: "gcp cloud-dns " + cmd.Use,
			ResolvedArgs: map[string]any{
				"zone_type":  f.zoneType,
				"region":     f.region,
				"commitment": f.commitment,
			},
			Shards: []string{shardGCPCloudDNS},
			Preset: s.Preset,
		})
	}
	if err := ensureShard(cmd.Context(), shardGCPCloudDNS, s.AutoFetch, cmd.ErrOrStderr()); err != nil {
		skuerrors.Write(cmd.ErrOrStderr(), err)
		return err
	}
	cat, err := catalog.Open(catalog.ShardPath(shardGCPCloudDNS))
	if err != nil {
		e := &skuerrors.E{Code: skuerrors.CodeServer, Message: err.Error()}
		skuerrors.Write(cmd.ErrOrStderr(), e)
		return e
	}
	defer func() { _ = cat.Close() }()

	if stale := applyStaleGate(cmd, cat, shardGCPCloudDNS, s); stale != nil {
		return stale
	}

	rows, err := cat.LookupDNSZone(context.Background(), catalog.DNSZoneFilter{
		Provider:     "gcp",
		Service:      "cloud-dns",
		ResourceName: f.zoneType,
		Region:       f.region,
		Terms:        f.terms(),
	})
	if err != nil {
		return skuerrors.WriteWrap(cmd.ErrOrStderr(), skuerrors.CodeGeneric, "gcp cloud-dns %s: %w", cmd.Use, err)
	}
	if len(rows) == 0 {
		e := skuerrors.NotFound("gcp", "cloud-dns",
			map[string]any{
				"zone_type": f.zoneType,
				"region":    f.region,
			},
			"Try `sku gcp cloud-dns list` to see available zone types")
		skuerrors.Write(cmd.ErrOrStderr(), e)
		return e
	}
	return renderRows(cmd, rows, s)
}
