package output

import (
	"encoding/json"
	"io"
)

// DryRunPlan carries the resolved query plan emitted by commands when
// --dry-run is set. The rendered JSON follows spec §4 "Dry-run output".
type DryRunPlan struct {
	Command      string         `json:"command"`
	ResolvedArgs map[string]any `json:"resolved_args"`
	Shards       []string       `json:"shards"`
	TermsHash    string         `json:"terms_hash,omitempty"`
	SQL          string         `json:"sql,omitempty"`
	Preset       string         `json:"preset"`
}

// EmitDryRun writes the stable dry-run envelope to w.
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
	return json.NewEncoder(w).Encode(doc)
}
