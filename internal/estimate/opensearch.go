package estimate

import (
	"context"
	"fmt"

	"github.com/sofq/sku/internal/catalog"
)

var lookupOpenSearch = func(ctx context.Context, shard string, f catalog.SearchEngineFilter) ([]catalog.Row, error) {
	cat, err := catalog.Open(catalog.ShardPath(shard))
	if err != nil {
		return nil, err
	}
	defer func() { _ = cat.Close() }()
	return cat.LookupSearchEngine(ctx, f)
}

type opensearchEstimator struct{}

func (opensearchEstimator) Kind() string { return "search.engine.opensearch" }

func (opensearchEstimator) Estimate(ctx context.Context, it Item) (LineItem, error) {
	region := it.Params["region"]
	if region == "" {
		return LineItem{}, fmt.Errorf("estimate/opensearch: %q missing :region=<name>", it.Raw)
	}

	// Resource is the instance type ("r6g.large.search") or "serverless".
	resource := it.Resource
	mode := "managed-cluster"
	if resource == "serverless" {
		mode = "serverless"
		resource = "opensearch-serverless"
	}

	rows, err := lookupOpenSearch(ctx, "aws-opensearch", catalog.SearchEngineFilter{
		Provider:     "aws",
		Service:      "opensearch",
		ResourceName: resource,
		Region:       region,
		Terms:        catalog.Terms{Commitment: "on_demand", Tenancy: "shared", OS: mode},
	})
	if err != nil {
		return LineItem{}, fmt.Errorf("estimate/opensearch: lookup: %w", err)
	}
	if len(rows) == 0 {
		return LineItem{}, fmt.Errorf("estimate/opensearch: no SKU for %s in %s (mode=%s)",
			it.Resource, region, mode)
	}

	if mode == "managed-cluster" {
		return opensearchManagedCluster(it, rows[0])
	}
	return opensearchServerless(it, rows)
}

func opensearchManagedCluster(it Item, r catalog.Row) (LineItem, error) {
	count, err := paramInt(it.Params, "count", 1, 1)
	if err != nil {
		return LineItem{}, err
	}
	hours, err := paramFloat(it.Params, "hours", 730, 0)
	if err != nil {
		return LineItem{}, err
	}

	var hourly float64
	var unit string
	for _, p := range r.Prices {
		if hourlyUnits[p.Unit] && (hourly == 0 || p.Amount < hourly) {
			hourly = p.Amount
			unit = p.Unit
		}
	}
	if hourly == 0 {
		return LineItem{}, fmt.Errorf("estimate/opensearch: no hourly price on SKU %s", r.SKUID)
	}

	qty := hours * float64(count)
	return LineItem{
		Item:         it,
		Kind:         "search.engine.opensearch",
		SKUID:        r.SKUID,
		Provider:     r.Provider,
		Service:      r.Service,
		Resource:     r.ResourceName,
		Region:       r.Region,
		HourlyUSD:    hourly,
		Quantity:     qty,
		QuantityUnit: unit,
		MonthlyUSD:   hourly * qty,
	}, nil
}

func opensearchServerless(it Item, rows []catalog.Row) (LineItem, error) {
	ocuHours, err := paramFloat(it.Params, "ocu_hours", 0, 0)
	if err != nil {
		return LineItem{}, err
	}
	if ocuHours <= 0 {
		return LineItem{}, fmt.Errorf("estimate/opensearch: serverless needs :ocu_hours=<n>")
	}

	// Find the compute-OCU price. The serverless ingest emits one logical
	// SKU per region with three price dimensions (compute-ocu / indexing-ocu
	// / storage); :ocu_hours bills against compute-ocu only.
	var r catalog.Row
	var ocuPrice float64
	for _, row := range rows {
		for _, p := range row.Prices {
			if p.Dimension == "compute-ocu" {
				r = row
				ocuPrice = p.Amount
			}
		}
	}
	if ocuPrice == 0 {
		return LineItem{}, fmt.Errorf("estimate/opensearch: no OCU price in serverless rows")
	}

	return LineItem{
		Item:         it,
		Kind:         "search.engine.opensearch",
		SKUID:        r.SKUID,
		Provider:     r.Provider,
		Service:      r.Service,
		Resource:     r.ResourceName,
		Region:       r.Region,
		HourlyUSD:    ocuPrice,
		Quantity:     ocuHours,
		QuantityUnit: "ocu-hour",
		MonthlyUSD:   ocuPrice * ocuHours,
	}, nil
}

func init() { Register(opensearchEstimator{}) }
