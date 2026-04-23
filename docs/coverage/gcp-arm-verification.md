# GCP serverless — arm architecture verification

_Verified against fresh catalog snapshots on 2026-04-23._

## Cloud Run (serviceId 152E-C115-5142)

Distinct ApplicationServices-family CPU/Memory SKU descriptions observed:

```
Jobs CPU in africa-south1
Jobs CPU in asia-east1
Jobs CPU in asia-east2
Jobs CPU in asia-northeast1
Jobs CPU in asia-northeast2
Jobs CPU in asia-northeast3
Jobs CPU in asia-south1
Jobs CPU in asia-south2
Jobs CPU in asia-southeast1
Jobs CPU in asia-southeast2
Jobs CPU in asia-southeast3
Jobs CPU in australia-southeast1
Jobs CPU in australia-southeast2
Jobs CPU in europe-central2
Jobs CPU in europe-north1
Jobs CPU in europe-north2
Jobs CPU in europe-southwest1
Jobs CPU in europe-west1
Jobs CPU in europe-west10
Jobs CPU in europe-west12
Jobs CPU in europe-west2
Jobs CPU in europe-west3
Jobs CPU in europe-west4
Jobs CPU in europe-west6
Jobs CPU in europe-west8
Jobs CPU in europe-west9
Jobs CPU in me-central1
Jobs CPU in me-central2
Jobs CPU in me-west1
Jobs CPU in northamerica-northeast1
Jobs CPU in northamerica-northeast2
Jobs CPU in northamerica-south1
Jobs CPU in southamerica-east1
Jobs CPU in southamerica-west1
Jobs CPU in us-central1
Jobs CPU in us-east1
Jobs CPU in us-east4
Jobs CPU in us-east5
Jobs CPU in us-east7
Jobs CPU in us-south1
Jobs CPU in us-west1
Jobs CPU in us-west2
Jobs CPU in us-west3
Jobs CPU in us-west4
Jobs CPU in us-west8
Jobs Memory in africa-south1
Jobs Memory in asia-east1
Jobs Memory in asia-east2
Jobs Memory in asia-northeast1
Jobs Memory in asia-northeast2
Jobs Memory in asia-northeast3
Jobs Memory in asia-south1
Jobs Memory in asia-south2
Jobs Memory in asia-southeast1
Jobs Memory in asia-southeast2
Jobs Memory in asia-southeast3
Jobs Memory in australia-southeast1
Jobs Memory in australia-southeast2
Jobs Memory in europe-central2
Jobs Memory in europe-north1
Jobs Memory in europe-north2
Jobs Memory in europe-southwest1
Jobs Memory in europe-west1
Jobs Memory in europe-west10
Jobs Memory in europe-west12
Jobs Memory in europe-west2
Jobs Memory in europe-west3
Jobs Memory in europe-west4
Jobs Memory in europe-west6
Jobs Memory in europe-west8
Jobs Memory in europe-west9
Jobs Memory in me-central1
Jobs Memory in me-central2
Jobs Memory in me-west1
Jobs Memory in northamerica-northeast1
Jobs Memory in northamerica-northeast2
Jobs Memory in northamerica-south1
Jobs Memory in southamerica-east1
Jobs Memory in southamerica-west1
Jobs Memory in us-central1
Jobs Memory in us-east1
Jobs Memory in us-east4
Jobs Memory in us-east5
Jobs Memory in us-east7
Jobs Memory in us-south1
Jobs Memory in us-west1
Jobs Memory in us-west2
Jobs Memory in us-west3
Jobs Memory in us-west4
Jobs Memory in us-west8
Services CPU (Instance-based billing) in africa-south1
Services CPU (Instance-based billing) in asia-east1
Services CPU (Instance-based billing) in asia-east2
Services CPU (Instance-based billing) in asia-northeast2
Services CPU (Instance-based billing) in asia-northeast3
Services CPU (Instance-based billing) in asia-south1
Services CPU (Instance-based billing) in asia-south2
Services CPU (Instance-based billing) in asia-southeast1
Services CPU (Instance-based billing) in asia-southeast2
Services CPU (Instance-based billing) in asia-southeast3
Services CPU (Instance-based billing) in australia-southeast1
Services CPU (Instance-based billing) in australia-southeast2
Services CPU (Instance-based billing) in europe-central2
Services CPU (Instance-based billing) in europe-north1
Services CPU (Instance-based billing) in europe-north2
Services CPU (Instance-based billing) in europe-southwest1
Services CPU (Instance-based billing) in europe-west1
Services CPU (Instance-based billing) in europe-west10
Services CPU (Instance-based billing) in europe-west12
Services CPU (Instance-based billing) in europe-west2
Services CPU (Instance-based billing) in europe-west3
Services CPU (Instance-based billing) in europe-west4
Services CPU (Instance-based billing) in europe-west6
Services CPU (Instance-based billing) in europe-west8
Services CPU (Instance-based billing) in europe-west9
Services CPU (Instance-based billing) in in asia-northeast1
Services CPU (Instance-based billing) in me-central1
Services CPU (Instance-based billing) in me-central2
Services CPU (Instance-based billing) in me-west1
Services CPU (Instance-based billing) in northamerica-northeast1
Services CPU (Instance-based billing) in northamerica-northeast2
Services CPU (Instance-based billing) in northamerica-south1
Services CPU (Instance-based billing) in southamerica-east1
Services CPU (Instance-based billing) in southamerica-west1
Services CPU (Instance-based billing) in us-central1
Services CPU (Instance-based billing) in us-east1
Services CPU (Instance-based billing) in us-east4
Services CPU (Instance-based billing) in us-east5
Services CPU (Instance-based billing) in us-east7
Services CPU (Instance-based billing) in us-south1
Services CPU (Instance-based billing) in us-west1
Services CPU (Instance-based billing) in us-west2
Services CPU (Instance-based billing) in us-west3
Services CPU (Instance-based billing) in us-west4
Services CPU (Instance-based billing) in us-west8
Services CPU (Request-based billing)
Services CPU Tier 2  (Request-based billing)
Services Memory (Instance-based billing) in africa-south1
Services Memory (Instance-based billing) in asia-east1
Services Memory (Instance-based billing) in asia-east2
Services Memory (Instance-based billing) in asia-northeast1
Services Memory (Instance-based billing) in asia-northeast2
Services Memory (Instance-based billing) in asia-northeast3
Services Memory (Instance-based billing) in asia-south1
Services Memory (Instance-based billing) in asia-south2
Services Memory (Instance-based billing) in asia-southeast1
Services Memory (Instance-based billing) in asia-southeast2
Services Memory (Instance-based billing) in asia-southeast3
Services Memory (Instance-based billing) in australia-southeast1
Services Memory (Instance-based billing) in australia-southeast2
Services Memory (Instance-based billing) in europe-central2
Services Memory (Instance-based billing) in europe-north1
Services Memory (Instance-based billing) in europe-north2
Services Memory (Instance-based billing) in europe-southwest1
Services Memory (Instance-based billing) in europe-west1
Services Memory (Instance-based billing) in europe-west10
Services Memory (Instance-based billing) in europe-west12
Services Memory (Instance-based billing) in europe-west2
Services Memory (Instance-based billing) in europe-west3
Services Memory (Instance-based billing) in europe-west4
Services Memory (Instance-based billing) in europe-west6
Services Memory (Instance-based billing) in europe-west8
Services Memory (Instance-based billing) in europe-west9
Services Memory (Instance-based billing) in me-central1
Services Memory (Instance-based billing) in me-central2
Services Memory (Instance-based billing) in me-west1
Services Memory (Instance-based billing) in northamerica-northeast1
Services Memory (Instance-based billing) in northamerica-northeast2
Services Memory (Instance-based billing) in northamerica-south1
Services Memory (Instance-based billing) in southamerica-east1
Services Memory (Instance-based billing) in southamerica-west1
Services Memory (Instance-based billing) in us-central1
Services Memory (Instance-based billing) in us-east1
Services Memory (Instance-based billing) in us-east4
Services Memory (Instance-based billing) in us-east5
Services Memory (Instance-based billing) in us-east7
Services Memory (Instance-based billing) in us-south1
Services Memory (Instance-based billing) in us-west1
Services Memory (Instance-based billing) in us-west2
Services Memory (Instance-based billing) in us-west3
Services Memory (Instance-based billing) in us-west4
Services Memory (Instance-based billing) in us-west8
Services Memory (Request-based billing)
Services Memory Tier 2 (Request-based billing)
Services Min Instance CPU (Request-based billing)
Services Min Instance CPU Tier 2 (Request-based billing)
Services Min Instance Memory (Request-based billing)
Services Min Instance Memory Tier 2 (Request-based billing)
Worker Pools CPU in africa-south1
Worker Pools CPU in asia-east1
Worker Pools CPU in asia-east2
Worker Pools CPU in asia-northeast1
Worker Pools CPU in asia-northeast2
Worker Pools CPU in asia-northeast3
Worker Pools CPU in asia-south1
Worker Pools CPU in asia-south2
Worker Pools CPU in asia-southeast1
Worker Pools CPU in asia-southeast2
Worker Pools CPU in asia-southeast3
Worker Pools CPU in australia-southeast1
Worker Pools CPU in australia-southeast2
Worker Pools CPU in europe-central2
Worker Pools CPU in europe-north1
Worker Pools CPU in europe-north2
Worker Pools CPU in europe-southwest1
Worker Pools CPU in europe-west1
Worker Pools CPU in europe-west10
Worker Pools CPU in europe-west12
Worker Pools CPU in europe-west2
Worker Pools CPU in europe-west3
Worker Pools CPU in europe-west4
Worker Pools CPU in europe-west6
Worker Pools CPU in europe-west8
Worker Pools CPU in europe-west9
Worker Pools CPU in me-central1
Worker Pools CPU in me-central2
Worker Pools CPU in me-west1
Worker Pools CPU in northamerica-northeast1
Worker Pools CPU in northamerica-northeast2
Worker Pools CPU in northamerica-south1
Worker Pools CPU in southamerica-east1
Worker Pools CPU in southamerica-west1
Worker Pools CPU in us-central1
Worker Pools CPU in us-east1
Worker Pools CPU in us-east4
Worker Pools CPU in us-east5
Worker Pools CPU in us-south1
Worker Pools CPU in us-west1
Worker Pools CPU in us-west2
Worker Pools CPU in us-west3
Worker Pools CPU in us-west4
Worker Pools CPU in us-west8
Worker Pools Memory in africa-south1
Worker Pools Memory in asia-east1
Worker Pools Memory in asia-east2
Worker Pools Memory in asia-northeast1
Worker Pools Memory in asia-northeast2
Worker Pools Memory in asia-northeast3
Worker Pools Memory in asia-south1
Worker Pools Memory in asia-south2
Worker Pools Memory in asia-southeast1
Worker Pools Memory in asia-southeast2
Worker Pools Memory in asia-southeast3
Worker Pools Memory in australia-southeast1
Worker Pools Memory in australia-southeast2
Worker Pools Memory in europe-central2
Worker Pools Memory in europe-north1
Worker Pools Memory in europe-north2
Worker Pools Memory in europe-southwest1
Worker Pools Memory in europe-west1
Worker Pools Memory in europe-west10
Worker Pools Memory in europe-west12
Worker Pools Memory in europe-west2
Worker Pools Memory in europe-west3
Worker Pools Memory in europe-west4
Worker Pools Memory in europe-west6
Worker Pools Memory in europe-west8
Worker Pools Memory in europe-west9
Worker Pools Memory in me-central1
Worker Pools Memory in me-central2
Worker Pools Memory in me-west1
Worker Pools Memory in northamerica-northeast1
Worker Pools Memory in northamerica-northeast2
Worker Pools Memory in northamerica-south1
Worker Pools Memory in southamerica-east1
Worker Pools Memory in southamerica-west1
Worker Pools Memory in us-central1
Worker Pools Memory in us-east1
Worker Pools Memory in us-east4
Worker Pools Memory in us-east5
Worker Pools Memory in us-south1
Worker Pools Memory in us-west1
Worker Pools Memory in us-west2
Worker Pools Memory in us-west3
Worker Pools Memory in us-west4
Worker Pools Memory in us-west8
```

