package catalog_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/sofq/sku/internal/catalog"
)

// ---------------------------------------------------------------------------
// MessagingQueue
// ---------------------------------------------------------------------------

func openSeededMessagingQueue(t *testing.T) *catalog.Catalog {
	t.Helper()
	cat, err := catalog.Open(seedShardFromFile(t, "seed_messaging_queue.sql", "aws-sqs.db"))
	require.NoError(t, err)
	t.Cleanup(func() { _ = cat.Close() })
	return cat
}

func TestLookupMessagingQueue_ByResourceNameAndRegion(t *testing.T) {
	cat := openSeededMessagingQueue(t)
	rows, err := cat.LookupMessagingQueue(context.Background(), catalog.MessagingQueueFilter{
		Provider:     "aws",
		Service:      "sqs",
		ResourceName: "standard",
		Region:       "us-east-1",
		Terms:        catalog.Terms{Commitment: "on_demand"},
	})
	require.NoError(t, err)
	require.Len(t, rows, 1)
	require.Equal(t, "standard", rows[0].ResourceName)
	require.Equal(t, "messaging.queue", rows[0].Kind)
}

func TestLookupMessagingQueue_ResourceNameNarrows(t *testing.T) {
	cat := openSeededMessagingQueue(t)
	// seed has standard + fifo; requesting fifo should return only 1 row
	rows, err := cat.LookupMessagingQueue(context.Background(), catalog.MessagingQueueFilter{
		Provider:     "aws",
		Service:      "sqs",
		ResourceName: "fifo",
		Terms:        catalog.Terms{Commitment: "on_demand"},
	})
	require.NoError(t, err)
	require.Len(t, rows, 1)
	require.Equal(t, "fifo", rows[0].ResourceName)
}

func TestLookupMessagingQueue_MissingResourceNameErrors(t *testing.T) {
	cat := openSeededMessagingQueue(t)
	_, err := cat.LookupMessagingQueue(context.Background(), catalog.MessagingQueueFilter{
		Provider: "aws",
		Service:  "sqs",
	})
	require.Error(t, err)
}

// ---------------------------------------------------------------------------
// MessagingTopic
// ---------------------------------------------------------------------------

func openSeededMessagingTopic(t *testing.T) *catalog.Catalog {
	t.Helper()
	cat, err := catalog.Open(seedShardFromFile(t, "seed_messaging_topic.sql", "aws-sns.db"))
	require.NoError(t, err)
	t.Cleanup(func() { _ = cat.Close() })
	return cat
}

func TestLookupMessagingTopic_ByResourceNameAndRegion(t *testing.T) {
	cat := openSeededMessagingTopic(t)
	rows, err := cat.LookupMessagingTopic(context.Background(), catalog.MessagingTopicFilter{
		Provider:     "aws",
		Service:      "sns",
		ResourceName: "standard",
		Region:       "us-east-1",
		Terms:        catalog.Terms{Commitment: "on_demand"},
	})
	require.NoError(t, err)
	require.Len(t, rows, 1)
	require.Equal(t, "standard", rows[0].ResourceName)
	require.Equal(t, "messaging.topic", rows[0].Kind)
}

func TestLookupMessagingTopic_RegionNarrows(t *testing.T) {
	cat := openSeededMessagingTopic(t)
	// seed has us-east-1 + eu-west-1; no region filter returns both
	rows, err := cat.LookupMessagingTopic(context.Background(), catalog.MessagingTopicFilter{
		Provider:     "aws",
		Service:      "sns",
		ResourceName: "standard",
		Terms:        catalog.Terms{Commitment: "on_demand"},
	})
	require.NoError(t, err)
	require.Len(t, rows, 2, "both regions returned when region unset")
}

func TestLookupMessagingTopic_MissingResourceNameErrors(t *testing.T) {
	cat := openSeededMessagingTopic(t)
	_, err := cat.LookupMessagingTopic(context.Background(), catalog.MessagingTopicFilter{
		Provider: "aws",
		Service:  "sns",
	})
	require.Error(t, err)
}

// ---------------------------------------------------------------------------
// DNSZone
// ---------------------------------------------------------------------------

func openSeededDNSZone(t *testing.T) *catalog.Catalog {
	t.Helper()
	cat, err := catalog.Open(seedShardFromFile(t, "seed_dns_zone.sql", "aws-route53.db"))
	require.NoError(t, err)
	t.Cleanup(func() { _ = cat.Close() })
	return cat
}

func TestLookupDNSZone_ByResourceName(t *testing.T) {
	cat := openSeededDNSZone(t)
	rows, err := cat.LookupDNSZone(context.Background(), catalog.DNSZoneFilter{
		Provider:     "aws",
		Service:      "route53",
		ResourceName: "public",
		Region:       "global",
		Terms:        catalog.Terms{Commitment: "on_demand"},
	})
	require.NoError(t, err)
	require.Len(t, rows, 1)
	require.Equal(t, "public", rows[0].ResourceName)
	require.Equal(t, "dns.zone", rows[0].Kind)
}

func TestLookupDNSZone_ResourceNameNarrows(t *testing.T) {
	cat := openSeededDNSZone(t)
	// seed has public + private; requesting private should return only 1 row
	rows, err := cat.LookupDNSZone(context.Background(), catalog.DNSZoneFilter{
		Provider:     "aws",
		Service:      "route53",
		ResourceName: "private",
		Terms:        catalog.Terms{Commitment: "on_demand"},
	})
	require.NoError(t, err)
	require.Len(t, rows, 1)
	require.Equal(t, "private", rows[0].ResourceName)
}

