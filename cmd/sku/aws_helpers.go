package sku

import (
	"errors"
	"fmt"
	"time"

	"github.com/spf13/cobra"

	"github.com/sofq/sku/internal/catalog"
	"github.com/sofq/sku/internal/config"
	skuerrors "github.com/sofq/sku/internal/errors"
	"github.com/sofq/sku/internal/output"
)

func shardMissingErr(shard string) *skuerrors.E {
	return &skuerrors.E{
		Code:       skuerrors.CodeNotFound,
		Message:    fmt.Sprintf("%s shard not installed", shard),
		Suggestion: fmt.Sprintf("Run: sku update %s", shard),
		Details: map[string]any{
			"shard":        shard,
			"install_hint": "sku update " + shard,
		},
	}
}

func applyStaleGate(cmd *cobra.Command, cat *catalog.Catalog, shard string, s config.Settings) error {
	age := cat.Age(time.Now().UTC())
	if s.StaleErrorDays > 0 && age >= s.StaleErrorDays && !s.StaleOK {
		e := &skuerrors.E{
			Code:       skuerrors.CodeStaleData,
			Message:    fmt.Sprintf("catalog %d days old exceeds threshold %d", age, s.StaleErrorDays),
			Suggestion: "Run: sku update " + shard,
			Details: map[string]any{
				"shard":          shard,
				"age_days":       age,
				"threshold_days": s.StaleErrorDays,
			},
		}
		skuerrors.Write(cmd.ErrOrStderr(), e)
		return e
	}
	if s.StaleWarningDays > 0 && age >= s.StaleWarningDays && !s.StaleOK {
		_, _ = fmt.Fprintf(cmd.ErrOrStderr(),
			"warning: catalog is %d days old (warn threshold %d); run `sku update %s`\n",
			age, s.StaleWarningDays, shard)
	}
	return nil
}

func renderRows(cmd *cobra.Command, rows []catalog.Row, s config.Settings) error {
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
}
