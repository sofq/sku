-- Minimal seed for LookupVM / LookupDBRelational unit tests against GCP data.
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
  ('source_url','https://cloudbilling.googleapis.com/v1/services/'),
  ('shard','gcp-gce'),
  ('allowed_kinds','["compute.vm","db.relational"]'),
  ('serving_providers','["gcp"]');

-- terms_hash values MUST match pipeline/normalize/terms.terms_hash for the
-- (commitment, tenancy, os, "", "", "") six-tuple.
--   on_demand|shared|linux                -> e12226d4ec2288df9ff43f2757a24e9c
--   on_demand|cloud-sql-postgres|zonal    -> e8c0e27d4471c2949daebb4eaf9badba
--   on_demand|cloud-sql-postgres|regional -> d98a3e4d3f46943254c9c629c31a0be1
--   on_demand|cloud-sql-mysql|zonal       -> 4dfb151b7e37417947fd77df3903b706

INSERT INTO skus VALUES
  ('gcp-gce-n1std2-use1-linux',        'gcp','gce','compute.vm','n1-standard-2','us-east1','us-east','e12226d4ec2288df9ff43f2757a24e9c'),
  ('gcp-gce-e2std2-use1-linux',        'gcp','gce','compute.vm','e2-standard-2','us-east1','us-east','e12226d4ec2288df9ff43f2757a24e9c'),
  ('gcp-sql-pg-c2-use1-zonal',         'gcp','cloud-sql','db.relational','db-custom-2-7680','us-east1','us-east','e8c0e27d4471c2949daebb4eaf9badba'),
  ('gcp-sql-pg-c2-use1-regional',      'gcp','cloud-sql','db.relational','db-custom-2-7680','us-east1','us-east','d98a3e4d3f46943254c9c629c31a0be1'),
  ('gcp-sql-my-c2-use1-zonal',         'gcp','cloud-sql','db.relational','db-custom-2-7680','us-east1','us-east','4dfb151b7e37417947fd77df3903b706');

INSERT INTO terms VALUES
  ('gcp-gce-n1std2-use1-linux',        'on_demand','shared','linux','','',''),
  ('gcp-gce-e2std2-use1-linux',        'on_demand','shared','linux','','',''),
  ('gcp-sql-pg-c2-use1-zonal',         'on_demand','cloud-sql-postgres','zonal','','',''),
  ('gcp-sql-pg-c2-use1-regional',      'on_demand','cloud-sql-postgres','regional','','',''),
  ('gcp-sql-my-c2-use1-zonal',         'on_demand','cloud-sql-mysql','zonal','','','');

INSERT INTO resource_attrs (sku_id, vcpu, memory_gb, architecture, extra) VALUES
  ('gcp-gce-n1std2-use1-linux',        2, 7.5,  'x86_64','{}'),
  ('gcp-gce-e2std2-use1-linux',        2, 8.0,  'x86_64','{}'),
  ('gcp-sql-pg-c2-use1-zonal',         NULL, NULL, NULL,'{"engine":"cloud-sql-postgres","deployment_option":"zonal"}'),
  ('gcp-sql-pg-c2-use1-regional',      NULL, NULL, NULL,'{"engine":"cloud-sql-postgres","deployment_option":"regional"}'),
  ('gcp-sql-my-c2-use1-zonal',         NULL, NULL, NULL,'{"engine":"cloud-sql-mysql","deployment_option":"zonal"}');

INSERT INTO prices VALUES
  ('gcp-gce-n1std2-use1-linux',        'compute','',0.095,'hrs'),
  ('gcp-gce-e2std2-use1-linux',        'compute','',0.067,'hrs'),
  ('gcp-sql-pg-c2-use1-zonal',         'compute','',0.115,'hrs'),
  ('gcp-sql-pg-c2-use1-regional',      'compute','',0.230,'hrs'),
  ('gcp-sql-my-c2-use1-zonal',         'compute','',0.103,'hrs');
