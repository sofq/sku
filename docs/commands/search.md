# `sku search`

Filter SKUs inside a single shard. Use [`compare`](compare.md) for cross-provider; `search` is one provider/service at a time.

## Synopsis

```
sku search --provider <aws|azure|gcp> --service <name> [filters] [flags]
```

## Flags

| Flag | Meaning |
|---|---|
| `--provider` | Required. One of `aws`, `azure`, `gcp` (not `llm` — use `sku llm list`). |
| `--service` | Required. Shard name within the provider (e.g. `ec2`, `vm`, `gce`). |
| `--kind` | Filter on unified kind (`compute.vm`, `storage.object`, …). |
| `--region` | Filter on region. |
| `--min-vcpu`, `--max-vcpu` | Numeric bounds on `resource.specs.vcpu`. |
| `--min-memory`, `--max-memory` | GiB bounds. |
| `--min-price`, `--max-price` | USD/unit bounds on `price[0].amount`. |
| `--sort` | `price` (default) or `resource`. |
| `--limit` | Cap row count. |

## Examples

```bash
sku search --provider aws --service ec2 --min-vcpu 4 --limit 5 --preset agent
sku search --provider aws --service ec2 --max-price 0.10 --sort price
sku search --provider aws --service ec2 --region us-east-1 --kind compute.vm
```

Output: JSON array of SKU objects (same schema as `price`). Use `--preset agent` to drop catalog metadata.

## Exit codes

`0`, `3`, `4`, `8`. Full: [`../reference/exit-codes.md`](../reference/exit-codes.md).
