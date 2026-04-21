package sku

import (
	"context"
	"os"

	"github.com/spf13/cobra"

	"github.com/sofq/sku/internal/catalog"
	skuerrors "github.com/sofq/sku/internal/errors"
	"github.com/sofq/sku/internal/output"
)

const shardAzureFunctions = "azure-functions"

func newAzureFunctionsCmd() *cobra.Command {
	c := &cobra.Command{Use: "functions", Short: "Azure Functions pricing"}
	c.AddCommand(newAzureFunctionsPriceCmd())
	c.AddCommand(newAzureFunctionsListCmd())
	return c
}

type azureFunctionsFlags struct {
	architecture string
	region       string
	commitment   string
}

func (f *azureFunctionsFlags) bind(c *cobra.Command) {
	c.Flags().StringVar(&f.architecture, "architecture", "x86_64",
		"x86_64 (only arch shipped in m3b.2)")
	c.Flags().StringVar(&f.region, "region", "", "Azure region (e.g. eastus)")
	c.Flags().StringVar(&f.commitment, "commitment", "on_demand", "on_demand (only on-demand shipped in m3b.2)")
}

func (f *azureFunctionsFlags) terms() catalog.Terms {
	return catalog.Terms{Commitment: f.commitment}
}

func newAzureFunctionsPriceCmd() *cobra.Command {
	var f azureFunctionsFlags
	c := &cobra.Command{
		Use:   "price",
		Short: "Price Azure Functions for one architecture + region",
		RunE:  func(cmd *cobra.Command, _ []string) error { return runAzureFunctions(cmd, &f, true) },
	}
	f.bind(c)
	return c
}

func newAzureFunctionsListCmd() *cobra.Command {
	var f azureFunctionsFlags
	c := &cobra.Command{
		Use:   "list",
		Short: "List Azure Functions SKUs matching filters (region optional)",
		RunE:  func(cmd *cobra.Command, _ []string) error { return runAzureFunctions(cmd, &f, false) },
	}
	f.bind(c)
	return c
}

func runAzureFunctions(cmd *cobra.Command, f *azureFunctionsFlags, requireRegion bool) error {
	s := globalSettings(cmd)
	if f.architecture == "" {
		e := skuerrors.Validation("flag_invalid", "architecture", "",
			"pass --architecture x86_64")
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
			Command: "azure functions " + cmd.Use,
			ResolvedArgs: map[string]any{
				"architecture": f.architecture,
				"region":       f.region,
				"commitment":   f.commitment,
			},
			Shards: []string{shardAzureFunctions},
			Preset: s.Preset,
		})
	}
	shardPath := catalog.ShardPath(shardAzureFunctions)
	if _, err := os.Stat(shardPath); err != nil {
		e := shardMissingErr(shardAzureFunctions)
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

	if stale := applyStaleGate(cmd, cat, shardAzureFunctions, s); stale != nil {
		return stale
	}

	rows, err := cat.LookupServerlessFunction(context.Background(), catalog.ServerlessFunctionFilter{
		Provider: "azure", Service: "functions",
		Architecture: f.architecture,
		Region:       f.region,
		Terms:        f.terms(),
	})
	if err != nil {
		return skuerrors.WriteWrap(cmd.ErrOrStderr(), skuerrors.CodeGeneric, "azure functions %s: %w", cmd.Use, err)
	}
	if len(rows) == 0 {
		e := skuerrors.NotFound("azure", "functions",
			map[string]any{"architecture": f.architecture, "region": f.region},
			"Try `sku schema azure functions` or drop --region for a list")
		skuerrors.Write(cmd.ErrOrStderr(), e)
		return e
	}
	return renderRows(cmd, rows, s)
}
