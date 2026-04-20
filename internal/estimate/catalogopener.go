package estimate

import (
	"github.com/sofq/sku/internal/catalog"
)

// catalogOpener is the seam the estimator uses to reach the shard layer.
// The default implementation delegates to catalog.Open + catalog.ShardPath;
// tests substitute an in-memory fake.
type catalogOpener interface {
	Open(shard string) (*catalog.Catalog, error)
}

type defaultOpener struct{}

func (defaultOpener) Open(shard string) (*catalog.Catalog, error) {
	return catalog.Open(catalog.ShardPath(shard))
}

// DefaultOpener returns the production opener; exposed for the CLI wiring.
func DefaultOpener() catalogOpener { return defaultOpener{} }
