"""Golden-row test: fixture Route53 offer JSON -> normalized NDJSON matches golden."""

import json
from pathlib import Path

import pytest

from ingest.aws_route53 import ingest
from ._tier_contiguity import assert_tiers_contiguous

FIXTURE = Path(__file__).resolve().parent.parent / "testdata" / "aws_route53" / "offer.json"
GOLDEN = Path(__file__).resolve().parent.parent / "testdata" / "golden" / "aws_route53_rows.jsonl"


def _canonical(rows):
    return sorted(rows, key=lambda r: r["sku_id"])


def test_fixture_matches_golden():
    rows = list(ingest(offer_path=FIXTURE))
    expected = [json.loads(line) for line in GOLDEN.read_text().splitlines() if line.strip()]
    assert _canonical(rows) == _canonical(expected)


def test_all_rows_are_dns_zone_kind():
    rows = list(ingest(offer_path=FIXTURE))
    assert rows, "expected at least one row"
    assert {r["kind"] for r in rows} == {"dns.zone"}


def test_zone_rows_have_hosted_zone_dimension():
    """Zone rows must include prices with dimension='hosted_zone'."""
    rows = list(ingest(offer_path=FIXTURE))
    for r in rows:
        dims = {p["dimension"] for p in r["prices"]}
        assert "hosted_zone" in dims, (
            f"row {r['resource_name']!r} missing hosted_zone dimension"
        )


def test_public_row_has_query_dimension():
    """The public zone row must include a query dimension for DNS query pricing."""
    rows = list(ingest(offer_path=FIXTURE))
    public_rows = [r for r in rows if r["resource_name"] == "public"]
    assert public_rows, "expected at least one public row"
    for r in public_rows:
        dims = {p["dimension"] for p in r["prices"]}
        assert "query" in dims, f"public row missing query dimension"


def test_private_row_has_no_query_dimension():
    """Private zone rows should only have hosted_zone pricing (no query dimension)."""
    rows = list(ingest(offer_path=FIXTURE))
    private_rows = [r for r in rows if r["resource_name"] == "private"]
    assert private_rows, "expected at least one private row"
    for r in private_rows:
        dims = {p["dimension"] for p in r["prices"]}
        assert "query" not in dims, f"private row should not have query dimension"


def test_region_is_global():
    """Route53 is a global service — all rows must use region='global'."""
    rows = list(ingest(offer_path=FIXTURE))
    for r in rows:
        assert r["region"] == "global", f"expected region='global', got {r['region']!r}"
        assert r["region_normalized"] == "global", (
            f"expected region_normalized='global', got {r['region_normalized']!r}"
        )


def test_resource_names():
    """Rows for both public and private zone types must be emitted."""
    rows = list(ingest(offer_path=FIXTURE))
    names = {r["resource_name"] for r in rows}
    assert "public" in names, "missing public zone row"
    assert "private" in names, "missing private zone row"


def test_tiers_contiguous():
    """Zone (hosted_zone) tiers must be contiguous per row; query tiers must be contiguous."""
    rows = list(ingest(offer_path=FIXTURE))
    for row in rows:
        assert_tiers_contiguous([row], "dns.zone", "count")


def test_terms_hash_matches_default():
    """All rows must use the on_demand default terms for dns.zone."""
    from normalize.terms import terms_hash
    from normalize.enums import apply_kind_defaults
    expected_terms = apply_kind_defaults("dns.zone", {
        "commitment": "on_demand",
        "tenancy": "",
        "os": "",
        "support_tier": "",
        "upfront": "",
        "payment_option": "",
    })
    expected_hash = terms_hash(expected_terms)
    rows = list(ingest(offer_path=FIXTURE))
    for r in rows:
        assert r["terms_hash"] == expected_hash, (
            f"row {r['resource_name']!r} has unexpected terms_hash {r['terms_hash']!r}"
        )


def test_zone_first_tier_price():
    """The first hosted_zone tier (0-25) must be $0.50/zone/month."""
    rows = list(ingest(offer_path=FIXTURE))
    for r in rows:
        zone_prices = [p for p in r["prices"] if p["dimension"] == "hosted_zone"]
        first_tier = next((p for p in zone_prices if p["tier"] == "0"), None)
        assert first_tier is not None, f"no tier 0 for hosted_zone in row {r['resource_name']!r}"
        assert first_tier["amount"] == pytest.approx(0.50, rel=1e-9)
        assert first_tier["tier_upper"] == "25"
