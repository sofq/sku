package sku

import (
	"context"
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/sofq/sku/internal/catalog"
	skuerrors "github.com/sofq/sku/internal/errors"
	"github.com/sofq/sku/internal/output"
)

const shardGCPFunctions = "gcp-functions"

func newGCPFunctionsCmd() *cobra.Command {
	c := &cobra.Command{Use: "functions", Short: "Google Cloud Functions (gen2) pricing"}
	c.AddCommand(newGCPFunctionsPriceCmd())
	c.AddCommand(newGCPFunctionsListCmd())
	return c
}

type gcpFunctionsFlags struct {
	architecture string
	region       string
	commitment   string
}

func (f *gcpFunctionsFlags) bind(c *cobra.Command) {
	c.Flags().StringVar(&f.architecture, "architecture", "x86_64",
		"x86_64 (only arch shipped in m3b.4)")
	c.Flags().StringVar(&f.region, "region", "", "GCP region (e.g. us-east1)")
	c.Flags().StringVar(&f.commitment, "commitment", "on_demand",
		"on_demand (only on-demand shipped in m3b.4)")
}

func (f *gcpFunctionsFlags) terms() catalog.Terms {
	return catalog.Terms{Commitment: f.commitment}
}

func newGCPFunctionsPriceCmd() *cobra.Command {
	var f gcpFunctionsFlags
	c := &cobra.Command{
		Use:   "price",
		Short: "Price Cloud Functions for one architecture + region",
		RunE:  func(cmd *cobra.Command, _ []string) error { return runGCPFunctions(cmd, &f, true) },
	}
	f.bind(c)
	return c
}

func newGCPFunctionsListCmd() *cobra.Command {
	var f gcpFunctionsFlags
	c := &cobra.Command{
		Use:   "list",
		Short: "List Cloud Functions SKUs matching filters (region optional)",
		RunE:  func(cmd *cobra.Command, _ []string) error { return runGCPFunctions(cmd, &f, false) },
	}
	f.bind(c)
	return c
}

func runGCPFunctions(cmd *cobra.Command, f *gcpFunctionsFlags, requireRegion bool) error {
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
			Command: "gcp functions " + cmd.Use,
			ResolvedArgs: map[string]any{
				"architecture": f.architecture,
				"region":       f.region,
				"commitment":   f.commitment,
			},
			Shards: []string{shardGCPFunctions},
			Preset: s.Preset,
		})
	}
	shardPath := catalog.ShardPath(shardGCPFunctions)
	if _, err := os.Stat(shardPath); err != nil {
		e := shardMissingErr(shardGCPFunctions)
		skuerrors.Write(cmd.ErrOrStderr(), e)
		return e
	}
	cat, err := catalog.Open(shardPath)
	if err != nil {
		e := &skuerrors.E{Code: skuerrors.CodeServer, Message: err.Error()}
		skuerrors.Write(cmd.ErrOrStderr(), e)
		return e
	}
	defer func() { _ = cat.Close() }()

	if stale := applyStaleGate(cmd, cat, shardGCPFunctions, s); stale != nil {
		return stale
	}

	rows, err := cat.LookupServerlessFunction(context.Background(), catalog.ServerlessFunctionFilter{
		Provider: "gcp", Service: "functions",
		Architecture: f.architecture,
		Region:       f.region,
		Terms:        f.terms(),
	})
	if err != nil {
		wrapped := fmt.Errorf("gcp functions %s: %w", cmd.Use, err)
		skuerrors.Write(cmd.ErrOrStderr(), wrapped)
		return wrapped
	}
	if len(rows) == 0 {
		e := skuerrors.NotFound("gcp", "functions",
			map[string]any{"architecture": f.architecture, "region": f.region},
			"Try `sku schema gcp functions` or drop --region for a list")
		skuerrors.Write(cmd.ErrOrStderr(), e)
		return e
	}
	return renderRows(cmd, rows, s)
}
