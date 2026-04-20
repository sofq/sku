package sku

import (
	"context"
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/sofq/sku/internal/catalog"
	skuerrors "github.com/sofq/sku/internal/errors"
	"github.com/sofq/sku/internal/output"
)

const shardAWSEBS = "aws-ebs"

func newAWSEBSCmd() *cobra.Command {
	c := &cobra.Command{Use: "ebs", Short: "AWS EBS pricing"}
	c.AddCommand(newAWSEBSPriceCmd())
	c.AddCommand(newAWSEBSListCmd())
	return c
}

type ebsFlags struct {
	volumeType string
	region     string
	commitment string
}

func (f *ebsFlags) bind(c *cobra.Command) {
	c.Flags().StringVar(&f.volumeType, "volume-type", "", "gp3 | gp2 | io2 | st1 | sc1")
	c.Flags().StringVar(&f.region, "region", "", "AWS region (e.g. us-east-1)")
	c.Flags().StringVar(&f.commitment, "commitment", "on_demand", "on_demand (only on-demand shipped in m3a.2)")
}

func (f *ebsFlags) terms() catalog.Terms {
	return catalog.Terms{Commitment: f.commitment}
}

func newAWSEBSPriceCmd() *cobra.Command {
	var f ebsFlags
	c := &cobra.Command{
		Use:   "price",
		Short: "Price one EBS volume type + region",
		RunE:  func(cmd *cobra.Command, _ []string) error { return runAWSEBS(cmd, &f, true) },
	}
	f.bind(c)
	return c
}

func newAWSEBSListCmd() *cobra.Command {
	var f ebsFlags
	c := &cobra.Command{
		Use:   "list",
		Short: "List EBS volume-type SKUs matching filters (region optional)",
		RunE:  func(cmd *cobra.Command, _ []string) error { return runAWSEBS(cmd, &f, false) },
	}
	f.bind(c)
	return c
}

func runAWSEBS(cmd *cobra.Command, f *ebsFlags, requireRegion bool) error {
	s := globalSettings(cmd)
	if f.volumeType == "" {
		e := skuerrors.Validation("flag_invalid", "volume-type", "", "pass --volume-type gp3|gp2|io2|st1|sc1")
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
			Command: "aws ebs " + cmd.Use,
			ResolvedArgs: map[string]any{
				"volume_type": f.volumeType,
				"region":      f.region,
				"commitment":  f.commitment,
			},
			Shards: []string{shardAWSEBS},
			Preset: s.Preset,
		})
	}
	shardPath := catalog.ShardPath(shardAWSEBS)
	if _, err := os.Stat(shardPath); err != nil {
		e := shardMissingErr(shardAWSEBS)
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

	if stale := applyStaleGate(cmd, cat, shardAWSEBS, s); stale != nil {
		return stale
	}

	rows, err := cat.LookupStorageBlock(context.Background(), catalog.StorageBlockFilter{
		Provider: "aws", Service: "ebs",
		VolumeType: f.volumeType,
		Region:     f.region,
		Terms:      f.terms(),
	})
	if err != nil {
		wrapped := fmt.Errorf("aws ebs %s: %w", cmd.Use, err)
		skuerrors.Write(cmd.ErrOrStderr(), wrapped)
		return wrapped
	}
	if len(rows) == 0 {
		e := skuerrors.NotFound("aws", "ebs",
			map[string]any{"volume_type": f.volumeType, "region": f.region},
			"Try `sku schema aws ebs` or drop --region for a list")
		skuerrors.Write(cmd.ErrOrStderr(), e)
		return e
	}
	return renderRows(cmd, rows, s)
}
