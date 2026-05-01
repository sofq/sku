package sku

import (
	"context"

	"github.com/spf13/cobra"

	"github.com/sofq/sku/internal/catalog"
	skuerrors "github.com/sofq/sku/internal/errors"
	"github.com/sofq/sku/internal/output"
)

const shardAWSRoute53 = "aws-route53"

func newAWSRoute53Cmd() *cobra.Command {
	c := &cobra.Command{Use: "route53", Short: "AWS Route 53 pricing"}
	c.AddCommand(newAWSRoute53PriceCmd())
	c.AddCommand(newAWSRoute53ListCmd())
	return c
}

type route53Flags struct {
	zoneType   string
	region     string
	commitment string
}

func (f *route53Flags) bind(c *cobra.Command) {
	c.Flags().StringVar(&f.zoneType, "zone-type", "public",
		"Zone type: public | private")
	c.Flags().StringVar(&f.region, "region", "global",
		"Region (Route53 is global; defaults to 'global')")
	c.Flags().StringVar(&f.commitment, "commitment", "on_demand",
		"on_demand (only on-demand supported)")
}

func (f *route53Flags) terms() catalog.Terms {
	return catalog.Terms{Commitment: f.commitment}
}

func newAWSRoute53PriceCmd() *cobra.Command {
	var f route53Flags
	c := &cobra.Command{
		Use:   "price",
		Short: "Price one Route 53 zone type",
		RunE:  func(cmd *cobra.Command, _ []string) error { return runAWSRoute53(cmd, &f, true) },
	}
	f.bind(c)
	return c
}

func newAWSRoute53ListCmd() *cobra.Command {
	var f route53Flags
	c := &cobra.Command{
		Use:   "list",
		Short: "List Route 53 zone types",
		RunE:  func(cmd *cobra.Command, _ []string) error { return runAWSRoute53(cmd, &f, false) },
	}
	f.bind(c)
	return c
}

func runAWSRoute53(cmd *cobra.Command, f *route53Flags, requireZoneType bool) error {
	s := globalSettings(cmd)
	if requireZoneType && f.zoneType == "" {
		e := skuerrors.Validation("flag_invalid", "zone-type", "",
			"pass --zone-type public|private")
		skuerrors.Write(cmd.ErrOrStderr(), e)
		return e
	}
	if s.DryRun {
		return output.EmitDryRun(cmd.OutOrStdout(), output.DryRunPlan{
			Command: "aws route53 " + cmd.Use,
			ResolvedArgs: map[string]any{
				"zone_type":  f.zoneType,
				"region":     f.region,
				"commitment": f.commitment,
			},
			Shards: []string{shardAWSRoute53},
			Preset: s.Preset,
		})
	}
	if err := ensureShard(cmd.Context(), shardAWSRoute53, s.AutoFetch, cmd.ErrOrStderr()); err != nil {
		skuerrors.Write(cmd.ErrOrStderr(), err)
		return err
	}
	cat, err := catalog.Open(catalog.ShardPath(shardAWSRoute53))
	if err != nil {
		e := &skuerrors.E{Code: skuerrors.CodeServer, Message: err.Error()}
		skuerrors.Write(cmd.ErrOrStderr(), e)
		return e
	}
	defer func() { _ = cat.Close() }()

	if stale := applyStaleGate(cmd, cat, shardAWSRoute53, s); stale != nil {
		return stale
	}

	rows, err := cat.LookupDNSZone(context.Background(), catalog.DNSZoneFilter{
		Provider:     "aws",
		Service:      "route53",
		ResourceName: f.zoneType,
		Region:       f.region,
		Terms:        f.terms(),
	})
	if err != nil {
		return skuerrors.WriteWrap(cmd.ErrOrStderr(), skuerrors.CodeGeneric, "aws route53 %s: %w", cmd.Use, err)
	}
	if len(rows) == 0 {
		e := skuerrors.NotFound("aws", "route53",
			map[string]any{
				"zone_type": f.zoneType,
				"region":    f.region,
			},
			"Try `sku aws route53 list` to see available zone types")
		skuerrors.Write(cmd.ErrOrStderr(), e)
		return e
	}
	return renderRows(cmd, rows, s)
}
