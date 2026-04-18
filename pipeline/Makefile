SHELL := bash
.DEFAULT_GOAL := help

PY       ?= python3
VENV     := .venv
PIP      := $(VENV)/bin/pip
PYTEST   := $(VENV)/bin/pytest
RUFF     := $(VENV)/bin/ruff
PYRUN    := $(VENV)/bin/python

SHARD    ?=
FIXTURE  ?=
INGEST_EXTRA ?=
OUT_DIR  := ../dist/pipeline

.PHONY: help
help: ## Show this help
	@awk 'BEGIN {FS = ":.*?## "} /^[a-zA-Z_.-]+:.*?## / {printf "  %-20s %s\n", $$1, $$2}' $(MAKEFILE_LIST)

$(VENV)/bin/activate: pyproject.toml
	$(PY) -m venv $(VENV)
	$(PIP) install -e '.[dev]'
	@touch $(VENV)/bin/activate

.PHONY: setup
setup: $(VENV)/bin/activate ## Create venv and install deps

.PHONY: ingest
ingest: setup ## Run ingest for SHARD=<name>; optional FIXTURE=<path>
	@test -n "$(SHARD)" || (echo "SHARD=<name> required" && exit 2)
	mkdir -p $(OUT_DIR)
	$(PYRUN) -m ingest.$(SHARD) --out $(OUT_DIR)/$(SHARD).rows.jsonl $(if $(FIXTURE),--fixture $(FIXTURE),) $(INGEST_EXTRA)

.PHONY: package
package: setup ## Build SQLite shard from rows for SHARD=<name>
	@test -n "$(SHARD)" || (echo "SHARD=<name> required" && exit 2)
	$(PYRUN) -m package.build_shard \
	  --rows $(OUT_DIR)/$(SHARD).rows.jsonl \
	  --shard $(SHARD) \
	  --out $(OUT_DIR)/$(SHARD).db

.PHONY: shard
shard: ingest package ## Ingest + package in one go

.PHONY: test
test: setup
	$(PYTEST) -q

.PHONY: lint
lint: setup
	$(RUFF) check .
	$(RUFF) format --check .

.PHONY: clean
clean:
	rm -rf $(VENV) .pytest_cache __pycache__ */__pycache__ */*/__pycache__
