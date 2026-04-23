package sku

import (
	"context"
	"os"

	"github.com/spf13/cobra"

	"github.com/sofq/sku/internal/catalog"
	skuerrors "github.com/sofq/sku/internal/errors"
	"github.com/sofq/sku/internal/output"
)

const shardAzureMySQL = "azure-mysql"

func newAzureMySQLCmd() *cobra.Command {
	c := &cobra.Command{Use: "mysql", Short: "Azure Database for MySQL pricing"}
	c.AddCommand(newAzureMySQLPriceCmd())
	c.AddCommand(newAzureMySQLListCmd())
	return c
}

type azureMySQLFlags struct {
	skuName          string
	region           string
	deploymentOption string
	commitment       string
}

func (f *azureMySQLFlags) bind(c *cobra.Command) {
	c.Flags().StringVar(&f.skuName, "sku-name", "", "Azure MySQL skuName, e.g. \"Gen5 2 vCore\"")
	c.Flags().StringVar(&f.region, "region", "", "Azure region")
	c.Flags().StringVar(&f.deploymentOption, "deployment-option", "flexible-server",
		"flexible-server | single-az")
	c.Flags().StringVar(&f.commitment, "commitment", "on_demand", "on_demand")
}

func (f *azureMySQLFlags) terms() catalog.Terms {
	return catalog.Terms{Commitment: f.commitment, Tenancy: "azure-mysql", OS: f.deploymentOption}
}

func newAzureMySQLPriceCmd() *cobra.Command {
	var f azureMySQLFlags
	c := &cobra.Command{
		Use:   "price",
		Short: "Price one Azure MySQL SKU",
		RunE:  func(cmd *cobra.Command, _ []string) error { return runAzureMySQL(cmd, &f, true) },
	}
	f.bind(c)
	return c
}

func newAzureMySQLListCmd() *cobra.Command {
	var f azureMySQLFlags
	c := &cobra.Command{
		Use:   "list",
		Short: "List Azure MySQL SKUs matching filters (region optional)",
		RunE:  func(cmd *cobra.Command, _ []string) error { return runAzureMySQL(cmd, &f, false) },
	}
	f.bind(c)
	return c
}

func runAzureMySQL(cmd *cobra.Command, f *azureMySQLFlags, requireRegion bool) error {
	s := globalSettings(cmd)
	if f.skuName == "" {
		e := skuerrors.Validation("flag_invalid", "sku-name", "",
			"pass --sku-name <sku>, e.g. --sku-name \"Gen5 2 vCore\"")
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
			Command: "azure mysql " + cmd.Use,
			ResolvedArgs: map[string]any{
				"sku_name":          f.skuName,
				"region":            f.region,
				"deployment_option": f.deploymentOption,
				"commitment":        f.commitment,
			},
			Shards: []string{shardAzureMySQL},
			Preset: s.Preset,
		})
	}
	shardPath := catalog.ShardPath(shardAzureMySQL)
	if _, err := os.Stat(shardPath); err != nil {
		e := shardMissingErr(shardAzureMySQL)
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

	if stale := applyStaleGate(cmd, cat, shardAzureMySQL, s); stale != nil {
		return stale
	}

	rows, err := cat.LookupDBRelational(context.Background(), catalog.DBRelationalFilter{
		Provider:     "azure",
		Service:      "mysql",
		InstanceType: f.skuName,
		Region:       f.region,
		Terms:        f.terms(),
	})
	if err != nil {
		return skuerrors.WriteWrap(cmd.ErrOrStderr(), skuerrors.CodeGeneric, "azure mysql %s: %w", cmd.Use, err)
	}
	if len(rows) == 0 {
		e := skuerrors.NotFound("azure", "mysql",
			map[string]any{
				"sku_name":          f.skuName,
				"region":            f.region,
				"deployment_option": f.deploymentOption,
			},
			"Try `sku schema azure mysql` or drop --region for a list")
		skuerrors.Write(cmd.ErrOrStderr(), e)
		return e
	}
	return renderRows(cmd, rows, s)
}
