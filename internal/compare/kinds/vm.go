// Package kinds contains per-kind equivalence queries used by internal/compare.
// Each file here owns one kind's SELECT shape; they all return catalog.Row so
// the merge/sort path in compare.Run is kind-agnostic.
package kinds

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/sofq/sku/internal/catalog"
)

// VMSpec captures the cross-provider compute.vm equivalence shape for m4.2.
// Zero values disable the corresponding predicate *except* GPUCount: when 0,
// the query excludes any SKU with gpu_count>0. When >0, it becomes a >=
// predicate. This mirrors the compute.vm preset's compare column set
// (spec §5).
type VMSpec struct {
	VCPU     int64
	MemoryGB float64
	GPUCount int64
	MaxPrice float64
	Regions  []string // already expanded by compare.Expand; empty means all regions
}

// QueryVM runs the compute.vm equivalence query against a single shard and
// returns rows with prices populated. Term pin: on_demand/shared/linux.
func QueryVM(ctx context.Context, c *catalog.Catalog, spec VMSpec) ([]catalog.Row, error) {
	where := []string{
		"s.kind = 'compute.vm'",
		"t.commitment = 'on_demand'",
		"t.tenancy = 'shared'",
		"t.os = 'linux'",
	}
	var args []any
	if spec.VCPU > 0 {
		where = append(where, "ra.vcpu >= ?")
		args = append(args, spec.VCPU)
	}
	if spec.MemoryGB > 0 {
		where = append(where, "ra.memory_gb >= ?")
		args = append(args, spec.MemoryGB)
	}
	switch {
	case spec.GPUCount > 0:
		where = append(where, "ra.gpu_count IS NOT NULL AND ra.gpu_count >= ?")
		args = append(args, spec.GPUCount)
	default:
		where = append(where, "(ra.gpu_count IS NULL OR ra.gpu_count = 0)")
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
		return nil, fmt.Errorf("compare: vm query: %w", err)
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

func scanVMRow(rs *sql.Rows) (catalog.Row, error) {
	var r catalog.Row
	var supportTier, upfront, paymentOption sql.NullString
	var vcpu sql.NullInt64
	var mem, storage sql.NullFloat64
	var gpuCount sql.NullInt64
	var gpuModel, arch, extraJSON sql.NullString
	var minPrice sql.NullFloat64
	if err := rs.Scan(
		&r.SKUID, &r.Provider, &r.Service, &r.Kind, &r.ResourceName, &r.Region,
		&r.RegionGroup, &r.TermsHash,
		&r.Terms.Commitment, &r.Terms.Tenancy, &r.Terms.OS,
		&supportTier, &upfront, &paymentOption,
		&vcpu, &mem, &storage, &gpuCount, &gpuModel, &arch, &extraJSON,
		&minPrice,
	); err != nil {
		return r, err
	}
	r.MinPrice = minPrice.Float64
	r.Terms.SupportTier = supportTier.String
	r.Terms.Upfront = upfront.String
	r.Terms.PaymentOption = paymentOption.String
	if vcpu.Valid {
		v := vcpu.Int64
		r.ResourceAttrs.VCPU = &v
	}
	if mem.Valid {
		v := mem.Float64
		r.ResourceAttrs.MemoryGB = &v
	}
	if storage.Valid {
		v := storage.Float64
		r.ResourceAttrs.StorageGB = &v
	}
	if gpuCount.Valid {
		v := gpuCount.Int64
		r.ResourceAttrs.GPUCount = &v
	}
	if gpuModel.Valid {
		v := gpuModel.String
		r.ResourceAttrs.GPUModel = &v
	}
	if arch.Valid {
		v := arch.String
		r.ResourceAttrs.Architecture = &v
	}
	if extraJSON.Valid && extraJSON.String != "" {
		_ = json.Unmarshal([]byte(extraJSON.String), &r.ResourceAttrs.Extra)
	}
	return r, nil
}
