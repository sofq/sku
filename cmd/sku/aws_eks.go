package sku

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/sofq/sku/internal/catalog"
	skuerrors "github.com/sofq/sku/internal/errors"
	"github.com/sofq/sku/internal/output"
)

const shardAWSEKS = "aws-eks"

func newAWSEKSCmd() *cobra.Command {
	c := &cobra.Command{Use: "eks", Short: "AWS EKS pricing (control plane + Fargate)"}
	c.AddCommand(newAWSEKSPriceCmd())
	c.AddCommand(newAWSEKSListCmd())
	return c
}

type eksFlags struct {
	tier       string
	mode       string
	region     string
	commitment string
}

func (f *eksFlags) bind(c *cobra.Command) {
	c.Flags().StringVar(&f.tier, "tier", "", "EKS tier (standard|extended-support); ignored when --mode=fargate")
	c.Flags().StringVar(&f.mode, "mode", "control-plane", "control-plane | fargate")
	c.Flags().StringVar(&f.region, "region", "", "AWS region")
	c.Flags().StringVar(&f.commitment, "commitment", "on_demand", "on_demand")
}

func (f *eksFlags) terms() catalog.Terms {
	osSlot := f.tier
	if f.mode == "fargate" {
		osSlot = "fargate"
	}
	return catalog.Terms{Commitment: f.commitment, Tenancy: "kubernetes", OS: osSlot}
}

func (f *eksFlags) resourceName() string {
	if f.mode == "fargate" {
		return "eks-fargate"
	}
	if f.tier == "" {
		return ""
	}
	return fmt.Sprintf("eks-%s", f.tier)
}

func newAWSEKSPriceCmd() *cobra.Command {
	var f eksFlags
	c := &cobra.Command{
		Use:   "price",
		Short: "Price one EKS SKU (control-plane tier or Fargate)",
		RunE:  func(cmd *cobra.Command, _ []string) error { return runAWSEKS(cmd, &f, true) },
	}
	f.bind(c)
	return c
}

func newAWSEKSListCmd() *cobra.Command {
	var f eksFlags
	c := &cobra.Command{
		Use:   "list",
		Short: "List EKS SKUs matching filters",
		RunE:  func(cmd *cobra.Command, _ []string) error { return runAWSEKS(cmd, &f, false) },
	}
	f.bind(c)
	return c
}

func runAWSEKS(cmd *cobra.Command, f *eksFlags, requireRegion bool) error {
	s := globalSettings(cmd)
	if f.mode != "control-plane" && f.mode != "fargate" {
		e := skuerrors.Validation("flag_invalid", "mode", f.mode,
			"use --mode control-plane or --mode fargate")
		skuerrors.Write(cmd.ErrOrStderr(), e)
		return e
	}
	if f.mode == "control-plane" && f.tier == "" {
		e := skuerrors.Validation("flag_invalid", "tier", "",
			"pass --tier standard or --tier extended-support (or use --mode fargate)")
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
			Command: "aws eks " + cmd.Use,
			ResolvedArgs: map[string]any{
				"tier":       f.tier,
				"mode":       f.mode,
				"region":     f.region,
				"commitment": f.commitment,
			},
			Shards: []string{shardAWSEKS},
			Preset: s.Preset,
		})
	}
	if err := ensureShard(cmd.Context(), shardAWSEKS, s.AutoFetch, cmd.ErrOrStderr()); err != nil {
		skuerrors.Write(cmd.ErrOrStderr(), err)
		return err
	}
	cat, err := catalog.Open(catalog.ShardPath(shardAWSEKS))
	if err != nil {
		e := &skuerrors.E{Code: skuerrors.CodeServer, Message: err.Error()}
		skuerrors.Write(cmd.ErrOrStderr(), e)
		return e
	}
	defer func() { _ = cat.Close() }()
	if stale := applyStaleGate(cmd, cat, shardAWSEKS, s); stale != nil {
		return stale
	}
	rows, err := cat.LookupContainerOrchestration(context.Background(), catalog.ContainerOrchestrationFilter{
		Provider:     "aws",
		Service:      "eks",
		ResourceName: f.resourceName(),
		Region:       f.region,
		Terms:        f.terms(),
	})
	if err != nil {
		return skuerrors.WriteWrap(cmd.ErrOrStderr(), skuerrors.CodeGeneric, "aws eks %s: %w", cmd.Use, err)
	}
	if len(rows) == 0 {
		e := skuerrors.NotFound("aws", "eks",
			map[string]any{"tier": f.tier, "mode": f.mode, "region": f.region},
			"Try `sku aws eks list` or drop --region for a list")
		skuerrors.Write(cmd.ErrOrStderr(), e)
		return e
	}
	return renderRows(cmd, rows, s)
}
