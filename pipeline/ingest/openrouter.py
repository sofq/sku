"""Normalize OpenRouter's two endpoints into row dicts ready for shard packaging.

Spec §3 "OpenRouter-specific ingest": one row per (model, serving_provider)
pair plus one synthetic aggregated row per model with provider='openrouter'.
"""

from __future__ import annotations

import argparse
import hashlib
import json
import os
import sys
import time
from collections.abc import Iterable
from pathlib import Path
from typing import Any

from normalize.enums import apply_kind_defaults, load_enums
from normalize.terms import terms_hash

from .http import FixtureClient, LiveClient


class NonUSDError(RuntimeError):
    """Raised when an upstream endpoint declares a non-USD currency."""


def _kind_for_modality(modality: str | None, input_modalities: list[str] | None) -> str:
    """Map OpenRouter modality hints to the sku kind taxonomy."""
    mods = {m.lower() for m in (input_modalities or [])}
    if (modality or "").lower() == "text" and mods <= {"text"}:
        return "llm.text"
    # Any non-text input modality -> multimodal.
    return "llm.multimodal"


def _pricing_dimensions(pricing: dict[str, Any]) -> list[dict[str, Any]]:
    """OpenRouter pricing fields -> sku price rows.

    OpenRouter uses per-token unit prices (USD per token). We publish them as-is
    with unit='token' so the renderer can reason about dimensions uniformly.
    Zero-priced dimensions are omitted (a row with amount=0 carries no info).
    """
    out: list[dict[str, Any]] = []
    mapping = {
        "prompt": "prompt",
        "completion": "completion",
        "image": "image",
        "request": "request",
    }
    for src, dim in mapping.items():
        raw = pricing.get(src)
        if raw is None or raw == "":
            continue
        amount = float(raw)
        if amount == 0.0:
            continue
        unit = "request" if dim == "request" else "token"
        out.append({"dimension": dim, "tier": "", "amount": amount, "unit": unit})
    return out


def _build_row(
    *,
    model: dict[str, Any],
    endpoint: dict[str, Any] | None,
    aggregated: bool,
) -> dict[str, Any]:
    """Construct one row dict. `endpoint=None` + `aggregated=True` builds the
    synthetic `::openrouter::default` row from the model's top-level pricing."""
    author_slug = model["id"]  # e.g. "anthropic/claude-opus-4.6"
    arch = model.get("architecture") or {}
    modality = arch.get("modality")
    input_modalities = arch.get("input_modalities") or []
    kind = _kind_for_modality(modality, input_modalities)

    if aggregated:
        serving_provider = "openrouter"
        quantization = None
        pricing = model.get("pricing") or {}
        top = model.get("top_provider") or {}
        context_length = top.get("context_length") or model.get("context_length")
        max_completion = top.get("max_completion_tokens")
        health: dict[str, Any] | None = None
    else:
        assert endpoint is not None
        serving_provider = endpoint.get("tag") or endpoint.get("provider_name")
        if not serving_provider:
            raise ValueError(f"missing serving provider for {author_slug}")
        quantization = endpoint.get("quantization")
        pricing = endpoint.get("pricing") or {}
        context_length = endpoint.get("context_length")
        max_completion = endpoint.get("max_completion_tokens")
        latency = endpoint.get("latency") or {}
        _fixed = os.environ.get("SKU_FIXED_OBSERVED_AT")
        observed_at = int(_fixed) if _fixed else int(time.time())
        health = {
            "uptime_30d": endpoint.get("uptime_last_30m"),
            "latency_p50_ms": latency.get("p50_ms"),
            "latency_p95_ms": latency.get("p95_ms"),
            "throughput_tokens_per_sec": endpoint.get("throughput_tokens_per_second"),
            "observed_at": observed_at,
        }

    # USD guard — spec §5 "OpenRouter currency guard"
    currency = (pricing.get("currency") or "USD").upper()
    if currency != "USD":
        raise NonUSDError(
            f"non-usd-endpoint: {author_slug}/{serving_provider} (currency={currency!r})"
        )

    quant_slug = quantization or "default"
    sku_id = f"{author_slug}::{serving_provider}::{quant_slug}"

    # Terms: on_demand with all other fields empty for LLMs.
    terms_raw = apply_kind_defaults(kind, {"commitment": "on_demand"})
    # (apply_kind_defaults fills the LLM defaults: tenancy="", os="", etc.)

    capabilities = model.get("supported_parameters") or []

    row: dict[str, Any] = {
        "sku_id": sku_id,
        "provider": serving_provider,
        "service": "llm",
        "kind": kind,
        "resource_name": author_slug,
        "region": "",
        "region_normalized": "",
        "terms": terms_raw,
        "terms_hash": terms_hash(terms_raw),
        "resource_attrs": {
            "context_length": context_length,
            "max_output_tokens": max_completion,
            "modality": input_modalities,
            "capabilities": capabilities,
            "quantization": quantization,
        },
        "prices": _pricing_dimensions(pricing),
        "health": health,
        "is_aggregated": aggregated,
    }
    return row


