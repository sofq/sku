-- Minimal seed for LookupDBRelational unit tests against GCP Spanner data.
-- Schema definition mirrors seed_gcp.sql.

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
  ('catalog_version','2026.04.25'),
  ('currency','USD'),
  ('generated_at','2026-04-25T00:00:00Z'),
  ('source_url','https://cloudbilling.googleapis.com/v1/services/'),
  ('shard','gcp-spanner'),
  ('allowed_kinds','["db.relational"]'),
  ('serving_providers','["gcp"]');

-- terms_hash values from pipeline/normalize/terms.terms_hash for the six-tuple:
--   on_demand|spanner-standard|||| -> c46106e81653aee1a78d2d00d9346e50
--   on_demand|spanner-enterprise|||| -> d6ba59063ecbadfcc5a8e22f45e8e150
--   on_demand|spanner-enterprise-plus|||| -> 7d9b21e7332c56064748a11e823981c5

INSERT INTO skus VALUES
  ('SPANNER-STD-USEAST1',  'gcp','spanner','db.relational','spanner-standard',       'us-east1','us-east','c46106e81653aee1a78d2d00d9346e50'),
  ('SPANNER-ENT-USEAST1',  'gcp','spanner','db.relational','spanner-enterprise',      'us-east1','us-east','d6ba59063ecbadfcc5a8e22f45e8e150'),
  ('SPANNER-EP-USEAST1',   'gcp','spanner','db.relational','spanner-enterprise-plus', 'us-east1','us-east','7d9b21e7332c56064748a11e823981c5');

INSERT INTO terms VALUES
  ('SPANNER-STD-USEAST1',  'on_demand','spanner-standard',       '','','',''),
  ('SPANNER-ENT-USEAST1',  'on_demand','spanner-enterprise',      '','','',''),
  ('SPANNER-EP-USEAST1',   'on_demand','spanner-enterprise-plus', '','','','');

INSERT INTO resource_attrs (sku_id, extra) VALUES
  ('SPANNER-STD-USEAST1',  '{"edition":"standard","pu_hour_usd":0.00009,"node_hour_usd":0.09}'),
  ('SPANNER-ENT-USEAST1',  '{"edition":"enterprise","pu_hour_usd":0.00014,"node_hour_usd":0.14}'),
  ('SPANNER-EP-USEAST1',   '{"edition":"enterprise-plus","pu_hour_usd":0.00028,"node_hour_usd":0.28}');

INSERT INTO prices VALUES
  ('SPANNER-STD-USEAST1',  'compute','','',0.00009,'pu-hour'),
  ('SPANNER-ENT-USEAST1',  'compute','','',0.00014,'pu-hour'),
  ('SPANNER-EP-USEAST1',   'compute','','',0.00028,'pu-hour');
