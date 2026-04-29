package estimate

import (
	"context"
	"fmt"

	"github.com/sofq/sku/internal/catalog"
)

var lookupBigQuery = func(ctx context.Context, shard string, f catalog.WarehouseQueryFilter) ([]catalog.Row, error) {
	cat, err := catalog.Open(catalog.ShardPath(shard))
	if err != nil {
		return nil, err
	}
	defer func() { _ = cat.Close() }()
	return cat.LookupWarehouseQuery(ctx, f)
}

type bigqueryEstimator struct{}

func (bigqueryEstimator) Kind() string { return "warehouse.query.bigquery" }

// Estimate dispatches to the correct BigQuery pricing model based on resource:
//
//	on-demand             — :tb_queried=<n>
//	capacity-standard     — :slots=<n>:hours=<n>
//	capacity-enterprise   — :slots=<n>:hours=<n>
//	storage-active        — :gb_month=<n>
//	storage-long-term     — :gb_month=<n>
func (bigqueryEstimator) Estimate(ctx context.Context, it Item) (LineItem, error) {
	region := it.Params["region"]
	if region == "" {
		return LineItem{}, fmt.Errorf("estimate/bigquery: %q missing :region=<name>", it.Raw)
	}

	resource := it.Resource
	switch resource {
	case "on-demand", "capacity-standard", "capacity-enterprise",
		"storage-active", "storage-long-term":
	default:
		return LineItem{}, fmt.Errorf(
			"estimate/bigquery: resource must be on-demand|capacity-standard|capacity-enterprise|"+
				"storage-active|storage-long-term, got %q", resource)
	}

	rows, err := lookupBigQuery(ctx, "gcp-bigquery", catalog.WarehouseQueryFilter{
		Provider:     "gcp",
		Service:      "bigquery",
		ResourceName: resource,
		Region:       region,
		Terms:        catalog.Terms{Commitment: "on_demand", OS: "on-demand"},
	})
	if err != nil {
		return LineItem{}, fmt.Errorf("estimate/bigquery: lookup: %w", err)
	}
	if len(rows) == 0 {
		return LineItem{}, fmt.Errorf("estimate/bigquery: no SKU for %s in %s", resource, region)
	}
	r := rows[0]

	switch resource {
	case "on-demand":
		return bigqueryOnDemand(it, r)
	case "capacity-standard", "capacity-enterprise":
		return bigqueryCapacity(it, r)
	default: // storage-active, storage-long-term
		return bigqueryStorage(it, r)
	}
}

func bigqueryOnDemand(it Item, r catalog.Row) (LineItem, error) {
	tbQueried, err := paramFloat(it.Params, "tb_queried", 0, 0)
	if err != nil {
		return LineItem{}, err
	}
	if tbQueried <= 0 {
		return LineItem{}, fmt.Errorf("estimate/bigquery: on-demand needs :tb_queried=<n>")
	}

	var pricePerTB float64
	for _, p := range r.Prices {
		if p.Unit == "tb" {
			pricePerTB = p.Amount
			break
		}
	}
	if pricePerTB == 0 {
		return LineItem{}, fmt.Errorf("estimate/bigquery: no per-tb price on SKU %s", r.SKUID)
	}

	return LineItem{
		Item:         it,
		Kind:         "warehouse.query.bigquery",
		SKUID:        r.SKUID,
		Provider:     r.Provider,
		Service:      r.Service,
		Resource:     r.ResourceName,
		Region:       r.Region,
		HourlyUSD:    pricePerTB,
		Quantity:     tbQueried,
		QuantityUnit: "tb",
		MonthlyUSD:   pricePerTB * tbQueried,
	}, nil
}

func bigqueryCapacity(it Item, r catalog.Row) (LineItem, error) {
	slots, err := paramFloat(it.Params, "slots", 0, 0)
	if err != nil {
		return LineItem{}, err
	}
	if slots <= 0 {
		return LineItem{}, fmt.Errorf("estimate/bigquery: capacity needs :slots=<n>")
	}
	hours, err := paramFloat(it.Params, "hours", 730, 0)
	if err != nil {
		return LineItem{}, err
	}

	var pricePerSlotHour float64
	for _, p := range r.Prices {
		if p.Unit == "slot-hour" {
			pricePerSlotHour = p.Amount
			break
		}
	}
	if pricePerSlotHour == 0 {
		return LineItem{}, fmt.Errorf("estimate/bigquery: no slot-hour price on SKU %s", r.SKUID)
	}

	qty := slots * hours
	return LineItem{
		Item:         it,
		Kind:         "warehouse.query.bigquery",
		SKUID:        r.SKUID,
		Provider:     r.Provider,
		Service:      r.Service,
		Resource:     r.ResourceName,
		Region:       r.Region,
		HourlyUSD:    pricePerSlotHour,
		Quantity:     qty,
		QuantityUnit: "slot-hour",
		MonthlyUSD:   pricePerSlotHour * qty,
	}, nil
}

func bigqueryStorage(it Item, r catalog.Row) (LineItem, error) {
	gbMonth, err := paramFloat(it.Params, "gb_month", 0, 0)
	if err != nil {
		return LineItem{}, err
	}
	if gbMonth <= 0 {
		return LineItem{}, fmt.Errorf("estimate/bigquery: storage needs :gb_month=<n>")
	}

	var pricePerGB float64
	for _, p := range r.Prices {
		if p.Unit == "gb-month" {
			pricePerGB = p.Amount
			break
		}
	}
	if pricePerGB == 0 {
		return LineItem{}, fmt.Errorf("estimate/bigquery: no gb-month price on SKU %s", r.SKUID)
	}

	return LineItem{
		Item:         it,
		Kind:         "warehouse.query.bigquery",
		SKUID:        r.SKUID,
		Provider:     r.Provider,
		Service:      r.Service,
		Resource:     r.ResourceName,
		Region:       r.Region,
		HourlyUSD:    pricePerGB,
		Quantity:     gbMonth,
		QuantityUnit: "gb-month",
		MonthlyUSD:   pricePerGB * gbMonth,
	}, nil
}

func init() { Register(bigqueryEstimator{}) }
