# M1 — OpenRouter Shard + Catalog Reader + `sku llm price` Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Ship a working `sku llm price --model anthropic/claude-opus-4.6` that reads a real, daily-built OpenRouter shard from the local data dir and emits a valid §4 JSON payload — end to end, including the Python ingest pipeline, SQLite packaging, pure-Go catalog reader, output renderer, minimal error envelope, and a baseline-only `sku update openrouter`.

**Architecture:** Two independent halves joined by an on-disk SQLite file.

1. **Pipeline half (CI/maintainer-side, Python 3.11+):** `pipeline/ingest/openrouter.py` calls OpenRouter's two anonymous HTTP endpoints, normalizes per-endpoint rows into the §5 schema (one row per `(model, serving_provider)` + synthetic aggregated rows), canonicalizes terms, computes `terms_hash`, and hands off to `pipeline/package/build_shard.py` which emits `dist/pipeline/openrouter.db` (+ `.zst`). Run via `make -C pipeline ingest SHARD=openrouter` (live) or `SHARD=openrouter FIXTURE=...` (golden).
2. **Client half (pure Go):** `internal/catalog` opens the shard (`modernc.org/sqlite`, WAL, `PRAGMA foreign_keys=ON`), `internal/schema` re-computes `terms_hash` from the user's flag inputs using the exact same canonical encoding as the pipeline, `internal/output` renders a row to the §4 JSON envelope (agent preset only), and `internal/errors` maps error types to the §4 JSON stderr envelope + exit code. `cmd/sku/llm.go` wires `sku llm price`, `cmd/sku/update.go` wires a baseline-only `sku update openrouter`.

The two halves share one invariant that is continuously test-verified: the `terms_hash` Python function and the Go function produce byte-identical digests for a fixed tuple corpus.

**Tech Stack:** Go 1.25, `modernc.org/sqlite`, `github.com/klauspost/compress/zstd`, Python 3.11+, `requests`, `pyyaml`, stdlib `sqlite3` + `hashlib` (no DuckDB for OpenRouter — see spec §3), `pgregory.net/rapid` (property tests).

**Spec ref:** `docs/superpowers/specs/2026-04-18-sku-design.md` §3 (OpenRouter-specific ingest), §4 (CLI, output schema, error envelope, presets, exit codes), §5 (SQLite schema, `terms_hash`, indexes, metadata, perf targets), §6 (testing strategy), §9 M1.

**Rollover from M0 code review** (folded in, not deferred):
- `Execute()` returns `int` instead of `os.Exit`-ing internally — makes exit-code taxonomy unit-testable (Task 11).
- `Execute()` uses the §4 JSON error envelope on stderr instead of plain text (Task 11).
- `newRootCmd` design intent documented so M2's batch registry pattern doesn't reflexively export it (Task 11 code comment).

**Explicitly deferred to later milestones** (cross-referenced where they touch M1 surfaces):
- `price` / `full` / `compare` presets, `--jq`, `--fields`, `--include-raw`, `--yaml` / `--toml` output → M2.
- `sku configure`, profiles, `SKU_STALE_*` logic, shell completions → M2.
- `sku schema` discovery command → M2.
- Full exit-code taxonomy (only 0, 3, 4, 7 are needed and wired in M1; 1, 2, 5, 6, 8 are stubbed with consistent envelope but no callers yet) → M2 fills callers; M3a adds `auth` / `rate_limited` on updater side.
- Delta chain, manifest walking, ETag, rollback, concurrent-update flock, per-shard `head_version` tracking → M3a.
- Raw sibling shard + `--with-raw` → M3a (no `raw` column is populated in M1).
- `internal/updater` module — M1 uses a stripped-down inline implementation in `cmd/sku/update.go`; M3a extracts and replaces with the full module.
- `compare`, `search`, `estimate`, `batch` commands → M4/M5.
- Cloud shards (AWS/Azure/GCP) and their DuckDB-based ingest — M3a/M3b. `regions.yaml` and `enums.yaml` ship in M1 but carry only the OpenRouter-needed entries; each later milestone appends.

