# M3a.2 Bootstrap Release — AWS S3 + Lambda + EBS Shards

One-shot maintainer operation. After m3a.3 wires `data-daily.yml`, daily
releases take over and this runbook is retired. Mirrors
`docs/ops/m3a1-bootstrap-release.md`.

## Producing the shards

```bash
# Live ingest (requires network). For m3a.2 the three offers come from:
#   AmazonS3       -> s3 (storage.object)
#   AWSLambda      -> lambda (compute.function)
#   AmazonEC2      -> ebs (storage.block, productFamily=Storage rows only)
# For the bootstrap only, ingest the five in-scope regions: us-east-1,
# us-east-2, us-west-2, eu-west-1, ap-northeast-1. Concatenate per-region
# offer JSONs into a single offer.json per shard.

make aws-s3-shard     SHARD_SRC=/path/to/s3/offer.json
make aws-lambda-shard SHARD_SRC=/path/to/lambda/offer.json
make aws-ebs-shard    SHARD_SRC=/path/to/ec2/offer.json  # same file as aws-ec2

for shard in aws-s3 aws-lambda aws-ebs; do
  zstd -19 dist/pipeline/$shard.db -o dist/pipeline/$shard.db.zst
  sha256sum dist/pipeline/$shard.db.zst > dist/pipeline/$shard.db.zst.sha256
done
```

## Uploading

Create three GitHub releases with tags `data-bootstrap-aws-s3`,
`data-bootstrap-aws-lambda`, `data-bootstrap-aws-ebs` and attach the
`.zst` + `.sha256` assets for each. The URLs in `cmd/sku/update.go`'s
`shardSources` map resolve to these assets.

## Verifying end-to-end on a fresh machine

```bash
export SKU_DATA_DIR=$(mktemp -d)
for shard in aws-s3 aws-lambda aws-ebs; do sku update $shard; done

sku aws s3     price --storage-class standard --region us-east-1
sku aws lambda price --architecture arm64     --region us-east-1
sku aws ebs    price --volume-type gp3        --region us-east-1
```
