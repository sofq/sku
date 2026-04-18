// Binary sku is the agent-friendly cloud & LLM pricing CLI.
//
// Keeping main.go at the module root makes `go install github.com/sofq/sku@latest`
// work without a `/cmd/sku` path suffix. All logic lives in cmd/sku.
package main

import "github.com/sofq/sku/cmd/sku"

func main() {
	sku.Execute()
}
