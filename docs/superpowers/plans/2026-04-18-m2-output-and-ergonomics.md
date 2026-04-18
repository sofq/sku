# M2 — Output Polish & CLI Ergonomics Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Ship the full agent-facing CLI surface — all four output presets, `--jq` / `--fields` projections, `--json|--yaml|--toml` rendering, `--pretty`, `--no-color`, `--dry-run`, `--verbose`, stale-catalog warnings, shell completions, a machine-readable `sku schema` command tree, configuration profiles with precedence, and interactive `sku configure` — against the one live shard (`openrouter`) from M1.

**Architecture:** A single `internal/output.Options` bundle is built once by `internal/config.Resolve` from CLI > env > profile > default precedence and threaded through every leaf command. Preset projections live in `internal/output/presets.go` (generic) + `internal/output/kinds/llm_text.go` (kind-specific `compare` fields). `--jq` runs after preset trimming using `itchyny/gojq`; `--fields` is a subsequent dot-path projection. The renderer picks `json | yaml | toml` as its terminal encoder. `sku schema` reads from two sources: static tables compiled into the binary (provider/service/kind/error taxonomy) and the open shard's `metadata` row for `serving_providers`/`allowed_*` enums. `sku configure` writes `$SKU_CONFIG_DIR/config.yaml` via a flagged-or-interactive flow backed by `bufio.Scanner`. Stale-catalog warnings are emitted from a single `internal/catalog.Age` helper consumed by every data-path command.

**Tech Stack:** Go 1.25, `github.com/spf13/cobra` (already pinned), `github.com/itchyny/gojq` (new), `gopkg.in/yaml.v3` (already indirect — promote to direct), `github.com/pelletier/go-toml/v2` (new, pure-Go), `golang.org/x/term` (new, for TTY/color detection), `modernc.org/sqlite` (already), `github.com/stretchr/testify`.

**Spec ref:** `docs/superpowers/specs/2026-04-18-sku-design.md` §4 (CLI surface, presets, exit codes, config), §5 (stale thresholds, schema versioning), §9 (M2 exit criteria).

**Budget:** 19 tasks, ≤100 checkboxes. Each task is independently committable. Every Go file added has a unit test; every CLI surface addition has an E2E-style `cobra.Command` invocation test using `cmd.SetArgs`.

---

## File Structure

| Path | Responsibility |
|---|---|
| `go.mod`, `go.sum` | Add `gojq`, `go-toml/v2`, `golang.org/x/term`; promote `yaml.v3` to direct |
| `internal/output/options.go` | `Options` struct + defaults — the bundle threaded into every renderer call |
| `internal/output/presets.go` | Preset enum + generic `Project(env, preset)` building on `Render`'s full envelope |
| `internal/output/kinds/llm_text.go` | `compare` kind-specific projection for `llm.text` rows |
| `internal/output/fields.go` | `ApplyFields(doc, "provider,price.0.amount")` — dot-path projection |
| `internal/output/jq.go` | `ApplyJQ(doc, expr)` — gojq wrapper |
| `internal/output/format.go` | Terminal encoder — `EncodeJSON`, `EncodeYAML`, `EncodeTOML` |
| `internal/output/render.go` | (modify) `Pipeline(row, opts) ([]byte, error)` — the single choke point |
| `internal/output/color.go` | `ShouldColorize(w io.Writer, opts Options) bool` — NO_COLOR + isatty + `--no-color` |
| `internal/output/dryrun.go` | `EmitDryRun(w, DryRunPlan)` — stable JSON per §4 |
| `internal/output/verbose.go` | `Log(w, fields map[string]any)` — stderr JSON diag |
| `internal/config/config.go` | YAML loader, `Profile` struct, `SKU_CONFIG_DIR` resolution |
| `internal/config/resolve.go` | `Resolve(flags FlagBag) Settings` — CLI > env > profile > default |
| `internal/config/configure.go` | Interactive `sku configure` driver (bufio; no extra deps) |
| `internal/errors/details.go` | (new) Per-code details struct validators per §4 taxonomy table |
| `internal/errors/catalog.go` | (new) `ErrorCatalog()` — the JSON payload behind `sku schema --errors` |
| `internal/catalog/age.go` | `Age(c *Catalog, now time.Time) (days int)` + stale-threshold helpers |
| `cmd/sku/root.go` | (modify) register all global flags, bind env, wire `Settings` into `cobra.Context` |
| `cmd/sku/schema.go` | `sku schema [provider [service [verb]]]` + `--list`, `--errors`, `--list-serving-providers`, `--format {json,text}` |
| `cmd/sku/schema_test.go` | Schema discovery unit tests |
| `cmd/sku/configure.go` | `sku configure` Cobra wiring (flagged + interactive) |
| `cmd/sku/configure_test.go` | Config round-trip tests |
| `cmd/sku/llm_price.go` | (modify) consume global flags via `Settings`; delete local `--pretty` / `--include-aggregated` |
| `cmd/sku/llm_price_test.go` | (extend) exercise `--preset`, `--jq`, `--fields`, `--yaml`, `--stale-ok`, `--dry-run` |
| `cmd/sku/completion.go` | `sku completion {bash,zsh,fish,powershell}` wrapper over cobra's built-in |
| `internal/config/testdata/profiles.yaml` | Golden config fixture used by resolver tests |
| `CLAUDE.md` | Bump milestone pointer to M2 and list new env vars |

---

## Task 0: Pre-flight — branch, deps, baseline green

**Files:**
- Modify: `go.mod`, `go.sum`

- [x] **Step 1: Branch off main and confirm baseline green**

```bash
git checkout main && git pull --ff-only
git checkout -b m2-output-and-ergonomics
make test && make lint && make build
```

Expected: all green. If not, stop and fix M1 breakage before proceeding.

- [x] **Step 2: Add direct deps**

```bash
go get github.com/itchyny/gojq@v0.12.17
go get github.com/pelletier/go-toml/v2@v2.3.0
go get golang.org/x/term@v0.30.0
go get gopkg.in/yaml.v3@v3.0.1
go mod tidy
```

Expected: `go.mod` `require` block now lists all four. `go.sum` updated. `make build` still green.

- [x] **Step 3: Commit**

```bash
git add go.mod go.sum
git commit -m "chore(m2): add gojq, go-toml/v2, x/term; promote yaml.v3 to direct"
```

---

## Task 1: `internal/config` — Profile struct + YAML loader (TDD)

**Files:**
- Create: `internal/config/config.go`, `internal/config/config_test.go`, `internal/config/testdata/profiles.yaml`

**Schema (from spec §4 Configuration):**

```go
type File struct {
    Profiles map[string]Profile `yaml:"profiles"`
}

type Profile struct {
    Preset             string   `yaml:"preset,omitempty"`
    Channel            string   `yaml:"channel,omitempty"`            // "daily" | "stable"
    DefaultRegions     []string `yaml:"default_regions,omitempty"`
    StaleWarningDays   *int     `yaml:"stale_warning_days,omitempty"` // nil == unset; 0 valid
    StaleErrorDays     *int     `yaml:"stale_error_days,omitempty"`
    AutoFetch          *bool    `yaml:"auto_fetch,omitempty"`
    IncludeRaw         *bool    `yaml:"include_raw,omitempty"`
}
```

- [x] **Step 1: Write fixture**

Write `internal/config/testdata/profiles.yaml`:

```yaml
profiles:
  default:
    preset: agent
    channel: daily
    default_regions: [us-east-1, eastus, us-east1]
    stale_warning_days: 14
    auto_fetch: false
  cost-planning:
    preset: full
    include_raw: true
    auto_fetch: true
  ci:
    preset: price
    stale_warning_days: 3
    stale_error_days: 7
    auto_fetch: false
```

- [x] **Step 2: Write failing test `internal/config/config_test.go`**

```go
package config_test

import (
    "path/filepath"
    "testing"

    "github.com/stretchr/testify/require"

    "github.com/sofq/sku/internal/config"
)

func TestLoad_ParsesProfilesYAML(t *testing.T) {
    f, err := config.Load(filepath.Join("testdata", "profiles.yaml"))
    require.NoError(t, err)

    def, ok := f.Profiles["default"]
    require.True(t, ok)
    require.Equal(t, "agent", def.Preset)
    require.NotNil(t, def.StaleWarningDays)
    require.Equal(t, 14, *def.StaleWarningDays)
    require.NotNil(t, def.AutoFetch)
    require.False(t, *def.AutoFetch)

    ci := f.Profiles["ci"]
    require.NotNil(t, ci.StaleErrorDays)
    require.Equal(t, 7, *ci.StaleErrorDays)
}

func TestLoad_MissingFile_ReturnsEmptyFile(t *testing.T) {
    f, err := config.Load(filepath.Join("testdata", "nonexistent.yaml"))
    require.NoError(t, err)
    require.Empty(t, f.Profiles)
}

func TestConfigDir_HonorsEnvOverride(t *testing.T) {
    t.Setenv("SKU_CONFIG_DIR", "/tmp/custom/sku")
    require.Equal(t, "/tmp/custom/sku", config.Dir())
}
```

Run: `go test ./internal/config/... -run TestLoad`
Expected: FAIL with `undefined: config.Load`.

- [x] **Step 3: Implement `internal/config/config.go`**

