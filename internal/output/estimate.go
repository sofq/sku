package output

import (
	"fmt"
	"io"
	"math"

	"github.com/sofq/sku/internal/estimate"
)

// roundSig rounds x to `digits` significant figures. Used to tame spurious
// trailing precision in `sku estimate` dollar amounts — raw arithmetic on
// per-token / per-gb-second rates produces long decimals that look like
// fake precision and break golden-file diffs across architectures.
// Mirrors Python's Decimal(..).quantize style without the import cost.
func roundSig(x float64, digits int) float64 {
	if x == 0 || math.IsNaN(x) || math.IsInf(x, 0) {
		return x
	}
	mag := math.Floor(math.Log10(math.Abs(x)))
	factor := math.Pow(10, float64(digits-1)-mag)
	return math.Round(x*factor) / factor
}

// EstimateMonthlyTotal is the monthly-total block carried in the envelope.
type EstimateMonthlyTotal struct {
	Amount   float64 `json:"amount" yaml:"amount" toml:"amount"`
	Currency string  `json:"currency" yaml:"currency" toml:"currency"`
}

// EstimateEnvelope is the §4 shape for `sku estimate`.
type EstimateEnvelope struct {
	Items        []EstimateItem       `json:"items" yaml:"items" toml:"-"`
	MonthlyTotal EstimateMonthlyTotal `json:"monthly_total" yaml:"monthly_total" toml:"monthly_total"`
}

// EstimateItem is one line-item in the output envelope.
type EstimateItem struct {
	Item         string  `json:"item" yaml:"item" toml:"item"`
	Kind         string  `json:"kind" yaml:"kind" toml:"kind"`
	SKUID        string  `json:"sku_id,omitempty" yaml:"sku_id,omitempty" toml:"sku_id,omitempty"`
	Provider     string  `json:"provider" yaml:"provider" toml:"provider"`
	Service      string  `json:"service" yaml:"service" toml:"service"`
	Resource     string  `json:"resource" yaml:"resource" toml:"resource"`
	Region       string  `json:"region,omitempty" yaml:"region,omitempty" toml:"region,omitempty"`
	HourlyUSD    float64 `json:"hourly_usd,omitempty" yaml:"hourly_usd,omitempty" toml:"hourly_usd,omitempty"`
	Quantity     float64 `json:"quantity,omitempty" yaml:"quantity,omitempty" toml:"quantity,omitempty"`
	QuantityUnit string  `json:"quantity_unit,omitempty" yaml:"quantity_unit,omitempty" toml:"quantity_unit,omitempty"`
	MonthlyUSD   float64 `json:"monthly_usd" yaml:"monthly_usd" toml:"monthly_usd"`
}

// EmitEstimate writes the result as JSON/YAML/TOML per Options.Format.
// For TOML, the envelope is re-wrapped as { "rows": items, ... } per the
// repo TOML convention (CLAUDE.md "TOML quirks").
func EmitEstimate(w io.Writer, r estimate.Result, opts Options) error {
	opts = opts.WithDefaults()
	env := EstimateEnvelope{Items: make([]EstimateItem, 0, len(r.Items))}
	env.MonthlyTotal.Amount = roundSig(r.MonthlyTotalUSD, 6)
	env.MonthlyTotal.Currency = r.Currency
	for _, li := range r.Items {
		env.Items = append(env.Items, EstimateItem{
			Item: li.Item.Raw, Kind: li.Kind, SKUID: li.SKUID,
			Provider: li.Provider, Service: li.Service, Resource: li.Resource,
			Region: li.Region, HourlyUSD: roundSig(li.HourlyUSD, 6),
			Quantity: li.Quantity, QuantityUnit: li.QuantityUnit,
			MonthlyUSD: roundSig(li.MonthlyUSD, 6),
		})
	}

	var doc any = env
	if opts.Format == "toml" {
		doc = map[string]any{
			"rows":          env.Items,
			"monthly_total": env.MonthlyTotal,
		}
	}
	b, err := Encode(doc, opts.Format, opts.Pretty)
	if err != nil {
		return fmt.Errorf("encode estimate: %w", err)
	}
	if _, err := w.Write(b); err != nil {
		return err
	}
	return nil
}
