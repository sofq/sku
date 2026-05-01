package sku

import (
	"context"

	"github.com/spf13/cobra"

	"github.com/sofq/sku/internal/catalog"
	skuerrors "github.com/sofq/sku/internal/errors"
	"github.com/sofq/sku/internal/output"
)

const shardGCPPubSubQueues = "gcp-pubsub-queues"

func newGCPPubSubQueuesCmd() *cobra.Command {
	c := &cobra.Command{Use: "pubsub-queues", Short: "GCP Cloud Pub/Sub delivery pricing"}
	c.AddCommand(newGCPPubSubQueuesPriceCmd())
	c.AddCommand(newGCPPubSubQueuesListCmd())
	return c
}

type gcpPubSubQueuesFlags struct {
	mode       string
	region     string
	commitment string
}

func (f *gcpPubSubQueuesFlags) bind(c *cobra.Command) {
	c.Flags().StringVar(&f.mode, "mode", "throughput",
		"delivery mode: throughput")
	c.Flags().StringVar(&f.region, "region", "global", "region (Pub/Sub is global-only)")
	c.Flags().StringVar(&f.commitment, "commitment", "on_demand", "on_demand")
}

func (f *gcpPubSubQueuesFlags) terms() catalog.Terms {
	return catalog.Terms{Commitment: f.commitment}
}

func newGCPPubSubQueuesPriceCmd() *cobra.Command {
	var f gcpPubSubQueuesFlags
	c := &cobra.Command{
		Use:   "price",
		Short: "Price GCP Pub/Sub throughput delivery",
		RunE:  func(cmd *cobra.Command, _ []string) error { return runGCPPubSubQueues(cmd, &f, true) },
	}
	f.bind(c)
	return c
}

func newGCPPubSubQueuesListCmd() *cobra.Command {
	var f gcpPubSubQueuesFlags
	c := &cobra.Command{
		Use:   "list",
		Short: "List GCP Pub/Sub pricing",
		RunE:  func(cmd *cobra.Command, _ []string) error { return runGCPPubSubQueues(cmd, &f, false) },
	}
	f.bind(c)
	return c
}

func runGCPPubSubQueues(cmd *cobra.Command, f *gcpPubSubQueuesFlags, _ bool) error {
	s := globalSettings(cmd)
	if f.mode == "" {
		e := skuerrors.Validation("flag_invalid", "mode", "",
			"pass --mode throughput")
		skuerrors.Write(cmd.ErrOrStderr(), e)
		return e
	}
	if s.DryRun {
		return output.EmitDryRun(cmd.OutOrStdout(), output.DryRunPlan{
			Command: "gcp pubsub-queues " + cmd.Use,
			ResolvedArgs: map[string]any{
				"mode":       f.mode,
				"region":     f.region,
				"commitment": f.commitment,
			},
			Shards: []string{shardGCPPubSubQueues},
			Preset: s.Preset,
		})
	}
	if err := ensureShard(cmd.Context(), shardGCPPubSubQueues, s.AutoFetch, cmd.ErrOrStderr()); err != nil {
		skuerrors.Write(cmd.ErrOrStderr(), err)
		return err
	}
	cat, err := catalog.Open(catalog.ShardPath(shardGCPPubSubQueues))
	if err != nil {
		e := &skuerrors.E{Code: skuerrors.CodeServer, Message: err.Error()}
		skuerrors.Write(cmd.ErrOrStderr(), e)
		return e
	}
	defer func() { _ = cat.Close() }()
	if stale := applyStaleGate(cmd, cat, shardGCPPubSubQueues, s); stale != nil {
		return stale
	}
	rows, err := cat.LookupMessagingQueue(context.Background(), catalog.MessagingQueueFilter{
		Provider:     "gcp",
		Service:      "pubsub-queues",
		ResourceName: f.mode,
		Region:       f.region,
		Terms:        f.terms(),
	})
	if err != nil {
		return skuerrors.WriteWrap(cmd.ErrOrStderr(), skuerrors.CodeGeneric, "gcp pubsub-queues %s: %w", cmd.Use, err)
	}
	if len(rows) == 0 {
		e := skuerrors.NotFound("gcp", "pubsub-queues",
			map[string]any{"mode": f.mode, "region": f.region},
			"Try `sku gcp pubsub-queues list` or check --mode")
		skuerrors.Write(cmd.ErrOrStderr(), e)
		return e
	}
	return renderRows(cmd, rows, s)
}
