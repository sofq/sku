// Package batch provides the in-process handler registry and NDJSON/array
// dispatcher that powers `sku batch`. Handlers are registered by their
// canonical command name (space-separated, no flags) and are called directly
// from the batch runner with byte-identical error envelopes to the standalone
// path.
package batch

import (
	"context"
	"io"
	"maps"
	"sort"
	"sync"
	"testing"
)

// Settings mirrors the subset of cmd/sku global settings that handlers need.
// Kept in internal/batch to avoid an import cycle with cmd/sku.
type Settings struct {
	Preset            string
	Profile           string
	Format            string
	Pretty            bool
	JQ                string
	Fields            string
	IncludeRaw        bool
	IncludeAggregated bool
	AutoFetch         bool
	StaleOK           bool
	StaleWarningDays  int
	StaleErrorDays    int
	DryRun            bool
	Verbose           bool
	NoColor           bool
}

// Env is the ambient context passed to a handler.
type Env struct {
	Settings *Settings
	Stdout   io.Writer
	Stderr   io.Writer
}

// Handler is the typed signature every batch-registered command implements.
// result is the raw Go value the standalone command would marshal to stdout.
type Handler func(ctx context.Context, args map[string]any, env Env) (result any, err error)

var (
	mu       sync.RWMutex
	registry = map[string]Handler{}
)

// Register associates name with h. Panics on duplicate.
func Register(name string, h Handler) {
	mu.Lock()
	defer mu.Unlock()
	if _, dup := registry[name]; dup {
		panic("batch: duplicate Register for " + name)
	}
	registry[name] = h
}

// Lookup returns the handler for name.
func Lookup(name string) (Handler, bool) {
	mu.RLock()
	defer mu.RUnlock()
	h, ok := registry[name]
	return h, ok
}

// RegisteredNames returns all registered command names in lex order.
func RegisteredNames() []string {
	mu.RLock()
	defer mu.RUnlock()
	out := make([]string, 0, len(registry))
	for k := range registry {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// ResetForTest snapshots and restores the registry. Test-only.
func ResetForTest(t *testing.T) {
	t.Helper()
	mu.Lock()
	snap := make(map[string]Handler, len(registry))
	maps.Copy(snap, registry)
	registry = map[string]Handler{}
	mu.Unlock()
	t.Cleanup(func() {
		mu.Lock()
		registry = snap
		mu.Unlock()
	})
}
