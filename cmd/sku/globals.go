package sku

import (
	"os"

	"github.com/spf13/cobra"

	"github.com/sofq/sku/internal/config"
	skuerrors "github.com/sofq/sku/internal/errors"
)

// validPresets enumerates the accepted --preset values (spec §3). Kept in
// sync with the pf.String help text on root.
var validPresets = map[string]struct{}{
	"agent":   {},
	"full":    {},
	"price":   {},
	"compare": {},
}

type settingsKeyT struct{}

var settingsKey settingsKeyT

// globalSettings returns the resolved Settings stashed on the command's
// context by PersistentPreRunE. Leaf commands always call this after
// flags parse. Unused until Task 10+ wires leaf commands through it.
//
//nolint:unused // consumed by later M2 tasks (see plan §Task 10-17)
func globalSettings(cmd *cobra.Command) config.Settings {
	if v, ok := cmd.Context().Value(settingsKey).(config.Settings); ok {
		return v
	}
	return config.Settings{}
}

// envMap snapshots the SKU_* / NO_COLOR env vars needed by
// config.Resolve. Extracted so tests can feed a deterministic map.
func envMap() map[string]string {
	keys := []string{
		"SKU_PROFILE", "SKU_PRESET", "SKU_FORMAT",
		"SKU_JQ", "SKU_FIELDS",
		"SKU_AUTO_FETCH", "SKU_STALE_OK",
		"SKU_STALE_WARNING_DAYS", "SKU_STALE_ERROR_DAYS",
		"SKU_NO_COLOR", "NO_COLOR",
		"SKU_PRETTY", "SKU_VERBOSE", "SKU_DRY_RUN",
		"SKU_INCLUDE_RAW", "SKU_INCLUDE_AGGREGATED",
	}
	m := make(map[string]string, len(keys))
	for _, k := range keys {
		if v, ok := os.LookupEnv(k); ok {
			m[k] = v
		}
	}
	return m
}

// resolveSettings reads persistent flags off root, loads the config
// file, then runs config.Resolve. Call from PersistentPreRunE.
func resolveSettings(cmd *cobra.Command) (config.Settings, error) {
	fb := readFlagBag(cmd)
	file, err := config.Load(config.Path())
	if err != nil {
		return config.Settings{}, err
	}
	s, err := config.Resolve(fb, file, envMap())
	if err != nil {
		return s, err
	}
	if _, ok := validPresets[s.Preset]; !ok {
		return s, &skuerrors.E{
			Code:       skuerrors.CodeValidation,
			Message:    "invalid --preset: " + s.Preset,
			Suggestion: "use one of: agent, full, price, compare",
			Details: map[string]any{
				"reason": "bad_preset",
				"flag":   "preset",
				"value":  s.Preset,
			},
		}
	}
	return s, nil
}

func readFlagBag(cmd *cobra.Command) config.FlagBag {
	f := cmd.Flags() // includes inherited persistent flags
	var fb config.FlagBag
	getStr := func(name string, dst *string, set *bool) {
		if fl := f.Lookup(name); fl != nil {
			*dst = fl.Value.String()
			*set = fl.Changed
		}
	}
	getBool := func(name string, dst, set *bool) {
		if fl := f.Lookup(name); fl != nil {
			*dst = fl.Value.String() == "true"
			*set = fl.Changed
		}
	}
	getStr("profile", &fb.Profile, &fb.ProfileSet)
	getStr("preset", &fb.Preset, &fb.PresetSet)
	getStr("jq", &fb.JQ, &fb.JQSet)
	getStr("fields", &fb.Fields, &fb.FieldsSet)
	getBool("pretty", &fb.Pretty, &fb.PrettySet)
	getBool("include-raw", &fb.IncludeRaw, &fb.IncludeRawSet)
	getBool("include-aggregated", &fb.IncludeAggregated, &fb.IncludeAggregatedSet)
	getBool("auto-fetch", &fb.AutoFetch, &fb.AutoFetchSet)
	getBool("stale-ok", &fb.StaleOK, &fb.StaleOKSet)
	getBool("dry-run", &fb.DryRun, &fb.DryRunSet)
	getBool("verbose", &fb.Verbose, &fb.VerboseSet)
	getBool("no-color", &fb.NoColor, &fb.NoColorSet)

	// Output format trio: --json / --yaml / --toml are mutually
	// exclusive. When Changed, the last one wins in flag-parse order.
	if fl := f.Lookup("json"); fl != nil && fl.Changed {
		fb.Format, fb.FormatSet = "json", true
	}
	if fl := f.Lookup("yaml"); fl != nil && fl.Changed {
		fb.Format, fb.FormatSet = "yaml", true
	}
	if fl := f.Lookup("toml"); fl != nil && fl.Changed {
		fb.Format, fb.FormatSet = "toml", true
	}
	return fb
}
