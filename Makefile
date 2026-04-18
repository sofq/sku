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
	SKU_FIXED_OBSERVED_AT=1745020800 \
	  $(MAKE) -C pipeline shard SHARD=openrouter FIXTURE=testdata/openrouter \
	  INGEST_EXTRA='--skip-non-usd --generated-at 2026-04-18T00:00:00Z'

.PHONY: aws-ec2-shard
aws-ec2-shard: ## Build aws-ec2 shard from fixtures
	$(MAKE) -C pipeline shard SHARD=aws_ec2 FIXTURE=testdata/aws_ec2 \
	  INGEST_EXTRA='--catalog-version 2026.04.18'
	@mv dist/pipeline/aws_ec2.db dist/pipeline/aws-ec2.db
	@mv dist/pipeline/aws_ec2.rows.jsonl dist/pipeline/aws-ec2.rows.jsonl

.PHONY: aws-rds-shard
aws-rds-shard: ## Build aws-rds shard from fixtures
	$(MAKE) -C pipeline shard SHARD=aws_rds FIXTURE=testdata/aws_rds \
	  INGEST_EXTRA='--catalog-version 2026.04.18'
	@mv dist/pipeline/aws_rds.db dist/pipeline/aws-rds.db
	@mv dist/pipeline/aws_rds.rows.jsonl dist/pipeline/aws-rds.rows.jsonl

.PHONY: aws-s3-shard
aws-s3-shard: ## Build aws-s3 shard from fixtures
	$(MAKE) -C pipeline shard SHARD=aws_s3 FIXTURE=testdata/aws_s3 \
	  INGEST_EXTRA='--catalog-version 2026.04.18'
	@mv dist/pipeline/aws_s3.db dist/pipeline/aws-s3.db
	@mv dist/pipeline/aws_s3.rows.jsonl dist/pipeline/aws-s3.rows.jsonl

.PHONY: aws-lambda-shard
aws-lambda-shard: ## Build aws-lambda shard from fixtures
	$(MAKE) -C pipeline shard SHARD=aws_lambda FIXTURE=testdata/aws_lambda \
	  INGEST_EXTRA='--catalog-version 2026.04.18'
	@mv dist/pipeline/aws_lambda.db dist/pipeline/aws-lambda.db
	@mv dist/pipeline/aws_lambda.rows.jsonl dist/pipeline/aws-lambda.rows.jsonl

.PHONY: aws-ebs-shard
aws-ebs-shard: ## Build aws-ebs shard from fixtures
	$(MAKE) -C pipeline shard SHARD=aws_ebs FIXTURE=testdata/aws_ebs \
	  INGEST_EXTRA='--catalog-version 2026.04.18'
	@mv dist/pipeline/aws_ebs.db dist/pipeline/aws-ebs.db
	@mv dist/pipeline/aws_ebs.rows.jsonl dist/pipeline/aws-ebs.rows.jsonl

.PHONY: aws-shards
aws-shards: aws-ec2-shard aws-rds-shard aws-s3-shard aws-lambda-shard aws-ebs-shard ## Build all aws shards (m3a.1+m3a.2)

.PHONY: pipeline-test
pipeline-test: ## Run Python pipeline tests
	$(MAKE) -C pipeline test

.PHONY: bench
bench: ## Run Go benchmarks against the built OpenRouter shard
	@test -f dist/pipeline/openrouter.db || (echo "run 'make openrouter-shard' first" && exit 2)
	SKU_BENCH_SHARD=$(CURDIR)/dist/pipeline/openrouter.db \
	  $(GO) test -run=^$$ -bench=. -benchmem -count=5 ./bench/...

.PHONY: test-integration
test-integration: ## Run Go integration tests (requires built shards)
	@test -f dist/pipeline/openrouter.db || (echo "run 'make openrouter-shard' first" && exit 2)
	@test -f dist/pipeline/aws-ec2.db    || (echo "run 'make aws-ec2-shard' first"    && exit 2)
	@test -f dist/pipeline/aws-rds.db    || (echo "run 'make aws-rds-shard' first"    && exit 2)
	@test -f dist/pipeline/aws-s3.db     || (echo "run 'make aws-s3-shard' first"     && exit 2)
	@test -f dist/pipeline/aws-lambda.db || (echo "run 'make aws-lambda-shard' first" && exit 2)
	@test -f dist/pipeline/aws-ebs.db    || (echo "run 'make aws-ebs-shard' first"    && exit 2)
	SKU_TEST_SHARD=$(CURDIR)/dist/pipeline/openrouter.db \
	  SKU_TEST_SHARD_DIR=$(CURDIR)/dist/pipeline \
	  $(GO) test -tags=integration -race -count=1 ./...

.PHONY: release-dry
release-dry: ## Snapshot build via goreleaser; no publish
	goreleaser release --snapshot --clean
