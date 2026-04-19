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
		Short: "Estimate monthly cost from workload items (compute.vm in m5.1)",
		RunE:  func(cmd *cobra.Command, _ []string) error { return runEstimate(cmd, &f) },
	}
	c.Flags().StringArrayVar(&f.items, "item", nil, "workload item, e.g. aws/ec2:m5.large:region=us-east-1:count=10:hours=730")
	c.Flags().StringVar(&f.config, "config", "", "YAML workload file (deferred to m5.2)")
	c.Flags().BoolVar(&f.stdin, "stdin", false, "read JSON workload from stdin (deferred to m5.2)")
	return c
}

func runEstimate(cmd *cobra.Command, f *estimateFlags) error {
	s := globalSettings(cmd)

	if f.stdin {
		e := skuerrors.Validation("flag_invalid", "stdin", "true", "deferred to m5.2; pass --item repeatedly in m5.1")
		skuerrors.Write(cmd.ErrOrStderr(), e)
		return e
	}
	if f.config != "" {
		e := skuerrors.Validation("flag_invalid", "config", f.config, "deferred to m5.2; pass --item repeatedly in m5.1")
		skuerrors.Write(cmd.ErrOrStderr(), e)
		return e
	}
	if len(f.items) == 0 {
		e := skuerrors.Validation("flag_invalid", "item", "", "pass --item at least once, e.g. --item aws/ec2:m5.large:region=us-east-1")
		skuerrors.Write(cmd.ErrOrStderr(), e)
		return e
	}

	items := make([]estimate.Item, 0, len(f.items))
	for i, raw := range f.items {
		it, err := estimate.ParseItem(raw)
		if err != nil {
			e := skuerrors.Validation("flag_invalid", "item", raw, err.Error())
			e.Details["item_index"] = i
			skuerrors.Write(cmd.ErrOrStderr(), e)
			return e
		}
		items = append(items, it)
	}

	if s.DryRun {
		return output.EmitDryRun(cmd.OutOrStdout(), output.DryRunPlan{
			Command:      "estimate",
			ResolvedArgs: map[string]any{"items": f.items},
			Preset:       s.Preset,
		})
	}

	res, err := estimate.Run(context.Background(), estimate.Config{Items: items})
	if err != nil {
		wrapped := fmt.Errorf("estimate: %w", err)
		skuerrors.Write(cmd.ErrOrStderr(), wrapped)
		return wrapped
	}
	return output.EmitEstimate(cmd.OutOrStdout(), res, output.Options{
		Preset: output.Preset(s.Preset),
		Format: s.Format,
		Pretty: s.Pretty,
	})
}
