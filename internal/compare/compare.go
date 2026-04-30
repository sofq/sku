package compare

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"golang.org/x/sync/errgroup"

	"github.com/sofq/sku/internal/catalog"
	"github.com/sofq/sku/internal/compare/kinds"
)

// ShardTarget names an installed shard and its absolute file path. The caller
// (cmd/sku/compare) assembles these from catalog.InstalledShards +
// catalog.ShardPath so Run stays oblivious to path-resolution rules.
type ShardTarget struct {
	Name string
	Path string
}

// Request is the cross-provider compare query.
type Request struct {
	Kind     string
	VCPU     int64
	MemoryGB float64
	GPUCount int64
	MaxPrice float64

	// storage.object
	StorageClass     string
	DurabilityNines  int64
	AvailabilityTier string

	// db.relational
	StorageGB        float64
	Engine           string
	DeploymentOption string

	// container.orchestration
	Tier string
	Mode string

	// paas.app
	PlanOS string

	// warehouse.query
	Edition     string
	StorageTier string

	Regions []string
	Sort    string
	Limit   int
	Targets []ShardTarget
}

const defaultLimit = 20

type kindQuery func(ctx context.Context, c *catalog.Catalog, req Request) ([]catalog.Row, error)

var kindRegistry = map[string]kindQuery{
	"compute.vm": func(ctx context.Context, c *catalog.Catalog, r Request) ([]catalog.Row, error) {
		return kinds.QueryVM(ctx, c, kinds.VMSpec{
			VCPU: r.VCPU, MemoryGB: r.MemoryGB, GPUCount: r.GPUCount,
			MaxPrice: r.MaxPrice, Regions: r.Regions,
		})
	},
	"storage.object": func(ctx context.Context, c *catalog.Catalog, r Request) ([]catalog.Row, error) {
		return kinds.QueryStorageObject(ctx, c, kinds.StorageObjectSpec{
			StorageClass:     r.StorageClass,
			DurabilityNines:  r.DurabilityNines,
			AvailabilityTier: r.AvailabilityTier,
			MaxPrice:         r.MaxPrice,
			Regions:          r.Regions,
		})
	},
	"db.relational": func(ctx context.Context, c *catalog.Catalog, r Request) ([]catalog.Row, error) {
		return kinds.QueryDBRelational(ctx, c, kinds.DBRelationalSpec{
			VCPU:             r.VCPU,
			MemoryGB:         r.MemoryGB,
			StorageGB:        r.StorageGB,
			Engine:           r.Engine,
			DeploymentOption: r.DeploymentOption,
			MaxPrice:         r.MaxPrice,
			Regions:          r.Regions,
		})
	},
	"cache.kv": func(ctx context.Context, c *catalog.Catalog, r Request) ([]catalog.Row, error) {
		return kinds.QueryCacheKV(ctx, c, kinds.CacheKVSpec{
			MemoryGB: r.MemoryGB,
			Engine:   r.Engine,
			MaxPrice: r.MaxPrice,
			Regions:  r.Regions,
		})
	},
	"container.orchestration": func(ctx context.Context, c *catalog.Catalog, r Request) ([]catalog.Row, error) {
		return kinds.QueryContainerOrchestration(ctx, c, kinds.ContainerOrchestrationSpec{
			Mode:     r.Mode,
			Tier:     r.Tier,
			MaxPrice: r.MaxPrice,
			Regions:  r.Regions,
		})
	},
	"search.engine": func(ctx context.Context, c *catalog.Catalog, r Request) ([]catalog.Row, error) {
		return kinds.QuerySearchEngine(ctx, c, kinds.SearchEngineSpec{
			Mode:        r.Mode,
			MinVCPU:     r.VCPU,
			MinMemoryGB: r.MemoryGB,
			MaxPrice:    r.MaxPrice,
			Regions:     r.Regions,
		})
	},
	"paas.app": func(ctx context.Context, c *catalog.Catalog, r Request) ([]catalog.Row, error) {
		return kinds.QueryPaasApp(ctx, c, kinds.PaasAppSpec{
			PlanOS:      r.PlanOS,
			Tier:        r.Tier,
			MinVCPU:     r.VCPU,
			MinMemoryGB: r.MemoryGB,
			MaxPrice:    r.MaxPrice,
			Regions:     r.Regions,
		})
	},
	"warehouse.query": func(ctx context.Context, c *catalog.Catalog, r Request) ([]catalog.Row, error) {
		return kinds.QueryWarehouseQuery(ctx, c, kinds.WarehouseQuerySpec{
			Mode:        r.Mode,
			Edition:     r.Edition,
			StorageTier: r.StorageTier,
			MaxPrice:    r.MaxPrice,
			Regions:     r.Regions,
		})
	},
}

