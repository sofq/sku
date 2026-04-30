-- Minimal api.gateway seed for QueryAPIGateway compare tests.
-- Three rows: rest (1M-req), http (1M-req), and provisioned (hr).
-- Two units ("1M-req" and "hr") to test the mixed-unit warning.
-- Used by internal/compare/kinds/api_gateway_test.go.

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
  ('catalog_version','2026.04.30'),
  ('currency','USD'),
  ('generated_at','2026-04-30T00:00:00Z'),
  ('source_url','https://pricing.us-east-1.amazonaws.com/'),
  ('shard','aws-apigateway'),
  ('allowed_kinds','["api.gateway"]'),
  ('serving_providers','["aws"]');

INSERT INTO skus VALUES
  ('apigw-rest-use1',        'aws','apigateway','api.gateway','rest',        'us-east-1','us-east','hash-rest'),
  ('apigw-http-use1',        'aws','apigateway','api.gateway','http',        'us-east-1','us-east','hash-http'),
  ('apigw-provisioned-use1', 'aws','apigateway','api.gateway','provisioned', 'us-east-1','us-east','hash-prov');

INSERT INTO terms (sku_id, commitment, tenancy, os) VALUES
  ('apigw-rest-use1',        'on_demand','',''),
  ('apigw-http-use1',        'on_demand','',''),
  ('apigw-provisioned-use1', 'on_demand','','');

INSERT INTO resource_attrs (sku_id, extra) VALUES
  ('apigw-rest-use1',        '{"mode":"rest"}'),
  ('apigw-http-use1',        '{"mode":"http"}'),
  ('apigw-provisioned-use1', '{"mode":"provisioned"}');

INSERT INTO prices VALUES
  ('apigw-rest-use1',        'request','','',0.0000035,'1M-req'),
  ('apigw-http-use1',        'request','','',0.0000010,'1M-req'),
  ('apigw-provisioned-use1', 'cache',  '','',0.020,    'hr');
