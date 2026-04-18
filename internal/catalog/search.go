package catalog

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
)

// SearchFilter captures the full set of filters `sku search` exposes in M4.1.
// All fields are optional except Provider + Service (which pin the shard).
// Zero values disable the corresponding predicate.
type SearchFilter struct {
	Provider     string
	Service      string
	Kind         string
	ResourceName string
	Region       string
	MinVCPU      int64
	MinMemoryGB  float64
	MaxPrice     float64 // 0 disables; negative is rejected by the caller
	Sort         string  // "", "price", "vcpu", "memory", "resource_name"
	Limit        int     // 0 disables (caller passes default)
}

// Search runs a generic filtered query over the shard's skus table. Unlike
// LookupVM / LookupDBRelational, Search does not require a resource_name or
// region and is free to return an empty slice. No-match is not an error —
// callers wrap empty results into skuerrors.NotFound at the command layer.
func (c *Catalog) Search(ctx context.Context, f SearchFilter) ([]Row, error) {
	if f.Provider == "" {
		return nil, fmt.Errorf("catalog: Search requires Provider")
	}
	if f.Service == "" {
		return nil, fmt.Errorf("catalog: Search requires Service")
	}
	query, args := buildSearchQuery(f)
	rs, err := c.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("catalog: Search: %w", err)
	}
	defer func() { _ = rs.Close() }()

	var out []Row
	for rs.Next() {
		r, err := scanSearchRow(rs)
		if err != nil {
			return nil, err
		}
		r.CatalogVersion = c.catalogVersion
		r.Currency = c.currency
		if err := c.fillPrices(ctx, &r); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rs.Err()
}

// buildSearchQuery composes the SELECT + WHERE + ORDER BY + LIMIT clauses
// from f. Only compile-time literals (never user input) are ever
// concatenated into the SQL text; every user-supplied value is bound as a ?.
func buildSearchQuery(f SearchFilter) (string, []any) {
	var where []string
	var args []any
	where = append(where, "s.provider = ?")
	args = append(args, f.Provider)
	where = append(where, "s.service = ?")
	args = append(args, f.Service)
	if f.Kind != "" {
		where = append(where, "s.kind = ?")
		args = append(args, f.Kind)
	}
	if f.ResourceName != "" {
		where = append(where, "s.resource_name = ?")
		args = append(args, f.ResourceName)
	}
	if f.Region != "" {
		where = append(where, "s.region = ?")
		args = append(args, f.Region)
	}
	if f.MinVCPU > 0 {
		where = append(where, "ra.vcpu >= ?")
		args = append(args, f.MinVCPU)
	}
	if f.MinMemoryGB > 0 {
		where = append(where, "ra.memory_gb >= ?")
		args = append(args, f.MinMemoryGB)
	}

	const base = `
SELECT s.sku_id, s.provider, s.service, s.kind, s.resource_name, s.region,
       s.region_normalized, s.terms_hash,
       t.commitment, t.tenancy, t.os, t.support_tier, t.upfront, t.payment_option,
       ra.vcpu, ra.memory_gb, ra.storage_gb, ra.gpu_count, ra.gpu_model,
       ra.architecture, ra.extra
FROM skus s
JOIN terms t ON t.sku_id = s.sku_id
LEFT JOIN resource_attrs ra ON ra.sku_id = s.sku_id
WHERE `
	query := base + strings.Join(where, " AND ") +
		"\nORDER BY s.resource_name, s.sku_id" //nolint:gosec // G202: no user input in SQL concatenation
	return query, args
}

func scanSearchRow(rs *sql.Rows) (Row, error) {
	var r Row
	var supportTier, upfront, paymentOption sql.NullString
	var vcpu sql.NullInt64
	var mem, storage sql.NullFloat64
	var gpuCount sql.NullInt64
	var gpuModel, arch, extraJSON sql.NullString
	if err := rs.Scan(
		&r.SKUID, &r.Provider, &r.Service, &r.Kind, &r.ResourceName, &r.Region,
		&r.RegionGroup, &r.TermsHash,
		&r.Terms.Commitment, &r.Terms.Tenancy, &r.Terms.OS,
		&supportTier, &upfront, &paymentOption,
		&vcpu, &mem, &storage, &gpuCount, &gpuModel, &arch, &extraJSON,
	); err != nil {
		return r, err
	}
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
