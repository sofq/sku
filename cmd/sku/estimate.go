package sku

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"

	skuerrors "github.com/sofq/sku/internal/errors"
	"github.com/sofq/sku/internal/estimate"
	"github.com/sofq/sku/internal/output"
)

type estimateFlags struct {
	items  []string
	config string
	stdin  bool
}

func newEstimateCmd() *cobra.Command {
	var f estimateFlags
	c := &cobra.Command{
		Use:   "estimate",
		Short: "Estimate monthly cost from workload items (compute.vm)",
		RunE:  func(cmd *cobra.Command, _ []string) error { return runEstimate(cmd, &f) },
	}
	c.Flags().StringArrayVar(&f.items, "item", nil, "workload item, e.g. aws/ec2:m5.large:region=us-east-1:count=10:hours=730 (repeatable)")
	c.Flags().StringVar(&f.config, "config", "", "workload file (.yaml, .yml, or .json)")
	c.Flags().BoolVar(&f.stdin, "stdin", false, "read JSON workload document from stdin")
	return c
}

func runEstimate(cmd *cobra.Command, f *estimateFlags) error {
	s := globalSettings(cmd)

	sources := 0
	if len(f.items) > 0 {
		sources++
	}
	if f.config != "" {
		sources++
	}
	if f.stdin {
		sources++
	}
	if sources == 0 {
		e := skuerrors.Validation("flag_invalid", "item|config|stdin", "", "pass exactly one input form (repeat --item, --config <path>, or --stdin)")
		skuerrors.Write(cmd.ErrOrStderr(), e)
		return e
	}
	if sources > 1 {
		e := skuerrors.Validation("flag_invalid", "item|config|stdin", "", "--item, --config, and --stdin are mutually exclusive")
		skuerrors.Write(cmd.ErrOrStderr(), e)
		return e
	}

	var (
		items []estimate.Item
		err   error
	)
	switch {
	case f.stdin:
		items, err = readEstimateStdin(cmd.InOrStdin())
	case f.config != "":
		items, err = readEstimateConfig(f.config)
	default:
		items = make([]estimate.Item, 0, len(f.items))
		for i, raw := range f.items {
			it, perr := estimate.ParseItem(raw)
			if perr != nil {
				e := skuerrors.Validation("flag_invalid", "item", raw, perr.Error())
				e.Details["item_index"] = i
				skuerrors.Write(cmd.ErrOrStderr(), e)
				return e
			}
			items = append(items, it)
		}
	}
	if err != nil {
		e := skuerrors.Validation("flag_invalid", stringFlagForSource(f), "", err.Error())
		skuerrors.Write(cmd.ErrOrStderr(), e)
		return e
	}

	if s.DryRun {
		raws := make([]string, len(items))
		for i, it := range items {
			raws[i] = it.Raw
		}
		return output.EmitDryRun(cmd.OutOrStdout(), output.DryRunPlan{
			Command:      "estimate",
			ResolvedArgs: map[string]any{"items": raws},
			Preset:       s.Preset,
		})
	}

	res, rerr := estimate.Run(context.Background(), estimate.Config{Items: items})
	if rerr != nil {
		wrapped := fmt.Errorf("estimate: %w", rerr)
		skuerrors.Write(cmd.ErrOrStderr(), wrapped)
		return wrapped
	}
	return output.EmitEstimate(cmd.OutOrStdout(), res, output.Options{
		Preset: output.Preset(s.Preset),
		Format: s.Format,
		Pretty: s.Pretty,
	})
}

func stringFlagForSource(f *estimateFlags) string {
	switch {
	case f.stdin:
		return "stdin"
	case f.config != "":
		return "config"
	default:
		return "item"
	}
}