def ingest(
    client: LiveClient | FixtureClient,
    *,
    generated_at: str,
    skip_non_usd: bool = False,
) -> list[dict[str, Any]]:
    """Normalize everything OpenRouter exposes into row dicts.

    By default, errors out on the first non-USD endpoint (spec §5 guard — the
    invariant release CI depends on). With skip_non_usd=True, non-USD endpoints
    are written to stderr and skipped instead; aggregated rows for a model
    whose own top-level pricing is non-USD are likewise skipped. This mode is
    used by the `make openrouter-shard` developer target against fixtures that
    include a non-USD case for the guard test — real release ingests never set
    it.
    """
    enums = load_enums()  # side-effect: validates the YAML is parseable
    _ = enums  # suppress unused in M1; real enum validation lands as rows are built
    _ = generated_at  # threaded through for future metadata population

    models_payload = client.get("/api/v1/models")
    rows: list[dict[str, Any]] = []
    for model in models_payload.get("data", []):
        # Fetch per-model endpoint detail.
        slug = model["id"]
        ep_payload = client.get(f"/api/v1/models/{slug}/endpoints")
        endpoints = (ep_payload.get("data") or {}).get("endpoints") or []
        if not endpoints:
            # OpenRouter sometimes lists a model with no concrete endpoints
            # (deprecated or region-restricted). Skip — the aggregated row
            # would be a lie without backing endpoints.
            continue
        model_rows: list[dict[str, Any]] = []
        model_skipped = False
        for ep in endpoints:
            try:
                model_rows.append(_build_row(model=model, endpoint=ep, aggregated=False))
            except NonUSDError as e:
                if not skip_non_usd:
                    raise
                sys.stderr.write(f"ingest.openrouter: skip {e}\n")
                model_skipped = True
        # Only emit the aggregated row if the model's top-level pricing is USD.
        try:
            agg = _build_row(model=model, endpoint=None, aggregated=True)
        except NonUSDError as e:
            if not skip_non_usd:
                raise
            sys.stderr.write(f"ingest.openrouter: skip aggregated {e}\n")
            model_skipped = True
            agg = None
        rows.extend(model_rows)
        if agg is not None:
            rows.append(agg)
        if model_skipped and not model_rows:
            # All endpoints for this model were non-USD — nothing to publish.
            continue
    return _dedupe_rows(rows)


def _uptime(row: dict[str, Any]) -> float:
    h = row.get("health") or {}
    v = h.get("uptime_30d")
    return float(v) if v is not None else -1.0


def _dedupe_rows(rows: list[dict[str, Any]]) -> list[dict[str, Any]]:
    """Collapse rows sharing a sku_id.

    OpenRouter occasionally lists multiple endpoints for the same
    (model, serving_provider, quantization) tuple — Bedrock/Vertex
    regional splits, for one. When prices match (the common case), we
    keep the endpoint with the highest ``uptime_30d`` and log. When
    prices diverge and upstream exposes no disambiguator we can put on
    the sku_id, we drop all rows in that group rather than silently
    publish a coin-flip price; the model's synthetic aggregated row
    still ships from the top-level pricing.
    """
    grouped: dict[str, list[dict[str, Any]]] = {}
    order: list[str] = []
    for r in rows:
        sid = r["sku_id"]
        if sid not in grouped:
            grouped[sid] = []
            order.append(sid)
        grouped[sid].append(r)

    out: list[dict[str, Any]] = []
    for sid in order:
        group = grouped[sid]
        if len(group) == 1:
            out.append(group[0])
            continue
        prices = group[0]["prices"]
        if all(r["prices"] == prices for r in group):
            sys.stderr.write(
                f"ingest.openrouter: merged {len(group)} rows for duplicate sku_id {sid}\n"
            )
            out.append(max(group, key=_uptime))
            continue
        sys.stderr.write(
            f"ingest.openrouter: dropped duplicate sku_id with divergent prices: {sid}\n"
        )
    return out


