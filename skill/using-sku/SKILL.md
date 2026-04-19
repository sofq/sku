---
name: using-sku
description: Use when the agent needs cloud or LLM pricing — AWS, Azure, GCP, or LLM models across 70+ serving providers. Covers point lookups, cross-provider compare, workload cost estimation, and batch multi-op queries. The CLI is offline-first (daily-updated local SQLite shards) and agent-shaped (JSON everywhere, semantic exit codes, field-projection presets).
---

# Using sku

`sku` is a pure-Go CLI that answers "what does this cost?" for AWS, Azure, GCP, and LLM serving providers. It is designed for agents: JSON output, semantic exit codes, presets that project to ~200 bytes per record.

## Check availability

```bash
sku version --pretty
```

If `sku` is not installed, direct the user to [install.md](../../docs/install.md) (`brew install sofq/tap/sku`, `npx @sofq/sku`, `pipx install sku-cli`, `scoop install sku`, `docker pull ghcr.io/sofq/sku`).

## First-run bootstrap

`sku` is offline — pricing data ships as daily-updated SQLite shards fetched by `sku update`.

```bash
# Install the shards you need; pick from:
# openrouter aws-ec2 aws-rds aws-s3 aws-lambda aws-ebs aws-dynamodb aws-cloudfront
# azure-vm azure-sql azure-blob azure-functions azure-disks
# gcp-gce gcp-cloud-sql gcp-gcs gcp-run gcp-functions
sku update openrouter aws-ec2
```

## Core verbs

### Point lookup

```bash
sku aws ec2 price   --instance-type m5.large --region us-east-1 --preset agent
sku azure vm price  --arm-sku-name Standard_D2_v3 --region eastus --os linux
sku gcp gce price   --machine-type n1-standard-2 --region us-east1
sku llm price       --model anthropic/claude-opus-4.6 --serving-provider aws-bedrock
```

### Cross-provider compare

```bash
sku compare --kind compute.vm --vcpu 4 --memory 16 --regions us-east --limit 5 --preset compare
sku llm compare --model anthropic/claude-opus-4.6 --preset compare
```

### Search within one shard

```bash
sku search --provider aws --service ec2 --min-vcpu 4 --max-price 0.10 --sort price --limit 5
```

### Estimate workload cost

```bash
sku estimate --item 'aws/ec2:m5.large:region=us-east-1:count=10:hours=730' --pretty
sku estimate --item 'llm:anthropic/claude-opus-4.6:input=1M:output=500K:serving_provider=aws-bedrock' --pretty
```

### Batch many ops in-process

```bash
cat <<'EOF' | sku batch
{"command":"aws ec2 price","args":{"instance_type":"m5.large","region":"us-east-1"}}
{"command":"llm price","args":{"model":"anthropic/claude-opus-4.6"}}
EOF
```

## Agent-shape flags (apply globally)

| Flag | Purpose |
|---|---|
| `--preset agent` (default) | ~200 B record — best default |
| `--preset price` | `price[]` only — lowest-token lookups |
| `--jq '<expr>'` | gojq filter on the response |
| `--fields a,b.c` | dot-path projection |
| `--pretty` | indent (interactive use only) |
| `--include-aggregated` | include OpenRouter synthetic rows (normally excluded) |

## Exit codes are a contract

| Exit | Meaning |
|---|---|
| `0` | ok |
| `3` | not_found (see `error.details.nearest_matches`) |
| `4` | validation (see `error.details.reason`) |
| `8` | stale_data (run `sku update`, or pass `--stale-ok`) |

Full taxonomy: `sku schema --errors`.

## Discovery

Agents should introspect rather than guess:

```bash
sku schema --errors                  # error-code catalog
sku schema --list-commands           # commands accepted by `sku batch`
sku schema --list-serving-providers  # LLM serving providers in the shard
```

## Common agent patterns

### "Cheapest cloud VM that fits N cores / M GB"

```bash
sku compare --kind compute.vm --vcpu N --memory M --regions us-east --limit 1 \
  --preset compare --jq '.[0]'
```

### "Cheapest way to serve a given LLM"

```bash
sku llm compare --model "$MODEL" \
  --jq '[.[] | select(.resource.attributes.aggregated != true)]
        | sort_by(.price[0].amount)[0]'
```

### "Budget check before committing to an architecture"

```bash
sku estimate --config <workload.yaml> --pretty
```

## Guardrails

- **Do not pass `--auto-fetch` inside tight loops.** It can cause a CDN fetch mid-query. Prefer an explicit `sku update` up front.
- **Aggregated OpenRouter rows** (`resource.attributes.aggregated = true`) are not real endpoints; exclude them with `--include-aggregated` off (the default).
- **Sku never reaches provider APIs**; do not prompt the user for cloud credentials in order to query pricing.

## When to stop and tell the user

- `sku version` fails → tell the user to install `sku`.
- `sku update` fails with exit 4 (`shard_too_new` / `binary_too_old`) → tell the user to upgrade `sku`.
- `sku update` fails with exit 7 → CDN upstream issue; retry in a few minutes.
- Exit code 8 with `--stale-ok` already set → data is stale beyond your configured `stale_error_days` — tell the user.
