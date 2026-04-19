# LLM routing by price

`sku`'s LLM data is uniform across 70+ serving providers, so you can answer "what is the cheapest way to run model X?" with one command.

## Cheapest serving provider for a model

```bash
sku llm compare --model anthropic/claude-opus-4.6 --preset compare --pretty
```

Expected: sorted list of serving providers that carry that model, cheapest first. Aggregated OpenRouter rows are **excluded** by default (see [serving-providers.md](../reference/serving-providers.md)).

Pick just the winner:

```bash
sku llm compare --model anthropic/claude-opus-4.6 \
  --jq '[.[] | select(.resource.attributes.aggregated != true)] | sort_by(.price[0].amount)[0]'
```

## Cheapest model for a budget

```bash
sku llm list --max-input-price 1.0 --max-output-price 5.0 --preset agent --sort input-price
```

Prices are USD per **million tokens** for `llm.text`. `--max-input-price 1.0` means ≤ \$1/M input tokens.

## Capability-filtered routing

```bash
sku llm list --capability tool_use --capability vision \
  --max-input-price 3.0 --preset agent --sort input-price
```

Capabilities are enumerated in each model's `resource.capabilities` array.

## Cost for a specific workload

Use `estimate`:

```bash
sku estimate \
  --item 'llm:anthropic/claude-opus-4.6:input=1M:output=500K:serving_provider=aws-bedrock' \
  --pretty
```

Or from YAML:

```bash
sku estimate --config docs/examples/workload-llm.yaml --pretty
```

## Why price alone isn't enough

Three second-order factors, all present in the `llm.text` rows:

| Column | Meaning |
|---|---|
| `health.uptime_30d` | Serving provider's availability over the last 30d |
| `health.latency_p95_ms` | P95 first-token latency (ms) |
| `resource.context_length` | Hard context window |

Route on a composite that weighs price + uptime + latency:

```bash
sku llm compare --model anthropic/claude-opus-4.6 \
  --jq '[.[] | select(.resource.attributes.aggregated != true)]
        | map(. + {score: (.price[0].amount - .health.uptime_30d*0.01 + .health.latency_p95_ms*0.0001)})
        | sort_by(.score)[0]'
```

(Example scoring — tune weights to your SLO.)

## Aggregated OpenRouter row

OpenRouter publishes a synthetic *aggregated* rate across all its endpoints for a given model. To see it:

```bash
sku openrouter llm price --model anthropic/claude-opus-4.6        # only aggregated rows
sku --include-aggregated llm compare --model anthropic/claude-opus-4.6
```

For most routing decisions you want the **concrete** serving providers, not the aggregated row (the aggregated row is not a real endpoint you can call).
