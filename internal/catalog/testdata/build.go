// Package testdata exports thin helpers that build shard databases from the
// checked-in seed SQL files. Kept outside internal/catalog's test-only files
// so other packages (cmd/sku tests) can build fixture shards without
// duplicating the BuildFromSQL call.
package testdata

import (
	_ "embed"

	"github.com/sofq/sku/internal/catalog"
)

//go:embed seed_aws.sql
var seedAWS string

// BuildAWSShard writes an aws-ec2 shard at dst using the checked-in AWS seed.
func BuildAWSShard(dst string) error {
	return catalog.BuildFromSQL(dst, seedAWS)
}
