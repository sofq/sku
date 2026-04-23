# Kinds

The *kind* is the unified type assigned to every SKU at ingest. It is what makes cross-provider `compare` and `search --kind` possible. The authoritative taxonomy and per-kind equivalence rules live in `internal/compare/kinds/*.go`; attribute schemas live in `internal/schema/kinds/`.

## Taxonomy

```
compute.vm        compute.function       compute.container     compute.kubernetes
compute.batch
storage.object    storage.block          storage.file          storage.archive
db.relational     db.nosql               db.inmemory           db.warehouse
network.cdn       network.dns            network.loadbalancer
queue.messaging
security.secrets  security.kms
observability.logs   observability.metrics
llm.text          llm.multimodal         llm.embedding
llm.image         llm.audio
```

Not every kind has shipped yet. Query the set your binary recognises:

```bash
sku schema --list-kinds   # planned — binary support lands as a follow-up in v1.0
```

## Shards currently contributing rows (v0.1 baseline)

| Kind | Shards |
|---|---|
| `compute.vm` | `aws-ec2`, `azure-vm`, `gcp-gce` |
| `compute.function` | `aws-lambda`, `azure-functions`, `gcp-run`, `gcp-functions` |
| `storage.object` | `aws-s3`, `azure-blob`, `gcp-gcs` |
| `storage.block` | `aws-ebs`, `azure-disks` |
| `db.relational` | `aws-rds`, `azure-sql`, `azure-postgres`, `azure-mysql`, `azure-mariadb`, `gcp-cloud-sql` |
| `db.nosql` | `aws-dynamodb` |
| `network.cdn` | `aws-cloudfront` |
| `llm.text` | `openrouter` (70+ serving providers) |

## Per-kind shape (what `compare` sees)

### `compute.vm`

| Field | Source |
|---|---|
| `resource.vcpu` | int |
| `resource.memory_gb` | number |
| `resource.gpu_count` | int (0 when absent) |
| `resource.gpu_model` | string (`""` when absent) |
| `resource.attributes.family` | e.g. `m5`, `n1-standard` |

### `storage.object`

| Field | Source |
|---|---|
| `resource.storage_class` | `standard`, `hot`, `cool`, `archive`, … |
| `resource.durability_nines` | int |
| `resource.availability_tier` | `standard`, `reduced-redundancy`, … |

### `db.relational`

| Field | Source |
|---|---|
| `resource.vcpu`, `resource.memory_gb`, `resource.storage_gb` | numeric |
| `resource.engine` | `postgres`, `mysql`, `mariadb`, `sqlserver`, … |
| `resource.deployment_option` | `single-az`, `multi-az`, `flexible-server`, `zonal`, `regional` |

### `llm.text`

| Field | Source |
|---|---|
| `resource.name` | `anthropic/claude-opus-4.6` etc. |
| `resource.context_length` | int (tokens) |
| `resource.capabilities` | array of strings (`tool_use`, `vision`, `cache`, …) |
| `health.uptime_30d`, `health.latency_p95_ms` | OpenRouter uptime telemetry |

## Adding a new kind

New kinds are a binary-version bump (requires registration in `internal/compare/kinds/` and `internal/schema/kinds/`). Tracked in spec §4 *Data/code decoupling*.
