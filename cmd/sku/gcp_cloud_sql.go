package sku

import (
	"context"
	"os"

	"github.com/spf13/cobra"

	"github.com/sofq/sku/internal/catalog"
	skuerrors "github.com/sofq/sku/internal/errors"
	"github.com/sofq/sku/internal/output"
)

const shardGCPCloudSQL = "gcp-cloud-sql"

func newGCPCloudSQLCmd() *cobra.Command {
	c := &cobra.Command{Use: "cloud-sql", Short: "Google Cloud SQL pricing"}
	c.AddCommand(newGCPCloudSQLPriceCmd())
	c.AddCommand(newGCPCloudSQLListCmd())
	return c
}

type gcpCloudSQLFlags struct {
	tier             string
	region           string
	engine           string
	deploymentOption string
	commitment       string
}

func (f *gcpCloudSQLFlags) bind(c *cobra.Command) {
	c.Flags().StringVar(&f.tier, "tier", "",
		"Cloud SQL machine tier, e.g. db-custom-2-7680")
	c.Flags().StringVar(&f.region, "region", "", "GCP region (e.g. us-east1)")
	c.Flags().StringVar(&f.engine, "engine", "postgres", "postgres | mysql | sqlserver")
	c.Flags().StringVar(&f.deploymentOption, "deployment-option", "zonal", "zonal | regional")
	c.Flags().StringVar(&f.commitment, "commitment", "on_demand",
		"on_demand (only on-demand shipped in m3b.3)")
}

func (f *gcpCloudSQLFlags) terms() catalog.Terms {
	tenancy := map[string]string{
		"postgres":  "cloud-sql-postgres",
		"mysql":     "cloud-sql-mysql",
		"sqlserver": "cloud-sql-sqlserver",
	}
	t, ok := tenancy[f.engine]
	if !ok {
		t = "cloud-sql-postgres"
	}
	return catalog.Terms{Commitment: f.commitment, Tenancy: t, OS: f.deploymentOption}
}

func newGCPCloudSQLPriceCmd() *cobra.Command {
	var f gcpCloudSQLFlags
	c := &cobra.Command{
		Use:   "price",
		Short: "Price one GCP Cloud SQL SKU",
		RunE:  func(cmd *cobra.Command, _ []string) error { return runGCPCloudSQL(cmd, &f, true) },
	}
	f.bind(c)
	return c
}

func newGCPCloudSQLListCmd() *cobra.Command {
	var f gcpCloudSQLFlags
	c := &cobra.Command{
		Use:   "list",
		Short: "List GCP Cloud SQL SKUs matching filters (region optional)",
		RunE:  func(cmd *cobra.Command, _ []string) error { return runGCPCloudSQL(cmd, &f, false) },
	}
	f.bind(c)
	return c
}

func runGCPCloudSQL(cmd *cobra.Command, f *gcpCloudSQLFlags, requireRegion bool) error {
	s := globalSettings(cmd)
	if f.tier == "" {
		e := skuerrors.Validation("flag_invalid", "tier", "",
			"pass --tier <tier>, e.g. --tier db-custom-2-7680")
		skuerrors.Write(cmd.ErrOrStderr(), e)
		return e
	}
	if requireRegion && f.region == "" {
		e := skuerrors.Validation("flag_invalid", "region", "",
			"pass --region <gcp-region>")
		skuerrors.Write(cmd.ErrOrStderr(), e)
		return e
	}
	if s.DryRun {
		return output.EmitDryRun(cmd.OutOrStdout(), output.DryRunPlan{
			Command: "gcp cloud-sql " + cmd.Use,
			ResolvedArgs: map[string]any{
				"tier":              f.tier,
				"region":            f.region,
				"engine":            f.engine,
				"deployment_option": f.deploymentOption,
				"commitment":        f.commitment,
			},
			Shards: []string{shardGCPCloudSQL},
			Preset: s.Preset,
		})
	}
	shardPath := catalog.ShardPath(shardGCPCloudSQL)
	if _, err := os.Stat(shardPath); err != nil {
		e := shardMissingErr(shardGCPCloudSQL)
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

	if stale := applyStaleGate(cmd, cat, shardGCPCloudSQL, s); stale != nil {
		return stale
	}

	rows, err := cat.LookupDBRelational(context.Background(), catalog.DBRelationalFilter{
		Provider:     "gcp",
		Service:      "cloud-sql",
		InstanceType: f.tier,
		Region:       f.region,
		Terms:        f.terms(),
	})
	if err != nil {
		return skuerrors.WriteWrap(cmd.ErrOrStderr(), skuerrors.CodeGeneric, "gcp cloud-sql %s: %w", cmd.Use, err)
	}
	if len(rows) == 0 {
		e := skuerrors.NotFound("gcp", "cloud-sql",
			map[string]any{
				"tier":              f.tier,
				"region":            f.region,
				"engine":            f.engine,
				"deployment_option": f.deploymentOption,
			},
			"Try `sku schema gcp cloud-sql` or drop --region for a list")
		skuerrors.Write(cmd.ErrOrStderr(), e)
		return e
	}
	return renderRows(cmd, rows, s)
}
