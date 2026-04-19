package sku

import "github.com/sofq/sku/internal/batch"

func init() {
	batch.Register("llm price", handleLLMPrice)
}
