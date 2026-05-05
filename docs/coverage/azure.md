# Azure pricing-feed coverage

_Generated 2026-05-04_

Raw SKU counts per bucket, and which `sku` shard (if any) ingests them.
Generated weekly by `make profile` against live feeds.

| Bucket | SKUs | Covered by | Coverage | Attrs fingerprint | Sample SKU ids |
| --- | ---: | --- | ---: | --- | --- |
| Virtual Machines / Virtual Machines | 702 | azure_vm | 100% | armRegionName, armSkuName, currencyCode, effectiveStartDate, isPrimaryMeterRegion, location, +16 more | `Standard_D14`, `Standard_D4as_v7`, `Standard_D2a_v4` |
| Storage / Storage | 57 | azure_blob | 100% | armRegionName, armSkuName, currencyCode, effectiveStartDate, isPrimaryMeterRegion, location, +16 more | ``, `Premium_SSD_Managed_Disks_P80`, `Cold` |
| Foundry Models / Foundry Models | 24 | **UNCOVERED** | — | armRegionName, armSkuName, currencyCode, effectiveStartDate, isPrimaryMeterRegion, location, +16 more | `54 pro Batch inp Dz`, `Grok 7 Inp glbl`, `K2 Thinking Inp DZ` |
| SQL Database / SQL Database | 23 | azure_sql | 100% | armRegionName, armSkuName, currencyCode, effectiveStartDate, isPrimaryMeterRegion, location, +16 more | `36 vCore`, `SQLDB_GP_Compute_Gen4_1`, `` |
| Foundry Tools / Foundry Tools | 15 | **UNCOVERED** | — | armRegionName, armSkuName, currencyCode, effectiveStartDate, isPrimaryMeterRegion, location, +16 more | `Commitment Tier CLU Azure 1M`, `Voice Live API Lite - LLM Text Cached`, `Standard` |
| Azure Database for PostgreSQL / Azure Database for PostgreSQL | 14 | azure_postgres | 100% | armRegionName, armSkuName, currencyCode, effectiveStartDate, isPrimaryMeterRegion, location, +16 more | `Standard_E64ds_v5`, `Standard_D16ads_v5`, `` |
| Azure Monitor / Azure Monitor | 13 | **UNCOVERED** | — | armRegionName, armSkuName, currencyCode, effectiveStartDate, isPrimaryMeterRegion, location, +16 more | `` |
| Azure Database for MySQL / Azure Database for MySQL | 12 | azure_mysql | 100% | armRegionName, armSkuName, currencyCode, effectiveStartDate, isPrimaryMeterRegion, location, +16 more | `AzureDB_MySQL_Flexible_Server_General_Purpose_Compute_Ddsv5_Series`, ``, `AzureDB_MySQL_Flexible_Server_Memory_Optimized_Edsv5Series_Compute_4vCore` |
| Cloud Services / Cloud Services | 10 | **UNCOVERED** | — | armRegionName, armSkuName, currencyCode, effectiveStartDate, isPrimaryMeterRegion, location, +16 more | `Standard_D2_v2`, `Standard_E64i_v3`, `` |
| Redis Cache / Redis Cache | 10 | **UNCOVERED** | — | armRegionName, armSkuName, currencyCode, effectiveStartDate, isPrimaryMeterRegion, location, +16 more | `Azure_Redis_Cache_Enterprise_Flash_F1500`, `Azure_Redis_Cache_Premium_P5_Cache`, `Azure_Redis_Cache_Standard_C3_Cache` |
| Azure App Service / Azure App Service | 9 | azure_functions | 100% | armRegionName, armSkuName, currencyCode, effectiveStartDate, isPrimaryMeterRegion, location, +16 more | ``, `Azure_App_Service_Premium_v3_Plan_Linux_P3mv3`, `Azure_App_Service_Premium_v4_Plan_Linux_P5mv4` |
| Azure Cosmos DB / Azure Cosmos DB | 7 | **UNCOVERED** | — | armRegionName, armSkuName, currencyCode, effectiveStartDate, isPrimaryMeterRegion, location, +16 more | `Azure Cosmos DB Materialized Views Builder D4s`, `Azure_Cosmos_DB_for_MongoDB_Worker_Node_32vCore`, `2 vCore - burstable` |
| Azure Front Door Service / Azure Front Door Service | 7 | **UNCOVERED** | — | armRegionName, armSkuName, currencyCode, effectiveStartDate, isPrimaryMeterRegion, location, +16 more | `` |
| Microsoft Fabric / Microsoft Fabric | 7 | **UNCOVERED** | — | armRegionName, armSkuName, currencyCode, effectiveStartDate, isPrimaryMeterRegion, location, +16 more | `Fabric Meter 101`, `Graph data science`, `RTI Event Listener and Alert` |
| SQL Managed Instance / SQL Managed Instance | 7 | **UNCOVERED** | — | armRegionName, armSkuName, currencyCode, effectiveStartDate, isPrimaryMeterRegion, location, +16 more | `SQLMI_BC_Compute_Gen5_32`, `80 vCore`, `16 vCore` |
| Azure Synapse Analytics / Azure Synapse Analytics | 6 | **UNCOVERED** | — | armRegionName, armSkuName, currencyCode, effectiveStartDate, isPrimaryMeterRegion, location, +16 more | ``, `SQL_DW100c` |
| Network Watcher / Network Watcher | 5 | **UNCOVERED** | — | armRegionName, armSkuName, currencyCode, effectiveStartDate, isPrimaryMeterRegion, location, +16 more | `` |
| Quantum Computing / Quantum Computing | 5 | **UNCOVERED** | — | armRegionName, armSkuName, currencyCode, effectiveStartDate, isPrimaryMeterRegion, location, +16 more | `` |
| Data Lake Store / Data Lake Store | 4 | **UNCOVERED** | — | armRegionName, armSkuName, currencyCode, effectiveStartDate, isPrimaryMeterRegion, location, +16 more | `` |
| Virtual Machines Licenses / Virtual Machines Licenses | 4 | **UNCOVERED** | — | armRegionName, armSkuName, currencyCode, effectiveStartDate, isPrimaryMeterRegion, location, +16 more | `192-vCPU VM`, `Tracking On Premises`, `7 vCPU VM` |
| Azure NetApp Files / Azure NetApp Files | 3 | **UNCOVERED** | — | armRegionName, armSkuName, currencyCode, effectiveStartDate, isPrimaryMeterRegion, location, +16 more | `Flexible Service Level`, `NetApp_Ultra_1 PiB` |
| ExpressRoute / ExpressRoute | 3 | **UNCOVERED** | — | armRegionName, armSkuName, currencyCode, effectiveStartDate, isPrimaryMeterRegion, location, +16 more | `` |
| HDInsight / HDInsight | 3 | **UNCOVERED** | — | armRegionName, armSkuName, currencyCode, effectiveStartDate, isPrimaryMeterRegion, location, +16 more | `Standard_E96a v4`, `E96d v5` |
| Notification Hubs / Notification Hubs | 3 | **UNCOVERED** | — | armRegionName, armSkuName, currencyCode, effectiveStartDate, isPrimaryMeterRegion, location, +16 more | `` |
| Phone Numbers / Phone Numbers | 3 | **UNCOVERED** | — | armRegionName, armSkuName, currencyCode, effectiveStartDate, isPrimaryMeterRegion, location, +16 more | `CN`, `AR`, `DE` |
| Application Gateway / Application Gateway | 2 | **UNCOVERED** | — | armRegionName, armSkuName, currencyCode, effectiveStartDate, isPrimaryMeterRegion, location, +16 more | `` |
| Azure Analysis Services / Azure Analysis Services | 2 | **UNCOVERED** | — | armRegionName, armSkuName, currencyCode, effectiveStartDate, isPrimaryMeterRegion, location, +16 more | `` |
| Azure Data Factory v2 / Azure Data Factory v2 | 2 | **UNCOVERED** | — | armRegionName, armSkuName, currencyCode, effectiveStartDate, isPrimaryMeterRegion, location, +16 more | `` |
| Azure Kubernetes Service / Azure Kubernetes Service | 2 | **UNCOVERED** | — | armRegionName, armSkuName, currencyCode, effectiveStartDate, isPrimaryMeterRegion, location, +16 more | `Automatic` |
| Azure Managed Instance for Apache Cassandra / Azure Managed Instance for Apache Cassandra | 2 | **UNCOVERED** | — | armRegionName, armSkuName, currencyCode, effectiveStartDate, isPrimaryMeterRegion, location, +16 more | `Apache_Cassandra_Standard_L8as_v3`, `Apache_Cassandra_Standard_E4s_v4` |
| Backup / Backup | 2 | **UNCOVERED** | — | armRegionName, armSkuName, currencyCode, effectiveStartDate, isPrimaryMeterRegion, location, +16 more | `ADLS Gen2 Vaulted` |
| Microsoft Defender for Cloud / Microsoft Defender for Cloud | 2 | **UNCOVERED** | — | armRegionName, armSkuName, currencyCode, effectiveStartDate, isPrimaryMeterRegion, location, +16 more | `` |
| Virtual WAN / Virtual WAN | 2 | **UNCOVERED** | — | armRegionName, armSkuName, currencyCode, effectiveStartDate, isPrimaryMeterRegion, location, +16 more | `` |
| App Configuration / App Configuration | 1 | **UNCOVERED** | — | armRegionName, armSkuName, currencyCode, effectiveStartDate, isPrimaryMeterRegion, location, +16 more | `` |
| Application Insights / Application Insights | 1 | **UNCOVERED** | — | armRegionName, armSkuName, currencyCode, effectiveStartDate, isPrimaryMeterRegion, location, +16 more | `` |
| Automation / Automation | 1 | **UNCOVERED** | — | armRegionName, armSkuName, currencyCode, effectiveStartDate, isPrimaryMeterRegion, location, +16 more | `` |
| Azure API for FHIR / Azure API for FHIR | 1 | **UNCOVERED** | — | armRegionName, armSkuName, currencyCode, effectiveStartDate, isPrimaryMeterRegion, location, +16 more | `` |
| Azure Blockchain / Azure Blockchain | 1 | **UNCOVERED** | — | armRegionName, armSkuName, currencyCode, effectiveStartDate, isPrimaryMeterRegion, location, +16 more | `` |
| Azure Cognitive Search / Azure Cognitive Search | 1 | **UNCOVERED** | — | armRegionName, armSkuName, currencyCode, effectiveStartDate, isPrimaryMeterRegion, location, +16 more | `Basic CC` |
| Azure Container Apps / Azure Container Apps | 1 | **UNCOVERED** | — | armRegionName, armSkuName, currencyCode, effectiveStartDate, isPrimaryMeterRegion, location, +16 more | `` |
| Azure Data Share / Azure Data Share | 1 | **UNCOVERED** | — | armRegionName, armSkuName, currencyCode, effectiveStartDate, isPrimaryMeterRegion, location, +16 more | `` |
| Azure Databricks / Azure Databricks | 1 | **UNCOVERED** | — | armRegionName, armSkuName, currencyCode, effectiveStartDate, isPrimaryMeterRegion, location, +16 more | `` |
| Azure Firewall / Azure Firewall | 1 | **UNCOVERED** | — | armRegionName, armSkuName, currencyCode, effectiveStartDate, isPrimaryMeterRegion, location, +16 more | `` |
| Azure Grafana Service / Azure Grafana Service | 1 | **UNCOVERED** | — | armRegionName, armSkuName, currencyCode, effectiveStartDate, isPrimaryMeterRegion, location, +16 more | `` |
| Azure Machine Learning / Azure Machine Learning | 1 | **UNCOVERED** | — | armRegionName, armSkuName, currencyCode, effectiveStartDate, isPrimaryMeterRegion, location, +16 more | `Llama-4-Scout-17B-16E-In` |
| Azure Maps / Azure Maps | 1 | **UNCOVERED** | — | armRegionName, armSkuName, currencyCode, effectiveStartDate, isPrimaryMeterRegion, location, +16 more | `` |
| Azure Purview / Azure Purview | 1 | **UNCOVERED** | — | armRegionName, armSkuName, currencyCode, effectiveStartDate, isPrimaryMeterRegion, location, +16 more | `` |
| Azure Spring Cloud / Azure Spring Cloud | 1 | **UNCOVERED** | — | armRegionName, armSkuName, currencyCode, effectiveStartDate, isPrimaryMeterRegion, location, +16 more | `` |
| Container Instances / Container Instances | 1 | **UNCOVERED** | — | armRegionName, armSkuName, currencyCode, effectiveStartDate, isPrimaryMeterRegion, location, +16 more | `` |
| Dynamics 365 for Customer Insights / Dynamics 365 for Customer Insights | 1 | **UNCOVERED** | — | armRegionName, armSkuName, currencyCode, effectiveStartDate, isPrimaryMeterRegion, location, +16 more | `` |
| Event Grid / Event Grid | 1 | **UNCOVERED** | — | armRegionName, armSkuName, currencyCode, effectiveStartDate, isPrimaryMeterRegion, location, +16 more | `` |
| Event Hubs / Event Hubs | 1 | **UNCOVERED** | — | armRegionName, armSkuName, currencyCode, effectiveStartDate, isPrimaryMeterRegion, location, +16 more | `` |
| Log Analytics / Log Analytics | 1 | **UNCOVERED** | — | armRegionName, armSkuName, currencyCode, effectiveStartDate, isPrimaryMeterRegion, location, +16 more | `` |
| SMS / SMS | 1 | **UNCOVERED** | — | armRegionName, armSkuName, currencyCode, effectiveStartDate, isPrimaryMeterRegion, location, +16 more | `CA` |
| SQL Server Stretch Database / SQL Server Stretch Database | 1 | **UNCOVERED** | — | armRegionName, armSkuName, currencyCode, effectiveStartDate, isPrimaryMeterRegion, location, +16 more | `` |
| Sentinel / Sentinel | 1 | **UNCOVERED** | — | armRegionName, armSkuName, currencyCode, effectiveStartDate, isPrimaryMeterRegion, location, +16 more | `Pay-as-you-go` |
| Service Bus / Service Bus | 1 | **UNCOVERED** | — | armRegionName, armSkuName, currencyCode, effectiveStartDate, isPrimaryMeterRegion, location, +16 more | `` |
| Syntex / Syntex | 1 | **UNCOVERED** | — | armRegionName, armSkuName, currencyCode, effectiveStartDate, isPrimaryMeterRegion, location, +16 more | `4K Video` |
| Time Series Insights / Time Series Insights | 1 | **UNCOVERED** | — | armRegionName, armSkuName, currencyCode, effectiveStartDate, isPrimaryMeterRegion, location, +16 more | `` |
| VPN Gateway / VPN Gateway | 1 | **UNCOVERED** | — | armRegionName, armSkuName, currencyCode, effectiveStartDate, isPrimaryMeterRegion, location, +16 more | `` |
| Voice / Voice | 1 | **UNCOVERED** | — | armRegionName, armSkuName, currencyCode, effectiveStartDate, isPrimaryMeterRegion, location, +16 more | `` |

