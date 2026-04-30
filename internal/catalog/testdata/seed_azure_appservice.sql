-- Minimal paas.app seed for LookupPaasApp unit tests.

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
  ('catalog_version','2026.04.29'),
  ('currency','USD'),
  ('generated_at','2026-04-29T00:00:00Z'),
  ('source_url','https://prices.azure.com/api/retail/prices'),
  ('shard','azure-appservice'),
  ('allowed_kinds','["paas.app"]'),
  ('serving_providers','["azure"]');

-- terms_hash for (on_demand,'dedicated','linux','premiumv3','','')   = 07f73ab9585e131d84d49e89db144803
-- terms_hash for (on_demand,'dedicated','windows','premiumv3','','') = a3cb4e403622da15fb4e1003d2c24f66
-- terms_hash for (on_demand,'dedicated','linux','free','','')        = 84aad2c4b6808440a9617cff5253309a
INSERT INTO skus VALUES
  ('as-p1v3-linux-eastus',   'azure', 'appservice', 'paas.app', 'P1v3', 'eastus', 'us-east', '07f73ab9585e131d84d49e89db144803'),
  ('as-p1v3-win-eastus',     'azure', 'appservice', 'paas.app', 'P1v3', 'eastus', 'us-east', 'a3cb4e403622da15fb4e1003d2c24f66'),
  ('as-f1-linux-eastus',     'azure', 'appservice', 'paas.app', 'F1',   'eastus', 'us-east', '84aad2c4b6808440a9617cff5253309a');

INSERT INTO terms (sku_id, commitment, tenancy, os, support_tier) VALUES
  ('as-p1v3-linux-eastus', 'on_demand', 'dedicated', 'linux',   'premiumv3'),
  ('as-p1v3-win-eastus',   'on_demand', 'dedicated', 'windows', 'premiumv3'),
  ('as-f1-linux-eastus',   'on_demand', 'dedicated', 'linux',   'free');

INSERT INTO resource_attrs (sku_id, vcpu, memory_gb) VALUES
  ('as-p1v3-linux-eastus', 1, 2.0),
  ('as-p1v3-win-eastus',   1, 2.0),
  ('as-f1-linux-eastus',   1, 1.0);

INSERT INTO prices VALUES
  ('as-p1v3-linux-eastus', 'instance', '', '',0.072, 'hour'),
  ('as-p1v3-win-eastus',   'instance', '', '',0.095, 'hour'),
  ('as-f1-linux-eastus',   'instance', '', '',0.0,   'hour');
