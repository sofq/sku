SHELL := bash
.DEFAULT_GOAL := help

GO            ?= go
BIN_DIR       := bin
BINARY        := $(BIN_DIR)/sku
PKG           := ./...
GO_LDFLAGS    := -s -w \
                 -X github.com/sofq/sku/internal/version.version=$(shell git describe --tags --always --dirty --match "v*" 2>/dev/null || echo dev) \
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

.PHONY: aws-dynamodb-shard
aws-dynamodb-shard: ## Build aws-dynamodb shard from fixtures
	$(MAKE) -C pipeline shard SHARD=aws_dynamodb FIXTURE=testdata/aws_dynamodb \
	  INGEST_EXTRA='--catalog-version 2026.04.18'
	@mv dist/pipeline/aws_dynamodb.db dist/pipeline/aws-dynamodb.db
	@mv dist/pipeline/aws_dynamodb.rows.jsonl dist/pipeline/aws-dynamodb.rows.jsonl

.PHONY: aws-cloudfront-shard
aws-cloudfront-shard: ## Build aws-cloudfront shard from fixtures
	$(MAKE) -C pipeline shard SHARD=aws_cloudfront FIXTURE=testdata/aws_cloudfront \
	  INGEST_EXTRA='--catalog-version 2026.04.18'
	@mv dist/pipeline/aws_cloudfront.db dist/pipeline/aws-cloudfront.db
	@mv dist/pipeline/aws_cloudfront.rows.jsonl dist/pipeline/aws-cloudfront.rows.jsonl

.PHONY: aws-shards
aws-shards: aws-ec2-shard aws-rds-shard aws-s3-shard aws-lambda-shard aws-ebs-shard aws-dynamodb-shard aws-cloudfront-shard ## Build all aws shards (m3a.1+m3a.2+m3a.3)

.PHONY: azure-vm-shard
azure-vm-shard: ## Build azure-vm shard from fixtures
	$(MAKE) -C pipeline shard SHARD=azure_vm FIXTURE=testdata/azure_vm \
	  INGEST_EXTRA='--catalog-version 2026.04.18'
	@mv dist/pipeline/azure_vm.db dist/pipeline/azure-vm.db
	@mv dist/pipeline/azure_vm.rows.jsonl dist/pipeline/azure-vm.rows.jsonl

.PHONY: azure-sql-shard
azure-sql-shard: ## Build azure-sql shard from fixtures
	$(MAKE) -C pipeline shard SHARD=azure_sql FIXTURE=testdata/azure_sql \
	  INGEST_EXTRA='--catalog-version 2026.04.18'
	@mv dist/pipeline/azure_sql.db dist/pipeline/azure-sql.db
	@mv dist/pipeline/azure_sql.rows.jsonl dist/pipeline/azure-sql.rows.jsonl

.PHONY: azure-blob-shard
azure-blob-shard: ## Build azure-blob shard from fixtures
	$(MAKE) -C pipeline shard SHARD=azure_blob FIXTURE=testdata/azure_blob \
	  INGEST_EXTRA='--catalog-version 2026.04.18'
	@mv dist/pipeline/azure_blob.db dist/pipeline/azure-blob.db
	@mv dist/pipeline/azure_blob.rows.jsonl dist/pipeline/azure-blob.rows.jsonl

.PHONY: azure-functions-shard
azure-functions-shard: ## Build azure-functions shard from fixtures
	$(MAKE) -C pipeline shard SHARD=azure_functions FIXTURE=testdata/azure_functions \
	  INGEST_EXTRA='--catalog-version 2026.04.18'
	@mv dist/pipeline/azure_functions.db dist/pipeline/azure-functions.db
	@mv dist/pipeline/azure_functions.rows.jsonl dist/pipeline/azure-functions.rows.jsonl

.PHONY: azure-disks-shard
azure-disks-shard: ## Build azure-disks shard from fixtures
	$(MAKE) -C pipeline shard SHARD=azure_disks FIXTURE=testdata/azure_disks \
	  INGEST_EXTRA='--catalog-version 2026.04.18'
	@mv dist/pipeline/azure_disks.db dist/pipeline/azure-disks.db
	@mv dist/pipeline/azure_disks.rows.jsonl dist/pipeline/azure-disks.rows.jsonl

