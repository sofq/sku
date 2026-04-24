package sku

import (
	"context"

	"github.com/spf13/cobra"

	"github.com/sofq/sku/internal/catalog"
	skuerrors "github.com/sofq/sku/internal/errors"
	"github.com/sofq/sku/internal/output"
)

const shardAzureMariaDB = "azure-mariadb"

func newAzureMariaDBCmd() *cobra.Command {
	c := &cobra.Command{Use: "mariadb", Short: "Azure Database for MariaDB pricing"}
	c.AddCommand(newAzureMariaDBPriceCmd())
	c.AddCommand(newAzureMariaDBListCmd())
	return c
}

type azureMariaDBFlags struct {
	skuName          string
	region           string
	deploymentOption string
	commitment       string
}

func (f *azureMariaDBFlags) bind(c *cobra.Command) {
	c.Flags().StringVar(&f.skuName, "sku-name", "", "Azure MariaDB skuName, e.g. \"Gen5 2 vCore\"")
	c.Flags().StringVar(&f.region, "region", "", "Azure region")
	c.Flags().StringVar(&f.deploymentOption, "deployment-option", "single-az",
		"single-az (MariaDB only ships Single Server)")
	c.Flags().StringVar(&f.commitment, "commitment", "on_demand", "on_demand")
}

func (f *azureMariaDBFlags) terms() catalog.Terms {
	return catalog.Terms{Commitment: f.commitment, Tenancy: "azure-mariadb", OS: f.deploymentOption}
}

func newAzureMariaDBPriceCmd() *cobra.Command {
	var f azureMariaDBFlags
	c := &cobra.Command{
		Use:   "price",
		Short: "Price one Azure MariaDB SKU",
		RunE:  func(cmd *cobra.Command, _ []string) error { return runAzureMariaDB(cmd, &f, true) },
	}
	f.bind(c)
	return c
}

func newAzureMariaDBListCmd() *cobra.Command {
	var f azureMariaDBFlags
	c := &cobra.Command{
		Use:   "list",
		Short: "List Azure MariaDB SKUs matching filters (region optional)",
		RunE:  func(cmd *cobra.Command, _ []string) error { return runAzureMariaDB(cmd, &f, false) },
	}
	f.bind(c)
	return c
}

func runAzureMariaDB(cmd *cobra.Command, f *azureMariaDBFlags, requireRegion bool) error {
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
			Command: "azure mariadb " + cmd.Use,
			ResolvedArgs: map[string]any{
				"sku_name":          f.skuName,
				"region":            f.region,
				"deployment_option": f.deploymentOption,
				"commitment":        f.commitment,
			},
			Shards: []string{shardAzureMariaDB},
			Preset: s.Preset,
		})
	}
	if err := ensureShard(cmd.Context(), shardAzureMariaDB, s.AutoFetch, cmd.ErrOrStderr()); err != nil {
		skuerrors.Write(cmd.ErrOrStderr(), err)
		return err
	}
	cat, err := catalog.Open(catalog.ShardPath(shardAzureMariaDB))
	if err != nil {
		e := &skuerrors.E{Code: skuerrors.CodeServer, Message: err.Error()}
		skuerrors.Write(cmd.ErrOrStderr(), e)
		return e
	}
	defer func() { _ = cat.Close() }()

	if stale := applyStaleGate(cmd, cat, shardAzureMariaDB, s); stale != nil {
		return stale
	}

	rows, err := cat.LookupDBRelational(context.Background(), catalog.DBRelationalFilter{
		Provider:     "azure",
		Service:      "mariadb",
		InstanceType: f.skuName,
		Region:       f.region,
		Terms:        f.terms(),
	})
	if err != nil {
		return skuerrors.WriteWrap(cmd.ErrOrStderr(), skuerrors.CodeGeneric, "azure mariadb %s: %w", cmd.Use, err)
	}
	if len(rows) == 0 {
		e := skuerrors.NotFound("azure", "mariadb",
			map[string]any{
				"sku_name":          f.skuName,
				"region":            f.region,
				"deployment_option": f.deploymentOption,
			},
			"Try `sku schema azure mariadb` or drop --region for a list")
		skuerrors.Write(cmd.ErrOrStderr(), e)
		return e
	}
	return renderRows(cmd, rows, s)
}
