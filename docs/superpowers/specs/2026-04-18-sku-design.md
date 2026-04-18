# sku - Agent-Friendly Cloud & LLM Pricing CLI

**Design Document**
**Date**: 2026-04-18 (rev 4, third-review fixes applied)
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

1. Point lookup returns in <30 ms warm, <60 ms cold (Go binary startup + pure-Go SQLite open), measured end-to-end including JSON render on linux/amd64 GitHub runners. Firmer targets are set per-path in §5 after an M1 benchmark establishes the pure-Go SQLite baseline.
2. Cold install + first query works with zero config. Missing shards yield an actionable error ("run `sku update aws-ec2`") or auto-fetch when `--auto-fetch` is set; see §4 *Lazy shard fetch*.
3. All outputs parseable by `jq` without post-processing.
4. Cross-compile to 6 platform targets (linux/darwin/windows x amd64/arm64) via `goreleaser` on every release.
5. Installable via `brew`, `npm`, `pipx`, `docker`, `scoop`, or direct binary download.
6. Daily data freshness: GitHub Actions daily job publishes deltas; users always within 24h of upstream once their client refreshes (CDN propagation window noted in §3).
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
- `internal/schema`: kind taxonomy, region map, unified model.
- `internal/compare`: cross-provider equivalence per kind.
- `internal/estimate`: workload DSL and cost math.
- `internal/output`: presets, jq, fields, JSON/YAML/TOML rendering.
- `internal/batch`: multi-operation dispatcher.
- `internal/config`: profile management.

The user-facing binary is **pure offline**: it never talks to provider APIs. All freshness is delivered through `sku update` pulling daily deltas from the CDN. This keeps the binary small, agent-friendly (no credential plumbing), and deterministic.

### Package boundaries

| Package | Responsibility | Depends on |
|---|---|---|
| `cmd/` | Cobra command tree, flag parsing, output rendering | `internal/*`, cobra, color, gojq |
| `internal/catalog` | Open/mmap SQLite shards, execute queries, shard paths | `modernc.org/sqlite` |
| `internal/updater` | Fetch manifest, compute diffs, apply SQL.gz deltas atomically | stdlib net/http, crypto/sha256, sqlite |
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
- **Offline-only client**. The binary never hits provider APIs; everything goes through `internal/catalog` against local SQLite shards. Unit and e2e tests don't need network (beyond `sku update`'s CDN fetch, which is stubbed).
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
  |     - sample 20 SKUs per on-demand shard with stratified sampling
  |       across regions and resource families (see Validation harness in
  |       §6); skip *-commitments, *-reservations, *-cud (those have no free
  |       per-SKU query endpoint and are revalidated weekly).
  |     - re-fetch from the upstream provider and compare values (1% tolerance).
  |       Provider access uses short-lived GitHub Actions OIDC federation only:
  |       * AWS on-demand: SigV4 GetProducts via OIDC-assumed read-only IAM role
  |       * Azure: anonymous prices.azure.com filtered query
  |       * GCP:   billing API via OIDC-federated service account (no standing secret)
  |       * OpenRouter: anonymous
  |     - EC2 cross-check against vantage-sh/ec2instances.info (offline JSON)
  |     - on drift >1%, fail release
  |     - commitment/reservation shards: lightweight **daily sanity
  |       check** (row count within +/-5% of 30-day moving average,
  |       schema shape matches expected column set, no orphan FKs) plus
  |       full weekly cross-check in data-validate.yml. Catches wholesale
  |       pipeline breakage same-day without burning provider API quota
  |       on deep reservation-lookup calls.
  |
  |-> Job 5: publish (1 runner, ~1 min)
        - gh release create data-YYYY.MM.DD with all deltas + baselines
        - publish manifest.json as a release asset on the same release
        - after the release succeeds, call the jsDelivr purge API for
          /gh/sofq/sku@latest/data/manifest.json so CDN edges drop the
          previous copy; without the purge, jsDelivr caches GitHub content
          for up to 12h. Purge calls retry 3x with exponential backoff
          (2s, 8s, 30s); on final failure the workflow auto-files a
          `cdn-purge-failed` GitHub issue and pages the maintainer via
          the `release-alerts` email webhook, so a 12h stale window is
          never silent.
        - manifest.json's authoritative URL is the release asset:
          https://github.com/sofq/sku/releases/latest/download/manifest.json
          (GitHub rewrites `latest` to the newest non-prerelease on each
          request). jsDelivr fronts this URL as a cache layer only:
          https://cdn.jsdelivr.net/gh/sofq/sku@latest/data/manifest.json
          remains available for users who prefer the CDN, mirrored from a
          lightweight `data` branch that contains ONLY manifest.json and is
          force-updated on each run. Clients hit the release-asset URL by
          default and fall back to jsDelivr on 5xx / timeout; this removes
          the single point of failure at jsDelivr and avoids relying on its
          best-effort purge API for freshness. `main` is not touched by
          daily data releases, so `git log main` stays clean. Worst-case
          propagation on the default path is single-digit seconds (GitHub
          release asset), on the CDN fallback up to 12h on purge failure.
