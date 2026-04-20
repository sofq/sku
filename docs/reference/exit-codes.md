# Exit codes & error envelope

Exit codes are a **contract**. Use them to branch in scripts and agents.

## Codes

| Exit | Code | Meaning |
|---|---|---|
| `0` | — | success |
| `1` | `generic_error` | unclassified; fall back to `error.message` |
| `2` | `auth` | CI-only: credential failure |
| `3` | `not_found` | no SKU matches filters |
| `4` | `validation` | input failed validation (see `reason`) |
| `5` | `rate_limited` | upstream rate limit (CDN or ingest) |
| `6` | `conflict` | state conflict (e.g. delta already applied) |
| `7` | `server` | upstream CDN or pricing API 5xx |
| `8` | `stale_data` | catalog older than the threshold and `--stale-ok` was not set |

Query the machine-readable catalog any time:

```bash
sku schema --errors --pretty
```

## Envelope (stderr)

All errors emit a single-line JSON object on **stderr**; stdout stays empty:

```json
{
  "error": {
    "code": "not_found",
    "message": "No SKU matches filters",
    "suggestion": "Try `sku schema aws ec2` to see valid filters",
    "details": {
      "provider": "aws",
      "service": "ec2",
      "applied_filters": {"instance_type": "m5.huge", "region": "us-east-1"},
      "nearest_matches": ["m5.large", "m5.xlarge"]
    }
  }
}
```

## Per-code `details` shape

Agents can branch on `error.details` — the shape is stable within a major version.

| `code` | `details` keys |
|---|---|
| `not_found` | `provider`, `service`, `applied_filters`, `nearest_matches?` |
| `validation` | `reason`, `flag?`, `value?`, `allowed?`, `shard?`, `required_binary_version?`, `hint?` |
| `auth` | `resource` (never credential material) |
| `rate_limited` | `retry_after_ms` |
| `conflict` | `shard`, `current_head_version`, `expected_from`, `operation` |
| `server` | `upstream`, `status_code?`, `correlation_id?` |
| `stale_data` | `shard`, `last_updated`, `age_days`, `threshold_days` |
| `generic_error` | free-form; agents should use `error.message` |

`validation.reason` is one of: `flag_invalid`, `binary_too_old`, `binary_too_new`, `shard_too_old`, `shard_too_new`.

## Aggregated exit (sku batch)

`sku batch` rolls individual per-op exits into one process exit code using a severity rank (high to low):

```
validation > server > conflict > rate_limited > auth > not_found > stale_data > ok
```

Every individual result is still present in the JSON array on stdout, with its own `exit_code` field.

## Branching example (bash)

```bash
sku aws ec2 price --instance-type "$T" --region "$R"
case $? in
  0) ;;                                   # got it
  3) echo "no such instance type" ;;      # not_found
  4) echo "bad flag or shard skew" ;;     # validation
  8) echo "catalog stale — run sku update" ;;
  *) echo "other failure" ;;
esac
```

## Branching example (Python)

```python
import json, subprocess
p = subprocess.run(
    ["sku", "aws", "ec2", "price", "--instance-type", t, "--region", r],
    capture_output=True,
)
if p.returncode == 0:
    data = json.loads(p.stdout)
elif p.returncode == 3:
    err = json.loads(p.stderr)
    hints = err["error"]["details"].get("nearest_matches", [])
    ...
```
