package catalog

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/sofq/sku/internal/schema"
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

	// MinPrice is caller-populated from MIN(prices.amount) when the query
	// carries it. Used by internal/compare for cross-row sort only; not
	// serialised.
	MinPrice float64 `json:"-"`
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
		if err := c.FillPrices(ctx, &r); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	if err := rs.Err(); err != nil {
		return nil, err
	}
	return out, nil
}

// VMFilter captures the flags `sku aws ec2 price/list` exposes.
type VMFilter struct {
	Provider     string
	Service      string
	InstanceType string
	Region       string
	Terms        Terms
}

// DBRelationalFilter captures the flags `sku aws rds price/list` exposes.
// Tenancy is repurposed for engine; OS for deployment-option — see
// pipeline/normalize/enums.yaml.
type DBRelationalFilter struct {
	Provider     string
	Service      string
	InstanceType string
	Region       string
	Terms        Terms
}

// LookupVM runs the compute.vm point lookup / list query.
func (c *Catalog) LookupVM(ctx context.Context, f VMFilter) ([]Row, error) {
	if f.InstanceType == "" {
		return nil, fmt.Errorf("catalog: LookupVM requires InstanceType")
	}
	return c.lookupResource(ctx, "compute.vm", f.Provider, f.Service,
		f.InstanceType, f.Region, f.Terms)
}

// LookupDBRelational runs the db.relational point lookup / list query.
func (c *Catalog) LookupDBRelational(ctx context.Context, f DBRelationalFilter) ([]Row, error) {
	if f.InstanceType == "" {
		return nil, fmt.Errorf("catalog: LookupDBRelational requires InstanceType")
	}
	return c.lookupResource(ctx, "db.relational", f.Provider, f.Service,
		f.InstanceType, f.Region, f.Terms)
}

// StorageObjectFilter captures the flags `sku aws s3 price/list` exposes.
// StorageClass is the user-facing name for resource_name on storage.object
// rows (e.g. "standard", "standard-ia"). Terms never carry tenancy/os for
// this kind — the client must zero-value those slots.
type StorageObjectFilter struct {
	Provider     string
	Service      string
	StorageClass string
	Region       string
	Terms        Terms
}

// ServerlessFunctionFilter captures the flags `sku aws lambda price/list` exposes.
// resource_name is the architecture string ("x86_64" / "arm64").
type ServerlessFunctionFilter struct {
	Provider     string
	Service      string
	Architecture string
	Region       string
	Terms        Terms
}

// StorageBlockFilter captures the flags `sku aws ebs price/list` exposes.
// resource_name is the volume-type slug ("gp3", "io2", ...).
type StorageBlockFilter struct {
	Provider   string
	Service    string
	VolumeType string
	Region     string
	Terms      Terms
}

// LookupStorageObject runs the storage.object point lookup / list query.
func (c *Catalog) LookupStorageObject(ctx context.Context, f StorageObjectFilter) ([]Row, error) {
	if f.StorageClass == "" {
		return nil, fmt.Errorf("catalog: LookupStorageObject requires StorageClass")
	}
	return c.lookupResource(ctx, "storage.object", f.Provider, f.Service,
		f.StorageClass, f.Region, f.Terms)
}

// LookupServerlessFunction runs the compute.function point lookup / list query.
func (c *Catalog) LookupServerlessFunction(ctx context.Context, f ServerlessFunctionFilter) ([]Row, error) {
	if f.Architecture == "" {
		return nil, fmt.Errorf("catalog: LookupServerlessFunction requires Architecture")
	}
	return c.lookupResource(ctx, "compute.function", f.Provider, f.Service,
		f.Architecture, f.Region, f.Terms)
}

// LookupStorageBlock runs the storage.block point lookup / list query.
func (c *Catalog) LookupStorageBlock(ctx context.Context, f StorageBlockFilter) ([]Row, error) {
	if f.VolumeType == "" {
		return nil, fmt.Errorf("catalog: LookupStorageBlock requires VolumeType")
	}
	return c.lookupResource(ctx, "storage.block", f.Provider, f.Service,
		f.VolumeType, f.Region, f.Terms)
}

// NoSQLDBFilter captures the flags `sku aws dynamodb price/list` exposes.
// resource_name holds the table class slug ("standard" / "standard-ia").
type NoSQLDBFilter struct {
	Provider     string
	Service      string
	ResourceName string // table class for DynamoDB; capacity-mode slug for Cosmos
	Region       string
	Terms        Terms
}

// CDNFilter captures the flags `sku aws cloudfront price/list` exposes.
// resource_name is the CloudFront offering slug ("standard"); region carries
// the canonical edge region (see pipeline/ingest/aws_cloudfront.LOCATION_MAP).
type CDNFilter struct {
	Provider     string
	Service      string
	ResourceName string
	Region       string
	Terms        Terms
}

// LookupNoSQLDB runs the db.nosql point lookup / list query.
func (c *Catalog) LookupNoSQLDB(ctx context.Context, f NoSQLDBFilter) ([]Row, error) {
	if f.ResourceName == "" {
		return nil, fmt.Errorf("catalog: LookupNoSQLDB requires ResourceName")
	}
	return c.lookupResource(ctx, "db.nosql", f.Provider, f.Service,
		f.ResourceName, f.Region, f.Terms)
}

