package sku

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/sofq/sku/internal/catalog"
	"github.com/sofq/sku/internal/config"
	skuerrors "github.com/sofq/sku/internal/errors"
	"github.com/sofq/sku/internal/output"
	"github.com/sofq/sku/internal/updater"
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

// autoFetchShard downloads shard using the manifest-based updater.
func autoFetchShard(ctx context.Context, shard string, stderr io.Writer) error {
	if stderr != nil {
		_, _ = fmt.Fprintf(stderr, "auto-fetch: downloading %s shard...\n", shard)
	}
	// No fallback: SKU_UPDATE_BASE_URL fully controls the manifest URL in tests;
	// production uses the default GitHub URL directly.
	manifestSrc := updater.NewHTTPSource(resolveManifestPrimaryURL(), "", nil)
	_, err := updater.Update(ctx, shard, updater.UpdateOptions{
		Options:  updater.Options{DestDir: catalog.DataDir()},
		Channel:  updater.ChannelStable,
		Manifest: manifestSrc,
		MaxChain: 20,
	})
	if err != nil {
		msg := err.Error()
		if idx := strings.Index(msg, ": "); idx >= 0 {
			msg = msg[idx+2:]
		}
		return &skuerrors.E{
			Code:       skuerrors.CodeServer,
			Message:    fmt.Sprintf("auto-fetch %s: %s", shard, msg),
			Suggestion: fmt.Sprintf("Run: sku update %s", shard),
		}
	}
	return nil
}

// ensureShard returns nil if the shard DB exists. If not and autoFetch is true
// it downloads via updater.Update; otherwise returns shardMissingErr.
func ensureShard(ctx context.Context, shard string, autoFetch bool, stderr io.Writer) error {
	if _, err := os.Stat(catalog.ShardPath(shard)); err == nil {
		return nil
	}
	if !autoFetch {
		return shardMissingErr(shard)
	}
	return autoFetchShard(ctx, shard, stderr)
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
			return skuerrors.WriteWrap(cmd.ErrOrStderr(), skuerrors.CodeGeneric, "render: %w", err)
		}
		if _, wErr := w.Write(b); wErr != nil {
			return wErr
		}
	}
	return nil
}
