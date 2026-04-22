package sku

import (
	"context"
	"os"

	"github.com/spf13/cobra"

	"github.com/sofq/sku/internal/catalog"
	skuerrors "github.com/sofq/sku/internal/errors"
	"github.com/sofq/sku/internal/output"
)

const shardGCPGCE = "gcp-gce"

func newGCPGCECmd() *cobra.Command {
	c := &cobra.Command{Use: "gce", Short: "Google Compute Engine pricing"}
	c.AddCommand(newGCPGCEPriceCmd())
	c.AddCommand(newGCPGCEListCmd())
	return c
}

type gcpGCEFlags struct {
	machineType string
	region      string
	os          string
	commitment  string
}

func (f *gcpGCEFlags) bind(c *cobra.Command) {
	c.Flags().StringVar(&f.machineType, "machine-type", "",
		"GCP machine type, e.g. n1-standard-2")
	c.Flags().StringVar(&f.region, "region", "", "GCP region (e.g. us-east1)")
	c.Flags().StringVar(&f.os, "os", "linux", "linux (only linux shipped in m3b.3)")
	c.Flags().StringVar(&f.commitment, "commitment", "on_demand",
		"on_demand (only on-demand shipped in m3b.3)")
}

func (f *gcpGCEFlags) terms() catalog.Terms {
	return catalog.Terms{Commitment: f.commitment, Tenancy: "shared", OS: f.os}
}

func newGCPGCEPriceCmd() *cobra.Command {
	var f gcpGCEFlags
	c := &cobra.Command{
		Use:   "price",
		Short: "Price one GCP GCE SKU",
		RunE:  func(cmd *cobra.Command, _ []string) error { return runGCPGCE(cmd, &f, true) },
	}
	f.bind(c)
	return c
}

func newGCPGCEListCmd() *cobra.Command {
	var f gcpGCEFlags
	c := &cobra.Command{
		Use:   "list",
		Short: "List GCP GCE SKUs matching filters (region optional)",
		RunE:  func(cmd *cobra.Command, _ []string) error { return runGCPGCE(cmd, &f, false) },
	}
	f.bind(c)
	return c
}

func runGCPGCE(cmd *cobra.Command, f *gcpGCEFlags, requireRegion bool) error {
	s := globalSettings(cmd)
	if f.machineType == "" {
		e := skuerrors.Validation("flag_invalid", "machine-type", "",
			"pass --machine-type <type>, e.g. --machine-type n1-standard-2")
		skuerrors.Write(cmd.ErrOrStderr(), e)
		return e
	}
	if requireRegion && f.region == "" {
		e := skuerrors.Validation("flag_invalid", "region", "",
			"pass --region <gcp-region>, e.g. --region us-east1")
		skuerrors.Write(cmd.ErrOrStderr(), e)
		return e
	}
	if s.DryRun {
		return output.EmitDryRun(cmd.OutOrStdout(), output.DryRunPlan{
			Command: "gcp gce " + cmd.Use,
			ResolvedArgs: map[string]any{
				"machine_type": f.machineType,
				"region":       f.region,
				"os":           f.os,
				"commitment":   f.commitment,
			},
			Shards: []string{shardGCPGCE},
			Preset: s.Preset,
		})
	}
	shardPath := catalog.ShardPath(shardGCPGCE)
	if _, err := os.Stat(shardPath); err != nil {
		e := shardMissingErr(shardGCPGCE)
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

	if stale := applyStaleGate(cmd, cat, shardGCPGCE, s); stale != nil {
		return stale
	}

	rows, err := cat.LookupVM(context.Background(), catalog.VMFilter{
		Provider:     "gcp",
		Service:      "gce",
		InstanceType: f.machineType,
		Region:       f.region,
		Terms:        f.terms(),
	})
	if err != nil {
		return skuerrors.WriteWrap(cmd.ErrOrStderr(), skuerrors.CodeGeneric, "gcp gce %s: %w", cmd.Use, err)
	}
	if len(rows) == 0 {
		e := skuerrors.NotFound("gcp", "gce",
			map[string]any{
				"machine_type": f.machineType,
				"region":       f.region,
				"os":           f.os,
			},
			"Try `sku schema gcp gce` or drop --region for a list")
		skuerrors.Write(cmd.ErrOrStderr(), e)
		return e
	}
	return renderRows(cmd, rows, s)
}
