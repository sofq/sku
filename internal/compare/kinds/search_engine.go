package kinds

import (
	"context"
	"fmt"
	"strings"

	"github.com/sofq/sku/internal/catalog"
)

// SearchEngineSpec captures the search.engine equivalence shape.
//
// Mode defaults to "managed-cluster" so the common case returns node-hour
// rows without mixing in serverless OCU pricing.
// Pass Mode = "serverless" to compare serverless rows instead.
//
// InstanceType filters resource_name exact-match.
type SearchEngineSpec struct {
	Mode         string // default "managed-cluster"; also "serverless"
	InstanceType string // optional resource_name filter
	MinVCPU      int64
	MinMemoryGB  float64
	MaxPrice     float64
	Regions      []string
}

const defaultSearchEngineMode = "managed-cluster"

// QuerySearchEngine runs the search.engine equivalence query against a single shard.
func QuerySearchEngine(ctx context.Context, c *catalog.Catalog, spec SearchEngineSpec) ([]catalog.Row, error) {
	where := []string{
		"s.kind = 'search.engine'",
		"t.commitment = 'on_demand'",
	}
	var args []any

	mode := spec.Mode
	if mode == "" {
		mode = defaultSearchEngineMode
	}
	where = append(where, "json_extract(ra.extra, '$.mode') = ?")
	args = append(args, mode)

	if spec.InstanceType != "" {
		where = append(where, "s.resource_name = ?")
		args = append(args, spec.InstanceType)
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
		return nil, fmt.Errorf("compare: search_engine query: %w", err)
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
