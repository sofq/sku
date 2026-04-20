# `sku estimate`

Compute monthly cost from a workload spec. Three input forms, all equivalent:

1. `--item <inline-dsl>` (repeatable)
2. `--config <yaml>` (may include multiple items)
3. `--stdin` (JSON on stdin)

## Inline DSL

```
<provider>/<service>:<resource>[:key=value[:key=value...]]
llm:<model>:[input=<count>][:output=<count>][:serving_provider=<name>]
```

Numeric suffixes `K`, `M`, `G` are accepted on `input` / `output` LLM token counts.

## Examples

```bash
# Compute
sku estimate --item aws/ec2:m5.large:region=us-east-1:count=10:hours=730 --pretty

# Mix two items
sku estimate --item aws/ec2:m5.large:region=us-east-1:count=2:hours=100 \
             --item aws/ec2:m5.xlarge:region=us-east-1:count=1:hours=730

# From YAML
sku estimate --config docs/examples/workload-vm.yaml --pretty

# From stdin JSON
echo '{"items":[{"provider":"aws","service":"ec2","resource":"m5.large",
  "params":{"region":"us-east-1","count":2,"hours":100}}]}' \
  | sku estimate --stdin --pretty

# Storage
sku estimate --item 'aws/s3:standard:region=us-east-1:gb_month=500:put_requests=1000:get_requests=5000' --pretty

# LLM
sku estimate --item 'llm:anthropic/claude-opus-4.6:input=1M:output=500K:serving_provider=anthropic' --pretty
sku estimate --config docs/examples/workload-llm.yaml --pretty
```

## Output shape

```json
{
  "items": [ { "provider": "...", "service": "...", "resource": "...", "params": {...}, "cost_usd_monthly": 73.00, "price_source": {...} } ],
  "total_usd_monthly": 730.00
}
```

## Exit codes

`0`, `3`, `4` (bad DSL), `8`.
