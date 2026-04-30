# M-δ Upstream Shape Notes (2026-04-30)

Verification of live upstream pricing API shapes for the 14 new M-δ shards.
Fetch method: AWS Bulk Pricing API (no auth), Azure Retail Prices API (no auth),
GCP Cloud Billing Catalog API (ADC — authenticated locally).

---

## AWS

### aws_sqs

- **Service code**: `AWSQueueService` (NOT `AmazonSQS` — plan uses `AmazonSQS` in filter; must be corrected to use servicecode field `AWSQueueService`)
- **productFamily values seen**: `API Request` only (109 SKUs total)
- **Key attribute names**: `queueType`, `messageDeliveryFrequency`, `messageDeliveryOrder`, `group`, `groupDescription`, `location`, `locationType`, `regionCode`, `usagetype`
- **queueType values**: `"Standard"`, `"FIFO (first-in, first-out)"`
- **group values**: `SQS-APIRequest-Tier1` only — all SKUs in one group despite tiers in priceDimensions
- **units in priceDimensions**: `Requests`
- **Sample tier ranges (Standard, us-east-1)**:
  - `beginRange=0, endRange=100000000000` → $0.40/million
  - `beginRange=100000000000, endRange=200000000000` → $0.35/million
  - `beginRange=200000000000, endRange=Inf` → $0.32/million
- **Sample tier ranges (FIFO)**:
  - `beginRange=0, endRange=100000000000` → $0.50/million
  - `beginRange=100000000000, endRange=200000000000` → $0.48/million
  - `beginRange=200000000000, endRange=Inf` → $0.45/million
- **Surprises vs plan**:
  - The plan says `serviceCode = 'AmazonSQS'` but the correct offer code is `AWSQueueService`. The DuckDB filter must use `servicecode = 'AWSQueueService'`.
  - Three tiers per SKU (not two). Plan fixture calls for 6 SKUs total which is still correct (Standard+FIFO × 3 regions), but each SKU has 3 price tiers embedded in priceDimensions, not separate rows.
  - Tier boundaries are 100B and 200B requests, not what the plan anticipated — ingest will need to read priceDimensions tiers and emit multiple price rows per SKU.
- **matches plan**: mostly yes — productFamily filter `API Request` ✓, queueType attribute ✓ — but service code name needs correction

---

### aws_sns

- **productFamily values seen**: `API Request` (36 SKUs), `Message Delivery` (522 SKUs), `NO_FAMILY` (412 SKUs)
- **Key attribute names**: `endpointType`, `group`, `groupDescription`, `location`, `locationType`, `regionCode`, `usagetype`
- **endpointType values** (in Message Delivery family): `HTTP`, `Amazon SQS`, `AWS Lambda`, `SMS`, `SMTP`, `Amazon Kinesis Data Firehose`, `Apple Push Notification Service (APNS) - iOS`, `Apple Push Notification Service (APNS) - Mac OS`, `Apple Push Notification Service (APNS) Sandbox - iOS`, `Apple Push Notification Service (APNS) Sandbox - Mac OS`, `Apple Push Notification Service (APNS) VOIP`, `Apple Push Notification Service (APNS) VOIP Sandbox`, `Google Cloud Messaging for Android (GCM)`, `Baidu Cloud Push (Baidu)`, `Microsoft Push Notification Service for Windows Phone (MPNS)`, `Windows Push Notification Services (WNS)`, `Apple Passbook`, `Apple Passbook Sandbox`
- **API Request family**: no `endpointType` attribute — these are publish-request counts, not deliveries. Group: `SNS-Requests-Tier1`.
- **units in priceDimensions** (API Request): `Requests`; (Message Delivery): `Notifications`
- **Sample tier ranges (API Request, first 1M free)**:
  - `beginRange=0, endRange=1000000` → $0.00 (free tier)
  - `beginRange=1000000, endRange=Inf` → ~$0.595/million (region-dependent)
