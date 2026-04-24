package sku

import (
	"context"

	"github.com/spf13/cobra"

	"github.com/sofq/sku/internal/catalog"
	skuerrors "github.com/sofq/sku/internal/errors"
	"github.com/sofq/sku/internal/output"
)

const shardAWSCloudFront = "aws-cloudfront"

func newAWSCloudFrontCmd() *cobra.Command {
	c := &cobra.Command{Use: "cloudfront", Short: "AWS CloudFront pricing"}
	c.AddCommand(newAWSCloudFrontPriceCmd())
	c.AddCommand(newAWSCloudFrontListCmd())
	return c
}

type cfFlags struct {
	resourceName string
	region       string
	commitment   string
}

func (f *cfFlags) bind(c *cobra.Command) {
	c.Flags().StringVar(&f.resourceName, "resource-name", "standard",
		"CloudFront offering (only 'standard' shipped in m3a.3)")
	c.Flags().StringVar(&f.region, "region", "",
		"canonical edge region: us-east-1 | eu-west-1 | ap-northeast-1")
	c.Flags().StringVar(&f.commitment, "commitment", "on_demand", "on_demand (only on-demand shipped)")
}

func (f *cfFlags) terms() catalog.Terms { return catalog.Terms{Commitment: f.commitment} }

func newAWSCloudFrontPriceCmd() *cobra.Command {
	var f cfFlags
	c := &cobra.Command{
		Use: "price", Short: "Price one CloudFront edge region",
		RunE: func(cmd *cobra.Command, _ []string) error { return runAWSCloudFront(cmd, &f, true) },
	}
	f.bind(c)
	return c
}

func newAWSCloudFrontListCmd() *cobra.Command {
	var f cfFlags
	c := &cobra.Command{
		Use: "list", Short: "List CloudFront edge-region SKUs (region optional)",
		RunE: func(cmd *cobra.Command, _ []string) error { return runAWSCloudFront(cmd, &f, false) },
	}
	f.bind(c)
	return c
}

func runAWSCloudFront(cmd *cobra.Command, f *cfFlags, requireRegion bool) error {
	s := globalSettings(cmd)
	if f.resourceName == "" {
		e := skuerrors.Validation("flag_invalid", "resource-name", "",
			"pass --resource-name <offering>, e.g. --resource-name standard")
		skuerrors.Write(cmd.ErrOrStderr(), e)
		return e
	}
	if requireRegion && f.region == "" {
		e := skuerrors.Validation("flag_invalid", "region", "",
			"pass --region <canonical-edge-region>")
		skuerrors.Write(cmd.ErrOrStderr(), e)
		return e
	}
	if s.DryRun {
		return output.EmitDryRun(cmd.OutOrStdout(), output.DryRunPlan{
			Command: "aws cloudfront " + cmd.Use,
			ResolvedArgs: map[string]any{
				"resource_name": f.resourceName,
				"region":        f.region,
				"commitment":    f.commitment,
			},
			Shards: []string{shardAWSCloudFront},
			Preset: s.Preset,
		})
	}
	if err := ensureShard(cmd.Context(), shardAWSCloudFront, s.AutoFetch, cmd.ErrOrStderr()); err != nil {
		skuerrors.Write(cmd.ErrOrStderr(), err)
		return err
	}
	cat, err := catalog.Open(catalog.ShardPath(shardAWSCloudFront))
	if err != nil {
		e := &skuerrors.E{Code: skuerrors.CodeServer, Message: err.Error()}
		skuerrors.Write(cmd.ErrOrStderr(), e)
		return e
	}
	defer func() { _ = cat.Close() }()

	if stale := applyStaleGate(cmd, cat, shardAWSCloudFront, s); stale != nil {
		return stale
	}

	rows, err := cat.LookupCDN(context.Background(), catalog.CDNFilter{
		Provider: "aws", Service: "cloudfront",
		ResourceName: f.resourceName,
		Region:       f.region,
		Terms:        f.terms(),
	})
	if err != nil {
		return skuerrors.WriteWrap(cmd.ErrOrStderr(), skuerrors.CodeGeneric, "aws cloudfront %s: %w", cmd.Use, err)
	}
	if len(rows) == 0 {
		e := skuerrors.NotFound("aws", "cloudfront",
			map[string]any{"resource_name": f.resourceName, "region": f.region},
			"Try `sku schema aws cloudfront` or drop --region for a list")
		skuerrors.Write(cmd.ErrOrStderr(), e)
		return e
	}
	return renderRows(cmd, rows, s)
}
