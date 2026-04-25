package sku

import (
	"context"

	"github.com/spf13/cobra"

	"github.com/sofq/sku/internal/catalog"
	skuerrors "github.com/sofq/sku/internal/errors"
	"github.com/sofq/sku/internal/output"
)

const shardAzureCosmosDB = "azure-cosmosdb"

func newAzureCosmosDBCmd() *cobra.Command {
	c := &cobra.Command{Use: "cosmosdb", Short: "Azure Cosmos DB pricing"}
	c.AddCommand(newAzureCosmosDBPriceCmd())
	c.AddCommand(newAzureCosmosDBListCmd())
	return c
}

type cosmosFlags struct {
	capacityMode string
	api          string
	region       string
}

func (f *cosmosFlags) bind(c *cobra.Command) {
	c.Flags().StringVar(&f.capacityMode, "capacity-mode", "provisioned", "provisioned | serverless")
	c.Flags().StringVar(&f.api, "api", "sql", "sql | mongo | cassandra | table | gremlin")
	c.Flags().StringVar(&f.region, "region", "", "Azure region, e.g. eastus")
}

func (f *cosmosFlags) terms() catalog.Terms {
	return catalog.Terms{Commitment: "on_demand", Tenancy: f.api, OS: f.capacityMode}
}

func (f *cosmosFlags) resourceName() string {
	if f.capacityMode == "serverless" {
		return "cosmos-serverless"
	}
	return "cosmos-provisioned"
}

func newAzureCosmosDBPriceCmd() *cobra.Command {
	var f cosmosFlags
	c := &cobra.Command{
		Use:   "price",
		Short: "Price one Azure Cosmos DB capacity mode",
		RunE:  func(cmd *cobra.Command, _ []string) error { return runAzureCosmos(cmd, &f, true) },
	}
	f.bind(c)
	return c
}

func newAzureCosmosDBListCmd() *cobra.Command {
	var f cosmosFlags
	c := &cobra.Command{
		Use:   "list",
		Short: "List Azure Cosmos DB SKUs matching filters (region optional)",
		RunE:  func(cmd *cobra.Command, _ []string) error { return runAzureCosmos(cmd, &f, false) },
	}
	f.bind(c)
	return c
}

func runAzureCosmos(cmd *cobra.Command, f *cosmosFlags, requireRegion bool) error {
	s := globalSettings(cmd)
	if requireRegion && f.region == "" {
		e := skuerrors.Validation("flag_invalid", "region", "", "pass --region <azure-region>")
		skuerrors.Write(cmd.ErrOrStderr(), e)
		return e
	}
	if s.DryRun {
		return output.EmitDryRun(cmd.OutOrStdout(), output.DryRunPlan{
			Command: "azure cosmosdb " + cmd.Use,
			ResolvedArgs: map[string]any{
				"capacity_mode": f.capacityMode,
				"api":           f.api,
				"region":        f.region,
			},
			Shards: []string{shardAzureCosmosDB},
			Preset: s.Preset,
		})
	}
	if err := ensureShard(cmd.Context(), shardAzureCosmosDB, s.AutoFetch, cmd.ErrOrStderr()); err != nil {
		skuerrors.Write(cmd.ErrOrStderr(), err)
		return err
	}
	cat, err := catalog.Open(catalog.ShardPath(shardAzureCosmosDB))
	if err != nil {
		e := &skuerrors.E{Code: skuerrors.CodeServer, Message: err.Error()}
		skuerrors.Write(cmd.ErrOrStderr(), e)
		return e
	}
	defer func() { _ = cat.Close() }()
	if stale := applyStaleGate(cmd, cat, shardAzureCosmosDB, s); stale != nil {
		return stale
	}
	rows, err := cat.LookupNoSQLDB(context.Background(), catalog.NoSQLDBFilter{
		Provider:     "azure",
		Service:      "cosmosdb",
		ResourceName: f.resourceName(),
		Region:       f.region,
		Terms:        f.terms(),
	})
	if err != nil {
		return skuerrors.WriteWrap(cmd.ErrOrStderr(), skuerrors.CodeGeneric, "azure cosmosdb %s: %w", cmd.Use, err)
	}
	if len(rows) == 0 {
		e := skuerrors.NotFound("azure", "cosmosdb",
			map[string]any{"capacity_mode": f.capacityMode, "api": f.api, "region": f.region},
			"Try `sku schema azure cosmosdb` or drop --region for a list")
		skuerrors.Write(cmd.ErrOrStderr(), e)
		return e
	}
	return renderRows(cmd, rows, s)
}