// LookupCDN runs the network.cdn point lookup / list query.
func (c *Catalog) LookupCDN(ctx context.Context, f CDNFilter) ([]Row, error) {
	if f.ResourceName == "" {
		return nil, fmt.Errorf("catalog: LookupCDN requires ResourceName")
	}
	return c.lookupResource(ctx, "network.cdn", f.Provider, f.Service,
		f.ResourceName, f.Region, f.Terms)
}

// CacheKVFilter captures the flags `sku <provider> <cache-service> price/list` exposes.
// resource_name holds the provider-specific node identifier:
//
//	AWS:   "cache.r6g.large"
//	Azure: "Standard C1" / "Premium P1" / "Enterprise E5"
//	GCP:   "memorystore-redis-standard-5gb"
//
// Engine is carried in Terms.Tenancy ("redis" | "memcached").
type CacheKVFilter struct {
	Provider     string
	Service      string
	ResourceName string
	Region       string
	Terms        Terms
}

// LookupCacheKV runs the cache.kv point lookup / list query.
func (c *Catalog) LookupCacheKV(ctx context.Context, f CacheKVFilter) ([]Row, error) {
	if f.ResourceName == "" {
		return nil, fmt.Errorf("catalog: LookupCacheKV requires ResourceName")
	}
	return c.lookupResource(ctx, "cache.kv", f.Provider, f.Service,
		f.ResourceName, f.Region, f.Terms)
}

// lookupResource is the shared scan path for cloud kinds. Region is
// optional (empty region returns every region's row for the given
// resource_name). terms_hash is computed client-side from f.Terms before
// the query so the planner uses idx_skus_lookup (resource_name, region,
// terms_hash); see spec §5.
func (c *Catalog) lookupResource(
	ctx context.Context,
	kind, provider, service, resourceName, region string, terms Terms,
) ([]Row, error) {
	var where []string
	var args []any
	where = append(where, "s.kind = ?")
	args = append(args, kind)
	where = append(where, "s.resource_name = ?")
	args = append(args, resourceName)
	if provider != "" {
		where = append(where, "s.provider = ?")
		args = append(args, provider)
	}
	if service != "" {
		where = append(where, "s.service = ?")
		args = append(args, service)
	}
	if region != "" {
		where = append(where, "s.region = ?")
		args = append(args, region)
	}
	if terms != (Terms{}) {
		h := schema.TermsHash(schema.Terms(terms))
		where = append(where, "s.terms_hash = ?")
		args = append(args, h)
	}

	const queryBase = `
SELECT s.sku_id, s.provider, s.service, s.kind, s.resource_name, s.region,
       s.region_normalized, s.terms_hash,
       t.commitment, t.tenancy, t.os, t.support_tier, t.upfront, t.payment_option,
       ra.vcpu, ra.memory_gb, ra.storage_gb, ra.gpu_count, ra.gpu_model,
       ra.architecture, ra.extra
FROM skus s
JOIN terms t ON t.sku_id = s.sku_id
LEFT JOIN resource_attrs ra ON ra.sku_id = s.sku_id
WHERE `
	query := queryBase + strings.Join(where, " AND ") + "\nORDER BY s.region, s.sku_id" //nolint:gosec // G202: no user input in SQL concatenation

	rs, err := c.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("catalog: lookupResource: %w", err)
	}
	defer func() { _ = rs.Close() }()

	var out []Row
	for rs.Next() {
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
			return nil, err
		}
		r.CatalogVersion = c.catalogVersion
		r.Currency = c.currency
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
		if err := c.FillPrices(ctx, &r); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rs.Err()
}

// ContainerOrchestrationFilter captures the flags `sku <provider> <k8s-service> price/list` exposes.
// resource_name holds the provider-specific cluster identifier:
//
//	AWS:   "eks-standard" / "eks-extended-support" / "eks-fargate"
//	Azure: "aks-free" / "aks-standard" / "aks-premium" / "aks-virtual-nodes-linux" / "aks-virtual-nodes-windows"
//	GCP:   "gke-standard" / "gke-autopilot"
//
// Tier is carried in Terms.OS; product family ("kubernetes") in Terms.Tenancy.
type ContainerOrchestrationFilter struct {
	Provider     string
	Service      string
	ResourceName string
	Region       string
	Terms        Terms
}

// LookupContainerOrchestration runs the container.orchestration point lookup / list query.
func (c *Catalog) LookupContainerOrchestration(ctx context.Context, f ContainerOrchestrationFilter) ([]Row, error) {
	if f.ResourceName == "" {
		return nil, fmt.Errorf("catalog: LookupContainerOrchestration requires ResourceName")
	}
	return c.lookupResource(ctx, "container.orchestration", f.Provider, f.Service,
		f.ResourceName, f.Region, f.Terms)
}

// FillPrices loads the prices rows for r.SKUID and appends them to r.Prices.
// Exported so internal/compare/kinds can reuse the same scan path without
// duplicating the query.
func (c *Catalog) FillPrices(ctx context.Context, r *Row) error {
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
