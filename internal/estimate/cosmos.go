package estimate

import (
	"context"
	"fmt"

	"github.com/sofq/sku/internal/catalog"
)

var lookupCosmos = func(ctx context.Context, shard string, f catalog.NoSQLDBFilter) ([]catalog.Row, error) {
	cat, err := catalog.Open(catalog.ShardPath(shard))
	if err != nil {
		return nil, err
	}
	defer func() { _ = cat.Close() }()
	return cat.LookupNoSQLDB(ctx, f)
}

type cosmosEstimator struct{}

func (cosmosEstimator) Kind() string { return "db.nosql.cosmos" }

func (cosmosEstimator) Estimate(ctx context.Context, it Item) (LineItem, error) {
	region := it.Params["region"]
	if region == "" {
		return LineItem{}, fmt.Errorf("estimate/cosmos: missing :region=<name>")
	}
	api := param(it.Params, "api", "sql")
	capacityMode := it.Resource
	if capacityMode != "provisioned" && capacityMode != "serverless" {
		return LineItem{}, fmt.Errorf("estimate/cosmos: capacity-mode must be provisioned|serverless, got %q", capacityMode)
	}
	resourceName := "cosmos-" + capacityMode

	rows, err := lookupCosmos(ctx, "azure-cosmosdb", catalog.NoSQLDBFilter{
		Provider:     "azure",
		Service:      "cosmosdb",
		ResourceName: resourceName,
		Region:       region,
		Terms:        catalog.Terms{Commitment: "on_demand", Tenancy: api, OS: capacityMode},
	})
	if err != nil {
		return LineItem{}, fmt.Errorf("estimate/cosmos: lookup: %w", err)
	}
	if len(rows) == 0 {
		return LineItem{}, fmt.Errorf("estimate/cosmos: no SKU for %s api=%s in %s", capacityMode, api, region)
	}
	r := rows[0]

	var qty, monthly, hourly float64
	var qtyUnit string

	if capacityMode == "provisioned" {
		ruPerSec, err := paramFloat(it.Params, "ru_per_sec", 0, 1)
		if err != nil {
			return LineItem{}, err
		}
		hours, err := paramFloat(it.Params, "hours", 730, 0)
		if err != nil {
			return LineItem{}, err
		}
		for _, p := range r.Prices {
			if p.Dimension == "provisioned" {
				hourly = p.Amount
				break
			}
		}
		qty = ruPerSec * hours
		qtyUnit = "ru/s-hour"
		monthly = hourly * qty
	} else {
		ruMillion, err := paramFloat(it.Params, "ru_million", 0, 1)
		if err != nil {
			return LineItem{}, err
		}
		for _, p := range r.Prices {
			if p.Dimension == "serverless" {
				hourly = p.Amount
				break
			}
		}
		qty = ruMillion
		qtyUnit = "1M-ru"
		monthly = hourly * qty
	}

	return LineItem{
		Item:         it,
		Kind:         "db.nosql.cosmos",
		SKUID:        r.SKUID,
		Provider:     r.Provider,
		Service:      r.Service,
		Resource:     r.ResourceName,
		Region:       r.Region,
		HourlyUSD:    hourly,
		Quantity:     qty,
		QuantityUnit: qtyUnit,
		MonthlyUSD:   monthly,
	}, nil
}

func init() { Register(cosmosEstimator{}) }
