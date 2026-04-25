package sku

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/sofq/sku/internal/catalog"
	skuerrors "github.com/sofq/sku/internal/errors"
	"github.com/sofq/sku/internal/output"
)

const shardGCPGKE = "gcp-gke"

func newGCPGKECmd() *cobra.Command {
	c := &cobra.Command{Use: "gke", Short: "GCP GKE pricing (standard cluster + Autopilot)"}
	c.AddCommand(newGCPGKEPriceCmd())
	c.AddCommand(newGCPGKEListCmd())
	return c
}

type gkeFlags struct {
	tier   string // standard | autopilot
	mode   string // control-plane | autopilot
	region string
}

func (f *gkeFlags) bind(c *cobra.Command) {
	c.Flags().StringVar(&f.tier, "tier", "", "GKE tier (standard|autopilot); for --mode autopilot, tier defaults to autopilot")
	c.Flags().StringVar(&f.mode, "mode", "control-plane", "control-plane | autopilot")
	c.Flags().StringVar(&f.region, "region", "", "GCP region")
}

func (f *gkeFlags) terms() catalog.Terms {
	return catalog.Terms{Commitment: "on_demand", Tenancy: "kubernetes", OS: f.tier}
}

func (f *gkeFlags) resourceName() string {
	if f.tier == "" {
		return ""
	}
	return fmt.Sprintf("gke-%s", f.tier)
}

func newGCPGKEPriceCmd() *cobra.Command {
	var f gkeFlags
	c := &cobra.Command{
		Use:   "price",
		Short: "Price one GKE SKU",
		RunE:  func(cmd *cobra.Command, _ []string) error { return runGCPGKE(cmd, &f, true) },
	}
	f.bind(c)
	return c
}

func newGCPGKEListCmd() *cobra.Command {
	var f gkeFlags
	c := &cobra.Command{
		Use:   "list",
		Short: "List GKE SKUs matching filters",
		RunE:  func(cmd *cobra.Command, _ []string) error { return runGCPGKE(cmd, &f, false) },
	}
	f.bind(c)
	return c
}

func runGCPGKE(cmd *cobra.Command, f *gkeFlags, requireRegion bool) error {
	s := globalSettings(cmd)
	if f.mode != "control-plane" && f.mode != "autopilot" {
		e := skuerrors.Validation("flag_invalid", "mode", f.mode,
			"use --mode control-plane or --mode autopilot")
		skuerrors.Write(cmd.ErrOrStderr(), e)
		return e
	}
	// For autopilot mode, tier defaults to "autopilot"
	if f.mode == "autopilot" && f.tier == "" {
		f.tier = "autopilot"
	}
	if f.mode == "control-plane" && f.tier == "" {
		e := skuerrors.Validation("flag_invalid", "tier", "",
			"pass --tier standard")
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
			Command: "gcp gke " + cmd.Use,
			ResolvedArgs: map[string]any{
				"tier": f.tier, "mode": f.mode, "region": f.region,
			},
			Shards: []string{shardGCPGKE},
			Preset: s.Preset,
		})
	}
	if err := ensureShard(cmd.Context(), shardGCPGKE, s.AutoFetch, cmd.ErrOrStderr()); err != nil {
		skuerrors.Write(cmd.ErrOrStderr(), err)
		return err
	}
	cat, err := catalog.Open(catalog.ShardPath(shardGCPGKE))
	if err != nil {
		e := &skuerrors.E{Code: skuerrors.CodeServer, Message: err.Error()}
		skuerrors.Write(cmd.ErrOrStderr(), e)
		return e
	}
	defer func() { _ = cat.Close() }()
	if stale := applyStaleGate(cmd, cat, shardGCPGKE, s); stale != nil {
		return stale
	}
	rows, err := cat.LookupContainerOrchestration(context.Background(), catalog.ContainerOrchestrationFilter{
		Provider:     "gcp",
		Service:      "gke",
		ResourceName: f.resourceName(),
		Region:       f.region,
		Terms:        f.terms(),
	})
	if err != nil {
		return skuerrors.WriteWrap(cmd.ErrOrStderr(), skuerrors.CodeGeneric, "gcp gke %s: %w", cmd.Use, err)
	}
	if len(rows) == 0 {
		e := skuerrors.NotFound("gcp", "gke",
			map[string]any{"tier": f.tier, "mode": f.mode, "region": f.region},
			"Try `sku gcp gke list` or drop --region for a list")
		skuerrors.Write(cmd.ErrOrStderr(), e)
		return e
	}
	return renderRows(cmd, rows, s)
}
