package kinds

import (
	"context"
	"fmt"
	"strings"

	"github.com/sofq/sku/internal/catalog"
)

// StorageObjectSpec captures the cross-provider storage.object equivalence
// shape. Zero values disable the matching predicate. StorageClass is an exact
// match on s.resource_name ("standard", "standard-ia", "hot", ...).
// Term pin: on_demand / '' / '' — storage has no tenancy/os.
type StorageObjectSpec struct {
	StorageClass     string
	DurabilityNines  int64
	AvailabilityTier string
	MaxPrice         float64
	Regions          []string
}

func QueryStorageObject(ctx context.Context, c *catalog.Catalog, spec StorageObjectSpec) ([]catalog.Row, error) {
	where := []string{
		"s.kind = 'storage.object'",
		"t.commitment = 'on_demand'",
		"t.tenancy = ''",
		"t.os = ''",
	}
	var args []any
	if spec.StorageClass != "" {
		where = append(where, "s.resource_name = ?")
		args = append(args, spec.StorageClass)
	}
	if spec.DurabilityNines > 0 {
		where = append(where, "ra.durability_nines IS NOT NULL AND ra.durability_nines >= ?")
		args = append(args, spec.DurabilityNines)
	}
	if spec.AvailabilityTier != "" {
		where = append(where, "ra.availability_tier = ?")
		args = append(args, spec.AvailabilityTier)
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
		return nil, fmt.Errorf("compare: storage_object query: %w", err)
	}
	defer func() { _ = rs.Close() }()
	var out []catalog.Row
	for rs.Next() {
		r, err := scanVMRow(rs)
		if err != nil {
			return nil, err
		}
		if err := fillStorageObjectAttrs(ctx, c, &r); err != nil {
			return nil, err
		}
		if err := c.FillPrices(ctx, &r); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rs.Err()
}

func fillStorageObjectAttrs(ctx context.Context, c *catalog.Catalog, r *catalog.Row) error {
	const q = `SELECT durability_nines, availability_tier FROM resource_attrs WHERE sku_id = ?`
	rs, err := c.QueryContext(ctx, q, r.SKUID)
	if err != nil {
		return err
	}
	defer func() { _ = rs.Close() }()
	if rs.Next() {
		var dur *int64
		var tier *string
		if err := rs.Scan(&dur, &tier); err != nil {
			return err
		}
		r.ResourceAttrs.DurabilityNines = dur
		r.ResourceAttrs.AvailabilityTier = tier
	}
	return rs.Err()
}
