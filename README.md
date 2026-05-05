# (Alpha - unstable) sku — agent-friendly cloud & LLM pricing CLI

[![CI](https://github.com/sofq/sku/actions/workflows/ci.yml/badge.svg)](https://github.com/sofq/sku/actions/workflows/ci.yml)
[![Release](https://img.shields.io/github/v/release/sofq/sku?include_prereleases&sort=semver)](https://github.com/sofq/sku/releases)
[![License](https://img.shields.io/badge/license-Apache--2.0-blue)](LICENSE)

`sku` answers "what does this cost?" for AWS, Azure, Google Cloud, and OpenRouter in one pure-Go binary. JSON-everywhere, semantic exit codes, sub-30ms warm point lookups — purpose-built for AI agents that make programmatic pricing decisions.

## Five-minute quickstart

```bash
# Install
brew install sofq/tap/sku         # or: npx @sofq/sku, pipx install sku-cli,
                                  # scoop install sku, docker run ghcr.io/sofq/sku

# Fetch pricing data (first run only)
sku update openrouter aws-ec2

# Point lookup
sku aws ec2 price --instance-type m5.large --region us-east-1 --pretty

# Cross-provider compare
sku compare --kind compute.vm --vcpu 4 --memory 16 --regions us-east --limit 5 --pretty

# Estimate monthly cost
sku estimate --item aws/ec2:m5.large:region=us-east-1:count=10:hours=730 --pretty

# Cheapest LLM for long context
sku llm price --model anthropic/claude-opus-4.6 --pretty
```

See **[`docs/getting-started.md`](docs/getting-started.md)** for the annotated walkthrough.

## Why sku

- **Offline-first.** Daily pricing data ships via `sku update`; the binary never calls provider APIs. Runs in airgapped CI.
- **Agent-shaped output.** Every response is JSON; pipe into `jq`, or use `--jq`, `--fields`, and `--preset` to project what you need. Exit codes are a contract ([`docs/reference/exit-codes.md`](docs/reference/exit-codes.md)).
- **Four providers, one schema.** AWS, Azure, GCP, and 70+ LLM serving providers via OpenRouter share a single `price[]` shape — cross-provider `compare` and `search` just work.
- **Pure Go, zero CGO.** Cross-compiles to Linux / macOS / Windows × amd64 / arm64. Signed releases with SLSA L3 provenance and SBOMs.

## Install

```bash
# Homebrew (macOS/Linux)
brew install sofq/tap/sku

# Scoop (Windows)
scoop bucket add sofq https://github.com/sofq/scoop-bucket
scoop install sku

# npm (JS/TS toolchains, uses platform-optional-dependencies)
npm i -g @sofq/sku        # or: npx @sofq/sku <cmd>

# PyPI (Python toolchains)
pipx install sku-cli       # or: pip install --user sku-cli

# Docker
docker pull ghcr.io/sofq/sku:latest
docker run --rm ghcr.io/sofq/sku:latest version

# Direct download (all else)
# see https://github.com/sofq/sku/releases
```

Full install doc including signature verification: [`docs/install.md`](docs/install.md).

## Commands

Full per-command reference: [`docs/commands/`](docs/commands/).

| Command | Purpose |
|---|---|
| [`sku price`](docs/commands/price.md) / `sku <provider> <service> price` | Point lookup |
| [`sku search`](docs/commands/search.md) | Filter SKUs within one shard |
| [`sku compare`](docs/commands/compare.md) | Cross-provider equivalence |
| [`sku estimate`](docs/commands/estimate.md) | Workload → monthly cost |
| [`sku batch`](docs/commands/batch.md) | NDJSON / JSON-array of multiple ops |
| [`sku schema`](docs/commands/schema.md) | Discover commands, error codes, serving providers |
| [`sku update`](docs/commands/update.md) | Fetch / refresh a shard |
| [`sku configure`](docs/commands/configure.md) | Manage named profiles |
| [`sku version`](docs/commands/version.md) | Build metadata (JSON) |

## Guides

- [Agent integration](docs/guides/agent-integration.md)
- [LLM routing](docs/guides/llm-routing.md)
- [Offline & airgapped use](docs/guides/offline-use.md)

## Reference

- [Kinds](docs/reference/kinds.md) — unified `compute.vm`, `storage.object`, `db.relational`, `llm.text`
- [Presets](docs/reference/presets.md) — `agent`, `full`, `price`, `compare`
- [Exit codes](docs/reference/exit-codes.md) — the contract for automation

## Security & support

- Found a bug? File an [issue](https://github.com/sofq/sku/issues).
- Found a vulnerability? See [`SECURITY.md`](SECURITY.md).
- [`CHANGELOG.md`](CHANGELOG.md) · [`LICENSE`](LICENSE) (Apache-2.0).
