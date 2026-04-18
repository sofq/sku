// Package output renders catalog.Row values into the spec §4 JSON envelope.
//
// The full envelope always carries the same keys (null when absent); preset
// trimming is applied by stripping keys before encoding. The rendering path
// is allocation-conscious but not micro-optimized — the hot path in M1 is
// the single-row lookup, not streaming output.
package output

import (
	"encoding/json"
	"io"

	"github.com/sofq/sku/internal/catalog"
)

// Preset enumerates the output-shape presets. M1 implements only Agent and
// Full; Price and Compare are stubbed and wired to Agent until M2 fleshes
// them out with kind-specific projections.
type Preset string

// Output preset constants. Agent and Full are fully implemented in M1;
// Price and Compare fall through to Agent until M2.
const (
	PresetAgent   Preset = "agent"
	PresetPrice   Preset = "price"
	PresetFull    Preset = "full"
	PresetCompare Preset = "compare"
)

// Envelope is the top-level §4 output object. Field ordering is enforced by
// the struct layout; json.Encoder writes keys in declaration order.
type Envelope struct {
	Provider string    `json:"provider"`
	Service  string    `json:"service"`
	SKUID    string    `json:"sku_id,omitempty"`
	Resource *Resource `json:"resource,omitempty"`
	Location *Location `json:"location,omitempty"`
	Price    []Price   `json:"price"`
	Terms    *Terms    `json:"terms,omitempty"`
	Health   *Health   `json:"health,omitempty"`
	Source   *Source   `json:"source,omitempty"`
	Raw      any       `json:"raw,omitempty"`
}

// Resource is the §4 resource block.
type Resource struct {
	Kind            string         `json:"kind"`
	Name            string         `json:"name"`
	VCPU            *int64         `json:"vcpu,omitempty"`
	MemoryGB        *float64       `json:"memory_gb,omitempty"`
	StorageGB       *float64       `json:"storage_gb,omitempty"`
	GPUCount        *int64         `json:"gpu_count,omitempty"`
	ContextLength   *int64         `json:"context_length,omitempty"`
	MaxOutputTokens *int64         `json:"max_output_tokens,omitempty"`
	Capabilities    []string       `json:"capabilities,omitempty"`
	Attributes      map[string]any `json:"attributes,omitempty"`
}

// Location is the §4 location block.
type Location struct {
	ProviderRegion   *string `json:"provider_region"`
	NormalizedRegion *string `json:"normalized_region"`
	AvailabilityZone *string `json:"availability_zone"`
}

// Price is a single price dimension as emitted in the output.
type Price struct {
	Amount    float64 `json:"amount"`
	Currency  string  `json:"currency"`
	Unit      string  `json:"unit"`
	Dimension string  `json:"dimension"`
	Tier      *string `json:"tier"`
}

// Terms is the §4 terms block. Empty-string sentinels become nil pointers.
type Terms struct {
	Commitment    string  `json:"commitment"`
	Tenancy       *string `json:"tenancy,omitempty"`
	OS            *string `json:"os,omitempty"`
	SupportTier   *string `json:"support_tier,omitempty"`
	Upfront       *string `json:"upfront,omitempty"`
	PaymentOption *string `json:"payment_option,omitempty"`
}

// Health is the §4 health block (LLM-populated).
type Health struct {
	Uptime30d              *float64 `json:"uptime_30d,omitempty"`
	LatencyP50Ms           *int64   `json:"latency_p50_ms,omitempty"`
	LatencyP95Ms           *int64   `json:"latency_p95_ms,omitempty"`
	ThroughputTokensPerSec *float64 `json:"throughput_tokens_per_sec,omitempty"`
	ObservedAt             *int64   `json:"observed_at,omitempty"`
}

// Source is the §4 source block.
type Source struct {
	CatalogVersion string `json:"catalog_version"`
	FetchedAt      string `json:"fetched_at,omitempty"`
	UpstreamID     string `json:"upstream_id,omitempty"`
	Freshness      string `json:"freshness,omitempty"`
}

