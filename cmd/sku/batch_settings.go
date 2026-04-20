package sku

import (
	"github.com/sofq/sku/internal/batch"
	"github.com/sofq/sku/internal/config"
)

// ToBatchSettings copies the subset of resolved globals that batch handlers
// need. Every field is a straight copy; no flag parsing happens here.
func ToBatchSettings(g config.Settings) batch.Settings {
	return batch.Settings{
		Preset:            g.Preset,
		Profile:           g.Profile,
		Format:            g.Format,
		Pretty:            g.Pretty,
		JQ:                g.JQ,
		Fields:            g.Fields,
		IncludeRaw:        g.IncludeRaw,
		IncludeAggregated: g.IncludeAggregated,
		StaleOK:           g.StaleOK,
		StaleWarningDays:  g.StaleWarningDays,
		StaleErrorDays:    g.StaleErrorDays,
		DryRun:            g.DryRun,
		Verbose:           g.Verbose,
		NoColor:           g.NoColor,
	}
}
