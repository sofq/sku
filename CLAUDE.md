# CLAUDE.md

Agent quick-start for the `sku` repo.

## What this is

`sku` is an agent-friendly CLI for querying cloud + LLM pricing. Offline-only client, daily data pipeline, pure-Go binary. See `docs/superpowers/specs/2026-04-18-sku-design.md` for the full design.

## Dev commands

| Task | Command |
|---|---|
| Build binary | `make build` (output: `bin/sku`) |
| Run tests | `make test` |
| Lint | `make lint` |
| Release dry-run | `make release-dry` |
| Regenerate code/docs | `make generate` (no-op until M4) |
| Build local OpenRouter shard | `make openrouter-shard` |
| Run Go integration tests | `make test-integration` |
| Run benchmarks | `make bench` |
| Run Python pipeline tests | `make pipeline-test` |

## Repo map

- `cmd/sku/` — Cobra command tree (thin; no business logic)
- `internal/` — all logic lives here; packages are added per milestone
- `internal/version/` — single source of truth for build metadata
- `pipeline/` — CI-only data pipeline (arrives in M1+)
- `docs/superpowers/specs/` — design spec (rev 4 dated 2026-04-18)
- `docs/superpowers/plans/` — per-milestone implementation plans
- `.github/workflows/` — `ci.yml` (PR/push), `release.yml` (tag), and data workflows from M3a

## Patterns

- **Pure Go, no CGO.** Every dependency must cross-compile without Docker tricks.
- **`cmd/` stays thin.** Flag parsing + calls into `internal/`. No business logic.
- **TDD.** Write failing test, implement, commit.
- **Exit codes are contract** (spec §4). Full taxonomy is live as of M2; `sku schema --errors` emits the machine-readable catalog.
- **Plans are session-sized.** One plan file = one `claude -p` session via `scripts/run-plan.sh`. Target ≤ ~25 tasks / ~100 checkboxes per plan. Split large milestones by sub-scope (e.g. M3a → `m3a.1-ec2-rds`, `m3a.2-s3-lambda-ebs`, `m3a.3-dynamodb-cloudfront-updater`). File names must lex-sort into build order; `scripts/run-spec.sh` picks plans up in that order.

## Current milestone

M2 — Output polish & CLI ergonomics. See `docs/superpowers/plans/2026-04-18-m2-output-and-ergonomics.md`.

### Quick path (agent, repeatable)

```bash
make openrouter-shard                                   # build local shard from fixtures
export SKU_DATA_DIR=$(pwd)/dist/pipeline

./bin/sku llm price --model anthropic/claude-opus-4.6 --preset agent
./bin/sku llm price --model anthropic/claude-opus-4.6 --preset price \
  --serving-provider aws-bedrock                        # price-only preset
./bin/sku llm price --model anthropic/claude-opus-4.6 --yaml --pretty
./bin/sku llm price --model anthropic/claude-opus-4.6 \
  --jq '.price[0].amount' --serving-provider aws-bedrock
./bin/sku llm price --model anthropic/claude-opus-4.6 \
  --fields provider,price.0.amount --serving-provider aws-bedrock
./bin/sku llm price --model anthropic/claude-opus-4.6 --dry-run
./bin/sku schema --errors                               # error-code catalog
./bin/sku schema --list-serving-providers
```

## Global flags (all subcommands)

`--profile`, `--preset {agent|full|price|compare}`, `--json|--yaml|--toml`, `--pretty`,
`--jq <expr>`, `--fields <paths>`, `--include-raw`, `--include-aggregated`, `--stale-ok`,
`--auto-fetch`, `--dry-run`, `--verbose`, `--no-color`. Env equivalents: `SKU_PROFILE`,
`SKU_PRESET`, `SKU_FORMAT`, `SKU_STALE_OK`, `SKU_STALE_ERROR_DAYS`, `NO_COLOR`,
`SKU_NO_COLOR`, `SKU_VERBOSE`, `SKU_DRY_RUN`. Config file at `$SKU_CONFIG_DIR/config.yaml`
(resolved via `sku configure`). Precedence: CLI > env > profile > default.

## TOML quirks

TOML cannot represent top-level arrays. `sku` wraps any top-level `[]` as `{ "rows": [...] }`
before emitting TOML; agents consuming `--toml` should look under `rows`.
