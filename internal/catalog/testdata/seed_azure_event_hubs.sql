-- Minimal messaging.queue seed for azure_event_hubs LookupMessagingQueue unit tests.

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
  ('shard','azure-event-hubs'),
  ('allowed_kinds','["messaging.queue"]'),
  ('serving_providers','["azure"]');

-- terms_hash for (on_demand,'','event-hubs-standard','','','') = b99455fedf1fb330c96d18aa30c73f41
-- terms_hash for (on_demand,'','event-hubs-premium', '','','') = 1eabb24b9539d7007d680f3a710013cb
INSERT INTO skus VALUES
  ('EH-STD-eastus',  'azure','event-hubs','messaging.queue','standard','eastus','us-east','b99455fedf1fb330c96d18aa30c73f41'),
  ('EH-PREM-eastus', 'azure','event-hubs','messaging.queue','premium', 'eastus','us-east','1eabb24b9539d7007d680f3a710013cb');

INSERT INTO terms (sku_id, commitment, tenancy, os) VALUES
  ('EH-STD-eastus',  'on_demand','','event-hubs-standard'),
  ('EH-PREM-eastus', 'on_demand','','event-hubs-premium');

INSERT INTO resource_attrs (sku_id, extra) VALUES
  ('EH-STD-eastus',  '{"mode":"standard"}'),
  ('EH-PREM-eastus', '{"mode":"premium"}');

INSERT INTO prices VALUES
  ('EH-STD-eastus',  'tu_hour', '0','',0.030, 'hr'),
  ('EH-STD-eastus',  'event',   '0','',0.028, 'request'),
  ('EH-PREM-eastus', 'ppu_hour','0','',1.063, 'hr');
