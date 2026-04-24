package sku

import (
	"context"

	"github.com/spf13/cobra"

	"github.com/sofq/sku/internal/catalog"
	skuerrors "github.com/sofq/sku/internal/errors"
	"github.com/sofq/sku/internal/output"
)

const shardGCPRun = "gcp-run"

func newGCPRunCmd() *cobra.Command {
	c := &cobra.Command{Use: "run", Short: "Google Cloud Run (gen2) pricing"}
	c.AddCommand(newGCPRunPriceCmd())
	c.AddCommand(newGCPRunListCmd())
	return c
}

type gcpRunFlags struct {
	architecture string
	region       string
	commitment   string
}

func (f *gcpRunFlags) bind(c *cobra.Command) {
	c.Flags().StringVar(&f.architecture, "architecture", "x86_64",
		"x86_64 (only arch shipped in m3b.4)")
	c.Flags().StringVar(&f.region, "region", "", "GCP region (e.g. us-east1)")
	c.Flags().StringVar(&f.commitment, "commitment", "on_demand",
		"on_demand (only on-demand shipped in m3b.4)")
}

func (f *gcpRunFlags) terms() catalog.Terms {
	return catalog.Terms{Commitment: f.commitment}
}

func newGCPRunPriceCmd() *cobra.Command {
	var f gcpRunFlags
	c := &cobra.Command{
		Use:   "price",
		Short: "Price Cloud Run for one architecture + region",
		RunE:  func(cmd *cobra.Command, _ []string) error { return runGCPRun(cmd, &f, true) },
	}
	f.bind(c)
	return c
}

func newGCPRunListCmd() *cobra.Command {
	var f gcpRunFlags
	c := &cobra.Command{
		Use:   "list",
		Short: "List Cloud Run SKUs matching filters (region optional)",
		RunE:  func(cmd *cobra.Command, _ []string) error { return runGCPRun(cmd, &f, false) },
	}
	f.bind(c)
	return c
}

func runGCPRun(cmd *cobra.Command, f *gcpRunFlags, requireRegion bool) error {
	s := globalSettings(cmd)
	if f.architecture == "" {
		e := skuerrors.Validation("flag_invalid", "architecture", "",
			"pass --architecture x86_64")
		skuerrors.Write(cmd.ErrOrStderr(), e)
		return e
	}
	if requireRegion && f.region == "" {
		e := skuerrors.Validation("flag_invalid", "region", "", "pass --region <gcp-region>")
		skuerrors.Write(cmd.ErrOrStderr(), e)
		return e
	}
	if s.DryRun {
		return output.EmitDryRun(cmd.OutOrStdout(), output.DryRunPlan{
			Command: "gcp run " + cmd.Use,
			ResolvedArgs: map[string]any{
				"architecture": f.architecture,
				"region":       f.region,
				"commitment":   f.commitment,
			},
			Shards: []string{shardGCPRun},
			Preset: s.Preset,
		})
	}
	if err := ensureShard(cmd.Context(), shardGCPRun, s.AutoFetch, cmd.ErrOrStderr()); err != nil {
		skuerrors.Write(cmd.ErrOrStderr(), err)
		return err
	}
	cat, err := catalog.Open(catalog.ShardPath(shardGCPRun))
	if err != nil {
		e := &skuerrors.E{Code: skuerrors.CodeServer, Message: err.Error()}
		skuerrors.Write(cmd.ErrOrStderr(), e)
		return e
	}
	defer func() { _ = cat.Close() }()

	if stale := applyStaleGate(cmd, cat, shardGCPRun, s); stale != nil {
		return stale
	}

	rows, err := cat.LookupServerlessFunction(context.Background(), catalog.ServerlessFunctionFilter{
		Provider: "gcp", Service: "run",
		Architecture: f.architecture,
		Region:       f.region,
		Terms:        f.terms(),
	})
	if err != nil {
		return skuerrors.WriteWrap(cmd.ErrOrStderr(), skuerrors.CodeGeneric, "gcp run %s: %w", cmd.Use, err)
	}
	if len(rows) == 0 {
		e := skuerrors.NotFound("gcp", "run",
			map[string]any{"architecture": f.architecture, "region": f.region},
			"Try `sku schema gcp run` or drop --region for a list")
		skuerrors.Write(cmd.ErrOrStderr(), e)
		return e
	}
	return renderRows(cmd, rows, s)
}
