-- Minimal cache.kv seed for LookupCacheKV unit tests.

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
  PRIMARY KEY (sku_id, dimension, tier, tier_upper)
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
  ('generated_at','2026-04-18T00:00:00Z'),
  ('source_url','https://pricing.us-east-1.amazonaws.com/'),
  ('shard','aws-elasticache'),
  ('allowed_kinds','["cache.kv"]'),
  ('serving_providers','["aws"]');

-- terms_hash for (on_demand,'redis','','','','') = da12688b4e48d4af5da1db4ca7cf2ac0
-- terms_hash for (on_demand,'memcached','','','','') = 995ad7c1a8c8aa97878c4c4e4334668c
INSERT INTO skus VALUES
  ('ec-r6g-large-redis-use1',    'aws','elasticache','cache.kv','cache.r6g.large','us-east-1','us-east','da12688b4e48d4af5da1db4ca7cf2ac0'),
  ('ec-r6g-large-memcd-use1',    'aws','elasticache','cache.kv','cache.r6g.large','us-east-1','us-east','995ad7c1a8c8aa97878c4c4e4334668c');

INSERT INTO terms (sku_id, commitment, tenancy, os) VALUES
  ('ec-r6g-large-redis-use1',    'on_demand','redis',''),
  ('ec-r6g-large-memcd-use1',    'on_demand','memcached','');

INSERT INTO resource_attrs (sku_id, memory_gb, extra) VALUES
  ('ec-r6g-large-redis-use1',    13.07, '{"engine":"redis"}'),
  ('ec-r6g-large-memcd-use1',    13.07, '{"engine":"memcached"}');

INSERT INTO prices VALUES
  ('ec-r6g-large-redis-use1',    'compute','','',0.156,'hour'),
  ('ec-r6g-large-memcd-use1',    'compute','','',0.133,'hour');