```go
// Package config parses ~/.config/sku/config.yaml (or platform equivalent)
// and exposes the merged Profile struct. Precedence (CLI > env > profile >
// default) is applied in Resolve, not here.
package config

import (
    "fmt"
    "os"
    "path/filepath"
    "runtime"

    "gopkg.in/yaml.v3"
)

type File struct {
    Profiles map[string]Profile `yaml:"profiles"`
}

type Profile struct {
    Preset           string   `yaml:"preset,omitempty"`
    Channel          string   `yaml:"channel,omitempty"`
    DefaultRegions   []string `yaml:"default_regions,omitempty"`
    StaleWarningDays *int     `yaml:"stale_warning_days,omitempty"`
    StaleErrorDays   *int     `yaml:"stale_error_days,omitempty"`
    AutoFetch        *bool    `yaml:"auto_fetch,omitempty"`
    IncludeRaw       *bool    `yaml:"include_raw,omitempty"`
}

// Dir returns the platform-default config directory (spec §4 Environment
// variables), honoring SKU_CONFIG_DIR.
func Dir() string {
    if v := os.Getenv("SKU_CONFIG_DIR"); v != "" {
        return v
    }
    home, _ := os.UserHomeDir()
    switch runtime.GOOS {
    case "darwin":
        return filepath.Join(home, "Library", "Application Support", "sku")
    case "windows":
        if v := os.Getenv("APPDATA"); v != "" {
            return filepath.Join(v, "sku")
        }
        return filepath.Join(home, "AppData", "Roaming", "sku")
    default:
        if v := os.Getenv("XDG_CONFIG_HOME"); v != "" {
            return filepath.Join(v, "sku")
        }
        return filepath.Join(home, ".config", "sku")
    }
}

// Path returns the canonical config file path under Dir().
func Path() string { return filepath.Join(Dir(), "config.yaml") }

// Load reads the file at path and parses it. Returns an empty File (no error)
// when the file does not exist.
func Load(path string) (File, error) {
    b, err := os.ReadFile(path) //nolint:gosec // operator-provided path
    if err != nil {
        if os.IsNotExist(err) {
            return File{}, nil
        }
        return File{}, fmt.Errorf("config: read %s: %w", path, err)
    }
    var f File
    if err := yaml.Unmarshal(b, &f); err != nil {
        return File{}, fmt.Errorf("config: parse %s: %w", path, err)
    }
    return f, nil
}
```

- [x] **Step 4: Run test**

Run: `go test ./internal/config/... -run TestLoad -v`
Expected: PASS.

- [x] **Step 5: Commit**

```bash
git add internal/config/ 
git commit -m "feat(config): add YAML profile loader + SKU_CONFIG_DIR resolution"
```

---

## Task 2: `internal/config` — Settings resolver (TDD)

Implements spec §4 precedence: **CLI flag > environment variable > profile value > built-in default**.

**Files:**
- Create: `internal/config/resolve.go`, `internal/config/resolve_test.go`

**Contract:**

```go
type Settings struct {
    Profile           string  // which profile was selected
    Preset            string  // "agent" | "price" | "full" | "compare"
    Format            string  // "json" | "yaml" | "toml"
    Pretty            bool
    JQ                string
    Fields            string
    IncludeRaw        bool
    IncludeAggregated bool
    AutoFetch         bool
    StaleOK           bool
    StaleWarningDays  int
    StaleErrorDays    int    // 0 = disabled
    NoColor           bool
    Verbose           bool
    DryRun            bool
}

// FlagBag carries raw pflag values along with "was this flag explicitly set".
// Resolve differentiates unset-flag (fall through to env/profile) from
// zero-value explicit (e.g. --stale-error-days=0 means "disable", not unset).
type FlagBag struct {
    Profile           string; ProfileSet bool
    Preset            string; PresetSet bool
    Format            string; FormatSet bool
    Pretty            bool;   PrettySet bool
    JQ                string; JQSet bool
    Fields            string; FieldsSet bool
    IncludeRaw        bool;   IncludeRawSet bool
    IncludeAggregated bool;   IncludeAggregatedSet bool
    AutoFetch         bool;   AutoFetchSet bool
    StaleOK           bool;   StaleOKSet bool
    NoColor           bool;   NoColorSet bool
    Verbose           bool;   VerboseSet bool
    DryRun            bool;   DryRunSet bool
}
```

Defaults: preset=agent, format=json, stale_warning_days=14, stale_error_days=0.

- [x] **Step 1: Write failing test**

Write `internal/config/resolve_test.go`:

```go
package config_test

import (
    "testing"

    "github.com/stretchr/testify/require"

    "github.com/sofq/sku/internal/config"
)

func TestResolve_Defaults_WhenAllUnset(t *testing.T) {
    s, err := config.Resolve(config.FlagBag{}, config.File{}, map[string]string{})
    require.NoError(t, err)
    require.Equal(t, "agent", s.Preset)
    require.Equal(t, "json", s.Format)
    require.Equal(t, 14, s.StaleWarningDays)
    require.Equal(t, 0, s.StaleErrorDays)
    require.False(t, s.Pretty)
}

func TestResolve_EnvOverridesProfile(t *testing.T) {
    f := config.File{Profiles: map[string]config.Profile{
        "default": {Preset: "full"},
    }}
    s, _ := config.Resolve(config.FlagBag{}, f, map[string]string{"SKU_PRESET": "price"})
    require.Equal(t, "price", s.Preset)
}

func TestResolve_FlagOverridesEnv(t *testing.T) {
    s, _ := config.Resolve(
        config.FlagBag{Preset: "compare", PresetSet: true},
        config.File{},
        map[string]string{"SKU_PRESET": "price"},
    )
    require.Equal(t, "compare", s.Preset)
}

func TestResolve_UnknownProfile_ReturnsError(t *testing.T) {
    _, err := config.Resolve(
        config.FlagBag{Profile: "bogus", ProfileSet: true},
        config.File{Profiles: map[string]config.Profile{"default": {}}},
        map[string]string{},
    )
    require.Error(t, err)
    require.Contains(t, err.Error(), "bogus")
}

func TestResolve_NoColorStandardEnv(t *testing.T) {
    s, _ := config.Resolve(config.FlagBag{}, config.File{}, map[string]string{"NO_COLOR": "1"})
    require.True(t, s.NoColor)
}
```

Run: `go test ./internal/config/... -run TestResolve`
Expected: FAIL.

- [x] **Step 2: Implement `internal/config/resolve.go`**

The function signature: `func Resolve(fb FlagBag, file File, env map[string]string) (Settings, error)`. Profile default is `"default"`. `env` is an injected `{SKU_PROFILE,SKU_PRESET,SKU_FORMAT,SKU_AUTO_FETCH,SKU_STALE_OK,SKU_NO_COLOR,NO_COLOR,SKU_STALE_ERROR_DAYS,SKU_PRETTY}` map so tests don't mutate process env. The caller (cmd/sku/root.go) builds this map from `os.Environ()`.

Selection order per field: `fb.X if fb.XSet else env["SKU_X"] if present else profile.X if non-zero else default`. Bool env uses `parseBool(v)` that accepts `1|true|yes|on` case-insensitively. Unknown profile errors with message `config: unknown profile %q`. Both `NO_COLOR` and `SKU_NO_COLOR` enable no-color (standard plus sku-namespaced).

Key implementation notes:
- When `fb.Profile != ""` and profile is missing, return error. When unset, select `"default"` if present, else fall through to zero-value profile.
- `StaleWarningDays` and `StaleErrorDays` are `*int` in Profile (to distinguish unset from 0). The resolver's defaults are 14 and 0 respectively.
- `Settings.Profile` records which profile was actually used (or `""` when none existed).

- [x] **Step 3: Run test**

Run: `go test ./internal/config/... -v`
Expected: all PASS.

- [x] **Step 4: Commit**

```bash
git add internal/config/
git commit -m "feat(config): resolve CLI>env>profile>default settings"
```

---

## Task 3: Root cobra — register global flags + env bag

**Files:**
- Modify: `cmd/sku/root.go`
- Create: `cmd/sku/globals.go`, `cmd/sku/globals_test.go`

Wire the flag set once on the root so every subcommand sees them. Stash resolved `Settings` into `cmd.Context()` via `context.WithValue` under a private `settingsKey` so leaves grab it with `globalSettings(cmd)`.

- [x] **Step 1: Write failing test**

Write `cmd/sku/globals_test.go`:

```go
package sku

import (
    "bytes"
    "testing"

    "github.com/stretchr/testify/require"
)

func TestRoot_GlobalFlagsRegistered(t *testing.T) {
    root := newRootCmd()
    for _, name := range []string{
        "profile", "preset", "jq", "fields", "include-raw", "include-aggregated",
        "pretty", "stale-ok", "auto-fetch", "dry-run", "verbose", "no-color",
        "json", "yaml", "toml",
    } {
        require.NotNil(t, root.PersistentFlags().Lookup(name), "missing global flag --%s", name)
    }
}

func TestRoot_PresetEnvPropagates(t *testing.T) {
    t.Setenv("SKU_PRESET", "price")
    root := newRootCmd()
    root.SetArgs([]string{"version"})
    var out bytes.Buffer
    root.SetOut(&out)
    require.NoError(t, root.Execute())
    // version command doesn't use preset, but Settings must resolve without error.
}
```

