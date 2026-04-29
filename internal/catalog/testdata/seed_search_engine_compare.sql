-- Minimal search.engine seed for QuerySearchEngine compare tests.

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

INSERT INTO skus VALUES
  ('os-mc-large-use1',  'aws', 'opensearch', 'search.engine', 'r6g.large.search',  'us-east-1', 'us-east', 'hash-mc-large'),
  ('os-mc-xl-use1',     'aws', 'opensearch', 'search.engine', 'r6g.xlarge.search', 'us-east-1', 'us-east', 'hash-mc-xl'),
  ('os-sv-use1',        'aws', 'opensearch', 'search.engine', 'opensearch-serverless', 'us-east-1', 'us-east', 'hash-sv');

INSERT INTO terms (sku_id, commitment, tenancy, os) VALUES
  ('os-mc-large-use1', 'on_demand', 'shared', 'managed-cluster'),
  ('os-mc-xl-use1',    'on_demand', 'shared', 'managed-cluster'),
  ('os-sv-use1',       'on_demand', 'shared', 'serverless');

INSERT INTO resource_attrs (sku_id, vcpu, memory_gb, extra) VALUES
  ('os-mc-large-use1', 2,    16.0, '{"mode":"managed-cluster","instance_family":"r6g"}'),
  ('os-mc-xl-use1',    4,    32.0, '{"mode":"managed-cluster","instance_family":"r6g"}'),
  ('os-sv-use1',       NULL, NULL, '{"mode":"serverless"}');

INSERT INTO prices VALUES
  ('os-mc-large-use1', 'instance', '', 0.166, 'hour'),
  ('os-mc-xl-use1',    'instance', '', 0.332, 'hour'),
  ('os-sv-use1',       'ocu',      '', 0.24,  'hour');
