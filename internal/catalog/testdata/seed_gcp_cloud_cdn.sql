-- Minimal seed for LookupCDN unit tests against GCP Cloud CDN data.
-- Schema definition mirrors seed_aws_m3a3.sql.

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
  ('source_url','https://cloudbilling.googleapis.com/v1/services/E505-1604-58F8/skus'),
  ('shard','gcp-cloud-cdn'),
  ('allowed_kinds','["network.cdn"]'),
  ('serving_providers','["gcp"]');

-- terms_hash for (on_demand,'','','','','') = 4b0dbf5efbd01c9e5f0a3f2e39227bc3

INSERT INTO skus VALUES
  ('CLOUD-CDN-EGRESS-NORTH-AMERICA', 'gcp','cloud-cdn','network.cdn','standard','us-east1',         'global','4b0dbf5efbd01c9e5f0a3f2e39227bc3'),
  ('CLOUD-CDN-EGRESS-EUROPE',        'gcp','cloud-cdn','network.cdn','standard','europe-west1',      'global','4b0dbf5efbd01c9e5f0a3f2e39227bc3'),
  ('CLOUD-CDN-REQUESTS-GLOBAL',      'gcp','cloud-cdn','network.cdn','standard','global',            'global','4b0dbf5efbd01c9e5f0a3f2e39227bc3');

INSERT INTO terms (sku_id, commitment, tenancy, os) VALUES
  ('CLOUD-CDN-EGRESS-NORTH-AMERICA', 'on_demand','',''),
  ('CLOUD-CDN-EGRESS-EUROPE',        'on_demand','',''),
  ('CLOUD-CDN-REQUESTS-GLOBAL',      'on_demand','','');

INSERT INTO resource_attrs (sku_id, extra) VALUES
  ('CLOUD-CDN-EGRESS-NORTH-AMERICA', '{"mode":"edge-egress","sku":"cloud-cdn-standard"}'),
  ('CLOUD-CDN-EGRESS-EUROPE',        '{"mode":"edge-egress","sku":"cloud-cdn-standard"}'),
  ('CLOUD-CDN-REQUESTS-GLOBAL',      '{"mode":"request","sku":"cloud-cdn-standard"}');

INSERT INTO prices VALUES
  ('CLOUD-CDN-EGRESS-NORTH-AMERICA', 'egress','0',    '10TB', 0.08,    'gb'),
  ('CLOUD-CDN-EGRESS-NORTH-AMERICA', 'egress','10TB', '150TB',0.055,   'gb'),
  ('CLOUD-CDN-EGRESS-NORTH-AMERICA', 'egress','150TB','',     0.03,    'gb'),
  ('CLOUD-CDN-EGRESS-EUROPE',        'egress','0',    '10TB', 0.08,    'gb'),
  ('CLOUD-CDN-EGRESS-EUROPE',        'egress','10TB', '150TB',0.055,   'gb'),
  ('CLOUD-CDN-EGRESS-EUROPE',        'egress','150TB','',     0.03,    'gb'),
  ('CLOUD-CDN-REQUESTS-GLOBAL',      'request','0',  '',     0.00000075,'request');
