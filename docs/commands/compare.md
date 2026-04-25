# `sku compare`

Cross-provider equivalence within one *kind*. Rows are normalized to a common schema so AWS, Azure, GCP (and LLM serving providers for `llm.text`) are sortable together.

## Synopsis

```
sku compare --kind <kind> [spec] [flags]
```

## Flags (common)

| Flag | Meaning |
|---|---|
| `--kind` | Required. `compute.vm`, `storage.object`, `db.relational`. LLM comparison uses `sku llm compare`. |
| `--vcpu`, `--memory` | `compute.vm` / `db.relational`. |
| `--storage-class` | `storage.object`. |
| `--engine` | `db.relational` (e.g. `postgres`, `mysql`). |
| `--deployment-option` | `db.relational` (e.g. `single-az`, `multi-az`). |
| `--regions` | Comma-separated region groups or provider regions (e.g. `us-east,eu-west,africa,middle-east`). |
| `--sort` | `price` (default), `provider`, `region`. |
| `--limit` | Cap row count (per shard, then merged). |

## Examples

```bash
sku compare --kind compute.vm      --vcpu 4 --memory 16 --regions us-east --limit 5 --preset compare
sku compare --kind compute.vm      --vcpu 8 --memory 32 --regions us-east,eu-west --sort price
sku compare --kind compute.vm      --vcpu 4 --memory 16 --regions africa,middle-east --limit 5
sku compare --kind storage.object  --storage-class standard --regions us-east --limit 5
sku compare --kind db.relational   --vcpu 2 --memory 8 --engine postgres \
             --deployment-option single-az --regions us-east --limit 5
```

## Exit codes

`0`, `3`, `4`, `8`.

See [`../reference/kinds.md`](../reference/kinds.md) for what each kind covers and which shards currently contribute rows.
