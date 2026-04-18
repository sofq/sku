# sku - Agent-Friendly Cloud & LLM Pricing CLI

**Design Document**
**Date**: 2026-04-18
**Status**: Draft for approval
**Author**: Quan Hoang
**License**: Apache-2.0

---

## 1. Overview & Goals

### What

`sku` is an agent-friendly CLI for querying cloud and LLM pricing across AWS, Azure, Google Cloud, and OpenRouter. Single static binary, JSON-everywhere output, semantic exit codes, token-efficient presets.

### Who

AI agents and scripts that need fast, reliable price lookups. Secondary audience: humans comparing providers from a terminal.

### Why different from existing tools

- Existing cloud pricing tools (Infracost CLI, AWS Pricing Calculator, Vantage UI) target humans running Terraform cost estimates or web dashboards, not agents doing composable CLI operations.
- LLM pricing tools (LiteLLM, tokencost) are libraries, not CLIs, and don't cover cloud infra.
- `sku` is the first tool purpose-built for agents making programmatic price decisions across all four providers with a unified schema and sub-10ms point lookups offline.

### Primary use cases (priority order)

1. **Point lookup**: "what does `m5.large` cost in `us-east-1`?" or "price of `claude-opus-4.6` input tokens via AWS Bedrock"
2. **Cross-provider comparison**: "compare 8-vCPU / 32GB VMs across AWS, Azure, GCP" or "cheapest LLM for long context"
3. **Cost estimation**: given a workload spec, output monthly cost
4. **Catalog browse / discovery**: "list all GPU instance types with >=80GB memory"

### Success criteria

