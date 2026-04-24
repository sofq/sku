package sku

import (
	"testing"

	"github.com/sofq/sku/internal/batch"
	"github.com/sofq/sku/internal/config"
)

func TestToBatchSettings_copiesAllFields(t *testing.T) {
	g := config.Settings{
		Preset: "compare", Profile: "default", Format: "json", Pretty: true,
		JQ: ".x", Fields: "a,b", IncludeRaw: true, IncludeAggregated: true,
		AutoFetch: true, StaleOK: true, StaleWarningDays: 14, StaleErrorDays: 30,
		DryRun: false, Verbose: true, NoColor: true,
	}
	s := ToBatchSettings(g)
	want := batch.Settings{
		Preset: "compare", Profile: "default", Format: "json", Pretty: true,
		JQ: ".x", Fields: "a,b", IncludeRaw: true, IncludeAggregated: true,
		AutoFetch: true, StaleOK: true, StaleWarningDays: 14, StaleErrorDays: 30,
		Verbose: true, NoColor: true,
	}
	if s != want {
		t.Fatalf("ToBatchSettings mismatch:\n got %+v\nwant %+v", s, want)
	}
}
