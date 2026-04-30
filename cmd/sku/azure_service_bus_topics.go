package sku

import (
	"context"

	"github.com/spf13/cobra"

	"github.com/sofq/sku/internal/catalog"
	skuerrors "github.com/sofq/sku/internal/errors"
	"github.com/sofq/sku/internal/output"
)

const shardAzureServiceBusTopics = "azure-service-bus-topics"

func newAzureServiceBusTopicsCmd() *cobra.Command {
	c := &cobra.Command{Use: "service-bus-topics", Short: "Azure Service Bus topic pricing (standard + premium)"}
	c.AddCommand(newAzureServiceBusTopicsPriceCmd())
	c.AddCommand(newAzureServiceBusTopicsListCmd())
	return c
}

type serviceBusTopicsFlags struct {
	tier       string
	region     string
	commitment string
}

func (f *serviceBusTopicsFlags) bind(c *cobra.Command) {
	c.Flags().StringVar(&f.tier, "tier", "", "service bus tier: standard | premium")
	c.Flags().StringVar(&f.region, "region", "", "Azure region (e.g. eastus)")
	c.Flags().StringVar(&f.commitment, "commitment", "on_demand", "on_demand")
}

func (f *serviceBusTopicsFlags) terms() catalog.Terms {
	return catalog.Terms{Commitment: f.commitment}
}

func newAzureServiceBusTopicsPriceCmd() *cobra.Command {
	var f serviceBusTopicsFlags
	c := &cobra.Command{
		Use:   "price",
		Short: "Price Azure Service Bus topic operations for a region",
		RunE:  func(cmd *cobra.Command, _ []string) error { return runAzureServiceBusTopics(cmd, &f, true) },
	}
	f.bind(c)
	return c
}

func newAzureServiceBusTopicsListCmd() *cobra.Command {
	var f serviceBusTopicsFlags
	c := &cobra.Command{
		Use:   "list",
		Short: "List Azure Service Bus topic pricing across regions",
		RunE:  func(cmd *cobra.Command, _ []string) error { return runAzureServiceBusTopics(cmd, &f, false) },
	}
	f.bind(c)
	return c
}

func runAzureServiceBusTopics(cmd *cobra.Command, f *serviceBusTopicsFlags, requireRegion bool) error {
	s := globalSettings(cmd)
	if f.tier == "" {
		e := skuerrors.Validation("flag_invalid", "tier", "",
			"pass --tier standard | premium")
		skuerrors.Write(cmd.ErrOrStderr(), e)
		return e
	}
	if f.tier != "standard" && f.tier != "premium" {
		e := skuerrors.Validation("flag_invalid", "tier", f.tier,
			"allowed: standard | premium")
		skuerrors.Write(cmd.ErrOrStderr(), e)
		return e
	}
	if requireRegion && f.region == "" {
		e := skuerrors.Validation("flag_invalid", "region", "",
			"pass --region <azure-region>")
		skuerrors.Write(cmd.ErrOrStderr(), e)
		return e
	}
	if s.DryRun {
		return output.EmitDryRun(cmd.OutOrStdout(), output.DryRunPlan{
			Command: "azure service-bus-topics " + cmd.Use,
			ResolvedArgs: map[string]any{
				"tier":       f.tier,
				"region":     f.region,
				"commitment": f.commitment,
			},
			Shards: []string{shardAzureServiceBusTopics},
			Preset: s.Preset,
		})
	}
	if err := ensureShard(cmd.Context(), shardAzureServiceBusTopics, s.AutoFetch, cmd.ErrOrStderr()); err != nil {
		skuerrors.Write(cmd.ErrOrStderr(), err)
		return err
	}
	cat, err := catalog.Open(catalog.ShardPath(shardAzureServiceBusTopics))
	if err != nil {
		e := &skuerrors.E{Code: skuerrors.CodeServer, Message: err.Error()}
		skuerrors.Write(cmd.ErrOrStderr(), e)
		return e
	}
	defer func() { _ = cat.Close() }()

	if stale := applyStaleGate(cmd, cat, shardAzureServiceBusTopics, s); stale != nil {
		return stale
	}

	rows, err := cat.LookupMessagingTopic(context.Background(), catalog.MessagingTopicFilter{
		Provider:     "azure",
		Service:      "service-bus-topics",
		ResourceName: f.tier,
		Region:       f.region,
		Terms:        f.terms(),
	})
	if err != nil {
		return skuerrors.WriteWrap(cmd.ErrOrStderr(), skuerrors.CodeGeneric, "azure service-bus-topics %s: %w", cmd.Use, err)
	}
	if len(rows) == 0 {
		e := skuerrors.NotFound("azure", "service-bus-topics",
			map[string]any{"tier": f.tier, "region": f.region},
			"Try `sku azure service-bus-topics list` or check --tier / --region")
		skuerrors.Write(cmd.ErrOrStderr(), e)
		return e
	}
	return renderRows(cmd, rows, s)
}
