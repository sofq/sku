-- Minimal seed for LookupStorageObject / LookupServerlessFunction unit tests
-- against GCP Cloud Storage + Cloud Run + Cloud Functions data.
-- Schema definition mirrors seed_gcp.sql.

PRAGMA foreign_keys = ON;

CREATE TABLE skus (
  sku_id TEXT NOT NULL PRIMARY KEY,
  provider TEXT NOT NULL, service TEXT NOT NULL, kind TEXT NOT NULL,
  resource_name TEXT NOT NULL, region TEXT NOT NULL,
  region_normalized TEXT NOT NULL, terms_hash TEXT NOT NULL
) WITHOUT ROWID;

CREATE TABLE resource_attrs (
  sku_id TEXT NOT NULL PRIMARY KEY REFERENCES skus(sku_id) ON DELETE CASCADE,
  vcpu INTEGER, memory_gb REAL, storage_gb REAL,
  gpu_count INTEGER, gpu_model TEXT, architecture TEXT,
  context_length INTEGER, max_output_tokens INTEGER,
  modality TEXT, capabilities TEXT, quantization TEXT,
  durability_nines INTEGER, availability_tier TEXT, extra TEXT
) WITHOUT ROWID;

CREATE TABLE terms (
  sku_id TEXT NOT NULL PRIMARY KEY REFERENCES skus(sku_id) ON DELETE CASCADE,
  commitment TEXT NOT NULL, tenancy TEXT NOT NULL DEFAULT '',
  os TEXT NOT NULL DEFAULT '',
  support_tier TEXT, upfront TEXT, payment_option TEXT
) WITHOUT ROWID;

CREATE TABLE prices (
  sku_id TEXT NOT NULL REFERENCES skus(sku_id) ON DELETE CASCADE,
  dimension TEXT NOT NULL, tier TEXT NOT NULL DEFAULT '',
  tier_upper TEXT NOT NULL DEFAULT '',
  amount REAL NOT NULL, unit TEXT NOT NULL,
  PRIMARY KEY (sku_id, dimension, tier)
) WITHOUT ROWID;

CREATE TABLE health (
  sku_id TEXT NOT NULL PRIMARY KEY REFERENCES skus(sku_id) ON DELETE CASCADE,
  uptime_30d REAL, latency_p50_ms INTEGER, latency_p95_ms INTEGER,
  throughput_tokens_per_sec REAL, observed_at INTEGER
) WITHOUT ROWID;

CREATE TABLE metadata (key TEXT PRIMARY KEY, value TEXT);

INSERT INTO metadata VALUES
  ('schema_version','1'),
  ('catalog_version','2026.04.18'),
  ('currency','USD'),
  ('generated_at','2026-04-18T00:00:00Z');

-- terms_hash = pipeline/normalize/terms.terms_hash for
-- (on_demand, '', '', '', '', '') -> 4b0dbf5efbd01c9e5f0a3f2e39227bc3.

-- Cloud Storage: one row per (class, region).
INSERT INTO skus VALUES
  ('GCS-STD-USEAST1','gcp','gcs','storage.object','standard','us-east1','us-east','4b0dbf5efbd01c9e5f0a3f2e39227bc3'),
  ('GCS-NL-USEAST1', 'gcp','gcs','storage.object','nearline','us-east1','us-east','4b0dbf5efbd01c9e5f0a3f2e39227bc3'),
  ('GCS-CL-EUWEST1', 'gcp','gcs','storage.object','coldline','europe-west1','eu-west','4b0dbf5efbd01c9e5f0a3f2e39227bc3'),
  ('GCS-AR-EUWEST1', 'gcp','gcs','storage.object','archive','europe-west1','eu-west','4b0dbf5efbd01c9e5f0a3f2e39227bc3');
INSERT INTO terms(sku_id, commitment, tenancy, os) VALUES
  ('GCS-STD-USEAST1','on_demand','',''),
  ('GCS-NL-USEAST1', 'on_demand','',''),
  ('GCS-CL-EUWEST1', 'on_demand','',''),
  ('GCS-AR-EUWEST1', 'on_demand','','');
