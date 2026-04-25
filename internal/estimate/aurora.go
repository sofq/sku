package estimate

import (
	"context"
	"fmt"

	"github.com/sofq/sku/internal/catalog"
)

var lookupAurora = func(ctx context.Context, shard string, f catalog.DBRelationalFilter) ([]catalog.Row, error) {
	cat, err := catalog.Open(catalog.ShardPath(shard))
	if err != nil {
		return nil, err
	}
	defer func() { _ = cat.Close() }()
	return cat.LookupDBRelational(ctx, f)
}

type auroraEstimator struct{}

func (auroraEstimator) Kind() string { return "db.relational.aurora" }

func (auroraEstimator) Estimate(ctx context.Context, it Item) (LineItem, error) {
	region := it.Params["region"]
	if region == "" {
		return LineItem{}, fmt.Errorf("estimate/aurora: %q missing :region=<name>", it.Raw)
	}
	engine := param(it.Params, "engine", "aurora-postgres")
	capacityMode := "provisioned"
	instanceType := it.Resource
	if instanceType == "serverless-v2" {
		capacityMode = "serverless-v2"
		instanceType = "aurora-serverless-v2"
	}

	rows, err := lookupAurora(ctx, "aws-aurora", catalog.DBRelationalFilter{
		Provider:     "aws",
		Service:      "aurora",
		InstanceType: instanceType,
		Region:       region,
		Terms:        catalog.Terms{Commitment: "on_demand", Tenancy: engine, OS: "single-az"},
	})
	if err != nil {
		return LineItem{}, fmt.Errorf("estimate/aurora: lookup: %w", err)
	}
	if len(rows) == 0 {
		return LineItem{}, fmt.Errorf("estimate/aurora: no SKU for %s/%s in %s (engine=%s)",
			it.Service, instanceType, region, engine)
	}
	if len(rows) > 1 {
		return LineItem{}, fmt.Errorf("estimate/aurora: ambiguous (%d rows)", len(rows))
	}
	r := rows[0]

	wantUnit := func(u string) bool { return hourlyUnits[u] }
	if capacityMode == "serverless-v2" {
		wantUnit = func(u string) bool { return u == "acu-hr" }
	}
	var hourly float64
	var unit string
	for _, p := range r.Prices {
		if p.Dimension == "compute" && wantUnit(p.Unit) {
			hourly = p.Amount
			unit = p.Unit
			break
		}
	}
	if hourly == 0 {
		return LineItem{}, fmt.Errorf("estimate/aurora: no hourly compute price on SKU %s", r.SKUID)
	}

	var qty float64
	var qErr error
	if capacityMode == "serverless-v2" {
		qty, qErr = paramFloat(it.Params, "acu_hours", 0, 0)
		if qErr != nil {
			return LineItem{}, qErr
		}
		if qty <= 0 {
			return LineItem{}, fmt.Errorf("estimate/aurora: serverless-v2 needs :acu_hours=<n>")
		}
	} else {
		count, cErr := paramInt(it.Params, "count", 1, 1)
		if cErr != nil {
			return LineItem{}, cErr
		}
		hours, hErr := paramFloat(it.Params, "hours", 730, 0)
		if hErr != nil {
			return LineItem{}, hErr
		}
		qty = hours * float64(count)
	}

	return LineItem{
		Item:         it,
		Kind:         "db.relational.aurora",
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

func init() { Register(auroraEstimator{}) }
