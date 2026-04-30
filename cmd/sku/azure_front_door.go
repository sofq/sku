package sku

import (
	"context"

	"github.com/spf13/cobra"

	"github.com/sofq/sku/internal/catalog"
	skuerrors "github.com/sofq/sku/internal/errors"
	"github.com/sofq/sku/internal/output"
)

const shardAzureFrontDoor = "azure-front-door"

func newAzureFrontDoorCmd() *cobra.Command {
	c := &cobra.Command{Use: "front-door", Short: "Azure Front Door pricing (Standard + Premium)"}
	c.AddCommand(newAzureFrontDoorPriceCmd())
	c.AddCommand(newAzureFrontDoorListCmd())
	return c
}

type azureFrontDoorFlags struct {
	tier   string
	region string
	mode   string
}

func (f *azureFrontDoorFlags) bind(c *cobra.Command) {
	c.Flags().StringVar(&f.tier, "tier", "", "Front Door SKU tier: standard | premium")
	c.Flags().StringVar(&f.region, "region", "", "Azure region (e.g. eastus) or 'global' for base-fee rows")
	c.Flags().StringVar(&f.mode, "mode", "", "pricing mode: edge-egress | request | base-fee")
}

func (f *azureFrontDoorFlags) resourceName() string {
	switch f.tier {
	case "standard":
		return "standard"
	case "premium":
		return "premium"
	default:
		return ""
	}
}

func (f *azureFrontDoorFlags) terms() catalog.Terms {
	return catalog.Terms{Commitment: "on_demand"}
}

func newAzureFrontDoorPriceCmd() *cobra.Command {
	var f azureFrontDoorFlags
	c := &cobra.Command{
		Use:   "price",
		Short: "Price one Azure Front Door SKU",
		RunE:  func(cmd *cobra.Command, _ []string) error { return runAzureFrontDoor(cmd, &f, true) },
	}
	f.bind(c)
	return c
}

func newAzureFrontDoorListCmd() *cobra.Command {
	var f azureFrontDoorFlags
	c := &cobra.Command{
		Use:   "list",
		Short: "List Azure Front Door SKUs matching filters",
		RunE:  func(cmd *cobra.Command, _ []string) error { return runAzureFrontDoor(cmd, &f, false) },
	}
	f.bind(c)
	return c
}

func runAzureFrontDoor(cmd *cobra.Command, f *azureFrontDoorFlags, requireRegion bool) error {
	s := globalSettings(cmd)
	if f.tier == "" {
		e := skuerrors.Validation("flag_invalid", "tier", "",
			"pass --tier standard | premium")
		skuerrors.Write(cmd.ErrOrStderr(), e)
		return e
	}
	if f.tier != "standard" && f.tier != "premium" {
		e := skuerrors.Validation("flag_invalid", "tier", f.tier,
			"allowed: standard | premium")
		skuerrors.Write(cmd.ErrOrStderr(), e)
		return e
	}
	if requireRegion && f.region == "" {
		e := skuerrors.Validation("flag_invalid", "region", "",
			"pass --region <azure-region> or 'global' for base-fee rows")
		skuerrors.Write(cmd.ErrOrStderr(), e)
		return e
	}
	if s.DryRun {
		return output.EmitDryRun(cmd.OutOrStdout(), output.DryRunPlan{
			Command: "azure front-door " + cmd.Use,
			ResolvedArgs: map[string]any{
				"tier":   f.tier,
				"region": f.region,
				"mode":   f.mode,
			},
			Shards: []string{shardAzureFrontDoor},
			Preset: s.Preset,
		})
	}
	if err := ensureShard(cmd.Context(), shardAzureFrontDoor, s.AutoFetch, cmd.ErrOrStderr()); err != nil {
		skuerrors.Write(cmd.ErrOrStderr(), err)
		return err
	}
	cat, err := catalog.Open(catalog.ShardPath(shardAzureFrontDoor))
	if err != nil {
		e := &skuerrors.E{Code: skuerrors.CodeServer, Message: err.Error()}
		skuerrors.Write(cmd.ErrOrStderr(), e)
		return e
	}
	defer func() { _ = cat.Close() }()

	if stale := applyStaleGate(cmd, cat, shardAzureFrontDoor, s); stale != nil {
		return stale
	}

	rows, err := cat.LookupCDN(context.Background(), catalog.CDNFilter{
		Provider:     "azure",
		Service:      "front-door",
		ResourceName: f.resourceName(),
		Region:       f.region,
		Mode:         f.mode,
		Terms:        f.terms(),
	})
	if err != nil {
		return skuerrors.WriteWrap(cmd.ErrOrStderr(), skuerrors.CodeGeneric, "azure front-door %s: %w", cmd.Use, err)
	}
	if len(rows) == 0 {
		e := skuerrors.NotFound("azure", "front-door",
			map[string]any{"tier": f.tier, "region": f.region, "mode": f.mode},
			"Try `sku azure front-door list` or check --tier / --region / --mode")
		skuerrors.Write(cmd.ErrOrStderr(), e)
		return e
	}
	return renderRows(cmd, rows, s)
}
