package sku

import (
	"context"

	"github.com/spf13/cobra"

	"github.com/sofq/sku/internal/catalog"
	skuerrors "github.com/sofq/sku/internal/errors"
	"github.com/sofq/sku/internal/output"
)

const shardAzureAPIM = "azure-apim"

var azureAPIMValidTiers = map[string]bool{
	"consumption": true,
	"developer":   true,
	"basic":       true,
	"standard":    true,
	"premium":     true,
	"isolated":    true,
	"premium-v2":  true,
}

func newAzureAPIMCmd() *cobra.Command {
	c := &cobra.Command{Use: "apim", Short: "Azure API Management pricing"}
	c.AddCommand(newAzureAPIMPriceCmd())
	c.AddCommand(newAzureAPIMListCmd())
	return c
}

type azureAPIMFlags struct {
	tier   string
	region string
}

func (f *azureAPIMFlags) bind(c *cobra.Command) {
	c.Flags().StringVar(&f.tier, "tier", "", "APIM tier: consumption | developer | basic | standard | premium | isolated | premium-v2")
	c.Flags().StringVar(&f.region, "region", "", "Azure region (e.g. eastus)")
}

func (f *azureAPIMFlags) terms() catalog.Terms {
	os := ""
	switch f.tier {
	case "consumption":
		os = "apim-consumption"
	case "developer":
		os = "apim-developer"
	case "basic":
		os = "apim-basic"
	case "standard":
		os = "apim-standard"
	case "premium":
		os = "apim-premium"
	case "isolated":
		os = "apim-isolated"
	case "premium-v2":
		os = "apim-premium-v2"
	}
	return catalog.Terms{Commitment: "on_demand", OS: os}
}

func newAzureAPIMPriceCmd() *cobra.Command {
	var f azureAPIMFlags
	c := &cobra.Command{
		Use:   "price",
		Short: "Price one APIM tier",
		RunE:  func(cmd *cobra.Command, _ []string) error { return runAzureAPIM(cmd, &f, true) },
	}
	f.bind(c)
	return c
}

func newAzureAPIMListCmd() *cobra.Command {
	var f azureAPIMFlags
	c := &cobra.Command{
		Use:   "list",
		Short: "List APIM tiers matching filters",
		RunE:  func(cmd *cobra.Command, _ []string) error { return runAzureAPIM(cmd, &f, false) },
	}
	f.bind(c)
	return c
}

func runAzureAPIM(cmd *cobra.Command, f *azureAPIMFlags, requireTier bool) error {
	s := globalSettings(cmd)
	if requireTier && f.tier == "" {
		e := skuerrors.Validation("flag_invalid", "tier", "",
			"pass --tier <tier>, e.g. --tier consumption or --tier standard")
		skuerrors.Write(cmd.ErrOrStderr(), e)
		return e
	}
	if f.tier != "" && !azureAPIMValidTiers[f.tier] {
		e := skuerrors.Validation("flag_invalid", "tier", f.tier,
			"valid values: consumption, developer, basic, standard, premium, isolated, premium-v2")
		skuerrors.Write(cmd.ErrOrStderr(), e)
		return e
	}
	if requireTier && f.region == "" {
		e := skuerrors.Validation("flag_invalid", "region", "", "pass --region <azure-region>")
		skuerrors.Write(cmd.ErrOrStderr(), e)
		return e
	}
	if s.DryRun {
		return output.EmitDryRun(cmd.OutOrStdout(), output.DryRunPlan{
			Command: "azure apim " + cmd.Use,
			ResolvedArgs: map[string]any{
				"tier":   f.tier,
				"region": f.region,
			},
			Shards: []string{shardAzureAPIM},
			Preset: s.Preset,
		})
	}
	if err := ensureShard(cmd.Context(), shardAzureAPIM, s.AutoFetch, cmd.ErrOrStderr()); err != nil {
		skuerrors.Write(cmd.ErrOrStderr(), err)
		return err
	}
	cat, err := catalog.Open(catalog.ShardPath(shardAzureAPIM))
	if err != nil {
		e := &skuerrors.E{Code: skuerrors.CodeServer, Message: err.Error()}
		skuerrors.Write(cmd.ErrOrStderr(), e)
		return e
	}
	defer func() { _ = cat.Close() }()
	if stale := applyStaleGate(cmd, cat, shardAzureAPIM, s); stale != nil {
		return stale
	}
	rows, err := cat.LookupAPIGateway(context.Background(), catalog.APIGatewayFilter{
		Provider:     "azure",
		Service:      "apim",
		ResourceName: f.tier,
		Region:       f.region,
		Terms:        f.terms(),
	})
	if err != nil {
		return skuerrors.WriteWrap(cmd.ErrOrStderr(), skuerrors.CodeGeneric, "azure apim %s: %w", cmd.Use, err)
	}
	if len(rows) == 0 {
		e := skuerrors.NotFound("azure", "apim",
			map[string]any{"tier": f.tier, "region": f.region},
			"Try `sku azure apim list` or drop --region for a list")
		skuerrors.Write(cmd.ErrOrStderr(), e)
		return e
	}
	return renderRows(cmd, rows, s)
}
