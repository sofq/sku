package estimate

import (
	"context"
	"sync"
	"testing"
)

// Estimator converts a parsed Item into a LineItem by resolving a catalog
// lookup and multiplying usage quantity by the on-demand rate.
type Estimator interface {
	Kind() string
	Estimate(ctx context.Context, item Item) (LineItem, error)
}

var (
	registryMu sync.RWMutex
	registry   = map[string]Estimator{}
)

// Register adds an estimator; panics on duplicate kind to catch wiring bugs.
func Register(e Estimator) {
	registryMu.Lock()
	defer registryMu.Unlock()
	if _, dup := registry[e.Kind()]; dup {
		panic("estimate: duplicate estimator for kind " + e.Kind())
	}
	registry[e.Kind()] = e
}

// Get returns the registered estimator for kind, if any.
func Get(kind string) (Estimator, bool) {
	registryMu.RLock()
	defer registryMu.RUnlock()
	e, ok := registry[kind]
	return e, ok
}

// resetRegistry empties the registry between tests and restores the
// original contents when the test ends. Test-only (takes *testing.T).
func resetRegistry(t *testing.T) {
	t.Helper()
	registryMu.Lock()
	prev := registry
	registry = map[string]Estimator{}
	registryMu.Unlock()
	t.Cleanup(func() {
		registryMu.Lock()
		registry = prev
		registryMu.Unlock()
	})
}
