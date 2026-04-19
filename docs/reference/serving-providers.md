# LLM serving providers

The `openrouter` shard carries prices for 70+ *serving providers* — the entity that actually serves a model to you, not the model's author. Filter and view semantics are in [`../commands/llm.md`](../commands/llm.md).

## Getting the live list

The shard's `metadata.serving_providers` row is the source of truth. Query it any time:

```bash
sku update openrouter        # if not installed
sku schema --list-serving-providers
```

Returns `{"serving_providers": ["anthropic", "aws-bedrock", "azure-openai", "gcp-vertex", "openai", "openrouter", ...]}`.

## Top-level subcommands vs `--serving-provider`

A handful of serving providers have dedicated top-level subcommands for ergonomics. These are **static**; newly appearing serving providers work via `--serving-provider` immediately, but a dedicated subcommand requires a binary release.

| Subcommand | Valid from | `--serving-provider` equivalent |
|---|---|---|
| `sku anthropic llm price` | v0.1.0 | `--serving-provider anthropic` |
| `sku openai llm price` | v0.1.0 | `--serving-provider openai` |
| `sku aws-bedrock llm price` | v0.1.0 | `--serving-provider aws-bedrock` |
| `sku gcp-vertex llm price` | v0.1.0 | `--serving-provider gcp-vertex` |
| `sku azure-openai llm price` | v0.1.0 | `--serving-provider azure-openai` |
| `sku openrouter llm price` | v0.1.0 | returns **only** aggregated rows |

## Aggregated-row semantics

OpenRouter publishes a synthetic *aggregated* row per model representing its own routed rate. These rows are excluded by default from `sku llm price`, `sku llm list`, `sku llm compare`, and `sku compare --kind llm.text` so `min(price)` queries are honest.

Include them one of two ways:

```bash
sku --include-aggregated llm price --model anthropic/claude-opus-4.6
sku openrouter llm price          --model anthropic/claude-opus-4.6
```

Aggregated rows carry `resource.attributes.aggregated = true`.

## Validation source

The daily validation harness (spec §6) cross-checks OpenRouter's reported Bedrock / Vertex / Azure-OpenAI rates against the matching cloud pricing APIs at a 1% tolerance. Drift fails the release and auto-files `llm-rate-mismatch: <serving-provider>/<model>`.
