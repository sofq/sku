# `sku llm` and serving-provider subtrees

LLM pricing lives in a single `openrouter` shard. Two surfaces query it:

1. `sku llm {price,list,compare}` — cross-provider view.
2. `sku <serving-provider> llm {price,list}` — scoped to one serving provider (`anthropic`, `openai`, `aws-bedrock`, `gcp-vertex`, `azure-openai`, `openrouter`).

All serving providers registered in the shard are discoverable:

```bash
sku schema --list-serving-providers
```

## `sku llm price`

```bash
sku llm price --model anthropic/claude-opus-4.6 --pretty
sku llm price --model anthropic/claude-opus-4.6 --serving-provider aws-bedrock
sku llm price --model anthropic/claude-opus-4.6 --fields provider,price.0.amount
sku llm price --model anthropic/claude-opus-4.6 --jq '.price[0].amount' --serving-provider aws-bedrock
sku llm price --model anthropic/claude-opus-4.6 --dry-run
```

## Aggregated rows

`sku llm price` / `list` / `compare` **exclude** OpenRouter's synthetic aggregated rows by default (marked with `resource.attributes.aggregated = true`). Two ways to include them:

```bash
sku --include-aggregated llm price --model anthropic/claude-opus-4.6
sku openrouter llm price          --model anthropic/claude-opus-4.6   # aggregated-only view
```

## Serving-provider view (`sku <serving-provider> llm`)

```bash
sku aws-bedrock  llm price --model anthropic/claude-opus-4.6
sku anthropic    llm price --model anthropic/claude-opus-4.6
sku openai       llm price --model openai/gpt-4o-mini
```

A serving-provider leaf filters `WHERE provider = '<name>'`. See the guide [`../guides/llm-routing.md`](../guides/llm-routing.md).
