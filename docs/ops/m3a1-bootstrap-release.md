# M3a.1 Bootstrap Release — AWS EC2 + RDS Shards

This is a one-shot maintainer operation. After m3a.3 wires `data-daily.yml`,
daily releases take over and this runbook is retired.

## Producing the shards

```bash
# Live ingest (requires network)
curl -fsSL 'https://pricing.us-east-1.amazonaws.com/offers/v1.0/aws/AmazonEC2/current/index.json' \
  | jq -r '.regions | to_entries[] | "\(.key)\t\(.value.currentVersionUrl)"' \
  > /tmp/ec2-region-index.tsv
# For m3a.1 bootstrap, ingest only the five in-scope regions:
# us-east-1, us-east-2, us-west-2, eu-west-1, ap-northeast-1.
# Concatenate their offer JSONs into one offer.json for the pipeline.

make aws-ec2-shard SHARD_SRC=/path/to/concat/offer.json
make aws-rds-shard SHARD_SRC=/path/to/rds/offer.json

zstd -19 dist/pipeline/aws-ec2.db -o dist/pipeline/aws-ec2.db.zst
zstd -19 dist/pipeline/aws-rds.db -o dist/pipeline/aws-rds.db.zst
sha256sum dist/pipeline/aws-ec2.db.zst > dist/pipeline/aws-ec2.db.zst.sha256
sha256sum dist/pipeline/aws-rds.db.zst > dist/pipeline/aws-rds.db.zst.sha256
```

## Uploading

Create a GitHub release with the tag `data-bootstrap-aws-ec2` (and likewise
`data-bootstrap-aws-rds`) and attach the `.zst` + `.sha256` assets. The URLs
in `cmd/sku/update.go`'s `shardSources` map resolve to these assets.

## Verifying end-to-end on a fresh machine

```bash
SKU_DATA_DIR=$(mktemp -d)
sku update aws-ec2
sku update aws-rds
sku aws ec2 price --instance-type m5.large --region us-east-1
sku aws rds price --instance-type db.m5.large --region us-east-1 \
  --engine postgres --deployment-option single-az
```
