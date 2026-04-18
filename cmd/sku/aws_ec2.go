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

const shardAWSEC2 = "aws-ec2"

func newAWSEC2Cmd() *cobra.Command {
	c := &cobra.Command{Use: "ec2", Short: "AWS EC2 pricing"}
	c.AddCommand(newAWSEC2PriceCmd())
	c.AddCommand(newAWSEC2ListCmd())
	return c
}

type ec2Flags struct {
	instanceType string
	region       string
	os           string
	tenancy      string
	commitment   string
}

func (f *ec2Flags) bind(c *cobra.Command) {
	c.Flags().StringVar(&f.instanceType, "instance-type", "", "EC2 instance type, e.g. m5.large")
	c.Flags().StringVar(&f.region, "region", "", "AWS region (e.g. us-east-1)")
	c.Flags().StringVar(&f.os, "os", "linux", "linux | windows | rhel")
	c.Flags().StringVar(&f.tenancy, "tenancy", "shared", "shared | dedicated")
	c.Flags().StringVar(&f.commitment, "commitment", "on_demand", "on_demand (only on-demand shipped in m3a.1)")
}

func (f *ec2Flags) terms() catalog.Terms {
	return catalog.Terms{Commitment: f.commitment, Tenancy: f.tenancy, OS: f.os}
}

func newAWSEC2PriceCmd() *cobra.Command {
	var f ec2Flags
	c := &cobra.Command{
		Use:   "price",
		Short: "Price one EC2 SKU",
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runAWSCompute(cmd, &f, true)
		},
	}
	f.bind(c)
	return c
}

func newAWSEC2ListCmd() *cobra.Command {
	var f ec2Flags
	c := &cobra.Command{
		Use:   "list",
		Short: "List EC2 SKUs matching filters (region optional)",
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runAWSCompute(cmd, &f, false)
		},
	}
	f.bind(c)
	return c
}

func runAWSCompute(cmd *cobra.Command, f *ec2Flags, requireRegion bool) error {
	s := globalSettings(cmd)

	if f.instanceType == "" {
		e := skuerrors.Validation("flag_invalid", "instance-type", "",
			"pass --instance-type <type>, e.g. --instance-type m5.large")
		skuerrors.Write(cmd.ErrOrStderr(), e)
		return e
	}
	if requireRegion && f.region == "" {
		e := skuerrors.Validation("flag_invalid", "region", "",
			"pass --region <aws-region>, e.g. --region us-east-1")
		skuerrors.Write(cmd.ErrOrStderr(), e)
		return e
	}

	if s.DryRun {
		return output.EmitDryRun(cmd.OutOrStdout(), output.DryRunPlan{
			Command: "aws ec2 " + cmd.Use,
			ResolvedArgs: map[string]any{
				"instance_type": f.instanceType,
				"region":        f.region,
				"os":            f.os,
				"tenancy":       f.tenancy,
				"commitment":    f.commitment,
			},
			Shards: []string{shardAWSEC2},
			Preset: s.Preset,
		})
	}

	shardPath := catalog.ShardPath(shardAWSEC2)
	if _, err := os.Stat(shardPath); err != nil {
		e := shardMissingErr(shardAWSEC2)
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

	if stale := applyStaleGate(cmd, cat, shardAWSEC2, s); stale != nil {
		return stale
	}

	rows, err := cat.LookupVM(context.Background(), catalog.VMFilter{
		Provider:     "aws",
		Service:      "ec2",
		InstanceType: f.instanceType,
		Region:       f.region,
		Terms:        f.terms(),
	})
	if err != nil {
		wrapped := fmt.Errorf("aws ec2 %s: %w", cmd.Use, err)
		skuerrors.Write(cmd.ErrOrStderr(), wrapped)
		return wrapped
	}
	if len(rows) == 0 {
		e := skuerrors.NotFound("aws", "ec2",
			map[string]any{
				"instance_type": f.instanceType,
				"region":        f.region,
				"os":            f.os,
				"tenancy":       f.tenancy,
			},
			"Try `sku schema aws ec2` or drop --region for a list")
		skuerrors.Write(cmd.ErrOrStderr(), e)
		return e
	}
	return renderRows(cmd, rows, s)
}
