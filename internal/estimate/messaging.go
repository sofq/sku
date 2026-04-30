package estimate

import (
	"context"
	"fmt"
	"math"
	"strconv"

	"github.com/sofq/sku/internal/catalog"
)

var lookupMessagingQueue = func(ctx context.Context, shard string, f catalog.MessagingQueueFilter) ([]catalog.Row, error) {
	cat, err := catalog.Open(catalog.ShardPath(shard))
	if err != nil {
		return nil, err
	}
	defer func() { _ = cat.Close() }()
	return cat.LookupMessagingQueue(ctx, f)
}

var providerServiceShardMessagingQueue = map[string]string{
	"aws/sqs":                  "aws-sqs",
	"azure/service-bus-queues": "azure-service-bus-queues",
	"azure/event-hubs":         "azure-event-hubs",
	"gcp/pubsub-queues":        "gcp-pubsub-queues",
}

// parseTierBoundCount parses a tier token as a count value.
// Accepts canonical tokens (e.g. "100M") and raw numeric strings.
func parseTierBoundCount(token string) (float64, error) {
	if token == "" {
		return math.MaxFloat64, nil
	}
	if v, ok := TierTokensCount[token]; ok {
		return v, nil
	}
	v, err := strconv.ParseFloat(token, 64)
	if err != nil {
		return 0, fmt.Errorf("parseTierBoundCount: cannot parse %q", token)
	}
	return v, nil
}

// parseTierBoundBytes parses a tier token as a bytes value.
// Accepts canonical tokens (e.g. "10TB") and raw numeric strings.
func parseTierBoundBytes(token string) (float64, error) {
	if token == "" {
		return math.MaxFloat64, nil
	}
	if v, ok := TierTokensBytes[token]; ok {
		return v, nil
	}
	v, err := strconv.ParseFloat(token, 64)
	if err != nil {
		return 0, fmt.Errorf("parseTierBoundBytes: cannot parse %q", token)
	}
	return v, nil
}

// pricesToTierEntriesCount converts prices for one dimension into TierEntry
// slices using the count-domain parser.
func pricesToTierEntriesCount(prices []catalog.Price, dim string) ([]TierEntry, error) {
	var entries []TierEntry
	for _, p := range prices {
		if p.Dimension != dim {
			continue
		}
		lower, err := parseTierBoundCount(p.Tier)
		if err != nil {
			return nil, err
		}
		upper, err := parseTierBoundCount(p.TierUpper)
		if err != nil {
			return nil, err
		}
		entries = append(entries, TierEntry{Lower: lower, Upper: upper, Amount: p.Amount})
	}
	return entries, nil
}

// flatDimPrice returns the amount for the first price entry with the given dimension and tier=="0".
func flatDimPrice(prices []catalog.Price, dim string) (float64, bool) {
	for _, p := range prices {
		if p.Dimension == dim && p.Tier == "0" {
			return p.Amount, true
		}
	}
	return 0, false
}

// messagingQueueEstimator handles messaging.queue kind estimation.
type messagingQueueEstimator struct{}

func (messagingQueueEstimator) Kind() string { return "messaging.queue" }

