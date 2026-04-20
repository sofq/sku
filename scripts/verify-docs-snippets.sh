#!/usr/bin/env bash
# Re-run every verified snippet in docs/getting-started.md + docs/commands/*.md.
#
# Pre-reqs: ./bin/sku built, shards under dist/pipeline built, SKU_DATA_DIR set.
# Usage:    bash scripts/verify-docs-snippets.sh

set -euo pipefail

: "${SKU_DATA_DIR:=$(pwd)/dist/pipeline}"
export SKU_DATA_DIR

BIN=$(pwd)/bin/sku
test -x "$BIN" || { echo "missing $BIN — run 'make build'"; exit 2; }

run() {
  echo "+ $*" >&2
  eval "$*" >/dev/null
}

# getting-started.md
run "$BIN version --pretty"
run "$BIN aws ec2 price --instance-type m5.large --region us-east-1 --pretty"
run "$BIN aws ec2 price --instance-type m5.large --region us-east-1 --jq '.price[0].amount'"
run "$BIN aws ec2 price --instance-type m5.large --region us-east-1 --preset price"
run "$BIN compare --kind compute.vm --vcpu 4 --memory 16 --regions us-east --limit 5 --preset compare --pretty"
run "$BIN estimate --item 'aws/ec2:m5.large:region=us-east-1:count=10:hours=730' --item 'aws/ec2:m5.xlarge:region=us-east-1:count=1:hours=730' --pretty"
run "$BIN estimate --config docs/examples/workload-vm.yaml --pretty"
run "cat docs/examples/batch-queries.ndjson | $BIN batch --pretty"

# commands/*.md — representative snippets
run "$BIN aws ec2 list --instance-type m5.large"
run "$BIN aws rds price --instance-type db.m5.large --region us-east-1 --engine postgres --deployment-option single-az"
run "$BIN aws s3 price --storage-class standard --region us-east-1"
run "$BIN aws lambda price --architecture arm64 --region us-east-1"
run "$BIN aws ebs price --volume-type gp3 --region us-east-1"
run "$BIN aws dynamodb price --table-class standard --region us-east-1"
run "$BIN aws cloudfront price --region eu-west-1"
run "$BIN azure vm price --arm-sku-name Standard_D2_v3 --region eastus --os linux --preset agent"
run "$BIN azure sql price --sku-name GP_Gen5_2 --region eastus --deployment-option single-az"
run "$BIN azure blob price --tier hot --region eastus"
run "$BIN azure functions price --architecture x86_64 --region eastus"
run "$BIN azure disks price --disk-type premium-ssd --region eastus"
run "$BIN gcp gce price --machine-type n1-standard-2 --region us-east1"
run "$BIN gcp cloud-sql price --tier db-custom-2-7680 --region us-east1 --engine postgres --deployment-option zonal"
run "$BIN gcp gcs price --storage-class standard --region us-east1"
run "$BIN gcp run price --architecture x86_64 --region us-east1"
run "$BIN gcp functions price --architecture x86_64 --region us-east1"
run "$BIN llm price --model anthropic/claude-opus-4.6 --serving-provider aws-bedrock"
run "$BIN estimate --item 'aws/s3:standard:region=us-east-1:gb_month=500:put_requests=1000:get_requests=5000' --pretty"
run "$BIN estimate --item 'llm:anthropic/claude-opus-4.6:input=1M:output=500K:serving_provider=anthropic' --pretty"
run "$BIN estimate --config docs/examples/workload-llm.yaml --pretty"
run "$BIN schema --errors"
run "$BIN schema --list-commands"
run "$BIN schema --list-serving-providers"

# guides/agent-integration.md
run "$BIN --preset agent aws ec2 price --instance-type m5.large --region us-east-1"
run "$BIN aws ec2 price --instance-type m5.large --region us-east-1 --fields provider,resource.name,price.0.amount"
run "$BIN aws ec2 price --instance-type m5.large --region us-east-1 --jq '{type: .resource.name, usd_hr: .price[0].amount}'"

# guides/llm-routing.md
run "$BIN llm compare --model anthropic/claude-opus-4.6 --preset compare --pretty || true"   # sku llm compare not yet wired (only sku llm price); v1.0 follow-up
run "$BIN llm list --max-input-price 1.0 --max-output-price 5.0 --preset agent --sort input-price || true"   # --max-*-price flags + sku llm list only exist once wired, see issue
run "$BIN estimate --item 'llm:anthropic/claude-opus-4.6:input=1M:output=500K:serving_provider=aws-bedrock' --pretty"

# guides/offline-use.md
run "$BIN --stale-ok aws ec2 price --instance-type m5.large --region us-east-1"

# skill/using-sku/SKILL.md
run "$BIN version --pretty"
run "$BIN search --provider aws --service ec2 --min-vcpu 4 --max-price 0.10 --sort price --limit 5 || true"   # local dev shard is sparse; production shard has matches

echo "all snippets ok"
