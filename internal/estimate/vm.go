package estimate

import (
	"context"
	"fmt"
	"strconv"

	"github.com/sofq/sku/internal/catalog"
)

// lookupVM is a seam for tests to bypass the real catalog layer. Production
// code assigns it to a thin closure that opens the shard and calls LookupVM.
var lookupVM = func(ctx context.Context, shard string, f catalog.VMFilter) ([]catalog.Row, error) {
	cat, err := catalog.Open(catalog.ShardPath(shard))
	if err != nil {
		return nil, err
	}
	defer func() { _ = cat.Close() }()
	return cat.LookupVM(ctx, f)
}

// providerServiceShard maps the DSL provider/service to the shard basename.
var providerServiceShard = map[string]string{
	"aws/ec2":  "aws-ec2",
	"aws/rds":  "aws-rds",
	"azure/vm": "azure-vm",
	"gcp/gce":  "gcp-gce",
}

// hourlyUnits matches the on-demand compute-dimension price row. Pipeline
// normalization emits "hrs"; earlier seeds and unit tests use "hour".
var hourlyUnits = map[string]bool{"hour": true, "hrs": true, "hr": true}

type vmEstimator struct{}

func (vmEstimator) Kind() string { return "compute.vm" }

func (vmEstimator) Estimate(ctx context.Context, it Item) (LineItem, error) {
	region := it.Params["region"]
	if region == "" {
		return LineItem{}, fmt.Errorf("estimate/compute.vm: %q missing :region=<name>", it.Raw)
	}
	count, err := paramInt(it.Params, "count", 1, 1)
	if err != nil {
		return LineItem{}, err
	}
	hours, err := paramFloat(it.Params, "hours", 730, 0)
	if err != nil {
		return LineItem{}, err
	}
	osName := param(it.Params, "os", "linux")
	tenancy := param(it.Params, "tenancy", "shared")
	commitment := param(it.Params, "commitment", "on_demand")

	shard, ok := providerServiceShard[it.Provider+"/"+it.Service]
	if !ok {
		return LineItem{}, fmt.Errorf("estimate/compute.vm: no shard mapped for %s/%s", it.Provider, it.Service)
	}

	rows, err := lookupVM(ctx, shard, catalog.VMFilter{
		Provider: it.Provider, Service: it.Service,
		InstanceType: it.Resource, Region: region,
		Terms: catalog.Terms{Commitment: commitment, Tenancy: tenancy, OS: osName},
	})
	if err != nil {
		return LineItem{}, fmt.Errorf("estimate/compute.vm: lookup: %w", err)
	}
	if len(rows) == 0 {
		return LineItem{}, fmt.Errorf("estimate/compute.vm: no SKU for %s/%s:%s in %s", it.Provider, it.Service, it.Resource, region)
	}
	if len(rows) > 1 {
		return LineItem{}, fmt.Errorf("estimate/compute.vm: ambiguous (%d rows) for %s/%s:%s in %s", len(rows), it.Provider, it.Service, it.Resource, region)
	}
	r := rows[0]
	var hourly float64
	var unit string
	for _, p := range r.Prices {
		if p.Dimension == "compute" && hourlyUnits[p.Unit] {
			hourly = p.Amount
			unit = p.Unit
			break
		}
	}
	if hourly == 0 {
		return LineItem{}, fmt.Errorf("estimate/compute.vm: no hourly compute price on SKU %s", r.SKUID)
	}
	qty := hours * float64(count)
	return LineItem{
		Item: it, Kind: "compute.vm", SKUID: r.SKUID,
		Provider: r.Provider, Service: r.Service, Resource: r.ResourceName,
		Region:    r.Region,
		HourlyUSD: hourly, Quantity: qty, QuantityUnit: unit,
		MonthlyUSD: hourly * qty,
	}, nil
}

func param(p map[string]string, k, def string) string {
	if v, ok := p[k]; ok && v != "" {
		return v
	}
	return def
}

func paramInt(p map[string]string, k string, def, minVal int) (int, error) {
	s, ok := p[k]
	if !ok {
		return def, nil
	}
	n, err := strconv.Atoi(s)
	if err != nil {
		return 0, fmt.Errorf("estimate/compute.vm: %s=%q: %w", k, s, err)
	}
	if n < minVal {
		return 0, fmt.Errorf("estimate/compute.vm: %s=%d below minimum %d", k, n, minVal)
	}
	return n, nil
}

func paramFloat(p map[string]string, k string, def, minVal float64) (float64, error) {
	s, ok := p[k]
	if !ok {
		return def, nil
	}
	f, err := parseQuantity(s)
	if err != nil {
		return 0, fmt.Errorf("estimate/compute.vm: %s=%q: %w", k, s, err)
	}
	if f < minVal {
		return 0, fmt.Errorf("estimate/compute.vm: %s=%v below minimum %v", k, f, minVal)
	}
	return f, nil
}

func init() {
	Register(vmEstimator{})
}
