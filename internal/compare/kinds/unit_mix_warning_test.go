package kinds

import (
	"bytes"
	"strings"
	"testing"

	"github.com/sofq/sku/internal/catalog"
)

func priceRow(units ...string) catalog.Row {
	r := catalog.Row{}
	for _, u := range units {
		r.Prices = append(r.Prices, catalog.Price{Unit: u})
	}
	return r
}

func TestWarnIfMixedMessagingUnits(t *testing.T) {
	cases := []struct {
		name    string
		rows    []catalog.Row
		wantHit bool
	}{
		{
			name:    "single family count only",
			rows:    []catalog.Row{priceRow("1M-req"), priceRow("request")},
			wantHit: false,
		},
		{
			name:    "single family hour only",
			rows:    []catalog.Row{priceRow("hr"), priceRow("hour")},
			wantHit: false,
		},
		{
			name:    "single family byte only",
			rows:    []catalog.Row{priceRow("gb-mo"), priceRow("gib-mo")},
			wantHit: false,
		},
		{
			name:    "count and hour mixed",
			rows:    []catalog.Row{priceRow("1M-req"), priceRow("hr")},
			wantHit: true,
		},
		{
			name:    "count and byte mixed (SQS vs Pub/Sub)",
			rows:    []catalog.Row{priceRow("1M-req"), priceRow("gb-mo")},
			wantHit: true,
		},
		{
			name:    "month-only is ignored for messaging",
			rows:    []catalog.Row{priceRow("month")},
			wantHit: false,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var buf bytes.Buffer
			prev := SetWarningWriter(&buf)
			t.Cleanup(func() { SetWarningWriter(prev) })

			warnIfMixedMessagingUnits(tc.rows, "messaging.queue")

			got := strings.Contains(buf.String(), "messaging.queue result mixes")
			if got != tc.wantHit {
				t.Fatalf("warning hit=%v want=%v; output=%q", got, tc.wantHit, buf.String())
			}
		})
	}
}

func TestWarnIfMixedCDNUnits(t *testing.T) {
	cases := []struct {
		name    string
		rows    []catalog.Row
		wantHit bool
	}{
		{
			name:    "egress only",
			rows:    []catalog.Row{priceRow("gb")},
			wantHit: false,
		},
		{
			name:    "egress and request mixed",
			rows:    []catalog.Row{priceRow("gb"), priceRow("request")},
			wantHit: true,
		},
		{
			name:    "egress and base-fee mixed",
			rows:    []catalog.Row{priceRow("gb"), priceRow("month")},
			wantHit: true,
		},
		{
			name:    "all three families",
			rows:    []catalog.Row{priceRow("gb"), priceRow("request"), priceRow("month")},
			wantHit: true,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var buf bytes.Buffer
			prev := SetWarningWriter(&buf)
			t.Cleanup(func() { SetWarningWriter(prev) })

			warnIfMixedCDNUnits(tc.rows)

			got := strings.Contains(buf.String(), "network.cdn result mixes")
			if got != tc.wantHit {
				t.Fatalf("warning hit=%v want=%v; output=%q", got, tc.wantHit, buf.String())
			}
		})
	}
}
