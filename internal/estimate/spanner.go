package estimate

import (
	"context"
	"fmt"

	"github.com/sofq/sku/internal/catalog"
)

var lookupSpanner = func(ctx context.Context, shard string, f catalog.DBRelationalFilter) ([]catalog.Row, error) {
	cat, err := catalog.Open(catalog.ShardPath(shard))
	if err != nil {
		return nil, err
	}
	defer func() { _ = cat.Close() }()
	return cat.LookupDBRelational(ctx, f)
}

type spannerEstimator struct{}

func (spannerEstimator) Kind() string { return "db.relational.spanner" }

func (spannerEstimator) Estimate(ctx context.Context, it Item) (LineItem, error) {
	region := it.Params["region"]
	if region == "" {
		return LineItem{}, fmt.Errorf("estimate/spanner: missing :region=<name>")
	}
	edition := it.Resource
	switch edition {
	case "standard", "enterprise", "enterprise-plus":
	default:
		return LineItem{}, fmt.Errorf("estimate/spanner: edition must be standard|enterprise|enterprise-plus, got %q", edition)
	}

	rows, err := lookupSpanner(ctx, "gcp-spanner", catalog.DBRelationalFilter{
		Provider:     "gcp",
		Service:      "spanner",
		InstanceType: "spanner-" + edition,
		Region:       region,
		Terms:        catalog.Terms{Commitment: "on_demand", Tenancy: "spanner-" + edition},
	})
	if err != nil {
		return LineItem{}, fmt.Errorf("estimate/spanner: lookup: %w", err)
	}
	if len(rows) == 0 {
		return LineItem{}, fmt.Errorf("estimate/spanner: no SKU for edition=%s in %s", edition, region)
	}
	r := rows[0]

	pu, err := paramFloat(it.Params, "pu", 0, 0)
	if err != nil {
		return LineItem{}, err
	}
	if pu == 0 {
		nodes, nErr := paramFloat(it.Params, "node", 0, 0)
		if nErr != nil {
			return LineItem{}, nErr
		}
		if nodes == 0 {
			return LineItem{}, fmt.Errorf("estimate/spanner: pass :pu=<n> or :node=<n>")
		}
		pu = nodes * 1000
	}
	hours, err := paramFloat(it.Params, "hours", 730, 0)
	if err != nil {
		return LineItem{}, err
	}

	var hourlyPerPU float64
	for _, p := range r.Prices {
		if p.Dimension == "compute" {
			hourlyPerPU = p.Amount
			break
		}
	}
	if hourlyPerPU == 0 {
		return LineItem{}, fmt.Errorf("estimate/spanner: no compute price on SKU %s", r.SKUID)
	}

	qty := pu * hours
	return LineItem{
		Item:         it,
		Kind:         "db.relational.spanner",
		SKUID:        r.SKUID,
		Provider:     r.Provider,
		Service:      r.Service,
		Resource:     r.ResourceName,
		Region:       r.Region,
		HourlyUSD:    hourlyPerPU,
		Quantity:     qty,
		QuantityUnit: "pu-hour",
		MonthlyUSD:   hourlyPerPU * qty,
	}, nil
}

func init() { Register(spannerEstimator{}) }
