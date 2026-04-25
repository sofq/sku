package sku

import (
	"context"

	"github.com/spf13/cobra"

	"github.com/sofq/sku/internal/catalog"
	skuerrors "github.com/sofq/sku/internal/errors"
	"github.com/sofq/sku/internal/output"
)

const shardAWSAurora = "aws-aurora"

func newAWSAuroraCmd() *cobra.Command {
	c := &cobra.Command{Use: "aurora", Short: "AWS Aurora pricing"}
	c.AddCommand(newAWSAuroraPriceCmd())
	c.AddCommand(newAWSAuroraListCmd())
	return c
}

type auroraFlags struct {
	instanceType string
	region       string
	engine       string
	capacityMode string
	commitment   string
}

func (f *auroraFlags) bind(c *cobra.Command) {
	c.Flags().StringVar(&f.instanceType, "instance-type", "", "Aurora instance type, e.g. db.r6g.large; ignored when --capacity-mode=serverless-v2")
	c.Flags().StringVar(&f.region, "region", "", "AWS region")
	c.Flags().StringVar(&f.engine, "engine", "aurora-postgres", "aurora-mysql | aurora-postgres")
	c.Flags().StringVar(&f.capacityMode, "capacity-mode", "provisioned", "provisioned | serverless-v2")
	c.Flags().StringVar(&f.commitment, "commitment", "on_demand", "on_demand (only on-demand shipped in m-γ.1)")
}

func (f *auroraFlags) terms() catalog.Terms {
	return catalog.Terms{Commitment: f.commitment, Tenancy: f.engine, OS: "single-az"}
}

func (f *auroraFlags) resourceName() string {
	if f.capacityMode == "serverless-v2" {
		return "aurora-serverless-v2"
	}
	return f.instanceType
}

func newAWSAuroraPriceCmd() *cobra.Command {
	var f auroraFlags
	c := &cobra.Command{
		Use:   "price",
		Short: "Price one Aurora SKU",
		RunE:  func(cmd *cobra.Command, _ []string) error { return runAWSAurora(cmd, &f, true) },
	}
	f.bind(c)
	return c
}

func newAWSAuroraListCmd() *cobra.Command {
	var f auroraFlags
	c := &cobra.Command{
		Use:   "list",
		Short: "List Aurora SKUs matching filters",
		RunE:  func(cmd *cobra.Command, _ []string) error { return runAWSAurora(cmd, &f, false) },
	}
	f.bind(c)
	return c
}

func runAWSAurora(cmd *cobra.Command, f *auroraFlags, requireRegion bool) error {
	s := globalSettings(cmd)
	if f.capacityMode == "provisioned" && f.instanceType == "" {
		e := skuerrors.Validation("flag_invalid", "instance-type", "",
			"pass --instance-type <type>, e.g. --instance-type db.r6g.large (or --capacity-mode=serverless-v2)")
		skuerrors.Write(cmd.ErrOrStderr(), e)
		return e
	}
	if requireRegion && f.region == "" {
		e := skuerrors.Validation("flag_invalid", "region", "", "pass --region <aws-region>")
		skuerrors.Write(cmd.ErrOrStderr(), e)
		return e
	}
	if s.DryRun {
		return output.EmitDryRun(cmd.OutOrStdout(), output.DryRunPlan{
			Command: "aws aurora " + cmd.Use,
			ResolvedArgs: map[string]any{
				"instance_type": f.instanceType,
				"region":        f.region,
				"engine":        f.engine,
				"capacity_mode": f.capacityMode,
				"commitment":    f.commitment,
			},
			Shards: []string{shardAWSAurora},
			Preset: s.Preset,
		})
	}
	if err := ensureShard(cmd.Context(), shardAWSAurora, s.AutoFetch, cmd.ErrOrStderr()); err != nil {
		skuerrors.Write(cmd.ErrOrStderr(), err)
		return err
	}
	cat, err := catalog.Open(catalog.ShardPath(shardAWSAurora))
	if err != nil {
		e := &skuerrors.E{Code: skuerrors.CodeServer, Message: err.Error()}
		skuerrors.Write(cmd.ErrOrStderr(), e)
		return e
	}
	defer func() { _ = cat.Close() }()
	if stale := applyStaleGate(cmd, cat, shardAWSAurora, s); stale != nil {
		return stale
	}
	rows, err := cat.LookupDBRelational(context.Background(), catalog.DBRelationalFilter{
		Provider:     "aws",
		Service:      "aurora",
		InstanceType: f.resourceName(),
		Region:       f.region,
		Terms:        f.terms(),
	})
	if err != nil {
		return skuerrors.WriteWrap(cmd.ErrOrStderr(), skuerrors.CodeGeneric, "aws aurora %s: %w", cmd.Use, err)
	}
	if len(rows) == 0 {
		e := skuerrors.NotFound("aws", "aurora",
			map[string]any{
				"instance_type": f.instanceType,
				"region":        f.region,
				"engine":        f.engine,
				"capacity_mode": f.capacityMode,
			},
			"Try `sku schema aws aurora` or drop --region for a list")
		skuerrors.Write(cmd.ErrOrStderr(), e)
		return e
	}
	return renderRows(cmd, rows, s)
}
