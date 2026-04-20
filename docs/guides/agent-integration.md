# Calling sku from an agent

`sku` is built for agent loops. This guide covers the four moves that pay off the most.

## 1. Start with `agent` preset

`agent` is the default — but if you've aliased `sku` through a profile that bumped `preset=full`, force it back:

```bash
sku --preset agent aws ec2 price --instance-type m5.large --region us-east-1
```

Output is ~200 bytes, JSON, with the minimum fields an agent needs: `provider`, `service`, `resource.name`, `location.provider_region`, `price`, `terms.commitment`.

## 2. Project aggressively

Agents rarely need the full record. Two tools do the same job with different ergonomics:

```bash
# --fields: declarative dot-path projection. Order preserved.
sku aws ec2 price --instance-type m5.large --region us-east-1 \
  --fields provider,resource.name,price.0.amount

# --jq: full gojq expression. Use when you need computation.
sku aws ec2 price --instance-type m5.large --region us-east-1 \
  --jq '{type: .resource.name, usd_hr: .price[0].amount}'
```

Combine both: `--fields` runs first, then `--jq` operates on the reduced tree.

## 3. Branch on exit code, parse stderr for details

Exit code is a cheap branch. Only parse stderr when you need the structured detail:

```bash
if out=$(sku aws ec2 price --instance-type "$T" --region "$R" 2>err.json); then
  amount=$(echo "$out" | jq -r '.price[0].amount')
else
  case $? in
    3) nearest=$(jq -r '.error.details.nearest_matches // [] | join(",")' err.json) ;;
    4) reason=$(jq -r '.error.details.reason' err.json) ;;
    8) echo "shard stale" ;;
    *) echo "unhandled"; cat err.json ;;
  esac
fi
```

Full exit-code table: [`../reference/exit-codes.md`](../reference/exit-codes.md).

## 4. Collapse N lookups into one `sku batch`

In-process dispatch. No shell-fork-per-op. No startup amortisation tricks.

```bash
cat <<'EOF' | sku batch --pretty
{"command":"aws ec2 price","args":{"instance_type":"m5.large","region":"us-east-1"}}
{"command":"aws ec2 price","args":{"instance_type":"m5.xlarge","region":"us-east-1"}}
{"command":"llm price","args":{"model":"anthropic/claude-opus-4.6"}}
EOF
```

Output is a JSON array with each op's own `exit_code` + body; the **process** exit is the aggregated worst-case (see [exit-codes.md](../reference/exit-codes.md)).

Discover the exact `command` strings `batch` accepts:

```bash
sku schema --list-commands
```

## 5. Avoid implicit auto-fetch in agent flows

`--auto-fetch` is a convenience for humans. In an agent loop, prefer the deterministic two-step:

```bash
sku update --status --pretty | jq -e '.shards[] | select(.name=="aws-ec2" and .installed==false)' >/dev/null \
  && sku update aws-ec2
sku aws ec2 price --instance-type m5.large --region us-east-1
```

This keeps shard fetches (slow, network) out of the hot path and explicit in the agent's trace.

## 6. Profile your caller

Stamp one profile per agent persona:

```bash
sku configure --profile copilot \
  --set preset=agent \
  --set stale_warning_days=14 \
  --set auto_fetch=false
```

Then invoke with `--profile copilot` or `SKU_PROFILE=copilot`. Keeps the CLI invocation short and behaviour reproducible.
