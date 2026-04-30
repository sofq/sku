package sku

import (
	"context"

	"github.com/spf13/cobra"

	"github.com/sofq/sku/internal/catalog"
	skuerrors "github.com/sofq/sku/internal/errors"
	"github.com/sofq/sku/internal/output"
)

const shardAzureEventHubs = "azure-event-hubs"

func newAzureEventHubsCmd() *cobra.Command {
	c := &cobra.Command{Use: "event-hubs", Short: "Azure Event Hubs pricing (standard + premium)"}
	c.AddCommand(newAzureEventHubsPriceCmd())
	c.AddCommand(newAzureEventHubsListCmd())
	return c
}

type eventHubsFlags struct {
	tier       string
	region     string
	commitment string
}

func (f *eventHubsFlags) bind(c *cobra.Command) {
	c.Flags().StringVar(&f.tier, "tier", "", "tier: standard | premium")
	c.Flags().StringVar(&f.region, "region", "", "Azure region (e.g. eastus)")
	c.Flags().StringVar(&f.commitment, "commitment", "on_demand", "on_demand")
}

func (f *eventHubsFlags) terms() catalog.Terms {
	return catalog.Terms{Commitment: f.commitment, OS: "event-hubs-" + f.tier}
}

func newAzureEventHubsPriceCmd() *cobra.Command {
	var f eventHubsFlags
	c := &cobra.Command{
		Use:   "price",
		Short: "Price Event Hubs throughput for a region",
		RunE:  func(cmd *cobra.Command, _ []string) error { return runAzureEventHubs(cmd, &f, true) },
	}
	f.bind(c)
	return c
}

func newAzureEventHubsListCmd() *cobra.Command {
	var f eventHubsFlags
	c := &cobra.Command{
		Use:   "list",
		Short: "List Event Hubs pricing across regions",
		RunE:  func(cmd *cobra.Command, _ []string) error { return runAzureEventHubs(cmd, &f, false) },
	}
	f.bind(c)
	return c
}

func runAzureEventHubs(cmd *cobra.Command, f *eventHubsFlags, requireRegion bool) error {
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
			Command: "azure event-hubs " + cmd.Use,
			ResolvedArgs: map[string]any{
				"tier":       f.tier,
				"region":     f.region,
				"commitment": f.commitment,
			},
			Shards: []string{shardAzureEventHubs},
			Preset: s.Preset,
		})
	}
	if err := ensureShard(cmd.Context(), shardAzureEventHubs, s.AutoFetch, cmd.ErrOrStderr()); err != nil {
		skuerrors.Write(cmd.ErrOrStderr(), err)
		return err
	}
	cat, err := catalog.Open(catalog.ShardPath(shardAzureEventHubs))
	if err != nil {
		e := &skuerrors.E{Code: skuerrors.CodeServer, Message: err.Error()}
		skuerrors.Write(cmd.ErrOrStderr(), e)
		return e
	}
	defer func() { _ = cat.Close() }()

	if stale := applyStaleGate(cmd, cat, shardAzureEventHubs, s); stale != nil {
		return stale
	}

	rows, err := cat.LookupMessagingQueue(context.Background(), catalog.MessagingQueueFilter{
		Provider:     "azure",
		Service:      "event-hubs",
		ResourceName: f.tier,
		Region:       f.region,
		Terms:        f.terms(),
	})
	if err != nil {
		return skuerrors.WriteWrap(cmd.ErrOrStderr(), skuerrors.CodeGeneric, "azure event-hubs %s: %w", cmd.Use, err)
	}
	if len(rows) == 0 {
		e := skuerrors.NotFound("azure", "event-hubs",
			map[string]any{"tier": f.tier, "region": f.region},
			"Try `sku azure event-hubs list` or check --tier / --region")
		skuerrors.Write(cmd.ErrOrStderr(), e)
		return e
	}
	return renderRows(cmd, rows, s)
}
