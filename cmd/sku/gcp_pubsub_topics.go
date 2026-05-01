package sku

import (
	"context"

	"github.com/spf13/cobra"

	"github.com/sofq/sku/internal/catalog"
	skuerrors "github.com/sofq/sku/internal/errors"
	"github.com/sofq/sku/internal/output"
)

const shardGCPPubSubTopics = "gcp-pubsub-topics"

func newGCPPubSubTopicsCmd() *cobra.Command {
	c := &cobra.Command{Use: "pubsub-topics", Short: "GCP Cloud Pub/Sub topic pricing"}
	c.AddCommand(newGCPPubSubTopicsPriceCmd())
	c.AddCommand(newGCPPubSubTopicsListCmd())
	return c
}

type gcpPubSubTopicsFlags struct {
	mode       string
	region     string
	commitment string
}

func (f *gcpPubSubTopicsFlags) bind(c *cobra.Command) {
	c.Flags().StringVar(&f.mode, "mode", "throughput",
		"delivery mode: throughput")
	c.Flags().StringVar(&f.region, "region", "global", "region (Pub/Sub is global-only)")
	c.Flags().StringVar(&f.commitment, "commitment", "on_demand", "on_demand")
}

func (f *gcpPubSubTopicsFlags) terms() catalog.Terms {
	return catalog.Terms{Commitment: f.commitment}
}

func newGCPPubSubTopicsPriceCmd() *cobra.Command {
	var f gcpPubSubTopicsFlags
	c := &cobra.Command{
		Use:   "price",
		Short: "Price GCP Pub/Sub topic throughput delivery",
		RunE:  func(cmd *cobra.Command, _ []string) error { return runGCPPubSubTopics(cmd, &f, true) },
	}
	f.bind(c)
	return c
}

func newGCPPubSubTopicsListCmd() *cobra.Command {
	var f gcpPubSubTopicsFlags
	c := &cobra.Command{
		Use:   "list",
		Short: "List GCP Pub/Sub topic pricing",
		RunE:  func(cmd *cobra.Command, _ []string) error { return runGCPPubSubTopics(cmd, &f, false) },
	}
	f.bind(c)
	return c
}

func runGCPPubSubTopics(cmd *cobra.Command, f *gcpPubSubTopicsFlags, _ bool) error {
	s := globalSettings(cmd)
	if f.mode == "" {
		e := skuerrors.Validation("flag_invalid", "mode", "",
			"pass --mode throughput")
		skuerrors.Write(cmd.ErrOrStderr(), e)
		return e
	}
	if s.DryRun {
		return output.EmitDryRun(cmd.OutOrStdout(), output.DryRunPlan{
			Command: "gcp pubsub-topics " + cmd.Use,
			ResolvedArgs: map[string]any{
				"mode":       f.mode,
				"region":     f.region,
				"commitment": f.commitment,
			},
			Shards: []string{shardGCPPubSubTopics},
			Preset: s.Preset,
		})
	}
	if err := ensureShard(cmd.Context(), shardGCPPubSubTopics, s.AutoFetch, cmd.ErrOrStderr()); err != nil {
		skuerrors.Write(cmd.ErrOrStderr(), err)
		return err
	}
	cat, err := catalog.Open(catalog.ShardPath(shardGCPPubSubTopics))
	if err != nil {
		e := &skuerrors.E{Code: skuerrors.CodeServer, Message: err.Error()}
		skuerrors.Write(cmd.ErrOrStderr(), e)
		return e
	}
	defer func() { _ = cat.Close() }()
	if stale := applyStaleGate(cmd, cat, shardGCPPubSubTopics, s); stale != nil {
		return stale
	}
	rows, err := cat.LookupMessagingTopic(context.Background(), catalog.MessagingTopicFilter{
		Provider:     "gcp",
		Service:      "pubsub-topics",
		ResourceName: f.mode,
		Region:       f.region,
		Terms:        f.terms(),
	})
	if err != nil {
		return skuerrors.WriteWrap(cmd.ErrOrStderr(), skuerrors.CodeGeneric, "gcp pubsub-topics %s: %w", cmd.Use, err)
	}
	if len(rows) == 0 {
		e := skuerrors.NotFound("gcp", "pubsub-topics",
			map[string]any{"mode": f.mode, "region": f.region},
			"Try `sku gcp pubsub-topics list` or check --mode")
		skuerrors.Write(cmd.ErrOrStderr(), e)
		return e
	}
	return renderRows(cmd, rows, s)
}