- **NO_FAMILY SKUs**: FIFO-specific features — `SNS-FIFO-Payload-MessageFiltering-Filtered-Out`, `SNS-FIFO-Storage`, `SNS-FIFO-Subscription-Messages-Payload`, `SNS-FIFO-Egress-SQS` groups; no priceDimensions in many
- **Surprises vs plan**:
  - Plan says filter `productFamily = 'API Request'` and emit rows for HTTP and SQS destinations only — but `API Request` family has NO `endpointType` attribute; it's the publish-side pricing (per request). Delivery-side (HTTP, SQS) pricing is in `Message Delivery` family. Plan ingest logic needs to decide: publish-request pricing (API Request family, per-publish count) or delivery pricing (Message Delivery family, per-delivery attempt by endpointType).
  - Recommend: for `messaging.topic` semantics, use `API Request` family (publish cost). Optionally add delivery cost via `Message Delivery` family filtered to `endpointType in ('HTTP', 'Amazon SQS')`. This is a design question for spec time.
  - No FIFO topics distinguished in API Request family (no `queueType` attribute unlike SQS).
- **matches plan**: partial — filter `productFamily = 'API Request'` yields publish-only pricing, not endpoint-typed delivery pricing; endpointType is not present in API Request rows

---

### aws_route53

- **productFamily values seen**: `DNS Query` (290 SKUs), `DNS Zone` (2 SKUs), `DNS Domain Names` (36 SKUs), `DNS Health Check` (4 SKUs), `Compute` (1 SKU), `NO_FAMILY` (23 SKUs)
- **Key attribute names**: `description`, `group`, `groupDescription`, `location`, `locationType`, `regionCode`, `routingTarget`, `routingType`, `resourceEndpoint`, `globalresolver-usagetypes`, `usagetype`
- **DNS Zone family** (2 SKUs): `HostedZone` (per-zone monthly), `Global-RRSets` (per-record-set monthly)
  - Zone tiers: `tier=0,end=25` → $0.50/zone; `tier=25,end=Inf` → $0.10/zone
  - No public/private zone distinction in the offer — single `HostedZone` usagetype covers all zones
- **DNS Query family usagetype suffixes**: `DNS-Queries` (standard resolver queries, regional), `DNS-FirewallQueries`, `Geo-Queries`, `Proximity-Queries`, `AWS-Cidr-Queries`, `AWS-Geo-Queries`, `AWS-LBR-Queries`, `ResolverNetworkInterface`, `ResolverNetworkInterface-SecurityEnabled`, `DNS-FirewallDomainName`, `DNS-ProfileAssociationCount`, `DNS-ProfileBasePackage`
- **Standard query tiers** (`DNS-Queries` usagetype, per region):
  - `beginRange=0, endRange=1000000000` → $0.40/million
  - `beginRange=1000000000, endRange=Inf` → $0.20/million
- **units**: `HostedZone` (for zones), `Queries` (for DNS queries), `Mo` (for RRSets), `hours` (for resolver NICs)
- **Surprises vs plan**:
  - Plan expects `zone-type=public|private` rows — **no such distinction exists in the offer**. There is only one `HostedZone` SKU (global). Public and private zones cost the same per AWS pricing page. The ingest should emit a single zone row (no zone-type fan-out), or emit two rows with the same price for compatibility with the compare kind interface.
  - Plan's `DNS Query` filter will pull in `ResolverNetworkInterface`, `DNS-FirewallQueries`, Geo/Proximity routing queries — must filter to `DNS-Queries` suffix only for standard query pricing.
  - Query tiers: plan expects `tier=0,tier_upper=1B` then `tier=1B,tier_upper=10B` then `tier=10B,tier_upper=""` (3 tiers) but actual offer has only 2 tiers (0–1B and 1B+). Tier spec needs adjustment.
  - Route 53 is a global service; region field will be `"global"` as planned ✓
- **matches plan**: partial — zone type distinction does not exist upstream; query tier count differs from plan (2 vs 3)

---

### aws_api_gateway

- **productFamily values seen**: `API Calls` (69 SKUs), `Amazon API Gateway Cache` (288 SKUs), `Amazon API Gateway Portals` (68 SKUs), `WebSocket` (67 SKUs)
- **Key attribute names**: `description`, `location`, `locationType`, `regionCode`, `usagetype`, `operation`, `cacheMemorySizeGb`, `servicename`
- **No `apiType` attribute** — REST vs HTTP distinguished by `operation` field and `usagetype` suffix:
  - REST: `operation=ApiGatewayRequest`, usagetype suffix `ApiGatewayRequest`
  - HTTP: `operation=ApiGatewayHttpApi`, usagetype suffix `ApiGatewayHttpRequest`
  - WebSocket: `operation=ApiGatewayWebSocket`, usagetype suffix `ApiGatewayMessage`