.PHONY: azure-postgres-shard
azure-postgres-shard: ## Build azure-postgres shard from fixtures
	$(MAKE) -C pipeline shard SHARD=azure_postgres FIXTURE=testdata/azure_postgres \
	  INGEST_EXTRA='--catalog-version 2026.04.18'
	@mv dist/pipeline/azure_postgres.db dist/pipeline/azure-postgres.db
	@mv dist/pipeline/azure_postgres.rows.jsonl dist/pipeline/azure-postgres.rows.jsonl

.PHONY: azure-mysql-shard
azure-mysql-shard: ## Build azure-mysql shard from fixtures
	$(MAKE) -C pipeline shard SHARD=azure_mysql FIXTURE=testdata/azure_mysql \
	  INGEST_EXTRA='--catalog-version 2026.04.18'
	@mv dist/pipeline/azure_mysql.db dist/pipeline/azure-mysql.db
	@mv dist/pipeline/azure_mysql.rows.jsonl dist/pipeline/azure-mysql.rows.jsonl

.PHONY: azure-mariadb-shard
azure-mariadb-shard: ## Build azure-mariadb shard from fixtures
	$(MAKE) -C pipeline shard SHARD=azure_mariadb FIXTURE=testdata/azure_mariadb \
	  INGEST_EXTRA='--catalog-version 2026.04.18'
	@mv dist/pipeline/azure_mariadb.db dist/pipeline/azure-mariadb.db
	@mv dist/pipeline/azure_mariadb.rows.jsonl dist/pipeline/azure-mariadb.rows.jsonl

.PHONY: azure-shards
azure-shards: azure-vm-shard azure-sql-shard azure-blob-shard azure-functions-shard azure-disks-shard azure-postgres-shard azure-mysql-shard azure-mariadb-shard ## Build azure shards

.PHONY: gcp-gce-shard
gcp-gce-shard: ## Build gcp-gce shard from fixtures
	$(MAKE) -C pipeline shard SHARD=gcp_gce FIXTURE=testdata/gcp_gce \
	  INGEST_EXTRA='--catalog-version 2026.04.18'
	@mv dist/pipeline/gcp_gce.db dist/pipeline/gcp-gce.db
	@mv dist/pipeline/gcp_gce.rows.jsonl dist/pipeline/gcp-gce.rows.jsonl

.PHONY: gcp-cloud-sql-shard
gcp-cloud-sql-shard: ## Build gcp-cloud-sql shard from fixtures
	$(MAKE) -C pipeline shard SHARD=gcp_cloud_sql FIXTURE=testdata/gcp_cloud_sql \
	  INGEST_EXTRA='--catalog-version 2026.04.18'
	@mv dist/pipeline/gcp_cloud_sql.db dist/pipeline/gcp-cloud-sql.db
	@mv dist/pipeline/gcp_cloud_sql.rows.jsonl dist/pipeline/gcp-cloud-sql.rows.jsonl

.PHONY: gcp-gcs-shard
gcp-gcs-shard: ## Build gcp-gcs shard from fixtures
	$(MAKE) -C pipeline shard SHARD=gcp_gcs FIXTURE=testdata/gcp_gcs \
	  INGEST_EXTRA='--catalog-version 2026.04.18'
	@mv dist/pipeline/gcp_gcs.db dist/pipeline/gcp-gcs.db
	@mv dist/pipeline/gcp_gcs.rows.jsonl dist/pipeline/gcp-gcs.rows.jsonl

.PHONY: gcp-run-shard
gcp-run-shard: ## Build gcp-run shard from fixtures
	$(MAKE) -C pipeline shard SHARD=gcp_run FIXTURE=testdata/gcp_run \
	  INGEST_EXTRA='--catalog-version 2026.04.18'
	@mv dist/pipeline/gcp_run.db dist/pipeline/gcp-run.db
	@mv dist/pipeline/gcp_run.rows.jsonl dist/pipeline/gcp-run.rows.jsonl

