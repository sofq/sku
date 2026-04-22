package output

import (
	"bytes"
	"encoding/json"
	"testing"

	"github.com/sofq/sku/internal/estimate"
)

func TestEmitEstimate_agentPreset(t *testing.T) {
	res := estimate.Result{
		Currency:        "USD",
		MonthlyTotalUSD: 70.08,
		Items: []estimate.LineItem{{
			Item:     estimate.Item{Raw: "aws/ec2:m5.large:region=us-east-1"},
			Kind:     "compute.vm",
			SKUID:    "sku-1",
			Provider: "aws", Service: "ec2", Resource: "m5.large", Region: "us-east-1",
			HourlyUSD: 0.096, Quantity: 730, QuantityUnit: "hour", MonthlyUSD: 70.08,
		}},
	}
	var buf bytes.Buffer
	if err := EmitEstimate(&buf, res, Options{Preset: PresetAgent, Format: "json"}); err != nil {
		t.Fatal(err)
	}
	var obj map[string]any
	if err := json.Unmarshal(buf.Bytes(), &obj); err != nil {
		t.Fatalf("invalid JSON: %v\n%s", err, buf.String())
	}
	if _, ok := obj["items"]; !ok {
		t.Fatalf("missing items key: %s", buf.String())
	}
	tot, ok := obj["monthly_total"].(map[string]any)
	if !ok {
		t.Fatalf("monthly_total not object: %s", buf.String())
	}
	if tot["currency"].(string) != "USD" {
		t.Fatalf("bad currency: %v", tot["currency"])
	}
}

func TestRoundSig_sixSignificantFigures(t *testing.T) {
	cases := []struct {
		in   float64
		want float64
	}{
		// Integer-magnitude amounts keep all six figs.
		{70.08, 70.08},
		{1234567.89, 1234570},
		// Sub-dollar amounts round to 6 sig figs (not 6 decimal places).
		{0.0000123456789, 0.0000123457},
		{0.000987654321, 0.000987654},
		// Edges.
		{0, 0},
		{-42.123456789, -42.1235},
	}
	for _, tc := range cases {
		got := roundSig(tc.in, 6)
		if got != tc.want {
			t.Errorf("roundSig(%g, 6) = %g, want %g", tc.in, got, tc.want)
		}
	}
}

func TestEmitEstimate_RoundsFloatAmounts(t *testing.T) {
	// Hourly rate × hours = an ugly float; make sure the envelope emits
	// the rounded value rather than the raw IEEE-754 drift.
	res := estimate.Result{
		Currency:        "USD",
		MonthlyTotalUSD: 0.09600000000000001 * 730, // 70.08000000000001
		Items: []estimate.LineItem{{
			Item:     estimate.Item{Raw: "aws/ec2:m5.large:region=us-east-1"},
			Kind:     "compute.vm",
			Provider: "aws", Service: "ec2", Resource: "m5.large", Region: "us-east-1",
			HourlyUSD:  0.09600000000000001,
			MonthlyUSD: 0.09600000000000001 * 730,
		}},
	}
	var buf bytes.Buffer
	if err := EmitEstimate(&buf, res, Options{Preset: PresetAgent, Format: "json"}); err != nil {
		t.Fatal(err)
	}
	var obj map[string]any
	if err := json.Unmarshal(buf.Bytes(), &obj); err != nil {
		t.Fatalf("invalid JSON: %v\n%s", err, buf.String())
	}
	tot := obj["monthly_total"].(map[string]any)
	if tot["amount"].(float64) != 70.08 {
		t.Fatalf("monthly total should round to 70.08, got %v", tot["amount"])
	}
	items := obj["items"].([]any)
	it0 := items[0].(map[string]any)
	if it0["hourly_usd"].(float64) != 0.096 {
		t.Fatalf("hourly should round to 0.096, got %v", it0["hourly_usd"])
	}
	if it0["monthly_usd"].(float64) != 70.08 {
		t.Fatalf("monthly should round to 70.08, got %v", it0["monthly_usd"])
	}
}
