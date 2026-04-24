package sku

import (
	"context"

	"github.com/spf13/cobra"

	"github.com/sofq/sku/internal/catalog"
	skuerrors "github.com/sofq/sku/internal/errors"
	"github.com/sofq/sku/internal/output"
)

const shardAzureVM = "azure-vm"

func newAzureVMCmd() *cobra.Command {
	c := &cobra.Command{Use: "vm", Short: "Azure Virtual Machine pricing"}
	c.AddCommand(newAzureVMPriceCmd())
	c.AddCommand(newAzureVMListCmd())
	return c
}

type azureVMFlags struct {
	armSkuName string
	region     string
	os         string
	commitment string
}

func (f *azureVMFlags) bind(c *cobra.Command) {
	c.Flags().StringVar(&f.armSkuName, "arm-sku-name", "", "Azure ARM SKU name, e.g. Standard_D2_v3")
	c.Flags().StringVar(&f.region, "region", "", "Azure region (e.g. eastus)")
	c.Flags().StringVar(&f.os, "os", "linux", "linux | windows")
	c.Flags().StringVar(&f.commitment, "commitment", "on_demand", "on_demand (only on-demand shipped in m3b.1)")
}

func (f *azureVMFlags) terms() catalog.Terms {
	return catalog.Terms{Commitment: f.commitment, Tenancy: "shared", OS: f.os}
}

func newAzureVMPriceCmd() *cobra.Command {
	var f azureVMFlags
	c := &cobra.Command{
		Use:   "price",
		Short: "Price one Azure VM SKU",
		RunE:  func(cmd *cobra.Command, _ []string) error { return runAzureVM(cmd, &f, true) },
	}
	f.bind(c)
	return c
}

func newAzureVMListCmd() *cobra.Command {
	var f azureVMFlags
	c := &cobra.Command{
		Use:   "list",
		Short: "List Azure VM SKUs matching filters (region optional)",
		RunE:  func(cmd *cobra.Command, _ []string) error { return runAzureVM(cmd, &f, false) },
	}
	f.bind(c)
	return c
}

func runAzureVM(cmd *cobra.Command, f *azureVMFlags, requireRegion bool) error {
	s := globalSettings(cmd)
	if f.armSkuName == "" {
		e := skuerrors.Validation("flag_invalid", "arm-sku-name", "",
			"pass --arm-sku-name <sku>, e.g. --arm-sku-name Standard_D2_v3")
		skuerrors.Write(cmd.ErrOrStderr(), e)
		return e
	}
	if requireRegion && f.region == "" {
		e := skuerrors.Validation("flag_invalid", "region", "",
			"pass --region <azure-region>, e.g. --region eastus")
		skuerrors.Write(cmd.ErrOrStderr(), e)
		return e
	}
	if s.DryRun {
		return output.EmitDryRun(cmd.OutOrStdout(), output.DryRunPlan{
			Command: "azure vm " + cmd.Use,
			ResolvedArgs: map[string]any{
				"arm_sku_name": f.armSkuName,
				"region":       f.region,
				"os":           f.os,
				"commitment":   f.commitment,
			},
			Shards: []string{shardAzureVM},
			Preset: s.Preset,
		})
	}
	if err := ensureShard(cmd.Context(), shardAzureVM, s.AutoFetch, cmd.ErrOrStderr()); err != nil {
		skuerrors.Write(cmd.ErrOrStderr(), err)
		return err
	}
	cat, err := catalog.Open(catalog.ShardPath(shardAzureVM))
	if err != nil {
		e := &skuerrors.E{Code: skuerrors.CodeServer, Message: err.Error()}
		skuerrors.Write(cmd.ErrOrStderr(), e)
		return e
	}
	defer func() { _ = cat.Close() }()

	if stale := applyStaleGate(cmd, cat, shardAzureVM, s); stale != nil {
		return stale
	}

	rows, err := cat.LookupVM(context.Background(), catalog.VMFilter{
		Provider:     "azure",
		Service:      "vm",
		InstanceType: f.armSkuName,
		Region:       f.region,
		Terms:        f.terms(),
	})
	if err != nil {
		return skuerrors.WriteWrap(cmd.ErrOrStderr(), skuerrors.CodeGeneric, "azure vm %s: %w", cmd.Use, err)
	}
	if len(rows) == 0 {
		e := skuerrors.NotFound("azure", "vm",
			map[string]any{
				"arm_sku_name": f.armSkuName,
				"region":       f.region,
				"os":           f.os,
			},
			"Try `sku schema azure vm` or drop --region for a list")
		skuerrors.Write(cmd.ErrOrStderr(), e)
		return e
	}
	return renderRows(cmd, rows, s)
}
