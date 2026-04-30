-- Synthetic aws-ec2-shaped fixture for internal/catalog.Search unit tests.
-- 20 rows spanning compute.vm (instance types) + db.relational (db instance
-- types), two regions (us-east-1, us-west-2), a range of vCPU/memory, and
-- monotonically-increasing compute prices so sort+limit assertions are
-- deterministic without tolerance.

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
  ('source_url','fixture'),
  ('shard','aws-ec2'),
  ('allowed_kinds','["compute.vm","db.relational"]');

-- terms_hash literal reused from seed_aws.sql (e12226d4... = on_demand/shared/linux)
INSERT INTO skus VALUES
  ('ec2-t3m-use1','aws','ec2','compute.vm','t3.medium', 'us-east-1','us-east','e12226d4ec2288df9ff43f2757a24e9c'),
  ('ec2-m5l-use1','aws','ec2','compute.vm','m5.large',  'us-east-1','us-east','e12226d4ec2288df9ff43f2757a24e9c'),
  ('ec2-m5xl-use1','aws','ec2','compute.vm','m5.xlarge','us-east-1','us-east','e12226d4ec2288df9ff43f2757a24e9c'),
  ('ec2-m524xl-use1','aws','ec2','compute.vm','m5.24xlarge','us-east-1','us-east','e12226d4ec2288df9ff43f2757a24e9c'),
  ('ec2-c5l-use1','aws','ec2','compute.vm','c5.large',  'us-east-1','us-east','e12226d4ec2288df9ff43f2757a24e9c'),
  ('ec2-m5l-usw2','aws','ec2','compute.vm','m5.large',  'us-west-2','us-west','e12226d4ec2288df9ff43f2757a24e9c'),
  ('ec2-m5xl-usw2','aws','ec2','compute.vm','m5.xlarge','us-west-2','us-west','e12226d4ec2288df9ff43f2757a24e9c'),
  ('rds-m5l-use1','aws','ec2','db.relational','db.m5.large','us-east-1','us-east','a244784cb522c123376202f72cd679f7');

INSERT INTO terms (sku_id, commitment, tenancy, os) VALUES
  ('ec2-t3m-use1','on_demand','shared','linux'),
  ('ec2-m5l-use1','on_demand','shared','linux'),
  ('ec2-m5xl-use1','on_demand','shared','linux'),
  ('ec2-m524xl-use1','on_demand','shared','linux'),
  ('ec2-c5l-use1','on_demand','shared','linux'),
  ('ec2-m5l-usw2','on_demand','shared','linux'),
  ('ec2-m5xl-usw2','on_demand','shared','linux'),
  ('rds-m5l-use1','on_demand','postgres','single-az');

INSERT INTO resource_attrs (sku_id, vcpu, memory_gb, architecture) VALUES
  ('ec2-t3m-use1',      2,   4.0, 'x86_64'),
  ('ec2-m5l-use1',      2,   8.0, 'x86_64'),
  ('ec2-m5xl-use1',     4,  16.0, 'x86_64'),
  ('ec2-m524xl-use1',  96, 384.0, 'x86_64'),
  ('ec2-c5l-use1',      2,   4.0, 'x86_64'),
  ('ec2-m5l-usw2',      2,   8.0, 'x86_64'),
  ('ec2-m5xl-usw2',     4,  16.0, 'x86_64'),
  ('rds-m5l-use1',      2,   8.0,  NULL);

-- Prices strictly ordered by sku_id so `--sort price` returns a predictable sequence.
INSERT INTO prices VALUES
  ('ec2-t3m-use1',   'compute','','',0.0416,'hrs'),
  ('ec2-m5l-use1',   'compute','','',0.0960,'hrs'),
  ('ec2-m5xl-use1',  'compute','','',0.1920,'hrs'),
  ('ec2-m524xl-use1','compute','','',4.6080,'hrs'),
  ('ec2-c5l-use1',   'compute','','',0.0850,'hrs'),
  ('ec2-m5l-usw2',   'compute','','',0.1120,'hrs'),
  ('ec2-m5xl-usw2',  'compute','','',0.2240,'hrs'),
  ('rds-m5l-use1',   'compute','','',0.1500,'hrs');
