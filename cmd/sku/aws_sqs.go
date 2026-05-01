package sku

import (
	"context"

	"github.com/spf13/cobra"

	"github.com/sofq/sku/internal/catalog"
	skuerrors "github.com/sofq/sku/internal/errors"
	"github.com/sofq/sku/internal/output"
)

const shardAWSSQS = "aws-sqs"

func newAWSSQSCmd() *cobra.Command {
	c := &cobra.Command{Use: "sqs", Short: "AWS SQS pricing (standard + FIFO)"}
	c.AddCommand(newAWSSQSPriceCmd())
	c.AddCommand(newAWSSQSListCmd())
	return c
}

type sqsFlags struct {
	queueType  string
	region     string
	commitment string
}

func (f *sqsFlags) bind(c *cobra.Command) {
	c.Flags().StringVar(&f.queueType, "queue-type", "", "queue type: standard | fifo")
	c.Flags().StringVar(&f.region, "region", "", "AWS region (e.g. us-east-1)")
	c.Flags().StringVar(&f.commitment, "commitment", "on_demand", "on_demand")
}

func (f *sqsFlags) terms() catalog.Terms { return catalog.Terms{Commitment: f.commitment} }

func newAWSSQSPriceCmd() *cobra.Command {
	var f sqsFlags
	c := &cobra.Command{
		Use:   "price",
		Short: "Price SQS request operations for a region",
		RunE:  func(cmd *cobra.Command, _ []string) error { return runAWSSQS(cmd, &f, true) },
	}
	f.bind(c)
	return c
}

func newAWSSQSListCmd() *cobra.Command {
	var f sqsFlags
	c := &cobra.Command{
		Use:   "list",
		Short: "List SQS pricing across regions",
		RunE:  func(cmd *cobra.Command, _ []string) error { return runAWSSQS(cmd, &f, false) },
	}
	f.bind(c)
	return c
}

func runAWSSQS(cmd *cobra.Command, f *sqsFlags, requireRegion bool) error {
	s := globalSettings(cmd)
	if f.queueType == "" {
		e := skuerrors.Validation("flag_invalid", "queue-type", "",
			"pass --queue-type standard | fifo")
		skuerrors.Write(cmd.ErrOrStderr(), e)
		return e
	}
	if f.queueType != "standard" && f.queueType != "fifo" {
		e := skuerrors.Validation("flag_invalid", "queue-type", f.queueType,
			"allowed: standard | fifo")
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
			Command: "aws sqs " + cmd.Use,
			ResolvedArgs: map[string]any{
				"queue_type": f.queueType,
				"region":     f.region,
				"commitment": f.commitment,
			},
			Shards: []string{shardAWSSQS},
			Preset: s.Preset,
		})
	}
	if err := ensureShard(cmd.Context(), shardAWSSQS, s.AutoFetch, cmd.ErrOrStderr()); err != nil {
		skuerrors.Write(cmd.ErrOrStderr(), err)
		return err
	}
	cat, err := catalog.Open(catalog.ShardPath(shardAWSSQS))
	if err != nil {
		e := &skuerrors.E{Code: skuerrors.CodeServer, Message: err.Error()}
		skuerrors.Write(cmd.ErrOrStderr(), e)
		return e
	}
	defer func() { _ = cat.Close() }()

	if stale := applyStaleGate(cmd, cat, shardAWSSQS, s); stale != nil {
		return stale
	}

	rows, err := cat.LookupMessagingQueue(context.Background(), catalog.MessagingQueueFilter{
		Provider:     "aws",
		Service:      "sqs",
		ResourceName: f.queueType,
		Region:       f.region,
		Terms:        f.terms(),
	})
	if err != nil {
		return skuerrors.WriteWrap(cmd.ErrOrStderr(), skuerrors.CodeGeneric, "aws sqs %s: %w", cmd.Use, err)
	}
	if len(rows) == 0 {
		e := skuerrors.NotFound("aws", "sqs",
			map[string]any{"queue_type": f.queueType, "region": f.region},
			"Try `sku aws sqs list` or check --queue-type / --region")
		skuerrors.Write(cmd.ErrOrStderr(), e)
		return e
	}
	return renderRows(cmd, rows, s)
}
