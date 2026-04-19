# Getting started with sku

By the end of this page you will have:

1. Installed `sku`.
2. Fetched pricing data for AWS EC2 and OpenRouter.
3. Run a point lookup, a cross-provider compare, an estimate, and a batch.
4. Understood how to pipe output into `jq` without post-processing.

Estimated time: **5 minutes**.

## 1. Install

Pick any one — see [install.md](install.md) for the full matrix:

```bash
brew install sofq/tap/sku
# or
pipx install sku-cli
```

Verify:

```bash
sku version --pretty
```

Expected: a JSON object with `version`, `commit`, `built`, `go_version`, `platform`.

## 2. Fetch pricing data

The binary is **offline-only**. `sku update` pulls daily deltas from the CDN and stores them under `$SKU_DATA_DIR` (default: `$XDG_DATA_HOME/sku`).

```bash
sku update openrouter aws-ec2
```

Expected exit 0. Check freshness:

```bash
sku update --status --pretty
```

## 3. Point lookup

```bash
sku aws ec2 price --instance-type m5.large --region us-east-1 --pretty
```

Expected: a JSON document with `provider`, `service`, `resource`, and a `price[]` array. Every numeric price is in USD-per-hour unless a different unit is declared in the row.

Agent-flavoured projection — drop the catalog metadata, keep just the price figure:

```bash
sku aws ec2 price --instance-type m5.large --region us-east-1 \
  --jq '.price[0].amount'
```

Or use a preset:

```bash
sku aws ec2 price --instance-type m5.large --region us-east-1 --preset price
```

Exit code 3 means no SKU matched ("not found") — the stderr envelope includes a `suggestion` and `nearest_matches`.

## 4. Cross-provider compare

```bash
sku compare --kind compute.vm --vcpu 4 --memory 16 --regions us-east \
  --limit 5 --preset compare --pretty
```

Expected: rows from AWS, Azure, and GCP sorted by price, unified under one schema. See [guides/llm-routing.md](guides/llm-routing.md) for the LLM equivalent.

## 5. Estimate

Inline DSL:

```bash
sku estimate \
  --item aws/ec2:m5.large:region=us-east-1:count=10:hours=730 \
  --item aws/ec2:m5.xlarge:region=us-east-1:count=1:hours=730 \
  --pretty
```

Or from a workload file:

```bash
sku estimate --config docs/examples/workload-vm.yaml --pretty
```

## 6. Batch

Run three ops in one invocation (all non-agent-shaped input forms are NDJSON or JSON-array on stdin):

```bash
cat docs/examples/batch-queries.ndjson | sku batch --pretty
```

The aggregated exit code is the **worst-case** of all ops (see [reference/exit-codes.md](reference/exit-codes.md)).

## Next

- Every command in detail: [`commands/`](commands/).
- Using sku from inside an agent: [`guides/agent-integration.md`](guides/agent-integration.md).
- What each *kind* means: [`reference/kinds.md`](reference/kinds.md).
