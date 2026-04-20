# `sku version`

Prints build metadata as JSON. Safe to parse from scripts.

```bash
sku version --pretty
```

```json
{
  "version":  "0.1.0",
  "commit":   "abc1234",
  "built":    "2026-04-19T12:00:00Z",
  "go_version": "go1.25.2",
  "platform": "linux/amd64"
}
```

Exit `0` always.
