-- Minimal seed for LookupStorageObject / LookupServerlessFunction /
-- LookupStorageBlock unit tests. Matches the canonical shard schema in
-- pipeline/package/schema.sql.

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
  ('shard','aws-s3'),
  ('allowed_kinds','["compute.function","storage.block","storage.object"]'),
  ('serving_providers','["aws"]');

-- terms_hash for (on_demand,'','','','','') per pipeline/normalize/terms.terms_hash.
INSERT INTO skus VALUES
  ('s3-standard-use1',    'aws','s3','storage.object',      'standard',   'us-east-1','us-east','4b0dbf5efbd01c9e5f0a3f2e39227bc3'),
  ('s3-standard-ia-use1', 'aws','s3','storage.object',      'standard-ia','us-east-1','us-east','4b0dbf5efbd01c9e5f0a3f2e39227bc3'),
  ('lambda-x86_64-use1',  'aws','lambda','compute.function','x86_64',     'us-east-1','us-east','4b0dbf5efbd01c9e5f0a3f2e39227bc3'),
  ('lambda-arm64-use1',   'aws','lambda','compute.function','arm64',      'us-east-1','us-east','4b0dbf5efbd01c9e5f0a3f2e39227bc3'),
  ('ebs-gp3-use1',        'aws','ebs','storage.block',      'gp3',        'us-east-1','us-east','4b0dbf5efbd01c9e5f0a3f2e39227bc3'),
  ('ebs-io2-use1',        'aws','ebs','storage.block',      'io2',        'us-east-1','us-east','4b0dbf5efbd01c9e5f0a3f2e39227bc3');

INSERT INTO terms (sku_id, commitment, tenancy, os) VALUES
  ('s3-standard-use1',    'on_demand','',''),
  ('s3-standard-ia-use1', 'on_demand','',''),
  ('lambda-x86_64-use1',  'on_demand','',''),
  ('lambda-arm64-use1',   'on_demand','',''),
  ('ebs-gp3-use1',        'on_demand','',''),
  ('ebs-io2-use1',        'on_demand','','');

INSERT INTO resource_attrs (sku_id, durability_nines, availability_tier, architecture, extra) VALUES
  ('s3-standard-use1',    11,'standard',   NULL, '{"volume_type":"Standard"}'),
  ('s3-standard-ia-use1', 11,'infrequent', NULL, '{"volume_type":"Standard - Infrequent Access"}'),
  ('lambda-x86_64-use1',  NULL, NULL,      'x86_64','{}'),
  ('lambda-arm64-use1',   NULL, NULL,      'arm64', '{}'),
  ('ebs-gp3-use1',        NULL, NULL,      NULL,    '{"volume_type":"gp3"}'),
  ('ebs-io2-use1',        NULL, NULL,      NULL,    '{"volume_type":"io2"}');

INSERT INTO prices VALUES
  ('s3-standard-use1',    'storage',      '', 0.023,        'gb-mo'),
  ('s3-standard-use1',    'requests-put', '', 0.000005,     'requests'),
  ('s3-standard-use1',    'requests-get', '', 0.0000004,    'requests'),
  ('s3-standard-ia-use1', 'storage',      '', 0.0125,       'gb-mo'),
  ('s3-standard-ia-use1', 'requests-put', '', 0.00001,      'requests'),
  ('s3-standard-ia-use1', 'requests-get', '', 0.000001,     'requests'),
  ('lambda-x86_64-use1',  'requests',     '', 0.0000002,    'requests'),
  ('lambda-x86_64-use1',  'duration',     '', 0.0000166667, 'second'),
  ('lambda-arm64-use1',   'requests',     '', 0.0000002,    'requests'),
  ('lambda-arm64-use1',   'duration',     '', 0.0000133334, 'second'),
  ('ebs-gp3-use1',        'storage',      '', 0.08,         'gb-mo'),
  ('ebs-io2-use1',        'storage',      '', 0.125,        'gb-mo');
