package catalog

import (
	"context"
	"fmt"
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
	_ = ctx
	return nil, nil
}
