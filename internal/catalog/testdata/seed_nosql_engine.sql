-- Minimal db.nosql seed for LookupNoSQLDB Engine-narrowing unit test.
-- Two rows with different tenancy values (dynamodb vs cosmos-sql) to verify
-- that passing Terms.Tenancy="dynamodb" returns only the dynamodb row.

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
  ('catalog_version','2026.04.18'),
  ('currency','USD'),
  ('generated_at','2026-04-18T00:00:00Z'),
  ('source_url','https://pricing.us-east-1.amazonaws.com/'),
  ('shard','nosql-engine-test'),
  ('allowed_kinds','["db.nosql"]'),
  ('serving_providers','["aws","azure"]');

-- terms_hash for (on_demand,'dynamodb','','','','') = 7ae215b340a9c51602f9fb64558b747e
-- terms_hash for (on_demand,'cosmos-sql','','','','') = 5f967967d0e698680a526e56a11b30f7
INSERT INTO skus VALUES
  ('ddb-std-use1',    'aws',  'dynamodb', 'db.nosql','standard','us-east-1','us-east','7ae215b340a9c51602f9fb64558b747e'),
  ('cosmos-std-eus1', 'azure','cosmosdb', 'db.nosql','standard','eastus',   'us-east','5f967967d0e698680a526e56a11b30f7');

INSERT INTO terms (sku_id, commitment, tenancy, os) VALUES
  ('ddb-std-use1',    'on_demand','dynamodb', ''),
  ('cosmos-std-eus1', 'on_demand','cosmos-sql','');

INSERT INTO resource_attrs (sku_id, extra) VALUES
  ('ddb-std-use1',    '{"engine":"dynamodb"}'),
  ('cosmos-std-eus1', '{"engine":"cosmos-sql"}');

INSERT INTO prices VALUES
  ('ddb-std-use1',    'storage','', 0.25,  'gb-mo'),
  ('cosmos-std-eus1', 'storage','', 0.25,  'gb-mo');
