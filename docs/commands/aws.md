# `sku aws`

AWS services are registered statically. Every leaf supports `price` (point lookup) and `list` (enumeration). Install the matching shard first: `sku update aws-<service>`.

| Leaf | Shard | Key flags (`price`) |
|---|---|---|
| `ec2` | `aws-ec2` | `--instance-type`, `--region`, `--os` (default `linux`), `--tenancy` |
| `rds` | `aws-rds` | `--instance-type`, `--region`, `--engine`, `--deployment-option` |
| `s3` | `aws-s3` | `--storage-class`, `--region` |
| `lambda` | `aws-lambda` | `--architecture` (`x86_64`,`arm64`), `--region` |
| `ebs` | `aws-ebs` | `--volume-type`, `--region` |
| `dynamodb` | `aws-dynamodb` | `--table-class`, `--region` |
| `cloudfront` | `aws-cloudfront` | `--region` |

`list` takes the same filters as `price` minus `--region` (which becomes optional).

## Examples

```bash
sku aws ec2 price        --instance-type m5.large --region us-east-1 --preset agent
sku aws ec2 list         --instance-type m5.large
sku aws rds price        --instance-type db.m5.large --region us-east-1 \
                          --engine postgres --deployment-option single-az
sku aws s3 price         --storage-class standard --region us-east-1
sku aws lambda price     --architecture arm64 --region us-east-1
sku aws ebs price        --volume-type gp3 --region us-east-1
sku aws dynamodb price   --table-class standard --region us-east-1
sku aws cloudfront price --region eu-west-1
```