def _write_rows(rows: Iterable[dict[str, Any]], out: Path | None) -> None:
    def encode(r: dict[str, Any]) -> str:
        return json.dumps(r, separators=(",", ":"), sort_keys=True, ensure_ascii=False)

    if out is None:
        for r in rows:
            sys.stdout.write(encode(r) + "\n")
    else:
        out.parent.mkdir(parents=True, exist_ok=True)
        with out.open("w") as fh:
            for r in rows:
                fh.write(encode(r) + "\n")


def fetch(target_dir: Path, *, client: LiveClient | None = None) -> str:
    """Materialise a full OpenRouter fixture tree under `target_dir`:

        target_dir/models.json
        target_dir/endpoints/{author}__{slug}.json

    Returns SHA256 hex digest over the sorted list of ``id`` strings from
    ``models.json.data[*].id`` — the version indicator for the OpenRouter shard.
    Atomic per-file write: write to ``<name>.part``, os.replace onto final name.
    Creates parent directories as needed.
    """
    client = client or LiveClient()

    models_payload = client.get("/api/v1/models")

    # Write models.json atomically.
    target_dir = Path(target_dir)
    target_dir.mkdir(parents=True, exist_ok=True)
    models_content = json.dumps(models_payload, sort_keys=True, separators=(",", ":"))
    models_final = target_dir / "models.json"
    models_part = target_dir / "models.json.part"
    models_part.write_text(models_content, encoding="utf-8")
    os.replace(models_part, models_final)

    # Write one endpoint file per model, atomically.
    endpoints_dir = target_dir / "endpoints"
    for model in models_payload.get("data", []):
        model_id: str = model["id"]
        ep_payload = client.get(f"/api/v1/models/{model_id}/endpoints")
        ep_content = json.dumps(ep_payload, sort_keys=True, separators=(",", ":"))
        flat = model_id.replace("/", "__")
        endpoints_dir.mkdir(parents=True, exist_ok=True)
        ep_final = endpoints_dir / f"{flat}.json"
        ep_part = endpoints_dir / f"{flat}.json.part"
        ep_part.write_text(ep_content, encoding="utf-8")
        os.replace(ep_part, ep_final)

    # Compute version digest over sorted model id list.
    sorted_ids = sorted(m["id"] for m in models_payload.get("data", []))
    digest = hashlib.sha256(json.dumps(sorted_ids).encode()).hexdigest()
    return digest


def main() -> int:
    ap = argparse.ArgumentParser(prog="ingest.openrouter")
    ap.add_argument("--out", type=Path, help="write NDJSON rows here (stdout if omitted)")
    ap.add_argument("--fixture", type=Path, help="use FixtureClient rooted at this directory")
    ap.add_argument("--generated-at", default="", help="ISO-8601 UTC; default now")
    ap.add_argument(
        "--skip-non-usd",
        action="store_true",
        help="log and skip non-USD endpoints instead of failing (dev/fixture use only)",
    )
    args = ap.parse_args()

    generated_at = args.generated_at or time.strftime("%Y-%m-%dT%H:%M:%SZ", time.gmtime())
    if args.fixture:
        client: LiveClient | FixtureClient = FixtureClient(args.fixture)
    else:
        client = LiveClient()

    try:
        rows = ingest(client, generated_at=generated_at, skip_non_usd=args.skip_non_usd)
    except NonUSDError as e:
        sys.stderr.write(f"ingest.openrouter: {e}\n")
        return 4

    _write_rows(rows, args.out)
    return 0


if __name__ == "__main__":
    sys.exit(main())