.PHONY: gcp-functions-shard
gcp-functions-shard: ## Build gcp-functions shard from fixtures
	$(MAKE) -C pipeline shard SHARD=gcp_functions FIXTURE=testdata/gcp_functions \
	  INGEST_EXTRA='--catalog-version 2026.04.18'
	@mv dist/pipeline/gcp_functions.db dist/pipeline/gcp-functions.db
	@mv dist/pipeline/gcp_functions.rows.jsonl dist/pipeline/gcp-functions.rows.jsonl

.PHONY: gcp-shards
gcp-shards: gcp-gce-shard gcp-cloud-sql-shard gcp-gcs-shard gcp-run-shard gcp-functions-shard ## Build gcp shards (m3b.3+m3b.4)

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
	@test -f dist/pipeline/aws-dynamodb.db   || (echo "run 'make aws-dynamodb-shard' first"   && exit 2)
	@test -f dist/pipeline/aws-cloudfront.db || (echo "run 'make aws-cloudfront-shard' first" && exit 2)
	@test -f dist/pipeline/azure-vm.db       || (echo "run 'make azure-vm-shard' first"      && exit 2)
	@test -f dist/pipeline/azure-sql.db      || (echo "run 'make azure-sql-shard' first"     && exit 2)
	@test -f dist/pipeline/azure-blob.db      || (echo "run 'make azure-blob-shard' first"      && exit 2)
	@test -f dist/pipeline/azure-functions.db || (echo "run 'make azure-functions-shard' first" && exit 2)
	@test -f dist/pipeline/azure-disks.db     || (echo "run 'make azure-disks-shard' first"     && exit 2)
	@test -f dist/pipeline/azure-postgres.db  || (echo "run 'make azure-postgres-shard' first"  && exit 2)
	@test -f dist/pipeline/azure-mysql.db     || (echo "run 'make azure-mysql-shard' first"     && exit 2)
	@test -f dist/pipeline/azure-mariadb.db   || (echo "run 'make azure-mariadb-shard' first"   && exit 2)
	@test -f dist/pipeline/gcp-gce.db         || (echo "run 'make gcp-gce-shard' first"         && exit 2)
	@test -f dist/pipeline/gcp-cloud-sql.db   || (echo "run 'make gcp-cloud-sql-shard' first"   && exit 2)
	@test -f dist/pipeline/gcp-gcs.db         || (echo "run 'make gcp-gcs-shard' first"         && exit 2)
	@test -f dist/pipeline/gcp-run.db         || (echo "run 'make gcp-run-shard' first"         && exit 2)
	@test -f dist/pipeline/gcp-functions.db   || (echo "run 'make gcp-functions-shard' first"   && exit 2)
	SKU_TEST_SHARD=$(CURDIR)/dist/pipeline/openrouter.db \
	  SKU_TEST_SHARD_DIR=$(CURDIR)/dist/pipeline \
	  $(GO) test -tags=integration -race -count=1 ./...

.PHONY: release-dry
release-dry: ## Snapshot build via goreleaser; no publish
	goreleaser release --snapshot --clean

.PHONY: docker-smoke
docker-smoke: ## Build a local Docker image from the snapshot binary and run sku version
	goreleaser release --snapshot --clean --skip=sign,publish,sbom
	docker buildx build --platform linux/amd64 --load \
		-f Dockerfile.goreleaser -t sku:smoke \
		dist/sku_linux_amd64_v1/
	docker run --rm sku:smoke version

.PHONY: release-check
release-check: ## Full local goreleaser dry-run incl. sign/sbom/docker (requires cosign+syft+docker)
	goreleaser release --snapshot --clean

.PHONY: npm-pack-smoke
npm-pack-smoke: build ## Pack root npm package and run `sku version` via shim
	cd npm && npm pack --dry-run
	node npm/bin/sku.js version || true

.PHONY: pypi-wheel-smoke
pypi-wheel-smoke: build ## Stage local binary + build a single wheel
	mkdir -p python/sku_cli/bin && cp bin/sku python/sku_cli/bin/
	cd python && python3 -m build --wheel

.PHONY: docs-verify
docs-verify: build ## Re-run every verified snippet in docs/getting-started.md + docs/commands/
	@test -d dist/pipeline || (echo "run 'make openrouter-shard aws-shards azure-shards gcp-shards' first" && exit 2)
	bash scripts/verify-docs-snippets.sh

