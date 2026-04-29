package sku

import (
	"context"

	"github.com/spf13/cobra"

	"github.com/sofq/sku/internal/catalog"
	skuerrors "github.com/sofq/sku/internal/errors"
	"github.com/sofq/sku/internal/output"
)

const shardAzureAppService = "azure-appservice"

func newAzureAppServiceCmd() *cobra.Command {
	c := &cobra.Command{Use: "appservice", Short: "Azure App Service pricing"}
	c.AddCommand(newAzureAppServicePriceCmd())
	c.AddCommand(newAzureAppServiceListCmd())
	return c
}

type appServiceFlags struct {
	sku    string
	region string
	os     string
	tier   string
}

func (f *appServiceFlags) bind(c *cobra.Command) {
	c.Flags().StringVar(&f.sku, "sku", "", "App Service plan SKU, e.g. P1v3, S1, B2")
	c.Flags().StringVar(&f.region, "region", "", "Azure region")
	c.Flags().StringVar(&f.os, "os", "linux", "linux | windows")
	c.Flags().StringVar(&f.tier, "tier", "", "plan tier (optional filter, e.g. standard, premium)")
}

func (f *appServiceFlags) terms() catalog.Terms {
	return catalog.Terms{OS: f.os, SupportTier: f.tier}
}

func newAzureAppServicePriceCmd() *cobra.Command {
	var f appServiceFlags
	c := &cobra.Command{
		Use:   "price",
		Short: "Price one App Service SKU",
		RunE:  func(cmd *cobra.Command, _ []string) error { return runAzureAppService(cmd, &f, true) },
	}
	f.bind(c)
	return c
}

func newAzureAppServiceListCmd() *cobra.Command {
	var f appServiceFlags
	c := &cobra.Command{
		Use:   "list",
		Short: "List App Service SKUs matching filters",
		RunE:  func(cmd *cobra.Command, _ []string) error { return runAzureAppService(cmd, &f, false) },
	}
	f.bind(c)
	return c
}

func runAzureAppService(cmd *cobra.Command, f *appServiceFlags, requireRegion bool) error {
	s := globalSettings(cmd)
	if f.os != "linux" && f.os != "windows" {
		e := skuerrors.Validation("flag_invalid", "os", f.os,
			"use --os linux or --os windows")
		skuerrors.Write(cmd.ErrOrStderr(), e)
		return e
	}
	if requireRegion && f.sku == "" {
		e := skuerrors.Validation("flag_invalid", "sku", "",
			"pass --sku <plan>, e.g. --sku P1v3")
		skuerrors.Write(cmd.ErrOrStderr(), e)
		return e
	}
	if requireRegion && f.region == "" {
		e := skuerrors.Validation("flag_invalid", "region", "", "pass --region <azure-region>")
		skuerrors.Write(cmd.ErrOrStderr(), e)
		return e
	}
	if s.DryRun {
		return output.EmitDryRun(cmd.OutOrStdout(), output.DryRunPlan{
			Command: "azure appservice " + cmd.Use,
			ResolvedArgs: map[string]any{
				"sku":    f.sku,
				"region": f.region,
				"os":     f.os,
				"tier":   f.tier,
			},
			Shards: []string{shardAzureAppService},
			Preset: s.Preset,
		})
	}
	if err := ensureShard(cmd.Context(), shardAzureAppService, s.AutoFetch, cmd.ErrOrStderr()); err != nil {
		skuerrors.Write(cmd.ErrOrStderr(), err)
		return err
	}
	cat, err := catalog.Open(catalog.ShardPath(shardAzureAppService))
	if err != nil {
		e := &skuerrors.E{Code: skuerrors.CodeServer, Message: err.Error()}
		skuerrors.Write(cmd.ErrOrStderr(), e)
		return e
	}
	defer func() { _ = cat.Close() }()
	if stale := applyStaleGate(cmd, cat, shardAzureAppService, s); stale != nil {
		return stale
	}
	rows, err := cat.LookupPaasApp(context.Background(), catalog.PaasAppFilter{
		Provider:     "azure",
		Service:      "appservice",
		ResourceName: f.sku,
		Region:       f.region,
		Terms:        f.terms(),
	})
	if err != nil {
		return skuerrors.WriteWrap(cmd.ErrOrStderr(), skuerrors.CodeGeneric, "azure appservice %s: %w", cmd.Use, err)
	}
	if len(rows) == 0 {
		e := skuerrors.NotFound("azure", "appservice",
			map[string]any{"sku": f.sku, "region": f.region, "os": f.os, "tier": f.tier},
			"Try `sku azure appservice list` or drop --region for a list")
		skuerrors.Write(cmd.ErrOrStderr(), e)
		return e
	}
	return renderRows(cmd, rows, s)
}
