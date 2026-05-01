-- Minimal db.nosql seed for QueryDBNoSQL compare tests.
-- Three rows: dynamodb (provisioned), dynamodb (serverless), cosmos-sql (provisioned).
-- Engine is encoded in terms.tenancy; mode in extra.mode.
-- Used by internal/compare/kinds/db_nosql_test.go.

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
  ('catalog_version','2026.04.30'),
  ('currency','USD'),
  ('generated_at','2026-04-30T00:00:00Z'),
  ('source_url','https://pricing.us-east-1.amazonaws.com/'),
  ('shard','nosql-compare-test'),
  ('allowed_kinds','["db.nosql"]'),
  ('serving_providers','["aws","azure"]');

-- tenancy encodes engine: "dynamodb" / "cosmos-sql"
INSERT INTO skus VALUES
  ('ddb-prov-use1',    'aws',  'dynamodb', 'db.nosql','standard','us-east-1','us-east','hash-ddb-prov'),
  ('ddb-sv-use1',      'aws',  'dynamodb', 'db.nosql','standard','us-east-1','us-east','hash-ddb-sv'),
  ('cosmos-prov-eus1', 'azure','cosmosdb', 'db.nosql','standard','eastus',   'us-east','hash-cosmos-prov');

INSERT INTO terms (sku_id, commitment, tenancy, os) VALUES
  ('ddb-prov-use1',    'on_demand','dynamodb',   ''),
  ('ddb-sv-use1',      'on_demand','dynamodb',   ''),
  ('cosmos-prov-eus1', 'on_demand','cosmos-sql', '');

INSERT INTO resource_attrs (sku_id, extra) VALUES
  ('ddb-prov-use1',    '{"mode":"provisioned","engine":"dynamodb"}'),
  ('ddb-sv-use1',      '{"mode":"serverless","engine":"dynamodb"}'),
  ('cosmos-prov-eus1', '{"mode":"provisioned","engine":"cosmos-sql"}');

INSERT INTO prices VALUES
  ('ddb-prov-use1',    'storage','','',0.25, 'gb-mo'),
  ('ddb-sv-use1',      'storage','','',0.25, 'gb-mo'),
  ('cosmos-prov-eus1', 'storage','','',0.25, 'gb-mo');
