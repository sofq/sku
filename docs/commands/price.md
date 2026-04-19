# `sku price`

Universal point-lookup verb. In practice you'll almost always use the per-provider form (`sku aws ec2 price`, `sku llm price`) — those pages describe the flags each shard accepts.

## Synopsis

```
sku <provider> <service> price [flags]
sku llm price --model <id> [--serving-provider <name>] [flags]
```

## Flags

See [global flags](README.md#global-flags) and each provider page:

- AWS: [`aws.md`](aws.md)
- Azure: [`azure.md`](azure.md)
- GCP: [`gcp.md`](gcp.md)
- LLM: [`llm.md`](llm.md)

## Examples

```bash
sku aws ec2 price   --instance-type m5.large --region us-east-1 --preset agent
sku azure vm price  --arm-sku-name Standard_D2_v3 --region eastus --os linux
sku gcp  gce price  --machine-type n1-standard-2 --region us-east1
sku llm  price      --model anthropic/claude-opus-4.6 --serving-provider aws-bedrock
```

## Exit codes

Common: `0` ok, `3` not_found, `4` validation, `8` stale_data. Full taxonomy: [`../reference/exit-codes.md`](../reference/exit-codes.md).
