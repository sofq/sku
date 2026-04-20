-- m3b.2 Cobra-test seed: two rows per azure shard (blob, functions, disks).
-- DDL matches spec §5 one-to-one; all secondary tables CASCADE on skus.sku_id.

CREATE TABLE skus (
  sku_id            TEXT NOT NULL PRIMARY KEY,
  provider          TEXT NOT NULL,
  service           TEXT NOT NULL,
  kind              TEXT NOT NULL,
  resource_name     TEXT NOT NULL,
  region            TEXT NOT NULL,
  region_normalized TEXT NOT NULL,
  terms_hash        TEXT NOT NULL
) WITHOUT ROWID;

CREATE TABLE resource_attrs (
  sku_id            TEXT NOT NULL PRIMARY KEY REFERENCES skus(sku_id) ON DELETE CASCADE,
  vcpu              INTEGER,
  memory_gb         REAL,
  storage_gb        REAL,
  gpu_count         INTEGER,
  gpu_model         TEXT,
  architecture      TEXT,
  context_length    INTEGER,
  max_output_tokens INTEGER,
  modality          TEXT,
  capabilities      TEXT,
  quantization      TEXT,
  durability_nines  INTEGER,
  availability_tier TEXT,
  extra             TEXT
) WITHOUT ROWID;

CREATE TABLE terms (
  sku_id         TEXT NOT NULL PRIMARY KEY REFERENCES skus(sku_id) ON DELETE CASCADE,
  commitment     TEXT NOT NULL,
  tenancy        TEXT NOT NULL DEFAULT '',
  os             TEXT NOT NULL DEFAULT '',
  support_tier   TEXT,
  upfront        TEXT,
  payment_option TEXT
) WITHOUT ROWID;

CREATE TABLE prices (
  sku_id    TEXT NOT NULL REFERENCES skus(sku_id) ON DELETE CASCADE,
  dimension TEXT NOT NULL,
  tier      TEXT NOT NULL DEFAULT '',
  amount    REAL NOT NULL,
  unit      TEXT NOT NULL,
  PRIMARY KEY (sku_id, dimension, tier)
) WITHOUT ROWID;

CREATE TABLE health (
  sku_id                    TEXT NOT NULL PRIMARY KEY REFERENCES skus(sku_id) ON DELETE CASCADE,
  uptime_30d                REAL,
  latency_p50_ms            INTEGER,
  latency_p95_ms            INTEGER,
  throughput_tokens_per_sec REAL,
  observed_at               INTEGER
) WITHOUT ROWID;

CREATE TABLE metadata (key TEXT PRIMARY KEY, value TEXT);

INSERT INTO metadata VALUES
  ('schema_version','1'),
  ('catalog_version','2026.04.18'),
  ('currency','USD'),
  ('generated_at','2026-04-18T00:00:00Z');

-- azure-blob: hot + archive LRS in eastus.
INSERT INTO skus VALUES
  ('azure-blob-hot-eastus',     'azure','blob','storage.object','hot',    'eastus','us-east','4b0dbf5efbd01c9e5f0a3f2e39227bc3'),
  ('azure-blob-archive-eastus', 'azure','blob','storage.object','archive','eastus','us-east','4b0dbf5efbd01c9e5f0a3f2e39227bc3');
INSERT INTO resource_attrs(sku_id, durability_nines, availability_tier, extra) VALUES
  ('azure-blob-hot-eastus',     11, 'standard', '{"redundancy":"lrs"}'),
  ('azure-blob-archive-eastus', 11, 'archive',  '{"redundancy":"lrs"}');
INSERT INTO terms(sku_id, commitment, tenancy, os) VALUES
  ('azure-blob-hot-eastus',     'on_demand','',''),
  ('azure-blob-archive-eastus', 'on_demand','','');
INSERT INTO prices VALUES
  ('azure-blob-hot-eastus',     'storage',   '', 0.0184, 'gb-mo'),
  ('azure-blob-hot-eastus',     'read-ops',  '', 0.0004, 'requests'),
  ('azure-blob-hot-eastus',     'write-ops', '', 0.0050, 'requests'),
  ('azure-blob-archive-eastus', 'storage',   '', 0.0020, 'gb-mo'),
  ('azure-blob-archive-eastus', 'read-ops',  '', 0.0005, 'requests'),
  ('azure-blob-archive-eastus', 'write-ops', '', 0.00001,'requests');

-- azure-functions: x86_64 in eastus + westeurope.
INSERT INTO skus VALUES
  ('azure-fn-x86_64-eastus', 'azure','functions','compute.function','x86_64','eastus',    'us-east','4b0dbf5efbd01c9e5f0a3f2e39227bc3'),
  ('azure-fn-x86_64-weu',    'azure','functions','compute.function','x86_64','westeurope','eu-west','4b0dbf5efbd01c9e5f0a3f2e39227bc3');
INSERT INTO resource_attrs(sku_id, architecture, extra) VALUES
  ('azure-fn-x86_64-eastus','x86_64','{"plan":"consumption"}'),
  ('azure-fn-x86_64-weu',   'x86_64','{"plan":"consumption"}');
INSERT INTO terms(sku_id, commitment, tenancy, os) VALUES
  ('azure-fn-x86_64-eastus','on_demand','',''),
  ('azure-fn-x86_64-weu',   'on_demand','','');
INSERT INTO prices VALUES
  ('azure-fn-x86_64-eastus','executions','', 0.00000020, 'requests'),
  ('azure-fn-x86_64-eastus','duration',  '', 0.000016,   'gb-seconds'),
  ('azure-fn-x86_64-weu',   'executions','', 0.00000022, 'requests'),
  ('azure-fn-x86_64-weu',   'duration',  '', 0.000018,   'gb-seconds');

-- azure-disks: standard-ssd + premium-ssd in eastus.
INSERT INTO skus VALUES
  ('azure-disk-std-ssd-eastus',  'azure','disks','storage.block','standard-ssd','eastus','us-east','4b0dbf5efbd01c9e5f0a3f2e39227bc3'),
  ('azure-disk-prem-ssd-eastus', 'azure','disks','storage.block','premium-ssd', 'eastus','us-east','4b0dbf5efbd01c9e5f0a3f2e39227bc3');
INSERT INTO resource_attrs(sku_id, extra) VALUES
  ('azure-disk-std-ssd-eastus', '{"redundancy":"lrs"}'),
  ('azure-disk-prem-ssd-eastus','{"redundancy":"lrs"}');
INSERT INTO terms(sku_id, commitment, tenancy, os) VALUES
  ('azure-disk-std-ssd-eastus', 'on_demand','',''),
  ('azure-disk-prem-ssd-eastus','on_demand','','');
INSERT INTO prices VALUES
  ('azure-disk-std-ssd-eastus', 'storage','', 4.80,  'month'),
  ('azure-disk-prem-ssd-eastus','storage','', 19.71, 'month');
