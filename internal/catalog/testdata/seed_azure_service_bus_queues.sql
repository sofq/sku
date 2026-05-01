-- Minimal messaging.queue seed for Azure Service Bus Queues LookupMessagingQueue unit tests.

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
  ('source_url','https://prices.azure.com/api/retail/prices'),
  ('shard','azure-service-bus-queues'),
  ('allowed_kinds','["messaging.queue"]'),
  ('serving_providers','["azure"]');

-- terms_hash for standard (on_demand,'','standard','','',''):
-- computed from apply_kind_defaults("messaging.queue", {..., "os":"standard"})
-- terms_hash for premium (on_demand,'','premium','','',''):
-- actual hashes from ingest output:
--   standard: a4b7ae059c59d6a9d0917ba8f655f681
--   premium:  4cd9a63daf2a46bc1d23c67cb722580d
INSERT INTO skus VALUES
  ('SB-STD-eastus',  'azure','service-bus-queues','messaging.queue','standard','eastus','us-east','a4b7ae059c59d6a9d0917ba8f655f681'),
  ('SB-PREM-eastus', 'azure','service-bus-queues','messaging.queue','premium', 'eastus','us-east','4cd9a63daf2a46bc1d23c67cb722580d');

INSERT INTO terms (sku_id, commitment, tenancy, os) VALUES
  ('SB-STD-eastus',  'on_demand','','standard'),
  ('SB-PREM-eastus', 'on_demand','','premium');

INSERT INTO resource_attrs (sku_id, extra) VALUES
  ('SB-STD-eastus',  '{"mode":"standard"}'),
  ('SB-PREM-eastus', '{"mode":"premium"}');

INSERT INTO prices VALUES
  ('SB-STD-eastus',  'request','0',    '13M',   0.0,   'request'),
  ('SB-STD-eastus',  'request','13M',  '100M',  0.8,   'request'),
  ('SB-STD-eastus',  'request','100M', '2500M', 0.5,   'request'),
  ('SB-STD-eastus',  'request','2500M','',      0.2,   'request'),
  ('SB-PREM-eastus', 'mu_hour','0',    '',      0.928, 'hr');
