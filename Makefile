SHELL := bash
.DEFAULT_GOAL := help

GO            ?= go
BIN_DIR       := bin
BINARY        := $(BIN_DIR)/sku
PKG           := ./...
GO_LDFLAGS    := -s -w \
                 -X github.com/sofq/sku/internal/version.version=$(shell git describe --tags --always --dirty 2>/dev/null || echo dev) \
                 -X github.com/sofq/sku/internal/version.commit=$(shell git rev-parse HEAD 2>/dev/null || echo unknown) \
                 -X github.com/sofq/sku/internal/version.date=$(shell date -u +%Y-%m-%dT%H:%M:%SZ)

.PHONY: help
help: ## Show this help
	@awk 'BEGIN {FS = ":.*?## "} /^[a-zA-Z_.-]+:.*?## / {printf "  %-18s %s\n", $$1, $$2}' $(MAKEFILE_LIST)

.PHONY: build
build: ## Compile ./bin/sku
	@mkdir -p $(BIN_DIR)
	$(GO) build -trimpath -ldflags "$(GO_LDFLAGS)" -o $(BINARY) .

.PHONY: test
test: ## Run unit + integration tests
	$(GO) test -race -count=1 $(PKG)

.PHONY: lint
lint: ## Run golangci-lint
	golangci-lint run

.PHONY: clean
clean: ## Remove build artifacts
	rm -rf $(BIN_DIR) dist

.PHONY: generate
generate: ## Run go generate across the module (placeholder; used from M4)
	$(GO) generate $(PKG)

.PHONY: openrouter-shard
openrouter-shard: ## Build OpenRouter shard from fixtures into dist/pipeline/openrouter.db
	$(MAKE) -C pipeline shard SHARD=openrouter FIXTURE=testdata/openrouter

.PHONY: pipeline-test
pipeline-test: ## Run Python pipeline tests
	$(MAKE) -C pipeline test

.PHONY: bench
bench: ## Run Go benchmarks against the built OpenRouter shard
	@test -f dist/pipeline/openrouter.db || (echo "run 'make openrouter-shard' first" && exit 2)
	SKU_BENCH_SHARD=$(CURDIR)/dist/pipeline/openrouter.db \
	  $(GO) test -run=^$$ -bench=. -benchmem -count=5 ./bench/...

.PHONY: test-integration
test-integration: ## Run Go integration tests (requires built shard)
	@test -f dist/pipeline/openrouter.db || (echo "run 'make openrouter-shard' first" && exit 2)
	SKU_TEST_SHARD=$(CURDIR)/dist/pipeline/openrouter.db \
	  $(GO) test -tags=integration -race -count=1 ./...

.PHONY: release-dry
release-dry: ## Snapshot build via goreleaser; no publish
	goreleaser release --snapshot --clean
