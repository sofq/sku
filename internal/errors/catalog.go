package errors

// CatalogSchema is the top-level envelope emitted by `sku schema --errors`.
// It pins every error Code to a fixed details shape so agents can author
// envelope-aware retry / fallback logic without spelunking through spec text.
// SchemaVersion bumps whenever a code is added, removed, or its DetailsFields
// list changes (see spec §4 error envelope).
type CatalogSchema struct {
	Entries       map[string]CodeEntry `json:"codes"`
	SchemaVersion int                  `json:"schema_version"`
}

// CodeEntry describes one row of the spec §4 error-code taxonomy.
type CodeEntry struct {
	ExitCode      int      `json:"exit_code"`
	Description   string   `json:"description"`
	DetailsFields []string `json:"details_fields"`
	// Reasons is non-empty only for "validation", which subdivides further.
	Reasons []string `json:"reasons,omitempty"`
}

// ErrorCatalog returns the in-memory catalog consumed by `sku schema --errors`
// and (eventually) by golden-envelope tests. The returned value is safe to
// mutate by callers — each call builds a fresh map.
func ErrorCatalog() CatalogSchema {
	return CatalogSchema{
		SchemaVersion: 1,
		Entries: map[string]CodeEntry{
			"generic_error": {
				ExitCode:      1,
				Description:   "unclassified error",
				DetailsFields: []string{"message_detail"},
			},
			"auth": {
				ExitCode:      2,
				Description:   "auth failure (CI-only)",
				DetailsFields: []string{"resource"},
			},
			"not_found": {
				ExitCode:      3,
				Description:   "no SKU matches filters",
				DetailsFields: []string{"provider", "service", "applied_filters", "nearest_matches"},
			},
			"validation": {
				ExitCode:    4,
				Description: "input failed validation",
				DetailsFields: []string{
					"reason", "flag", "value", "allowed", "shard",
					"required_binary_version", "hint",
				},
				Reasons: ValidationReasons(),
			},
			"rate_limited": {
				ExitCode:      5,
				Description:   "provider rate limited",
				DetailsFields: []string{"retry_after_ms"},
			},
			"conflict": {
				ExitCode:      6,
				Description:   "state conflict",
				DetailsFields: []string{"shard", "current_head_version", "expected_from", "operation"},
			},
			"server": {
				ExitCode:      7,
				Description:   "upstream server error",
				DetailsFields: []string{"upstream", "status_code", "correlation_id"},
			},
			"stale_data": {
				ExitCode:      8,
				Description:   "catalog older than threshold",
				DetailsFields: []string{"shard", "last_updated", "age_days", "threshold_days"},
			},
		},
	}
}

// ValidationReasons is the closed set of reason strings allowed under
// details.reason for a CodeValidation envelope.
func ValidationReasons() []string {
	return []string{
		"flag_invalid",
		"binary_too_old",
		"binary_too_new",
		"shard_too_old",
		"shard_too_new",
	}
}
