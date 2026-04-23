package sku

import (
	"context"
	"os"

	"github.com/spf13/cobra"

	"github.com/sofq/sku/internal/catalog"
	skuerrors "github.com/sofq/sku/internal/errors"
	"github.com/sofq/sku/internal/output"
)

const shardAzurePostgres = "azure-postgres"

func newAzurePostgresCmd() *cobra.Command {
	c := &cobra.Command{Use: "postgres", Short: "Azure Database for PostgreSQL pricing"}
	c.AddCommand(newAzurePostgresPriceCmd())
	c.AddCommand(newAzurePostgresListCmd())
	return c
}

type azurePostgresFlags struct {
	skuName          string
	region           string
	deploymentOption string
	commitment       string
}

func (f *azurePostgresFlags) bind(c *cobra.Command) {
	c.Flags().StringVar(&f.skuName, "sku-name", "", "Azure PostgreSQL skuName, e.g. \"Gen5 2 vCore\"")
	c.Flags().StringVar(&f.region, "region", "", "Azure region")
	c.Flags().StringVar(&f.deploymentOption, "deployment-option", "flexible-server",
		"flexible-server | single-az")
	c.Flags().StringVar(&f.commitment, "commitment", "on_demand", "on_demand")
}

func (f *azurePostgresFlags) terms() catalog.Terms {
	return catalog.Terms{Commitment: f.commitment, Tenancy: "azure-postgres", OS: f.deploymentOption}
}

func newAzurePostgresPriceCmd() *cobra.Command {
	var f azurePostgresFlags
	c := &cobra.Command{
		Use:   "price",
		Short: "Price one Azure PostgreSQL SKU",
		RunE:  func(cmd *cobra.Command, _ []string) error { return runAzurePostgres(cmd, &f, true) },
	}
	f.bind(c)
	return c
}

func newAzurePostgresListCmd() *cobra.Command {
	var f azurePostgresFlags
	c := &cobra.Command{
		Use:   "list",
		Short: "List Azure PostgreSQL SKUs matching filters (region optional)",
		RunE:  func(cmd *cobra.Command, _ []string) error { return runAzurePostgres(cmd, &f, false) },
	}
	f.bind(c)
	return c
}

func runAzurePostgres(cmd *cobra.Command, f *azurePostgresFlags, requireRegion bool) error {
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
			Command: "azure postgres " + cmd.Use,
			ResolvedArgs: map[string]any{
				"sku_name":          f.skuName,
				"region":            f.region,
				"deployment_option": f.deploymentOption,
				"commitment":        f.commitment,
			},
			Shards: []string{shardAzurePostgres},
			Preset: s.Preset,
		})
	}
	shardPath := catalog.ShardPath(shardAzurePostgres)
	if _, err := os.Stat(shardPath); err != nil {
		e := shardMissingErr(shardAzurePostgres)
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

	if stale := applyStaleGate(cmd, cat, shardAzurePostgres, s); stale != nil {
		return stale
	}

	rows, err := cat.LookupDBRelational(context.Background(), catalog.DBRelationalFilter{
		Provider:     "azure",
		Service:      "postgres",
		InstanceType: f.skuName,
		Region:       f.region,
		Terms:        f.terms(),
	})
	if err != nil {
		return skuerrors.WriteWrap(cmd.ErrOrStderr(), skuerrors.CodeGeneric, "azure postgres %s: %w", cmd.Use, err)
	}
	if len(rows) == 0 {
		e := skuerrors.NotFound("azure", "postgres",
			map[string]any{
				"sku_name":          f.skuName,
				"region":            f.region,
				"deployment_option": f.deploymentOption,
			},
			"Try `sku schema azure postgres` or drop --region for a list")
		skuerrors.Write(cmd.ErrOrStderr(), e)
		return e
	}
	return renderRows(cmd, rows, s)
}
