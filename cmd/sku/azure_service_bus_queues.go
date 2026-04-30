package sku

import (
	"context"

	"github.com/spf13/cobra"

	"github.com/sofq/sku/internal/catalog"
	skuerrors "github.com/sofq/sku/internal/errors"
	"github.com/sofq/sku/internal/output"
)

const shardAzureServiceBusQueues = "azure-service-bus-queues"

func newAzureServiceBusQueuesCmd() *cobra.Command {
	c := &cobra.Command{Use: "service-bus-queues", Short: "Azure Service Bus queue pricing (standard + premium)"}
	c.AddCommand(newAzureServiceBusQueuesPriceCmd())
	c.AddCommand(newAzureServiceBusQueuesListCmd())
	return c
}

type serviceBusQueuesFlags struct {
	tier       string
	region     string
	commitment string
}

func (f *serviceBusQueuesFlags) bind(c *cobra.Command) {
	c.Flags().StringVar(&f.tier, "tier", "", "service bus tier: standard | premium")
	c.Flags().StringVar(&f.region, "region", "", "Azure region (e.g. eastus)")
	c.Flags().StringVar(&f.commitment, "commitment", "on_demand", "on_demand")
}

func (f *serviceBusQueuesFlags) terms() catalog.Terms {
	return catalog.Terms{Commitment: f.commitment, OS: f.tier}
}

func newAzureServiceBusQueuesPriceCmd() *cobra.Command {
	var f serviceBusQueuesFlags
	c := &cobra.Command{
		Use:   "price",
		Short: "Price Azure Service Bus queue operations for a region",
		RunE:  func(cmd *cobra.Command, _ []string) error { return runAzureServiceBusQueues(cmd, &f, true) },
	}
	f.bind(c)
	return c
}

func newAzureServiceBusQueuesListCmd() *cobra.Command {
	var f serviceBusQueuesFlags
	c := &cobra.Command{
		Use:   "list",
		Short: "List Azure Service Bus queue pricing across regions",
		RunE:  func(cmd *cobra.Command, _ []string) error { return runAzureServiceBusQueues(cmd, &f, false) },
	}
	f.bind(c)
	return c
}

func runAzureServiceBusQueues(cmd *cobra.Command, f *serviceBusQueuesFlags, requireRegion bool) error {
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
			Command: "azure service-bus-queues " + cmd.Use,
			ResolvedArgs: map[string]any{
				"tier":       f.tier,
				"region":     f.region,
				"commitment": f.commitment,
			},
			Shards: []string{shardAzureServiceBusQueues},
			Preset: s.Preset,
		})
	}
	if err := ensureShard(cmd.Context(), shardAzureServiceBusQueues, s.AutoFetch, cmd.ErrOrStderr()); err != nil {
		skuerrors.Write(cmd.ErrOrStderr(), err)
		return err
	}
	cat, err := catalog.Open(catalog.ShardPath(shardAzureServiceBusQueues))
	if err != nil {
		e := &skuerrors.E{Code: skuerrors.CodeServer, Message: err.Error()}
		skuerrors.Write(cmd.ErrOrStderr(), e)
		return e
	}
	defer func() { _ = cat.Close() }()

	if stale := applyStaleGate(cmd, cat, shardAzureServiceBusQueues, s); stale != nil {
		return stale
	}

	rows, err := cat.LookupMessagingQueue(context.Background(), catalog.MessagingQueueFilter{
		Provider:     "azure",
		Service:      "service-bus-queues",
		ResourceName: f.tier,
		Region:       f.region,
		Terms:        f.terms(),
	})
	if err != nil {
		return skuerrors.WriteWrap(cmd.ErrOrStderr(), skuerrors.CodeGeneric, "azure service-bus-queues %s: %w", cmd.Use, err)
	}
	if len(rows) == 0 {
		e := skuerrors.NotFound("azure", "service-bus-queues",
			map[string]any{"tier": f.tier, "region": f.region},
			"Try `sku azure service-bus-queues list` or check --tier / --region")
		skuerrors.Write(cmd.ErrOrStderr(), e)
		return e
	}
	return renderRows(cmd, rows, s)
}