# ---------------------------------------------------------------------------
# Discover + live ingest (m3a.4.1)

.PHONY: discover
discover: ## Run the discover module; DISCOVER_LIVE=1 hits real upstreams
	@mkdir -p $(CURDIR)/dist/pipeline/discover
	$(MAKE) -C pipeline setup
	cd pipeline && .venv/bin/python -m discover \
	  --state $(CURDIR)/dist/pipeline/discover/state.json \
	  --out   $(CURDIR)/dist/pipeline/discover/changed.json \
	  $(if $(DISCOVER_LIVE),--live,) \
	  $(if $(BASELINE_REBUILD),--baseline-rebuild,) \
	  $(if $(DISCOVER_SHARDS),--shards $(DISCOVER_SHARDS),)

# Map SHARD=<underscored> to the correct live-source flag for its ingest script.
_SHARD_LIVE_FLAG = $(if $(filter aws_%,$(SHARD)),--offer,\
                    $(if $(filter azure_%,$(SHARD)),--prices,\
                    $(if $(filter gcp_%,$(SHARD)),--skus,)))

.PHONY: gcp-machine-types-refresh
gcp-machine-types-refresh: ## Re-fetch GCE machineTypes fixture and verify all prefix strings against live billing SKUs (requires ADC + GCP_PROJECT=<id>)
	@if [ -z "$$GCP_PROJECT" ]; then echo "GCP_PROJECT=<project-id> required" >&2; exit 2; fi
	$(MAKE) -C pipeline setup
	cd pipeline && .venv/bin/python -c "\
import json, pathlib, sys, tempfile; \
from ingest.gcp_machine_types import _fetch_live, load_specs, verify_prefix_map, _FAMILY_PREFIX_MAP; \
from ingest.gcp_common import build_authenticated_session, fetch_skus; \
p = pathlib.Path('testdata/gcp_gce/machine_types.json'); \
p.write_text(json.dumps(_fetch_live(project_id='$$GCP_PROJECT'), indent=2, sort_keys=True) + '\n'); \
load_specs(fixture_path=p); \
print('fetching GCE billing SKUs for prefix verification...', file=sys.stderr); \
tmp = pathlib.Path(tempfile.mktemp(suffix='.json')); \
fetch_skus('gcp_gce', tmp, session=build_authenticated_session()); \
missing = verify_prefix_map(json.loads(tmp.read_text()), _FAMILY_PREFIX_MAP); \
tmp.unlink(missing_ok=True); \
(sys.exit(print(f'ERROR: _FAMILY_PREFIX_MAP has wrong/missing prefixes for: {missing}', file=sys.stderr) or 1) if missing \
 else print(f'prefix_map OK — all {len(_FAMILY_PREFIX_MAP)} families verified against live billing catalog', file=sys.stderr))"
	@echo "wrote pipeline/testdata/gcp_gce/machine_types.json — commit if it changed"

.PHONY: shard-live
shard-live: ## Ingest + package SHARD=<name> using SRC=<local-offer-path>
	@test -n "$(SHARD)" || (echo "SHARD=<name> required" && exit 2)
	@test -n "$(SRC)"   || (echo "SRC=<local-offer-path> required" && exit 2)
	@test -n "$(_SHARD_LIVE_FLAG)" || \
	  (echo "shard-live: SHARD=$(SHARD) is not aws_*, azure_*, or gcp_*" && exit 2)
	mkdir -p dist/pipeline
	$(MAKE) -C pipeline shard SHARD=$(SHARD) \
	  INGEST_EXTRA='$(_SHARD_LIVE_FLAG) $(SRC) $(INGEST_EXTRA)'

.PHONY: profile
profile:  ## Generate catalog coverage reports under docs/coverage/ from raw feeds.
	cd pipeline && uv run python -m catalog_profiler aws   --offer-dir   ../dist/pipeline/raw/aws    --out ../docs/coverage/aws.md
	cd pipeline && uv run python -m catalog_profiler azure --prices      ../dist/pipeline/raw/azure/prices.json --out ../docs/coverage/azure.md
	cd pipeline && uv run python -m catalog_profiler gcp   --catalog-paths ../dist/pipeline/raw/gcp/*.json      --out ../docs/coverage/gcp.md
