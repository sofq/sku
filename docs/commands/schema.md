# `sku schema`

Machine-readable discovery endpoint. Every flag returns JSON so agents can introspect without parsing help text.

## Flags

| Flag | Emits |
|---|---|
| `--errors` | Full error-code catalog: `{codes: {name: {exit_code, description, details_fields, reasons?}}, schema_version}` |
| `--list-commands` | Registered batch commands (exact strings for `sku batch` input) |
| `--list-serving-providers` | Serving providers seeded in the `openrouter` shard (reads `metadata.serving_providers`) |
| `--list-shards` | Shards the binary statically registers + which are installed locally |
| `--list-kinds` | Kinds this binary understands + which shards contribute |

## Examples

```bash
sku schema --errors                  | jq '.codes | keys'
sku schema --list-commands           | jq '.commands'
sku schema --list-serving-providers
```

## Exit codes

`0` ok; `3` if an asked-for shard is missing (e.g. `--list-serving-providers` without `openrouter` installed).
