package estimate

// providerServiceKind maps "provider/service" to the kind string the
// estimator registry dispatches on. New entries land with each new
// estimator kind (storage.object, llm.text, ...).
var providerServiceKind = map[string]string{
	"aws/ec2":  "compute.vm",
	"aws/rds":  "compute.vm",
	"azure/vm": "compute.vm",
	"gcp/gce":  "compute.vm",
}
