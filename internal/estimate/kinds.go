package estimate

// providerServiceKind maps "provider/service" to the kind string the
// estimator registry dispatches on. New entries land with each new
// estimator kind (storage.object, llm.text, ...).
var providerServiceKind = map[string]string{
	"aws/ec2":          "compute.vm",
	"aws/rds":          "compute.vm",
	"azure/vm":         "compute.vm",
	"gcp/gce":          "compute.vm",
	"aws/s3":           "storage.object",
	"azure/blob":       "storage.object",
	"gcp/gcs":          "storage.object",
	"llm/text":         "llm.text",
	"aws/aurora":       "db.relational.aurora",
	"azure/cosmosdb":   "db.nosql.cosmos",
	"gcp/spanner":      "db.relational.spanner",
	"aws/opensearch":   "search.engine.opensearch",
	"azure/appservice": "paas.app.appservice",
	"gcp/bigquery":     "warehouse.query.bigquery",
	// M-δ messaging.queue shards
	"aws/sqs":                  "messaging.queue",
	"azure/service-bus-queues": "messaging.queue",
	"azure/event-hubs":         "messaging.queue",
	"gcp/pubsub-queues":        "messaging.queue",
	// M-δ messaging.topic shards
	"aws/sns":                  "messaging.topic",
	"azure/service-bus-topics": "messaging.topic",
	"gcp/pubsub-topics":        "messaging.topic",
	// M-δ dns.zone shards
	"aws/route53":   "dns.zone",
	"gcp/cloud-dns": "dns.zone",
	// M-δ api.gateway shards
	"aws/api-gateway": "api.gateway",
	"azure/apim":      "api.gateway",
	// M-δ network.cdn shards
	"aws/cloudfront":   "network.cdn",
	"azure/front-door": "network.cdn",
	"gcp/cloud-cdn":    "network.cdn",
	// M-δ db.nosql (Firestore joins existing Cosmos)
	"gcp/firestore": "db.nosql",
}