func TestLookupDNSZone_MissingResourceNameErrors(t *testing.T) {
	cat := openSeededDNSZone(t)
	_, err := cat.LookupDNSZone(context.Background(), catalog.DNSZoneFilter{
		Provider: "aws",
		Service:  "route53",
	})
	require.Error(t, err)
}

// ---------------------------------------------------------------------------
// APIGateway
// ---------------------------------------------------------------------------

func openSeededAPIGateway(t *testing.T) *catalog.Catalog {
	t.Helper()
	cat, err := catalog.Open(seedShardFromFile(t, "seed_api_gateway.sql", "aws-apigateway.db"))
	require.NoError(t, err)
	t.Cleanup(func() { _ = cat.Close() })
	return cat
}

func TestLookupAPIGateway_ByResourceNameAndRegion(t *testing.T) {
	cat := openSeededAPIGateway(t)
	rows, err := cat.LookupAPIGateway(context.Background(), catalog.APIGatewayFilter{
		Provider:     "aws",
		Service:      "apigateway",
		ResourceName: "rest",
		Region:       "us-east-1",
		Terms:        catalog.Terms{Commitment: "on_demand"},
	})
	require.NoError(t, err)
	require.Len(t, rows, 1)
	require.Equal(t, "rest", rows[0].ResourceName)
	require.Equal(t, "api.gateway", rows[0].Kind)
}

func TestLookupAPIGateway_ResourceNameNarrows(t *testing.T) {
	cat := openSeededAPIGateway(t)
	// seed has rest + http; requesting http should return only 1 row
	rows, err := cat.LookupAPIGateway(context.Background(), catalog.APIGatewayFilter{
		Provider:     "aws",
		Service:      "apigateway",
		ResourceName: "http",
		Terms:        catalog.Terms{Commitment: "on_demand"},
	})
	require.NoError(t, err)
	require.Len(t, rows, 1)
	require.Equal(t, "http", rows[0].ResourceName)
}

func TestLookupAPIGateway_MissingResourceNameErrors(t *testing.T) {
	cat := openSeededAPIGateway(t)
	_, err := cat.LookupAPIGateway(context.Background(), catalog.APIGatewayFilter{
		Provider: "aws",
		Service:  "apigateway",
	})
	require.Error(t, err)
}

// ---------------------------------------------------------------------------
// CDNFilter extended fields (Mode + Sku)
// ---------------------------------------------------------------------------

func TestCDNFilter_ModeAndSkuFieldsAccessible(t *testing.T) {
	// Compile-time assertion: Mode and Sku fields exist on CDNFilter.
	// The compare handler (Task 0.5) will populate these for post-filtering;
	// LookupCDN itself ignores them. This test confirms the fields are
	// structurally present and zero-valued by default.
	f := catalog.CDNFilter{}
	require.Equal(t, "", f.Mode)
	require.Equal(t, "", f.Sku)

	f.Mode = "regional"
	f.Sku = "PriceClass_All"
	require.Equal(t, "regional", f.Mode)
	require.Equal(t, "PriceClass_All", f.Sku)
}

// ---------------------------------------------------------------------------
// NoSQLDBFilter extended field (Engine) — narrowing via Terms.Tenancy
// ---------------------------------------------------------------------------

func openSeededNoSQLEngine(t *testing.T) *catalog.Catalog {
	t.Helper()
	cat, err := catalog.Open(seedShardFromFile(t, "seed_nosql_engine.sql", "nosql-engine.db"))
	require.NoError(t, err)
	t.Cleanup(func() { _ = cat.Close() })
	return cat
}

func TestNoSQLDBFilter_EngineFieldAccessible(t *testing.T) {
	// Compile-time assertion: Engine and Mode fields exist on NoSQLDBFilter.
	f := catalog.NoSQLDBFilter{}
	require.Equal(t, "", f.Engine)
	require.Equal(t, "", f.Mode)

	f.Engine = "dynamodb"
	f.Mode = "provisioned"
	require.Equal(t, "dynamodb", f.Engine)
	require.Equal(t, "provisioned", f.Mode)
}

func TestLookupNoSQLDB_EngineNarrowingViaTermsTenancy(t *testing.T) {
	// Seed has two rows: one with tenancy="dynamodb", one with tenancy="cosmos-sql".
	// Passing Terms.Tenancy="dynamodb" causes LookupNoSQLDB to narrow via
	// terms_hash and return only the dynamodb row.
	// Engine field on the filter is set to "dynamodb" to document the intent
	// (compare handlers will use it as a post-filter in Task 0.5).
	cat := openSeededNoSQLEngine(t)
	rows, err := cat.LookupNoSQLDB(context.Background(), catalog.NoSQLDBFilter{
		ResourceName: "standard",
		Engine:       "dynamodb",
		Terms:        catalog.Terms{Commitment: "on_demand", Tenancy: "dynamodb"},
	})
	require.NoError(t, err)
	require.Len(t, rows, 1, "only dynamodb tenancy row should match")
	require.Equal(t, "dynamodb", rows[0].Terms.Tenancy)
	require.Equal(t, "aws", rows[0].Provider)
}
