package sku

import (
	"context"
	"os"

	"github.com/spf13/cobra"

	"github.com/sofq/sku/internal/catalog"
	skuerrors "github.com/sofq/sku/internal/errors"
	"github.com/sofq/sku/internal/output"
)

const shardAzureDisks = "azure-disks"

func newAzureDisksCmd() *cobra.Command {
	c := &cobra.Command{Use: "disks", Short: "Azure Managed Disks pricing"}
	c.AddCommand(newAzureDisksPriceCmd())
	c.AddCommand(newAzureDisksListCmd())
	return c
}

type azureDisksFlags struct {
	diskType   string
	region     string
	commitment string
}

func (f *azureDisksFlags) bind(c *cobra.Command) {
	c.Flags().StringVar(&f.diskType, "disk-type", "",
		"standard-hdd | standard-ssd | premium-ssd (LRS only in m3b.2)")
	c.Flags().StringVar(&f.region, "region", "", "Azure region (e.g. eastus)")
	c.Flags().StringVar(&f.commitment, "commitment", "on_demand", "on_demand (only on-demand shipped in m3b.2)")
}

func (f *azureDisksFlags) terms() catalog.Terms {
	return catalog.Terms{Commitment: f.commitment}
}

func newAzureDisksPriceCmd() *cobra.Command {
	var f azureDisksFlags
	c := &cobra.Command{
		Use:   "price",
		Short: "Price one Azure managed-disk type",
		RunE:  func(cmd *cobra.Command, _ []string) error { return runAzureDisks(cmd, &f, true) },
	}
	f.bind(c)
	return c
}

func newAzureDisksListCmd() *cobra.Command {
	var f azureDisksFlags
	c := &cobra.Command{
		Use:   "list",
		Short: "List Azure managed-disk SKUs matching filters (region optional)",
		RunE:  func(cmd *cobra.Command, _ []string) error { return runAzureDisks(cmd, &f, false) },
	}
	f.bind(c)
	return c
}

func runAzureDisks(cmd *cobra.Command, f *azureDisksFlags, requireRegion bool) error {
	s := globalSettings(cmd)
	if f.diskType == "" {
		e := skuerrors.Validation("flag_invalid", "disk-type", "",
			"pass --disk-type <type>, e.g. --disk-type standard-ssd")
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
			Command: "azure disks " + cmd.Use,
			ResolvedArgs: map[string]any{
				"disk_type":  f.diskType,
				"region":     f.region,
				"commitment": f.commitment,
			},
			Shards: []string{shardAzureDisks},
			Preset: s.Preset,
		})
	}
	shardPath := catalog.ShardPath(shardAzureDisks)
	if _, err := os.Stat(shardPath); err != nil {
		e := shardMissingErr(shardAzureDisks)
		skuerrors.Write(cmd.ErrOrStderr(), e)
		return e
	}
	cat, err := catalog.Open(shardPath)
	if err != nil {
		e := &skuerrors.E{Code: skuerrors.CodeServer, Message: err.Error()}
		skuerrors.Write(cmd.ErrOrStderr(), e)
		return e
	}
	defer func() { _ = cat.Close() }()

	if stale := applyStaleGate(cmd, cat, shardAzureDisks, s); stale != nil {
		return stale
	}

	rows, err := cat.LookupStorageBlock(context.Background(), catalog.StorageBlockFilter{
		Provider: "azure", Service: "disks",
		VolumeType: f.diskType,
		Region:     f.region,
		Terms:      f.terms(),
	})
	if err != nil {
		return skuerrors.WriteWrap(cmd.ErrOrStderr(), skuerrors.CodeGeneric, "azure disks %s: %w", cmd.Use, err)
	}
	if len(rows) == 0 {
		e := skuerrors.NotFound("azure", "disks",
			map[string]any{"disk_type": f.diskType, "region": f.region},
			"Try `sku schema azure disks` or drop --region for a list")
		skuerrors.Write(cmd.ErrOrStderr(), e)
		return e
	}
	return renderRows(cmd, rows, s)
}
