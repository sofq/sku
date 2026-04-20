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

const shardGCPGCS = "gcp-gcs"

func newGCPGCSCmd() *cobra.Command {
	c := &cobra.Command{Use: "gcs", Short: "Google Cloud Storage pricing"}
	c.AddCommand(newGCPGCSPriceCmd())
	c.AddCommand(newGCPGCSListCmd())
	return c
}

type gcpGCSFlags struct {
	storageClass string // standard | nearline | coldline | archive
	region       string
	commitment   string
}

func (f *gcpGCSFlags) bind(c *cobra.Command) {
	c.Flags().StringVar(&f.storageClass, "storage-class", "",
		"standard | nearline | coldline | archive (single-region only in m3b.4)")
	c.Flags().StringVar(&f.region, "region", "", "GCP region (e.g. us-east1)")
	c.Flags().StringVar(&f.commitment, "commitment", "on_demand",
		"on_demand (only on-demand shipped in m3b.4)")
}

func (f *gcpGCSFlags) terms() catalog.Terms {
	return catalog.Terms{Commitment: f.commitment}
}

func newGCPGCSPriceCmd() *cobra.Command {
	var f gcpGCSFlags
	c := &cobra.Command{
		Use:   "price",
		Short: "Price one GCP Cloud Storage class",
		RunE:  func(cmd *cobra.Command, _ []string) error { return runGCPGCS(cmd, &f, true) },
	}
	f.bind(c)
	return c
}

func newGCPGCSListCmd() *cobra.Command {
	var f gcpGCSFlags
	c := &cobra.Command{
		Use:   "list",
		Short: "List GCP Cloud Storage SKUs matching filters (region optional)",
		RunE:  func(cmd *cobra.Command, _ []string) error { return runGCPGCS(cmd, &f, false) },
	}
	f.bind(c)
	return c
}

func runGCPGCS(cmd *cobra.Command, f *gcpGCSFlags, requireRegion bool) error {
	s := globalSettings(cmd)
	if f.storageClass == "" {
		e := skuerrors.Validation("flag_invalid", "storage-class", "",
			"pass --storage-class <class>, e.g. --storage-class standard")
		skuerrors.Write(cmd.ErrOrStderr(), e)
		return e
	}
	if requireRegion && f.region == "" {
		e := skuerrors.Validation("flag_invalid", "region", "",
			"pass --region <gcp-region>")
		skuerrors.Write(cmd.ErrOrStderr(), e)
		return e
	}
	if s.DryRun {
		return output.EmitDryRun(cmd.OutOrStdout(), output.DryRunPlan{
			Command: "gcp gcs " + cmd.Use,
			ResolvedArgs: map[string]any{
				"storage_class": f.storageClass,
				"region":        f.region,
				"commitment":    f.commitment,
			},
			Shards: []string{shardGCPGCS},
			Preset: s.Preset,
		})
	}
	shardPath := catalog.ShardPath(shardGCPGCS)
	if _, err := os.Stat(shardPath); err != nil {
		e := shardMissingErr(shardGCPGCS)
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

	if stale := applyStaleGate(cmd, cat, shardGCPGCS, s); stale != nil {
		return stale
	}

	rows, err := cat.LookupStorageObject(context.Background(), catalog.StorageObjectFilter{
		Provider: "gcp", Service: "gcs",
		StorageClass: f.storageClass,
		Region:       f.region,
		Terms:        f.terms(),
	})
	if err != nil {
		wrapped := fmt.Errorf("gcp gcs %s: %w", cmd.Use, err)
		skuerrors.Write(cmd.ErrOrStderr(), wrapped)
		return wrapped
	}
	if len(rows) == 0 {
		e := skuerrors.NotFound("gcp", "gcs",
			map[string]any{"storage_class": f.storageClass, "region": f.region},
			"Try `sku schema gcp gcs` or drop --region for a list")
		skuerrors.Write(cmd.ErrOrStderr(), e)
		return e
	}
	return renderRows(cmd, rows, s)
}
