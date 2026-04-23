# AWS pricing-feed coverage

_Generated 2026-04-22_

Raw SKU counts per bucket, and which `sku` shard (if any) ingests them.
Generated weekly by `make profile` against live feeds.

| Bucket | SKUs | Covered by | Coverage | Attrs fingerprint | Sample SKU ids |
| --- | ---: | --- | ---: | --- | --- |
| Compute Instance | 24 | aws_ec2 | 100% | capacitystatus, instanceType, memory, networkPerformance, operatingSystem, physicalProcessor, +5 more | `SKU-M5LARGE-EUWEST1-LIN-SHR`, `SKU-M5LARGE-EUWEST1-LIN-TEN`, `SKU-M5LARGE-EUWEST1-WIN-SHR` |
| API Request | 20 | aws_s3 | 100% | group, groupDescription, regionCode, servicecode, usagetype, volumeType | `SKU-S3-STD-USE1-REQ-PUT`, `SKU-S3-STD-USE1-REQ-GET`, `SKU-S3-STD-EUW1-REQ-PUT` |
| Database Instance | 16 | aws_rds | 100% | databaseEngine, deploymentOption, instanceType, licenseModel, memory, regionCode, +3 more | `SKU-DBM5LARGE-EUWEST1-MY-MAZ`, `SKU-DBM5LARGE-EUWEST1-MY-SAZ`, `SKU-DBM5LARGE-EUWEST1-PO-MAZ` |
| Storage | 10 | aws_s3 | 100% | regionCode, servicecode, storageClass, usagetype, volumeType | `SKU-S3-STD-USE1-STORAGE`, `SKU-S3-STD-EUW1-STORAGE`, `SKU-S3-SIA-USE1-STORAGE` |
| Amazon DynamoDB PayPerRequest Throughput | 8 | aws_dynamodb | 100% | group, regionCode, servicecode, storageClass, usagetype | `SKU-DDB-STD-USE1-READ`, `SKU-DDB-STD-USE1-WRITE`, `SKU-DDB-IA-USE1-READ` |
| Serverless | 8 | aws_lambda | 100% | archSupport, group, groupDescription, regionCode, servicecode, usagetype | `SKU-LAMBDA-X86-USE1-REQ`, `SKU-LAMBDA-X86-USE1-DUR`, `SKU-LAMBDA-X86-EUW1-REQ` |
| Database Storage | 4 | **UNCOVERED** | — | group, regionCode, servicecode, storageClass, usagetype | `SKU-DDB-STD-USE1-STORAGE`, `SKU-DDB-IA-USE1-STORAGE`, `SKU-DDB-STD-EUW1-STORAGE` |
| Data Transfer | 3 | aws_cloudfront | 100% | fromLocation, location, locationType, servicecode, transferType, usagetype | `SKU-CF-USMEXCA-DTO`, `SKU-CF-EU-DTO`, `SKU-CF-AP-DTO` |

