package sku

import (
	"context"
	"os"

	"github.com/spf13/cobra"

	"github.com/sofq/sku/internal/catalog"
	skuerrors "github.com/sofq/sku/internal/errors"
	"github.com/sofq/sku/internal/output"
)

const shardAWSS3 = "aws-s3"

func newAWSS3Cmd() *cobra.Command {
	c := &cobra.Command{Use: "s3", Short: "AWS S3 pricing"}
	c.AddCommand(newAWSS3PriceCmd())
	c.AddCommand(newAWSS3ListCmd())
	return c
}

type s3Flags struct {
	storageClass string
	region       string
	commitment   string
}

func (f *s3Flags) bind(c *cobra.Command) {
	c.Flags().StringVar(&f.storageClass, "storage-class", "",
		"standard | standard-ia | one-zone-ia | intelligent-tiering | glacier-instant")
	c.Flags().StringVar(&f.region, "region", "", "AWS region (e.g. us-east-1)")
	c.Flags().StringVar(&f.commitment, "commitment", "on_demand", "on_demand (only on-demand shipped in m3a.2)")
}

func (f *s3Flags) terms() catalog.Terms {
	return catalog.Terms{Commitment: f.commitment}
}

func newAWSS3PriceCmd() *cobra.Command {
	var f s3Flags
	c := &cobra.Command{
		Use:   "price",
		Short: "Price one S3 storage class",
		RunE:  func(cmd *cobra.Command, _ []string) error { return runAWSS3(cmd, &f, true) },
	}
	f.bind(c)
	return c
}

func newAWSS3ListCmd() *cobra.Command {
	var f s3Flags
	c := &cobra.Command{
		Use:   "list",
		Short: "List S3 storage-class SKUs matching filters (region optional)",
		RunE:  func(cmd *cobra.Command, _ []string) error { return runAWSS3(cmd, &f, false) },
	}
	f.bind(c)
	return c
}

func runAWSS3(cmd *cobra.Command, f *s3Flags, requireRegion bool) error {
	s := globalSettings(cmd)
	if f.storageClass == "" {
		e := skuerrors.Validation("flag_invalid", "storage-class", "",
			"pass --storage-class <class>, e.g. --storage-class standard")
		skuerrors.Write(cmd.ErrOrStderr(), e)
		return e
	}
	if requireRegion && f.region == "" {
		e := skuerrors.Validation("flag_invalid", "region", "",
			"pass --region <aws-region>")
		skuerrors.Write(cmd.ErrOrStderr(), e)
		return e
	}
	if s.DryRun {
		return output.EmitDryRun(cmd.OutOrStdout(), output.DryRunPlan{
			Command: "aws s3 " + cmd.Use,
			ResolvedArgs: map[string]any{
				"storage_class": f.storageClass,
				"region":        f.region,
				"commitment":    f.commitment,
			},
			Shards: []string{shardAWSS3},
			Preset: s.Preset,
		})
	}
	shardPath := catalog.ShardPath(shardAWSS3)
	if _, err := os.Stat(shardPath); err != nil {
		e := shardMissingErr(shardAWSS3)
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

	if stale := applyStaleGate(cmd, cat, shardAWSS3, s); stale != nil {
		return stale
	}

	rows, err := cat.LookupStorageObject(context.Background(), catalog.StorageObjectFilter{
		Provider: "aws", Service: "s3",
		StorageClass: f.storageClass,
		Region:       f.region,
		Terms:        f.terms(),
	})
	if err != nil {
		return skuerrors.WriteWrap(cmd.ErrOrStderr(), skuerrors.CodeGeneric, "aws s3 %s: %w", cmd.Use, err)
	}
	if len(rows) == 0 {
		e := skuerrors.NotFound("aws", "s3",
			map[string]any{
				"storage_class": f.storageClass,
				"region":        f.region,
			},
			"Try `sku schema aws s3` or drop --region for a list")
		skuerrors.Write(cmd.ErrOrStderr(), e)
		return e
	}
	return renderRows(cmd, rows, s)
}
