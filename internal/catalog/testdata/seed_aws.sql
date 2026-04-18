-- Minimal seed for LookupVM / LookupDBRelational unit tests.
-- Matches the canonical shard schema in pipeline/package/schema.sql.

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
  ('catalog_version','2026.04.18'),
  ('currency','USD'),
  ('generated_at','2026-04-18T00:00:00Z'),
  ('source_url','https://pricing.us-east-1.amazonaws.com/'),
  ('shard','aws-ec2'),
  ('allowed_kinds','["compute.vm","db.relational"]'),
  ('serving_providers','["aws"]');

-- terms_hash values computed from pipeline/normalize/terms.terms_hash so they
-- match what schema.TermsHash produces on the Go side.
INSERT INTO skus VALUES
  ('ec2-m5l-use1-linux-shared','aws','ec2','compute.vm','m5.large','us-east-1','us-east','e12226d4ec2288df9ff43f2757a24e9c'),
  ('ec2-m5l-usw2-linux-shared','aws','ec2','compute.vm','m5.large','us-west-2','us-west','e12226d4ec2288df9ff43f2757a24e9c'),
  ('rds-dbm5l-use1-pg-single', 'aws','rds','db.relational','db.m5.large','us-east-1','us-east','a244784cb522c123376202f72cd679f7'),
  ('rds-dbm5l-use1-pg-multi',  'aws','rds','db.relational','db.m5.large','us-east-1','us-east','a7744247d2e7ddea87ec125fd46dc7a8');

INSERT INTO terms (sku_id, commitment, tenancy, os) VALUES
  ('ec2-m5l-use1-linux-shared','on_demand','shared','linux'),
  ('ec2-m5l-usw2-linux-shared','on_demand','shared','linux'),
  ('rds-dbm5l-use1-pg-single', 'on_demand','postgres','single-az'),
  ('rds-dbm5l-use1-pg-multi',  'on_demand','postgres','multi-az');

INSERT INTO resource_attrs (sku_id, vcpu, memory_gb, architecture, extra) VALUES
  ('ec2-m5l-use1-linux-shared',2,8.0,'x86_64','{}'),
  ('ec2-m5l-usw2-linux-shared',2,8.0,'x86_64','{}'),
  ('rds-dbm5l-use1-pg-single', 2,8.0,NULL,'{"engine":"postgres","deployment_option":"single-az"}'),
  ('rds-dbm5l-use1-pg-multi',  2,8.0,NULL,'{"engine":"postgres","deployment_option":"multi-az"}');

INSERT INTO prices VALUES
  ('ec2-m5l-use1-linux-shared','compute','',0.096,'hrs'),
  ('ec2-m5l-usw2-linux-shared','compute','',0.112,'hrs'),
  ('rds-dbm5l-use1-pg-single', 'compute','',0.150,'hrs'),
  ('rds-dbm5l-use1-pg-multi',  'compute','',0.300,'hrs');
