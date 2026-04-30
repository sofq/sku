-- Minimal seed for LookupVM / LookupDBRelational unit tests against Azure data.
-- Schema definition is copy-pasted from seed_aws.sql to keep this file standalone.

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
  ('source_url','https://prices.azure.com/'),
  ('shard','azure-vm'),
  ('allowed_kinds','["compute.vm","db.relational"]'),
  ('serving_providers','["azure"]');

INSERT INTO skus VALUES
  ('azure-vm-d2v3-eastus-linux',         'azure','vm','compute.vm','Standard_D2_v3','eastus','us-east','e12226d4ec2288df9ff43f2757a24e9c'),
  ('azure-vm-d2v3-eastus-windows',       'azure','vm','compute.vm','Standard_D2_v3','eastus','us-east','05effc17b6884c2b673191790f8c3a52'),
  ('azure-sql-gp-gen5-2-eastus-single',  'azure','sql','db.relational','GP_Gen5_2','eastus','us-east','45a3d6870dc4dac5c63343dd4edafb31'),
  ('azure-sql-bc-gen5-2-eastus-mi',      'azure','sql','db.relational','BC_Gen5_2','eastus','us-east','bb5e12242d40d27454cac617965b231a');

INSERT INTO terms VALUES
  ('azure-vm-d2v3-eastus-linux',         'on_demand','shared','linux','','',''),
  ('azure-vm-d2v3-eastus-windows',       'on_demand','shared','windows','','',''),
  ('azure-sql-gp-gen5-2-eastus-single',  'on_demand','azure-sql','single-az','','',''),
  ('azure-sql-bc-gen5-2-eastus-mi',      'on_demand','azure-sql','managed-instance','','','');

INSERT INTO resource_attrs (sku_id, vcpu, memory_gb, architecture, extra) VALUES
  ('azure-vm-d2v3-eastus-linux',        2, 8.0,  'x86_64','{}'),
  ('azure-vm-d2v3-eastus-windows',      2, 8.0,  'x86_64','{}'),
  ('azure-sql-gp-gen5-2-eastus-single', NULL, NULL, NULL, '{"deployment_option":"single-az"}'),
  ('azure-sql-bc-gen5-2-eastus-mi',     NULL, NULL, NULL, '{"deployment_option":"managed-instance"}');

INSERT INTO prices VALUES
  ('azure-vm-d2v3-eastus-linux',        'compute','','',0.096,'hrs'),
  ('azure-vm-d2v3-eastus-windows',      'compute','','',0.144,'hrs'),
  ('azure-sql-gp-gen5-2-eastus-single', 'compute','','',0.252,'hrs'),
  ('azure-sql-bc-gen5-2-eastus-mi',     'compute','','',1.058,'hrs');
