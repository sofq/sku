package kinds

import (
	"context"
	"fmt"
	"strings"

	"github.com/sofq/sku/internal/catalog"
)

// CacheKVSpec captures the cache.kv equivalence shape. Memory is the universal
// cross-provider dimension. Window: [memory*0.9, memory*1.5] — loose to
// accommodate Azure tier discreteness (C1=1GB, C3=6GB, C4=13GB, ...).
//
// Engine is matched against terms.tenancy — all three cache.kv ingest modules
// write engine into terms.tenancy ("redis" | "memcached").
type CacheKVSpec struct {
	MemoryGB float64
	Engine   string
	MaxPrice float64
	Regions  []string
}

const (
	cacheKVMemMin = 0.9
	cacheKVMemMax = 1.5
)

// QueryCacheKV runs the cache.kv equivalence query against a single shard and
// returns rows with prices populated. Term pin: commitment='on_demand';
// tenancy=<Engine> when specified.
func QueryCacheKV(ctx context.Context, c *catalog.Catalog, spec CacheKVSpec) ([]catalog.Row, error) {
	where := []string{
		"s.kind = 'cache.kv'",
		"t.commitment = 'on_demand'",
	}
	var args []any
	if spec.MemoryGB > 0 {
		where = append(where,
			"ra.memory_gb IS NOT NULL",
			"ra.memory_gb >= ?",
			"ra.memory_gb <= ?",
		)
		args = append(args, spec.MemoryGB*cacheKVMemMin, spec.MemoryGB*cacheKVMemMax)
	}
	if spec.Engine != "" {
		where = append(where, "t.tenancy = ?")
		args = append(args, spec.Engine)
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
		return nil, fmt.Errorf("compare: cache_kv query: %w", err)
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
