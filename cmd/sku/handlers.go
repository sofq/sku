package sku

import "github.com/sofq/sku/internal/batch"

func init() {
	batch.Register("llm price", handleLLMPrice)
	batch.Register("aws ec2 price", handleAWSEC2Price)
	batch.Register("aws ec2 list", handleAWSEC2List)
}
