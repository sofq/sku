package sku

import (
	"context"

	"github.com/spf13/cobra"

	"github.com/sofq/sku/internal/catalog"
	skuerrors "github.com/sofq/sku/internal/errors"
	"github.com/sofq/sku/internal/output"
)

const shardAWSDynamoDB = "aws-dynamodb"

func newAWSDynamoDBCmd() *cobra.Command {
	c := &cobra.Command{Use: "dynamodb", Short: "AWS DynamoDB on-demand pricing"}
	c.AddCommand(newAWSDynamoDBPriceCmd())
	c.AddCommand(newAWSDynamoDBListCmd())
	return c
}

type ddbFlags struct {
	tableClass string
	region     string
	commitment string
}

func (f *ddbFlags) bind(c *cobra.Command) {
	c.Flags().StringVar(&f.tableClass, "table-class", "",
		"standard | standard-ia")
	c.Flags().StringVar(&f.region, "region", "", "AWS region (e.g. us-east-1)")
	c.Flags().StringVar(&f.commitment, "commitment", "on_demand",
		"on_demand (only on-demand shipped in m3a.3)")
}

func (f *ddbFlags) terms() catalog.Terms { return catalog.Terms{Commitment: f.commitment} }

func newAWSDynamoDBPriceCmd() *cobra.Command {
	var f ddbFlags
	c := &cobra.Command{
		Use: "price", Short: "Price one DynamoDB table class",
		RunE: func(cmd *cobra.Command, _ []string) error { return runAWSDynamoDB(cmd, &f, true) },
	}
	f.bind(c)
	return c
}

func newAWSDynamoDBListCmd() *cobra.Command {
	var f ddbFlags
	c := &cobra.Command{
		Use: "list", Short: "List DynamoDB table-class SKUs (region optional)",
		RunE: func(cmd *cobra.Command, _ []string) error { return runAWSDynamoDB(cmd, &f, false) },
	}
	f.bind(c)
	return c
}

func runAWSDynamoDB(cmd *cobra.Command, f *ddbFlags, requireRegion bool) error {
	s := globalSettings(cmd)
	if f.tableClass == "" {
		e := skuerrors.Validation("flag_invalid", "table-class", "",
			"pass --table-class <class>, e.g. --table-class standard")
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
			Command: "aws dynamodb " + cmd.Use,
			ResolvedArgs: map[string]any{
				"table_class": f.tableClass,
				"region":      f.region,
				"commitment":  f.commitment,
			},
			Shards: []string{shardAWSDynamoDB},
			Preset: s.Preset,
		})
	}
	if err := ensureShard(cmd.Context(), shardAWSDynamoDB, s.AutoFetch, cmd.ErrOrStderr()); err != nil {
		skuerrors.Write(cmd.ErrOrStderr(), err)
		return err
	}
	cat, err := catalog.Open(catalog.ShardPath(shardAWSDynamoDB))
	if err != nil {
		e := &skuerrors.E{Code: skuerrors.CodeServer, Message: err.Error()}
		skuerrors.Write(cmd.ErrOrStderr(), e)
		return e
	}
	defer func() { _ = cat.Close() }()

	if stale := applyStaleGate(cmd, cat, shardAWSDynamoDB, s); stale != nil {
		return stale
	}

	rows, err := cat.LookupNoSQLDB(context.Background(), catalog.NoSQLDBFilter{
		Provider: "aws", Service: "dynamodb",
		TableClass: f.tableClass,
		Region:     f.region,
		Terms:      f.terms(),
	})
	if err != nil {
		return skuerrors.WriteWrap(cmd.ErrOrStderr(), skuerrors.CodeGeneric, "aws dynamodb %s: %w", cmd.Use, err)
	}
	if len(rows) == 0 {
		e := skuerrors.NotFound("aws", "dynamodb",
			map[string]any{"table_class": f.tableClass, "region": f.region},
			"Try `sku schema aws dynamodb` or drop --region for a list")
		skuerrors.Write(cmd.ErrOrStderr(), e)
		return e
	}
	return renderRows(cmd, rows, s)
}
