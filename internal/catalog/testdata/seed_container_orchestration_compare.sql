-- Minimal container.orchestration seed for QueryContainerOrchestration compare tests.

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
  ('catalog_version','2026.04.25'),
  ('currency','USD'),
  ('generated_at','2026-04-25T00:00:00Z'),
  ('source_url','https://pricing.us-east-1.amazonaws.com/'),
  ('shard','aws-eks'),
  ('allowed_kinds','["container.orchestration"]'),
  ('serving_providers','["aws"]');

INSERT INTO skus VALUES
  ('eks-std-use1',  'aws',   'eks', 'container.orchestration', 'eks-standard', 'us-east-1', 'us-east', 'hash-eks-std'),
  ('aks-prem-eus',  'azure', 'aks', 'container.orchestration', 'aks-premium',  'eastus',    'us-east', 'hash-aks-prem'),
  ('gke-std-use1',  'gcp',   'gke', 'container.orchestration', 'gke-standard', 'us-east1',  'us-east', 'hash-gke-std'),
  ('gke-auto-use1', 'gcp',   'gke', 'container.orchestration', 'gke-autopilot','us-east1',  'us-east', 'hash-gke-auto');

INSERT INTO terms (sku_id, commitment, tenancy, os) VALUES
  ('eks-std-use1',  'on_demand', 'kubernetes', 'standard'),
  ('aks-prem-eus',  'on_demand', 'kubernetes', 'premium'),
  ('gke-std-use1',  'on_demand', 'kubernetes', 'standard'),
  ('gke-auto-use1', 'on_demand', 'kubernetes', 'autopilot');

INSERT INTO resource_attrs (sku_id, extra) VALUES
  ('eks-std-use1',  '{"mode":"control-plane","tier":"standard"}'),
  ('aks-prem-eus',  '{"mode":"control-plane","tier":"premium"}'),
  ('gke-std-use1',  '{"mode":"control-plane","tier":"standard"}'),
  ('gke-auto-use1', '{"mode":"autopilot","tier":"autopilot"}');

INSERT INTO prices VALUES
  ('eks-std-use1',  'cluster', '', '',0.10,  'hour'),
  ('aks-prem-eus',  'cluster', '', '',0.15,  'hour'),
  ('gke-std-use1',  'cluster', '', '',0.10,  'hour'),
  ('gke-auto-use1', 'vcpu',    '', '',0.048, 'hour');
