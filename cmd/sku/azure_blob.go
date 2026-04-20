package sku

import (
	"context"
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/sofq/sku/internal/catalog"
	skuerrors "github.com/sofq/sku/internal/errors"
	"github.com/sofq/sku/internal/output"
)

const shardAzureBlob = "azure-blob"

func newAzureBlobCmd() *cobra.Command {
	c := &cobra.Command{Use: "blob", Short: "Azure Blob Storage pricing"}
	c.AddCommand(newAzureBlobPriceCmd())
	c.AddCommand(newAzureBlobListCmd())
	return c
}

type azureBlobFlags struct {
	tier       string // hot | cool | archive
	region     string
	commitment string
}

func (f *azureBlobFlags) bind(c *cobra.Command) {
	c.Flags().StringVar(&f.tier, "tier", "",
		"hot | cool | archive (LRS redundancy only in m3b.2)")
	c.Flags().StringVar(&f.region, "region", "", "Azure region (e.g. eastus)")
	c.Flags().StringVar(&f.commitment, "commitment", "on_demand", "on_demand (only on-demand shipped in m3b.2)")
}

func (f *azureBlobFlags) terms() catalog.Terms {
	return catalog.Terms{Commitment: f.commitment}
}

func newAzureBlobPriceCmd() *cobra.Command {
	var f azureBlobFlags
	c := &cobra.Command{
		Use:   "price",
		Short: "Price one Azure Blob tier",
		RunE:  func(cmd *cobra.Command, _ []string) error { return runAzureBlob(cmd, &f, true) },
	}
	f.bind(c)
	return c
}

func newAzureBlobListCmd() *cobra.Command {
	var f azureBlobFlags
	c := &cobra.Command{
		Use:   "list",
		Short: "List Azure Blob tier SKUs matching filters (region optional)",
		RunE:  func(cmd *cobra.Command, _ []string) error { return runAzureBlob(cmd, &f, false) },
	}
	f.bind(c)
	return c
}

func runAzureBlob(cmd *cobra.Command, f *azureBlobFlags, requireRegion bool) error {
	s := globalSettings(cmd)
	if f.tier == "" {
		e := skuerrors.Validation("flag_invalid", "tier", "",
			"pass --tier <tier>, e.g. --tier hot")
		skuerrors.Write(cmd.ErrOrStderr(), e)
		return e
	}
	if requireRegion && f.region == "" {
		e := skuerrors.Validation("flag_invalid", "region", "",
			"pass --region <azure-region>")
		skuerrors.Write(cmd.ErrOrStderr(), e)
		return e
	}
	if s.DryRun {
		return output.EmitDryRun(cmd.OutOrStdout(), output.DryRunPlan{
			Command: "azure blob " + cmd.Use,
			ResolvedArgs: map[string]any{
				"tier":       f.tier,
				"region":     f.region,
				"commitment": f.commitment,
			},
			Shards: []string{shardAzureBlob},
			Preset: s.Preset,
		})
	}
	shardPath := catalog.ShardPath(shardAzureBlob)
	if _, err := os.Stat(shardPath); err != nil {
		e := shardMissingErr(shardAzureBlob)
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

	if stale := applyStaleGate(cmd, cat, shardAzureBlob, s); stale != nil {
		return stale
	}

	rows, err := cat.LookupStorageObject(context.Background(), catalog.StorageObjectFilter{
		Provider: "azure", Service: "blob",
		StorageClass: f.tier,
		Region:       f.region,
		Terms:        f.terms(),
	})
	if err != nil {
		wrapped := fmt.Errorf("azure blob %s: %w", cmd.Use, err)
		skuerrors.Write(cmd.ErrOrStderr(), wrapped)
		return wrapped
	}
	if len(rows) == 0 {
		e := skuerrors.NotFound("azure", "blob",
			map[string]any{"tier": f.tier, "region": f.region},
			"Try `sku schema azure blob` or drop --region for a list")
		skuerrors.Write(cmd.ErrOrStderr(), e)
		return e
	}
	return renderRows(cmd, rows, s)
}
