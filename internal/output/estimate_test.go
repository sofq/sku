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
