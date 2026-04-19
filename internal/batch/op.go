package batch

import skuerrors "github.com/sofq/sku/internal/errors"

// Op is one entry in the batch input stream. Per-op flags override the
// corresponding global setting for that op only.
type Op struct {
	Command           string         `json:"command"`
	Args              map[string]any `json:"args,omitempty"`
	Preset            string         `json:"preset,omitempty"`
	Profile           string         `json:"profile,omitempty"`
	JQ                string         `json:"jq,omitempty"`
	Fields            string         `json:"fields,omitempty"`
	Format            string         `json:"format,omitempty"`
	Pretty            bool           `json:"pretty,omitempty"`
	IncludeRaw        bool           `json:"include_raw,omitempty"`
	IncludeAggregated bool           `json:"include_aggregated,omitempty"`
}

// Record is the per-op result emitted on stdout.
type Record struct {
	Index    int          `json:"index"`
	ExitCode int          `json:"exit_code"`
	Output   any          `json:"output"`
	Error    *skuerrors.E `json:"error,omitempty"`
}

// ApplyOverrides returns a copy of base with op-level overrides applied.
func ApplyOverrides(base Settings, op Op) Settings {
	out := base
	if op.Preset != "" {
		out.Preset = op.Preset
	}
	if op.Profile != "" {
		out.Profile = op.Profile
	}
	if op.JQ != "" {
		out.JQ = op.JQ
	}
	if op.Fields != "" {
		out.Fields = op.Fields
	}
	if op.Format != "" {
		out.Format = op.Format
	}
	if op.Pretty {
		out.Pretty = true
	}
	if op.IncludeRaw {
		out.IncludeRaw = true
	}
	if op.IncludeAggregated {
		out.IncludeAggregated = true
	}
	return out
}
