# Command reference

Every command accepts the [global flags](#global-flags) below. Exit codes are documented in [`../reference/exit-codes.md`](../reference/exit-codes.md).

| Command | Purpose | Page |
|---|---|---|
| `price` | Universal point lookup | [price.md](price.md) |
| `search` | Filter SKUs in one shard | [search.md](search.md) |
| `compare` | Cross-provider equivalence | [compare.md](compare.md) |
| `estimate` | Workload → monthly cost | [estimate.md](estimate.md) |
| `batch` | Many ops from stdin | [batch.md](batch.md) |
| `schema` | Discovery / introspection | [schema.md](schema.md) |
| `update` | Fetch or refresh a shard | [update.md](update.md) |
| `configure` | Named profiles | [configure.md](configure.md) |
| `version` | Build metadata | [version.md](version.md) |
| `aws` | AWS provider subtree | [aws.md](aws.md) |
| `azure` | Azure provider subtree | [azure.md](azure.md) |
| `gcp` | GCP provider subtree | [gcp.md](gcp.md) |
| `llm` | Cross-provider LLM + serving-provider views | [llm.md](llm.md) |

## Global flags

All commands accept:

| Flag | Env | Meaning |
|---|---|---|
| `--profile` | `SKU_PROFILE` | Named profile |
| `--preset {agent,full,price,compare}` | `SKU_PRESET` | Field projection |
| `--json` / `--yaml` / `--toml` | `SKU_FORMAT` | Output format |
| `--pretty` | – | Indent JSON |
| `--jq <expr>` | – | Filter via gojq |
| `--fields <paths>` | – | Comma-separated dot-path projection |
| `--include-raw` | – | Include raw passthrough row |
| `--include-aggregated` | – | Include OpenRouter aggregated rows |
| `--stale-ok` | `SKU_STALE_OK` | Suppress stale-catalog warning |
| `--auto-fetch` | – | Download missing shards on demand |
| `--dry-run` | `SKU_DRY_RUN` | Print plan; no execution |
| `--verbose` | `SKU_VERBOSE` | Stderr JSON log |
| `--no-color` | `NO_COLOR` / `SKU_NO_COLOR` | Disable color |

Precedence: CLI flag > env var > profile value > default.

## Output conventions

- **JSON is the default.** YAML and TOML are exact mappings of the same JSON tree (TOML wraps top-level arrays as `{ "rows": [...] }` — spec §CLAUDE.md *TOML quirks*).
- **Errors go to stderr as JSON** with `{ "error": { "code", "message", "suggestion", "details" } }` and a semantic exit code.
- **Every successful price response** carries `price[]` with `unit`, `currency`, `amount`, and (optional) `tier`.
