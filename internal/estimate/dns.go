package estimate

import (
	"context"
	"fmt"

	"github.com/sofq/sku/internal/catalog"
)

var lookupDNSZone = func(ctx context.Context, shard string, f catalog.DNSZoneFilter) ([]catalog.Row, error) {
	cat, err := catalog.Open(catalog.ShardPath(shard))
	if err != nil {
		return nil, err
	}
	defer func() { _ = cat.Close() }()
	return cat.LookupDNSZone(ctx, f)
}

var providerServiceShardDNS = map[string]string{
	"aws/route53":   "aws-route53",
	"gcp/cloud-dns": "gcp-cloud-dns",
}

// dnsZoneEstimator handles dns.zone kind estimation.
type dnsZoneEstimator struct{}

func (dnsZoneEstimator) Kind() string { return "dns.zone" }

func (dnsZoneEstimator) Estimate(ctx context.Context, it Item) (LineItem, error) {
	region := param(it.Params, "region", "global")
	mode := param(it.Params, "mode", "public")

	psKey := it.Provider + "/" + it.Service
	shard, ok := providerServiceShardDNS[psKey]
	if !ok {
		return LineItem{}, fmt.Errorf("estimate/dns.zone: no shard for %s", psKey)
	}

	resource := it.Resource
	if resource == "" {
		resource = mode
	}

	rows, err := lookupDNSZone(ctx, shard, catalog.DNSZoneFilter{
		Provider:     it.Provider,
		Service:      it.Service,
		ResourceName: resource,
		Region:       region,
		Terms:        catalog.Terms{Commitment: "on_demand"},
	})
	if err != nil {
		return LineItem{}, fmt.Errorf("estimate/dns.zone: lookup: %w", err)
	}
	if len(rows) == 0 {
		return LineItem{}, fmt.Errorf("estimate/dns.zone: no SKU for %s/%s:%s in %s",
			it.Provider, it.Service, resource, region)
	}
	r := rows[0]

	zones, err := paramFloat(it.Params, "zones", 0, 0)
	if err != nil {
		return LineItem{}, err
	}
	queries, err := paramFloat(it.Params, "queries", 0, 0)
	if err != nil {
		return LineItem{}, err
	}
	if zones == 0 && queries == 0 {
		return LineItem{}, fmt.Errorf("estimate/dns.zone: %q requires :zones=<n> and/or :queries=<n>", it.Raw)
	}

	var totalUSD float64

	if zones > 0 {
		entries, err := pricesToTierEntriesCount(r.Prices, "hosted_zone")
		if err != nil {
			return LineItem{}, fmt.Errorf("estimate/dns.zone: zone tier parse: %w", err)
		}
		totalUSD += WalkTiers(entries, zones)
	}

	if queries > 0 {
		entries, err := pricesToTierEntriesCount(r.Prices, "query")
		if err != nil {
			return LineItem{}, fmt.Errorf("estimate/dns.zone: query tier parse: %w", err)
		}
		totalUSD += WalkTiers(entries, queries)
	}

	return LineItem{
		Item: it, Kind: "dns.zone",
		SKUID: r.SKUID, Provider: r.Provider, Service: r.Service,
		Resource: r.ResourceName, Region: r.Region,
		MonthlyUSD: totalUSD,
	}, nil
}

func init() {
	Register(dnsZoneEstimator{})
}
