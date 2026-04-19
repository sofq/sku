# Global flags

Every `sku` command accepts these. Precedence (high → low): **CLI flag > env var > profile value > default**.

| Flag | Env var | Default | Meaning |
|---|---|---|---|
| `--profile <name>` | `SKU_PROFILE` | `default` | Named profile from `$SKU_CONFIG_DIR/config.yaml` |
| `--preset {agent,full,price,compare}` | `SKU_PRESET` | `agent` | Field projection (see [presets.md](presets.md)) |
| `--json` / `--yaml` / `--toml` | `SKU_FORMAT` | `json` | Output format |
| `--pretty` | – | off | Indent output |
| `--jq <expr>` | – | – | Apply a gojq filter to the response body |
| `--fields <paths>` | – | – | Comma-separated dot-path projection (e.g. `provider,price.0.amount`) |
| `--include-raw` | – | off | Attach the raw upstream row to each record |
| `--include-aggregated` | – | off | Include OpenRouter's aggregated synthetic rows |
| `--stale-ok` | `SKU_STALE_OK` | off | Suppress the stale-catalog warning |
| `--stale-error-days` (via profile) | `SKU_STALE_ERROR_DAYS` | unset | Escalate stale warning to exit code 8 beyond N days |
| `--auto-fetch` | – | off | Download missing shards on demand |
| `--dry-run` | `SKU_DRY_RUN` | off | Print resolved query plan, do not execute |
| `--verbose` | `SKU_VERBOSE` | off | Emit a JSON log line to stderr |
| `--no-color` | `NO_COLOR`, `SKU_NO_COLOR` | off | Disable ANSI color in `--pretty` output |

## TOML quirk

TOML forbids top-level arrays. `--toml` wraps any top-level `[]` as `{"rows": [...]}`. Agents that dispatch on format should look under `rows` when parsing TOML output.

## Profile resolution

`$SKU_CONFIG_DIR` default:

| OS | Default |
|---|---|
| Linux | `$XDG_CONFIG_HOME/sku/config.yaml` |
| macOS | `$HOME/Library/Application Support/sku/config.yaml` |
| Windows | `%APPDATA%\sku\config.yaml` |

Managed via [`sku configure`](../commands/configure.md).
