package config

import (
	"fmt"
	"strconv"
	"strings"
)

// Settings is the resolved per-invocation configuration fed to command
// leaves. It always contains concrete values (defaults applied), so leaf
// code never needs to worry about "unset vs. zero".
type Settings struct {
	Profile           string // which profile was selected ("" if none)
	Preset            string // "agent" | "price" | "full" | "compare"
	Format            string // "json" | "yaml" | "toml"
	Pretty            bool
	JQ                string
	Fields            string
	IncludeRaw        bool
	IncludeAggregated bool
	AutoFetch         bool
	StaleOK           bool
	StaleWarningDays  int
	StaleErrorDays    int // 0 = disabled
	NoColor           bool
	Verbose           bool
	DryRun            bool
}

// FlagBag carries raw pflag values along with "was this flag explicitly
// set". Resolve differentiates unset-flag (fall through to env/profile)
// from an explicit zero value (e.g. --stale-error-days=0 means "disable",
// not "unset").
type FlagBag struct {
	Profile    string
	ProfileSet bool
	Preset     string
	PresetSet  bool
	Format     string
	FormatSet  bool
	JQ         string
	JQSet      bool
	Fields     string
	FieldsSet  bool

	Pretty               bool
	PrettySet            bool
	IncludeRaw           bool
	IncludeRawSet        bool
	IncludeAggregated    bool
	IncludeAggregatedSet bool
	AutoFetch            bool
	AutoFetchSet         bool
	StaleOK              bool
	StaleOKSet           bool
	NoColor              bool
	NoColorSet           bool
	Verbose              bool
	VerboseSet           bool
	DryRun               bool
	DryRunSet            bool
}

// Resolve applies spec §4 precedence (CLI > env > profile > default) and
// returns a fully-populated Settings. env is an injected map so tests do
// not mutate process-level environment state.
func Resolve(fb FlagBag, file File, env map[string]string) (Settings, error) {
	// Profile selection.
	profileName := ""
	switch {
	case fb.ProfileSet && fb.Profile != "":
		profileName = fb.Profile
		if _, ok := file.Profiles[profileName]; !ok {
			return Settings{}, fmt.Errorf("config: unknown profile %q", profileName)
		}
	case env["SKU_PROFILE"] != "":
		profileName = env["SKU_PROFILE"]
		if _, ok := file.Profiles[profileName]; !ok {
			return Settings{}, fmt.Errorf("config: unknown profile %q", profileName)
		}
	default:
		if _, ok := file.Profiles["default"]; ok {
			profileName = "default"
		}
	}
	profile := file.Profiles[profileName]

	s := Settings{Profile: profileName}

	// String fields: CLI > env > profile > default.
	s.Preset = pickString(fb.PresetSet, fb.Preset, env["SKU_PRESET"], profile.Preset, "agent")
	s.Format = pickString(fb.FormatSet, fb.Format, env["SKU_FORMAT"], "", "json")
	s.JQ = pickString(fb.JQSet, fb.JQ, env["SKU_JQ"], "", "")
	s.Fields = pickString(fb.FieldsSet, fb.Fields, env["SKU_FIELDS"], "", "")

	// Bool fields from flag > env > profile > false.
	s.Pretty = pickBool(fb.PrettySet, fb.Pretty, env["SKU_PRETTY"], nil, false)
	s.IncludeRaw = pickBool(fb.IncludeRawSet, fb.IncludeRaw, env["SKU_INCLUDE_RAW"], profile.IncludeRaw, false)
	s.IncludeAggregated = pickBool(fb.IncludeAggregatedSet, fb.IncludeAggregated, env["SKU_INCLUDE_AGGREGATED"], nil, false)
	s.AutoFetch = pickBool(fb.AutoFetchSet, fb.AutoFetch, env["SKU_AUTO_FETCH"], profile.AutoFetch, false)
	s.StaleOK = pickBool(fb.StaleOKSet, fb.StaleOK, env["SKU_STALE_OK"], nil, false)
	s.Verbose = pickBool(fb.VerboseSet, fb.Verbose, env["SKU_VERBOSE"], nil, false)
	s.DryRun = pickBool(fb.DryRunSet, fb.DryRun, env["SKU_DRY_RUN"], nil, false)

	// NO_COLOR: standard spec says any presence disables color. We also
	// honor SKU_NO_COLOR (parsed as bool) and the explicit --no-color flag.
	s.NoColor = pickBool(fb.NoColorSet, fb.NoColor, env["SKU_NO_COLOR"], nil, false)
	if v, ok := env["NO_COLOR"]; ok && v != "" {
		s.NoColor = true
	}

	// Int fields.
	s.StaleWarningDays = pickInt(env["SKU_STALE_WARNING_DAYS"], profile.StaleWarningDays, 14)
	s.StaleErrorDays = pickInt(env["SKU_STALE_ERROR_DAYS"], profile.StaleErrorDays, 0)

	return s, nil
}

func pickString(flagSet bool, flagVal, envVal, profileVal, def string) string {
	if flagSet {
		return flagVal
	}
	if envVal != "" {
		return envVal
	}
	if profileVal != "" {
		return profileVal
	}
	return def
}

func pickBool(flagSet bool, flagVal bool, envVal string, profileVal *bool, def bool) bool {
	if flagSet {
		return flagVal
	}
	if envVal != "" {
		return parseBool(envVal)
	}
	if profileVal != nil {
		return *profileVal
	}
	return def
}

func pickInt(envVal string, profileVal *int, def int) int {
	if envVal != "" {
		if n, err := strconv.Atoi(envVal); err == nil {
			return n
		}
	}
	if profileVal != nil {
		return *profileVal
	}
	return def
}

// parseBool accepts 1|true|yes|on (case-insensitive) as truthy.
func parseBool(v string) bool {
	switch strings.ToLower(strings.TrimSpace(v)) {
	case "1", "true", "yes", "on":
		return true
	}
	return false
}
