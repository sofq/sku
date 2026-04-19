# Presets

A *preset* is a named field projection applied before rendering. Choose a preset with `--preset` or `profiles.<name>.preset` in config.

| Preset | Kept fields | Typical size | Use case |
|---|---|---|---|
| `agent` (default) | `provider`, `service`, `resource.name`, `location.provider_region`, `price`, `terms.commitment` | ~200 B | Agent default — minimum tokens |
| `price` | `price` only | ~50 B | One-shot "how much" lookups |
| `full` | Everything + raw (implies `--include-raw`) | 2–10 KB | Human inspection, debugging |
| `compare` | `provider`, `resource.name`, `price`, `location.normalized_region` + kind-specific columns | ~150 B | Cross-provider rows |

## Kind-specific `compare` columns

The `compare` preset merges additional columns depending on `--kind`:

| Kind | Extra columns |
|---|---|
| `compute.vm` | `resource.vcpu`, `resource.memory_gb`, `resource.gpu_count`, `resource.gpu_model` |
| `storage.object` | `resource.durability_nines`, `resource.availability_tier` |
| `db.relational` | `resource.vcpu`, `resource.memory_gb`, `resource.storage_gb` |
| `llm.text` | `resource.context_length`, `resource.capabilities`, `health.uptime_30d`, `health.latency_p95_ms` |

## Composition with `--fields` and `--jq`

Presets apply first; `--fields` and `--jq` apply after. This means `--fields` works on the preset's projected keys:

```bash
# agent preset + just two keys
sku aws ec2 price --instance-type m5.large --region us-east-1 \
  --preset agent --fields resource.name,price.0.amount

# price preset + jq to pull a scalar
sku aws ec2 price --instance-type m5.large --region us-east-1 \
  --preset price --jq '.price[0].amount'
```

## Which preset for which caller

| Caller | Recommended preset |
|---|---|
| Interactive terminal use | `full --pretty` |
| Agent picking one SKU | `agent` (default) |
| Agent doing math on the amount | `price --jq '.price[0].amount'` |
| Agent comparing rows | `compare --limit N` |
| CI budget-diff tooling | `full` (for stable hash across minor revs) |

## Adding a preset

Presets are defined in `internal/output/presets.go`. New preset names become part of the stable CLI contract — adding one is a minor version bump; removing one is major.
