package estimate

import (
	"context"
	"fmt"
)

// Config bundles the inputs to Run.
type Config struct {
	Items []Item
}

// Run dispatches each Item through its registered Estimator and accumulates
// MonthlyTotalUSD. Errors short-circuit with item_index context.
func Run(ctx context.Context, cfg Config) (Result, error) {
	out := Result{Currency: "USD", Items: make([]LineItem, 0, len(cfg.Items))}
	for i, it := range cfg.Items {
		e, ok := Get(it.Kind)
		if !ok {
			return Result{}, fmt.Errorf("estimate: item %d (%q): no estimator for kind %q", i, it.Raw, it.Kind)
		}
		li, err := e.Estimate(ctx, it)
		if err != nil {
			return Result{}, fmt.Errorf("estimate: item %d (%q): %w", i, it.Raw, err)
		}
		out.Items = append(out.Items, li)
		out.MonthlyTotalUSD += li.MonthlyUSD
	}
	return out, nil
}