- **units in priceDimensions**: `Requests` (API Calls), `Messages` (WebSocket), `GB` (Cache)
- **REST API tier ranges (us-east-1)**:
  - `0 – 333,000,000` → $3.50/million (first 333M)
  - `333,000,000 – 1,000,000,000` → $3.19/million (next 667M)
  - `1,000,000,000 – 20,000,000,000` → $2.71/million (next 19B)
  - `20,000,000,000 – Inf` → $1.72/million (over 20B)
- **HTTP API tier ranges (global sample)**:
  - `0 – 300,000,000` → ~$1.00/million (first 300M)
  - `300,000,000 – Inf` → ~$0.90/million (over 300M)
- **Surprises vs plan**:
  - Plan expects tier breakpoints: REST `333M → 667M → 19B → ∞`. Actual boundaries seen are `333M, 1B, 20B` (4 tiers total in the new region, 3 price steps after free). The plan's tier spec uses relative sizes (333M, 667M more) which don't match the absolute beginRange values. Ingest must use beginRange/endRange directly.
  - HTTP API: plan says `tier=0,tier_upper=300M` then `tier=300M,tier_upper=""` — matches observed 2 tiers ✓
  - No `apiType` attribute exists; must derive type from `operation` field value.
  - `Amazon API Gateway Portals` and `Amazon API Gateway Cache` families are present and must be excluded from the `API Calls` filter.
  - WebSocket: plan defers WebSocket — confirmed present as a separate productFamily, easy to exclude ✓
