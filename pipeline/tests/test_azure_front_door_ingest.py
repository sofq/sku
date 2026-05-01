"""Golden-row test: fixture Azure Front Door prices.json -> normalized NDJSON matches golden."""

from __future__ import annotations

import json
from pathlib import Path

from ingest.azure_front_door import _EGRESS_TIERS, ingest
from normalize.tier_tokens import TIER_TOKENS_BYTES, parse_bytes_tier
from ._tier_contiguity import assert_tiers_contiguous

_DATA = Path(__file__).resolve().parent.parent / "testdata"
FIXTURE = _DATA / "azure_front_door" / "prices.json"
GOLDEN = _DATA / "golden" / "azure_front_door_rows.jsonl"


def _canonical(rows):
    return sorted(rows, key=lambda r: r["sku_id"])


def test_fixture_matches_golden():
    rows = list(ingest(prices_path=FIXTURE))
    expected = [json.loads(line) for line in GOLDEN.read_text().splitlines() if line.strip()]
    assert _canonical(rows) == _canonical(expected)


def test_all_rows_are_network_cdn_kind():
    rows = list(ingest(prices_path=FIXTURE))
    assert rows
    assert {r["kind"] for r in rows} == {"network.cdn"}


def test_base_fee_rows_are_global():
    rows = list(ingest(prices_path=FIXTURE))
    base_fee_rows = [r for r in rows if r["resource_attrs"]["extra"]["mode"] == "base-fee"]
    assert base_fee_rows, "expected at least one base-fee row"
    for r in base_fee_rows:
        assert r["region"] == "global", (
            f"base-fee row {r['sku_id']} has region={r['region']!r}, expected 'global'"
        )
        assert r["region_normalized"] == "global", (
            f"base-fee row {r['sku_id']} has region_normalized={r['region_normalized']!r}"
        )


def test_egress_rows_have_tier_structure():
    rows = list(ingest(prices_path=FIXTURE))
    egress_rows = [r for r in rows if r["resource_attrs"]["extra"]["mode"] == "edge-egress"]
    assert egress_rows, "expected at least one edge-egress row"
    for r in egress_rows:
        assert len(r["prices"]) > 1, (
            f"egress row {r['sku_id']} has only {len(r['prices'])} price entry (expected multiple tiers)"
        )
        for p in r["prices"]:
            assert p["dimension"] == "egress"
            assert p["unit"] == "gb"


def test_request_rows_have_single_tier():
    rows = list(ingest(prices_path=FIXTURE))
    request_rows = [r for r in rows if r["resource_attrs"]["extra"]["mode"] == "request"]
    assert request_rows, "expected at least one request row"
    for r in request_rows:
        assert len(r["prices"]) == 1, (
            f"request row {r['sku_id']} has {len(r['prices'])} price entries, expected 1"
        )
        p = r["prices"][0]
        assert p["dimension"] == "request"
        assert p["tier"] == "0"
        assert p["tier_upper"] == ""
        assert p["unit"] == "requests"


def test_egress_tier_tokens_in_canonical_vocabulary():
    """The hardcoded `_EGRESS_TIERS` sequence must use tokens that exist in the
    shared `TIER_TOKENS_BYTES` vocabulary; otherwise the Go side
    (`internal/estimate/tiertokens.go`, generated from this set) won't be able
    to parse the emitted tier boundaries and `parseTierBoundBytes` will fall
    back to numeric parsing."""
    for tier_lower, tier_upper in _EGRESS_TIERS:
        assert tier_lower in TIER_TOKENS_BYTES, (
            f"_EGRESS_TIERS lower bound {tier_lower!r} not in TIER_TOKENS_BYTES; "
            f"add it to pipeline/normalize/tier_tokens.py and rerun "
            f"`make generate-go-tier-tokens`"
        )
        if tier_upper:
            assert tier_upper in TIER_TOKENS_BYTES, (
                f"_EGRESS_TIERS upper bound {tier_upper!r} not in TIER_TOKENS_BYTES"
            )


def test_egress_tiers_contiguous():
    """Verify that egress tiers within each row are contiguous (tier[i].tier_upper == tier[i+1].tier)."""
    rows = list(ingest(prices_path=FIXTURE))
    egress_rows = [r for r in rows if r["resource_attrs"]["extra"]["mode"] == "edge-egress"]
    assert egress_rows, "expected at least one edge-egress row"
    for row in egress_rows:
        prices = sorted(row["prices"], key=lambda p: parse_bytes_tier(p["tier"]))
        assert prices, f"row {row['sku_id']} has no prices"
        for i, p in enumerate(prices):
            if i < len(prices) - 1:
                assert p["tier_upper"] != "", (
                    f"row {row['sku_id']} tier[{i}].tier_upper is empty (non-last entry)"
                )
                assert p["tier_upper"] == prices[i + 1]["tier"], (
                    f"row {row['sku_id']} tier[{i}].tier_upper={p['tier_upper']!r} "
                    f"!= tier[{i+1}].tier={prices[i+1]['tier']!r} (not contiguous)"
                )
            else:
                assert p["tier_upper"] == "", (
                    f"row {row['sku_id']} last tier should have tier_upper='', "
                    f"got {p['tier_upper']!r}"
                )
