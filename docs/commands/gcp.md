# `sku gcp`

| Leaf | Shard | Key flags (`price`) |
|---|---|---|
| `gce` | `gcp-gce` | `--machine-type`, `--region` |
| `cloud-sql` | `gcp-cloud-sql` | `--tier`, `--region`, `--engine`, `--deployment-option` |
| `gcs` | `gcp-gcs` | `--storage-class`, `--region` |
| `run` | `gcp-run` | `--architecture`, `--region` |
| `functions` | `gcp-functions` | `--architecture`, `--region` |

## Examples

```bash
sku gcp gce price       --machine-type n1-standard-2 --region us-east1 --preset agent
sku gcp cloud-sql price --tier db-custom-2-7680 --region us-east1 --engine postgres --deployment-option zonal
sku gcp gcs price       --storage-class standard --region us-east1
sku gcp run price       --architecture x86_64 --region us-east1
sku gcp functions price --architecture x86_64 --region us-east1
```
