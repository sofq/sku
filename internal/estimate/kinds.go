package estimate

// providerServiceKind maps "provider/service" to the kind string the
// estimator registry dispatches on. New entries land with each new
// estimator kind (storage.object, llm.text, ...).
var providerServiceKind = map[string]string{
	"aws/ec2":        "compute.vm",
	"aws/rds":        "compute.vm",
	"azure/vm":       "compute.vm",
	"gcp/gce":        "compute.vm",
	"aws/s3":         "storage.object",
	"azure/blob":     "storage.object",
	"gcp/gcs":        "storage.object",
	"llm/text":       "llm.text",
	"aws/aurora":        "db.relational.aurora",
	"azure/cosmosdb":    "db.nosql.cosmos",
	"gcp/spanner":       "db.relational.spanner",
	"aws/opensearch":    "search.engine.opensearch",
	"azure/appservice":  "paas.app.appservice",
	"gcp/bigquery":      "warehouse.query.bigquery",
}