func nilIfEmpty(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

// Render builds an Envelope for a single catalog.Row at the given preset.
// The full envelope is built first and then trimmed per-preset so callers
// can compare presets to the same canonical shape.
func Render(r catalog.Row, p Preset) Envelope {
	env := buildFull(r)
	switch p {
	case PresetFull:
		return env
	case PresetAgent, PresetPrice, PresetCompare, "":
		return trimForAgent(env)
	default:
		return trimForAgent(env)
	}
}

func buildFull(r catalog.Row) Envelope {
	prices := make([]Price, 0, len(r.Prices))
	for _, rp := range r.Prices {
		prices = append(prices, Price{
			Amount:    rp.Amount,
			Currency:  r.Currency,
			Unit:      rp.Unit,
			Dimension: rp.Dimension,
			Tier:      nilIfEmpty(rp.Tier),
		})
	}

	resource := &Resource{
		Kind:            r.Kind,
		Name:            r.ResourceName,
		VCPU:            r.ResourceAttrs.VCPU,
		MemoryGB:        r.ResourceAttrs.MemoryGB,
		StorageGB:       r.ResourceAttrs.StorageGB,
		GPUCount:        r.ResourceAttrs.GPUCount,
		ContextLength:   r.ResourceAttrs.ContextLength,
		MaxOutputTokens: r.ResourceAttrs.MaxOutputTokens,
		Capabilities:    r.ResourceAttrs.Capabilities,
	}
	if r.Aggregated {
		resource.Attributes = map[string]any{"aggregated": true}
	}

	var terms *Terms
	if r.Terms.Commitment != "" {
		terms = &Terms{
			Commitment:    r.Terms.Commitment,
			Tenancy:       nilIfEmpty(r.Terms.Tenancy),
			OS:            nilIfEmpty(r.Terms.OS),
			SupportTier:   nilIfEmpty(r.Terms.SupportTier),
			Upfront:       nilIfEmpty(r.Terms.Upfront),
			PaymentOption: nilIfEmpty(r.Terms.PaymentOption),
		}
	}

	location := &Location{
		ProviderRegion:   nilIfEmpty(r.Region),
		NormalizedRegion: nilIfEmpty(r.RegionGroup),
		AvailabilityZone: nil,
	}

	var health *Health
	if r.Health != nil {
		health = &Health{
			Uptime30d:              r.Health.Uptime30d,
			LatencyP50Ms:           r.Health.LatencyP50Ms,
			LatencyP95Ms:           r.Health.LatencyP95Ms,
			ThroughputTokensPerSec: r.Health.ThroughputTokensPerSec,
			ObservedAt:             r.Health.ObservedAt,
		}
	}

	source := &Source{
		CatalogVersion: r.CatalogVersion,
		Freshness:      "daily",
	}

	return Envelope{
		Provider: r.Provider,
		Service:  r.Service,
		SKUID:    r.SKUID,
		Resource: resource,
		Location: location,
		Price:    prices,
		Terms:    terms,
		Health:   health,
		Source:   source,
	}
}

// trimForAgent keeps the fields spec §4 "Presets" declares for the agent preset:
// provider, service, resource.name, location.provider_region, price, terms.commitment.
func trimForAgent(env Envelope) Envelope {
	out := Envelope{
		Provider: env.Provider,
		Service:  env.Service,
		Price:    env.Price,
	}
	if env.Resource != nil {
		out.Resource = &Resource{Name: env.Resource.Name}
	}
	if env.Location != nil {
		out.Location = &Location{
			ProviderRegion: env.Location.ProviderRegion,
		}
	}
	if env.Terms != nil {
		out.Terms = &Terms{Commitment: env.Terms.Commitment}
	}
	// health, source, raw dropped
	return out
}

// Encode writes the envelope as JSON to w. When pretty is false the encoding
// is compact (json.Encoder defaults); when true it's indented with two spaces.
// Always writes a trailing newline.
func Encode(w io.Writer, env Envelope, pretty bool) error {
	enc := json.NewEncoder(w)
	if pretty {
		enc.SetIndent("", "  ")
	}
	return enc.Encode(env)
}
