package kinds

import (
	"context"
	"fmt"
	"strings"

	"github.com/sofq/sku/internal/catalog"
)

// DBRelationalSpec captures the cross-provider db.relational equivalence shape.
// Engine and DeploymentOption are mandatory (caller always supplies a default
// in cmd/sku/compare.go). Zero-value VCPU / MemoryGB / StorageGB / MaxPrice
// disable the corresponding predicate.
//
// Term pin: commitment='on_demand', tenancy=<Engine>, os=<DeploymentOption>.
type DBRelationalSpec struct {
	VCPU             int64
	MemoryGB         float64
	StorageGB        float64
	Engine           string
	DeploymentOption string
	MaxPrice         float64
	Regions          []string
}

func QueryDBRelational(ctx context.Context, c *catalog.Catalog, spec DBRelationalSpec) ([]catalog.Row, error) {
	if spec.Engine == "" || spec.DeploymentOption == "" {
		return nil, fmt.Errorf("compare: db.relational requires engine + deployment option")
	}
	where := []string{
		"s.kind = 'db.relational'",
		"t.commitment = 'on_demand'",
		"t.tenancy = ?",
		"t.os = ?",
	}
	args := []any{spec.Engine, spec.DeploymentOption}
	if spec.VCPU > 0 {
		where = append(where, "ra.vcpu >= ?")
		args = append(args, spec.VCPU)
	}
	if spec.MemoryGB > 0 {
		where = append(where, "ra.memory_gb >= ?")
		args = append(args, spec.MemoryGB)
	}
	if spec.StorageGB > 0 {
		where = append(where, "ra.storage_gb IS NOT NULL AND ra.storage_gb >= ?")
		args = append(args, spec.StorageGB)
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
		return nil, fmt.Errorf("compare: db_relational query: %w", err)
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