```

Total daily runtime: 20-30 min wall-clock, 0-15 min on quiet days. Monthly CI: ~5-10 hours (free for public repo). User bandwidth: manifest ETag (<1 KB on quiet days) plus changed shard deltas (typically ~10 KB to 1 MB per shard; up to ~5 MB on AWS price-adjustment days when EC2/RDS/S3 see broad updates). Deltas larger than 8 MB are automatically split into a sequence of smaller deltas by the packager so individual downloads stay predictable.

### Manifest structure (`data/manifest.json`)

```json
{
  "schema_version": 1,
  "generated_at": "2026-04-18T03:15:00Z",
  "catalog_version": "2026.04.18",
  "shards": {
    "aws-ec2": {
      "baseline_version": "2026.04.01",
      "baseline_url": "https://github.com/sofq/sku/releases/download/data-2026.04.01/aws-ec2.db.zst",
      "baseline_sha256": "ab3d...",
      "baseline_size": 62914560,
      "head_version": "2026.04.18.01",
      "min_binary_version": "1.0.0",
      "shard_schema_version": 1,
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

1. GET `https://github.com/sofq/sku/releases/latest/download/manifest.json` (authoritative release-asset URL; ETag cached, typically 304). On 5xx or timeout, fall back to `https://cdn.jsdelivr.net/gh/sofq/sku@latest/data/manifest.json`.
2. For each installed shard: if `local.head_version < remote.head_version`, walk delta chain, download each `.sql.gz`, verify sha256, apply in one SQLite transaction. If the chain length exceeds `max_delta_chain` (default 20), skip deltas and download the newest baseline instead - cheaper than replaying a long chain. The default of 20 covers: 14 worst-case days between baselines (the 1st-and-15th cadence from §3 gives a maximum 14-day gap) + up to 5 same-day split deltas (from the 8 MB split rule in §3) + 1 slack, with headroom for an off-cycle baseline skip.
3. On failure: rollback; last-known-good state preserved.
4. Delta application requires the `data` branch's `head_version` on disk to equal the manifest's `from` for the first delta. If it doesn't match (force-push race or prior partial apply), fall back to baseline download.

**Concurrency**: The reader opens shards in WAL mode with `busy_timeout=5000ms`, so concurrent `sku price` invocations share readers without blocking. `sku update` takes an advisory file lock on each shard file before applying deltas to that shard; a second `sku update` waits or fails fast with exit code 6 (`conflict`) when `--no-wait` is set. The lock is taken via `github.com/gofrs/flock`, which abstracts over POSIX `fcntl` advisory locks on linux/macOS and `LockFileEx` on Windows, so the behaviour is uniform across platforms. WAL mode requires write access to `<shard>.db-wal` and `<shard>.db-shm` sidecar files; read-only shard directories (e.g. a Docker image with `-v <host>:<container>:ro`) fall back to `journal_mode=delete` via a one-shot opener and log a stderr note - write paths in that mode error with exit code 2.

### Reliability

- **Idempotent stages**: deterministic hashes, re-run safe.
- **Signed releases**: `actions/attest-build-provenance` and cosign.
- **Stale data warning**: `sku` commands warn on stderr if catalog age exceeds `stale_warning_days` (profile-configurable, default 14) and exit with code 8 (`stale_data`) if age exceeds `stale_error_days` (profile-configurable, default disabled; also set via `--stale-error-days N` or env `SKU_STALE_ERROR_DAYS`). `--stale-ok` / `SKU_STALE_OK=1` suppresses both warning and error, downgrading the condition to a `--verbose`-only note. CI profiles typically set `stale_error_days: 7` so a week-old catalog fails the pipeline rather than silently mispricing.
- **Graceful degradation**: per-shard failure isolated; other shards still work.

### Data licensing and redistribution

The `sku` tool is Apache-2.0. Shipped shards contain rate-card data derived from public provider endpoints: AWS Pricing API, Azure `prices.azure.com`, GCP Cloud Billing Catalog, and OpenRouter's public models API. Each provider's terms of use govern redistribution of their rate-card data; `sku` treats shards as derivative public pricing information (same posture as Infracost's public pricing API and vantage-sh/ec2instances.info). A `docs/DATA_LICENSING.md` page enumerates per-provider citations and links, and `SECURITY.md` carries a prominent "users are responsible for complying with upstream provider terms when redistributing shards" disclaimer. If a provider ever objects to redistribution, that provider's shards are removed from the next release and a `sku update` surfaces a deprecation note via the manifest's `deprecated_shards` list.

### OpenRouter-specific ingest

OpenRouter ingest is implemented in `pipeline/ingest/openrouter.py` rather than a DuckDB SQL flow because the dataset is tiny (~1K rows, <200 KB after packaging) and the upstream shape requires two coordinated HTTP calls + per-endpoint joins that are clearer in Python than in DuckDB JSON functions. All other shards use DuckDB.

OpenRouter covers all LLM pricing via two free endpoints (no auth):

1. `GET /api/v1/models`: base pricing, context_length, architecture (modality), supported_parameters, top_provider.
2. `GET /api/v1/models/{author}/{slug}/endpoints`: per-serving-provider pricing, context/max_completion, quantization, health metrics (uptime, latency, throughput).

Total rows: ~300 models x ~2-4 endpoints each = ~1K rows. Shard size <200 KB.

One row per (model, serving_provider) pair, with `sku_id = {author}/{model}::{serving_provider}::{quantization|default}`, plus a synthetic aggregated row per model with `sku_id = {author}/{model}::openrouter::default` and `provider = 'openrouter'`, representing OpenRouter's own routed rate (the price a caller pays when hitting `openrouter.ai` without pinning a provider). The aggregated row carries `health = NULL` since it's a meta-rate, not a concrete endpoint.

Both endpoints are anonymous. An `OPENROUTER_API_KEY` is optional and only improves rate-limit headroom or unlocks private endpoints; `sku update` never requires it.

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

Cloud provider subcommands (`aws`, `azure`, `gcp`) and their service leaves (`ec2`, `rds`, ...) are **registered statically from the known shard inventory** (§3). A command for an un-installed shard still appears in help and, when invoked, returns exit code 3 (`not_found`) with a suggestion to run `sku update <shard>` - or auto-downloads when `--auto-fetch` / `profiles.*.auto_fetch: true` is set. Static registration keeps cobra-generated shell completions stable across installs.

**Data/code decoupling boundary**: new shard *files* (rows, deltas, even entirely new shards within an already-registered provider namespace) publish daily without a binary release. New shard *commands* (a whole new top-level provider like `oracle`, or a new service leaf like `aws-eks-fargate`) require a binary release. When the manifest lists a shard the binary doesn't statically register, `sku update --status` and `sku schema --list` surface it as `available, needs binary upgrade >=vX.Y.Z` (minimum version taken from the manifest's per-shard `min_binary_version` field); `sku update <shard>` refuses the install with exit code 4 and the same upgrade hint. This prevents a user from downloading a shard they can't query.

LLM serving-provider subcommands (`anthropic`, `openai`, `aws-bedrock`, etc.) are **views into the single `openrouter` shard** filtered by the `provider` column in the `skus` table - they are not separate shards. The list of serving providers is seeded into the shard's `metadata` table at build time (`key='serving_providers'`, comma-separated) and re-emitted each day; `--help` reads that metadata row instead of hitting the data path with `SELECT DISTINCT`.

**Data-driven extension vs command-tree extension.** A serving provider newly appearing upstream is immediately valid as a `--serving-provider <name>` filter value (validated against the `metadata.serving_providers` row) and appears in `sku schema --list-serving-providers` output after `sku update`, all without a binary release. A *dedicated top-level subcommand* (e.g. `sku <new-provider> llm price ...`) requires a binary release because cobra subcommands are registered statically for stable shell completions. When the manifest advertises a `min_binary_version` for a serving-provider-subcommand addition, the upgrade hint surfaces via the same `sku update --status` channel as new shards (§4 *Data/code decoupling boundary*).

**Serving-provider filter semantics**: `sku <serving-provider> llm price --model X` filters `WHERE provider = '<serving-provider>'`. A row's `provider` column holds the *serving* provider (e.g. `aws-bedrock`, `anthropic`, `openai`), not the model *author*; the author lives in `resource_name` / `sku_id`. So `sku anthropic llm price --model anthropic/claude-opus-4.6` returns Anthropic's first-party API pricing, while `sku aws-bedrock llm price --model anthropic/claude-opus-4.6` returns the Bedrock rate for the same model.

**LLM-pricing source authority.** LLM rates (including Bedrock / Vertex / Azure-OpenAI-hosted models) live in the single `openrouter` shard, not in the cloud shards. The AWS Pricing API, Azure prices.azure.com, and GCP billing catalog each also publish model rates for their respective hosted-LLM services, but `sku` treats OpenRouter's aggregated view as authoritative for LLM `kind = llm.*` rows because it normalizes prompt/completion/cache dimensions identically across all serving providers - the cloud APIs use per-provider ad-hoc unit schemes that defeat the unified `price[]` shape. The cloud shards (`aws-bedrock`, etc.) therefore explicitly `WHERE kind NOT LIKE 'llm.%'` at ingest, preventing duplicate rows. The daily validation harness (§6) cross-checks OpenRouter-reported Bedrock/Vertex/Azure-OpenAI rates against the matching cloud pricing API at 1% tolerance; on drift >1% the release fails and an `llm-rate-mismatch: <serving-provider>/<model>` issue auto-files. Users needing the cloud-authoritative figure for procurement can pass `--source cloud` (deferred to v1.1; tracked in Appendix B).

**Aggregated OpenRouter row semantics**: the synthetic `provider='openrouter'` rows (§3) represent OpenRouter's own routed rate, not a concrete endpoint. To keep sort/min queries honest, these rows are **excluded by default** from `sku llm price`, `sku llm list`, `sku llm compare`, and cross-provider `sku compare --kind llm.text`. They are included only when:
- the user explicitly queries the OpenRouter view: `sku openrouter llm price ...` (filters `provider='openrouter'`, returns only aggregated rows), or
- the global flag `--include-aggregated` is passed.

The renderer marks aggregated rows with `resource.attributes.aggregated = true` so agents can filter them downstream.

### Lazy shard fetch

Behavior when a queried shard isn't installed:

- Default: return exit code 3 with `error.code = "shard_missing"` and `error.suggestion = "Run: sku update <shard-name>"`. Deterministic for agents and CI.
- With `--auto-fetch` (or `profiles.*.auto_fetch: true`): fetch the shard synchronously before querying; emit a one-line stderr note (`note: fetched aws-ec2 (12 MB)`) unless `--quiet`. On network failure, fall through to the exit-code-3 error above.
- **Inside `sku batch`**: the batch runner deduplicates missing shards across ops, takes the per-shard flock once, fetches each unique shard once at the head of the batch, and then dispatches ops serially. A fetch failure marks every op depending on that shard as exit-code-3 in its envelope slot but does not abort the batch - other ops still run.
- `sku update <shard>` and `sku update --install {core,full}` remain the explicit paths; `--auto-fetch` is purely a convenience for interactive users.

### Global flags

```
--profile <name>             named config profile (default "default")
--preset <name>              agent | full | price | compare (default agent)
--jq <expr>                  jq filter on response
--fields <list>              comma-separated field projection
--include-raw                include "raw" passthrough object
--include-aggregated         include OpenRouter's synthetic aggregated rows (see §4 LLM semantics)
--pretty                     pretty-print JSON (default compact)
--stale-ok                   suppress stale-catalog warning
--auto-fetch                 download missing shards on demand (default: off)
--dry-run                    show resolved query plan without executing
--verbose                    stderr JSON log
--no-color                   disable color
--json | --yaml | --toml     output format (default json)
```

### `--dry-run` output

Emits a JSON object describing what *would* execute, then exits 0 without touching the data path:

```jsonc
{
  "dry_run": true,
  "command": "aws ec2 price",
  "resolved_args": {"instance_type": "m5.large", "region": "us-east-1",
                    "commitment": "on_demand", "tenancy": "shared", "os": "linux"},
  "shards": ["aws-ec2"],
  "terms_hash": "7f3c...",
  "sql": "SELECT ... FROM skus WHERE resource_name = ? AND region = ? AND terms_hash = ?",
  "preset": "agent"
}
```

Stable schema (`schema_version: 1`) so agents can parse it to validate their inputs before committing to a real query.

### Environment variables

Agents and CI often prefer env over flags. All global flags have an env equivalent (`SKU_` prefix, upper snake case): `SKU_PROFILE`, `SKU_PRESET`, `SKU_AUTO_FETCH`, `SKU_STALE_OK`, `SKU_NO_COLOR`. Additionally:

- `SKU_DATA_DIR` overrides the shard storage root (default: `$XDG_CACHE_HOME/sku` on linux, `$HOME/Library/Caches/sku` on macOS, `%LOCALAPPDATA%\sku` on Windows).
- `SKU_CONFIG_DIR` overrides the config root (default: `$XDG_CONFIG_HOME/sku` on linux, `$HOME/Library/Application Support/sku` on macOS, `%APPDATA%\sku` on Windows).
- `SKU_NO_UPDATE_CHECK=1` suppresses the once-per-day manifest HEAD check that prints an update hint on stderr. The check is a single conditional HTTP HEAD against the release-asset manifest URL, carrying only `User-Agent: sku/<version> (<os>/<arch>)` and `If-None-Match: <cached-etag>`. No query string, no cookies, no analytics beacon, no install-ID. The server's stock access log is the only place the request appears; `sku` itself ships no telemetry pipeline. Documented in `SECURITY.md` under "network behavior".
- `NO_COLOR` (standard) is honored equivalently to `--no-color` / `SKU_NO_COLOR`.

### Precedence

For any option, precedence is: **CLI flag > environment variable > profile value > built-in default**. A flag on the command line always wins, even against an explicit profile setting.

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

**Input format.** `sku batch` accepts JSON array *or* NDJSON on stdin, auto-detected from the first non-whitespace byte: `[` selects array mode (parsed with `encoding/json`); `{` selects NDJSON mode (parsed line-by-line with `bufio.Scanner`, one op per line, blank lines and `#`-prefixed comments skipped). Any other first byte is exit code 4 (`validation`, `reason="flag_invalid"`, `details.flag="stdin"`). NDJSON is preferred for agent pipelines because it streams (per-op output can emit as earlier ops complete in a future `--concurrency` flag); array mode is preferred for small hand-written batches. The explicit `--format {array,ndjson}` flag overrides auto-detection.

Returns an array of `{"index", "exit_code", "output", "error"}` (or NDJSON of the same records, one per line, in NDJSON-input mode). `output` is the op's JSON result (or `null` on error); `error` is the same stderr error envelope (`{code, message, suggestion, details}`) that the op would have written to stderr when run alone, so per-op errors don't interleave on the process stderr. The process stderr only carries batch-level parse failures. Global exit code = highest severity across ops.

Per-op stderr hints (e.g. "raw not installed; run `sku update <shard> --with-raw`") are folded into the op's `error.details.warnings` array instead of the process stderr, preserving the one-error-per-op invariant.

**Dispatch mechanism**: `internal/batch` holds a registry keyed by canonical command name (`"aws ec2 price"`, `"compare"`, `"llm price"`, ...); each entry points at a typed handler `func(ctx, args map[string]any) (result any, err error)`. Every leaf cobra command registers its handler in `init()` so the registry is populated at binary start. Batch dispatch calls handlers directly - no `cobra.Command.Execute`, no string reassembly, no sub-process, no shell quoting surface. Errors returned by handlers are converted to the error envelope by the same `internal/errors` mapper that the single-invocation path uses, guaranteeing byte-identical error shapes between batch and standalone execution. Ops run sequentially in v1; parallelism (`--concurrency N`) is a post-v1 knob once read-only WAL reader concurrency is benchmarked.

**Update:**

```bash
sku update                        # update installed + lazy-fetch on demand
sku update --install core         # compute + storage + LLMs, on-demand only
sku update --install full         # full catalog (~360 MB, all shards)
sku update aws-ec2 azure-vm       # targeted
sku update --channel stable       # twice-monthly baselines only (1st + 15th)
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

**`raw`** preserves provider-native attributes, included only with `--include-raw` or preset `full` (which implies `--include-raw`). Raw JSON is **packaged into a separate sibling shard** (`aws-ec2-raw.db.zst`, etc.) rather than the main `.db.zst`, because raw payloads multiply on-disk size by roughly 5-10x. Raw shards are not installed by default; `sku update --install full` or `sku update <shard> --with-raw` fetches them. When a raw shard is absent, `--include-raw` and preset `full` emit the rest of the payload with `"raw": null` plus a one-line stderr hint ("raw not installed; run `sku update <shard> --with-raw`"). The ~360 MB "full install" figure in §3 excludes raw shards; including raw roughly doubles the total.

**Renderer conventions** (mapping from on-disk sentinels to output JSON):
- `terms.tenancy = ''`, `terms.os = ''` → `null` in JSON. Same for `price[].tier`.
- `location.provider_region = ''` → `null`; `location.normalized_region = ''` → `null`.
- `currency` is a shard-wide invariant pulled from the `metadata` table and injected into every `price` element; not stored per-row.

### Presets

| Preset | Fields kept | Typical size | Use case |
|---|---|---|---|
| `agent` (default) | provider, service, resource.name, location.provider_region, price, terms.commitment | ~200 bytes | Agent default, minimum tokens |
| `price` | price only | ~50 bytes | One-line lookups |
| `full` | Everything including raw (implies `--include-raw`) | 2-10 KB | Human inspection, debugging |
| `compare` | provider, resource.name, price, location.normalized_region, plus kind-specific fields (see below) | ~150 bytes | Cross-provider rows |

**Kind-specific `compare` projections** are merged into the base `compare` preset at render time; each kind defines its own discriminating columns in `internal/compare/kinds/*.go`:

| Kind | Extra fields in `compare` preset |
|---|---|
| `compute.vm` | `resource.vcpu`, `resource.memory_gb`, `resource.gpu_count`, `resource.gpu_model` |
| `storage.object` | `resource.durability_nines`, `resource.availability_tier` |
| `db.relational` | `resource.vcpu`, `resource.memory_gb`, `resource.storage_gb` |
| `llm.text` | `resource.context_length`, `resource.capabilities`, `health.uptime_30d`, `health.latency_p95_ms` |

Adding a new kind requires registering a projection in the same file as its equivalence rules. `--fields` and `--jq` apply after preset selection for further trimming.

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

**Per-code `details` schema.** Every error code has a fixed `details` shape, documented in the machine-readable `sku schema --errors` output (stable under `schema_version`). Agents can depend on these fields existing for the matching code:

| `code` | `details` shape |
|---|---|
| `not_found` | `{provider, service, applied_filters, nearest_matches?: [sku_id]}` |
| `validation` | `{reason, flag?, value?, allowed?: [...], shard?, required_binary_version?, hint?}` where `reason` is one of `flag_invalid`, `binary_too_old`, `binary_too_new`, `shard_too_old`, `shard_too_new` |
| `auth` | `{resource}` (never echoes credential material) |
| `rate_limited` | `{retry_after_ms}` |
| `conflict` | `{shard, current_head_version, expected_from, operation}` |
| `server` | `{upstream, status_code?, correlation_id?}` |
| `stale_data` | `{shard, last_updated, age_days, threshold_days}` |
| `shard_missing` | `{shard, install_hint}` |
| `generic_error` | free-form `{message_detail?}` - agents should fall back to `error.message` |

`error.suggestion` is always present and stable text; `details` fields are additive across minor versions (new keys can appear, existing keys never change shape within a major).

### Configuration (`$SKU_CONFIG_DIR/config.yaml`)

Resolved via `SKU_CONFIG_DIR` env var, else platform default (§4 *Environment variables*): `$XDG_CONFIG_HOME/sku/config.yaml` on linux, `$HOME/Library/Application Support/sku/config.yaml` on macOS, `%APPDATA%\sku\config.yaml` on Windows.

```yaml
profiles:
  default:
    preset: agent
    channel: daily                        # daily | stable
    default_regions: [us-east-1, eastus, us-east1]
    stale_warning_days: 14
    auto_fetch: false                     # see §4 "Lazy shard fetch"
  cost-planning:
    preset: full
    include_raw: true
    auto_fetch: true
  ci:
    preset: price
    stale_warning_days: 3
    stale_error_days: 7                   # exit 8 (stale_data) beyond this; unset = never escalate
    auto_fetch: false                     # deterministic: fail if shard missing
```

There are no provider credentials in the config: the binary is offline and only ever talks to the CDN for `sku update`.

---

## 5. Data Schema

### SQLite shard schema (uniform across all shards)

`sku_id` is globally unique within a shard: AWS `sku` from the Pricing API, Azure `meterId`, GCP `skuId`, or synthetic `{model_id}::{serving_provider}::{quantization}` for OpenRouter. All provider-native SKU identifiers already encode region + terms, so `sku_id` alone is a sound primary key.

Nullable columns that would participate in `WITHOUT ROWID` primary keys use `''` as the sentinel (SQLite disallows NULL in `WITHOUT ROWID` PKs). `region = ''` marks regionless / global SKUs (e.g. OpenRouter models, AWS Route53). `tier = ''` marks non-tiered prices.

```sql
CREATE TABLE skus (
  sku_id             TEXT    NOT NULL PRIMARY KEY,
  provider           TEXT    NOT NULL,
  service            TEXT    NOT NULL,
  kind               TEXT    NOT NULL,
  resource_name      TEXT    NOT NULL,
  region             TEXT    NOT NULL,     -- '' for regionless/global SKUs
  region_normalized  TEXT    NOT NULL,     -- '' for global; see note below
  terms_hash         TEXT    NOT NULL      -- see 'terms_hash' note below
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
  modality           TEXT,          -- JSON array
  capabilities       TEXT,          -- JSON array
  quantization       TEXT,
  durability_nines   INTEGER,
  availability_tier  TEXT,
  extra              TEXT           -- JSON
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
  tier       TEXT NOT NULL DEFAULT '',   -- '' when non-tiered
  amount     REAL NOT NULL,              -- see 'Price precision contract' below
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
-- Seeded at build time:
--   schema_version     = '1'
--   catalog_version    = '2026.04.18'
--   currency           = 'USD'          -- single-currency invariant for v1
--   generated_at       = ISO-8601 UTC
--   source_url         = upstream provider doc
--   row_count          = SELECT count(*) FROM skus
--   allowed_kinds      = JSON array of kind values present in this shard
--   allowed_commitments= JSON array of commitment values present in this shard
--   allowed_tenancies  = JSON array of tenancy values present in this shard
--   allowed_oses       = JSON array of os values present in this shard
```

**Enum validation lives in the ingest pipeline, not in SQLite `CHECK` constraints.** The pipeline validates every row against `pipeline/normalize/enums.yaml` before the shard is built, and rejects a build on unknown values. Moving enforcement out of the shard lets the data side introduce a new `kind`, `commitment`, `tenancy`, or `os` (e.g. AWS `capacity-block`, Oracle `ucc_1y`, `windows_2025`) in a daily data release without requiring a binary upgrade; the `metadata.allowed_*` rows describe what the shard actually contains so the client can surface precise validation errors ("`--commitment ucc_1y` is valid in `aws-ec2` but not `gcp-gce`") without duplicating the enum list in Go code. Known-to-the-binary enums used for flag validation and help text are still generated from the same `enums.yaml` at build time (same drift guard as `regions.yaml`, see §5 *Region normalization*).

`currency` lives in `metadata` rather than per-row on `prices`: v1 ingests only USD-native rate cards from AWS, Azure, GCP, and OpenRouter (all four publish USD-denominated prices for the shards covered in §3). No FX conversion happens at ingest, because it would make deltas FX-date-dependent and break baseline reproducibility. The output renderer injects `"currency": "USD"` from metadata. Multi-currency support (post-v1) will add a per-row `currency` column (and an optional `--currency` flag with live FX conversion at query time) behind a `schema_version` bump.

**OpenRouter currency guard.** OpenRouter occasionally surfaces partner-hosted endpoints whose upstream rate card is not USD-denominated. The ingest pipeline enforces the invariant at build time: any row whose upstream `pricing.currency` is absent, empty, or `USD` is accepted; any other value fails the shard build and files a `non-usd-endpoint: <model>/<provider>` repo issue. No silent FX assumption ever reaches a shard.

### Raw shard schema (sibling file)

Raw provider payloads live in a **sibling shard file**, not in the main shard. The main shard file is `<shard>.db.zst`; the raw sibling is `<shard>-raw.db.zst` with this minimal schema:

```sql
CREATE TABLE raw (
  sku_id   TEXT    NOT NULL PRIMARY KEY,    -- FK by convention to the main shard; not enforced cross-file
  payload  TEXT    NOT NULL                 -- JSON from upstream provider
) WITHOUT ROWID;

CREATE TABLE metadata (
  key    TEXT PRIMARY KEY,
  value  TEXT
);
-- Seeded identically to the main shard: schema_version, catalog_version, generated_at.
-- The 'main_shard_sku' metadata row pins the raw sibling to a specific main shard release.
-- On open, if the sibling's 'main_shard_sku' doesn't equal the main shard's metadata.catalog_version,
-- the reader emits exit code 4 with error.details.reason='shard_too_old' (or 'shard_too_new' if the
-- sibling is ahead), error.details.shard='<shard>-raw', and error.suggestion=
-- "raw sibling out of sync; run: sku update <shard> --with-raw". The main shard continues to
-- serve non-raw queries; only --include-raw / preset full is blocked.
```

Raw siblings are optional; when missing, `--include-raw` / preset `full` emits `"raw": null` plus a stderr hint (§4). The reader opens the sibling lazily and only when `--include-raw` is in effect.

**Price precision contract.** `prices.amount` is stored as SQLite `REAL` (IEEE-754 double). Upstream providers publish rate-card values at most to 10 significant decimal digits, which round-trip exactly through float64, so point lookups are bit-stable across runs. `sku estimate` performs all intermediate math in float64 and rounds the final per-item and aggregate totals to 10 decimal places before rendering, so small accumulated error (e.g. 1e-15 from summing millions of per-hour rates) never leaks into user-visible output. Agents diffing JSON across runs can depend on identical bytes for identical inputs. An integer-micros schema is deferred to a future `schema_version` bump if real drift appears; the regression harness in §6 includes a golden-estimate suite that catches any precision regression >1 cent on a $10k/month reference workload.

**`terms_hash`**: a stable 128-bit hex digest (`sha256(JSON-encoded canonical terms tuple)[:32]`) over the row's `(commitment, tenancy, os, support_tier, upfront, payment_option)` values, with empty strings canonicalised to `""`. Populated at ingest. Used for two purposes: (a) compact composite lookup (`idx_skus_lookup` includes `terms_hash` so the planner can short-circuit filter composition to a single equality predicate), and (b) diffing - two SKUs with the same `terms_hash` have identical terms, so the packager can detect term-only changes without comparing each field. The client computes `terms_hash` on the fly from the user's flag values using the same canonical encoding (shared `internal/schema` helper, invoked by both the client and the CI ingester to guarantee bit-identical hashes); no terms flag ever reaches the shard query as six separate predicates.

**Unspecified-flag resolution.** A single source of truth for per-kind term defaults lives in `pipeline/normalize/terms_defaults.yaml` (e.g. `compute.vm: {tenancy: shared, os: linux, support_tier: ''}`; `storage.object: {commitment: on_demand, tenancy: '', os: '', support_tier: ''}`). The ingest pipeline uses it to canonicalise every row before hashing; `make generate` bakes an identical copy into `gen/terms_defaults.go` for client-side resolution. Drift is caught by the same `git diff --exit-code` guard used for `regions.yaml` (§5 *Region normalization*). When a user omits a terms flag, the client substitutes the kind's default *before* hashing, so partial-filter lookups hit the same row the ingester wrote. Flags that accept `any` (e.g. `--tenancy any`) degrade to a non-indexed scan over the residual `(service, resource_name, region)` prefix and are logged under `--verbose` for agent awareness.

128 bits is intentional: a 64-bit truncation has birthday collision probability ~1e-4 at the 60M-row scale you reach once commitment/reservation shards are fully populated, and collisions silently return the wrong row under single-equality lookup. 128 bits keeps collision probability below 1e-20 even at 1B rows.

### Delta semantics

Delta `.sql.gz` patches use `INSERT OR REPLACE` for upserts and `DELETE FROM skus WHERE sku_id IN (...)` for removals. `ON DELETE CASCADE` on every child FK guarantees a single statement removes all associated rows.

Deltas are **idempotent only when applied as an ordered chain**, not individually: a delta `N` that deletes row X, replayed after a later delta `N+1` that re-added X, would wrongly delete. The client tracks `head_version` per shard in the `metadata` table and advances it transactionally with each delta apply; a delta whose `from` doesn't match the stored `head_version` is refused. Re-applying a delta that matches the current `head_version` is rejected as already-applied rather than replayed (exit code 6 `conflict`). The packager guarantees linear chains (no branching).

Each delta applies in a single SQLite transaction that includes the `head_version` update; on failure the transaction rolls back and the shard remains at its prior version.

### Indexes

```sql
-- Within a shard, provider AND service are effectively constant
-- (an aws-ec2 shard only holds service='ec2'), so both are dropped from
-- the index; lead with resource_name for point-lookup selectivity.
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

`PRAGMA foreign_keys = ON` is set by both the pipeline builder and the client reader so CASCADE fires and FK violations surface immediately during ingest.

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

The canonical region-group map lives in a single file checked into the repo (`pipeline/normalize/regions.yaml`, ~15 groups) and is the authoritative source for ingest. Shards are built with `region_normalized` already populated so the client reader can filter on it directly without consulting a binary-side map; this keeps the shard self-describing and avoids binary/shard skew.

```yaml
# pipeline/normalize/regions.yaml (excerpt)
us-east:
  - {provider: aws,   region: us-east-1}
  - {provider: aws,   region: us-east-2}
  - {provider: azure, region: eastus}
  - {provider: azure, region: eastus2}
  - {provider: gcp,   region: us-east1}
  - {provider: gcp,   region: us-east4}
us-west: [...]
eu-west: [...]
asia-se: [...]
```

The Go binary embeds a generated copy of the same map (`gen/regions.go` produced by `make generate`) only to validate `--region` flag input and to render human-readable group names. Lookups against shards use the on-disk `region_normalized` column.

**Drift guard**: `ci.yml` runs `make generate && git diff --exit-code gen/regions.go` so any edit to `pipeline/normalize/regions.yaml` that was not regenerated into `gen/regions.go` fails the PR. This prevents binary/shard skew where a user's `--region` flag would be rejected against a region that newer shards consider valid.

**Unknown upstream regions**: the ingest pipeline fails the shard build when upstream data contains a region not present in `regions.yaml`, and `data-daily.yml` auto-files a repo issue (`new-region: <provider>/<region>`) tagged for maintainer triage. `--channel stable` users are unaffected (previous baseline stays live); `--channel daily` users see a stale-warning after 14 days if the issue remains unresolved. This keeps `region_normalized` honest (never silently `''`) and catches new-region launches within 24h.

### SKU ID sources

- **AWS**: `sku` from Pricing API (stable).
- **Azure**: `meterId` UUID (stable).
- **GCP**: `skuId` (stable).
- **OpenRouter**: synthetic composite `{model_id}::{serving_provider}::{quantization}`.

### Schema versioning

- `metadata.schema_version` per shard. Each shard's version is independent; installs may mix shards at different schema versions as long as each is individually within the binary's supported range. Cross-provider commands (`compare`, `search` across shards) operate on the shared column subset defined by the binary's *minimum* supported schema across the installed shards.
- A shard whose `schema_version` is newer than the binary supports is refused with an "upgrade `sku`" message (exit code 4, `error.details.reason = "shard_too_new"`). A shard whose `schema_version` is older than the binary's minimum supported version is refused with a "run `sku update`" message (exit code 4, `error.details.reason = "shard_too_old"`). Refusals from `min_binary_version` in the manifest (§4 *Data/code decoupling*) use exit code 4 with `error.details.reason = "binary_too_old"` and carry the required version in `error.details.required_binary_version`. Agents should branch on `reason` rather than parse `error.message`.
- Currency policy lives in `metadata.currency` (see §5 preamble); multi-currency support is deferred to a future `schema_version` bump.

### Performance targets

Targets below assume `modernc.org/sqlite` (pure-Go VFS) on linux/amd64 GitHub runners, WAL mode, warm page cache unless noted. M1 ships with a bench harness (`make bench`) that establishes the real baseline; if any target is unattainable the spec is updated before M2, not silently relaxed.

- Point lookup (warm): <5 ms p99 from in-process query to rendered JSON on stdout.
- Cross-provider compare (3 providers): <80 ms p99, parallelised per shard.
- Catalog open + first query cold: <60 ms end-to-end from `exec` to stdout, dominated by Go binary startup + pure-Go SQLite file open. The historical "<10 ms cold" goal assumed mmap'd CGO SQLite and is dropped; agents calling `sku` in a loop should use `sku batch` (in-process, amortises startup).
- `sku search` on the largest shard (AWS EC2, ~1.3M rows) with typical filters (provider+service+region+min-vcpu+min-memory) and `--limit 50`: <40 ms p99 warm, <100 ms p99 cold. Search paths rely on `idx_skus_lookup`, `idx_resource_compute`, and a composite join plan; benchmarks in M1 confirm the index set is sufficient or motivate additions before M4.
- Delta apply (10K rows): <3 s p99, including FK cascade and fsync.

Regression gate: benchstat fails PR if any bench regresses >15% vs `main`.

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
- Schema migration: the binary supports an inclusive `[min_schema, max_schema]` range. A shard at `schema_version > max_schema` (shard is newer than this binary understands) is refused with "upgrade `sku`" (exit code 4). A shard at `schema_version < min_schema` (shard is older than this binary reads) is refused with "run `sku update`" (exit code 4). Shards anywhere inside the inclusive range load. Integration tests exercise both the old-shard boundary and the new-shard boundary with fixtures tagged `schema_v{min-1, min, max, max+1}`.
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
- stale catalog warns unless override
- preset `full` includes raw
- `--jq` reduces output
- schema lists installed shards
- bad JSON goes to stderr
- validation error returns 4
- configure creates profile
- offline update retains last-known-good state (no network during the test)

### Data pipeline tests

Separate test suite under `pipeline/testdata/`:
- **Ingest golden tests**: raw JSON fixture -> DuckDB SQL -> assert output Parquet matches golden.
- **Diff golden tests**: (yesterday, today) Parquet -> assert patch SQL matches golden.
- **Delta apply round-trip**: apply fixture delta to baseline, dump DB, assert equals hand-built expected.
- **Schema consistency**: every shard's rows satisfy schema (PRAGMA foreign_keys=ON + CHECK constraints).

**Determinism note.** Shard byte output depends on `metadata.generated_at` and `metadata.catalog_version`, which change per build. Golden comparisons therefore exclude the `metadata` table from their hash and compare the remaining tables row-for-row. The ingest pipeline honors `SOURCE_DATE_EPOCH` when set (reproducible-builds convention): if present, it overrides `generated_at` so downstream consumers can produce bit-identical shards for a given upstream dataset. CI sets `SOURCE_DATE_EPOCH` to the workflow-run start time for release builds.

### Validation harness (CI production gate)

Daily release pipeline stage:
- Per changed on-demand shard: **stratified sampling** rather than uniform random. For each shard, sample ~20 SKUs spread across (a) the top 3 regions by row count, (b) one SKU from the long-tail region set, (c) the top-N resource families by row count in *this* shard build (computed per-run from `SELECT substr(resource_name, 1, instr(resource_name, '.') - 1) AS family, count(*) FROM skus GROUP BY family ORDER BY 2 DESC LIMIT N`, covering common and accelerator/GPU-class rows without hard-coding family prefixes), and (d) one sample from any family that is new vs yesterday's baseline (catches `inf*`, `trn*`, `mac*`, etc. on first appearance). Uniform random on a million-row shard would miss entire regions; stratification keeps the per-shard budget flat while covering regional, family, and new-family regressions.
- Re-fetch the same SKU from upstream using per-provider credentials held as GitHub Action secrets. Provider access:
  - AWS: GitHub Actions OIDC federation assumes a CI-scoped read-only IAM role in the `sofq/sku` org AWS account (role ARN recorded in `docs/ops/validation.md`; trust policy scoped to `repo:sofq/sku:ref:refs/heads/main`). No long-lived AWS keys in secrets.
  - Azure: anonymous `prices.azure.com` filtered query.
  - GCP: billing API access via Workload Identity Federation from GitHub Actions OIDC (service account `sku-validator@sofq-sku.iam`, read-only `roles/billing.viewer`; details in `docs/ops/validation.md`). No long-lived `GCP_BILLING_API_KEY` secret; the older key-based approach is retired for the same reasons as AWS.
  - OpenRouter: anonymous.
  This is CI-only and never reaches the user's binary.
- Skip `*-commitments`, `*-reservations`, `*-cud` shards: reservation SKUs typically have no free per-SKU query endpoint. These are revalidated weekly by `data-validate.yml` with a dedicated reservation-lookup path.
- Assert `|catalog_price - upstream_price| / upstream_price < 0.01`.
- >1% mismatch -> fail release, auto-file issue.
- EC2 extra cross-check vs `vantage-sh/ec2instances.info` runs daily because it's an offline JSON comparison (no API calls).

### Property-based tests

Framework: `pgregory.net/rapid` (pure-Go, stdlib-compatible `testing.T`, shrinking support). Preferred over `gopkg.in/check.v1` + gocheck for its modern shrinking and better error output.

- Delta chain determinism: applying the chain `[d1..dN]` to a baseline produces the same final state regardless of how many intermediate saves/restores occur partway through. Replaying an already-applied delta errors with exit code 6 and leaves DB state byte-identical (transactional rollback), never mutates rows.
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

Go 1.25 and 1.26 x (linux/amd64, linux/arm64, darwin/amd64, darwin/arm64, windows/amd64) in CI. Go 1.25 is the documented build floor to match `go.mod`'s `go 1.25` directive; Go 1.21+ enforces the directive as a minimum toolchain requirement, so earlier versions cannot build the module. The "supported" matrix rolls forward with Go's stable release train: the CI matrix always pins *the two most recent stable Go minors* (currently 1.25 + 1.26; bumps to 1.26 + 1.27 when 1.27 ships). Dropping a supported minor is a minor-version bump of `sku`. Windows/arm64 binary is produced by goreleaser but not covered in the test matrix (cross-compile only; user reports catch bugs). E2E runs on linux + darwin amd64 only.

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

- `ci.yml`: PR + push to main. lint (golangci-lint), test (5-platform matrix: linux/amd64, linux/arm64, darwin/amd64, darwin/arm64, windows/amd64, Go 1.25 + 1.26), integration, e2e, benchmark regression, build smoke. All required before merge.
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
- Homebrew tap at `sofq/homebrew-tap`.
- Scoop bucket at `sofq/scoop-bucket`.
- Docker multi-arch at `ghcr.io/sofq/sku`.
- nfpms (optional deb/rpm/apk).
- npm publish via release workflow step.
- PyPI publish via release workflow step.

### npm wrapper (`npm/`)

Published as `@sofq/sku` using the **platform-optional-dependencies** pattern (as used by esbuild, swc, Rollup): the root package declares one `optionalDependencies` entry per `os`/`cpu` pair, each pointing at a tiny platform-specific package (`@sofq/sku-linux-x64`, `@sofq/sku-darwin-arm64`, ...) that ships only the matching prebuilt binary. npm resolves and installs just the correct one. `bin/sku` is a thin JS shim that execs the sibling package's binary; no postinstall network download, so it works in airgapped/locked-down CI, Yarn PnP, pnpm `--frozen-lockfile`, and npm `--ignore-scripts`. goreleaser publishes all platform packages plus the root in one release step.

### PyPI wrapper (`python/`)

Published as `sku-cli`, command `sku`, using **platform-tagged wheels** built by `cibuildwheel`: one wheel per (`manylinux_2_28_x86_64`, `manylinux_2_28_aarch64`, `musllinux_1_2_x86_64`, `musllinux_1_2_aarch64`, `macosx_11_0_x86_64`, `macosx_11_0_arm64`, `win_amd64`, `win_arm64`) that vendors the prebuilt binary inside the wheel. `pip install` picks the correct wheel for the local platform; `pipx install sku-cli` gets an isolated install with no network fetch at install time. No `setup.py` postinstall hook - postinstall binary downloads are discouraged by PyPI policy, break under PEP 668 / Debian externally-managed environments, and fail in hermetic CI.

### Docker image (`Dockerfile.goreleaser`)

```dockerfile
# Base pinned to the newest Alpine stable at release time; bumped in
# lockstep with Alpine's release train. 3.19 EOLs 2026-11, so any v1.0
# shipping in 2026 pins >= 3.20.
FROM alpine:3.21
RUN apk add --no-cache ca-certificates
COPY sku /usr/local/bin/sku
USER 65534:65534
ENTRYPOINT ["/usr/local/bin/sku"]
CMD ["--help"]
```

Final image size will depend on the static binary; a Go binary importing `modernc.org/sqlite` + `gojq` + `cobra` typically lands in the 25-40 MB range when built with `-ldflags="-s -w"`. On alpine with ca-certificates the image is roughly binary + 8 MB. Measured size is published on each release and tracked as a CI regression signal.  Non-root. GHCR (free for public).

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
|            batch.go, provider_registry.go)
|-internal/
|   |-catalog/
|   |-updater/
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
|-data/manifest.json (published via `data` branch, not tracked on `main`)
|-examples/ (estimate-workloads/*.yaml, batch-queries.ndjson)
|-e2e_test.go
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

`github.com/sofq/sku`. Binary name: `sku`.

### Go dependencies (intentionally minimal)

- `github.com/spf13/cobra` CLI
- `modernc.org/sqlite` pure-Go SQLite
- `github.com/itchyny/gojq` pure-Go jq
- `github.com/klauspost/compress/zstd` pure-Go zstd
- `github.com/stretchr/testify` test assertions
- `golang.org/x/term` TTY detection
- `gopkg.in/yaml.v3` YAML parsing

Excluded: any provider SDK (the client never talks to provider APIs); DuckDB/Parquet libs (CI-side only).

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

### Milestones to v1.0 (~14-16 weeks solo)

**M0 - Foundations (1 week):**
Repo scaffolded, `go.mod` + Makefile + goreleaser dry-run + CI workflow green on the 5-platform matrix. `sku version` returns JSON.
*Exit criteria include binary-name availability check: `sku` must be claimable on PyPI, npm, Homebrew tap namespace, and Scoop bucket namespace before M0 closes. If any registry conflicts, the final name is chosen here and propagated through the entire doc before M1 begins. This blocks M0 exit because renaming after shards publish would invalidate manifest URLs and installed-catalog paths.*

**M1 - Single-shard pipeline + catalog reader (2 weeks):**
OpenRouter shard ingestion (Python + DuckDB). Full SQLite schema. `internal/catalog` point lookup. `sku llm price` working. Output JSON, `--pretty`, basic error codes.
*Exit criteria: `sku update` downloads OpenRouter shard; `sku llm price --model anthropic/claude-opus-4.6` returns correct JSON, meeting §5 perf targets (<60 ms cold, <5 ms p99 warm) on the M1 bench harness.*

**M2 - Output polish + CLI ergonomics (1.5 weeks):**
All presets, `--jq`, `--fields`, all exit codes, `sku schema` discovery, config profiles, `sku configure`, stale warnings, shell completions.
*Exit criteria: all global flags work; preset token sizes correct; schema discovery machine-readable.*

**M3a - AWS core shards + daily pipeline (3 weeks):**
AWS (ec2, rds, s3, lambda, ebs, dynamodb, cloudfront). Updater module with delta apply, atomic transactions, offline safety. Daily data pipeline live in CI. Validation harness (AWS subset).
*Exit criteria: ~7 AWS shards live; `sku update` fetches all; point lookups correct vs ec2instances.info; validation harness fails intentionally-broken data.*

**M3b - Azure + GCP core shards (2 weeks):**
Azure (vm, sql, blob, functions, disks), GCP (gce, cloud-sql, gcs, run, functions). Validation harness extended to Azure + GCP. Pipeline matrix fan-out proven at real scale.
*Exit criteria: ~17 shards total live across three clouds; point lookups work cross-provider.*

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
| M3a | 3 wk | AWS EC2 JSON quirks, RDS multi-AZ pricing, pipeline hardening |
| M3b | 2 wk | Azure meter-index quirks, GCP SKU mapping |
| M4 | 1.5 wk | Region equivalence edge cases |
| M5 | 2 wk | Estimate DSL + per-kind math |
| M6 | 1 wk | Cross-platform install quirks |
| M7 | 1.5 wk | Doc quality |

### Post-v1 roadmap

**v1.1** (~2 months after v1.0): `sku watch`, commitment shards if deferred, `sku mcp-server`, audit logging, `--currency` native conversion.

**v1.2**: Oracle Cloud / Alibaba / Fly.io / Vercel / Cloudflare Workers providers, `sku recommend`, CSV/TSV output.

**v2.0** (breaking): full multi-currency, persistent semantic cache, historical price tracking.

### First week (M0) concrete TODOs

1. `git init`, repo at `github.com/sofq/sku`.
2. Initial commit with LICENSE (Apache-2.0), README placeholder, `.gitignore`, `go.mod` (Go 1.25).
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
- [jira-cli (jr)](https://github.com/sofq/jira-cli) (user's reference) - CLI shape, npm/python wrapper pattern, e2e test shape.

## Appendix B: Open questions deferred to planning

- Final binary name - resolved during M0 exit (blocker; see §9 M0). Listed here only to track that the `sku` working name is contingent on registry availability.
- Whether `--currency` native conversion ships in v1 or v1.1.
- Whether `--source {openrouter,cloud}` flag for LLM rows (see §4 *LLM-pricing source authority*) ships in v1 or v1.1.
- Whether Scoop manifest lives in a separate repo or the main repo.
- Exact cadence for `sku-mcp-server` (v1.1 vs v1.0).
- Whether to ship `apt`/`yum` repos beyond nfpms packages.
