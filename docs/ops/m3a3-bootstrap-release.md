# M3a.3 Bootstrap Release — AWS DynamoDB + CloudFront Shards

One-shot maintainer operation. After m3a.4 wires `data-daily.yml`, daily
releases take over and this runbook is retired. Mirrors
`docs/ops/m3a1-bootstrap-release.md` and `docs/ops/m3a2-bootstrap-release.md`.

## Producing the shards

```bash
# Live ingest (requires network). For m3a.3 the two offers come from:
#   AmazonDynamoDB  -> dynamodb (db.nosql)
#   AmazonCloudFront -> cloudfront (network.cdn)
# For the bootstrap, fetch each offer's current index.json:
#   https://pricing.us-east-1.amazonaws.com/offers/v1.0/aws/AmazonDynamoDB/current/index.json
#   https://pricing.us-east-1.amazonaws.com/offers/v1.0/aws/AmazonCloudFront/current/index.json

make aws-dynamodb-shard   SHARD_SRC=/path/to/dynamodb/offer.json
make aws-cloudfront-shard SHARD_SRC=/path/to/cloudfront/offer.json

for shard in aws-dynamodb aws-cloudfront; do
  zstd -19 dist/pipeline/$shard.db -o dist/pipeline/$shard.db.zst
  sha256sum dist/pipeline/$shard.db.zst > dist/pipeline/$shard.db.zst.sha256
done
```

## Uploading

Create two GitHub releases with tags `data-bootstrap-aws-dynamodb` and
`data-bootstrap-aws-cloudfront` and attach the `.zst` + `.sha256` assets
for each. The URLs in `internal/updater/updater.go`'s `DefaultSources`
map resolve to these assets.

## Verifying end-to-end on a fresh machine

```bash
export SKU_DATA_DIR=$(mktemp -d)
for shard in aws-dynamodb aws-cloudfront; do sku update $shard; done

sku aws dynamodb   price --table-class standard --region us-east-1
sku aws cloudfront price --region eu-west-1
```
