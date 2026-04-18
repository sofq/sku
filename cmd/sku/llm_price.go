package sku

import (
	"context"
	"errors"
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/sofq/sku/internal/catalog"
	skuerrors "github.com/sofq/sku/internal/errors"
	"github.com/sofq/sku/internal/output"
)

func newLLMPriceCmd() *cobra.Command {
	var (
		model           string
		servingProvider string
	)
	c := &cobra.Command{
		Use:   "price",
		Short: "Price one or more serving-provider options for an LLM",
		RunE: func(cmd *cobra.Command, _ []string) error {
			s := globalSettings(cmd)

			if model == "" {
				err := skuerrors.Validation(
					"flag_invalid", "model", "",
					"pass --model <author>/<slug>, e.g. --model anthropic/claude-opus-4.6",
				)
				skuerrors.Write(cmd.ErrOrStderr(), err)
				return err
			}

			if s.DryRun {
				return output.EmitDryRun(cmd.OutOrStdout(), output.DryRunPlan{
					Command: "llm price",
					ResolvedArgs: map[string]any{
						"model":            model,
						"serving_provider": servingProvider,
					},
					Shards: []string{"openrouter"},
					Preset: s.Preset,
				})
			}

			shardPath := catalog.ShardPath("openrouter")
			if _, statErr := os.Stat(shardPath); statErr != nil {
				err := &skuerrors.E{
					Code:       skuerrors.CodeNotFound,
					Message:    "openrouter shard not installed",
					Suggestion: "Run: sku update openrouter",
					Details: map[string]any{
						"shard":        "openrouter",
						"install_hint": "sku update openrouter",
					},
				}
				skuerrors.Write(cmd.ErrOrStderr(), err)
				return err
			}

			cat, err := catalog.Open(shardPath)
			if err != nil {
				skuErr := &skuerrors.E{
					Code:       skuerrors.CodeServer,
					Message:    err.Error(),
					Suggestion: "Check that the shard file is readable and not truncated",
				}
				skuerrors.Write(cmd.ErrOrStderr(), skuErr)
				return skuErr
			}
			defer func() { _ = cat.Close() }()

			if s.Verbose {
				output.Log(cmd.ErrOrStderr(), "catalog.open",
					map[string]any{"shard": "openrouter", "path": shardPath})
			}

			rows, err := cat.LookupLLM(context.Background(), catalog.LLMFilter{
				Model:             model,
				ServingProvider:   servingProvider,
				IncludeAggregated: s.IncludeAggregated,
			})
			if err != nil {
				wrappedErr := fmt.Errorf("llm price: %w", err)
				skuerrors.Write(cmd.ErrOrStderr(), wrappedErr)
				return wrappedErr
			}
			if len(rows) == 0 {
				notFoundErr := skuerrors.NotFound(
					"openrouter", "llm",
					map[string]any{
						"model":            model,
						"serving_provider": servingProvider,
					},
					"Try `sku update openrouter` or drop --serving-provider",
				)
				skuerrors.Write(cmd.ErrOrStderr(), notFoundErr)
				return notFoundErr
			}

			opts := output.Options{
				Preset:            output.Preset(s.Preset),
				Format:            s.Format,
				Pretty:            s.Pretty,
				Fields:            s.Fields,
				JQ:                s.JQ,
				IncludeRaw:        s.IncludeRaw,
				IncludeAggregated: s.IncludeAggregated,
				NoColor:           s.NoColor,
			}

			w := cmd.OutOrStdout()
			for _, r := range rows {
				b, err := output.Pipeline(r, opts)
				if errors.Is(err, output.ErrDropped) {
					continue
				}
				if err != nil {
					wrapped := fmt.Errorf("render: %w", err)
					skuerrors.Write(cmd.ErrOrStderr(), wrapped)
					return wrapped
				}
				if _, wErr := w.Write(b); wErr != nil {
					return wErr
				}
			}
			return nil
		},
	}
	c.Flags().StringVar(&model, "model", "", "Model ID, e.g. anthropic/claude-opus-4.6")
	c.Flags().StringVar(&servingProvider, "serving-provider", "", "Filter to a single serving provider (e.g. aws-bedrock)")
	return c
}
