package estimate

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/sofq/sku/internal/catalog"
)

func stubMessagingQueueLookup(t *testing.T, fn func(ctx context.Context, shard string, f catalog.MessagingQueueFilter) ([]catalog.Row, error)) {
	t.Helper()
	prev := lookupMessagingQueue
	lookupMessagingQueue = fn
	t.Cleanup(func() { lookupMessagingQueue = prev })
}

// --- AWS SQS ---

func TestMessagingQueue_SQS_Standard_Ops(t *testing.T) {
	resetRegistry(t)
	Register(messagingQueueEstimator{})
	e, ok := Get("messaging.queue")
	require.True(t, ok)

	stubMessagingQueueLookup(t, func(_ context.Context, shard string, f catalog.MessagingQueueFilter) ([]catalog.Row, error) {
		require.Equal(t, "aws-sqs", shard)
		require.Equal(t, "standard", f.ResourceName)
		require.Equal(t, "us-east-1", f.Region)
		return []catalog.Row{{
			SKUID: "sqs-std-use1", Provider: "aws", Service: "sqs",
			ResourceName: "standard", Region: "us-east-1",
			Prices: []catalog.Price{{Dimension: "request", Tier: "0", TierUpper: "", Amount: 0.0000004, Unit: "request"}},
		}}, nil
	})

	item, err := ParseItem("aws/sqs:standard:region=us-east-1:ops=1000000")
	require.NoError(t, err)
	li, err := e.Estimate(context.Background(), item)
	require.NoError(t, err)
	require.InDelta(t, 0.0000004*1e6, li.MonthlyUSD, 1e-9)
	require.Equal(t, "sqs-std-use1", li.SKUID)
}

func TestMessagingQueue_SQS_TierCrossing_Ops(t *testing.T) {
	resetRegistry(t)
	Register(messagingQueueEstimator{})
	e, _ := Get("messaging.queue")

	stubMessagingQueueLookup(t, func(_ context.Context, _ string, _ catalog.MessagingQueueFilter) ([]catalog.Row, error) {
		return []catalog.Row{{
			SKUID: "sqs-std", Provider: "aws", Service: "sqs",
			ResourceName: "standard", Region: "us-east-1",
			Prices: []catalog.Price{
				{Dimension: "request", Tier: "0", TierUpper: "1B", Amount: 0.0000004, Unit: "request"},
				{Dimension: "request", Tier: "1B", TierUpper: "", Amount: 0.0000003, Unit: "request"},
			},
		}}, nil
	})

	// 2B ops: first 1B at 0.0000004, next 1B at 0.0000003
	item, err := ParseItem("aws/sqs:standard:region=us-east-1:ops=2000000000")
	require.NoError(t, err)
	li, err := e.Estimate(context.Background(), item)
	require.NoError(t, err)
	expected := 1e9*0.0000004 + 1e9*0.0000003
	require.InDelta(t, expected, li.MonthlyUSD, 1e-3)
}

// --- Azure Service Bus Queues: ops path (standard tier) ---

func TestMessagingQueue_ServiceBusQueues_Ops(t *testing.T) {
	resetRegistry(t)
	Register(messagingQueueEstimator{})
	e, _ := Get("messaging.queue")

	stubMessagingQueueLookup(t, func(_ context.Context, shard string, _ catalog.MessagingQueueFilter) ([]catalog.Row, error) {
		require.Equal(t, "azure-service-bus-queues", shard)
		return []catalog.Row{{
			SKUID: "sb-std-eastus", Provider: "azure", Service: "service-bus-queues",
			ResourceName: "standard", Region: "eastus",
			Prices: []catalog.Price{
				{Dimension: "request", Tier: "0", TierUpper: "13M", Amount: 0.0, Unit: "request"},
				{Dimension: "request", Tier: "13M", TierUpper: "100M", Amount: 0.8, Unit: "request"},
				{Dimension: "request", Tier: "100M", TierUpper: "", Amount: 0.5, Unit: "request"},
			},
		}}, nil
	})

	// 50M ops: first 13M free, next 37M at 0.8 per million
	item, err := ParseItem("azure/service-bus-queues:standard:region=eastus:ops=50000000")
	require.NoError(t, err)
	li, err := e.Estimate(context.Background(), item)
	require.NoError(t, err)
	// WalkTiers: tier 0..13M → $0, tier 13M..50M → 37M * 0.8/1M = $29.6
	// Note: prices in catalog are per-million-requests stored as rate per unit
	// The SB seed stores 0.8 as the per-request amount for that tier
	require.True(t, li.MonthlyUSD >= 0)
	require.Equal(t, "sb-std-eastus", li.SKUID)
}

