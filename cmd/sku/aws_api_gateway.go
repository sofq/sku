package sku

import (
	"context"

	"github.com/spf13/cobra"

	"github.com/sofq/sku/internal/catalog"
	skuerrors "github.com/sofq/sku/internal/errors"
	"github.com/sofq/sku/internal/output"
)

const shardAWSAPIGateway = "aws-api-gateway"

func newAWSAPIGatewayCmd() *cobra.Command {
	c := &cobra.Command{Use: "api-gateway", Short: "AWS API Gateway pricing"}
	c.AddCommand(newAWSAPIGatewayPriceCmd())
	c.AddCommand(newAWSAPIGatewayListCmd())
	return c
}

type apiGatewayFlags struct {
	apiType string
	region  string
}

func (f *apiGatewayFlags) bind(c *cobra.Command) {
	c.Flags().StringVar(&f.apiType, "api-type", "rest", "rest | http")
	c.Flags().StringVar(&f.region, "region", "", "AWS region (e.g. us-east-1)")
}

func (f *apiGatewayFlags) terms() catalog.Terms {
	return catalog.Terms{Commitment: "on_demand"}
}

func newAWSAPIGatewayPriceCmd() *cobra.Command {
	var f apiGatewayFlags
	c := &cobra.Command{
		Use:   "price",
		Short: "Price API Gateway for one api-type + region",
		RunE:  func(cmd *cobra.Command, _ []string) error { return runAWSAPIGateway(cmd, &f, true) },
	}
	f.bind(c)
	return c
}

func newAWSAPIGatewayListCmd() *cobra.Command {
	var f apiGatewayFlags
	c := &cobra.Command{
		Use:   "list",
		Short: "List API Gateway SKUs matching filters (region optional)",
		RunE:  func(cmd *cobra.Command, _ []string) error { return runAWSAPIGateway(cmd, &f, false) },
	}
	f.bind(c)
	return c
}

func runAWSAPIGateway(cmd *cobra.Command, f *apiGatewayFlags, requireRegion bool) error {
	s := globalSettings(cmd)
	if f.apiType == "" {
		e := skuerrors.Validation("flag_invalid", "api-type", "", "pass --api-type rest|http")
		skuerrors.Write(cmd.ErrOrStderr(), e)
		return e
	}
	if f.apiType != "rest" && f.apiType != "http" {
		e := skuerrors.Validation("flag_invalid", "api-type", f.apiType, "valid values: rest, http")
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
			Command: "aws api-gateway " + cmd.Use,
			ResolvedArgs: map[string]any{
				"api_type": f.apiType,
				"region":   f.region,
			},
			Shards: []string{shardAWSAPIGateway},
			Preset: s.Preset,
		})
	}
	if err := ensureShard(cmd.Context(), shardAWSAPIGateway, s.AutoFetch, cmd.ErrOrStderr()); err != nil {
		skuerrors.Write(cmd.ErrOrStderr(), err)
		return err
	}
	cat, err := catalog.Open(catalog.ShardPath(shardAWSAPIGateway))
	if err != nil {
		e := &skuerrors.E{Code: skuerrors.CodeServer, Message: err.Error()}
		skuerrors.Write(cmd.ErrOrStderr(), e)
		return e
	}
	defer func() { _ = cat.Close() }()

	if stale := applyStaleGate(cmd, cat, shardAWSAPIGateway, s); stale != nil {
		return stale
	}

	rows, err := cat.LookupAPIGateway(context.Background(), catalog.APIGatewayFilter{
		Provider:     "aws",
		Service:      "api-gateway",
		ResourceName: f.apiType,
		Region:       f.region,
		Terms:        f.terms(),
	})
	if err != nil {
		return skuerrors.WriteWrap(cmd.ErrOrStderr(), skuerrors.CodeGeneric, "aws api-gateway %s: %w", cmd.Use, err)
	}
	if len(rows) == 0 {
		e := skuerrors.NotFound("aws", "api-gateway",
			map[string]any{"api_type": f.apiType, "region": f.region},
			"Try `sku aws api-gateway list` or drop --region for a list")
		skuerrors.Write(cmd.ErrOrStderr(), e)
		return e
	}
	return renderRows(cmd, rows, s)
}
