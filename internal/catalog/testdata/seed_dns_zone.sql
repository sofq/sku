-- Minimal dns.zone seed for LookupDNSZone unit tests.

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
  ('shard','aws-route53'),
  ('allowed_kinds','["dns.zone"]'),
  ('serving_providers','["aws"]');

-- terms_hash for (on_demand,'','','','','') = 4b0dbf5efbd01c9e5f0a3f2e39227bc3
INSERT INTO skus VALUES
  ('r53-public-global',  'aws','route53','dns.zone','public', 'global','global','4b0dbf5efbd01c9e5f0a3f2e39227bc3'),
  ('r53-private-global', 'aws','route53','dns.zone','private','global','global','4b0dbf5efbd01c9e5f0a3f2e39227bc3');

INSERT INTO terms (sku_id, commitment, tenancy, os) VALUES
  ('r53-public-global',  'on_demand','',''),
  ('r53-private-global', 'on_demand','','');

INSERT INTO resource_attrs (sku_id, extra) VALUES
  ('r53-public-global',  '{"zone_type":"public"}'),
  ('r53-private-global', '{"zone_type":"private"}');

INSERT INTO prices VALUES
  ('r53-public-global',  'hosted_zone','', '',0.50, 'mo'),
  ('r53-public-global',  'query',      '', '',0.0000004, 'query'),
  ('r53-private-global', 'hosted_zone','', '',0.50, 'mo');
