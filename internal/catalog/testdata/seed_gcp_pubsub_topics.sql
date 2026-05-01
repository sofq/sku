-- Minimal messaging.topic seed for GCP Pub/Sub LookupMessagingTopic unit tests.

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
  ('source_url','https://cloudbilling.googleapis.com/v1/'),
  ('shard','gcp-pubsub-topics'),
  ('allowed_kinds','["messaging.topic"]'),
  ('serving_providers','["gcp"]');

-- terms_hash for (on_demand,'','','','','') = 4b0dbf5efbd01c9e5f0a3f2e39227bc3
INSERT INTO skus VALUES
  ('PUBSUB-BASIC-GLOBAL','gcp','pubsub-topics','messaging.topic','throughput','global','global','4b0dbf5efbd01c9e5f0a3f2e39227bc3');

INSERT INTO terms (sku_id, commitment, tenancy, os) VALUES
  ('PUBSUB-BASIC-GLOBAL','on_demand','','');

INSERT INTO resource_attrs (sku_id, extra) VALUES
  ('PUBSUB-BASIC-GLOBAL','{"mode":"throughput"}');

INSERT INTO prices VALUES
  ('PUBSUB-BASIC-GLOBAL','throughput','0','',0.0390625,'gb-mo');
