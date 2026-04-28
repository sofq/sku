package sku

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/sofq/sku/internal/catalog"
	skuerrors "github.com/sofq/sku/internal/errors"
	"github.com/sofq/sku/internal/output"
)

const shardAzureAKS = "azure-aks"

func newAzureAKSCmd() *cobra.Command {
	c := &cobra.Command{Use: "aks", Short: "Azure AKS pricing (control plane + virtual nodes)"}
	c.AddCommand(newAzureAKSPriceCmd())
	c.AddCommand(newAzureAKSListCmd())
	return c
}

type azureAKSFlags struct {
	tier   string
	mode   string
	aciOS  string
	region string
}

func (f *azureAKSFlags) bind(c *cobra.Command) {
	c.Flags().StringVar(&f.tier, "tier", "", "AKS tier (free|standard|premium); ignored when --mode=virtual-nodes")
	c.Flags().StringVar(&f.mode, "mode", "control-plane", "control-plane | virtual-nodes")
	c.Flags().StringVar(&f.aciOS, "aci-os", "linux", "linux (required when --mode=virtual-nodes)")
	c.Flags().StringVar(&f.region, "region", "", "Azure region")
}

func (f *azureAKSFlags) terms() catalog.Terms {
	osSlot := f.tier
	if f.mode == "virtual-nodes" {
		osSlot = "virtual-nodes"
	}
	return catalog.Terms{Commitment: "on_demand", Tenancy: "kubernetes", OS: osSlot}
}

func (f *azureAKSFlags) resourceName() string {
	if f.mode == "virtual-nodes" {
		if f.aciOS == "" {
			return ""
		}
		return fmt.Sprintf("aks-virtual-nodes-%s", f.aciOS)
	}
	if f.tier == "" {
		return ""
	}
	return fmt.Sprintf("aks-%s", f.tier)
}

func newAzureAKSPriceCmd() *cobra.Command {
	var f azureAKSFlags
	c := &cobra.Command{
		Use:   "price",
		Short: "Price one AKS SKU",
		RunE:  func(cmd *cobra.Command, _ []string) error { return runAzureAKS(cmd, &f, true) },
	}
	f.bind(c)
	return c
}

func newAzureAKSListCmd() *cobra.Command {
	var f azureAKSFlags
	c := &cobra.Command{
		Use:   "list",
		Short: "List AKS SKUs matching filters",
		RunE:  func(cmd *cobra.Command, _ []string) error { return runAzureAKS(cmd, &f, false) },
	}
	f.bind(c)
	return c
}

func runAzureAKS(cmd *cobra.Command, f *azureAKSFlags, requireRegion bool) error {
	s := globalSettings(cmd)
	if f.mode != "control-plane" && f.mode != "virtual-nodes" {
		e := skuerrors.Validation("flag_invalid", "mode", f.mode,
			"use --mode control-plane or --mode virtual-nodes")
		skuerrors.Write(cmd.ErrOrStderr(), e)
		return e
	}
	if f.mode == "control-plane" && f.tier == "" {
		e := skuerrors.Validation("flag_invalid", "tier", "",
			"pass --tier free, --tier standard, or --tier premium")
		skuerrors.Write(cmd.ErrOrStderr(), e)
		return e
	}
	if f.mode == "virtual-nodes" && f.aciOS == "" {
		e := skuerrors.Validation("flag_invalid", "aci-os", "",
			"pass --aci-os linux when using --mode virtual-nodes")
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
			Command: "azure aks " + cmd.Use,
			ResolvedArgs: map[string]any{
				"tier": f.tier, "mode": f.mode, "aci_os": f.aciOS, "region": f.region,
			},
			Shards: []string{shardAzureAKS},
			Preset: s.Preset,
		})
	}
	if err := ensureShard(cmd.Context(), shardAzureAKS, s.AutoFetch, cmd.ErrOrStderr()); err != nil {
		skuerrors.Write(cmd.ErrOrStderr(), err)
		return err
	}
	cat, err := catalog.Open(catalog.ShardPath(shardAzureAKS))
	if err != nil {
		e := &skuerrors.E{Code: skuerrors.CodeServer, Message: err.Error()}
		skuerrors.Write(cmd.ErrOrStderr(), e)
		return e
	}
	defer func() { _ = cat.Close() }()
	if stale := applyStaleGate(cmd, cat, shardAzureAKS, s); stale != nil {
		return stale
	}
	rows, err := cat.LookupContainerOrchestration(context.Background(), catalog.ContainerOrchestrationFilter{
		Provider:     "azure",
		Service:      "aks",
		ResourceName: f.resourceName(),
		Region:       f.region,
		Terms:        f.terms(),
	})
	if err != nil {
		return skuerrors.WriteWrap(cmd.ErrOrStderr(), skuerrors.CodeGeneric, "azure aks %s: %w", cmd.Use, err)
	}
	if len(rows) == 0 {
		e := skuerrors.NotFound("azure", "aks",
			map[string]any{"tier": f.tier, "mode": f.mode, "aci_os": f.aciOS, "region": f.region},
			"Try `sku azure aks list` or drop --region for a list")
		skuerrors.Write(cmd.ErrOrStderr(), e)
		return e
	}
	return renderRows(cmd, rows, s)
}