func supportedKinds() string {
	keys := make([]string, 0, len(kindRegistry))
	for k := range kindRegistry {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return strings.Join(keys, ", ")
}

// Run fans out the request across every target shard in parallel, merges the
// returned rows, applies Sort + Limit, and returns. Rows are safe to render.
func Run(ctx context.Context, req Request) ([]catalog.Row, error) {
	query, ok := kindRegistry[req.Kind]
	if !ok {
		return nil, fmt.Errorf("compare: unsupported kind %q; supported: %s", req.Kind, supportedKinds())
	}
	if len(req.Targets) == 0 {
		return nil, fmt.Errorf("compare: no shards available; run `sku update` or pass --auto-fetch (m5)")
	}
	limit := req.Limit
	if limit <= 0 {
		limit = defaultLimit
	}

	slots := make([][]catalog.Row, len(req.Targets))
	g, gctx := errgroup.WithContext(ctx)
	for i, t := range req.Targets {
		i, t := i, t
		g.Go(func() error {
			cat, err := catalog.Open(t.Path)
			if err != nil {
				return fmt.Errorf("compare: open %s: %w", t.Name, err)
			}
			defer func() { _ = cat.Close() }()
			rows, err := query(gctx, cat, req)
			if err != nil {
				return fmt.Errorf("compare: %s: %w", t.Name, err)
			}
			for j := range rows {
				rows[j].CatalogVersion = cat.CatalogVersion()
				rows[j].Currency = cat.Currency()
			}
			slots[i] = rows
			return nil
		})
	}
	if err := g.Wait(); err != nil {
		return nil, err
	}

	var all []catalog.Row
	for _, s := range slots {
		all = append(all, s...)
	}
	sortRows(all, req.Sort)
	if limit > 0 && len(all) > limit {
		all = all[:limit]
	}
	return all, nil
}

// sortRows applies a stable sort matching the user's --sort choice. Ties break
// on (provider, resource_name, sku_id) for diff-friendly output.
func sortRows(rows []catalog.Row, key string) {
	less := func(i, j int) bool {
		a, b := rows[i], rows[j]
		switch key {
		case "vcpu":
			ai, bi := derefI64(a.ResourceAttrs.VCPU), derefI64(b.ResourceAttrs.VCPU)
			if ai != bi {
				return ai < bi
			}
		case "memory":
			af, bf := derefF64(a.ResourceAttrs.MemoryGB), derefF64(b.ResourceAttrs.MemoryGB)
			if af != bf {
				return af < bf
			}
		default:
			if a.MinPrice != b.MinPrice {
				return a.MinPrice < b.MinPrice
			}
		}
		if a.Provider != b.Provider {
			return a.Provider < b.Provider
		}
		if a.ResourceName != b.ResourceName {
			return a.ResourceName < b.ResourceName
		}
		return a.SKUID < b.SKUID
	}
	sort.SliceStable(rows, less)
}

func derefI64(p *int64) int64 {
	if p == nil {
		return 0
	}
	return *p
}

func derefF64(p *float64) float64 {
	if p == nil {
		return 0
	}
	return *p
}
