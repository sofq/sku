package estimate

import (
	"context"
	"fmt"

	"github.com/sofq/sku/internal/catalog"
)

var lookupAppService = func(ctx context.Context, shard string, f catalog.PaasAppFilter) ([]catalog.Row, error) {
	cat, err := catalog.Open(catalog.ShardPath(shard))
	if err != nil {
		return nil, err
	}
	defer func() { _ = cat.Close() }()
	return cat.LookupPaasApp(ctx, f)
}

type appServiceEstimator struct{}

func (appServiceEstimator) Kind() string { return "paas.app.appservice" }

func (appServiceEstimator) Estimate(ctx context.Context, it Item) (LineItem, error) {
	region := it.Params["region"]
	if region == "" {
		return LineItem{}, fmt.Errorf("estimate/appservice: %q missing :region=<name>", it.Raw)
	}
	// Resource is the plan SKU: "P1v3", "B2", "S1", etc.
	sku := it.Resource
	if sku == "" {
		return LineItem{}, fmt.Errorf("estimate/appservice: missing plan SKU in resource")
	}
	os := param(it.Params, "os", "linux")
	tier := it.Params["tier"]

	rows, err := lookupAppService(ctx, "azure-appservice", catalog.PaasAppFilter{
		Provider:     "azure",
		Service:      "appservice",
		ResourceName: sku,
		Region:       region,
		Terms:        catalog.Terms{OS: os, SupportTier: tier},
	})
	if err != nil {
		return LineItem{}, fmt.Errorf("estimate/appservice: lookup: %w", err)
	}
	if len(rows) == 0 {
		return LineItem{}, fmt.Errorf("estimate/appservice: no SKU for %s in %s (os=%s, tier=%s)",
			sku, region, os, tier)
	}

	r := rows[0]
	var hourly float64
	var unit string
	for _, p := range r.Prices {
		if hourlyUnits[p.Unit] && (hourly == 0 || p.Amount < hourly) {
			hourly = p.Amount
			unit = p.Unit
		}
	}
	if hourly == 0 {
		return LineItem{}, fmt.Errorf("estimate/appservice: no hourly price on SKU %s", r.SKUID)
	}

	count, err := paramInt(it.Params, "count", 1, 1)
	if err != nil {
		return LineItem{}, err
	}
	hours, err := paramFloat(it.Params, "hours", 730, 0)
	if err != nil {
		return LineItem{}, err
	}

	qty := hours * float64(count)
	return LineItem{
		Item:         it,
		Kind:         "paas.app.appservice",
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

func init() { Register(appServiceEstimator{}) }
