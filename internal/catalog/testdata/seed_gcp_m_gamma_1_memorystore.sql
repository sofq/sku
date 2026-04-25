-- Minimal seed for LookupCacheKV unit tests against GCP Memorystore data.
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
  ('catalog_version','2026.04.25'),
  ('currency','USD'),
  ('generated_at','2026-04-25T00:00:00Z'),
  ('source_url','https://cloudbilling.googleapis.com/v1/services/'),
  ('shard','gcp-memorystore'),
  ('allowed_kinds','["cache.kv"]'),
  ('serving_providers','["gcp"]');

-- terms_hash values from pipeline/normalize/terms.terms_hash for the six-tuple:
--   on_demand|redis|||| -> da12688b4e48d4af5da1db4ca7cf2ac0
--   on_demand|memcached|||| -> 995ad7c1a8c8aa97878c4c4e4334668c

INSERT INTO skus VALUES
  ('MEMORYSTORE-REDIS-STD-16GB-USEAST1',  'gcp','memorystore','cache.kv','memorystore-redis-standard-16gb','us-east1','us-east','da12688b4e48d4af5da1db4ca7cf2ac0'),
  ('MEMORYSTORE-REDIS-BASIC-5GB-USEAST1', 'gcp','memorystore','cache.kv','memorystore-redis-basic-5gb',    'us-east1','us-east','da12688b4e48d4af5da1db4ca7cf2ac0');

INSERT INTO terms VALUES
  ('MEMORYSTORE-REDIS-STD-16GB-USEAST1',  'on_demand','redis','','','',''),
  ('MEMORYSTORE-REDIS-BASIC-5GB-USEAST1', 'on_demand','redis','','','','');

INSERT INTO resource_attrs (sku_id, memory_gb, extra) VALUES
  ('MEMORYSTORE-REDIS-STD-16GB-USEAST1',  16.0, '{"engine":"redis","tier":"standard"}'),
  ('MEMORYSTORE-REDIS-BASIC-5GB-USEAST1',  5.0, '{"engine":"redis","tier":"basic"}');

INSERT INTO prices VALUES
  ('MEMORYSTORE-REDIS-STD-16GB-USEAST1',  'compute','',0.21,'hour'),
  ('MEMORYSTORE-REDIS-BASIC-5GB-USEAST1', 'compute','',0.049,'hour');
