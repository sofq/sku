package sku

import (
	"context"
	"os"

	"github.com/spf13/cobra"

	"github.com/sofq/sku/internal/catalog"
	skuerrors "github.com/sofq/sku/internal/errors"
	"github.com/sofq/sku/internal/output"
)

const shardAWSRDS = "aws-rds"

func newAWSRDSCmd() *cobra.Command {
	c := &cobra.Command{Use: "rds", Short: "AWS RDS pricing"}
	c.AddCommand(newAWSRDSPriceCmd())
	c.AddCommand(newAWSRDSListCmd())
	return c
}

type rdsFlags struct {
	instanceType     string
	region           string
	engine           string
	deploymentOption string
	commitment       string
}

func (f *rdsFlags) bind(c *cobra.Command) {
	c.Flags().StringVar(&f.instanceType, "instance-type", "", "RDS instance type, e.g. db.m5.large")
	c.Flags().StringVar(&f.region, "region", "", "AWS region")
	c.Flags().StringVar(&f.engine, "engine", "mysql", "postgres | mysql | mariadb")
	c.Flags().StringVar(&f.deploymentOption, "deployment-option", "single-az", "single-az | multi-az")
	c.Flags().StringVar(&f.commitment, "commitment", "on_demand", "on_demand (only on-demand shipped in m3a.1)")
}

func (f *rdsFlags) terms() catalog.Terms {
	return catalog.Terms{Commitment: f.commitment, Tenancy: f.engine, OS: f.deploymentOption}
}

func newAWSRDSPriceCmd() *cobra.Command {
	var f rdsFlags
	c := &cobra.Command{
		Use:   "price",
		Short: "Price one RDS SKU",
		RunE:  func(cmd *cobra.Command, _ []string) error { return runAWSDB(cmd, &f, true) },
	}
	f.bind(c)
	return c
}

func newAWSRDSListCmd() *cobra.Command {
	var f rdsFlags
	c := &cobra.Command{
		Use:   "list",
		Short: "List RDS SKUs matching filters (region optional)",
		RunE:  func(cmd *cobra.Command, _ []string) error { return runAWSDB(cmd, &f, false) },
	}
	f.bind(c)
	return c
}

func runAWSDB(cmd *cobra.Command, f *rdsFlags, requireRegion bool) error {
	s := globalSettings(cmd)
	if f.instanceType == "" {
		e := skuerrors.Validation("flag_invalid", "instance-type", "",
			"pass --instance-type <type>, e.g. --instance-type db.m5.large")
		skuerrors.Write(cmd.ErrOrStderr(), e)
		return e
	}
	if requireRegion && f.region == "" {
		e := skuerrors.Validation("flag_invalid", "region", "",
			"pass --region <aws-region>")
		skuerrors.Write(cmd.ErrOrStderr(), e)
		return e
	}
	if s.DryRun {
		return output.EmitDryRun(cmd.OutOrStdout(), output.DryRunPlan{
			Command: "aws rds " + cmd.Use,
			ResolvedArgs: map[string]any{
				"instance_type":     f.instanceType,
				"region":            f.region,
				"engine":            f.engine,
				"deployment_option": f.deploymentOption,
				"commitment":        f.commitment,
			},
			Shards: []string{shardAWSRDS},
			Preset: s.Preset,
		})
	}
	shardPath := catalog.ShardPath(shardAWSRDS)
	if _, err := os.Stat(shardPath); err != nil {
		e := shardMissingErr(shardAWSRDS)
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

	if stale := applyStaleGate(cmd, cat, shardAWSRDS, s); stale != nil {
		return stale
	}

	rows, err := cat.LookupDBRelational(context.Background(), catalog.DBRelationalFilter{
		Provider: "aws", Service: "rds",
		InstanceType: f.instanceType,
		Region:       f.region,
		Terms:        f.terms(),
	})
	if err != nil {
		return skuerrors.WriteWrap(cmd.ErrOrStderr(), skuerrors.CodeGeneric, "aws rds %s: %w", cmd.Use, err)
	}
	if len(rows) == 0 {
		e := skuerrors.NotFound("aws", "rds",
			map[string]any{
				"instance_type":     f.instanceType,
				"region":            f.region,
				"engine":            f.engine,
				"deployment_option": f.deploymentOption,
			},
			"Try `sku schema aws rds` or drop --region for a list")
		skuerrors.Write(cmd.ErrOrStderr(), e)
		return e
	}
	return renderRows(cmd, rows, s)
}
