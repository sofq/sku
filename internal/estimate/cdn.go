package estimate

import (
	"context"
	"fmt"
	"math"

	"github.com/sofq/sku/internal/catalog"
)

var lookupCDN = func(ctx context.Context, shard string, f catalog.CDNFilter) ([]catalog.Row, error) {
	cat, err := catalog.Open(catalog.ShardPath(shard))
	if err != nil {
		return nil, err
	}
	defer func() { _ = cat.Close() }()
	return cat.LookupCDN(ctx, f)
}

var providerServiceShardCDN = map[string]string{
	"aws/cloudfront":   "aws-cloudfront",
	"azure/front-door": "azure-front-door",
	"gcp/cloud-cdn":    "gcp-cloud-cdn",
}

// extraMode returns the "mode" value from a row's ResourceAttrs.Extra map.
func extraMode(r catalog.Row) string {
	if m, ok := r.ResourceAttrs.Extra["mode"]; ok {
		if s, ok := m.(string); ok {
			return s
		}
	}
	return ""
}

// extraSku returns the "sku" value from a row's ResourceAttrs.Extra map.
func extraSku(r catalog.Row) string {
	if s, ok := r.ResourceAttrs.Extra["sku"]; ok {
		if sv, ok := s.(string); ok {
			return sv
		}
	}
	return ""
}

// networkCDNTier0Estimator handles network.cdn kind estimation (tier-0 only).
type networkCDNTier0Estimator struct{}

func (networkCDNTier0Estimator) Kind() string { return "network.cdn" }

func (networkCDNTier0Estimator) Estimate(ctx context.Context, it Item) (LineItem, error) {
	region := it.Params["region"]
	if region == "" {
		return LineItem{}, fmt.Errorf("estimate/network.cdn: %q missing :region=<name>", it.Raw)
	}
	mode := param(it.Params, "mode", "edge-egress")
	gbStr := it.Params["gb"]
	if gbStr == "" {
		return LineItem{}, fmt.Errorf("estimate/network.cdn: %q missing :gb=<n>", it.Raw)
	}
	gb, err := paramFloat(it.Params, "gb", 0, 0)
	if err != nil {
		return LineItem{}, err
	}

	psKey := it.Provider + "/" + it.Service
	shard, ok := providerServiceShardCDN[psKey]
	if !ok {
		return LineItem{}, fmt.Errorf("estimate/network.cdn: no shard for %s", psKey)
	}

	// Look up egress rows for the requested region.
	egressRows, err := lookupCDN(ctx, shard, catalog.CDNFilter{
		Provider:     it.Provider,
		Service:      it.Service,
		ResourceName: it.Resource,
		Region:       region,
		Terms:        catalog.Terms{Commitment: "on_demand"},
	})
	if err != nil {
		return LineItem{}, fmt.Errorf("estimate/network.cdn: lookup: %w", err)
	}

	// Find egress row matching the requested mode.
	var egressRow *catalog.Row
	for i := range egressRows {
		if extraMode(egressRows[i]) == mode {
			egressRow = &egressRows[i]
			break
		}
	}
	if egressRow == nil {
		// Single-row CDN providers store all prices on one row; use first.
		if len(egressRows) == 1 {
			egressRow = &egressRows[0]
		} else {
			return LineItem{}, fmt.Errorf("estimate/network.cdn: no row with mode=%q for %s/%s:%s in %s",
				mode, it.Provider, it.Service, it.Resource, region)
		}
	}

	// Find the tier-0 entry on the egress dimension.
	var tier0Price *catalog.Price
	for i := range egressRow.Prices {
		p := &egressRow.Prices[i]
		if p.Dimension == "egress" && p.Tier == "0" {
			tier0Price = p
			break
		}
	}
	if tier0Price == nil {
		return LineItem{}, fmt.Errorf("estimate/network.cdn: no tier=0 egress price on SKU %s", egressRow.SKUID)
	}

	// Parse tier-0 upper bound.
	tier0Upper, err := parseTierBoundBytes(tier0Price.TierUpper)
	if err != nil {
		return LineItem{}, fmt.Errorf("estimate/network.cdn: tier_upper parse: %w", err)
	}
	// Convert GB input to bytes for comparison.
	gbInBytes := gb * 1e9

	if tier0Upper != math.MaxFloat64 && gbInBytes > tier0Upper {
		tier0UpperGB := tier0Upper / 1e9
		return LineItem{}, fmt.Errorf(
			"volume %.0f GB exceeds tier-0 boundary (%.0f GB) on network.cdn:%s mode %s; "+
				"multi-tier walking is deferred to M-ε; use 'sku compare --kind network.cdn --gb %.0f' for per-tier prices",
			gb, tier0UpperGB, it.Resource, mode, gb)
	}

	// Charge = gb × tier-0 amount.
	cost := gb * tier0Price.Amount

	// Base-fee join: find base-fee rows for same (provider, service, resource_name) in global region.
	var baseFee float64
	skuToken := extraSku(*egressRow)
	if skuToken != "" {
		baseFeeRows, err := lookupCDN(ctx, shard, catalog.CDNFilter{
			Provider:     it.Provider,
			Service:      it.Service,
			ResourceName: it.Resource,
			Region:       "global",
			Terms:        catalog.Terms{Commitment: "on_demand"},
		})
		if err == nil {
			for _, bfRow := range baseFeeRows {
				if extraMode(bfRow) == "base-fee" && extraSku(bfRow) == skuToken {
					for _, p := range bfRow.Prices {
						baseFee += p.Amount
					}
				}
			}
		}
	}

	return LineItem{
		Item: it, Kind: "network.cdn",
		SKUID: egressRow.SKUID, Provider: egressRow.Provider, Service: egressRow.Service,
		Resource: egressRow.ResourceName, Region: egressRow.Region,
		Quantity: gb, QuantityUnit: "gb",
		MonthlyUSD: cost + baseFee,
	}, nil
}

func init() {
	Register(networkCDNTier0Estimator{})
}
