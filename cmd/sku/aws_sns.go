package sku

import (
	"context"

	"github.com/spf13/cobra"

	"github.com/sofq/sku/internal/catalog"
	skuerrors "github.com/sofq/sku/internal/errors"
	"github.com/sofq/sku/internal/output"
)

const shardAWSSNS = "aws-sns"

func newAWSSNSCmd() *cobra.Command {
	c := &cobra.Command{Use: "sns", Short: "AWS SNS publish-request pricing"}
	c.AddCommand(newAWSSNSPriceCmd())
	c.AddCommand(newAWSSNSListCmd())
	return c
}

type snsFlags struct {
	region     string
	commitment string
}

func (f *snsFlags) bind(c *cobra.Command) {
	c.Flags().StringVar(&f.region, "region", "", "AWS region (e.g. us-east-1)")
	c.Flags().StringVar(&f.commitment, "commitment", "on_demand", "on_demand")
}

func (f *snsFlags) terms() catalog.Terms { return catalog.Terms{Commitment: f.commitment} }

func newAWSSNSPriceCmd() *cobra.Command {
	var f snsFlags
	c := &cobra.Command{
		Use:   "price",
		Short: "Price SNS publish requests for a region",
		RunE:  func(cmd *cobra.Command, _ []string) error { return runAWSSNS(cmd, &f, true) },
	}
	f.bind(c)
	return c
}

func newAWSSNSListCmd() *cobra.Command {
	var f snsFlags
	c := &cobra.Command{
		Use:   "list",
		Short: "List SNS pricing (region optional)",
		RunE:  func(cmd *cobra.Command, _ []string) error { return runAWSSNS(cmd, &f, false) },
	}
	f.bind(c)
	return c
}

func runAWSSNS(cmd *cobra.Command, f *snsFlags, requireRegion bool) error {
	s := globalSettings(cmd)
	if requireRegion && f.region == "" {
		e := skuerrors.Validation("flag_invalid", "region", "",
			"pass --region <aws-region>")
		skuerrors.Write(cmd.ErrOrStderr(), e)
		return e
	}
	if s.DryRun {
		return output.EmitDryRun(cmd.OutOrStdout(), output.DryRunPlan{
			Command: "aws sns " + cmd.Use,
			ResolvedArgs: map[string]any{
				"region":     f.region,
				"commitment": f.commitment,
			},
			Shards: []string{shardAWSSNS},
			Preset: s.Preset,
		})
	}
	if err := ensureShard(cmd.Context(), shardAWSSNS, s.AutoFetch, cmd.ErrOrStderr()); err != nil {
		skuerrors.Write(cmd.ErrOrStderr(), err)
		return err
	}
	cat, err := catalog.Open(catalog.ShardPath(shardAWSSNS))
	if err != nil {
		e := &skuerrors.E{Code: skuerrors.CodeServer, Message: err.Error()}
		skuerrors.Write(cmd.ErrOrStderr(), e)
		return e
	}
	defer func() { _ = cat.Close() }()

	if stale := applyStaleGate(cmd, cat, shardAWSSNS, s); stale != nil {
		return stale
	}

	rows, err := cat.LookupMessagingTopic(context.Background(), catalog.MessagingTopicFilter{
		Provider:     "aws",
		Service:      "sns",
		ResourceName: "standard",
		Region:       f.region,
		Terms:        f.terms(),
	})
	if err != nil {
		return skuerrors.WriteWrap(cmd.ErrOrStderr(), skuerrors.CodeGeneric, "aws sns %s: %w", cmd.Use, err)
	}
	if len(rows) == 0 {
		e := skuerrors.NotFound("aws", "sns",
			map[string]any{"region": f.region},
			"Try `sku aws sns list` or drop --region for a list")
		skuerrors.Write(cmd.ErrOrStderr(), e)
		return e
	}
	return renderRows(cmd, rows, s)
}
