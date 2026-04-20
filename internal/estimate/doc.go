// Package estimate turns workload declarations into line-item costs.
//
// v1 accepts inline `--item <dsl>` values; later m5.x plans add YAML and
// stdin input forms. Each line item is dispatched through a kind-specific
// Estimator that resolves a catalog lookup and multiplies the on-demand
// hourly rate by the declared usage quantity.
package estimate
