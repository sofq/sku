# CLAUDE.md

Agent quick-start for the `sku` repo.

## What this is

`sku` is an agent-friendly CLI for querying cloud + LLM pricing. Offline-only client, daily data pipeline, pure-Go binary.

## Dev commands

| Task | Command |
|---|---|
| Build binary | `make build` (output: `bin/sku`) |
| Run tests | `make test` |
| Lint | `make lint` |
| Release dry-run | `make release-dry` |
| Regenerate code/docs | `make generate` (no-op until M4) |
| Build local OpenRouter shard | `make openrouter-shard` |
| Run Go integration tests | `make test-integration` |
| Run benchmarks | `make bench` |
| Run Python pipeline tests | `make pipeline-test` |
| Run discover (fixture / dry-run) | `make discover` |
| Run discover against real upstreams | `DISCOVER_LIVE=1 make discover` (GCP uses ADC; run `gcloud auth application-default login` first) |
| Live-ingest a single shard | `make shard-live SHARD=aws_ec2 SRC=/path/to/offer.json` |
| Dispatch full data pipeline (dry-run) | `gh workflow run data-dispatch.yml -F force_baseline=true` |
| Dispatch full data pipeline (publish) | `gh workflow run data-dispatch.yml -F dry_run=false -F force_baseline=true` |
| Replace today's data release | `gh workflow run data-dispatch.yml -F dry_run=false -F force_baseline=true -F replace_existing_release=true` |

## Repo map

- `cmd/sku/` — Cobra command tree (thin; no business logic)
- `internal/` — all logic lives here; packages are added per milestone
- `internal/version/` — single source of truth for build metadata
- `pipeline/` — CI-only data pipeline (arrives in M1+)
- `.github/workflows/` — `ci.yml` (PR/push), `release.yml` (tag), and data workflows from M3a

## Patterns

- **Pure Go, no CGO.** Every dependency must cross-compile without Docker tricks.
- **`cmd/` stays thin.** Flag parsing + calls into `internal/`. No business logic.
- **TDD.** Write failing test, implement, commit.
- **Exit codes are contract** (spec §4). Full taxonomy is live as of M2; `sku schema --errors` emits the machine-readable catalog.

## Current milestone

M-α — pipeline architecture for coverage expansion:
- Monolithic `data-daily.yml` split into three per-provider workflows
  (`data-aws.yml` 03:00, `data-azure.yml` 03:15, `data-gcp.yml` 03:30 UTC)
  plus `data-publish.yml` (04:30 UTC fallback); `data-dispatch.yml` (formerly
  `data-daily.yml`) kept as a thin manual-dispatch dispatcher. See
  `docs/superpowers/specs/2026-04-24-m-alpha-pipeline-architecture-design.md`
  for the full design (note: Feature D dedup and Go-side codegen were cut —
  see the plan file).
- `pipeline/shards/*.yaml` is now the single source of truth; `make generate`
  regenerates `package/budgets.py` and `discover/_shards_gen.py`.
- ETag fast path wired for AWS non-streaming shards; controlled by
  `SKU_ETAG_MODE` env var.

