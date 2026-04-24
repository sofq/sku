package sku

import (
	"context"

	"github.com/spf13/cobra"

	"github.com/sofq/sku/internal/catalog"
	skuerrors "github.com/sofq/sku/internal/errors"
	"github.com/sofq/sku/internal/output"
)

const shardAzureSQL = "azure-sql"

func newAzureSQLCmd() *cobra.Command {
	c := &cobra.Command{Use: "sql", Short: "Azure SQL Database pricing"}
	c.AddCommand(newAzureSQLPriceCmd())
	c.AddCommand(newAzureSQLListCmd())
	return c
}

type azureSQLFlags struct {
	skuName          string
	region           string
	deploymentOption string
	commitment       string
}

func (f *azureSQLFlags) bind(c *cobra.Command) {
	c.Flags().StringVar(&f.skuName, "sku-name", "", "Azure SQL skuName, e.g. GP_Gen5_2")
	c.Flags().StringVar(&f.region, "region", "", "Azure region")
	c.Flags().StringVar(&f.deploymentOption, "deployment-option", "single-az",
		"single-az | managed-instance | elastic-pool")
	c.Flags().StringVar(&f.commitment, "commitment", "on_demand",
		"on_demand (only on-demand shipped in m3b.1)")
}

func (f *azureSQLFlags) terms() catalog.Terms {
	return catalog.Terms{Commitment: f.commitment, Tenancy: "azure-sql", OS: f.deploymentOption}
}

func newAzureSQLPriceCmd() *cobra.Command {
	var f azureSQLFlags
	c := &cobra.Command{
		Use:   "price",
		Short: "Price one Azure SQL SKU",
		RunE:  func(cmd *cobra.Command, _ []string) error { return runAzureSQL(cmd, &f, true) },
	}
	f.bind(c)
	return c
}

func newAzureSQLListCmd() *cobra.Command {
	var f azureSQLFlags
	c := &cobra.Command{
		Use:   "list",
		Short: "List Azure SQL SKUs matching filters (region optional)",
		RunE:  func(cmd *cobra.Command, _ []string) error { return runAzureSQL(cmd, &f, false) },
	}
	f.bind(c)
	return c
}

func runAzureSQL(cmd *cobra.Command, f *azureSQLFlags, requireRegion bool) error {
	s := globalSettings(cmd)
	if f.skuName == "" {
		e := skuerrors.Validation("flag_invalid", "sku-name", "",
			"pass --sku-name <sku>, e.g. --sku-name GP_Gen5_2")
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
			Command: "azure sql " + cmd.Use,
			ResolvedArgs: map[string]any{
				"sku_name":          f.skuName,
				"region":            f.region,
				"deployment_option": f.deploymentOption,
				"commitment":        f.commitment,
			},
			Shards: []string{shardAzureSQL},
			Preset: s.Preset,
		})
	}
	if err := ensureShard(cmd.Context(), shardAzureSQL, s.AutoFetch, cmd.ErrOrStderr()); err != nil {
		skuerrors.Write(cmd.ErrOrStderr(), err)
		return err
	}
	cat, err := catalog.Open(catalog.ShardPath(shardAzureSQL))
	if err != nil {
		e := &skuerrors.E{Code: skuerrors.CodeServer, Message: err.Error()}
		skuerrors.Write(cmd.ErrOrStderr(), e)
		return e
	}
	defer func() { _ = cat.Close() }()

	if stale := applyStaleGate(cmd, cat, shardAzureSQL, s); stale != nil {
		return stale
	}

	rows, err := cat.LookupDBRelational(context.Background(), catalog.DBRelationalFilter{
		Provider:     "azure",
		Service:      "sql",
		InstanceType: f.skuName,
		Region:       f.region,
		Terms:        f.terms(),
	})
	if err != nil {
		return skuerrors.WriteWrap(cmd.ErrOrStderr(), skuerrors.CodeGeneric, "azure sql %s: %w", cmd.Use, err)
	}
	if len(rows) == 0 {
		e := skuerrors.NotFound("azure", "sql",
			map[string]any{
				"sku_name":          f.skuName,
				"region":            f.region,
				"deployment_option": f.deploymentOption,
			},
			"Try `sku schema azure sql` or drop --region for a list")
		skuerrors.Write(cmd.ErrOrStderr(), e)
		return e
	}
	return renderRows(cmd, rows, s)
}
