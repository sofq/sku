"""Shard definition schema — single source of truth for sku shards.

Each `pipeline/shards/<shard>.yaml` declares one shard's metadata. This module
parses them into `ShardDef` objects; code generators consume the result.

Validation is intentionally strict — the generators assume well-formed input
and will emit corrupt Python/Go if this module lets a malformed YAML slip
through.

Price-row schema note (M-δ): individual price entries in normalized NDJSON rows
accept a ``tier_upper: str`` field (default ``""`` meaning +∞). This field is
not validated here — it lives in ``pipeline/package/build_shard.py`` which
reads each ``p.get("tier_upper", "")`` when inserting into the prices table.
See ``pipeline/normalize/tier_tokens.py`` for the closed vocabularies.
"""

from __future__ import annotations

from dataclasses import dataclass, field
from pathlib import Path

import yaml

_VALID_PROVIDERS: frozenset[str] = frozenset({"aws", "azure", "gcp", "openrouter"})
_VALID_KINDS: frozenset[str] = frozenset({
    "compute.vm",
    "compute.container",          # reserved for Fargate/ACI/Cloud Run reclass (M3b.4 note)
    "compute.serverless",
    "storage.object",
    "storage.block",
    "db.relational",
    "db.nosql",
    "network.cdn",
    "llm.text",
    "cache.kv",
    "container.orchestration",    # M-γ.2: EKS/AKS/GKE control plane + serverless modes
    "search.engine",              # M-γ.3: aws_opensearch
    "paas.app",                   # M-γ.3: azure_appservice
    "warehouse.query",            # M-γ.3: gcp_bigquery
    "messaging.queue",            # M-δ: SQS/Service Bus/Cloud Tasks
    "messaging.topic",            # M-δ: SNS/Event Hubs/Pub Sub
    "dns.zone",                   # M-δ: Route 53/Azure DNS/Cloud DNS
    "api.gateway",                # M-δ: API GW/APIM/Cloud Endpoints
})


@dataclass(frozen=True)
class CLISpec:
    group: str
    command: str
    resource_flag: str


@dataclass(frozen=True)
class IngestSpec:
    module: str
    discover: str


@dataclass(frozen=True)
class ShardDef:
    shard: str
    provider: str
    service: str
    kind: str
    cli: CLISpec
    ingest: IngestSpec
    budget_bytes: int
    extra: dict = field(default_factory=dict)


def _parse_one(path: Path) -> ShardDef:
    doc = yaml.safe_load(path.read_text())
    if not isinstance(doc, dict):
        raise ValueError(f"{path}: not a mapping")

    shard = doc.get("shard")
    if shard != path.stem:
        raise ValueError(
            f"{path}: filename/shard mismatch (filename={path.stem!r}, shard={shard!r})"
        )

    provider = doc.get("provider")
    if provider not in _VALID_PROVIDERS:
        raise ValueError(f"{path}: provider {provider!r} not in {sorted(_VALID_PROVIDERS)}")

    kind = doc.get("kind")
    if kind not in _VALID_KINDS:
        raise ValueError(f"{path}: kind {kind!r} not in {sorted(_VALID_KINDS)}")

    cli = doc.get("cli") or {}
    ingest = doc.get("ingest") or {}

    budget = doc.get("budget_bytes")
    if not isinstance(budget, int) or budget <= 0:
        raise ValueError(f"{path}: budget_bytes must be a positive int, got {budget!r}")

    return ShardDef(
        shard=shard,
        provider=provider,
        service=doc.get("service", ""),
        kind=kind,
        cli=CLISpec(
            group=cli.get("group", ""),
            command=cli.get("command", ""),
            resource_flag=cli.get("resource_flag", ""),
        ),
        ingest=IngestSpec(
            module=ingest.get("module", shard),
            discover=ingest.get("discover", ""),
        ),
        budget_bytes=budget,
        extra={k: v for k, v in doc.items() if k not in {
            "shard", "provider", "service", "kind", "cli", "ingest",
            "budget_bytes",
        }},
    )


def load_all(shards_dir: Path | str) -> dict[str, ShardDef]:
    p = Path(shards_dir)
    out: dict[str, ShardDef] = {}
    for f in sorted(p.glob("*.yaml")):
        s = _parse_one(f)
        out[s.shard] = s
    return out