Next: M-β (R1 regions), then M-γ (S1 services) (dedup respec pending; will
ride with M-γ's schema bump).

M-γ.1 S1 databases & caches: Aurora, ElastiCache, Cosmos DB, Azure Redis, Spanner, Memorystore shards + cache.kv compare kind + estimators.

M-γ.2 S1 containers (managed Kubernetes): EKS, AKS, GKE shards + container.orchestration compare kind. Worker/node prices remain in `aws_ec2` / `azure_vm` / `gcp_gce`.

M-γ.3 S1 PaaS / search / warehouse: aws_opensearch, azure_appservice, gcp_bigquery shards + search.engine, paas.app, warehouse.query compare kinds + estimators (opensearch, appservice, bigquery). BigQuery pseudo-regions bq-us / bq-eu added.

### Quick path (agent, repeatable, M3b.4 surface)

```bash
make openrouter-shard aws-shards azure-shards gcp-shards  # build all local shards
export SKU_DATA_DIR=$(pwd)/dist/pipeline

./bin/sku llm price --model anthropic/claude-opus-4.6 --preset agent
./bin/sku llm price --model anthropic/claude-opus-4.6 --preset price \
  --serving-provider aws-bedrock                        # price-only preset
./bin/sku llm price --model anthropic/claude-opus-4.6 --yaml --pretty
./bin/sku llm price --model anthropic/claude-opus-4.6 \
  --jq '.price[0].amount' --serving-provider aws-bedrock
./bin/sku llm price --model anthropic/claude-opus-4.6 \
  --fields provider,price.0.amount --serving-provider aws-bedrock
./bin/sku llm price --model anthropic/claude-opus-4.6 --dry-run
./bin/sku schema --errors                               # error-code catalog
./bin/sku schema --list-serving-providers

./bin/sku aws ec2 price --instance-type m5.large --region us-east-1 --preset agent
./bin/sku aws ec2 price --instance-type m5.large --region ap-south-1 --preset agent    # P1: India
./bin/sku aws ec2 price --instance-type m5.large --region sa-east-1 --preset agent     # P1: São Paulo
./bin/sku aws ec2 list  --instance-type m5.large
./bin/sku aws rds price --instance-type db.m5.large --region us-east-1 \
  --engine postgres --deployment-option single-az
./bin/sku aws rds price --instance-type db.m5.large --region ap-southeast-2 \
  --engine postgres --deployment-option single-az                           # P1: Sydney
./bin/sku aws rds list  --instance-type db.m5.large --engine postgres

./bin/sku aws s3     price --storage-class standard --region us-east-1 --preset agent
./bin/sku aws s3     list  --storage-class standard
./bin/sku aws lambda price --architecture arm64     --region us-east-1
./bin/sku aws lambda list  --architecture x86_64
./bin/sku aws ebs    price --volume-type gp3        --region us-east-1
./bin/sku aws ebs    list  --volume-type gp3

./bin/sku aws dynamodb   price --table-class standard --region us-east-1 --preset agent
./bin/sku aws dynamodb   list  --table-class standard
./bin/sku aws cloudfront price --region eu-west-1 --preset agent
./bin/sku aws cloudfront list

./bin/sku azure vm  price --arm-sku-name Standard_D2_v3 --region eastus --os linux --preset agent
./bin/sku azure vm  price --arm-sku-name Standard_D2_v3 --region centralindia --os linux --preset agent  # P1: India
./bin/sku azure vm  list  --arm-sku-name Standard_D2_v3
./bin/sku azure sql price --sku-name GP_Gen5_2 --region eastus \
  --deployment-option single-az --preset agent
./bin/sku azure sql list  --sku-name GP_Gen5_2

./bin/sku azure blob      price --tier hot            --region eastus       --preset agent
./bin/sku azure blob      list  --tier hot
./bin/sku azure functions price --architecture x86_64 --region eastus       --preset agent
./bin/sku azure functions list  --architecture x86_64
./bin/sku azure disks     price --disk-type premium-ssd --region eastus     --preset agent
./bin/sku azure disks     list  --disk-type standard-ssd

./bin/sku gcp gce       price --machine-type n1-standard-2  --region us-east1 --preset agent
./bin/sku gcp gce       price --machine-type n1-standard-2  --region asia-south1 --preset agent  # P1: Mumbai
./bin/sku gcp gce       list  --machine-type n1-standard-2
./bin/sku gcp cloud-sql price --tier db-custom-2-7680 --region us-east1 \
                              --engine postgres --deployment-option zonal --preset agent
./bin/sku gcp cloud-sql list  --tier db-custom-2-7680  --engine postgres

./bin/sku gcp gcs       price --storage-class standard --region us-east1 --preset agent
./bin/sku gcp gcs       list  --storage-class standard
./bin/sku gcp run       price --architecture x86_64 --region us-east1 --preset agent
./bin/sku gcp run       list  --architecture x86_64
./bin/sku gcp functions price --architecture x86_64 --region us-east1 --preset agent
./bin/sku gcp functions list  --architecture x86_64

./bin/sku search --provider aws --service ec2 --min-vcpu 4 --limit 5 --preset agent
./bin/sku search --provider aws --service ec2 --max-price 0.10 --sort price
./bin/sku search --provider aws --service ec2 --region us-east-1 --kind compute.vm

./bin/sku compare --kind compute.vm --vcpu 4 --memory 16 --regions us-east --limit 5 --preset compare
./bin/sku compare --kind compute.vm --vcpu 8 --memory 32 --regions us-east,eu-west --sort price
./bin/sku compare --kind compute.vm --vcpu 4 --memory 16 --regions asia-south --limit 5 --preset compare  # R1: India
./bin/sku compare --kind compute.vm --vcpu 4 --memory 16 --regions africa --limit 5 --preset compare      # R1: Africa
./bin/sku compare --kind compute.vm --vcpu 4 --memory 16 --regions middle-east --limit 5 --preset compare # R1: Middle East
./bin/sku compare --kind storage.object --storage-class standard --regions us-east --limit 5 --preset compare
./bin/sku compare --kind db.relational --vcpu 2 --memory 8 \
                   --engine postgres --deployment-option single-az --regions us-east --limit 5 --preset compare

./bin/sku estimate --item aws/ec2:m5.large:region=us-east-1:count=10:hours=730 --pretty
./bin/sku estimate --item aws/ec2:m5.large:region=us-east-1:count=2:hours=100 \
                   --item aws/ec2:m5.xlarge:region=us-east-1:count=1:hours=730
# Workload from YAML file
./bin/sku estimate --config docs/examples/workload-vm.yaml --pretty
# Workload from stdin
echo '{"items":[{"provider":"aws","service":"ec2","resource":"m5.large","params":{"region":"us-east-1","count":2,"hours":100}}]}' | ./bin/sku estimate --stdin --pretty
# storage.object estimator (m5.3)
./bin/sku estimate --item aws/s3:standard:region=us-east-1:gb_month=500:put_requests=1000:get_requests=5000 --pretty
./bin/sku estimate --item azure/blob:hot:region=eastus:gb_month=200:put_requests=500:get_requests=2000 --pretty
./bin/sku estimate --item gcp/gcs:standard:region=us-east1:gb_month=1000:put_requests=10000:get_requests=50000 --pretty
# llm.text estimator (m5.4)
./bin/sku estimate --item llm:anthropic/claude-opus-4.6:input=1M:output=500K:serving_provider=anthropic --pretty
./bin/sku estimate --item llm:anthropic/claude-opus-4.6:input=2M:output=1M:serving_provider=aws-bedrock --pretty
./bin/sku estimate --config docs/examples/workload-llm.yaml --pretty
# sku batch (m5.5)
echo '[
  {"command":"aws ec2 price","args":{"instance_type":"m5.large","region":"us-east-1"}},
  {"command":"llm price","args":{"model":"anthropic/claude-opus-4.6"}}
]' | ./bin/sku batch --pretty
cat docs/examples/batch-queries.ndjson | ./bin/sku batch
./bin/sku schema --list-commands

./bin/sku update openrouter --channel daily            # delta-chain walk
./bin/sku update aws-ec2   --channel stable            # baseline-only

./bin/sku aws aurora       price --instance-type db.r6g.large --region us-east-1 --engine aurora-postgres --preset agent
./bin/sku aws aurora       list  --instance-type db.r6g.large
./bin/sku aws elasticache  price --instance-type cache.r6g.large --region us-east-1 --engine redis --preset agent
./bin/sku aws elasticache  list  --instance-type cache.r6g.large

./bin/sku azure cosmosdb   price --capacity-mode provisioned --region eastus --api sql --preset agent
./bin/sku azure cosmosdb   price --capacity-mode serverless  --region eastus --api sql
./bin/sku azure redis      price --tier standard --size C1 --region eastus --preset agent
./bin/sku azure redis      list  --tier premium  --size P1

./bin/sku gcp spanner      price --edition standard --pu 1000 --region us-east1 --preset agent
./bin/sku gcp spanner      price --edition enterprise --pu 1000 --region us-east1
./bin/sku gcp memorystore  price --instance-type memorystore-redis-standard-5gb --region us-east1 --preset agent

./bin/sku compare --kind cache.kv --memory 16 --regions us-east --limit 5 --preset compare
./bin/sku compare --kind cache.kv --memory 16 --engine memcached --regions us-east --limit 5

./bin/sku estimate --item aws/aurora:db.r6g.large:region=us-east-1:engine=aurora-postgres:hours=730 --pretty
./bin/sku estimate --item aws/aurora:serverless-v2:region=us-east-1:engine=aurora-postgres:acu_hours=8000 --pretty
./bin/sku estimate --item azure/cosmosdb:provisioned:region=eastus:api=sql:ru_per_sec=1000:hours=730 --pretty
./bin/sku estimate --item azure/cosmosdb:serverless:region=eastus:api=sql:ru_million=50 --pretty
./bin/sku estimate --item gcp/spanner:standard:region=us-east1:pu=1000:hours=730 --pretty
./bin/sku estimate --item gcp/spanner:enterprise:region=us-east1:node=2:hours=730 --pretty

# M-γ.2 S1 containers (managed Kubernetes): EKS, AKS, GKE shards + container.orchestration compare kind.
./bin/sku aws eks   price --tier standard          --region us-east-1 --preset agent
./bin/sku aws eks   price --mode fargate           --region us-east-1
./bin/sku aws eks   list  --tier extended-support
./bin/sku azure aks price --tier premium           --region eastus    --preset agent
./bin/sku azure aks price --mode virtual-nodes     --region eastus    --aci-os linux
./bin/sku azure aks list  --tier free
./bin/sku gcp gke   price --tier standard          --region us-east1  --preset agent
./bin/sku gcp gke   price --mode autopilot         --region us-east1
./bin/sku gcp gke   list  --tier autopilot

./bin/sku compare --kind container.orchestration --regions us-east             --limit 5 --preset compare
./bin/sku compare --kind container.orchestration --tier standard --regions us  --limit 5
./bin/sku compare --kind container.orchestration --mode autopilot --regions us-east

# Reminder: worker / node-pool prices live in the existing VM shards
./bin/sku aws ec2   price --instance-type m5.large --region us-east-1    # EKS workers
./bin/sku azure vm  price --arm-sku-name Standard_D2_v3 --region eastus  # AKS nodes
./bin/sku gcp gce   price --machine-type n1-standard-2 --region us-east1 # GKE nodes

# M-γ.3 S1 PaaS / search / warehouse
./bin/sku aws opensearch   price --instance-type r6g.large.search --region us-east-1 --preset agent
./bin/sku aws opensearch   price --mode serverless                --region us-east-1
./bin/sku aws opensearch   list  --instance-type r6g.large.search
./bin/sku azure appservice price --sku P1v3  --region eastus --os linux  --preset agent
./bin/sku azure appservice price --sku P1v3  --region eastus --os windows
./bin/sku azure appservice list  --sku P2v3
./bin/sku gcp bigquery     price --mode on-demand       --region bq-us   --preset agent
./bin/sku gcp bigquery     price --mode capacity-standard --region bq-us
./bin/sku gcp bigquery     price --mode storage-active   --region bq-eu
./bin/sku gcp bigquery     list  --mode on-demand

./bin/sku compare --kind search.engine   --regions us-east --limit 5 --preset compare
./bin/sku compare --kind search.engine   --mode serverless --regions us-east --limit 5
./bin/sku compare --kind paas.app        --regions us-east --limit 5 --preset compare
./bin/sku compare --kind paas.app        --os windows      --regions us-east --limit 5
./bin/sku compare --kind warehouse.query --regions us-east --limit 5 --preset compare
./bin/sku compare --kind warehouse.query --mode capacity-standard --regions us-east

./bin/sku estimate --item aws/opensearch:r6g.large.search:region=us-east-1:count=3:hours=730 --pretty
./bin/sku estimate --item aws/opensearch:serverless:region=us-east-1:ocu_hours=720 --pretty
./bin/sku estimate --item azure/appservice:P1v3:region=eastus:os=linux:count=2:hours=730 --pretty
./bin/sku estimate --item gcp/bigquery:on-demand:region=bq-us:tb_queried=100 --pretty
./bin/sku estimate --item gcp/bigquery:capacity-standard:region=bq-us:slots=500:hours=730 --pretty
./bin/sku estimate --item gcp/bigquery:storage-active:region=bq-us:gb_month=5000 --pretty
```

### Distribution smoke (M6)

```bash
make release-check          # Full local goreleaser dry-run
make docker-smoke           # Build + run sku:smoke container
make npm-pack-smoke         # Dry npm pack + shim sanity-check
make pypi-wheel-smoke       # Build one wheel with the local binary vendored
```

## Global flags (all subcommands)

`--profile`, `--preset {agent|full|price|compare}`, `--json|--yaml|--toml`, `--pretty`,
`--jq <expr>`, `--fields <paths>`, `--include-raw`, `--include-aggregated`, `--stale-ok`,
`--auto-fetch`, `--dry-run`, `--verbose`, `--no-color`. Env equivalents: `SKU_PROFILE`,
`SKU_PRESET`, `SKU_FORMAT`, `SKU_STALE_OK`, `SKU_STALE_ERROR_DAYS`, `NO_COLOR`,
`SKU_NO_COLOR`, `SKU_VERBOSE`, `SKU_DRY_RUN`. Config file at `$SKU_CONFIG_DIR/config.yaml`
(resolved via `sku configure`). Precedence: CLI > env > profile > default.

## TOML quirks

TOML cannot represent top-level arrays. `sku` wraps any top-level `[]` as `{ "rows": [...] }`
before emitting TOML; agents consuming `--toml` should look under `rows`.
