package compare

import (
	"context"
	"fmt"
	"sort"

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
	Regions  []string
	Sort     string
	Limit    int
	Targets  []ShardTarget
}

const defaultLimit = 20

// Run fans out the request across every target shard in parallel, merges the
// returned rows, applies Sort + Limit, and returns. Rows are safe to render.
func Run(ctx context.Context, req Request) ([]catalog.Row, error) {
	if req.Kind != "compute.vm" {
		return nil, fmt.Errorf("compare: kind %q not supported in m4.2 (compute.vm only)", req.Kind)
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
			rows, err := kinds.QueryVM(gctx, cat, kinds.VMSpec{
				VCPU:     req.VCPU,
				MemoryGB: req.MemoryGB,
				GPUCount: req.GPUCount,
				MaxPrice: req.MaxPrice,
				Regions:  req.Regions,
			})
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