INSERT INTO resource_attrs(sku_id, durability_nines, availability_tier, extra) VALUES
  ('GCS-STD-USEAST1', 11, 'standard',   '{}'),
  ('GCS-NL-USEAST1',  11, 'infrequent', '{}'),
  ('GCS-CL-EUWEST1',  11, 'cold',       '{}'),
  ('GCS-AR-EUWEST1',  11, 'archive',    '{}');
INSERT INTO prices VALUES
  ('GCS-STD-USEAST1','storage','','',0.02,'gb-mo'),
  ('GCS-STD-USEAST1','read-ops','','',5e-7,'requests'),
  ('GCS-STD-USEAST1','write-ops','','',4e-6,'requests'),
  ('GCS-NL-USEAST1','storage','','',0.01,'gb-mo'),
  ('GCS-NL-USEAST1','read-ops','','',1e-6,'requests'),
  ('GCS-NL-USEAST1','write-ops','','',1e-5,'requests'),
  ('GCS-CL-EUWEST1','storage','','',0.007,'gb-mo'),
  ('GCS-CL-EUWEST1','read-ops','','',1e-5,'requests'),
  ('GCS-CL-EUWEST1','write-ops','','',5e-5,'requests'),
  ('GCS-AR-EUWEST1','storage','','',0.0012,'gb-mo'),
  ('GCS-AR-EUWEST1','read-ops','','',5e-5,'requests'),
  ('GCS-AR-EUWEST1','write-ops','','',5e-4,'requests');

-- Cloud Run: one row per region. resource_name is the architecture slug so
-- LookupServerlessFunction (which filters resource_name = Architecture) can
-- point-lookup; the service column distinguishes run vs functions.
INSERT INTO skus VALUES
  ('RUN-USEAST1','gcp','run','compute.function','x86_64','us-east1','us-east','4b0dbf5efbd01c9e5f0a3f2e39227bc3'),
  ('RUN-EUWEST1','gcp','run','compute.function','x86_64','europe-west1','eu-west','4b0dbf5efbd01c9e5f0a3f2e39227bc3');
INSERT INTO terms(sku_id, commitment, tenancy, os) VALUES
  ('RUN-USEAST1','on_demand','',''),
  ('RUN-EUWEST1','on_demand','','');
INSERT INTO resource_attrs(sku_id, architecture, extra) VALUES
  ('RUN-USEAST1','x86_64','{"resource_group":"CloudRunV2"}'),
  ('RUN-EUWEST1','x86_64','{"resource_group":"CloudRunV2"}');
INSERT INTO prices VALUES
  ('RUN-USEAST1','cpu-second','','',2.4e-5,'s'),
  ('RUN-USEAST1','memory-gb-second','','',2.5e-6,'gb-s'),
  ('RUN-USEAST1','requests','','',4e-7,'requests'),
  ('RUN-EUWEST1','cpu-second','','',2.6e-5,'s'),
  ('RUN-EUWEST1','memory-gb-second','','',2.7e-6,'gb-s'),
  ('RUN-EUWEST1','requests','','',4.4e-7,'requests');

-- Cloud Functions: one row per region.
INSERT INTO skus VALUES
  ('FN-USEAST1','gcp','functions','compute.function','x86_64','us-east1','us-east','4b0dbf5efbd01c9e5f0a3f2e39227bc3'),
  ('FN-EUWEST1','gcp','functions','compute.function','x86_64','europe-west1','eu-west','4b0dbf5efbd01c9e5f0a3f2e39227bc3');
INSERT INTO terms(sku_id, commitment, tenancy, os) VALUES
  ('FN-USEAST1','on_demand','',''),
  ('FN-EUWEST1','on_demand','','');
INSERT INTO resource_attrs(sku_id, architecture, extra) VALUES
  ('FN-USEAST1','x86_64','{"resource_group":"CloudFunctionsV2"}'),
  ('FN-EUWEST1','x86_64','{"resource_group":"CloudFunctionsV2"}');
INSERT INTO prices VALUES
  ('FN-USEAST1','cpu-second','','',2.4e-5,'s'),
  ('FN-USEAST1','memory-gb-second','','',2.5e-6,'gb-s'),
  ('FN-USEAST1','requests','','',4e-7,'requests'),
  ('FN-EUWEST1','cpu-second','','',2.6e-5,'s'),
  ('FN-EUWEST1','memory-gb-second','','',2.7e-6,'gb-s'),
  ('FN-EUWEST1','requests','','',4.4e-7,'requests');
