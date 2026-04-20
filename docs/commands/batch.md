# `sku batch`

Run multiple operations in a single process. Input is NDJSON or a JSON array on stdin; the output is always a JSON array of records, one per input op.

## Synopsis

```
sku batch [--concurrency N] [--continue-on-error] [flags] < input
```

## Input record shape

```json
{"command": "aws ec2 price", "args": {"instance_type": "m5.large", "region": "us-east-1"}}
```

`args` keys are the op's flag names with dashes replaced by underscores (e.g. `instance-type` → `instance_type`).

## Examples

```bash
echo '[
  {"command":"aws ec2 price","args":{"instance_type":"m5.large","region":"us-east-1"}},
  {"command":"llm price",    "args":{"model":"anthropic/claude-opus-4.6"}}
]' | sku batch --pretty

cat docs/examples/batch-queries.ndjson | sku batch
```

## Aggregated exit code

Aggregated exit = **worst-case** of all ops, by severity rank (validation > not_found > stale_data > ok). The full ranking and JSON envelope shape are in [`../reference/exit-codes.md`](../reference/exit-codes.md).

## Discovering registered commands

```bash
sku schema --list-commands
```

Returns the exact `command` strings `batch` accepts.
