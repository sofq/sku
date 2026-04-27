# AWS pricing-feed coverage

_Generated 2026-04-27_

Raw SKU counts per bucket, and which `sku` shard (if any) ingests them.
Generated weekly by `make profile` against live feeds.

| Bucket | SKUs | Covered by | Coverage | Attrs fingerprint | Sample SKU ids |
| --- | ---: | --- | ---: | --- | --- |
| Compute Instance | 92,785 | aws_ec2 | 100% | availabilityzone, capacitystatus, classicnetworkingsupport, clockSpeed, currentGeneration, dedicatedEbsThroughput, +33 more | `QUMEF4UK3NPT4MN3`, `DBCQPZ6Z853WRE98`, `PSKQUF3DN4PQUY5X` |
| Compute Instance (bare metal) | 11,038 | **UNCOVERED** | — | availabilityzone, capacitystatus, classicnetworkingsupport, clockSpeed, currentGeneration, dedicatedEbsThroughput, +32 more | `TNWBCS5ACPZ2KN5K`, `PKKXPZKDGX8TTETG`, `9ZNF8HF7HCD62DUE` |
| Database Instance | 4,926 | aws_rds | 100% | clockSpeed, currentGeneration, databaseEdition, databaseEngine, dedicatedEbsThroughput, deploymentModel, +24 more | `AS89MVW9EQ98NPCS`, `9QH3PUGXCYKNCYPB`, `9TUPTR977JMXWRWK` |
| Serverless | 511 | aws_lambda | 100% | group, groupDescription, lambdaManagedInstanceType, lambdaManagedInstances-RequestType, location, locationType, +5 more | `8MV6QCXGTMB3SQSA`, `8VMHUU6HBE25BH4E`, `3DQDA3PNUY939FRS` |
| (no productFamily) | 273 | **UNCOVERED** | — | feeCode, feeDescription, fromLocation, fromLocationType, fromRegionCode, group, +14 more | `SG7CKZDA3VUU22WX`, `Z8X7XRC6YMB3B8CX`, `UMHS42HQRXCX35Z3` |
| Database Storage | 236 | **UNCOVERED** | — | databaseEdition, databaseEngine, deploymentModel, deploymentOption, engineCode, licenseModel, +12 more | `5CY9R54BDJWYJ22J`, `TYAUEGUCCZJVV7J5`, `657RUPPEV3YQSGDA` |
| Dedicated Host | 222 | **UNCOVERED** | — | availabilityzone, capacitystatus, classicnetworkingsupport, clockSpeed, currentGeneration, ecu, +42 more | `US76PBRY3DCZYN6N`, `GUSF94EQ4ATETPFP`, `YK93RGC4R47ACWRA` |
| Provisioned IOPS | 151 | **UNCOVERED** | — | databaseEdition, databaseEngine, deploymentModel, deploymentOption, engineCode, group, +11 more | `HZNNAFUGB668T2AB`, `CSJDFDAZPE4BE5BD`, `NW7WAC7H9ZCHD68P` |
| Request | 67 | **UNCOVERED** | — | group, groupDescription, location, locationType, operation, regionCode, +5 more | `F24VFUTRVY389U5R`, `NPFD6SEH9MXS8M4J`, `8TR9SADHNBYGFMTW` |
| Performance Insights | 52 | **UNCOVERED** | — | databaseEngine, engineCode, group, groupDescription, instanceTypeFamily, location, +6 more | `NY662M2P6WNTPGY2`, `27HUFMH8U28DFE4X`, `KZ4WTWHJMFEQZJJR` |
| Provisioned Throughput | 52 | **UNCOVERED** | — | databaseEdition, databaseEngine, deploymentModel, deploymentOption, engineCode, group, +10 more | `ED9TKE4RUFPF5G5F`, `XY9WTPTPG3MUCTUV`, `7TQTD99Y2SEGAXHP` |
| Data Transfer | 47 | aws_cloudfront | 100% | fromLocation, fromLocationType, fromRegionCode, operation, servicecode, servicename, +5 more | `S89KGB9FHD5TVGSR`, `GV2WFGX37Q9PDSHF`, `U8P37H38743VG7RS` |
| Storage Snapshot | 35 | **UNCOVERED** | — | databaseEdition, databaseEngine, deploymentModel, deploymentOption, engineCode, engineMediaType, +9 more | `FJBXDXSNQ7XGXNKR`, `B2XDPTNXCRU552G7`, `EWNJUQ4BXM8XM9BT` |
| Data Transfer | 34 | **UNCOVERED** | — | fromLocation, fromLocationType, fromRegionCode, operation, servicecode, servicename, +5 more | `83K59X2AV5V696PA`, `WW3ZJQMWZXAVGFZV`, `6PZEWA636YBNU2AQ` |
| (no productFamily) | 30 | **UNCOVERED** | — | databaseEngine, engineCode, engineMajorVersion, extendedSupportPricingYear, group, groupDescription, +7 more | `RMEDM9YAEZ5RRW4J`, `6EQ8UVDGQRZNA7SC`, `28U5G3XN9865JMQT` |
| Serverless | 30 | **UNCOVERED** | — | group, groupDescription, location, locationType, operation, regionCode, +3 more | `RN2BPS8XT2GYJ4UF`, `78833EM5YM3WXMT5`, `SW89832HP7DYZ3CG` |
| API Request | 29 | aws_s3 | 100% | group, groupDescription, location, locationType, operation, regionCode, +3 more | `26TUJV6FNGAN44UY`, `GWU4HUHFNSRYMYNK`, `2C7JEQ54QWYYWGZ6` |
| CPU Credits | 24 | **UNCOVERED** | — | databaseEngine, engineCode, instanceFamily, location, locationType, operation, +4 more | `DWKMJS7PYD85JEZ3`, `2P89SZDJEU38HUPJ`, `VZZY8GGAD2F3FVCM` |
| Fee | 19 | **UNCOVERED** | — | feeCode, feeDescription, location, locationType, operation, regionCode, +3 more | `VBHP3NBUHWJHYBZA`, `KJTHJV8KK6PVBZKM`, `PWNMYRK4GT93REV7` |
| System Operation | 18 | **UNCOVERED** | — | databaseEngine, engineCode, group, groupDescription, limitlesspreview, location, +7 more | `5Z2UYT8WXMZGM6EY`, `TCS8QS9MYHQ97BU5`, `SHATDUXG5H8C5N35` |
| Storage | 16 | aws_s3 | 100% | availability, durability, location, locationType, operation, overhead, +6 more | `G76SDFUV63NSS6N2`, `2N3NKSP5AF8HYTS2`, `EXB3YJ6YV5CRH4JN` |
| System Operation | 13 | **UNCOVERED** | — | group, groupDescription, location, locationType, operation, provisioned, +7 more | `U76K6MD4GPND4GAE`, `EBJ5G6KM7HXQUUE9`, `7Q58NR58VQEASA4W` |
| RDSProxy | 8 | **UNCOVERED** | — | acu, databaseEngine, engineCode, location, locationType, operation, +5 more | `746KK4645QA9UP4N`, `AHVT2C6WP2BB5BXQ`, `WPQAFKY87PKPA7PX` |
| Storage | 7 | aws_ebs | 100% | location, locationType, maxIopsBurstPerformance, maxIopsvolume, maxThroughputvolume, maxVolumeSize, +8 more | `HY3BZPP2B6K8MSJF`, `JG3KUJMBRGHV3N8G`, `269VXUCZZ7E6JNXT` |
| Amazon DynamoDB PayPerRequest Throughput | 6 | aws_dynamodb | 100% | group, groupDescription, location, locationType, operation, regionCode, +3 more | `FGXVD96DKJMUASY3`, `6NJM3K66JDV3SGNV`, `MJVXP493BSJ5SKSQ` |
| CPU Credits | 6 | **UNCOVERED** | — | instance, instanceFamilyCategory, location, locationType, operatingSystem, operation, +4 more | `3XWHFAKQQFKM4JSY`, `ZA9GXD8U7NKKZHC8`, `DHXM9J4FT38HQAQ6` |
| NAT Gateway | 6 | **UNCOVERED** | — | group, groupDescription, location, locationType, operation, regionCode, +3 more | `ZCCQ4VCQFKQSGGHQ`, `M2YSHUBETB3JX4M4`, `KVHMMSJX774M3XRN` |
| Storage Snapshot | 6 | **UNCOVERED** | — | location, locationType, operation, regionCode, servicecode, servicename, +3 more | `GHTDRQFFXQZVJNXF`, `TQQM6CHBJQ7MKT24`, `PXAF4HC3TVCEH5XQ` |
| Data Transfer | 4 | **UNCOVERED** | — | fromLocation, fromLocationType, fromRegionCode, operation, servicecode, servicename, +5 more | `RYYTS5ZVVDHWE4WC`, `QP7TRQVACWFWF76K`, `RX66S3KERGV4YBS4` |
| Elastic Graphics | 4 | **UNCOVERED** | — | elasticGraphicsType, gpuMemory, location, locationType, operation, regionCode, +3 more | `J74QHQCMTJZ4DBQG`, `B8S8THNDW6NR86EM`, `42DW5QPNKSF6HTVW` |
| Fee | 4 | **UNCOVERED** | — | group, groupDescription, location, locationType, operation, regionCode, +3 more | `U3HVZF9WJEM7TE55`, `56WG9S5MJEKU8SJ5`, `QUEZ7XDZJJXURBU7` |
| Provisioned IOPS | 4 | **UNCOVERED** | — | group, groupDescription, location, locationType, operation, regionCode, +3 more | `4V475Q49DCKGXQZ2`, `R6PXMNYCEDGZ2EYN`, `P7CSCRKZN8P5MR8C` |
| ServerlessV2 | 4 | **UNCOVERED** | — | databaseEngine, engineCode, location, locationType, operation, regionCode, +4 more | `AVKW7ZJP8D72HNQC`, `KG668U3YXJQ6YH9Y`, `MK5Y97VKQVCZA44G` |
| Database Storage | 3 | **UNCOVERED** | — | location, locationType, operation, regionCode, servicecode, servicename, +2 more | `TFXQN9PEG62G7A67`, `F3E2EDSYC6ZNW7XP`, `DUA7FNC8NKEMZ7AA` |
| EBS direct API Requests | 3 | **UNCOVERED** | — | group, groupDescription, location, locationType, operation, regionCode, +4 more | `9PNNGFWQP7349GGM`, `QRY3U66NSBVRCCHZ`, `TUB3X35AQ4QARUZC` |
| (no productFamily) | 2 | **UNCOVERED** | — | group, groupDescription, location, locationType, operation, regionCode, +3 more | `9HGBWJU6RANR5U7Y`, `AKFBEQDKQJE2TU7K` |
| Amazon DynamoDB Export Data Size | 2 | **UNCOVERED** | — | location, locationType, operation, regionCode, servicecode, servicename, +2 more | `3ADXMU9RYGKRUA9W`, `3W5BGZNA3M3V96PD` |
| Aurora Global Database | 2 | **UNCOVERED** | — | databaseEngine, engineCode, group, groupDescription, location, locationType, +5 more | `UB3UB2DBZ2EPWCSU`, `WGQEPZSRBYVMTDM9` |
| DDB-Operation-ReplicatedWrite | 2 | **UNCOVERED** | — | group, groupDescription, location, locationType, operation, regionCode, +3 more | `QM7WV2JADZJDV45W`, `MNQBR5JKF47TW9TE` |
| Limitless | 2 | **UNCOVERED** | — | databaseEngine, engineCode, limitlesspreview, location, locationType, operation, +5 more | `DCHS7P8497H7BUPK`, `AHNRC7XWNY95GTJR` |
| Load Balancer | 2 | **UNCOVERED** | — | group, groupDescription, location, locationType, operation, regionCode, +3 more | `T3YPSF6NJ69JWWNM`, `4YYNPC2WZJJXCMPG` |
| Load Balancer-Application | 2 | **UNCOVERED** | — | group, groupDescription, location, locationType, operation, regionCode, +3 more | `BJMX7R52F3VNFNEH`, `U4ACUCANS9BM4R7H` |
| Load Balancer-Network | 2 | **UNCOVERED** | — | group, groupDescription, location, locationType, operation, regionCode, +3 more | `PQ44QRGZYQ87XPN2`, `6CNJHBFSUT74HD2K` |
| ProvisionedRateVolumeInitialization | 2 | **UNCOVERED** | — | location, locationType, operation, productType, regionCode, servicecode, +2 more | `8V6NA8YTE8MAQNES`, `CFVCBBJG3GTKMWBE` |
| Serverless | 2 | **UNCOVERED** | — | databaseEngine, engineCode, location, locationType, operation, regionCode, +3 more | `HK9832HSTD4XQSKN`, `4ZZ835TJ7A3HA6BU` |
| (no productFamily) | 1 | **UNCOVERED** | — | location, locationType, operation, productType, regionCode, servicecode, +2 more | `47S977VFHVMKFDRB` |
| API Request | 1 | **UNCOVERED** | — | group, groupDescription, location, locationType, operation, regionCode, +3 more | `C7EPEWE3GFXXDMHR` |
| Amazon DynamoDB Import Data Size | 1 | **UNCOVERED** | — | location, locationType, operation, regionCode, servicecode, servicename, +2 more | `V54YY5S4S3DECX7A` |
| Amazon DynamoDB On-Demand Backup Storage | 1 | **UNCOVERED** | — | location, locationType, operation, regionCode, servicecode, servicename, +2 more | `EDC5UVDZNFN2S777` |
| Amazon DynamoDB Restore Data Size | 1 | **UNCOVERED** | — | location, locationType, operation, regionCode, servicecode, servicename, +2 more | `Z79SYD7KXFPZZ9SF` |
| Fast Snapshot Restore | 1 | **UNCOVERED** | — | location, locationType, operation, productType, regionCode, servicecode, +2 more | `JMGBAZ4CB88RKXT5` |
| Fee | 1 | **UNCOVERED** | — | group, groupDescription, location, locationType, operation, regionCode, +3 more | `MCUV5B88GT577CFE` |
| Provisioned Throughput | 1 | **UNCOVERED** | — | group, groupDescription, location, locationType, operation, provisioned, +5 more | `SQUFRQX4K92S4SBB` |
| RealTime | 1 | **UNCOVERED** | — | group, groupDescription, operation, servicecode, servicename, usagetype | `JFNCBB6Y5CDJ9SKY` |