- **matches plan**: yes with notes — tier absolute values need verification at fixture-write time; REST has 4 tiers (not 3 as implied by plan's tier_upper tokens)

---

## Azure

### azure_service_bus_queues

- **serviceName in API**: `'Service Bus'`
- **productName in items**: `"Service Bus"` (single product name — no queue/topic split)
- **Relevant skuNames / meterNames**:
  - `Basic / Basic Messaging Operations` — unit: `1M`
  - `Standard / Standard Messaging Operations` — unit: `1M`, tiered (0, 13M, 100M, 2500M units)
  - `Standard / Standard Base Unit` — unit: `1/Hour` or `1/Month`
  - `Premium / Premium Messaging Unit` — unit: `1/Hour`
  - `Standard / Standard Brokered Connection` — unit: `1`
  - `Hybrid Connections / Hybrid Connections Listener Unit` — unit: `1 Hour`
  - `Hybrid Connections / Hybrid Connections Data Transfer` — unit: `1 GB`
  - `WCF Relay / WCF Relay Message` — unit: `10K`
  - `Geo Replication Zone 1/2/3 / Geo Replication Zone N Data Transfer` — unit: `1 GB`
- **Standard Messaging Operations tiers (eastus)**: 0 → $0.00; 13M → $0.80/M; 100M → $0.50/M; 2500M → $0.20/M
- **Surprises vs plan**:
  - No queue vs topic distinction in the API — both queues and topics use the same `Standard Messaging Operations` meter. Ingest split into `_queues` and `_topics` shards is logical grouping only (both ingest the same meter; the common helper pattern is appropriate).
  - Basic tier only has messaging operations pricing (no base unit fee).
  - Standard tier has both a base unit fee (`1/Month = $10`, or hourly `~$0.013/hr`) AND per-operation pricing. Ingest must decide how to emit the base unit fee row.
  - Premium has per-MU-hour pricing only (no per-operation fee).
  - Hybrid Connections and WCF Relay are unrelated — must be excluded.
  - `Geo Replication` meters present — exclude from core ingest.
- **matches plan**: yes — filter for Standard/Premium operations meters; shared common helper pattern ✓; tiered pricing structure confirmed

---

### azure_service_bus_topics

- **serviceName / productName**: same as queues — `'Service Bus'` / `"Service Bus"`
- **Same meter set as queues** — no topic-specific meters exist
- **Relevant meters for topics**: `Standard Messaging Operations` (same as queues), `Standard Base Unit`, `Premium Messaging Unit`
- **Surprises vs plan**:
  - Identical upstream shape to queues — the split into `_queues` vs `_topics` shards must be a logical/semantic split with shared common helper ✓
  - Plan says "topic-shaped meters" but no such distinction exists upstream — both shards will filter the same meters.
- **matches plan**: yes — shared common helper confirmed as the right approach

---

### azure_event_hubs

- **serviceName**: `'Event Hubs'`
- **productName**: `"Event Hubs"`
- **Relevant skuNames / meterNames**:
  - `Basic / Basic Throughput Unit` — unit: `1 Hour`
  - `Basic / Basic Ingress Events` — unit: `1M`
  - `Standard / Standard Throughput Unit` — unit: `1 Hour`
  - `Standard / Standard Ingress Events` — unit: `1M`
  - `Standard / Standard Kafka Endpoint` — unit: `1 Hour`
  - `Standard / Standard Capture` — unit: `1 Hour`
  - `Premium / Premium Processing Unit` — unit: `1 Hour`
  - `Premium / Premium Extended Retention` — unit: `1 GB/Month`
  - `Dedicated / Dedicated Capacity Unit` — unit: `1 Hour`
  - `Geo Replication Zone N / Geo Replication Zone N Data Transfer` — unit: `1 GB`
- **Surprises vs plan**:
  - Plan expects `per-throughput-unit-hour + per-million-events` dims — confirmed ✓ (Throughput Unit + Ingress Events)
  - Standard tier has Kafka endpoint hour pricing and Capture hour pricing in addition — must decide whether to include or exclude.
  - Dedicated Cluster tier exists (deferred per plan ✓).
  - Premium tier uses Processing Unit (PPU) not TU — different unit than Standard.
  - No dedicated `messaging.queue` semantics in API — Event Hubs is a stream, but plan maps it to `messaging.queue` ✓
- **matches plan**: yes — Standard and Premium tiers confirmed; TU + events dims confirmed; Dedicated deferred ✓

---

### azure_front_door

- **serviceName**: `'Azure Front Door Service'`
- **Two productNames present**:
  - `"Azure Front Door"` — the newer CDN-native AFD (Standard/Premium profiles)
  - `"Azure Front Door Service"` — the classic AFD (also uses Standard skuName but different meters)
- **Azure Front Door (new) meters**:
  - `Standard / Standard Base Fees` — unit: `1/Month`, price: $35
  - `Standard / Standard Data Transfer In` — unit: `1 GB`, price: $0.02
  - `Standard / Standard Data Transfer Out` — unit: `1 GB`, tiered (0→$0.0825, 10TB→$0.065, 50TB→$0.056, 150TB→$0.014, 500TB→$0.0069, 1PB→$0.00574, 5PB→$0.0054)
  - `Standard / Standard Requests` — unit: `10K`, tiered (0→$0.009, 250M→$0.0081, 1B→$0.0073, 5B→$0.0066 per 10K)
  - `Premium / Premium Base Fees` — unit: `1/Month`, price: $330
  - `Premium / Premium Data Transfer Out` — similar tiers
  - `Premium / Premium Requests` — tiered
  - `Premium / Premium Captcha Sessions` — unit: `1K`
  - `Premium / Premium Data Transfer In` — unit: `1 GB`
- **Azure Front Door Service (classic) meters**: includes routing rules, bot protection, custom domain, WAF rules — deferred/exclude per plan
- **Tier ranges for Standard Data Transfer Out (Zone 1)**:
  - 0 → 10,000 GB: $0.0825/GB
  - 10,000 → 50,000 GB: $0.065/GB
  - 50,000 → 150,000 GB: $0.056/GB
  - 150,000 → 500,000 GB: $0.014/GB
  - 500,000 → 1,000,000 GB: $0.0069/GB
  - 1,000,000 → 5,000,000 GB: $0.00574/GB
  - 5,000,000+ GB: $0.0054/GB
- **Surprises vs plan**:
  - Two productNames for AFD in the same serviceName — must filter to `productName = 'Azure Front Door'` (new) and exclude `'Azure Front Door Service'` (classic).
  - 7 egress tiers, not 3 as implied by plan's `tier=0,tier_upper=10TB` sketch — actual tier_upper values are 10TB, 50TB, 150TB, 500TB, 1PB, 5PB, ∞.
  - `Base Fees` is a flat monthly charge per SKU, confirming `extra.sku` join pattern needed ✓
  - `Standard Data Transfer In` is $0.02/GB — not free, but usually ignored for CDN cost estimation.
  - Requests are per `10K` unit (not per million like SNS) — unit conversion needed.
- **matches plan**: yes with tier count note — more egress tiers than plan sketch; `extra.sku` base-fee pattern confirmed

---

### azure_apim

- **serviceName**: `'API Management'`
- **productName**: `"API Management"`
- **Relevant skuNames / meterNames**:
  - `Consumption / Consumption Calls` — unit: `10K`, first 1M free then $0.035/10K
  - `Developer / Developer Unit` — unit: `1 Hour` ($0.0658/hr) or `1/Month` ($49)
  - `Basic / Basic Unit` — unit: `1 Hour` ($0.2016/hr)
  - `Standard / Standard Unit` — unit: `1 Hour` ($0.9407/hr)
  - `Premium / Premium Unit` — unit: `1 Hour` ($3.829/hr) or `1/Month` ($2849)
  - `Isolated / Isolated Unit` — unit: `1 Hour` ($21.77/hr)
  - `Basic v2 / Basic v2 Unit` — unit: `1 Hour` ($0.205/hr)
  - `Standard v2 / Standard v2 Unit` — unit: `1 Hour` ($0.959/hr)
  - `Premium v2 / Premium v2 Unit` — unit: `1 Hour` ($3.836/hr)
  - `Gateway / Gateway Unit` — unit: `1 Hour` (self-hosted gateway)
  - Workspace Pack meters for v2 tiers (Isolated, Standard, Premium v2)
  - Secondary Unit meters for v2 multi-region
- **Surprises vs plan**:
  - Plan says "Developer / Basic / Standard / Premium / Premium-v2 / Isolated" — actual data also has `Basic v2`, `Standard v2`, `Gateway`, `Workspace Gateway Standard/Premium` skus. Need to decide scope.
  - Both hourly and monthly prices appear for the same tier in different regions — prefer hourly for uniform emission.
  - `Consumption` tier: first 1M calls free (tier=0 price=0, tier=100 (×10K=1M) price=$0.035/10K) — plan says "per-million-calls" ✓ but unit is per-10K not per-million in the API.
  - `Developer` tier should be dev/test only — plan includes it ✓
  - `terms.os = "apim-{tier}"` slot encoding in plan is reasonable given single dimension per tier.
- **matches plan**: yes — consumption + provisioned tiers confirmed; unit is per-10K calls (not per-million); v2 tiers exist as additional scope not in plan sketch

---

## GCP

**Auth note**: GCP Cloud Billing Catalog API requires authentication. Used local ADC (`gcloud auth application-default login`). GCP service IDs not yet in `gcp_common._GCP_SERVICE_IDS` for these 4 new shards — must be added.

### gcp_pubsub_queues

- **Service**: Cloud Pub/Sub, serviceId: `A1E8-BE35-7EBC`
- **Total SKUs**: 77
- **resourceGroups**: `Message` (13), `InterregionEgress` (25), `PremiumInternetEgress` (30), `PeeringOrInterconnectEgress` (5), `Subscription` (2), `InterzoneEgress` (1), `GoogleEgress` (1)
- **Message group SKUs** (most relevant):
  - `Message Delivery Basic` — unit: `TiBy`, prices: free up to 10GiB/month then $40/TiB
  - `Message Delivery to Google Cloud Storage` — unit: `TiBy`, $50/TiB
  - `Message Delivery to BigQuery` — unit: `TiBy`, $50/TiB
  - `Message Delivery to Bigtable` — unit: `TiBy`, $50/TiB
  - `Message Transform Data Processing` — unit: `TiBy`, $40/TiB
  - `Message Transform Data Enrichment` — unit: `TiBy`, $60/TiB
  - `Message Delivery From Azure Event Hubs` — unit: `TiBy`, $80/TiB
  - `Message Delivery From AWS MSK` — unit: `TiBy`, $80/TiB
  - `Message Delivery From Kinesis Data Streams` — unit: `TiBy`, $50/TiB
  - `Message Delivery From Confluent Cloud` — unit: `TiBy`, $80/TiB
  - `Message Delivery From Google Cloud Storage` — unit: `TiBy`, $80/TiB
  - `Topics message backlog` — unit: `GiBy.mo`, $0.27/GiB-month
  - `Subscriptions retained acknowledged messages` — unit: `GiBy.mo`, $0.27/GiB-month
- **serviceRegions**: all in `['global']`
- **Surprises vs plan**:
  - **No region-level granularity** — Pub/Sub pricing is global only, not per-region. Plan expects "3 region rows" for the fixture but all Pub/Sub Message SKUs have `serviceRegions=['global']`. The fixture must use `region='global'` not per-region values.
  - Plan says "per-TiB pricing" — confirmed, unit is `TiBy` ✓
  - Ingest filter for `Message Delivery Basic` (the main throughput SKU) needs to exclude the connector/transform SKUs (GCS, BigQuery, Bigtable, Azure, AWS, Confluent deliveries) and retain-acknowledged/backlog messages.
  - No queue/topic distinction in the Pub/Sub billing catalog — both `gcp_pubsub_queues` and `gcp_pubsub_topics` will read the same service. Shared `_gcp_pubsub_common.py` pattern is correct ✓
  - First 10 GiB/month is free (tier price=0 up to 0.009765625 TiB) — plan should account for this in fixture.
- **matches plan**: partial — no per-region SKUs (all global); fixture should reflect `region=global`

---

### gcp_pubsub_topics

- **Same service as queues**: `A1E8-BE35-7EBC`
- **Same SKU set** — no topic-specific vs queue-specific meter distinction in Pub/Sub billing catalog
- **Surprises vs plan**:
  - Same as `gcp_pubsub_queues` — all pricing is global, not per-region
  - The queue/topic split is semantic only; both shards ingest `Message Delivery Basic` (and backlog/retain meters) from the same service
- **matches plan**: same as queues — global-only pricing, no queue/topic meter split

---

### gcp_firestore

- **Service**: Cloud Firestore, serviceId: `EE2C-7FAC-5E08`
- **Total SKUs**: 2000
- **resourceGroups**: `FirestoreEntityPutOps` (94), `FirestoreEntityDeleteOps` (94), `FirestoreReadOps` (110), `FirestoreStorage` (94), `FirestoreSmallOps` (94), `FirestorePITRStorage` (90), `FirestoreBandwidth` (92), `FirestoreZonalBackupStorage` (92), `FirestoreRestoreOps` (90), `FirestoreTtlDeleteOps` (93), `DatastoreOps` (1020), `DatastoreBandwidth` (94)
- **Native mode groups** (target):
  - `FirestoreEntityPutOps` (writes): unit=`count`, tiered (free up to 20K/day, then ~$1.8/million)
  - `FirestoreEntityDeleteOps` (deletes): unit=`count`, tiered (free up to 20K/day, then ~$0.10/million — significantly cheaper than writes)
  - `FirestoreReadOps` (reads): unit=`count`, ~$0.33/million (no free tier in sample seen)
  - `FirestoreStorage`: unit=`GiBy.mo`, ~$0.135/GiB/month (region-dependent)
  - `FirestoreSmallOps` (list, get, etc.): unit=`count`, appears $0.00 in samples
- **Datastore mode groups**: `DatastoreOps` (1020 SKUs), `DatastoreBandwidth` — these are the "Datastore mode deferred" group per plan ✓
- **SKU descriptions include region** (e.g., "Cloud Firestore Entity Writes Europe 3", "Cloud Firestore Read Ops Mexico") — region is embedded in description, not a structured field
- **Surprises vs plan**:
  - Plan says "Native mode only (Datastore mode deferred)" — Firestore service contains both `FirestoreXxx` and `DatastoreOps` groups in the same service. Filter must use resourceGroup prefix `Firestore` not `Datastore` ✓
  - `FirestoreSmallOps` ($0.00) appears in the data — likely free metadata operations. Should be included or excluded depending on spec intent.
  - Free tiers embedded in tieredRates (price=0 up to threshold) — ingest must handle.
  - Delete ops are significantly cheaper than write ops ($0.10/M vs $1.8/M) — spec says "document_delete" as a pricing dim ✓ but the price difference is notable.
  - Region is embedded in the SKU description string (not a structured region attribute) — region normalization must parse description text.
  - `FirestorePITRStorage`, `FirestoreBandwidth`, `FirestoreZonalBackupStorage`, `FirestoreRestoreOps`, `FirestoreTtlDeleteOps` are supplemental dims — exclude from core price rows unless spec explicitly requires them.
- **matches plan**: yes — 4 pricing dims (storage, read, write, delete) confirmed; Datastore mode can be excluded by resourceGroup filter; region extraction from description needed

---

### gcp_cloud_cdn

- **Service**: CDN is billed under **Networking** service, serviceId: `E505-1604-58F8` (NOT a separate Cloud CDN service)
- **No dedicated serviceId for Cloud CDN** in the billing catalog — part of Networking
- **Relevant resourceGroups**: `CDNCacheEgress` (7 SKUs — geography-based egress), `Cdn` (1 SKU — cache lookups)
- **CDNCacheEgress SKUs** (egress from cache to clients):
  - `Networking Cloud CDN Traffic Cache Data Transfer to North America` — $0.08/GiB (0–10TB), $0.055 (10–150TB), $0.03 (150TB+)
  - `Networking Cloud CDN Traffic Cache Data Transfer to Europe` — same tiers as NA
  - `Networking Cloud CDN Traffic Cache Data Transfer to Asia` — $0.09, $0.06, $0.05
  - `Networking Cloud CDN Traffic Cache Data Transfer to Latin America` — $0.09, $0.06, $0.05
  - `Networking Cloud CDN Traffic Cache Data Transfer to Oceania` — $0.11, $0.09, $0.08
  - `Networking Cloud CDN Traffic Cache Data Transfer to Other` — $0.09, $0.06, $0.05
  - `Networking Cloud CDN Traffic Cache Data Transfer to China` — $0.20, $0.17, $0.16
- **Cdn group**: `Networking Cloud Cdn Cache Lookups` — unit: `count`, $0.00000075/lookup (~$0.75/million)
- **EdgeCacheEgress group** (separate Media CDN): `Networking Edge Cache Data Transfer from *` — different product (Media CDN); exclude from `gcp_cloud_cdn`
- **Tier ranges (bytes-domain)**: tiers at 0, 10240 GiB (10TB), 153600 GiB (150TB)
- **No cache-fill pricing found**: the plan expects a `cache-fill` dimension but no cache-fill (origin-to-cache) meter found under `CDNCacheEgress` or `Cdn` groups in Networking. Cache fill may be priced under Compute Engine egress or may not be separately billed.
- **Surprises vs plan**:
  - Cloud CDN is under the Networking service (not a standalone service ID) — `gcp_common._GCP_SERVICE_IDS` must be set to `E505-1604-58F8` for this shard (or ingest reads Networking service and filters).
  - 3 egress price tiers (not more) — plan expects tiered bytes-domain rows, confirmed ✓ but only 3 tier breakpoints (0, 10TB, 150TB).
  - Cache-fill dimension: **not found** in Networking service under CDN-related groups. Either it's free, bundled, or priced under Compute Engine internet egress. Needs further investigation — recommend dropping `cache-fill` mode from initial implementation or mapping it to the Compute Engine egress path.
  - Media CDN (`EdgeCacheEgress`) is a distinct product — must be excluded from `gcp_cloud_cdn`.
  - Requests unit: `count` at $0.00000075/request ($0.75/million) — confirms `request` mode ✓
  - No `extra.sku` candidate found (unlike Front Door); plan says `extra.sku = cloud-cdn-<region-class>` which is a synthetic key, not from the data.
- **matches plan**: partial — egress tiers confirmed; cache-fill meter not found upstream; service ID is Networking not a Cloud CDN–specific service

---

### gcp_cloud_dns

- **Service**: Cloud DNS, serviceId: `FA26-5236-B8B5`
- **Total SKUs**: 2 (very small!)
- **resourceGroups**: `DNS` (2 SKUs)
- **SKUs**:
  - `DNS Query (port 53)` — unit: `count`, tiered:
    - `0 – 1,000,000,000` → $0.40/million
    - `1,000,000,000 – Inf` → $0.20/million
  - `ManagedZone` — unit: `mo`, tiered:
    - `0 – 25` zones → $0.20/zone/month
    - `25 – 10,000` zones → $0.10/zone/month
    - `10,000+` zones → $0.03/zone/month
- **No public/private zone distinction** — single `ManagedZone` SKU covers both (same pricing)
- **serviceRegions**: `global` (Cloud DNS is a global service) ✓
- **Surprises vs plan**:
  - Plan expects `public + private` zone rows and `tier=0,tier_upper=25` / `tier=25,tier_upper=""` for zones. Actual data has 3 zone tiers: 0–25, 25–10K, 10K+. Plan's zone tier spec is incomplete (missing the 10K breakpoint).
  - Plan expects `tier=0,tier_upper=1B` / `tier=1B,tier_upper=""` for queries — actual data has exactly 2 query tiers matching this ✓
  - No public/private distinction in billing data — emit single `zone-type=public` row (or emit both with same price as a workaround for compare semantics).
  - Only 2 SKUs total in this service — very small dataset, easy to manage.
  - Zone pricing includes a 3rd tier at 10K+ zones ($0.03/zone) not mentioned in plan.
- **matches plan**: mostly yes — 2 query tiers ✓; zone tiers differ (3 not 2); public/private distinction not in upstream data

---

## Summary of Cross-Shard Observations

| Shard | Auth | Total SKUs | Key Surprise |
|-------|------|-----------|--------------|
| aws_sqs | none | 109 | Service code is `AWSQueueService` not `AmazonSQS` |
| aws_sns | none | 971 | `API Request` has no `endpointType`; endpoint types are in `Message Delivery` |
| aws_route53 | none | 356 | No public/private zone distinction; query tiers are 2 not 3 |
| aws_api_gateway | none | 492 | No `apiType` attr; REST/HTTP distinguished by `operation` field; REST has 4 tiers |
| azure_service_bus_queues | none | 1000 (top=1000) | No queue/topic meter split; common helper pattern confirmed |
| azure_service_bus_topics | none | same | Identical upstream to queues |
| azure_event_hubs | none | 713 | Premium uses PPU not TU; Kafka endpoint + Capture are extra meters |
| azure_front_door | none | 382 | Two productNames (classic vs new); 7 egress tiers not 3 |
| azure_apim | none | 1000 (top=1000) | v2 tiers (Basic v2, Standard v2, Premium v2) exist beyond plan scope; Consumption unit is per-10K |
| gcp_pubsub_queues | ADC | 77 | All pricing is global only, no per-region SKUs |
| gcp_pubsub_topics | ADC | 77 (shared) | Same as queues — no topic/queue meter split |
| gcp_firestore | ADC | 2000 | Region in description string only; 5 native-mode groups confirmed |
| gcp_cloud_cdn | ADC | ~17 CDN SKUs | Under Networking service (E505-1604-58F8); cache-fill meter not found |
| gcp_cloud_dns | ADC | 2 | 3 zone tiers (not 2); no public/private distinction |

### Items requiring spec update before Phase 1/2/3

1. **aws_sqs**: Filter must use `servicecode = 'AWSQueueService'` not `AmazonSQS`
2. **aws_sns**: Clarify whether ingest targets `API Request` (publish) or `Message Delivery` (delivery-by-endpoint)
3. **aws_route53**: Drop public/private zone distinction (not in upstream); correct zone+query tier count
4. **aws_api_gateway**: No `apiType` attr — filter by `operation` field; REST has 4 tiers
5. **gcp_pubsub_***: Fixture should use `region=global` not per-region
6. **gcp_cloud_cdn**: Service ID is Networking `E505-1604-58F8`; cache-fill mode not found upstream
7. **gcp_cloud_dns**: 3 zone tiers (add `tier_upper=10000` breakpoint)

---

## Ship mode decision

Single PR — all 14 shards + Phase 0 wired into one branch.
