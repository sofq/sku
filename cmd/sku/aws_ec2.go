package sku

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/spf13/cobra"

	"github.com/sofq/sku/internal/batch"
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

// ec2Lookup runs the shared shard-open + filter path used by both the
// standalone Cobra command and the batch handlers. Returns canonical
// *skuerrors.E envelopes on failure and []catalog.Row on success.
func ec2Lookup(ctx context.Context, f ec2Flags, requireRegion bool, s *batch.Settings) ([]catalog.Row, error) {
	if f.instanceType == "" {
		return nil, skuerrors.Validation("flag_invalid", "instance-type", "",
			"pass --instance-type <type>, e.g. --instance-type m5.large")
	}
	if requireRegion && f.region == "" {
		return nil, skuerrors.Validation("flag_invalid", "region", "",
			"pass --region <aws-region>, e.g. --region us-east-1")
	}

	autoFetch := s != nil && s.AutoFetch
	if err := ensureShard(ctx, shardAWSEC2, autoFetch, nil); err != nil {
		return nil, err
	}
	cat, err := catalog.Open(catalog.ShardPath(shardAWSEC2))
	if err != nil {
		return nil, &skuerrors.E{Code: skuerrors.CodeServer, Message: err.Error()}
	}
	defer func() { _ = cat.Close() }()

	if s != nil && s.StaleErrorDays > 0 {
		age := cat.Age(time.Now().UTC())
		if age >= s.StaleErrorDays && !s.StaleOK {
			return nil, &skuerrors.E{
				Code:       skuerrors.CodeStaleData,
				Message:    fmt.Sprintf("catalog %d days old exceeds threshold %d", age, s.StaleErrorDays),
				Suggestion: "Run: sku update " + shardAWSEC2,
				Details:    map[string]any{"shard": shardAWSEC2, "age_days": age, "threshold_days": s.StaleErrorDays},
			}
		}
	}

	if f.os == "" {
		f.os = "linux"
	}
	if f.tenancy == "" {
		f.tenancy = "shared"
	}
	if f.commitment == "" {
		f.commitment = "on_demand"
	}

	rows, err := cat.LookupVM(ctx, catalog.VMFilter{
		Provider:     "aws",
		Service:      "ec2",
		InstanceType: f.instanceType,
		Region:       f.region,
		Terms:        f.terms(),
	})
	if err != nil {
		return nil, fmt.Errorf("aws ec2: %w", err)
	}
	if len(rows) == 0 {
		return nil, skuerrors.NotFound("aws", "ec2",
			map[string]any{
				"instance_type": f.instanceType,
				"region":        f.region,
				"os":            f.os,
				"tenancy":       f.tenancy,
			},
			"Try `sku schema aws ec2` or drop --region for a list")
	}
	return rows, nil
}

func ec2FlagsFromArgs(args map[string]any) ec2Flags {
	return ec2Flags{
		instanceType: argString(args, "instance_type"),
		region:       argString(args, "region"),
		os:           argString(args, "os"),
		tenancy:      argString(args, "tenancy"),
		commitment:   argString(args, "commitment"),
	}
}

func handleAWSEC2Price(ctx context.Context, args map[string]any, env batch.Env) (any, error) {
	return ec2Lookup(ctx, ec2FlagsFromArgs(args), true, env.Settings)
}

func handleAWSEC2List(ctx context.Context, args map[string]any, env batch.Env) (any, error) {
	return ec2Lookup(ctx, ec2FlagsFromArgs(args), false, env.Settings)
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
	if _, err := os.Stat(shardPath); err == nil {
		if cat, err := catalog.Open(shardPath); err == nil {
			if staleErr := applyStaleGate(cmd, cat, shardAWSEC2, s); staleErr != nil {
				_ = cat.Close()
				return staleErr
			}
			_ = cat.Close()
		}
	}

	bs := ToBatchSettings(s)
	rows, err := ec2Lookup(context.Background(), *f, requireRegion, &bs)
	if err != nil {
		skuerrors.Write(cmd.ErrOrStderr(), err)
		return err
	}
	return renderRows(cmd, rows, s)
}
