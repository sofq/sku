package estimate

import (
	"context"
	"strings"
	"testing"
)

type fakeEstimator struct{ kind string }

func (f fakeEstimator) Kind() string { return f.kind }
func (f fakeEstimator) Estimate(_ context.Context, it Item) (LineItem, error) {
	return LineItem{Item: it, Kind: f.kind, HourlyUSD: 1, Quantity: 1, MonthlyUSD: 1}, nil
}

func TestRegistry_registerAndGet(t *testing.T) {
	resetRegistry(t)
	Register(fakeEstimator{kind: "compute.vm"})
	e, ok := Get("compute.vm")
	if !ok {
		t.Fatal("estimator not registered")
	}
	if e.Kind() != "compute.vm" {
		t.Fatalf("wrong kind: %q", e.Kind())
	}
}

func TestRegistry_missing(t *testing.T) {
	resetRegistry(t)
	if _, ok := Get("storage.object"); ok {
		t.Fatal("expected miss for unregistered kind")
	}
}

// TestMDeltaKindMap verifies that all 15 new M-δ shard keys are present in
// the providerServiceKind map. It does NOT require a registered estimator
// (db.nosql has no stub; it just needs to be in the map).
func TestMDeltaKindMap(t *testing.T) {
	newShards := []struct {
		key      string
		wantKind string
	}{
		// messaging.queue
		{"aws/sqs", "messaging.queue"},
		{"azure/service-bus-queues", "messaging.queue"},
		{"azure/event-hubs", "messaging.queue"},
		{"gcp/pubsub-queues", "messaging.queue"},
		// messaging.topic
		{"aws/sns", "messaging.topic"},
		{"azure/service-bus-topics", "messaging.topic"},
		{"gcp/pubsub-topics", "messaging.topic"},
		// dns.zone
		{"aws/route53", "dns.zone"},
		{"gcp/cloud-dns", "dns.zone"},
		// api.gateway
		{"aws/api-gateway", "api.gateway"},
		{"azure/apim", "api.gateway"},
		// network.cdn
		{"aws/cloudfront", "network.cdn"},
		{"azure/front-door", "network.cdn"},
		{"gcp/cloud-cdn", "network.cdn"},
		// db.nosql (Firestore)
		{"gcp/firestore", "db.nosql"},
	}
	for _, tc := range newShards {
		got, ok := providerServiceKind[tc.key]
		if !ok {
			t.Errorf("providerServiceKind[%q]: missing", tc.key)
			continue
		}
		if got != tc.wantKind {
			t.Errorf("providerServiceKind[%q] = %q, want %q", tc.key, got, tc.wantKind)
		}
	}
}

// TestMDeltaStubEstimators verifies that the 4 Phase-4 stub kinds are
// registered and return the expected placeholder error. Uses the real
// init()-registered stubs — do NOT call resetRegistry here.
func TestMDeltaStubEstimators(t *testing.T) {
	stubKinds := []struct {
		kind        string
		wantErrFrag string
	}{
		{"messaging.queue", "Phase 4"},
		{"dns.zone", "Phase 4"},
		{"api.gateway", "Phase 4"},
		{"network.cdn", "Phase 4"},
	}
	ctx := context.Background()
	for _, tc := range stubKinds {
		e, ok := Get(tc.kind)
		if !ok {
			t.Errorf("Get(%q): not registered", tc.kind)
			continue
		}
		_, err := e.Estimate(ctx, Item{Raw: "test", Kind: tc.kind})
		if err == nil {
			t.Errorf("kind %q: expected error, got nil", tc.kind)
			continue
		}
		if !strings.Contains(err.Error(), tc.wantErrFrag) {
			t.Errorf("kind %q: error %q does not contain %q", tc.kind, err.Error(), tc.wantErrFrag)
		}
	}
}

// TestMDeltaMessagingTopicDeferred verifies that messaging.topic shards
// in the kind map resolve and that the stub returns the M-ε deferral error.
func TestMDeltaMessagingTopicDeferred(t *testing.T) {
	topicShards := []string{"aws/sns", "azure/service-bus-topics", "gcp/pubsub-topics"}
	for _, shard := range topicShards {
		got, ok := providerServiceKind[shard]
		if !ok {
			t.Errorf("providerServiceKind[%q]: missing", shard)
			continue
		}
		if got != "messaging.topic" {
			t.Errorf("providerServiceKind[%q] = %q, want %q", shard, got, "messaging.topic")
		}
	}

	e, ok := Get("messaging.topic")
	if !ok {
		t.Fatal("messaging.topic estimator not registered")
	}
	ctx := context.Background()
	_, err := e.Estimate(ctx, Item{Raw: "test", Kind: "messaging.topic"})
	if err == nil {
		t.Fatal("messaging.topic: expected error, got nil")
	}
	if !strings.Contains(err.Error(), "M-ε") {
		t.Errorf("messaging.topic error %q does not contain %q", err.Error(), "M-ε")
	}
}
