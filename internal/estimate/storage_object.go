package estimate

import (
	"context"
	"fmt"

	"github.com/sofq/sku/internal/catalog"
)

var lookupStorageObject = func(ctx context.Context, shard string, f catalog.StorageObjectFilter) ([]catalog.Row, error) {
	cat, err := catalog.Open(catalog.ShardPath(shard))
	if err != nil {
		return nil, err
	}
	defer func() { _ = cat.Close() }()
	return cat.LookupStorageObject(ctx, f)
}

var providerServiceShardStorage = map[string]string{
	"aws/s3":     "aws-s3",
	"azure/blob": "azure-blob",
	"gcp/gcs":    "gcp-gcs",
}

var (
	storagePutDims = map[string]bool{"requests-put": true, "write-ops": true}
	storageGetDims = map[string]bool{"requests-get": true, "read-ops": true}
)

type storageObjectEstimator struct{}

func (storageObjectEstimator) Kind() string { return "storage.object" }

func (storageObjectEstimator) Estimate(ctx context.Context, it Item) (LineItem, error) {
	region := it.Params["region"]
	if region == "" {
		return LineItem{}, fmt.Errorf("estimate/storage.object: %q missing :region=<name>", it.Raw)
	}
	gbMonth, err := paramFloatStorage(it.Params, "gb_month", 0, 0)
	if err != nil {
		return LineItem{}, err
	}
	putReqs, err := paramFloatStorage(it.Params, "put_requests", 0, 0)
	if err != nil {
		return LineItem{}, err
	}
	getReqs, err := paramFloatStorage(it.Params, "get_requests", 0, 0)
	if err != nil {
		return LineItem{}, err
	}

	shard, ok := providerServiceShardStorage[it.Provider+"/"+it.Service]
	if !ok {
		return LineItem{}, fmt.Errorf("estimate/storage.object: no shard mapped for %s/%s", it.Provider, it.Service)
	}

	rows, err := lookupStorageObject(ctx, shard, catalog.StorageObjectFilter{
		Provider: it.Provider, Service: it.Service,
		StorageClass: it.Resource, Region: region,
		Terms: catalog.Terms{Commitment: "on_demand"},
	})
	if err != nil {
		return LineItem{}, fmt.Errorf("estimate/storage.object: lookup: %w", err)
	}
	if len(rows) == 0 {
		return LineItem{}, fmt.Errorf("estimate/storage.object: no SKU for %s/%s:%s in %s", it.Provider, it.Service, it.Resource, region)
	}
	if len(rows) > 1 {
		return LineItem{}, fmt.Errorf("estimate/storage.object: ambiguous (%d rows) for %s/%s:%s in %s", len(rows), it.Provider, it.Service, it.Resource, region)
	}
	r := rows[0]

	var storagePrice, putPrice, getPrice float64
	var haveStorage, havePut, haveGet bool
	for _, p := range r.Prices {
		switch {
		case p.Dimension == "storage":
			storagePrice, haveStorage = p.Amount, true
		case storagePutDims[p.Dimension]:
			putPrice, havePut = p.Amount, true
		case storageGetDims[p.Dimension]:
			getPrice, haveGet = p.Amount, true
		}
	}
	if gbMonth > 0 && !haveStorage {
		return LineItem{}, fmt.Errorf("estimate/storage.object: no storage (gb-mo) price on SKU %s", r.SKUID)
	}
	if putReqs > 0 && !havePut {
		return LineItem{}, fmt.Errorf("estimate/storage.object: no put-requests price on SKU %s", r.SKUID)
	}
	if getReqs > 0 && !haveGet {
		return LineItem{}, fmt.Errorf("estimate/storage.object: no get-requests price on SKU %s", r.SKUID)
	}

	monthly := gbMonth*storagePrice + putReqs*putPrice + getReqs*getPrice
	return LineItem{
		Item: it, Kind: "storage.object", SKUID: r.SKUID,
		Provider: r.Provider, Service: r.Service, Resource: r.ResourceName,
		Region:   r.Region,
		Quantity: gbMonth, QuantityUnit: "gb-mo",
		MonthlyUSD: monthly,
	}, nil
}

func paramFloatStorage(p map[string]string, k string, def, minVal float64) (float64, error) {
	s, ok := p[k]
	if !ok {
		return def, nil
	}
	f, err := parseQuantity(s)
	if err != nil {
		return 0, fmt.Errorf("estimate/storage.object: %s=%q: %w", k, s, err)
	}
	if f < minVal {
		return 0, fmt.Errorf("estimate/storage.object: %s=%v below minimum %v", k, f, minVal)
	}
	return f, nil
}

func init() {
	Register(storageObjectEstimator{})
}
