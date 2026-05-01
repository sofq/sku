-- Minimal network.cdn seed for QueryNetworkCDN compare tests.
-- Two rows: edge-egress (GB pricing) and origin-shield (GB pricing).
-- Used by internal/compare/kinds/network_cdn_test.go.

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
  ('shard','aws-cloudfront'),
  ('allowed_kinds','["network.cdn"]'),
  ('serving_providers','["aws"]');

INSERT INTO skus VALUES
  ('cf-egress-use1',       'aws','cloudfront','network.cdn','standard','us-east-1','us-east','hash-egress'),
  ('cf-shield-use1',       'aws','cloudfront','network.cdn','standard','us-east-1','us-east','hash-shield');

INSERT INTO terms (sku_id, commitment, tenancy, os) VALUES
  ('cf-egress-use1',       'on_demand','',''),
  ('cf-shield-use1',       'on_demand','','');

INSERT INTO resource_attrs (sku_id, extra) VALUES
  ('cf-egress-use1',       '{"mode":"edge-egress","tier":"PriceClass_All"}'),
  ('cf-shield-use1',       '{"mode":"origin-shield","tier":"PriceClass_All"}');

INSERT INTO prices VALUES
  ('cf-egress-use1',       'egress','','',0.085,'gb'),
  ('cf-shield-use1',       'shield','','',0.010,'gb');
