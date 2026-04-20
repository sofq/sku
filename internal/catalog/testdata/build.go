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

//go:embed seed_aws_m3a2.sql
var seedAWSM3A2 string

//go:embed seed.sql
var seedOpenRouter string

// BuildAWSShard writes an aws-ec2 shard at dst using the checked-in AWS seed.
func BuildAWSShard(dst string) error {
	return catalog.BuildFromSQL(dst, seedAWS)
}

// BuildAWSS3Shard writes an aws-s3 shard at dst using the m3a.2 seed, which
// ships with storage.object rows for storage-class=standard in us-east-1 and
// carries storage + requests-put + requests-get prices.
func BuildAWSS3Shard(dst string) error {
	return catalog.BuildFromSQL(dst, seedAWSM3A2)
}

// BuildOpenRouterShard writes an openrouter shard at dst using the M1 LLM
// seed, which ships three rows for anthropic/claude-opus-4.6 (anthropic,
// aws-bedrock, openrouter aggregated) with prompt + completion prices.
func BuildOpenRouterShard(dst string) error {
	return catalog.BuildFromSQL(dst, seedOpenRouter)
}
