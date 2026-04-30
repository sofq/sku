package kinds

import (
	"context"
	"fmt"
	"strings"

	"github.com/sofq/sku/internal/catalog"
)

// PaasAppSpec captures the paas.app equivalence shape.
//
// PlanOS defaults to "linux". Pass PlanOS = "windows" to compare Windows plans.
// Tier filters terms.support_tier exact-match.
// SKU filters resource_name exact-match (optional).
type PaasAppSpec struct {
	PlanOS      string // default "linux"; also "windows"
	Tier        string // optional terms.support_tier filter
	SKU         string // optional resource_name filter
	MinVCPU     int64
	MinMemoryGB float64
	MaxPrice    float64
	Regions     []string
}

const defaultPaasAppOS = "linux"

// QueryPaasApp runs the paas.app equivalence query against a single shard.
func QueryPaasApp(ctx context.Context, c *catalog.Catalog, spec PaasAppSpec) ([]catalog.Row, error) {
	where := []string{
		"s.kind = 'paas.app'",
		"t.commitment = 'on_demand'",
	}
	var args []any

	planOS := spec.PlanOS
	if planOS == "" {
		planOS = defaultPaasAppOS
	}
	where = append(where, "t.os = ?")
	args = append(args, planOS)

	if spec.Tier != "" {
		where = append(where, "t.support_tier = ?")
		args = append(args, spec.Tier)
	}
	if spec.SKU != "" {
		where = append(where, "s.resource_name = ?")
		args = append(args, spec.SKU)
	}
	if spec.MinVCPU > 0 {
		where = append(where, "ra.vcpu IS NOT NULL AND ra.vcpu >= ?")
		args = append(args, spec.MinVCPU)
	}
	if spec.MinMemoryGB > 0 {
		where = append(where, "ra.memory_gb IS NOT NULL AND ra.memory_gb >= ?")
		args = append(args, spec.MinMemoryGB)
	}
	if spec.MaxPrice > 0 {
		where = append(where, "mp.min_price IS NOT NULL AND mp.min_price <= ?")
		args = append(args, spec.MaxPrice)
	}
	if len(spec.Regions) > 0 {
		placeholders := strings.Repeat("?,", len(spec.Regions))
		placeholders = placeholders[:len(placeholders)-1]
		where = append(where, "s.region IN ("+placeholders+")")
		for _, r := range spec.Regions {
			args = append(args, r)
		}
	}

	query := `
SELECT s.sku_id, s.provider, s.service, s.kind, s.resource_name, s.region,
       s.region_normalized, s.terms_hash,
       t.commitment, t.tenancy, t.os, t.support_tier, t.upfront, t.payment_option,
       ra.vcpu, ra.memory_gb, ra.storage_gb, ra.gpu_count, ra.gpu_model,
       ra.architecture, ra.extra,
       COALESCE(mp.min_price, 0) AS min_price
FROM skus s
JOIN terms t ON t.sku_id = s.sku_id
LEFT JOIN resource_attrs ra ON ra.sku_id = s.sku_id
LEFT JOIN (
  SELECT sku_id, MIN(amount) AS min_price FROM prices GROUP BY sku_id
) mp ON mp.sku_id = s.sku_id
WHERE ` + strings.Join(where, " AND ") + `
ORDER BY COALESCE(mp.min_price, 1e308) ASC, s.provider, s.resource_name, s.sku_id` //nolint:gosec // G202: WHERE composed from literals + placeholders only

	rs, err := c.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("compare: paas_app query: %w", err)
	}
	defer func() { _ = rs.Close() }()
	var out []catalog.Row
	for rs.Next() {
		r, err := scanVMRow(rs)
		if err != nil {
			return nil, err
		}
		if err := c.FillPrices(ctx, &r); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rs.Err()
}