1. Point lookup returns in <10 ms from a cold shell (mmap'd SQLite).
2. Cold install + first query works with zero config (lazy shard fetch on first use).
3. All outputs parseable by `jq` without post-processing.
4. Cross-compile to 6 platform targets (linux/darwin/windows x amd64/arm64) via `goreleaser` on every release.
5. Installable via `brew`, `npm`, `pipx`, `docker`, `scoop`, or direct binary download.
6. Daily data freshness: GitHub Actions daily job publishes deltas; users always within 24h of upstream.
7. Test coverage: >=80% line coverage on `internal/`; e2e tests against real SQLite shards in CI.

### Non-goals (v1)

- Real-time spot-price streaming.
- Cost optimization recommendations (separate tool could build on `sku`'s data).
- Usage billing integration (pulling your actual cloud bill). Pricing only, not spending.
- Terraform/IaC parsing.
- GUI / web UI. CLI only.
- Enterprise SSO, RBAC. Single-user, read-only.

### Roadmap after v1 (non-blocking)

- `sku watch` for price change notifications.
- Reserved/savings-plan optimization queries.
- Additional providers (Oracle Cloud, Alibaba, Cloudflare, Fly.io, Vercel).
- `sku mcp-server` subcommand exposing sku as an MCP tool surface.

---

## 2. Architecture

### High-level components

The system has two decoupled halves: a CI-side data pipeline and a client-side CLI.

**Data pipeline (CI-only)**:
- Fetchers: download raw provider data.
- Ingester: DuckDB-based stream parsing and normalization into Parquet.
- Packager: builds SQLite shards and SQL.gz delta patches for distribution.
- Output: release assets + manifest.json on GitHub Releases, fronted by jsDelivr.

**CLI binary (distributed to users)**:
- `cmd/`: thin Cobra command tree, flag parsing, output rendering.
- `internal/catalog`: SQLite reader and shard manager.
- `internal/updater`: manifest fetch, delta apply, atomic transactions.
- `internal/providers`: `--live` API clients per provider.
- `internal/schema`: kind taxonomy, region map, unified model.
- `internal/compare`: cross-provider equivalence per kind.
- `internal/estimate`: workload DSL and cost math.
- `internal/output`: presets, jq, fields, JSON/YAML/TOML rendering.
- `internal/batch`: multi-operation dispatcher.
- `internal/config`: profile management.

### Package boundaries

| Package | Responsibility | Depends on |
|---|---|---|
| `cmd/` | Cobra command tree, flag parsing, output rendering | `internal/*`, cobra, color, gojq |
| `internal/catalog` | Open/mmap SQLite shards, execute queries, shard paths | `modernc.org/sqlite` |
| `internal/updater` | Fetch manifest, compute diffs, apply SQL.gz deltas atomically | stdlib net/http, crypto/sha256, sqlite |
| `internal/providers/{aws,azure,gcp,openrouter}` | `--live` API calls and normalization | stdlib net/http |
| `internal/schema` | Unified schema definitions, kind taxonomy, validation | - |
| `internal/compare` | Cross-provider matching logic | `internal/catalog` |
| `internal/estimate` | Workload DSL parsing and cost math | `internal/catalog` |
| `internal/config` | Profile management, config file I/O | stdlib |
| `internal/output` | Preset rendering, `--jq`, `--fields`, `--pretty`, error envelope | `github.com/itchyny/gojq` |
| `internal/batch` | NDJSON/array of ops, dispatch, aggregate exit code | `cmd/` registry |
| `internal/errors` | Typed error to exit-code mapping | - |
| `gen/` | Code-generated per-kind validators | (build-time only) |

### Design principles

- **Pure Go, no CGO anywhere**. Every dependency is pure-Go (`modernc.org/sqlite`, `gojq`, `cobra`). goreleaser cross-compiles to 6 targets without Docker tricks.
- **`cmd/` stays thin**. No business logic; flag parsing and calls into `internal/`. Enables batch execution in-process (no shell exec).
- **Provider logic isolated**. `--live` fallback is the only place that hits real APIs; everywhere else goes through `internal/catalog`. Unit tests don't need network.
- **Updater independent of reader**. You can `sku update` without loading the query path, and vice versa.
- **Output renderer is one choke point**. Presets, `--jq`, `--fields`, `--pretty` all implemented once, applied to every command's result.

### Data flow per use case

1. **Point lookup**: `cmd/price` -> `internal/catalog.Get(key)` -> SQLite indexed query -> `internal/output.Render` -> stdout JSON.
2. **Cross-provider compare**: `cmd/compare` -> `internal/compare.Match(spec)` -> N shards queried in parallel -> unified results sorted by price.
3. **Estimate**: `cmd/estimate` -> `internal/estimate.Parse(spec)` -> N lookups + math -> aggregated JSON.
4. **Search**: `cmd/search` -> `internal/catalog.Query(filter)` -> SQLite WHERE-driven scan -> JSON array.

---

## 3. Data Pipeline

### Format stack

| Stage | Tool | Format | Rationale |
|---|---|---|---|
| Fetch | curl / aria2c / Go net/http | raw JSON(.gz) | Static S3 downloads or paginated API calls |
| Stream parse + normalize | **DuckDB** (engine) | -> Parquet | `read_json_auto` handles 20 GB streaming with constant memory; SQL normalization replaces hundreds of lines of parser code |
| Intermediate (CI cache) | - | **Parquet (zstd)** | Columnar, 10-30% smaller than SQLite, native DuckDB I/O, great for diffs |
| Diff yesterday vs today | **DuckDB** (engine) | Parquet in, Parquet out | `EXCEPT` / hash-join, seconds on 3M rows |
| Build distribution artifact | DuckDB / sqlite3 | **SQLite** | Pure-Go driver, B-tree indexes optimal for composite point lookups |
| Delta patches | bash + sqlite3 | **SQL.gz** | Boring, self-describing, zero client-side deps |
| Client reader | `modernc.org/sqlite` | SQLite | No CGO, clean cross-compile |

**Not vendoring Infracost scrapers.** Writing pure DuckDB SQL scripts instead. Reasoning: Infracost's scrapers are Node.js to MongoDB; adapting them would add Node + MongoDB dump + conversion to CI. Their schema is opinionated for their GraphQL API. DuckDB `read_json_auto` on the raw provider files is actually simpler. We still read their code as a learning resource for edge cases (e.g., AWS RDS multi-AZ pricing quirks), but don't vendor or copy.

**LiteLLM pricing JSON dropped.** OpenRouter's two free endpoints (`/api/v1/models` and `/api/v1/models/{id}/endpoints`) cover all LLM pricing including per-serving-provider breakdowns plus health metrics. Always current, no extra dependency.

### Shard granularity

- **Provider + service** is the sweet spot (~40 shards): manageable manifest, cleans CI matrix fan-out, per-shard deltas typically <1 MB, most users download only what they need.
- **Split commitment pricing** into separate shards (`aws-ec2-commitments`, `azure-vm-reservations`, `gcp-gce-cud`) because reserved/savings-plan pricing multiplies row counts ~20x. 80% of agent queries are on-demand + spot.
- Single `llm` shard (no split by serving provider); `provider` column discriminates within.

### Target shard inventory

**AWS (~20 shards, ~250 MB full install):**
- `aws-ec2` + `aws-ec2-commitments`
- `aws-rds` + `aws-rds-commitments`
- `aws-elasticache` + `aws-elasticache-commitments`
- `aws-lambda`, `aws-s3`, `aws-ebs`, `aws-dynamodb`, `aws-cloudfront`, `aws-route53`, `aws-sqs`, `aws-sns`, `aws-apigateway`, `aws-cloudwatch`, `aws-kms`, `aws-secretsmanager`, `aws-eks`

**Azure (~12 shards, ~80 MB full):**
- `azure-vm` + `azure-vm-reservations`
- `azure-sql` + `azure-sql-reservations`
- `azure-functions`, `azure-blob`, `azure-disks`, `azure-aks`, `azure-cosmosdb`, `azure-cdn`, `azure-keyvault`, `azure-appservice`

**GCP (~10 shards, ~30 MB full):**
- `gcp-gce` + `gcp-gce-cud`
- `gcp-cloud-sql` + `gcp-cloud-sql-cud`
- `gcp-gcs`, `gcp-run`, `gcp-functions`, `gcp-bigquery`, `gcp-gke`, `gcp-cloud-cdn`

**LLM**: `openrouter` (single shard, <200 KB).

**Total**: ~45 shards, ~360 MB full install, ~5-20 MB typical agent.

### Update flow (daily)

```
03:00 UTC (cron)
  |
  |-> Job 1: discover (1 runner, ~2 min)
  |     - fetch provider version indices
  |     - diff vs yesterday's indices (cached)
  |     - emit changed-shards.json; exit early on empty days
  |
  |-> Job 2: ingest (matrix over changed shards, up to 20 parallel runners)
  |     per shard:
  |       1. stream-fetch raw JSON
  |       2. DuckDB: read_json_auto -> normalize -> Parquet (zstd)
  |       3. upload Parquet as workflow artifact
  |
  |-> Job 3: diff + package (matrix, same shape as ingest)
  |     per shard:
  |       1. download today's Parquet + restore yesterday's from actions/cache
  |       2. DuckDB: EXCEPT join -> upserts + deletes
  |       3. render as SQL.gz delta
  |       4. if day-of-month in {1, 15}: rebuild full SQLite baseline
  |
  |-> Job 4: validate (1 runner, ~3 min)
  |     - sample 100 random SKUs per shard
  |     - query live provider API, compare values (1% tolerance)
  |     - diff vs vantage-sh/ec2instances.info for EC2 cross-check
  |     - on drift >1%, fail release
  |
  |-> Job 5: publish (1 runner, ~1 min)
        - gh release create data-YYYY.MM.DD with all deltas + baselines
        - update data/manifest.json via bot commit
        - jsDelivr CDN picks up automatically
```

Total daily runtime: 20-30 min wall-clock, 0-15 min on quiet days. Monthly CI: ~5-10 hours (free for public repo). User bandwidth: manifest ETag (<1 KB on quiet days) plus changed shard deltas (~10 KB to 1 MB per shard).

### Manifest structure (`data/manifest.json`)

```json
{
  "schema_version": 1,
  "generated_at": "2026-04-18T03:15:00Z",
  "catalog_version": "2026.04.18",
  "shards": {
    "aws-ec2": {
      "baseline_version": "2026.04.01",
      "baseline_url": "https://github.com/quanhoang/sku/releases/download/data-2026.04.01/aws-ec2.db.zst",
      "baseline_sha256": "ab3d...",
      "baseline_size": 62914560,
      "head_version": "2026.04.18.01",
      "deltas": [
        {"from": "2026.04.01", "to": "2026.04.02.01", "url": "...", "sha256": "...", "size": 12345}
      ],
      "row_count": 1289453,
      "last_updated": "2026-04-18T03:12:00Z"
    }
  }
}
```

### Client `sku update` flow

1. GET `https://cdn.jsdelivr.net/gh/quanhoang/sku/data/manifest.json` (ETag cached, typically 304).
2. For each installed shard: if `local.head_version < remote.head_version`, walk delta chain, download each `.sql.gz`, verify sha256, apply in one SQLite transaction.
3. On failure: rollback; last-known-good state preserved.

### Reliability

- **Idempotent stages**: deterministic hashes, re-run safe.
- **Signed releases**: `actions/attest-build-provenance` and cosign.
- **Stale data warning**: `sku` commands warn on stderr if catalog >14 days old; `--stale-ok` suppresses. Exit code 8 if warning becomes an error.
- **Graceful degradation**: per-shard failure isolated; other shards still work.
- **Live fallback**: `--live` bypasses cache and hits provider API for on-demand freshness.

### OpenRouter-specific ingest

OpenRouter covers all LLM pricing via two free endpoints (no auth):

1. `GET /api/v1/models`: base pricing, context_length, architecture (modality), supported_parameters, top_provider.
2. `GET /api/v1/models/{author}/{slug}/endpoints`: per-serving-provider pricing, context/max_completion, quantization, health metrics (uptime, latency, throughput).

Total rows: ~300 models x ~2-4 endpoints each = ~1K rows. Shard size <200 KB.

One row per (model, serving_provider) pair plus a synthetic row for `provider="openrouter"` representing the routed aggregated rate.

---

## 4. CLI Surface

### Command tree

```
sku
 |- configure                    # profile setup (interactive or flagged)
 |- update                       # fetch latest manifest + deltas
 |- schema                       # discovery: providers, services, flags
 |- price                        # generic point-lookup verb
 |- search                       # list SKUs matching filter
 |- compare                      # cross-provider comparison
 |- estimate                     # workload cost estimation
 |- batch                        # multi-operation from stdin
 |- version                      # JSON version info
 |
 |- aws
 |   |- ec2 {price, list}
 |   |- rds {price, list}
 |   |- s3 {price, list}
 |   |- lambda {price, list}
 |   |- ... (auto-registered from installed shards)
 |
 |- azure
 |   |- vm, sql, blob, functions, disks, ...
 |
 |- gcp
 |   |- gce, gcs, run, cloud-sql, ...
 |
 |- llm                          # cross-cutting LLM verbs
 |   |- price
 |   |- list
 |   |- compare
 |
 |- anthropic, openai, aws-bedrock, gcp-vertex, azure-openai, openrouter
     |- llm {price, list}        # serving-provider-scoped views
```

Provider subcommands are **auto-registered** from the installed shard list. Missing shard -> subcommand doesn't appear in help. Keeps surface clean and discoverable.

### Global flags

```
--profile <name>             named config profile (default "default")
--preset <name>              agent | full | price | compare (default agent)
--jq <expr>                  jq filter on response
--fields <list>              comma-separated field projection
--include-raw                include "raw" passthrough object
--pretty                     pretty-print JSON (default compact)
--live                       bypass cache, hit provider API
--stale-ok                   suppress stale-catalog warning
--timeout <duration>         HTTP timeout for --live (default 30s)
--cache <duration>           per-call cache TTL override
--dry-run                    show query plan without executing
--verbose                    stderr JSON log
--no-color                   disable color
--json | --yaml | --toml     output format (default json)
```

### Representative commands

**Point lookup:**

```bash
sku aws ec2 price \
  --instance-type m5.large \
  --region us-east-1 \
  --os linux \
  --tenancy shared \
  --commitment on_demand
```

**Verb-first equivalent:**

```bash
sku price --provider aws --service ec2 \
  --instance-type m5.large --region us-east-1
```

**Cross-provider compare:**

```bash
sku compare --kind compute.vm \
  --vcpu 8 --memory 32 \
  --regions "us-east-1,eastus,us-east1" \
  --sort price --limit 5
```

**LLM queries:**

```bash
# all serving options for a model
sku llm price --model "anthropic/claude-opus-4.6"

# filter + sort by price dimension
sku llm list \
  --capability tools --capability vision \
  --min-context 200000 \
  --max-price-prompt 5 \
  --min-uptime 0.99 \
  --sort price.completion

# compare models for a workload pattern
sku llm compare \
  --workload "1M prompt + 500K completion + 100 requests/day" \
  --capability tools --sort daily_cost
```

**Estimate (three input forms):**

```bash
# inline
sku estimate \
  --item aws/ec2:m5.large:count=10:hours=730 \
  --item aws/s3:standard:gb=500:put_requests=1M \
  --item llm:anthropic/claude-opus-4.6:input=1M:output=500K:requests=1000

# config file
sku estimate --config workload.yaml

# stdin
echo '{"items":[...]}' | sku estimate --stdin
```

**Search / discovery:**

```bash
sku search \
  --provider aws --service ec2 \
  --kind compute.vm \
  --min-vcpu 8 --min-memory 32 \
  --max-price 1.0 --region us-east-1 \
  --sort price
```

**Schema introspection:**

```bash
sku schema                         # all providers + services
sku schema --list                  # flat shard names
sku schema aws                     # all services for AWS
sku schema aws ec2                 # filter flags + allowed values
sku schema aws ec2 price           # full operation signature with examples
sku schema --format json           # machine-readable
```

**Batch:**

```bash
echo '[
  {"command": "aws ec2 price", "args": {"instance_type": "m5.large", "region": "us-east-1"}, "jq": ".price[0].amount"},
  {"command": "compare", "args": {"kind": "compute.vm", "vcpu": 8, "memory": 32}, "preset": "compare"},
  {"command": "llm price", "args": {"model": "anthropic/claude-opus-4.6", "serving_provider": "aws-bedrock"}}
]' | sku batch
```

Returns array of `{"index", "exit_code", "output"}`. Global exit code = highest severity across ops.

**Update:**

```bash
sku update                        # update installed + lazy-fetch on demand
sku update --install core         # compute + storage + LLMs, on-demand only
sku update --install full         # full catalog (~150 MB)
sku update aws-ec2 azure-vm       # targeted
sku update --channel stable       # weekly baselines only
sku update --status               # show installed shards + versions
```

### Output schema

```jsonc
{
  "provider": "aws",
  "service": "ec2",
  "sku_id": "JRTCKXETXF...",
  "resource": {
    "kind": "compute.vm",
    "name": "m5.large",
    "vcpu": 2,
    "memory_gb": 8,
    "storage_gb": null,
    "gpu_count": 0,
    "attributes": {
      "architecture": "x86_64",
      "network_performance": "Up to 10 Gigabit"
    }
  },
  "location": {
    "provider_region": "us-east-1",
    "normalized_region": "us-east",
    "availability_zone": null
  },
  "price": [
    {"amount": 0.096, "currency": "USD", "unit": "hour", "dimension": "compute", "tier": null}
  ],
  "terms": {
    "commitment": "on_demand",
    "tenancy": "shared",
    "os": "linux",
    "support_tier": null,
    "upfront": null,
    "payment_option": null
  },
  "health": null,
  "source": {
    "catalog_version": "2026.04.18",
    "fetched_at": "2026-04-18T03:12:00Z",
    "upstream_id": "aws-pricing-api@v1.0",
    "freshness": "daily"
  },
  "raw": null
}
```

**`price` is always an array** to accommodate multi-dimensional pricing (LLMs: prompt/completion/cache; S3: storage/request/egress). Cloud VMs simply carry one element.

**`health`** is first-class (null for cloud SKUs). LLM rows populate `uptime_30d`, `latency_p50_ms`, `latency_p95_ms`, `throughput_tokens_per_sec`, `observed_at`.

**`raw`** preserves provider-native attributes, included only with `--include-raw` or preset `full`.

### Presets

| Preset | Fields kept | Typical size | Use case |
|---|---|---|---|
| `agent` (default) | provider, service, resource.name, location.provider_region, price, terms.commitment | ~200 bytes | Agent default, minimum tokens |
| `price` | price only | ~50 bytes | One-line lookups |
| `full` | Everything including raw | 2-10 KB | Human inspection, debugging |
| `compare` | provider, resource.name, resource.vcpu, resource.memory_gb, price, location.normalized_region | ~150 bytes | Cross-provider rows |

`--jq` and `--fields` apply after preset selection for further trimming.

### Error envelope (stderr JSON)

```json
{
  "error": {
    "code": "not_found",
    "message": "No SKU matches filters",
    "suggestion": "Try `sku schema aws ec2` to see valid filters",
    "details": {
      "provider": "aws",
      "service": "ec2",
      "applied_filters": {"instance_type": "m5.huge", "region": "us-east-1"}
    }
  }
}
```

**Exit codes**: 0=ok, 1=generic_error, 2=auth, 3=not_found, 4=validation, 5=rate_limited, 6=conflict, 7=server, 8=stale_data.

### Configuration (`~/.config/sku/config.yaml`)

```yaml
profiles:
  default:
    preset: agent
    channel: daily
    cache_ttl: 24h
    default_regions: [us-east-1, eastus, us-east1]
    default_currency: USD
    live_api_keys:
      openrouter: $OPENROUTER_API_KEY
    stale_warning_days: 14
  cost-planning:
    preset: full
    include_raw: true
  ci:
    preset: price
    cache_ttl: 0
    stale_warning_days: 3
```

---

## 5. Data Schema

### SQLite shard schema (uniform across all shards)

```sql
CREATE TABLE skus (
  sku_id             TEXT    NOT NULL,
  provider           TEXT    NOT NULL,
  service            TEXT    NOT NULL,
  kind               TEXT    NOT NULL,
  resource_name      TEXT    NOT NULL,
  region             TEXT,
  region_normalized  TEXT,
  terms_hash         TEXT    NOT NULL,
  PRIMARY KEY (sku_id, provider, region, terms_hash)
) WITHOUT ROWID;

CREATE TABLE resource_attrs (
  sku_id             TEXT    NOT NULL,
  vcpu               INTEGER,
  memory_gb          REAL,
  storage_gb         REAL,
  gpu_count          INTEGER,
  gpu_model          TEXT,
  architecture       TEXT,
  context_length     INTEGER,
  max_output_tokens  INTEGER,
  modality           TEXT,          -- JSON array
  capabilities       TEXT,          -- JSON array
  quantization       TEXT,
  durability_nines   INTEGER,
  availability_tier  TEXT,
  extra              TEXT,          -- JSON
  PRIMARY KEY (sku_id),
  FOREIGN KEY (sku_id) REFERENCES skus(sku_id)
) WITHOUT ROWID;

CREATE TABLE terms (
  sku_id             TEXT    NOT NULL,
  commitment         TEXT    NOT NULL,
  tenancy            TEXT,
  os                 TEXT,
  support_tier       TEXT,
  upfront            TEXT,
  payment_option     TEXT,
  PRIMARY KEY (sku_id),
  FOREIGN KEY (sku_id) REFERENCES skus(sku_id)
) WITHOUT ROWID;

CREATE TABLE prices (
  sku_id             TEXT    NOT NULL,
  dimension          TEXT    NOT NULL,
  tier               TEXT,
  amount             REAL    NOT NULL,
  currency           TEXT    NOT NULL,
  unit               TEXT    NOT NULL,
  PRIMARY KEY (sku_id, dimension, tier),
  FOREIGN KEY (sku_id) REFERENCES skus(sku_id)
) WITHOUT ROWID;

CREATE TABLE health (
  sku_id                     TEXT    PRIMARY KEY,
  uptime_30d                 REAL,
  latency_p50_ms             INTEGER,
  latency_p95_ms             INTEGER,
  throughput_tokens_per_sec  REAL,
  observed_at                INTEGER,
  FOREIGN KEY (sku_id) REFERENCES skus(sku_id)
) WITHOUT ROWID;

CREATE TABLE raw (
  sku_id   TEXT PRIMARY KEY,
  payload  TEXT NOT NULL,
  FOREIGN KEY (sku_id) REFERENCES skus(sku_id)
) WITHOUT ROWID;

CREATE TABLE metadata (
  key    TEXT PRIMARY KEY,
  value  TEXT
);
```

### Indexes

```sql
CREATE INDEX idx_skus_lookup
  ON skus (provider, service, resource_name, region, terms_hash);
CREATE INDEX idx_resource_compute
  ON resource_attrs (vcpu, memory_gb) WHERE vcpu IS NOT NULL;
CREATE INDEX idx_resource_llm
  ON resource_attrs (context_length) WHERE context_length IS NOT NULL;
CREATE INDEX idx_skus_region
  ON skus (region_normalized, kind, service);
CREATE INDEX idx_prices_by_dim
  ON prices (dimension, amount);
CREATE INDEX idx_terms_commitment
  ON terms (commitment, tenancy, os);
```

### Kind taxonomy

```
compute.vm, compute.function, compute.container, compute.kubernetes, compute.batch
storage.object, storage.block, storage.file, storage.archive
db.relational, db.nosql, db.inmemory, db.warehouse
network.cdn, network.dns, network.loadbalancer
queue.messaging
security.secrets, security.kms
observability.logs, observability.metrics
llm.text, llm.multimodal, llm.embedding, llm.image, llm.audio
```

Each kind has:
- Documented attribute schema enforced at ingest time.
- Its own `compare` equivalence rules in `internal/compare/kinds/`.
- Its own `estimate` workload shape in `internal/estimate/kinds/`.

### Region normalization

Baked into binary (not shards):

```go
var RegionGroups = map[string][]ProviderRegion{
  "us-east":  {aws:us-east-1, aws:us-east-2, azure:eastus, azure:eastus2, gcp:us-east1, gcp:us-east4},
  "us-west":  {...},
  "eu-west":  {...},
  "asia-se":  {...},
  // ~15 groups
}
```

At ingest time, `region_normalized` is populated. Compare queries filter on it.

### SKU ID sources

- **AWS**: `sku` from Pricing API (stable).
- **Azure**: `meterId` UUID (stable).
- **GCP**: `skuId` (stable).
- **OpenRouter**: synthetic composite `{model_id}::{serving_provider}::{quantization}`.

### Currency

All prices normalized to USD at ingest time using a daily-refreshed ECB-style rate. `currency` column always "USD" in v1; schema supports multi-currency expansion.

### Schema versioning

- `metadata.schema_version` per shard.
- Binary refuses to read if shard schema is out of its supported range.
- Error messages direct user to upgrade binary or run `sku update`.

### Performance targets

- Point lookup: <3 ms p99 including JSON render.
- Cross-provider compare (3 providers): <50 ms p99.
- Catalog open + first query cold: <20 ms.
- Delta apply (10K rows): <2 s.

---

## 6. Testing Strategy

### Pyramid

```
     e2e    (15 tests, real binary, real shards, ~5-10s)
    int     (80 tests, sqlite + fixtures, ~3-5s)
   unit     (500+ tests, pure functions, <1s)
```

### Unit tests

Every non-trivial function in `internal/*`. Table-driven. No mocks unless unavoidable (pass interfaces).

Focus areas:
- `internal/schema`: kind taxonomy, region normalization, currency conversion, SKU ID generation.
- `internal/compare/kinds/*`: cross-provider equivalence per kind.
- `internal/estimate/kinds/*`: workload math, tiered pricing, LLM token math.
- `internal/output`: preset application, jq, fields.
- `internal/errors`: error type to exit code mapping.
- `internal/batch`: parser, dispatch, aggregation.
- `internal/updater`: delta chain, SHA verification, rollback, manifest ETag. Uses `http.RoundTripper` fake.
- `internal/catalog`: query builder, filter composition. In-memory SQLite fixtures.
- `cmd/*`: flag parsing edge cases, help generation, preset auto-selection.

Gate: **>=80% line coverage on `internal/`**.

### Integration tests (`-tags=integration`)

- Real SQLite files built from `testdata/fixtures/`.
- End-to-end catalog queries with real driver against real files (not `:memory:`).
- Update flow: baseline + sequential deltas + final-state assertion.
- Schema migration: v1 DB with v2 binary -> graceful refusal; v2 DB with v1 binary -> version-mismatch error.
- Corruption recovery: truncated shard -> clean error, no panic.

### E2E tests (`e2e_test.go`, `-tags=e2e`)

Pattern borrowed from jira-cli. Build real binary, exec as subprocess, assert stdout/stderr/exit.

Canonical flows covered:
- point lookup per provider
- cross-provider compare
- estimate (all three input forms)
- batch (three operations)
- update (apply delta)
- not-found returns 3 with suggestion
- `--live` skips cache
- stale catalog warns unless override
- preset `full` includes raw
- `--jq` reduces output
- schema lists installed shards
- bad JSON goes to stderr
- validation error returns 4
- configure creates profile
- offline update retains last-known-good state

### Data pipeline tests

Separate test suite under `pipeline/testdata/`:
- **Ingest golden tests**: raw JSON fixture -> DuckDB SQL -> assert output Parquet matches golden.
- **Diff golden tests**: (yesterday, today) Parquet -> assert patch SQL matches golden.
- **Delta apply round-trip**: apply fixture delta to baseline, dump DB, assert equals hand-built expected.
- **Schema consistency**: every shard's rows satisfy schema (PRAGMA foreign_keys=ON + CHECK constraints).

### Validation harness (CI production gate)

Daily release pipeline stage:
- Per changed shard: sample 100 random SKUs.
- Fetch live price from provider API.
- Assert `|catalog_price - live_price| / live_price < 0.01`.
- >1% mismatch -> fail release, auto-file issue.
- Weekly EC2 cross-check vs `vantage-sh/ec2instances.info`.

### Property-based tests

- Delta apply idempotent: `apply(apply(db, delta)) == apply(db, delta)`.
- Preset projection stable under re-application.
- Estimator linearity: `estimate([A, B]) == estimate([A]) + estimate([B])`.
- Region normalization symmetry.

### Performance benchmarks with CI regression gate

Go `testing.B` benchmarks on hot paths. CI runs on consistent runner. Regression gate fails PR if any key bench >15% worse vs `main` baseline (benchstat).

### Fuzz tests

- `FuzzFlagParse` in `cmd/` (30s per PR).
- `FuzzBatchParse` for NDJSON parser.
- JQ expression parsing via `gojq`'s existing corpus.

### Compatibility matrix

Go 1.22 and 1.23 x (linux/amd64, linux/arm64, darwin/amd64, darwin/arm64, windows/amd64) in CI. Windows/arm64 binary is produced by goreleaser but not covered in the test matrix (cross-compile only; user reports catch bugs). E2E runs on linux + darwin amd64 only.

### Test data hygiene

- Fixture data <50 KB total per package.
- Deterministic (fixed DuckDB ordering).
- Golden update: `UPDATE_GOLDEN=1 go test`, reviewers inspect diff.
- No network in unit/integration; `internal/transport.RoundTripper` interface.

### Test helpers (`test/testhelp/`)

- `sku.go`: subprocess runner with tempdir + env scrubbing.
- `catalog.go`: seed fixture shards in temp SQLite.
- `http.go`: `RoundTripperFunc` for stubbing provider APIs.
- `workload.go`: canned workload specs.

---

## 7. CI/CD & Release

### Two pipelines, independent

| Pipeline | Trigger | Frequency | Artifact | Version scheme |
|---|---|---|---|---|
| Code releases | Git tag `v*.*.*` | On demand | Binaries + packages | SemVer (`v1.2.3`) |
| Data releases | Cron | Daily (deltas), twice a month (baselines) | Shards + deltas | CalVer (`data-2026.04.18`) |

Decoupled: binary `v1.2.3` reads any catalog with `schema_version` in its supported range. Fresh data doesn't require a binary release.

### Workflows (`.github/workflows/`)

- `ci.yml`: PR + push to main. lint (golangci-lint), test (5-OS matrix, Go 1.22 + 1.23), integration, e2e, benchmark regression, build smoke. All required before merge.
- `release.yml`: on tag `v*.*.*`. goreleaser builds binaries, signs with cosign, generates SBOM, publishes to GH Releases + Homebrew tap + Scoop bucket + npm + PyPI + GHCR.
- `data-daily.yml`: cron 03:00 UTC. Discover -> ingest (matrix) -> diff+package (matrix) -> validate -> publish.
- `data-baseline.yml`: cron 1st and 15th of month. Full shard rebuild.
- `data-validate.yml`: weekly cross-check vs external sources.
- `security.yml`: weekly govulncheck, CodeQL, dependency audit.
- `docs.yml`: on push to main. Regenerate command docs, publish GH Pages.

### goreleaser (`.goreleaser.yml`)

- `CGO_ENABLED=0`.
- Targets: linux/macos/windows x amd64/arm64 (6 total). Windows/arm64 is published to GH Releases; Scoop manifest may skip arm64 if bucket convention dictates, no impact on binary availability.
- Archives: tar.gz on linux/mac, zip on windows.
- Checksum file signed with cosign.
- SBOM via syft (SPDX).
- Homebrew tap at `quanhoang/homebrew-tap`.
- Scoop bucket at `quanhoang/scoop-bucket`.
- Docker multi-arch at `ghcr.io/quanhoang/sku`.
- nfpms (optional deb/rpm/apk).
- npm publish via release workflow step.
- PyPI publish via release workflow step.

### npm wrapper (`npm/`)

Mirrors jira-cli pattern. `install.js` detects platform, downloads correct binary from GH release. `bin/sku` shim execs the downloaded binary. Published as `@quanhoang/sku`.

### PyPI wrapper (`python/`)

`setup.py` post-install hook downloads binary. Published as `sku-cli`. Command: `sku`.

### Docker image (`Dockerfile.goreleaser`)

```dockerfile
FROM alpine:3.19
RUN apk add --no-cache ca-certificates
COPY sku /usr/local/bin/sku
USER 65534:65534
ENTRYPOINT ["/usr/local/bin/sku"]
CMD ["--help"]
```

Final image ~20 MB. Non-root. GHCR (free for public).

### Supply chain

- SLSA L3 provenance via `actions/attest-build-provenance`.
- Cosign keyless signing (OIDC).
- Syft SBOM attached to each release.
- Dependabot + Renovate for dep + GH Action SHA updates.

### Versioning policy

- Patch: bugfixes, data-schema backward compatible. Weekly as needed.
- Minor: new features, new commands, new shards. Bi-weekly target.
- Major: breaking CLI or schema. Expect <= 1/year.
- Pre-releases (`-rc.N`) skip brew/npm/pypi (goreleaser `prerelease` flag).

### Release checklist (`docs/contributing/RELEASING.md`)

1. Merge intended PRs to main, CI green.
2. Update `CHANGELOG.md` with human summary.
3. `git tag -s vX.Y.Z -m "..."` and push.
4. Verify GH release, brew tap, npm, pypi, GHCR.
5. Update install docs if distribution changed.
6. Announce in `docs/news/` and README.

### Failure handling

- Build fails mid-matrix: no release created; fix and retag.
- Distribution channel fails: partial release; manual re-run failing step.
- Bad data: `--channel stable` remains unaffected; mark broken daily release as pre-release, publish corrected, users auto-roll forward.
- Critical bug: patch release within hours; brew/npm/scoop pick it up automatically.

### Observability

- `docs/releases.md` auto-generated table.
- `data/manifest.json` + `sku update --status` surface freshness.
- GH release download counts as README badge.

---

## 8. Repository Layout

```
sku/
|-.github/ (workflows, issue/PR templates, dependabot, renovate)
|-cmd/
|   |-sku/ (main.go, root.go, version.go, configure.go, update.go,
|            schema.go, price.go, search.go, compare.go, estimate.go,
|            batch.go, provider_registry.go,
|            providers/{aws,azure,gcp,openrouter}.go)
|-internal/
|   |-catalog/
|   |-updater/
|   |-providers/{aws,azure,gcp,openrouter}
|   |-schema/
|   |-compare/{compare.go, kinds/{vm,storage_object,db_relational,llm}.go}
|   |-estimate/{estimate.go, parser.go, kinds/{vm,storage_object,llm}.go}
|   |-output/
|   |-batch/
|   |-config/
|   |-errors/
|   |-version/
|-gen/
|   |-schema/ (generated kind validators)
|-pipeline/
|   |-discover/, ingest/ (SQL per shard + openrouter.py), normalize/,
|    diff/, package/, validate/, manifest/, Makefile, pyproject.toml, testdata/
|-npm/ (wrapper package)
|-python/ (wrapper package)
|-docs/ (getting-started, commands/, guides/, reference/, contributing/,
|         superpowers/specs/)
|-website/ (Docusaurus or MkDocs)
|-test/ (testhelp/, testdata/)
|-skill/using-sku/SKILL.md
|-data/manifest.json (tracked in repo)
|-examples/ (estimate-workloads/*.yaml, batch-queries.ndjson)
|-e2e_test.go
|-main.go (root shim)
|-go.mod, go.sum
|-.golangci.yml, .goreleaser.yml, .dockerignore, .gitignore
|-Dockerfile, Dockerfile.goreleaser
|-Makefile
|-LICENSE (Apache-2.0), NOTICE
|-SECURITY.md, CODE_OF_CONDUCT.md, CHANGELOG.md
|-CLAUDE.md (agent guide)
|-README.md
```

### Module path

`github.com/quanhoang/sku`. Binary name: `sku`.

### Go dependencies (intentionally minimal)

- `github.com/spf13/cobra` CLI
- `modernc.org/sqlite` pure-Go SQLite
- `github.com/itchyny/gojq` pure-Go jq
- `github.com/klauspost/compress/zstd` pure-Go zstd
- `github.com/stretchr/testify` test assertions
- `golang.org/x/term` TTY detection
- `gopkg.in/yaml.v3` YAML parsing

Excluded: AWS/Azure/GCP SDKs (raw HTTP for `--live`), DuckDB/Parquet libs (CI-side only).

### Makefile targets

```
# Development
make dev           go build + run smoke tests
make build         go build -o bin/sku ./cmd/sku
make test          unit + integration
make test-e2e      builds binary + runs e2e
make bench         benchmarks
make lint          golangci-lint
make generate      codegen + docs
make docs          regenerate command docs from cobra
make clean

# Pipeline
make -C pipeline ingest SHARD=aws-ec2
make -C pipeline diff-package SHARD=aws-ec2
make -C pipeline validate
make -C pipeline rebuild-baselines

# Release (usually run by goreleaser)
make release-dry   goreleaser release --snapshot --clean
```

### Repo metadata

- Branch protection on `main`: CI green required, 1 review, no force-push, signed commits preferred.
- CODEOWNERS with provider/kind sub-owners if contributors appear.
- Apache-2.0 LICENSE + NOTICE at root.

---

## 9. Rollout & Milestones

### Milestones to v1.0 (~12-14 weeks solo)

**M0 - Foundations (1 week):**
Repo scaffolded, `go.mod` + Makefile + goreleaser dry-run + CI workflow green on 5 OSes. `sku version` returns JSON.

**M1 - Single-shard pipeline + catalog reader (2 weeks):**
OpenRouter shard ingestion (Python + DuckDB). Full SQLite schema. `internal/catalog` point lookup. `sku llm price` working. Output JSON, `--pretty`, basic error codes.
*Exit criteria: `sku update` downloads OpenRouter shard; `sku llm price --model anthropic/claude-opus-4.6` returns correct JSON in <10 ms.*

**M2 - Output polish + CLI ergonomics (1.5 weeks):**
All presets, `--jq`, `--fields`, all exit codes, `sku schema` discovery, config profiles, `sku configure`, stale warnings, shell completions.
*Exit criteria: all global flags work; preset token sizes correct; schema discovery machine-readable.*

**M3 - AWS + Azure + GCP core shards (3 weeks):**
AWS (ec2, rds, s3, lambda, ebs, dynamodb, cloudfront), Azure (vm, sql, blob, functions, disks), GCP (gce, cloud-sql, gcs, run, functions). Updater module with delta apply, atomic transactions, offline safety. Daily data pipeline live in CI. Validation harness.
*Exit criteria: ~20 shards live; `sku update` fetches all; point lookups work cross-provider; validation harness fails intentionally-broken data.*

**M4 - Compare + search (1.5 weeks):**
Region normalization map. Per-kind equivalence (vm, storage_object, db_relational, llm). `sku compare` and `sku search`. Parallel shard fan-out.
*Exit criteria: cross-provider VM compare <50 ms; LLM compare across serving providers; search supports all common filters.*

**M5 - Estimate + batch + more shards (2 weeks):**
Inline `--item` DSL, YAML config, stdin JSON. Per-kind estimators. `sku batch` NDJSON/array. Remaining AWS/Azure/GCP shards.
*Exit criteria: full shard coverage per Section 2; estimate works all three input forms; batch handles 50-op test with correct aggregated exit code.*

**M6 - Distribution channels (1 week):**
Homebrew tap, Scoop bucket, npm wrapper, PyPI wrapper, Docker multi-arch. Cosign signing + SLSA provenance. First pre-release `v0.1.0`.
*Exit criteria: `brew install`, `npx`, `pipx install`, `docker run`, `scoop install` all succeed on fresh machines.*

**M7 - Docs, polish, v1.0 (1.5 weeks):**
Complete docs (getting-started, command reference, guides, kinds, presets, exit codes, agent-integration, llm-routing, offline-use). Website. `skill/using-sku/SKILL.md`. Examples. Security (SECURITY.md, govulncheck clean). Bench baseline + regression gate. CHANGELOG. Tag `v1.0.0`.
*Exit criteria: new user installs, queries, updates, batches, estimates within 5 minutes of landing on README. Agents have documented machine-readable schema.*

### Effort summary

| Milestone | Time | Biggest risk |
|---|---|---|
| M0 | 1 wk | None |
| M1 | 2 wk | Catalog reader correctness; establishes patterns |
| M2 | 1.5 wk | gojq integration edge cases |
| M3 | 3 wk | AWS EC2 JSON quirks, RDS multi-AZ pricing, currency |
| M4 | 1.5 wk | Region equivalence edge cases |
| M5 | 2 wk | Estimate DSL + per-kind math |
| M6 | 1 wk | Cross-platform install quirks |
| M7 | 1.5 wk | Doc quality |

### Post-v1 roadmap

**v1.1** (~2 months after v1.0): `sku watch`, commitment shards if deferred, `sku mcp-server`, audit logging, `--currency` native conversion.

**v1.2**: Oracle Cloud / Alibaba / Fly.io / Vercel / Cloudflare Workers providers, `sku recommend`, CSV/TSV output.

**v2.0** (breaking): full multi-currency, persistent semantic cache, historical price tracking.

### First week (M0) concrete TODOs

1. `git init`, repo at `github.com/quanhoang/sku`.
2. Initial commit with LICENSE (Apache-2.0), README placeholder, `.gitignore`, `go.mod` (Go 1.23).
3. `Makefile` with build, test, lint, clean, generate, release-dry.
4. `.golangci.yml` (adapt from jira-cli).
5. `.goreleaser.yml` minimal: Linux/Mac/Windows amd64/arm64, tar.gz/zip, checksums.
6. `cmd/sku/main.go`, `cmd/sku/root.go`, `cmd/sku/version.go` emitting JSON version.
7. `main.go` at root shim.
8. `.github/workflows/ci.yml`: lint + test + build smoke on 5-OS matrix.
9. `.github/workflows/release.yml` goreleaser template, dry-run green.
10. `CLAUDE.md` at root (quick-start, patterns, dev commands).

After M0, every subsequent milestone plugs into this spine.

---

## Appendix A: References

- [IBM-Cloud/infracost-cloud-pricing-api](https://github.com/IBM-Cloud/infracost-cloud-pricing-api) - Apache-2.0; referenced for AWS/Azure/GCP scraper logic and edge-case handling; not vendored.
- [vantage-sh/ec2instances.info](https://github.com/vantage-sh/ec2instances.info) - MIT; cross-check source for EC2 validation.
- [OpenRouter Models API](https://openrouter.ai/docs/api/api-reference/models/get-models) - LLM data source.
- [DuckDB](https://duckdb.org/) - ingest engine.
- [modernc.org/sqlite](https://modernc.org/sqlite) - pure-Go SQLite driver.
- [goreleaser](https://goreleaser.com/) - release automation.
- [jira-cli (jr)](https://github.com/quanhoang/jira-cli) (user's reference) - CLI shape, npm/python wrapper pattern, e2e test shape.

## Appendix B: Open questions deferred to planning

- Final binary name if `sku` conflicts on any registry (PyPI/npm uniqueness check before M0 ends).
- Whether `--currency` native conversion ships in v1 or v1.1.
- Whether Scoop manifest lives in a separate repo or the main repo.
- Exact cadence for `sku-mcp-server` (v1.1 vs v1.0).
- Whether to ship `apt`/`yum` repos beyond nfpms packages.