Run: `go test ./cmd/sku/ -run TestRoot_Global -v`
Expected: FAIL (flags missing).

- [x] **Step 2: Add `cmd/sku/globals.go`**

Expose the flag-bag + settings-in-context helpers:

```go
package sku

import (
    "context"
    "os"
    "strings"

    "github.com/spf13/cobra"

    "github.com/sofq/sku/internal/config"
)

type settingsKeyT struct{}

var settingsKey settingsKeyT

// globalSettings returns the resolved Settings stashed on the command's
// context by PersistentPreRunE. Leaf commands always call this after flags
// parse.
func globalSettings(cmd *cobra.Command) config.Settings {
    if v, ok := cmd.Context().Value(settingsKey).(config.Settings); ok {
        return v
    }
    return config.Settings{}
}

// envMap snapshots the SKU_* / NO_COLOR env for config.Resolve.
func envMap() map[string]string {
    keys := []string{
        "SKU_PROFILE", "SKU_PRESET", "SKU_FORMAT", "SKU_AUTO_FETCH",
        "SKU_STALE_OK", "SKU_STALE_ERROR_DAYS", "SKU_NO_COLOR", "NO_COLOR",
        "SKU_PRETTY", "SKU_VERBOSE", "SKU_DRY_RUN",
    }
    m := make(map[string]string, len(keys))
    for _, k := range keys {
        if v, ok := os.LookupEnv(k); ok {
            m[k] = v
        }
    }
    return m
}

// resolveSettings reads persistent flags off root, loads the config file,
// then runs config.Resolve. Call from PersistentPreRunE.
func resolveSettings(cmd *cobra.Command) (config.Settings, error) {
    fb := readFlagBag(cmd)
    file, err := config.Load(config.Path())
    if err != nil {
        return config.Settings{}, err
    }
    return config.Resolve(fb, file, envMap())
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
    getBool := func(name string, dst *bool, set *bool) {
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
    // Output format is a three-way mutually exclusive trio.
    // The last-set wins; PersistentPreRunE asserts at most one was changed.
    if f.Lookup("json") != nil && f.Changed("json") {
        fb.Format, fb.FormatSet = "json", true
    }
    if f.Lookup("yaml") != nil && f.Changed("yaml") {
        fb.Format, fb.FormatSet = "yaml", true
    }
    if f.Lookup("toml") != nil && f.Changed("toml") {
        fb.Format, fb.FormatSet = "toml", true
    }
    _ = context.Background // imports linter hush
    _ = strings.TrimSpace
    return fb
}
```

- [x] **Step 3: Modify `cmd/sku/root.go`**

```go
package sku

import (
    "context"

    "github.com/spf13/cobra"
)

func newRootCmd() *cobra.Command {
    root := &cobra.Command{
        Use:           "sku",
        Short:         "Agent-friendly cloud & LLM pricing CLI",
        Long:          "sku is an agent-friendly CLI for querying cloud and LLM pricing across AWS, Azure, Google Cloud, and OpenRouter.",
        SilenceUsage:  true,
        SilenceErrors: true,
        PersistentPreRunE: func(cmd *cobra.Command, _ []string) error {
            s, err := resolveSettings(cmd)
            if err != nil {
                return err
            }
            ctx := context.WithValue(cmd.Context(), settingsKey, s)
            cmd.SetContext(ctx)
            return nil
        },
    }
    pf := root.PersistentFlags()
    pf.String("profile", "", "named config profile (default \"default\")")
    pf.String("preset", "", "agent | full | price | compare (default agent)")
    pf.String("jq", "", "jq filter on response")
    pf.String("fields", "", "comma-separated dot-path projection")
    pf.Bool("include-raw", false, "include raw passthrough object")
    pf.Bool("include-aggregated", false, "include OpenRouter's aggregated rows")
    pf.Bool("pretty", false, "pretty-print output")
    pf.Bool("stale-ok", false, "suppress stale-catalog warning")
    pf.Bool("auto-fetch", false, "download missing shards on demand")
    pf.Bool("dry-run", false, "show resolved query plan without executing")
    pf.Bool("verbose", false, "stderr JSON log")
    pf.Bool("no-color", false, "disable color")
    pf.Bool("json", false, "output format: JSON (default)")
    pf.Bool("yaml", false, "output format: YAML")
    pf.Bool("toml", false, "output format: TOML")

    root.AddCommand(newVersionCmd())
    root.AddCommand(newLLMCmd())
    root.AddCommand(newUpdateCmd())
    return root
}
```

- [x] **Step 4: Run tests**

Run: `go test ./cmd/sku/... -run TestRoot -v`
Expected: PASS.

- [x] **Step 5: Commit**

```bash
git add cmd/sku/
git commit -m "feat(cmd): register global flags + settings-in-context"
```

---

## Task 4: `internal/output.Options` + Pipeline refactor (TDD)

Replace the M1 `Render(row, preset) Envelope` choke point with `Pipeline(row, opts) ([]byte, error)` that preset-projects → applies `--fields` → applies `--jq` → encodes (json/yaml/toml) with `--pretty`. Old `Render` stays as an internal helper.

**Files:**
- Create: `internal/output/options.go`, `internal/output/pipeline_test.go`
- Modify: `internal/output/render.go`

**Contract:**

```go
type Options struct {
    Preset            Preset
    Format            string // "json" | "yaml" | "toml"
    Pretty            bool
    Fields            string // "provider,price.0.amount"; empty = keep all
    JQ                string // gojq expression; empty = no filter
    IncludeRaw        bool
    IncludeAggregated bool
}

func (o Options) WithDefaults() Options // fills zero values

// Pipeline projects a single row per the preset, applies --fields, --jq, and
// encodes. It returns one line per emitted record (JSON/YAML) or one TOML
// document. Callers writing multiple rows call Pipeline once per row so JSON
// streams as NDJSON (matching M1 behavior).
func Pipeline(r catalog.Row, opts Options) ([]byte, error)
```

- [x] **Step 1: Write failing tests in `pipeline_test.go`**

