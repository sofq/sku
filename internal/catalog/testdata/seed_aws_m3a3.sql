-- Minimal DynamoDB + CloudFront rows for unit tests. Mirrors the production
-- shard schema. Self-contained: carries DDL so catalog.BuildFromSQL can apply
-- it against a fresh SQLite file.

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
  ('shard','aws-dynamodb-cloudfront'),
  ('allowed_kinds','["db.nosql","network.cdn"]'),
  ('serving_providers','["aws"]');

-- terms_hash for (on_demand,'','','','','') — same precomputed hex as seed_aws_m3a2.sql.
INSERT INTO skus VALUES
  ('ddb-std-use1',    'aws','dynamodb','db.nosql',   'standard',   'us-east-1','us-east','4b0dbf5efbd01c9e5f0a3f2e39227bc3'),
  ('ddb-std-ia-use1', 'aws','dynamodb','db.nosql',   'standard-ia','us-east-1','us-east','4b0dbf5efbd01c9e5f0a3f2e39227bc3'),
  ('cf-eu-dto',       'aws','cloudfront','network.cdn','standard','eu-west-1','eu-west','4b0dbf5efbd01c9e5f0a3f2e39227bc3'),
  ('cf-use-dto',      'aws','cloudfront','network.cdn','standard','us-east-1','us-east','4b0dbf5efbd01c9e5f0a3f2e39227bc3');

INSERT INTO terms (sku_id, commitment, tenancy, os) VALUES
  ('ddb-std-use1',    'on_demand','',''),
  ('ddb-std-ia-use1', 'on_demand','',''),
  ('cf-eu-dto',       'on_demand','',''),
  ('cf-use-dto',      'on_demand','','');

INSERT INTO resource_attrs (sku_id, durability_nines, availability_tier, extra) VALUES
  ('ddb-std-use1',    11, 'standard',   '{"table_class":"standard"}'),
  ('ddb-std-ia-use1', 11, 'infrequent', '{"table_class":"standard-ia"}'),
  ('cf-eu-dto',       NULL, NULL,       '{"tier":"PriceClass_All"}'),
  ('cf-use-dto',      NULL, NULL,       '{"tier":"PriceClass_100"}');

INSERT INTO prices VALUES
  ('ddb-std-use1',    'storage',             '', '',0.25,         'gb-mo'),
  ('ddb-std-use1',    'read_request_units',  '', '',0.000000125,  'readrequestunits'),
  ('ddb-std-use1',    'write_request_units', '', '',0.000000625,  'writerequestunits'),
  ('ddb-std-ia-use1', 'storage',             '', '',0.1125,       'gb-mo'),
  ('ddb-std-ia-use1', 'read_request_units',  '', '',0.00000025,   'readrequestunits'),
  ('ddb-std-ia-use1', 'write_request_units', '', '',0.00000125,   'writerequestunits'),
  ('cf-eu-dto',       'data_transfer_out',   '', '',0.085,        'gb'),
  ('cf-use-dto',      'data_transfer_out',   '', '',0.085,        'gb');
