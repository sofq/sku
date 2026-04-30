package schema

// TermSlotOverride describes one repurposed slot in the shared `terms` row
// for a particular kind. The catalog's terms columns (commitment, tenancy,
// os, support_tier, upfront, payment_option) are fixed across every kind so
// that terms_hash remains a stable per-row identity. For db-shaped kinds
// (db.relational, cache.kv, container.orchestration) some slots carry
// kind-specific semantics rather than their literal column names; this
// struct tells agents how to read those slots.
//
// SemanticName is what the value actually represents (e.g. "engine"). CLIFlag
// is the price/list flag that filters on the slot for that kind. Values is
// the enumeration the validator accepts in pipeline/normalize/enums.yaml.
type TermSlotOverride struct {
	SemanticName string   `json:"semantic_name"`
	CLIFlag      string   `json:"cli_flag,omitempty"`
	Description  string   `json:"description,omitempty"`
	Values       []string `json:"values,omitempty"`
}

// KindTermOverrides lists the slots a single kind repurposes. A nil pointer
// means the slot carries its literal column meaning (or is empty) for that
// kind.
type KindTermOverrides struct {
	Tenancy *TermSlotOverride `json:"tenancy,omitempty"`
	OS      *TermSlotOverride `json:"os,omitempty"`
}

// kindTermOverrides is the canonical mapping. Kinds absent from this map use
// the literal `terms` column semantics (compute.vm: shared/dedicated tenancy
// + linux/windows os; storage.* and llm.* leave both empty). Adding a new
// kind here MUST stay consistent with the ingest pipelines and the
// pipeline/normalize/enums.yaml allowlists.
var kindTermOverrides = map[string]KindTermOverrides{
	"db.relational": {
		Tenancy: &TermSlotOverride{
			SemanticName: "engine",
			CLIFlag:      "--engine",
			Description:  "Database engine (participates in terms_hash).",
			Values: []string{
				"postgres", "mysql", "mariadb", "oracle", "sqlserver",
				"aurora-postgres", "aurora-mysql",
				"azure-sql", "azure-postgres", "azure-mysql", "azure-mariadb",
				"cloud-sql-postgres", "cloud-sql-mysql", "cloud-sql-sqlserver",
			},
		},
		OS: &TermSlotOverride{
			SemanticName: "deployment_option",
			CLIFlag:      "--deployment-option",
			Description:  "Deployment topology (participates in terms_hash).",
			Values: []string{
				"single-az", "multi-az", "multi-az-cluster",
				"managed-instance", "elastic-pool", "flexible-server",
				"zonal", "regional",
			},
		},
	},
	"cache.kv": {
		Tenancy: &TermSlotOverride{
			SemanticName: "engine",
			CLIFlag:      "--engine",
			Description:  "Cache engine (participates in terms_hash).",
			Values: []string{
				"redis", "memcached",
				"sql", "mongo", "cassandra", "table", "gremlin",
				"spanner-standard", "spanner-enterprise", "spanner-enterprise-plus", "spanner-storage",
			},
		},
		OS: &TermSlotOverride{
			SemanticName: "tier_or_mode",
			CLIFlag:      "--tier or --capacity-mode",
			Description:  "Cache tier or capacity mode (participates in terms_hash).",
			Values: []string{
				"basic", "standard", "premium", "enterprise",
				"provisioned", "serverless",
				"storage",
			},
		},
	},
	"container.orchestration": {
		Tenancy: &TermSlotOverride{
			SemanticName: "product_family",
			Description:  "Container orchestration product family.",
			Values:       []string{"kubernetes"},
		},
		OS: &TermSlotOverride{
			SemanticName: "tier_or_mode",
			CLIFlag:      "--tier or --mode",
			Description:  "Cluster tier or run mode (e.g. EKS Fargate, AKS virtual-nodes, GKE Autopilot).",
			Values: []string{
				"standard", "extended-support",
				"fargate", "free", "virtual-nodes", "autopilot",
			},
		},
	},
}

// KindTermOverridesCatalog returns the full catalog of term-slot overrides,
// suitable for direct JSON encoding by `sku schema --kind-term-overrides`.
//
// Spec-level intent: agents fetch this once and use it to interpret the
// `terms.tenancy` / `terms.os` fields in subsequent price/list/compare
// envelopes. For any kind not in the returned map, both slots carry their
// literal column meaning (or are empty).
func KindTermOverridesCatalog() map[string]any {
	return map[string]any{
		"schema_version":      1,
		"kind_term_overrides": kindTermOverrides,
	}
}