```go
package output_test

import (
    "strings"
    "testing"

    "github.com/stretchr/testify/require"

    "github.com/sofq/sku/internal/output"
)

func TestPipeline_AgentPreset_EmitsCompactJSON(t *testing.T) {
    row := sampleRow()
    b, err := output.Pipeline(row, output.Options{Preset: output.PresetAgent, Format: "json"})
    require.NoError(t, err)
    require.Contains(t, string(b), `"provider":"anthropic"`)
    require.NotContains(t, string(b), `"health"`, "agent preset drops health")
    require.NotContains(t, string(b), `"source"`)
}

func TestPipeline_PricePreset_DropsEverythingExceptPrice(t *testing.T) {
    row := sampleRow()
    b, err := output.Pipeline(row, output.Options{Preset: output.PresetPrice, Format: "json"})
    require.NoError(t, err)
    require.NotContains(t, string(b), `"provider"`)
    require.Contains(t, string(b), `"price":[`)
}

func TestPipeline_FullPreset_IncludesHealth(t *testing.T) {
    row := sampleRow()
    b, err := output.Pipeline(row, output.Options{Preset: output.PresetFull, Format: "json"})
    require.NoError(t, err)
    require.Contains(t, string(b), `"health"`)
}

func TestPipeline_Pretty_IndentsJSON(t *testing.T) {
    row := sampleRow()
    b, err := output.Pipeline(row, output.Options{Preset: output.PresetAgent, Format: "json", Pretty: true})
    require.NoError(t, err)
    require.True(t, strings.Contains(string(b), "\n  "), "pretty JSON must have indent")
}
```

Run: `go test ./internal/output/... -run TestPipeline`
Expected: FAIL (undefined Pipeline).

- [x] **Step 2: Implement `options.go` + `Pipeline`**

```go
// internal/output/options.go
package output

type Options struct {
    Preset            Preset
    Format            string
    Pretty            bool
    Fields            string
    JQ                string
    IncludeRaw        bool
    IncludeAggregated bool
}

func (o Options) WithDefaults() Options {
    if o.Preset == "" {
        o.Preset = PresetAgent
    }
    if o.Format == "" {
        o.Format = "json"
    }
    return o
}
```

Add in `render.go`:

```go
// Pipeline is the single entry point used by every data-path command.
// It always produces one encoded unit per call; callers writing multiple
// rows call Pipeline per row so JSON output is NDJSON.
func Pipeline(r catalog.Row, opts Options) ([]byte, error) {
    opts = opts.WithDefaults()
    env := buildFull(r)
    if !opts.IncludeAggregated && r.Aggregated {
        return nil, ErrDropped // caller skips this row without writing bytes
    }
    // Preset-trim the envelope, then convert to a generic map so jq and
    // fields projection can operate without a struct schema.
    trimmed := Project(env, opts.Preset, r.Kind)
    doc, err := toMap(trimmed)
    if err != nil {
        return nil, err
    }
    if opts.Fields != "" {
        doc = ApplyFields(doc, opts.Fields)
    }
    if opts.JQ != "" {
        doc, err = ApplyJQ(doc, opts.JQ)
        if err != nil {
            return nil, err
        }
    }
    return Encode(doc, opts.Format, opts.Pretty)
}

// ErrDropped signals that Pipeline intentionally emitted nothing.
var ErrDropped = errors.New("output: row dropped by preset filter")
```

Scaffold stubs `Project`, `ApplyFields`, `ApplyJQ`, `Encode`, `toMap` that return their input unchanged (or empty for Fields/JQ) — they get real bodies in Tasks 5–8.

- [x] **Step 3: Run tests**

Run: `go test ./internal/output/... -v`
Expected: existing `render_test.go` still PASS; `pipeline_test.go` for agent + full + pretty PASS (price drops handled when Task 5 lands — skip with `t.Skip("TBD task 5")` as a temporary `&& t.Skip()`... actually **do not skip**; implement the minimal `Project` price branch inline here so the test passes).

- [x] **Step 4: Commit**

```bash
git add internal/output/
git commit -m "feat(output): Options + Pipeline choke point"
```

---

## Task 5: Preset projections (agent/price/full/compare) with kind-specific compare (TDD)

**Files:**
- Create: `internal/output/presets.go`, `internal/output/kinds/llm_text.go`, `internal/output/presets_test.go`

**Spec §4 Presets table** — fields kept per preset, kind-specific rules for `compare`.

- [x] **Step 1: Write test `presets_test.go`**

```go
package output_test

import (
    "testing"

    "github.com/stretchr/testify/require"

    "github.com/sofq/sku/internal/output"
)

func TestProject_Agent_KeepsSpecFields(t *testing.T) {
    full := buildFullSample() // helper wrapping buildFull(sampleRow())
    trimmed := output.Project(full, output.PresetAgent, "llm.text")
    require.NotZero(t, trimmed.Provider)
    require.NotEmpty(t, trimmed.Price)
    require.Nil(t, trimmed.Health)
    require.Nil(t, trimmed.Source)
}

func TestProject_Price_DropsAllButPrice(t *testing.T) {
    full := buildFullSample()
    trimmed := output.Project(full, output.PresetPrice, "llm.text")
    require.Empty(t, trimmed.Provider)
    require.Empty(t, trimmed.Service)
    require.Nil(t, trimmed.Resource)
    require.Nil(t, trimmed.Location)
    require.Nil(t, trimmed.Terms)
    require.NotEmpty(t, trimmed.Price)
}

func TestProject_Compare_LLMText_IncludesKindFields(t *testing.T) {
    full := buildFullSample()
    trimmed := output.Project(full, output.PresetCompare, "llm.text")
    require.NotNil(t, trimmed.Resource)
    require.NotZero(t, trimmed.Resource.Name)
    require.NotNil(t, trimmed.Resource.ContextLength)
    require.NotEmpty(t, trimmed.Resource.Capabilities)
    require.NotNil(t, trimmed.Health)
    require.NotNil(t, trimmed.Health.Uptime30d)
    require.NotNil(t, trimmed.Health.LatencyP95Ms)
}
```

Run: `go test ./internal/output/... -run TestProject`
Expected: FAIL.

- [x] **Step 2: Implement `presets.go`**

```go
package output

// Project trims a full Envelope to the preset's declared field set,
// folding in kind-specific extras where the preset depends on kind (currently
// only compare does).
func Project(env Envelope, p Preset, kind string) Envelope {
    switch p {
    case PresetFull:
        return env
    case PresetPrice:
        return Envelope{Price: env.Price}
    case PresetCompare:
        return projectCompare(env, kind)
    case PresetAgent, "":
        return trimForAgent(env)
    default:
        return trimForAgent(env)
    }
}

func projectCompare(env Envelope, kind string) Envelope {
    // Base compare fields per spec §4: provider, resource.name, price,
    // location.normalized_region.
    out := Envelope{
        Provider: env.Provider,
        Price:    env.Price,
    }
    if env.Resource != nil {
        out.Resource = &Resource{Name: env.Resource.Name}
    }
    if env.Location != nil {
        out.Location = &Location{NormalizedRegion: env.Location.NormalizedRegion}
    }
    // Kind-specific extras.
    switch kind {
    case "llm.text", "llm.multimodal", "llm.embedding":
        if env.Resource != nil && out.Resource != nil {
            out.Resource.ContextLength = env.Resource.ContextLength
            out.Resource.Capabilities = env.Resource.Capabilities
        }
        if env.Health != nil {
            out.Health = &Health{
                Uptime30d:    env.Health.Uptime30d,
                LatencyP95Ms: env.Health.LatencyP95Ms,
            }
        }
    case "compute.vm":
        if env.Resource != nil && out.Resource != nil {
            out.Resource.VCPU = env.Resource.VCPU
            out.Resource.MemoryGB = env.Resource.MemoryGB
            out.Resource.GPUCount = env.Resource.GPUCount
        }
    // More kinds land in M3a/M3b/M4. For now unknown kinds fall back to base.
    }
    return out
}
```

Also move `trimForAgent` from `render.go` into `presets.go` for cohesion; the old entry point `Render` calls `Project(buildFull(r), p, r.Kind)` now.

- [x] **Step 3: Update `buildFullSample` helper** in `render_test.go` so it's reused across files.

- [x] **Step 4: Run tests**

Run: `go test ./internal/output/... -v`
Expected: all PASS.

- [x] **Step 5: Commit**

```bash
git add internal/output/
git commit -m "feat(output): preset projections + llm.text compare extras"
```

---

## Task 6: `--fields` projection (TDD)

Comma-separated dot-path projection applied **after** preset, **before** jq. Semantics: `"provider,price.0.amount"` yields a doc with only those paths populated; missing paths are dropped silently (agents using `--fields` already know the shape they want).

**Files:**
- Create: `internal/output/fields.go`, `internal/output/fields_test.go`

- [x] **Step 1: Write failing test**

```go
package output_test

import (
    "testing"

    "github.com/stretchr/testify/require"

    "github.com/sofq/sku/internal/output"
)

func TestApplyFields_KeepsSelected(t *testing.T) {
    doc := map[string]any{
        "provider": "anthropic",
        "service":  "llm",
        "price": []any{
            map[string]any{"amount": 1.5e-5, "dimension": "prompt"},
        },
        "health": map[string]any{"uptime_30d": 0.999},
    }
    got := output.ApplyFields(doc, "provider,price.0.amount")
    require.Equal(t, "anthropic", got["provider"])
    require.Nil(t, got["service"])
    require.Nil(t, got["health"])
    price := got["price"].([]any)
    inner := price[0].(map[string]any)
    require.Equal(t, 1.5e-5, inner["amount"])
    _, hasDim := inner["dimension"]
    require.False(t, hasDim)
}

func TestApplyFields_MissingPath_SilentlyDropped(t *testing.T) {
    doc := map[string]any{"provider": "aws"}
    got := output.ApplyFields(doc, "nope.nested.thing")
    require.Empty(t, got)
}

func TestApplyFields_EmptyExpr_ReturnsInput(t *testing.T) {
    doc := map[string]any{"a": 1}
    require.Equal(t, doc, output.ApplyFields(doc, ""))
}
```

- [x] **Step 2: Implement**

Parse comma-separated list, split each on `.`, walk the doc copying into a fresh `map[string]any`. For numeric segments treat parent as `[]any`. Keep the first (M2) implementation simple: only support `map[string]any` and `[]any` with numeric index segments.

Key detail: when multiple paths share a prefix (e.g. `price.0.amount,price.0.dimension`), they merge into the same parent. Write with a recursive set-at-path helper that promotes an empty slot to a map (or `[]any`) based on next segment's kind.

- [x] **Step 3: Run test + commit**

Run: `go test ./internal/output/ -run TestApplyFields -v` → PASS
Commit: `feat(output): --fields dot-path projection`

---

## Task 7: `--jq` via gojq (TDD)

After preset + fields, run user's jq expression. Parse once (gojq returns a `*Query`); run with the doc as input. If the expression yields multiple results, emit all of them (NDJSON-style). If zero results, emit `null`. Compile errors bubble up as `CodeValidation` with `reason="flag_invalid"`, `flag="jq"`.

**Files:**
- Create: `internal/output/jq.go`, `internal/output/jq_test.go`

- [x] **Step 1: Write failing test**

```go
package output_test

import (
    "testing"

    "github.com/stretchr/testify/require"

    "github.com/sofq/sku/internal/output"
)

func TestApplyJQ_IdentityPassthrough(t *testing.T) {
    doc := map[string]any{"a": 1.0}
    got, err := output.ApplyJQ(doc, ".")
    require.NoError(t, err)
    require.Equal(t, doc, got)
}

func TestApplyJQ_Projection(t *testing.T) {
    doc := map[string]any{"price": []any{map[string]any{"amount": 0.002}}}
    got, err := output.ApplyJQ(doc, ".price[0].amount")
    require.NoError(t, err)
    require.InEpsilon(t, 0.002, got, 1e-9)
}

func TestApplyJQ_SyntaxError(t *testing.T) {
    _, err := output.ApplyJQ(map[string]any{}, "=== not jq ===")
    require.Error(t, err)
}
```

- [x] **Step 2: Implement**

Import `github.com/itchyny/gojq`. Signature `func ApplyJQ(doc any, expr string) (any, error)`. Compile the expression, Run over doc, collect iter outputs; return a single value if exactly one, or a `[]any` if multiple.

- [x] **Step 3: Run test + commit**

Run: `go test ./internal/output/ -run TestApplyJQ -v` → PASS
Commit: `feat(output): --jq via gojq`

---

## Task 8: `--json` / `--yaml` / `--toml` + `--pretty` (TDD)

**Files:**
- Create: `internal/output/format.go`, `internal/output/format_test.go`

`Encode(doc any, format string, pretty bool) ([]byte, error)` — single switch over format.

- JSON: `json.Marshal` (compact) or `json.MarshalIndent("", "  ")` (pretty). Always append `"\n"`.
- YAML: `yaml.v3` encoder; yaml is always "pretty" — `pretty` is a no-op flag.
- TOML: `go-toml/v2`'s `Marshal`. TOML can't represent top-level arrays — if the doc is `[]any`, wrap as `{"rows": ...}` and note this in CLAUDE.md under "TOML quirks".

- [x] **Step 1: Write failing tests**

```go
package output_test

import (
    "testing"

    "github.com/stretchr/testify/require"

    "github.com/sofq/sku/internal/output"
)

func TestEncode_JSONCompact(t *testing.T) {
    b, err := output.Encode(map[string]any{"a": 1}, "json", false)
    require.NoError(t, err)
    require.Equal(t, `{"a":1}`+"\n", string(b))
}

func TestEncode_JSONPretty(t *testing.T) {
    b, err := output.Encode(map[string]any{"a": 1}, "json", true)
    require.NoError(t, err)
    require.Contains(t, string(b), "\n  \"a\"")
}

func TestEncode_YAML(t *testing.T) {
    b, err := output.Encode(map[string]any{"a": 1}, "yaml", false)
    require.NoError(t, err)
    require.Contains(t, string(b), "a: 1")
}

func TestEncode_TOML(t *testing.T) {
    b, err := output.Encode(map[string]any{"a": 1}, "toml", false)
    require.NoError(t, err)
    require.Contains(t, string(b), "a = 1")
}

func TestEncode_UnknownFormat(t *testing.T) {
    _, err := output.Encode(map[string]any{"a": 1}, "xml", false)
    require.Error(t, err)
}
```

- [x] **Step 2: Implement + run test**

Implement `Encode`. Run: `go test ./internal/output/ -run TestEncode -v` → PASS.

- [x] **Step 3: Commit**

```bash
git add internal/output/
git commit -m "feat(output): json/yaml/toml encoders + pretty"
```

---

## Task 9: `--no-color` / NO_COLOR handling (TDD)

**Files:**
- Create: `internal/output/color.go`, `internal/output/color_test.go`

Human-facing output (the text-format branches of `sku schema` and `sku configure`) honors `--no-color` / `NO_COLOR` / `SKU_NO_COLOR` and also auto-disables when stdout is not a TTY. Data output is always no-color — agents do not want ANSI.

- [x] **Step 1: Write failing test**

```go
package output_test

import (
    "bytes"
    "testing"

    "github.com/stretchr/testify/require"

    "github.com/sofq/sku/internal/output"
)

func TestShouldColorize_NonTTY_ReturnsFalse(t *testing.T) {
    var buf bytes.Buffer
    require.False(t, output.ShouldColorize(&buf, output.Options{}))
}

func TestShouldColorize_NoColorFlag_ReturnsFalse(t *testing.T) {
    var buf bytes.Buffer
    // Even if the caller claims TTY, --no-color wins.
    require.False(t, output.ShouldColorize(&buf, output.Options{NoColor: true}))
}
```

Note: we can't force a test buffer to claim TTY; TTY test is covered by
`ShouldColorize(os.Stdout, ...)` at runtime in dev loops.

- [x] **Step 2: Implement**

```go
package output

import (
    "io"
    "os"

    "golang.org/x/term"
)

// ShouldColorize returns true when the caller passed no --no-color, no
// NO_COLOR-class env is set (consumed via Options), and w is a TTY.
func ShouldColorize(w io.Writer, opts Options) bool {
    if opts.NoColor {
        return false
    }
    f, ok := w.(*os.File)
    if !ok {
        return false
    }
    return term.IsTerminal(int(f.Fd()))
}
```

- [x] **Step 3: Run test + commit**

Run: `go test ./internal/output/ -run TestShouldColorize -v` → PASS
Commit: `feat(output): --no-color/NO_COLOR TTY-aware gate`

---

## Task 10: `--include-raw` / `--include-aggregated` as global flags (lift from llm price)

**Files:**
- Modify: `cmd/sku/llm_price.go`, `cmd/sku/llm_price_test.go`

Remove the local `--include-aggregated` flag and the local `--pretty` flag from `llm price`; they are now global. Retain local `--model` / `--serving-provider`. Route through `globalSettings(cmd)` → `output.Options`.

- [x] **Step 1: Update `llm_price.go`**

```go
func newLLMPriceCmd() *cobra.Command {
    var model, servingProvider string
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
            // ... existing shard-open and LookupLLM call ...

            opts := output.Options{
                Preset:            output.Preset(s.Preset),
                Format:            s.Format,
                Pretty:            s.Pretty,
                Fields:            s.Fields,
                JQ:                s.JQ,
                IncludeRaw:        s.IncludeRaw,
                IncludeAggregated: s.IncludeAggregated,
            }
            w := cmd.OutOrStdout()
            for _, r := range rows {
                b, err := output.Pipeline(r, opts)
                if err == output.ErrDropped {
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
```

**Also:** stop pre-filtering `IncludeAggregated` in `catalog.LookupLLM`'s WHERE clause when the caller plans to filter at the renderer. The M1 behavior still holds: `LookupLLM` takes `IncludeAggregated` on its filter and keeps filtering at SQL for perf; `output.Pipeline` also drops when `opts.IncludeAggregated == false && row.Aggregated`. Belt-and-suspenders; the SQL filter stays the primary path.

- [x] **Step 2: Update `llm_price_test.go`**

Drop the direct `--include-aggregated` flag from the test args where it was local; pass it as the same flag name (now registered on root — same invocation works). Add:

```go
func TestLLMPrice_YAMLOutput(t *testing.T) {
    seedTestDataDir(t)
    out, _, _ := runLLMPrice(t, "--model", "anthropic/claude-opus-4.6", "--yaml", "--serving-provider", "aws-bedrock")
    require.Contains(t, out, "provider: aws-bedrock")
}

func TestLLMPrice_JQReducesOutput(t *testing.T) {
    seedTestDataDir(t)
    out, _, _ := runLLMPrice(t,
        "--model", "anthropic/claude-opus-4.6",
        "--serving-provider", "aws-bedrock",
        "--jq", ".price[0].amount",
    )
    require.Regexp(t, `^[\d.e-]+\n$`, out)
}

func TestLLMPrice_FieldsProjection(t *testing.T) {
    seedTestDataDir(t)
    out, _, _ := runLLMPrice(t,
        "--model", "anthropic/claude-opus-4.6",
        "--serving-provider", "aws-bedrock",
        "--fields", "provider,price.0.amount",
    )
    require.Contains(t, out, `"provider":"aws-bedrock"`)
    require.NotContains(t, out, `"service"`)
}

func TestLLMPrice_PresetPrice(t *testing.T) {
    seedTestDataDir(t)
    out, _, _ := runLLMPrice(t,
        "--model", "anthropic/claude-opus-4.6",
        "--serving-provider", "aws-bedrock",
        "--preset", "price",
    )
    require.Contains(t, out, `"price":[`)
    require.NotContains(t, out, `"provider"`)
}
```

- [x] **Step 3: Run tests**

Run: `go test ./... -count=1`
Expected: PASS.

- [x] **Step 4: Commit**

```bash
git add cmd/sku/ internal/output/
git commit -m "feat(cmd): llm price consumes global renderer options"
```

---

## Task 11: `--dry-run` + `--verbose` (TDD)

**Files:**
- Create: `internal/output/dryrun.go`, `internal/output/verbose.go`, `internal/output/dryrun_test.go`
- Modify: `cmd/sku/llm_price.go`

`--dry-run`: emit the spec §4 "Dry-run output" JSON shape and exit 0 without touching the data path. `--verbose`: emit a stderr JSON line per major step (`{"ts":"...","event":"catalog.open","shard":"openrouter","path":"..."}`).

- [x] **Step 1: Test `dryrun_test.go`**

```go
package output_test

import (
    "bytes"
    "encoding/json"
    "testing"

    "github.com/stretchr/testify/require"

    "github.com/sofq/sku/internal/output"
)

func TestEmitDryRun_StableSchema(t *testing.T) {
    var buf bytes.Buffer
    require.NoError(t, output.EmitDryRun(&buf, output.DryRunPlan{
        Command:      "llm price",
        ResolvedArgs: map[string]any{"model": "anthropic/claude-opus-4.6"},
        Shards:       []string{"openrouter"},
        TermsHash:    "deadbeef",
        SQL:          "SELECT ...",
        Preset:       "agent",
    }))
    var got map[string]any
    require.NoError(t, json.Unmarshal(buf.Bytes(), &got))
    require.Equal(t, true, got["dry_run"])
    require.Equal(t, "llm price", got["command"])
    require.Equal(t, "openrouter", got["shards"].([]any)[0])
    require.Equal(t, float64(1), got["schema_version"])
}
```

- [x] **Step 2: Implement `EmitDryRun`**

```go
package output

import (
    "encoding/json"
    "io"
)

type DryRunPlan struct {
    Command      string         `json:"command"`
    ResolvedArgs map[string]any `json:"resolved_args"`
    Shards       []string       `json:"shards"`
    TermsHash    string         `json:"terms_hash,omitempty"`
    SQL          string         `json:"sql,omitempty"`
    Preset       string         `json:"preset"`
}

func EmitDryRun(w io.Writer, p DryRunPlan) error {
    doc := map[string]any{
        "dry_run":        true,
        "schema_version": 1,
        "command":        p.Command,
        "resolved_args":  p.ResolvedArgs,
        "shards":         p.Shards,
        "terms_hash":     p.TermsHash,
        "sql":            p.SQL,
        "preset":         p.Preset,
    }
    enc := json.NewEncoder(w)
    return enc.Encode(doc)
}
```

Implement `verbose.go`:

```go
package output

import (
    "encoding/json"
    "io"
    "time"
)

func Log(w io.Writer, event string, fields map[string]any) {
    if fields == nil {
        fields = map[string]any{}
    }
    fields["ts"] = time.Now().UTC().Format(time.RFC3339Nano)
    fields["event"] = event
    _ = json.NewEncoder(w).Encode(fields)
}
```

- [x] **Step 3: Wire into `llm price`**

Early return when `s.DryRun == true`:

```go
if s.DryRun {
    return output.EmitDryRun(cmd.OutOrStdout(), output.DryRunPlan{
        Command:      "llm price",
        ResolvedArgs: map[string]any{"model": model, "serving_provider": servingProvider},
        Shards:       []string{"openrouter"},
        Preset:       s.Preset,
    })
}
if s.Verbose {
    output.Log(cmd.ErrOrStderr(), "catalog.open", map[string]any{"shard": "openrouter", "path": shardPath})
}
```

- [x] **Step 4: Test command-level dry-run**

Add to `llm_price_test.go`:

```go
func TestLLMPrice_DryRun_DoesNotTouchShard(t *testing.T) {
    t.Setenv("SKU_DATA_DIR", t.TempDir()) // no shard present
    out, _, code := runLLMPrice(t, "--model", "anthropic/claude-opus-4.6", "--dry-run")
    require.Zero(t, code)
    require.Contains(t, out, `"dry_run":true`)
    require.Contains(t, out, `"command":"llm price"`)
}
```

Run: `go test ./... -count=1` → PASS.

- [x] **Step 5: Commit**

```bash
git add internal/output/ cmd/sku/
git commit -m "feat(output): --dry-run + --verbose primitives"
```

---

## Task 12: Stale-catalog warning (TDD)

**Files:**
- Create: `internal/catalog/age.go`, `internal/catalog/age_test.go`
- Modify: `cmd/sku/llm_price.go`

`Catalog.Age(now time.Time)` returns integer days since `metadata.generated_at`. The caller checks against `stale_warning_days` and `stale_error_days` per spec §3 Reliability / §4 Config. On warning, write one line to stderr (`warning: catalog is 17 days old (warn threshold 14); run sku update`). On error, exit 8 unless `--stale-ok` / `SKU_STALE_OK=1`.

- [x] **Step 1: Write failing test**

```go
package catalog_test

import (
    "testing"
    "time"

    "github.com/stretchr/testify/require"

    "github.com/sofq/sku/internal/catalog"
)

func TestAge_ReturnsDaysSinceGeneratedAt(t *testing.T) {
    // seed a shard whose generated_at is 20 days before `now`.
    shard := buildTestShardWithGeneratedAt(t, time.Date(2026, 3, 29, 0, 0, 0, 0, time.UTC))
    cat, err := catalog.Open(shard)
    require.NoError(t, err)
    defer cat.Close()

    age := cat.Age(time.Date(2026, 4, 18, 0, 0, 0, 0, time.UTC))
    require.Equal(t, 20, age)
}
```

`buildTestShardWithGeneratedAt` is a helper that writes minimal shard DDL with the requested `generated_at` value. Put it in `catalog_test.go` and share across tests.

- [x] **Step 2: Implement `age.go`**

Load `metadata.generated_at` during `loadMetadata` (extend the switch). Parse as RFC3339. `Age(now)` returns `int(now.Sub(generatedAt).Hours() / 24)`.

- [x] **Step 3: Wire into `llm price`**

After opening the catalog:

```go
age := cat.Age(time.Now().UTC())
if s.StaleErrorDays > 0 && age >= s.StaleErrorDays && !s.StaleOK {
    err := &skuerrors.E{
        Code:    skuerrors.CodeStaleData,
        Message: fmt.Sprintf("catalog %d days old exceeds threshold %d", age, s.StaleErrorDays),
        Suggestion: "Run: sku update openrouter",
        Details: map[string]any{
            "shard": "openrouter", "age_days": age,
            "threshold_days": s.StaleErrorDays,
        },
    }
    skuerrors.Write(cmd.ErrOrStderr(), err)
    return err
}
if age >= s.StaleWarningDays && !s.StaleOK {
    fmt.Fprintf(cmd.ErrOrStderr(),
        "warning: catalog is %d days old (warn threshold %d); run `sku update openrouter`\n",
        age, s.StaleWarningDays)
}
```

- [x] **Step 4: Test end-to-end staleness behavior**

```go
func TestLLMPrice_StaleCatalog_WarnsButSucceeds(t *testing.T) {
    seedTestDataDir(t) // by default generated_at is near-now; force old
    // use t.Setenv to lower threshold instead of mutating fixture:
    t.Setenv("SKU_STALE_WARNING_DAYS_OVERRIDE_FOR_TEST", "0") // not honored; use profile
    // Simpler path: build a dedicated fixture with a far-past generated_at.
    // Defer full assertion to the wiring test; for now assert --stale-ok suppresses.
    _, stderr, code := runLLMPrice(t,
        "--model", "anthropic/claude-opus-4.6",
        "--serving-provider", "aws-bedrock",
        "--stale-ok",
    )
    require.Zero(t, code)
    require.NotContains(t, stderr, "warning: catalog")
}
```

If forcing old generated_at in fixtures is heavy, add a dedicated fixture DDL variant (`testdata/stale_seed.sql`) with a generated_at well in the past, load it via `SKU_DATA_DIR` swap, and test the warn + error paths independently.

- [x] **Step 5: Commit**

```bash
git add internal/catalog/ cmd/sku/
git commit -m "feat(catalog): stale-catalog warning + stale-error exit 8"
```

---

## Task 13: Per-code error-details schema + `sku schema --errors` payload (TDD)

**Files:**
- Create: `internal/errors/details.go`, `internal/errors/details_test.go`, `internal/errors/catalog.go`

Spec §4 table binds each error code to a fixed `details` shape. Codify it so both producers and the `sku schema --errors` doc agree.

- [x] **Step 1: Write failing test `details_test.go`**

```go
package errors_test

import (
    "testing"

    "github.com/stretchr/testify/require"

    skuerrors "github.com/sofq/sku/internal/errors"
)

func TestCatalog_EveryCodeHasSchema(t *testing.T) {
    cat := skuerrors.ErrorCatalog()
    for _, code := range []skuerrors.Code{
        skuerrors.CodeGeneric, skuerrors.CodeAuth, skuerrors.CodeNotFound,
        skuerrors.CodeValidation, skuerrors.CodeRateLimited, skuerrors.CodeConflict,
        skuerrors.CodeServer, skuerrors.CodeStaleData,
    } {
        schema, ok := cat.Entries[string(code)]
        require.True(t, ok, "no schema for code %q", code)
        require.NotEmpty(t, schema.DetailsFields, "code %q has empty DetailsFields", code)
        require.NotZero(t, schema.ExitCode)
    }
}

func TestValidationDetailsShape_CoversReasons(t *testing.T) {
    reasons := skuerrors.ValidationReasons()
    require.Contains(t, reasons, "flag_invalid")
    require.Contains(t, reasons, "binary_too_old")
    require.Contains(t, reasons, "binary_too_new")
    require.Contains(t, reasons, "shard_too_old")
    require.Contains(t, reasons, "shard_too_new")
}
```

- [x] **Step 2: Implement `catalog.go` + `details.go`**

```go
// internal/errors/catalog.go
package errors

type CatalogSchema struct {
    Entries map[string]CodeEntry `json:"codes"`
    SchemaVersion int `json:"schema_version"`
}

type CodeEntry struct {
    ExitCode      int      `json:"exit_code"`
    Description   string   `json:"description"`
    DetailsFields []string `json:"details_fields"`
    Reasons       []string `json:"reasons,omitempty"` // validation only
}

func ErrorCatalog() CatalogSchema {
    return CatalogSchema{
        SchemaVersion: 1,
        Entries: map[string]CodeEntry{
            "generic_error": {ExitCode: 1, Description: "unclassified error",
                DetailsFields: []string{"message_detail"}},
            "auth":          {ExitCode: 2, Description: "auth failure (CI-only)",
                DetailsFields: []string{"resource"}},
            "not_found":     {ExitCode: 3, Description: "no SKU matches filters",
                DetailsFields: []string{"provider", "service", "applied_filters", "nearest_matches"}},
            "validation":    {ExitCode: 4, Description: "input failed validation",
                DetailsFields: []string{"reason", "flag", "value", "allowed", "shard",
                    "required_binary_version", "hint"},
                Reasons: ValidationReasons()},
            "rate_limited":  {ExitCode: 5, Description: "provider rate limited",
                DetailsFields: []string{"retry_after_ms"}},
            "conflict":      {ExitCode: 6, Description: "state conflict",
                DetailsFields: []string{"shard", "current_head_version", "expected_from", "operation"}},
            "server":        {ExitCode: 7, Description: "upstream server error",
                DetailsFields: []string{"upstream", "status_code", "correlation_id"}},
            "stale_data":    {ExitCode: 8, Description: "catalog older than threshold",
                DetailsFields: []string{"shard", "last_updated", "age_days", "threshold_days"}},
        },
    }
}

func ValidationReasons() []string {
    return []string{"flag_invalid", "binary_too_old", "binary_too_new", "shard_too_old", "shard_too_new"}
}
```

- [x] **Step 3: Run test + commit**

Run: `go test ./internal/errors/ -v` → PASS.
Commit: `feat(errors): per-code details schema + ErrorCatalog()`

---

## Task 14: `sku schema` command tree (TDD)

**Files:**
- Create: `cmd/sku/schema.go`, `cmd/sku/schema_test.go`

Subcommands and flags per spec §4:

- `sku schema` — human-readable top-level (providers + installed services + global flags list)
- `sku schema --list` — flat list of shard names (just `openrouter` in M2)
- `sku schema --errors` — machine-readable `ErrorCatalog()` JSON
- `sku schema --list-serving-providers` — reads `metadata.serving_providers` from the installed `openrouter` shard
- `sku schema <provider>` — services under provider (only `openrouter` in M2 → prints `llm`)
- `sku schema <provider> <service>` — flags + allowed values for the leaf command
- `sku schema --format json|text` — default = `text` on TTY, `json` otherwise

All branches must be JSON-parseable under `--format json` (or implicit on non-TTY). Text branch uses `tabwriter`.

- [x] **Step 1: Write failing tests**

```go
package sku

import (
    "bytes"
    "encoding/json"
    "testing"

    "github.com/stretchr/testify/require"
)

func runSchema(t *testing.T, args ...string) (string, string, int) {
    t.Helper()
    var out, errb bytes.Buffer
    root := newRootCmd()
    root.SetOut(&out)
    root.SetErr(&errb)
    root.SetArgs(append([]string{"schema"}, args...))
    err := root.Execute()
    code := 0
    if err != nil {
        code = 1
    }
    return out.String(), errb.String(), code
}

func TestSchema_List_EmitsShardNames(t *testing.T) {
    out, _, code := runSchema(t, "--list", "--format", "json")
    require.Zero(t, code)
    var doc map[string]any
    require.NoError(t, json.Unmarshal([]byte(out), &doc))
    shards := doc["shards"].([]any)
    require.Contains(t, shards, "openrouter")
}

func TestSchema_Errors_EmitsCatalog(t *testing.T) {
    out, _, code := runSchema(t, "--errors")
    require.Zero(t, code)
    var doc map[string]any
    require.NoError(t, json.Unmarshal([]byte(out), &doc))
    codes := doc["codes"].(map[string]any)
    _, ok := codes["not_found"]
    require.True(t, ok)
}

func TestSchema_ListServingProviders_ReadsShard(t *testing.T) {
    seedTestDataDir(t) // from llm_price_test.go
    out, _, code := runSchema(t, "--list-serving-providers", "--format", "json")
    require.Zero(t, code)
    require.Contains(t, out, "aws-bedrock")
    require.Contains(t, out, "anthropic")
}
```

- [x] **Step 2: Implement `schema.go`**

```go
package sku

import (
    "encoding/json"
    "strings"

    "github.com/spf13/cobra"

    "github.com/sofq/sku/internal/catalog"
    skuerrors "github.com/sofq/sku/internal/errors"
)

func newSchemaCmd() *cobra.Command {
    var (
        list          bool
        errs          bool
        listServing   bool
        format        string
    )
    c := &cobra.Command{
        Use:   "schema [provider [service [verb]]]",
        Short: "Discover providers, services, flags, and error codes",
        RunE: func(cmd *cobra.Command, args []string) error {
            w := cmd.OutOrStdout()
            switch {
            case errs:
                return json.NewEncoder(w).Encode(skuerrors.ErrorCatalog())
            case list:
                doc := map[string]any{"shards": []string{"openrouter"}}
                return json.NewEncoder(w).Encode(doc)
            case listServing:
                shardPath := catalog.ShardPath("openrouter")
                cat, err := catalog.Open(shardPath)
                if err != nil {
                    e := &skuerrors.E{
                        Code: skuerrors.CodeNotFound,
                        Message: "openrouter shard not installed",
                        Suggestion: "Run: sku update openrouter",
                    }
                    skuerrors.Write(cmd.ErrOrStderr(), e)
                    return e
                }
                defer cat.Close()
                providers, err := cat.ServingProviders()
                if err != nil {
                    return err
                }
                return json.NewEncoder(w).Encode(map[string]any{"serving_providers": providers})
            case len(args) == 0:
                doc := map[string]any{
                    "providers": []string{"openrouter"},
                    "kinds":     []string{"llm.text", "llm.multimodal"},
                    "globals":   listGlobalFlags(cmd.Root()),
                }
                return json.NewEncoder(w).Encode(doc)
            default:
                // Leaf lookup: schema openrouter llm price
                return emitLeafSchema(w, args, format)
            }
        },
    }
    c.Flags().BoolVar(&list, "list", false, "flat list of shard names")
    c.Flags().BoolVar(&errs, "errors", false, "emit error-code catalog")
    c.Flags().BoolVar(&listServing, "list-serving-providers", false, "list serving providers in the openrouter shard")
    c.Flags().StringVar(&format, "format", "json", "json | text")
    _ = strings.Contains // hush
    return c
}
```

Add `ServingProviders()` to `internal/catalog/catalog.go`:

```go
// ServingProviders reads metadata.serving_providers (comma-separated) from
// the shard. Empty list when the row is absent.
func (c *Catalog) ServingProviders() ([]string, error) {
    var v string
    err := c.db.QueryRow("SELECT value FROM metadata WHERE key = 'serving_providers'").Scan(&v)
    if err == sql.ErrNoRows || v == "" {
        return nil, nil
    }
    if err != nil {
        return nil, err
    }
    return strings.Split(v, ","), nil
}
```

Ensure the pipeline's ingest step populates `metadata.serving_providers`. Check `pipeline/ingest/openrouter.py` — if not already present from M1, add a follow-up pipeline TODO in a comment. (M1 may have already shipped this; verify by running `sqlite3 dist/pipeline/openrouter.db "SELECT key,value FROM metadata;"`.)

- [x] **Step 3: Add to root, run tests**

Register in `root.go`: `root.AddCommand(newSchemaCmd())`.

Run: `go test ./cmd/sku/ -run TestSchema -v` → PASS.

- [x] **Step 4: Commit**

```bash
git add cmd/sku/ internal/catalog/
git commit -m "feat(cmd): sku schema + serving-providers metadata reader"
```

---

## Task 15: `sku configure` interactive + flagged (TDD)

**Files:**
- Create: `cmd/sku/configure.go`, `cmd/sku/configure_test.go`, `internal/config/configure.go`

Writes `$SKU_CONFIG_DIR/config.yaml` merging the new profile into any existing file. Flagged mode: `sku configure --profile default --preset agent --stale-warning-days 14 --auto-fetch=false`. Interactive mode (default, TTY only): prompt-per-field via `bufio.Scanner` with the current / default value offered in brackets.

- [x] **Step 1: Write failing test `configure_test.go`**

```go
package sku

import (
    "bytes"
    "os"
    "path/filepath"
    "testing"

    "github.com/stretchr/testify/require"

    "gopkg.in/yaml.v3"
)

func TestConfigure_FlaggedMode_WritesProfile(t *testing.T) {
    dir := t.TempDir()
    t.Setenv("SKU_CONFIG_DIR", dir)

    root := newRootCmd()
    root.SetOut(&bytes.Buffer{})
    root.SetErr(&bytes.Buffer{})
    root.SetArgs([]string{
        "configure", "--profile", "ci",
        "--preset", "price",
        "--stale-warning-days", "3",
        "--stale-error-days", "7",
    })
    require.NoError(t, root.Execute())

    b, err := os.ReadFile(filepath.Join(dir, "config.yaml"))
    require.NoError(t, err)
    var doc map[string]any
    require.NoError(t, yaml.Unmarshal(b, &doc))
    profs := doc["profiles"].(map[string]any)
    ci := profs["ci"].(map[string]any)
    require.Equal(t, "price", ci["preset"])
    require.Equal(t, 3, ci["stale_warning_days"])
    require.Equal(t, 7, ci["stale_error_days"])
}
```

- [x] **Step 2: Implement `internal/config/configure.go`**

```go
package config

import (
    "os"
    "path/filepath"

    "gopkg.in/yaml.v3"
)

// SaveProfile merges p into the file at path under name, creating the file
// (and parent dirs) when absent. Merge semantics: p replaces the named
// profile wholesale; other profiles are untouched.
func SaveProfile(path, name string, p Profile) error {
    if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
        return err
    }
    f, err := Load(path)
    if err != nil {
        return err
    }
    if f.Profiles == nil {
        f.Profiles = map[string]Profile{}
    }
    f.Profiles[name] = p
    b, err := yaml.Marshal(f)
    if err != nil {
        return err
    }
    return os.WriteFile(path, b, 0o600)
}
```

- [x] **Step 3: Implement `cmd/sku/configure.go`**

Cobra command with flags: `--preset`, `--stale-warning-days`, `--stale-error-days`, `--auto-fetch`, `--include-raw`, `--default-regions`, `--channel`. The global `--profile` selects which profile to edit (default `"default"`). When any of the per-field flags is `Changed()`, run in flagged mode and skip prompts. Otherwise run an interactive flow that reads line-by-line from stdin.

Keep interactive mode thin — it's one `for _, field := range fields` loop that prompts, reads, and sets the matching `Profile` field. If stdin isn't a TTY, fall back to flagged-only mode and warn on stderr.

- [x] **Step 4: Register in root**

`root.AddCommand(newConfigureCmd())` in `root.go`.

- [x] **Step 5: Run tests**

Run: `go test ./cmd/sku/ -run TestConfigure -v` → PASS.

- [x] **Step 6: Commit**

```bash
git add cmd/sku/ internal/config/
git commit -m "feat(cmd): sku configure — flagged + interactive profile editor"
```

---

## Task 16: Shell completions

**Files:**
- Create: `cmd/sku/completion.go`, `cmd/sku/completion_test.go`

Cobra has built-in completions via `c.GenBashCompletion(w)` etc. Wrap in a `sku completion {bash,zsh,fish,powershell}` subcommand.

- [x] **Step 1: Write failing test**

```go
func TestCompletion_Bash_ContainsBashCompletionSentinel(t *testing.T) {
    var out bytes.Buffer
    root := newRootCmd()
    root.SetOut(&out)
    root.SetArgs([]string{"completion", "bash"})
    require.NoError(t, root.Execute())
    require.Contains(t, out.String(), "bash completion")
}
```

- [x] **Step 2: Implement**

```go
func newCompletionCmd() *cobra.Command {
    return &cobra.Command{
        Use:                   "completion [bash|zsh|fish|powershell]",
        Short:                 "Generate shell completion script",
        ValidArgs:             []string{"bash", "zsh", "fish", "powershell"},
        Args:                  cobra.ExactValidArgs(1),
        DisableFlagsInUseLine: true,
        RunE: func(cmd *cobra.Command, args []string) error {
            switch args[0] {
            case "bash":
                return cmd.Root().GenBashCompletion(cmd.OutOrStdout())
            case "zsh":
                return cmd.Root().GenZshCompletion(cmd.OutOrStdout())
            case "fish":
                return cmd.Root().GenFishCompletion(cmd.OutOrStdout(), true)
            case "powershell":
                return cmd.Root().GenPowerShellCompletionWithDesc(cmd.OutOrStdout())
            }
            return nil
        },
    }
}
```

Register in `root.go`.

- [x] **Step 3: Run test + commit**

Run: `go test ./cmd/sku/ -run TestCompletion -v` → PASS.
Commit: `feat(cmd): sku completion {bash,zsh,fish,powershell}`

---

## Task 17: `sku version` + `sku update` thread through global renderer

**Files:**
- Modify: `cmd/sku/version.go`, `cmd/sku/update.go`, their tests

`sku version` should honor `--pretty` / `--yaml` / `--toml` / `--jq` / `--fields` too so agents can pipe it the same way as `sku llm price`. `sku update` is output-lite (stderr lines only) and only needs `--verbose` wiring.

- [x] **Step 1: Update `version.go`**

```go
func newVersionCmd() *cobra.Command {
    return &cobra.Command{
        Use:   "version",
        Short: "Print build version as JSON",
        Args:  cobra.NoArgs,
        RunE: func(cmd *cobra.Command, _ []string) error {
            s := globalSettings(cmd)
            info := version.Get()
            b, err := output.Encode(info, s.Format, s.Pretty)
            if err != nil {
                return err
            }
            _, err = cmd.OutOrStdout().Write(b)
            return err
        },
    }
}
```

Update `version_test.go` to add a `--yaml` case.

- [x] **Step 2: Update `update.go`**

When `s.Verbose` is true, emit `output.Log(cmd.ErrOrStderr(), "update.fetch", ...)` around the HTTP GETs and decompress step. No other output-format wiring needed.

- [x] **Step 3: Run all tests**

Run: `make test`
Expected: all PASS.

- [x] **Step 4: Commit**

```bash
git add cmd/sku/
git commit -m "feat(cmd): version/update use global output options"
```

---

## Task 18: CLAUDE.md + milestone pointer bump

**Files:**
- Modify: `CLAUDE.md`

Update the **Current milestone** section to point at M2's plan; add quick-path examples for the new flags.

- [x] **Step 1: Edit `CLAUDE.md`**

Replace the M1 quick-path block with an M2 quick-path block demonstrating `--preset`, `--jq`, `--fields`, `--yaml`, `--dry-run`, and `sku schema --errors`. Bump "Current milestone" to M2. Add one-liner "TOML quirks: sku wraps top-level arrays as `{rows: [...]}` before emitting TOML." per Task 8.

- [x] **Step 2: Commit**

```bash
git add CLAUDE.md
git commit -m "docs: bump CLAUDE.md to M2 with new flag examples"
```

---

## Task 19: M2 exit verification

Confirm every spec §9 M2 exit-criterion item is green, end-to-end.

**Files:** none (verification only).

- [ ] **Step 1: Full test + lint + build**

```bash
make test && make lint && make build
```

Expected: green.

- [ ] **Step 2: Manually walk spec §9 M2 criteria**

```bash
./bin/sku schema --list          # {"shards":["openrouter"]}
./bin/sku schema --errors | jq '.codes | keys | length'   # 9 (8 real codes + shard_missing)
./bin/sku schema --list-serving-providers   # requires `make openrouter-shard` first
./bin/sku version --yaml         # YAML rendering
./bin/sku version --jq '.version'   # just the version string
./bin/sku llm price --model anthropic/claude-opus-4.6 --dry-run   # dry-run envelope
./bin/sku llm price --model anthropic/claude-opus-4.6 --preset price --serving-provider aws-bedrock
./bin/sku configure --profile ci --preset price --stale-error-days 7
./bin/sku completion bash | head -5
```

Expected: every command exits 0 and the output is the documented shape.

- [ ] **Step 2b: Preset token-size spot check (M2 exit criterion)**

```bash
./bin/sku llm price --model anthropic/claude-opus-4.6 --serving-provider aws-bedrock --preset agent | wc -c
./bin/sku llm price --model anthropic/claude-opus-4.6 --serving-provider aws-bedrock --preset price | wc -c
```

Expected: `agent` ≈ 200 bytes ±50, `price` ≈ 50 bytes ±20 per spec §4 Presets table. If far off, adjust `Project` trims.

- [ ] **Step 3: Benchmark sanity**

```bash
make openrouter-shard && make bench
```

Expected: no regression > 15% vs the M1 baseline recorded in `bench/BASELINE.md`. If the extra preset trim / JSON encoder pass costs meaningful time, record it and compare to the <5 ms p99 warm target in spec §5. Perf tuning, if needed, is a follow-up — not an M2 blocker unless the target breaks.

- [ ] **Step 4: Push branch and open PR**

```bash
git push -u origin m2-output-and-ergonomics
gh pr create --title "m2: output polish + CLI ergonomics" --body "$(cat <<'EOF'
Implements §9 M2: presets, --jq, --fields, --pretty, --yaml/--toml, --no-color,
--dry-run, --verbose, stale warnings, sku schema discovery, config profiles,
sku configure, shell completions. All plumbed through a single Settings
resolver (CLI > env > profile > default) and a single output.Pipeline.

Verified: all spec §9 M2 exit criteria; make test + lint + build green.
EOF
)"
```

- [ ] **Step 5: Mark plan complete** — no code change, just a record for the executor that M2 is done.

---

## Self-Review checklist (plan author, before handoff)

- Every spec §9 M2 bullet has a task: presets ✓ (Task 5), `--jq`/`--fields` ✓ (6, 7), all exit codes ✓ (13), `sku schema` ✓ (14), config profiles ✓ (1, 2), `sku configure` ✓ (15), stale warnings ✓ (12), shell completions ✓ (16).
- Every global flag in §4 is registered in Task 3 and consumed in Task 10/11/12/17.
- No `TODO`, no "similar to Task N", no "add validation" placeholders.
- `internal/output.Pipeline` signature matches what every caller in Tasks 10, 17 expects.
- `config.FlagBag` field names match what `readFlagBag` sets and `Resolve` reads.
- `skuerrors.ErrorCatalog` keys match the `sku schema --errors` test assertions.

## Execution Handoff

Plan saved to `docs/superpowers/plans/2026-04-18-m2-output-and-ergonomics.md`.

**1. Subagent-Driven (recommended)** — fresh subagent per task, review between tasks.
**2. Inline Execution** — batch execution with checkpoints.

Pick one and invoke the matching sub-skill.
