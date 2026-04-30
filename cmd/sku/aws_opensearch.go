package sku

import (
	"github.com/spf13/cobra"

	"github.com/sofq/sku/internal/catalog"
	skuerrors "github.com/sofq/sku/internal/errors"
	"github.com/sofq/sku/internal/output"
)

const shardAWSOpenSearch = "aws-opensearch"

func newAWSOpenSearchCmd() *cobra.Command {
	c := &cobra.Command{Use: "opensearch", Short: "AWS OpenSearch pricing (managed cluster + serverless)"}
	c.AddCommand(newAWSOpenSearchPriceCmd())
	c.AddCommand(newAWSOpenSearchListCmd())
	return c
}

type opensearchFlags struct {
	instanceType string
	mode         string
	region       string
}

func (f *opensearchFlags) bind(c *cobra.Command) {
	c.Flags().StringVar(&f.instanceType, "instance-type", "", "OpenSearch instance type, e.g. r6g.large.search; omit when --mode=serverless")
	c.Flags().StringVar(&f.mode, "mode", "managed-cluster", "managed-cluster | serverless")
	c.Flags().StringVar(&f.region, "region", "", "AWS region")
}

func (f *opensearchFlags) terms() catalog.Terms {
	return catalog.Terms{Commitment: "on_demand", Tenancy: "shared", OS: f.mode}
}

func (f *opensearchFlags) resourceName() string {
	if f.mode == "serverless" {
		return "opensearch-serverless"
	}
	return f.instanceType
}

func newAWSOpenSearchPriceCmd() *cobra.Command {
	var f opensearchFlags
	c := &cobra.Command{
		Use:   "price",
		Short: "Price one OpenSearch SKU",
		RunE:  func(cmd *cobra.Command, _ []string) error { return runAWSOpenSearch(cmd, &f, true) },
	}
	f.bind(c)
	return c
}

func newAWSOpenSearchListCmd() *cobra.Command {
	var f opensearchFlags
	c := &cobra.Command{
		Use:   "list",
		Short: "List OpenSearch SKUs matching filters",
		RunE:  func(cmd *cobra.Command, _ []string) error { return runAWSOpenSearch(cmd, &f, false) },
	}
	f.bind(c)
	return c
}

func runAWSOpenSearch(cmd *cobra.Command, f *opensearchFlags, requireRegion bool) error {
	s := globalSettings(cmd)
	if f.mode != "managed-cluster" && f.mode != "serverless" {
		e := skuerrors.Validation("flag_invalid", "mode", f.mode,
			"use --mode managed-cluster or --mode serverless")
		skuerrors.Write(cmd.ErrOrStderr(), e)
		return e
	}
	if f.mode == "managed-cluster" && f.instanceType == "" {
		e := skuerrors.Validation("flag_invalid", "instance-type", "",
			"pass --instance-type <type>, e.g. --instance-type r6g.large.search (or --mode serverless)")
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
			Command: "aws opensearch " + cmd.Use,
			ResolvedArgs: map[string]any{
				"instance_type": f.instanceType,
				"mode":          f.mode,
				"region":        f.region,
			},
			Shards: []string{shardAWSOpenSearch},
			Preset: s.Preset,
		})
	}
	if err := ensureShard(cmd.Context(), shardAWSOpenSearch, s.AutoFetch, cmd.ErrOrStderr()); err != nil {
		skuerrors.Write(cmd.ErrOrStderr(), err)
		return err
	}
	cat, err := catalog.Open(catalog.ShardPath(shardAWSOpenSearch))
	if err != nil {
		e := &skuerrors.E{Code: skuerrors.CodeServer, Message: err.Error()}
		skuerrors.Write(cmd.ErrOrStderr(), e)
		return e
	}
	defer func() { _ = cat.Close() }()
	if stale := applyStaleGate(cmd, cat, shardAWSOpenSearch, s); stale != nil {
		return stale
	}
	rows, err := cat.LookupSearchEngine(cmd.Context(), catalog.SearchEngineFilter{
		Provider:     "aws",
		Service:      "opensearch",
		ResourceName: f.resourceName(),
		Region:       f.region,
		Terms:        f.terms(),
	})
	if err != nil {
		return skuerrors.WriteWrap(cmd.ErrOrStderr(), skuerrors.CodeGeneric, "aws opensearch %s: %w", cmd.Use, err)
	}
	if len(rows) == 0 {
		e := skuerrors.NotFound("aws", "opensearch",
			map[string]any{"instance_type": f.instanceType, "mode": f.mode, "region": f.region},
			"Try `sku aws opensearch list` or drop --region for a list")
		skuerrors.Write(cmd.ErrOrStderr(), e)
		return e
	}
	return renderRows(cmd, rows, s)
}
