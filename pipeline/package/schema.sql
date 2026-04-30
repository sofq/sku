-- sku shard schema v1. Spec §5. Keep in sync across ingest + client.

PRAGMA foreign_keys = ON;

CREATE TABLE skus (
  sku_id             TEXT    NOT NULL PRIMARY KEY,
  provider           TEXT    NOT NULL,
  service            TEXT    NOT NULL,
  kind               TEXT    NOT NULL,
  resource_name      TEXT    NOT NULL,
  region             TEXT    NOT NULL,
  region_normalized  TEXT    NOT NULL,
  terms_hash         TEXT    NOT NULL
) WITHOUT ROWID;

CREATE TABLE resource_attrs (
  sku_id             TEXT    NOT NULL PRIMARY KEY REFERENCES skus(sku_id) ON DELETE CASCADE,
  vcpu               INTEGER,
  memory_gb          REAL,
  storage_gb         REAL,
  gpu_count          INTEGER,
  gpu_model          TEXT,
  architecture       TEXT,
  context_length     INTEGER,
  max_output_tokens  INTEGER,
  modality           TEXT,
  capabilities       TEXT,
  quantization       TEXT,
  durability_nines   INTEGER,
  availability_tier  TEXT,
  extra              TEXT
) WITHOUT ROWID;

CREATE TABLE terms (
  sku_id             TEXT    NOT NULL PRIMARY KEY REFERENCES skus(sku_id) ON DELETE CASCADE,
  commitment         TEXT    NOT NULL,
  tenancy            TEXT    NOT NULL DEFAULT '',
  os                 TEXT    NOT NULL DEFAULT '',
  support_tier       TEXT,
  upfront            TEXT,
  payment_option     TEXT
) WITHOUT ROWID;

CREATE TABLE prices (
  sku_id     TEXT NOT NULL REFERENCES skus(sku_id) ON DELETE CASCADE,
  dimension  TEXT NOT NULL,
  tier       TEXT NOT NULL DEFAULT '',
  tier_upper TEXT NOT NULL DEFAULT '',
  amount     REAL NOT NULL,
  unit       TEXT NOT NULL,
  PRIMARY KEY (sku_id, dimension, tier, tier_upper)
) WITHOUT ROWID;

CREATE TABLE health (
  sku_id                     TEXT    NOT NULL PRIMARY KEY REFERENCES skus(sku_id) ON DELETE CASCADE,
  uptime_30d                 REAL,
  latency_p50_ms             INTEGER,
  latency_p95_ms             INTEGER,
  throughput_tokens_per_sec  REAL,
  observed_at                INTEGER
) WITHOUT ROWID;

CREATE TABLE metadata (
  key    TEXT PRIMARY KEY,
  value  TEXT
);

CREATE INDEX idx_skus_lookup
  ON skus (resource_name, region, terms_hash);
CREATE INDEX idx_resource_compute
  ON resource_attrs (vcpu, memory_gb) WHERE vcpu IS NOT NULL;
CREATE INDEX idx_resource_llm
  ON resource_attrs (context_length) WHERE context_length IS NOT NULL;
CREATE INDEX idx_skus_region
  ON skus (region_normalized, kind);
CREATE INDEX idx_prices_by_dim
  ON prices (dimension, amount);
CREATE INDEX idx_terms_commitment
  ON terms (commitment, tenancy, os);