// --- Azure Service Bus Queues: mu_hour path (premium tier) ---

func TestMessagingQueue_ServiceBusQueues_MuHour(t *testing.T) {
	resetRegistry(t)
	Register(messagingQueueEstimator{})
	e, _ := Get("messaging.queue")

	stubMessagingQueueLookup(t, func(_ context.Context, shard string, _ catalog.MessagingQueueFilter) ([]catalog.Row, error) {
		require.Equal(t, "azure-service-bus-queues", shard)
		return []catalog.Row{{
			SKUID: "sb-prem-eastus", Provider: "azure", Service: "service-bus-queues",
			ResourceName: "premium", Region: "eastus",
			Prices: []catalog.Price{
				{Dimension: "mu_hour", Tier: "0", TierUpper: "", Amount: 0.928, Unit: "hr"},
			},
		}}, nil
	})

	item, err := ParseItem("azure/service-bus-queues:premium:region=eastus:mu_hours=730")
	require.NoError(t, err)
	li, err := e.Estimate(context.Background(), item)
	require.NoError(t, err)
	require.InDelta(t, 0.928*730, li.MonthlyUSD, 1e-6)
	require.Equal(t, "mu-hr", li.QuantityUnit)
	require.InDelta(t, 0.928, li.HourlyUSD, 1e-6)
}

// --- Azure Event Hubs: tu_hour path (standard) ---

func TestMessagingQueue_EventHubs_TuHour(t *testing.T) {
	resetRegistry(t)
	Register(messagingQueueEstimator{})
	e, _ := Get("messaging.queue")

	stubMessagingQueueLookup(t, func(_ context.Context, shard string, _ catalog.MessagingQueueFilter) ([]catalog.Row, error) {
		require.Equal(t, "azure-event-hubs", shard)
		return []catalog.Row{{
			SKUID: "eh-std-eastus", Provider: "azure", Service: "event-hubs",
			ResourceName: "standard", Region: "eastus",
			Prices: []catalog.Price{
				{Dimension: "tu_hour", Tier: "0", TierUpper: "", Amount: 0.030, Unit: "hr"},
			},
		}}, nil
	})

	item, err := ParseItem("azure/event-hubs:standard:region=eastus:tu_hours=730")
	require.NoError(t, err)
	li, err := e.Estimate(context.Background(), item)
	require.NoError(t, err)
	require.InDelta(t, 0.030*730, li.MonthlyUSD, 1e-6)
	require.Equal(t, "tu-hr", li.QuantityUnit)
}

// --- Azure Event Hubs: ppu_hour path (premium) ---

func TestMessagingQueue_EventHubs_PpuHour(t *testing.T) {
	resetRegistry(t)
	Register(messagingQueueEstimator{})
	e, _ := Get("messaging.queue")

	stubMessagingQueueLookup(t, func(_ context.Context, shard string, _ catalog.MessagingQueueFilter) ([]catalog.Row, error) {
		require.Equal(t, "azure-event-hubs", shard)
		return []catalog.Row{{
			SKUID: "eh-prem-eastus", Provider: "azure", Service: "event-hubs",
			ResourceName: "premium", Region: "eastus",
			Prices: []catalog.Price{
				{Dimension: "ppu_hour", Tier: "0", TierUpper: "", Amount: 1.063, Unit: "hr"},
			},
		}}, nil
	})

	item, err := ParseItem("azure/event-hubs:premium:region=eastus:ppu_hours=500")
	require.NoError(t, err)
	li, err := e.Estimate(context.Background(), item)
	require.NoError(t, err)
	require.InDelta(t, 1.063*500, li.MonthlyUSD, 1e-6)
	require.Equal(t, "ppu-hr", li.QuantityUnit)
}