**Requests/Invocations with arm suffix:** none found

**Finding:** arm SKUs absent

## Cloud Run Functions (serviceId 29E7-DA93-CA13)

Distinct ApplicationServices-family CPU/Memory SKU descriptions observed:

```
Cloud Run functions (1st Gen) CPU (Request-based billing)
Cloud Run functions (1st Gen) CPU Tier 2 (Request-based billing)
Cloud Run functions (1st Gen) Memory (Request-based billing)
Cloud Run functions (1st Gen) Memory Tier 2 (Request-based billing)
Cloud Run functions (1st Gen) Min Instance CPU (Request-based billing)
Cloud Run functions (1st Gen) Min Instance CPU Tier 2 (Request-based billing)
Cloud Run functions (1st Gen) Min Instance Memory (Request-based billing)
Cloud Run functions (1st Gen) Min Instance Memory Tier 2 (Request-based billing)
Cloud Run functions CPU (Request-based billing) in africa-south1
Cloud Run functions CPU (Request-based billing) in asia-east1
Cloud Run functions CPU (Request-based billing) in asia-east2
Cloud Run functions CPU (Request-based billing) in asia-northeast1
Cloud Run functions CPU (Request-based billing) in asia-northeast2
Cloud Run functions CPU (Request-based billing) in asia-northeast3
Cloud Run functions CPU (Request-based billing) in asia-south1
Cloud Run functions CPU (Request-based billing) in asia-south2
Cloud Run functions CPU (Request-based billing) in asia-southeast1
Cloud Run functions CPU (Request-based billing) in asia-southeast2
Cloud Run functions CPU (Request-based billing) in australia-southeast1
Cloud Run functions CPU (Request-based billing) in australia-southeast2
Cloud Run functions CPU (Request-based billing) in europe-central2
Cloud Run functions CPU (Request-based billing) in europe-north1
Cloud Run functions CPU (Request-based billing) in europe-southwest1
Cloud Run functions CPU (Request-based billing) in europe-west1
Cloud Run functions CPU (Request-based billing) in europe-west10
Cloud Run functions CPU (Request-based billing) in europe-west12
Cloud Run functions CPU (Request-based billing) in europe-west2
Cloud Run functions CPU (Request-based billing) in europe-west3
Cloud Run functions CPU (Request-based billing) in europe-west4
Cloud Run functions CPU (Request-based billing) in europe-west6
Cloud Run functions CPU (Request-based billing) in europe-west8
Cloud Run functions CPU (Request-based billing) in europe-west9
Cloud Run functions CPU (Request-based billing) in me-central1
Cloud Run functions CPU (Request-based billing) in me-central2
Cloud Run functions CPU (Request-based billing) in me-west1
Cloud Run functions CPU (Request-based billing) in northamerica-northeast1
Cloud Run functions CPU (Request-based billing) in northamerica-northeast2
Cloud Run functions CPU (Request-based billing) in southamerica-east1
Cloud Run functions CPU (Request-based billing) in southamerica-west1
Cloud Run functions CPU (Request-based billing) in us-central1
Cloud Run functions CPU (Request-based billing) in us-east1
Cloud Run functions CPU (Request-based billing) in us-east4
Cloud Run functions CPU (Request-based billing) in us-east5
Cloud Run functions CPU (Request-based billing) in us-east7
Cloud Run functions CPU (Request-based billing) in us-south1
Cloud Run functions CPU (Request-based billing) in us-west1
Cloud Run functions CPU (Request-based billing) in us-west2
Cloud Run functions CPU (Request-based billing) in us-west3
Cloud Run functions CPU (Request-based billing) in us-west4
Cloud Run functions Memory (Request-based billing) in africa-south1
Cloud Run functions Memory (Request-based billing) in asia-east1
Cloud Run functions Memory (Request-based billing) in asia-east2
Cloud Run functions Memory (Request-based billing) in asia-northeast1
Cloud Run functions Memory (Request-based billing) in asia-northeast2
Cloud Run functions Memory (Request-based billing) in asia-northeast3
Cloud Run functions Memory (Request-based billing) in asia-south1
Cloud Run functions Memory (Request-based billing) in asia-south2
Cloud Run functions Memory (Request-based billing) in asia-southeast1
Cloud Run functions Memory (Request-based billing) in asia-southeast2
Cloud Run functions Memory (Request-based billing) in australia-southeast1
Cloud Run functions Memory (Request-based billing) in australia-southeast2
Cloud Run functions Memory (Request-based billing) in europe-central2
Cloud Run functions Memory (Request-based billing) in europe-north1
Cloud Run functions Memory (Request-based billing) in europe-southwest1
Cloud Run functions Memory (Request-based billing) in europe-west1
Cloud Run functions Memory (Request-based billing) in europe-west10
Cloud Run functions Memory (Request-based billing) in europe-west12
Cloud Run functions Memory (Request-based billing) in europe-west2
Cloud Run functions Memory (Request-based billing) in europe-west3
Cloud Run functions Memory (Request-based billing) in europe-west4
Cloud Run functions Memory (Request-based billing) in europe-west6
Cloud Run functions Memory (Request-based billing) in europe-west8
Cloud Run functions Memory (Request-based billing) in europe-west9
Cloud Run functions Memory (Request-based billing) in me-central1
Cloud Run functions Memory (Request-based billing) in me-central2
Cloud Run functions Memory (Request-based billing) in me-west1
Cloud Run functions Memory (Request-based billing) in northamerica-northeast1
Cloud Run functions Memory (Request-based billing) in northamerica-northeast2
Cloud Run functions Memory (Request-based billing) in southamerica-east1
Cloud Run functions Memory (Request-based billing) in southamerica-west1
Cloud Run functions Memory (Request-based billing) in us-central1
Cloud Run functions Memory (Request-based billing) in us-east1
Cloud Run functions Memory (Request-based billing) in us-east4
Cloud Run functions Memory (Request-based billing) in us-east5
Cloud Run functions Memory (Request-based billing) in us-east7
Cloud Run functions Memory (Request-based billing) in us-south1
Cloud Run functions Memory (Request-based billing) in us-west1
Cloud Run functions Memory (Request-based billing) in us-west2
Cloud Run functions Memory (Request-based billing) in us-west3
Cloud Run functions Memory (Request-based billing) in us-west4
Cloud Run functions Min-Instance CPU (Request-based billing) in africa-south1
Cloud Run functions Min-Instance CPU (Request-based billing) in asia-east1
Cloud Run functions Min-Instance CPU (Request-based billing) in asia-east2
Cloud Run functions Min-Instance CPU (Request-based billing) in asia-northeast1
Cloud Run functions Min-Instance CPU (Request-based billing) in asia-northeast2
Cloud Run functions Min-Instance CPU (Request-based billing) in asia-northeast3
Cloud Run functions Min-Instance CPU (Request-based billing) in asia-south1
Cloud Run functions Min-Instance CPU (Request-based billing) in asia-south2
Cloud Run functions Min-Instance CPU (Request-based billing) in asia-southeast1
Cloud Run functions Min-Instance CPU (Request-based billing) in asia-southeast2
Cloud Run functions Min-Instance CPU (Request-based billing) in australia-southeast1
Cloud Run functions Min-Instance CPU (Request-based billing) in australia-southeast2
Cloud Run functions Min-Instance CPU (Request-based billing) in europe-central2
Cloud Run functions Min-Instance CPU (Request-based billing) in europe-north1
Cloud Run functions Min-Instance CPU (Request-based billing) in europe-southwest1
Cloud Run functions Min-Instance CPU (Request-based billing) in europe-west1
Cloud Run functions Min-Instance CPU (Request-based billing) in europe-west10
Cloud Run functions Min-Instance CPU (Request-based billing) in europe-west12
Cloud Run functions Min-Instance CPU (Request-based billing) in europe-west2
Cloud Run functions Min-Instance CPU (Request-based billing) in europe-west3
Cloud Run functions Min-Instance CPU (Request-based billing) in europe-west4
Cloud Run functions Min-Instance CPU (Request-based billing) in europe-west6
Cloud Run functions Min-Instance CPU (Request-based billing) in europe-west8
Cloud Run functions Min-Instance CPU (Request-based billing) in europe-west9
Cloud Run functions Min-Instance CPU (Request-based billing) in me-central1
Cloud Run functions Min-Instance CPU (Request-based billing) in me-central2
Cloud Run functions Min-Instance CPU (Request-based billing) in me-west1
Cloud Run functions Min-Instance CPU (Request-based billing) in northamerica-northeast1
Cloud Run functions Min-Instance CPU (Request-based billing) in northamerica-northeast2
Cloud Run functions Min-Instance CPU (Request-based billing) in southamerica-east1
Cloud Run functions Min-Instance CPU (Request-based billing) in southamerica-west1
Cloud Run functions Min-Instance CPU (Request-based billing) in us-central1
Cloud Run functions Min-Instance CPU (Request-based billing) in us-east1
Cloud Run functions Min-Instance CPU (Request-based billing) in us-east4
Cloud Run functions Min-Instance CPU (Request-based billing) in us-east5
Cloud Run functions Min-Instance CPU (Request-based billing) in us-east7
Cloud Run functions Min-Instance CPU (Request-based billing) in us-south1
Cloud Run functions Min-Instance CPU (Request-based billing) in us-west1
Cloud Run functions Min-Instance CPU (Request-based billing) in us-west2
Cloud Run functions Min-Instance CPU (Request-based billing) in us-west3
Cloud Run functions Min-Instance CPU (Request-based billing) in us-west4
Cloud Run functions Min-Instance Memory (Request-based billing) in africa-south1
Cloud Run functions Min-Instance Memory (Request-based billing) in asia-east1
Cloud Run functions Min-Instance Memory (Request-based billing) in asia-east2
Cloud Run functions Min-Instance Memory (Request-based billing) in asia-northeast1
Cloud Run functions Min-Instance Memory (Request-based billing) in asia-northeast2
Cloud Run functions Min-Instance Memory (Request-based billing) in asia-northeast3
Cloud Run functions Min-Instance Memory (Request-based billing) in asia-south1
Cloud Run functions Min-Instance Memory (Request-based billing) in asia-south2
Cloud Run functions Min-Instance Memory (Request-based billing) in asia-southeast1
Cloud Run functions Min-Instance Memory (Request-based billing) in asia-southeast2
Cloud Run functions Min-Instance Memory (Request-based billing) in australia-southeast1
Cloud Run functions Min-Instance Memory (Request-based billing) in australia-southeast2
Cloud Run functions Min-Instance Memory (Request-based billing) in europe-central2
Cloud Run functions Min-Instance Memory (Request-based billing) in europe-north1
Cloud Run functions Min-Instance Memory (Request-based billing) in europe-southwest1
Cloud Run functions Min-Instance Memory (Request-based billing) in europe-west1
Cloud Run functions Min-Instance Memory (Request-based billing) in europe-west10
Cloud Run functions Min-Instance Memory (Request-based billing) in europe-west12
Cloud Run functions Min-Instance Memory (Request-based billing) in europe-west2
Cloud Run functions Min-Instance Memory (Request-based billing) in europe-west3
Cloud Run functions Min-Instance Memory (Request-based billing) in europe-west4
Cloud Run functions Min-Instance Memory (Request-based billing) in europe-west6
Cloud Run functions Min-Instance Memory (Request-based billing) in europe-west8
Cloud Run functions Min-Instance Memory (Request-based billing) in europe-west9
Cloud Run functions Min-Instance Memory (Request-based billing) in me-central1
Cloud Run functions Min-Instance Memory (Request-based billing) in me-central2
Cloud Run functions Min-Instance Memory (Request-based billing) in me-west1
Cloud Run functions Min-Instance Memory (Request-based billing) in northamerica-northeast1
Cloud Run functions Min-Instance Memory (Request-based billing) in northamerica-northeast2
Cloud Run functions Min-Instance Memory (Request-based billing) in southamerica-east1
Cloud Run functions Min-Instance Memory (Request-based billing) in southamerica-west1
Cloud Run functions Min-Instance Memory (Request-based billing) in us-central1
Cloud Run functions Min-Instance Memory (Request-based billing) in us-east1
Cloud Run functions Min-Instance Memory (Request-based billing) in us-east4
Cloud Run functions Min-Instance Memory (Request-based billing) in us-east5
Cloud Run functions Min-Instance Memory (Request-based billing) in us-east7
Cloud Run functions Min-Instance Memory (Request-based billing) in us-south1
Cloud Run functions Min-Instance Memory (Request-based billing) in us-west1
Cloud Run functions Min-Instance Memory (Request-based billing) in us-west2
Cloud Run functions Min-Instance Memory (Request-based billing) in us-west3
Cloud Run functions Min-Instance Memory (Request-based billing) in us-west4
```

**Requests/Invocations with arm suffix:** none found

**Finding:** arm SKUs absent

## Decision

Document x86-only status and schedule quarterly recheck. GCP's Cloud Billing Catalog API (as of 2026-04-23) exposes no architecture-differentiated SKUs for either Cloud Run or Cloud Run Functions — all CPU/memory line items are architecture-agnostic. Both services bill identically regardless of whether a workload runs on x86 or arm hardware. Implement `--architecture` flag as a UI-only filter (returning x86_64 and arm64 at the same price) rather than distinct catalog rows, and re-verify in ~Q3 2026.
