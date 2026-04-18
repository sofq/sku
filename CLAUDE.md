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
- **Exit codes are contract** (spec §4). Not wired up in M0; full taxonomy arrives in M2.

## Current milestone

M1 — OpenRouter shard + catalog reader + `sku llm price`. See `docs/superpowers/plans/2026-04-18-m1-openrouter-shard-and-llm-price.md`.

### Quick path (agent, repeatable)

```bash
make openrouter-shard                                   # build local shard from fixtures
SKU_DATA_DIR=$(pwd)/dist/pipeline ./bin/sku llm price \
  --model anthropic/claude-opus-4.6                     # two JSON lines out
SKU_DATA_DIR=$(pwd)/dist/pipeline ./bin/sku llm price \
  --model anthropic/claude-opus-4.6 --pretty            # indented
```
