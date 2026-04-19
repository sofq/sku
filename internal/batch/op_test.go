package batch

import "testing"

func TestApplyOverrides_perOpWins(t *testing.T) {
	base := Settings{Preset: "agent", Format: "json", IncludeRaw: false}
	op := Op{Preset: "compare", IncludeRaw: true}
	got := ApplyOverrides(base, op)
	if got.Preset != "compare" || !got.IncludeRaw || got.Format != "json" {
		t.Fatalf("overrides did not apply: %+v", got)
	}
	if base.Preset != "agent" {
		t.Fatal("base mutated — ApplyOverrides must return a copy")
	}
}

func TestApplyOverrides_emptyOpKeepsBase(t *testing.T) {
	base := Settings{Preset: "agent", Format: "yaml"}
	got := ApplyOverrides(base, Op{})
	if got.Preset != "agent" || got.Format != "yaml" {
		t.Fatalf("empty op should keep base: %+v", got)
	}
}