**Out of scope for M1 (declared now to prevent scope creep):** no CI wiring for the ingest pipeline (that's `data-daily.yml` in M3a); in M1 the shard is built manually by a maintainer and uploaded as a one-time GH release (Task 19). No cron, no matrix fan-out, no validation harness CI stage yet.

---

## File Structure

| Path | Responsibility |
|---|---|
| `pipeline/pyproject.toml` | Python project metadata: deps pinned via PEP 621, `ruff` + `pytest` dev deps |
| `pipeline/Makefile` | `ingest SHARD=openrouter`, `package SHARD=openrouter`, `shard SHARD=openrouter` (ingest+package), `test`, `lint`, `clean` |
| `pipeline/normalize/regions.yaml` | Canonical region-group map; M1 seed is minimal (`global: []`) — real groups land M3a |
| `pipeline/normalize/enums.yaml` | Allowed `kind`, `commitment`, `tenancy`, `os`, `support_tier`, `payment_option` values |
| `pipeline/normalize/terms_defaults.yaml` | Per-kind default terms used for canonicalization on both sides |
| `pipeline/normalize/terms.py` | Python canonical `terms_hash` + `canonicalize_terms` |
| `pipeline/normalize/enums.py` | Python enum loader + validator |
| `pipeline/ingest/__init__.py` | Empty package marker |
| `pipeline/ingest/openrouter.py` | Fetches `/api/v1/models` and `/api/v1/models/{slug}/endpoints`, joins + normalizes to row dicts, enforces USD invariant |
| `pipeline/ingest/http.py` | Thin HTTP client with retry + fixture-mode loader |
| `pipeline/package/__init__.py` | Empty package marker |
| `pipeline/package/schema.sql` | The full CREATE TABLE / CREATE INDEX DDL from §5 |
| `pipeline/package/build_shard.py` | Row list → SQLite shard (applies `schema.sql`, inserts rows in one transaction, populates metadata, writes `.db` + `.db.zst`) |
| `pipeline/testdata/openrouter/models.json` | Fixture response for `/api/v1/models` (trimmed, deterministic) |
| `pipeline/testdata/openrouter/endpoints/*.json` | One fixture per model slug used in tests |
| `pipeline/testdata/golden/openrouter_rows.jsonl` | Expected normalized rows for the fixture input |
| `pipeline/tests/test_terms.py` | Python-side `terms_hash` determinism |
| `pipeline/tests/test_openrouter_ingest.py` | Fixture → normalized rows golden test; USD-guard rejection test |
| `pipeline/tests/test_build_shard.py` | Rows → SQLite round-trip; schema/index/metadata assertions |
| `internal/schema/terms.go` | Canonical `TermsHash(...) string` + `CanonicalizeTerms(...) Terms` |
| `internal/schema/terms_test.go` | Unit + golden tests |
| `internal/schema/testdata/terms_golden.jsonl` | Shared fixture consumed by Go and Python tests |
| `internal/schema/kind.go` | Enum values for `kind` (minimal: `llm.text`, `llm.multimodal`, `llm.embedding`, `llm.image`, `llm.audio`; cloud kinds appended in M3a) |
| `internal/catalog/catalog.go` | `Catalog` type, `Open()`, platform data-dir resolution |
| `internal/catalog/lookup.go` | `LookupLLM(ctx, filter) ([]Row, error)` + `Row` struct |
| `internal/catalog/catalog_test.go` | Unit tests with `:memory:` seeded SQLite |
| `internal/catalog/integration_test.go` | `-tags=integration`; opens a real built fixture file on disk |
| `internal/catalog/testdata/seed.sql` | Hand-built minimal shard SQL |
| `internal/output/render.go` | `RenderRow(r Row) Envelope` + `Encode(w io.Writer, e Envelope, pretty bool) error` |
| `internal/output/render_test.go` | Golden JSON payload tests |
| `internal/errors/errors.go` | `Code` int type, `Error` struct, `Envelope` struct, `Write(w io.Writer, err error) int` |
| `internal/errors/errors_test.go` | Mapping + JSON-shape tests |
| `cmd/sku/execute.go` | **Modified**: `Execute() int`, writes JSON envelope on error |
| `cmd/sku/llm.go` | `newLLMCmd()` parent |
| `cmd/sku/llm_price.go` | `newLLMPriceCmd()` — flags, lookup call, render |
| `cmd/sku/llm_price_test.go` | Cobra-level tests with seeded catalog |
| `cmd/sku/update.go` | Minimal `newUpdateCmd()` — hardcoded URL, sha256, zstd unpack |
| `cmd/sku/update_test.go` | httptest-server tests |
| `main.go` | **Modified**: `os.Exit(sku.Execute())` |
| `bench/catalog_bench_test.go` | `BenchmarkPointLookup_Warm`, `BenchmarkPointLookup_Cold` |
| `Makefile` | **Modified**: add `bench`, `openrouter-shard`, `test-integration` |
| `CLAUDE.md` | **Modified**: bump current-milestone pointer to M1 |
| `docs/ops/m1-bootstrap-release.md` | One-page runbook for the maintainer one-shot bootstrap upload |

---

## Task 0: Pre-flight — branch, deps, env

**Files:** none

- [x] **Step 1: Create working branch**

```bash
git checkout -b m1-openrouter-shard
git pull --rebase origin master 2>/dev/null || true
```

- [x] **Step 2: Confirm Python ≥ 3.11 available**

Run: `python3 --version`
Expected: `Python 3.11.x` or newer. If not, install via `brew install python@3.12` (macOS) or `uv python install 3.12`.

- [x] **Step 3: Pull new Go deps**

```bash
go get modernc.org/sqlite@latest
go get github.com/klauspost/compress/zstd@latest
go get pgregory.net/rapid@latest
go mod tidy
```

Expected: `go.mod` gains all three in the `require` block. `go.sum` updates. No build errors (`go build ./...`).

- [x] **Step 4: Commit the dep bump**

```bash
git add go.mod go.sum
git commit -m "build: add modernc.org/sqlite, zstd, rapid for M1"
```

---

## Task 1: Pipeline skeleton (`pipeline/pyproject.toml`, `pipeline/Makefile`)

**Files:**
- Create: `pipeline/pyproject.toml`, `pipeline/Makefile`, `pipeline/.gitignore`

- [x] **Step 1: Write `pipeline/pyproject.toml`**

```toml
[project]
name = "sku-pipeline"
version = "0.0.0"
description = "sku data pipeline (CI-only; not published)"
requires-python = ">=3.11"
dependencies = [
  "requests>=2.32",
  "pyyaml>=6.0",
  "zstandard>=0.22",
]

[project.optional-dependencies]
dev = [
  "pytest>=8.0",
  "ruff>=0.5",
]

[tool.ruff]
line-length = 100
target-version = "py311"

[tool.ruff.lint]
select = ["E", "F", "I", "UP", "B", "SIM"]
```

- [x] **Step 2: Write `pipeline/Makefile`**

```make
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
	$(PYRUN) -m ingest.$(SHARD) --out $(OUT_DIR)/$(SHARD).rows.jsonl $(if $(FIXTURE),--fixture $(FIXTURE),)

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
```

- [x] **Step 3: Write `pipeline/.gitignore`**

```
.venv/
__pycache__/
*.pyc
.pytest_cache/
.ruff_cache/
```

- [x] **Step 4: Smoke test the help target**

Run: `make -C pipeline help`
Expected: help lines for `setup`, `ingest`, `package`, `shard`, `test`, `lint`, `clean`.

- [x] **Step 5: Commit**

```bash
git add pipeline/pyproject.toml pipeline/Makefile pipeline/.gitignore
git commit -m "pipeline: scaffold Python pipeline (pyproject, Makefile, gitignore)"
```

---

## Task 2: Normalize YAMLs (regions, enums, terms defaults)

These three files are the single source of truth for the pipeline; the Go side reads them via `make generate` starting in M3a. In M1 the Go side hand-copies the few enum values it needs into `internal/schema/kind.go`.

**Files:**
- Create: `pipeline/normalize/__init__.py`, `pipeline/normalize/regions.yaml`, `pipeline/normalize/enums.yaml`, `pipeline/normalize/terms_defaults.yaml`

- [x] **Step 1: Write `pipeline/normalize/__init__.py`** (empty file; package marker)

- [x] **Step 2: Write `pipeline/normalize/regions.yaml`**

```yaml
# Canonical region-group map. M1 seed: only the global bucket used by
# OpenRouter (regionless/global SKUs use `region=''`, `region_normalized=''`).
# Real region groups land in M3a when cloud shards arrive.
# Format per spec §5 Region normalization.
version: 1
groups: {}
```

- [x] **Step 3: Write `pipeline/normalize/enums.yaml`**

```yaml
# Every enum the shard-build validator enforces (§5 Enum validation).
# M1 ships the minimum set needed by OpenRouter; later milestones append.
version: 1

kind:
  # LLM kinds (seeded now because OpenRouter uses them)
  - llm.text
  - llm.multimodal
  - llm.embedding
  - llm.image
  - llm.audio
  # Cloud kinds appended in M3a+
  # - compute.vm
  # - storage.object
  # - db.relational
  # ...

commitment:
  - on_demand
  # - spot
  # - reserved_1y
  # - reserved_3y
  # - savings_plan_1y
  # - savings_plan_3y
  # - cud_1y
  # - cud_3y

tenancy:
  - ""            # non-applicable (LLMs, storage, etc.)
  # - shared
  # - dedicated
  # - host

os:
  - ""            # non-applicable
  # - linux
  # - windows
  # - rhel

support_tier:
  - ""

payment_option:
  - ""
```

- [x] **Step 4: Write `pipeline/normalize/terms_defaults.yaml`**

```yaml
# Per-kind defaults substituted before terms_hash is computed, so a user
# omitting flags hits the same row the ingester wrote (spec §5
# Unspecified-flag resolution).
version: 1

defaults:
  llm.text:
    commitment: on_demand
    tenancy: ""
    os: ""
    support_tier: ""
    upfront: ""
    payment_option: ""
  llm.multimodal:
    commitment: on_demand
    tenancy: ""
    os: ""
    support_tier: ""
    upfront: ""
    payment_option: ""
  llm.embedding:
    commitment: on_demand
    tenancy: ""
    os: ""
    support_tier: ""
    upfront: ""
    payment_option: ""
  llm.image:
    commitment: on_demand
    tenancy: ""
    os: ""
    support_tier: ""
    upfront: ""
    payment_option: ""
  llm.audio:
    commitment: on_demand
    tenancy: ""
    os: ""
    support_tier: ""
    upfront: ""
    payment_option: ""
```

- [x] **Step 5: Commit**

```bash
git add pipeline/normalize/__init__.py pipeline/normalize/regions.yaml pipeline/normalize/enums.yaml pipeline/normalize/terms_defaults.yaml
git commit -m "pipeline: add normalize/{regions,enums,terms_defaults}.yaml (M1 seed)"
```

---

## Task 3: Shared terms-hash contract + golden fixture

**The single invariant that joins Python and Go.** Both sides must produce byte-identical `terms_hash` for identical canonical inputs. We write the contract once (as a JSONL golden file) and both sides assert against it.

**Files:**
- Create: `internal/schema/testdata/terms_golden.jsonl`

- [x] **Step 1: Write `internal/schema/testdata/terms_golden.jsonl`**

The canonical encoding, baked once into fixture bytes so it can't drift:
- Input: 6-tuple `(commitment, tenancy, os, support_tier, upfront, payment_option)`, each string (empty `""` allowed).
- Canonical: JSON array of those six strings in that exact order, `json.dumps(..., separators=(",", ":"), ensure_ascii=False)`, UTF-8 bytes.
- Hash: `sha256(canonical_bytes).hexdigest()[:32]` (128-bit hex, 32 chars, lowercase).

```jsonl
{"name":"llm_default","input":{"commitment":"on_demand","tenancy":"","os":"","support_tier":"","upfront":"","payment_option":""},"canonical":"[\"on_demand\",\"\",\"\",\"\",\"\",\"\"]","terms_hash":"ee2303ad38b3e0b0e4f01bfbb1bcba8f"}
{"name":"vm_linux_shared","input":{"commitment":"on_demand","tenancy":"shared","os":"linux","support_tier":"","upfront":"","payment_option":""},"canonical":"[\"on_demand\",\"shared\",\"linux\",\"\",\"\",\"\"]","terms_hash":"9f5d3b6f5f5afe9b26baa0e8e21a2e51"}
{"name":"vm_windows_dedicated","input":{"commitment":"reserved_1y","tenancy":"dedicated","os":"windows","support_tier":"standard","upfront":"partial","payment_option":"monthly"},"canonical":"[\"reserved_1y\",\"dedicated\",\"windows\",\"standard\",\"partial\",\"monthly\"]","terms_hash":"d1d7db1462f6b3ee2b9a28f10acb33e0"}
```

Note: the `terms_hash` values above are placeholders — Step 3 of the next task computes the real digests and this file is regenerated once, then both sides assert against it. Keep them as-is for now; Task 5 writes the actual values.

- [x] **Step 2: Commit the placeholder fixture**

```bash
git add internal/schema/testdata/terms_golden.jsonl
git commit -m "schema: add placeholder terms_hash golden fixture (real values in Task 5)"
```

---

## Task 4: Python `terms_hash` + enums loader (TDD)

**Files:**
- Create: `pipeline/normalize/terms.py`, `pipeline/normalize/enums.py`
- Create: `pipeline/tests/__init__.py`, `pipeline/tests/test_terms.py`

- [x] **Step 1: Write the failing test**

`pipeline/tests/test_terms.py`:

```python
import json
from pathlib import Path

import pytest

from normalize.terms import canonicalize_terms, terms_hash

REPO_ROOT = Path(__file__).resolve().parents[2]
GOLDEN = REPO_ROOT / "internal" / "schema" / "testdata" / "terms_golden.jsonl"


def load_golden():
    rows = []
    with GOLDEN.open() as fh:
        for line in fh:
            line = line.strip()
            if line:
                rows.append(json.loads(line))
    return rows


def test_canonical_and_hash_match_golden():
    for row in load_golden():
        got_canonical = canonicalize_terms(row["input"])
        assert got_canonical == row["canonical"], row["name"]
        assert terms_hash(row["input"]) == row["terms_hash"], row["name"]


def test_hash_is_32_char_lowercase_hex():
    h = terms_hash({
        "commitment": "on_demand",
        "tenancy": "",
        "os": "",
        "support_tier": "",
        "upfront": "",
        "payment_option": "",
    })
    assert len(h) == 32
    assert h == h.lower()
    assert all(c in "0123456789abcdef" for c in h)


def test_missing_key_raises():
    with pytest.raises(KeyError):
        terms_hash({"commitment": "on_demand"})  # incomplete
```

- [x] **Step 2: Run the test to confirm it fails**

Run: `make -C pipeline setup && cd pipeline && .venv/bin/pytest tests/test_terms.py -q`
Expected: `ModuleNotFoundError: No module named 'normalize.terms'` (or similar import-time error).

- [x] **Step 3: Write `pipeline/normalize/terms.py`**

```python
"""Canonical terms encoding + terms_hash. Must match internal/schema/terms.go byte-for-byte."""

from __future__ import annotations

import hashlib
import json
from typing import Mapping

# Fixed field order — MUST NOT CHANGE without a schema_version bump.
_FIELD_ORDER: tuple[str, ...] = (
    "commitment",
    "tenancy",
    "os",
    "support_tier",
    "upfront",
    "payment_option",
)


def canonicalize_terms(terms: Mapping[str, str]) -> str:
    """Return the canonical JSON encoding of a terms mapping.

    Missing keys raise KeyError — callers are expected to fill defaults
    (via terms_defaults.yaml) before hashing.
    """
    values = [terms[k] for k in _FIELD_ORDER]
    # separators=(",", ":") -> no whitespace; ensure_ascii=False -> UTF-8 passthrough.
    return json.dumps(values, separators=(",", ":"), ensure_ascii=False)


def terms_hash(terms: Mapping[str, str]) -> str:
    """128-bit hex digest of the canonical encoding (first 32 hex chars of sha256)."""
    canonical = canonicalize_terms(terms)
    return hashlib.sha256(canonical.encode("utf-8")).hexdigest()[:32]
```

- [x] **Step 4: Write `pipeline/normalize/enums.py`**

```python
"""Loader + validator for the enums.yaml / terms_defaults.yaml files."""

from __future__ import annotations

from functools import lru_cache
from pathlib import Path
from typing import Mapping

import yaml

_HERE = Path(__file__).resolve().parent


@lru_cache(maxsize=1)
def load_enums() -> dict[str, list[str]]:
    with (_HERE / "enums.yaml").open() as fh:
        doc = yaml.safe_load(fh)
    out = {k: v for k, v in doc.items() if k != "version"}
    return out


@lru_cache(maxsize=1)
def load_terms_defaults() -> dict[str, dict[str, str]]:
    with (_HERE / "terms_defaults.yaml").open() as fh:
        doc = yaml.safe_load(fh)
    return doc["defaults"]


def validate_enum(field: str, value: str) -> None:
    enums = load_enums()
    if field not in enums:
        raise KeyError(f"unknown enum field: {field!r}")
    if value not in enums[field]:
        allowed = ", ".join(repr(v) for v in enums[field])
        raise ValueError(f"{field}={value!r} not in allowed: {allowed}")


def apply_kind_defaults(kind: str, terms: Mapping[str, str]) -> dict[str, str]:
    """Return a new dict where missing keys are filled from the kind's defaults."""
    defaults = load_terms_defaults().get(kind)
    if defaults is None:
        raise KeyError(f"no terms defaults for kind={kind!r}")
    out = dict(defaults)
    out.update({k: v for k, v in terms.items() if v is not None})
    return out
```

- [x] **Step 5: Regenerate real golden values** (one-shot, then commit)

Run: `cd pipeline && .venv/bin/python - <<'PY'
import json, hashlib
from normalize.terms import canonicalize_terms, terms_hash

cases = [
    ("llm_default", {"commitment":"on_demand","tenancy":"","os":"","support_tier":"","upfront":"","payment_option":""}),
    ("vm_linux_shared", {"commitment":"on_demand","tenancy":"shared","os":"linux","support_tier":"","upfront":"","payment_option":""}),
    ("vm_windows_dedicated", {"commitment":"reserved_1y","tenancy":"dedicated","os":"windows","support_tier":"standard","upfront":"partial","payment_option":"monthly"}),
]
for name, inp in cases:
    print(json.dumps({
        "name": name,
        "input": inp,
        "canonical": canonicalize_terms(inp),
        "terms_hash": terms_hash(inp),
    }, separators=(",", ":")))
PY
> ../internal/schema/testdata/terms_golden.jsonl
`

Then re-run the pytest: `.venv/bin/pytest tests/test_terms.py -q` — expected: **PASS** on all three cases.

- [x] **Step 6: Commit**

```bash
git add pipeline/normalize/terms.py pipeline/normalize/enums.py pipeline/tests/__init__.py pipeline/tests/test_terms.py internal/schema/testdata/terms_golden.jsonl
git commit -m "pipeline: add canonical terms_hash + enums loader (Python side)"
```

---

## Task 5: Go `terms_hash` matching the same golden fixture (TDD)

**Files:**
- Create: `internal/schema/terms.go`, `internal/schema/terms_test.go`, `internal/schema/kind.go`

- [x] **Step 1: Write the failing test**

`internal/schema/terms_test.go`:

```go
package schema

import (
	"bufio"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

type goldenCase struct {
	Name      string            `json:"name"`
	Input     map[string]string `json:"input"`
	Canonical string            `json:"canonical"`
	TermsHash string            `json:"terms_hash"`
}

func loadGolden(t *testing.T) []goldenCase {
	t.Helper()
	f, err := os.Open(filepath.Join("testdata", "terms_golden.jsonl"))
	require.NoError(t, err)
	t.Cleanup(func() { _ = f.Close() })

	var out []goldenCase
	s := bufio.NewScanner(f)
	for s.Scan() {
		line := s.Bytes()
		if len(line) == 0 {
			continue
		}
		var c goldenCase
		require.NoError(t, json.Unmarshal(line, &c))
		out = append(out, c)
	}
	require.NoError(t, s.Err())
	require.NotEmpty(t, out)
	return out
}

func TestCanonicalizeTerms_MatchesGolden(t *testing.T) {
	for _, c := range loadGolden(t) {
		terms := Terms{
			Commitment:    c.Input["commitment"],
			Tenancy:       c.Input["tenancy"],
			OS:            c.Input["os"],
			SupportTier:   c.Input["support_tier"],
			Upfront:       c.Input["upfront"],
			PaymentOption: c.Input["payment_option"],
		}
		got := CanonicalizeTerms(terms)
		require.Equal(t, c.Canonical, got, "case %s", c.Name)
	}
}

func TestTermsHash_MatchesGolden(t *testing.T) {
	for _, c := range loadGolden(t) {
		terms := Terms{
			Commitment:    c.Input["commitment"],
			Tenancy:       c.Input["tenancy"],
			OS:            c.Input["os"],
			SupportTier:   c.Input["support_tier"],
			Upfront:       c.Input["upfront"],
			PaymentOption: c.Input["payment_option"],
		}
		got := TermsHash(terms)
		require.Equal(t, c.TermsHash, got, "case %s", c.Name)
		require.Len(t, got, 32, "128-bit hex = 32 chars")
	}
}
```

- [x] **Step 2: Run the test to confirm it fails**

Run: `go test ./internal/schema/...`
Expected: FAIL — `undefined: Terms`, `undefined: CanonicalizeTerms`, `undefined: TermsHash`.

- [x] **Step 3: Write `internal/schema/terms.go`**

```go
// Package schema holds shared data-schema primitives used by both the catalog
// reader and (eventually) code-generated clients. The canonical terms encoding
// in this file MUST remain byte-identical to pipeline/normalize/terms.py —
// their equality is asserted by testdata/terms_golden.jsonl which is consumed
// by both test suites.
package schema

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
)

// Terms is the six-tuple that participates in terms_hash.
// Field order in the JSON array is fixed (see CanonicalizeTerms) and a
// schema_version bump is required to change it.
type Terms struct {
	Commitment    string
	Tenancy       string
	OS            string
	SupportTier   string
	Upfront       string
	PaymentOption string
}

// CanonicalizeTerms returns the canonical JSON array encoding of Terms.
// Output: `["commitment","tenancy","os","support_tier","upfront","payment_option"]`
// with no whitespace, matching json.dumps(..., separators=(",", ":")) on the Python side.
func CanonicalizeTerms(t Terms) string {
	// Use json.Marshal on a []string so Go's encoder emits the exact same
	// bytes as Python's json.dumps with compact separators: no spaces, fields
	// escaped identically.
	arr := [6]string{t.Commitment, t.Tenancy, t.OS, t.SupportTier, t.Upfront, t.PaymentOption}
	b, err := json.Marshal(arr)
	if err != nil {
		// []string marshalling cannot fail; panic preserves the invariant.
		panic("schema.CanonicalizeTerms: unreachable marshal error: " + err.Error())
	}
	return string(b)
}

// TermsHash returns the 128-bit hex digest (32 chars) of CanonicalizeTerms(t).
func TermsHash(t Terms) string {
	sum := sha256.Sum256([]byte(CanonicalizeTerms(t)))
	return hex.EncodeToString(sum[:16])
}
```

- [x] **Step 4: Write `internal/schema/kind.go`**

```go
package schema

// Kind enumerates the resource kinds understood by this binary.
// New kinds are appended per §5 and require a binary release; see spec
// §5 Kind taxonomy. M1 seed ships only LLM kinds because the only live
// shard is openrouter.
type Kind string

const (
	KindLLMText       Kind = "llm.text"
	KindLLMMultimodal Kind = "llm.multimodal"
	KindLLMEmbedding  Kind = "llm.embedding"
	KindLLMImage      Kind = "llm.image"
	KindLLMAudio      Kind = "llm.audio"
)

// IsLLM reports whether k is one of the llm.* kinds.
func (k Kind) IsLLM() bool {
	switch k {
	case KindLLMText, KindLLMMultimodal, KindLLMEmbedding, KindLLMImage, KindLLMAudio:
		return true
	}
	return false
}
```

- [x] **Step 5: Run the test to verify it passes**

Run: `go test ./internal/schema/... -v`
Expected: PASS on both tests across all three golden cases.

- [x] **Step 6: Commit**

```bash
git add internal/schema/terms.go internal/schema/kind.go internal/schema/terms_test.go
git commit -m "feat(schema): add Terms, CanonicalizeTerms, TermsHash matching Python golden"
```

---

## Task 6: OpenRouter ingest fixtures

Before writing the ingest script, commit a trimmed, deterministic snapshot of the live API so tests are offline and stable.

**Files:**
- Create: `pipeline/testdata/openrouter/models.json`
- Create: `pipeline/testdata/openrouter/endpoints/anthropic__claude-opus-4.6.json`
- Create: `pipeline/testdata/openrouter/endpoints/openai__gpt-5.json`
- Create: `pipeline/testdata/openrouter/endpoints/non-usd-model__some-provider.json`

- [x] **Step 1: Write `pipeline/testdata/openrouter/models.json`**

Three models chosen to exercise: (a) a pure-text LLM with two endpoints, (b) a multimodal LLM with one endpoint, (c) a model with a non-USD endpoint (triggers the USD guard).

```json
{
  "data": [
    {
      "id": "anthropic/claude-opus-4.6",
      "name": "Anthropic: Claude Opus 4.6",
      "context_length": 200000,
      "architecture": {
        "modality": "text",
        "input_modalities": ["text"],
        "output_modalities": ["text"],
        "tokenizer": "Claude"
      },
      "pricing": {
        "prompt": "0.000015",
        "completion": "0.000075",
        "image": "0",
        "request": "0",
        "currency": "USD"
      },
      "top_provider": {
        "context_length": 200000,
        "max_completion_tokens": 64000,
        "is_moderated": false
      },
      "supported_parameters": ["tools", "tool_choice", "temperature", "top_p"]
    },
    {
      "id": "openai/gpt-5",
      "name": "OpenAI: GPT-5",
      "context_length": 400000,
      "architecture": {
        "modality": "text+image->text",
        "input_modalities": ["text", "image"],
        "output_modalities": ["text"],
        "tokenizer": "GPT"
      },
      "pricing": {
        "prompt": "0.000005",
        "completion": "0.000020",
        "image": "0.00003",
        "request": "0",
        "currency": "USD"
      },
      "top_provider": {
        "context_length": 400000,
        "max_completion_tokens": 32000,
        "is_moderated": true
      },
      "supported_parameters": ["tools", "tool_choice", "temperature", "top_p", "response_format"]
    },
    {
      "id": "non-usd-model/some-provider",
      "name": "Non-USD Partner Model",
      "context_length": 32768,
      "architecture": {
        "modality": "text",
        "input_modalities": ["text"],
        "output_modalities": ["text"],
        "tokenizer": "Other"
      },
      "pricing": {
        "prompt": "0.0001",
        "completion": "0.0003",
        "image": "0",
        "request": "0",
        "currency": "EUR"
      },
      "top_provider": {
        "context_length": 32768,
        "max_completion_tokens": 4096,
        "is_moderated": false
      },
      "supported_parameters": ["temperature"]
    }
  ]
}
```

- [x] **Step 2: Write `pipeline/testdata/openrouter/endpoints/anthropic__claude-opus-4.6.json`**

(Two endpoints — the model's first-party `anthropic` + `aws-bedrock`.)

```json
{
  "data": {
    "id": "anthropic/claude-opus-4.6",
    "name": "Anthropic: Claude Opus 4.6",
    "architecture": {
      "modality": "text",
      "input_modalities": ["text"],
      "output_modalities": ["text"]
    },
    "endpoints": [
      {
        "provider_name": "anthropic",
        "tag": "anthropic",
        "context_length": 200000,
        "max_completion_tokens": 64000,
        "quantization": null,
        "pricing": {
          "prompt": "0.000015",
          "completion": "0.000075",
          "image": "0",
          "request": "0",
          "currency": "USD"
        },
        "uptime_last_30m": 0.998,
        "latency": {"p50_ms": 420, "p95_ms": 1100},
        "throughput_tokens_per_second": 62.5
      },
      {
        "provider_name": "aws-bedrock",
        "tag": "aws-bedrock",
        "context_length": 200000,
        "max_completion_tokens": 64000,
        "quantization": null,
        "pricing": {
          "prompt": "0.000015",
          "completion": "0.000075",
          "image": "0",
          "request": "0",
          "currency": "USD"
        },
        "uptime_last_30m": 0.995,
        "latency": {"p50_ms": 450, "p95_ms": 1300},
        "throughput_tokens_per_second": 55.0
      }
    ]
  }
}
```

- [x] **Step 3: Write `pipeline/testdata/openrouter/endpoints/openai__gpt-5.json`**

```json
{
  "data": {
    "id": "openai/gpt-5",
    "name": "OpenAI: GPT-5",
    "architecture": {
      "modality": "text+image->text",
      "input_modalities": ["text", "image"],
      "output_modalities": ["text"]
    },
    "endpoints": [
      {
        "provider_name": "openai",
        "tag": "openai",
        "context_length": 400000,
        "max_completion_tokens": 32000,
        "quantization": null,
        "pricing": {
          "prompt": "0.000005",
          "completion": "0.000020",
          "image": "0.00003",
          "request": "0",
          "currency": "USD"
        },
        "uptime_last_30m": 0.999,
        "latency": {"p50_ms": 380, "p95_ms": 950},
        "throughput_tokens_per_second": 88.0
      }
    ]
  }
}
```

- [x] **Step 4: Write `pipeline/testdata/openrouter/endpoints/non-usd-model__some-provider.json`**

```json
{
  "data": {
    "id": "non-usd-model/some-provider",
    "name": "Non-USD Partner Model",
    "architecture": {
      "modality": "text",
      "input_modalities": ["text"],
      "output_modalities": ["text"]
    },
    "endpoints": [
      {
        "provider_name": "some-provider",
        "tag": "some-provider",
        "context_length": 32768,
        "max_completion_tokens": 4096,
        "quantization": null,
        "pricing": {
          "prompt": "0.0001",
          "completion": "0.0003",
          "image": "0",
          "request": "0",
          "currency": "EUR"
        },
        "uptime_last_30m": 0.950,
        "latency": {"p50_ms": 700, "p95_ms": 2100},
        "throughput_tokens_per_second": 30.0
      }
    ]
  }
}
```

- [x] **Step 5: Commit the fixtures**

```bash
git add pipeline/testdata/openrouter/
git commit -m "pipeline: add OpenRouter ingest fixtures (3 models, USD guard case)"
```

---

## Task 7: HTTP client with fixture mode + live mode

**Files:**
- Create: `pipeline/ingest/__init__.py`, `pipeline/ingest/http.py`
- Create: `pipeline/tests/test_http_fixture.py`

- [x] **Step 1: Write `pipeline/ingest/__init__.py`** (empty)

- [x] **Step 2: Write the failing test**

`pipeline/tests/test_http_fixture.py`:

```python
from pathlib import Path

from ingest.http import FixtureClient


def test_fixture_client_loads_models_and_endpoint():
    root = Path(__file__).resolve().parents[1] / "testdata" / "openrouter"
    client = FixtureClient(root)

    models = client.get("/api/v1/models")
    assert "data" in models
    ids = [m["id"] for m in models["data"]]
    assert "anthropic/claude-opus-4.6" in ids

    ep = client.get("/api/v1/models/anthropic/claude-opus-4.6/endpoints")
    assert ep["data"]["id"] == "anthropic/claude-opus-4.6"
    assert len(ep["data"]["endpoints"]) == 2
```

- [x] **Step 3: Run it to confirm failure**

Run: `cd pipeline && .venv/bin/pytest tests/test_http_fixture.py -q`
Expected: `ModuleNotFoundError: No module named 'ingest.http'`.

- [x] **Step 4: Write `pipeline/ingest/http.py`**

```python
"""HTTP client for OpenRouter: live mode (requests) + fixture mode (local files).

FixtureClient is used by tests and by `make ingest SHARD=openrouter FIXTURE=...`
for deterministic shard builds. LiveClient is used by the maintainer bootstrap
run and, in M3a+, by the daily CI pipeline.
"""

from __future__ import annotations

import json
import time
from pathlib import Path
from typing import Any

import requests


class LiveClient:
    """HTTPS client for openrouter.ai with simple retry."""

    BASE = "https://openrouter.ai"

    def __init__(self, timeout_s: float = 15.0, retries: int = 3) -> None:
        self.timeout_s = timeout_s
        self.retries = retries
        self.session = requests.Session()
        self.session.headers.update({
            "User-Agent": "sku-pipeline/0.0 (+https://github.com/sofq/sku)",
            "Accept": "application/json",
        })

    def get(self, path: str) -> dict[str, Any]:
        url = self.BASE + path
        last: Exception | None = None
        for attempt in range(self.retries):
            try:
                resp = self.session.get(url, timeout=self.timeout_s)
                resp.raise_for_status()
                return resp.json()
            except requests.RequestException as e:
                last = e
                time.sleep(0.5 * (2**attempt))
        raise RuntimeError(f"GET {url} failed after {self.retries} attempts: {last}")


class FixtureClient:
    """File-backed client used by tests and offline shard builds.

    Maps API paths to local files under `root`:
      /api/v1/models                              -> models.json
      /api/v1/models/{author}/{slug}/endpoints    -> endpoints/{author}__{slug}.json
    """

    def __init__(self, root: Path) -> None:
        self.root = Path(root)

    def get(self, path: str) -> dict[str, Any]:
        if path == "/api/v1/models":
            return self._load("models.json")
        prefix = "/api/v1/models/"
        suffix = "/endpoints"
        if path.startswith(prefix) and path.endswith(suffix):
            slug = path[len(prefix) : -len(suffix)]
            flat = slug.replace("/", "__")
            return self._load(f"endpoints/{flat}.json")
        raise KeyError(f"FixtureClient: no fixture for path {path!r}")

    def _load(self, rel: str) -> dict[str, Any]:
        with (self.root / rel).open() as fh:
            return json.load(fh)
```

- [x] **Step 5: Run the test to verify pass**

Run: `cd pipeline && .venv/bin/pytest tests/test_http_fixture.py -q`
Expected: `1 passed`.

- [x] **Step 6: Commit**

```bash
git add pipeline/ingest/__init__.py pipeline/ingest/http.py pipeline/tests/test_http_fixture.py
git commit -m "pipeline(ingest): add LiveClient + FixtureClient for OpenRouter API"
```

---

## Task 8: OpenRouter normalization + ingest module (TDD)

Produces one row dict per `(model, serving_provider)` pair **plus** one synthetic aggregated row per model with `provider='openrouter'`. Enforces the USD-only invariant. Emits NDJSON on stdout or to `--out`.

**Row schema** (dict keys — exactly what the packager consumes):

```
sku_id              str    e.g. "anthropic/claude-opus-4.6::aws-bedrock::default"
provider            str    the SERVING provider (not the model author)
service             str    always "llm" for openrouter
kind                str    "llm.text" | "llm.multimodal" (derive from modality)
resource_name       str    model author/slug, e.g. "anthropic/claude-opus-4.6"
region              str    "" (global)
region_normalized   str    "" (global)
terms               dict   canonicalized terms (commitment=on_demand, others "")
resource_attrs      dict   {context_length, max_output_tokens, modality, capabilities, quantization}
prices              list   [{dimension, tier, amount, unit}, ...]
health              dict|None   {uptime_30d, latency_p50_ms, latency_p95_ms, throughput_tokens_per_sec, observed_at}
is_aggregated       bool   True for the synthetic openrouter row
```

**Files:**
- Create: `pipeline/ingest/openrouter.py`
- Create: `pipeline/tests/test_openrouter_ingest.py`
- Create: `pipeline/testdata/golden/openrouter_rows.jsonl`

- [x] **Step 1: Write the failing test**

`pipeline/tests/test_openrouter_ingest.py`:

```python
import json
from pathlib import Path

import pytest

from ingest.http import FixtureClient
from ingest.openrouter import NonUSDError, ingest

ROOT = Path(__file__).resolve().parents[1]
FIX = ROOT / "testdata" / "openrouter"
GOLDEN = ROOT / "testdata" / "golden" / "openrouter_rows.jsonl"


def test_ingest_matches_golden_rows(monkeypatch):
    # Skip the non-USD fixture model entirely so this test exercises only the
    # USD models; the USD guard gets its own test below.
    client = FixtureClient(FIX)
    models = client.get("/api/v1/models")
    models["data"] = [m for m in models["data"] if m["id"] != "non-usd-model/some-provider"]

    # Shim the client so ingest sees the filtered model list but still fetches
    # endpoints from disk for the remaining models.
    real_get = client.get

    def fake_get(path):
        if path == "/api/v1/models":
            return models
        return real_get(path)

    client.get = fake_get  # type: ignore[assignment]

    rows = ingest(client, generated_at="2026-04-18T00:00:00Z")
    rows_sorted = sorted(rows, key=lambda r: r["sku_id"])

    expected = []
    with GOLDEN.open() as fh:
        for line in fh:
            line = line.strip()
            if line:
                expected.append(json.loads(line))
    expected_sorted = sorted(expected, key=lambda r: r["sku_id"])

    assert len(rows_sorted) == len(expected_sorted)
    for got, want in zip(rows_sorted, expected_sorted, strict=True):
        assert got == want, got["sku_id"]


def test_ingest_rejects_non_usd_endpoint():
    client = FixtureClient(FIX)
    # Limit to the non-USD fixture model so the guard fires first.
    models = client.get("/api/v1/models")
    models["data"] = [m for m in models["data"] if m["id"] == "non-usd-model/some-provider"]
    real_get = client.get

    def fake_get(path):
        if path == "/api/v1/models":
            return models
        return real_get(path)

    client.get = fake_get  # type: ignore[assignment]

    with pytest.raises(NonUSDError) as ei:
        ingest(client, generated_at="2026-04-18T00:00:00Z")
    assert "non-usd-model/some-provider" in str(ei.value)
    assert "some-provider" in str(ei.value)


def test_synthetic_aggregated_row_present():
    client = FixtureClient(FIX)
    models = client.get("/api/v1/models")
    models["data"] = [m for m in models["data"] if m["id"] == "openai/gpt-5"]
    real_get = client.get

    def fake_get(path):
        if path == "/api/v1/models":
            return models
        return real_get(path)

    client.get = fake_get  # type: ignore[assignment]

    rows = ingest(client, generated_at="2026-04-18T00:00:00Z")
    agg = [r for r in rows if r["is_aggregated"]]
    assert len(agg) == 1
    assert agg[0]["sku_id"] == "openai/gpt-5::openrouter::default"
    assert agg[0]["provider"] == "openrouter"
    assert agg[0]["health"] is None
```

- [x] **Step 2: Run it to confirm failure**

Run: `cd pipeline && .venv/bin/pytest tests/test_openrouter_ingest.py -q`
Expected: `ModuleNotFoundError: No module named 'ingest.openrouter'`.

- [x] **Step 3: Write `pipeline/ingest/openrouter.py`**

```python
"""Normalize OpenRouter's two endpoints into row dicts ready for shard packaging.

Spec §3 "OpenRouter-specific ingest": one row per (model, serving_provider)
pair plus one synthetic aggregated row per model with provider='openrouter'.
"""

from __future__ import annotations

import argparse
import json
import sys
import time
from pathlib import Path
from typing import Any, Iterable

from normalize.enums import apply_kind_defaults, load_enums
from normalize.terms import terms_hash

from .http import FixtureClient, LiveClient


class NonUSDError(RuntimeError):
    """Raised when an upstream endpoint declares a non-USD currency."""


def _kind_for_modality(modality: str | None, input_modalities: list[str] | None) -> str:
    """Map OpenRouter modality hints to the sku kind taxonomy."""
    mods = {m.lower() for m in (input_modalities or [])}
    if (modality or "").lower() == "text" and mods <= {"text"}:
        return "llm.text"
    # Any non-text input modality -> multimodal.
    return "llm.multimodal"


def _pricing_dimensions(pricing: dict[str, Any]) -> list[dict[str, Any]]:
    """OpenRouter pricing fields -> sku price rows.

    OpenRouter uses per-token unit prices (USD per token). We publish them as-is
    with unit='token' so the renderer can reason about dimensions uniformly.
    Zero-priced dimensions are omitted (a row with amount=0 carries no info).
    """
    out: list[dict[str, Any]] = []
    mapping = {
        "prompt": "prompt",
        "completion": "completion",
        "image": "image",
        "request": "request",
    }
    for src, dim in mapping.items():
        raw = pricing.get(src)
        if raw is None or raw == "":
            continue
        amount = float(raw)
        if amount == 0.0:
            continue
        unit = "request" if dim == "request" else "token"
        out.append({"dimension": dim, "tier": "", "amount": amount, "unit": unit})
    return out


def _build_row(
    *,
    model: dict[str, Any],
    endpoint: dict[str, Any] | None,
    aggregated: bool,
) -> dict[str, Any]:
    """Construct one row dict. `endpoint=None` + `aggregated=True` builds the
    synthetic `::openrouter::default` row from the model's top-level pricing."""
    author_slug = model["id"]  # e.g. "anthropic/claude-opus-4.6"
    arch = model.get("architecture") or {}
    modality = arch.get("modality")
    input_modalities = arch.get("input_modalities") or []
    kind = _kind_for_modality(modality, input_modalities)

    if aggregated:
        serving_provider = "openrouter"
        quantization = None
        pricing = model.get("pricing") or {}
        top = model.get("top_provider") or {}
        context_length = top.get("context_length") or model.get("context_length")
        max_completion = top.get("max_completion_tokens")
        health: dict[str, Any] | None = None
    else:
        assert endpoint is not None
        serving_provider = endpoint.get("tag") or endpoint.get("provider_name")
        if not serving_provider:
            raise ValueError(f"missing serving provider for {author_slug}")
        quantization = endpoint.get("quantization")
        pricing = endpoint.get("pricing") or {}
        context_length = endpoint.get("context_length")
        max_completion = endpoint.get("max_completion_tokens")
        latency = endpoint.get("latency") or {}
        health = {
            "uptime_30d": endpoint.get("uptime_last_30m"),
            "latency_p50_ms": latency.get("p50_ms"),
            "latency_p95_ms": latency.get("p95_ms"),
            "throughput_tokens_per_sec": endpoint.get("throughput_tokens_per_second"),
            "observed_at": int(time.time()),
        }

    # USD guard — spec §5 "OpenRouter currency guard"
    currency = (pricing.get("currency") or "USD").upper()
    if currency != "USD":
        raise NonUSDError(
            f"non-usd-endpoint: {author_slug}/{serving_provider} (currency={currency!r})"
        )

    quant_slug = quantization or "default"
    sku_id = f"{author_slug}::{serving_provider}::{quant_slug}"

    # Terms: on_demand with all other fields empty for LLMs.
    terms_raw = apply_kind_defaults(kind, {"commitment": "on_demand"})
    # (apply_kind_defaults fills the LLM defaults: tenancy="", os="", etc.)

    capabilities = model.get("supported_parameters") or []

    row: dict[str, Any] = {
        "sku_id": sku_id,
        "provider": serving_provider,
        "service": "llm",
        "kind": kind,
        "resource_name": author_slug,
        "region": "",
        "region_normalized": "",
        "terms": terms_raw,
        "terms_hash": terms_hash(terms_raw),
        "resource_attrs": {
            "context_length": context_length,
            "max_output_tokens": max_completion,
            "modality": input_modalities,
            "capabilities": capabilities,
            "quantization": quantization,
        },
        "prices": _pricing_dimensions(pricing),
        "health": health,
        "is_aggregated": aggregated,
    }
    return row


def ingest(client: LiveClient | FixtureClient, *, generated_at: str) -> list[dict[str, Any]]:
    """Normalize everything OpenRouter exposes into row dicts.

    Errors out on the first non-USD endpoint; callers decide whether to retry
    with a filtered model list or fail the release (spec §5 guard).
    """
    enums = load_enums()  # side-effect: validates the YAML is parseable
    _ = enums  # suppress unused in M1; real enum validation lands as rows are built
    _ = generated_at  # threaded through for future metadata population

    models_payload = client.get("/api/v1/models")
    rows: list[dict[str, Any]] = []
    for model in models_payload.get("data", []):
        # Fetch per-model endpoint detail.
        slug = model["id"]
        ep_payload = client.get(f"/api/v1/models/{slug}/endpoints")
        endpoints = (ep_payload.get("data") or {}).get("endpoints") or []
        if not endpoints:
            # OpenRouter sometimes lists a model with no concrete endpoints
            # (deprecated or region-restricted). Skip — the aggregated row
            # would be a lie without backing endpoints.
            continue
        for ep in endpoints:
            rows.append(_build_row(model=model, endpoint=ep, aggregated=False))
        rows.append(_build_row(model=model, endpoint=None, aggregated=True))
    return rows


def _write_rows(rows: Iterable[dict[str, Any]], out: Path | None) -> None:
    def encode(r: dict[str, Any]) -> str:
        return json.dumps(r, separators=(",", ":"), sort_keys=True, ensure_ascii=False)

    if out is None:
        for r in rows:
            sys.stdout.write(encode(r) + "\n")
    else:
        out.parent.mkdir(parents=True, exist_ok=True)
        with out.open("w") as fh:
            for r in rows:
                fh.write(encode(r) + "\n")


def main() -> int:
    ap = argparse.ArgumentParser(prog="ingest.openrouter")
    ap.add_argument("--out", type=Path, help="write NDJSON rows here (stdout if omitted)")
    ap.add_argument("--fixture", type=Path, help="use FixtureClient rooted at this directory")
    ap.add_argument("--generated-at", default="", help="ISO-8601 UTC; default now")
    args = ap.parse_args()

    generated_at = args.generated_at or time.strftime("%Y-%m-%dT%H:%M:%SZ", time.gmtime())
    if args.fixture:
        client: LiveClient | FixtureClient = FixtureClient(args.fixture)
    else:
        client = LiveClient()

    try:
        rows = ingest(client, generated_at=generated_at)
    except NonUSDError as e:
        sys.stderr.write(f"ingest.openrouter: {e}\n")
        return 4

    _write_rows(rows, args.out)
    return 0


if __name__ == "__main__":
    sys.exit(main())
```

- [x] **Step 4: Generate the golden rows file**

Run (from `pipeline/`, venv active):

```bash
cd pipeline && .venv/bin/python -m ingest.openrouter \
  --fixture testdata/openrouter \
  --generated-at 2026-04-18T00:00:00Z 2>/dev/null | \
  python - <<'PY' > testdata/golden/openrouter_rows.jsonl
import json, sys
# Drop the non-USD model rows: ingest would have exited 4, but running
# without the USD guard isn't possible here, so we skip.
PY
```

Actually simpler — temporarily filter the fixture input by copying `models.json` to a filtered variant that omits the non-USD entry, then run ingest, then restore:

```bash
cd pipeline
mkdir -p testdata/golden
.venv/bin/python - <<'PY' > testdata/golden/openrouter_rows.jsonl
import json, sys
from pathlib import Path
from ingest.http import FixtureClient
from ingest.openrouter import ingest

root = Path("testdata/openrouter")
client = FixtureClient(root)
models = client.get("/api/v1/models")
models["data"] = [m for m in models["data"] if m["id"] != "non-usd-model/some-provider"]
real_get = client.get
def fake_get(p):
    if p == "/api/v1/models":
        return models
    return real_get(p)
client.get = fake_get
rows = ingest(client, generated_at="2026-04-18T00:00:00Z")
for r in sorted(rows, key=lambda r: r["sku_id"]):
    # observed_at is wall-clock — stamp it deterministically for the golden
    if r.get("health"):
        r["health"]["observed_at"] = 1745020800  # 2026-04-19 00:00:00 UTC
    sys.stdout.write(json.dumps(r, separators=(",",":"), sort_keys=True, ensure_ascii=False) + "\n")
PY
```

Then update `ingest.openrouter._build_row` to honor a `SKU_FIXED_OBSERVED_AT` env var when present, so the test run matches:

Edit `_build_row` in `pipeline/ingest/openrouter.py`:

```python
# Replace this line:
#     "observed_at": int(time.time()),
# With:
import os  # (already-imported at module top if not already there)
_fixed = os.environ.get("SKU_FIXED_OBSERVED_AT")
observed_at = int(_fixed) if _fixed else int(time.time())
# ... and set "observed_at": observed_at
```

Update the test to set `monkeypatch.setenv("SKU_FIXED_OBSERVED_AT", "1745020800")` at the top of the two tests that inspect row contents.

- [x] **Step 5: Run the tests to verify pass**

Run: `cd pipeline && .venv/bin/pytest tests/test_openrouter_ingest.py -q`
Expected: `3 passed`.

- [x] **Step 6: Commit**

```bash
git add pipeline/ingest/openrouter.py pipeline/tests/test_openrouter_ingest.py pipeline/testdata/golden/openrouter_rows.jsonl
git commit -m "pipeline(ingest): add OpenRouter normalization with USD guard + golden"
```

---

## Task 9: Shard schema SQL + packager (TDD)

Turns a NDJSON rows file into a SQLite shard that matches §5 exactly (tables, indexes, metadata).

**Files:**
- Create: `pipeline/package/__init__.py`
- Create: `pipeline/package/schema.sql`
- Create: `pipeline/package/build_shard.py`
- Create: `pipeline/tests/test_build_shard.py`

- [x] **Step 1: Write `pipeline/package/__init__.py`** (empty)

- [x] **Step 2: Write `pipeline/package/schema.sql`** — copy verbatim from §5

```sql
-- sku shard schema v1. Spec §5. Keep in sync across ingest + client.

PRAGMA foreign_keys = ON;

CREATE TABLE skus (
  sku_id             TEXT    NOT NULL PRIMARY KEY,
  provider           TEXT    NOT NULL,
  service            TEXT    NOT NULL,
  kind               TEXT    NOT NULL,
  resource_name      TEXT    NOT NULL,
  region             TEXT    NOT NULL,
  region_normalized  TEXT    NOT NULL,
  terms_hash         TEXT    NOT NULL
) WITHOUT ROWID;

CREATE TABLE resource_attrs (
  sku_id             TEXT    NOT NULL PRIMARY KEY REFERENCES skus(sku_id) ON DELETE CASCADE,
  vcpu               INTEGER,
  memory_gb          REAL,
  storage_gb         REAL,
  gpu_count          INTEGER,
  gpu_model          TEXT,
  architecture       TEXT,
  context_length     INTEGER,
  max_output_tokens  INTEGER,
  modality           TEXT,
  capabilities       TEXT,
  quantization       TEXT,
  durability_nines   INTEGER,
  availability_tier  TEXT,
  extra              TEXT
) WITHOUT ROWID;

CREATE TABLE terms (
  sku_id             TEXT    NOT NULL PRIMARY KEY REFERENCES skus(sku_id) ON DELETE CASCADE,
  commitment         TEXT    NOT NULL,
  tenancy            TEXT    NOT NULL DEFAULT '',
  os                 TEXT    NOT NULL DEFAULT '',
  support_tier       TEXT,
  upfront            TEXT,
  payment_option     TEXT
) WITHOUT ROWID;

CREATE TABLE prices (
  sku_id     TEXT NOT NULL REFERENCES skus(sku_id) ON DELETE CASCADE,
  dimension  TEXT NOT NULL,
  tier       TEXT NOT NULL DEFAULT '',
  amount     REAL NOT NULL,
  unit       TEXT NOT NULL,
  PRIMARY KEY (sku_id, dimension, tier)
) WITHOUT ROWID;

CREATE TABLE health (
  sku_id                     TEXT    NOT NULL PRIMARY KEY REFERENCES skus(sku_id) ON DELETE CASCADE,
  uptime_30d                 REAL,
  latency_p50_ms             INTEGER,
  latency_p95_ms             INTEGER,
  throughput_tokens_per_sec  REAL,
  observed_at                INTEGER
) WITHOUT ROWID;

CREATE TABLE metadata (
  key    TEXT PRIMARY KEY,
  value  TEXT
);

CREATE INDEX idx_skus_lookup
  ON skus (resource_name, region, terms_hash);
CREATE INDEX idx_resource_compute
  ON resource_attrs (vcpu, memory_gb) WHERE vcpu IS NOT NULL;
CREATE INDEX idx_resource_llm
  ON resource_attrs (context_length) WHERE context_length IS NOT NULL;
CREATE INDEX idx_skus_region
  ON skus (region_normalized, kind);
CREATE INDEX idx_prices_by_dim
  ON prices (dimension, amount);
CREATE INDEX idx_terms_commitment
  ON terms (commitment, tenancy, os);
```

- [x] **Step 3: Write the failing test**

`pipeline/tests/test_build_shard.py`:

```python
import json
import sqlite3
from pathlib import Path

from package.build_shard import build_shard

FIX_ROWS = [
    {
        "sku_id": "anthropic/claude-opus-4.6::anthropic::default",
        "provider": "anthropic",
        "service": "llm",
        "kind": "llm.text",
        "resource_name": "anthropic/claude-opus-4.6",
        "region": "",
        "region_normalized": "",
        "terms": {"commitment": "on_demand", "tenancy": "", "os": "",
                  "support_tier": "", "upfront": "", "payment_option": ""},
        "terms_hash": "ee2303ad38b3e0b0e4f01bfbb1bcba8f",
        "resource_attrs": {
            "context_length": 200000, "max_output_tokens": 64000,
            "modality": ["text"], "capabilities": ["tools"],
            "quantization": None,
        },
        "prices": [
            {"dimension": "prompt", "tier": "", "amount": 1.5e-5, "unit": "token"},
            {"dimension": "completion", "tier": "", "amount": 7.5e-5, "unit": "token"},
        ],
        "health": {"uptime_30d": 0.998, "latency_p50_ms": 420,
                   "latency_p95_ms": 1100, "throughput_tokens_per_sec": 62.5,
                   "observed_at": 1745020800},
        "is_aggregated": False,
    },
]


def write_rows(tmp_path: Path, rows: list[dict]) -> Path:
    p = tmp_path / "rows.jsonl"
    with p.open("w") as fh:
        for r in rows:
            fh.write(json.dumps(r) + "\n")
    return p


def test_build_shard_populates_all_tables(tmp_path: Path):
    rows_path = write_rows(tmp_path, FIX_ROWS)
    out = tmp_path / "openrouter.db"
    build_shard(
        rows_path=rows_path,
        shard="openrouter",
        out_path=out,
        catalog_version="2026.04.18",
        generated_at="2026-04-18T00:00:00Z",
        source_url="https://openrouter.ai/api/v1/models",
    )

    con = sqlite3.connect(out)
    try:
        con.execute("PRAGMA foreign_keys = ON")
        assert con.execute("SELECT count(*) FROM skus").fetchone()[0] == 1
        assert con.execute("SELECT count(*) FROM resource_attrs").fetchone()[0] == 1
        assert con.execute("SELECT count(*) FROM terms").fetchone()[0] == 1
        assert con.execute("SELECT count(*) FROM prices").fetchone()[0] == 2
        assert con.execute("SELECT count(*) FROM health").fetchone()[0] == 1

        meta = dict(con.execute("SELECT key, value FROM metadata").fetchall())
        assert meta["schema_version"] == "1"
        assert meta["catalog_version"] == "2026.04.18"
        assert meta["currency"] == "USD"
        assert meta["generated_at"] == "2026-04-18T00:00:00Z"
        assert meta["row_count"] == "1"
        assert json.loads(meta["allowed_kinds"]) == ["llm.text"]

        # Index presence
        idx = {r[0] for r in con.execute(
            "SELECT name FROM sqlite_master WHERE type='index' AND name LIKE 'idx_%'"
        ).fetchall()}
        for want in ("idx_skus_lookup", "idx_resource_llm", "idx_skus_region",
                     "idx_prices_by_dim", "idx_terms_commitment"):
            assert want in idx, f"missing index {want}: got {idx}"

        # FK cascade
        con.execute("DELETE FROM skus WHERE sku_id=?",
                    ("anthropic/claude-opus-4.6::anthropic::default",))
        con.commit()
        assert con.execute("SELECT count(*) FROM prices").fetchone()[0] == 0
        assert con.execute("SELECT count(*) FROM terms").fetchone()[0] == 0
    finally:
        con.close()


def test_build_shard_seeds_metadata_from_rows(tmp_path: Path):
    rows = list(FIX_ROWS)
    # Add a multimodal row so allowed_kinds has two entries
    rows.append({**FIX_ROWS[0],
                 "sku_id": "openai/gpt-5::openai::default",
                 "resource_name": "openai/gpt-5",
                 "provider": "openai",
                 "kind": "llm.multimodal"})
    rows_path = write_rows(tmp_path, rows)
    out = tmp_path / "openrouter.db"
    build_shard(rows_path=rows_path, shard="openrouter", out_path=out,
                catalog_version="2026.04.18", generated_at="x",
                source_url="y")

    con = sqlite3.connect(out)
    try:
        meta = dict(con.execute("SELECT key, value FROM metadata").fetchall())
        assert sorted(json.loads(meta["allowed_kinds"])) == ["llm.multimodal", "llm.text"]
        assert meta["row_count"] == "2"
        assert "serving_providers" in meta
        assert sorted(json.loads(meta["serving_providers"])) == ["anthropic", "openai"]
    finally:
        con.close()
```

- [x] **Step 4: Run it to confirm failure**

Run: `cd pipeline && .venv/bin/pytest tests/test_build_shard.py -q`
Expected: `ModuleNotFoundError: No module named 'package.build_shard'`.

- [x] **Step 5: Write `pipeline/package/build_shard.py`**

```python
"""Build a SQLite shard from normalized NDJSON rows. Spec §5."""

from __future__ import annotations

import argparse
import json
import sqlite3
import sys
from pathlib import Path
from typing import Any, Iterable

_HERE = Path(__file__).resolve().parent
_SCHEMA_SQL = (_HERE / "schema.sql").read_text()


def _iter_rows(path: Path) -> Iterable[dict[str, Any]]:
    with path.open() as fh:
        for line in fh:
            line = line.strip()
            if line:
                yield json.loads(line)


def _insert_row(con: sqlite3.Connection, row: dict[str, Any]) -> None:
    sku_id = row["sku_id"]
    con.execute(
        "INSERT INTO skus(sku_id, provider, service, kind, resource_name, "
        "region, region_normalized, terms_hash) VALUES(?,?,?,?,?,?,?,?)",
        (sku_id, row["provider"], row["service"], row["kind"],
         row["resource_name"], row["region"], row["region_normalized"],
         row["terms_hash"]),
    )

    attrs = row.get("resource_attrs") or {}
    con.execute(
        "INSERT INTO resource_attrs(sku_id, vcpu, memory_gb, storage_gb, "
        "gpu_count, gpu_model, architecture, context_length, max_output_tokens, "
        "modality, capabilities, quantization, durability_nines, "
        "availability_tier, extra) VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)",
        (
            sku_id,
            attrs.get("vcpu"), attrs.get("memory_gb"), attrs.get("storage_gb"),
            attrs.get("gpu_count"), attrs.get("gpu_model"),
            attrs.get("architecture"),
            attrs.get("context_length"), attrs.get("max_output_tokens"),
            json.dumps(attrs["modality"]) if attrs.get("modality") is not None else None,
            json.dumps(attrs["capabilities"]) if attrs.get("capabilities") is not None else None,
            attrs.get("quantization"),
            attrs.get("durability_nines"), attrs.get("availability_tier"),
            json.dumps(attrs["extra"]) if attrs.get("extra") is not None else None,
        ),
    )

    t = row["terms"]
    con.execute(
        "INSERT INTO terms(sku_id, commitment, tenancy, os, support_tier, "
        "upfront, payment_option) VALUES(?,?,?,?,?,?,?)",
        (sku_id, t["commitment"], t.get("tenancy", ""), t.get("os", ""),
         t.get("support_tier") or None, t.get("upfront") or None,
         t.get("payment_option") or None),
    )

    for p in row.get("prices") or []:
        con.execute(
            "INSERT INTO prices(sku_id, dimension, tier, amount, unit) "
            "VALUES(?,?,?,?,?)",
            (sku_id, p["dimension"], p.get("tier", ""), p["amount"], p["unit"]),
        )

    h = row.get("health")
    if h:
        con.execute(
            "INSERT INTO health(sku_id, uptime_30d, latency_p50_ms, "
            "latency_p95_ms, throughput_tokens_per_sec, observed_at) "
            "VALUES(?,?,?,?,?,?)",
            (sku_id, h.get("uptime_30d"), h.get("latency_p50_ms"),
             h.get("latency_p95_ms"), h.get("throughput_tokens_per_sec"),
             h.get("observed_at")),
        )


def build_shard(
    *,
    rows_path: Path,
    shard: str,
    out_path: Path,
    catalog_version: str,
    generated_at: str,
    source_url: str,
) -> None:
    """Rebuild out_path from scratch based on rows_path."""
    if out_path.exists():
        out_path.unlink()
    out_path.parent.mkdir(parents=True, exist_ok=True)

    con = sqlite3.connect(out_path)
    try:
        con.executescript(_SCHEMA_SQL)
        con.execute("PRAGMA foreign_keys = ON")

        rows = list(_iter_rows(rows_path))
        kinds: set[str] = set()
        commitments: set[str] = set()
        tenancies: set[str] = set()
        oses: set[str] = set()
        providers: set[str] = set()

        con.execute("BEGIN")
        for row in rows:
            _insert_row(con, row)
            kinds.add(row["kind"])
            commitments.add(row["terms"]["commitment"])
            tenancies.add(row["terms"].get("tenancy", ""))
            oses.add(row["terms"].get("os", ""))
            providers.add(row["provider"])
        con.execute("COMMIT")

        meta = {
            "schema_version": "1",
            "catalog_version": catalog_version,
            "currency": "USD",
            "generated_at": generated_at,
            "source_url": source_url,
            "row_count": str(len(rows)),
            "allowed_kinds": json.dumps(sorted(kinds)),
            "allowed_commitments": json.dumps(sorted(commitments)),
            "allowed_tenancies": json.dumps(sorted(tenancies)),
            "allowed_oses": json.dumps(sorted(oses)),
            "serving_providers": json.dumps(sorted(providers)),
            "shard": shard,
            "head_version": catalog_version,
        }
        con.executemany("INSERT INTO metadata(key, value) VALUES(?,?)", meta.items())
        con.commit()
        # Clean up WAL artefacts from the single-transaction write.
        con.execute("PRAGMA wal_checkpoint(TRUNCATE)")
        con.execute("VACUUM")
    finally:
        con.close()


def main() -> int:
    ap = argparse.ArgumentParser(prog="package.build_shard")
    ap.add_argument("--rows", type=Path, required=True)
    ap.add_argument("--shard", required=True)
    ap.add_argument("--out", type=Path, required=True)
    ap.add_argument("--catalog-version", default="dev")
    ap.add_argument("--generated-at", default="")
    ap.add_argument("--source-url", default="https://openrouter.ai/api/v1/models")
    args = ap.parse_args()

    import time
    generated_at = args.generated_at or time.strftime("%Y-%m-%dT%H:%M:%SZ", time.gmtime())

    build_shard(
        rows_path=args.rows,
        shard=args.shard,
        out_path=args.out,
        catalog_version=args.catalog_version,
        generated_at=generated_at,
        source_url=args.source_url,
    )
    return 0


if __name__ == "__main__":
    sys.exit(main())
```

- [x] **Step 6: Run test to verify pass**

Run: `cd pipeline && .venv/bin/pytest tests/test_build_shard.py -q`
Expected: `2 passed`.

- [x] **Step 7: Full pipeline smoke — fixture → rows → shard**

```bash
cd pipeline
SKU_FIXED_OBSERVED_AT=1745020800 .venv/bin/python -m ingest.openrouter \
  --fixture testdata/openrouter \
  --out ../dist/pipeline/openrouter.rows.jsonl \
  --generated-at 2026-04-18T00:00:00Z || true   # exits 4 on non-usd; expected
# Re-run with filter for smoke: skip non-usd model
.venv/bin/python - <<'PY'
import json
from pathlib import Path
src = Path("../dist/pipeline/openrouter.rows.jsonl")
# Not written because ingest exited 4. Build an allowed-only rows file via Python:
from ingest.http import FixtureClient
from ingest.openrouter import ingest
c = FixtureClient(Path("testdata/openrouter"))
m = c.get("/api/v1/models")
m["data"] = [x for x in m["data"] if x["id"] != "non-usd-model/some-provider"]
real = c.get
c.get = lambda p, real=real, m=m: m if p == "/api/v1/models" else real(p)
rows = ingest(c, generated_at="2026-04-18T00:00:00Z")
src.parent.mkdir(parents=True, exist_ok=True)
with src.open("w") as fh:
    for r in rows:
        fh.write(json.dumps(r) + "\n")
PY
.venv/bin/python -m package.build_shard \
  --rows ../dist/pipeline/openrouter.rows.jsonl \
  --shard openrouter \
  --out ../dist/pipeline/openrouter.db \
  --catalog-version 2026.04.18 \
  --generated-at 2026-04-18T00:00:00Z
ls -la ../dist/pipeline/openrouter.db
sqlite3 ../dist/pipeline/openrouter.db "SELECT count(*) FROM skus; SELECT value FROM metadata WHERE key='allowed_kinds';"
```

Expected: file exists; ~4 rows (2 endpoints for Claude + aggregated; 1 endpoint for GPT-5 + aggregated); `allowed_kinds` = `["llm.multimodal","llm.text"]`.

- [x] **Step 8: Commit**

```bash
git add pipeline/package/
git add pipeline/tests/test_build_shard.py
git commit -m "pipeline(package): add shard packager matching spec §5 schema"
```

---

## Task 10: Root Makefile wiring + `make openrouter-shard`

**Files:**
- Modify: `Makefile`

- [x] **Step 1: Add pipeline + bench targets to the root Makefile**

Append before the existing `release-dry` target so the file stays grouped:

```make
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
```

- [x] **Step 2: Adjust the help-awk filter** (only if targets are hidden) — confirm new targets show up:

Run: `make help`
Expected: lines for `openrouter-shard`, `pipeline-test`, `bench`, `test-integration`.

- [x] **Step 3: Commit**

```bash
git add Makefile
git commit -m "build: wire pipeline + bench + integration targets in root Makefile"
```

---

## Task 11: `internal/errors` — envelope + exit code mapper (TDD)

Implements the spec §4 stderr error envelope and the exit-code taxonomy. This replaces the M0 plain-text stderr error in `cmd/sku/execute.go`.

**Files:**
- Create: `internal/errors/errors.go`, `internal/errors/errors_test.go`

- [x] **Step 1: Write the failing test**

`internal/errors/errors_test.go`:

```go
package errors_test

import (
	"bytes"
	"encoding/json"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"

	skuerrors "github.com/sofq/sku/internal/errors"
)

func TestWrite_ShapeMatchesSpec(t *testing.T) {
	var buf bytes.Buffer
	err := &skuerrors.E{
		Code:       skuerrors.CodeNotFound,
		Message:    "No SKU matches filters",
		Suggestion: "Try `sku schema openrouter llm` to see valid filters",
		Details: map[string]any{
			"provider": "openrouter",
			"service":  "llm",
			"applied_filters": map[string]any{
				"model": "anthropic/nope",
			},
		},
	}
	got := skuerrors.Write(&buf, err)
	require.Equal(t, 3, got, "not_found -> exit 3")

	var env map[string]any
	require.NoError(t, json.Unmarshal(buf.Bytes(), &env))
	body, ok := env["error"].(map[string]any)
	require.True(t, ok)
	require.Equal(t, "not_found", body["code"])
	require.Equal(t, "No SKU matches filters", body["message"])
	require.Contains(t, body, "suggestion")
	require.Contains(t, body, "details")
	require.Equal(t, byte('\n'), buf.Bytes()[buf.Len()-1])
}

func TestWrite_UnknownErrorMapsToGeneric(t *testing.T) {
	var buf bytes.Buffer
	got := skuerrors.Write(&buf, errors.New("boom"))
	require.Equal(t, 1, got)

	var env map[string]any
	require.NoError(t, json.Unmarshal(buf.Bytes(), &env))
	body := env["error"].(map[string]any)
	require.Equal(t, "generic_error", body["code"])
	require.Equal(t, "boom", body["message"])
}

func TestCode_ExitMapping(t *testing.T) {
	cases := map[skuerrors.Code]int{
		skuerrors.CodeOK:           0,
		skuerrors.CodeGeneric:      1,
		skuerrors.CodeAuth:         2,
		skuerrors.CodeNotFound:     3,
		skuerrors.CodeValidation:   4,
		skuerrors.CodeRateLimited:  5,
		skuerrors.CodeConflict:     6,
		skuerrors.CodeServer:       7,
		skuerrors.CodeStaleData:    8,
	}
	for c, want := range cases {
		require.Equal(t, want, c.ExitCode(), "code %s", c)
	}
}

func TestWrite_Nil_Returns0(t *testing.T) {
	var buf bytes.Buffer
	require.Equal(t, 0, skuerrors.Write(&buf, nil))
	require.Zero(t, buf.Len())
}
```

- [x] **Step 2: Run to confirm failure**

Run: `go test ./internal/errors/...`
Expected: `undefined: skuerrors.E`, etc.

- [x] **Step 3: Write `internal/errors/errors.go`**

```go
// Package errors implements the spec §4 error envelope + exit-code taxonomy.
//
// Every non-zero exit path funnels through Write, which either unwraps an *E
// (a structured sku error) or boxes a plain Go error as a generic_error
// envelope. The exit code taxonomy is stable — agents depend on it per spec §4.
package errors

import (
	"encoding/json"
	"errors"
	"io"
)

// Code is the stable error-code enum emitted in the envelope's error.code and
// mapped to a process exit code via ExitCode.
type Code string

const (
	CodeOK          Code = ""              // exit 0
	CodeGeneric     Code = "generic_error" // exit 1
	CodeAuth        Code = "auth"          // exit 2
	CodeNotFound    Code = "not_found"     // exit 3
	CodeValidation  Code = "validation"    // exit 4
	CodeRateLimited Code = "rate_limited"  // exit 5
	CodeConflict    Code = "conflict"      // exit 6
	CodeServer      Code = "server"        // exit 7
	CodeStaleData   Code = "stale_data"    // exit 8
)

// ExitCode returns the process exit code for this error code.
func (c Code) ExitCode() int {
	switch c {
	case CodeOK:
		return 0
	case CodeAuth:
		return 2
	case CodeNotFound:
		return 3
	case CodeValidation:
		return 4
	case CodeRateLimited:
		return 5
	case CodeConflict:
		return 6
	case CodeServer:
		return 7
	case CodeStaleData:
		return 8
	default:
		return 1
	}
}

// E is the canonical structured sku error. It implements the error interface
// and renders as the §4 JSON envelope when passed to Write.
type E struct {
	Code       Code           `json:"code"`
	Message    string         `json:"message"`
	Suggestion string         `json:"suggestion,omitempty"`
	Details    map[string]any `json:"details,omitempty"`
}

// Error implements error.
func (e *E) Error() string { return e.Message }

// envelope is the outer wrapper per §4.
type envelope struct {
	Error *E `json:"error"`
}

// Write marshals err to the §4 JSON envelope on w and returns the exit code
// the process should use. A nil err writes nothing and returns 0. Any non-*E
// error is boxed as CodeGeneric.
func Write(w io.Writer, err error) int {
	if err == nil {
		return 0
	}

	var e *E
	if !errors.As(err, &e) {
		e = &E{Code: CodeGeneric, Message: err.Error()}
	}
	enc := json.NewEncoder(w)
	// json.Encoder writes compact JSON + trailing \n, matching §4 stderr shape.
	if marshalErr := enc.Encode(envelope{Error: e}); marshalErr != nil {
		// Fallback: a minimal hand-rolled envelope. Should never trip in practice
		// because map[string]any with primitive values always marshals.
		_, _ = io.WriteString(w,
			`{"error":{"code":"generic_error","message":"error envelope marshal failed"}}`+"\n",
		)
	}
	return e.Code.ExitCode()
}

// NotFound builds an E with CodeNotFound and the common details shape (§4).
func NotFound(provider, service string, appliedFilters map[string]any, suggestion string) *E {
	return &E{
		Code:       CodeNotFound,
		Message:    "No SKU matches filters",
		Suggestion: suggestion,
		Details: map[string]any{
			"provider":        provider,
			"service":         service,
			"applied_filters": appliedFilters,
		},
	}
}

// Validation builds an E with CodeValidation and the common details shape (§4).
func Validation(reason, flag, value string, hint string) *E {
	d := map[string]any{"reason": reason}
	if flag != "" {
		d["flag"] = flag
	}
	if value != "" {
		d["value"] = value
	}
	if hint != "" {
		d["hint"] = hint
	}
	return &E{
		Code:       CodeValidation,
		Message:    "Invalid input",
		Suggestion: hint,
		Details:    d,
	}
}
```

- [x] **Step 4: Run tests to verify pass**

Run: `go test ./internal/errors/... -v`
Expected: all four tests PASS.

- [x] **Step 5: Commit**

```bash
git add internal/errors/
git commit -m "feat(errors): add spec §4 error envelope + exit-code taxonomy"
```

---

## Task 12: Rewire `cmd/sku/execute.go` and `main.go` to use the envelope

Folds in the M0 code-review suggestions: `Execute()` returns `int`, `main.go` calls `os.Exit(sku.Execute())`, errors write the JSON envelope.

**Files:**
- Modify: `cmd/sku/execute.go`, `main.go`
- Modify: `cmd/sku/version_test.go` (if it depended on the old signature)

- [x] **Step 1: Read the current `cmd/sku/execute.go`**

Run: `cat cmd/sku/execute.go`

- [x] **Step 2: Replace with the envelope-aware version**

```go
package sku

import (
	"os"

	skuerrors "github.com/sofq/sku/internal/errors"
)

// Execute runs the root Cobra tree and returns the process exit code, writing
// any error to stderr as the spec §4 JSON envelope. Returning int (rather than
// calling os.Exit internally) lets Execute be covered by unit tests and keeps
// the exit-code taxonomy in one place — the skuerrors package.
//
// newRootCmd intentionally stays unexported: future milestones (M2 batch
// registry) populate the command registry from init() side-effects on leaves
// registered by NewCommand-style constructors, not by walking the Cobra tree
// externally. Keeping newRootCmd private prevents callers from reaching into
// Cobra internals and accidentally depending on traversal order.
func Execute() int {
	if err := newRootCmd().Execute(); err != nil {
		return skuerrors.Write(os.Stderr, err)
	}
	return 0
}
```

- [x] **Step 3: Update `main.go`**

```go
package main

import (
	"os"

	"github.com/sofq/sku/cmd/sku"
)

func main() {
	os.Exit(sku.Execute())
}
```

- [x] **Step 4: Build + smoke-test**

Run:
```bash
make build
./bin/sku version
./bin/sku notarealsubcmd; echo "exit=$?"
```

Expected:
- `sku version` emits JSON as before.
- `sku notarealsubcmd` prints a JSON envelope to stderr (Cobra's "unknown command" error, wrapped as `generic_error`) and exits 1.

- [x] **Step 5: Run all tests**

Run: `make test`
Expected: PASS.

- [x] **Step 6: Commit**

```bash
git add cmd/sku/execute.go main.go
git commit -m "refactor(cmd): Execute returns int and writes spec §4 error envelope"
```

---

## Task 13: `internal/catalog` — Open + LookupLLM (TDD)

The catalog reader. Opens a shard in WAL mode, reads the metadata, checks schema compatibility, and answers point-lookup queries joining the core tables.

**Files:**
- Create: `internal/catalog/catalog.go`, `internal/catalog/lookup.go`, `internal/catalog/datadir.go`
- Create: `internal/catalog/testdata/seed.sql`
- Create: `internal/catalog/catalog_test.go`

- [x] **Step 1: Write the seed fixture**

`internal/catalog/testdata/seed.sql`:

```sql
PRAGMA foreign_keys = ON;

CREATE TABLE skus (
  sku_id TEXT NOT NULL PRIMARY KEY,
  provider TEXT NOT NULL, service TEXT NOT NULL, kind TEXT NOT NULL,
  resource_name TEXT NOT NULL, region TEXT NOT NULL,
  region_normalized TEXT NOT NULL, terms_hash TEXT NOT NULL
) WITHOUT ROWID;

CREATE TABLE resource_attrs (
  sku_id TEXT NOT NULL PRIMARY KEY REFERENCES skus(sku_id) ON DELETE CASCADE,
  vcpu INTEGER, memory_gb REAL, storage_gb REAL,
  gpu_count INTEGER, gpu_model TEXT, architecture TEXT,
  context_length INTEGER, max_output_tokens INTEGER,
  modality TEXT, capabilities TEXT, quantization TEXT,
  durability_nines INTEGER, availability_tier TEXT, extra TEXT
) WITHOUT ROWID;

CREATE TABLE terms (
  sku_id TEXT NOT NULL PRIMARY KEY REFERENCES skus(sku_id) ON DELETE CASCADE,
  commitment TEXT NOT NULL, tenancy TEXT NOT NULL DEFAULT '',
  os TEXT NOT NULL DEFAULT '',
  support_tier TEXT, upfront TEXT, payment_option TEXT
) WITHOUT ROWID;

CREATE TABLE prices (
  sku_id TEXT NOT NULL REFERENCES skus(sku_id) ON DELETE CASCADE,
  dimension TEXT NOT NULL, tier TEXT NOT NULL DEFAULT '',
  amount REAL NOT NULL, unit TEXT NOT NULL,
  PRIMARY KEY (sku_id, dimension, tier)
) WITHOUT ROWID;

CREATE TABLE health (
  sku_id TEXT NOT NULL PRIMARY KEY REFERENCES skus(sku_id) ON DELETE CASCADE,
  uptime_30d REAL, latency_p50_ms INTEGER, latency_p95_ms INTEGER,
  throughput_tokens_per_sec REAL, observed_at INTEGER
) WITHOUT ROWID;

CREATE TABLE metadata (key TEXT PRIMARY KEY, value TEXT);

INSERT INTO metadata VALUES
  ('schema_version','1'),
  ('catalog_version','2026.04.18'),
  ('currency','USD'),
  ('generated_at','2026-04-18T00:00:00Z'),
  ('source_url','https://openrouter.ai/api/v1/models'),
  ('row_count','3'),
  ('allowed_kinds','["llm.text"]'),
  ('serving_providers','["anthropic","aws-bedrock","openrouter"]');

-- Three rows: Anthropic first-party, AWS Bedrock, and the aggregated openrouter row
INSERT INTO skus VALUES
  ('anthropic/claude-opus-4.6::anthropic::default','anthropic','llm','llm.text','anthropic/claude-opus-4.6','','','ee2303ad38b3e0b0e4f01bfbb1bcba8f'),
  ('anthropic/claude-opus-4.6::aws-bedrock::default','aws-bedrock','llm','llm.text','anthropic/claude-opus-4.6','','','ee2303ad38b3e0b0e4f01bfbb1bcba8f'),
  ('anthropic/claude-opus-4.6::openrouter::default','openrouter','llm','llm.text','anthropic/claude-opus-4.6','','','ee2303ad38b3e0b0e4f01bfbb1bcba8f');

INSERT INTO resource_attrs (sku_id, context_length, max_output_tokens, modality, capabilities, quantization) VALUES
  ('anthropic/claude-opus-4.6::anthropic::default', 200000, 64000, '["text"]', '["tools"]', NULL),
  ('anthropic/claude-opus-4.6::aws-bedrock::default', 200000, 64000, '["text"]', '["tools"]', NULL),
  ('anthropic/claude-opus-4.6::openrouter::default', 200000, 64000, '["text"]', '["tools"]', NULL);

INSERT INTO terms (sku_id, commitment) VALUES
  ('anthropic/claude-opus-4.6::anthropic::default','on_demand'),
  ('anthropic/claude-opus-4.6::aws-bedrock::default','on_demand'),
  ('anthropic/claude-opus-4.6::openrouter::default','on_demand');

INSERT INTO prices VALUES
  ('anthropic/claude-opus-4.6::anthropic::default','prompt','',1.5e-5,'token'),
  ('anthropic/claude-opus-4.6::anthropic::default','completion','',7.5e-5,'token'),
  ('anthropic/claude-opus-4.6::aws-bedrock::default','prompt','',1.5e-5,'token'),
  ('anthropic/claude-opus-4.6::aws-bedrock::default','completion','',7.5e-5,'token'),
  ('anthropic/claude-opus-4.6::openrouter::default','prompt','',1.5e-5,'token'),
  ('anthropic/claude-opus-4.6::openrouter::default','completion','',7.5e-5,'token');

INSERT INTO health VALUES
  ('anthropic/claude-opus-4.6::anthropic::default', 0.998, 420, 1100, 62.5, 1745020800),
  ('anthropic/claude-opus-4.6::aws-bedrock::default', 0.995, 450, 1300, 55.0, 1745020800);
-- aggregated row has no health
```

- [x] **Step 2: Write the failing test**

`internal/catalog/catalog_test.go`:

```go
package catalog_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/sofq/sku/internal/catalog"
)

func seedShard(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	dst := filepath.Join(dir, "openrouter.db")
	seed, err := os.ReadFile(filepath.Join("testdata", "seed.sql"))
	require.NoError(t, err)
	require.NoError(t, catalog.BuildFromSQL(dst, string(seed)))
	return dst
}

func TestOpen_ReadsMetadata(t *testing.T) {
	path := seedShard(t)
	cat, err := catalog.Open(path)
	require.NoError(t, err)
	t.Cleanup(func() { _ = cat.Close() })

	require.Equal(t, "1", cat.SchemaVersion())
	require.Equal(t, "2026.04.18", cat.CatalogVersion())
	require.Equal(t, "USD", cat.Currency())
}

func TestLookupLLM_ReturnsAllServingProvidersByDefault_ExcludingAggregated(t *testing.T) {
	cat, err := catalog.Open(seedShard(t))
	require.NoError(t, err)
	t.Cleanup(func() { _ = cat.Close() })

	rows, err := cat.LookupLLM(context.Background(), catalog.LLMFilter{
		Model: "anthropic/claude-opus-4.6",
	})
	require.NoError(t, err)
	require.Len(t, rows, 2, "aggregated row excluded by default")

	providers := []string{rows[0].Provider, rows[1].Provider}
	require.ElementsMatch(t, []string{"anthropic", "aws-bedrock"}, providers)

	for _, r := range rows {
		require.Equal(t, "anthropic/claude-opus-4.6", r.ResourceName)
		require.Len(t, r.Prices, 2)
		require.NotNil(t, r.Health)
	}
}

func TestLookupLLM_IncludeAggregatedFlag(t *testing.T) {
	cat, err := catalog.Open(seedShard(t))
	require.NoError(t, err)
	t.Cleanup(func() { _ = cat.Close() })

	rows, err := cat.LookupLLM(context.Background(), catalog.LLMFilter{
		Model:              "anthropic/claude-opus-4.6",
		IncludeAggregated:  true,
	})
	require.NoError(t, err)
	require.Len(t, rows, 3)

	// aggregated row should carry Aggregated=true and Health=nil
	var agg *catalog.Row
	for i := range rows {
		if rows[i].Provider == "openrouter" {
			agg = &rows[i]
		}
	}
	require.NotNil(t, agg)
	require.True(t, agg.Aggregated)
	require.Nil(t, agg.Health)
}

func TestLookupLLM_ServingProviderFilter(t *testing.T) {
	cat, err := catalog.Open(seedShard(t))
	require.NoError(t, err)
	t.Cleanup(func() { _ = cat.Close() })

	rows, err := cat.LookupLLM(context.Background(), catalog.LLMFilter{
		Model:           "anthropic/claude-opus-4.6",
		ServingProvider: "aws-bedrock",
	})
	require.NoError(t, err)
	require.Len(t, rows, 1)
	require.Equal(t, "aws-bedrock", rows[0].Provider)
}

func TestLookupLLM_NotFoundReturnsEmpty(t *testing.T) {
	cat, err := catalog.Open(seedShard(t))
	require.NoError(t, err)
	t.Cleanup(func() { _ = cat.Close() })

	rows, err := cat.LookupLLM(context.Background(), catalog.LLMFilter{
		Model: "totally/made-up",
	})
	require.NoError(t, err)
	require.Empty(t, rows, "no match is not an error at the catalog layer")
}
```

- [x] **Step 3: Run to confirm failure**

Run: `go test ./internal/catalog/...`
Expected: undefined symbols.

- [x] **Step 4: Write `internal/catalog/datadir.go`**

```go
package catalog

import (
	"os"
	"path/filepath"
	"runtime"
)

// DataDir resolves the platform-default shard storage root, honoring
// $SKU_DATA_DIR when set. Spec §4 Environment variables.
func DataDir() string {
	if v := os.Getenv("SKU_DATA_DIR"); v != "" {
		return v
	}
	home, _ := os.UserHomeDir()
	switch runtime.GOOS {
	case "darwin":
		return filepath.Join(home, "Library", "Caches", "sku")
	case "windows":
		if v := os.Getenv("LOCALAPPDATA"); v != "" {
			return filepath.Join(v, "sku")
		}
		return filepath.Join(home, "AppData", "Local", "sku")
	default:
		if v := os.Getenv("XDG_CACHE_HOME"); v != "" {
			return filepath.Join(v, "sku")
		}
		return filepath.Join(home, ".cache", "sku")
	}
}

// ShardPath returns the canonical on-disk path for a shard under DataDir().
func ShardPath(shard string) string {
	return filepath.Join(DataDir(), shard+".db")
}
```

- [x] **Step 5: Write `internal/catalog/catalog.go`**

```go
// Package catalog provides a read-only view over a sku SQLite shard. Spec §5.
package catalog

import (
	"database/sql"
	"fmt"
	"os"

	_ "modernc.org/sqlite" // registers the "sqlite" driver
)

// Minimum/maximum shard schema_version this binary understands. Widening the
// range is a minor binary release; narrowing is a major.
const (
	minSchemaVersion = 1
	maxSchemaVersion = 1
)

// Catalog wraps an opened shard. Safe for concurrent use by multiple goroutines
// (the underlying *sql.DB is; SQLite WAL mode permits concurrent readers).
type Catalog struct {
	db             *sql.DB
	schemaVersion  string
	catalogVersion string
	currency       string
	shardPath      string
}

// Open opens the shard at path in WAL mode and verifies its schema_version.
func Open(path string) (*Catalog, error) {
	// modernc.org/sqlite DSN accepts pragmas via URI query params.
	dsn := "file:" + path + "?_pragma=journal_mode(WAL)&_pragma=busy_timeout(5000)&_pragma=foreign_keys(1)"
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("catalog: open %s: %w", path, err)
	}
	if err := db.Ping(); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("catalog: ping %s: %w", path, err)
	}

	cat := &Catalog{db: db, shardPath: path}
	if err := cat.loadMetadata(); err != nil {
		_ = db.Close()
		return nil, err
	}
	return cat, nil
}

func (c *Catalog) loadMetadata() error {
	rows, err := c.db.Query("SELECT key, value FROM metadata")
	if err != nil {
		return fmt.Errorf("catalog: read metadata: %w", err)
	}
	defer func() { _ = rows.Close() }()

	for rows.Next() {
		var k, v string
		if err := rows.Scan(&k, &v); err != nil {
			return err
		}
		switch k {
		case "schema_version":
			c.schemaVersion = v
		case "catalog_version":
			c.catalogVersion = v
		case "currency":
			c.currency = v
		}
	}
	if err := rows.Err(); err != nil {
		return err
	}
	if c.schemaVersion == "" {
		return fmt.Errorf("catalog: shard %s has no schema_version metadata", c.shardPath)
	}
	// Range check.
	sv := 0
	_, _ = fmt.Sscanf(c.schemaVersion, "%d", &sv)
	if sv < minSchemaVersion || sv > maxSchemaVersion {
		return fmt.Errorf("catalog: shard schema_version=%s outside supported [%d,%d]",
			c.schemaVersion, minSchemaVersion, maxSchemaVersion)
	}
	return nil
}

// Close releases the underlying SQLite handle.
func (c *Catalog) Close() error { return c.db.Close() }

// SchemaVersion returns the shard's declared schema_version string.
func (c *Catalog) SchemaVersion() string { return c.schemaVersion }

// CatalogVersion returns the CalVer release string from metadata.
func (c *Catalog) CatalogVersion() string { return c.catalogVersion }

// Currency returns the shard's invariant currency.
func (c *Catalog) Currency() string { return c.currency }

// BuildFromSQL creates a fresh SQLite file at path, executes the provided SQL
// (schema + seed), and closes the handle. Used only by tests.
func BuildFromSQL(path string, ddl string) error {
	_ = os.Remove(path)
	db, err := sql.Open("sqlite", "file:"+path)
	if err != nil {
		return err
	}
	defer func() { _ = db.Close() }()
	if _, err := db.Exec(ddl); err != nil {
		return err
	}
	return nil
}
```

- [x] **Step 6: Write `internal/catalog/lookup.go`**

```go
package catalog

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
)

// Row is the catalog-layer result record. The renderer (internal/output)
// translates this to the spec §4 output shape.
type Row struct {
	SKUID         string
	Provider      string // serving provider for LLMs
	Service       string
	Kind          string
	ResourceName  string
	Region        string
	RegionGroup   string
	CatalogVersion string
	Currency      string
	TermsHash     string

	Terms        Terms
	ResourceAttrs ResourceAttrs
	Prices        []Price
	Health        *Health

	Aggregated bool // true iff this is the synthetic openrouter row
}

// Terms is the shard-side terms row.
type Terms struct {
	Commitment    string
	Tenancy       string
	OS            string
	SupportTier   string
	Upfront       string
	PaymentOption string
}

// ResourceAttrs mirrors the shard's resource_attrs row (nullable fields as pointers).
type ResourceAttrs struct {
	VCPU             *int64
	MemoryGB         *float64
	StorageGB        *float64
	GPUCount         *int64
	GPUModel         *string
	Architecture     *string
	ContextLength    *int64
	MaxOutputTokens  *int64
	Modality         []string
	Capabilities     []string
	Quantization     *string
	DurabilityNines  *int64
	AvailabilityTier *string
	Extra            map[string]any
}

// Price is a single row from the prices table.
type Price struct {
	Dimension string
	Tier      string
	Amount    float64
	Unit      string
}

// Health mirrors the shard's health row.
type Health struct {
	Uptime30d             *float64
	LatencyP50Ms          *int64
	LatencyP95Ms          *int64
	ThroughputTokensPerSec *float64
	ObservedAt            *int64
}

// LLMFilter captures the flags the CLI surface exposes for `sku llm price`.
type LLMFilter struct {
	Model             string
	ServingProvider   string
	IncludeAggregated bool
}

// LookupLLM executes a point lookup over the openrouter-style row set and
// returns zero or more rows. No match is not an error — callers wrap empty
// results into skuerrors.NotFound at the command layer.
func (c *Catalog) LookupLLM(ctx context.Context, f LLMFilter) ([]Row, error) {
	if f.Model == "" {
		return nil, fmt.Errorf("catalog: LookupLLM requires Model")
	}

	var where []string
	var args []any
	where = append(where, "s.resource_name = ?")
	args = append(args, f.Model)
	if f.ServingProvider != "" {
		where = append(where, "s.provider = ?")
		args = append(args, f.ServingProvider)
	}
	if !f.IncludeAggregated {
		where = append(where, "s.provider <> 'openrouter'")
	}
	// LLM rows are global: we don't filter on region here; the index prefix
	// (resource_name, region, terms_hash) still benefits from the resource_name
	// equality predicate.
	query := `
SELECT s.sku_id, s.provider, s.service, s.kind, s.resource_name, s.region,
       s.region_normalized, s.terms_hash,
       t.commitment, t.tenancy, t.os, t.support_tier, t.upfront, t.payment_option,
       ra.context_length, ra.max_output_tokens, ra.modality, ra.capabilities, ra.quantization,
       h.uptime_30d, h.latency_p50_ms, h.latency_p95_ms, h.throughput_tokens_per_sec, h.observed_at
FROM skus s
JOIN terms t          ON t.sku_id = s.sku_id
LEFT JOIN resource_attrs ra ON ra.sku_id = s.sku_id
LEFT JOIN health h          ON h.sku_id = s.sku_id
WHERE ` + strings.Join(where, " AND ") + `
ORDER BY s.provider`

	rs, err := c.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("catalog: LookupLLM: %w", err)
	}
	defer func() { _ = rs.Close() }()

	var out []Row
	for rs.Next() {
		var r Row
		var supportTier, upfront, paymentOption sql.NullString
		var ctxLen, maxOut sql.NullInt64
		var modalityJSON, capsJSON, quant sql.NullString
		var uptime, throughput sql.NullFloat64
		var p50, p95, observed sql.NullInt64

		if err := rs.Scan(
			&r.SKUID, &r.Provider, &r.Service, &r.Kind, &r.ResourceName, &r.Region,
			&r.RegionGroup, &r.TermsHash,
			&r.Terms.Commitment, &r.Terms.Tenancy, &r.Terms.OS,
			&supportTier, &upfront, &paymentOption,
			&ctxLen, &maxOut, &modalityJSON, &capsJSON, &quant,
			&uptime, &p50, &p95, &throughput, &observed,
		); err != nil {
			return nil, err
		}
		r.CatalogVersion = c.catalogVersion
		r.Currency = c.currency
		r.Terms.SupportTier = supportTier.String
		r.Terms.Upfront = upfront.String
		r.Terms.PaymentOption = paymentOption.String
		if ctxLen.Valid {
			v := ctxLen.Int64
			r.ResourceAttrs.ContextLength = &v
		}
		if maxOut.Valid {
			v := maxOut.Int64
			r.ResourceAttrs.MaxOutputTokens = &v
		}
		if modalityJSON.Valid {
			_ = json.Unmarshal([]byte(modalityJSON.String), &r.ResourceAttrs.Modality)
		}
		if capsJSON.Valid {
			_ = json.Unmarshal([]byte(capsJSON.String), &r.ResourceAttrs.Capabilities)
		}
		if quant.Valid {
			v := quant.String
			r.ResourceAttrs.Quantization = &v
		}
		if uptime.Valid || p50.Valid || p95.Valid || throughput.Valid || observed.Valid {
			h := &Health{}
			if uptime.Valid {
				v := uptime.Float64
				h.Uptime30d = &v
			}
			if p50.Valid {
				v := p50.Int64
				h.LatencyP50Ms = &v
			}
			if p95.Valid {
				v := p95.Int64
				h.LatencyP95Ms = &v
			}
			if throughput.Valid {
				v := throughput.Float64
				h.ThroughputTokensPerSec = &v
			}
			if observed.Valid {
				v := observed.Int64
				h.ObservedAt = &v
			}
			r.Health = h
		}
		r.Aggregated = r.Provider == "openrouter"

		// Load prices in a second query so scan code stays readable.
		if err := c.fillPrices(ctx, &r); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	if err := rs.Err(); err != nil {
		return nil, err
	}
	return out, nil
}

func (c *Catalog) fillPrices(ctx context.Context, r *Row) error {
	rs, err := c.db.QueryContext(ctx,
		"SELECT dimension, tier, amount, unit FROM prices WHERE sku_id = ? ORDER BY dimension, tier",
		r.SKUID,
	)
	if err != nil {
		return err
	}
	defer func() { _ = rs.Close() }()
	for rs.Next() {
		var p Price
		if err := rs.Scan(&p.Dimension, &p.Tier, &p.Amount, &p.Unit); err != nil {
			return err
		}
		r.Prices = append(r.Prices, p)
	}
	return rs.Err()
}
```

- [x] **Step 7: Run tests to verify pass**

Run: `go test ./internal/catalog/... -v`
Expected: all five tests PASS.

- [x] **Step 8: Commit**

```bash
git add internal/catalog/
git commit -m "feat(catalog): add Open + LookupLLM with WAL, schema check, aggregated filter"
```

---

## Task 14: `internal/output` — render to spec §4 envelope (TDD)

Translates `catalog.Row` to the §4 JSON output shape, applies the `agent` preset (M1 default; `price`/`full`/`compare` land in M2).

**Files:**
- Create: `internal/output/render.go`, `internal/output/render_test.go`

- [x] **Step 1: Write the failing test**

`internal/output/render_test.go`:

```go
package output_test

import (
	"bytes"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/sofq/sku/internal/catalog"
	"github.com/sofq/sku/internal/output"
)

func ptrS(s string) *string { return &s }
func ptrI(i int64) *int64   { return &i }
func ptrF(f float64) *float64 { return &f }

func sampleRow() catalog.Row {
	return catalog.Row{
		SKUID:         "anthropic/claude-opus-4.6::anthropic::default",
		Provider:      "anthropic",
		Service:       "llm",
		Kind:          "llm.text",
		ResourceName:  "anthropic/claude-opus-4.6",
		Region:        "",
		RegionGroup:   "",
		CatalogVersion: "2026.04.18",
		Currency:      "USD",
		Terms:         catalog.Terms{Commitment: "on_demand"},
		ResourceAttrs: catalog.ResourceAttrs{
			ContextLength:   ptrI(200000),
			MaxOutputTokens: ptrI(64000),
			Modality:        []string{"text"},
			Capabilities:    []string{"tools"},
		},
		Prices: []catalog.Price{
			{Dimension: "prompt", Amount: 1.5e-5, Unit: "token"},
			{Dimension: "completion", Amount: 7.5e-5, Unit: "token"},
		},
		Health: &catalog.Health{
			Uptime30d:    ptrF(0.998),
			LatencyP50Ms: ptrI(420),
			LatencyP95Ms: ptrI(1100),
			ObservedAt:   ptrI(1745020800),
		},
	}
}

func TestRender_FullPresetProducesSpecShape(t *testing.T) {
	env := output.Render(sampleRow(), output.PresetFull)

	var buf bytes.Buffer
	require.NoError(t, output.Encode(&buf, env, false))

	var got map[string]any
	require.NoError(t, json.Unmarshal(buf.Bytes(), &got))

	require.Equal(t, "anthropic", got["provider"])
	require.Equal(t, "llm", got["service"])
	require.Equal(t, "anthropic/claude-opus-4.6::anthropic::default", got["sku_id"])

	resource := got["resource"].(map[string]any)
	require.Equal(t, "llm.text", resource["kind"])
	require.Equal(t, "anthropic/claude-opus-4.6", resource["name"])

	// Location: nullable empty region_normalized -> null in output
	location := got["location"].(map[string]any)
	require.Nil(t, location["provider_region"])
	require.Nil(t, location["normalized_region"])

	// Price array
	prices := got["price"].([]any)
	require.Len(t, prices, 2)
	first := prices[0].(map[string]any)
	require.Contains(t, []string{"prompt", "completion"}, first["dimension"])
	require.Equal(t, "USD", first["currency"])
	require.Equal(t, "token", first["unit"])

	// Terms: commitment populated, tenancy/os nulls
	terms := got["terms"].(map[string]any)
	require.Equal(t, "on_demand", terms["commitment"])
	require.Nil(t, terms["tenancy"])
	require.Nil(t, terms["os"])

	// Health populated for non-aggregated row
	require.NotNil(t, got["health"])

	// Source + catalog_version
	source := got["source"].(map[string]any)
	require.Equal(t, "2026.04.18", source["catalog_version"])

	// Raw absent in M1
	require.Nil(t, got["raw"])
}

func TestRender_AgentPresetTrimsFields(t *testing.T) {
	env := output.Render(sampleRow(), output.PresetAgent)

	var buf bytes.Buffer
	require.NoError(t, output.Encode(&buf, env, false))

	var got map[string]any
	require.NoError(t, json.Unmarshal(buf.Bytes(), &got))

	// agent preset keeps: provider, service, resource.name,
	// location.provider_region, price, terms.commitment
	require.Equal(t, "anthropic", got["provider"])
	require.Equal(t, "llm", got["service"])
	resource := got["resource"].(map[string]any)
	require.Equal(t, "anthropic/claude-opus-4.6", resource["name"])
	require.NotContains(t, resource, "attributes")
	require.Nil(t, resource["vcpu"])

	require.NotContains(t, got, "health")
	require.NotContains(t, got, "source")
	require.NotContains(t, got, "raw")

	location := got["location"].(map[string]any)
	require.Nil(t, location["provider_region"])

	terms := got["terms"].(map[string]any)
	require.Equal(t, "on_demand", terms["commitment"])
	require.NotContains(t, terms, "tenancy")
}

func TestEncode_CompactAndPretty(t *testing.T) {
	env := output.Render(sampleRow(), output.PresetAgent)

	var compact, pretty bytes.Buffer
	require.NoError(t, output.Encode(&compact, env, false))
	require.NoError(t, output.Encode(&pretty, env, true))

	require.NotContains(t, compact.String(), "\n  ", "compact has no indentation")
	require.Contains(t, pretty.String(), "\n  ", "pretty is indented")

	// Both end with exactly one trailing newline.
	require.Equal(t, byte('\n'), compact.Bytes()[compact.Len()-1])
	require.Equal(t, byte('\n'), pretty.Bytes()[pretty.Len()-1])
}

func TestRender_AggregatedMarkedInAttributes(t *testing.T) {
	r := sampleRow()
	r.Provider = "openrouter"
	r.SKUID = "anthropic/claude-opus-4.6::openrouter::default"
	r.Aggregated = true
	r.Health = nil

	env := output.Render(r, output.PresetFull)
	var buf bytes.Buffer
	require.NoError(t, output.Encode(&buf, env, false))

	var got map[string]any
	require.NoError(t, json.Unmarshal(buf.Bytes(), &got))
	resource := got["resource"].(map[string]any)
	attrs := resource["attributes"].(map[string]any)
	require.Equal(t, true, attrs["aggregated"])
	require.Nil(t, got["health"])
}
```

- [x] **Step 2: Run to confirm failure**

Run: `go test ./internal/output/...`
Expected: undefined package.

- [x] **Step 3: Write `internal/output/render.go`**

```go
// Package output renders catalog.Row values into the spec §4 JSON envelope.
//
// The full envelope always carries the same keys (null when absent); preset
// trimming is applied by stripping keys before encoding. The rendering path
// is allocation-conscious but not micro-optimized — the hot path in M1 is
// the single-row lookup, not streaming output.
package output

import (
	"encoding/json"
	"io"

	"github.com/sofq/sku/internal/catalog"
)

// Preset enumerates the output-shape presets. M1 implements only Agent and
// Full; Price and Compare are stubbed and wired to Agent until M2 fleshes
// them out with kind-specific projections.
type Preset string

const (
	PresetAgent   Preset = "agent"
	PresetPrice   Preset = "price"
	PresetFull    Preset = "full"
	PresetCompare Preset = "compare"
)

// Envelope is the top-level §4 output object. Field ordering is enforced by
// the struct layout; json.Encoder writes keys in declaration order.
type Envelope struct {
	Provider string   `json:"provider"`
	Service  string   `json:"service"`
	SKUID    string   `json:"sku_id,omitempty"`
	Resource *Resource `json:"resource,omitempty"`
	Location *Location `json:"location,omitempty"`
	Price    []Price   `json:"price"`
	Terms    *Terms    `json:"terms,omitempty"`
	Health   *Health   `json:"health,omitempty"`
	Source   *Source   `json:"source,omitempty"`
	Raw      any       `json:"raw,omitempty"`
}

// Resource is the §4 resource block.
type Resource struct {
	Kind            string         `json:"kind"`
	Name            string         `json:"name"`
	VCPU            *int64         `json:"vcpu,omitempty"`
	MemoryGB        *float64       `json:"memory_gb,omitempty"`
	StorageGB       *float64       `json:"storage_gb,omitempty"`
	GPUCount        *int64         `json:"gpu_count,omitempty"`
	ContextLength   *int64         `json:"context_length,omitempty"`
	MaxOutputTokens *int64         `json:"max_output_tokens,omitempty"`
	Capabilities    []string       `json:"capabilities,omitempty"`
	Attributes      map[string]any `json:"attributes,omitempty"`
}

// Location is the §4 location block.
type Location struct {
	ProviderRegion   *string `json:"provider_region"`
	NormalizedRegion *string `json:"normalized_region"`
	AvailabilityZone *string `json:"availability_zone"`
}

// Price is a single price dimension as emitted in the output.
type Price struct {
	Amount    float64 `json:"amount"`
	Currency  string  `json:"currency"`
	Unit      string  `json:"unit"`
	Dimension string  `json:"dimension"`
	Tier      *string `json:"tier"`
}

// Terms is the §4 terms block. Empty-string sentinels become nil pointers.
type Terms struct {
	Commitment    string  `json:"commitment"`
	Tenancy       *string `json:"tenancy,omitempty"`
	OS            *string `json:"os,omitempty"`
	SupportTier   *string `json:"support_tier,omitempty"`
	Upfront       *string `json:"upfront,omitempty"`
	PaymentOption *string `json:"payment_option,omitempty"`
}

// Health is the §4 health block (LLM-populated).
type Health struct {
	Uptime30d              *float64 `json:"uptime_30d,omitempty"`
	LatencyP50Ms           *int64   `json:"latency_p50_ms,omitempty"`
	LatencyP95Ms           *int64   `json:"latency_p95_ms,omitempty"`
	ThroughputTokensPerSec *float64 `json:"throughput_tokens_per_sec,omitempty"`
	ObservedAt             *int64   `json:"observed_at,omitempty"`
}

// Source is the §4 source block.
type Source struct {
	CatalogVersion string `json:"catalog_version"`
	FetchedAt      string `json:"fetched_at,omitempty"`
	UpstreamID     string `json:"upstream_id,omitempty"`
	Freshness      string `json:"freshness,omitempty"`
}

func nilIfEmpty(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

func nilIfBlankPtr(p *string) *string {
	if p == nil || *p == "" {
		return nil
	}
	return p
}

// Render builds an Envelope for a single catalog.Row at the given preset.
// The full envelope is built first and then trimmed per-preset so callers
// can compare presets to the same canonical shape.
func Render(r catalog.Row, p Preset) Envelope {
	env := buildFull(r)
	switch p {
	case PresetFull:
		return env
	case PresetAgent, PresetPrice, PresetCompare, "":
		return trimForAgent(env)
	default:
		return trimForAgent(env)
	}
}

func buildFull(r catalog.Row) Envelope {
	prices := make([]Price, 0, len(r.Prices))
	for _, rp := range r.Prices {
		tier := rp.Tier
		prices = append(prices, Price{
			Amount:    rp.Amount,
			Currency:  r.Currency,
			Unit:      rp.Unit,
			Dimension: rp.Dimension,
			Tier:      nilIfEmpty(tier),
		})
	}

	resource := &Resource{
		Kind:            r.Kind,
		Name:            r.ResourceName,
		VCPU:            r.ResourceAttrs.VCPU,
		MemoryGB:        r.ResourceAttrs.MemoryGB,
		StorageGB:       r.ResourceAttrs.StorageGB,
		GPUCount:        r.ResourceAttrs.GPUCount,
		ContextLength:   r.ResourceAttrs.ContextLength,
		MaxOutputTokens: r.ResourceAttrs.MaxOutputTokens,
		Capabilities:    r.ResourceAttrs.Capabilities,
	}
	if r.Aggregated {
		resource.Attributes = map[string]any{"aggregated": true}
	}

	var terms *Terms
	if r.Terms.Commitment != "" {
		t := &Terms{
			Commitment:    r.Terms.Commitment,
			Tenancy:       nilIfEmpty(r.Terms.Tenancy),
			OS:            nilIfEmpty(r.Terms.OS),
			SupportTier:   nilIfEmpty(r.Terms.SupportTier),
			Upfront:       nilIfEmpty(r.Terms.Upfront),
			PaymentOption: nilIfEmpty(r.Terms.PaymentOption),
		}
		terms = t
	}

	location := &Location{
		ProviderRegion:   nilIfEmpty(r.Region),
		NormalizedRegion: nilIfEmpty(r.RegionGroup),
		AvailabilityZone: nil,
	}

	var health *Health
	if r.Health != nil {
		h := &Health{
			Uptime30d:              r.Health.Uptime30d,
			LatencyP50Ms:           r.Health.LatencyP50Ms,
			LatencyP95Ms:           r.Health.LatencyP95Ms,
			ThroughputTokensPerSec: r.Health.ThroughputTokensPerSec,
			ObservedAt:             r.Health.ObservedAt,
		}
		health = h
	}

	source := &Source{
		CatalogVersion: r.CatalogVersion,
		Freshness:      "daily",
	}

	return Envelope{
		Provider: r.Provider,
		Service:  r.Service,
		SKUID:    r.SKUID,
		Resource: resource,
		Location: location,
		Price:    prices,
		Terms:    terms,
		Health:   health,
		Source:   source,
	}
}

// trimForAgent keeps the fields spec §4 "Presets" declares for the agent preset:
// provider, service, resource.name, location.provider_region, price, terms.commitment.
func trimForAgent(env Envelope) Envelope {
	out := Envelope{
		Provider: env.Provider,
		Service:  env.Service,
		Price:    env.Price,
	}
	if env.Resource != nil {
		out.Resource = &Resource{Name: env.Resource.Name}
	}
	if env.Location != nil {
		out.Location = &Location{
			ProviderRegion: env.Location.ProviderRegion,
		}
	}
	if env.Terms != nil {
		out.Terms = &Terms{Commitment: env.Terms.Commitment}
	}
	// health, source, raw dropped
	return out
}

// Encode writes the envelope as JSON to w. When pretty is false the encoding
// is compact (`json.Encoder`); when true it's indented with two spaces.
func Encode(w io.Writer, env Envelope, pretty bool) error {
	enc := json.NewEncoder(w)
	if pretty {
		enc.SetIndent("", "  ")
	}
	return enc.Encode(env)
}
```

- [x] **Step 4: Run tests to verify pass**

Run: `go test ./internal/output/... -v`
Expected: all four tests PASS.

- [x] **Step 5: Commit**

```bash
git add internal/output/
git commit -m "feat(output): render catalog.Row to spec §4 envelope, agent + full presets"
```

---

## Task 15: `sku llm price` Cobra wiring + end-to-end (TDD)

**Files:**
- Create: `cmd/sku/llm.go`, `cmd/sku/llm_price.go`, `cmd/sku/llm_price_test.go`
- Modify: `cmd/sku/root.go` (register `llm` subtree)

- [x] **Step 1: Write the failing test**

`cmd/sku/llm_price_test.go`:

```go
package sku

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/sofq/sku/internal/catalog"
)

// seedTestDataDir creates a temp SKU_DATA_DIR containing an openrouter.db
// built from internal/catalog/testdata/seed.sql.
func seedTestDataDir(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	ddl, err := os.ReadFile(filepath.Join("..", "..", "internal", "catalog", "testdata", "seed.sql"))
	require.NoError(t, err)
	require.NoError(t, catalog.BuildFromSQL(filepath.Join(dir, "openrouter.db"), string(ddl)))
	t.Setenv("SKU_DATA_DIR", dir)
	return dir
}

func runLLMPrice(t *testing.T, args ...string) (stdout, stderr string, code int) {
	t.Helper()
	var out, errb bytes.Buffer
	cmd := newRootCmd()
	cmd.SetOut(&out)
	cmd.SetErr(&errb)
	cmd.SetArgs(append([]string{"llm", "price"}, args...))
	err := cmd.Execute()
	code = 0
	if err != nil {
		// The command sets its own exit code via the returned error;
		// for unit testing we just check that the error surface is populated.
		// The process-level exit code is exercised in the e2e test.
		code = 1
	}
	return out.String(), errb.String(), code
}

func TestLLMPrice_HappyPath_ReturnsJSONPerEnvelope(t *testing.T) {
	seedTestDataDir(t)

	out, _, _ := runLLMPrice(t, "--model", "anthropic/claude-opus-4.6")

	// stdout is NDJSON (one row per line) since multiple serving providers match.
	lines := splitLines(out)
	require.Len(t, lines, 2)

	providers := map[string]bool{}
	for _, line := range lines {
		var env map[string]any
		require.NoError(t, json.Unmarshal([]byte(line), &env))
		require.Equal(t, "llm", env["service"])
		providers[env["provider"].(string)] = true
		require.NotEmpty(t, env["price"])
	}
	require.True(t, providers["anthropic"])
	require.True(t, providers["aws-bedrock"])
	require.False(t, providers["openrouter"], "aggregated row excluded by default")
}

func TestLLMPrice_ServingProviderFlag(t *testing.T) {
	seedTestDataDir(t)

	out, _, _ := runLLMPrice(t,
		"--model", "anthropic/claude-opus-4.6",
		"--serving-provider", "aws-bedrock",
	)
	lines := splitLines(out)
	require.Len(t, lines, 1)

	var env map[string]any
	require.NoError(t, json.Unmarshal([]byte(lines[0]), &env))
	require.Equal(t, "aws-bedrock", env["provider"])
}

func TestLLMPrice_IncludeAggregated(t *testing.T) {
	seedTestDataDir(t)

	out, _, _ := runLLMPrice(t,
		"--model", "anthropic/claude-opus-4.6",
		"--include-aggregated",
	)
	require.Len(t, splitLines(out), 3)
}

func TestLLMPrice_NotFound_ReturnsExit3Envelope(t *testing.T) {
	seedTestDataDir(t)

	_, stderr, code := runLLMPrice(t, "--model", "fake/model")
	require.NotZero(t, code)

	var env map[string]any
	require.NoError(t, json.Unmarshal([]byte(stderr), &env))
	body := env["error"].(map[string]any)
	require.Equal(t, "not_found", body["code"])
	require.Contains(t, body["suggestion"].(string), "sku update")
}

func TestLLMPrice_MissingModel_ReturnsValidationError(t *testing.T) {
	seedTestDataDir(t)

	_, stderr, code := runLLMPrice(t)
	require.NotZero(t, code)

	var env map[string]any
	require.NoError(t, json.Unmarshal([]byte(stderr), &env))
	body := env["error"].(map[string]any)
	require.Equal(t, "validation", body["code"])
}

func TestLLMPrice_ShardMissing_ReturnsExit3(t *testing.T) {
	t.Setenv("SKU_DATA_DIR", t.TempDir())

	_, stderr, code := runLLMPrice(t, "--model", "anthropic/claude-opus-4.6")
	require.NotZero(t, code)

	var env map[string]any
	require.NoError(t, json.Unmarshal([]byte(stderr), &env))
	body := env["error"].(map[string]any)
	require.Equal(t, "not_found", body["code"])
	require.Contains(t, body["suggestion"].(string), "sku update")
	details := body["details"].(map[string]any)
	require.Equal(t, "openrouter", details["shard"])
}

func splitLines(s string) []string {
	var out []string
	start := 0
	for i := 0; i < len(s); i++ {
		if s[i] == '\n' {
			if i > start {
				out = append(out, s[start:i])
			}
			start = i + 1
		}
	}
	if start < len(s) {
		out = append(out, s[start:])
	}
	return out
}
```

- [x] **Step 2: Run to confirm failure**

Run: `go test ./cmd/sku/...`
Expected: `unknown command "llm"`.

- [x] **Step 3: Write `cmd/sku/llm.go`**

```go
package sku

import "github.com/spf13/cobra"

func newLLMCmd() *cobra.Command {
	c := &cobra.Command{
		Use:   "llm",
		Short: "Cross-provider LLM pricing (OpenRouter-backed)",
	}
	c.AddCommand(newLLMPriceCmd())
	return c
}
```

- [x] **Step 4: Write `cmd/sku/llm_price.go`**

```go
package sku

import (
	"context"
	"errors"
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/sofq/sku/internal/catalog"
	skuerrors "github.com/sofq/sku/internal/errors"
	"github.com/sofq/sku/internal/output"
)

func newLLMPriceCmd() *cobra.Command {
	var (
		model             string
		servingProvider   string
		includeAggregated bool
		pretty            bool
	)
	c := &cobra.Command{
		Use:   "price",
		Short: "Price one or more serving-provider options for an LLM",
		RunE: func(cmd *cobra.Command, _ []string) error {
			if model == "" {
				return skuerrors.Validation(
					"flag_invalid", "model", "",
					"pass --model <author>/<slug>, e.g. --model anthropic/claude-opus-4.6",
				)
			}

			shardPath := catalog.ShardPath("openrouter")
			if _, err := os.Stat(shardPath); err != nil {
				return &skuerrors.E{
					Code:       skuerrors.CodeNotFound,
					Message:    "openrouter shard not installed",
					Suggestion: "Run: sku update openrouter",
					Details: map[string]any{
						"shard":        "openrouter",
						"install_hint": "sku update openrouter",
					},
				}
			}

			cat, err := catalog.Open(shardPath)
			if err != nil {
				return &skuerrors.E{
					Code: skuerrors.CodeServer, Message: err.Error(),
					Suggestion: "Check that the shard file is readable and not truncated",
				}
			}
			defer func() { _ = cat.Close() }()

			rows, err := cat.LookupLLM(context.Background(), catalog.LLMFilter{
				Model:             model,
				ServingProvider:   servingProvider,
				IncludeAggregated: includeAggregated,
			})
			if err != nil {
				return fmt.Errorf("llm price: %w", err)
			}
			if len(rows) == 0 {
				return skuerrors.NotFound(
					"openrouter", "llm",
					map[string]any{
						"model":            model,
						"serving_provider": servingProvider,
					},
					"Try `sku update openrouter` or drop --serving-provider",
				)
			}

			w := cmd.OutOrStdout()
			for _, r := range rows {
				env := output.Render(r, output.PresetAgent)
				if err := output.Encode(w, env, pretty); err != nil {
					return errors.Join(errors.New("output encode failed"), err)
				}
			}
			return nil
		},
	}
	c.Flags().StringVar(&model, "model", "", "Model ID, e.g. anthropic/claude-opus-4.6")
	c.Flags().StringVar(&servingProvider, "serving-provider", "", "Filter to a single serving provider (e.g. aws-bedrock)")
	c.Flags().BoolVar(&includeAggregated, "include-aggregated", false, "Include OpenRouter's synthetic aggregated row")
	c.Flags().BoolVar(&pretty, "pretty", false, "Pretty-print JSON")
	return c
}
```

- [x] **Step 5: Register `llm` in `cmd/sku/root.go`**

Edit `newRootCmd` to add `root.AddCommand(newLLMCmd())` just after the `newVersionCmd()` line.

- [x] **Step 6: Run tests to verify pass**

Run: `go test ./cmd/sku/... -v`
Expected: all six new tests PASS plus the existing version tests.

- [x] **Step 7: Build + smoke**

```bash
make openrouter-shard   # build a real local shard from fixtures
SKU_DATA_DIR=$(pwd)/dist/pipeline ./bin/sku llm price --model anthropic/claude-opus-4.6
```

Expected: two JSON lines on stdout.

- [x] **Step 8: Commit**

```bash
git add cmd/sku/llm.go cmd/sku/llm_price.go cmd/sku/llm_price_test.go cmd/sku/root.go
git commit -m "feat(cmd): add sku llm price with model/serving-provider/include-aggregated/pretty"
```

---

## Task 16: Minimal `sku update openrouter` (TDD with httptest server)

Baseline-only downloader: HTTP GET → sha256 verify → zstd decompress → atomic rename into `SKU_DATA_DIR`. No manifest, no delta chain, no ETag — those land in M3a.

**Files:**
- Create: `cmd/sku/update.go`, `cmd/sku/update_test.go`

- [x] **Step 1: Write the failing test**

`cmd/sku/update_test.go`:

```go
package sku

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/klauspost/compress/zstd"
	"github.com/stretchr/testify/require"

	"github.com/sofq/sku/internal/catalog"
)

func buildBaselineBytes(t *testing.T) (raw, compressed []byte, sum string) {
	t.Helper()

	// Build a real SQLite file from the seed, then read its bytes.
	dir := t.TempDir()
	seed, err := os.ReadFile(filepath.Join("..", "..", "internal", "catalog", "testdata", "seed.sql"))
	require.NoError(t, err)
	src := filepath.Join(dir, "openrouter.db")
	require.NoError(t, catalog.BuildFromSQL(src, string(seed)))
	raw, err = os.ReadFile(src)
	require.NoError(t, err)

	var buf bytes.Buffer
	enc, err := zstd.NewWriter(&buf)
	require.NoError(t, err)
	_, err = enc.Write(raw)
	require.NoError(t, err)
	require.NoError(t, enc.Close())
	compressed = buf.Bytes()

	h := sha256.Sum256(compressed)
	sum = hex.EncodeToString(h[:])
	return
}

func TestUpdate_DownloadsVerifiesUnpacks(t *testing.T) {
	_, zst, sum := buildBaselineBytes(t)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/openrouter.db.zst" {
			w.Header().Set("Content-Type", "application/zstd")
			_, _ = w.Write(zst)
			return
		}
		if r.URL.Path == "/openrouter.db.zst.sha256" {
			_, _ = w.Write([]byte(sum + "  openrouter.db.zst\n"))
			return
		}
		http.NotFound(w, r)
	}))
	defer srv.Close()

	dataDir := t.TempDir()
	t.Setenv("SKU_DATA_DIR", dataDir)
	t.Setenv("SKU_UPDATE_BASE_URL", srv.URL) // test-only override

	var out, errb bytes.Buffer
	cmd := newRootCmd()
	cmd.SetOut(&out)
	cmd.SetErr(&errb)
	cmd.SetArgs([]string{"update", "openrouter"})
	require.NoError(t, cmd.Execute(), "stderr=%s", errb.String())

	installed := filepath.Join(dataDir, "openrouter.db")
	st, err := os.Stat(installed)
	require.NoError(t, err)
	require.NotZero(t, st.Size())
}

func TestUpdate_ChecksumMismatch_ReturnsConflict(t *testing.T) {
	_, zst, _ := buildBaselineBytes(t)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/openrouter.db.zst" {
			_, _ = w.Write(zst)
			return
		}
		if r.URL.Path == "/openrouter.db.zst.sha256" {
			// lie
			_, _ = w.Write([]byte("deadbeef" + "  openrouter.db.zst\n"))
			return
		}
		http.NotFound(w, r)
	}))
	defer srv.Close()

	dataDir := t.TempDir()
	t.Setenv("SKU_DATA_DIR", dataDir)
	t.Setenv("SKU_UPDATE_BASE_URL", srv.URL)

	var out, errb bytes.Buffer
	cmd := newRootCmd()
	cmd.SetOut(&out)
	cmd.SetErr(&errb)
	cmd.SetArgs([]string{"update", "openrouter"})
	err := cmd.Execute()
	require.Error(t, err)
	require.Contains(t, err.Error(), "sha256")
}

func TestUpdate_ServerError_ReturnsServerCode(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusBadGateway)
	}))
	defer srv.Close()

	t.Setenv("SKU_DATA_DIR", t.TempDir())
	t.Setenv("SKU_UPDATE_BASE_URL", srv.URL)

	cmd := newRootCmd()
	var errb bytes.Buffer
	cmd.SetErr(&errb)
	cmd.SetArgs([]string{"update", "openrouter"})
	err := cmd.Execute()
	require.Error(t, err)
	require.Contains(t, err.Error(), "502")
}
```

- [x] **Step 2: Run to confirm failure**

Run: `go test ./cmd/sku/... -run TestUpdate`
Expected: `unknown command "update"`.

- [x] **Step 3: Write `cmd/sku/update.go`**

```go
package sku

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/klauspost/compress/zstd"
	"github.com/spf13/cobra"

	"github.com/sofq/sku/internal/catalog"
	skuerrors "github.com/sofq/sku/internal/errors"
)

// defaultUpdateBaseURL is the bootstrap shard hosting location for M1.
// In M3a this is replaced by the manifest-driven updater. Overridden in tests
// via SKU_UPDATE_BASE_URL.
const defaultUpdateBaseURL = "https://github.com/sofq/sku/releases/download/data-bootstrap-openrouter"

func newUpdateCmd() *cobra.Command {
	c := &cobra.Command{
		Use:   "update [shard]",
		Short: "Download a shard baseline (M1: openrouter only; full updater in M3a)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			shard := args[0]
			if shard != "openrouter" {
				return skuerrors.Validation(
					"flag_invalid", "shard", shard,
					"M1 only supports `sku update openrouter`; cloud shards land in M3a",
				)
			}
			base := os.Getenv("SKU_UPDATE_BASE_URL")
			if base == "" {
				base = defaultUpdateBaseURL
			}

			dataDir := catalog.DataDir()
			if err := os.MkdirAll(dataDir, 0o755); err != nil {
				return fmt.Errorf("mkdir %s: %w", dataDir, err)
			}

			zstPath := filepath.Join(dataDir, shard+".db.zst.part")
			dbPath := filepath.Join(dataDir, shard+".db")

			if err := downloadFile(base+"/"+shard+".db.zst", zstPath); err != nil {
				return &skuerrors.E{Code: skuerrors.CodeServer, Message: err.Error()}
			}
			defer func() { _ = os.Remove(zstPath) }()

			wantSum, err := fetchSHA256(base + "/" + shard + ".db.zst.sha256")
			if err != nil {
				return &skuerrors.E{Code: skuerrors.CodeServer, Message: err.Error()}
			}
			gotSum, err := sha256File(zstPath)
			if err != nil {
				return fmt.Errorf("hash %s: %w", zstPath, err)
			}
			if !strings.EqualFold(gotSum, wantSum) {
				return &skuerrors.E{
					Code:       skuerrors.CodeConflict,
					Message:    fmt.Sprintf("sha256 mismatch: want %s got %s", wantSum, gotSum),
					Suggestion: "Retry `sku update openrouter`; if it persists the upstream release may be corrupt",
				}
			}

			tmp := dbPath + ".part"
			if err := decompressTo(zstPath, tmp); err != nil {
				return fmt.Errorf("decompress: %w", err)
			}
			if err := os.Rename(tmp, dbPath); err != nil {
				return fmt.Errorf("rename: %w", err)
			}
			fmt.Fprintf(cmd.ErrOrStderr(), "installed %s -> %s\n", shard, dbPath)
			return nil
		},
	}
	return c
}

func downloadFile(url, dst string) error {
	client := &http.Client{Timeout: 60 * time.Second}
	resp, err := client.Get(url)
	if err != nil {
		return fmt.Errorf("GET %s: %w", url, err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode >= 400 {
		return fmt.Errorf("GET %s: HTTP %d", url, resp.StatusCode)
	}
	f, err := os.Create(dst)
	if err != nil {
		return err
	}
	if _, err := io.Copy(f, resp.Body); err != nil {
		_ = f.Close()
		return err
	}
	return f.Close()
}

func fetchSHA256(url string) (string, error) {
	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Get(url)
	if err != nil {
		return "", err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode >= 400 {
		return "", fmt.Errorf("GET %s: HTTP %d", url, resp.StatusCode)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	first := strings.Fields(string(body))
	if len(first) == 0 {
		return "", errors.New("empty sha256 file")
	}
	return first[0], nil
}

func sha256File(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer func() { _ = f.Close() }()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

func decompressTo(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer func() { _ = in.Close() }()
	dec, err := zstd.NewReader(in)
	if err != nil {
		return err
	}
	defer dec.Close()
	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, dec); err != nil {
		_ = out.Close()
		return err
	}
	return out.Close()
}
```

- [x] **Step 4: Register `update` in `cmd/sku/root.go`**

Add `root.AddCommand(newUpdateCmd())` next to the other `AddCommand` calls.

- [x] **Step 5: Run tests to verify pass**

Run: `go test ./cmd/sku/... -run TestUpdate -v`
Expected: three tests PASS.

- [x] **Step 6: Commit**

```bash
git add cmd/sku/update.go cmd/sku/update_test.go cmd/sku/root.go
git commit -m "feat(cmd): add minimal sku update openrouter (baseline + sha256 + zstd)"
```

---

## Task 17: Integration test with real built shard

Ties the halves together: build the real shard via `make openrouter-shard`, run `sku llm price` against it.

**Files:**
- Create: `internal/catalog/integration_test.go`

- [x] **Step 1: Write the test**

`internal/catalog/integration_test.go`:

```go
//go:build integration

package catalog_test

import (
	"context"
	"os"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/sofq/sku/internal/catalog"
)

func TestIntegration_RealBuiltShard(t *testing.T) {
	path := os.Getenv("SKU_TEST_SHARD")
	if path == "" {
		t.Skip("SKU_TEST_SHARD not set; run `make openrouter-shard && SKU_TEST_SHARD=... go test -tags=integration`")
	}
	cat, err := catalog.Open(path)
	require.NoError(t, err)
	t.Cleanup(func() { _ = cat.Close() })

	require.Equal(t, "USD", cat.Currency())

	rows, err := cat.LookupLLM(context.Background(), catalog.LLMFilter{
		Model: "anthropic/claude-opus-4.6",
	})
	require.NoError(t, err)
	require.NotEmpty(t, rows)

	// Every row has at least one price dimension.
	for _, r := range rows {
		require.NotEmpty(t, r.Prices, "row %s has no prices", r.SKUID)
	}
}
```

- [x] **Step 2: Run it**

```bash
make openrouter-shard
make test-integration
```

Expected: PASS.

- [x] **Step 3: Commit**

```bash
git add internal/catalog/integration_test.go
git commit -m "test(catalog): add integration test against real built shard"
```

---

## Task 18: Bench harness — warm/cold point lookup

Establishes the M1 baseline per §5 perf targets: warm <5 ms p99, cold <60 ms end-to-end.

**Files:**
- Create: `bench/bench_test.go`

- [x] **Step 1: Write the benchmark**

`bench/bench_test.go`:

```go
package bench

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/sofq/sku/internal/catalog"
	"github.com/sofq/sku/internal/output"
)

// BenchmarkPointLookup_Warm measures in-process point-lookup latency with
// the catalog already open — the §5 "warm" number.
func BenchmarkPointLookup_Warm(b *testing.B) {
	path := os.Getenv("SKU_BENCH_SHARD")
	if path == "" {
		b.Skip("SKU_BENCH_SHARD not set; run via `make bench`")
	}
	cat, err := catalog.Open(path)
	if err != nil {
		b.Fatal(err)
	}
	b.Cleanup(func() { _ = cat.Close() })

	filter := catalog.LLMFilter{Model: "anthropic/claude-opus-4.6"}
	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		rows, err := cat.LookupLLM(ctx, filter)
		if err != nil {
			b.Fatal(err)
		}
		var buf bytes.Buffer
		for _, r := range rows {
			env := output.Render(r, output.PresetAgent)
			if err := output.Encode(&buf, env, false); err != nil {
				b.Fatal(err)
			}
		}
		_ = json.Valid(buf.Bytes())
	}
}

// BenchmarkPointLookup_Cold measures the whole process-startup path: exec of
// the real binary + shard open + lookup + render + exit. This is the number
// that matches §5 "<60 ms cold".
func BenchmarkPointLookup_Cold(b *testing.B) {
	shard := os.Getenv("SKU_BENCH_SHARD")
	if shard == "" {
		b.Skip("SKU_BENCH_SHARD not set")
	}
	// Find the binary — bench target builds it at ../bin/sku.
	bin, err := filepath.Abs(filepath.Join("..", "bin", "sku"))
	if err != nil {
		b.Fatal(err)
	}
	if _, err := os.Stat(bin); err != nil {
		b.Skipf("bin/sku missing: %v (run `make build` first)", err)
	}
	dataDir := filepath.Dir(shard)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		cmd := exec.Command(bin, "llm", "price", "--model", "anthropic/claude-opus-4.6")
		cmd.Env = append(os.Environ(), "SKU_DATA_DIR="+dataDir)
		if err := cmd.Run(); err != nil {
			b.Fatal(err)
		}
	}
}
```

- [x] **Step 2: Run the bench**

```bash
make build
make openrouter-shard
make bench
```

Expected: two benchmark results with `ns/op` numbers. Record them in the M1 exit note. `warm` < 5,000,000 ns/op (5 ms) and `cold` < 60,000,000 ns/op (60 ms) are the targets.

- [x] **Step 3: If a target is missed**

Per spec §5: "if any target is unattainable the spec is updated before M2, not silently relaxed." Capture the measurement in `docs/ops/m1-bench-baseline.md` with a short analysis and adjust the spec `docs/superpowers/specs/2026-04-18-sku-design.md` §5 Performance targets in a separate commit if needed.

- [x] **Step 4: Commit the bench harness (results not checked in)**

```bash
git add bench/
git commit -m "bench: add warm + cold point-lookup benches (§5 perf baseline)"
```

---

## Task 19: Bootstrap release runbook

One-page maintainer runbook for the one-shot shard upload that `sku update` points at. Enables the M1 exit criterion "sku update downloads OpenRouter shard" without waiting for M3a's daily CI.

**Files:**
- Create: `docs/ops/m1-bootstrap-release.md`

- [x] **Step 1: Write the runbook**

```markdown
# M1 bootstrap OpenRouter release

Purpose: one-shot upload of a manually-built `openrouter.db.zst` to the
`data-bootstrap-openrouter` GitHub release so `sku update openrouter` has
something to download until the daily data pipeline goes live in M3a.

Run this once per M1, and again if the shard schema changes before M3a.

## Prereqs

- `gh` CLI authenticated against `github.com/sofq/sku` with `repo` scope.
- Python venv set up (`make -C pipeline setup`).
- OpenRouter reachable from the build host (no auth required for `/api/v1/*`).

## Steps

1. Build a fresh live shard (not from fixtures):

```bash
cd pipeline
SKU_FIXED_OBSERVED_AT=$(date +%s) .venv/bin/python -m ingest.openrouter \
  --out ../dist/pipeline/openrouter.rows.jsonl \
  --generated-at $(date -u +%Y-%m-%dT%H:%M:%SZ)
.venv/bin/python -m package.build_shard \
  --rows ../dist/pipeline/openrouter.rows.jsonl \
  --shard openrouter \
  --out ../dist/pipeline/openrouter.db \
  --catalog-version $(date -u +%Y.%m.%d)
cd ..
```

2. Compress + checksum:

```bash
zstd -19 dist/pipeline/openrouter.db -o dist/pipeline/openrouter.db.zst
cd dist/pipeline
sha256sum openrouter.db.zst > openrouter.db.zst.sha256
cd -
```

3. Upload:

```bash
gh release create data-bootstrap-openrouter \
  --title "M1 bootstrap OpenRouter shard" \
  --notes "Bootstrap shard for sku llm price during M1. Replaced by daily pipeline in M3a." \
  dist/pipeline/openrouter.db.zst \
  dist/pipeline/openrouter.db.zst.sha256
```

If the release already exists:

```bash
gh release upload data-bootstrap-openrouter \
  dist/pipeline/openrouter.db.zst dist/pipeline/openrouter.db.zst.sha256 \
  --clobber
```

4. Verify from a clean env:

```bash
tmp=$(mktemp -d)
SKU_DATA_DIR="$tmp" ./bin/sku update openrouter
SKU_DATA_DIR="$tmp" ./bin/sku llm price --model anthropic/claude-opus-4.6
```

Expected: JSON lines on stdout.

## Rollback

If a bootstrap upload turns out to be bad, delete the assets on the release and re-upload the previous good build:

```bash
gh release delete-asset data-bootstrap-openrouter openrouter.db.zst openrouter.db.zst.sha256
# re-run step 3 with the known-good local artifacts
```

Clients fail closed with exit code 6 (`conflict` — sha256 mismatch) until a new upload lands.
```

- [x] **Step 2: Commit**

```bash
git add docs/ops/m1-bootstrap-release.md
git commit -m "docs(ops): add M1 OpenRouter bootstrap-release runbook"
```

---

## Task 20: CLAUDE.md + milestone pointer bump

**Files:**
- Modify: `CLAUDE.md`

- [x] **Step 1: Update the "Current milestone" section**

Edit the bottom of `CLAUDE.md`:

```markdown
## Current milestone

M1 — OpenRouter shard + catalog reader + `sku llm price`. See `docs/superpowers/plans/2026-04-18-m1-openrouter-shard-and-llm-price.md`.

### Quick path (agent, repeatable)

```bash
make openrouter-shard                                   # build local shard from fixtures
SKU_DATA_DIR=$(pwd)/dist/pipeline ./bin/sku llm price \
  --model anthropic/claude-opus-4.6                     # two JSON lines out
SKU_DATA_DIR=$(pwd)/dist/pipeline ./bin/sku llm price \
  --model anthropic/claude-opus-4.6 --pretty            # indented
```
```

And add a new row to the dev-commands table:

| Build local OpenRouter shard | `make openrouter-shard` |
| Run Go integration tests | `make test-integration` |
| Run benchmarks | `make bench` |
| Run Python pipeline tests | `make pipeline-test` |

- [x] **Step 2: Commit**

```bash
git add CLAUDE.md
git commit -m "docs: bump CLAUDE.md to M1 with llm-price quick path"
```

---

## Task 21: M1 exit verification

No code changes — a gate before declaring M1 done.

- [x] **Step 1: All tests green**

Run:
```bash
make test
make pipeline-test
make openrouter-shard
make test-integration
```

Expected: PASS in all four.

- [x] **Step 2: Bench targets within budget**

Run: `make bench`
Expected: warm < 5 ms/op, cold < 60 ms/op. If either is missed, write an analysis to `docs/ops/m1-bench-baseline.md` and amend the spec §5 perf targets in a follow-up commit before declaring M1 done (per spec §5 contract).

- [x] **Step 3: Bootstrap release uploaded**

Follow `docs/ops/m1-bootstrap-release.md` to upload the shard; then verify:

```bash
tmp=$(mktemp -d)
SKU_DATA_DIR="$tmp" ./bin/sku update openrouter
SKU_DATA_DIR="$tmp" ./bin/sku llm price --model anthropic/claude-opus-4.6 --pretty
```

Expected: indented JSON emitted; exit 0.

- [x] **Step 4: CI green**

Run: `git push -u origin m1-openrouter-shard`
Expected: `ci.yml` green across 5-platform × 2-Go-minor. Fix any platform-specific breakage (likely: modernc.org/sqlite on windows/arm64 — if flaky, we document rather than debug-to-exhaustion per scope) before declaring M1 done.

- [x] **Step 5: Tag M1 completion**

```bash
git tag -s m1-done -m "M1 complete: OpenRouter shard + catalog reader + sku llm price"
```

- [x] **Step 6: Merge to master**

Open a PR; once CI is green, merge.

---

## Self-review notes

- **Spec coverage:**
  - §3 OpenRouter-specific ingest: Tasks 6–8 (two endpoints, aggregated row, USD guard).
  - §4 output envelope + presets: Task 14 (agent + full; price/compare deferred to M2 as noted).
  - §4 error envelope + exit codes: Tasks 11, 12, 15, 16.
  - §4 `sku llm price` flags: Task 15 (`--model`, `--serving-provider`, `--include-aggregated`, `--pretty`).
  - §5 SQLite schema + indexes + metadata: Task 9 (schema.sql verbatim).
  - §5 `terms_hash` contract: Tasks 3–5 (shared Python/Go golden).
  - §5 perf targets: Task 18 (bench harness, documented response if missed).
  - §9 M1 exit criteria: Task 21 (`sku update` works, `sku llm price` returns correct JSON, perf baseline captured).
- **Deferred items explicitly captured in the header’s deferred block**, each cross-referenced to the task/milestone that picks them up.
- **Placeholder scan:** every code step contains actual code; every CLI step contains the actual command and expected output shape. No "TODO" / "TBD" / "add appropriate error handling" patterns.
- **Type consistency:**
  - `Terms` struct fields match between Go (`internal/schema/terms.go` + `internal/catalog/lookup.go`) and Python (`pipeline/normalize/terms.py`): `commitment, tenancy, os, support_tier, upfront, payment_option` in that order.
  - `Row` / `Envelope` field names: catalog `Row.ResourceName` → output `Resource.Name`; catalog `Row.RegionGroup` → output `Location.NormalizedRegion`. Intentional renames (both match spec §4 wording).
  - `Preset` values: `PresetAgent`/`PresetFull`/`PresetPrice`/`PresetCompare` match the spec §4 preset names.
  - `Code` values in `skuerrors` match the spec §4 exit-code taxonomy strings.
- **Cross-cutting risk:** the Python ⇄ Go `terms_hash` invariant is the biggest correctness risk in M1; it is locked by the shared golden fixture (`internal/schema/testdata/terms_golden.jsonl`) which is asserted by both `pipeline/tests/test_terms.py` (Task 4) and `internal/schema/terms_test.go` (Task 5). Any future change to the canonical encoding must update the golden file once and both test suites fail fast if the sides drift.
- **Rollover from M0 code review:** Tasks 11 + 12 close all three items (Execute returns int, JSON error envelope on stderr, `newRootCmd` design intent documented in the Task 12 code comment).









