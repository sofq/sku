package estimate

import (
	"context"
	"fmt"

	"github.com/sofq/sku/internal/catalog"
)

var lookupAPIGateway = func(ctx context.Context, shard string, f catalog.APIGatewayFilter) ([]catalog.Row, error) {
	cat, err := catalog.Open(catalog.ShardPath(shard))
	if err != nil {
		return nil, err
	}
	defer func() { _ = cat.Close() }()
	return cat.LookupAPIGateway(ctx, f)
}

var providerServiceShardAPIGateway = map[string]string{
	"aws/api-gateway": "aws-api-gateway",
	"azure/apim":      "azure-apim",
}

// apiGatewayEstimator handles api.gateway kind estimation.
type apiGatewayEstimator struct{}

func (apiGatewayEstimator) Kind() string { return "api.gateway" }

func (apiGatewayEstimator) Estimate(ctx context.Context, it Item) (LineItem, error) {
	region := it.Params["region"]
	if region == "" {
		return LineItem{}, fmt.Errorf("estimate/api.gateway: %q missing :region=<name>", it.Raw)
	}
	psKey := it.Provider + "/" + it.Service
	shard, ok := providerServiceShardAPIGateway[psKey]
	if !ok {
		return LineItem{}, fmt.Errorf("estimate/api.gateway: no shard for %s", psKey)
	}

	rows, err := lookupAPIGateway(ctx, shard, catalog.APIGatewayFilter{
		Provider:     it.Provider,
		Service:      it.Service,
		ResourceName: it.Resource,
		Region:       region,
		Terms:        catalog.Terms{Commitment: "on_demand"},
	})
	if err != nil {
		return LineItem{}, fmt.Errorf("estimate/api.gateway: lookup: %w", err)
	}
	if len(rows) == 0 {
		return LineItem{}, fmt.Errorf("estimate/api.gateway: no SKU for %s/%s:%s in %s",
			it.Provider, it.Service, it.Resource, region)
	}
	r := rows[0]

	// Detect pricing shape: per-call vs per-unit-hour.
	hasUnitHour := false
	for _, p := range r.Prices {
		if p.Dimension == "unit_hour" {
			hasUnitHour = true
			break
		}
	}

	if hasUnitHour {
		// Per-unit-hour: APIM provisioned tiers.
		if it.Params["requests"] != "" {
			return LineItem{}, fmt.Errorf("estimate/api.gateway: %s/%s:%s is unit-hour pricing; use :units=<n>:hours=<n>",
				it.Provider, it.Service, it.Resource)
		}
		units, err := paramFloat(it.Params, "units", 1, 1)
		if err != nil {
			return LineItem{}, err
		}
		hours, err := paramFloat(it.Params, "hours", 730, 0)
		if err != nil {
			return LineItem{}, err
		}
		amount, found := flatDimPrice(r.Prices, "unit_hour")
		if !found {
			return LineItem{}, fmt.Errorf("estimate/api.gateway: no unit_hour price on SKU %s", r.SKUID)
		}
		return LineItem{
			Item: it, Kind: "api.gateway",
			SKUID: r.SKUID, Provider: r.Provider, Service: r.Service,
			Resource: r.ResourceName, Region: r.Region,
			HourlyUSD: amount, Quantity: units * hours, QuantityUnit: "unit-hr",
			MonthlyUSD: amount * units * hours,
		}, nil
	}

	// Per-call: AWS API Gateway or APIM consumption.
	requests, err := paramFloat(it.Params, "requests", 0, 0)
	if err != nil {
		return LineItem{}, err
	}
	if requests == 0 {
		return LineItem{}, fmt.Errorf("estimate/api.gateway: %q requires :requests=<n> or :units=<n>:hours=<n>", it.Raw)
	}
	if it.Params["units"] != "" || it.Params["hours"] != "" {
		return LineItem{}, fmt.Errorf("estimate/api.gateway: %s/%s:%s is per-call pricing; use :requests=<n>",
			it.Provider, it.Service, it.Resource)
	}

	for _, dim := range []string{"request", "call"} {
		entries, err := pricesToTierEntriesCount(r.Prices, dim)
		if err != nil {
			return LineItem{}, fmt.Errorf("estimate/api.gateway: tier parse: %w", err)
		}
		if len(entries) == 0 {
			continue
		}
		cost := WalkTiers(entries, requests)
		return LineItem{
			Item: it, Kind: "api.gateway",
			SKUID: r.SKUID, Provider: r.Provider, Service: r.Service,
			Resource: r.ResourceName, Region: r.Region,
			Quantity: requests, QuantityUnit: dim,
			MonthlyUSD: cost,
		}, nil
	}

	return LineItem{}, fmt.Errorf("estimate/api.gateway: no request/call dimension for %s/%s:%s", it.Provider, it.Service, it.Resource)
}

func init() {
	Register(apiGatewayEstimator{})
}