func (messagingQueueEstimator) Estimate(ctx context.Context, it Item) (LineItem, error) {
	region := it.Params["region"]
	if region == "" {
		return LineItem{}, fmt.Errorf("estimate/messaging.queue: %q missing :region=<name>", it.Raw)
	}
	psKey := it.Provider + "/" + it.Service
	shard, ok := providerServiceShardMessagingQueue[psKey]
	if !ok {
		return LineItem{}, fmt.Errorf("estimate/messaging.queue: no shard for %s", psKey)
	}

	rows, err := lookupMessagingQueue(ctx, shard, catalog.MessagingQueueFilter{
		Provider:     it.Provider,
		Service:      it.Service,
		ResourceName: it.Resource,
		Region:       region,
		Terms:        catalog.Terms{Commitment: "on_demand"},
	})
	if err != nil {
		return LineItem{}, fmt.Errorf("estimate/messaging.queue: lookup: %w", err)
	}
	if len(rows) == 0 {
		return LineItem{}, fmt.Errorf("estimate/messaging.queue: no SKU for %s/%s:%s in %s",
			it.Provider, it.Service, it.Resource, region)
	}
	r := rows[0]

	switch {
	case it.Params["ops"] != "":
		ops, err := paramFloat(it.Params, "ops", 0, 0)
		if err != nil {
			return LineItem{}, err
		}
		for _, dim := range []string{"request", "call", "event"} {
			entries, err := pricesToTierEntriesCount(r.Prices, dim)
			if err != nil {
				return LineItem{}, fmt.Errorf("estimate/messaging.queue: tier parse: %w", err)
			}
			if len(entries) == 0 {
				continue
			}
			cost := WalkTiers(entries, ops)
			return LineItem{
				Item: it, Kind: "messaging.queue",
				SKUID: r.SKUID, Provider: r.Provider, Service: r.Service,
				Resource: r.ResourceName, Region: r.Region,
				Quantity: ops, QuantityUnit: dim,
				MonthlyUSD: cost,
			}, nil
		}
		return LineItem{}, fmt.Errorf("estimate/messaging.queue: no tiered request dimension for %s/%s:%s", it.Provider, it.Service, it.Resource)

	case it.Params["tib"] != "":
		// GCP Pub/Sub: throughput prices stored per-GiB; input is TiB/month.
		tib, err := paramFloat(it.Params, "tib", 0, 0)
		if err != nil {
			return LineItem{}, err
		}
		gib := tib * 1024.0
		entries, err := pricesToTierEntriesCount(r.Prices, "throughput")
		if err != nil {
			return LineItem{}, fmt.Errorf("estimate/messaging.queue: tier parse: %w", err)
		}
		cost := WalkTiers(entries, gib)
		return LineItem{
			Item: it, Kind: "messaging.queue",
			SKUID: r.SKUID, Provider: r.Provider, Service: r.Service,
			Resource: r.ResourceName, Region: r.Region,
			Quantity: tib, QuantityUnit: "tib-mo",
			MonthlyUSD: cost,
		}, nil

	case it.Params["tu_hours"] != "":
		tuHours, err := paramFloat(it.Params, "tu_hours", 0, 0)
		if err != nil {
			return LineItem{}, err
		}
		amount, found := flatDimPrice(r.Prices, "tu_hour")
		if !found {
			return LineItem{}, fmt.Errorf("estimate/messaging.queue: no tu_hour dimension for %s/%s:%s", it.Provider, it.Service, it.Resource)
		}
		return LineItem{
			Item: it, Kind: "messaging.queue",
			SKUID: r.SKUID, Provider: r.Provider, Service: r.Service,
			Resource: r.ResourceName, Region: r.Region,
			Quantity: tuHours, QuantityUnit: "tu-hr",
			HourlyUSD: amount, MonthlyUSD: amount * tuHours,
		}, nil

	case it.Params["ppu_hours"] != "":
		ppuHours, err := paramFloat(it.Params, "ppu_hours", 0, 0)
		if err != nil {
			return LineItem{}, err
		}
		amount, found := flatDimPrice(r.Prices, "ppu_hour")
		if !found {
			return LineItem{}, fmt.Errorf("estimate/messaging.queue: no ppu_hour dimension for %s/%s:%s", it.Provider, it.Service, it.Resource)
		}
		return LineItem{
			Item: it, Kind: "messaging.queue",
			SKUID: r.SKUID, Provider: r.Provider, Service: r.Service,
			Resource: r.ResourceName, Region: r.Region,
			Quantity: ppuHours, QuantityUnit: "ppu-hr",
			HourlyUSD: amount, MonthlyUSD: amount * ppuHours,
		}, nil

	case it.Params["mu_hours"] != "":
		muHours, err := paramFloat(it.Params, "mu_hours", 0, 0)
		if err != nil {
			return LineItem{}, err
		}
		amount, found := flatDimPrice(r.Prices, "mu_hour")
		if !found {
			return LineItem{}, fmt.Errorf("estimate/messaging.queue: no mu_hour dimension for %s/%s:%s", it.Provider, it.Service, it.Resource)
		}
		return LineItem{
			Item: it, Kind: "messaging.queue",
			SKUID: r.SKUID, Provider: r.Provider, Service: r.Service,
			Resource: r.ResourceName, Region: r.Region,
			Quantity: muHours, QuantityUnit: "mu-hr",
			HourlyUSD: amount, MonthlyUSD: amount * muHours,
		}, nil

	default:
		return LineItem{}, fmt.Errorf("estimate/messaging.queue: %q requires :ops=<n>, :tib=<n>, :tu_hours=<n>, :ppu_hours=<n>, or :mu_hours=<n>", it.Raw)
	}
}

// messagingTopicEstimator handles messaging.topic kind estimation.
// Full implementation deferred to M-ε.
type messagingTopicEstimator struct{}

func (messagingTopicEstimator) Kind() string { return "messaging.topic" }

func (messagingTopicEstimator) Estimate(_ context.Context, _ Item) (LineItem, error) {
	return LineItem{}, fmt.Errorf("estimator for messaging.topic deferred to M-ε")
}

func init() {
	Register(messagingQueueEstimator{})
	Register(messagingTopicEstimator{})
}
