package catalog

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
)

// Row is the catalog-layer result record. The renderer (internal/output)
// translates this to the spec §4 output shape.
type Row struct {
	SKUID          string
	Provider       string // serving provider for LLMs
	Service        string
	Kind           string
	ResourceName   string
	Region         string
	RegionGroup    string
	CatalogVersion string
	Currency       string
	TermsHash      string

	Terms         Terms
	ResourceAttrs ResourceAttrs
	Prices        []Price
	Health        *Health

	Aggregated bool // true iff this is the synthetic openrouter row
}

// Terms is the shard-side terms row.
type Terms struct {
	Commitment    string
	Tenancy       string
	OS            string
	SupportTier   string
	Upfront       string
	PaymentOption string
}

// ResourceAttrs mirrors the shard's resource_attrs row (nullable fields as pointers).
type ResourceAttrs struct {
	VCPU             *int64
	MemoryGB         *float64
	StorageGB        *float64
	GPUCount         *int64
	GPUModel         *string
	Architecture     *string
	ContextLength    *int64
	MaxOutputTokens  *int64
	Modality         []string
	Capabilities     []string
	Quantization     *string
	DurabilityNines  *int64
	AvailabilityTier *string
	Extra            map[string]any
}

// Price is a single row from the prices table.
type Price struct {
	Dimension string
	Tier      string
	Amount    float64
	Unit      string
}

// Health mirrors the shard's health row.
type Health struct {
	Uptime30d              *float64
	LatencyP50Ms           *int64
	LatencyP95Ms           *int64
	ThroughputTokensPerSec *float64
	ObservedAt             *int64
}

// LLMFilter captures the flags the CLI surface exposes for `sku llm price`.
type LLMFilter struct {
	Model             string
	ServingProvider   string
	IncludeAggregated bool
}

// LookupLLM executes a point lookup over the openrouter-style row set and
// returns zero or more rows. No match is not an error — callers wrap empty
// results into skuerrors.NotFound at the command layer.
func (c *Catalog) LookupLLM(ctx context.Context, f LLMFilter) ([]Row, error) {
	if f.Model == "" {
		return nil, fmt.Errorf("catalog: LookupLLM requires Model")
	}

	var where []string
	var args []any
	where = append(where, "s.resource_name = ?")
	args = append(args, f.Model)
	if f.ServingProvider != "" {
		where = append(where, "s.provider = ?")
		args = append(args, f.ServingProvider)
	}
	if !f.IncludeAggregated {
		where = append(where, "s.provider <> 'openrouter'")
	}
	// LLM rows are global: we don't filter on region here; the index prefix
	// (resource_name, region, terms_hash) still benefits from the resource_name
	// equality predicate.
	// WHERE clauses are composed from compile-time literals and validated enum
	// values only — no raw user input is ever interpolated.
	const queryBase = `
SELECT s.sku_id, s.provider, s.service, s.kind, s.resource_name, s.region,
       s.region_normalized, s.terms_hash,
       t.commitment, t.tenancy, t.os, t.support_tier, t.upfront, t.payment_option,
       ra.context_length, ra.max_output_tokens, ra.modality, ra.capabilities, ra.quantization,
       h.uptime_30d, h.latency_p50_ms, h.latency_p95_ms, h.throughput_tokens_per_sec, h.observed_at
FROM skus s
JOIN terms t          ON t.sku_id = s.sku_id
LEFT JOIN resource_attrs ra ON ra.sku_id = s.sku_id
LEFT JOIN health h          ON h.sku_id = s.sku_id
WHERE `
	query := queryBase + strings.Join(where, " AND ") + "\nORDER BY s.provider" //nolint:gosec // G202: no user input in SQL concatenation

	rs, err := c.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("catalog: LookupLLM: %w", err)
	}
	defer func() { _ = rs.Close() }()

	var out []Row
	for rs.Next() {
		var r Row
		var supportTier, upfront, paymentOption sql.NullString
		var ctxLen, maxOut sql.NullInt64
		var modalityJSON, capsJSON, quant sql.NullString
		var uptime, throughput sql.NullFloat64
		var p50, p95, observed sql.NullInt64

		if err := rs.Scan(
			&r.SKUID, &r.Provider, &r.Service, &r.Kind, &r.ResourceName, &r.Region,
			&r.RegionGroup, &r.TermsHash,
			&r.Terms.Commitment, &r.Terms.Tenancy, &r.Terms.OS,
			&supportTier, &upfront, &paymentOption,
			&ctxLen, &maxOut, &modalityJSON, &capsJSON, &quant,
			&uptime, &p50, &p95, &throughput, &observed,
		); err != nil {
			return nil, err
		}
		r.CatalogVersion = c.catalogVersion
		r.Currency = c.currency
		r.Terms.SupportTier = supportTier.String
		r.Terms.Upfront = upfront.String
		r.Terms.PaymentOption = paymentOption.String
		if ctxLen.Valid {
			v := ctxLen.Int64
			r.ResourceAttrs.ContextLength = &v
		}
		if maxOut.Valid {
			v := maxOut.Int64
			r.ResourceAttrs.MaxOutputTokens = &v
		}
		if modalityJSON.Valid {
			_ = json.Unmarshal([]byte(modalityJSON.String), &r.ResourceAttrs.Modality)
		}
		if capsJSON.Valid {
			_ = json.Unmarshal([]byte(capsJSON.String), &r.ResourceAttrs.Capabilities)
		}
		if quant.Valid {
			v := quant.String
			r.ResourceAttrs.Quantization = &v
		}
		if uptime.Valid || p50.Valid || p95.Valid || throughput.Valid || observed.Valid {
			h := &Health{}
			if uptime.Valid {
				v := uptime.Float64
				h.Uptime30d = &v
			}
			if p50.Valid {
				v := p50.Int64
				h.LatencyP50Ms = &v
			}
			if p95.Valid {
				v := p95.Int64
				h.LatencyP95Ms = &v
			}
			if throughput.Valid {
				v := throughput.Float64
				h.ThroughputTokensPerSec = &v
			}
			if observed.Valid {
				v := observed.Int64
				h.ObservedAt = &v
			}
			r.Health = h
		}
		r.Aggregated = r.Provider == "openrouter"

		// Load prices in a second query so scan code stays readable.
		if err := c.fillPrices(ctx, &r); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	if err := rs.Err(); err != nil {
		return nil, err
	}
	return out, nil
}

func (c *Catalog) fillPrices(ctx context.Context, r *Row) error {
	rs, err := c.db.QueryContext(ctx,
		"SELECT dimension, tier, amount, unit FROM prices WHERE sku_id = ? ORDER BY dimension, tier",
		r.SKUID,
	)
	if err != nil {
		return err
	}
	defer func() { _ = rs.Close() }()
	for rs.Next() {
		var p Price
		if err := rs.Scan(&p.Dimension, &p.Tier, &p.Amount, &p.Unit); err != nil {
			return err
		}
		r.Prices = append(r.Prices, p)
	}
	return rs.Err()
}
