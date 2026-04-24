package sku

import (
	"context"

	"github.com/spf13/cobra"

	"github.com/sofq/sku/internal/catalog"
	skuerrors "github.com/sofq/sku/internal/errors"
	"github.com/sofq/sku/internal/output"
)

const shardAWSLambda = "aws-lambda"

func newAWSLambdaCmd() *cobra.Command {
	c := &cobra.Command{Use: "lambda", Short: "AWS Lambda pricing"}
	c.AddCommand(newAWSLambdaPriceCmd())
	c.AddCommand(newAWSLambdaListCmd())
	return c
}

type lambdaFlags struct {
	architecture string
	region       string
	commitment   string
}

func (f *lambdaFlags) bind(c *cobra.Command) {
	c.Flags().StringVar(&f.architecture, "architecture", "x86_64", "x86_64 | arm64")
	c.Flags().StringVar(&f.region, "region", "", "AWS region (e.g. us-east-1)")
	c.Flags().StringVar(&f.commitment, "commitment", "on_demand", "on_demand (only on-demand shipped in m3a.2)")
}

func (f *lambdaFlags) terms() catalog.Terms {
	return catalog.Terms{Commitment: f.commitment}
}

func newAWSLambdaPriceCmd() *cobra.Command {
	var f lambdaFlags
	c := &cobra.Command{
		Use:   "price",
		Short: "Price Lambda for one architecture + region",
		RunE:  func(cmd *cobra.Command, _ []string) error { return runAWSLambda(cmd, &f, true) },
	}
	f.bind(c)
	return c
}

func newAWSLambdaListCmd() *cobra.Command {
	var f lambdaFlags
	c := &cobra.Command{
		Use:   "list",
		Short: "List Lambda SKUs matching filters (region optional)",
		RunE:  func(cmd *cobra.Command, _ []string) error { return runAWSLambda(cmd, &f, false) },
	}
	f.bind(c)
	return c
}

func runAWSLambda(cmd *cobra.Command, f *lambdaFlags, requireRegion bool) error {
	s := globalSettings(cmd)
	if f.architecture == "" {
		e := skuerrors.Validation("flag_invalid", "architecture", "", "pass --architecture x86_64|arm64")
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
			Command: "aws lambda " + cmd.Use,
			ResolvedArgs: map[string]any{
				"architecture": f.architecture,
				"region":       f.region,
				"commitment":   f.commitment,
			},
			Shards: []string{shardAWSLambda},
			Preset: s.Preset,
		})
	}
	if err := ensureShard(cmd.Context(), shardAWSLambda, s.AutoFetch, cmd.ErrOrStderr()); err != nil {
		skuerrors.Write(cmd.ErrOrStderr(), err)
		return err
	}
	cat, err := catalog.Open(catalog.ShardPath(shardAWSLambda))
	if err != nil {
		e := &skuerrors.E{Code: skuerrors.CodeServer, Message: err.Error()}
		skuerrors.Write(cmd.ErrOrStderr(), e)
		return e
	}
	defer func() { _ = cat.Close() }()

	if stale := applyStaleGate(cmd, cat, shardAWSLambda, s); stale != nil {
		return stale
	}

	rows, err := cat.LookupServerlessFunction(context.Background(), catalog.ServerlessFunctionFilter{
		Provider: "aws", Service: "lambda",
		Architecture: f.architecture,
		Region:       f.region,
		Terms:        f.terms(),
	})
	if err != nil {
		return skuerrors.WriteWrap(cmd.ErrOrStderr(), skuerrors.CodeGeneric, "aws lambda %s: %w", cmd.Use, err)
	}
	if len(rows) == 0 {
		e := skuerrors.NotFound("aws", "lambda",
			map[string]any{"architecture": f.architecture, "region": f.region},
			"Try `sku schema aws lambda` or drop --region for a list")
		skuerrors.Write(cmd.ErrOrStderr(), e)
		return e
	}
	return renderRows(cmd, rows, s)
}
