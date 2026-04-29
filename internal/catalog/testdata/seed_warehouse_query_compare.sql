-- Minimal warehouse.query seed for QueryWarehouseQuery compare tests.

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
  ('source_url','https://cloudbilling.googleapis.com/'),
  ('shard','gcp-bigquery'),
  ('allowed_kinds','["warehouse.query"]'),
  ('serving_providers','["gcp"]');

INSERT INTO skus VALUES
  ('bq-od-bq-us',      'gcp', 'bigquery', 'warehouse.query', 'on-demand',          'bq-us', 'bq-us', 'hash-od'),
  ('bq-cap-ent-bq-us', 'gcp', 'bigquery', 'warehouse.query', 'capacity-standard',  'bq-us', 'bq-us', 'hash-cap-ent'),
  ('bq-cap-ep-bq-us',  'gcp', 'bigquery', 'warehouse.query', 'capacity-enterprise','bq-us', 'bq-us', 'hash-cap-ep'),
  ('bq-st-act-bq-us',  'gcp', 'bigquery', 'warehouse.query', 'storage-active',     'bq-us', 'bq-us', 'hash-st-act'),
  ('bq-st-lt-bq-us',   'gcp', 'bigquery', 'warehouse.query', 'storage-long-term',  'bq-us', 'bq-us', 'hash-st-lt');

INSERT INTO terms (sku_id, commitment, tenancy, os, support_tier) VALUES
  ('bq-od-bq-us',      'on_demand', 'shared', 'on-demand', ''),
  ('bq-cap-ent-bq-us', 'on_demand', 'shared', 'on-demand', 'enterprise'),
  ('bq-cap-ep-bq-us',  'on_demand', 'shared', 'on-demand', 'enterprise-plus'),
  ('bq-st-act-bq-us',  'on_demand', 'shared', 'on-demand', 'storage-active'),
  ('bq-st-lt-bq-us',   'on_demand', 'shared', 'on-demand', 'storage-long-term');

INSERT INTO resource_attrs (sku_id, extra) VALUES
  ('bq-od-bq-us',      '{"mode":"on-demand"}'),
  ('bq-cap-ent-bq-us', '{"mode":"capacity","edition":"enterprise"}'),
  ('bq-cap-ep-bq-us',  '{"mode":"capacity","edition":"enterprise-plus"}'),
  ('bq-st-act-bq-us',  '{"mode":"storage","storage_tier":"active"}'),
  ('bq-st-lt-bq-us',   '{"mode":"storage","storage_tier":"long-term"}');

INSERT INTO prices VALUES
  ('bq-od-bq-us',      'query',   '', 5.0,  'tb'),
  ('bq-cap-ent-bq-us', 'slot',    '', 0.04, 'slot-hour'),
  ('bq-cap-ep-bq-us',  'slot',    '', 0.06, 'slot-hour'),
  ('bq-st-act-bq-us',  'storage', '', 0.02, 'gb-month'),
  ('bq-st-lt-bq-us',   'storage', '', 0.01, 'gb-month');
