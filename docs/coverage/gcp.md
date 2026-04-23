# GCP pricing-feed coverage

_Generated 2026-04-22_

Raw SKU counts per bucket, and which `sku` shard (if any) ingests them.
Generated weekly by `make profile` against live feeds.

| Bucket | SKUs | Covered by | Coverage | Attrs fingerprint | Sample SKU ids |
| --- | ---: | --- | ---: | --- | --- |
| Cloud SQL / SQLGen2InstancesN1Standard | 17 | gcp_cloud_sql | 100% | category, category.resourceFamily, category.resourceGroup, category.serviceDisplayName, category.usageType, description, +3 more | `SQL-PG-C2-USE1-ZONAL`, `SQL-PG-C2-USE1-REGIONAL`, `SQL-PG-C2-EUW1-ZONAL` |
| Cloud Run / Compute | 8 | gcp_run | 100% | category, category.resourceFamily, category.resourceGroup, category.serviceDisplayName, category.usageType, description, +3 more | `RUN-GLOBAL-REQ`, `RUN-USEAST1-CPU-EUR`, `RUN-USEAST1-CPU-CUD` |
| Cloud Run Functions / Compute | 7 | gcp_functions | 100% | category, category.resourceFamily, category.resourceGroup, category.serviceDisplayName, category.usageType, description, +3 more | `FN-USEAST1-MEM`, `FN-USEAST1-CPU-EUR`, `FN-USEAST1-CPU` |
| Compute Engine / N1Standard | 7 | gcp_gce | 100% | category, category.resourceFamily, category.resourceGroup, category.serviceDisplayName, category.usageType, description, +3 more | `SKU-N1-RAM-USE1`, `SKU-N1-RAM-EUW1`, `SKU-WIN-LIC-USE1` |
| Cloud Storage / RegionalStorage | 3 | gcp_gcs | 100% | category, category.resourceFamily, category.resourceGroup, category.serviceDisplayName, category.usageType, description, +3 more | `STD-USEAST1-STORAGE`, `STD-USEAST1-STORAGE-EUR`, `STD-USEAST1-STORAGE-CUD` |
| Cloud Storage / ArchiveOps | 2 | **UNCOVERED** | — | category, category.resourceFamily, category.resourceGroup, category.serviceDisplayName, category.usageType, description, +3 more | `AR-GLOBAL-READOPS`, `AR-GLOBAL-WRITEOPS` |
| Cloud Storage / ColdlineOps | 2 | **UNCOVERED** | — | category, category.resourceFamily, category.resourceGroup, category.serviceDisplayName, category.usageType, description, +3 more | `CL-GLOBAL-WRITEOPS`, `CL-GLOBAL-READOPS` |
| Cloud Storage / NearlineOps | 2 | **UNCOVERED** | — | category, category.resourceFamily, category.resourceGroup, category.serviceDisplayName, category.usageType, description, +3 more | `NL-GLOBAL-READOPS`, `NL-GLOBAL-WRITEOPS` |
| Cloud Storage / RegionalOps | 2 | **UNCOVERED** | — | category, category.resourceFamily, category.resourceGroup, category.serviceDisplayName, category.usageType, description, +3 more | `STD-GLOBAL-WRITEOPS`, `STD-GLOBAL-READOPS` |
| Compute Engine / C2Standard | 2 | **UNCOVERED** | — | category, category.resourceFamily, category.resourceGroup, category.serviceDisplayName, category.usageType, description, +3 more | `C2CORE-USEAST1`, `C2RAM-USEAST1` |
| Compute Engine / N2Standard | 2 | gcp_gce | 100% | category, category.resourceFamily, category.resourceGroup, category.serviceDisplayName, category.usageType, description, +3 more | `N2RAM-USEAST1`, `N2CORE-USEAST1` |
| Cloud Run Functions / Functions | 1 | **UNCOVERED** | — | category, category.resourceFamily, category.resourceGroup, category.serviceDisplayName, category.usageType, description, +3 more | `FN-USEAST1-CPU-GEN1` |
| Cloud SQL / SSD | 1 | **UNCOVERED** | — | category, category.resourceFamily, category.resourceGroup, category.serviceDisplayName, category.usageType, description, +3 more | `SQL-STORAGE-SSD-USE1` |
| Cloud Storage / ArchiveStorage | 1 | **UNCOVERED** | — | category, category.resourceFamily, category.resourceGroup, category.serviceDisplayName, category.usageType, description, +3 more | `AR-EUWEST1-STORAGE` |
| Cloud Storage / ColdlineStorage | 1 | **UNCOVERED** | — | category, category.resourceFamily, category.resourceGroup, category.serviceDisplayName, category.usageType, description, +3 more | `CL-EUWEST1-STORAGE` |
| Cloud Storage / MultiRegionalStorage | 1 | **UNCOVERED** | — | category, category.resourceFamily, category.resourceGroup, category.serviceDisplayName, category.usageType, description, +3 more | `STD-MULTIUS-STORAGE` |
| Cloud Storage / NearlineStorage | 1 | **UNCOVERED** | — | category, category.resourceFamily, category.resourceGroup, category.serviceDisplayName, category.usageType, description, +3 more | `NL-USEAST1-STORAGE` |
| Compute Engine / CPU | 1 | **UNCOVERED** | — | category, category.resourceFamily, category.resourceGroup, category.serviceDisplayName, category.usageType, description, +3 more | `SKU-E2-CPU-USE1` |
| Compute Engine / RAM | 1 | **UNCOVERED** | — | category, category.resourceFamily, category.resourceGroup, category.serviceDisplayName, category.usageType, description, +3 more | `SKU-E2-RAM-USE1` |