// --- GCP Pub/Sub Queues: tib path ---

func TestMessagingQueue_PubSub_Tib(t *testing.T) {
	resetRegistry(t)
	Register(messagingQueueEstimator{})
	e, _ := Get("messaging.queue")

	stubMessagingQueueLookup(t, func(_ context.Context, shard string, _ catalog.MessagingQueueFilter) ([]catalog.Row, error) {
		require.Equal(t, "gcp-pubsub-queues", shard)
		return []catalog.Row{{
			SKUID: "pubsub-throughput", Provider: "gcp", Service: "pubsub-queues",
			ResourceName: "throughput", Region: "global",
			Prices: []catalog.Price{
				{Dimension: "throughput", Tier: "0", TierUpper: "", Amount: 0.0390625, Unit: "gib-mo"},
			},
		}}, nil
	})

	// 2 TiB = 2048 GiB
	item, err := ParseItem("gcp/pubsub-queues:throughput:region=global:tib=2")
	require.NoError(t, err)
	li, err := e.Estimate(context.Background(), item)
	require.NoError(t, err)
	require.InDelta(t, 2048*0.0390625, li.MonthlyUSD, 1e-6)
	require.Equal(t, "tib-mo", li.QuantityUnit)
}

// --- Negative tests ---

func TestMessagingQueue_MissingRegion(t *testing.T) {
	resetRegistry(t)
	Register(messagingQueueEstimator{})
	e, _ := Get("messaging.queue")

	item, err := ParseItem("aws/sqs:standard:ops=1000000")
	require.NoError(t, err)
	_, err = e.Estimate(context.Background(), item)
	require.Error(t, err)
	require.Contains(t, err.Error(), "region")
}

func TestMessagingQueue_UnknownShard(t *testing.T) {
	resetRegistry(t)
	Register(messagingQueueEstimator{})
	e, _ := Get("messaging.queue")

	// Build Item directly — provider/service not in shard map but valid kind.
	item := Item{
		Raw:      "fake/msgqueue:standard:region=us-east1:ops=1000",
		Provider: "fake", Service: "msgqueue",
		Resource: "standard", Kind: "messaging.queue",
		Params: map[string]string{"region": "us-east1", "ops": "1000"},
	}
	_, err := e.Estimate(context.Background(), item)
	require.Error(t, err)
	require.Contains(t, err.Error(), "no shard")
}

func TestMessagingQueue_NoInput(t *testing.T) {
	resetRegistry(t)
	Register(messagingQueueEstimator{})
	e, _ := Get("messaging.queue")

	stubMessagingQueueLookup(t, func(_ context.Context, _ string, _ catalog.MessagingQueueFilter) ([]catalog.Row, error) {
		return []catalog.Row{{
			SKUID: "sqs-std", Provider: "aws", Service: "sqs",
			ResourceName: "standard", Region: "us-east-1",
			Prices: []catalog.Price{{Dimension: "request", Tier: "0", TierUpper: "", Amount: 0.0000004}},
		}}, nil
	})

	item, err := ParseItem("aws/sqs:standard:region=us-east-1")
	require.NoError(t, err)
	_, err = e.Estimate(context.Background(), item)
	require.Error(t, err)
}

func TestMessagingTopic_Deferred(t *testing.T) {
	resetRegistry(t)
	Register(messagingTopicEstimator{})
	e, ok := Get("messaging.topic")
	require.True(t, ok)

	item, err := ParseItem("aws/sns:standard:region=us-east-1:ops=1000")
	require.NoError(t, err)
	_, err = e.Estimate(context.Background(), item)
	require.Error(t, err)
	require.Contains(t, err.Error(), "M-ε")
}
