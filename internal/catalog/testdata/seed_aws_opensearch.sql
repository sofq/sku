-- Minimal search.engine seed for LookupSearchEngine unit tests.

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
  ('catalog_version','2026.04.29'),
  ('currency','USD'),
  ('generated_at','2026-04-29T00:00:00Z'),
  ('source_url','https://pricing.us-east-1.amazonaws.com/'),
  ('shard','aws-opensearch'),
  ('allowed_kinds','["search.engine"]'),
  ('serving_providers','["aws"]');

-- terms_hash for (on_demand,'shared','managed-cluster','','','') = 0df65f0c4bf29db48ae1f9ace2721286
-- terms_hash for (on_demand,'shared','serverless','','','')       = 0df3838b557f5488799e9fe1c139756c
INSERT INTO skus VALUES
  ('os-mc-r6g-large-use1',    'aws', 'opensearch', 'search.engine', 'r6g.large.search',      'us-east-1', 'us-east', '0df65f0c4bf29db48ae1f9ace2721286'),
  ('os-serverless-use1',      'aws', 'opensearch', 'search.engine', 'opensearch-serverless', 'us-east-1', 'us-east', '0df3838b557f5488799e9fe1c139756c');

INSERT INTO terms (sku_id, commitment, tenancy, os) VALUES
  ('os-mc-r6g-large-use1', 'on_demand', 'shared', 'managed-cluster'),
  ('os-serverless-use1',   'on_demand', 'shared', 'serverless');

INSERT INTO resource_attrs (sku_id, vcpu, memory_gb, extra) VALUES
  ('os-mc-r6g-large-use1', 2, 16.0, '{"mode":"managed-cluster","instance_family":"r6g"}'),
  ('os-serverless-use1',   NULL, NULL, '{"mode":"serverless"}');

INSERT INTO prices VALUES
  ('os-mc-r6g-large-use1', 'instance', '', 0.166, 'hour'),
  ('os-serverless-use1',   'ocu',      '', 0.24,  'hour'),
  ('os-serverless-use1',   'storage',  '', 0.024, 'gb-month');
