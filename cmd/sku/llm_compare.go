package sku

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"time"

	"github.com/spf13/cobra"

	"github.com/sofq/sku/internal/batch"
	"github.com/sofq/sku/internal/catalog"
	skuerrors "github.com/sofq/sku/internal/errors"
	"github.com/sofq/sku/internal/output"
)

// handleLLMCompare is the shared body used by both the Cobra command and the
// batch dispatcher. Returns []catalog.Row sorted by cheapest prompt price first.
func handleLLMCompare(ctx context.Context, args map[string]any, env batch.Env) (any, error) {
	model := argString(args, "model")
	servingProvider := argString(args, "serving_provider")
	if model == "" {
		return nil, skuerrors.Validation(
			"flag_invalid", "model", "",
			"pass --model <author>/<slug>, e.g. --model anthropic/claude-opus-4.6",
		)
	}

	autoFetch := env.Settings != nil && env.Settings.AutoFetch
	if err := ensureShard(ctx, shardOpenRouter, autoFetch, env.Stderr); err != nil {
		return nil, err
	}

	cat, err := catalog.Open(catalog.ShardPath(shardOpenRouter))
	if err != nil {
		return nil, &skuerrors.E{
			Code:       skuerrors.CodeServer,
			Message:    err.Error(),
			Suggestion: "Check that the shard file is readable and not truncated",
		}
	}
	defer func() { _ = cat.Close() }()

	s := env.Settings
	age := cat.Age(time.Now().UTC())
	if s != nil && s.StaleErrorDays > 0 && age >= s.StaleErrorDays && !s.StaleOK {
		return nil, &skuerrors.E{
			Code:       skuerrors.CodeStaleData,
			Message:    fmt.Sprintf("catalog %d days old exceeds threshold %d", age, s.StaleErrorDays),
			Suggestion: "Run: sku update " + shardOpenRouter,
			Details:    map[string]any{"shard": shardOpenRouter, "age_days": age, "threshold_days": s.StaleErrorDays},
		}
	}
	if s != nil && env.Stderr != nil && s.StaleWarningDays > 0 && age >= s.StaleWarningDays && !s.StaleOK {
		_, _ = fmt.Fprintf(env.Stderr,
			"warning: catalog is %d days old (warn threshold %d); run `sku update %s`\n",
			age, s.StaleWarningDays, shardOpenRouter)
	}

	includeAggregated := s != nil && s.IncludeAggregated
	rows, err := cat.LookupLLM(ctx, catalog.LLMFilter{
		Model:             model,
		ServingProvider:   servingProvider,
		IncludeAggregated: includeAggregated,
	})
	if err != nil {
		return nil, fmt.Errorf("llm compare: %w", err)
	}
	if len(rows) == 0 {
		return nil, skuerrors.NotFound(
			shardOpenRouter, "llm",
			map[string]any{"model": model, "serving_provider": servingProvider},
			"Try `sku update openrouter` or drop --serving-provider",
		)
	}

	// Populate MinPrice from prompt dimension; fall back to minimum across all.
	for i := range rows {
		for _, p := range rows[i].Prices {
			if p.Dimension == "prompt" && (rows[i].MinPrice == 0 || p.Amount < rows[i].MinPrice) {
				rows[i].MinPrice = p.Amount
			}
		}
		if rows[i].MinPrice == 0 {
			for _, p := range rows[i].Prices {
				if rows[i].MinPrice == 0 || p.Amount < rows[i].MinPrice {
					rows[i].MinPrice = p.Amount
				}
			}
		}
	}
	sort.SliceStable(rows, func(i, j int) bool {
		return rows[i].MinPrice < rows[j].MinPrice
	})

	return rows, nil
}

func newLLMCompareCmd() *cobra.Command {
	var (
		model           string
		servingProvider string
	)
	c := &cobra.Command{
		Use:   "compare",
		Short: "Compare serving-provider costs for a model, cheapest first",
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
					Command: "llm compare",
					ResolvedArgs: map[string]any{
						"model":            model,
						"serving_provider": servingProvider,
					},
					Shards: []string{shardOpenRouter},
					Preset: s.Preset,
				})
			}

			batchSettings := ToBatchSettings(s)
			args := map[string]any{"model": model, "serving_provider": servingProvider}
			result, err := handleLLMCompare(cmd.Context(), args, batch.Env{
				Settings: &batchSettings,
				Stdout:   cmd.OutOrStdout(),
				Stderr:   cmd.ErrOrStderr(),
			})
			if err != nil {
				skuerrors.Write(cmd.ErrOrStderr(), err)
				return err
			}
			rows := result.([]catalog.Row)

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
					return skuerrors.WriteWrap(cmd.ErrOrStderr(), skuerrors.CodeGeneric, "render: %w", err)
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
