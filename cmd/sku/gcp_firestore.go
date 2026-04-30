package sku

import (
	"context"

	"github.com/spf13/cobra"

	"github.com/sofq/sku/internal/catalog"
	skuerrors "github.com/sofq/sku/internal/errors"
	"github.com/sofq/sku/internal/output"
)

const shardGCPFirestore = "gcp-firestore"

func newGCPFirestoreCmd() *cobra.Command {
	c := &cobra.Command{Use: "firestore", Short: "GCP Cloud Firestore pricing"}
	c.AddCommand(newGCPFirestorePriceCmd())
	c.AddCommand(newGCPFirestoreListCmd())
	return c
}

type gcpFirestoreFlags struct {
	mode   string
	region string
}

func (f *gcpFirestoreFlags) bind(c *cobra.Command) {
	c.Flags().StringVar(&f.mode, "mode", "native", "Firestore mode (native)")
	c.Flags().StringVar(&f.region, "region", "", "GCP region (e.g. us-east1)")
}

func (f *gcpFirestoreFlags) terms() catalog.Terms {
	return catalog.Terms{Commitment: "on_demand", Tenancy: "firestore-native"}
}

func newGCPFirestorePriceCmd() *cobra.Command {
	var f gcpFirestoreFlags
	c := &cobra.Command{
		Use:   "price",
		Short: "Price one GCP Cloud Firestore SKU",
		RunE:  func(cmd *cobra.Command, _ []string) error { return runGCPFirestore(cmd, &f, true) },
	}
	f.bind(c)
	return c
}

func newGCPFirestoreListCmd() *cobra.Command {
	var f gcpFirestoreFlags
	c := &cobra.Command{
		Use:   "list",
		Short: "List GCP Cloud Firestore SKUs matching filters",
		RunE:  func(cmd *cobra.Command, _ []string) error { return runGCPFirestore(cmd, &f, false) },
	}
	f.bind(c)
	return c
}

func runGCPFirestore(cmd *cobra.Command, f *gcpFirestoreFlags, requireRegion bool) error {
	s := globalSettings(cmd)
	if requireRegion && f.region == "" {
		e := skuerrors.Validation("flag_invalid", "region", "", "pass --region <gcp-region>")
		skuerrors.Write(cmd.ErrOrStderr(), e)
		return e
	}
	if s.DryRun {
		return output.EmitDryRun(cmd.OutOrStdout(), output.DryRunPlan{
			Command: "gcp firestore " + cmd.Use,
			ResolvedArgs: map[string]any{
				"mode":   f.mode,
				"region": f.region,
			},
			Shards: []string{shardGCPFirestore},
			Preset: s.Preset,
		})
	}
	if err := ensureShard(cmd.Context(), shardGCPFirestore, s.AutoFetch, cmd.ErrOrStderr()); err != nil {
		skuerrors.Write(cmd.ErrOrStderr(), err)
		return err
	}
	cat, err := catalog.Open(catalog.ShardPath(shardGCPFirestore))
	if err != nil {
		e := &skuerrors.E{Code: skuerrors.CodeServer, Message: err.Error()}
		skuerrors.Write(cmd.ErrOrStderr(), e)
		return e
	}
	defer func() { _ = cat.Close() }()
	if stale := applyStaleGate(cmd, cat, shardGCPFirestore, s); stale != nil {
		return stale
	}
	rows, err := cat.LookupNoSQLDB(context.Background(), catalog.NoSQLDBFilter{
		Provider:     "gcp",
		Service:      "firestore",
		ResourceName: f.mode,
		Region:       f.region,
		Terms:        f.terms(),
	})
	if err != nil {
		return skuerrors.WriteWrap(cmd.ErrOrStderr(), skuerrors.CodeGeneric, "gcp firestore %s: %w", cmd.Use, err)
	}
	if len(rows) == 0 {
		e := skuerrors.NotFound("gcp", "firestore",
			map[string]any{"mode": f.mode, "region": f.region},
			"Try `sku gcp firestore list` or drop --region for a list")
		skuerrors.Write(cmd.ErrOrStderr(), e)
		return e
	}
	return renderRows(cmd, rows, s)
}
