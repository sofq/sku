# `sku configure`

Create or edit a named profile in `$SKU_CONFIG_DIR/config.yaml` (default: `$XDG_CONFIG_HOME/sku/config.yaml`).

## Synopsis

```
sku configure                               # interactive
sku configure --profile <name> --set key=value  # scripted
sku configure --profile <name> --show --pretty
```

## Scripted example

```bash
sku configure --profile work \
  --set preset=agent \
  --set format=json \
  --set auto_fetch=true
```

## Using a profile

```bash
sku --profile work aws ec2 price --instance-type m5.large --region us-east-1
# or
SKU_PROFILE=work sku aws ec2 price --instance-type m5.large --region us-east-1
```

Precedence: CLI flag > env > profile > default.

## Exit codes

`0`, `4` (invalid key/value).
