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
  ('source_url','https://openrouter.ai/api/v1/models'),
  ('row_count','3'),
  ('allowed_kinds','["llm.text"]'),
  ('serving_providers','["anthropic","aws-bedrock","openrouter"]');

-- Three rows: Anthropic first-party, AWS Bedrock, and the aggregated openrouter row
INSERT INTO skus VALUES
  ('anthropic/claude-opus-4.6::anthropic::default','anthropic','llm','llm.text','anthropic/claude-opus-4.6','','','ee2303ad38b3e0b0e4f01bfbb1bcba8f'),
  ('anthropic/claude-opus-4.6::aws-bedrock::default','aws-bedrock','llm','llm.text','anthropic/claude-opus-4.6','','','ee2303ad38b3e0b0e4f01bfbb1bcba8f'),
  ('anthropic/claude-opus-4.6::openrouter::default','openrouter','llm','llm.text','anthropic/claude-opus-4.6','','','ee2303ad38b3e0b0e4f01bfbb1bcba8f');

INSERT INTO resource_attrs (sku_id, context_length, max_output_tokens, modality, capabilities, quantization) VALUES
  ('anthropic/claude-opus-4.6::anthropic::default', 200000, 64000, '["text"]', '["tools"]', NULL),
  ('anthropic/claude-opus-4.6::aws-bedrock::default', 200000, 64000, '["text"]', '["tools"]', NULL),
  ('anthropic/claude-opus-4.6::openrouter::default', 200000, 64000, '["text"]', '["tools"]', NULL);

INSERT INTO terms (sku_id, commitment) VALUES
  ('anthropic/claude-opus-4.6::anthropic::default','on_demand'),
  ('anthropic/claude-opus-4.6::aws-bedrock::default','on_demand'),
  ('anthropic/claude-opus-4.6::openrouter::default','on_demand');

INSERT INTO prices VALUES
  ('anthropic/claude-opus-4.6::anthropic::default','prompt','','',1.5e-5,'token'),
  ('anthropic/claude-opus-4.6::anthropic::default','completion','','',7.5e-5,'token'),
  ('anthropic/claude-opus-4.6::aws-bedrock::default','prompt','','',1.5e-5,'token'),
  ('anthropic/claude-opus-4.6::aws-bedrock::default','completion','','',7.5e-5,'token'),
  ('anthropic/claude-opus-4.6::openrouter::default','prompt','','',1.5e-5,'token'),
  ('anthropic/claude-opus-4.6::openrouter::default','completion','','',7.5e-5,'token');

INSERT INTO health VALUES
  ('anthropic/claude-opus-4.6::anthropic::default', 0.998, 420, 1100, 62.5, 1745020800),
  ('anthropic/claude-opus-4.6::aws-bedrock::default', 0.995, 450, 1300, 55.0, 1745020800);
-- aggregated row has no health
